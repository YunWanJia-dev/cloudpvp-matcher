package matchmaking

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"cloudpvp-matcher/internal/domain/config"
	domainlobby "cloudpvp-matcher/internal/domain/lobby"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
)

type testLobbyRepository struct {
	lobbies map[string]*domainlobby.Lobby
}

// Save 保存测试 lobby。
func (r *testLobbyRepository) Save(_ context.Context, lobby *domainlobby.Lobby) error {
	r.lobbies[lobby.LobbyID] = lobby
	return nil
}

// FindByLobbyID 查询测试 lobby。
func (r *testLobbyRepository) FindByLobbyID(_ context.Context, lobbyID string) (*domainlobby.Lobby, error) {
	return r.lobbies[lobbyID], nil
}

// Remove 删除测试 lobby。
func (r *testLobbyRepository) Remove(_ context.Context, lobbyID string) error {
	delete(r.lobbies, lobbyID)
	return nil
}

// RemoveMany 批量删除测试 lobby。
func (r *testLobbyRepository) RemoveMany(_ context.Context, lobbyIDs []string) error {
	for _, lobbyID := range lobbyIDs {
		delete(r.lobbies, lobbyID)
	}
	return nil
}

type testLockManager struct {
	locked [][]string
}

// WithLobbyLock 记录并执行测试临界区。
func (m *testLockManager) WithLobbyLock(ctx context.Context, lobbyIDs []string, fn func(context.Context) error) error {
	m.locked = append(m.locked, append([]string(nil), lobbyIDs...))
	return fn(ctx)
}

type testPublisher struct {
	match       *domainmatchmaking.Match
	lobbyID     string
	lobbyStatus domainmatchmaking.LobbyStatus
	lobbyReason string
}

// PublishLobbyStatus 记录测试状态事件。
func (p *testPublisher) PublishLobbyStatus(_ context.Context, lobbyID string, status domainmatchmaking.LobbyStatus, reason string) error {
	p.lobbyID = lobbyID
	p.lobbyStatus = status
	p.lobbyReason = reason
	return nil
}

// PublishMatch 记录完整比赛。
func (p *testPublisher) PublishMatch(_ context.Context, match *domainmatchmaking.Match) error {
	p.match = match
	return nil
}

type testMatchmaker struct {
	submitErr       error
	canceled        string
	findResults     []*domainmatchmaking.Match
	findCalls       int
	removedLobbyIDs []string
	queued          bool
}

var _ domainmatch.Matchmaker = (*testMatchmaker)(nil)

// Mode 返回测试游戏模式。
func (*testMatchmaker) Mode() config.GameMode {
	return config.GameModeCSGO5v5
}

// Submit 返回配置的测试结果。
func (m *testMatchmaker) Submit(context.Context, *domainlobby.Lobby) error {
	return m.submitErr
}

// Cancel 记录被取消的大厅。
func (m *testMatchmaker) Cancel(_ context.Context, lobbyID string) error {
	m.canceled = lobbyID
	return nil
}

// FindMatch 按顺序返回测试候选。
func (m *testMatchmaker) FindMatch(context.Context) (*domainmatchmaking.Match, error) {
	if m.findCalls >= len(m.findResults) {
		return nil, nil
	}
	match := m.findResults[m.findCalls]
	m.findCalls++
	return match, nil
}

// RemoveMatched 记录清理的匹配大厅。
func (m *testMatchmaker) RemoveMatched(_ context.Context, lobbyIDs []string) error {
	m.removedLobbyIDs = append([]string(nil), lobbyIDs...)
	return nil
}

// HasQueuedLobbies 返回测试候选是否仍在队列。
func (m *testMatchmaker) HasQueuedLobbies(context.Context, []string) (bool, error) {
	return m.queued, nil
}

// TestSubmitAndCancelUseSameLobbyLock 校验开始和取消匹配使用同一 lobby 锁键。
func TestSubmitAndCancelUseSameLobbyLock(t *testing.T) {
	repo := &testLobbyRepository{lobbies: map[string]*domainlobby.Lobby{}}
	publisher := &testPublisher{}
	lockManager := &testLockManager{}
	matchmaker := &testMatchmaker{}
	uc := NewUseCase(repo, publisher, lockManager)
	if err := uc.AddMatchmaker(matchmaker); err != nil {
		t.Fatal(err)
	}
	lobby := &domainlobby.Lobby{LobbyID: "1", GameMode: config.GameModeCSGO5v5, Members: []domainlobby.PlayerInfo{{PlayerID: "p1"}}}

	if err := uc.SubmitLobby(context.Background(), lobby); err != nil {
		t.Fatalf("SubmitLobby() error = %v", err)
	}
	if err := uc.CancelLobby(context.Background(), "1"); err != nil {
		t.Fatalf("CancelLobby() error = %v", err)
	}
	if len(lockManager.locked) != 2 || !reflect.DeepEqual(lockManager.locked[0], []string{"1"}) || !reflect.DeepEqual(lockManager.locked[1], []string{"1"}) {
		t.Fatalf("locked = %#v", lockManager.locked)
	}
	if publisher.lobbyStatus != domainmatchmaking.LobbyStatusWaiting || matchmaker.canceled != "1" {
		t.Fatalf("status=%q canceled=%q", publisher.lobbyStatus, matchmaker.canceled)
	}
}

