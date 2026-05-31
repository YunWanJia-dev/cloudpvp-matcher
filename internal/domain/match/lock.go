package match

import (
	"context"
	"time"
)

// LockManager 为跨实例匹配扫描提供互斥执行能力。
type LockManager interface {
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error
}
