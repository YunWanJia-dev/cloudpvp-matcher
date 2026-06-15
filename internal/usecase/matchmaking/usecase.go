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
	publisher   domainmatchmaking.Publisher
}

// NewUseCase 创建匹配用例实例。
func NewUseCase(lobbyRepo domainlobby.Repository, publisher domainmatchmaking.Publisher) *UseCase {
	return &UseCase{
		matchmakers: make(map[config.GameMode]domainmatch.Matchmaker),
		lobbyRepo:   lobbyRepo,
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
		_ = uc.publisher.PublishKickedQueue(ctx, lobby.LobbyID, err.Error())
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
}

// HandleMatchResult 处理 matchmaker 产出的最终匹配结果。
func (uc *UseCase) HandleMatchResult(ctx context.Context, match *domainmatchmaking.MatchResult) error {
	if match == nil {
		return fmt.Errorf("match 不能为空")
	}

	if err := uc.publisher.PublishMatchResult(ctx, match); err != nil {
		return err
	}

	if err := uc.lobbyRepo.RemoveMany(ctx, match.LobbyIDs); err != nil {
		return fmt.Errorf("清理原始 lobby 失败 match_id=%s: %w", match.MatchID, err)
	}
	return nil
}

// HandleConfirmRequest 处理 matchmaker 产出的确认请求。
func (uc *UseCase) HandleConfirmRequest(ctx context.Context, lobbyIDs []string) error {
	if len(lobbyIDs) == 0 {
		return fmt.Errorf("确认 lobby 列表不能为空")
	}
	return uc.publisher.PublishConfirmRequest(ctx, lobbyIDs)
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
