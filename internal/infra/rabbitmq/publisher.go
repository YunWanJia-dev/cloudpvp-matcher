// Package rabbitmq 实现领域层的事件发布端口。
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"cloudpvp-matcher/internal/domain/entity"
	"cloudpvp-matcher/internal/domain/repository"
	"cloudpvp-matcher/internal/interface/dto"
)

const (
	RoutingKeyMatchResult    = "match.result"
	RoutingKeyServerCreate   = "server.create"
	RoutingKeyConfirmRequest = "match.confirm.request"
)

// MqPublisher 通过 RabbitMQ 实现 domain/repository/EventPublisher 端口。
type MqPublisher struct {
	conn *Connection
}

// 编译期检查接口实现
var _ repository.EventPublisher = (*MqPublisher)(nil)

// NewPublisher 创建一个新的 RabbitMQ 事件发布者。
func NewPublisher(conn *Connection) *MqPublisher {
	return &MqPublisher{conn: conn}
}

// PublishMatchResult 发布匹配结果到 cloudpvp-biz。
func (p *MqPublisher) PublishMatchResult(ctx context.Context, match *entity.Match) error {
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
		MessageID: "", // TODO: 由上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     teams,
		MatchedAt: match.CreatedAt,
	}

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("publish match result: marshal: %w", err)
	}

	return p.conn.Publish(RoutingKeyMatchResult, body)
}

// PublishServerCreateRequest 发布创建服务器请求。
func (p *MqPublisher) PublishServerCreateRequest(ctx context.Context, match *entity.Match) error {
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
		MessageID: "", // TODO: 由上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Players:   players,
		CreatedAt: match.CreatedAt,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("publish server create request: marshal: %w", err)
	}

	return p.conn.Publish(RoutingKeyServerCreate, body)
}

// PublishConfirmRequest 发布玩家确认请求。
func (p *MqPublisher) PublishConfirmRequest(ctx context.Context, match *entity.Match) error {
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
		MessageID: "", // TODO: 由上层注入 message_id
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     teams,
		CreatedAt: match.CreatedAt,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("publish confirm request: marshal: %w", err)
	}

	return p.conn.Publish(RoutingKeyConfirmRequest, body)
}
