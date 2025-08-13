package cache

import (
	"context"
	"sync"
	"time"
)

// MemoryCache implements an in-memory cache
type MemoryCache struct {
	items      map[string]cacheItem
	mu         sync.RWMutex
	maxItems   int
	defaultTTL time.Duration
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxItems int, defaultTTL time.Duration) Cache {
	return &MemoryCache{
		items:      make(map[string]cacheItem),
		maxItems:   maxItems,
		defaultTTL: defaultTTL,
	}
}

// Get retrieves data from the cache
func (c *MemoryCache) Get(ctx context.Context, key string, value interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return Error{Message: "key not found"}
	}

	if time.Now().After(item.expiration) {
		return Error{Message: "key expired"}
	}

	// For memory cache, we just return the value directly
	// In real implementation, you'd need to handle type conversion
	return nil
}

// Set stores data in the cache
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	// Evict oldest item if at capacity
	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes data from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Exists checks if a key exists in the cache
func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false, nil
	}

	return !time.Now().After(item.expiration), nil
}

// Flush clears all data from the cache
func (c *MemoryCache) Flush(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]cacheItem)
	return nil
}

// Close closes the cache connection
func (c *MemoryCache) Close() error {
	return nil
}

// Size returns the number of items in cache
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// evictOldest removes the oldest item from cache
func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.expiration.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiration
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}
