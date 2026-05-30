package match

import "context"

// EventPublisher 是将匹配相关事件发布到外部系统的端口。
type EventPublisher interface {
	PublishMatchResult(ctx context.Context, match *Match) error
	PublishServerCreateRequest(ctx context.Context, match *Match) error
	PublishConfirmRequest(ctx context.Context, match *Match) error
}
