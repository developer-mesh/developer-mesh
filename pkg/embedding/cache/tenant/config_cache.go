package tenant

import (
	"context"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// ConfigCache caches tenant configurations with TTL
type ConfigCache struct {
	repo       repository.TenantConfigRepository
	cache      sync.Map // map[uuid.UUID]*cachedConfig
	ttl        time.Duration
	cancelFunc context.CancelFunc
}

type cachedConfig struct {
	config    *CacheTenantConfig
	expiresAt time.Time
}

// NewConfigCache creates a new configuration cache
func NewConfigCache(repo repository.TenantConfigRepository, ttl time.Duration) *ConfigCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute // default TTL
	}

	ctx, cancel := context.WithCancel(context.Background())

	cc := &ConfigCache{
		repo:       repo,
		ttl:        ttl,
		cancelFunc: cancel,
	}

	// Start cleanup routine
	go cc.cleanupLoop(ctx)

	return cc
}

// Get retrieves a tenant configuration from cache or database
func (c *ConfigCache) Get(ctx context.Context, tenantID uuid.UUID) (*CacheTenantConfig, error) {
	// Check cache first
	if cached, ok := c.cache.Load(tenantID); ok {
		config := cached.(*cachedConfig)
		if time.Now().Before(config.expiresAt) {
			return config.config, nil
		}
		// Expired, delete it
		c.cache.Delete(tenantID)
	}

	// Load from database
	dbConfig, err := c.repo.GetByTenantID(ctx, tenantID.String())
	if err != nil {
		return nil, err
	}

	// Parse to cache config
	cacheConfig := ParseFromTenantConfig(dbConfig)

	// Cache it
	c.cache.Store(tenantID, &cachedConfig{
		config:    cacheConfig,
		expiresAt: time.Now().Add(c.ttl),
	})

	return cacheConfig, nil
}

// Invalidate removes a tenant configuration from cache
func (c *ConfigCache) Invalidate(tenantID uuid.UUID) {
	c.cache.Delete(tenantID)
}

// InvalidateAll clears the entire cache
func (c *ConfigCache) InvalidateAll() {
	c.cache.Range(func(key, value interface{}) bool {
		c.cache.Delete(key)
		return true
	})
}

// Stop gracefully shuts down the config cache
func (c *ConfigCache) Stop() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
}

// cleanupLoop periodically removes expired entries
func (c *ConfigCache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired entries from cache
func (c *ConfigCache) cleanupExpired() {
	now := time.Now()
	c.cache.Range(func(key, value interface{}) bool {
		config := value.(*cachedConfig)
		if now.After(config.expiresAt) {
			c.cache.Delete(key)
		}
		return true
	})
}

// GetStats returns cache statistics
func (c *ConfigCache) GetStats() map[string]interface{} {
	count := 0
	validCount := 0
	now := time.Now()

	c.cache.Range(func(key, value interface{}) bool {
		count++
		config := value.(*cachedConfig)
		if now.Before(config.expiresAt) {
			validCount++
		}
		return true
	})

	return map[string]interface{}{
		"total_entries": count,
		"valid_entries": validCount,
		"ttl_seconds":   c.ttl.Seconds(),
	}
}
