// Package repository 为领域层定义仓储和事件端口（接口）。
// 这些是抽象定义；具体实现位于 interface/repository/。
package repository

import (
	"context"

	"cloudpvp-matcher/internal/domain/entity"
	"cloudpvp-matcher/internal/domain/valueobject"
)

// TicketRepository 是持久化和查询匹配票据的端口。
type TicketRepository interface {
	Save(ctx context.Context, ticket *entity.Ticket) error
	FindByID(ctx context.Context, id string) (*entity.Ticket, error)
	FindByLobbyID(ctx context.Context, lobbyID string) (*entity.Ticket, error)
	FindByStatus(ctx context.Context, mode valueobject.GameMode, status valueobject.TicketStatus) ([]*entity.Ticket, error)
	UpdateStatus(ctx context.Context, id string, status valueobject.TicketStatus) error
	Remove(ctx context.Context, id string) error
}
