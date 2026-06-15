package publisher

import (
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	"context"
)

var _ domainmatchmaking.Publisher = (*Publisher)(nil)

const (
	inQueueRoutingKey        = "matchmaking.in_queue"
	kickedQueueRoutingKey    = "matchmaking.kicked_queue"
	matchResultRoutingKey    = "match.result"
	confirmRequestRoutingKey = "match.confirm.request"
)

// PublishInQueue 发布 lobby 已进入匹配队列事件。
func (p *Publisher) PublishInQueue(ctx context.Context, lobbyID string) error {
	return p.publish(ctx, inQueueRoutingKey, domainmatchmaking.NewInQueueEvent(lobbyID))
}

// PublishKickedQueue 发布 lobby 被移出匹配队列事件。
func (p *Publisher) PublishKickedQueue(ctx context.Context, lobbyID, reason string) error {
	return p.publish(ctx, kickedQueueRoutingKey, domainmatchmaking.NewKickedQueueEvent(lobbyID, reason))
}

// PublishConfirmRequest 发布玩家确认请求。
func (p *Publisher) PublishConfirmRequest(ctx context.Context, lobbyIDs []string) error {
	return p.publish(ctx, confirmRequestRoutingKey, domainmatchmaking.NewConfirmRequest(lobbyIDs))
}

// PublishMatchResult 发布匹配结果。
func (p *Publisher) PublishMatchResult(ctx context.Context, match *domainmatchmaking.MatchResult) error {
	return p.publish(ctx, matchResultRoutingKey, match)
}
