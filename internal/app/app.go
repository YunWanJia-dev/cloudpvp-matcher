// Package app 负责装配并运行匹配服务。
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	domainticket "cloudpvp-matcher/internal/domain/ticket"
	"cloudpvp-matcher/internal/handler"
	"cloudpvp-matcher/internal/infra/cache"
	localconfig "cloudpvp-matcher/internal/infra/config"
	"cloudpvp-matcher/internal/infra/mq"
	"cloudpvp-matcher/internal/usecase/matchmaking"
	ticketusecase "cloudpvp-matcher/internal/usecase/ticket"
)

// Options 配置匹配服务启动参数。
type Options struct {
	ConfigPath string
}

// Run 初始化匹配服务依赖，并阻塞直到服务关闭。
func Run(ctx context.Context, opts Options) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 本地只读取 Apollo 连接参数，其他运行时配置均从 Apollo 拉取。
	apolloCfg, err := localconfig.LoadApollo(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("读取本地 Apollo 配置失败: %w", err)
	}

	apolloClient, err := localconfig.NewClient(runCtx, apolloCfg)
	if err != nil {
		return fmt.Errorf("Apollo 连接失败: %w", err)
	}
	defer apolloClient.Close()

	redisCfg, err := redisConfigFromApollo(apolloClient)
	if err != nil {
		return fmt.Errorf("读取 Apollo Redis 配置失败: %w", err)
	}
	redisClient, err := cache.NewRedisClient(runCtx, redisCfg)
	if err != nil {
		return fmt.Errorf("Redis 连接失败: %w", err)
	}
	defer redisClient.Close()

	rabbitMQCfg, err := rabbitMQConfigFromApollo(apolloClient)
	if err != nil {
		return fmt.Errorf("读取 Apollo RabbitMQ 配置失败: %w", err)
	}
	rabbitMQ, err := mq.NewRabbitMQ(rabbitMQCfg)
	if err != nil {
		return fmt.Errorf("RabbitMQ 连接失败: %w", err)
	}
	defer rabbitMQ.Close()

	if err := mq.DeclareTopology(rabbitMQ); err != nil {
		return fmt.Errorf("声明 RabbitMQ 拓扑失败: %w", err)
	}

	ticketRepo := cache.NewRedisTicketRepository(redisClient)
	matchConfigs, err := apolloClient.GetMatchConfigs("")
	if err != nil {
		return fmt.Errorf("读取 Apollo 匹配配置失败: %w", err)
	}
	slog.Info("已加载 Apollo 匹配配置", "count", len(matchConfigs))
	configRepo := localconfig.NewLocalConfigRepository(matchConfigs)
	publisher := mq.NewPublisher(rabbitMQ)

	matchmakers := []domainmatch.Matchmaker{
		domainmatch.NewCSGO5v5Matchmaker(),
		// 后续新增游戏模式在此注册
	}

	matchmakingUC := matchmaking.NewUseCase(ticketRepo, configRepo, publisher, matchmakers)
	matchHandler := handler.NewMatchHandler(matchmakingUC)
	requestConsumer := mq.NewConsumer(rabbitMQ, mq.RequestQueue, matchHandler.Handle)

	go runConsumer(runCtx, requestConsumer)
	go runTicketCleanup(runCtx, ticketRepo)

	slog.Info("cloudpvp-matcher 已启动")
	<-runCtx.Done()

	if err := ctx.Err(); err != nil {
		return err
	}

	slog.Info("正在优雅关闭...")
	time.Sleep(2 * time.Second) // 等待消费循环退出
	return nil
}

func runConsumer(ctx context.Context, requestConsumer handler.Consumer) {
	slog.Info("匹配请求消费者启动", "queue", mq.RequestQueue)
	if err := requestConsumer.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("匹配请求消费者异常退出", "error", err)
	}
}

func runTicketCleanup(ctx context.Context, ticketRepo domainticket.Repository) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	lifecycle := ticketusecase.NewLifecycle(ticketRepo)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			modes := []config.GameMode{config.GameModeCSGO5v5}
			cleaned, err := lifecycle.CleanupExpiredTickets(ctx, modes, 5*time.Minute)
			if err != nil {
				slog.Warn("清理过期票据失败", "error", err)
			} else if cleaned > 0 {
				slog.Info("清理过期票据", "count", cleaned)
			}
		}
	}
}

func redisConfigFromApollo(client *localconfig.Client) (cache.Config, error) {
	addr := client.GetString("", "redis.addr", "")
	if addr == "" {
		return cache.Config{}, errors.New("apollo: redis.addr is required")
	}
	return cache.Config{
		Addr:     addr,
		Password: client.GetString("", "redis.password", ""),
		DB:       client.GetInt("", "redis.db", 0),
	}, nil
}

func rabbitMQConfigFromApollo(client *localconfig.Client) (mq.Config, error) {
	url := client.GetString("", "rabbitmq.url", "")
	if url == "" {
		return mq.Config{}, errors.New("apollo: rabbitmq.url is required")
	}
	exchangeName := client.GetString("", "rabbitmq.exchange_name", "")
	if exchangeName == "" {
		return mq.Config{}, errors.New("apollo: rabbitmq.exchange_name is required")
	}
	return mq.Config{
		URL:          url,
		ExchangeName: exchangeName,
	}, nil
}
