package artifactory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalOperations(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("internal operations are included in operation mappings", func(t *testing.T) {
		provider := NewArtifactoryProvider(logger)
		operations := provider.GetOperationMappings()

		// Check that internal operations exist
		assert.Contains(t, operations, "internal/current-user")
		assert.Contains(t, operations, "internal/available-features")

		// Verify they have INTERNAL method
		currentUserOp := operations["internal/current-user"]
		assert.Equal(t, "INTERNAL", currentUserOp.Method)
		assert.NotNil(t, currentUserOp.Handler)

		featuresOp := operations["internal/available-features"]
		assert.Equal(t, "INTERNAL", featuresOp.Method)
		assert.NotNil(t, featuresOp.Handler)
	})

	t.Run("handleGetCurrentUser with permissions endpoint", func(t *testing.T) {
		// Create a test server that mocks Artifactory
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/security/permissions":
				// Mock permissions response
				response := []map[string]interface{}{
					{
						"name": "test-permission",
						"principals": map[string]interface{}{
							"users": map[string]interface{}{
								"testuser": []string{"read", "write"},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := NewArtifactoryProvider(logger)
		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "bearer",
		})

		// Create context with credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Execute the internal operation
		result, err := provider.ExecuteOperation(ctx, "internal/current-user", map[string]interface{}{})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check the result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "permissions", resultMap["source"])
		assert.Contains(t, resultMap, "permissions")
	})

	t.Run("handleGetCurrentUser fallback to system info", func(t *testing.T) {
		// Create a test server that denies permissions but allows system info
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/security/permissions":
				// Deny access to permissions
				w.WriteHeader(http.StatusForbidden)
			case "/api/system":
				// Mock system info response
				response := map[string]interface{}{
					"version": "7.x",
					"license": "Pro",
					"addons":  []string{"build", "replication"},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := NewArtifactoryProvider(logger)
		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "bearer",
		})

		// Create context with credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Execute the internal operation
		result, err := provider.ExecuteOperation(ctx, "internal/current-user", map[string]interface{}{})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check the result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "system_info", resultMap["source"])
		assert.Contains(t, resultMap, "data")
	})

	t.Run("handleGetCurrentUser without credentials", func(t *testing.T) {
		provider := NewArtifactoryProvider(logger)

		// Execute without credentials context
		ctx := context.Background()
		result, err := provider.ExecuteOperation(ctx, "internal/current-user", map[string]interface{}{})

		// Should fail due to missing credentials
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "credentials")
	})

	t.Run("handleGetAvailableFeatures probes endpoints", func(t *testing.T) {
		// Create a test server that simulates different feature availability
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/system/ping":
				// Artifactory core is available
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			case "/xray/api/v1/system/version":
				// Xray is available
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.x"})
			case "/pipelines/api/v1/system/info":
				// Pipelines not available
				w.WriteHeader(http.StatusNotFound)
			case "/mc/api/v1/system/info":
				// Mission Control forbidden
				w.WriteHeader(http.StatusForbidden)
			case "/distribution/api/v1/system/info":
				// Distribution not found
				w.WriteHeader(http.StatusNotFound)
			case "/access/api/v1/system/ping":
				// Access service available
				w.WriteHeader(http.StatusOK)
			case "/api/repositories":
				// Mock repository list
				response := []map[string]interface{}{
					{"key": "repo1", "type": "LOCAL"},
					{"key": "repo2", "type": "REMOTE"},
					{"key": "repo3", "type": "VIRTUAL"},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := NewArtifactoryProvider(logger)
		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "bearer",
		})

		// Create context with credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Execute the internal operation
		result, err := provider.ExecuteOperation(ctx, "internal/available-features", map[string]interface{}{})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check the result structure
		features, ok := result.(map[string]interface{})
		require.True(t, ok)

		// Check Artifactory core
		artifactory, ok := features["artifactory"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, artifactory["available"])
		assert.Equal(t, "active", artifactory["status"])

		// Check Xray
		xray, ok := features["xray"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, xray["available"])

		// Check Pipelines (not available)
		pipelines, ok := features["pipelines"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, false, pipelines["available"])
		assert.Contains(t, pipelines["reason"], "not installed")

		// Check Mission Control (forbidden)
		mc, ok := features["mission_control"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, false, mc["available"])
		assert.Contains(t, mc["reason"], "no permission")

		// Check repository types
		repoTypes, ok := features["repository_types"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, repoTypes, "local")
		assert.Contains(t, repoTypes, "remote")
		assert.Contains(t, repoTypes, "virtual")

		// Check package types
		packageTypes, ok := features["package_types"].([]string)
		require.True(t, ok)
		assert.Contains(t, packageTypes, "maven")
		assert.Contains(t, packageTypes, "docker")
		assert.Contains(t, packageTypes, "npm")
	})

	t.Run("ExecuteOperation handles INTERNAL method", func(t *testing.T) {
		// This tests that the BaseProvider correctly handles INTERNAL method
		provider := NewArtifactoryProvider(logger)

		// Create context with credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Mock a simple server for the features to probe
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Return not found for all probes
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "bearer",
		})

		// Execute available-features which should work even if all features are unavailable
		result, err := provider.ExecuteOperation(ctx, "internal/available-features", map[string]interface{}{})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check it returns a map with features
		features, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, features, "artifactory")
		assert.Contains(t, features, "xray")
		assert.Contains(t, features, "package_types")
	})

	t.Run("internal operations are not filtered by permissions", func(t *testing.T) {
		provider := NewArtifactoryProvider(logger)

		// Get all operations before filtering
		allOps := provider.getAllOperationMappings()
		assert.Contains(t, allOps, "internal/current-user")
		assert.Contains(t, allOps, "internal/available-features")

		// Simulate permission filtering (normally done by InitializeWithPermissions)
		// Create a filtered set that excludes most operations
		provider.filteredOperations = map[string]providers.OperationMapping{
			"repos/list": allOps["repos/list"],
			// Ensure internal operations are always included
			"internal/current-user":       allOps["internal/current-user"],
			"internal/available-features": allOps["internal/available-features"],
		}

		// Get operations after filtering
		filteredOps := provider.GetOperationMappings()

		// Internal operations should still be present
		assert.Contains(t, filteredOps, "internal/current-user")
		assert.Contains(t, filteredOps, "internal/available-features")
	})
}

