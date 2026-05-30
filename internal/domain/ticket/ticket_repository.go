package ticket

import (
	"context"

	"cloudpvp-matcher/internal/domain/config"
)

// TicketRepository 是持久化和查询匹配票据的端口。
type TicketRepository interface {
	Save(ctx context.Context, ticket *Ticket) error
	FindByID(ctx context.Context, id string) (*Ticket, error)
	FindByLobbyID(ctx context.Context, lobbyID string) (*Ticket, error)
	FindByStatus(ctx context.Context, mode config.GameMode, status TicketStatus) ([]*Ticket, error)
	UpdateStatus(ctx context.Context, id string, status TicketStatus) error
	Remove(ctx context.Context, id string) error
}
