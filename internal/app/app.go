// Package app 负责装配并运行匹配服务。
package app

import (
	domainmatchmaker "cloudpvp-matcher/internal/domain/match/matchmaker/csgo_5v5"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	domainlobby "cloudpvp-matcher/internal/domain/lobby"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	"cloudpvp-matcher/internal/infra/asynclock"
	"cloudpvp-matcher/internal/infra/cache"
	"cloudpvp-matcher/internal/infra/cache/repository"
	localconfig "cloudpvp-matcher/internal/infra/config"
	"cloudpvp-matcher/internal/infra/mq"
	"cloudpvp-matcher/internal/infra/mq/publisher"
	"cloudpvp-matcher/internal/usecase/matchmaking"

	amqp "github.com/rabbitmq/amqp091-go"
)

var errInvalidLobbyMessage = errors.New("无效 lobby 消息")

// Options 配置匹配服务启动参数。
type Options struct {
	ConfigPath string
}

// Run 初始化基础设施依赖并阻塞直到服务关闭。
func Run(ctx context.Context, opts Options) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		err := localconfig.GenerateLocalAppConfig(configPath)
		if err != nil {
			log.Fatalln("生成配置文件失败：", err)
		}
		log.Fatalln("检测到配置文件不存在，已生成配置文件...")
	}

	appConfig, err := localconfig.LoadLocalAppConfig(configPath)
	if err != nil {
		return fmt.Errorf("读取本地 Apollo 配置失败: %w", err)
	}

	apolloClient := localconfig.NewApolloClient(appConfig.Apollo)
	defer apolloClient.Close()

	redisConfig, err := localconfig.Get[cache.Config](apolloClient)
	if err != nil {
		return fmt.Errorf("读取 Redis 配置失败: %w", err)
	}
	redisClient, err := cache.NewRedisClient(runCtx, *redisConfig)
	if err != nil {
		return fmt.Errorf("连接 Redis 失败: %w", err)
	}
	defer redisClient.Close()

	rabbitMQConfig, err := localconfig.Get[mq.RabbitMQConfig](apolloClient)
	if err != nil {
		return fmt.Errorf("读取 RabbitMQ 配置失败: %w", err)
	}

	rabbitMQ := mq.NewRabbitMQConnection(rabbitMQConfig)
	defer rabbitMQ.Close()
	slog.Info("RabbitMQ 连接成功", "exchange", rabbitMQConfig.ExchangeName)
	if err := mq.DeclareTopology(rabbitMQ, rabbitMQConfig.ExchangeName); err != nil {
		return err
	}

	publishChannel, err := rabbitMQ.Channel()
	if err != nil {
		return fmt.Errorf("创建 RabbitMQ 发布 channel 失败: %w", err)
	}
	defer publishChannel.Close()

	requestChannel, err := rabbitMQ.Channel()
	if err != nil {
		return fmt.Errorf("创建 RabbitMQ 请求消费 channel 失败: %w", err)
	}
	defer func(requestChannel *amqp.Channel) {
		err := requestChannel.Close()
		if err != nil {
			log.Fatalln(err.Error())
		}
	}(requestChannel)

	cancelChannel, err := rabbitMQ.Channel()
	if err != nil {
		return fmt.Errorf("创建 RabbitMQ 取消消费 channel 失败: %w", err)
	}
	defer func(cancelChannel *amqp.Channel) {
		err := cancelChannel.Close()
		if err != nil {
			log.Fatalln(err.Error())
		}
	}(cancelChannel)

	rabbitMQPublisher, err := publisher.NewPublisher(publishChannel, rabbitMQConfig)
	if err != nil {
		return err
	}
	lobbyPublisher := publisher.NewLobbyPublisher(rabbitMQPublisher)
	matchPublisher := publisher.NewMatchPublisher(rabbitMQPublisher)
	lobbyRepo := repository.NewRedisLobbyRepository(redisClient)
	queueRepo := repository.NewRedisMatchmakerQueueRepository(redisClient)
	lockManager := asynclock.NewRedisLockManager(redisClient)

	matchmakingUC := matchmaking.NewUseCase(lobbyRepo, lobbyPublisher, matchPublisher, lockManager)

	matchmakers := []domainmatch.Matchmaker{
		domainmatchmaker.NewCSGO5v5Matchmaker(queueRepo),
	}

	for _, matchmaker := range matchmakers {
		if err := matchmakingUC.AddMatchmaker(matchmaker); err != nil {
			return err
		}
	}

	go runMatchScanner(runCtx, matchmakingUC)

	go runConsumer(runCtx, requestChannel, mq.RequestQueue, "匹配请求", func(ctx context.Context, body []byte) error {
		lobby, err := decodeLobby(body)
		if err != nil {
			return err
		}
		if err := matchmakingUC.SubmitLobby(ctx, lobby); err != nil {
			return classifyLobbyHandlingError(err)
		}
		slog.Info("匹配请求处理完成", "lobby_id", lobby.LobbyID, "game_mode", lobby.GameMode, "player_count", lobby.PlayerCount)
		return nil
	})
	go runConsumer(runCtx, cancelChannel, mq.CancelQueue, "取消匹配", func(ctx context.Context, body []byte) error {
		lobby, err := decodeLobby(body)
		if err != nil {
			return err
		}
		if err := matchmakingUC.CancelLobby(ctx, lobby.LobbyID); err != nil {
			return classifyLobbyHandlingError(err)
		}
		slog.Info("取消匹配请求处理完成", "lobby_id", lobby.LobbyID)
		return nil
	})

	slog.Info("cloudpvp-matcher 已启动")
	<-runCtx.Done()
	return ctx.Err()
}

