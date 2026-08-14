package publisher

import (
	"context"

	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	"cloudpvp-matcher/internal/infra/mq"
)

var _ domainmatchmaking.LobbyPublisher = (*LobbyPublisher)(nil)

const lobbyRoutingKey = mq.LobbyRoutingKey

// LobbyPublisher 负责发布大厅状态更新消息。
type LobbyPublisher struct {
	publisher *Publisher
}

// NewLobbyPublisher 创建大厅状态发布器。
func NewLobbyPublisher(publisher *Publisher) *LobbyPublisher {
	return &LobbyPublisher{publisher: publisher}
}

// PublishLobbyStatus 发布 lobby 状态更新事件。
func (p *LobbyPublisher) PublishLobbyStatus(ctx context.Context, lobbyID string, status domainmatchmaking.LobbyStatus, reason string) error {
	return p.publisher.publish(ctx, lobbyRoutingKey, domainmatchmaking.NewLobbyEvent(lobbyID, status, reason))
}
