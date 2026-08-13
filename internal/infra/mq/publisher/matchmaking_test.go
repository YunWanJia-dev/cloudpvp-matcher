package publisher

import (
	"encoding/json"
	"testing"

	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	"cloudpvp-matcher/internal/infra/mq"
)

// TestLobbyEventContract 校验大厅状态回传使用统一消息结构。
func TestLobbyEventContract(t *testing.T) {
	event := domainmatchmaking.NewLobbyEvent("123", domainmatchmaking.LobbyStatusWaiting, "cancelled")
	if event.LobbyID != "123" || event.Status != domainmatchmaking.LobbyStatusWaiting || event.Reason != "cancelled" {
		t.Fatalf("LobbyEvent = %#v", event)
	}
	if lobbyRoutingKey != "matchmaking.lobby" {
		t.Fatalf("lobbyRoutingKey = %q", lobbyRoutingKey)
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(body) != `{"lobby_id":"123","status":"WAITING","reason":"cancelled"}` {
		t.Fatalf("json = %s", body)
	}
}

// TestMatchRoutingKey 校验完整比赛通过 match.create 发布。
func TestMatchRoutingKey(t *testing.T) {
	if matchRoutingKey != "match.create" {
		t.Fatalf("matchRoutingKey = %q", matchRoutingKey)
	}
}

// TestMatchQueueTopologyNames 校验 Biz 与 Allocator 使用独立队列。
func TestMatchQueueTopologyNames(t *testing.T) {
	if mq.MatchBizQueue != "match.biz.queue" {
		t.Fatalf("MatchBizQueue = %q", mq.MatchBizQueue)
	}
	if mq.MatchServerAllocatorQueue != "match.server-allocator.queue" {
		t.Fatalf("MatchServerAllocatorQueue = %q", mq.MatchServerAllocatorQueue)
	}
	if mq.MatchUpdateRoutingKey != "match.update" {
		t.Fatalf("MatchUpdateRoutingKey = %q", mq.MatchUpdateRoutingKey)
	}
}
