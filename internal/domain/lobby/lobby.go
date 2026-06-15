// Package lobby 定义匹配请求中的原始 lobby 模型。
package lobby

import (
	"time"

	"cloudpvp-matcher/internal/domain/config"
)

// PlayerInfo 描述 lobby 中的一个玩家。
type PlayerInfo struct {
	PlayerID string `json:"player_id"`
}

// Lobby 表示业务侧提交给匹配服务的原始 lobby。
type Lobby struct {
	LobbyID   string          `json:"lobby_id"`
	GameMode  config.GameMode `json:"game_mode"`
	Members   []PlayerInfo    `json:"members"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// TeamSize 返回 lobby 当前成员数量。
func (l *Lobby) TeamSize() int {
	if l == nil {
		return 0
	}
	return len(l.Members)
}
