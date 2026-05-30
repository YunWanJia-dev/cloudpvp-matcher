// Package ticket 包含匹配票据相关的领域类型。
package ticket

// PlayerInfo 描述匹配票据中的一个玩家。
type PlayerInfo struct {
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Region   string `json:"region,omitempty"`
}
