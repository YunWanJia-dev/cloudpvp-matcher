package ticket_test

import (
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

func newTestTicket(lobbyID string, memberCount int) *domainticket.Ticket {
	now := time.Now()
	members := make([]domainticket.PlayerInfo, memberCount)
	for i := range members {
		members[i] = domainticket.PlayerInfo{PlayerID: "player"}
	}
	return &domainticket.Ticket{
		LobbyID:   lobbyID,
		GameMode:  config.GameModeCSGO5v5,
		Members:   members,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestTicket_TeamSize(t *testing.T) {
	ticket := newTestTicket("lobby1", 5)
	if got := ticket.TeamSize(); got != 5 {
		t.Errorf("TeamSize() = %d, want 5", got)
	}
}
