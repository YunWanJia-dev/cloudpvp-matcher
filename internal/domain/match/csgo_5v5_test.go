package match_test

import (
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	"cloudpvp-matcher/internal/domain/ticket"
)

func newTestTicket(id, lobbyID string) *ticket.Ticket {
	now := time.Now()
	return &ticket.Ticket{
		ID:       id,
		LobbyID:  lobbyID,
		GameMode: config.GameModeCSGO5v5,
		Members: []ticket.PlayerInfo{
			{PlayerID: "p1", Name: "Player1", Region: "cn-east"},
			{PlayerID: "p2", Name: "Player2", Region: "cn-east"},
			{PlayerID: "p3", Name: "Player3", Region: "cn-east"},
			{PlayerID: "p4", Name: "Player4", Region: "cn-east"},
			{PlayerID: "p5", Name: "Player5", Region: "cn-east"},
		},
		Status:    ticket.TicketStatusMatching,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newTestMembers(n int) []ticket.PlayerInfo {
	members := make([]ticket.PlayerInfo, n)
	for i := 0; i < n; i++ {
		members[i] = ticket.PlayerInfo{
			PlayerID: "player-" + string(rune('a'+i)),
			Name:     "Player " + string(rune('A'+i)),
			Region:   "cn-east",
		}
	}
	return members
}

func TestCSGO5v5Matchmaker_Supports(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()
	if !mm.Supports(config.GameModeCSGO5v5) {
		t.Error("应支持 csgo/5v5/competitive")
	}
	if mm.Supports("other/mode") {
		t.Error("不应支持其他模式")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_Success(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()

	candidate := newTestTicket("t1", "lobby1")
	pool := []*ticket.Ticket{
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
	if match.GameMode != config.GameModeCSGO5v5 {
		t.Errorf("GameMode = %s, want %s", match.GameMode, config.GameModeCSGO5v5)
	}
}

func TestCSGO5v5Matchmaker_FindMatch_NoOpponent(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")
	pool := []*ticket.Ticket{candidate}

	match := mm.FindMatch(candidate, pool)
	if match != nil {
		t.Error("只有自己时不应匹配")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_EmptyPool(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")

	match := mm.FindMatch(candidate, nil)
	if match != nil {
		t.Error("空池不应返回匹配")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_WrongStatus(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")

	t2 := newTestTicket("t2", "lobby2")
	t2.Status = ticket.TicketStatusCancelled

	match := mm.FindMatch(candidate, []*ticket.Ticket{candidate, t2})
	if match != nil {
		t.Error("对方已取消时不应匹配")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_WrongTeamSize(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()
	candidate := newTestTicket("t1", "lobby1")

	t2 := newTestTicket("t2", "lobby2")
	t2.Members = newTestMembers(4)

	match := mm.FindMatch(candidate, []*ticket.Ticket{candidate, t2})
	if match != nil {
		t.Error("对方不满员时不应匹配")
	}
}

func TestMatch_AllPlayers(t *testing.T) {
	now := time.Now()
	match := &domainmatch.Match{
		ID:       "match-1",
		GameMode: config.GameModeCSGO5v5,
		Tickets: []*ticket.Ticket{
			{ID: "t1", Members: []ticket.PlayerInfo{{PlayerID: "p1"}, {PlayerID: "p2"}}},
			{ID: "t2", Members: []ticket.PlayerInfo{{PlayerID: "p3"}, {PlayerID: "p4"}, {PlayerID: "p5"}}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	players := match.AllPlayers()
	if len(players) != 5 {
		t.Errorf("AllPlayers() 应返回5人，实际 %d 人", len(players))
	}
}
