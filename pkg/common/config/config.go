package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/metrics"
	"github.com/spf13/viper"
)

// APIConfig defines the API server configuration
type APIConfig struct {
	ListenAddress  string         `mapstructure:"listen_address"`
	BaseURL        string         `mapstructure:"base_url"`
	TLSCertFile    string         `mapstructure:"tls_cert_file"`
	TLSKeyFile     string         `mapstructure:"tls_key_file"`
	CORSAllowed    string         `mapstructure:"cors_allowed"`
	RateLimit      map[string]any `mapstructure:"rate_limit"`
	RequestTimeout int            `mapstructure:"request_timeout"`
	ReadTimeout    time.Duration  `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration  `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration  `mapstructure:"idle_timeout"`
	EnableCORS     bool           `mapstructure:"enable_cors"`
	EnableSwagger  bool           `mapstructure:"enable_swagger"`
	Auth           map[string]any `mapstructure:"auth"`
	Webhook        map[string]any `mapstructure:"webhook"`
}

// CoreConfig defines the engine core configuration
type CoreConfig struct {
	EventBufferSize  int           `mapstructure:"event_buffer_size"`
	ConcurrencyLimit int           `mapstructure:"concurrency_limit"`
	EventTimeout     time.Duration `mapstructure:"event_timeout"`
}

// Config holds the complete application configuration
type Config struct {
	API         APIConfig         `mapstructure:"api"`
	Cache       cache.RedisConfig `mapstructure:"cache"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Engine      CoreConfig        `mapstructure:"engine"`
	Metrics     metrics.Config    `mapstructure:"metrics"`
	AWS         AWSConfig         `mapstructure:"aws"`
	Environment string            `mapstructure:"environment"`
	Adapters    map[string]any    `mapstructure:"adapters"`
	WebSocket   *WebSocketConfig  `mapstructure:"websocket"`
	MCPServer   *MCPServerConfig  `mapstructure:"mcp_server"`
	Embedding   EmbeddingConfig   `mapstructure:"embedding"`
}

// RestAPIConfig holds configuration for connecting to REST API service
type RestAPIConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RetryCount int           `mapstructure:"retry_count"`
}

// MCPServerConfig holds MCP-specific configuration overrides
type MCPServerConfig struct {
	ListenAddress string        `mapstructure:"listen_address"`
	RestAPI       RestAPIConfig `mapstructure:"rest_api"`
}

// WebSocketConfig holds WebSocket server configuration
type WebSocketConfig struct {
	Enabled         bool                      `mapstructure:"enabled"`
	MaxConnections  int                       `mapstructure:"max_connections"`
	ReadBufferSize  int                       `mapstructure:"read_buffer_size"`
	WriteBufferSize int                       `mapstructure:"write_buffer_size"`
	PingInterval    time.Duration             `mapstructure:"ping_interval"`
	PongTimeout     time.Duration             `mapstructure:"pong_timeout"`
	MaxMessageSize  int64                     `mapstructure:"max_message_size"`
	Security        *WebSocketSecurityConfig  `mapstructure:"security"`
	RateLimit       *WebSocketRateLimitConfig `mapstructure:"rate_limit"`
}

// WebSocketSecurityConfig holds WebSocket security configuration
type WebSocketSecurityConfig struct {
	RequireAuth    bool     `mapstructure:"require_auth"`
	HMACSignatures bool     `mapstructure:"hmac_signatures"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// WebSocketRateLimitConfig holds WebSocket rate limiting configuration
type WebSocketRateLimitConfig struct {
	Rate    float64 `mapstructure:"rate"`
	Burst   int     `mapstructure:"burst"`
	PerIP   bool    `mapstructure:"per_ip"`
	PerUser bool    `mapstructure:"per_user"`
}

// AWSConfig holds configuration for AWS services
type AWSConfig struct {
	RDS         aws.RDSConfig         `mapstructure:"rds"`
	ElastiCache aws.ElastiCacheConfig `mapstructure:"elasticache"`
	S3          aws.S3Config          `mapstructure:"s3"`
}

// EmbeddingConfig contains configuration for the embedding system
type EmbeddingConfig struct {
	Providers ProvidersConfig `mapstructure:"providers"`
}

// ProvidersConfig contains configuration for embedding providers
type ProvidersConfig struct {
	OpenAI  OpenAIConfig  `mapstructure:"openai"`
	Bedrock BedrockConfig `mapstructure:"bedrock"`
	Google  GoogleConfig  `mapstructure:"google"`
}

// OpenAIConfig contains OpenAI provider configuration
type OpenAIConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	APIKey  string `mapstructure:"api_key"`
}

// BedrockConfig contains AWS Bedrock provider configuration
type BedrockConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Region   string `mapstructure:"region"`
	Endpoint string `mapstructure:"endpoint"`
}

