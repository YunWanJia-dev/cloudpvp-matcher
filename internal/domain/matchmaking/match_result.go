package matchmaking

import (
	"time"

	"cloudpvp-matcher/internal/domain/config"
)

// MatchStatus 表示完整比赛在服务器分配流程中的状态。
type MatchStatus string

const (
	// MatchStatusWaitingForServer 表示 Matcher 已完成组队，等待服务器分配。
	MatchStatusWaitingForServer MatchStatus = "WAITING_FOR_SERVER"
	// MatchStatusInProgress 表示 Allocator 已补充服务器信息，可以开始比赛。
	MatchStatusInProgress MatchStatus = "IN_PROGRESS"
)

// Match 是 Matcher 与 Allocator 之间传递的完整比赛快照。
// Allocator 必须在 match.update 中保留 Matcher 生成的队伍与时间字段。
type Match struct {
	MatchID   string          `json:"match_id"`
	GameMode  config.GameMode `json:"game_mode"`
	Status    MatchStatus     `json:"status"`
	Teams     []Team          `json:"teams"`
	Server    *Server         `json:"server"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Team 描述一支由一个或多个 lobby 拼成的队伍。
type Team struct {
	LobbyIDs []string `json:"lobby_ids"`
	Members  []Member `json:"members"`
}

// Member 描述比赛中的一个玩家。
type Member struct {
	PlayerID string `json:"player_id"`
}

// Server 描述 Allocator 为比赛分配的服务器。
type Server struct {
	IP string `json:"ip"`
}

// LobbyIDs 按队伍顺序返回比赛涉及的全部 lobby ID，并去除重复值。
func (m *Match) LobbyIDs() []string {
	if m == nil {
		return nil
	}

	seen := make(map[string]struct{})
	lobbyIDs := make([]string, 0)
	for _, team := range m.Teams {
		for _, lobbyID := range team.LobbyIDs {
			if _, exists := seen[lobbyID]; exists {
				continue
			}
			seen[lobbyID] = struct{}{}
			lobbyIDs = append(lobbyIDs, lobbyID)
		}
	}
	return lobbyIDs
}
