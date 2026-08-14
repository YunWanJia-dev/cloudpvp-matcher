// Package matchmaking 实现匹配流程的用例编排。
package matchmaking

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainlobby "cloudpvp-matcher/internal/domain/lobby"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
)

// UseCase 负责匹配请求入口编排、模式处理器路由和匹配完成发布。
type UseCase struct {
	mu          sync.RWMutex
	matchmakers map[config.GameMode]domainmatch.Matchmaker
	lobbyRepo   domainlobby.Repository
	lockManager domainlobby.LockManager
	publisher   domainmatchmaking.Publisher
}

// NewUseCase 创建匹配用例实例。
func NewUseCase(lobbyRepo domainlobby.Repository, publisher domainmatchmaking.Publisher, lockManager domainlobby.LockManager) *UseCase {
	return &UseCase{
		matchmakers: make(map[config.GameMode]domainmatch.Matchmaker),
		lobbyRepo:   lobbyRepo,
		lockManager: lockManager,
		publisher:   publisher,
	}
}

// AddMatchmaker 注册或替换指定游戏模式的匹配处理器。
func (uc *UseCase) AddMatchmaker(matchmaker domainmatch.Matchmaker) error {
	if matchmaker == nil {
		return fmt.Errorf("匹配处理器不能为空")
	}
	mode := matchmaker.Mode()
	if mode == "" {
		return fmt.Errorf("game_mode 不能为空")
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.matchmakers[mode] = matchmaker
	return nil
}

// RemoveMatchmaker 移除指定游戏模式的匹配处理器。
func (uc *UseCase) RemoveMatchmaker(mode config.GameMode) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.matchmakers, mode)
}

// SubmitLobby 处理开始匹配请求。
func (uc *UseCase) SubmitLobby(ctx context.Context, lobby *domainlobby.Lobby) error {
	if lobby == nil {
		return fmt.Errorf("lobby 不能为空")
	}
	if strings.TrimSpace(lobby.LobbyID) == "" {
		return fmt.Errorf("lobby_id 不能为空")
	}
	if lobby.GameMode == "" {
		return fmt.Errorf("game_mode 不能为空")
	}

	matchmaker, err := uc.getMatchmaker(lobby.GameMode)
	if err != nil {
		return err
	}

	return uc.withLobbyLock(ctx, []string{lobby.LobbyID}, func(ctx context.Context) error {
		if err := matchmaker.Submit(ctx, lobby); err != nil {
			if publishErr := uc.publisher.PublishLobbyStatus(ctx, lobby.LobbyID, domainmatchmaking.LobbyStatusWaiting, err.Error()); publishErr != nil {
				return fmt.Errorf("提交匹配失败并且发布等待状态失败 lobby_id=%s submit_error=%v: %w", lobby.LobbyID, err, publishErr)
			}
			return err
		}
		if err := uc.lobbyRepo.Save(ctx, lobby); err != nil {
			// 队列写入早于快照保存，失败时必须补偿删除，避免扫描器匹配到无法还原的幽灵 lobby。
			_ = matchmaker.Cancel(ctx, lobby.LobbyID)
			return fmt.Errorf("保存原始 lobby 失败 lobby_id=%s: %w", lobby.LobbyID, err)
		}
		if err := uc.publisher.PublishLobbyStatus(ctx, lobby.LobbyID, domainmatchmaking.LobbyStatusMatching, ""); err != nil {
			return fmt.Errorf("发布 lobby 入队事件失败 lobby_id=%s: %w", lobby.LobbyID, err)
		}
		return nil
	})
}

// CancelLobby 处理取消匹配请求。
func (uc *UseCase) CancelLobby(ctx context.Context, lobbyID string) error {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return fmt.Errorf("lobby_id 不能为空")
	}

	return uc.withLobbyLock(ctx, []string{lobbyID}, func(ctx context.Context) error {
		lobby, err := uc.lobbyRepo.FindByLobbyID(ctx, lobbyID)
		if err != nil {
			return fmt.Errorf("查询原始 lobby 失败 lobby_id=%s: %w", lobbyID, err)
		}
		if lobby == nil {
			return nil
		}

		matchmaker, err := uc.getMatchmaker(lobby.GameMode)
		if err != nil {
			return err
		}
		if err := matchmaker.Cancel(ctx, lobbyID); err != nil {
			return err
		}
		if err := uc.lobbyRepo.Remove(ctx, lobbyID); err != nil {
			return err
		}
		if err := uc.publisher.PublishLobbyStatus(ctx, lobbyID, domainmatchmaking.LobbyStatusWaiting, ""); err != nil {
			return fmt.Errorf("发布 lobby 取消状态失败 lobby_id=%s: %w", lobbyID, err)
		}
		return nil
	})
}

// RunMatchCycle 扫描所有已注册模式，并尽可能形成完整比赛。
func (uc *UseCase) RunMatchCycle(ctx context.Context) (int, error) {
	matchmakers := uc.listMatchmakers()
	matchedCount := 0
	for _, matchmaker := range matchmakers {
		for {
			matched, err := uc.runMatchOnce(ctx, matchmaker)
			if err != nil {
				return matchedCount, err
			}
			if !matched {
				break
			}
			matchedCount++
		}
	}
	return matchedCount, nil
}

