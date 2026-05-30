package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domainmatch "cloudpvp-matcher/internal/domain/match"
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
	teams := make([]dto.TeamInfo, 0, len(match.Tickets))
	for _, t := range match.Tickets {
		members := make([]dto.MemberInfo, 0, len(t.Members))
		for _, m := range t.Members {
			members = append(members, dto.MemberInfo{
				PlayerID: m.PlayerID,
				Name:     m.Name,
				Region:   m.Region,
			})
		}
		teams = append(teams, dto.TeamInfo{
			LobbyID: t.LobbyID,
			Members: members,
		})
	}

	result := dto.MatchResult{
		MessageID: "", // TODO: 待上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     teams,
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
	players := make([]dto.MemberInfo, 0)
	for _, t := range match.Tickets {
		for _, m := range t.Members {
			players = append(players, dto.MemberInfo{
				PlayerID: m.PlayerID,
				Name:     m.Name,
				Region:   m.Region,
			})
		}
	}

	req := dto.ServerCreateRequest{
		MessageID: "", // TODO: 待上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Players:   players,
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
	teams := make([]dto.TeamInfo, 0, len(match.Tickets))
	for _, t := range match.Tickets {
		members := make([]dto.MemberInfo, 0, len(t.Members))
		for _, m := range t.Members {
			members = append(members, dto.MemberInfo{
				PlayerID: m.PlayerID,
				Name:     m.Name,
				Region:   m.Region,
			})
		}
		teams = append(teams, dto.TeamInfo{
			LobbyID: t.LobbyID,
			Members: members,
		})
	}

	req := dto.ConfirmRequest{
		MessageID: "", // TODO: 待上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     teams,
		CreatedAt: match.CreatedAt,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("publish confirm request: marshal: %w", err)
	}

	return p.publish(ConfirmRequestRoutingKey, body)
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