// classifyLobbyHandlingError 将确定性的业务校验错误标记为不可重试消息错误。
func classifyLobbyHandlingError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	for _, marker := range []string{
		"未找到匹配处理器",
		"unsupported game_mode",
		"member count must be positive",
		"member count exceeds",
	} {
		if strings.Contains(message, marker) {
			return fmt.Errorf("%w: %v", errInvalidLobbyMessage, err)
		}
	}
	return err
}

// runMatchScanner 按固定间隔触发自然组队扫描。
func runMatchScanner(ctx context.Context, usecase *matchmaking.UseCase) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			matchedCount, err := usecase.RunMatchCycle(ctx)
			if err != nil {
				slog.Warn("匹配扫描失败", "error", err)
				continue
			}
			if matchedCount > 0 {
				slog.Info("匹配扫描完成", "matched_count", matchedCount)
			}
		}
	}
}

// runConsumer 启动一个 RabbitMQ 队列消费者并将消息体交给业务处理函数。
func runConsumer(ctx context.Context, ch *amqp.Channel, queueName, name string, handle func(context.Context, []byte) error) {
	msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		slog.Error("RabbitMQ 消费者启动失败", "name", name, "queue", queueName, "error", err)
		return
	}

	slog.Info("RabbitMQ 消费者已启动", "name", name, "queue", queueName)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				slog.Warn("RabbitMQ 消费通道关闭", "name", name, "queue", queueName)
				return
			}
			slog.Info(
				"收到 RabbitMQ 消息",
				"name", name,
				"queue", queueName,
				"routing_key", msg.RoutingKey,
				"delivery_tag", msg.DeliveryTag,
				"redelivered", msg.Redelivered,
				"body_bytes", len(msg.Body),
			)
			if err := handle(ctx, msg.Body); err != nil {
				slog.Error("RabbitMQ 消息处理失败", "name", name, "queue", queueName, "routing_key", msg.RoutingKey, "delivery_tag", msg.DeliveryTag, "error", err)
				// 契约错误直接丢弃；Redis、锁或发布确认等瞬时错误保留消息等待恢复。
				requeue := !errors.Is(err, errInvalidLobbyMessage)
				if nackErr := msg.Nack(false, requeue); nackErr != nil {
					slog.Error("RabbitMQ 消息拒绝失败", "name", name, "queue", queueName, "delivery_tag", msg.DeliveryTag, "error", nackErr)
				}
				continue
			}
			if err := msg.Ack(false); err != nil {
				slog.Error("RabbitMQ 消息确认失败", "name", name, "queue", queueName, "delivery_tag", msg.DeliveryTag, "error", err)
				continue
			}
			slog.Info("RabbitMQ 消息已确认", "name", name, "queue", queueName, "delivery_tag", msg.DeliveryTag)
		}
	}
}

// decodeLobby 将入站 lobby 消息转换为领域 lobby。
func decodeLobby(body []byte) (*domainlobby.Lobby, error) {
	var lobby domainlobby.Lobby
	if err := json.Unmarshal(body, &lobby); err != nil {
		return nil, fmt.Errorf("%w: 反序列化失败: %v", errInvalidLobbyMessage, err)
	}
	if strings.TrimSpace(lobby.LobbyID) == "" {
		return nil, fmt.Errorf("%w: lobby_id 不能为空", errInvalidLobbyMessage)
	}
	if lobby.GameMode == "" {
		return nil, fmt.Errorf("%w: game_mode 不能为空", errInvalidLobbyMessage)
	}

	now := time.Now()
	if !lobby.CreatedAt.IsZero() {
		now = lobby.CreatedAt
	}
	lobby.CreatedAt = now
	lobby.UpdatedAt = now
	slog.Info("RabbitMQ lobby 消息解析成功", "lobby_id", lobby.LobbyID, "game_mode", lobby.GameMode, "player_count", lobby.PlayerCount)
	return &lobby, nil
}
