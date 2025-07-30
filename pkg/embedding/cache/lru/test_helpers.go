package lru

import (
	"os"
	"testing"

	"github.com/go-redis/redis/v8"
)

// GetTestRedisAddr returns the Redis address for tests, defaulting to localhost:6379
func GetTestRedisAddr() string {
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:6379"
}

// GetTestRedisClient creates a Redis client for tests using TEST_REDIS_ADDR
func GetTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{
		Addr: GetTestRedisAddr(),
		DB:   15, // Use test database
	})
}
