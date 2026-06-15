package matchmaking

// ConfirmRequest 是请求玩家确认的 lobby ID 集合。
type ConfirmRequest struct {
	LobbyIDs []string `json:"lobby_ids"`
}

// NewConfirmRequest 创建确认请求消息。
func NewConfirmRequest(lobbyIDs []string) ConfirmRequest {
	return ConfirmRequest{LobbyIDs: append([]string(nil), lobbyIDs...)}
}
