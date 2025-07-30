package tenant

import (
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// CacheTenantConfig extends the base tenant config with cache-specific settings
type CacheTenantConfig struct {
	*models.TenantConfig

	// Cache-specific limits
	MaxCacheEntries  int           `json:"max_cache_entries"`
	MaxCacheBytes    int64         `json:"max_cache_bytes"`
	CacheTTLOverride time.Duration `json:"cache_ttl_override"`

	// Feature flags
	EnabledFeatures CacheFeatureFlags `json:"enabled_features"`
}

type CacheFeatureFlags struct {
	EnableSemanticCache bool `json:"enable_semantic_cache"`
	EnableCacheWarming  bool `json:"enable_cache_warming"`
	EnableAsyncEviction bool `json:"enable_async_eviction"`
	EnableMetrics       bool `json:"enable_metrics"`
}

// DefaultCacheTenantConfig returns default cache configuration for a tenant
func DefaultCacheTenantConfig() *CacheTenantConfig {
	return &CacheTenantConfig{
		MaxCacheEntries:  10000,
		MaxCacheBytes:    100 * 1024 * 1024, // 100MB
		CacheTTLOverride: 0,                 // Use global default
		EnabledFeatures: CacheFeatureFlags{
			EnableSemanticCache: true,
			EnableCacheWarming:  false,
			EnableAsyncEviction: true,
			EnableMetrics:       true,
		},
	}
}

// ParseFromTenantConfig extracts cache configuration from base tenant config
func ParseFromTenantConfig(baseConfig *models.TenantConfig) *CacheTenantConfig {
	config := &CacheTenantConfig{
		TenantConfig: baseConfig,
		// Set defaults
		MaxCacheEntries: 10000,
		MaxCacheBytes:   100 * 1024 * 1024, // 100MB
		EnabledFeatures: CacheFeatureFlags{
			EnableSemanticCache: true,
			EnableCacheWarming:  false,
			EnableAsyncEviction: true,
			EnableMetrics:       true,
		},
	}

	// Override from features JSON if present
	if features, ok := baseConfig.Features["cache"]; ok {
		if cacheFeatures, ok := features.(map[string]interface{}); ok {
			// Parse max entries
			if maxEntries, ok := cacheFeatures["max_entries"].(float64); ok {
				config.MaxCacheEntries = int(maxEntries)
			}

			// Parse max bytes
			if maxBytes, ok := cacheFeatures["max_bytes"].(float64); ok {
				config.MaxCacheBytes = int64(maxBytes)
			}

			// Parse TTL override (in seconds)
			if ttl, ok := cacheFeatures["ttl_seconds"].(float64); ok {
				config.CacheTTLOverride = time.Duration(ttl) * time.Second
			}

			// Parse feature flags
			if enabled, ok := cacheFeatures["enabled"].(bool); ok {
				config.EnabledFeatures.EnableSemanticCache = enabled
			}

			if warmingEnabled, ok := cacheFeatures["cache_warming"].(bool); ok {
				config.EnabledFeatures.EnableCacheWarming = warmingEnabled
			}

			if asyncEviction, ok := cacheFeatures["async_eviction"].(bool); ok {
				config.EnabledFeatures.EnableAsyncEviction = asyncEviction
			}

			if metricsEnabled, ok := cacheFeatures["metrics_enabled"].(bool); ok {
				config.EnabledFeatures.EnableMetrics = metricsEnabled
			}
		}
	}

	return config
}
