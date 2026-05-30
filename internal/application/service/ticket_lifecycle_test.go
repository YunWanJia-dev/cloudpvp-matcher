package service_test

import (
	"context"
	"testing"
	"time"

	appsvc "cloudpvp-matcher/internal/application/service"
	"cloudpvp-matcher/internal/application/service/testutil"
	"cloudpvp-matcher/internal/domain/valueobject"
)

func TestTicketLifecycle_CancelTicket(t *testing.T) {
	repo := testutil.NewMockTicketRepository()
	lifecycle := appsvc.NewTicketLifecycle(repo)
	ctx := context.Background()

	// 先保存一张票据
	ticket := newTestTicket("t1", "lobby1")
	_ = repo.Save(ctx, ticket)

	err := lifecycle.CancelTicket(ctx, "t1")
	if err != nil {
		t.Fatalf("取消票据不应失败: %v", err)
	}

	saved, _ := repo.FindByID(ctx, "t1")
	if saved.Status != valueobject.TicketStatusCancelled {
		t.Errorf("票据状态应为 Cancelled，实际 %s", saved.Status)
	}
}

func TestTicketLifecycle_CancelTicket_NotFound(t *testing.T) {
	repo := testutil.NewMockTicketRepository()
	lifecycle := appsvc.NewTicketLifecycle(repo)
	ctx := context.Background()

	err := lifecycle.CancelTicket(ctx, "nonexistent")
	if err == nil {
		t.Error("取消不存在的票据应返回错误")
	}
}

func TestTicketLifecycle_CleanupExpiredTickets(t *testing.T) {
	repo := testutil.NewMockTicketRepository()
	lifecycle := appsvc.NewTicketLifecycle(repo)
	ctx := context.Background()

	// 创建新的匹配中票据
	active := newTestTicket("t1", "lobby1")
	active.CreatedAt = time.Now()
	_ = repo.Save(ctx, active)

	// 创建过期的匹配中票据（2分钟前创建，使用 sub-minute 过期检查）
	expired := newTestTicket("t2", "lobby2")
	expired.CreatedAt = time.Now().Add(-2 * time.Minute)
	_ = repo.Save(ctx, expired)

	// 清理超过 1 分钟的票据（注意：传入的 maxAge 是从 CreatedAt 算起）
	modes := []valueobject.GameMode{valueobject.GameModeCSGO5v5}
	cleaned, err := lifecycle.CleanupExpiredTickets(ctx, modes, 1*time.Minute)
	if err != nil {
		t.Fatalf("清理过期票据失败: %v", err)
	}

	if cleaned != 1 {
		t.Errorf("应清理1张票据，实际 %d 张", cleaned)
	}

	// 验证过期票据已超时
	saved, _ := repo.FindByID(ctx, "t2")
	if saved == nil || saved.Status != valueobject.TicketStatusTimedOut {
		t.Errorf("过期票据状态应为 TimedOut，实际 %v", saved)
	}

	// 验证活跃票据未受影响
	savedActive, _ := repo.FindByID(ctx, "t1")
	if savedActive == nil || savedActive.Status != valueobject.TicketStatusMatching {
		t.Errorf("活跃票据不应被清理")
	}
}
