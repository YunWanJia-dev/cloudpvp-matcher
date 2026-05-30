// 匹配服务入口，负责依赖注入和启动所有组件。
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloudpvp-matcher/internal/domain/config"
	domainmatch "cloudpvp-matcher/internal/domain/match"
	"cloudpvp-matcher/internal/handler"
	"cloudpvp-matcher/internal/infra/apollo"
	"cloudpvp-matcher/internal/infra/cache"
	localconfig "cloudpvp-matcher/internal/infra/config"
	"cloudpvp-matcher/internal/infra/mq"
	"cloudpvp-matcher/internal/usecase/matchmaking"
	ticketusecase "cloudpvp-matcher/internal/usecase/ticket"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 本地只读取 Apollo 连接参数，其他运行时配置均从 Apollo 拉取。
	apolloCfg, err := localconfig.LoadApollo("")
	if err != nil {
		slog.Error("读取本地 Apollo 配置失败", "error", err)
		os.Exit(1)
	}

	// 2. 初始化 Apollo 配置客户端
	apolloClient, err := apollo.NewClient(ctx, apolloCfg)
	if err != nil {
		slog.Error("Apollo 连接失败", "error", err)
		os.Exit(1)
	}
	defer apolloClient.Close()

	// 3. 初始化 Redis 客户端
	redisCfg, err := redisConfigFromApollo(apolloClient)
	if err != nil {
		slog.Error("读取 Apollo Redis 配置失败", "error", err)
		os.Exit(1)
	}
	redisClient, err := cache.NewRedisClient(ctx, redisCfg)
	if err != nil {
		slog.Error("Redis 连接失败", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// 4. 初始化 RabbitMQ 连接
	rabbitMQCfg, err := rabbitMQConfigFromApollo(apolloClient)
	if err != nil {
		slog.Error("读取 Apollo RabbitMQ 配置失败", "error", err)
		os.Exit(1)
	}
	rabbitMQ, err := mq.NewRabbitMQ(rabbitMQCfg)
	if err != nil {
		slog.Error("RabbitMQ 连接失败", "error", err)
		os.Exit(1)
	}
	defer rabbitMQ.Close()

	// 5. 声明 MQ 拓扑
	if err := mq.DeclareTopology(rabbitMQ); err != nil {
		slog.Error("声明 RabbitMQ 拓扑失败", "error", err)
		os.Exit(1)
	}

	// 6. 初始化仓储（适配器层）
	ticketRepo := cache.NewRedisTicketRepository(redisClient)
	matchConfigs, err := apolloClient.GetMatchConfigs("")
	if err != nil {
		slog.Error("读取 Apollo 匹配配置失败", "error", err)
		os.Exit(1)
	}
	slog.Info("已加载 Apollo 匹配配置", "count", len(matchConfigs))
	configRepo := localconfig.NewLocalConfigRepository(matchConfigs)

	// 7. 初始化事件发布者（实现 domain/match.EventPublisher 端口）
	publisher := mq.NewPublisher(rabbitMQ)

	// 8. 注册所有匹配器（领域服务实现）
	matchmakers := []domainmatch.Matchmaker{
		domainmatch.NewCSGO5v5Matchmaker(),
		// 后续新增游戏模式在此注册
	}

	// 9. 初始化用例层
	matchmakingUC := matchmaking.NewUseCase(ticketRepo, configRepo, publisher, matchmakers)

	// 10. 初始化消息处理器
	matchHandler := handler.NewMatchHandler(matchmakingUC)

	// 11. 启动请求消费者
	requestConsumer := mq.NewConsumer(rabbitMQ, mq.RequestQueue, matchHandler.Handle)

	go func() {
		slog.Info("匹配请求消费者启动", "queue", mq.RequestQueue)
		if err := requestConsumer.Run(ctx); err != nil && err != context.Canceled {
			slog.Error("匹配请求消费者异常退出", "error", err)
		}
	}()

	// 12. 启动定时任务：清理过期票据
	go func() {
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
	}()

	slog.Info("cloudpvp-matcher 已启动")

	// 13. 等待退出信号
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("正在优雅关闭...")
	cancel()
	time.Sleep(2 * time.Second) // 等待消费循环退出
}

func redisConfigFromApollo(client *apollo.Client) (cache.Config, error) {
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

func rabbitMQConfigFromApollo(client *apollo.Client) (mq.Config, error) {
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
