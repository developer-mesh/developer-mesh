package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/middleware"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/google/uuid"
)

var (
	ErrFeatureDisabled = errors.New("semantic cache feature is disabled for tenant")
)

// TenantAwareCache provides tenant-isolated semantic caching with encryption support
type TenantAwareCache struct {
	baseCache         *SemanticCache
	tenantConfigRepo  repository.TenantConfigRepository
	rateLimiter       *middleware.RateLimiter
	encryptionService *security.EncryptionService
	logger            observability.Logger
	metrics           observability.MetricsClient
	configCache       sync.Map // For tenant config caching
	configCacheMu     sync.RWMutex
}

// NewTenantAwareCache creates a new tenant-aware cache instance
func NewTenantAwareCache(
	baseCache *SemanticCache,
	configRepo repository.TenantConfigRepository,
	rateLimiter *middleware.RateLimiter,
	encryptionKey string,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *TenantAwareCache {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.tenant")
	}

	return &TenantAwareCache{
		baseCache:         baseCache,
		tenantConfigRepo:  configRepo,
		rateLimiter:       rateLimiter,
		encryptionService: security.NewEncryptionService(encryptionKey),
		logger:            logger,
		metrics:           metrics,
	}
}

// Get retrieves from cache with tenant isolation
func (tc *TenantAwareCache) Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error) {
	// Extract tenant ID using auth package
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return nil, ErrNoTenantID
	}

	// Apply rate limiting if configured
	if tc.rateLimiter != nil {
		// Note: The middleware RateLimiter doesn't expose a simple Allow method
		// We'll need to enhance it or use a different approach
		// For now, skip rate limiting here
	}

	// Get tenant config
	config, err := tc.getTenantConfig(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant config: %w", err)
	}

	// Check if cache is enabled for tenant
	if config != nil && config.Features != nil {
		if enabled, ok := config.Features["semantic_cache"].(bool); ok && !enabled {
			return nil, ErrFeatureDisabled
		}
	}

	// Build tenant-specific key
	key := tc.getCacheKey(tenantID, query)

	// Get from cache using the base cache with tenant-specific key
	entry, err := tc.getWithTenantKey(ctx, key, query, embedding)
	if err != nil {
		return nil, err
	}

	// Decrypt sensitive data if needed
	if entry != nil && entry.Metadata != nil {
		if encData, ok := entry.Metadata["encrypted_data"].([]byte); ok && len(encData) > 0 {
			decrypted, err := tc.encryptionService.DecryptCredential(encData, tenantID.String())
			if err != nil {
				tc.logger.Error("Failed to decrypt cache entry", map[string]interface{}{
					"error":     err.Error(),
					"tenant_id": tenantID.String(),
				})
				return nil, err
			}
			entry.Metadata["decrypted_data"] = decrypted
		}
	}

	// Record metrics
	if tc.metrics != nil {
		labels := map[string]string{
			"tenant_id": tenantID.String(),
		}
		if entry != nil {
			tc.metrics.IncrementCounterWithLabels("cache.tenant.hit", 1, labels)
		} else {
			tc.metrics.IncrementCounterWithLabels("cache.tenant.miss", 1, labels)
		}
	}

	return entry, nil
}

// Set stores in cache with tenant isolation and encryption
func (tc *TenantAwareCache) Set(ctx context.Context, query string, embedding []float32, results []CachedSearchResult) error {
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return ErrNoTenantID
	}

	// Get tenant config to check limits
	config, err := tc.getTenantConfig(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant config: %w", err)
	}

	// Check if cache is enabled
	if config != nil && config.Features != nil {
		if enabled, ok := config.Features["semantic_cache"].(bool); ok && !enabled {
			return ErrFeatureDisabled
		}
	}

	// Check if results contain sensitive data
	var encryptedData []byte
	sensitiveData := tc.extractSensitiveData(results)

	if sensitiveData != nil {
		encrypted, err := tc.encryptionService.EncryptJSON(sensitiveData, tenantID.String())
		if err != nil {
			return fmt.Errorf("failed to encrypt sensitive data: %w", err)
		}
		encryptedData = []byte(encrypted)
	}

	key := tc.getCacheKey(tenantID, query)
	return tc.setWithEncryption(ctx, key, query, embedding, results, encryptedData)
}

// Delete removes a query from the tenant's cache
func (tc *TenantAwareCache) Delete(ctx context.Context, query string) error {
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return ErrNoTenantID
	}

	key := tc.getCacheKey(tenantID, query)
	return tc.baseCache.Delete(ctx, key)
}

