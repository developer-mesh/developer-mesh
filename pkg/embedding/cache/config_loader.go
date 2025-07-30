package cache

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// LoadConfigFromViper loads semantic cache configuration from viper
func LoadConfigFromViper() (*Config, error) {
	config := DefaultConfig()

	// Check if semantic cache is enabled
	if !viper.GetBool("cache.semantic.enabled") {
		return nil, fmt.Errorf("semantic cache is disabled in configuration")
	}

	// Load mode
	if mode := viper.GetString("cache.semantic.mode"); mode != "" {
		if mode != "legacy" && mode != "tenant_aware" {
			return nil, fmt.Errorf("invalid semantic cache mode: %s", mode)
		}
		// Mode is handled at a higher level, not in base config
	}

	// Load Redis configuration
	if prefix := viper.GetString("cache.semantic.redis.prefix"); prefix != "" {
		config.Prefix = prefix
	}

	if ttl := viper.GetDuration("cache.semantic.redis.ttl"); ttl > 0 {
		config.TTL = ttl * time.Second
	}

	if maxEntries := viper.GetInt("cache.semantic.redis.max_entries"); maxEntries > 0 {
		config.MaxCacheSize = maxEntries
	}

	if compressionEnabled := viper.GetBool("cache.semantic.redis.compression_enabled"); compressionEnabled {
		config.EnableCompression = compressionEnabled
	}

	// Load validation configuration
	_ = viper.GetInt("cache.semantic.validation.max_query_length") // Available for use by validator

	// Load monitoring configuration
	if metricsEnabled := viper.GetBool("monitoring.metrics.enabled"); metricsEnabled {
		config.EnableMetrics = true
	}

	// Load warmup queries if any
	if warmupEnabled := viper.GetBool("cache.semantic.warmup.enabled"); warmupEnabled {
		// Warmup queries would be loaded from a separate source
		config.WarmupQueries = []string{}
	}

	// Load eviction strategy configuration
	if maxCandidates := viper.GetInt("cache.semantic.redis.max_candidates"); maxCandidates > 0 {
		config.MaxCandidates = maxCandidates
	}

	// Similarity threshold could be configured per-tenant or globally
	if threshold := viper.GetFloat64("cache.semantic.similarity_threshold"); threshold > 0 {
		config.SimilarityThreshold = float32(threshold)
	}

	return config, nil
}

// SemanticCacheConfig represents the full configuration for semantic cache
type SemanticCacheConfig struct {
	Enabled        bool                 `mapstructure:"enabled"`
	Mode           string               `mapstructure:"mode"`
	Redis          RedisCacheConfig     `mapstructure:"redis"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	Retry          RetryConfig          `mapstructure:"retry"`
	Validation     ValidationConfig     `mapstructure:"validation"`
	Tenant         TenantConfig         `mapstructure:"tenant"`
	Warmup         WarmupConfig         `mapstructure:"warmup"`
	Monitoring     MonitoringConfig     `mapstructure:"monitoring"`
	Eviction       EvictionConfig       `mapstructure:"eviction"`
}

// RedisCacheConfig represents Redis-specific cache configuration
type RedisCacheConfig struct {
	Prefix             string `mapstructure:"prefix"`
	TTL                int    `mapstructure:"ttl"`
	MaxEntries         int    `mapstructure:"max_entries"`
	MaxMemoryMB        int    `mapstructure:"max_memory_mb"`
	CompressionEnabled bool   `mapstructure:"compression_enabled"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold    int           `mapstructure:"failure_threshold"`
	FailureRatio        float64       `mapstructure:"failure_ratio"`
	ResetTimeout        time.Duration `mapstructure:"reset_timeout"`
	MaxRequestsHalfOpen int           `mapstructure:"max_requests_half_open"`
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	MaxAttempts     int           `mapstructure:"max_attempts"`
	InitialInterval time.Duration `mapstructure:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval"`
	Multiplier      float64       `mapstructure:"multiplier"`
}

// ValidationConfig represents validation configuration
type ValidationConfig struct {
	MaxQueryLength int `mapstructure:"max_query_length"`
	RateLimitRPS   int `mapstructure:"rate_limit_rps"`
	RateLimitBurst int `mapstructure:"rate_limit_burst"`
}

// TenantConfig represents tenant-specific configuration
type TenantConfig struct {
	DefaultMaxEntries int  `mapstructure:"default_max_entries"`
	DefaultTTL        int  `mapstructure:"default_ttl"`
	EncryptionEnabled bool `mapstructure:"encryption_enabled"`
}

// WarmupConfig represents cache warmup configuration
type WarmupConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	Schedule           string `mapstructure:"schedule"`
	BatchSize          int    `mapstructure:"batch_size"`
	ConcurrentRequests int    `mapstructure:"concurrent_requests"`
}

// MonitoringConfig represents monitoring configuration
type MonitoringConfig struct {
	MetricsInterval    time.Duration `mapstructure:"metrics_interval"`
	SlowQueryThreshold time.Duration `mapstructure:"slow_query_threshold"`
}

// EvictionConfig represents eviction configuration
type EvictionConfig struct {
	Strategy      string        `mapstructure:"strategy"`
	CheckInterval time.Duration `mapstructure:"check_interval"`
	BatchSize     int           `mapstructure:"batch_size"`
}

// LoadSemanticCacheConfig loads the complete semantic cache configuration
func LoadSemanticCacheConfig() (*SemanticCacheConfig, error) {
	var config SemanticCacheConfig

	// Load from viper
	if err := viper.UnmarshalKey("cache.semantic", &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal semantic cache config: %w", err)
	}

	// Validate configuration
	if err := validateSemanticCacheConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid semantic cache config: %w", err)
	}

	return &config, nil
}

// validateSemanticCacheConfig validates the semantic cache configuration
func validateSemanticCacheConfig(config *SemanticCacheConfig) error {
	if config.Mode != "legacy" && config.Mode != "tenant_aware" {
		return fmt.Errorf("invalid mode: %s", config.Mode)
	}

	if config.Redis.MaxEntries <= 0 {
		return fmt.Errorf("max_entries must be positive")
	}

	if config.Redis.TTL <= 0 {
		return fmt.Errorf("ttl must be positive")
	}

	if config.CircuitBreaker.FailureThreshold <= 0 {
		return fmt.Errorf("failure_threshold must be positive")
	}

	if config.Retry.MaxAttempts <= 0 {
		return fmt.Errorf("max_attempts must be positive")
	}

	if config.Validation.MaxQueryLength <= 0 {
		return fmt.Errorf("max_query_length must be positive")
	}

	return nil
}
