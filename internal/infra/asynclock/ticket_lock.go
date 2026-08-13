package asynclock

import (
	domainmatch "cloudpvp-matcher/internal/domain/lobby"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
)

const lobbyLockTTL = 5 * time.Second
const lobbyLockRenewInterval = lobbyLockTTL / 3
const lobbyLockAcquireTries = 5

var _ domainmatch.LockManager = (*RedisLockManager)(nil)

type lobbyMutex interface {
	Name() string
	ExtendContext(context.Context) (bool, error)
	UnlockContext(context.Context) (bool, error)
}

// WithLobbyLock 在持有一个或多个 lobby 锁时执行函数。
func (m *RedisLockManager) WithLobbyLock(ctx context.Context, lobbyIDs []string, fn func(context.Context) error) error {
	if len(lobbyIDs) == 0 {
		return fn(ctx)
	}

	mutexes := make([]lobbyMutex, 0, len(lobbyIDs))

	defer func() {
		unlockLobbyMutexes(mutexes)
	}()

	for _, lobbyID := range lobbyIDs {
		key := fmt.Sprintf("matcher:lock:lobby:%s", lobbyID)
		mutex := m.redsync.NewMutex(
			key,
			redsync.WithExpiry(lobbyLockTTL),
			redsync.WithTries(lobbyLockAcquireTries),
		)
		if err := mutex.LockContext(ctx); err != nil {
			return fmt.Errorf("redis lock: acquire %s: %w", key, err)
		}
		mutexes = append(mutexes, mutex)
	}

	return runWithLobbyMutexRenewal(ctx, mutexes, lobbyLockRenewInterval, fn)
}

// runWithLobbyMutexRenewal 在回调执行期间续租锁；任一锁续租失败会立即取消临界区上下文。
func runWithLobbyMutexRenewal(ctx context.Context, mutexes []lobbyMutex, renewInterval time.Duration, fn func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renewResult := make(chan error, 1)
	callbackDone := make(chan struct{})
	go func() {
		renewErr := renewLobbyMutexes(runCtx, mutexes, renewInterval, callbackDone)
		if renewErr != nil {
			cancel()
		}
		renewResult <- renewErr
	}()

	callbackErr := fn(runCtx)
	close(callbackDone)
	cancel()
	renewErr := <-renewResult
	if renewErr != nil {
		return errors.Join(callbackErr, fmt.Errorf("redis lock renewal failed: %w", renewErr))
	}
	return callbackErr
}

// renewLobbyMutexes 定期续租全部大厅锁。
func renewLobbyMutexes(ctx context.Context, mutexes []lobbyMutex, renewInterval time.Duration, callbackDone <-chan struct{}) error {
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-callbackDone:
			return nil
		case <-ticker.C:
			for _, mutex := range mutexes {
				extended, err := mutex.ExtendContext(ctx)
				select {
				case <-callbackDone:
					return nil
				default:
				}
				if err != nil || !extended {
					if err == nil {
						err = fmt.Errorf("lock lease expired")
					}
					return fmt.Errorf("extend %s: %w", mutex.Name(), err)
				}
			}
		}
	}
}

// unlockLobbyMutexes 按加锁相反顺序释放已经获取的锁。
func unlockLobbyMutexes(mutexes []lobbyMutex) {
	for i := len(mutexes) - 1; i >= 0; i-- {
		_, _ = mutexes[i].UnlockContext(context.Background())
	}
}
