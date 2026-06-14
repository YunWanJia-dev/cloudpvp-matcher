// Package app 负责装配并运行匹配服务。
package app

import (
	"cloudpvp-matcher/internal/infra/asynclock"
	"cloudpvp-matcher/internal/infra/cache/repository"
	"cloudpvp-matcher/internal/infra/mq/publisher"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	domainconfig "cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
	"cloudpvp-matcher/internal/handler/dto"
	"cloudpvp-matcher/internal/infra/cache"
	localconfig "cloudpvp-matcher/internal/infra/config"
	"cloudpvp-matcher/internal/infra/mq"
	"cloudpvp-matcher/internal/usecase/matchmaking"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultMatchRequestQueue = "matchmaking.request.queue"
	defaultMatchCancelQueue  = "matchmaking.cancel.queue"
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
	normalizeRabbitMQConfig(rabbitMQConfig)

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

	matchConfigs, err := localconfig.Get[[]*domainconfig.MatchConfig](apolloClient)
	if err != nil {
		return fmt.Errorf("读取匹配配置失败: %w", err)
	}

	ticketRepo := repository.NewRedisTicketRepository(redisClient)
	lockManager := asynclock.NewRedisLockManager(redisClient)
	newPublisher := publisher.NewPublisher(publishChannel, rabbitMQConfig)
	matchmakers := []domainmatch.Matchmaker{
		domainmatch.NewCSGO5v5Matchmaker(),
	}
	matchmakingUC := matchmaking.NewUseCase(
		ticketRepo,
		*matchConfigs,
		newPublisher,
		matchmakers,
		matchmaking.WithLockManager(lockManager),
	)

	go runConsumer(runCtx, requestChannel, rabbitMQConfig.MatchRequestQueue, "匹配请求", func(ctx context.Context, body []byte) error {
		ticket, err := decodeMatchRequest(body)
		if err != nil {
			return err
		}
		return matchmakingUC.SubmitTicket(ctx, ticket)
	})
	go runConsumer(runCtx, cancelChannel, rabbitMQConfig.MatchCancelQueue, "取消匹配", func(ctx context.Context, body []byte) error {
		lobbyID, err := decodeMatchCancelRequest(body)
		if err != nil {
			return err
		}
		return matchmakingUC.CancelTicket(ctx, lobbyID)
	})
	go runMatchScanner(runCtx, matchmakingUC)

	slog.Info("cloudpvp-matcher 已启动")
	<-runCtx.Done()
	return ctx.Err()
}

// normalizeRabbitMQConfig 为未配置的消费队列填充默认值。
func normalizeRabbitMQConfig(cfg *mq.RabbitMQConfig) {
	if cfg.MatchRequestQueue == "" {
		cfg.MatchRequestQueue = defaultMatchRequestQueue
	}
	if cfg.MatchCancelQueue == "" {
		cfg.MatchCancelQueue = defaultMatchCancelQueue
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
			if err := handle(ctx, msg.Body); err != nil {
				slog.Error("RabbitMQ 消息处理失败", "name", name, "queue", queueName, "error", err)
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}
}

// runMatchScanner 按固定间隔触发匹配扫描。
func runMatchScanner(ctx context.Context, usecase *matchmaking.UseCase) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := usecase.RunMatchCycle(ctx)
			if err != nil {
				slog.Warn("匹配扫描失败", "error", err)
				continue
			}
			if result.MatchedCount > 0 {
				slog.Info("匹配扫描完成", "matched_count", result.MatchedCount, "scanned_modes", result.ScannedModes)
			}
		}
	}
}

// decodeMatchRequest 将入站匹配请求消息转换为领域票据。
func decodeMatchRequest(body []byte) (*domainticket.Ticket, error) {
	var req dto.MatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("反序列化匹配请求失败: %w", err)
	}

	members := make([]domainticket.PlayerInfo, 0, len(req.Members))
	for _, member := range req.Members {
		members = append(members, domainticket.PlayerInfo{
			PlayerID: member.PlayerID,
		})
	}

	now := time.Now()
	if !req.CreatedAt.IsZero() {
		now = req.CreatedAt
	}
	return &domainticket.Ticket{
		LobbyID:   req.LobbyID,
		GameMode:  domainconfig.GameMode(req.GameMode),
		Members:   members,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// decodeMatchCancelRequest 从取消匹配请求中提取 lobby ID。
func decodeMatchCancelRequest(body []byte) (string, error) {
	var req dto.MatchCancelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", fmt.Errorf("反序列化取消匹配请求失败: %w", err)
	}
	return req.LobbyID, nil
}
