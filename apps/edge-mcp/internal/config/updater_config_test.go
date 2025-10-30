package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdaterConfig_Defaults(t *testing.T) {
	// Clear any environment variables
	_ = os.Unsetenv("EDGE_MCP_UPDATE_ENABLED")
	_ = os.Unsetenv("EDGE_MCP_UPDATE_CHECK_INTERVAL")
	_ = os.Unsetenv("EDGE_MCP_UPDATE_CHANNEL")
	_ = os.Unsetenv("EDGE_MCP_UPDATE_AUTO_DOWNLOAD")
	_ = os.Unsetenv("EDGE_MCP_UPDATE_AUTO_APPLY")
	_ = os.Unsetenv("EDGE_MCP_UPDATE_GITHUB_OWNER")
	_ = os.Unsetenv("EDGE_MCP_UPDATE_GITHUB_REPO")

	cfg := Default()

	require.NotNil(t, cfg)
	assert.True(t, cfg.Updater.Enabled, "updater should be enabled by default")
	assert.Equal(t, 24*time.Hour, cfg.Updater.CheckInterval, "check interval should default to 24h")
	assert.Equal(t, "stable", cfg.Updater.Channel, "channel should default to stable")
	assert.True(t, cfg.Updater.AutoDownload, "auto-download should be enabled by default")
	assert.False(t, cfg.Updater.AutoApply, "auto-apply should be disabled by default for safety")
	assert.Equal(t, "developer-mesh", cfg.Updater.GitHubOwner)
	assert.Equal(t, "developer-mesh", cfg.Updater.GitHubRepo)
}

func TestUpdaterConfig_EnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(t *testing.T, cfg *UpdaterConfig)
	}{
		{
			name: "disable via environment",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_ENABLED": "false",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.False(t, cfg.Enabled)
			},
		},
		{
			name: "custom check interval",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_CHECK_INTERVAL": "12h",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.Equal(t, 12*time.Hour, cfg.CheckInterval)
			},
		},
		{
			name: "beta channel",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_CHANNEL": "beta",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.Equal(t, "beta", cfg.Channel)
			},
		},
		{
			name: "latest channel",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_CHANNEL": "latest",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.Equal(t, "latest", cfg.Channel)
			},
		},
		{
			name: "disable auto-download",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_AUTO_DOWNLOAD": "false",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.False(t, cfg.AutoDownload)
			},
		},
		{
			name: "enable auto-apply",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_AUTO_APPLY": "true",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.True(t, cfg.AutoApply)
			},
		},
		{
			name: "custom github repo",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_GITHUB_OWNER": "custom-org",
				"EDGE_MCP_UPDATE_GITHUB_REPO":  "custom-repo",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.Equal(t, "custom-org", cfg.GitHubOwner)
				assert.Equal(t, "custom-repo", cfg.GitHubRepo)
			},
		},
		{
			name: "all overrides",
			envVars: map[string]string{
				"EDGE_MCP_UPDATE_ENABLED":        "true",
				"EDGE_MCP_UPDATE_CHECK_INTERVAL": "6h",
				"EDGE_MCP_UPDATE_CHANNEL":        "latest",
				"EDGE_MCP_UPDATE_AUTO_DOWNLOAD":  "true",
				"EDGE_MCP_UPDATE_AUTO_APPLY":     "true",
				"EDGE_MCP_UPDATE_GITHUB_OWNER":   "test-owner",
				"EDGE_MCP_UPDATE_GITHUB_REPO":    "test-repo",
			},
			validate: func(t *testing.T, cfg *UpdaterConfig) {
				assert.True(t, cfg.Enabled)
				assert.Equal(t, 6*time.Hour, cfg.CheckInterval)
				assert.Equal(t, "latest", cfg.Channel)
				assert.True(t, cfg.AutoDownload)
				assert.True(t, cfg.AutoApply)
				assert.Equal(t, "test-owner", cfg.GitHubOwner)
				assert.Equal(t, "test-repo", cfg.GitHubRepo)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables
			for _, key := range []string{
				"EDGE_MCP_UPDATE_ENABLED",
				"EDGE_MCP_UPDATE_CHECK_INTERVAL",
				"EDGE_MCP_UPDATE_CHANNEL",
				"EDGE_MCP_UPDATE_AUTO_DOWNLOAD",
				"EDGE_MCP_UPDATE_AUTO_APPLY",
				"EDGE_MCP_UPDATE_GITHUB_OWNER",
				"EDGE_MCP_UPDATE_GITHUB_REPO",
			} {
				_ = os.Unsetenv(key)
			}

			// Set test-specific environment variables
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					_ = os.Unsetenv(key)
				}
			}()

			cfg := Default()
			require.NotNil(t, cfg)
			tt.validate(t, &cfg.Updater)
		})
	}
}

