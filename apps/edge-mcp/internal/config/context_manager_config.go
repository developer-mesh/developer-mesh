package config

import "time"

// ContextManagerConfig provides configuration for Edge MCP context management
// Note: Edge MCP uses only in-memory storage and syncs with Core Platform via API
type ContextManagerConfig struct {
	// In-memory cache configuration
	Cache ContextCacheConfig `yaml:"cache"`

	// Circuit breaker settings for Core Platform API calls
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`

	// Performance monitoring settings
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

// Note: Edge MCP does not use direct database connections
// All persistent state is managed through Core Platform API

// ContextCacheConfig configures caching strategy for Edge MCP (in-memory only)
type ContextCacheConfig struct {
	// In-memory LRU cache settings
	InMemory InMemoryCacheConfig `yaml:"in_memory"`

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

// Note: Edge MCP uses only in-memory caching
// Any distributed state is synchronized through Core Platform API, not direct Redis access

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

// DefaultContextManagerConfig returns production-ready default configuration for Edge MCP
func DefaultContextManagerConfig() *ContextManagerConfig {
	return &ContextManagerConfig{
		Cache: ContextCacheConfig{
			InMemory: InMemoryCacheConfig{
				Enabled:     true,
				MaxSize:     10000,
				TTL:         5 * time.Minute,
				MaxItemSize: 1048576, // 1MB
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
