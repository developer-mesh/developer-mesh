package config

import (
	"os"
	"time"
)

// Config represents the Edge MCP configuration
type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
	Core   CoreConfig   `yaml:"core"`
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
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func generateEdgeMCPID() string {
	hostname, _ := os.Hostname()
	return "edge-" + hostname + "-" + time.Now().Format("20060102")
}
