package ticket

import (
	"context"

	"cloudpvp-matcher/internal/domain/config"
)

// Repository 是持久化和查询匹配票据的端口。
type Repository interface {
	SaveTicket(ctx context.Context, ticket *Ticket) error
	FindByLobbyID(ctx context.Context, lobbyID string) (*Ticket, error)
	ListByGameMode(ctx context.Context, mode config.GameMode) ([]*Ticket, error)
	Remove(ctx context.Context, lobbyID string) error
}
