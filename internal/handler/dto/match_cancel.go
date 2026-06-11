package dto

import "time"

// MatchCancelRequest 是 cloudpvp-biz 发来的取消匹配请求消息体。
type MatchCancelRequest struct {
	MessageID string    `json:"message_id"`
	LobbyID   string    `json:"lobby_id"`
	CreatedAt time.Time `json:"created_at"`
}
