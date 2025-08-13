package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestAuthMiddlewareRateLimiting tests rate limiting functionality
func TestAuthMiddlewareRateLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock cache
	mockCache := &mockCache{
		data: make(map[string]interface{}),
	}

	// Create auth service
	authConfig := auth.DefaultConfig()
	authConfig.JWTSecret = "test-secret"
	authService := auth.NewService(authConfig, nil, mockCache, observability.NewNoopLogger())

	// Add test API key
	authService.InitializeDefaultAPIKeys(map[string]string{
		"test-key": "admin",
	})

	// Setup enhanced auth with rate limiting
	rateLimiter := auth.NewRateLimiter(mockCache, observability.NewNoopLogger(), &auth.RateLimiterConfig{
		Enabled:       true,
		MaxAttempts:   3,
		WindowSize:    1 * time.Minute,
		LockoutPeriod: 5 * time.Minute,
	})

	metricsCollector := auth.NewMetricsCollector(observability.NewNoOpMetricsClient())
	auditLogger := auth.NewAuditLogger(observability.NewNoopLogger())
	authMiddleware := auth.NewAuthMiddleware(authService, rateLimiter, metricsCollector, auditLogger)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Auth endpoint for testing rate limits
	router.POST("/auth/login", authMiddleware.GinMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// API endpoint with auth
	v1 := router.Group("/api/v1")
	v1.Use(authMiddleware.GinMiddleware())
	v1.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "authenticated"})
	})

	t.Run("Rate limit on auth endpoint", func(t *testing.T) {
		// Make 3 failed attempts
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.Header.Set("Authorization", "Bearer invalid-key")
			req.RemoteAddr = "192.168.1.1:12345"

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code, "Request %d should be unauthorized", i+1)
		}

		// 4th attempt should be rate limited
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer invalid-key")
		req.RemoteAddr = "192.168.1.1:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "300", w.Header().Get("Retry-After"))
		assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("No rate limit on non-auth endpoints", func(t *testing.T) {
		// Clear cache
		mockCache.data = make(map[string]interface{})

		// Make multiple requests to API endpoint
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			req.RemoteAddr = "192.168.1.2:12345"

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
	})

	t.Run("Rate limit per IP", func(t *testing.T) {
		// Clear cache
		mockCache.data = make(map[string]interface{})

		// Different IPs should have separate rate limits
		ips := []string{"10.0.0.1:12345", "10.0.0.2:12345", "10.0.0.3:12345"}

		for _, ip := range ips {
			for i := 0; i < 3; i++ {
				req := httptest.NewRequest("POST", "/auth/login", nil)
				req.Header.Set("Authorization", "Bearer invalid-key")
				req.RemoteAddr = ip

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusUnauthorized, w.Code, "IP %s request %d should be unauthorized", ip, i+1)
			}
		}

		// Each IP should now be rate limited
		for _, ip := range ips {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.Header.Set("Authorization", "Bearer invalid-key")
			req.RemoteAddr = ip

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusTooManyRequests, w.Code, "IP %s should be rate limited", ip)
		}
	})
}

// TestAuthMiddlewareConcurrency tests concurrent request handling
func TestAuthMiddlewareConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup similar to above
	mockCache := &mockCache{
		data: make(map[string]interface{}),
		mu:   sync.RWMutex{},
	}

	authConfig := auth.DefaultConfig()
	authConfig.JWTSecret = "test-secret"
	authService := auth.NewService(authConfig, nil, mockCache, observability.NewNoopLogger())

	authService.InitializeDefaultAPIKeys(map[string]string{
		"test-key": "admin",
	})

	// Create auth middleware with proper setup
	rateLimiter := auth.NewRateLimiter(mockCache, observability.NewNoopLogger(), nil)
	metricsCollector := auth.NewMetricsCollector(observability.NewNoOpMetricsClient())
	auditLogger := auth.NewAuditLogger(observability.NewNoopLogger())
	authMiddleware := auth.NewAuthMiddleware(authService, rateLimiter, metricsCollector, auditLogger)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(authMiddleware.GinMiddleware())
	v1.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test concurrent requests
	var wg sync.WaitGroup
	numRequests := 50
	results := make([]int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			req.RemoteAddr = "192.168.1.100:12345"

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			results[index] = w.Code
		}(i)
	}

	wg.Wait()

	// All requests should succeed
	for i, code := range results {
		assert.Equal(t, http.StatusOK, code, "Request %d should succeed", i)
	}
}

// isRateLimiterKey checks if a key is used by the rate limiter
func isRateLimiterKey(key string) bool {
	// Rate limiter keys typically have patterns like:
	// "auth:ratelimit:ip:192.168.1.1:count"
	// "auth:ratelimit:ip:192.168.1.1:lockout"
	return strings.HasPrefix(key, "auth:ratelimit:") ||
		strings.Contains(key, ":count") ||
		strings.Contains(key, ":lockout")
}

// mockCache implements a simple thread-safe cache for testing
type mockCache struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if v, ok := m.data[key]; ok {
		// Simple assignment for testing
		switch dst := value.(type) {
		case *int:
			if val, ok := v.(int); ok {
				*dst = val
				return nil
			}
		case *bool:
			if val, ok := v.(bool); ok {
				*dst = val
				return nil
			}
		}
	}

	// For rate limiter keys, return zero value instead of error
	if isRateLimiterKey(key) {
		switch dst := value.(type) {
		case *int:
			*dst = 0
			return nil
		case *bool:
			*dst = false
			return nil
		}
	}

	return fmt.Errorf("key not found")
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockCache) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCache) Close() error {
	return nil
}

func (m *mockCache) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}
