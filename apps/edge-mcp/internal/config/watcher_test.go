package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWatcher(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	t.Run("Success", func(t *testing.T) {
		// Create temporary config file
		tmpFile := createTempConfigFile(t, getValidConfigYAML())
		defer func() { _ = os.Remove(tmpFile) }()

		watcher, err := NewConfigWatcher(tmpFile, logger)
		require.NoError(t, err)
		require.NotNil(t, watcher)
		defer func() { _ = watcher.Stop() }()

		assert.Equal(t, tmpFile, watcher.configFile)
		assert.NotNil(t, watcher.config)
		assert.NotNil(t, watcher.watcher)
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		watcher, err := NewConfigWatcher("/nonexistent/config.yaml", logger)
		assert.Error(t, err)
		assert.Nil(t, watcher)
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, "invalid: yaml: content:")
		defer func() { _ = os.Remove(tmpFile) }()

		watcher, err := NewConfigWatcher(tmpFile, logger)
		assert.Error(t, err)
		assert.Nil(t, watcher)
	})
}

func TestConfigWatcher_GetConfig(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	cfg := watcher.GetConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, 8082, cfg.Server.Port)
}

func TestConfigWatcher_RegisterCallback(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	callbackCalled := false
	watcher.RegisterCallback(func(oldConfig, newConfig *Config) error {
		callbackCalled = true
		return nil
	})

	// Update config file to trigger callback
	newYAML := `
server:
  port: 9090
`
	err = os.WriteFile(tmpFile, []byte(newYAML), 0644)
	require.NoError(t, err)

	// Trigger reload
	err = watcher.ForceReload()
	require.NoError(t, err)
	assert.True(t, callbackCalled)
}

func TestConfigWatcher_ValidateConfig(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "Valid",
			config:  Default(),
			wantErr: false,
		},
		{
			name: "InvalidPort_TooLow",
			config: &Config{
				Server: ServerConfig{Port: 0},
			},
			wantErr: true,
		},
		{
			name: "InvalidPort_TooHigh",
			config: &Config{
				Server: ServerConfig{Port: 70000},
			},
			wantErr: true,
		},
		{
			name: "InvalidCoreURL",
			config: &Config{
				Server: ServerConfig{Port: 8082},
				Core:   CoreConfig{URL: "bad"},
			},
			wantErr: true,
		},
		{
			name: "NegativeGlobalRPS",
			config: &Config{
				Server:    ServerConfig{Port: 8082},
				RateLimit: RateLimitConfig{GlobalRPS: -1},
			},
			wantErr: true,
		},
		{
			name: "NegativeTenantRPS",
			config: &Config{
				Server:    ServerConfig{Port: 8082},
				RateLimit: RateLimitConfig{TenantRPS: -1},
			},
			wantErr: true,
		},
		{
			name: "NegativeToolRPS",
			config: &Config{
				Server:    ServerConfig{Port: 8082},
				RateLimit: RateLimitConfig{ToolRPS: -1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := watcher.validateConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigWatcher_DetectChanges(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	oldConfig := &Config{
		Server: ServerConfig{Port: 8082},
		Auth:   AuthConfig{APIKey: "old-key"},
		Core: CoreConfig{
			URL:       "http://old-url",
			APIKey:    "old-api-key",
			EdgeMCPID: "old-id",
		},
		RateLimit: RateLimitConfig{
			GlobalRPS: 100,
			TenantRPS: 50,
			ToolRPS:   25,
		},
	}

	newConfig := &Config{
		Server: ServerConfig{Port: 9090},
		Auth:   AuthConfig{APIKey: "new-key"},
		Core: CoreConfig{
			URL:       "http://new-url",
			APIKey:    "new-api-key",
			EdgeMCPID: "new-id",
		},
		RateLimit: RateLimitConfig{
			GlobalRPS: 200,
			TenantRPS: 100,
			ToolRPS:   50,
		},
	}

	changes := watcher.detectChanges(oldConfig, newConfig)

	// Should detect all changes
	assert.Len(t, changes, 8)

	// Verify specific changes
	changeFields := make(map[string]bool)
	for _, change := range changes {
		changeFields[change.Field] = true
	}

	assert.True(t, changeFields["Server.Port"])
	assert.True(t, changeFields["Auth.APIKey"])
	assert.True(t, changeFields["Core.URL"])
	assert.True(t, changeFields["Core.APIKey"])
	assert.True(t, changeFields["Core.EdgeMCPID"])
	assert.True(t, changeFields["RateLimit.GlobalRPS"])
	assert.True(t, changeFields["RateLimit.TenantRPS"])
	assert.True(t, changeFields["RateLimit.ToolRPS"])
}

func TestConfigWatcher_DetectChanges_NoChanges(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	config := Default()
	changes := watcher.detectChanges(config, config)

	assert.Len(t, changes, 0)
}

func TestConfigWatcher_ReloadConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file watch test in short mode")
	}

	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	// Track callback invocations
	var mu sync.Mutex
	callbackCount := 0
	var lastOldConfig, lastNewConfig *Config

	watcher.RegisterCallback(func(oldConfig, newConfig *Config) error {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
		lastOldConfig = oldConfig
		lastNewConfig = newConfig
		return nil
	})

	// Update config file
	newYAML := `
server:
  port: 9090
auth:
  api_key: "new-test-key"
core:
  url: "http://new-platform"
  api_key: "new-api-key"
  edge_mcp_id: "edge-new-123"
rate_limit:
  global_rps: 2000
  tenant_rps: 200
  tool_rps: 100
`
	err = os.WriteFile(tmpFile, []byte(newYAML), 0644)
	require.NoError(t, err)

	// Force reload
	err = watcher.ForceReload()
	require.NoError(t, err)

	// Verify config was updated
	cfg := watcher.GetConfig()
	assert.Equal(t, 9090, cfg.Server.Port)

	// Verify callback was called
	mu.Lock()
	assert.Equal(t, 1, callbackCount)
	assert.NotNil(t, lastOldConfig)
	assert.NotNil(t, lastNewConfig)
	assert.Equal(t, 8082, lastOldConfig.Server.Port)
	assert.Equal(t, 9090, lastNewConfig.Server.Port)
	mu.Unlock()
}

func TestConfigWatcher_ReloadConfig_ValidationError(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	// Update with invalid config
	invalidYAML := `
server:
  port: 70000  # Invalid port
`
	err = os.WriteFile(tmpFile, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	// Reload should fail validation
	err = watcher.ForceReload()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	// Config should remain unchanged
	cfg := watcher.GetConfig()
	assert.Equal(t, 8082, cfg.Server.Port)
}

func TestConfigWatcher_CallbackError(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	// Register callback that returns error
	watcher.RegisterCallback(func(oldConfig, newConfig *Config) error {
		return assert.AnError
	})

	// Update config file
	newYAML := `
server:
  port: 9090
`
	err = os.WriteFile(tmpFile, []byte(newYAML), 0644)
	require.NoError(t, err)

	// Reload should fail due to callback error
	err = watcher.ForceReload()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "callback failed")
}

func TestConfigWatcher_WatchLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping watch loop test in short mode")
	}

	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	// Set short debounce time for testing
	watcher.SetDebounceTime(100 * time.Millisecond)

	// Track reloads
	var mu sync.Mutex
	reloadCount := 0

	watcher.RegisterCallback(func(oldConfig, newConfig *Config) error {
		mu.Lock()
		defer mu.Unlock()
		reloadCount++
		return nil
	})

	// Start watching
	watcher.Start()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Update config file
	newYAML := `
server:
  port: 9090
`
	err = os.WriteFile(tmpFile, []byte(newYAML), 0644)
	require.NoError(t, err)

	// Wait for reload (debounce time + processing time)
	time.Sleep(500 * time.Millisecond)

	// Verify reload happened
	mu.Lock()
	assert.Greater(t, reloadCount, 0)
	mu.Unlock()

	// Verify config was updated
	cfg := watcher.GetConfig()
	assert.Equal(t, 9090, cfg.Server.Port)
}

