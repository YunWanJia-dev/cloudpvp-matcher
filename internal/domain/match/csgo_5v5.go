package match

import (
	"sort"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"
)

// CSGO5v5Matchmaker 实现 CS:GO 5v5 竞技匹配的 Matchmaker 接口。
// 它将队列中的 lobby 拼成两支 5 人队伍。
type CSGO5v5Matchmaker struct{}

var _ Matchmaker = (*CSGO5v5Matchmaker)(nil)

// NewCSGO5v5Matchmaker 创建一个新的 CS:GO 5v5 匹配器。
func NewCSGO5v5Matchmaker() *CSGO5v5Matchmaker {
	return &CSGO5v5Matchmaker{}
}

// Supports 仅在模式为 CS:GO 5v5 竞技时返回 true。
func (m *CSGO5v5Matchmaker) Supports(mode config.GameMode) bool {
	return mode == config.GameModeCSGO5v5
}

// FindMatch 从池中组装满足 5v5 配置的两个队伍。
func (m *CSGO5v5Matchmaker) FindMatch(pool []*ticket.Ticket, cfg *config.MatchConfig) *Match {
	if cfg == nil || cfg.GameMode != config.GameModeCSGO5v5 || cfg.TeamSize <= 0 || cfg.TeamCount <= 0 {
		return nil
	}

	candidates := eligibleTickets(pool, cfg)
	teams, ok := buildTeams(candidates, cfg.TeamCount, cfg.TeamSize)
	if !ok {
		return nil
	}

	now := time.Now()
	return &Match{
		ID:        "", // 由用例层分配
		GameMode:  cfg.GameMode,
		Teams:     teams,
		Status:    MatchStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// eligibleTickets 过滤出可参与当前配置匹配的候选票据。
func eligibleTickets(pool []*ticket.Ticket, cfg *config.MatchConfig) []*ticket.Ticket {
	candidates := make([]*ticket.Ticket, 0, len(pool))
	seenLobbyIDs := make(map[string]struct{}, len(pool))
	for _, t := range pool {
		if t == nil || t.GameMode != cfg.GameMode || t.LobbyID == "" {
			continue
		}
		if _, exists := seenLobbyIDs[t.LobbyID]; exists {
			continue
		}
		memberCount := t.TeamSize()
		if memberCount <= 0 || memberCount > cfg.TeamSize {
			continue
		}
		seenLobbyIDs[t.LobbyID] = struct{}{}
		candidates = append(candidates, t)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].LobbyID < candidates[j].LobbyID
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})
	return candidates
}

// buildTeams 从候选票据中搜索满足队伍数量和人数要求的队伍组合。
func buildTeams(candidates []*ticket.Ticket, teamCount, teamSize int) ([]Team, bool) {
	used := make([]bool, len(candidates))
	teams := make([]Team, 0, teamCount)

	var search func() bool
	search = func() bool {
		if len(teams) == teamCount {
			return true
		}

		for _, indexes := range teamCombinations(candidates, used, teamSize) {
			for _, idx := range indexes {
				used[idx] = true
			}
			teams = append(teams, Team{Tickets: ticketsAt(candidates, indexes)})

			if search() {
				return true
			}

			teams = teams[:len(teams)-1]
			for _, idx := range indexes {
				used[idx] = false
			}
		}
		return false
	}

	if !search() {
		return nil, false
	}
	return teams, true
}

// teamCombinations 枚举未使用候选票据中可凑满目标人数的组合。
func teamCombinations(candidates []*ticket.Ticket, used []bool, targetSize int) [][]int {
	var result [][]int
	var current []int

	var search func(start, currentSize int)
	search = func(start, currentSize int) {
		if currentSize == targetSize {
			combination := append([]int(nil), current...)
			result = append(result, combination)
			return
		}
		if currentSize > targetSize {
			return
		}

		for i := start; i < len(candidates); i++ {
			if used[i] {
				continue
			}
			nextSize := currentSize + candidates[i].TeamSize()
			if nextSize > targetSize {
				continue
			}
			current = append(current, i)
			search(i+1, nextSize)
			current = current[:len(current)-1]
		}
	}

	search(0, 0)
	return result
}

// ticketsAt 按候选索引取出票据列表。
func ticketsAt(candidates []*ticket.Ticket, indexes []int) []*ticket.Ticket {
	tickets := make([]*ticket.Ticket, 0, len(indexes))
	for _, idx := range indexes {
		tickets = append(tickets, candidates[idx])
	}
	return tickets
}
