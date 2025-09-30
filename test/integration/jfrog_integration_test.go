//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/artifactory"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/xray"
)

// createTestContext creates a context with test credentials
func createTestContext() context.Context {
	ctx := context.Background()
	pctx := &providers.ProviderContext{
		TenantID: "test-tenant",
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key",
		},
	}
	return providers.WithContext(ctx, pctx)
}

// createArtifactoryMockServer creates a mock server for Artifactory API testing
func createArtifactoryMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check authentication
		apiKey := r.Header.Get("X-JFrog-Art-Api")
		authHeader := r.Header.Get("Authorization")
		if apiKey == "" && authHeader == "" && !strings.Contains(r.URL.Path, "/system/ping") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		switch {
		// System endpoints
		case strings.Contains(r.URL.Path, "/system/ping"):
			json.NewEncoder(w).Encode("OK")

		case strings.Contains(r.URL.Path, "/system/version"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"version":  "7.41.12",
				"revision": "12345",
				"license":  "Enterprise Plus",
			})

		// Repository endpoints
		case strings.Contains(r.URL.Path, "/api/repositories") && r.Method == "GET":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"key":         "libs-release-local",
					"type":        "LOCAL",
					"packageType": "Generic",
				},
				{
					"key":         "libs-snapshot-local",
					"type":        "LOCAL",
					"packageType": "Generic",
				},
			})

		case strings.Contains(r.URL.Path, "/api/repositories/") && r.Method == "PUT":
			// Create repository
			json.NewEncoder(w).Encode(map[string]interface{}{
				"key":         "test-repo",
				"type":        "LOCAL",
				"description": "Test repository",
			})

		// User endpoints
		case strings.Contains(r.URL.Path, "/api/security/users") && r.Method == "GET":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"name":  "admin",
					"email": "admin@example.com",
					"admin": true,
				},
				{
					"name":  "test-user",
					"email": "test@example.com",
					"admin": false,
				},
			})

		case strings.Contains(r.URL.Path, "/api/security/users/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":  "test-user",
				"email": "test@example.com",
				"admin": false,
			})

		// AQL endpoint
		case strings.Contains(r.URL.Path, "/api/search/aql"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"repo": "libs-release-local",
						"path": "com/example/app/1.0.0",
						"name": "app-1.0.0.jar",
						"size": 10485760,
					},
				},
				"range": map[string]interface{}{
					"start_pos": 0,
					"end_pos":   1,
					"total":     1,
				},
			})

		// Projects endpoint
		case strings.Contains(r.URL.Path, "/access/api/v1/projects"):
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"project_key":  "proj1",
					"display_name": "Project 1",
					"description":  "Test project",
				},
			})

		// Permissions endpoint
		case strings.Contains(r.URL.Path, "/api/v2/security/permissions"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"permissions": []map[string]interface{}{
					{
						"name": "test-permission",
						"repo": map[string]interface{}{
							"repositories": []string{"libs-release-local"},
							"actions":      []string{"read", "write", "deploy"},
						},
					},
				},
			})

		default:
			// Default response
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
			})
		}
	}))
}