// GoogleConfig contains Google provider configuration
type GoogleConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	APIKey   string `mapstructure:"api_key"`
	Endpoint string `mapstructure:"endpoint"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// Initialize configuration
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from config file
	configFile := os.Getenv("MCP_CONFIG_FILE")
	if configFile == "" {
		configFile = "configs/config.yaml"
	}

	v.SetConfigFile(configFile)

	// Read from environment variables prefixed with MCP_
	v.SetEnvPrefix("MCP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific environment variables that don't follow the MCP_ prefix
	// These are commonly used in Docker environments
	_ = v.BindEnv("cache.address", "REDIS_ADDR")      // Best effort - viper handles errors internally
	_ = v.BindEnv("cache.address", "CACHE_ADDRESS")   // Best effort - viper handles errors internally
	_ = v.BindEnv("cache.username", "REDIS_USERNAME") // Redis 6.0+ ACL username
	_ = v.BindEnv("cache.password", "REDIS_PASSWORD") // Redis password

	// Bind database environment variables used by Docker
	_ = v.BindEnv("database.host", "DATABASE_HOST")
	_ = v.BindEnv("database.port", "DATABASE_PORT")
	_ = v.BindEnv("database.database", "DATABASE_NAME")
	_ = v.BindEnv("database.username", "DATABASE_USER")
	_ = v.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("database.ssl_mode", "DATABASE_SSL_MODE")
	_ = v.BindEnv("database.dsn", "DATABASE_DSN")

	// Enable environment variable interpolation in config values
	v.AllowEmptyEnv(true)

	// Read config
	if err := v.ReadInConfig(); err != nil {
		// Config file is not required if environment variables are set
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Process environment variable expansions in the config file
	// This allows using ${VAR} or ${VAR:-default} syntax in config values
	processEnvExpansion(v)

	// Unmarshal config
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// processEnvExpansion processes environment variable expansions in config values
// Supports ${VAR} and ${VAR:-default} syntax
func processEnvExpansion(v *viper.Viper) {
	// Get all keys in the config
	keys := v.AllKeys()

	// Process each key
	for _, key := range keys {
		// Get the value as a string
		value := v.GetString(key)

		// Skip empty values
		if value == "" {
			continue
		}

		// Look for environment variable references in the value
		if strings.Contains(value, "${") && strings.Contains(value, "}") {
			// Process the value for environment variable expansion
			expandedValue := expandEnvVars(value)

			// If the value changed, update it in Viper
			if expandedValue != value {
				v.Set(key, expandedValue)
			}
		}
	}
}

// expandEnvVars expands environment variables in a string
// Supports ${VAR} and ${VAR:-default} syntax
func expandEnvVars(value string) string {
	result := value

	// Find all environment variable references
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}

		end := strings.Index(result[start:], "}") + start
		if end == -1 {
			break
		}

		// Extract the variable reference
		varRef := result[start+2 : end]

		// Check if there's a default value
		var envVar, defaultVal string
		if strings.Contains(varRef, ":-") {
			parts := strings.SplitN(varRef, ":-", 2)
			envVar = parts[0]
			defaultVal = parts[1]
		} else {
			envVar = varRef
		}

		// Get the environment variable value
		envVal := os.Getenv(envVar)

		// Use default if env var is not set
		if envVal == "" && defaultVal != "" {
			envVal = defaultVal
		}

		// Replace the variable reference in the string
		result = result[:start] + envVal + result[end+1:]
	}

	return result
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Environment (dev, staging, prod)
	v.SetDefault("environment", "dev")

	// API defaults
	v.SetDefault("api.listen_address", ":8080")
	v.SetDefault("api.read_timeout", 30*time.Second)
	v.SetDefault("api.write_timeout", 30*time.Second)
	v.SetDefault("api.idle_timeout", 90*time.Second)
	v.SetDefault("api.base_path", "/api/v1")
	v.SetDefault("api.enable_cors", true)
	v.SetDefault("api.cors_origins", []string{"http://localhost:3000"})
	v.SetDefault("api.log_requests", true)

	// TLS defaults (empty means no TLS)
	v.SetDefault("api.tls_cert_file", "")
	v.SetDefault("api.tls_key_file", "")

	// API rate limiting defaults
	v.SetDefault("api.rate_limit.enabled", true)
	v.SetDefault("api.rate_limit.limit", 100)
	v.SetDefault("api.rate_limit.burst", 150)
	v.SetDefault("api.rate_limit.expiration", 1*time.Hour)

	// Auth defaults - No default values for secrets
	v.SetDefault("api.auth.require_auth", true)
	v.SetDefault("api.auth.jwt_expiration", 24*time.Hour)
	v.SetDefault("api.auth.token_renewal_threshold", 1*time.Hour)

	// Database defaults
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)
	v.SetDefault("database.use_aws", true) // Default to using AWS RDS
	v.SetDefault("database.use_iam", true) // Default to using IAM authentication

	// Cache defaults
	v.SetDefault("cache.type", "redis")
	v.SetDefault("cache.address", "localhost:6379")
	v.SetDefault("cache.max_retries", 3)
	v.SetDefault("cache.dial_timeout", 5)
	v.SetDefault("cache.read_timeout", 3)
	v.SetDefault("cache.write_timeout", 3)
	v.SetDefault("cache.pool_size", 10)
	v.SetDefault("cache.min_idle_conns", 2)
	v.SetDefault("cache.pool_timeout", 4)
	v.SetDefault("cache.use_iam_auth", true) // Default to using IAM authentication for Redis

	// Engine defaults
	v.SetDefault("engine.event_buffer_size", 1000)
	v.SetDefault("engine.concurrency_limit", 5)
	v.SetDefault("engine.event_timeout", 30*time.Second)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.type", "prometheus")
	v.SetDefault("metrics.push_interval", 10*time.Second)

	// AWS defaults
	// Get AWS region from environment variable or use default
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		awsRegion = "us-west-2"
	}

	// S3 defaults
	v.SetDefault("aws.s3.auth.region", awsRegion)
	v.SetDefault("aws.s3.upload_part_size", 5*1024*1024)   // 5MB
	v.SetDefault("aws.s3.download_part_size", 5*1024*1024) // 5MB
	v.SetDefault("aws.s3.concurrency", 5)
	v.SetDefault("aws.s3.request_timeout", 30*time.Second)
	v.SetDefault("aws.s3.server_side_encryption", "AES256")
	v.SetDefault("aws.s3.use_iam_auth", true) // Default to IAM authentication

	// RDS defaults
	v.SetDefault("aws.rds.auth.region", awsRegion)
	v.SetDefault("aws.rds.port", 5432)
	v.SetDefault("aws.rds.database", "mcp")
	v.SetDefault("aws.rds.use_iam_auth", true)      // Always use IAM authentication by default
	v.SetDefault("aws.rds.token_expiration", 15*60) // 15 minutes in seconds
	v.SetDefault("aws.rds.max_open_conns", 25)
	v.SetDefault("aws.rds.max_idle_conns", 5)
	v.SetDefault("aws.rds.conn_max_lifetime", 5*time.Minute)
	v.SetDefault("aws.rds.enable_pooling", true)
	v.SetDefault("aws.rds.min_pool_size", 2)
	v.SetDefault("aws.rds.max_pool_size", 10)
	v.SetDefault("aws.rds.connection_timeout", 30)

	// ElastiCache defaults
	v.SetDefault("aws.elasticache.auth.region", awsRegion)
	v.SetDefault("aws.elasticache.port", 6379)
	v.SetDefault("aws.elasticache.use_iam_auth", true) // Always use IAM authentication by default
	v.SetDefault("aws.elasticache.cluster_mode", true)
	v.SetDefault("aws.elasticache.cluster_discovery", true)
	v.SetDefault("aws.elasticache.use_tls", true)
	v.SetDefault("aws.elasticache.insecure_skip_verify", false)
	v.SetDefault("aws.elasticache.max_retries", 3)
	v.SetDefault("aws.elasticache.min_idle_connections", 2)
	v.SetDefault("aws.elasticache.pool_size", 10)
	v.SetDefault("aws.elasticache.dial_timeout", 5)
	v.SetDefault("aws.elasticache.read_timeout", 3)
	v.SetDefault("aws.elasticache.write_timeout", 3)
	v.SetDefault("aws.elasticache.pool_timeout", 4)
	v.SetDefault("aws.elasticache.token_expiration", 15*60) // 15 minutes in seconds

	// Context storage defaults
	v.SetDefault("storage.context_storage.s3_path_prefix", "contexts")
}

// IsProduction returns true if the environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == "prod" || c.Environment == "production"
}

// IsDevelopment returns true if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "dev" || c.Environment == "development"
}

// IsStaging returns true if the environment is staging
func (c *Config) IsStaging() bool {
	return c.Environment == "staging" || c.Environment == "stage"
}

// GetListenPort returns the port number the API should listen on
func (c *Config) GetListenPort() int {
	// Parse port from listen address (format ":8080")
	addr := c.API.ListenAddress

	// If in production, use port 443 for HTTPS
	if c.IsProduction() && c.API.TLSCertFile != "" && c.API.TLSKeyFile != "" {
		return 443
	}

	// Otherwise parse from listen address or use 8080 as default
	port := 8080
	if addr != "" && strings.HasPrefix(addr, ":") {
		if _, err := fmt.Sscanf(addr[1:], "%d", &port); err != nil {
			// If parsing fails, use default port
			port = 8080
		}
	}

	return port
}
