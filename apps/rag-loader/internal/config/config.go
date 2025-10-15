// Package config handles configuration for the RAG loader service
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete configuration for the RAG loader
type Config struct {
	Service      ServiceConfig      `mapstructure:"service"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Sources      []SourceConfig     `mapstructure:"sources"`
	Processing   ProcessingConfig   `mapstructure:"processing"`
	Search       SearchConfig       `mapstructure:"search"`
	RateLimiting RateLimitingConfig `mapstructure:"rate_limiting"`
	Scheduler    SchedulerConfig    `mapstructure:"scheduler"`
}

// ServiceConfig contains service-level configuration
type ServiceConfig struct {
	Port                int           `mapstructure:"port"`
	MetricsPort         int           `mapstructure:"metrics_port"`
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	ShutdownTimeout     time.Duration `mapstructure:"shutdown_timeout"`
	LogLevel            string        `mapstructure:"log_level"`
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Database     string `mapstructure:"database"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	SSLMode      string `mapstructure:"ssl_mode"`
	SearchPath   string `mapstructure:"search_path"`
	MaxConns     int    `mapstructure:"max_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// RedisConfig contains Redis connection settings
type RedisConfig struct {
	Address     string        `mapstructure:"address"`
	Password    string        `mapstructure:"password"`
	Database    int           `mapstructure:"database"`
	MaxRetries  int           `mapstructure:"max_retries"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
	PoolSize    int           `mapstructure:"pool_size"`
}

// SourceConfig represents configuration for a data source
type SourceConfig struct {
	ID       string                 `mapstructure:"id"`
	Type     string                 `mapstructure:"type"`
	Enabled  bool                   `mapstructure:"enabled"`
	Schedule string                 `mapstructure:"schedule"`
	Config   map[string]interface{} `mapstructure:"config"`
}

