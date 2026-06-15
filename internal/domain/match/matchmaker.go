package match

import (
	"context"

	"cloudpvp-matcher/internal/domain/config"
	domainlobby "cloudpvp-matcher/internal/domain/lobby"
)

// Matchmaker 是单个游戏模式的匹配流程处理器。
// 每个实现自行负责该模式的校验、持久化、候选查询和取消逻辑。
type Matchmaker interface {
	// Mode 返回该处理器负责的游戏模式。
	Mode() config.GameMode

	// Submit 处理指定模式的开始匹配请求。
	Submit(ctx context.Context, lobby *domainlobby.Lobby) error

	// Cancel 处理指定模式的取消匹配请求。
	Cancel(ctx context.Context, lobbyID string) error
}
