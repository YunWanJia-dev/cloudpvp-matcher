package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"

	goredis "github.com/redis/go-redis/v9"
)

const (
	ticketKeyPrefix = "matcher:ticket:"
	ticketTTL       = 10 * time.Minute
	modeIndexPrefix = "matcher:mode:"
)

// RedisTicketRepository 通过 Redis 实现 TicketRepository 端口。
type RedisTicketRepository struct {
	client *goredis.Client
}

var _ ticket.Repository = (*RedisTicketRepository)(nil)

// NewRedisTicketRepository 创建一个新的 Redis 票据仓储。
func NewRedisTicketRepository(client *goredis.Client) *RedisTicketRepository {
	return &RedisTicketRepository{client: client}
}

// Save 保存队列票据，并按游戏模式维护待匹配索引。
func (r *RedisTicketRepository) Save(ctx context.Context, queuedTicket *ticket.Ticket) error {
	if queuedTicket == nil {
		return fmt.Errorf("redis ticket repo: ticket is nil")
	}
	if queuedTicket.LobbyID == "" {
		return fmt.Errorf("redis ticket repo: lobby id is required")
	}

	existing, err := r.FindByLobbyID(ctx, queuedTicket.LobbyID)
	if err != nil {
		return fmt.Errorf("redis ticket repo: load existing ticket: %w", err)
	}

	data, err := json.Marshal(queuedTicket)
	if err != nil {
		return fmt.Errorf("redis ticket repo: marshal failed: %w", err)
	}

	key := ticketKey(queuedTicket.LobbyID)
	if err := r.client.Set(ctx, key, string(data), ticketTTL).Err(); err != nil {
		return fmt.Errorf("redis ticket repo: save ticket: %w", err)
	}

	if existing != nil && existing.GameMode != queuedTicket.GameMode {
		_ = r.client.SRem(ctx, modeIndexKey(existing.GameMode), queuedTicket.LobbyID).Err()
	}
	if err := r.client.SAdd(ctx, modeIndexKey(queuedTicket.GameMode), queuedTicket.LobbyID).Err(); err != nil {
		return fmt.Errorf("redis ticket repo: save mode index: %w", err)
	}
	return nil
}

// FindByLobbyID 通过唯一 lobby ID 查找票据。
func (r *RedisTicketRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*ticket.Ticket, error) {
	if lobbyID == "" {
		return nil, nil
	}

	data, err := r.client.Get(ctx, ticketKey(lobbyID)).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: find by lobby id: %w", err)
	}

	var queuedTicket ticket.Ticket
	if err := json.Unmarshal([]byte(data), &queuedTicket); err != nil {
		return nil, fmt.Errorf("redis ticket repo: unmarshal ticket: %w", err)
	}
	return &queuedTicket, nil
}

// ListByGameMode 返回指定模式下仍在队列中的票据。
func (r *RedisTicketRepository) ListByGameMode(ctx context.Context, mode config.GameMode) ([]*ticket.Ticket, error) {
	lobbyIDs, err := r.client.SMembers(ctx, modeIndexKey(mode)).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: load mode index: %w", err)
	}

	tickets := make([]*ticket.Ticket, 0, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		queuedTicket, err := r.FindByLobbyID(ctx, lobbyID)
		if err != nil {
			continue
		}
		if queuedTicket == nil {
			_ = r.client.SRem(ctx, modeIndexKey(mode), lobbyID).Err()
			continue
		}
		if queuedTicket.GameMode == mode {
			tickets = append(tickets, queuedTicket)
		}
	}
	return tickets, nil
}

// Remove 从 Redis 中删除票据及其模式索引。
func (r *RedisTicketRepository) Remove(ctx context.Context, lobbyID string) error {
	queuedTicket, err := r.FindByLobbyID(ctx, lobbyID)
	if err != nil {
		return fmt.Errorf("redis ticket repo: remove ticket lookup: %w", err)
	}
	if queuedTicket == nil {
		return nil
	}

	if err := r.client.Del(ctx, ticketKey(lobbyID)).Err(); err != nil {
		return fmt.Errorf("redis ticket repo: remove ticket: %w", err)
	}
	_ = r.client.SRem(ctx, modeIndexKey(queuedTicket.GameMode), lobbyID).Err()
	return nil
}

func ticketKey(lobbyID string) string {
	return ticketKeyPrefix + lobbyID
}

func modeIndexKey(mode config.GameMode) string {
	return modeIndexPrefix + string(mode)
}
