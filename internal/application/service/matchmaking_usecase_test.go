package service_test

import (
	"context"
	"testing"
	"time"

	appsvc "cloudpvp-matcher/internal/application/service"
	"cloudpvp-matcher/internal/application/service/testutil"
	"cloudpvp-matcher/internal/domain/entity"
	domainsvc "cloudpvp-matcher/internal/domain/service"
	"cloudpvp-matcher/internal/domain/valueobject"
)

func newTestTicket(id, lobbyID string) *entity.Ticket {
	now := time.Now()
	return &entity.Ticket{
		ID:       id,
		LobbyID:  lobbyID,
		GameMode: valueobject.GameModeCSGO5v5,
		Members: []entity.PlayerInfo{
			{PlayerID: "p1", Name: "Player1", Region: "cn-east"},
			{PlayerID: "p2", Name: "Player2", Region: "cn-east"},
			{PlayerID: "p3", Name: "Player3", Region: "cn-east"},
			{PlayerID: "p4", Name: "Player4", Region: "cn-east"},
			{PlayerID: "p5", Name: "Player5", Region: "cn-east"},
		},
		Status:    valueobject.TicketStatusMatching,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newTestMembers(n int) []entity.PlayerInfo {
	members := make([]entity.PlayerInfo, n)
	for i := 0; i < n; i++ {
		members[i] = entity.PlayerInfo{
			PlayerID: "player-" + string(rune('a'+i)),
			Name:     "Player " + string(rune('A'+i)),
			Region:   "cn-east",
		}
	}
	return members
}

func setupUseCase() (*appsvc.MatchmakingUseCase, *testutil.MockTicketRepository, *testutil.MockEventPublisher) {
	ticketRepo := testutil.NewMockTicketRepository()
	configRepo := testutil.NewMockConfigRepository([]*valueobject.MatchConfig{
		{
			GameMode:       valueobject.GameModeCSGO5v5,
			TeamSize:       5,
			TeamCount:      2,
			NeedConfirm:    false,
			ConfirmTimeout: 30 * time.Second,
			MatchTimeout:   5 * time.Minute,
		},
	})
	publisher := testutil.NewMockEventPublisher()

	matchmakers := []domainsvc.Matchmaker{
		domainsvc.NewCSGO5v5Matchmaker(),
	}

	uc := appsvc.NewMatchmakingUseCase(ticketRepo, configRepo, publisher, matchmakers)
	return uc, ticketRepo, publisher
}

func TestMatchmakingUseCase_EnqueueAndMatch_Success(t *testing.T) {
	uc, ticketRepo, publisher := setupUseCase()
	ctx := context.Background()

	// 先入队第一张票据（此时应无对手，留在池中）
	ticket1 := newTestTicket("t1", "lobby1")
	ticket1.Status = valueobject.TicketStatusPending

	err := uc.EnqueueAndMatch(ctx, ticket1)
	if err != nil {
		t.Fatalf("第一张票据入队不应失败: %v", err)
	}

	saved1, _ := ticketRepo.FindByID(ctx, "t1")
	if saved1 == nil {
		t.Fatal("票据应已保存")
	}
	if saved1.Status != valueobject.TicketStatusMatching {
		t.Errorf("状态应为 Matching，实际 %s", saved1.Status)
	}
	if len(publisher.MatchResults) != 0 {
		t.Error("无对手时不应发布匹配结果")
	}

	// 入队第二张票据（应找到匹配）
	ticket2 := newTestTicket("t2", "lobby2")
	ticket2.Status = valueobject.TicketStatusPending

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
		if saved == nil || saved.Status != valueobject.TicketStatusConfirmed {
			t.Errorf("票据 %s 状态应为 Confirmed", id)
		}
	}
}

func TestMatchmakingUseCase_EnqueueAndMatch_InvalidTeamSize(t *testing.T) {
	uc, _, _ := setupUseCase()
	ctx := context.Background()

	ticket := newTestTicket("t1", "lobby1")
	ticket.Members = newTestMembers(3)
	ticket.Status = valueobject.TicketStatusPending

	err := uc.EnqueueAndMatch(ctx, ticket)
	if err == nil {
		t.Error("人数不足时应返回错误")
	}
}

func TestMatchmakingUseCase_EnqueueAndMatch_UnknownMode(t *testing.T) {
	uc, _, _ := setupUseCase()
	ctx := context.Background()

	ticket := newTestTicket("t1", "lobby1")
	ticket.GameMode = "unknown/mode"
	ticket.Status = valueobject.TicketStatusPending

	err := uc.EnqueueAndMatch(ctx, ticket)
	if err == nil {
		t.Error("未知模式时应返回错误")
	}
}

func TestMatchmakingUseCase_EnqueueAndMatch_NeedConfirm(t *testing.T) {
	ticketRepo := testutil.NewMockTicketRepository()
	configRepo := testutil.NewMockConfigRepository([]*valueobject.MatchConfig{
		{
			GameMode:       valueobject.GameModeCSGO5v5,
			TeamSize:       5,
			TeamCount:      2,
			NeedConfirm:    true,
			ConfirmTimeout: 30 * time.Second,
			MatchTimeout:   5 * time.Minute,
		},
	})
	publisher := testutil.NewMockEventPublisher()

	uc := appsvc.NewMatchmakingUseCase(
		ticketRepo, configRepo, publisher,
		[]domainsvc.Matchmaker{domainsvc.NewCSGO5v5Matchmaker()},
	)

	ctx := context.Background()

	ticket1 := newTestTicket("t1", "lobby1")
	ticket1.Status = valueobject.TicketStatusPending
	_ = uc.EnqueueAndMatch(ctx, ticket1)

	ticket2 := newTestTicket("t2", "lobby2")
	ticket2.Status = valueobject.TicketStatusPending
	_ = uc.EnqueueAndMatch(ctx, ticket2)

	if len(publisher.ConfirmReqs) != 1 {
		t.Errorf("应发布1个确认请求，实际 %d 个", len(publisher.ConfirmReqs))
	}
	if len(publisher.MatchResults) != 0 {
		t.Error("需要确认时不应立即发布匹配结果")
	}

	for _, id := range []string{"t1", "t2"} {
		saved, _ := ticketRepo.FindByID(ctx, id)
		if saved != nil && saved.Status != valueobject.TicketStatusConfirming {
			t.Errorf("需要确认时票据 %s 状态应为 Confirming", id)
		}
	}
}
