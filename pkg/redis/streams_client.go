package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/redis/go-redis/v9"
)

// StreamsConfig represents the configuration for Redis Streams
type StreamsConfig struct {
	// Connection settings
	Addresses    []string      `yaml:"addresses" json:"addresses"`
	Password     string        `yaml:"password" json:"password"`
	DB           int           `yaml:"db" json:"db"`
	MaxRetries   int           `yaml:"max_retries" json:"max_retries"`
	RetryBackoff time.Duration `yaml:"retry_backoff" json:"retry_backoff"`

	// TLS settings
	TLSEnabled bool        `yaml:"tls_enabled" json:"tls_enabled"`
	TLSConfig  *tls.Config `yaml:"-" json:"-"`

	// Pool settings
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	MaxConnAge   time.Duration `yaml:"max_conn_age" json:"max_conn_age"`
	PoolTimeout  time.Duration `yaml:"pool_timeout" json:"pool_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`

	// Cluster settings
	ClusterEnabled bool `yaml:"cluster_enabled" json:"cluster_enabled"`
	ReadOnly       bool `yaml:"read_only" json:"read_only"`
	RouteByLatency bool `yaml:"route_by_latency" json:"route_by_latency"`

	// Sentinel settings
	SentinelEnabled  bool     `yaml:"sentinel_enabled" json:"sentinel_enabled"`
	MasterName       string   `yaml:"master_name" json:"master_name"`
	SentinelAddrs    []string `yaml:"sentinel_addrs" json:"sentinel_addrs"`
	SentinelPassword string   `yaml:"sentinel_password" json:"sentinel_password"`
}

// DefaultConfig returns a default configuration for Redis Streams
func DefaultConfig() *StreamsConfig {
	return &StreamsConfig{
		Addresses:      []string{"localhost:6379"},
		MaxRetries:     3,
		RetryBackoff:   100 * time.Millisecond,
		PoolSize:       10,
		MinIdleConns:   5,
		MaxConnAge:     30 * time.Minute,
		PoolTimeout:    4 * time.Second,
		IdleTimeout:    5 * time.Minute,
		RouteByLatency: true,
	}
}

// StreamsClient provides Redis Streams functionality with connection pooling
type StreamsClient struct {
	client redis.UniversalClient
	config *StreamsConfig
	logger observability.Logger
	mu     sync.RWMutex

	// Health check state
	healthy         bool
	healthMu        sync.RWMutex
	lastHealthCheck time.Time
}

// NewStreamsClient creates a new Redis Streams client
func NewStreamsClient(config *StreamsConfig, logger observability.Logger) (*StreamsClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	client := &StreamsClient{
		config:  config,
		logger:  logger,
		healthy: true,
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Start health check routine
	go client.healthCheckLoop()

	return client, nil
}

// connect establishes connection to Redis based on configuration
func (c *StreamsClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var client redis.UniversalClient

	if c.config.SentinelEnabled {
		// Sentinel mode for high availability
		if len(c.config.SentinelAddrs) == 0 {
			return fmt.Errorf("no Sentinel addresses configured")
		}
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       c.config.MasterName,
			SentinelAddrs:    c.config.SentinelAddrs,
			SentinelPassword: c.config.SentinelPassword,
			Password:         c.config.Password,
			DB:               c.config.DB,
			MaxRetries:       c.config.MaxRetries,
			MinRetryBackoff:  c.config.RetryBackoff,
			PoolSize:         c.config.PoolSize,
			MinIdleConns:     c.config.MinIdleConns,
			PoolTimeout:      c.config.PoolTimeout,
			ConnMaxIdleTime:  c.config.IdleTimeout,
			TLSConfig:        c.config.TLSConfig,
		})
	} else if c.config.ClusterEnabled {
		// Cluster mode for horizontal scaling
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           c.config.Addresses,
			Password:        c.config.Password,
			MaxRetries:      c.config.MaxRetries,
			MinRetryBackoff: c.config.RetryBackoff,
			PoolSize:        c.config.PoolSize,
			MinIdleConns:    c.config.MinIdleConns,
			PoolTimeout:     c.config.PoolTimeout,
			ConnMaxIdleTime: c.config.IdleTimeout,
			TLSConfig:       c.config.TLSConfig,
			ReadOnly:        c.config.ReadOnly,
			RouteByLatency:  c.config.RouteByLatency,
		})
	} else {
		// Single instance mode
		if len(c.config.Addresses) == 0 {
			return fmt.Errorf("no Redis addresses configured")
		}

		client = redis.NewClient(&redis.Options{
			Addr:            c.config.Addresses[0],
			Password:        c.config.Password,
			DB:              c.config.DB,
			MaxRetries:      c.config.MaxRetries,
			MinRetryBackoff: c.config.RetryBackoff,
			PoolSize:        c.config.PoolSize,
			MinIdleConns:    c.config.MinIdleConns,
			PoolTimeout:     c.config.PoolTimeout,
			ConnMaxIdleTime: c.config.IdleTimeout,
			TLSConfig:       c.config.TLSConfig,
		})
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis: %w", err)
	}

	c.client = client
	c.logger.Info("Connected to Redis", map[string]interface{}{
		"mode":      c.getMode(),
		"addresses": c.config.Addresses,
	})

	return nil
}

