// Package dto 定义入站和出站消息的 DTO。
package dto

import "time"

// MatchRequest 是 cloudpvp-biz 发来的匹配请求消息体。
type MatchRequest struct {
	MessageID string       `json:"message_id"`
	LobbyID   string       `json:"lobby_id"`
	GameMode  string       `json:"game_mode"`
	Members   []MemberInfo `json:"members"`
	CreatedAt time.Time    `json:"created_at"`
}

// MemberInfo 描述请求中的单个玩家。
type MemberInfo struct {
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Region   string `json:"region,omitempty"`
}
