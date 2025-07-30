package cache

import (
	"time"
)

// PerformanceProfile represents predefined performance configurations
type PerformanceProfile string

const (
	// ProfileLowLatency optimizes for minimal response time
	ProfileLowLatency PerformanceProfile = "low-latency"
	// ProfileHighThroughput optimizes for maximum throughput
	ProfileHighThroughput PerformanceProfile = "high-throughput"
	// ProfileBalanced provides a balance between latency and throughput
	ProfileBalanced PerformanceProfile = "balanced"
	// ProfileCustom allows custom configuration
	ProfileCustom PerformanceProfile = "custom"
)

// PerformanceConfig contains all performance-related parameters
type PerformanceConfig struct {
	// Cache operation timeouts
	GetTimeout    time.Duration `json:"get_timeout" yaml:"get_timeout"`
	SetTimeout    time.Duration `json:"set_timeout" yaml:"set_timeout"`
	DeleteTimeout time.Duration `json:"delete_timeout" yaml:"delete_timeout"`

	// Batch processing
	BatchSize     int           `json:"batch_size" yaml:"batch_size"`
	FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`

	// Compression
	CompressionThreshold int    `json:"compression_threshold" yaml:"compression_threshold"` // bytes
	CompressionLevel     string `json:"compression_level" yaml:"compression_level"`         // none, fast, best

	// Similarity search
	MaxCandidates       int           `json:"max_candidates" yaml:"max_candidates"`
	SimilarityThreshold float32       `json:"similarity_threshold" yaml:"similarity_threshold"`
	VectorSearchTimeout time.Duration `json:"vector_search_timeout" yaml:"vector_search_timeout"`

	// LRU tracking
	TrackingBufferSize int           `json:"tracking_buffer_size" yaml:"tracking_buffer_size"`
	TrackingBatchSize  int           `json:"tracking_batch_size" yaml:"tracking_batch_size"`
	EvictionBatchSize  int           `json:"eviction_batch_size" yaml:"eviction_batch_size"`
	EvictionInterval   time.Duration `json:"eviction_interval" yaml:"eviction_interval"`

	// Circuit breaker
	CircuitBreakerEnabled      bool          `json:"circuit_breaker_enabled" yaml:"circuit_breaker_enabled"`
	CircuitBreakerThreshold    int           `json:"circuit_breaker_threshold" yaml:"circuit_breaker_threshold"`
	CircuitBreakerTimeout      time.Duration `json:"circuit_breaker_timeout" yaml:"circuit_breaker_timeout"`
	CircuitBreakerResetTimeout time.Duration `json:"circuit_breaker_reset_timeout" yaml:"circuit_breaker_reset_timeout"`

	// Retry policy
	RetryEnabled      bool          `json:"retry_enabled" yaml:"retry_enabled"`
	RetryMaxAttempts  int           `json:"retry_max_attempts" yaml:"retry_max_attempts"`
	RetryInitialDelay time.Duration `json:"retry_initial_delay" yaml:"retry_initial_delay"`
	RetryMaxDelay     time.Duration `json:"retry_max_delay" yaml:"retry_max_delay"`
	RetryMultiplier   float64       `json:"retry_multiplier" yaml:"retry_multiplier"`

	// Connection pooling (extends RedisPoolConfig)
	ConnectionPoolProfile string `json:"connection_pool_profile" yaml:"connection_pool_profile"`
}

// GetPerformanceProfile returns a predefined performance configuration.
// Each profile is optimized for different use cases:
//
//   - ProfileLowLatency: Minimal response time, smaller batches, aggressive timeouts
//   - ProfileHighThroughput: Maximum throughput, larger batches, relaxed timeouts
//   - ProfileBalanced: Good balance for general use cases
//   - ProfileCustom: Returns base configuration for customization
//
// The returned configuration can be further customized as needed.
func GetPerformanceProfile(profile PerformanceProfile) *PerformanceConfig {
	switch profile {
	case ProfileLowLatency:
		return &PerformanceConfig{
			// Aggressive timeouts for fast failure
			GetTimeout:    500 * time.Millisecond,
			SetTimeout:    1 * time.Second,
			DeleteTimeout: 500 * time.Millisecond,

			// Small batches for quick processing
			BatchSize:     10,
			FlushInterval: 50 * time.Millisecond,

			// No compression for speed
			CompressionThreshold: 10240, // 10KB
			CompressionLevel:     "none",

			// Limited search for speed
			MaxCandidates:       5,
			SimilarityThreshold: 0.95,
			VectorSearchTimeout: 200 * time.Millisecond,

			// Small buffers for low memory
			TrackingBufferSize: 100,
			TrackingBatchSize:  10,
			EvictionBatchSize:  10,
			EvictionInterval:   5 * time.Minute,

			// Aggressive circuit breaker
			CircuitBreakerEnabled:      true,
			CircuitBreakerThreshold:    3,
			CircuitBreakerTimeout:      100 * time.Millisecond,
			CircuitBreakerResetTimeout: 5 * time.Second,

			// Limited retries
			RetryEnabled:      true,
			RetryMaxAttempts:  2,
			RetryInitialDelay: 50 * time.Millisecond,
			RetryMaxDelay:     200 * time.Millisecond,
			RetryMultiplier:   2.0,

			ConnectionPoolProfile: "low-latency",
		}

	case ProfileHighThroughput:
		return &PerformanceConfig{
			// Relaxed timeouts for reliability
			GetTimeout:    3 * time.Second,
			SetTimeout:    5 * time.Second,
			DeleteTimeout: 3 * time.Second,

			// Large batches for efficiency
			BatchSize:     100,
			FlushInterval: 500 * time.Millisecond,

			// Aggressive compression
			CompressionThreshold: 1024, // 1KB
			CompressionLevel:     "best",

			// Thorough search
			MaxCandidates:       20,
			SimilarityThreshold: 0.90,
			VectorSearchTimeout: 2 * time.Second,

			// Large buffers
			TrackingBufferSize: 10000,
			TrackingBatchSize:  100,
			EvictionBatchSize:  50,
			EvictionInterval:   1 * time.Minute,

			// Relaxed circuit breaker
			CircuitBreakerEnabled:      true,
			CircuitBreakerThreshold:    10,
			CircuitBreakerTimeout:      1 * time.Second,
			CircuitBreakerResetTimeout: 30 * time.Second,

			// More retries
			RetryEnabled:      true,
			RetryMaxAttempts:  5,
			RetryInitialDelay: 100 * time.Millisecond,
			RetryMaxDelay:     5 * time.Second,
			RetryMultiplier:   2.0,

			ConnectionPoolProfile: "high-load",
		}

	case ProfileBalanced:
		fallthrough
	default:
		return &PerformanceConfig{
			// Balanced timeouts
			GetTimeout:    1 * time.Second,
			SetTimeout:    2 * time.Second,
			DeleteTimeout: 1 * time.Second,

			// Medium batches
			BatchSize:     50,
			FlushInterval: 200 * time.Millisecond,

			// Selective compression
			CompressionThreshold: 4096, // 4KB
			CompressionLevel:     "fast",

			// Reasonable search
			MaxCandidates:       10,
			SimilarityThreshold: 0.93,
			VectorSearchTimeout: 500 * time.Millisecond,

			// Medium buffers
			TrackingBufferSize: 1000,
			TrackingBatchSize:  50,
			EvictionBatchSize:  25,
			EvictionInterval:   2 * time.Minute,

			// Standard circuit breaker
			CircuitBreakerEnabled:      true,
			CircuitBreakerThreshold:    5,
			CircuitBreakerTimeout:      500 * time.Millisecond,
			CircuitBreakerResetTimeout: 15 * time.Second,

			// Standard retries
			RetryEnabled:      true,
			RetryMaxAttempts:  3,
			RetryInitialDelay: 100 * time.Millisecond,
			RetryMaxDelay:     2 * time.Second,
			RetryMultiplier:   2.0,

			ConnectionPoolProfile: "default",
		}
	}
}

// ApplyProfile applies a performance profile to the cache configuration
func (c *Config) ApplyPerformanceProfile(profile PerformanceProfile) {
	perfConfig := GetPerformanceProfile(profile)

	// Apply timeouts
	// Note: These would need to be added to the Config struct
	c.MaxCandidates = perfConfig.MaxCandidates
	c.SimilarityThreshold = perfConfig.SimilarityThreshold

	// Apply Redis pool config based on profile
	switch perfConfig.ConnectionPoolProfile {
	case "low-latency":
		c.RedisPoolConfig = LowLatencyRedisPoolConfig()
	case "high-load":
		c.RedisPoolConfig = HighLoadRedisPoolConfig()
	default:
		c.RedisPoolConfig = DefaultRedisPoolConfig()
	}
}

// Validate checks if the performance configuration is valid
func (pc *PerformanceConfig) Validate() error {
	if pc.GetTimeout <= 0 {
		pc.GetTimeout = 1 * time.Second
	}
	if pc.SetTimeout <= 0 {
		pc.SetTimeout = 2 * time.Second
	}
	if pc.DeleteTimeout <= 0 {
		pc.DeleteTimeout = 1 * time.Second
	}

	if pc.BatchSize <= 0 {
		pc.BatchSize = 50
	}
	if pc.FlushInterval <= 0 {
		pc.FlushInterval = 200 * time.Millisecond
	}

	if pc.CompressionThreshold <= 0 {
		pc.CompressionThreshold = 4096
	}

	if pc.MaxCandidates <= 0 {
		pc.MaxCandidates = 10
	}
	if pc.SimilarityThreshold <= 0 || pc.SimilarityThreshold > 1 {
		pc.SimilarityThreshold = 0.93
	}

	if pc.TrackingBufferSize <= 0 {
		pc.TrackingBufferSize = 1000
	}
	if pc.TrackingBatchSize <= 0 {
		pc.TrackingBatchSize = 50
	}
	if pc.EvictionBatchSize <= 0 {
		pc.EvictionBatchSize = 25
	}
	if pc.EvictionInterval <= 0 {
		pc.EvictionInterval = 2 * time.Minute
	}

	if pc.RetryMaxAttempts < 0 {
		pc.RetryMaxAttempts = 3
	}
	if pc.RetryInitialDelay <= 0 {
		pc.RetryInitialDelay = 100 * time.Millisecond
	}
	if pc.RetryMaxDelay <= 0 {
		pc.RetryMaxDelay = 2 * time.Second
	}
	if pc.RetryMultiplier <= 1 {
		pc.RetryMultiplier = 2.0
	}

	return nil
}
