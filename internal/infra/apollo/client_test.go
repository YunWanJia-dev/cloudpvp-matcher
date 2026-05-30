package apollo

import (
	"testing"
	"time"

	"cloudpvp-matcher/internal/domain/valueobject"
)

func TestParseMatchConfigs(t *testing.T) {
	raw := `[
		{
			"game_mode": "csgo/5v5/competitive",
			"team_size": 5,
			"team_count": 2,
			"need_confirm": true,
			"confirm_timeout": "30s",
			"match_timeout": "5m"
		}
	]`

	configs, err := parseMatchConfigs(raw)
	if err != nil {
		t.Fatalf("parseMatchConfigs() error = %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("len(configs) = %d, want 1", len(configs))
	}

	cfg := configs[0]
	if cfg.GameMode != valueobject.GameModeCSGO5v5 {
		t.Fatalf("GameMode = %s, want %s", cfg.GameMode, valueobject.GameModeCSGO5v5)
	}
	if cfg.TeamSize != 5 || cfg.TeamCount != 2 {
		t.Fatalf("team shape = %dx%d, want 5x2", cfg.TeamSize, cfg.TeamCount)
	}
	if !cfg.NeedConfirm {
		t.Fatal("NeedConfirm = false, want true")
	}
	if cfg.ConfirmTimeout != 30*time.Second {
		t.Fatalf("ConfirmTimeout = %s, want 30s", cfg.ConfirmTimeout)
	}
	if cfg.MatchTimeout != 5*time.Minute {
		t.Fatalf("MatchTimeout = %s, want 5m", cfg.MatchTimeout)
	}
}

func TestParseMatchConfigsNumericDurationsUseSeconds(t *testing.T) {
	raw := `[
		{
			"game_mode": "csgo/5v5/competitive",
			"team_size": 5,
			"team_count": 2,
			"need_confirm": false,
			"confirm_timeout": 30,
			"match_timeout": 300
		}
	]`

	configs, err := parseMatchConfigs(raw)
	if err != nil {
		t.Fatalf("parseMatchConfigs() error = %v", err)
	}

	cfg := configs[0]
	if cfg.ConfirmTimeout != 30*time.Second {
		t.Fatalf("ConfirmTimeout = %s, want 30s", cfg.ConfirmTimeout)
	}
	if cfg.MatchTimeout != 5*time.Minute {
		t.Fatalf("MatchTimeout = %s, want 5m", cfg.MatchTimeout)
	}
}

func TestParseMatchConfigsRejectsInvalidConfig(t *testing.T) {
	raw := `[
		{
			"game_mode": "csgo/5v5/competitive",
			"team_size": 0,
			"team_count": 2,
			"need_confirm": false,
			"confirm_timeout": "30s",
			"match_timeout": "5m"
		}
	]`

	if _, err := parseMatchConfigs(raw); err == nil {
		t.Fatal("parseMatchConfigs() error = nil, want error")
	}
}
