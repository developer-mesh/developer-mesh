package config

import "time"

// ContextManagerConfig provides optimized configuration for high-performance context management
type ContextManagerConfig struct {
	// Database connection pooling settings
	Database DatabasePoolConfig `yaml:"database"`

	// Multi-level cache configuration
	Cache CacheConfig `yaml:"cache"`

	// Circuit breaker settings for resilience
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`

	// Read replica configuration for scaling
	ReadReplicas []DatabasePoolConfig `yaml:"read_replicas"`

	// Performance monitoring settings
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

// DatabasePoolConfig optimizes database connections for high concurrency
type DatabasePoolConfig struct {
	DSN                 string        `yaml:"dsn"`
	MaxOpenConns        int           `yaml:"max_open_conns" default:"50"`
	MaxIdleConns        int           `yaml:"max_idle_conns" default:"25"`
	ConnMaxLifetime     time.Duration `yaml:"conn_max_lifetime" default:"5m"`
	ConnMaxIdleTime     time.Duration `yaml:"conn_max_idle_time" default:"90s"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" default:"30s"`
}

// CacheConfig configures multi-level caching strategy
type CacheConfig struct {
	// In-memory LRU cache settings
	InMemory InMemoryCacheConfig `yaml:"in_memory"`

	// Redis distributed cache settings
	Redis RedisCacheConfig `yaml:"redis"`

	// Cache warming configuration
	Warming CacheWarmingConfig `yaml:"warming"`
}

// InMemoryCacheConfig for local high-speed caching
type InMemoryCacheConfig struct {
	Enabled     bool          `yaml:"enabled" default:"true"`
	MaxSize     int           `yaml:"max_size" default:"10000"` // Number of items
	TTL         time.Duration `yaml:"ttl" default:"5m"`
	MaxItemSize int           `yaml:"max_item_size" default:"1048576"` // 1MB
}

// RedisCacheConfig for distributed caching
type RedisCacheConfig struct {
	Enabled      bool          `yaml:"enabled" default:"true"`
	Endpoints    []string      `yaml:"endpoints"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db" default:"0"`
	TTL          time.Duration `yaml:"ttl" default:"1h"`
	MaxRetries   int           `yaml:"max_retries" default:"3"`
	DialTimeout  time.Duration `yaml:"dial_timeout" default:"5s"`
	ReadTimeout  time.Duration `yaml:"read_timeout" default:"3s"`
	WriteTimeout time.Duration `yaml:"write_timeout" default:"3s"`
	PoolSize     int           `yaml:"pool_size" default:"100"`
	MinIdleConns int           `yaml:"min_idle_conns" default:"10"`
	MaxConnAge   time.Duration `yaml:"max_conn_age" default:"0"`
	PoolTimeout  time.Duration `yaml:"pool_timeout" default:"4s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" default:"5m"`
}

// CacheWarmingConfig for proactive cache population
type CacheWarmingConfig struct {
	Enabled              bool          `yaml:"enabled" default:"true"`
	RecentContextsCount  int           `yaml:"recent_contexts_count" default:"100"`
	PopularContextsCount int           `yaml:"popular_contexts_count" default:"50"`
	WarmupInterval       time.Duration `yaml:"warmup_interval" default:"5m"`
}

// CircuitBreakerConfig for graceful degradation
type CircuitBreakerConfig struct {
	Enabled               bool          `yaml:"enabled" default:"true"`
	FailureThreshold      int           `yaml:"failure_threshold" default:"5"`
	SuccessThreshold      int           `yaml:"success_threshold" default:"2"`
	Timeout               time.Duration `yaml:"timeout" default:"60s"`
	MaxConcurrentRequests int           `yaml:"max_concurrent_requests" default:"100"`
}

// MonitoringConfig for performance tracking
type MonitoringConfig struct {
	MetricsEnabled     bool          `yaml:"metrics_enabled" default:"true"`
	TracingEnabled     bool          `yaml:"tracing_enabled" default:"true"`
	SlowQueryThreshold time.Duration `yaml:"slow_query_threshold" default:"100ms"`
	SamplingRate       float64       `yaml:"sampling_rate" default:"0.1"`
}

// DefaultContextManagerConfig returns production-ready default configuration
func DefaultContextManagerConfig() *ContextManagerConfig {
	return &ContextManagerConfig{
		Database: DatabasePoolConfig{
			MaxOpenConns:        50,
			MaxIdleConns:        25,
			ConnMaxLifetime:     5 * time.Minute,
			ConnMaxIdleTime:     90 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		},
		Cache: CacheConfig{
			InMemory: InMemoryCacheConfig{
				Enabled:     true,
				MaxSize:     10000,
				TTL:         5 * time.Minute,
				MaxItemSize: 1048576, // 1MB
			},
			Redis: RedisCacheConfig{
				Enabled:      true,
				TTL:          1 * time.Hour,
				MaxRetries:   3,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
				PoolSize:     100,
				MinIdleConns: 10,
				PoolTimeout:  4 * time.Second,
				IdleTimeout:  5 * time.Minute,
			},
			Warming: CacheWarmingConfig{
				Enabled:              true,
				RecentContextsCount:  100,
				PopularContextsCount: 50,
				WarmupInterval:       5 * time.Minute,
			},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:               true,
			FailureThreshold:      5,
			SuccessThreshold:      2,
			Timeout:               60 * time.Second,
			MaxConcurrentRequests: 100,
		},
		Monitoring: MonitoringConfig{
			MetricsEnabled:     true,
			TracingEnabled:     true,
			SlowQueryThreshold: 100 * time.Millisecond,
			SamplingRate:       0.1,
		},
	}
}
