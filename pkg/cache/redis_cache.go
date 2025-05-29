// Package cache provides caching functionality for the application
package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCache implements the Cache interface for Redis
type RedisCache struct {
	client *redis.Client
	config RedisConfig
}

// marshal converts a value to JSON bytes
func marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// unmarshal converts JSON bytes to a value
func unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(cfg RedisConfig) (*RedisCache, error) {
	// Create Redis options
	options := &redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.Database,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		MaxRetries:   cfg.MaxRetries,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	}

	// Add username if provided (for Redis 6.0+)
	if cfg.Username != "" {
		options.Username = cfg.Username
	}

	// Add TLS if needed
	if cfg.UseIAMAuth {
		options.TLSConfig = &tls.Config{}
	}

	// Create the Redis client
	client := redis.NewClient(options)

	// Create cache wrapper
	cache := &RedisCache{
		client: client,
		config: cfg,
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return cache, nil
}

// Get retrieves a value from the cache
func (c *RedisCache) Get(ctx context.Context, key string, value interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrNotFound
		}
		return fmt.Errorf("failed to get value from cache: %w", err)
	}

	// Unmarshal the data into the value
	if err := unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal cache value: %w", err)
	}

	return nil
}

// Set stores a value in the cache
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Marshal the value
	data, err := marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}

	// Set with expiration
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set value in cache: %w", err)
	}

	return nil
}

// Delete removes a value from the cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete value from cache: %w", err)
	}
	return nil
}

// Exists checks if a key exists in the cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if key exists: %w", err)
	}
	return result > 0, nil
}

// Flush clears all values from the cache
func (c *RedisCache) Flush(ctx context.Context) error {
	if err := c.client.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}
	return nil
}

// Close closes the cache connection
func (c *RedisCache) Close() error {
	// Close the Redis client
	return c.client.Close()
}

// Note: RedisClusterCache and related types are defined in redis_cluster.go
