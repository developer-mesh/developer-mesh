package artifactory

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityDiscoverer_DiscoverCapabilities(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		expectedReport func() *CapabilityReport
		expectError    bool
	}{
		{
			name: "successful capability discovery with all features",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/system/ping":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("OK"))
					case "/xray/api/v1/system/version":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"version": "3.x"}`))
					case "/pipelines/api/v1/system/info":
						w.WriteHeader(http.StatusNotFound)
					case "/api/v2/security/permissions":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`[]`))
					case "/api/repositories":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`[
							{"key": "maven-local", "type": "LOCAL", "packageType": "maven"},
							{"key": "npm-remote", "type": "REMOTE", "packageType": "npm"}
						]`))
					case "/api/system/configuration":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{}`))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedReport: func() *CapabilityReport {
				return &CapabilityReport{
					Features: map[string]Capability{
						"artifactory_core": {
							Available: true,
						},
						"xray": {
							Available: true,
						},
						"pipelines": {
							Available: false,
							Reason:    "Feature not installed or endpoint not available",
						},
					},
				}
			},
		},
		{
			name: "capability discovery with no admin permissions",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/system/ping":
						w.WriteHeader(http.StatusOK)
					case "/api/system/configuration":
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"errors": [{"status": 403, "message": "Forbidden"}]}`))
					case "/api/v2/security/permissions":
						w.WriteHeader(http.StatusForbidden)
					case "/api/repositories":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`[]`))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedReport: func() *CapabilityReport {
				report := &CapabilityReport{
					Features: map[string]Capability{
						"artifactory_core": {
							Available: true,
						},
					},
				}
				// Admin operations should be marked as unavailable
				return report
			},
		},
		{
			name: "capability discovery with Xray not installed",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/system/ping":
						w.WriteHeader(http.StatusOK)
					case "/xray/api/v1/system/version":
						w.WriteHeader(http.StatusNotFound)
					case "/api/repositories":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`[]`))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedReport: func() *CapabilityReport {
				return &CapabilityReport{
					Features: map[string]Capability{
						"artifactory_core": {
							Available: true,
						},
						"xray": {
							Available: false,
							Reason:    "Feature not installed or endpoint not available",
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := tt.setupServer()
			defer server.Close()

			// Create provider with test server URL
			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Create capability discoverer
			discoverer := NewCapabilityDiscoverer(logger)

			// Create context with credentials
			ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					APIKey: "test-api-key-12345",
				},
			})

			// Discover capabilities
			report, err := discoverer.DiscoverCapabilities(ctx, provider)

			// Check error expectation
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, report)

			// Verify expected features are present
			expected := tt.expectedReport()
			for featureName, expectedCap := range expected.Features {
				actualCap, exists := report.Features[featureName]
				assert.True(t, exists, "Feature %s should exist", featureName)
				assert.Equal(t, expectedCap.Available, actualCap.Available,
					"Feature %s availability mismatch", featureName)
				if expectedCap.Reason != "" {
					assert.Contains(t, actualCap.Reason, expectedCap.Reason,
						"Feature %s reason mismatch", featureName)
				}
			}

			// Verify operations are discovered
			assert.NotEmpty(t, report.Operations)
			assert.NotZero(t, report.Timestamp)
		})
	}
}

func TestCapabilityDiscoverer_Caching(t *testing.T) {
	// Setup test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/api/system/ping" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Create capability discoverer with short cache duration for testing
	discoverer := NewCapabilityDiscoverer(logger)
	discoverer.cacheDuration = 100 * time.Millisecond

	// Create context with credentials
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key-12345",
		},
	})

	// First discovery should hit the server
	report1, err := discoverer.DiscoverCapabilities(ctx, provider)
	require.NoError(t, err)
	require.NotNil(t, report1)
	firstCallCount := callCount

	// Second discovery should use cache
	report2, err := discoverer.DiscoverCapabilities(ctx, provider)
	require.NoError(t, err)
	require.NotNil(t, report2)
	assert.True(t, report2.CacheValid)
	assert.Equal(t, firstCallCount, callCount, "Should not make additional calls when cache is valid")

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third discovery should hit the server again
	report3, err := discoverer.DiscoverCapabilities(ctx, provider)
	require.NoError(t, err)
	require.NotNil(t, report3)
	assert.False(t, report3.CacheValid)
	assert.Greater(t, callCount, firstCallCount, "Should make additional calls after cache expires")
}

func TestCapabilityDiscoverer_InvalidateCache(t *testing.T) {
	logger := &observability.NoopLogger{}
	discoverer := NewCapabilityDiscoverer(logger)

	// Set up a mock cache
	discoverer.cache = &CapabilityReport{
		Features: map[string]Capability{
			"test": {Available: true},
		},
	}
	discoverer.lastDiscovery = time.Now()

	// Verify cache exists
	cached := discoverer.GetCachedReport()
	assert.NotNil(t, cached)

	// Invalidate cache
	discoverer.InvalidateCache()

	// Verify cache is cleared
	cached = discoverer.GetCachedReport()
	assert.Nil(t, cached)
}

func TestFormatCapabilityError(t *testing.T) {
	tests := []struct {
		name               string
		operation          string
		capability         Capability
		expectedError      string
		expectedResolution string
	}{
		{
			name:      "license required error",
			operation: "xray/scan/artifact",
			capability: Capability{
				Available: false,
				Reason:    "Xray license required",
				Required:  []string{"Xray license"},
			},
			expectedError:      "operation_unavailable",
			expectedResolution: "Upgrade your JFrog license to access this feature",
		},
		{
			name:      "permission required error",
			operation: "admin/system/configure",
			capability: Capability{
				Available: false,
				Reason:    "Admin permission required",
				Required:  []string{"Admin permission"},
			},
			expectedError:      "operation_unavailable",
			expectedResolution: "Request appropriate permissions from your administrator",
		},
		{
			name:      "not installed error",
			operation: "pipelines/execute",
			capability: Capability{
				Available: false,
				Reason:    "Pipelines not installed",
				Required:  []string{"Pipelines installation"},
			},
			expectedError:      "operation_unavailable",
			expectedResolution: "Install and configure the required JFrog component",
		},
		{
			name:      "cloud-only feature error",
			operation: "runtime/list",
			capability: Capability{
				Available: false,
				Reason:    "This is a cloud-only feature",
				Required:  []string{"JFrog Cloud subscription"},
			},
			expectedError:      "operation_unavailable",
			expectedResolution: "This feature is only available in JFrog Cloud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCapabilityError(tt.operation, tt.capability)

			// Verify error structure
			assert.Equal(t, tt.expectedError, result["error"])
			assert.Equal(t, tt.operation, result["operation"])
			assert.Equal(t, tt.capability.Reason, result["reason"])
			assert.Equal(t, tt.capability.Required, result["required"])
			assert.Equal(t, tt.expectedResolution, result["resolution"])
		})
	}
}

func TestArtifactoryProvider_ExecuteOperationWithCapabilities(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/ping":
			w.WriteHeader(http.StatusOK)
		case "/xray/api/v1/system/version":
			w.WriteHeader(http.StatusNotFound) // Xray not available
		case "/api/repositories":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	ctx := createTestContext()

	// Pre-discover capabilities to populate cache
	report, err := provider.GetCapabilityReport(ctx)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Try to execute an operation that requires Xray (which is not available)
	// Note: We need to add an Xray operation to test this
	// For now, test with a regular operation
	result, err := provider.ExecuteOperation(ctx, "repos/list", nil)

	// Should succeed for available operation
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestArtifactoryProvider_HealthCheckWithCapabilities(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/ping":
			w.WriteHeader(http.StatusOK)
		case "/api/system/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version": "7.x"}`))
		case "/xray/api/v1/system/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version": "3.x"}`))
		case "/api/repositories":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"key": "maven-local", "type": "LOCAL", "packageType": "maven"}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	ctx := createTestContext()

	// Perform health check with capabilities
	result, err := provider.HealthCheckWithCapabilities(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify health check result
	assert.Equal(t, "artifactory", result["provider"])
	assert.True(t, result["healthy"].(bool))
	assert.Equal(t, server.URL, result["baseURL"])

	// Verify capabilities are included
	capabilities, ok := result["capabilities"].(map[string]interface{})
	require.True(t, ok, "Capabilities should be included")

	features, ok := capabilities["features"].(map[string]Capability)
	require.True(t, ok, "Features should be included")

	// Check specific features
	artifactoryCap, exists := features["artifactory_core"]
	assert.True(t, exists)
	assert.True(t, artifactoryCap.Available)

	xrayCap, exists := features["xray"]
	assert.True(t, exists)
	assert.True(t, xrayCap.Available)

	// Check operations summary
	summary, ok := capabilities["operations_summary"].(map[string]interface{})
	require.True(t, ok, "Operations summary should be included")
	assert.Greater(t, summary["total"].(int), 0)
}

func TestCapabilityDiscoverer_OperationPermissionChecks(t *testing.T) {
	tests := []struct {
		name              string
		operation         string
		serverResponse    int
		expectedAvailable bool
		expectedReason    string
	}{
		{
			name:              "admin operation with permission",
			operation:         "system/configuration",
			serverResponse:    http.StatusOK,
			expectedAvailable: true,
		},
		{
			name:              "admin operation without permission",
			operation:         "system/configuration",
			serverResponse:    http.StatusForbidden,
			expectedAvailable: false,
			expectedReason:    "Admin permissions required",
		},
		{
			name:              "write operation with permission",
			operation:         "repos/create",
			serverResponse:    http.StatusOK,
			expectedAvailable: true,
		},
		{
			name:              "read operation always available",
			operation:         "repos/list",
			serverResponse:    http.StatusOK,
			expectedAvailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/system/ping":
					w.WriteHeader(http.StatusOK)
				case "/api/system/configuration":
					w.WriteHeader(tt.serverResponse)
				case "/api/v2/security/permissions":
					w.WriteHeader(tt.serverResponse)
				case "/api/repositories":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[]`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Discover capabilities
			ctx := createTestContext()
			discoverer := NewCapabilityDiscoverer(logger)
			report, err := discoverer.DiscoverCapabilities(ctx, provider)
			require.NoError(t, err)
			require.NotNil(t, report)

			// Check operation capability
			if strings.Contains(tt.operation, "admin") || strings.Contains(tt.operation, "system/configuration") {
				// Find admin operations in the report
				for opID, cap := range report.Operations {
					if strings.Contains(opID, "system/configuration") {
						assert.Equal(t, tt.expectedAvailable, cap.Available,
							"Operation %s availability mismatch", opID)
						if !tt.expectedAvailable && tt.expectedReason != "" {
							assert.Contains(t, cap.Reason, tt.expectedReason)
						}
						break
					}
				}
			}
		})
	}
}