// runMatchOnce 为一个模式寻找并完成一场比赛。
func (uc *UseCase) runMatchOnce(ctx context.Context, matchmaker domainmatch.Matchmaker) (bool, error) {
	match, err := matchmaker.FindMatch(ctx)
	if err != nil {
		return false, fmt.Errorf("扫描匹配候选失败 mode=%s: %w", matchmaker.Mode(), err)
	}
	if match == nil {
		return false, nil
	}

	lobbyIDs := match.LobbyIDs()
	if len(lobbyIDs) == 0 {
		return false, fmt.Errorf("匹配候选 lobby_ids 不能为空 mode=%s", matchmaker.Mode())
	}

	matched := false
	err = uc.withLobbyLock(ctx, lobbyIDs, func(ctx context.Context) error {
		// 获取全部 lobby 锁后校验原候选仍在队列，避免无关队列变化导致已锁候选被跳过。
		queued, err := matchmaker.HasQueuedLobbies(ctx, lobbyIDs)
		if err != nil {
			return fmt.Errorf("锁内校验匹配候选失败 mode=%s: %w", matchmaker.Mode(), err)
		}
		if !queued {
			return nil
		}
		if err := uc.completeMatch(ctx, matchmaker, match); err != nil {
			return err
		}
		matched = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return matched, nil
}

// completeMatch 补全队伍玩家，发布 match.create，并从等待队列清理已匹配 lobby。
func (uc *UseCase) completeMatch(ctx context.Context, matchmaker domainmatch.Matchmaker, match *domainmatchmaking.Match) error {
	if match == nil {
		return fmt.Errorf("match 不能为空")
	}
	lobbyIDs := match.LobbyIDs()
	if len(lobbyIDs) == 0 {
		return fmt.Errorf("match lobby_ids 不能为空")
	}

	lobbies := make(map[string]*domainlobby.Lobby, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		lobby, err := uc.lobbyRepo.FindByLobbyID(ctx, lobbyID)
		if err != nil {
			return fmt.Errorf("查询原始 lobby 失败 lobby_id=%s: %w", lobbyID, err)
		}
		if lobby == nil {
			// 无快照的队列条目无法形成完整 Match，清理后允许后续候选继续扫描。
			if err := matchmaker.RemoveMatched(ctx, []string{lobbyID}); err != nil {
				return fmt.Errorf("清理无快照队列条目失败 lobby_id=%s: %w", lobbyID, err)
			}
			return nil
		}
		lobbies[lobbyID] = lobby
	}

	for teamIndex := range match.Teams {
		members := make([]domainmatchmaking.Member, 0)
		for _, lobbyID := range match.Teams[teamIndex].LobbyIDs {
			for _, member := range lobbies[lobbyID].Members {
				members = append(members, domainmatchmaking.Member{PlayerID: member.PlayerID})
			}
		}
		match.Teams[teamIndex].Members = members
	}

	now := time.Now().UTC()
	if match.CreatedAt.IsZero() {
		match.CreatedAt = now
	}
	match.MatchID = generateMatchID(now)
	match.Status = domainmatchmaking.MatchStatusWaitingForServer
	match.Server = nil
	match.UpdatedAt = now

	if err := uc.publisher.PublishMatch(ctx, match); err != nil {
		return fmt.Errorf("发布完整比赛失败 match_id=%s: %w", match.MatchID, err)
	}
	if err := matchmaker.RemoveMatched(ctx, lobbyIDs); err != nil {
		return fmt.Errorf("清理已匹配队列条目失败 match_id=%s: %w", match.MatchID, err)
	}
	if err := uc.lobbyRepo.RemoveMany(ctx, lobbyIDs); err != nil {
		return fmt.Errorf("清理原始 lobby 失败 match_id=%s: %w", match.MatchID, err)
	}

	slog.Info("匹配成功并发布 match.create", "match_id", match.MatchID, "game_mode", match.GameMode, "lobby_count", len(lobbyIDs), "team_count", len(match.Teams))
	return nil
}

// listMatchmakers 返回按游戏模式稳定排序的处理器快照。
func (uc *UseCase) listMatchmakers() []domainmatch.Matchmaker {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	matchmakers := make([]domainmatch.Matchmaker, 0, len(uc.matchmakers))
	for _, matchmaker := range uc.matchmakers {
		matchmakers = append(matchmakers, matchmaker)
	}
	sort.Slice(matchmakers, func(i, j int) bool {
		return matchmakers[i].Mode() < matchmakers[j].Mode()
	})
	return matchmakers
}

// withLobbyLock 在存在锁管理器时按排序后的 lobby ID 加锁。
func (uc *UseCase) withLobbyLock(ctx context.Context, lobbyIDs []string, fn func(context.Context) error) error {
	lobbyIDs = normalizedLobbyIDs(lobbyIDs)
	if uc.lockManager == nil {
		return fn(ctx)
	}
	return uc.lockManager.WithLobbyLock(ctx, lobbyIDs, fn)
}

// normalizedLobbyIDs 去重并排序 lobby ID，保证多锁获取顺序稳定。
func normalizedLobbyIDs(lobbyIDs []string) []string {
	seen := make(map[string]struct{}, len(lobbyIDs))
	result := make([]string, 0, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		lobbyID = strings.TrimSpace(lobbyID)
		if lobbyID == "" {
			continue
		}
		if _, exists := seen[lobbyID]; exists {
			continue
		}
		seen[lobbyID] = struct{}{}
		result = append(result, lobbyID)
	}
	sort.Strings(result)
	return result
}

// generateMatchID 生成无需外部依赖的高熵比赛 ID。
func generateMatchID(now time.Time) string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("match-%d", now.UnixNano())
	}
	return fmt.Sprintf("match-%d-%s", now.UnixMilli(), hex.EncodeToString(random))
}

// getMatchmaker 返回指定游戏模式的匹配处理器。
func (uc *UseCase) getMatchmaker(mode config.GameMode) (domainmatch.Matchmaker, error) {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	matchmaker, ok := uc.matchmakers[mode]
	if !ok || matchmaker == nil {
		return nil, fmt.Errorf("未找到匹配处理器 mode=%s", mode)
	}
	return matchmaker, nil
}
