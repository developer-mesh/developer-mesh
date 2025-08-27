package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// PermissionCacheService provides caching for tool permissions
// Uses a two-tier cache: L1 in-memory LRU cache for fast lookups,
// L2 Redis cache for distributed caching across instances
type PermissionCacheService struct {
	l1Cache cache.Cache // In-memory LRU cache
	l2Cache cache.Cache // Redis cache
	logger  observability.Logger
	ttl     time.Duration
}

// PermissionCacheEntry represents a cached permission result
type PermissionCacheEntry struct {
	// AllowedOperations contains the list of operation IDs the token has access to
	AllowedOperations []string `json:"allowed_operations"`
	// Scopes contains the OAuth scopes associated with the token
	Scopes []string `json:"scopes"`
	// Provider is the tool provider name
	Provider string `json:"provider"`
	// CachedAt is when this entry was cached
	CachedAt time.Time `json:"cached_at"`
}

// NewPermissionCacheService creates a new permission cache service
func NewPermissionCacheService(l1Cache, l2Cache cache.Cache, logger observability.Logger) *PermissionCacheService {
	return &PermissionCacheService{
		l1Cache: l1Cache,
		l2Cache: l2Cache,
		logger:  logger,
		ttl:     15 * time.Minute, // Default TTL for permission cache
	}
}

// buildCacheKey creates a cache key for a provider and token combination
func (s *PermissionCacheService) buildCacheKey(provider, tokenHash string) string {
	return fmt.Sprintf("permissions:%s:%s", provider, tokenHash)
}

// Get retrieves cached permissions for a provider and token
func (s *PermissionCacheService) Get(ctx context.Context, provider, tokenHash string) (*PermissionCacheEntry, error) {
	key := s.buildCacheKey(provider, tokenHash)

	// Try L1 cache first
	var entry PermissionCacheEntry
	if s.l1Cache != nil {
		if err := s.l1Cache.Get(ctx, key, &entry); err == nil {
			s.logger.Debug("Permission cache L1 hit", map[string]interface{}{
				"provider": provider,
				"key":      key,
			})
			return &entry, nil
		}
	}

	// Try L2 cache
	if s.l2Cache != nil {
		if err := s.l2Cache.Get(ctx, key, &entry); err == nil {
			s.logger.Debug("Permission cache L2 hit", map[string]interface{}{
				"provider": provider,
				"key":      key,
			})

			// Populate L1 cache for next time
			if s.l1Cache != nil {
				_ = s.l1Cache.Set(ctx, key, entry, s.ttl)
			}

			return &entry, nil
		}
	}

	s.logger.Debug("Permission cache miss", map[string]interface{}{
		"provider": provider,
		"key":      key,
	})

	return nil, fmt.Errorf("permission not found in cache")
}

// Set stores permissions in the cache
func (s *PermissionCacheService) Set(ctx context.Context, provider, tokenHash string, entry *PermissionCacheEntry) error {
	key := s.buildCacheKey(provider, tokenHash)
	entry.CachedAt = time.Now()

	// Store in both caches
	if s.l1Cache != nil {
		if err := s.l1Cache.Set(ctx, key, entry, s.ttl); err != nil {
			s.logger.Warn("Failed to set L1 cache", map[string]interface{}{
				"error": err.Error(),
				"key":   key,
			})
		}
	}

	if s.l2Cache != nil {
		if err := s.l2Cache.Set(ctx, key, entry, s.ttl); err != nil {
			s.logger.Warn("Failed to set L2 cache", map[string]interface{}{
				"error": err.Error(),
				"key":   key,
			})
			return err
		}
	}

	s.logger.Debug("Cached permissions", map[string]interface{}{
		"provider":   provider,
		"key":        key,
		"operations": len(entry.AllowedOperations),
	})

	return nil
}

// Invalidate removes permissions from the cache
func (s *PermissionCacheService) Invalidate(ctx context.Context, provider, tokenHash string) error {
	key := s.buildCacheKey(provider, tokenHash)

	// Remove from both caches
	if s.l1Cache != nil {
		_ = s.l1Cache.Delete(ctx, key)
	}

	if s.l2Cache != nil {
		if err := s.l2Cache.Delete(ctx, key); err != nil {
			s.logger.Warn("Failed to delete from L2 cache", map[string]interface{}{
				"error": err.Error(),
				"key":   key,
			})
			return err
		}
	}

	s.logger.Debug("Invalidated permission cache", map[string]interface{}{
		"provider": provider,
		"key":      key,
	})

	return nil
}

// InvalidateProvider removes all cached permissions for a provider
func (s *PermissionCacheService) InvalidateProvider(ctx context.Context, provider string) error {
	// Note: This would require scanning all keys with the provider prefix
	// For now, we'll just log it - full implementation would need
	// Redis SCAN command support in the cache interface
	s.logger.Info("Provider cache invalidation requested", map[string]interface{}{
		"provider": provider,
	})

	// In production, you'd want to:
	// 1. Use Redis SCAN to find all keys matching "permissions:provider:*"
	// 2. Delete them in batches
	// 3. Also clear from L1 cache

	return nil
}

// SetTTL updates the TTL for cache entries
func (s *PermissionCacheService) SetTTL(ttl time.Duration) {
	s.ttl = ttl
}

// GetStats returns cache statistics
func (s *PermissionCacheService) GetStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})

	if s.l1Cache != nil {
		stats["l1_size"] = s.l1Cache.Size()
	}

	// L2 cache size would require Redis INFO command
	stats["ttl_seconds"] = s.ttl.Seconds()

	return stats
}

// WarmCache pre-populates the cache for common tokens
// This can be called during startup or after cache invalidation
func (s *PermissionCacheService) WarmCache(ctx context.Context, provider string, entries map[string]*PermissionCacheEntry) error {
	for tokenHash, entry := range entries {
		if err := s.Set(ctx, provider, tokenHash, entry); err != nil {
			s.logger.Warn("Failed to warm cache entry", map[string]interface{}{
				"error":    err.Error(),
				"provider": provider,
			})
		}
	}

	s.logger.Info("Warmed permission cache", map[string]interface{}{
		"provider": provider,
		"entries":  len(entries),
	})

	return nil
}

// CacheKey represents a cache key that can be marshaled/unmarshaled
type CacheKey struct {
	Provider  string `json:"provider"`
	TokenHash string `json:"token_hash"`
}

// MarshalBinary implements the encoding.BinaryMarshaler interface
func (k CacheKey) MarshalBinary() ([]byte, error) {
	return json.Marshal(k)
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (k *CacheKey) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, k)
}
