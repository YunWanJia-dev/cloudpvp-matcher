package app

import (
	"errors"
	"fmt"
	"testing"
)

// TestDecodeLobbyFromBizMessage 校验 biz lobby 的 MQ 消息可被 matcher 解析。
func TestDecodeLobbyFromBizMessage(t *testing.T) {
	body := []byte(`{"lobby_id":"123","game_mode":"CS2/5v5/competitive","player_count":1,"created_at":"2026-08-12T06:31:00Z"}`)

	lobby, err := decodeLobby(body)
	if err != nil {
		t.Fatalf("decodeLobby() error = %v", err)
	}
	if lobby.LobbyID != "123" {
		t.Fatalf("LobbyID = %q, want %q", lobby.LobbyID, "123")
	}
	if lobby.GameMode != "CS2/5v5/competitive" {
		t.Fatalf("GameMode = %q", lobby.GameMode)
	}
	if lobby.PlayerCount != 1 {
		t.Fatalf("PlayerCount = %d", lobby.PlayerCount)
	}
	if lobby.CreatedAt.IsZero() || lobby.UpdatedAt.IsZero() {
		t.Fatal("CreatedAt and UpdatedAt must be populated")
	}
}

// TestClassifyLobbyHandlingError 校验确定性校验失败不会进入无限重试。
func TestClassifyLobbyHandlingError(t *testing.T) {
	validationErr := classifyLobbyHandlingError(fmt.Errorf("csgo 5v5 matchmaker: lobby member count exceeds 5"))
	if !errors.Is(validationErr, errInvalidLobbyMessage) {
		t.Fatalf("validation error = %v", validationErr)
	}
	infrastructureErr := fmt.Errorf("redis unavailable")
	if classified := classifyLobbyHandlingError(infrastructureErr); errors.Is(classified, errInvalidLobbyMessage) {
		t.Fatalf("infrastructure error misclassified = %v", classified)
	}
}

// TestDecodeLobbyFromCancelMessage 校验取消消息可复用同一 lobby 契约。
func TestDecodeLobbyFromCancelMessage(t *testing.T) {
	lobby, err := decodeLobby([]byte(`{"lobby_id":"456","game_mode":"CS2/5v5/competitive","player_count":1,"created_at":"2026-08-12T06:31:00Z"}`))
	if err != nil {
		t.Fatalf("decodeLobby() error = %v", err)
	}
	if lobby.LobbyID != "456" {
		t.Fatalf("LobbyID = %q, want %q", lobby.LobbyID, "456")
	}
}

// TestDecodeLobbyRejectsInvalidContract 校验坏消息会被标记为不可重试契约错误。
func TestDecodeLobbyRejectsInvalidContract(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`not-json`),
		[]byte(`{"game_mode":"CS2/5v5/competitive"}`),
		[]byte(`{"lobby_id":"123"}`),
	} {
		if _, err := decodeLobby(body); !errors.Is(err, errInvalidLobbyMessage) {
			t.Fatalf("decodeLobby(%q) error = %v", body, err)
		}
	}
}
