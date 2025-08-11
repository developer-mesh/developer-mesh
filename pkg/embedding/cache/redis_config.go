package cache

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisPoolConfig contains Redis connection pool configuration
type RedisPoolConfig struct {
	// Connection pool settings
	PoolSize      int           `json:"pool_size" yaml:"pool_size"`             // Maximum number of connections
	MinIdleConns  int           `json:"min_idle_conns" yaml:"min_idle_conns"`   // Minimum idle connections
	MaxRetries    int           `json:"max_retries" yaml:"max_retries"`         // Maximum retries before giving up
	DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout"`       // Dial timeout for establishing new connections
	ReadTimeout   time.Duration `json:"read_timeout" yaml:"read_timeout"`       // Timeout for socket reads
	WriteTimeout  time.Duration `json:"write_timeout" yaml:"write_timeout"`     // Timeout for socket writes
	PoolTimeout   time.Duration `json:"pool_timeout" yaml:"pool_timeout"`       // Time to wait for connection from pool
	IdleTimeout   time.Duration `json:"idle_timeout" yaml:"idle_timeout"`       // Time after which idle connections are closed
	IdleCheckFreq time.Duration `json:"idle_check_freq" yaml:"idle_check_freq"` // Frequency of idle checks
	MaxConnAge    time.Duration `json:"max_conn_age" yaml:"max_conn_age"`       // Maximum age of a connection
}

// DefaultRedisPoolConfig returns production-ready Redis pool configuration
func DefaultRedisPoolConfig() *RedisPoolConfig {
	return &RedisPoolConfig{
		PoolSize:      100,              // Sufficient for most production loads
		MinIdleConns:  10,               // Keep some connections ready
		MaxRetries:    3,                // Retry failed commands
		DialTimeout:   5 * time.Second,  // Reasonable timeout for new connections
		ReadTimeout:   3 * time.Second,  // Timeout for reads
		WriteTimeout:  3 * time.Second,  // Timeout for writes
		PoolTimeout:   4 * time.Second,  // Wait time for connection from pool
		IdleTimeout:   5 * time.Minute,  // Close idle connections after 5 minutes
		IdleCheckFreq: 1 * time.Minute,  // Check for idle connections every minute
		MaxConnAge:    30 * time.Minute, // Recycle connections after 30 minutes
	}
}

// HighLoadRedisPoolConfig returns configuration optimized for high load
func HighLoadRedisPoolConfig() *RedisPoolConfig {
	return &RedisPoolConfig{
		PoolSize:      500,             // More connections for high load
		MinIdleConns:  50,              // Keep more connections ready
		MaxRetries:    5,               // More retries for resilience
		DialTimeout:   3 * time.Second, // Faster timeout for new connections
		ReadTimeout:   2 * time.Second, // Shorter timeouts
		WriteTimeout:  2 * time.Second,
		PoolTimeout:   2 * time.Second,  // Don't wait too long
		IdleTimeout:   3 * time.Minute,  // Close idle connections sooner
		IdleCheckFreq: 30 * time.Second, // Check more frequently
		MaxConnAge:    15 * time.Minute, // Recycle connections more often
	}
}

// LowLatencyRedisPoolConfig returns configuration optimized for low latency
func LowLatencyRedisPoolConfig() *RedisPoolConfig {
	return &RedisPoolConfig{
		PoolSize:      200,                    // Moderate pool size
		MinIdleConns:  20,                     // Keep connections warm
		MaxRetries:    2,                      // Fail fast
		DialTimeout:   1 * time.Second,        // Very fast timeout
		ReadTimeout:   500 * time.Millisecond, // Sub-second timeouts
		WriteTimeout:  500 * time.Millisecond,
		PoolTimeout:   1 * time.Second,  // Don't wait long
		IdleTimeout:   10 * time.Minute, // Keep connections longer
		IdleCheckFreq: 2 * time.Minute,  // Check less frequently
		MaxConnAge:    1 * time.Hour,    // Keep connections longer
	}
}

// ToRedisOptions converts pool config to redis.Options
func (c *RedisPoolConfig) ToRedisOptions(addr string, db int) *redis.Options {
	return &redis.Options{
		Addr:            addr,
		DB:              db,
		PoolSize:        c.PoolSize,
		MinIdleConns:    c.MinIdleConns,
		MaxRetries:      c.MaxRetries,
		DialTimeout:     c.DialTimeout,
		ReadTimeout:     c.ReadTimeout,
		WriteTimeout:    c.WriteTimeout,
		PoolTimeout:     c.PoolTimeout,
		ConnMaxIdleTime: c.IdleTimeout,
	}
}

// NewRedisClientWithPool creates a new Redis client with configured connection pool
func NewRedisClientWithPool(addr string, db int, poolConfig *RedisPoolConfig) *redis.Client {
	if poolConfig == nil {
		poolConfig = DefaultRedisPoolConfig()
	}

	options := poolConfig.ToRedisOptions(addr, db)
	return redis.NewClient(options)
}