func TestConfigWatcher_Stop(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)

	watcher.Start()

	err = watcher.Stop()
	assert.NoError(t, err)
}

func TestConfigWatcher_ConcurrentAccess(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	// Concurrent readers
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := watcher.GetConfig()
			assert.NotNil(t, cfg)
		}()
	}

	wg.Wait()
}

func TestLoadConfigFile(t *testing.T) {
	t.Run("ValidYAML", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, getValidConfigYAML())
		defer func() { _ = os.Remove(tmpFile) }()

		cfg, err := loadConfigFile(tmpFile)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, 8082, cfg.Server.Port)
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		cfg, err := loadConfigFile("/nonexistent/config.yaml")
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, "invalid: yaml: content:")
		defer func() { _ = os.Remove(tmpFile) }()

		cfg, err := loadConfigFile(tmpFile)
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})
}

func TestMergeWithEnv(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("EDGE_MCP_API_KEY", "env-api-key")
	_ = os.Setenv("DEV_MESH_URL", "http://env-url")
	_ = os.Setenv("EDGE_MCP_GLOBAL_RPS", "5000")
	defer func() {
		_ = os.Unsetenv("EDGE_MCP_API_KEY")
		_ = os.Unsetenv("DEV_MESH_URL")
		_ = os.Unsetenv("EDGE_MCP_GLOBAL_RPS")
	}()

	cfg := &Config{
		Auth: AuthConfig{APIKey: "file-api-key"},
		Core: CoreConfig{URL: "http://file-url"},
		RateLimit: RateLimitConfig{
			GlobalRPS: 1000,
		},
	}

	mergeWithEnv(cfg)

	// Environment should override file values
	assert.Equal(t, "env-api-key", cfg.Auth.APIKey)
	assert.Equal(t, "http://env-url", cfg.Core.URL)
	assert.Equal(t, 5000, cfg.RateLimit.GlobalRPS)
}

func TestConfigWatcher_SetDebounceTime(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	tmpFile := createTempConfigFile(t, getValidConfigYAML())
	defer func() { _ = os.Remove(tmpFile) }()

	watcher, err := NewConfigWatcher(tmpFile, logger)
	require.NoError(t, err)
	defer func() { _ = watcher.Stop() }()

	watcher.SetDebounceTime(1 * time.Second)
	assert.Equal(t, 1*time.Second, watcher.debounceTime)
}

// Helper functions

func createTempConfigFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	return tmpFile
}

func getValidConfigYAML() string {
	return `
server:
  port: 8082
auth:
  api_key: "test-api-key"
core:
  url: "http://localhost:8080"
  api_key: "test-core-api-key"
  edge_mcp_id: "edge-test-123"
rate_limit:
  global_rps: 1000
  global_burst: 2000
  tenant_rps: 100
  tenant_burst: 200
  tool_rps: 50
  tool_burst: 100
  enable_quotas: true
  quota_reset_interval: 24h
  default_quota: 10000
  cleanup_interval: 5m
  max_age: 1h
`
}
