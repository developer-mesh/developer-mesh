package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
)

// InMemoryCache provides a simple in-memory cache implementation
type InMemoryCache struct {
	data sync.Map
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache() cache.Cache {
	return &InMemoryCache{}
}

// Get retrieves a value from cache
func (c *InMemoryCache) Get(ctx context.Context, key string, value interface{}) error {
	if val, ok := c.data.Load(key); ok {
		// Try to assert to the expected type
		if v, ok := val.([]byte); ok && value != nil {
			if pv, ok := value.(*[]byte); ok {
				*pv = v
				return nil
			}
		}
		// For other types, just return as-is
		if pv, ok := value.(*interface{}); ok {
			*pv = val
			return nil
		}
		return nil
	}
	return cache.ErrNotFound
}

// Set stores a value in cache
func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.data.Store(key, value)

	// Simple TTL implementation
	if ttl > 0 {
		go func() {
			time.Sleep(ttl)
			c.data.Delete(key)
		}()
	}

	return nil
}

// Delete removes a value from cache
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.data.Delete(key)
	return nil
}

// Exists checks if a key exists
func (c *InMemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := c.data.Load(key)
	return ok, nil
}

// Clear removes all values from cache
func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.data.Range(func(key, value interface{}) bool {
		c.data.Delete(key)
		return true
	})
	return nil
}

// Keys returns all keys (not implemented for simplicity)
func (c *InMemoryCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	c.data.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys, nil
}

// TTL returns remaining TTL (not implemented for simplicity)
func (c *InMemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

// Ping checks if cache is available
func (c *InMemoryCache) Ping(ctx context.Context) error {
	return nil
}

// Close closes the cache (no-op for in-memory)
func (c *InMemoryCache) Close() error {
	return nil
}

// GetMulti retrieves multiple values (not implemented)
func (c *InMemoryCache) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, key := range keys {
		var val interface{}
		if err := c.Get(ctx, key, &val); err == nil {
			result[key] = val
		}
	}
	return result, nil
}

// SetMulti stores multiple values (not implemented)
func (c *InMemoryCache) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	for key, value := range items {
		if err := c.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// Increment increments a counter (not implemented)
func (c *InMemoryCache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return 0, nil
}

// Decrement decrements a counter (not implemented)
func (c *InMemoryCache) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return 0, nil
}

// GetSet gets old value and sets new value atomically (not implemented)
func (c *InMemoryCache) GetSet(ctx context.Context, key string, value interface{}, ttl time.Duration) (interface{}, error) {
	var old interface{}
	_ = c.Get(ctx, key, &old)
	_ = c.Set(ctx, key, value, ttl)
	return old, nil
}

// Flush clears all cache entries
func (c *InMemoryCache) Flush(ctx context.Context) error {
	return c.Clear(ctx)
}
