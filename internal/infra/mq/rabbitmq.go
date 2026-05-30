// Package mq provides RabbitMQ infrastructure implementations.
package mq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Config RabbitMQ 连接配置。
type Config struct {
	URL          string `yaml:"url"`
	ExchangeName string `yaml:"exchange_name"`
}

// RabbitMQ 管理 AMQP 连接和通道。
type RabbitMQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	cfg  Config
}

// NewRabbitMQ 建立与 RabbitMQ 的连接。
func NewRabbitMQ(cfg Config) (*RabbitMQ, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: create channel failed: %w", err)
	}

	return &RabbitMQ{conn: conn, ch: ch, cfg: cfg}, nil
}

// Channel 返回当前的 AMQP 通道。
func (r *RabbitMQ) Channel() *amqp.Channel {
	return r.ch
}

// ExchangeName 返回配置的交换器名称。
func (r *RabbitMQ) ExchangeName() string {
	return r.cfg.ExchangeName
}

// Close 关闭通道和连接。
func (r *RabbitMQ) Close() {
	if r.ch != nil {
		r.ch.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}
