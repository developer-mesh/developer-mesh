package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnhancedAuthRateLimiting verifies rate limiting works correctly
func TestEnhancedAuthRateLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test cache
	testCache := newTestCache()
	logger := observability.NewNoopLogger()
	metrics := observability.NewNoOpMetricsClient()

	// Create auth configuration with API keys
	config := &auth.AuthSystemConfig{
		Service: &auth.ServiceConfig{
			JWTSecret:         "test-secret-minimum-32-characters!!",
			JWTExpiration:     1 * time.Hour,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      true,
			CacheTTL:          5 * time.Minute,
			MaxFailedAttempts: 3, // Set to 3 for the test
			LockoutDuration:   15 * time.Minute,
		},
		RateLimiter: &auth.RateLimiterConfig{
			Enabled:       true,
			MaxAttempts:   3, // After 3 failed attempts, lock out
			WindowSize:    15 * time.Minute,
			LockoutPeriod: 15 * time.Minute,
		},
		APIKeys: map[string]auth.APIKeySettings{
			"valid-key-1234567890123456": {
				Role:     "admin",
				Scopes:   []string{"read", "write", "admin"},
				TenantID: testutil.TestTenantIDString(),
			},
			"limited-key-1234567890123456": {
				Role:     "user",
				Scopes:   []string{"read"},
				TenantID: testutil.TestTenantIDString(),
			},
		},
	}

	// Setup enhanced auth
	authMiddleware, err := auth.SetupAuthenticationWithConfig(config, nil, testCache, logger, metrics)
	require.NoError(t, err)

	// Create test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Auth endpoints (subject to rate limiting)
	router.POST("/auth/login", authMiddleware.GinMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// API endpoints
	v1 := router.Group("/api/v1")
	v1.Use(authMiddleware.GinMiddleware())
	v1.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "authenticated"})
	})

	t.Run("Auth endpoint rate limiting", func(t *testing.T) {
		// Clear cache for fresh test
		testCache.Clear()

		// Test IP-based rate limiting
		testIP := "10.0.0.1:12345"

		// First 3 failed attempts should be allowed
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.Header.Set("Authorization", "Bearer wrong-key")
			req.RemoteAddr = testIP

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code, "Attempt %d should fail with 401", i+1)
		}

		// 4th attempt should be rate limited
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		req.RemoteAddr = testIP

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code, "Should be rate limited after 3 failed attempts")
		assert.Equal(t, "900", w.Header().Get("Retry-After"), "Should have 15 minute retry-after")
		assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))

		// Even valid credentials should be rate limited now
		req = httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer valid-key-1234567890123456")
		req.RemoteAddr = testIP

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code, "Should remain rate limited")
	})

	t.Run("Different IPs have separate limits", func(t *testing.T) {
		testCache.Clear()

		// Two different IPs
		ip1 := "192.168.1.1:12345"
		ip2 := "192.168.1.2:12345"

		// IP1: Make 3 failed attempts
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.Header.Set("Authorization", "Bearer invalid")
			req.RemoteAddr = ip1

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// IP2: Should still be able to make requests
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer valid-key-1234567890123456")
		req.RemoteAddr = ip2

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Different IP should not be rate limited")

		// IP1: Should be rate limited
		req = httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer valid-key-1234567890123456")
		req.RemoteAddr = ip1

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code, "Original IP should be rate limited")
	})

	t.Run("API endpoints not rate limited", func(t *testing.T) {
		testCache.Clear()

		// Regular API endpoints should not trigger rate limiting
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer valid-key-1234567890123456")
			req.RemoteAddr = "10.0.0.5:12345"

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
	})
}