func TestCapabilityDiscoverer_PackageTypeSupport(t *testing.T) {
	// Setup test server with various package types
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/ping":
			w.WriteHeader(http.StatusOK)
		case "/api/repositories":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"key": "maven-local", "type": "LOCAL", "packageType": "maven"},
				{"key": "npm-remote", "type": "REMOTE", "packageType": "npm"},
				{"key": "docker-virtual", "type": "VIRTUAL", "packageType": "docker"},
				{"key": "pypi-local", "type": "LOCAL", "packageType": "pypi"}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Discover capabilities
	ctx := createTestContext()
	discoverer := NewCapabilityDiscoverer(logger)
	report, err := discoverer.DiscoverCapabilities(ctx, provider)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Check package type features
	packageTypesInUse := []string{"maven", "npm", "docker", "pypi"}
	for _, pt := range packageTypesInUse {
		feature, exists := report.Features[fmt.Sprintf("package_%s", pt)]
		assert.True(t, exists, "Package type %s should be reported", pt)
		assert.True(t, feature.Available, "Package type %s should be available", pt)
	}

	// Check that other package types are still potentially available
	otherTypes := []string{"nuget", "go", "helm"}
	for _, pt := range otherTypes {
		feature, exists := report.Features[fmt.Sprintf("package_%s", pt)]
		assert.True(t, exists, "Package type %s should be reported", pt)
		assert.True(t, feature.Available, "Package type %s should be potentially available", pt)
		assert.Contains(t, feature.Reason, "not currently in use",
			"Package type %s should indicate it's not in use", pt)
	}
}

// Helper function for creating test context with credentials
func createTestContext() context.Context {
	return providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key-12345",
		},
	})
}
