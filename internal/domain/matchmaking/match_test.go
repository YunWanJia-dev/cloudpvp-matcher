package matchmaking

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/config"
)

// TestMatchCreateJSONContract 校验 Matcher 输出与 Allocator 共享的完整比赛契约。
func TestMatchCreateJSONContract(t *testing.T) {
	now := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	match := Match{
		MatchID:  "match-1",
		GameMode: config.GameModeCSGO5v5,
		Status:   MatchStatusWaitingForServer,
		Teams: []Team{
			{LobbyIDs: []string{"lobby-1"}, Members: []Member{{PlayerID: "player-1"}}},
		},
		Server:    nil,
		CreatedAt: now,
		UpdatedAt: now,
	}

	body, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(body)
	for _, expected := range []string{
		`"match_id":"match-1"`,
		`"status":"WAITING_FOR_SERVER"`,
		`"lobby_ids":["lobby-1"]`,
		`"members":[{"player_id":"player-1"}]`,
		`"server":null`,
	} {
		if !strings.Contains(jsonText, expected) {
			t.Fatalf("json = %s, missing %s", jsonText, expected)
		}
	}
	for _, forbidden := range []string{`"reason"`, `"provider"`, `"server_id"`, `"host"`, `"port"`} {
		if strings.Contains(jsonText, forbidden) {
			t.Fatalf("json = %s, contains forbidden %s", jsonText, forbidden)
		}
	}
}
