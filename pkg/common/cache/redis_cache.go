// Package cache provides caching functionality for the application
package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
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
	fmt.Printf("NewRedisCache called with TLS config: %+v\n", cfg.TLS)
	fmt.Printf("Redis config - Address: %s, Password: %s, DB: %d\n", cfg.Address, cfg.Password, cfg.Database)

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
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	} else if cfg.TLS != nil && cfg.TLS.Enabled {
		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}
	}

	// Create the Redis client
	client := redis.NewClient(options)

	// Create cache wrapper
	cache := &RedisCache{
		client: client,
		config: cfg,
	}

	// Test the connection
	timeout := cfg.DialTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Testing Redis connection to %s with TLS=%v, timeout=%v\n", cfg.Address, cfg.TLS != nil && cfg.TLS.Enabled, timeout)

	// Try a simple ping first
	start := time.Now()
	if err := client.Ping(ctx).Err(); err != nil {
		elapsed := time.Since(start)
		fmt.Printf("Ping failed after %v: %v\n", elapsed, err)

		// Check if it's a network error
		if err == context.DeadlineExceeded {
			fmt.Printf("Connection timed out - check if SSH tunnel is working\n")
		}

		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	fmt.Printf("Successfully connected to Redis via TLS! Ping took %v\n", time.Since(start))

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

// Size returns the number of keys in cache (approximation for Redis)
func (c *RedisCache) Size() int {
	// Redis doesn't provide exact count without scanning all keys
	// Return 0 as placeholder - actual implementation would need DBSIZE command
	return 0
}

// Note: RedisClusterCache and related types are defined in redis_cluster.go
