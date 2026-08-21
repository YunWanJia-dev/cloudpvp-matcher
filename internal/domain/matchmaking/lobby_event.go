package matchmaking

// LobbyStatus 表示 matcher 回传给业务大厅的状态。
type LobbyStatus string

const (
	// LobbyStatusMatching 表示大厅已进入匹配队列。
	LobbyStatusMatching LobbyStatus = "MATCHING"
	// LobbyStatusWaiting 表示大厅已退出匹配并恢复等待状态。
	LobbyStatusWaiting LobbyStatus = "WAITING"
)

// LobbyEvent 表示单个大厅的状态更新。
type LobbyEvent struct {
	LobbyID string      `json:"lobby_id"`
	Status  LobbyStatus `json:"status"`
	MatchID string      `json:"match_id,omitempty"`
	Reason  string      `json:"reason,omitempty"`
}

// NewLobbyEvent 创建大厅状态更新消息。
func NewLobbyEvent(lobbyID string, status LobbyStatus, matchID string, reason string) LobbyEvent {
	return LobbyEvent{LobbyID: lobbyID, Status: status, MatchID: matchID, Reason: reason}
}
