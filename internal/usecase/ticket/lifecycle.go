// Package ticket 实现票据生命周期用例。
package ticket

import (
	"context"
	"fmt"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

// Lifecycle 管理队列票据的取消和过期清理。
type Lifecycle struct {
	ticketRepo domainticket.Repository
}

// NewLifecycle 创建票据生命周期管理器。
func NewLifecycle(ticketRepo domainticket.Repository) *Lifecycle {
	return &Lifecycle{ticketRepo: ticketRepo}
}

// CancelTicket 按 lobby ID 取消队列票据，并从匹配池中移除。
func (l *Lifecycle) CancelTicket(ctx context.Context, lobbyID string) error {
	ticket, err := l.ticketRepo.FindByLobbyID(ctx, lobbyID)
	if err != nil {
		return fmt.Errorf("取消票据失败: 查找 lobby %s: %w", lobbyID, err)
	}
	if ticket == nil {
		return fmt.Errorf("取消票据失败: 未找到 lobby %s", lobbyID)
	}

	if err := l.ticketRepo.Remove(ctx, lobbyID); err != nil {
		return fmt.Errorf("取消票据失败: 移除 lobby %s: %w", lobbyID, err)
	}
	return nil
}

// CleanupExpiredTickets 清理超过指定时长的队列票据。
func (l *Lifecycle) CleanupExpiredTickets(ctx context.Context, modes []config.GameMode, maxAge time.Duration) (int, error) {
	var cleaned int
	cutoff := time.Now().Add(-maxAge)

	for _, mode := range modes {
		tickets, err := l.ticketRepo.ListByGameMode(ctx, mode)
		if err != nil {
			continue
		}

		for _, t := range tickets {
			if t.CreatedAt.Before(cutoff) {
				if err := l.ticketRepo.Remove(ctx, t.LobbyID); err != nil {
					continue
				}
				cleaned++
			}
		}
	}

	return cleaned, nil
}
