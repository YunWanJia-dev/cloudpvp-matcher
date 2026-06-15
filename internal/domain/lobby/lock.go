package lobby

import (
	"context"
)

// LockManager 为跨实例匹配扫描提供互斥执行能力。
type LockManager interface {
	WithLobbyLock(ctx context.Context, lobbyIDs []string, fn func(context.Context) error) error
}
