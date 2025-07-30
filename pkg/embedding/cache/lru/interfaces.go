package lru

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// RedisClient defines the interface for Redis operations needed by LRU
type RedisClient interface {
	Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error)
	GetClient() *redis.Client
}
