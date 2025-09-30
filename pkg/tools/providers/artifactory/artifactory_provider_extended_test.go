package artifactory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockOpenAPICacheRepository for testing cache functionality
type MockOpenAPICacheRepository struct {
	mock.Mock
}

func (m *MockOpenAPICacheRepository) Get(ctx context.Context, url string) (*openapi3.T, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*openapi3.T), args.Error(1)
}

func (m *MockOpenAPICacheRepository) Set(ctx context.Context, url string, spec *openapi3.T, ttl time.Duration) error {
	args := m.Called(ctx, url, spec, ttl)
	return args.Error(0)
}

func (m *MockOpenAPICacheRepository) Invalidate(ctx context.Context, url string) error {
	args := m.Called(ctx, url)
	return args.Error(0)
}

func (m *MockOpenAPICacheRepository) GetByHash(ctx context.Context, url string, hash string) (*openapi3.T, error) {
	args := m.Called(ctx, url, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*openapi3.T), args.Error(1)
}

// TestNewArtifactoryProviderWithCache tests the cache-enabled constructor
func TestNewArtifactoryProviderWithCache(t *testing.T) {
	logger := &observability.NoopLogger{}
	mockCache := &MockOpenAPICacheRepository{}

	provider := NewArtifactoryProviderWithCache(logger, mockCache)

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.specCache)
	assert.Equal(t, mockCache, provider.specCache)
	assert.Equal(t, "artifactory", provider.GetProviderName())
}

// TestHealthCheck_Failure tests health check failure scenarios
func TestHealthCheck_Failure(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		serverError   bool
		expectedError string
	}{
		{
			name:          "Server returns 500",
			statusCode:    http.StatusInternalServerError,
			expectedError: "request failed", // 500 errors are retried
		},
		{
			name:          "Server returns 401 Unauthorized",
			statusCode:    http.StatusUnauthorized,
			expectedError: "returned unexpected status 401",
		},
		{
			name:          "Server returns 403 Forbidden",
			statusCode:    http.StatusForbidden,
			expectedError: "returned unexpected status 403",
		},
		{
			name:          "Server connection error",
			serverError:   true,
			expectedError: "artifactory health check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)

			if tt.serverError {
				// Use an invalid URL to simulate connection error
				config := provider.GetDefaultConfiguration()
				config.BaseURL = "http://invalid-url-that-does-not-exist:99999"
				provider.SetConfiguration(config)
			} else {
				// Create a mock server that returns error status
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
				}))
				defer server.Close()

				config := provider.GetDefaultConfiguration()
				config.BaseURL = server.URL
				provider.SetConfiguration(config)
			}

			ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			err := provider.HealthCheck(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// TestHealthCheck_ContextCancellation tests context cancellation
func TestHealthCheck_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(createExtendedTestContext())
	ctx = providers.WithContext(ctx, &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	// Cancel context immediately
	cancel()

	err := provider.HealthCheck(ctx)
	assert.Error(t, err)
}

// TestExecuteOperation_MissingCredentials tests execution without credentials
func TestExecuteOperation_MissingCredentials(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Context without credentials
	ctx := context.Background()

	_, err := provider.ExecuteOperation(ctx, "repos/list", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials found")
}

// TestExecuteOperation_InvalidParameters tests parameter validation
func TestExecuteOperation_InvalidParameters(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		params    map[string]interface{}
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Missing required parameter for repo get",
			operation: "repos/get",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "repoKey",
		},
		{
			name:      "Missing required parameters for artifact upload",
			operation: "artifacts/upload",
			params: map[string]interface{}{
				"repoKey": "test-repo",
				// Missing itemPath
			},
			wantError: true,
			errorMsg:  "itemPath",
		},
		// Note: users/create requires userName and email, but userName is a path parameter
		// and email is a body parameter. BaseProvider only validates path parameters.
		// This test case is removed as body parameter validation happens at the API level.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestMade bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle common discovery endpoints
				if handleCommonDiscoveryEndpointsExtended(t, w, r) {
					return
				}
				// Track if a real operation request was made
				requestMade = true
				// Return an error response to ensure the test fails if request is made
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"Missing required parameter"}`))
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			result, err := provider.ExecuteOperation(ctx, tt.operation, tt.params)

			if tt.wantError {
				// The base provider now validates required parameters before making requests.
				// It should return an error for missing required parameters.
				assert.Error(t, err, "Should return error for missing required parameters")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error should mention the missing parameter")
				assert.False(t, requestMade, "Request should not be made with missing required parameters")
				assert.Nil(t, result, "Result should be nil when error occurs")
			}
		})
	}
}

// TestNormalizeOperationName_EdgeCases tests edge cases in normalization
func TestNormalizeOperationName_EdgeCases(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Multiple slashes",
			input:    "repos//list",
			expected: "repos//list",
		},
		{
			name:     "Mixed case",
			input:    "REPOS/LIST",
			expected: "REPOS/LIST",
		},
		{
			name:     "Special characters",
			input:    "repos@list",
			expected: "repos@list",
		},
		{
			name:     "Very long operation name",
			input:    strings.Repeat("a", 1000),
			expected: strings.Repeat("a", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetOpenAPISpec tests the OpenAPI spec method
func TestGetOpenAPISpec(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	spec, err := provider.GetOpenAPISpec()
	assert.Nil(t, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAPI specification not available")
}

// TestDiscoverOperations tests operation discovery
func TestDiscoverOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Ensure the operation mappings are correctly returned
	mappings := provider.GetOperationMappings()
	assert.Greater(t, len(mappings), 40) // Should have 50+ operations
}

// TestExecuteOperation_LargePayload tests handling of large request/response
func TestExecuteOperation_LargePayload(t *testing.T) {
	// Create a large mock response
	largeArray := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		largeArray[i] = map[string]interface{}{
			"id":          i,
			"name":        strings.Repeat("item", 100),
			"description": strings.Repeat("description", 200),
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(largeArray); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	result, err := provider.ExecuteOperation(ctx, "repos/list", nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestExecuteOperation_Timeout tests timeout handling
func TestExecuteOperation_Timeout(t *testing.T) {
	// Create a server that delays longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	config.Timeout = 100 * time.Millisecond // Very short timeout
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	_, err := provider.ExecuteOperation(ctx, "repos/list", nil)
	assert.Error(t, err)
}

// TestExecuteOperation_RetryLogic tests retry mechanism
func TestExecuteOperation_RetryLogic(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints except the test endpoint
		if r.URL.Path != "/api/storage/test-repo/test.jar" && handleCommonDiscoveryEndpointsExtended(t, w, r) {
			return
		}

		// For the test endpoint, implement retry logic
		if r.URL.Path == "/api/storage/test-repo/test.jar" {
			attemptCount++
			if attemptCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"repo": "test-repo", "path": "/test.jar", "created": "2023-01-01T00:00:00.000Z", "size": 1024}`))
			return
		}

		// Default handler for unexpected paths
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	// Retry policy is already set in default config
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	// Use artifacts/info operation which is a valid operation
	result, err := provider.ExecuteOperation(ctx, "artifacts/info", map[string]interface{}{
		"repoKey":  "test-repo",
		"itemPath": "test.jar",
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, attemptCount) // Should have retried
}

