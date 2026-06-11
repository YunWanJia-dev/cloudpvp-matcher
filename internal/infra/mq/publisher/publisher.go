package publisher

import (
	"cloudpvp-matcher/internal/infra/mq"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ticketQueuedRoutingKey   = "matchmaking.queued"
	matchResultRoutingKey    = "match.result"
	serverCreateRoutingKey   = "server.create"
	confirmRequestRoutingKey = "match.confirm.request"
)

// Publisher 基于 RabbitMQ 实现 matchmaking 强类型发布端口。
type Publisher struct {
	ch       *amqp.Channel
	exchange string
}

// NewPublisher 创建基于 RabbitMQ channel 的强类型发布器。
func NewPublisher(ch *amqp.Channel, cfg *mq.RabbitMQConfig) *Publisher {
	return &Publisher{
		ch:       ch,
		exchange: cfg.ExchangeName,
	}
}
