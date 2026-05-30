package ticket

import (
	"time"

	"cloudpvp-matcher/internal/domain/config"
)

// Ticket 表示一个匹配票据 —— 一个正在寻找对局的 lobby/队伍。
type Ticket struct {
	ID        string          `json:"id"`
	LobbyID   string          `json:"lobby_id"`
	GameMode  config.GameMode `json:"game_mode"`
	Members   []PlayerInfo    `json:"members"`
	Status    TicketStatus    `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// IsActive 如果票据仍可参与匹配（非终止态），返回 true。
func (t *Ticket) IsActive() bool {
	switch t.Status {
	case TicketStatusCancelled, TicketStatusTimedOut, TicketStatusConfirmed:
		return false
	default:
		return true
	}
}

// TeamSize 返回票据中的玩家数量。
func (t *Ticket) TeamSize() int {
	return len(t.Members)
}

// IsFull 校验票据是否已达到其模式要求的队伍人数。
func (t *Ticket) IsFull(cfg *config.MatchConfig) bool {
	return len(t.Members) == cfg.TeamSize
}
