package repository

import goredis "github.com/redis/go-redis/v9"

type RedisRepository struct {
	client *goredis.Client
}

// NewRedisTicketRepository 创建一个新的 Redis 票据仓储。
func NewRedisTicketRepository(client *goredis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}
