package matchmaking_test

import (
	"context"
	"fmt"
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
	Queued           []*domainticket.Ticket
	MatchResults     []*domainmatch.Match
	ServerCreateReqs []*domainmatch.Match
	ConfirmReqs      []*domainmatch.Match
}

var _ matchmaking.Publisher = (*mockEventPublisher)(nil)

func (m *mockEventPublisher) PublishTicketQueued(ctx context.Context, ticket *domainticket.Ticket) error {
	t := *ticket
	m.Queued = append(m.Queued, &t)
	return nil
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

type mockLockManager struct {
	mu   sync.Mutex
	keys []string
}

var _ domainmatch.LockManager = (*mockLockManager)(nil)

func (m *mockLockManager) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error {
	m.mu.Lock()
	m.keys = append(m.keys, key)
	m.mu.Unlock()
	return fn(ctx)
}

func newTestTicket(lobbyID string, memberCount int) *domainticket.Ticket {
	now := time.Now()
	return &domainticket.Ticket{
		LobbyID:   lobbyID,
		GameMode:  config.GameModeCSGO5v5,
		Members:   newTestMembers(lobbyID, memberCount),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newTestMembers(prefix string, n int) []domainticket.PlayerInfo {
	members := make([]domainticket.PlayerInfo, n)
	for i := 0; i < n; i++ {
		members[i] = domainticket.PlayerInfo{
			PlayerID: fmt.Sprintf("%s-player-%d", prefix, i+1),
			Name:     fmt.Sprintf("Player %d", i+1),
			Region:   "cn-east",
		}
	}
	return members
}

func setupUseCase(options ...matchmaking.Option) (*matchmaking.UseCase, *mockTicketRepository, *mockEventPublisher) {
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
	publisher := &mockEventPublisher{}
	matchmakers := []domainmatch.Matchmaker{domainmatch.NewCSGO5v5Matchmaker()}

	uc := matchmaking.NewUseCase(ticketRepo, configRepo, publisher, matchmakers, options...)
	return uc, ticketRepo, publisher
}

func TestUseCase_Enqueue_PersistsAndPublishesQueuedEvent(t *testing.T) {
	uc, ticketRepo, publisher := setupUseCase()
	ctx := context.Background()

	ticket := newTestTicket("lobby1", 3)
	err := uc.Enqueue(ctx, ticket)
	if err != nil {
		t.Fatalf("未满员 lobby 入队不应失败: %v", err)
	}

	saved, _ := ticketRepo.FindByLobbyID(ctx, "lobby1")
	if saved == nil {
		t.Fatal("票据应已保存")
	}
	if saved.TeamSize() != 3 {
		t.Fatalf("应保留原 lobby 人数，实际 %d", saved.TeamSize())
	}
	if len(publisher.Queued) != 1 {
		t.Fatalf("应发布1个入队成功事件，实际 %d 个", len(publisher.Queued))
	}
	if len(publisher.MatchResults) != 0 {
		t.Error("入队阶段不应直接发布匹配结果")
	}
}

func TestUseCase_Enqueue_RejectsOversizedLobby(t *testing.T) {
	uc, ticketRepo, _ := setupUseCase()
	ctx := context.Background()

	err := uc.Enqueue(ctx, newTestTicket("lobby1", 6))
	if err == nil {
		t.Fatal("超过队伍上限时应返回错误")
	}
	if saved, _ := ticketRepo.FindByLobbyID(ctx, "lobby1"); saved != nil {
		t.Error("非法票据不应持久化")
	}
}

func TestUseCase_Enqueue_UnknownMode(t *testing.T) {
	uc, _, _ := setupUseCase()
	ctx := context.Background()

	ticket := newTestTicket("lobby1", 5)
	ticket.GameMode = "unknown/mode"

	err := uc.Enqueue(ctx, ticket)
	if err == nil {
		t.Error("未知模式时应返回错误")
	}
}

func TestUseCase_RunMatchOnce_ComposesPartialTeams(t *testing.T) {
	uc, ticketRepo, publisher := setupUseCase()
	ctx := context.Background()

	for _, ticket := range []*domainticket.Ticket{
		newTestTicket("lobby1", 3),
		newTestTicket("lobby2", 2),
		newTestTicket("lobby3", 4),
		newTestTicket("lobby4", 1),
	} {
		if err := uc.Enqueue(ctx, ticket); err != nil {
			t.Fatalf("入队失败: %v", err)
		}
	}

	matched, err := uc.RunMatchOnce(ctx, config.GameModeCSGO5v5)
	if err != nil {
		t.Fatalf("匹配扫描不应失败: %v", err)
	}
	if !matched {
		t.Fatal("应形成一场对局")
	}
	if len(publisher.MatchResults) != 1 {
		t.Fatalf("应发布1个匹配结果，实际 %d 个", len(publisher.MatchResults))
	}
	if len(publisher.ServerCreateReqs) != 1 {
		t.Fatalf("应发布1个创建服务器请求，实际 %d 个", len(publisher.ServerCreateReqs))
	}
	if len(publisher.MatchResults[0].Teams) != 2 {
		t.Fatalf("应组成2支队伍，实际 %d 支", len(publisher.MatchResults[0].Teams))
	}

	remaining, _ := ticketRepo.ListByGameMode(ctx, config.GameModeCSGO5v5)
	if len(remaining) != 0 {
		t.Fatalf("匹配成功后票据应离开队列，剩余 %d 张", len(remaining))
	}
}

func TestUseCase_RunMatchOnce_NeedConfirm(t *testing.T) {
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
	publisher := &mockEventPublisher{}
	uc := matchmaking.NewUseCase(
		ticketRepo,
		configRepo,
		publisher,
		[]domainmatch.Matchmaker{domainmatch.NewCSGO5v5Matchmaker()},
	)
	ctx := context.Background()

	_ = uc.Enqueue(ctx, newTestTicket("lobby1", 5))
	_ = uc.Enqueue(ctx, newTestTicket("lobby2", 5))

	matched, err := uc.RunMatchOnce(ctx, config.GameModeCSGO5v5)
	if err != nil {
		t.Fatalf("匹配扫描不应失败: %v", err)
	}
	if !matched {
		t.Fatal("应形成一场待确认对局")
	}
	if len(publisher.ConfirmReqs) != 1 {
		t.Errorf("应发布1个确认请求，实际 %d 个", len(publisher.ConfirmReqs))
	}
	if len(publisher.MatchResults) != 0 {
		t.Error("需要确认时不应立即发布匹配结果")
	}
}

func TestUseCase_RunMatchOnce_UsesLockManager(t *testing.T) {
	lockManager := &mockLockManager{}
	uc, _, _ := setupUseCase(matchmaking.WithLockManager(lockManager))
	ctx := context.Background()

	_, err := uc.RunMatchOnce(ctx, config.GameModeCSGO5v5)
	if err != nil {
		t.Fatalf("匹配扫描不应失败: %v", err)
	}
	if len(lockManager.keys) != 1 {
		t.Fatalf("应使用分布式锁，实际调用 %d 次", len(lockManager.keys))
	}
}
