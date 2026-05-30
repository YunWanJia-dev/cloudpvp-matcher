// Package rabbitmq 封装 AMQP 连接、消费者和发布者。
package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Config RabbitMQ 连接配置。
type Config struct {
	URL          string `yaml:"url"`
	ExchangeName string `yaml:"exchange_name"`
}

// Connection 管理 AMQP 连接和通道。
type Connection struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	cfg  Config
}

// NewConnection 建立与 RabbitMQ 的连接并声明主题交换器。
func NewConnection(cfg Config) (*Connection, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: create channel failed: %w", err)
	}

	// 声明主题交换器
	err = ch.ExchangeDeclare(
		cfg.ExchangeName,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: declare exchange failed: %w", err)
	}

	return &Connection{conn: conn, ch: ch, cfg: cfg}, nil
}

// Channel 返回当前的 AMQP 通道。
func (c *Connection) Channel() *amqp.Channel {
	return c.ch
}

// ExchangeName 返回配置的交换器名称。
func (c *Connection) ExchangeName() string {
	return c.cfg.ExchangeName
}

// Close 关闭通道和连接。
func (c *Connection) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// DeclareQueue 声明一个持久化队列并绑定到指定的路由键。
func (c *Connection) DeclareQueue(queueName, routingKey string) error {
	_, err := c.ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: declare queue %s failed: %w", queueName, err)
	}

	err = c.ch.QueueBind(queueName, routingKey, c.cfg.ExchangeName, false, nil)
	if err != nil {
		return fmt.Errorf("rabbitmq: bind queue %s to routing key %s failed: %w", queueName, routingKey, err)
	}

	return nil
}

// Publish 向指定路由键发布消息。
func (c *Connection) Publish(routingKey string, body []byte) error {
	return c.ch.Publish(
		c.cfg.ExchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
}

// Consume 从指定队列消费消息，返回消息通道。
func (c *Connection) Consume(queueName string) (<-chan amqp.Delivery, error) {
	return c.ch.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack — 需手动确认
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
}
