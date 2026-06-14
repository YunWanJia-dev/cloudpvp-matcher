package asynclock

import (
	"github.com/go-redsync/redsync/v4"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredis "github.com/redis/go-redis/v9"
)

// RedisLockManager 基于 RedSync 实现跨实例互斥锁。
type RedisLockManager struct {
	redsync *redsync.Redsync
}

// NewRedisLockManager 创建 Redis 分布式锁管理器。
func NewRedisLockManager(client *goredis.Client) *RedisLockManager {
	return &RedisLockManager{
		redsync: redsync.New(redsyncgoredis.NewPool(client)),
	}
}
