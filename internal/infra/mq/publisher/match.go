package publisher

import (
	"context"

	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	"cloudpvp-matcher/internal/infra/mq"
)

var _ domainmatchmaking.MatchPublisher = (*MatchPublisher)(nil)

const matchRoutingKey = mq.MatchCreateRoutingKey

// MatchPublisher 负责发布完整比赛快照。
type MatchPublisher struct {
	publisher *Publisher
}

// NewMatchPublisher 创建完整比赛发布器。
func NewMatchPublisher(publisher *Publisher) *MatchPublisher {
	return &MatchPublisher{publisher: publisher}
}

// PublishMatch 发布等待服务器分配的完整比赛快照。
func (p *MatchPublisher) PublishMatch(ctx context.Context, match *domainmatchmaking.Match) error {
	return p.publisher.publish(ctx, matchRoutingKey, match)
}
