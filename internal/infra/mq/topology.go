package mq

import "fmt"

// DeclareTopology 声明 matcher 使用的 RabbitMQ exchange、queue 和 binding。
func DeclareTopology(rabbitMQ *RabbitMQ) error {
	ch := rabbitMQ.Channel()

	if err := ch.ExchangeDeclare(
		rabbitMQ.ExchangeName(),
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("rabbitmq: declare exchange failed: %w", err)
	}

	queues := map[string]string{
		RequestQueue:      RequestRoutingKey,
		ResultQueue:       ResultRoutingKey,
		ServerCreateQueue: ServerCreateRoutingKey,
		ConfirmQueue:      ConfirmBindingRoutingKey,
	}

	for queue, routingKey := range queues {
		if err := declareQueueBinding(rabbitMQ, queue, routingKey); err != nil {
			return err
		}
	}
	return nil
}

func declareQueueBinding(rabbitMQ *RabbitMQ, queueName, routingKey string) error {
	ch := rabbitMQ.Channel()

	if _, err := ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("rabbitmq: declare queue %s failed: %w", queueName, err)
	}

	if err := ch.QueueBind(queueName, routingKey, rabbitMQ.ExchangeName(), false, nil); err != nil {
		return fmt.Errorf("rabbitmq: bind queue %s to routing key %s failed: %w", queueName, routingKey, err)
	}

	return nil
}