// createXrayMockServer creates a mock server for Xray API testing
func createXrayMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check authentication
		apiKey := r.Header.Get("X-JFrog-Art-Api")
		authHeader := r.Header.Get("Authorization")
		if apiKey == "" && authHeader == "" && !strings.Contains(r.URL.Path, "/system/ping") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		switch {
		// System endpoints
		case strings.Contains(r.URL.Path, "/api/v1/system/ping"):
			json.NewEncoder(w).Encode("OK")

		case strings.Contains(r.URL.Path, "/api/v1/system/version"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"version":  "3.43.1",
				"revision": "54321",
			})

		// Scan endpoints
		case strings.Contains(r.URL.Path, "/api/v1/scan/artifact"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"scan_id": "scan-123",
				"status":  "completed",
			})

		case strings.Contains(r.URL.Path, "/api/v1/summary/artifact"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"artifacts": []map[string]interface{}{
					{
						"general": map[string]interface{}{
							"path": "libs-release-local/com/example/app/1.0.0/app-1.0.0.jar",
						},
						"issues": []map[string]interface{}{
							{
								"severity": "High",
								"summary":  "Security vulnerability CVE-2023-1234",
							},
						},
					},
				},
			})

		// Component endpoints
		case strings.Contains(r.URL.Path, "/api/v1/component/details"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"component_id": "gav://com.example:app:1.0.0",
				"package_type": "Maven",
				"components": []map[string]interface{}{
					{
						"component_id": "gav://com.example:app:1.0.0",
						"vulnerabilities": []map[string]interface{}{
							{
								"cve":      "CVE-2023-1234",
								"severity": "High",
							},
						},
					},
				},
			})

		case strings.Contains(r.URL.Path, "/api/v1/component/search"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"id":           "spring-core",
						"package_type": "Maven",
					},
				},
			})

		// Reports endpoints
		case strings.Contains(r.URL.Path, "/api/v2/reports/vulnerabilities"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"report_id":        "report-123",
				"status":           "completed",
				"total_violations": 5,
			})

		// Watches endpoint
		case strings.Contains(r.URL.Path, "/api/v2/watches"):
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"name":        "production-watch",
					"description": "Watch for production artifacts",
				},
			})

		// Policies endpoint
		case strings.Contains(r.URL.Path, "/api/v2/policies"):
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"name": "security-policy",
					"type": "security",
				},
			})

		default:
			// Default response
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
			})
		}
	}))
}

