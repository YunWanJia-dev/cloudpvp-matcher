package matchmaking_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
	"cloudpvp-matcher/internal/usecase/matchmaking"
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

type mockConfigRepository struct {
	configs map[config.GameMode]*config.MatchConfig
}

var _ config.ConfigRepository = (*mockConfigRepository)(nil)

func newMockConfigRepository(configs []*config.MatchConfig) *mockConfigRepository {
	m := make(map[config.GameMode]*config.MatchConfig, len(configs))
	for _, cfg := range configs {
		m[cfg.GameMode] = cfg
	}
	return &mockConfigRepository{configs: m}
}

func (m *mockConfigRepository) GetMatchConfig(ctx context.Context, mode config.GameMode) (*config.MatchConfig, error) {
	cfg, ok := m.configs[mode]
	if !ok {
		return nil, nil
	}
	return cfg, nil
}

func (m *mockConfigRepository) GetAllMatchConfigs(ctx context.Context) ([]*config.MatchConfig, error) {
	result := make([]*config.MatchConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result, nil
}

type mockEventPublisher struct {
	MatchResults     []*domainmatch.Match
	ServerCreateReqs []*domainmatch.Match
	ConfirmReqs      []*domainmatch.Match
}

var _ domainmatch.EventPublisher = (*mockEventPublisher)(nil)

func newMockEventPublisher() *mockEventPublisher {
	return &mockEventPublisher{}
}

func (m *mockEventPublisher) PublishMatchResult(ctx context.Context, match *domainmatch.Match) error {
	m.MatchResults = append(m.MatchResults, match)
	return nil
}

func (m *mockEventPublisher) PublishServerCreateRequest(ctx context.Context, match *domainmatch.Match) error {
	m.ServerCreateReqs = append(m.ServerCreateReqs, match)
	return nil
}

func (m *mockEventPublisher) PublishConfirmRequest(ctx context.Context, match *domainmatch.Match) error {
	m.ConfirmReqs = append(m.ConfirmReqs, match)
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

func newTestMembers(n int) []domainticket.PlayerInfo {
	members := make([]domainticket.PlayerInfo, n)
	for i := 0; i < n; i++ {
		members[i] = domainticket.PlayerInfo{
			PlayerID: "player-" + string(rune('a'+i)),
			Name:     "Player " + string(rune('A'+i)),
			Region:   "cn-east",
		}
	}
	return members
}

func setupUseCase() (*matchmaking.UseCase, *mockTicketRepository, *mockEventPublisher) {
	ticketRepo := newMockTicketRepository()
	configRepo := newMockConfigRepository([]*config.MatchConfig{
		{
			GameMode:       config.GameModeCSGO5v5,
			TeamSize:       5,
			TeamCount:      2,
			NeedConfirm:    false,
			ConfirmTimeout: 30 * time.Second,
			MatchTimeout:   5 * time.Minute,
		},
	})
	publisher := newMockEventPublisher()

	matchmakers := []domainmatch.Matchmaker{
		domainmatch.NewCSGO5v5Matchmaker(),
	}

	uc := matchmaking.NewUseCase(ticketRepo, configRepo, publisher, matchmakers)
	return uc, ticketRepo, publisher
}

func TestUseCase_EnqueueAndMatch_Success(t *testing.T) {
	uc, ticketRepo, publisher := setupUseCase()
	ctx := context.Background()

	// 先入队第一张票据（此时应无对手，留在池中）
	ticket1 := newTestTicket("t1", "lobby1")
	ticket1.Status = domainticket.TicketStatusPending

	err := uc.EnqueueAndMatch(ctx, ticket1)
	if err != nil {
		t.Fatalf("第一张票据入队不应失败: %v", err)
	}

	saved1, _ := ticketRepo.FindByID(ctx, "t1")
	if saved1 == nil {
		t.Fatal("票据应已保存")
	}
	if saved1.Status != domainticket.TicketStatusMatching {
		t.Errorf("状态应为 Matching，实际 %s", saved1.Status)
	}
	if len(publisher.MatchResults) != 0 {
		t.Error("无对手时不应发布匹配结果")
	}

	// 入队第二张票据（应找到匹配）
	ticket2 := newTestTicket("t2", "lobby2")
	ticket2.Status = domainticket.TicketStatusPending

	err = uc.EnqueueAndMatch(ctx, ticket2)
	if err != nil {
		t.Fatalf("第二张票据入队不应失败: %v", err)
	}

	if len(publisher.MatchResults) != 1 {
		t.Fatalf("应发布1个匹配结果，实际 %d 个", len(publisher.MatchResults))
	}
	if len(publisher.ServerCreateReqs) != 1 {
		t.Fatalf("应发布1个创建服务器请求，实际 %d 个", len(publisher.ServerCreateReqs))
	}

	for _, id := range []string{"t1", "t2"} {
		saved, _ := ticketRepo.FindByID(ctx, id)
		if saved == nil || saved.Status != domainticket.TicketStatusConfirmed {
			t.Errorf("票据 %s 状态应为 Confirmed", id)
		}
	}
}

func TestUseCase_EnqueueAndMatch_InvalidTeamSize(t *testing.T) {
	uc, _, _ := setupUseCase()
	ctx := context.Background()

	ticket := newTestTicket("t1", "lobby1")
	ticket.Members = newTestMembers(3)
	ticket.Status = domainticket.TicketStatusPending

	err := uc.EnqueueAndMatch(ctx, ticket)
	if err == nil {
		t.Error("人数不足时应返回错误")
	}
}

func TestUseCase_EnqueueAndMatch_UnknownMode(t *testing.T) {
	uc, _, _ := setupUseCase()
	ctx := context.Background()

	ticket := newTestTicket("t1", "lobby1")
	ticket.GameMode = "unknown/mode"
	ticket.Status = domainticket.TicketStatusPending

	err := uc.EnqueueAndMatch(ctx, ticket)
	if err == nil {
		t.Error("未知模式时应返回错误")
	}
}

func TestUseCase_EnqueueAndMatch_NeedConfirm(t *testing.T) {
	ticketRepo := newMockTicketRepository()
	configRepo := newMockConfigRepository([]*config.MatchConfig{
		{
			GameMode:       config.GameModeCSGO5v5,
			TeamSize:       5,
			TeamCount:      2,
			NeedConfirm:    true,
			ConfirmTimeout: 30 * time.Second,
			MatchTimeout:   5 * time.Minute,
		},
	})
	publisher := newMockEventPublisher()

	uc := matchmaking.NewUseCase(
		ticketRepo, configRepo, publisher,
		[]domainmatch.Matchmaker{domainmatch.NewCSGO5v5Matchmaker()},
	)

	ctx := context.Background()

	ticket1 := newTestTicket("t1", "lobby1")
	ticket1.Status = domainticket.TicketStatusPending
	_ = uc.EnqueueAndMatch(ctx, ticket1)

	ticket2 := newTestTicket("t2", "lobby2")
	ticket2.Status = domainticket.TicketStatusPending
	_ = uc.EnqueueAndMatch(ctx, ticket2)

	if len(publisher.ConfirmReqs) != 1 {
		t.Errorf("应发布1个确认请求，实际 %d 个", len(publisher.ConfirmReqs))
	}
	if len(publisher.MatchResults) != 0 {
		t.Error("需要确认时不应立即发布匹配结果")
	}

	for _, id := range []string{"t1", "t2"} {
		saved, _ := ticketRepo.FindByID(ctx, id)
		if saved != nil && saved.Status != domainticket.TicketStatusConfirming {
			t.Errorf("需要确认时票据 %s 状态应为 Confirming", id)
		}
	}
}
