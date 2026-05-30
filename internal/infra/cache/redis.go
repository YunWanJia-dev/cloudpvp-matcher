// Package cache initializes cache clients used by infrastructure adapters.
package cache

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Config contains Redis connection settings.
type Config struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// NewRedisClient creates and verifies a Redis client.
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
