package publisher

import (
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
	"cloudpvp-matcher/internal/handler/dto"
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var _ domainmatchmaking.Publisher = (*Publisher)(nil)

// PublishTicketQueued 发布 lobby 已进入匹配队列事件。
func (p *Publisher) PublishTicketQueued(ctx context.Context, ticket *domainticket.Ticket) error {
	queuedAt := ticket.UpdatedAt
	if queuedAt.IsZero() {
		queuedAt = time.Now()
	}

	return p.publish(ctx, ticketQueuedRoutingKey, dto.TicketQueued{
		MessageID:   "",
		LobbyID:     ticket.LobbyID,
		GameMode:    string(ticket.GameMode),
		MemberCount: ticket.TeamSize(),
		QueuedAt:    queuedAt,
	})
}

// PublishConfirmRequest 发布玩家确认请求。
func (p *Publisher) PublishConfirmRequest(ctx context.Context, match *domainmatch.Match, timeout time.Duration) error {
	return p.publish(ctx, confirmRequestRoutingKey, dto.ConfirmRequest{
		MessageID:      "",
		MatchID:        match.ID,
		GameMode:       string(match.GameMode),
		Teams:          toTeamInfos(match),
		TimeoutSeconds: int(timeout / time.Second),
		CreatedAt:      match.CreatedAt,
	})
}

// PublishMatchResult 发布无需确认的匹配结果。
func (p *Publisher) PublishMatchResult(ctx context.Context, match *domainmatch.Match) error {
	return p.publish(ctx, matchResultRoutingKey, dto.MatchResult{
		MessageID: "",
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Teams:     toTeamInfos(match),
		MatchedAt: match.CreatedAt,
	})
}

// PublishServerCreateRequest 发布创建对局服务器请求。
func (p *Publisher) PublishServerCreateRequest(ctx context.Context, match *domainmatch.Match) error {
	return p.publish(ctx, serverCreateRoutingKey, dto.ServerCreateRequest{
		MessageID: "",
		MatchID:   match.ID,
		GameMode:  string(match.GameMode),
		Players:   toMemberInfos(match.AllPlayers()),
		CreatedAt: match.CreatedAt,
	})
}

// publish 将出站 DTO 序列化并发布到指定路由键。
func (p *Publisher) publish(ctx context.Context, routingKey string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化发布消息失败: %w", err)
	}
	return p.ch.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
}

// toTeamInfos 将领域队伍转换为出站 DTO 队伍。
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

// toMemberInfos 将领域玩家列表转换为出站 DTO 玩家列表。
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
