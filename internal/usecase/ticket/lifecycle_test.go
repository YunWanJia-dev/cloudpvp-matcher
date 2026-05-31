package ticket_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
	ticketusecase "cloudpvp-matcher/internal/usecase/ticket"
)

type mockTicketRepository struct {
	mu      sync.RWMutex
	tickets map[string]*domainticket.Ticket
}

var _ domainticket.Repository = (*mockTicketRepository)(nil)

func newMockTicketRepository() *mockTicketRepository {
	return &mockTicketRepository{tickets: make(map[string]*domainticket.Ticket)}
}

func (m *mockTicketRepository) Save(ctx context.Context, ticket *domainticket.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := *ticket
	m.tickets[ticket.LobbyID] = &t
	return nil
}

func (m *mockTicketRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*domainticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tickets[lobbyID]
	if !ok {
		return nil, nil
	}
	t2 := *t
	return &t2, nil
}

func (m *mockTicketRepository) ListByGameMode(ctx context.Context, mode config.GameMode) ([]*domainticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domainticket.Ticket
	for _, t := range m.tickets {
		if t.GameMode == mode {
			t2 := *t
			result = append(result, &t2)
		}
	}
	return result, nil
}

func (m *mockTicketRepository) Remove(ctx context.Context, lobbyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tickets, lobbyID)
	return nil
}

func newTestTicket(lobbyID string) *domainticket.Ticket {
	now := time.Now()
	return &domainticket.Ticket{
		LobbyID:  lobbyID,
		GameMode: config.GameModeCSGO5v5,
		Members: []domainticket.PlayerInfo{
			{PlayerID: "p1", Name: "Player1"},
			{PlayerID: "p2", Name: "Player2"},
			{PlayerID: "p3", Name: "Player3"},
			{PlayerID: "p4", Name: "Player4"},
			{PlayerID: "p5", Name: "Player5"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestLifecycle_CancelTicket(t *testing.T) {
	repo := newMockTicketRepository()
	lifecycle := ticketusecase.NewLifecycle(repo)
	ctx := context.Background()

	_ = repo.Save(ctx, newTestTicket("lobby1"))

	err := lifecycle.CancelTicket(ctx, "lobby1")
	if err != nil {
		t.Fatalf("取消票据不应失败: %v", err)
	}

	saved, _ := repo.FindByLobbyID(ctx, "lobby1")
	if saved != nil {
		t.Error("取消后票据应从队列移除")
	}
}

func TestLifecycle_CancelTicket_NotFound(t *testing.T) {
	repo := newMockTicketRepository()
	lifecycle := ticketusecase.NewLifecycle(repo)
	ctx := context.Background()

	err := lifecycle.CancelTicket(ctx, "nonexistent")
	if err == nil {
		t.Error("取消不存在的票据应返回错误")
	}
}

func TestLifecycle_CleanupExpiredTickets(t *testing.T) {
	repo := newMockTicketRepository()
	lifecycle := ticketusecase.NewLifecycle(repo)
	ctx := context.Background()

	active := newTestTicket("lobby1")
	active.CreatedAt = time.Now()
	_ = repo.Save(ctx, active)

	expired := newTestTicket("lobby2")
	expired.CreatedAt = time.Now().Add(-2 * time.Minute)
	_ = repo.Save(ctx, expired)

	cleaned, err := lifecycle.CleanupExpiredTickets(ctx, []config.GameMode{config.GameModeCSGO5v5}, time.Minute)
	if err != nil {
		t.Fatalf("清理过期票据失败: %v", err)
	}
	if cleaned != 1 {
		t.Errorf("应清理1张票据，实际 %d 张", cleaned)
	}

	if saved, _ := repo.FindByLobbyID(ctx, "lobby2"); saved != nil {
		t.Error("过期票据应被移除")
	}
	if saved, _ := repo.FindByLobbyID(ctx, "lobby1"); saved == nil {
		t.Error("未过期票据不应被清理")
	}
}
