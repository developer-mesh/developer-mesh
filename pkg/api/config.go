package api

import (
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/interfaces"
	securitytls "github.com/S-Corkum/devops-mcp/pkg/security/tls"
)

// Config holds configuration for the API server
type Config struct {
	ListenAddress string                   `mapstructure:"listen_address"`
	ReadTimeout   time.Duration            `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration            `mapstructure:"write_timeout"`
	IdleTimeout   time.Duration            `mapstructure:"idle_timeout"`
	EnableCORS    bool                     `mapstructure:"enable_cors"`
	EnableSwagger bool                     `mapstructure:"enable_swagger"`
	TLS           *securitytls.Config      `mapstructure:"tls"` // TLS configuration
	Auth          AuthConfig               `mapstructure:"auth"`
	RateLimit     RateLimitConfig          `mapstructure:"rate_limit"`
	Versioning    VersioningConfig         `mapstructure:"versioning"`
	Performance   PerformanceConfig        `mapstructure:"performance"`
	Webhook       interfaces.WebhookConfig `mapstructure:"webhook"`
}

// VersioningConfig holds API versioning configuration
type VersioningConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	DefaultVersion    string   `mapstructure:"default_version"`
	SupportedVersions []string `mapstructure:"supported_versions"`
}

// PerformanceConfig holds configuration for performance optimization
type PerformanceConfig struct {
	// Connection pooling for database
	DBMaxIdleConns    int           `mapstructure:"db_max_idle_conns"`
	DBMaxOpenConns    int           `mapstructure:"db_max_open_conns"`
	DBConnMaxLifetime time.Duration `mapstructure:"db_conn_max_lifetime"`

	// HTTP client settings
	HTTPMaxIdleConns    int           `mapstructure:"http_max_idle_conns"`
	HTTPMaxConnsPerHost int           `mapstructure:"http_max_conns_per_host"`
	HTTPIdleConnTimeout time.Duration `mapstructure:"http_idle_conn_timeout"`

	// Response optimization
	EnableCompression bool `mapstructure:"enable_compression"`
	EnableETagCaching bool `mapstructure:"enable_etag_caching"`

	// Cache control settings
	StaticContentMaxAge  time.Duration `mapstructure:"static_content_max_age"`
	DynamicContentMaxAge time.Duration `mapstructure:"dynamic_content_max_age"`

	// Circuit breaker settings for external services
	CircuitBreakerEnabled bool          `mapstructure:"circuit_breaker_enabled"`
	CircuitBreakerTimeout time.Duration `mapstructure:"circuit_breaker_timeout"`
	MaxRetries            int           `mapstructure:"max_retries"`
	RetryBackoff          time.Duration `mapstructure:"retry_backoff"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret string      `mapstructure:"jwt_secret"`
	APIKeys   interface{} `mapstructure:"api_keys"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Limit       int           `mapstructure:"limit"`
	Period      time.Duration `mapstructure:"period"`
	BurstFactor int           `mapstructure:"burst_factor"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		ListenAddress: ":8080",
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  60 * time.Second,
		IdleTimeout:   120 * time.Second,
		EnableCORS:    true,
		EnableSwagger: true,
		Auth: AuthConfig{
			JWTSecret: "", // Must be provided by user
			APIKeys:   make(map[string]string),
		},
		RateLimit: RateLimitConfig{
			Enabled:     true,
			Limit:       100,
			Period:      time.Minute,
			BurstFactor: 3,
		},
		Versioning: VersioningConfig{
			Enabled:           true,
			DefaultVersion:    "1.0",
			SupportedVersions: []string{"1.0"},
		},
		Performance: PerformanceConfig{
			// Database connection pooling defaults
			DBMaxIdleConns:    10,
			DBMaxOpenConns:    100,
			DBConnMaxLifetime: 30 * time.Minute,

			// HTTP client settings
			HTTPMaxIdleConns:    100,
			HTTPMaxConnsPerHost: 10,
			HTTPIdleConnTimeout: 90 * time.Second,

			// Response optimization
			EnableCompression: true,
			EnableETagCaching: true,

			// Cache control settings
			StaticContentMaxAge:  24 * time.Hour,
			DynamicContentMaxAge: 5 * time.Minute,

			// Circuit breaker settings
			CircuitBreakerEnabled: true,
			CircuitBreakerTimeout: 30 * time.Second,
			MaxRetries:            3,
			RetryBackoff:          500 * time.Millisecond,
		},
		Webhook: interfaces.WebhookConfig{
			EnabledField:             false,
			GitHubEndpointField:      "/api/webhooks/github",
			GitHubSecretField:        "",
			GitHubIPValidationField:  true,
			GitHubAllowedEventsField: []string{"push", "pull_request", "issues", "issue_comment", "release"},
		},
	}
}
