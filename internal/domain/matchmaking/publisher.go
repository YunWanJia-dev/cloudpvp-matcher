// Package matchmaking 定义匹配流程相关的领域端口。
package matchmaking

import (
	"context"
)

// LobbyPublisher 定义大厅状态更新的出站发布端口。
type LobbyPublisher interface {
	// PublishLobbyStatus 发布单个大厅的状态更新。
	PublishLobbyStatus(ctx context.Context, lobbyID string, status LobbyStatus, matchID string, reason string) error
}

// MatchPublisher 定义完整比赛快照的出站发布端口。
type MatchPublisher interface {
	// PublishMatch 发布 Matcher 生成的完整比赛快照。
	PublishMatch(ctx context.Context, match *Match) error
}
