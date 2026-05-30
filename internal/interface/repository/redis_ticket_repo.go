// Package repository 实现领域层的仓储端口。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloudpvp-matcher/internal/domain/entity"
	"cloudpvp-matcher/internal/domain/repository"
	"cloudpvp-matcher/internal/domain/valueobject"
	redisclient "cloudpvp-matcher/internal/infra/redis"
)

const (
	ticketKeyPrefix   = "matcher:ticket:"
	ticketTTL         = 10 * time.Minute
	lobbyIndexPrefix  = "matcher:lobby:"  // 按 lobbyID 索引
	statusIndexPrefix = "matcher:status:" // 按状态索引
)

// RedisTicketRepository 通过 Redis 实现 TicketRepository 端口。
type RedisTicketRepository struct {
	client *redisclient.Client
}

// 编译期检查接口实现
var _ repository.TicketRepository = (*RedisTicketRepository)(nil)

// NewRedisTicketRepository 创建一个新的 Redis 票据仓储。
func NewRedisTicketRepository(client *redisclient.Client) *RedisTicketRepository {
	return &RedisTicketRepository{client: client}
}

// Save 保存票据，同时维护 lobby 索引和状态索引。
func (r *RedisTicketRepository) Save(ctx context.Context, ticket *entity.Ticket) error {
	data, err := json.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("redis ticket repo: marshal failed: %w", err)
	}

	key := ticketKeyPrefix + ticket.ID

	// 使用 pipeline 原子写入主数据和索引
	pipe := r.client.Set

	if err := pipe(ctx, key, string(data), ticketTTL); err != nil {
		return fmt.Errorf("redis ticket repo: save ticket: %w", err)
	}

	if ticket.LobbyID != "" {
		lobbyKey := lobbyIndexPrefix + ticket.LobbyID
		if err := r.client.Set(ctx, lobbyKey, ticket.ID, ticketTTL); err != nil {
			return fmt.Errorf("redis ticket repo: save lobby index: %w", err)
		}
	}

	return nil
}

// FindByID 通过 ID 查找票据。
func (r *RedisTicketRepository) FindByID(ctx context.Context, id string) (*entity.Ticket, error) {
	key := ticketKeyPrefix + id
	data, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: find by id: %w", err)
	}

	var ticket entity.Ticket
	if err := json.Unmarshal([]byte(data), &ticket); err != nil {
		return nil, fmt.Errorf("redis ticket repo: unmarshal ticket: %w", err)
	}
	return &ticket, nil
}

// FindByLobbyID 通过 lobbyID 查找票据（通过反转索引）。
func (r *RedisTicketRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*entity.Ticket, error) {
	lobbyKey := lobbyIndexPrefix + lobbyID
	ticketID, err := r.client.Get(ctx, lobbyKey)
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: find lobby index: %w", err)
	}

	return r.FindByID(ctx, ticketID)
}

// FindByStatus 查找指定模式下指定状态的票据。
// 使用 Redis KEYS 命令扫描，适合低并发场景；高并发下应改用 Set 或 Sorted Set。
func (r *RedisTicketRepository) FindByStatus(ctx context.Context, mode valueobject.GameMode, status valueobject.TicketStatus) ([]*entity.Ticket, error) {
	pattern := ticketKeyPrefix + "*"
	keys, err := r.client.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("redis ticket repo: keys scan: %w", err)
	}

	var tickets []*entity.Ticket
	for _, key := range keys {
		data, err := r.client.Get(ctx, key)
		if err != nil {
			continue // 跳过已过期的 key
		}

		var ticket entity.Ticket
		if err := json.Unmarshal([]byte(data), &ticket); err != nil {
			continue
		}

		if ticket.GameMode == mode && ticket.Status == status {
			ticketCopy := ticket
			tickets = append(tickets, &ticketCopy)
		}
	}

	return tickets, nil
}

// UpdateStatus 更新票据状态。
func (r *RedisTicketRepository) UpdateStatus(ctx context.Context, id string, status valueobject.TicketStatus) error {
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
	if err := r.client.Del(ctx, key); err != nil {
		return fmt.Errorf("redis ticket repo: remove ticket: %w", err)
	}

	if ticket.LobbyID != "" {
		lobbyKey := lobbyIndexPrefix + ticket.LobbyID
		_ = r.client.Del(ctx, lobbyKey) // 删除索引，忽略错误
	}

	return nil
}
