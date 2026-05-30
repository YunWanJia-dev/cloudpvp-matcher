// Package handler 处理入站 RabbitMQ 消息，将其转换为用例层的调用。
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"
	"cloudpvp-matcher/internal/handler/dto"
	"cloudpvp-matcher/internal/usecase/matchmaking"
)

// MatchHandler 处理来自 cloudpvp-biz 的匹配请求消息。
type MatchHandler struct {
	usecase *matchmaking.UseCase
}

// NewMatchHandler 创建一个新的匹配请求处理器。
func NewMatchHandler(usecase *matchmaking.UseCase) *MatchHandler {
	return &MatchHandler{usecase: usecase}
}

// Handle 反序列化匹配请求、转换为领域实体并委托给用例层。
func (h *MatchHandler) Handle(ctx context.Context, body []byte) error {
	var req dto.MatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return fmt.Errorf("match handler: 反序列化请求失败: %w", err)
	}

	// 基本字段校验
	if req.LobbyID == "" {
		return fmt.Errorf("match handler: lobby_id 不能为空")
	}
	if req.GameMode == "" {
		return fmt.Errorf("match handler: game_mode 不能为空")
	}
	if len(req.Members) == 0 {
		return fmt.Errorf("match handler: members 不能为空")
	}

	// DTO → 领域实体转换
	members := make([]ticket.PlayerInfo, 0, len(req.Members))
	for _, m := range req.Members {
		members = append(members, ticket.PlayerInfo{
			PlayerID: m.PlayerID,
			Name:     m.Name,
			Region:   m.Region,
		})
	}

	ticketID := fmt.Sprintf("ticket-%s-%d", req.LobbyID, time.Now().UnixMilli())
	ticket := toDomainTicket(
		ticketID,
		req.LobbyID,
		config.GameMode(req.GameMode),
		members,
	)

	slog.Info("收到匹配请求",
		"message_id", req.MessageID,
		"lobby_id", req.LobbyID,
		"game_mode", req.GameMode,
		"member_count", len(members),
	)

	return h.usecase.EnqueueAndMatch(ctx, ticket)
}

func toDomainTicket(ticketID, lobbyID string, gameMode config.GameMode, members []ticket.PlayerInfo) *ticket.Ticket {
	now := time.Now()
	return &ticket.Ticket{
		ID:        ticketID,
		LobbyID:   lobbyID,
		GameMode:  gameMode,
		Members:   members,
		Status:    ticket.TicketStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
