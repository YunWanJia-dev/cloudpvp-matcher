package ticket

import (
	"cloudpvp-matcher/internal/domain/config"
	"time"
)

// Ticket 表示一个已进入匹配队列的 lobby/队伍。
type Ticket struct {
	LobbyID   string          `json:"lobby_id"`
	GameMode  config.GameMode `json:"game_mode"`
	Members   []PlayerInfo    `json:"members"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// TeamSize 返回票据中的玩家数量。
func (t *Ticket) TeamSize() int {
	return len(t.Members)
}
