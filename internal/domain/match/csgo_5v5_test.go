package match_test

import (
	"fmt"
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	"cloudpvp-matcher/internal/domain/ticket"
)

func newTestTicket(lobbyID string, memberCount int) *ticket.Ticket {
	now := time.Now()
	return &ticket.Ticket{
		LobbyID:   lobbyID,
		GameMode:  config.GameModeCSGO5v5,
		Members:   newTestMembers(lobbyID, memberCount),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newTestMembers(prefix string, n int) []ticket.PlayerInfo {
	members := make([]ticket.PlayerInfo, n)
	for i := 0; i < n; i++ {
		members[i] = ticket.PlayerInfo{
			PlayerID: fmt.Sprintf("%s-player-%d", prefix, i+1),
		}
	}
	return members
}

func csgoConfig() *config.MatchConfig {
	return &config.MatchConfig{
		GameMode:  config.GameModeCSGO5v5,
		TeamSize:  5,
		TeamCount: 2,
	}
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

func TestCSGO5v5Matchmaker_FindMatch_FullLobbyTeams(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()

	match := mm.FindMatch([]*ticket.Ticket{
		newTestTicket("lobby1", 5),
		newTestTicket("lobby2", 5),
	}, csgoConfig())

	if match == nil {
		t.Fatal("应找到对局")
	}
	if len(match.Teams) != 2 {
		t.Fatalf("应组成2支队伍，实际 %d 支", len(match.Teams))
	}
	for _, team := range match.Teams {
		if len(team.Tickets) != 1 || len(team.Members()) != 5 {
			t.Fatalf("满员 lobby 应单独组成 5 人队伍: %+v", team)
		}
	}
}

func TestCSGO5v5Matchmaker_FindMatch_ComposesPartialLobbyTeams(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()

	match := mm.FindMatch([]*ticket.Ticket{
		newTestTicket("lobby1", 3),
		newTestTicket("lobby2", 2),
		newTestTicket("lobby3", 4),
		newTestTicket("lobby4", 1),
	}, csgoConfig())

	if match == nil {
		t.Fatal("应将未满员 lobby 拼成完整对局")
	}
	if len(match.Teams) != 2 {
		t.Fatalf("应组成2支队伍，实际 %d 支", len(match.Teams))
	}
	for _, team := range match.Teams {
		if len(team.Members()) != 5 {
			t.Fatalf("每支队伍应为5人，实际 %d 人", len(team.Members()))
		}
	}
}

func TestCSGO5v5Matchmaker_FindMatch_NoEnoughPlayers(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()

	match := mm.FindMatch([]*ticket.Ticket{
		newTestTicket("lobby1", 3),
		newTestTicket("lobby2", 2),
	}, csgoConfig())
	if match != nil {
		t.Error("只有一支完整队伍时不应返回对局")
	}
}

func TestCSGO5v5Matchmaker_FindMatch_IgnoresOversizedLobby(t *testing.T) {
	mm := domainmatch.NewCSGO5v5Matchmaker()

	match := mm.FindMatch([]*ticket.Ticket{
		newTestTicket("lobby1", 6),
		newTestTicket("lobby2", 5),
		newTestTicket("lobby3", 5),
	}, csgoConfig())
	if match == nil {
		t.Fatal("应忽略超员 lobby 后仍找到对局")
	}
	if len(match.AllTickets()) != 2 {
		t.Fatalf("超员 lobby 不应参与匹配，实际票据数 %d", len(match.AllTickets()))
	}
}

func TestMatch_AllPlayers(t *testing.T) {
	now := time.Now()
	match := &domainmatch.Match{
		ID:       "match-1",
		GameMode: config.GameModeCSGO5v5,
		Teams: []domainmatch.Team{
			{Tickets: []*ticket.Ticket{{LobbyID: "lobby1", Members: newTestMembers("lobby1", 2)}}},
			{Tickets: []*ticket.Ticket{{LobbyID: "lobby2", Members: newTestMembers("lobby2", 3)}}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	players := match.AllPlayers()
	if len(players) != 5 {
		t.Errorf("AllPlayers() 应返回5人，实际 %d 人", len(players))
	}
}
