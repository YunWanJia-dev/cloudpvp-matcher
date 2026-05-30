// Package cache 初始化基础设施适配器使用的缓存客户端。
package cache

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Config 包含 Redis 连接配置。
type Config struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// NewRedisClient 创建并校验 Redis 客户端。
func NewRedisClient(ctx context.Context, cfg Config) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return client, nil
}