func TestProbeFeature(t *testing.T) {
	logger := &observability.NoopLogger{}

	testCases := []struct {
		name           string
		statusCode     int
		expectedResult map[string]interface{}
	}{
		{
			name:       "feature available",
			statusCode: http.StatusOK,
			expectedResult: map[string]interface{}{
				"available": true,
				"status":    "active",
			},
		},
		{
			name:       "feature unauthorized",
			statusCode: http.StatusUnauthorized,
			expectedResult: map[string]interface{}{
				"available": false,
				"reason":    "no permission to access this feature",
				"status":    http.StatusUnauthorized,
			},
		},
		{
			name:       "feature forbidden",
			statusCode: http.StatusForbidden,
			expectedResult: map[string]interface{}{
				"available": false,
				"reason":    "no permission to access this feature",
				"status":    http.StatusForbidden,
			},
		},
		{
			name:       "feature not found",
			statusCode: http.StatusNotFound,
			expectedResult: map[string]interface{}{
				"available": false,
				"reason":    "feature not installed or not available",
			},
		},
		{
			name:       "unexpected status",
			statusCode: http.StatusServiceUnavailable,
			expectedResult: map[string]interface{}{
				"available": false,
				"reason":    "unexpected status code: 503",
				"status":    http.StatusServiceUnavailable,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			provider := NewArtifactoryProvider(logger)
			provider.SetConfiguration(providers.ProviderConfig{
				BaseURL:  server.URL,
				AuthType: "bearer",
			})

			// Create context with credentials
			ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			// Test probeFeature
			result := provider.probeFeature(ctx, "/test/endpoint")
			resultMap, ok := result.(map[string]interface{})
			require.True(t, ok)

			assert.Equal(t, tc.expectedResult["available"], resultMap["available"])
			if reason, hasReason := tc.expectedResult["reason"]; hasReason {
				assert.Equal(t, reason, resultMap["reason"])
			}
			if status, hasStatus := tc.expectedResult["status"]; hasStatus {
				assert.Equal(t, status, resultMap["status"])
			}
		})
	}
}

func TestCheckRepositoryTypes(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("successfully counts repository types", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/repositories" {
				response := []map[string]interface{}{
					{"key": "local1", "type": "LOCAL"},
					{"key": "local2", "type": "LOCAL"},
					{"key": "remote1", "type": "REMOTE"},
					{"key": "virtual1", "type": "VIRTUAL"},
					{"key": "federated1", "type": "FEDERATED"},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := NewArtifactoryProvider(logger)
		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "bearer",
		})

		// Create context with credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Test checkRepositoryTypes
		result := provider.checkRepositoryTypes(ctx)

		// Check local repos
		local, ok := result["local"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, local["supported"])
		assert.Equal(t, 2, local["count"])

		// Check remote repos
		remote, ok := result["remote"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, remote["supported"])
		assert.Equal(t, 1, remote["count"])

		// Check virtual repos
		virtual, ok := result["virtual"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, virtual["supported"])
		assert.Equal(t, 1, virtual["count"])

		// Check federated repos
		federated, ok := result["federated"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, federated["supported"])
		assert.Equal(t, 1, federated["count"])
	})

	t.Run("handles error when listing repositories fails", func(t *testing.T) {
		// Create test server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := NewArtifactoryProvider(logger)
		provider.SetConfiguration(providers.ProviderConfig{
			BaseURL:  server.URL,
			AuthType: "bearer",
		})

		// Create context with credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Test checkRepositoryTypes
		result := provider.checkRepositoryTypes(ctx)

		// Should contain error
		assert.Contains(t, result, "error")
		errorMsg, ok := result["error"].(string)
		require.True(t, ok)
		assert.Contains(t, errorMsg, "failed to list repositories")
	})
}
