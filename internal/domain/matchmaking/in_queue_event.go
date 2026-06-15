package matchmaking

// InQueueEvent 表示 lobby 已进入匹配队列。
type InQueueEvent struct {
	LobbyID string `json:"lobby_id"`
}

// NewInQueueEvent 创建 lobby 入队事件。
func NewInQueueEvent(lobbyID string) InQueueEvent {
	return InQueueEvent{LobbyID: lobbyID}
}
