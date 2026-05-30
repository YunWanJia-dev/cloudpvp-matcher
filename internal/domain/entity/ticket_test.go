package entity_test

import (
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/entity"
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

func TestTicket_IsActive(t *testing.T) {
	ticket := newTestTicket("t1", "lobby1")

	tests := []struct {
		name   string
		status valueobject.TicketStatus
		active bool
	}{
		{"pending 应为活跃", valueobject.TicketStatusPending, true},
		{"matching 应为活跃", valueobject.TicketStatusMatching, true},
		{"matched 应为活跃", valueobject.TicketStatusMatched, true},
		{"confirming 应为活跃", valueobject.TicketStatusConfirming, true},
		{"confirmed 不应为活跃", valueobject.TicketStatusConfirmed, false},
		{"cancelled 不应为活跃", valueobject.TicketStatusCancelled, false},
		{"timed_out 不应为活跃", valueobject.TicketStatusTimedOut, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket.Status = tt.status
			if got := ticket.IsActive(); got != tt.active {
				t.Errorf("IsActive() = %v, want %v", got, tt.active)
			}
		})
	}
}

func TestTicket_IsFull(t *testing.T) {
	cfg := &valueobject.MatchConfig{GameMode: valueobject.GameModeCSGO5v5, TeamSize: 5, TeamCount: 2}

	ticket := newTestTicket("t1", "lobby1")
	if !ticket.IsFull(cfg) {
		t.Error("5人票据应满员")
	}

	ticket.Members = newTestMembers(4)
	if ticket.IsFull(cfg) {
		t.Error("4人票据不应满员")
	}
}

func TestTicket_TeamSize(t *testing.T) {
	ticket := newTestTicket("t1", "lobby1")
	if got := ticket.TeamSize(); got != 5 {
		t.Errorf("TeamSize() = %d, want 5", got)
	}
}
