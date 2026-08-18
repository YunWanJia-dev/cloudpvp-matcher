package mq

import (
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	RequestQueue              = "matcher.lobby.enqueue"
	EnqueueRoutingKey         = "lobby.matchmaking.enqueue"
	CancelQueue               = "matcher.lobby.cancel"
	CancelRoutingKey          = "matchmaking.cancel"
	LobbyQueue                = "lobby.lobby.update"
	LobbyRoutingKey           = "lobby.update"
	MatchBizQueue             = "lobby.match.update"
	MatchServerAllocatorQueue = "allocator.match.create"
	MatchCreateRoutingKey     = "match.create"
	MatchUpdateRoutingKey     = "match.update"
)

// DeclareTopology 声明匹配命令与回传消息所需的 RabbitMQ 拓扑。
func DeclareTopology(connection *amqp.Connection, exchangeName string) error {
	ch, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("创建 RabbitMQ 拓扑 channel 失败: %w", err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 交换器失败 exchange=%s: %w", exchangeName, err)
	}
	slog.Info("RabbitMQ 交换器已声明", "exchange", exchangeName, "type", "topic")

	bindings := []struct {
		queue      string
		routingKey string
	}{
		{queue: RequestQueue, routingKey: EnqueueRoutingKey},
		{queue: CancelQueue, routingKey: CancelRoutingKey},
		{queue: LobbyQueue, routingKey: LobbyRoutingKey},
		{queue: MatchBizQueue, routingKey: MatchCreateRoutingKey},
		{queue: MatchBizQueue, routingKey: MatchUpdateRoutingKey},
		{queue: MatchServerAllocatorQueue, routingKey: MatchCreateRoutingKey},
	}
	for _, binding := range bindings {
		if _, err := ch.QueueDeclare(binding.queue, true, false, false, false, nil); err != nil {
			return fmt.Errorf("声明 RabbitMQ 队列失败 queue=%s: %w", binding.queue, err)
		}
		if err := ch.QueueBind(binding.queue, binding.routingKey, exchangeName, false, nil); err != nil {
			return fmt.Errorf("绑定 RabbitMQ 队列失败 queue=%s routing_key=%s: %w", binding.queue, binding.routingKey, err)
		}
		slog.Info("RabbitMQ 队列已绑定", "exchange", exchangeName, "queue", binding.queue, "routing_key", binding.routingKey)
	}
	return nil
}
