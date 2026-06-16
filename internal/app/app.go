// Package app 负责装配并运行匹配服务。
package app

import (
	domainmatchmaker "cloudpvp-matcher/internal/domain/match/matchmaker/csgo_5v5"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	domainlobby "cloudpvp-matcher/internal/domain/lobby"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	"cloudpvp-matcher/internal/infra/cache"
	"cloudpvp-matcher/internal/infra/cache/repository"
	localconfig "cloudpvp-matcher/internal/infra/config"
	"cloudpvp-matcher/internal/infra/mq"
	"cloudpvp-matcher/internal/infra/mq/publisher"
	"cloudpvp-matcher/internal/usecase/matchmaking"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

	newPublisher := publisher.NewPublisher(publishChannel, rabbitMQConfig)
	lobbyRepo := repository.NewRedisLobbyRepository(redisClient)
	queueRepo := repository.NewRedisMatchmakerQueueRepository(redisClient)

	matchmakingUC := matchmaking.NewUseCase(lobbyRepo, newPublisher)

	matchmakers := []domainmatch.Matchmaker{
		domainmatchmaker.NewCSGO5v5Matchmaker(queueRepo),
	}

	for _, matchmaker := range matchmakers {
		if err := matchmakingUC.AddMatchmaker(matchmaker); err != nil {
			return err
		}
	}

	go runConsumer(runCtx, requestChannel, "matchmaking.request.queue", "匹配请求", func(ctx context.Context, body []byte) error {
		lobby, err := decodeLobby(body)
		if err != nil {
			return err
		}
		return matchmakingUC.SubmitLobby(ctx, lobby)
	})
	go runConsumer(runCtx, cancelChannel, "matchmaking.cancel.queue", "取消匹配", func(ctx context.Context, body []byte) error {
		lobby, err := decodeLobby(body)
		if err != nil {
			return err
		}
		return matchmakingUC.CancelLobby(ctx, lobby.LobbyID)
	})

	slog.Info("cloudpvp-matcher 已启动")
	<-runCtx.Done()
	return ctx.Err()
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
			if err := handle(ctx, msg.Body); err != nil {
				slog.Error("RabbitMQ 消息处理失败", "name", name, "queue", queueName, "error", err)
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}
}

// decodeLobby 将入站 lobby 消息转换为领域 lobby。
func decodeLobby(body []byte) (*domainlobby.Lobby, error) {
	var lobby domainlobby.Lobby
	if err := json.Unmarshal(body, &lobby); err != nil {
		return nil, fmt.Errorf("反序列化 lobby 失败: %w", err)
	}

	now := time.Now()
	if !lobby.CreatedAt.IsZero() {
		now = lobby.CreatedAt
	}
	lobby.CreatedAt = now
	lobby.UpdatedAt = now
	return &lobby, nil
}
