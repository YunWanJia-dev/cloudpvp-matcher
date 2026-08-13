package csgo_5v5

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainlobby "cloudpvp-matcher/internal/domain/lobby"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
)

const (
	csgo5v5TeamSize             = 5
	csgo5v5TeamCount            = 2
	csgo5v5CandidateLimitPerBin = 20
)

// CSGO5v5Matchmaker 实现 CS:GO 5v5 竞技匹配的入队和取消流程。
type CSGO5v5Matchmaker struct {
	queueRepo LobbyQueueRepository
}

var _ domainmatch.Matchmaker = (*CSGO5v5Matchmaker)(nil)

// NewCSGO5v5Matchmaker 创建一个新的 CS:GO 5v5 匹配器。
func NewCSGO5v5Matchmaker(queueRepo LobbyQueueRepository) *CSGO5v5Matchmaker {
	return &CSGO5v5Matchmaker{
		queueRepo: queueRepo,
	}
}

// Mode 返回当前匹配器负责的游戏模式。
func (m *CSGO5v5Matchmaker) Mode() config.GameMode {
	return config.GameModeCSGO5v5
}

// Submit 校验 lobby 并按成员数量写入对应人数桶。
func (m *CSGO5v5Matchmaker) Submit(ctx context.Context, lobby *domainlobby.Lobby) error {
	if m == nil || m.queueRepo == nil {
		return fmt.Errorf("csgo 5v5 matchmaker: queue repository is nil")
	}
	if lobby == nil {
		return fmt.Errorf("csgo 5v5 matchmaker: lobby is nil")
	}

	lobbyID := strings.TrimSpace(lobby.LobbyID)
	if lobbyID == "" {
		return fmt.Errorf("csgo 5v5 matchmaker: lobby_id is required")
	}
	if lobby.GameMode != "" && lobby.GameMode != config.GameModeCSGO5v5 {
		return fmt.Errorf("csgo 5v5 matchmaker: unsupported game_mode=%s", lobby.GameMode)
	}

	memberCount := lobby.TeamSize()
	if memberCount <= 0 {
		return fmt.Errorf("csgo 5v5 matchmaker: lobby member count must be positive lobby_id=%s", lobbyID)
	}
	if memberCount > csgo5v5TeamSize {
		return fmt.Errorf("csgo 5v5 matchmaker: lobby member count exceeds 5 lobby_id=%s count=%d", lobbyID, memberCount)
	}

	queuedAt := lobby.CreatedAt
	if queuedAt.IsZero() {
		queuedAt = time.Now()
	}

	return m.queueRepo.Enqueue(ctx, LobbyQueueEntry{
		LobbyID:     lobbyID,
		MemberCount: memberCount,
		QueuedAt:    queuedAt,
	})
}

// Cancel 从所有人数桶中移除指定 lobby。
func (m *CSGO5v5Matchmaker) Cancel(ctx context.Context, lobbyID string) error {
	if m == nil || m.queueRepo == nil {
		return fmt.Errorf("csgo 5v5 matchmaker: queue repository is nil")
	}

	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return fmt.Errorf("csgo 5v5 matchmaker: lobby_id is required")
	}

	return m.queueRepo.RemoveQueuedLobby(ctx, lobbyID)
}

// FindMatch 从各人数桶读取最早候选，并组装两支完整的 5 人队伍。
func (m *CSGO5v5Matchmaker) FindMatch(ctx context.Context) (*domainmatchmaking.Match, error) {
	if m == nil || m.queueRepo == nil {
		return nil, fmt.Errorf("csgo 5v5 matchmaker: queue repository is nil")
	}

	candidates := make([]LobbyQueueEntry, 0)
	for memberCount := 1; memberCount <= csgo5v5TeamSize; memberCount++ {
		entries, err := m.queueRepo.ListOldestByMemberCount(ctx, memberCount, csgo5v5CandidateLimitPerBin)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, entries...)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].QueuedAt.Equal(candidates[j].QueuedAt) {
			return candidates[i].LobbyID < candidates[j].LobbyID
		}
		return candidates[i].QueuedAt.Before(candidates[j].QueuedAt)
	})

	teams, ok := buildTeams(candidates, csgo5v5TeamCount, csgo5v5TeamSize)
	if !ok {
		return nil, nil
	}

	matchTeams := make([]domainmatchmaking.Team, 0, len(teams))
	for _, team := range teams {
		lobbyIDs := make([]string, 0, len(team))
		for _, candidateIndex := range team {
			lobbyIDs = append(lobbyIDs, candidates[candidateIndex].LobbyID)
		}
		matchTeams = append(matchTeams, domainmatchmaking.Team{LobbyIDs: lobbyIDs})
	}

	now := time.Now().UTC()
	return &domainmatchmaking.Match{
		GameMode:  config.GameModeCSGO5v5,
		Status:    domainmatchmaking.MatchStatusWaitingForServer,
		Teams:     matchTeams,
		Server:    nil,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// RemoveMatched 从匹配队列中批量移除已组成比赛的 lobby。
func (m *CSGO5v5Matchmaker) RemoveMatched(ctx context.Context, lobbyIDs []string) error {
	if m == nil || m.queueRepo == nil {
		return fmt.Errorf("csgo 5v5 matchmaker: queue repository is nil")
	}
	return m.queueRepo.RemoveQueuedLobbies(ctx, lobbyIDs)
}

// HasQueuedLobbies 判断候选大厅在持锁后是否仍全部处于等待队列。
func (m *CSGO5v5Matchmaker) HasQueuedLobbies(ctx context.Context, lobbyIDs []string) (bool, error) {
	if m == nil || m.queueRepo == nil {
		return false, fmt.Errorf("csgo 5v5 matchmaker: queue repository is nil")
	}
	return m.queueRepo.HasQueuedLobbies(ctx, lobbyIDs)
}

// buildTeams 通过短路回溯从候选中组出指定数量和人数的队伍。
func buildTeams(candidates []LobbyQueueEntry, teamCount, teamSize int) ([][]int, bool) {
	used := make([]bool, len(candidates))
	teams := make([][]int, 0, teamCount)

	var buildNextTeam func() bool
	buildNextTeam = func() bool {
		if len(teams) == teamCount {
			return true
		}

		var findTeam func(start, currentSize int, current []int) bool
		findTeam = func(start, currentSize int, current []int) bool {
			if currentSize == teamSize {
				team := append([]int(nil), current...)
				for _, index := range team {
					used[index] = true
				}
				teams = append(teams, team)
				if buildNextTeam() {
					return true
				}
				teams = teams[:len(teams)-1]
				for _, index := range team {
					used[index] = false
				}
				return false
			}

			for index := start; index < len(candidates); index++ {
				if used[index] {
					continue
				}
				nextSize := currentSize + candidates[index].MemberCount
				if nextSize > teamSize {
					continue
				}
				if findTeam(index+1, nextSize, append(current, index)) {
					return true
				}
			}
			return false
		}

		return findTeam(0, 0, nil)
	}

	if !buildNextTeam() {
		return nil, false
	}
	return teams, true
}
