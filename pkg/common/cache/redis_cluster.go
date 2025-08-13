package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClusterCache implements Cache using Redis in cluster mode
type RedisClusterCache struct {
	client *redis.ClusterClient
	config RedisClusterConfig
}

// RedisClusterConfig holds configuration for Redis in cluster mode
type RedisClusterConfig struct {
	Addrs          []string
	Username       string
	Password       string
	MaxRetries     int
	MinIdleConns   int
	PoolSize       int
	DialTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	PoolTimeout    time.Duration
	UseTLS         bool
	TLSConfig      *tls.Config
	RouteRandomly  bool
	RouteByLatency bool
}

// NewRedisClusterCache creates a new Redis cluster cache
func NewRedisClusterCache(cfg RedisClusterConfig) (*RedisClusterCache, error) {
	// Create Redis cluster options
	options := &redis.ClusterOptions{
		Addrs:          cfg.Addrs,
		MaxRetries:     cfg.MaxRetries,
		MinIdleConns:   cfg.MinIdleConns,
		PoolSize:       cfg.PoolSize,
		DialTimeout:    cfg.DialTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		PoolTimeout:    cfg.PoolTimeout,
		RouteRandomly:  cfg.RouteRandomly,
		RouteByLatency: cfg.RouteByLatency,
	}

	// Add authentication if provided
	if cfg.Username != "" {
		options.Username = cfg.Username
	}

	if cfg.Password != "" {
		options.Password = cfg.Password
	}

	// Add TLS if enabled
	if cfg.UseTLS {
		options.TLSConfig = cfg.TLSConfig
	}

	// Create Redis cluster client
	client := redis.NewClusterClient(options)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisClusterCache{
		client: client,
		config: cfg,
	}, nil
}

// Get retrieves a value from cache
func (c *RedisClusterCache) Get(ctx context.Context, key string, value interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrNotFound
		}
		return err
	}

	return json.Unmarshal(data, value)
}

// Set stores a value in cache with TTL
func (c *RedisClusterCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a value from cache
func (c *RedisClusterCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in cache
func (c *RedisClusterCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// Flush clears all values in cache
func (c *RedisClusterCache) Flush(ctx context.Context) error {
	// In a Redis cluster, we need to flush each node separately
	err := c.client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
		return shard.FlushAll(ctx).Err()
	})

	return err
}

// Close closes the Redis cluster connection
func (c *RedisClusterCache) Close() error {
	return c.client.Close()
}

// Size returns the number of keys in cache (approximation for Redis Cluster)
func (c *RedisClusterCache) Size() int {
	// Redis Cluster doesn't provide exact count without scanning all keys
	// Return 0 as placeholder
	return 0
}

// GetClient returns the underlying Redis cluster client
func (c *RedisClusterCache) GetClient() *redis.ClusterClient {
	return c.client
}
