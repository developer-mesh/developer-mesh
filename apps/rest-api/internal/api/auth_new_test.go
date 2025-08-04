package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedAuthenticationComplete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Rate limiting after failed attempts", func(t *testing.T) {
		// Setup
		config := auth.TestAuthConfig()
		middleware, _ := auth.SetupTestAuthWithConfig(t, config)

		// Create router
		router := gin.New()
		// The middleware should handle authentication - if it passes, then auth succeeded
		// If auth fails, the middleware should return 401 and this handler won't be called
		router.POST("/auth/login", middleware.GinMiddleware(), func(c *gin.Context) {
			// This should only be reached if authentication succeeds
			c.JSON(http.StatusOK, gin.H{"status": "success"})
		})

		testIP := "192.168.1.100:12345"

		// Make 3 failed attempts (rate limit is 3)
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.Header.Set("Authorization", "Bearer invalid-key")
			req.RemoteAddr = testIP

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"Attempt %d should fail with 401", i+1)
		}

		// 4th attempt should be rate limited
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer invalid-key")
		req.RemoteAddr = testIP

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code,
			"Should be rate limited after 3 attempts")
		assert.Equal(t, "300", w.Header().Get("Retry-After"),
			"Should have 5 minute retry-after header")
	})

	t.Run("Different IPs have separate limits", func(t *testing.T) {
		config := auth.TestAuthConfig()
		middleware, _ := auth.SetupTestAuthWithConfig(t, config)

		router := gin.New()
		router.POST("/auth/login", middleware.GinMiddleware(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "success"})
		})

		// IP 1: exhaust rate limit
		ip1 := "10.0.0.1:12345"
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/auth/login", nil)
			req.Header.Set("Authorization", "Bearer bad-key")
			req.RemoteAddr = ip1

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// IP 1: should be rate limited
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer bad-key")
		req.RemoteAddr = ip1

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		// IP 2: should still work
		req = httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("Authorization", "Bearer test-key-1234567890")
		req.RemoteAddr = "10.0.0.2:12345"

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code,
			"Different IP should not be rate limited")
	})

	t.Run("Successful auth resets rate limit", func(t *testing.T) {
		config := auth.TestAuthConfig()
		middleware, _ := auth.SetupTestAuthWithConfig(t, config)

		router := gin.New()
		v1 := router.Group("/api/v1")
		v1.Use(middleware.GinMiddleware())
		v1.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"data": "success"})
		})

		testIP := "192.168.1.50:12345"

		// Make 2 failed attempts
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer wrong-key")
			req.RemoteAddr = testIP

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		// Successful auth should reset counter
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("Authorization", "Bearer test-key-1234567890")
		req.RemoteAddr = testIP

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Should be able to fail 3 more times
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer wrong-key")
			req.RemoteAddr = testIP

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"Should allow failures after successful auth")
		}
	})
}

func TestEnhancedAuthJWTFixed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Valid JWT authentication", func(t *testing.T) {
		config := auth.TestAuthConfig()
		middleware, cache := auth.SetupTestAuthWithConfig(t, config)

		// Create a JWT token using the auth service
		authService := auth.NewService(config.Service, nil, cache, observability.NewNoopLogger())
		user := &auth.User{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Email:    "test@example.com",
			Scopes:   []string{"read", "write"},
		}

		token, err := authService.GenerateJWT(context.Background(), user)
		require.NoError(t, err)

		router := gin.New()
		v1 := router.Group("/api/v1")
		v1.Use(middleware.GinMiddleware())
		v1.GET("/profile", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "authenticated"})
		})

		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Invalid JWT token", func(t *testing.T) {
		config := auth.TestAuthConfig()
		middleware, _ := auth.SetupTestAuthWithConfig(t, config)

		router := gin.New()
		v1 := router.Group("/api/v1")
		v1.Use(middleware.GinMiddleware())
		v1.GET("/profile", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "authenticated"})
		})

		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestEnhancedAuthConcurrencyFixed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := auth.TestAuthConfig()
	middleware, _ := auth.SetupTestAuthWithConfig(t, config)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.Use(middleware.GinMiddleware())
	v1.GET("/concurrent", func(c *gin.Context) {
		time.Sleep(1 * time.Millisecond) // Small delay to test concurrency
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test concurrent requests
	numRequests := 10
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			req := httptest.NewRequest("GET", "/api/v1/concurrent", nil)
			req.Header.Set("Authorization", "Bearer test-key-1234567890")
			req.RemoteAddr = fmt.Sprintf("10.0.0.%d:12345", id)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			results <- w.Code
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		code := <-results
		if code == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
}
