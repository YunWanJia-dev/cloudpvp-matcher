package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/domain/ticket"

	goredis "github.com/redis/go-redis/v9"
)

const (
	ticketKeyPrefix   = "matcher:ticket:"
	ticketTTL         = 10 * time.Minute
	lobbyIndexPrefix  = "matcher:lobby:"  // 按 lobbyID 索引
	statusIndexPrefix = "matcher:status:" // 按状态索引
)

// RedisTicketRepository 通过 Redis 实现 TicketRepository 端口。
type RedisTicketRepository struct {
	client *goredis.Client
}

// 编译期检查接口实现
var _ ticket.Repository = (*RedisTicketRepository)(nil)

// NewRedisTicketRepository 创建一个新的 Redis 票据仓储。
func NewRedisTicketRepository(client *goredis.Client) *RedisTicketRepository {
	return &RedisTicketRepository{client: client}
}

// Save 保存票据，同时维护 lobby 索引和状态索引。
func (r *RedisTicketRepository) Save(ctx context.Context, ticket *ticket.Ticket) error {
	data, err := json.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("redis ticket repo: marshal failed: %w", err)
	}

	key := ticketKeyPrefix + ticket.ID

	if err := r.client.Set(ctx, key, string(data), ticketTTL).Err(); err != nil {
		return fmt.Errorf("redis ticket repo: save ticket: %w", err)
	}

	if ticket.LobbyID != "" {
		lobbyKey := lobbyIndexPrefix + ticket.LobbyID
		if err := r.client.Set(ctx, lobbyKey, ticket.ID, ticketTTL).Err(); err != nil {
			return fmt.Errorf("redis ticket repo: save lobby index: %w", err)
		}
	}

	return nil
}

// FindByID 通过 ID 查找票据。
func (r *RedisTicketRepository) FindByID(ctx context.Context, id string) (*ticket.Ticket, error) {
	key := ticketKeyPrefix + id
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: find by id: %w", err)
	}

	var t ticket.Ticket
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, fmt.Errorf("redis ticket repo: unmarshal ticket: %w", err)
	}
	return &t, nil
}

// FindByLobbyID 通过 lobbyID 查找票据（通过反转索引）。
func (r *RedisTicketRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*ticket.Ticket, error) {
	lobbyKey := lobbyIndexPrefix + lobbyID
	ticketID, err := r.client.Get(ctx, lobbyKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: find lobby index: %w", err)
	}

	return r.FindByID(ctx, ticketID)
}

// FindByStatus 查找指定模式下指定状态的票据。
// 使用 Redis KEYS 命令扫描，适合低并发场景；高并发下应改用 Set 或 Sorted Set。
func (r *RedisTicketRepository) FindByStatus(ctx context.Context, mode config.GameMode, status ticket.TicketStatus) ([]*ticket.Ticket, error) {
	pattern := ticketKeyPrefix + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: keys scan: %w", err)
	}

	var tickets []*ticket.Ticket
	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue // 跳过已过期的 key
		}

		var t ticket.Ticket
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			continue
		}

		if t.GameMode == mode && t.Status == status {
			tickets = append(tickets, new(t))
		}
	}

	return tickets, nil
}

// UpdateStatus 更新票据状态。
func (r *RedisTicketRepository) UpdateStatus(ctx context.Context, id string, status ticket.TicketStatus) error {
	ticket, err := r.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("redis ticket repo: update status: find: %w", err)
	}

	ticket.Status = status
	ticket.UpdatedAt = time.Now()

	return r.Save(ctx, ticket)
}

// Remove 从 Redis 中删除票据及其索引。
func (r *RedisTicketRepository) Remove(ctx context.Context, id string) error {
	ticket, err := r.FindByID(ctx, id)
	if err != nil {
		// 票据不存在视为已删除
		return nil
	}

	key := ticketKeyPrefix + id
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis ticket repo: remove ticket: %w", err)
	}

	if ticket.LobbyID != "" {
		lobbyKey := lobbyIndexPrefix + ticket.LobbyID
		_ = r.client.Del(ctx, lobbyKey).Err() // 删除索引，忽略错误
	}

	return nil
}