// getMode returns the current connection mode
func (c *StreamsClient) getMode() string {
	if c.config.SentinelEnabled {
		return "sentinel"
	}
	if c.config.ClusterEnabled {
		return "cluster"
	}
	return "single"
}

// healthCheckLoop runs periodic health checks
func (c *StreamsClient) healthCheckLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.checkHealth()
	}
}

// checkHealth performs a health check on the Redis connection
func (c *StreamsClient) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := c.client.Ping(ctx).Err()

	c.healthMu.Lock()
	c.healthy = err == nil
	c.lastHealthCheck = time.Now()
	c.healthMu.Unlock()

	if err != nil {
		c.logger.Error("Redis health check failed", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// IsHealthy returns the current health status
func (c *StreamsClient) IsHealthy() bool {
	c.healthMu.RLock()
	defer c.healthMu.RUnlock()
	return c.healthy
}

// Close closes the Redis client connection
func (c *StreamsClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// GetClient returns the underlying Redis client for direct access
func (c *StreamsClient) GetClient() redis.UniversalClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// Stream Operations

// AddToStream adds a message to a Redis stream
func (c *StreamsClient) AddToStream(ctx context.Context, stream string, values map[string]interface{}) (string, error) {
	if !c.IsHealthy() {
		return "", fmt.Errorf("redis connection is unhealthy")
	}

	// Convert map to args
	args := redis.XAddArgs{
		Stream: stream,
		Values: values,
	}

	result := c.client.XAdd(ctx, &args)
	return result.Result()
}

// AddToStreamWithID adds a message with a specific ID to a Redis stream
func (c *StreamsClient) AddToStreamWithID(ctx context.Context, stream, id string, values map[string]interface{}) error {
	if !c.IsHealthy() {
		return fmt.Errorf("redis connection is unhealthy")
	}

	args := redis.XAddArgs{
		Stream: stream,
		ID:     id,
		Values: values,
	}

	return c.client.XAdd(ctx, &args).Err()
}

// ReadFromStream reads messages from a stream
func (c *StreamsClient) ReadFromStream(ctx context.Context, streams []string, count int64, block time.Duration) ([]redis.XStream, error) {
	if !c.IsHealthy() {
		return nil, fmt.Errorf("redis connection is unhealthy")
	}

	// Build stream arguments (stream_name > for new messages)
	streamArgs := make([]string, len(streams))
	copy(streamArgs, streams)

	// Add starting positions
	for range streams {
		streamArgs = append(streamArgs, ">")
	}

	args := &redis.XReadArgs{
		Streams: streamArgs,
		Count:   count,
		Block:   block,
	}

	return c.client.XRead(ctx, args).Result()
}

// CreateConsumerGroup creates a consumer group for a stream
func (c *StreamsClient) CreateConsumerGroup(ctx context.Context, stream, group string, start string) error {
	if !c.IsHealthy() {
		return fmt.Errorf("redis connection is unhealthy")
	}

	return c.client.XGroupCreate(ctx, stream, group, start).Err()
}

// CreateConsumerGroupMkStream creates a consumer group and the stream if it doesn't exist
func (c *StreamsClient) CreateConsumerGroupMkStream(ctx context.Context, stream, group string, start string) error {
	if !c.IsHealthy() {
		return fmt.Errorf("redis connection is unhealthy")
	}

	return c.client.XGroupCreateMkStream(ctx, stream, group, start).Err()
}

// ReadFromConsumerGroup reads messages from a consumer group
func (c *StreamsClient) ReadFromConsumerGroup(ctx context.Context, group, consumer string, streams []string, count int64, block time.Duration, noAck bool) ([]redis.XStream, error) {
	if !c.IsHealthy() {
		return nil, fmt.Errorf("redis connection is unhealthy")
	}

	// Build stream arguments
	streamArgs := make([]string, len(streams))
	copy(streamArgs, streams)

	// Add starting positions (> for new messages in consumer group)
	for range streams {
		streamArgs = append(streamArgs, ">")
	}

	args := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  streamArgs,
		Count:    count,
		Block:    block,
		NoAck:    noAck,
	}

	// DEBUG logging
	fmt.Printf("[DEBUG] XReadGroup args: group=%s, consumer=%s, streams=%v\n", group, consumer, streamArgs)

	result, err := c.client.XReadGroup(ctx, args).Result()
	if err != nil {
		fmt.Printf("[DEBUG] XReadGroup error: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] XReadGroup success: %d stream(s) returned\n", len(result))
	}
	return result, err
}

// AckMessages acknowledges messages in a consumer group
func (c *StreamsClient) AckMessages(ctx context.Context, stream, group string, ids ...string) error {
	if !c.IsHealthy() {
		return fmt.Errorf("redis connection is unhealthy")
	}

	return c.client.XAck(ctx, stream, group, ids...).Err()
}

// ClaimMessages claims pending messages from other consumers
func (c *StreamsClient) ClaimMessages(ctx context.Context, stream, group, consumer string, minIdleTime time.Duration, ids ...string) ([]redis.XMessage, error) {
	if !c.IsHealthy() {
		return nil, fmt.Errorf("redis connection is unhealthy")
	}

	return c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdleTime,
		Messages: ids,
	}).Result()
}

// GetStreamInfo returns information about a stream
func (c *StreamsClient) GetStreamInfo(ctx context.Context, stream string) (*redis.XInfoStream, error) {
	if !c.IsHealthy() {
		return nil, fmt.Errorf("redis connection is unhealthy")
	}

	return c.client.XInfoStream(ctx, stream).Result()
}

// GetConsumerGroupInfo returns information about consumer groups
func (c *StreamsClient) GetConsumerGroupInfo(ctx context.Context, stream string) ([]redis.XInfoGroup, error) {
	if !c.IsHealthy() {
		return nil, fmt.Errorf("redis connection is unhealthy")
	}

	return c.client.XInfoGroups(ctx, stream).Result()
}

// TrimStream trims a stream to a specific length
func (c *StreamsClient) TrimStream(ctx context.Context, stream string, maxLen int64, approximate bool) error {
	if !c.IsHealthy() {
		return fmt.Errorf("redis connection is unhealthy")
	}

	if approximate {
		return c.client.XTrimMaxLenApprox(ctx, stream, maxLen, 0).Err()
	}
	return c.client.XTrimMaxLen(ctx, stream, maxLen).Err()
}
