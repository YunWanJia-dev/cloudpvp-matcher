package repository

import (
	domainmatchmaker "cloudpvp-matcher/internal/domain/match/matchmaker/csgo_5v5"
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	lobbyQueueKeyPrefix       = "matcher:lobby_queue:members:"
	maxLobbyQueueMemberBucket = 5
)

var _ domainmatchmaker.LobbyQueueRepository = (*RedisRepository)(nil)

// NewRedisMatchmakerQueueRepository 创建一个新的 Redis 匹配队列仓储。
func NewRedisMatchmakerQueueRepository(client *goredis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}

// Enqueue 按 lobby 人数将 lobbyID 写入对应 sorted set。
func (r *RedisRepository) Enqueue(ctx context.Context, entry domainmatchmaker.LobbyQueueEntry) error {
	lobbyID := strings.TrimSpace(entry.LobbyID)
	if lobbyID == "" {
		return fmt.Errorf("redis matchmaker queue repo: lobby_id is required")
	}
	if entry.MemberCount <= 0 || entry.MemberCount > maxLobbyQueueMemberBucket {
		return fmt.Errorf("redis matchmaker queue repo: unsupported member_count=%d lobby_id=%s", entry.MemberCount, lobbyID)
	}

	queuedAt := entry.QueuedAt
	if queuedAt.IsZero() {
		queuedAt = time.Now()
	}

	pipe := r.client.TxPipeline()
	for memberCount := 1; memberCount <= maxLobbyQueueMemberBucket; memberCount++ {
		pipe.ZRem(ctx, lobbyQueueKey(memberCount), lobbyID)
	}
	pipe.ZAdd(ctx, lobbyQueueKey(entry.MemberCount), goredis.Z{
		Score:  float64(queuedAt.UnixMicro()),
		Member: lobbyID,
	})

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis matchmaker queue repo: enqueue lobby: %w", err)
	}
	return nil
}

// ListOldestByMemberCount 返回指定人数桶中最早入队的候选。
func (r *RedisRepository) ListOldestByMemberCount(ctx context.Context, memberCount, limit int) ([]domainmatchmaker.LobbyQueueEntry, error) {
	if memberCount <= 0 || memberCount > maxLobbyQueueMemberBucket {
		return nil, fmt.Errorf("redis matchmaker queue repo: unsupported member_count=%d", memberCount)
	}
	if limit <= 0 {
		return nil, nil
	}

	values, err := r.client.ZRangeWithScores(ctx, lobbyQueueKey(memberCount), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis matchmaker queue repo: list oldest lobbies: %w", err)
	}

	entries := make([]domainmatchmaker.LobbyQueueEntry, 0, len(values))
	for _, value := range values {
		lobbyID, ok := value.Member.(string)
		if !ok || strings.TrimSpace(lobbyID) == "" {
			continue
		}
		queuedMicros := int64(math.Round(value.Score))
		entries = append(entries, domainmatchmaker.LobbyQueueEntry{
			LobbyID:     lobbyID,
			MemberCount: memberCount,
			QueuedAt:    time.UnixMicro(queuedMicros),
		})
	}
	return entries, nil
}

// RemoveQueuedLobby 从所有人数桶中删除 lobbyID。
func (r *RedisRepository) RemoveQueuedLobby(ctx context.Context, lobbyID string) error {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return nil
	}
	return r.RemoveQueuedLobbies(ctx, []string{lobbyID})
}

// RemoveQueuedLobbies 从所有人数桶中批量删除 lobby ID。
func (r *RedisRepository) RemoveQueuedLobbies(ctx context.Context, lobbyIDs []string) error {
	if len(lobbyIDs) == 0 {
		return nil
	}

	members := make([]interface{}, 0, len(lobbyIDs))
	seen := make(map[string]struct{}, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		lobbyID = strings.TrimSpace(lobbyID)
		if lobbyID == "" {
			continue
		}
		if _, exists := seen[lobbyID]; exists {
			continue
		}
		seen[lobbyID] = struct{}{}
		members = append(members, lobbyID)
	}
	if len(members) == 0 {
		return nil
	}

	pipe := r.client.TxPipeline()
	for memberCount := 1; memberCount <= maxLobbyQueueMemberBucket; memberCount++ {
		pipe.ZRem(ctx, lobbyQueueKey(memberCount), members...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis matchmaker queue repo: remove lobbies: %w", err)
	}
	return nil
}

// HasQueuedLobbies 判断指定 lobby 是否仍全部存在于任意等待桶中。
func (r *RedisRepository) HasQueuedLobbies(ctx context.Context, lobbyIDs []string) (bool, error) {
	lobbyIDs = normalizedQueueLobbyIDs(lobbyIDs)
	if len(lobbyIDs) == 0 {
		return false, nil
	}

	pipe := r.client.Pipeline()
	commands := make([][]*goredis.FloatCmd, 0, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		lobbyCommands := make([]*goredis.FloatCmd, 0, maxLobbyQueueMemberBucket)
		for memberCount := 1; memberCount <= maxLobbyQueueMemberBucket; memberCount++ {
			lobbyCommands = append(lobbyCommands, pipe.ZScore(ctx, lobbyQueueKey(memberCount), lobbyID))
		}
		commands = append(commands, lobbyCommands)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != goredis.Nil {
		return false, fmt.Errorf("redis matchmaker queue repo: inspect lobbies: %w", err)
	}

	for _, lobbyCommands := range commands {
		found := false
		for _, command := range lobbyCommands {
			if command.Err() == nil {
				found = true
				break
			}
			if command.Err() != goredis.Nil {
				return false, fmt.Errorf("redis matchmaker queue repo: inspect lobby: %w", command.Err())
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// normalizedQueueLobbyIDs 去重并清理等待队列查询参数。
func normalizedQueueLobbyIDs(lobbyIDs []string) []string {
	result := make([]string, 0, len(lobbyIDs))
	seen := make(map[string]struct{}, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		lobbyID = strings.TrimSpace(lobbyID)
		if lobbyID == "" {
			continue
		}
		if _, exists := seen[lobbyID]; exists {
			continue
		}
		seen[lobbyID] = struct{}{}
		result = append(result, lobbyID)
	}
	return result
}

// lobbyQueueKey 生成按成员数量分桶的匹配队列 key。
func lobbyQueueKey(memberCount int) string {
	return fmt.Sprintf("%s%d", lobbyQueueKeyPrefix, memberCount)
}