// TestExecuteOperation_RateLimiting tests rate limit handling
func TestExecuteOperation_RateLimiting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1234567890")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "Rate limit exceeded"}`))
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	// Disable retries for this test to avoid infinite retry loop
	config.RetryPolicy.MaxRetries = 0
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	_, err := provider.ExecuteOperation(ctx, "repos/list", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "429", "Error should indicate rate limiting")
}

// TestExecuteOperation_ContentTypeHandling tests different content types
func TestExecuteOperation_ContentTypeHandling(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		expectError bool
	}{
		{
			name:        "JSON response",
			contentType: "application/json",
			body:        `{"key": "value"}`,
			expectError: false,
		},
		{
			name:        "JSON with charset",
			contentType: "application/json; charset=utf-8",
			body:        `{"key": "value"}`,
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			contentType: "application/json",
			body:        `{invalid json}`,
			expectError: true,
		},
		{
			name:        "HTML error response",
			contentType: "text/html",
			body:        `<html><body>Error</body></html>`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			result, err := provider.ExecuteOperation(ctx, "repos/list", nil)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestConcurrentOperations tests concurrent execution safety
func TestConcurrentOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	// Run multiple concurrent operations
	concurrency := 10
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			_, err := provider.ExecuteOperation(ctx, "repos/list", nil)
			errors <- err
		}()
	}

	// Collect results
	for i := 0; i < concurrency; i++ {
		err := <-errors
		assert.NoError(t, err)
	}
}

// TestSecurityHeaders tests security headers are properly set
func TestSecurityHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check security headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "2", r.Header.Get("X-JFrog-Art-Api-Version"))
		assert.NotEmpty(t, r.Header.Get("Authorization"))
		assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-secure-token",
		},
	})

	_, err := provider.ExecuteOperation(ctx, "repos/list", nil)
	assert.NoError(t, err)
}

// BenchmarkNormalizeOperationName benchmarks the normalization function
func BenchmarkNormalizeOperationName(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	operations := []string{
		"list",
		"repos_list",
		"repos-list",
		"repos/list",
		"artifacts_properties_set",
		"very_long_operation_name_with_many_underscores",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, op := range operations {
			_ = provider.normalizeOperationName(op)
		}
	}
}

// BenchmarkExecuteOperation benchmarks operation execution
func BenchmarkExecuteOperation(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	ctx := providers.WithContext(createExtendedTestContext(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.ExecuteOperation(ctx, "repos/list", nil)
	}
}

// Helper function for creating test context with credentials
func createExtendedTestContext() context.Context {
	return providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key-12345",
		},
	})
}

// Helper function to handle common discovery endpoints in extended mock servers
func handleCommonDiscoveryEndpointsExtended(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/api/system/ping", "/access/api/v1/system/ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return true
	case "/api/system/configuration":
		// System configuration endpoint
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"urlBase":     "http://test.artifactory.com",
			"offlineMode": false,
		})
		return true
	case "/xray/api/v1/system/version":
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.0.0", "revision": "123"})
		return true
	case "/pipelines/api/v1/system/info", "/mc/api/v1/system/info",
		"/distribution/api/v1/system/info", "/api/federation/status":
		// These are feature discovery endpoints - return 404 to indicate not available
		w.WriteHeader(http.StatusNotFound)
		return true
	case "/api/repositories":
		// Repository list for permission discovery
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{"key": "test-repo", "type": "LOCAL"},
		})
		return true
	case "/api/v2/security/permissions":
		// Permission discovery endpoint
		if r.Method == "GET" {
			// Return empty permissions list for discovery
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"permissions": []map[string]interface{}{},
			})
		} else {
			// For other methods, just return OK
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
			})
		}
		return true
	case "/access/api/v1/projects":
		// Handle GET for projects list (capability discovery)
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"projects": []map[string]interface{}{},
			})
			return true
		}
		return false
	}
	return false
}
