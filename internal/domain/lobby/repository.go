package lobby

import "context"

// Repository 是原始 lobby 快照的持久化端口。
type Repository interface {
	// Save 保存原始 lobby 快照。
	Save(ctx context.Context, lobby *Lobby) error

	// FindByLobbyID 按 lobby ID 查询原始 lobby 快照。
	FindByLobbyID(ctx context.Context, lobbyID string) (*Lobby, error)

	// Remove 删除指定 lobby 的原始快照。
	Remove(ctx context.Context, lobbyID string) error

	// RemoveMany 批量删除已完成匹配的原始 lobby 快照。
	RemoveMany(ctx context.Context, lobbyIDs []string) error
}
