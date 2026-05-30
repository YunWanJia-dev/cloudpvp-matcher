// Package entity 定义匹配领域中的核心业务实体。
package entity

// PlayerInfo 描述匹配票据中的一个玩家。
type PlayerInfo struct {
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Region   string `json:"region,omitempty"`
}
