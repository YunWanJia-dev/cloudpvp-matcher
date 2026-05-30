package dto

import "time"

// ServerCreateRequest 是向服务器管理服务发出的创建服务器请求消息体。
type ServerCreateRequest struct {
	MessageID string       `json:"message_id"`
	MatchID   string       `json:"match_id"`
	GameMode  string       `json:"game_mode"`
	Players   []MemberInfo `json:"players"`
	CreatedAt time.Time    `json:"created_at"`
}
