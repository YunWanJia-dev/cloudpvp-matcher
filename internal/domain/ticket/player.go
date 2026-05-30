// Package ticket contains matchmaking ticket domain types.
package ticket

// PlayerInfo 描述匹配票据中的一个玩家。
type PlayerInfo struct {
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Region   string `json:"region,omitempty"`
}
