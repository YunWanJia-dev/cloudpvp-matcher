// Package matchmaking 实现匹配流程的用例编排。
package matchmaking

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
	if err := matchmaker.Submit(ctx, lobby); err != nil {
		publishErr := uc.lockManager.WithLobbyLock(ctx, []string{lobby.LobbyID}, func(ctx context.Context) error {
			return uc.publisher.PublishKickedQueue(ctx, lobby.LobbyID, err.Error())
		})
		if publishErr != nil {
			return fmt.Errorf("提交匹配失败并且发布出队事件失败 lobby_id=%s submit_error=%v: %w", lobby.LobbyID, err, publishErr)
		}
		return err
	}
	if err := uc.lobbyRepo.Save(ctx, lobby); err != nil {
		return fmt.Errorf("保存原始 lobby 失败 lobby_id=%s: %w", lobby.LobbyID, err)
	}
	if err := uc.publisher.PublishInQueue(ctx, lobby.LobbyID); err != nil {
		return fmt.Errorf("发布 lobby 入队事件失败 lobby_id=%s: %w", lobby.LobbyID, err)
	}
	return nil
}

// CancelLobby 处理取消匹配请求。
func (uc *UseCase) CancelLobby(ctx context.Context, lobbyID string) error {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return fmt.Errorf("lobby_id 不能为空")
	}

	return uc.lockManager.WithLobbyLock(ctx, []string{lobbyID}, func(ctx context.Context) error {
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
		return nil
	})
}

// HandleMatchResult 处理 matchmaker 产出的最终匹配结果。
func (uc *UseCase) HandleMatchResult(ctx context.Context, match *domainmatchmaking.MatchResult) error {
	if match == nil {
		return fmt.Errorf("match 不能为空")
	}
	if len(match.LobbyIDs) == 0 {
		return fmt.Errorf("match lobby_ids 不能为空")
	}

	return uc.lockManager.WithLobbyLock(ctx, match.LobbyIDs, func(ctx context.Context) error {
		lobbies := make([]*domainlobby.Lobby, 0, len(match.LobbyIDs))
		for _, lobbyID := range match.LobbyIDs {
			lobby, err := uc.lobbyRepo.FindByLobbyID(ctx, lobbyID)

			if err != nil && lobby != nil {
				lobbies = append(lobbies, lobby)
			}
		}

		if len(lobbies) != len(match.LobbyIDs) {
			for _, lobby := range lobbies {
				if err := uc.SubmitLobby(ctx, lobby); err != nil {
					_ = uc.lobbyRepo.Remove(ctx, lobby.LobbyID)
				}
			}
			return nil
		}

		if err := uc.publisher.PublishMatchResult(ctx, match); err != nil {
			return err
		}
		if err := uc.lobbyRepo.RemoveMany(ctx, match.LobbyIDs); err != nil {
			return fmt.Errorf("清理原始 lobby 失败 match_id=%s: %w", match.MatchID, err)
		}
		return nil
	})
}

// HandleConfirmRequest 处理 matchmaker 产出的确认请求。
func (uc *UseCase) HandleConfirmRequest(ctx context.Context, lobbyIDs []string) error {
	if len(lobbyIDs) == 0 {
		return fmt.Errorf("确认 lobby 列表不能为空")
	}
	return uc.lockManager.WithLobbyLock(ctx, lobbyIDs, func(ctx context.Context) error {
		for _, lobbyID := range lobbyIDs {
			lobby, err := uc.lobbyRepo.FindByLobbyID(ctx, lobbyID)
			if err != nil {
				return fmt.Errorf("查询原始 lobby 失败 lobby_id=%s: %w", lobbyID, err)
			}
			if lobby == nil {
				return nil
			}
		}
		return uc.publisher.PublishConfirmRequest(ctx, lobbyIDs)
	})
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
