package repository

import goredis "github.com/redis/go-redis/v9"

type RedisRepository struct {
	client *goredis.Client
}

// NewRedisLobbyRepository 创建一个新的 Redis lobby 仓储。
func NewRedisLobbyRepository(client *goredis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}
