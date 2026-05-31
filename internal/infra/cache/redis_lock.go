package cache

import (
	"context"
	"fmt"
	"time"

	domainmatch "cloudpvp-matcher/internal/domain/match"

	"github.com/go-redsync/redsync/v4"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredis "github.com/redis/go-redis/v9"
)

// RedisLockManager 基于 RedSync 实现跨实例互斥锁。
type RedisLockManager struct {
	redsync *redsync.Redsync
}

var _ domainmatch.LockManager = (*RedisLockManager)(nil)

// NewRedisLockManager 创建 Redis 分布式锁管理器。
func NewRedisLockManager(client *goredis.Client) *RedisLockManager {
	return &RedisLockManager{
		redsync: redsync.New(redsyncgoredis.NewPool(client)),
	}
}

// WithLock 在持有指定锁时执行函数。
func (m *RedisLockManager) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error {
	mutex := m.redsync.NewMutex(
		key,
		redsync.WithExpiry(ttl),
		redsync.WithTries(1),
	)
	if err := mutex.LockContext(ctx); err != nil {
		return fmt.Errorf("redis lock: acquire %s: %w", key, err)
	}

	defer func() {
		_, _ = mutex.UnlockContext(context.Background())
	}()

	return fn(ctx)
}
