package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// ConfigWatcher watches configuration files for changes and reloads them
//
// Example usage:
//
//	logger := observability.NewStandardLogger("edge-mcp")
//	watcher, err := config.NewConfigWatcher("configs/config.yaml", logger)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer watcher.Stop()
//
//	// Register callback for config changes
//	watcher.RegisterCallback(func(oldConfig, newConfig *config.Config) error {
//		// Handle configuration changes
//		if oldConfig.Server.Port != newConfig.Server.Port {
//			log.Printf("Server port changed from %d to %d (requires restart)",
//				oldConfig.Server.Port, newConfig.Server.Port)
//		}
//
//		// Update rate limiter if rate limits changed
//		if oldConfig.RateLimit.GlobalRPS != newConfig.RateLimit.GlobalRPS {
//			rateLimiter.UpdateGlobalLimit(newConfig.RateLimit.GlobalRPS)
//		}
//
//		return nil
//	})
//
//	// Start watching for changes
//	watcher.Start()
//
//	// Get current config (thread-safe)
//	cfg := watcher.GetConfig()
type ConfigWatcher struct {
	configFile   string
	config       *Config
	configMu     sync.RWMutex
	watcher      *fsnotify.Watcher
	logger       observability.Logger
	callbacks    []ReloadCallback
	callbacksMu  sync.RWMutex
	debounceTime time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
}

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(oldConfig, newConfig *Config) error

// ConfigChange represents a configuration change
type ConfigChange struct {
	Field    string
	OldValue interface{}
	NewValue interface{}
	Changed  bool
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configFile string, logger observability.Logger) (*ConfigWatcher, error) {
	// Load initial configuration
	cfg, err := loadConfigFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add config file to watcher
	err = watcher.Add(configFile)
	if err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("failed to watch config file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	cw := &ConfigWatcher{
		configFile:   configFile,
		config:       cfg,
		watcher:      watcher,
		logger:       logger,
		callbacks:    make([]ReloadCallback, 0),
		debounceTime: 500 * time.Millisecond, // Default debounce time
		ctx:          ctx,
		cancel:       cancel,
	}

	return cw, nil
}

// GetConfig returns the current configuration (thread-safe)
func (cw *ConfigWatcher) GetConfig() *Config {
	cw.configMu.RLock()
	defer cw.configMu.RUnlock()
	return cw.config
}

