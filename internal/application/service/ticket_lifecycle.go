// Package service 实现应用层用例编排，依赖领域层端口。
package service

import (
	"context"
	"fmt"
	"time"

	"cloudpvp-matcher/internal/domain/entity"
	domainrepo "cloudpvp-matcher/internal/domain/repository"
	"cloudpvp-matcher/internal/domain/valueobject"
)

// TicketLifecycle 管理票据的完整生命周期，包括取消、超时清理等。
type TicketLifecycle struct {
	ticketRepo domainrepo.TicketRepository
}

// NewTicketLifecycle 创建票据生命周期管理器。
func NewTicketLifecycle(ticketRepo domainrepo.TicketRepository) *TicketLifecycle {
	return &TicketLifecycle{ticketRepo: ticketRepo}
}

// CancelTicket 取消指定票据并将其从匹配池中移除。
func (l *TicketLifecycle) CancelTicket(ctx context.Context, ticketID string) error {
	ticket, err := l.ticketRepo.FindByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("取消票据失败: 查找票据 %s: %w", ticketID, err)
	}
	if ticket == nil {
		return fmt.Errorf("取消票据失败: 未找到票据 %s", ticketID)
	}

	if !ticket.IsActive() {
		return fmt.Errorf("取消票据失败: 票据 %s 已处于终止态 %s", ticketID, ticket.Status)
	}

	ticket.Status = valueobject.TicketStatusCancelled
	ticket.UpdatedAt = time.Now()

	return l.ticketRepo.Save(ctx, ticket)
}

// CleanupExpiredTickets 清理超过指定时长的匹配中票据。
// 该方法应由定时任务调用。
func (l *TicketLifecycle) CleanupExpiredTickets(ctx context.Context, modes []valueobject.GameMode, maxAge time.Duration) (int, error) {
	var cleaned int
	cutoff := time.Now().Add(-maxAge)

	for _, mode := range modes {
		tickets, err := l.ticketRepo.FindByStatus(ctx, mode, valueobject.TicketStatusMatching)
		if err != nil {
			continue
		}

		for _, t := range tickets {
			if t.CreatedAt.Before(cutoff) {
				t.Status = valueobject.TicketStatusTimedOut
				t.UpdatedAt = time.Now()
				if err := l.ticketRepo.Save(ctx, t); err != nil {
					continue
				}
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// ToDomainTicket 从 DTO 的成员和模式信息构造领域票据实体。
// 此函数由 handler 层调用，将入站 DTO 转换为领域实体。
func ToDomainTicket(ticketID, lobbyID string, gameMode valueobject.GameMode, members []entity.PlayerInfo) *entity.Ticket {
	now := time.Now()
	return &entity.Ticket{
		ID:        ticketID,
		LobbyID:   lobbyID,
		GameMode:  gameMode,
		Members:   members,
		Status:    valueobject.TicketStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
