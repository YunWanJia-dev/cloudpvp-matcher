// Package matchmaking 定义匹配流程相关的领域端口。
package matchmaking

import (
	"context"
)

// Publisher 定义匹配流程需要的强类型出站发布端口。
type Publisher interface {
	// PublishLobbyStatus 发布单个大厅的状态更新。
	PublishLobbyStatus(ctx context.Context, lobbyID string, status LobbyStatus, reason string) error

	// PublishMatch 发布 Matcher 生成的完整比赛快照。
	PublishMatch(ctx context.Context, match *Match) error
}
