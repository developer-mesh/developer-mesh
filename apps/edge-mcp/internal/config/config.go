package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents the Edge MCP configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Core      CoreConfig      `yaml:"core"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port int `yaml:"port"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	APIKey string `yaml:"api_key"`
}

// CoreConfig represents Core Platform configuration
type CoreConfig struct {
	URL       string `yaml:"url"`
	APIKey    string `yaml:"api_key"`
	EdgeMCPID string `yaml:"edge_mcp_id"`
	// TenantID is determined from the API key, not needed as separate config
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	// Global limits
	GlobalRPS   int `yaml:"global_rps"`   // Requests per second globally
	GlobalBurst int `yaml:"global_burst"` // Burst size globally

	// Per-tenant limits
	TenantRPS   int `yaml:"tenant_rps"`   // Requests per second per tenant
	TenantBurst int `yaml:"tenant_burst"` // Burst size per tenant

	// Per-tool limits
	ToolRPS   int `yaml:"tool_rps"`   // Requests per second per tool
	ToolBurst int `yaml:"tool_burst"` // Burst size per tool

	// Quota management
	EnableQuotas       bool          `yaml:"enable_quotas"`        // Enable quota tracking
	QuotaResetInterval time.Duration `yaml:"quota_reset_interval"` // How often quotas reset
	DefaultQuota       int64         `yaml:"default_quota"`        // Default quota per tenant

	// Cleanup
	CleanupInterval time.Duration `yaml:"cleanup_interval"` // How often to clean up old limiters
	MaxAge          time.Duration `yaml:"max_age"`          // Maximum age for unused limiters
}

// Load loads configuration from file or environment
func Load(configFile string) (*Config, error) {
	// For Edge MCP, we primarily use environment variables and defaults
	// Config file is optional
	return Default(), nil
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8082,
		},
		Auth: AuthConfig{
			APIKey: getEnv("EDGE_MCP_API_KEY", ""),
		},
		Core: CoreConfig{
			URL:       getEnv("DEV_MESH_URL", ""),
			APIKey:    getEnv("DEV_MESH_API_KEY", ""),
			EdgeMCPID: getEnv("EDGE_MCP_ID", generateEdgeMCPID()),
		},
		RateLimit: RateLimitConfig{
			GlobalRPS:          getEnvInt("EDGE_MCP_GLOBAL_RPS", 1000),
			GlobalBurst:        getEnvInt("EDGE_MCP_GLOBAL_BURST", 2000),
			TenantRPS:          getEnvInt("EDGE_MCP_TENANT_RPS", 100),
			TenantBurst:        getEnvInt("EDGE_MCP_TENANT_BURST", 200),
			ToolRPS:            getEnvInt("EDGE_MCP_TOOL_RPS", 50),
			ToolBurst:          getEnvInt("EDGE_MCP_TOOL_BURST", 100),
			EnableQuotas:       getEnvBool("EDGE_MCP_ENABLE_QUOTAS", true),
			QuotaResetInterval: getEnvDuration("EDGE_MCP_QUOTA_RESET_INTERVAL", 24*time.Hour),
			DefaultQuota:       getEnvInt64("EDGE_MCP_DEFAULT_QUOTA", 10000),
			CleanupInterval:    getEnvDuration("EDGE_MCP_CLEANUP_INTERVAL", 5*time.Minute),
			MaxAge:             getEnvDuration("EDGE_MCP_MAX_AGE", 1*time.Hour),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func generateEdgeMCPID() string {
	hostname, _ := os.Hostname()
	return "edge-" + hostname + "-" + time.Now().Format("20060102")
}
