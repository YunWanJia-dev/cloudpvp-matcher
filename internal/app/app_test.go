package app

import "testing"

// TestDecodeLobbyFromBizMessage 校验 biz lobby 的 MQ 消息可被 matcher 解析。
func TestDecodeLobbyFromBizMessage(t *testing.T) {
	body := []byte(`{"lobby_id":"123","game_mode":"matchmaker/5v5/competitive","members":[{"player_id":"76561198000000001"}],"created_at":"2026-08-12T06:31:00Z"}`)

	lobby, err := decodeLobby(body)
	if err != nil {
		t.Fatalf("decodeLobby() error = %v", err)
	}
	if lobby.LobbyID != "123" {
		t.Fatalf("LobbyID = %q, want %q", lobby.LobbyID, "123")
	}
	if lobby.GameMode != "matchmaker/5v5/competitive" {
		t.Fatalf("GameMode = %q", lobby.GameMode)
	}
	if len(lobby.Members) != 1 || lobby.Members[0].PlayerID != "76561198000000001" {
		t.Fatalf("Members = %#v", lobby.Members)
	}
	if lobby.CreatedAt.IsZero() || lobby.UpdatedAt.IsZero() {
		t.Fatal("CreatedAt and UpdatedAt must be populated")
	}
}

// TestDecodeLobbyFromCancelMessage 校验取消消息可复用同一 lobby 契约。
func TestDecodeLobbyFromCancelMessage(t *testing.T) {
	lobby, err := decodeLobby([]byte(`{"lobby_id":"456","game_mode":"matchmaker/5v5/competitive","members":[],"created_at":"2026-08-12T06:31:00Z"}`))
	if err != nil {
		t.Fatalf("decodeLobby() error = %v", err)
	}
	if lobby.LobbyID != "456" {
		t.Fatalf("LobbyID = %q, want %q", lobby.LobbyID, "456")
	}
}