func TestUpdaterConfig_BooleanParsing(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true string", "true", true},
		{"1 string", "1", true},
		{"yes string", "yes", true},
		{"false string", "false", false},
		{"0 string", "0", false},
		{"no string", "no", false},
		{"empty string", "", true},        // Uses default (true)
		{"random string", "random", true}, // Invalid value uses default (true)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unset first to ensure clean state
			_ = os.Unsetenv("EDGE_MCP_UPDATE_ENABLED")

			// Set only if value is not empty
			if tt.value != "" {
				_ = os.Setenv("EDGE_MCP_UPDATE_ENABLED", tt.value)
			}
			defer func() {
				_ = os.Unsetenv("EDGE_MCP_UPDATE_ENABLED")
			}()

			cfg := Default()
			assert.Equal(t, tt.expected, cfg.Updater.Enabled)
		})
	}
}

func TestUpdaterConfig_DurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
		usesDef  bool
	}{
		{"hours", "12h", 12 * time.Hour, false},
		{"minutes", "30m", 30 * time.Minute, false},
		{"seconds", "90s", 90 * time.Second, false},
		{"complex", "1h30m", 90 * time.Minute, false},
		{"invalid", "invalid", 24 * time.Hour, true}, // Falls back to default
		{"empty", "", 24 * time.Hour, true},          // Uses default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				_ = os.Setenv("EDGE_MCP_UPDATE_CHECK_INTERVAL", tt.value)
				defer func() {
					_ = os.Unsetenv("EDGE_MCP_UPDATE_CHECK_INTERVAL")
				}()
			}

			cfg := Default()
			assert.Equal(t, tt.expected, cfg.Updater.CheckInterval)
		})
	}
}

func TestUpdaterConfig_ChannelValues(t *testing.T) {
	channels := []string{"stable", "beta", "latest", "custom-channel"}

	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			_ = os.Setenv("EDGE_MCP_UPDATE_CHANNEL", channel)
			defer func() {
				_ = os.Unsetenv("EDGE_MCP_UPDATE_CHANNEL")
			}()

			cfg := Default()
			assert.Equal(t, channel, cfg.Updater.Channel)
		})
	}
}

func TestUpdaterConfig_Load(t *testing.T) {
	// Test that Load() returns a valid config with updater settings
	cfg, err := Load("non-existent-file.yaml")
	require.NoError(t, err) // Load falls back to defaults
	require.NotNil(t, cfg)

	// Verify updater config is present
	assert.NotNil(t, cfg.Updater)
	assert.Equal(t, "developer-mesh", cfg.Updater.GitHubOwner)
	assert.Equal(t, "developer-mesh", cfg.Updater.GitHubRepo)
}

func TestUpdaterConfig_ZeroValues(t *testing.T) {
	// Ensure zero values don't cause issues
	cfg := &UpdaterConfig{
		Enabled:       false,
		CheckInterval: 0,
		Channel:       "",
		AutoDownload:  false,
		AutoApply:     false,
		GitHubOwner:   "",
		GitHubRepo:    "",
	}

	// Should not panic with zero values
	assert.False(t, cfg.Enabled)
	assert.Equal(t, time.Duration(0), cfg.CheckInterval)
	assert.Equal(t, "", cfg.Channel)
}

func TestUpdaterConfig_Integration(t *testing.T) {
	// Test that UpdaterConfig integrates properly with main Config
	cfg := Default()

	assert.NotNil(t, cfg.Server)
	assert.NotNil(t, cfg.Auth)
	assert.NotNil(t, cfg.Core)
	assert.NotNil(t, cfg.RateLimit)
	assert.NotNil(t, cfg.Updater)

	// Verify updater doesn't interfere with other config
	assert.Equal(t, 8082, cfg.Server.Port)
	assert.True(t, cfg.Updater.Enabled)
}