// TestSubmitFailurePublishesWaiting 校验入队失败会在同一锁内回传 WAITING 和原因。
func TestSubmitFailurePublishesWaiting(t *testing.T) {
	repo := &testLobbyRepository{lobbies: map[string]*domainlobby.Lobby{}}
	publisher := &testPublisher{}
	uc := NewUseCase(repo, publisher, &testLockManager{})
	if err := uc.AddMatchmaker(&testMatchmaker{submitErr: fmt.Errorf("invalid lobby")}); err != nil {
		t.Fatal(err)
	}

	err := uc.SubmitLobby(context.Background(), &domainlobby.Lobby{LobbyID: "1", GameMode: config.GameModeCSGO5v5})
	if err == nil {
		t.Fatal("SubmitLobby() error = nil")
	}
	if publisher.lobbyStatus != domainmatchmaking.LobbyStatusWaiting || publisher.lobbyReason != "invalid lobby" {
		t.Fatalf("status=%q reason=%q", publisher.lobbyStatus, publisher.lobbyReason)
	}
}

// TestRunMatchCyclePublishesCompleteWaitingForServerMatch 校验自然扫描补全玩家并发布完整比赛。
func TestRunMatchCyclePublishesCompleteWaitingForServerMatch(t *testing.T) {
	lobby1 := testLobby("1", 5)
	lobby2 := testLobby("2", 5)
	repo := &testLobbyRepository{lobbies: map[string]*domainlobby.Lobby{"1": lobby1, "2": lobby2}}
	publisher := &testPublisher{}
	lockManager := &testLockManager{}
	candidate := &domainmatchmaking.Match{
		GameMode: config.GameModeCSGO5v5,
		Teams: []domainmatchmaking.Team{
			{LobbyIDs: []string{"1"}},
			{LobbyIDs: []string{"2"}},
		},
	}
	matchmaker := &testMatchmaker{findResults: []*domainmatchmaking.Match{candidate}, queued: true}
	uc := NewUseCase(repo, publisher, lockManager)
	if err := uc.AddMatchmaker(matchmaker); err != nil {
		t.Fatal(err)
	}

	matchedCount, err := uc.RunMatchCycle(context.Background())
	if err != nil {
		t.Fatalf("RunMatchCycle() error = %v", err)
	}
	if matchedCount != 1 || publisher.match == nil {
		t.Fatalf("matchedCount=%d match=%#v", matchedCount, publisher.match)
	}
	if publisher.match.MatchID == "" || publisher.match.Status != domainmatchmaking.MatchStatusWaitingForServer || publisher.match.Server != nil {
		t.Fatalf("match = %#v", publisher.match)
	}
	if len(publisher.match.Teams) != 2 || len(publisher.match.Teams[0].Members) != 5 || len(publisher.match.Teams[1].Members) != 5 {
		t.Fatalf("teams = %#v", publisher.match.Teams)
	}
	if !reflect.DeepEqual(matchmaker.removedLobbyIDs, []string{"1", "2"}) || len(repo.lobbies) != 0 {
		t.Fatalf("removed=%v remaining=%v", matchmaker.removedLobbyIDs, repo.lobbies)
	}
	if len(lockManager.locked) != 1 || !reflect.DeepEqual(lockManager.locked[0], []string{"1", "2"}) {
		t.Fatalf("locked = %#v", lockManager.locked)
	}
}

// testLobby 创建指定人数的测试大厅。
func testLobby(lobbyID string, memberCount int) *domainlobby.Lobby {
	members := make([]domainlobby.PlayerInfo, memberCount)
	for index := range members {
		members[index] = domainlobby.PlayerInfo{PlayerID: fmt.Sprintf("%s-player-%d", lobbyID, index+1)}
	}
	return &domainlobby.Lobby{LobbyID: lobbyID, GameMode: config.GameModeCSGO5v5, Members: members}
}
