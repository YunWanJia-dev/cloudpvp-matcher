package publisher

import (
	"cloudpvp-matcher/internal/infra/mq"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher 基于 RabbitMQ 实现 matchmaking 强类型发布端口。
type Publisher struct {
	ch       *amqp.Channel
	exchange string
}

// NewPublisher 创建启用 broker confirm 的 RabbitMQ 强类型发布器。
func NewPublisher(ch *amqp.Channel, cfg *mq.RabbitMQConfig) (*Publisher, error) {
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("启用 RabbitMQ publisher confirm 失败: %w", err)
	}
	return &Publisher{
		ch:       ch,
		exchange: cfg.ExchangeName,
	}, nil
}

// publish 将出站消息序列化并发布到指定路由键。
func (p *Publisher) publish(ctx context.Context, routingKey string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化发布消息失败: %w", err)
	}
	confirmation, err := p.ch.PublishWithDeferredConfirmWithContext(
		ctx,
		p.exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("提交 RabbitMQ 消息失败 routing_key=%s: %w", routingKey, err)
	}
	acknowledged, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("等待 RabbitMQ 发布确认失败 routing_key=%s: %w", routingKey, err)
	}
	if !acknowledged {
		return fmt.Errorf("RabbitMQ broker 拒绝消息 routing_key=%s", routingKey)
	}
	slog.Info("RabbitMQ 消息发布已确认", "exchange", p.exchange, "routing_key", routingKey, "body_bytes", len(body))
	return nil
}
