package repository

import (
	domainmatchmaker "cloudpvp-matcher/internal/domain/match/matchmaker/csgo_5v5"
	"context"
	"fmt"
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

// RemoveQueuedLobby 从所有人数桶中删除 lobbyID。
func (r *RedisRepository) RemoveQueuedLobby(ctx context.Context, lobbyID string) error {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return nil
	}

	pipe := r.client.TxPipeline()
	for memberCount := 1; memberCount <= maxLobbyQueueMemberBucket; memberCount++ {
		pipe.ZRem(ctx, lobbyQueueKey(memberCount), lobbyID)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis matchmaker queue repo: remove lobby: %w", err)
	}
	return nil
}

// lobbyQueueKey 生成按成员数量分桶的匹配队列 key。
func lobbyQueueKey(memberCount int) string {
	return fmt.Sprintf("%s%d", lobbyQueueKeyPrefix, memberCount)
}