// ProcessingConfig contains document processing settings
type ProcessingConfig struct {
	ChunkingStrategy string               `mapstructure:"chunking_strategy"`
	ChunkSize        int                  `mapstructure:"chunk_size"`
	ChunkOverlap     int                  `mapstructure:"chunk_overlap"`
	Embedding        EmbeddingConfig      `mapstructure:"embedding"`
	CircuitBreaker   CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

// EmbeddingConfig contains embedding generation settings
type EmbeddingConfig struct {
	BatchSize     int           `mapstructure:"batch_size"`
	RateLimitRPM  int           `mapstructure:"rate_limit_rpm"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
	RetryDelay    time.Duration `mapstructure:"retry_delay"`
	ModelOverride string        `mapstructure:"model_override"`
}

// CircuitBreakerConfig contains circuit breaker settings
type CircuitBreakerConfig struct {
	FailureThreshold    int           `mapstructure:"failure_threshold"`
	SuccessThreshold    int           `mapstructure:"success_threshold"`
	Timeout             time.Duration `mapstructure:"timeout"`
	HalfOpenMaxRequests int           `mapstructure:"half_open_max_requests"`
}

// SearchConfig contains hybrid search settings
type SearchConfig struct {
	VectorWeight     float64 `mapstructure:"vector_weight"`
	BM25Weight       float64 `mapstructure:"bm25_weight"`
	ImportanceWeight float64 `mapstructure:"importance_weight"`
	MMRLambda        float64 `mapstructure:"mmr_lambda"`
	MinScore         float64 `mapstructure:"min_score"`
	DefaultLimit     int     `mapstructure:"default_limit"`
	MaxCandidates    int     `mapstructure:"max_candidates"`
	ApplyMMR         bool    `mapstructure:"apply_mmr"`
}

// RateLimitingConfig contains rate limiting settings
type RateLimitingConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	EmbeddingRPM    int  `mapstructure:"embedding_rpm"`
	SearchRPM       int  `mapstructure:"search_rpm"`
	APIRPM          int  `mapstructure:"api_rpm"`
	BurstMultiplier int  `mapstructure:"burst_multiplier"`
}

// SchedulerConfig contains scheduler settings
type SchedulerConfig struct {
	DefaultSchedule   string        `mapstructure:"default_schedule"`
	JobTimeout        time.Duration `mapstructure:"job_timeout"`
	EnableAPI         bool          `mapstructure:"enable_api"`
	EnableEvents      bool          `mapstructure:"enable_events"`
	MaxConcurrentJobs int           `mapstructure:"max_concurrent_jobs"`
}

// Load loads configuration from environment and config files
func Load() (*Config, error) {
	viper.SetConfigName("rag-loader")
	viper.SetConfigType("yaml")

	// Add config paths
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("./apps/rag-loader/configs")
	viper.AddConfigPath("/configs")

	// Set defaults
	setDefaults()

	// Bind environment variables
	bindEnvVars()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we'll use defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Override with environment variables
	overrideFromEnv(&config)

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	// Service defaults
	viper.SetDefault("service.port", 8084)
	viper.SetDefault("service.metrics_port", 9094)
	viper.SetDefault("service.health_check_interval", "30s")
	viper.SetDefault("service.shutdown_timeout", "30s")
	viper.SetDefault("service.log_level", "info")

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.database", "devmesh_development")
	viper.SetDefault("database.username", "devmesh")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.search_path", "rag,mcp,public")
	viper.SetDefault("database.max_conns", 10)
	viper.SetDefault("database.max_idle_conns", 5)

	// Redis defaults
	viper.SetDefault("redis.address", "localhost:6379")
	viper.SetDefault("redis.database", 0)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.pool_size", 10)

	// Processing defaults
	viper.SetDefault("processing.chunking_strategy", "fixed")
	viper.SetDefault("processing.chunk_size", 500)
	viper.SetDefault("processing.chunk_overlap", 50)

	// Embedding defaults (Phase 4)
	viper.SetDefault("processing.embedding.batch_size", 10)
	viper.SetDefault("processing.embedding.rate_limit_rpm", 100)
	viper.SetDefault("processing.embedding.retry_attempts", 3)
	viper.SetDefault("processing.embedding.retry_delay", "1s")
	viper.SetDefault("processing.embedding.model_override", "")

	// Circuit breaker defaults (Phase 4)
	viper.SetDefault("processing.circuit_breaker.failure_threshold", 5)
	viper.SetDefault("processing.circuit_breaker.success_threshold", 2)
	viper.SetDefault("processing.circuit_breaker.timeout", "30s")
	viper.SetDefault("processing.circuit_breaker.half_open_max_requests", 3)

	// Search defaults (Phase 4)
	viper.SetDefault("search.vector_weight", 0.6)
	viper.SetDefault("search.bm25_weight", 0.2)
	viper.SetDefault("search.importance_weight", 0.2)
	viper.SetDefault("search.mmr_lambda", 0.7)
	viper.SetDefault("search.min_score", 0.4)
	viper.SetDefault("search.default_limit", 20)
	viper.SetDefault("search.max_candidates", 100)
	viper.SetDefault("search.apply_mmr", true)

	// Rate limiting defaults (Phase 4)
	viper.SetDefault("rate_limiting.enabled", true)
	viper.SetDefault("rate_limiting.embedding_rpm", 100)
	viper.SetDefault("rate_limiting.search_rpm", 200)
	viper.SetDefault("rate_limiting.api_rpm", 50)
	viper.SetDefault("rate_limiting.burst_multiplier", 2)

	// Scheduler defaults
	viper.SetDefault("scheduler.default_schedule", "*/30 * * * *") // Every 30 minutes
	viper.SetDefault("scheduler.job_timeout", "10m")
	viper.SetDefault("scheduler.enable_api", true)
	viper.SetDefault("scheduler.enable_events", true)
	viper.SetDefault("scheduler.max_concurrent_jobs", 3)
}

// bindEnvVars binds environment variables to configuration keys
func bindEnvVars() {
	viper.AutomaticEnv()

	// Service bindings
	_ = viper.BindEnv("service.port", "RAG_LOADER_PORT")
	_ = viper.BindEnv("service.log_level", "LOG_LEVEL")

	// Database bindings
	_ = viper.BindEnv("database.host", "DATABASE_HOST")
	_ = viper.BindEnv("database.port", "DATABASE_PORT")
	_ = viper.BindEnv("database.database", "DATABASE_NAME")
	_ = viper.BindEnv("database.username", "DATABASE_USER")
	_ = viper.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = viper.BindEnv("database.ssl_mode", "DATABASE_SSL_MODE")

	// Redis bindings
	_ = viper.BindEnv("redis.address", "REDIS_ADDR")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")

	// Scheduler bindings
	_ = viper.BindEnv("scheduler.default_schedule", "DEFAULT_SCHEDULE")
}

// overrideFromEnv overrides configuration with environment variables
func overrideFromEnv(cfg *Config) {
	// Override database from DATABASE_URL if provided
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		// TODO: Parse DATABASE_URL and override individual settings
		// Format: postgresql://user:pass@host:port/database
		// This is a simplified implementation - enhance as needed
		_ = dbURL
	}

	// Override Redis from REDIS_URL if provided
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		// TODO: Parse REDIS_URL and override individual settings
		// Format: redis://[:password@]host[:port][/database]
		// This is a simplified implementation - enhance as needed
		_ = redisURL
	}
}

// validate validates the configuration
func validate(cfg *Config) error {
	if cfg.Service.Port <= 0 || cfg.Service.Port > 65535 {
		return fmt.Errorf("invalid service port: %d", cfg.Service.Port)
	}

	// Database.Host and Database.Database will have defaults from viper
	// So we can't validate them as required fields since they'll never be empty
	// We can check specific environment scenarios if needed

	// Redis.Address will also have a default from viper
	// So we'll accept the defaults

	return nil
}
