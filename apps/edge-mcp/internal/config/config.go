package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the Edge MCP configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Core      CoreConfig      `yaml:"core"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Updater   UpdaterConfig   `yaml:"updater"`
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

// UpdaterConfig represents auto-update configuration
type UpdaterConfig struct {
	Enabled       bool          `yaml:"enabled"`        // Master switch for auto-updates
	CheckInterval time.Duration `yaml:"check_interval"` // How often to check for updates
	Channel       string        `yaml:"channel"`        // Update channel: stable, beta, latest
	AutoDownload  bool          `yaml:"auto_download"`  // Automatically download updates
	AutoApply     bool          `yaml:"auto_apply"`     // Automatically apply updates (requires restart)
	GitHubOwner   string        `yaml:"github_owner"`   // GitHub repository owner
	GitHubRepo    string        `yaml:"github_repo"`    // GitHub repository name
}

// Load loads configuration from file or environment
// Priority: Environment variables > User config file > Specified config file > Defaults
func Load(configFile string) (*Config, error) {
	cfg := Default()

	// Try loading from specified config file first
	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			if err := loadFromFile(configFile, cfg); err != nil {
				return nil, fmt.Errorf("failed to load config file %s: %w", configFile, err)
			}
			fmt.Fprintf(os.Stderr, "[edge-mcp] Loaded base config from: %s\n", configFile)
		}
	}

	// If critical values are missing, try loading from user config locations
	if cfg.Core.URL == "" || cfg.Auth.APIKey == "" {
		userConfigFile := findConfigFile()
		if userConfigFile != "" {
			// Only load non-empty values from user config
			userCfg := &Config{}
			if err := loadFromFile(userConfigFile, userCfg); err == nil {
				fmt.Fprintf(os.Stderr, "[edge-mcp] Merging user config from: %s\n", userConfigFile)
				// Merge non-empty values
				if cfg.Core.URL == "" && userCfg.Core.URL != "" {
					cfg.Core.URL = userCfg.Core.URL
				}
				if cfg.Auth.APIKey == "" && userCfg.Auth.APIKey != "" {
					cfg.Auth.APIKey = userCfg.Auth.APIKey
					cfg.Core.APIKey = userCfg.Auth.APIKey
				}
			}
		}
	}

	// Environment variables override all file values
	applyEnvOverrides(cfg)

	// Debug: Print resolved configuration
	fmt.Fprintf(os.Stderr, "[edge-mcp] Resolved configuration:\n")
	fmt.Fprintf(os.Stderr, "[edge-mcp]   Core URL: %s\n", cfg.Core.URL)
	if cfg.Auth.APIKey != "" {
		fmt.Fprintf(os.Stderr, "[edge-mcp]   Auth API Key: %s...%s\n", cfg.Auth.APIKey[:10], cfg.Auth.APIKey[len(cfg.Auth.APIKey)-3:])
	} else {
		fmt.Fprintf(os.Stderr, "[edge-mcp]   Auth API Key: (empty)\n")
	}

	return cfg, nil
}

// findConfigFile searches for config file in standard locations
func findConfigFile() string {
	locations := []string{
		"edge-mcp.yaml",                                    // Current directory
		"~/.edge-mcp.yaml",                                 // Home directory
		"~/.config/edge-mcp/config.yaml",                   // XDG config directory
		"/etc/edge-mcp/config.yaml",                        // System-wide config
	}

	for _, loc := range locations {
		// Expand ~ to home directory
		if loc[0] == '~' {
			home, err := os.UserHomeDir()
			if err == nil {
				loc = filepath.Join(home, loc[1:])
			}
		}

		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// loadFromFile loads configuration from YAML file
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(cfg *Config) {
	// Auth configuration
	if val := os.Getenv("DEV_MESH_API_KEY"); val != "" {
		cfg.Auth.APIKey = val
		cfg.Core.APIKey = val
	}

	// Core configuration
	if val := os.Getenv("DEV_MESH_URL"); val != "" {
		cfg.Core.URL = val
	}
	if val := os.Getenv("EDGE_MCP_ID"); val != "" {
		cfg.Core.EdgeMCPID = val
	}

	// Server configuration
	if val := getEnvInt("EDGE_MCP_PORT", 0); val != 0 {
		cfg.Server.Port = val
	}

	// Rate limit configuration
	if val := getEnvInt("EDGE_MCP_GLOBAL_RPS", 0); val != 0 {
		cfg.RateLimit.GlobalRPS = val
	}
	if val := getEnvInt("EDGE_MCP_GLOBAL_BURST", 0); val != 0 {
		cfg.RateLimit.GlobalBurst = val
	}
	if val := getEnvInt("EDGE_MCP_TENANT_RPS", 0); val != 0 {
		cfg.RateLimit.TenantRPS = val
	}
	if val := getEnvInt("EDGE_MCP_TENANT_BURST", 0); val != 0 {
		cfg.RateLimit.TenantBurst = val
	}
	if val := getEnvInt("EDGE_MCP_TOOL_RPS", 0); val != 0 {
		cfg.RateLimit.ToolRPS = val
	}
	if val := getEnvInt("EDGE_MCP_TOOL_BURST", 0); val != 0 {
		cfg.RateLimit.ToolBurst = val
	}

	// Updater configuration
	if val, ok := getEnvBoolOpt("EDGE_MCP_UPDATE_ENABLED"); ok {
		cfg.Updater.Enabled = val
	}
	if val := os.Getenv("EDGE_MCP_UPDATE_CHANNEL"); val != "" {
		cfg.Updater.Channel = val
	}
	if val, ok := getEnvBoolOpt("EDGE_MCP_UPDATE_AUTO_DOWNLOAD"); ok {
		cfg.Updater.AutoDownload = val
	}
	if val, ok := getEnvBoolOpt("EDGE_MCP_UPDATE_AUTO_APPLY"); ok {
		cfg.Updater.AutoApply = val
	}
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8082,
		},
		Auth: AuthConfig{
			APIKey: getEnv("DEV_MESH_API_KEY", ""),
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
		Updater: UpdaterConfig{
			Enabled:       getEnvBool("EDGE_MCP_UPDATE_ENABLED", true),
			CheckInterval: getEnvDuration("EDGE_MCP_UPDATE_CHECK_INTERVAL", 24*time.Hour),
			Channel:       getEnv("EDGE_MCP_UPDATE_CHANNEL", "stable"),
			AutoDownload:  getEnvBool("EDGE_MCP_UPDATE_AUTO_DOWNLOAD", true),
			AutoApply:     getEnvBool("EDGE_MCP_UPDATE_AUTO_APPLY", false), // Require manual restart for safety
			GitHubOwner:   getEnv("EDGE_MCP_UPDATE_GITHUB_OWNER", "developer-mesh"),
			GitHubRepo:    getEnv("EDGE_MCP_UPDATE_GITHUB_REPO", "developer-mesh"),
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

// getEnvBoolOpt returns bool value from env and whether it was set
func getEnvBoolOpt(key string) (bool, bool) {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue, true
		}
	}
	return false, false
}
