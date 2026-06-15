package asynclock

import (
	domainmatch "cloudpvp-matcher/internal/domain/lobby"
	"context"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
)

const lobbyLockTTL = 5 * time.Second

var _ domainmatch.LockManager = (*RedisLockManager)(nil)

// WithLobbyLock 在持有一个或多个 lobby 锁时执行函数。
func (m *RedisLockManager) WithLobbyLock(ctx context.Context, lobbyIDs []string, fn func(context.Context) error) error {
	if len(lobbyIDs) == 0 {
		return fn(ctx)
	}

	mutexes := make([]*redsync.Mutex, 0, len(lobbyIDs))

	defer func() {
		unlockLobbyMutexes(mutexes)
	}()

	for _, lobbyID := range lobbyIDs {
		key := fmt.Sprintf("matcher:lock:lobby:%s", lobbyID)
		mutex := m.redsync.NewMutex(
			key,
			redsync.WithExpiry(lobbyLockTTL),
			redsync.WithTries(1),
		)
		if err := mutex.LockContext(ctx); err != nil {
			return fmt.Errorf("redis lock: acquire %s: %w", key, err)
		}
		mutexes = append(mutexes, mutex)
	}

	return fn(ctx)
}

// unlockLobbyMutexes 按加锁相反顺序释放已经获取的锁。
func unlockLobbyMutexes(mutexes []*redsync.Mutex) {
	for i := len(mutexes) - 1; i >= 0; i-- {
		_, _ = mutexes[i].UnlockContext(context.Background())
	}
}
