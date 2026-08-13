package publisher

import (
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	"cloudpvp-matcher/internal/infra/mq"
	"context"
)

var _ domainmatchmaking.Publisher = (*Publisher)(nil)

const (
	lobbyRoutingKey = mq.LobbyRoutingKey
	matchRoutingKey = mq.MatchCreateRoutingKey
)

// PublishLobbyStatus 发布 lobby 状态更新事件。
func (p *Publisher) PublishLobbyStatus(ctx context.Context, lobbyID string, status domainmatchmaking.LobbyStatus, reason string) error {
	return p.publish(ctx, lobbyRoutingKey, domainmatchmaking.NewLobbyEvent(lobbyID, status, reason))
}

// PublishMatch 发布等待服务器分配的完整比赛快照。
func (p *Publisher) PublishMatch(ctx context.Context, match *domainmatchmaking.Match) error {
	return p.publish(ctx, matchRoutingKey, match)
}
