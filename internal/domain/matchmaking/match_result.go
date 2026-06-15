package matchmaking

import (
	"time"
)

// MatchResult 是匹配成功后的结果响应。
type MatchResult struct {
	MatchID   string    `json:"match_id"`
	GameMode  string    `json:"game_mode"`
	LobbyIDs  []string  `json:"lobby_ids"`
	MatchedAt time.Time `json:"matched_at"`
}
