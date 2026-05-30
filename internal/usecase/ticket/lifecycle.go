// Package ticket implements ticket lifecycle use cases.
package ticket

import (
	"context"
	"fmt"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

// Lifecycle 管理票据的完整生命周期，包括取消、超时清理等。
type Lifecycle struct {
	ticketRepo domainticket.TicketRepository
}

// NewLifecycle 创建票据生命周期管理器。
func NewLifecycle(ticketRepo domainticket.TicketRepository) *Lifecycle {
	return &Lifecycle{ticketRepo: ticketRepo}
}

// CancelTicket 取消指定票据并将其从匹配池中移除。
func (l *Lifecycle) CancelTicket(ctx context.Context, ticketID string) error {
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

	ticket.Status = domainticket.TicketStatusCancelled
	ticket.UpdatedAt = time.Now()

	return l.ticketRepo.Save(ctx, ticket)
}

// CleanupExpiredTickets 清理超过指定时长的匹配中票据。
// 该方法应由定时任务调用。
func (l *Lifecycle) CleanupExpiredTickets(ctx context.Context, modes []config.GameMode, maxAge time.Duration) (int, error) {
	var cleaned int
	cutoff := time.Now().Add(-maxAge)

	for _, mode := range modes {
		tickets, err := l.ticketRepo.FindByStatus(ctx, mode, domainticket.TicketStatusMatching)
		if err != nil {
			continue
		}

		for _, t := range tickets {
			if t.CreatedAt.Before(cutoff) {
				t.Status = domainticket.TicketStatusTimedOut
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
