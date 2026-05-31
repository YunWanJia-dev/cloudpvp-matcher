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
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

const matchLockTTL = 5 * time.Second

// Publisher 聚合匹配用例需要发布的出站事件端口。
type Publisher interface {
	domainticket.QueueEventPublisher
	domainmatch.EventPublisher
}

// Option 配置匹配用例的可选依赖。
type Option func(*UseCase)

// WithLockManager 注入跨实例匹配互斥锁。
func WithLockManager(lockManager domainmatch.LockManager) Option {
	return func(uc *UseCase) {
		uc.lockManager = lockManager
	}
}

// UseCase 是匹配流程的核心用例，负责入队、队列扫描和匹配结果发布。
type UseCase struct {
	ticketRepo  domainticket.Repository
	configRepo  config.ConfigRepository
	publisher   Publisher
	lockManager domainmatch.LockManager
	matchmakers []domainmatch.Matchmaker
}

// NewUseCase 创建匹配用例实例。
func NewUseCase(
	ticketRepo domainticket.Repository,
	configRepo config.ConfigRepository,
	publisher Publisher,
	matchmakers []domainmatch.Matchmaker,
	options ...Option,
) *UseCase {
	uc := &UseCase{
		ticketRepo:  ticketRepo,
		configRepo:  configRepo,
		publisher:   publisher,
		matchmakers: matchmakers,
	}
	for _, option := range options {
		option(uc)
	}
	return uc
}

// Enqueue 校验 lobby 后将其持久化到匹配队列，并发布入队成功事件。
func (uc *UseCase) Enqueue(ctx context.Context, ticket *domainticket.Ticket) error {
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

	if err := uc.ticketRepo.Save(ctx, ticket); err != nil {
		return fmt.Errorf("保存票据入队失败 lobby_id=%s: %w", ticket.LobbyID, err)
	}

	// 入队事件必须在持久化成功后发布，消费者重试时依赖 lobby_id 覆盖保存保证幂等。
	if err := uc.publisher.PublishTicketQueued(ctx, ticket); err != nil {
		return fmt.Errorf("发布入队成功事件失败 lobby_id=%s: %w", ticket.LobbyID, err)
	}

	slog.Info("票据入队", "lobby_id", ticket.LobbyID, "mode", ticket.GameMode, "member_count", ticket.TeamSize())
	return nil
}

// RunMatchOnce 在指定模式队列中尝试形成一场对局。
func (uc *UseCase) RunMatchOnce(ctx context.Context, mode config.GameMode) (bool, error) {
	cfg, err := uc.getMatchConfig(ctx, mode)
	if err != nil {
		return false, err
	}

	matchmaker, err := uc.getMatchmaker(mode)
	if err != nil {
		return false, fmt.Errorf("获取匹配器失败: %w", err)
	}

	if uc.lockManager == nil {
		matched, err := uc.runMatchOnceLocked(ctx, cfg, matchmaker)
		return matched, err
	}

	var matched bool
	lockKey := fmt.Sprintf("matcher:lock:%s", mode)
	err = uc.lockManager.WithLock(ctx, lockKey, matchLockTTL, func(ctx context.Context) error {
		var matchErr error
		matched, matchErr = uc.runMatchOnceLocked(ctx, cfg, matchmaker)
		return matchErr
	})
	if err != nil {
		return false, err
	}
	return matched, nil
}

func (uc *UseCase) runMatchOnceLocked(ctx context.Context, cfg *config.MatchConfig, matchmaker domainmatch.Matchmaker) (bool, error) {
	pool, err := uc.ticketRepo.ListByGameMode(ctx, cfg.GameMode)
	if err != nil {
		return false, fmt.Errorf("查询匹配池失败 mode=%s: %w", cfg.GameMode, err)
	}

	match := matchmaker.FindMatch(pool, cfg)
	if match == nil {
		return false, nil
	}

	now := time.Now()
	match.ID = uc.generateMatchID(now)
	match.UpdatedAt = now

	if err := uc.publishMatch(ctx, match, cfg); err != nil {
		return false, err
	}
	if err := uc.removeMatchedTickets(ctx, match); err != nil {
		return false, err
	}

	slog.Info("匹配成功",
		"match_id", match.ID,
		"mode", match.GameMode,
		"team_count", len(match.Teams),
		"ticket_count", len(match.AllTickets()),
	)
	return true, nil
}

func (uc *UseCase) publishMatch(ctx context.Context, match *domainmatch.Match, cfg *config.MatchConfig) error {
	if cfg.NeedConfirm {
		if err := uc.publisher.PublishConfirmRequest(ctx, match); err != nil {
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

func (uc *UseCase) removeMatchedTickets(ctx context.Context, match *domainmatch.Match) error {
	for _, ticket := range match.AllTickets() {
		if err := uc.ticketRepo.Remove(ctx, ticket.LobbyID); err != nil {
			return fmt.Errorf("移除已匹配票据失败 lobby_id=%s: %w", ticket.LobbyID, err)
		}
	}
	return nil
}

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

	cfg, err := uc.getMatchConfig(ctx, ticket.GameMode)
	if err != nil {
		return nil, err
	}
	if err := validateMembers(ticket, cfg.TeamSize); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (uc *UseCase) getMatchConfig(ctx context.Context, mode config.GameMode) (*config.MatchConfig, error) {
	cfg, err := uc.configRepo.GetMatchConfig(ctx, mode)
	if err != nil {
		return nil, fmt.Errorf("获取匹配配置失败 mode=%s: %w", mode, err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("未找到匹配配置 mode=%s", mode)
	}
	if cfg.TeamSize <= 0 || cfg.TeamCount <= 0 {
		return nil, fmt.Errorf("匹配配置非法 mode=%s team_size=%d team_count=%d", mode, cfg.TeamSize, cfg.TeamCount)
	}
	return cfg, nil
}

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

func (uc *UseCase) getMatchmaker(mode config.GameMode) (domainmatch.Matchmaker, error) {
	for _, matchmaker := range uc.matchmakers {
		if matchmaker.Supports(mode) {
			return matchmaker, nil
		}
	}
	return nil, fmt.Errorf("未找到匹配器 mode=%s", mode)
}

func (uc *UseCase) generateMatchID(now time.Time) string {
	return fmt.Sprintf("match-%d-%d", now.UnixMilli(), now.Nanosecond()%10000)
}
