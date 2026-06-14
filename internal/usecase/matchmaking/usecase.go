// Package matchmaking 实现匹配流程的用例编排。
package matchmaking

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainmatchmaking "cloudpvp-matcher/internal/domain/matchmaking"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

// MatchCycleResult 描述一轮匹配扫描的结果。
type MatchCycleResult struct {
	MatchedCount int
	ScannedModes []config.GameMode
}

// Option 配置匹配用例的可选依赖。
type Option func(*UseCase)

// WithLockManager 注入跨实例匹配互斥锁。
func WithLockManager(lockManager domainmatch.LockManager) Option {
	return func(uc *UseCase) {
		uc.lockManager = lockManager
	}
}

// UseCase 负责编排入队、取消和匹配扫描流程。
type UseCase struct {
	ticketRepo   domainticket.Repository
	matchConfigs map[config.GameMode]*config.MatchConfig
	matchModes   []config.GameMode
	publisher    domainmatchmaking.Publisher
	lockManager  domainmatch.LockManager
	matchmakers  []domainmatch.Matchmaker
}

// NewUseCase 创建匹配用例实例。
func NewUseCase(
	ticketRepo domainticket.Repository,
	matchConfigs []*config.MatchConfig,
	publisher domainmatchmaking.Publisher,
	matchmakers []domainmatch.Matchmaker,
	options ...Option,
) *UseCase {
	uc := &UseCase{
		ticketRepo:   ticketRepo,
		matchConfigs: make(map[config.GameMode]*config.MatchConfig, len(matchConfigs)),
		matchModes:   make([]config.GameMode, 0, len(matchConfigs)),
		publisher:    publisher,
		matchmakers:  matchmakers,
	}
	for _, cfg := range matchConfigs {
		if cfg == nil {
			continue
		}
		if _, exists := uc.matchConfigs[cfg.GameMode]; !exists {
			uc.matchModes = append(uc.matchModes, cfg.GameMode)
		}
		uc.matchConfigs[cfg.GameMode] = cfg
	}
	for _, option := range options {
		option(uc)
	}
	return uc
}

// SubmitTicket 将 lobby 票据提交到匹配队列。
func (uc *UseCase) SubmitTicket(ctx context.Context, ticket *domainticket.Ticket) error {
	cfg, err := uc.validateQueueTicket(ctx, ticket)
	if err != nil {
		return err
	}
	if _, err := uc.getMatchmaker(cfg.GameMode); err != nil {
		return fmt.Errorf("获取匹配器失败: %w", err)
	}

	if err := uc.prepareQueuedTicket(ctx, ticket); err != nil {
		return err
	}

	if err := uc.ticketRepo.SaveTicket(ctx, ticket); err != nil {
		return fmt.Errorf("保存票据入队失败 lobby_id=%s: %w", ticket.LobbyID, err)
	}

	if err := uc.publisher.PublishTicketQueued(ctx, ticket); err != nil {
		return fmt.Errorf("发布入队成功事件失败 lobby_id=%s: %w", ticket.LobbyID, err)
	}

	slog.Info("票据入队", "lobby_id", ticket.LobbyID, "mode", ticket.GameMode, "member_count", ticket.TeamSize())
	return nil
}

// CancelTicket 按 lobby ID 幂等取消匹配队列中的票据。
func (uc *UseCase) CancelTicket(ctx context.Context, lobbyID string) error {
	lobbyID = strings.TrimSpace(lobbyID)
	if lobbyID == "" {
		return fmt.Errorf("lobby_id 不能为空")
	}
	if err := uc.ticketRepo.Remove(ctx, lobbyID); err != nil {
		return fmt.Errorf("取消匹配失败 lobby_id=%s: %w", lobbyID, err)
	}
	slog.Info("票据取消匹配", "lobby_id", lobbyID)
	return nil
}

// RunMatchCycle 执行一轮所有已配置模式的匹配扫描。
func (uc *UseCase) RunMatchCycle(ctx context.Context) (MatchCycleResult, error) {
	var result MatchCycleResult

	for _, mode := range uc.matchModes {
		cfg, err := uc.getMatchConfig(mode)
		if err != nil {
			return result, err
		}
		matchmaker, err := uc.getMatchmaker(mode)
		if err != nil {
			return result, fmt.Errorf("获取匹配器失败: %w", err)
		}

		result.ScannedModes = append(result.ScannedModes, mode)
		for {
			matched, err := uc.runMatchAttempt(ctx, cfg, matchmaker)
			if err != nil {
				return result, err
			}
			if !matched {
				break
			}
			result.MatchedCount++
		}
	}

	return result, nil
}