// TestEnhancedAuthConcurrency tests thread safety
func TestEnhancedAuthConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCache := newTestCache()
	logger := observability.NewNoopLogger()
	metrics := observability.NewNoOpMetricsClient()

	// Create auth configuration with API keys
	config := &auth.AuthSystemConfig{
		Service: &auth.ServiceConfig{
			JWTSecret:         "test-secret-minimum-32-characters!!",
			JWTExpiration:     1 * time.Hour,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      true,
			CacheTTL:          5 * time.Minute,
			MaxFailedAttempts: 5,
			LockoutDuration:   15 * time.Minute,
		},
		RateLimiter: auth.DefaultRateLimiterConfig(),
		APIKeys: map[string]auth.APIKeySettings{
			"concurrent-key-1234567890123456": {
				Role:     "admin",
				Scopes:   []string{"read", "write", "admin"},
				TenantID: testutil.TestTenantIDString(),
			},
		},
	}

	authMiddleware, err := auth.SetupAuthenticationWithConfig(config, nil, testCache, logger, metrics)
	require.NoError(t, err)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(authMiddleware.GinMiddleware())
	v1.GET("/concurrent", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Run concurrent requests
	var wg sync.WaitGroup
	numGoroutines := 20
	requestsPerGoroutine := 5
	results := make(chan int, numGoroutines*requestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/api/v1/concurrent", nil)
				req.Header.Set("Authorization", "Bearer concurrent-key-1234567890123456")
				req.RemoteAddr = fmt.Sprintf("10.0.%d.%d:12345", workerID, j)

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				results <- w.Code
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Check all requests succeeded
	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, numGoroutines*requestsPerGoroutine, successCount, "All concurrent requests should succeed")
}

// TestJWTAuthentication tests JWT token validation
func TestJWTAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCache := newTestCache()
	logger := observability.NewNoopLogger()
	metrics := observability.NewNoOpMetricsClient()

	// Create auth configuration with consistent JWT secret
	config := &auth.AuthSystemConfig{
		Service: &auth.ServiceConfig{
			JWTSecret:         "super-secret-key-minimum-32-chars",
			JWTExpiration:     1 * time.Hour,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      true,
			CacheTTL:          5 * time.Minute,
			MaxFailedAttempts: 5,
			LockoutDuration:   15 * time.Minute,
		},
		RateLimiter: auth.DefaultRateLimiterConfig(),
		APIKeys:     make(map[string]auth.APIKeySettings),
	}

	// Setup auth middleware with config
	authMiddleware, err := auth.SetupAuthenticationWithConfig(config, nil, testCache, logger, metrics)
	require.NoError(t, err)

	// Create auth service with same config for generating JWT
	authService := auth.NewService(config.Service, nil, testCache, logger)

	// Generate a valid JWT
	user := &auth.User{
		ID:       testutil.TestUserID,
		TenantID: testutil.TestTenantID,
		Email:    "test@example.com",
		Scopes:   []string{"read", "write"},
	}

	token, err := authService.GenerateJWT(context.Background(), user)
	require.NoError(t, err)

	// Create test router
	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(authMiddleware.GinMiddleware())
	v1.GET("/profile", func(c *gin.Context) {
		if user, exists := c.Get("user"); exists {
			c.JSON(http.StatusOK, user)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no user in context"})
		}
	})

	t.Run("Valid JWT authentication", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Invalid JWT", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer invalid-jwt-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Expired JWT", func(t *testing.T) {
		// Create auth service with very short expiration using same secret
		shortConfig := &auth.ServiceConfig{
			JWTSecret:         "super-secret-key-minimum-32-chars", // Same secret as main config
			JWTExpiration:     1 * time.Millisecond,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      true,
			CacheTTL:          5 * time.Minute,
			MaxFailedAttempts: 5,
			LockoutDuration:   15 * time.Minute,
		}
		shortService := auth.NewService(shortConfig, nil, testCache, logger)

		expiredToken, err := shortService.GenerateJWT(context.Background(), user)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(5 * time.Millisecond)

		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// testCache is a simple thread-safe cache for testing
type testCache struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func newTestCache() *testCache {
	return &testCache{
		data: make(map[string]interface{}),
	}
}

func (c *testCache) Get(ctx context.Context, key string, value interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if v, ok := c.data[key]; ok {
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
		case *string:
			if val, ok := v.(string); ok {
				*dst = val
				return nil
			}
		}
	}
	return fmt.Errorf("key not found")
}

func (c *testCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	return nil
}

func (c *testCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *testCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.data[key]
	return ok, nil
}

func (c *testCache) Flush(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]interface{})
	return nil
}

func (c *testCache) Close() error {
	return nil
}

func (c *testCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]interface{})
}
