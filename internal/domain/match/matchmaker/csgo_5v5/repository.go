package csgo_5v5

import (
	"context"
	"time"
)

// LobbyQueueEntry 描述一个只保存 lobbyID 的匹配队列条目。
type LobbyQueueEntry struct {
	LobbyID     string
	MemberCount int
	QueuedAt    time.Time
}

// LobbyQueueRepository 是按 lobby 人数分桶的匹配队列仓储端口。
type LobbyQueueRepository interface {
	// Enqueue 将 lobbyID 写入对应人数桶，排序分值由入队时间决定。
	Enqueue(ctx context.Context, entry LobbyQueueEntry) error

	// RemoveQueuedLobby 从所有人数桶中移除指定 lobbyID。
	RemoveQueuedLobby(ctx context.Context, lobbyID string) error
}
