package dto

import "time"

// TicketQueued 是发送给 cloudpvp-biz 的 lobby 入队成功消息体。
type TicketQueued struct {
	MessageID   string    `json:"message_id"`
	LobbyID     string    `json:"lobby_id"`
	GameMode    string    `json:"game_mode"`
	MemberCount int       `json:"member_count"`
	QueuedAt    time.Time `json:"queued_at"`
}
