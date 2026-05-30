package match

import (
	"time"

	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"
)

// MatchStatus 表示已形成匹配的状态。
type MatchStatus int

const (
	MatchStatusPending   MatchStatus = iota // 匹配已创建，等待确认或服务器分配
	MatchStatusConfirmed                    // 所有玩家已确认
	MatchStatusActive                       // 服务器已创建，游戏进行中
	MatchStatusCompleted                    // 游戏结束
	MatchStatusCancelled                    // 匹配在游戏开始前或过程中被取消
)

// Match 表示两个或多个票据之间的已形成的匹配对局。
type Match struct {
	ID        string           `json:"id"`
	GameMode  config.GameMode  `json:"game_mode"`
	Tickets   []*ticket.Ticket `json:"tickets"`
	Status    MatchStatus      `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// AllPlayers 返回所有票据中所有玩家的扁平列表。
func (m *Match) AllPlayers() []ticket.PlayerInfo {
	var players []ticket.PlayerInfo
	for _, t := range m.Tickets {
		players = append(players, t.Members...)
	}
	return players
}
