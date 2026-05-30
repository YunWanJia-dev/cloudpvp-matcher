package match

import (
	"time"

	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"
)

// CSGO5v5Matchmaker 实现 CS:GO 5v5 竞技匹配的 Matchmaker 接口。
// 它将两支满员的 5 人队伍配对在一起。
type CSGO5v5Matchmaker struct{}

// NewCSGO5v5Matchmaker 创建一个新的 CS:GO 5v5 匹配器。
func NewCSGO5v5Matchmaker() *CSGO5v5Matchmaker {
	return &CSGO5v5Matchmaker{}
}

// Supports 仅在模式为 CS:GO 5v5 竞技时返回 true。
func (m *CSGO5v5Matchmaker) Supports(mode config.GameMode) bool {
	return mode == config.GameModeCSGO5v5
}

// FindMatch 从池中找到另一支 5 人队伍组成对局。
// 当前实现按顺序选择首个符合条件者，后续可扩展 MMR 排序和区域偏好。
func (m *CSGO5v5Matchmaker) FindMatch(candidate *ticket.Ticket, pool []*ticket.Ticket) *Match {
	for _, t := range pool {
		if t.ID == candidate.ID {
			continue
		}
		if t.Status != ticket.TicketStatusMatching {
			continue
		}
		if len(t.Members) == 5 && len(candidate.Members) == 5 {
			now := time.Now()
			return &Match{
				ID:        "", // 由用例层分配
				GameMode:  candidate.GameMode,
				Tickets:   []*ticket.Ticket{candidate, t},
				Status:    MatchStatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}
		}
	}
	return nil
}
