package mq

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"cloudpvp-matcher/internal/handler"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer 从 RabbitMQ 队列消费消息并委托给 handler 处理。
type Consumer struct {
	rabbitMQ *RabbitMQ
	handler  handler.MessageHandler
	queue    string
}

var _ handler.Consumer = (*Consumer)(nil)

// NewConsumer 创建一个新的消费者。
func NewConsumer(rabbitMQ *RabbitMQ, queue string, handler handler.MessageHandler) *Consumer {
	return &Consumer{
		rabbitMQ: rabbitMQ,
		queue:    queue,
		handler:  handler,
	}
}

// Run 开始消费消息，该方法会阻塞。
func (c *Consumer) Run(ctx context.Context) error {
	msgs, err := c.consume()
	if err != nil {
		return err
	}

	slog.Info("rabbitmq consumer started", "queue", c.queue)

	for {
		select {
		case <-ctx.Done():
			slog.Info("rabbitmq consumer stopped", "queue", c.queue)
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				slog.Warn("rabbitmq message channel closed", "queue", c.queue)
				// 通道关闭后重连等待
				time.Sleep(time.Second)
				msgs, err = c.consume()
				if err != nil {
					slog.Error("rabbitmq consumer reconnect failed", "queue", c.queue, "error", err)
					time.Sleep(5 * time.Second)
				}
				continue
			}

			// 处理消息，确认前打印部分内容方便排查
			if slog.Default().Enabled(ctx, slog.LevelDebug) {
				slog.Debug("rabbitmq message received", "queue", c.queue, "body", json.RawMessage(msg.Body))
			}

			if err := c.handler(ctx, msg.Body); err != nil {
				slog.Error("rabbitmq handler error", "queue", c.queue, "error", err)
				// Nack 并重新入队
				_ = msg.Nack(false, true)
				continue
			}

			// 处理成功则确认
			_ = msg.Ack(false)
		}
	}
}

func (c *Consumer) consume() (<-chan amqp.Delivery, error) {
	return c.rabbitMQ.Channel().Consume(
		c.queue,
		"",    // consumer tag
		false, // auto-ack: handler success controls Ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
}
