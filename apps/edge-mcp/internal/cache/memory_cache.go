// Package cache provides a simple in-memory cache for Edge MCP
// This avoids dependencies on Redis or other infrastructure
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Cache interface for Edge MCP's in-memory cache
type Cache interface {
	Get(ctx context.Context, key string, value interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Size() int
}

// MemoryCache implements a simple in-memory cache
type MemoryCache struct {
	items      map[string]*cacheItem
	mu         sync.RWMutex
	maxItems   int
	defaultTTL time.Duration
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache creates a new in-memory cache for Edge MCP
func NewMemoryCache(maxItems int, defaultTTL time.Duration) Cache {
	cache := &MemoryCache{
		items:      make(map[string]*cacheItem),
		maxItems:   maxItems,
		defaultTTL: defaultTTL,
	}

	// Start cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(ctx context.Context, key string, value interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	if time.Now().After(item.expiration) {
		return fmt.Errorf("key expired: %s", key)
	}

	// Simple type assertion - in production you'd use reflection or encoding
	switch v := value.(type) {
	case *map[string]interface{}:
		if data, ok := item.value.(map[string]interface{}); ok {
			*v = data
			return nil
		}
	case *string:
		if data, ok := item.value.(string); ok {
			*v = data
			return nil
		}
	case *[]byte:
		if data, ok := item.value.([]byte); ok {
			*v = data
			return nil
		}
	}

	// Fallback: direct assignment
	if ptr, ok := value.(*interface{}); ok {
		*ptr = item.value
		return nil
	}

	return fmt.Errorf("type mismatch for key: %s", key)
}

// Set stores a value in the cache
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple eviction if at max capacity
	if len(c.items) >= c.maxItems && c.items[key] == nil {
		// Remove oldest expired item
		for k, item := range c.items {
			if time.Now().After(item.expiration) {
				delete(c.items, k)
				break
			}
		}

		// If still at capacity, remove a random item
		if len(c.items) >= c.maxItems {
			for k := range c.items {
				delete(c.items, k)
				break
			}
		}
	}

	c.items[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a key from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Size returns the number of items in the cache
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// cleanupExpired periodically removes expired items
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiration) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}
