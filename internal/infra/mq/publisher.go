package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
	"cloudpvp-matcher/internal/handler/dto"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher 通过 RabbitMQ 实现 domain/match.EventPublisher 端口。
type Publisher struct {
	rabbitMQ *RabbitMQ
}

// 编译期检查接口实现
var _ domainmatch.EventPublisher = (*Publisher)(nil)

// NewPublisher 创建一个新的 RabbitMQ 事件发布者。
func NewPublisher(rabbitMQ *RabbitMQ) *Publisher {
	return &Publisher{rabbitMQ: rabbitMQ}
}

// PublishMatchResult 发布匹配结果到 cloudpvp-biz。
func (p *Publisher) PublishMatchResult(ctx context.Context, match *domainmatch.Match) error {
	result := dto.MatchResult{
		MessageID: "", // TODO: 待上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     toTeamInfos(match),
		MatchedAt: match.CreatedAt,
	}

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("publish match result: marshal: %w", err)
	}

	return p.publish(ResultRoutingKey, body)
}

// PublishServerCreateRequest 发布创建服务器请求。
func (p *Publisher) PublishServerCreateRequest(ctx context.Context, match *domainmatch.Match) error {
	req := dto.ServerCreateRequest{
		MessageID: "", // TODO: 待上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Players:   toMemberInfos(match.AllPlayers()),
		CreatedAt: match.CreatedAt,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("publish server create request: marshal: %w", err)
	}

	return p.publish(ServerCreateRoutingKey, body)
}

// PublishConfirmRequest 发布玩家确认请求。
func (p *Publisher) PublishConfirmRequest(ctx context.Context, match *domainmatch.Match) error {
	req := dto.ConfirmRequest{
		MessageID: "", // TODO: 待上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     toTeamInfos(match),
		CreatedAt: match.CreatedAt,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("publish confirm request: marshal: %w", err)
	}

	return p.publish(ConfirmRequestRoutingKey, body)
}

// PublishTicketQueued 发布 lobby 入队成功事件。
func (p *Publisher) PublishTicketQueued(ctx context.Context, ticket *domainticket.Ticket) error {
	queuedAt := ticket.UpdatedAt
	if queuedAt.IsZero() {
		queuedAt = time.Now()
	}

	event := dto.TicketQueued{
		MessageID:   "", // TODO: 待上层注入 message_id
		LobbyID:     ticket.LobbyID,
		GameMode:    string(ticket.GameMode),
		MemberCount: ticket.TeamSize(),
		QueuedAt:    queuedAt,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("publish ticket queued: marshal: %w", err)
	}

	return p.publish(QueuedRoutingKey, body)
}

func (p *Publisher) publish(routingKey string, body []byte) error {
	return p.rabbitMQ.Channel().Publish(
		p.rabbitMQ.ExchangeName(),
		routingKey,
		false, // 不强制要求可路由
		false, // 不要求立即投递
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
}

func toTeamInfos(match *domainmatch.Match) []dto.TeamInfo {
	teams := make([]dto.TeamInfo, 0, len(match.Teams))
	for _, team := range match.Teams {
		lobbyIDs := team.LobbyIDs()
		teamInfo := dto.TeamInfo{
			LobbyIDs: lobbyIDs,
			Members:  toMemberInfos(team.Members()),
		}
		if len(lobbyIDs) == 1 {
			teamInfo.LobbyID = lobbyIDs[0]
		}
		teams = append(teams, teamInfo)
	}
	return teams
}

func toMemberInfos(players []domainticket.PlayerInfo) []dto.MemberInfo {
	members := make([]dto.MemberInfo, 0, len(players))
	for _, player := range players {
		members = append(members, dto.MemberInfo{
			PlayerID: player.PlayerID,
			Name:     player.Name,
			Region:   player.Region,
		})
	}
	return members
}
