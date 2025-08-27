// Package feature provides feature flag functionality to support the
// migration to the Go workspace structure.
package feature

import (
	"os"
	"strings"
	"sync"
)

// Migration-specific feature flags
const (
	// UseNewConfig controls whether to use the new config package
	UseNewConfig = "USE_NEW_CONFIG"

	// UseNewAWS controls whether to use the new AWS implementation
	UseNewAWS = "USE_NEW_AWS"

	// UseNewMetrics controls whether to use the new metrics implementation
	UseNewMetrics = "USE_NEW_METRICS"

	// UseNewRelationship controls whether to use the new relationship implementation
	UseNewRelationship = "USE_NEW_RELATIONSHIP"

	// UseNewObservability controls whether to use the new observability implementation
	UseNewObservability = "USE_NEW_OBSERVABILITY"
)

// Standard Tools feature flags
const (
	// EnableStandardTools controls whether standard tools (GitHub, GitLab, etc.) are enabled
	EnableStandardTools = "ENABLE_STANDARD_TOOLS"

	// EnablePermissionCaching controls whether permission caching is enabled
	EnablePermissionCaching = "ENABLE_PERMISSION_CACHING"

	// EnableToolExpansion controls whether single providers are expanded into multiple MCP tools
	EnableToolExpansion = "ENABLE_TOOL_EXPANSION"

	// EnableAsyncPermissionDiscovery controls whether permission discovery happens asynchronously
	EnableAsyncPermissionDiscovery = "ENABLE_ASYNC_PERMISSION_DISCOVERY"

	// EnableChaosEngineering controls whether chaos engineering hooks are enabled
	EnableChaosEngineering = "ENABLE_CHAOS_ENGINEERING"

	// EnableEnhancedMetrics controls whether enhanced metrics collection is enabled
	EnableEnhancedMetrics = "ENABLE_ENHANCED_METRICS"
)

var (
	// flags stores the current state of all feature flags
	flags map[string]bool

	// flagsMu protects concurrent access to the flags map
	flagsMu sync.RWMutex

	// flagInitialized ensures init() is only called once
	flagInitialized sync.Once
)

// init initializes the feature flags from environment variables
func init() {
	flagInitialized.Do(func() {
		flags = make(map[string]bool)

		// Initialize from environment
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "FEATURE_") {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					name := strings.TrimPrefix(parts[0], "FEATURE_")
					value := strings.ToLower(parts[1])
					flags[name] = value == "true" || value == "1" || value == "yes"
				}
			}
		}
	})
}

// IsEnabled returns whether a feature flag is enabled
func IsEnabled(name string) bool {
	flagsMu.RLock()
	defer flagsMu.RUnlock()

	return flags[name]
}

// SetEnabled sets a feature flag's state programmatically
// This is primarily for testing; in production, flags should
// be set through environment variables
func SetEnabled(name string, enabled bool) {
	flagsMu.Lock()
	defer flagsMu.Unlock()

	flags[name] = enabled
}

// RegisterFlag registers a new feature flag with a default value
// This is useful for ensuring a flag exists even if not set in the environment
func RegisterFlag(name string, defaultValue bool) {
	flagsMu.Lock()
	defer flagsMu.Unlock()

	if _, exists := flags[name]; !exists {
		flags[name] = defaultValue
	}
}

// GetAllFlags returns a copy of the current feature flags
// This is useful for debugging and logging
func GetAllFlags() map[string]bool {
	flagsMu.RLock()
	defer flagsMu.RUnlock()

	// Make a copy to avoid concurrent access issues
	result := make(map[string]bool, len(flags))
	for name, value := range flags {
		result[name] = value
	}

	return result
}