// RegisterCallback registers a callback to be called when configuration is reloaded
func (cw *ConfigWatcher) RegisterCallback(callback ReloadCallback) {
	cw.callbacksMu.Lock()
	defer cw.callbacksMu.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// Start starts watching the configuration file for changes
func (cw *ConfigWatcher) Start() {
	go cw.watchLoop()
	cw.logger.Info("Configuration watcher started", map[string]interface{}{
		"config_file": cw.configFile,
	})
}

// Stop stops watching the configuration file
func (cw *ConfigWatcher) Stop() error {
	cw.cancel()
	if cw.watcher != nil {
		return cw.watcher.Close()
	}
	return nil
}

// watchLoop is the main watch loop
func (cw *ConfigWatcher) watchLoop() {
	var debounceTimer *time.Timer

	for {
		select {
		case <-cw.ctx.Done():
			cw.logger.Info("Configuration watcher stopped", nil)
			return

		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Only process write and create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Debounce rapid file changes (e.g., from text editors)
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(cw.debounceTime, func() {
					if err := cw.reloadConfig(); err != nil {
						cw.logger.Error("Failed to reload configuration", map[string]interface{}{
							"error": err.Error(),
						})
					}
				})
			}

			// Handle file removal (some editors delete and recreate on save)
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				// Re-add the file to the watcher when it's recreated
				go cw.readdWatchFile()
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Error("Configuration watcher error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

// readdWatchFile re-adds the config file to the watcher (for editors that delete/recreate)
func (cw *ConfigWatcher) readdWatchFile() {
	// Wait for file to be recreated (up to 5 seconds)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(cw.configFile); err == nil {
			// File exists, re-add to watcher
			if err := cw.watcher.Add(cw.configFile); err != nil {
				cw.logger.Error("Failed to re-add config file to watcher", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				cw.logger.Info("Re-added config file to watcher", map[string]interface{}{
					"config_file": cw.configFile,
				})
			}
			return
		}
	}
	cw.logger.Error("Config file was not recreated within timeout", map[string]interface{}{
		"config_file": cw.configFile,
	})
}

// reloadConfig reloads the configuration from file
func (cw *ConfigWatcher) reloadConfig() error {
	// Load new configuration
	newConfig, err := loadConfigFile(cw.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Validate new configuration
	if err := cw.validateConfig(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Get old config for comparison
	cw.configMu.RLock()
	oldConfig := cw.config
	cw.configMu.RUnlock()

	// Detect changes
	changes := cw.detectChanges(oldConfig, newConfig)

	// Log changes
	if len(changes) > 0 {
		cw.logConfigChanges(changes)
	} else {
		cw.logger.Info("Configuration reloaded with no changes", nil)
		return nil
	}

	// Execute callbacks
	cw.callbacksMu.RLock()
	callbacks := cw.callbacks
	cw.callbacksMu.RUnlock()

	for _, callback := range callbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			return fmt.Errorf("callback failed: %w", err)
		}
	}

	// Atomically update configuration
	cw.configMu.Lock()
	cw.config = newConfig
	cw.configMu.Unlock()

	cw.logger.Info("Configuration reloaded successfully", map[string]interface{}{
		"changes": len(changes),
	})

	return nil
}

// validateConfig validates the configuration
func (cw *ConfigWatcher) validateConfig(cfg *Config) error {
	// Validate server config
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d (must be 1-65535)", cfg.Server.Port)
	}

	// Validate core platform URL if provided
	if cfg.Core.URL != "" {
		// Basic URL validation (could be enhanced)
		if len(cfg.Core.URL) < 7 { // Minimum: http://
			return fmt.Errorf("invalid core platform URL: %s", cfg.Core.URL)
		}
	}

	// Validate rate limit config
	if cfg.RateLimit.GlobalRPS < 0 {
		return fmt.Errorf("invalid global RPS: %d (must be >= 0)", cfg.RateLimit.GlobalRPS)
	}
	if cfg.RateLimit.TenantRPS < 0 {
		return fmt.Errorf("invalid tenant RPS: %d (must be >= 0)", cfg.RateLimit.TenantRPS)
	}
	if cfg.RateLimit.ToolRPS < 0 {
		return fmt.Errorf("invalid tool RPS: %d (must be >= 0)", cfg.RateLimit.ToolRPS)
	}

	return nil
}

// detectChanges detects changes between old and new configuration
func (cw *ConfigWatcher) detectChanges(oldCfg, newCfg *Config) []ConfigChange {
	changes := make([]ConfigChange, 0)

	// Server changes
	if oldCfg.Server.Port != newCfg.Server.Port {
		changes = append(changes, ConfigChange{
			Field:    "Server.Port",
			OldValue: oldCfg.Server.Port,
			NewValue: newCfg.Server.Port,
			Changed:  true,
		})
	}

	// Auth changes
	if oldCfg.Auth.APIKey != newCfg.Auth.APIKey {
		changes = append(changes, ConfigChange{
			Field:    "Auth.APIKey",
			OldValue: "[REDACTED]",
			NewValue: "[REDACTED]",
			Changed:  true,
		})
	}

	// Core platform changes
	if oldCfg.Core.URL != newCfg.Core.URL {
		changes = append(changes, ConfigChange{
			Field:    "Core.URL",
			OldValue: oldCfg.Core.URL,
			NewValue: newCfg.Core.URL,
			Changed:  true,
		})
	}
	if oldCfg.Core.APIKey != newCfg.Core.APIKey {
		changes = append(changes, ConfigChange{
			Field:    "Core.APIKey",
			OldValue: "[REDACTED]",
			NewValue: "[REDACTED]",
			Changed:  true,
		})
	}
	if oldCfg.Core.EdgeMCPID != newCfg.Core.EdgeMCPID {
		changes = append(changes, ConfigChange{
			Field:    "Core.EdgeMCPID",
			OldValue: oldCfg.Core.EdgeMCPID,
			NewValue: newCfg.Core.EdgeMCPID,
			Changed:  true,
		})
	}

	// Rate limit changes
	if oldCfg.RateLimit.GlobalRPS != newCfg.RateLimit.GlobalRPS {
		changes = append(changes, ConfigChange{
			Field:    "RateLimit.GlobalRPS",
			OldValue: oldCfg.RateLimit.GlobalRPS,
			NewValue: newCfg.RateLimit.GlobalRPS,
			Changed:  true,
		})
	}
	if oldCfg.RateLimit.TenantRPS != newCfg.RateLimit.TenantRPS {
		changes = append(changes, ConfigChange{
			Field:    "RateLimit.TenantRPS",
			OldValue: oldCfg.RateLimit.TenantRPS,
			NewValue: newCfg.RateLimit.TenantRPS,
			Changed:  true,
		})
	}
	if oldCfg.RateLimit.ToolRPS != newCfg.RateLimit.ToolRPS {
		changes = append(changes, ConfigChange{
			Field:    "RateLimit.ToolRPS",
			OldValue: oldCfg.RateLimit.ToolRPS,
			NewValue: newCfg.RateLimit.ToolRPS,
			Changed:  true,
		})
	}

	return changes
}

// logConfigChanges logs configuration changes
func (cw *ConfigWatcher) logConfigChanges(changes []ConfigChange) {
	for _, change := range changes {
		cw.logger.Info("Configuration changed", map[string]interface{}{
			"field":     change.Field,
			"old_value": change.OldValue,
			"new_value": change.NewValue,
		})
	}
}

// loadConfigFile loads configuration from a YAML file
func loadConfigFile(configFile string) (*Config, error) {
	// Sanitize file path to prevent directory traversal
	cleanPath := filepath.Clean(configFile)

	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", cleanPath)
	}

	// Read file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge with environment variables (environment takes precedence)
	mergeWithEnv(&cfg)

	return &cfg, nil
}

// mergeWithEnv merges configuration with environment variables
// Environment variables take precedence over file values
func mergeWithEnv(cfg *Config) {
	// Auth
	if apiKey := os.Getenv("EDGE_MCP_API_KEY"); apiKey != "" {
		cfg.Auth.APIKey = apiKey
	}

	// Core platform
	if url := os.Getenv("DEV_MESH_URL"); url != "" {
		cfg.Core.URL = url
	}
	if apiKey := os.Getenv("DEV_MESH_API_KEY"); apiKey != "" {
		cfg.Core.APIKey = apiKey
	}
	if edgeMCPID := os.Getenv("EDGE_MCP_ID"); edgeMCPID != "" {
		cfg.Core.EdgeMCPID = edgeMCPID
	}

	// Rate limits
	if val := getEnvInt("EDGE_MCP_GLOBAL_RPS", 0); val > 0 {
		cfg.RateLimit.GlobalRPS = val
	}
	if val := getEnvInt("EDGE_MCP_TENANT_RPS", 0); val > 0 {
		cfg.RateLimit.TenantRPS = val
	}
	if val := getEnvInt("EDGE_MCP_TOOL_RPS", 0); val > 0 {
		cfg.RateLimit.ToolRPS = val
	}
}

// ForceReload forces a configuration reload (useful for testing)
func (cw *ConfigWatcher) ForceReload() error {
	return cw.reloadConfig()
}

// SetDebounceTime sets the debounce time for file change events
func (cw *ConfigWatcher) SetDebounceTime(duration time.Duration) {
	cw.debounceTime = duration
}
