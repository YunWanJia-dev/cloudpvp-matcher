// Package handler 处理入站 RabbitMQ 消息，将其转换为用例层的调用。
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	appsvc "cloudpvp-matcher/internal/application/service"
	"cloudpvp-matcher/internal/domain/entity"
	"cloudpvp-matcher/internal/domain/valueobject"
	redisclient "cloudpvp-matcher/internal/infra/redis"
	"cloudpvp-matcher/internal/interface/dto"
)

const dedupKeyPrefix = "matcher:dedup:"

// MatchHandler 处理来自 cloudpvp-biz 的匹配请求消息。
type MatchHandler struct {
	usecase     *appsvc.MatchmakingUseCase
	redisClient *redisclient.Client
}

// NewMatchHandler 创建一个新的匹配请求处理器。
func NewMatchHandler(usecase *appsvc.MatchmakingUseCase, redisClient *redisclient.Client) *MatchHandler {
	return &MatchHandler{usecase: usecase, redisClient: redisClient}
}

// Handle 反序列化匹配请求、幂等检查、转换为领域实体并委托给用例层。
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

	// 幂等检查：防止重复消费同一消息
	if req.MessageID != "" {
		dedupKey := dedupKeyPrefix + req.MessageID
		exists, err := h.redisClient.Get(ctx, dedupKey)
		if err == nil && exists != "" {
			slog.Warn("重复消息，已跳过", "message_id", req.MessageID)
			return nil
		}
		// 标记消息已处理，TTL 1小时兜底
		_ = h.redisClient.Set(ctx, dedupKey, "1", time.Hour)
	}

	// DTO → 领域实体转换
	members := make([]entity.PlayerInfo, 0, len(req.Members))
	for _, m := range req.Members {
		members = append(members, entity.PlayerInfo{
			PlayerID: m.PlayerID,
			Name:     m.Name,
			Region:   m.Region,
		})
	}

	ticketID := fmt.Sprintf("ticket-%s-%d", req.LobbyID, time.Now().UnixMilli())
	ticket := appsvc.ToDomainTicket(
		ticketID,
		req.LobbyID,
		valueobject.GameMode(req.GameMode),
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
