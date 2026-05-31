package mq

import "fmt"

// DeclareTopology 声明匹配服务使用的 RabbitMQ 交换器、队列和绑定。
func DeclareTopology(rabbitMQ *RabbitMQ) error {
	ch := rabbitMQ.Channel()

	if err := ch.ExchangeDeclare(
		rabbitMQ.ExchangeName(),
		"topic",
		true,  // 持久化
		false, // 不自动删除
		false, // 非内部交换器
		false, // 等待服务端响应
		nil,
	); err != nil {
		return fmt.Errorf("rabbitmq: declare exchange failed: %w", err)
	}

	queues := map[string]string{
		RequestQueue:      RequestRoutingKey,
		QueuedQueue:       QueuedRoutingKey,
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
		true,  // 持久化
		false, // 不自动删除
		false, // 非独占队列
		false, // 等待服务端响应
		nil,
	); err != nil {
		return fmt.Errorf("rabbitmq: declare queue %s failed: %w", queueName, err)
	}

	if err := ch.QueueBind(queueName, routingKey, rabbitMQ.ExchangeName(), false, nil); err != nil {
		return fmt.Errorf("rabbitmq: bind queue %s to routing key %s failed: %w", queueName, routingKey, err)
	}

	return nil
}
