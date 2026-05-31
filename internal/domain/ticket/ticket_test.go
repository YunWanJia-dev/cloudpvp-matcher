package ticket_test

import (
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

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

func TestTicket_IsActive(t *testing.T) {
	ticket := newTestTicket("t1", "lobby1")

	tests := []struct {
		name   string
		status domainticket.TicketStatus
		active bool
	}{
		{"pending 应为活跃", domainticket.TicketStatusPending, true},
		{"matching 应为活跃", domainticket.TicketStatusMatching, true},
		{"matched 应为活跃", domainticket.TicketStatusMatched, true},
		{"confirming 应为活跃", domainticket.TicketStatusConfirming, true},
		{"confirmed 不应为活跃", domainticket.TicketStatusConfirmed, false},
		{"cancelled 不应为活跃", domainticket.TicketStatusCancelled, false},
		{"timed_out 不应为活跃", domainticket.TicketStatusTimedOut, false},
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

func TestTicket_TeamSize(t *testing.T) {
	ticket := newTestTicket("t1", "lobby1")
	if got := ticket.TeamSize(); got != 5 {
		t.Errorf("TeamSize() = %d, want 5", got)
	}
}
