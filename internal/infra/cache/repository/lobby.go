package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domainlobby "cloudpvp-matcher/internal/domain/lobby"

	goredis "github.com/redis/go-redis/v9"
)

const (
	originalLobbyKeyPrefix = "matcher:lobby:original:"
	originalLobbyTTL       = 30 * time.Minute
)

var _ domainlobby.Repository = (*RedisRepository)(nil)

// Save 保存原始 lobby 快照。
func (r *RedisRepository) Save(ctx context.Context, lobby *domainlobby.Lobby) error {
	if lobby == nil {
		return fmt.Errorf("redis lobby repo: lobby is nil")
	}
	lobbyID := strings.TrimSpace(lobby.LobbyID)
	if lobbyID == "" {
		return fmt.Errorf("redis lobby repo: lobby_id is required")
	}

	data, err := json.Marshal(lobby)
	if err != nil {
		return fmt.Errorf("redis lobby repo: marshal lobby: %w", err)
	}
	if err := r.client.Set(ctx, originalLobbyKey(lobbyID), data, originalLobbyTTL).Err(); err != nil {
		return fmt.Errorf("redis lobby repo: save lobby: %w", err)
	}
	return nil
}

// FindByLobbyID 按 lobby ID 查询原始 lobby 快照。
func (r *RedisRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*domainlobby.Lobby, error) {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return nil, nil
	}

	data, err := r.client.Get(ctx, originalLobbyKey(lobbyID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis lobby repo: find lobby: %w", err)
	}

	var lobby domainlobby.Lobby
	if err := json.Unmarshal(data, &lobby); err != nil {
		return nil, fmt.Errorf("redis lobby repo: unmarshal lobby: %w", err)
	}
	return &lobby, nil
}

// Remove 删除指定 lobby 的原始快照。
func (r *RedisRepository) Remove(ctx context.Context, lobbyID string) error {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return nil
	}
	if err := r.client.Del(ctx, originalLobbyKey(lobbyID)).Err(); err != nil {
		return fmt.Errorf("redis lobby repo: remove lobby: %w", err)
	}
	return nil
}

// RemoveMany 批量删除已完成匹配的原始 lobby 快照。
func (r *RedisRepository) RemoveMany(ctx context.Context, lobbyIDs []string) error {
	if len(lobbyIDs) == 0 {
		return nil
	}

	keys := make([]string, 0, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		lobbyID = strings.TrimSpace(lobbyID)
		if lobbyID == "" {
			continue
		}
		keys = append(keys, originalLobbyKey(lobbyID))
	}
	if len(keys) == 0 {
		return nil
	}

	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis lobby repo: remove lobbies: %w", err)
	}
	return nil
}

// originalLobbyKey 生成原始 lobby 快照的 Redis key。
func originalLobbyKey(lobbyID string) string {
	return originalLobbyKeyPrefix + lobbyID
}
