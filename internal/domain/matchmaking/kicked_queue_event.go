package matchmaking

// KickedQueueEvent 表示 lobby 被移出匹配队列。
type KickedQueueEvent struct {
	LobbyID string `json:"lobby_id"`
	Reason  string `json:"reason,omitempty"`
}

// NewKickedQueueEvent 创建 lobby 踢出队列事件。
func NewKickedQueueEvent(lobbyID, reason string) KickedQueueEvent {
	return KickedQueueEvent{LobbyID: lobbyID, Reason: reason}
}
