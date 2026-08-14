// Package lobby 定义匹配请求中的原始 lobby 模型。
package lobby

import (
	"time"

	"cloudpvp-matcher/internal/domain/config"
)

// Lobby 表示业务侧提交给匹配服务的原始 lobby。
type Lobby struct {
	LobbyID     string          `json:"lobby_id"`
	GameMode    config.GameMode `json:"game_mode"`
	PlayerCount int             `json:"player_count"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
