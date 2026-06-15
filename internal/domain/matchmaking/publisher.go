// Package matchmaking 定义匹配流程相关的领域端口。
package matchmaking

import (
	"context"
)

// Publisher 定义匹配流程需要的强类型出站发布端口。
type Publisher interface {
	// PublishInQueue 广播对应 lobby ID 已在匹配队列中。
	PublishInQueue(ctx context.Context, lobbyID string) error

	// PublishKickedQueue 广播对应 lobby ID 被移出匹配队列及原因。
	PublishKickedQueue(ctx context.Context, lobbyID, reason string) error

	// PublishConfirmRequest 请求对应 lobby ID 集合确认比赛。
	PublishConfirmRequest(ctx context.Context, lobbyIDs []string) error

	// PublishMatchResult 广播匹配结果。
	PublishMatchResult(ctx context.Context, match *MatchResult) error
}
