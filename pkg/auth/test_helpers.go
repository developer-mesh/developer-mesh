package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/require"
)

// TestCache implements cache.Cache interface for testing
type TestCache struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewTestCache creates a new test cache instance
func NewTestCache() *TestCache {
	return &TestCache{
		data: make(map[string]interface{}),
	}
}

// Get retrieves a value from cache, returns error if not found
func (c *TestCache) Get(ctx context.Context, key string, value interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stored, exists := c.data[key]
	if !exists {
		// For rate limiter keys, return zero value without error
		// For other keys (like auth keys), return an error
		if isRateLimiterKey(key) {
			return c.setZeroValue(value)
		}
		return fmt.Errorf("key not found: %s", key)
	}

	// Handle different type conversions
	return c.unmarshalValue(stored, value)
}

// Set stores a value in cache
func (c *TestCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store a copy to prevent external mutations
	c.data[key] = c.marshalValue(value)
	return nil
}

// Delete removes a key from cache
func (c *TestCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	return nil
}

// Exists checks if key exists
func (c *TestCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.data[key]
	return exists, nil
}

// Flush clears all cache data
func (c *TestCache) Flush(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]interface{})
	return nil
}

// Close is a no-op for test cache
func (c *TestCache) Close() error {
	return nil
}

// Helper methods

// isRateLimiterKey checks if a key is used by the rate limiter
func isRateLimiterKey(key string) bool {
	// Rate limiter keys typically have patterns like:
	// "auth:ratelimit:ip:192.168.1.1:count"
	// "auth:ratelimit:ip:192.168.1.1:lockout"
	// "rate_limit:ip:192.168.1.1"
	// "failed_attempts:ip:192.168.1.1"
	return strings.HasPrefix(key, "auth:ratelimit:") ||
		strings.HasPrefix(key, "rate_limit:") ||
		strings.HasPrefix(key, "failed_attempts:") ||
		strings.Contains(key, ":attempts:") ||
		strings.Contains(key, ":count") ||
		strings.Contains(key, ":lockout")
}

func (c *TestCache) setZeroValue(value interface{}) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("value must be a pointer")
	}

	elem := v.Elem()
	elem.Set(reflect.Zero(elem.Type()))
	return nil
}

func (c *TestCache) marshalValue(value interface{}) interface{} {
	// For test purposes, use JSON marshaling for deep copy
	data, err := json.Marshal(value)
	if err != nil {
		return value // Fallback to shallow copy
	}
	return string(data)
}

func (c *TestCache) unmarshalValue(stored, target interface{}) error {
	// Type assertion for common types
	switch v := target.(type) {
	case *int:
		switch s := stored.(type) {
		case int:
			*v = s
			return nil
		case float64: // JSON numbers
			*v = int(s)
			return nil
		case string: // Stored as JSON
			return json.Unmarshal([]byte(s), v)
		}
	case *string:
		if s, ok := stored.(string); ok {
			*v = s
			return nil
		}
	case *bool:
		if b, ok := stored.(bool); ok {
			*v = b
			return nil
		}
	case *time.Time:
		switch s := stored.(type) {
		case time.Time:
			*v = s
			return nil
		case string:
			return json.Unmarshal([]byte(s), v)
		}
	default:
		// Generic JSON unmarshal for complex types
		if s, ok := stored.(string); ok {
			return json.Unmarshal([]byte(s), target)
		}
	}

	return fmt.Errorf("cannot unmarshal %T into %T", stored, target)
}

// Size returns the size of the cache
func (c *TestCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// Test Configuration Helpers

// TestAuthConfig creates a complete test configuration
func TestAuthConfig() *AuthSystemConfig {
	return &AuthSystemConfig{
		Service: &ServiceConfig{
			JWTSecret:         "test-secret-minimum-32-characters!!",
			JWTExpiration:     1 * time.Hour,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      true, // Enable for rate limit testing
			MaxFailedAttempts: 5,
			LockoutDuration:   15 * time.Minute,
		},
		RateLimiter: TestRateLimiterConfig(),
		APIKeys: map[string]APIKeySettings{
			"test-key-1234567890": {
				Role:     "admin",
				Scopes:   []string{"read", "write", "admin"},
				TenantID: "11111111-1111-1111-1111-111111111111",
			},
			"user-key-1234567890": {
				Role:     "user",
				Scopes:   []string{"read"},
				TenantID: "11111111-1111-1111-1111-111111111111",
			},
		},
	}
}

// SetupTestAuth creates a complete test authentication system
func SetupTestAuth(t *testing.T) (*AuthMiddleware, *TestCache, observability.MetricsClient) {
	cache := NewTestCache()
	logger := observability.NewNoopLogger()
	metrics := observability.NewNoOpMetricsClient()

	config := TestAuthConfig()

	middleware, err := SetupAuthenticationWithConfig(
		config,
		nil, // No database for unit tests
		cache,
		logger,
		metrics,
	)
	require.NoError(t, err, "Failed to setup test auth")

	return middleware, cache, metrics
}

// SetupTestAuthWithConfig allows custom configuration
func SetupTestAuthWithConfig(t *testing.T, config *AuthSystemConfig) (*AuthMiddleware, *TestCache) {
	cache := NewTestCache()
	logger := observability.NewNoopLogger()
	metrics := observability.NewNoOpMetricsClient()

	middleware, err := SetupAuthenticationWithConfig(
		config,
		nil,
		cache,
		logger,
		metrics,
	)
	require.NoError(t, err, "Failed to setup test auth with config")

	return middleware, cache
}
