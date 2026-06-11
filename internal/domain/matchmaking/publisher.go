// Package matchmaking 定义匹配流程相关的领域端口。
package matchmaking

import (
	"context"
	"time"

	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
)

// Publisher 定义匹配流程需要的强类型出站发布端口。
type Publisher interface {
	PublishTicketQueued(ctx context.Context, ticket *domainticket.Ticket) error
	PublishConfirmRequest(ctx context.Context, match *domainmatch.Match, timeout time.Duration) error
	PublishMatchResult(ctx context.Context, match *domainmatch.Match) error
	PublishServerCreateRequest(ctx context.Context, match *domainmatch.Match) error
}
