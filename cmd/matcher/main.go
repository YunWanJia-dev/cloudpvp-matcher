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

	appsvc "cloudpvp-matcher/internal/application/service"
	domainsvc "cloudpvp-matcher/internal/domain/service"
	"cloudpvp-matcher/internal/domain/valueobject"
	"cloudpvp-matcher/internal/infra/apollo"
	localconfig "cloudpvp-matcher/internal/infra/config"
	"cloudpvp-matcher/internal/infra/rabbitmq"
	infraredis "cloudpvp-matcher/internal/infra/redis"
	"cloudpvp-matcher/internal/interface/handler"
	adapterrepo "cloudpvp-matcher/internal/interface/repository"
)

const (
	requestQueue      = "matchmaking.request.queue"
	requestRoutingKey = "matchmaking.request"

	resultQueue      = "match.result.queue"
	resultRoutingKey = "match.result"

	serverCreateQueue      = "server.create.queue"
	serverCreateRoutingKey = "server.create"

	confirmQueue      = "match.confirm.queue"
	confirmRoutingKey = "match.confirm.*"
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
	redisClient, err := infraredis.NewClient(ctx, redisCfg)
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
	rmqConn, err := rabbitmq.NewConnection(rabbitMQCfg)
	if err != nil {
		slog.Error("RabbitMQ 连接失败", "error", err)
		os.Exit(1)
	}
	defer rmqConn.Close()

	// 5. 声明队列和绑定
	declareQueues(rmqConn)

	// 6. 初始化仓储（适配器层）
	ticketRepo := adapterrepo.NewRedisTicketRepository(redisClient)
	matchConfigs, err := apolloClient.GetMatchConfigs("")
	if err != nil {
		slog.Error("读取 Apollo 匹配配置失败", "error", err)
		os.Exit(1)
	}
	slog.Info("已加载 Apollo 匹配配置", "count", len(matchConfigs))
	configRepo := adapterrepo.NewLocalConfigRepository(matchConfigs)

	// 7. 初始化事件发布者（实现 domain/repository/EventPublisher 端口）
	publisher := rabbitmq.NewPublisher(rmqConn)

	// 8. 注册所有匹配器（领域服务实现）
	matchmakers := []domainsvc.Matchmaker{
		domainsvc.NewCSGO5v5Matchmaker(),
		// 后续新增游戏模式在此注册
	}

	// 9. 初始化用例层
	matchmakingUC := appsvc.NewMatchmakingUseCase(ticketRepo, configRepo, publisher, matchmakers)

	// 10. 初始化消息处理器
	matchHandler := handler.NewMatchHandler(matchmakingUC, redisClient)

	// 11. 启动请求消费者
	requestConsumer := rabbitmq.NewConsumer(rmqConn, requestQueue, matchHandler.Handle)

	go func() {
		slog.Info("匹配请求消费者启动", "queue", requestQueue)
		if err := requestConsumer.Run(ctx); err != nil && err != context.Canceled {
			slog.Error("匹配请求消费者异常退出", "error", err)
		}
	}()

	// 12. 启动定时任务：清理过期票据
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		lifecycle := appsvc.NewTicketLifecycle(ticketRepo)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				modes := []valueobject.GameMode{valueobject.GameModeCSGO5v5}
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

func redisConfigFromApollo(client *apollo.Client) (infraredis.Config, error) {
	addr := client.GetString("", "redis.addr", "")
	if addr == "" {
		return infraredis.Config{}, errors.New("apollo: redis.addr is required")
	}
	return infraredis.Config{
		Addr:     addr,
		Password: client.GetString("", "redis.password", ""),
		DB:       client.GetInt("", "redis.db", 0),
	}, nil
}

func rabbitMQConfigFromApollo(client *apollo.Client) (rabbitmq.Config, error) {
	url := client.GetString("", "rabbitmq.url", "")
	if url == "" {
		return rabbitmq.Config{}, errors.New("apollo: rabbitmq.url is required")
	}
	exchangeName := client.GetString("", "rabbitmq.exchange_name", "")
	if exchangeName == "" {
		return rabbitmq.Config{}, errors.New("apollo: rabbitmq.exchange_name is required")
	}
	return rabbitmq.Config{
		URL:          url,
		ExchangeName: exchangeName,
	}, nil
}

// declareQueues 声明所有需要的队列并绑定路由键。
func declareQueues(conn *rabbitmq.Connection) {
	queues := map[string]string{
		requestQueue:      requestRoutingKey,
		resultQueue:       resultRoutingKey,
		serverCreateQueue: serverCreateRoutingKey,
		confirmQueue:      confirmRoutingKey,
	}

	for queue, routingKey := range queues {
		if err := conn.DeclareQueue(queue, routingKey); err != nil {
			slog.Error("声明队列失败", "queue", queue, "error", err)
			os.Exit(1)
		}
	}
}
