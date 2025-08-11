package lru

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisClient defines the interface for Redis operations needed by LRU
type RedisClient interface {
	Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error)
	GetClient() *redis.Client
}
