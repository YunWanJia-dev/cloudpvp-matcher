package repository

import (
	"context"

	"cloudpvp-matcher/internal/domain/entity"
)

// EventPublisher 是将匹配相关事件发布到外部系统的端口。
type EventPublisher interface {
	PublishMatchResult(ctx context.Context, match *entity.Match) error
	PublishServerCreateRequest(ctx context.Context, match *entity.Match) error
	PublishConfirmRequest(ctx context.Context, match *entity.Match) error
}
