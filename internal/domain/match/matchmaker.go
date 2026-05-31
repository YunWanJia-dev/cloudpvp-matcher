package match

import (
	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"
)

// Matchmaker 是从票据池中寻找对手配对的核心领域服务接口。
// 每个游戏模式提供各自的 Matchmaker 实现。
type Matchmaker interface {
	// Supports 返回此匹配器是否支持给定的游戏模式。
	Supports(mode config.GameMode) bool

	// FindMatch 尝试从池中组装一个满足配置的完整对局。
	// 如果找到对手则返回 Match，否则返回 nil（暂无合适对手）。
	FindMatch(pool []*ticket.Ticket, cfg *config.MatchConfig) *Match
}
