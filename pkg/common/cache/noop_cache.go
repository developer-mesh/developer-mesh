package cache

import (
	"context"
	"time"
)

// NoOpCache is a cache implementation that does nothing
// Used for graceful degradation when cache is unavailable
type NoOpCache struct{}

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() Cache {
	return &NoOpCache{}
}

// Get always returns cache miss
func (n *NoOpCache) Get(ctx context.Context, key string, value any) error {
	return ErrNotFound
}

// Set does nothing and always returns success
func (n *NoOpCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return nil
}

// Delete does nothing and always returns success
func (n *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

// Exists always returns false
func (n *NoOpCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

// Flush does nothing and always returns success
func (n *NoOpCache) Flush(ctx context.Context) error {
	return nil
}

// Close does nothing
func (n *NoOpCache) Close() error {
	return nil
}

// Size returns 0 for no-op cache
func (n *NoOpCache) Size() int {
	return 0
}