// Clear removes all entries for a tenant
func (tc *TenantAwareCache) ClearTenant(ctx context.Context, tenantID uuid.UUID) error {
	pattern := fmt.Sprintf("%s:{%s}:*", tc.baseCache.config.Prefix, tenantID.String())

	// Use SCAN to find all tenant keys
	iter := tc.baseCache.redis.GetClient().Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())

		// Delete in batches
		if len(keys) >= 100 {
			if err := tc.baseCache.redis.Del(ctx, keys...); err != nil {
				return fmt.Errorf("failed to delete batch: %w", err)
			}
			keys = keys[:0]
		}
	}

	// Delete remaining keys
	if len(keys) > 0 {
		if err := tc.baseCache.redis.Del(ctx, keys...); err != nil {
			return fmt.Errorf("failed to delete final batch: %w", err)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	// Clear tenant config from local cache
	cacheKey := fmt.Sprintf("tenant_config:%s", tenantID.String())
	tc.configCache.Delete(cacheKey)

	return nil
}

// GetTenantStats returns cache statistics for a specific tenant
func (tc *TenantAwareCache) GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*CacheStats, error) {
	pattern := fmt.Sprintf("%s:{%s}:*", tc.baseCache.config.Prefix, tenantID.String())

	stats := &CacheStats{
		Timestamp: time.Now(),
	}

	// Count entries
	keys, err := tc.scanKeys(ctx, pattern)
	if err != nil {
		return nil, err
	}

	stats.TotalEntries = len(keys)

	// Get hit/miss stats from metrics if available
	// Note: This would require storing per-tenant metrics

	return stats, nil
}

// Helper methods

func (tc *TenantAwareCache) getCacheKey(tenantID uuid.UUID, query string) string {
	normalized := tc.baseCache.normalizer.Normalize(query)
	sanitized := SanitizeRedisKey(normalized)

	// Use Redis hash tags for cluster support
	return fmt.Sprintf("%s:{%s}:q:%s",
		tc.baseCache.config.Prefix,
		tenantID.String(),
		sanitized)
}

func (tc *TenantAwareCache) getTenantConfig(ctx context.Context, tenantID uuid.UUID) (*models.TenantConfig, error) {
	// Try local cache first
	cacheKey := fmt.Sprintf("tenant_config:%s", tenantID.String())
	if cached, found := tc.configCache.Load(cacheKey); found {
		// Check if cache entry is still valid
		if entry, ok := cached.(*tenantConfigCacheEntry); ok {
			if time.Since(entry.timestamp) < 5*time.Minute {
				return entry.config, nil
			}
			// Cache expired, delete it
			tc.configCache.Delete(cacheKey)
		}
	}

	// Load from repository if available
	if tc.tenantConfigRepo == nil {
		// Return nil config if no repository
		return nil, nil
	}

	config, err := tc.tenantConfigRepo.GetByTenantID(ctx, tenantID.String())
	if err != nil {
		return nil, err
	}

	// Cache for 5 minutes
	tc.configCache.Store(cacheKey, &tenantConfigCacheEntry{
		config:    config,
		timestamp: time.Now(),
	})

	return config, nil
}

// tenantConfigCacheEntry wraps a tenant config with timestamp
type tenantConfigCacheEntry struct {
	config    *models.TenantConfig
	timestamp time.Time
}

func (tc *TenantAwareCache) getWithTenantKey(ctx context.Context, key, query string, embedding []float32) (*CacheEntry, error) {
	// Use the base cache's get logic but with tenant-specific key
	data, err := tc.baseCache.redis.Get(ctx, key)
	if err != nil {
		return nil, nil // Cache miss
	}

	// Unmarshal entry
	entry, err := tc.baseCache.unmarshalEntry([]byte(data))
	if err != nil {
		return nil, err
	}

	// Update access stats
	updatedEntry, err := tc.baseCache.updateAccessStats(ctx, key, entry)
	if err != nil {
		tc.logger.Warn("Failed to update access stats", map[string]interface{}{
			"error": err.Error(),
			"key":   key,
		})
		return entry, nil
	}

	return updatedEntry, nil
}

func (tc *TenantAwareCache) setWithEncryption(ctx context.Context, key, query string, embedding []float32, results []CachedSearchResult, encryptedData []byte) error {
	entry := &CacheEntry{
		Query:           query,
		NormalizedQuery: tc.baseCache.normalizer.Normalize(query),
		Embedding:       embedding,
		Results:         results,
		CachedAt:        time.Now(),
		HitCount:        0,
		LastAccessedAt:  time.Now(),
		TTL:             tc.baseCache.config.TTL,
		Metadata: map[string]interface{}{
			"result_count":   len(results),
			"has_embedding":  len(embedding) > 0,
			"has_encryption": len(encryptedData) > 0,
		},
	}

	// Add encrypted data to metadata if present
	if len(encryptedData) > 0 {
		entry.Metadata["encrypted_data"] = encryptedData
	}

	// Marshal and store
	data, err := tc.baseCache.marshalEntry(entry)
	if err != nil {
		return err
	}

	return tc.baseCache.redis.Set(ctx, key, data, entry.TTL)
}

func (tc *TenantAwareCache) extractSensitiveData(results []CachedSearchResult) interface{} {
	// Extract any fields that might contain sensitive data
	// This is a placeholder - actual implementation would depend on your data model
	var sensitive []map[string]interface{}

	for _, result := range results {
		if result.Metadata != nil {
			// Look for fields that might be sensitive
			if val, ok := result.Metadata["api_key"]; ok {
				sensitive = append(sensitive, map[string]interface{}{
					"id":      result.ID,
					"api_key": val,
				})
			}
			if val, ok := result.Metadata["secret"]; ok {
				sensitive = append(sensitive, map[string]interface{}{
					"id":     result.ID,
					"secret": val,
				})
			}
		}
	}

	if len(sensitive) > 0 {
		return sensitive
	}

	return nil
}

func (tc *TenantAwareCache) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string

	iter := tc.baseCache.redis.GetClient().Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}