// runMatchAttempt 在可选分布式锁保护下尝试完成一次匹配。
func (uc *UseCase) runMatchAttempt(ctx context.Context, cfg *config.MatchConfig, matchmaker domainmatch.Matchmaker) (bool, error) {
	pool, err := uc.ticketRepo.ListByGameMode(ctx, cfg.GameMode)
	if err != nil {
		return false, fmt.Errorf("查询匹配池失败 mode=%s: %w", cfg.GameMode, err)
	}

	match := matchmaker.FindMatch(pool, cfg)
	if match == nil {
		return false, nil
	}

	if uc.lockManager == nil {
		if err := uc.completeMatch(ctx, match, cfg); err != nil {
			return false, err
		}
		return true, nil
	}

	var matched bool
	lobbyIDs := matchLobbyIDs(match)
	err = uc.lockManager.WithLobbyLock(ctx, lobbyIDs, func(ctx context.Context) error {
		lockedMatch, err := uc.reloadLockedMatch(ctx, lobbyIDs, cfg, matchmaker)
		if err != nil {
			return err
		}
		if lockedMatch == nil {
			return nil
		}
		if err := uc.completeMatch(ctx, lockedMatch, cfg); err != nil {
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

// reloadLockedMatch 在持有 lobby 锁后重新读取票据并确认匹配仍然成立。
func (uc *UseCase) reloadLockedMatch(ctx context.Context, lobbyIDs []string, cfg *config.MatchConfig, matchmaker domainmatch.Matchmaker) (*domainmatch.Match, error) {
	pool := make([]*domainticket.Ticket, 0, len(lobbyIDs))
	for _, lobbyID := range lobbyIDs {
		ticket, err := uc.ticketRepo.FindByLobbyID(ctx, lobbyID)
		if err != nil {
			return nil, fmt.Errorf("查询锁定票据失败 lobby_id=%s: %w", lobbyID, err)
		}
		if ticket != nil {
			pool = append(pool, ticket)
		}
	}
	return matchmaker.FindMatch(pool, cfg), nil
}

// completeMatch 完成匹配发布和已匹配票据移除。
func (uc *UseCase) completeMatch(ctx context.Context, match *domainmatch.Match, cfg *config.MatchConfig) error {
	now := time.Now()
	match.ID = uc.generateMatchID(now)
	match.UpdatedAt = now

	if err := uc.publishMatch(ctx, match, cfg); err != nil {
		return err
	}
	if err := uc.removeMatchedTickets(ctx, match); err != nil {
		return err
	}

	slog.Info("匹配成功",
		"match_id", match.ID,
		"mode", match.GameMode,
		"team_count", len(match.Teams),
		"ticket_count", len(match.AllTickets()),
	)
	return nil
}

// matchLobbyIDs 返回候选匹配中涉及的全部 lobby ID。
func matchLobbyIDs(match *domainmatch.Match) []string {
	if match == nil {
		return nil
	}
	tickets := match.AllTickets()
	lobbyIDs := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		if ticket != nil {
			lobbyIDs = append(lobbyIDs, ticket.LobbyID)
		}
	}
	return lobbyIDs
}

// publishMatch 根据匹配配置发布确认请求或直接发布匹配结果。
func (uc *UseCase) publishMatch(ctx context.Context, match *domainmatch.Match, cfg *config.MatchConfig) error {
	if cfg.NeedConfirm {
		if err := uc.publisher.PublishConfirmRequest(ctx, match, cfg.ConfirmTimeout); err != nil {
			return fmt.Errorf("发布确认请求失败 match_id=%s: %w", match.ID, err)
		}
		return nil
	}

	match.Status = domainmatch.MatchStatusConfirmed
	if err := uc.publisher.PublishMatchResult(ctx, match); err != nil {
		return fmt.Errorf("发布匹配结果失败 match_id=%s: %w", match.ID, err)
	}
	if err := uc.publisher.PublishServerCreateRequest(ctx, match); err != nil {
		return fmt.Errorf("发布创建服务器请求失败 match_id=%s: %w", match.ID, err)
	}
	return nil
}

// removeMatchedTickets 从匹配池中移除已经组成对局的票据。
func (uc *UseCase) removeMatchedTickets(ctx context.Context, match *domainmatch.Match) error {
	for _, ticket := range match.AllTickets() {
		if err := uc.ticketRepo.Remove(ctx, ticket.LobbyID); err != nil {
			return fmt.Errorf("移除已匹配票据失败 lobby_id=%s: %w", ticket.LobbyID, err)
		}
	}
	return nil
}

// validateQueueTicket 校验入队票据并返回对应模式配置。
func (uc *UseCase) validateQueueTicket(ctx context.Context, ticket *domainticket.Ticket) (*config.MatchConfig, error) {
	if ticket == nil {
		return nil, fmt.Errorf("票据不能为空")
	}
	if strings.TrimSpace(ticket.LobbyID) == "" {
		return nil, fmt.Errorf("lobby_id 不能为空")
	}
	if ticket.GameMode == "" {
		return nil, fmt.Errorf("game_mode 不能为空")
	}

	cfg, err := uc.getMatchConfig(ticket.GameMode)
	if err != nil {
		return nil, err
	}
	if err := validateMembers(ticket, cfg.TeamSize); err != nil {
		return nil, err
	}
	return cfg, nil
}

// getMatchConfig 从启动时注入的配置快照中读取指定模式配置。
func (uc *UseCase) getMatchConfig(mode config.GameMode) (*config.MatchConfig, error) {
	cfg, ok := uc.matchConfigs[mode]
	if !ok || cfg == nil {
		return nil, fmt.Errorf("未找到匹配配置 mode=%s", mode)
	}
	if cfg.TeamSize <= 0 || cfg.TeamCount <= 0 {
		return nil, fmt.Errorf("匹配配置非法 mode=%s team_size=%d team_count=%d", mode, cfg.TeamSize, cfg.TeamCount)
	}
	if cfg.NeedConfirm && cfg.ConfirmTimeout <= 0 {
		return nil, fmt.Errorf("匹配配置非法 mode=%s confirm_timeout=%s", mode, cfg.ConfirmTimeout)
	}
	return cfg, nil
}

// validateMembers 校验票据成员数量和玩家 ID 唯一性。
func validateMembers(ticket *domainticket.Ticket, maxTeamSize int) error {
	memberCount := ticket.TeamSize()
	if memberCount == 0 {
		return fmt.Errorf("票据人数不能为空 lobby_id=%s", ticket.LobbyID)
	}
	if memberCount > maxTeamSize {
		return fmt.Errorf("票据人数超过队伍上限 lobby_id=%s current=%d max=%d", ticket.LobbyID, memberCount, maxTeamSize)
	}

	seenPlayerIDs := make(map[string]struct{}, memberCount)
	for _, member := range ticket.Members {
		playerID := strings.TrimSpace(member.PlayerID)
		if playerID == "" {
			return fmt.Errorf("player_id 不能为空 lobby_id=%s", ticket.LobbyID)
		}
		if _, exists := seenPlayerIDs[playerID]; exists {
			return fmt.Errorf("重复 player_id lobby_id=%s player_id=%s", ticket.LobbyID, playerID)
		}
		seenPlayerIDs[playerID] = struct{}{}
	}
	return nil
}

// prepareQueuedTicket 处理重复入队时的创建时间保留和更新时间刷新。
func (uc *UseCase) prepareQueuedTicket(ctx context.Context, ticket *domainticket.Ticket) error {
	existing, err := uc.ticketRepo.FindByLobbyID(ctx, ticket.LobbyID)
	if err != nil {
		return fmt.Errorf("查询已有票据失败 lobby_id=%s: %w", ticket.LobbyID, err)
	}

	now := time.Now()
	if existing != nil && !existing.CreatedAt.IsZero() {
		ticket.CreatedAt = existing.CreatedAt
	} else if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = now
	}
	ticket.UpdatedAt = now
	return nil
}

// getMatchmaker 查找支持指定游戏模式的领域匹配器。
func (uc *UseCase) getMatchmaker(mode config.GameMode) (domainmatch.Matchmaker, error) {
	for _, matchmaker := range uc.matchmakers {
		if matchmaker.Supports(mode) {
			return matchmaker, nil
		}
	}
	return nil, fmt.Errorf("未找到匹配器 mode=%s", mode)
}

// generateMatchID 生成当前进程内足够区分的匹配 ID。
func (uc *UseCase) generateMatchID(now time.Time) string {
	return fmt.Sprintf("match-%d-%d", now.UnixMilli(), now.Nanosecond()%10000)
}
