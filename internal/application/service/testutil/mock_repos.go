// Package testutil 提供测试用的 mock 实现，避免测试依赖外部基础设施。
package testutil

import (
	"context"
	"sync"

	"cloudpvp-matcher/internal/domain/entity"
	"cloudpvp-matcher/internal/domain/repository"
	"cloudpvp-matcher/internal/domain/valueobject"
)

// MockTicketRepository 基于内存的 TicketRepository mock 实现。
type MockTicketRepository struct {
	mu      sync.RWMutex
	tickets map[string]*entity.Ticket
}

var _ repository.TicketRepository = (*MockTicketRepository)(nil)

// NewMockTicketRepository 创建内存票据仓储。
func NewMockTicketRepository() *MockTicketRepository {
	return &MockTicketRepository{tickets: make(map[string]*entity.Ticket)}
}

func (m *MockTicketRepository) Save(ctx context.Context, ticket *entity.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := *ticket
	m.tickets[ticket.ID] = &t
	return nil
}

func (m *MockTicketRepository) FindByID(ctx context.Context, id string) (*entity.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tickets[id]
	if !ok {
		return nil, nil
	}
	t2 := *t
	return &t2, nil
}

func (m *MockTicketRepository) FindByLobbyID(ctx context.Context, lobbyID string) (*entity.Ticket, error) {
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

func (m *MockTicketRepository) FindByStatus(ctx context.Context, mode valueobject.GameMode, status valueobject.TicketStatus) ([]*entity.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*entity.Ticket
	for _, t := range m.tickets {
		if t.GameMode == mode && t.Status == status {
			t2 := *t
			result = append(result, &t2)
		}
	}
	return result, nil
}

func (m *MockTicketRepository) UpdateStatus(ctx context.Context, id string, status valueobject.TicketStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[id]
	if !ok {
		return nil
	}
	t.Status = status
	return nil
}

func (m *MockTicketRepository) Remove(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tickets, id)
	return nil
}
