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

var _ domainticket.TicketRepository = (*mockTicketRepository)(nil)

func newMockTicketRepository() *mockTicketRepository {
	return &mockTicketRepository{tickets: make(map[string]*domainticket.Ticket)}
}

func (m *mockTicketRepository) Save(ctx context.Context, ticket *domainticket.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := *ticket
	m.tickets[ticket.ID] = &t
	return nil
}

func (m *mockTicketRepository) FindByID(ctx context.Context, id string) (*domainticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tickets[id]
	if !ok {
		return nil, nil
	}
	t2 := *t
	return &t2, nil
}

func (m *mockTicketRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*domainticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tickets {
		if t.LobbyID == lobbyID {
			t2 := *t
			return &t2, nil
		}
	}
	return nil, nil
}

func (m *mockTicketRepository) FindByStatus(ctx context.Context, mode config.GameMode, status domainticket.TicketStatus) ([]*domainticket.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domainticket.Ticket
	for _, t := range m.tickets {
		if t.GameMode == mode && t.Status == status {
			t2 := *t
			result = append(result, &t2)
		}
	}
	return result, nil
}

func (m *mockTicketRepository) UpdateStatus(ctx context.Context, id string, status domainticket.TicketStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[id]
	if !ok {
		return nil
	}
	t.Status = status
	return nil
}

func (m *mockTicketRepository) Remove(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tickets, id)
	return nil
}

func newTestTicket(id, lobbyID string) *domainticket.Ticket {
	now := time.Now()
	return &domainticket.Ticket{
		ID:       id,
		LobbyID:  lobbyID,
		GameMode: config.GameModeCSGO5v5,
		Members: []domainticket.PlayerInfo{
			{PlayerID: "p1", Name: "Player1", Region: "cn-east"},
			{PlayerID: "p2", Name: "Player2", Region: "cn-east"},
			{PlayerID: "p3", Name: "Player3", Region: "cn-east"},
			{PlayerID: "p4", Name: "Player4", Region: "cn-east"},
			{PlayerID: "p5", Name: "Player5", Region: "cn-east"},
		},
		Status:    domainticket.TicketStatusMatching,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestLifecycle_CancelTicket(t *testing.T) {
	repo := newMockTicketRepository()
	lifecycle := ticketusecase.NewLifecycle(repo)
	ctx := context.Background()

	// 先保存一张票据
	ticket := newTestTicket("t1", "lobby1")
	_ = repo.Save(ctx, ticket)

	err := lifecycle.CancelTicket(ctx, "t1")
	if err != nil {
		t.Fatalf("取消票据不应失败: %v", err)
	}

	saved, _ := repo.FindByID(ctx, "t1")
	if saved.Status != domainticket.TicketStatusCancelled {
		t.Errorf("票据状态应为 Cancelled，实际 %s", saved.Status)
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

	// 创建新的匹配中票据
	active := newTestTicket("t1", "lobby1")
	active.CreatedAt = time.Now()
	_ = repo.Save(ctx, active)

	// 创建过期的匹配中票据（2分钟前创建，用于小于一分钟粒度的过期检查）
	expired := newTestTicket("t2", "lobby2")
	expired.CreatedAt = time.Now().Add(-2 * time.Minute)
	_ = repo.Save(ctx, expired)

	// 清理超过 1 分钟的票据（注意：传入的 maxAge 从 CreatedAt 算起）
	modes := []config.GameMode{config.GameModeCSGO5v5}
	cleaned, err := lifecycle.CleanupExpiredTickets(ctx, modes, 1*time.Minute)
	if err != nil {
		t.Fatalf("清理过期票据失败: %v", err)
	}

	if cleaned != 1 {
		t.Errorf("应清理1张票据，实际 %d 张", cleaned)
	}

	// 验证过期票据已超时
	saved, _ := repo.FindByID(ctx, "t2")
	if saved == nil || saved.Status != domainticket.TicketStatusTimedOut {
		t.Errorf("过期票据状态应为 TimedOut，实际 %v", saved)
	}

	// 验证活跃票据未受影响
	savedActive, _ := repo.FindByID(ctx, "t1")
	if savedActive == nil || savedActive.Status != domainticket.TicketStatusMatching {
		t.Errorf("活跃票据不应被清理")
	}
}
