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
	ID        string          `json:"id"`
	GameMode  config.GameMode `json:"game_mode"`
	Teams     []Team          `json:"teams"`
	Status    MatchStatus     `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Team 表示一场对局中的一个阵营，可由多个未满员 lobby 拼成。
type Team struct {
	Tickets []*ticket.Ticket `json:"tickets"`
}

// LobbyIDs 返回组成队伍的 lobby ID 列表。
func (t Team) LobbyIDs() []string {
	lobbyIDs := make([]string, 0, len(t.Tickets))
	for _, ticket := range t.Tickets {
		lobbyIDs = append(lobbyIDs, ticket.LobbyID)
	}
	return lobbyIDs
}

// Members 返回组成队伍的玩家列表。
func (t Team) Members() []ticket.PlayerInfo {
	var players []ticket.PlayerInfo
	for _, ticket := range t.Tickets {
		players = append(players, ticket.Members...)
	}
	return players
}

// AllPlayers 返回所有票据中所有玩家的扁平列表。
func (m *Match) AllPlayers() []ticket.PlayerInfo {
	var players []ticket.PlayerInfo
	for _, team := range m.Teams {
		players = append(players, team.Members()...)
	}
	return players
}

// AllTickets 返回参与对局的所有队列票据。
func (m *Match) AllTickets() []*ticket.Ticket {
	var tickets []*ticket.Ticket
	for _, team := range m.Teams {
		tickets = append(tickets, team.Tickets...)
	}
	return tickets
}
