package mq

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig 是 Apollo 下发的 RabbitMQ 连接和消费队列配置。
type RabbitMQConfig struct {
	URL          string `json:"url" mapstructure:"url"`
	ExchangeName string `json:"exchange_name" mapstructure:"exchange_name"`
}

// NewRabbitMQConnection 建立 RabbitMQ 连接。
func NewRabbitMQConnection(config *RabbitMQConfig) *amqp.Connection {
	instance, err := amqp.Dial(config.URL)
	if err != nil {
		log.Panicf("failed to connect to RabbitMQ: %s", err)
	}
	return instance
}
