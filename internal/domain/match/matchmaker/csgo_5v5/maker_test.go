package csgo_5v5

import (
	"context"
	"reflect"
	"testing"
	"time"

	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
)

type testQueueRepository struct {
	buckets map[int][]LobbyQueueEntry
	removed []string
}

// Enqueue 保存测试队列条目。
func (r *testQueueRepository) Enqueue(_ context.Context, entry LobbyQueueEntry) error {
	r.buckets[entry.MemberCount] = append(r.buckets[entry.MemberCount], entry)
	return nil
}

// ListOldestByMemberCount 返回测试人数桶候选。
func (r *testQueueRepository) ListOldestByMemberCount(_ context.Context, memberCount, limit int) ([]LobbyQueueEntry, error) {
	entries := r.buckets[memberCount]
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return append([]LobbyQueueEntry(nil), entries...), nil
}

// RemoveQueuedLobby 删除一个测试队列条目。
func (r *testQueueRepository) RemoveQueuedLobby(_ context.Context, lobbyID string) error {
	return r.RemoveQueuedLobbies(context.Background(), []string{lobbyID})
}

// RemoveQueuedLobbies 记录批量删除的测试队列条目。
func (r *testQueueRepository) RemoveQueuedLobbies(_ context.Context, lobbyIDs []string) error {
	r.removed = append([]string(nil), lobbyIDs...)
	return nil
}

// HasQueuedLobbies 判断测试候选是否仍全部存在。
func (r *testQueueRepository) HasQueuedLobbies(_ context.Context, lobbyIDs []string) (bool, error) {
	for _, lobbyID := range lobbyIDs {
		found := false
		for _, entries := range r.buckets {
			for _, entry := range entries {
				if entry.LobbyID == lobbyID {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// TestFindMatchComposesTwoFivePlayerTeams 校验 3+2 和 4+1 可自然组成 5v5。
func TestFindMatchComposesTwoFivePlayerTeams(t *testing.T) {
	base := time.Now().Add(-time.Minute)
	repo := &testQueueRepository{buckets: map[int][]LobbyQueueEntry{
		1: {{LobbyID: "lobby-4", MemberCount: 1, QueuedAt: base.Add(4 * time.Second)}},
		2: {{LobbyID: "lobby-2", MemberCount: 2, QueuedAt: base.Add(2 * time.Second)}},
		3: {{LobbyID: "lobby-1", MemberCount: 3, QueuedAt: base.Add(time.Second)}},
		4: {{LobbyID: "lobby-3", MemberCount: 4, QueuedAt: base.Add(3 * time.Second)}},
	}}
	matchmaker := NewCSGO5v5Matchmaker(repo)

	match, err := matchmaker.FindMatch(context.Background())
	if err != nil {
		t.Fatalf("FindMatch() error = %v", err)
	}
	if match == nil || match.Status != domainmatchmaking.MatchStatusWaitingForServer || match.Server != nil {
		t.Fatalf("match = %#v", match)
	}
	if len(match.Teams) != 2 {
		t.Fatalf("teams = %#v", match.Teams)
	}
	if !reflect.DeepEqual(match.Teams[0].LobbyIDs, []string{"lobby-1", "lobby-2"}) ||
		!reflect.DeepEqual(match.Teams[1].LobbyIDs, []string{"lobby-3", "lobby-4"}) {
		t.Fatalf("teams = %#v", match.Teams)
	}
}

// TestFindMatchReturnsNilWithoutTenPlayers 校验总人数不足时不产生比赛。
func TestFindMatchReturnsNilWithoutTenPlayers(t *testing.T) {
	repo := &testQueueRepository{buckets: map[int][]LobbyQueueEntry{
		5: {{LobbyID: "lobby-1", MemberCount: 5, QueuedAt: time.Now()}},
	}}
	match, err := NewCSGO5v5Matchmaker(repo).FindMatch(context.Background())
	if err != nil {
		t.Fatalf("FindMatch() error = %v", err)
	}
	if match != nil {
		t.Fatalf("match = %#v", match)
	}
}
