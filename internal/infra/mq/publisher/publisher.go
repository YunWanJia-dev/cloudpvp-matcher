package publisher

import (
	"cloudpvp-matcher/internal/infra/mq"
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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

// publish 将出站消息序列化并发布到指定路由键。
func (p *Publisher) publish(ctx context.Context, routingKey string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化发布消息失败: %w", err)
	}
	return p.ch.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
}
