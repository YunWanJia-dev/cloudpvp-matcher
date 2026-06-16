package csgo_5v5

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainlobby "cloudpvp-matcher/internal/domain/lobby"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
)

const csgo5v5MaxLobbyMembers = 5

// CSGO5v5Matchmaker 实现 CS:GO 5v5 竞技匹配的入队和取消流程。
type CSGO5v5Matchmaker struct {
	queueRepo     LobbyQueueRepository
	matchResultCh chan<- *domainmatchmaking.MatchResult
}

var _ domainmatch.Matchmaker = (*CSGO5v5Matchmaker)(nil)

// NewCSGO5v5Matchmaker 创建一个新的 CS:GO 5v5 匹配器。
func NewCSGO5v5Matchmaker(queueRepo LobbyQueueRepository, matchResultCh chan<- *domainmatchmaking.MatchResult) *CSGO5v5Matchmaker {
	return &CSGO5v5Matchmaker{
		queueRepo:     queueRepo,
		matchResultCh: matchResultCh,
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
	if memberCount > csgo5v5MaxLobbyMembers {
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