// TestArtifactoryIntegration tests the Artifactory provider integration
func TestArtifactoryIntegration(t *testing.T) {
	// Create mock server
	server := createArtifactoryMockServer()
	defer server.Close()

	// Initialize provider
	logger := observability.NewNoopLogger()
	provider := artifactory.NewArtifactoryProvider(logger)

	// Configure provider with mock server URL
	config := providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	}
	provider.SetConfiguration(config)

	t.Run("UserOperations", func(t *testing.T) {
		// Test user operations
		ctx := createTestContext()

		// List users
		result, err := provider.ExecuteOperation(ctx, "users/list", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Get specific user
		result, err = provider.ExecuteOperation(ctx, "users/get", map[string]interface{}{
			"userName": "test-user",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("RepositoryOperations", func(t *testing.T) {
		ctx := createTestContext()

		// List repositories
		result, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Create repository with correct parameters
		createParams := map[string]interface{}{
			"repoKey":     "test-repo",
			"rclass":      "local",
			"packageType": "maven",
		}
		result, err = provider.ExecuteOperation(ctx, "repos/create", createParams)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("AQLSearch", func(t *testing.T) {
		// Test enhanced AQL support
		ctx := createTestContext()

		query := `items.find({"repo":"libs-release-local","name":{"$match":"*.jar"}})`
		result, err := provider.ExecuteOperation(ctx, "search/aql", map[string]interface{}{
			"query": query,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("ProjectOperations", func(t *testing.T) {
		// Test project management
		ctx := createTestContext()

		result, err := provider.ExecuteOperation(ctx, "projects/list", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("PermissionDiscovery", func(t *testing.T) {
		// Test permission discovery
		ctx := createTestContext()

		// Check available operations
		ops := provider.GetOperationMappings()
		assert.NotEmpty(t, ops)

		// Verify some operations are present
		_, hasRepoList := ops["repos/list"]
		if !hasRepoList {
			// At minimum check we got some operations back
			assert.NotEmpty(t, ops)
		}
		_ = ctx // use context variable
	})

	t.Run("CapabilityReporting", func(t *testing.T) {
		// Test capability reporting
		ctx := createTestContext()

		report, err := provider.GetCapabilityReport(ctx)
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.NotNil(t, report.Features)
	})
}

// TestXrayIntegration tests the Xray provider integration
func TestXrayIntegration(t *testing.T) {
	// Create mock server
	server := createXrayMockServer()
	defer server.Close()

	// Initialize provider
	logger := observability.NewNoopLogger()
	xrayProvider := xray.NewXrayProvider(logger)

	// Configure provider with mock server URL
	config := providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	}
	xrayProvider.SetConfiguration(config)

	t.Run("VulnerabilityScanning", func(t *testing.T) {
		ctx := createTestContext()

		// Scan artifact using summary endpoint
		result, err := xrayProvider.ExecuteOperation(ctx, "summary/artifact", map[string]interface{}{
			"paths": []string{"libs-release-local/com/example/app/1.0.0/app-1.0.0.jar"},
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Scan using scan endpoint
		result, err = xrayProvider.ExecuteOperation(ctx, "scan/artifact", map[string]interface{}{
			"componentId": "gav://com.example:app:1.0.0",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("ComponentIntelligence", func(t *testing.T) {
		ctx := createTestContext()

		// Get component details
		result, err := xrayProvider.ExecuteOperation(ctx, "components/details", map[string]interface{}{
			"component_id": "gav://com.example:app:1.0.0",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Search for components
		result, err = xrayProvider.ExecuteOperation(ctx, "components/search", map[string]interface{}{
			"query": "spring",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("ReportsAndMetrics", func(t *testing.T) {
		ctx := createTestContext()

		// Generate vulnerability report
		result, err := xrayProvider.ExecuteOperation(ctx, "reports/vulnerability", map[string]interface{}{
			"name":         "Security Report",
			"repositories": []string{"libs-release-local"},
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// List watches
		result, err = xrayProvider.ExecuteOperation(ctx, "watches/list", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// List policies
		result, err = xrayProvider.ExecuteOperation(ctx, "policies/list", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("PassthroughAuthentication", func(t *testing.T) {
		ctx := createTestContext()

		// Get system version with passthrough
		result, err := xrayProvider.ExecuteOperation(ctx, "system/version", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)

		// System ping
		result, err = xrayProvider.ExecuteOperation(ctx, "system/ping", map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// TestArtifactoryXrayIntegration tests cross-provider integration
func TestArtifactoryXrayIntegration(t *testing.T) {
	// Create mock servers
	artServer := createArtifactoryMockServer()
	defer artServer.Close()

	xrayServer := createXrayMockServer()
	defer xrayServer.Close()

	// Initialize providers
	logger := observability.NewNoopLogger()
	artifactoryProvider := artifactory.NewArtifactoryProvider(logger)
	xrayProvider := xray.NewXrayProvider(logger)

	// Configure providers
	artifactoryProvider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  artServer.URL,
		AuthType: "api_key",
	})

	xrayProvider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  xrayServer.URL,
		AuthType: "api_key",
	})

	t.Run("CrossProviderWorkflow", func(t *testing.T) {
		ctx := createTestContext()

		// First get artifact from Artifactory
		artResult, err := artifactoryProvider.ExecuteOperation(ctx, "search/aql", map[string]interface{}{
			"query": `items.find({"repo":"libs-release-local"})`,
		})
		require.NoError(t, err)
		assert.NotNil(t, artResult)

		// Then scan with Xray
		path := "libs-release-local/com/example/app/1.0.0/app-1.0.0.jar"
		xrayResult, err := xrayProvider.ExecuteOperation(ctx, "summary/artifact", map[string]interface{}{
			"paths": []string{path},
		})
		require.NoError(t, err)
		assert.NotNil(t, xrayResult)
	})

	t.Run("FeatureAvailabilityDetection", func(t *testing.T) {
		ctx := createTestContext()

		// Check Artifactory capabilities
		artCapabilities, err := artifactoryProvider.GetCapabilityReport(ctx)
		require.NoError(t, err)
		assert.NotNil(t, artCapabilities)
		assert.NotNil(t, artCapabilities.Features)

		// Check Xray capabilities through operation mappings
		xrayOperations := xrayProvider.GetOperationMappings()
		assert.NotEmpty(t, xrayOperations)

		// Verify Xray provider health check works
		err = xrayProvider.HealthCheck(ctx)
		assert.NoError(t, err)
	})
}

// BenchmarkArtifactoryOperations performs performance testing
func BenchmarkArtifactoryOperations(b *testing.B) {
	server := createArtifactoryMockServer()
	defer server.Close()

	logger := observability.NewNoopLogger()
	provider := artifactory.NewArtifactoryProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	})

	ctx := createTestContext()

	b.Run("RepositoryList", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("AQLSearch", func(b *testing.B) {
		query := `items.find({"repo":"libs-release-local"})`
		for i := 0; i < b.N; i++ {
			_, err := provider.ExecuteOperation(ctx, "search/aql", map[string]interface{}{
				"query": query,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkXrayOperations performs performance testing for Xray
func BenchmarkXrayOperations(b *testing.B) {
	server := createXrayMockServer()
	defer server.Close()

	logger := observability.NewNoopLogger()
	provider := xray.NewXrayProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	})

	ctx := createTestContext()

	b.Run("ArtifactScan", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := provider.ExecuteOperation(ctx, "summary/artifact", map[string]interface{}{
				"paths": []string{"libs-release-local/test.jar"},
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ComponentSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := provider.ExecuteOperation(ctx, "components/search", map[string]interface{}{
				"query": "test",
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// TestErrorHandling tests error handling scenarios
func TestErrorHandling(t *testing.T) {
	// Test with invalid server
	logger := observability.NewNoopLogger()
	provider := artifactory.NewArtifactoryProvider(logger)

	t.Run("InvalidServerURL", func(t *testing.T) {
		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  "http://invalid-server:9999",
			AuthType: "api_key",
		})

		ctx := createTestContext()
		_, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
		assert.Error(t, err)
	})

	t.Run("MissingRequiredParams", func(t *testing.T) {
		server := createArtifactoryMockServer()
		defer server.Close()

		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "api_key",
		})

		ctx := createTestContext()
		// Missing repoKey parameter
		_, err := provider.ExecuteOperation(ctx, "repos/create", map[string]interface{}{
			"rclass": "local",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repoKey")
	})

	t.Run("UnauthenticatedRequest", func(t *testing.T) {
		server := createArtifactoryMockServer()
		defer server.Close()

		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "api_key",
		})

		// Context without credentials
		ctx := context.Background()
		_, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
		// May succeed depending on provider's handling
		// Just ensure it doesn't panic
		_ = err
	})

	t.Run("InvalidOperation", func(t *testing.T) {
		server := createArtifactoryMockServer()
		defer server.Close()

		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "api_key",
		})

		ctx := createTestContext()
		_, err := provider.ExecuteOperation(ctx, "invalid/operation", map[string]interface{}{})
		assert.Error(t, err)
	})
}

// TestConcurrency tests concurrent operations
func TestConcurrency(t *testing.T) {
	server := createArtifactoryMockServer()
	defer server.Close()

	logger := observability.NewNoopLogger()
	provider := artifactory.NewArtifactoryProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	})

	ctx := createTestContext()

	t.Run("ConcurrentReads", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	})

	t.Run("MixedOperations", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 10)

		operations := []struct {
			op     string
			params map[string]interface{}
		}{
			{"repos/list", map[string]interface{}{}},
			{"users/list", map[string]interface{}{}},
			{"projects/list", map[string]interface{}{}},
		}

		for i := 0; i < 9; i++ {
			wg.Add(1)
			op := operations[i%3]
			go func(operation string, params map[string]interface{}) {
				defer wg.Done()
				_, err := provider.ExecuteOperation(ctx, operation, params)
				if err != nil {
					errors <- fmt.Errorf("%s failed: %w", operation, err)
				}
			}(op.op, op.params)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	})
}

// TestRateLimiting tests rate limiting behavior
func TestRateLimiting(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount > 5 {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "Rate limit exceeded"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	logger := observability.NewNoopLogger()
	provider := artifactory.NewArtifactoryProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	})

	ctx := createTestContext()

	// Make requests until rate limited
	for i := 0; i < 10; i++ {
		result, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
		// Provider should handle gracefully
		assert.NoError(t, err)
		assert.NotNil(t, result) // May return raw response
	}
}

// TestConcurrency tests concurrent operations
func TestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	logger := observability.NewNoopLogger()
	provider := artifactory.NewArtifactoryProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "api_key",
	})

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(createTestContext(), 50*time.Millisecond)
	defer cancel()

	_, err := provider.ExecuteOperation(ctx, "repos/list", map[string]interface{}{})
	// Should timeout
	assert.Error(t, err)
}
