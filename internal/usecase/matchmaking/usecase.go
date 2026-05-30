// Package matchmaking implements matchmaking use case orchestration.
package matchmaking

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

// UseCase 是匹配流程的核心用例，编排票据入队、匹配、确认和结果发布。
type UseCase struct {
	ticketRepo  domainticket.TicketRepository
	configRepo  config.ConfigRepository
	publisher   domainmatch.EventPublisher
	matchmakers []domainmatch.Matchmaker
	mu          sync.Mutex
}

// NewUseCase 创建匹配用例实例。
// matchmakers 是所有注册的匹配器实现，按游戏模式路由。
func NewUseCase(
	ticketRepo domainticket.TicketRepository,
	configRepo config.ConfigRepository,
	publisher domainmatch.EventPublisher,
	matchmakers []domainmatch.Matchmaker,
) *UseCase {
	return &UseCase{
		ticketRepo:  ticketRepo,
		configRepo:  configRepo,
		publisher:   publisher,
		matchmakers: matchmakers,
	}
}

// EnqueueAndMatch 将一张票据加入匹配队列并尝试配对。
// 如果找到对手，执行后续流程（确认或直接发布结果）。
// 如果未找到对手，票据留在池中等待后续匹配。
func (uc *UseCase) EnqueueAndMatch(ctx context.Context, ticket *domainticket.Ticket) error {
	// 1. 获取该游戏模式的匹配配置
	cfg, err := uc.configRepo.GetMatchConfig(ctx, ticket.GameMode)
	if err != nil {
		return fmt.Errorf("获取匹配配置失败 mode=%s: %w", ticket.GameMode, err)
	}
	if cfg == nil {
		return fmt.Errorf("未找到匹配配置 mode=%s", ticket.GameMode)
	}

	// 2. 校验票据人数是否满足队伍要求
	if !ticket.IsFull(cfg) {
		return fmt.Errorf("票据人数不足: 当前=%d, 需要=%d", ticket.TeamSize(), cfg.TeamSize)
	}

	// 3. 标记为匹配中并持久化
	ticket.Status = domainticket.TicketStatusMatching
	ticket.UpdatedAt = time.Now()
	if err := uc.ticketRepo.Save(ctx, ticket); err != nil {
		return fmt.Errorf("保存票据入队失败: %w", err)
	}

	slog.Info("票据入队", "ticket_id", ticket.ID, "lobby_id", ticket.LobbyID, "mode", ticket.GameMode)

	// 4. 获取该模式对应的匹配器
	matchmaker, err := uc.getMatchmaker(ticket.GameMode)
	if err != nil {
		return fmt.Errorf("获取匹配器失败: %w", err)
	}

	// 5. 获取匹配池中同模式的等待中票据
	pool, err := uc.ticketRepo.FindByStatus(ctx, ticket.GameMode, domainticket.TicketStatusMatching)
	if err != nil {
		return fmt.Errorf("查询匹配池失败: %w", err)
	}

	// 6. 加锁防止并发匹配到同一张票据
	uc.mu.Lock()
	defer uc.mu.Unlock()

	// 7. 尝试匹配
	match := matchmaker.FindMatch(ticket, pool)
	if match == nil {
		slog.Info("暂未找到对手，票据留在池中", "ticket_id", ticket.ID)
		return nil
	}

	// 8. 为匹配分配 ID
	match.ID = uc.generateMatchID()

	slog.Info("匹配成功",
		"match_id", match.ID,
		"mode", match.GameMode,
		"ticket_count", len(match.Tickets),
	)

	// 9. 更新参与匹配的票据状态为已匹配
	for _, t := range match.Tickets {
		t.Status = domainticket.TicketStatusMatched
		t.UpdatedAt = time.Now()
		if err := uc.ticketRepo.Save(ctx, t); err != nil {
			return fmt.Errorf("更新票据状态失败 ticket=%s: %w", t.ID, err)
		}
	}

	// 10. 根据配置决定是否需要玩家确认
	if cfg.NeedConfirm {
		return uc.requestConfirm(ctx, match, cfg)
	}

	// 11. 不需要确认，直接完成匹配
	return uc.finalizeMatch(ctx, match)
}

// finalizeMatch 完成匹配流程：发布匹配结果和创建服务器请求。
func (uc *UseCase) finalizeMatch(ctx context.Context, match *domainmatch.Match) error {
	match.Status = domainmatch.MatchStatusConfirmed
	match.UpdatedAt = time.Now()

	if err := uc.publisher.PublishMatchResult(ctx, match); err != nil {
		return fmt.Errorf("发布匹配结果失败: %w", err)
	}

	if err := uc.publisher.PublishServerCreateRequest(ctx, match); err != nil {
		return fmt.Errorf("发布创建服务器请求失败: %w", err)
	}

	// 将票据标记为已确认
	for _, t := range match.Tickets {
		t.Status = domainticket.TicketStatusConfirmed
		t.UpdatedAt = time.Now()
		_ = uc.ticketRepo.Save(ctx, t)
	}

	slog.Info("匹配完成", "match_id", match.ID)
	return nil
}

// requestConfirm 向玩家发起确认请求，并启动超时检测。
func (uc *UseCase) requestConfirm(ctx context.Context, match *domainmatch.Match, cfg *config.MatchConfig) error {
	// 更新票据状态为确认中
	for _, t := range match.Tickets {
		t.Status = domainticket.TicketStatusConfirming
		t.UpdatedAt = time.Now()
		_ = uc.ticketRepo.Save(ctx, t)
	}

	if err := uc.publisher.PublishConfirmRequest(ctx, match); err != nil {
		return fmt.Errorf("发布确认请求失败: %w", err)
	}

	// 启动确认超时 goroutine
	go uc.startConfirmTimeout(match, cfg.ConfirmTimeout)

	slog.Info("等待玩家确认", "match_id", match.ID, "timeout", cfg.ConfirmTimeout)
	return nil
}

// startConfirmTimeout 在超时后检查确认状态，未确认的票据重新入池。
func (uc *UseCase) startConfirmTimeout(match *domainmatch.Match, timeout time.Duration) {
	time.Sleep(timeout)

	// 超时后检查票据状态，仍在确认中的视为超时取消
	for _, t := range match.Tickets {
		// 此处在 goroutine 中无法获取原始 ctx，使用 context.Background()
		current, err := uc.ticketRepo.FindByID(context.Background(), t.ID)
		if err != nil {
			continue
		}
		if current.Status == domainticket.TicketStatusConfirming {
			slog.Warn("确认超时，取消票据", "ticket_id", t.ID, "match_id", match.ID)
			_ = uc.ticketRepo.UpdateStatus(context.Background(), t.ID, domainticket.TicketStatusCancelled)
		}
	}
}

// getMatchmaker 根据游戏模式查找对应的匹配器。
func (uc *UseCase) getMatchmaker(mode config.GameMode) (domainmatch.Matchmaker, error) {
	for _, mm := range uc.matchmakers {
		if mm.Supports(mode) {
			return mm, nil
		}
	}
	return nil, fmt.Errorf("未找到匹配器 mode=%s", mode)
}

// generateMatchID 生成唯一的匹配 ID。
func (uc *UseCase) generateMatchID() string {
	return fmt.Sprintf("match-%d-%d", time.Now().UnixMilli(), time.Now().Nanosecond()%10000)
}
