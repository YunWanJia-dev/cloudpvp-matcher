package dto

import "time"

// ConfirmRequest 是发送给 cloudpvp-biz 的玩家确认请求消息体。
type ConfirmRequest struct {
	MessageID      string     `json:"message_id"`
	MatchID        string     `json:"match_id"`
	GameMode       string     `json:"game_mode"`
	Teams          []TeamInfo `json:"teams"`
	TimeoutSeconds int        `json:"timeout_seconds"`
	CreatedAt      time.Time  `json:"created_at"`
}
