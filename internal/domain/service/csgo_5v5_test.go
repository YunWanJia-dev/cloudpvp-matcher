package service_test

import (
	"testing"
	"time"

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

func TestCSGO5v5Matchmaker_Supports(t *testing.T) {
	mm := domainsvc.NewCSGO5v5Matchmaker()
	if !mm.Supports(valueobject.GameModeCSGO5v5) {
		t.Error("应支持 csgo/5v5/competitive")
	}
	if mm.Supports("other/mode") {
		t.Error("不应支持其他模式")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_Success(t *testing.T) {
	mm := domainsvc.NewCSGO5v5Matchmaker()

	candidate := newTestTicket("t1", "lobby1")
	pool := []*entity.Ticket{
		candidate,
		newTestTicket("t2", "lobby2"),
	}

	match := mm.FindMatch(candidate, pool)
	if match == nil {
		t.Fatal("应找到对手")
	}
	if len(match.Tickets) != 2 {
		t.Errorf("匹配应包含2张票据，实际 %d 张", len(match.Tickets))
	}
	if match.Tickets[0].ID != candidate.ID && match.Tickets[1].ID != candidate.ID {
		t.Error("匹配应收录候选票据")
	}
	if match.GameMode != valueobject.GameModeCSGO5v5 {
		t.Errorf("GameMode = %s, want %s", match.GameMode, valueobject.GameModeCSGO5v5)
	}
}

func TestCSGO5v5Matchmaker_FindMatch_NoOpponent(t *testing.T) {
	mm := domainsvc.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")
	pool := []*entity.Ticket{candidate}

	match := mm.FindMatch(candidate, pool)
	if match != nil {
		t.Error("只有自己时不应匹配")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_EmptyPool(t *testing.T) {
	mm := domainsvc.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")

	match := mm.FindMatch(candidate, nil)
	if match != nil {
		t.Error("空池不应返回匹配")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_WrongStatus(t *testing.T) {
	mm := domainsvc.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")

	t2 := newTestTicket("t2", "lobby2")
	t2.Status = valueobject.TicketStatusCancelled

	match := mm.FindMatch(candidate, []*entity.Ticket{candidate, t2})
	if match != nil {
		t.Error("对方已取消时不应匹配")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_WrongTeamSize(t *testing.T) {
	mm := domainsvc.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")

	t2 := newTestTicket("t2", "lobby2")
	t2.Members = newTestMembers(4)

	match := mm.FindMatch(candidate, []*entity.Ticket{candidate, t2})
	if match != nil {
		t.Error("对方不满员时不应匹配")
	}
}

func TestMatch_AllPlayers(t *testing.T) {
	now := time.Now()
	match := &entity.Match{
		ID:       "match-1",
		GameMode: valueobject.GameModeCSGO5v5,
		Tickets: []*entity.Ticket{
			{ID: "t1", Members: []entity.PlayerInfo{{PlayerID: "p1"}, {PlayerID: "p2"}}},
			{ID: "t2", Members: []entity.PlayerInfo{{PlayerID: "p3"}, {PlayerID: "p4"}, {PlayerID: "p5"}}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	players := match.AllPlayers()
	if len(players) != 5 {
		t.Errorf("AllPlayers() 应返回5人，实际 %d 人", len(players))
	}
}
