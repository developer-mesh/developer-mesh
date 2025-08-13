// Package cache provides stub implementations for cache-related functionality
package cache

import (
	"context"
	"time"
)

// Cache interface defines the operations for a caching system
type Cache interface {
	// Get retrieves data from the cache
	Get(ctx context.Context, key string, value interface{}) error
	// Set stores data in the cache
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Delete removes data from the cache
	Delete(ctx context.Context, key string) error
	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)
	// Flush clears all data from the cache
	Flush(ctx context.Context) error
	// Close closes the cache connection
	Close() error
	// Size returns the number of items in cache
	Size() int
}

// Error represents a cache-related error
type Error struct {
	Message string
}

// Error implements the error interface
func (e Error) Error() string {
	return e.Message
}

// Note: RedisConfig is now fully defined in init.go to prevent redeclaration errors

// Note: stubCache has been removed as part of the Go workspace migration.
// Use redis_cache.go and redis_cluster.go implementations instead,
// which provide full cache functionality with Redis.

// Note: NewCache is now fully implemented in init.go to prevent redeclaration errors

// ConvertFromCommonRedisConfig converts a common/cache.RedisConfig to our RedisConfig
// Kept for compatibility with external packages that might use this function
func ConvertFromCommonRedisConfig(commonConfig interface{}) RedisConfig {
	// PRODUCTION SAFETY: Type assertion with proper fallback
	if cfg, ok := commonConfig.(RedisConfig); ok {
		// IMPORTANT: Preserve the configured address, don't override it!
		result := cfg

		// Only set defaults for timeout/pool settings, NOT the address
		if result.DialTimeout == 0 {
			result.DialTimeout = time.Second * 5
		}
		if result.ReadTimeout == 0 {
			result.ReadTimeout = time.Second * 3
		}
		if result.WriteTimeout == 0 {
			result.WriteTimeout = time.Second * 3
		}
		if result.PoolSize == 0 {
			result.PoolSize = 10
		}
		if result.MinIdleConns == 0 {
			result.MinIdleConns = 2
		}
		if result.PoolTimeout == 0 {
			result.PoolTimeout = 3 // seconds
		}
		if result.MaxRetries == 0 {
			result.MaxRetries = 3
		}
		// CRITICAL: Return the result with the original Address intact
		return result
	}

	// FALLBACK: Only use localhost if type assertion completely fails
	// This should rarely happen in production
	return RedisConfig{
		Type:         "redis",
		Address:      "localhost:6379", // Default fallback ONLY if config is nil/wrong type
		DialTimeout:  time.Second * 5,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
		PoolSize:     10,
		MinIdleConns: 2,
		PoolTimeout:  3, // seconds
		MaxRetries:   3,
	}
}
