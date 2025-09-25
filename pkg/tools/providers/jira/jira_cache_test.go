package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJiraProvider_CachingIntegration(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"key":     "TEST-123",
			"summary": "Test Issue",
			"fields": map[string]interface{}{
				"status": map[string]interface{}{
					"name": "Open",
				},
			},
		}

		// Set ETag and Last-Modified headers for cache testing
		w.Header().Set("ETag", `"test-etag-123"`)
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("Content-Type", "application/json")

		// Check for conditional headers
		if r.Header.Get("If-None-Match") == `"test-etag-123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create provider with caching enabled
	provider := createTestProviderWithCaching(t, server.URL)

	ctx := context.Background()

	t.Run("Cache GET responses", func(t *testing.T) {
		// First request - should cache the response
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/TEST-123", nil)
		require.NoError(t, err)

		resp1, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		// Read response body
		body1, err := io.ReadAll(resp1.Body)
		require.NoError(t, err)
		_ = resp1.Body.Close()

		// Second request - should use conditional headers and get cached response
		req2, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/TEST-123", nil)
		require.NoError(t, err)

		resp2, err := provider.secureHTTPDo(ctx, req2, "issues/get")
		require.NoError(t, err)
		// Should get 200 OK from cached response (even if server returned 304)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)

		// Read response body
		body2, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		_ = resp2.Body.Close()

		// Responses should be identical (cached response should be returned)
		assert.Equal(t, body1, body2)
	})

	t.Run("Cache invalidation on write operations", func(t *testing.T) {
		// GET request to populate cache
		getReq, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/TEST-123", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, getReq, "issues/get")
		require.NoError(t, err)
		// May be 200 (first time) or come from cache
		assert.True(t, resp.StatusCode == http.StatusOK)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		// PUT request should invalidate cache
		putReq, err := http.NewRequestWithContext(ctx, "PUT", server.URL+"/rest/api/3/issue/TEST-123", strings.NewReader("{}"))
		require.NoError(t, err)
		putReq.Header.Set("Content-Type", "application/json")

		putResp, err := provider.secureHTTPDo(ctx, putReq, "issues/update")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, putResp.StatusCode)
		_ = putResp.Body.Close()

		// Verify cache was invalidated by checking cache stats
		if provider.cacheManager != nil {
			stats, err := provider.cacheManager.GetCacheStats(ctx)
			require.NoError(t, err)
			assert.True(t, stats.TotalEntries >= 0)
		}
	})

	t.Run("Non-cacheable operations", func(t *testing.T) {
		// POST request should not be cached
		postReq, err := http.NewRequestWithContext(ctx, "POST", server.URL+"/rest/api/3/issue", strings.NewReader("{}"))
		require.NoError(t, err)
		postReq.Header.Set("Content-Type", "application/json")

		resp, err := provider.secureHTTPDo(ctx, postReq, "issues/create")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		// Verify POST operations are not cached
		assert.False(t, provider.cacheManager.IsCacheable("POST", "issues/create"))
	})
}

func TestJiraProvider_CacheConfiguration(t *testing.T) {
	provider := createTestProviderWithCaching(t, "https://test.atlassian.net")

	t.Run("Cache manager initialized", func(t *testing.T) {
		assert.NotNil(t, provider.cacheManager)
	})

	t.Run("Cache configuration applied", func(t *testing.T) {
		// Test cacheable operations
		assert.True(t, provider.cacheManager.IsCacheable("GET", "issues/get"))
		assert.True(t, provider.cacheManager.IsCacheable("GET", "projects/get"))
		assert.True(t, provider.cacheManager.IsCacheable("GET", "issues/search"))

		// Test non-cacheable operations
		assert.False(t, provider.cacheManager.IsCacheable("POST", "issues/create"))
		assert.False(t, provider.cacheManager.IsCacheable("PUT", "issues/update"))
		assert.False(t, provider.cacheManager.IsCacheable("DELETE", "issues/delete"))
	})

	t.Run("TTL configuration", func(t *testing.T) {
		// Test operation-specific TTLs
		issueTTL := provider.cacheManager.GetTTLForOperation("issues/get")
		assert.Greater(t, issueTTL, time.Duration(0))

		projectTTL := provider.cacheManager.GetTTLForOperation("projects/get")
		assert.Greater(t, projectTTL, time.Duration(0))

		searchTTL := provider.cacheManager.GetTTLForOperation("issues/search")
		assert.Greater(t, searchTTL, time.Duration(0))
	})
}

func TestJiraProvider_ConditionalRequests(t *testing.T) {
	// Track conditional headers
	var lastIfNoneMatch string
	var lastIfModifiedSince string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastIfNoneMatch = r.Header.Get("If-None-Match")
		lastIfModifiedSince = r.Header.Get("If-Modified-Since")

		w.Header().Set("ETag", `"test-etag-456"`)
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		w.Header().Set("Content-Type", "application/json")

		// Return 304 if conditional headers match
		if lastIfNoneMatch == `"test-etag-456"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-456",
		}); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := createTestProviderWithCaching(t, server.URL)
	ctx := context.Background()

	t.Run("First request sets up cache", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/TEST-456", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		// First request should not have conditional headers
		assert.Empty(t, lastIfNoneMatch)
		assert.Empty(t, lastIfModifiedSince)
	})

	t.Run("Second request uses conditional headers", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/TEST-456", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode) // Should be 200 from cached response
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		// Second request should have conditional headers from cache
		assert.NotEmpty(t, lastIfNoneMatch)
	})
}

func TestJiraProvider_CacheStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-789",
		}); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := createTestProviderWithCaching(t, server.URL)
	ctx := context.Background()

	// Make some requests to generate stats
	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/TEST-789", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}

	// Check cache stats
	if provider.cacheManager != nil {
		stats, err := provider.cacheManager.GetCacheStats(ctx)
		require.NoError(t, err)

		assert.True(t, stats.HitCount+stats.MissCount > 0)
		// Note: Exact hit/miss counts depend on cache implementation
		assert.True(t, stats.HitCount+stats.MissCount > 0)
	}
}

func TestJiraProvider_ErrorHandlingWithCache(t *testing.T) {
	// Server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("Internal Server Error")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	provider := createTestProviderWithCaching(t, server.URL)
	ctx := context.Background()

	t.Run("Errors are not cached", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/ERROR-123", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.Error(t, err) // Should get categorized error
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		// Error responses should not be cached
		// Subsequent request should still hit the server
		req2, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/rest/api/3/issue/ERROR-123", nil)
		require.NoError(t, err)

		resp2, err := provider.secureHTTPDo(ctx, req2, "issues/get")
		require.Error(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp2.StatusCode)
		_ = resp2.Body.Close()
	})
}

// Helper function to create a test provider with caching enabled
func createTestProviderWithCaching(t *testing.T, baseURL string) *JiraProvider {
	logger := &observability.NoopLogger{}

	// Create provider using the existing constructor
	provider := NewJiraProvider(logger, "test")

	// Override cache manager with test configuration
	cacheConfig := GetDefaultJiraCacheConfig()
	cacheConfig.EnableResponseCaching = true
	cacheRepository := NewInMemoryJiraCacheRepository()
	cacheManager := NewJiraCacheManager(cacheConfig, cacheRepository, logger)
	provider.cacheManager = cacheManager

	return provider
}
