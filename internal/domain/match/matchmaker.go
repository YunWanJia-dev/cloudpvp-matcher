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

	// FindMatch 尝试在池中为给定候选者寻找匹配的对手。
	// 如果找到对手则返回 Match，否则返回 nil（暂无合适对手）。
	FindMatch(candidate *ticket.Ticket, pool []*ticket.Ticket) *Match
}
