package ticket

import "context"

// QueueEventPublisher 发布票据队列事件。
type QueueEventPublisher interface {
	PublishTicketQueued(ctx context.Context, ticket *Ticket) error
}
