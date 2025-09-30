package xray

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

func TestNewXrayProvider(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	assert.NotNil(t, provider)
	assert.Equal(t, "xray", provider.GetProviderName())
	assert.Equal(t, []string{"v1", "v2"}, provider.GetSupportedVersions())
	assert.NotNil(t, provider.BaseProvider)
	assert.NotNil(t, provider.httpClient)
	assert.NotNil(t, provider.permissionDiscoverer)
	assert.NotNil(t, provider.allOperations)
}

func TestNewXrayProviderWithNilLogger(t *testing.T) {
	provider := NewXrayProvider(nil)

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.BaseProvider)
	// Should create a NoopLogger internally
	assert.Equal(t, "xray", provider.GetProviderName())
}

func TestNewXrayProviderWithCache(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProviderWithCache(logger, nil)

	assert.NotNil(t, provider)
	assert.Equal(t, "xray", provider.GetProviderName())
	assert.Nil(t, provider.specCache) // Nil cache is acceptable
}

func TestGetToolDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	tools := provider.GetToolDefinitions()
	assert.NotEmpty(t, tools)

	// Check that all expected tool categories are present
	expectedCategories := map[string]bool{
		"security":                   false,
		"compliance":                 false,
		"monitoring":                 false,
		"policy_management":          false,
		"vulnerability_intelligence": false,
		"reporting":                  false,
	}

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.DisplayName)
		assert.NotEmpty(t, tool.Description)
		assert.NotEmpty(t, tool.Category)

		if _, ok := expectedCategories[tool.Category]; ok {
			expectedCategories[tool.Category] = true
		}
	}

	// Verify all expected categories are present
	for category, found := range expectedCategories {
		assert.True(t, found, "Expected category %s not found", category)
	}
}

func TestGetToolDefinitionsWithNilProvider(t *testing.T) {
	var provider *XrayProvider
	tools := provider.GetToolDefinitions()
	assert.Nil(t, tools)
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name           string
		credentials    map[string]string
		serverResponse int
		expectError    bool
		errorContains  string
	}{
		{
			name: "valid API key",
			credentials: map[string]string{
				"api_key": "test-api-key-1234567890",
			},
			serverResponse: http.StatusOK,
			expectError:    false,
		},
		{
			name: "valid access token",
			credentials: map[string]string{
				"api_key": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test",
			},
			serverResponse: http.StatusOK,
			expectError:    false,
		},
		{
			name:          "missing API key",
			credentials:   map[string]string{},
			expectError:   true,
			errorContains: "api_key is required",
		},
		{
			name: "invalid credentials",
			credentials: map[string]string{
				"api_key": "invalid-key",
			},
			serverResponse: http.StatusUnauthorized,
			expectError:    true,
			errorContains:  "invalid credentials",
		},
		{
			name: "server error",
			credentials: map[string]string{
				"api_key": "test-key",
			},
			serverResponse: http.StatusInternalServerError,
			expectError:    true,
			errorContains:  "received status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip server creation if we're testing missing API key
			if tt.name == "missing API key" {
				logger := &observability.NoopLogger{}
				provider := NewXrayProvider(logger)
				err := provider.ValidateCredentials(context.Background(), tt.credentials)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check authentication header
				authHeader := r.Header.Get("Authorization")
				xrayHeader := r.Header.Get("X-JFrog-Art-Api")

				if authHeader == "" && xrayHeader == "" && tt.serverResponse != http.StatusUnauthorized {
					t.Error("Expected authentication header")
				}

				w.WriteHeader(tt.serverResponse)
				if tt.serverResponse == http.StatusOK {
					_, _ = w.Write([]byte(`{"status": "ok"}`))
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewXrayProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			err := provider.ValidateCredentials(context.Background(), tt.credentials)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecuteOperation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	// Mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/system/ping":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "pong"})
		case "/api/v1/scan/artifact":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"scan_id": "scan-123",
				"status":  "initiated",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Test successful operation
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-key",
		},
	})

	result, err := provider.ExecuteOperation(ctx, "system/ping", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Test operation with parameters
	params := map[string]interface{}{
		"componentId": "docker://myrepo/myimage:latest",
	}
	result, err = provider.ExecuteOperation(ctx, "scan/artifact", params)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Test non-existent operation
	_, err = provider.ExecuteOperation(ctx, "non/existent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestGetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	mappings := provider.GetOperationMappings()
	assert.NotEmpty(t, mappings)

	// Check key operation categories exist
	expectedOps := []string{
		"system/ping",
		"system/version",
		"scan/artifact",
		"scan/build",
		"summary/artifact",
		"violations/list",
		"watches/create",
		"policies/create",
		"reports/vulnerability",
		"components/details",
	}

	for _, op := range expectedOps {
		mapping, exists := mappings[op]
		assert.True(t, exists, "Expected operation %s not found", op)
		assert.NotEmpty(t, mapping.Method)
		assert.NotEmpty(t, mapping.PathTemplate)
	}
}

func TestGetDefaultConfiguration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	config := provider.GetDefaultConfiguration()

	assert.Equal(t, "https://mycompany.jfrog.io/xray", config.BaseURL)
	assert.Equal(t, "api_key", config.AuthType)
	assert.Equal(t, "/api/v1/system/ping", config.HealthEndpoint)
	assert.NotNil(t, config.RateLimits)
	assert.NotNil(t, config.RetryPolicy)
	assert.NotEmpty(t, config.OperationGroups)

	// Check operation groups
	groupNames := make(map[string]bool)
	for _, group := range config.OperationGroups {
		groupNames[group.Name] = true
		assert.NotEmpty(t, group.DisplayName)
		assert.NotEmpty(t, group.Description)
		assert.NotEmpty(t, group.Operations)
	}

	// Verify expected groups exist
	expectedGroups := []string{"scanning", "violations", "watches", "policies", "components", "reports", "system"}
	for _, group := range expectedGroups {
		assert.True(t, groupNames[group], "Expected group %s not found", group)
	}
}

func TestGetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()
	assert.NotEmpty(t, definitions)

	// Check that definitions have required fields
	for _, def := range definitions {
		assert.NotEmpty(t, def.Name)
		assert.NotEmpty(t, def.DisplayName)
		assert.NotEmpty(t, def.Category)
		assert.NotEmpty(t, def.Description)
		assert.NotEmpty(t, def.SemanticTags)
		assert.NotEmpty(t, def.CommonPhrases)
		assert.NotNil(t, def.InputSchema)
		assert.NotEmpty(t, def.ComplexityLevel)
	}
}

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		serverStatus  int
		expectError   bool
		errorContains string
	}{
		{
			name:         "healthy service",
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:          "service unavailable",
			serverStatus:  http.StatusServiceUnavailable,
			expectError:   true,
			errorContains: "status 503",
		},
		{
			name:         "unauthorized (still healthy)",
			serverStatus: http.StatusUnauthorized,
			expectError:  false, // Auth errors don't mean service is unhealthy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/system/ping", r.URL.Path)
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewXrayProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Add credentials to context for BaseProvider's ExecuteHTTPRequest
			ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					APIKey: "test-health-check-key",
				},
			})

			err := provider.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInitializeWithPermissions(t *testing.T) {
	// Create a mock server for permission discovery
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/system/version":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"version": "3.0.0",
				"build":   "123",
			})
		case "/api/v2/policies":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "policy1"},
				{"name": "policy2"},
			})
		case "/api/v2/watches":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "watch1"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Update permission discoverer with test URL
	provider.permissionDiscoverer = NewXrayPermissionDiscoverer(logger, server.URL)

	// Initialize with permissions
	err := provider.InitializeWithPermissions(context.Background(), "test-api-key")
	assert.NoError(t, err)

	// Check that operations are filtered
	assert.NotNil(t, provider.filteredOperations)
	// The filtered operations should be a subset of all operations
	assert.LessOrEqual(t, len(provider.filteredOperations), len(provider.allOperations))
}

func TestIsJFrogAPIKey(t *testing.T) {
	provider := NewXrayProvider(nil)

	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "JFrog API key (64 chars)",
			apiKey:   "AKCp5budTFpbypBNQbGJPz3pGCi4Q1SrCxnjcrRvAhNfHqVcSfPp5ssGCtTVXBdXv",
			expected: true,
		},
		{
			name:     "JFrog API key (73 chars)",
			apiKey:   "AKCp5budTFpbypBNQbGJPz3pGCi4Q1SrCxnjcrRvAhNfHqVcSfPp5ssGCtTVXBdXvExtended",
			expected: true,
		},
		{
			name:     "JWT access token",
			apiKey:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
			expected: false,
		},
		{
			name:     "Short string",
			apiKey:   "short",
			expected: false,
		},
		{
			name:     "Very long string",
			apiKey:   "verylongstringthatismorethan73charactersandshoulddefinitelynotbeconsideredanapikey1234567890",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.isJFrogAPIKey(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetOpenAPISpec(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	spec, err := provider.GetOpenAPISpec()
	require.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, "3.0.0", spec.OpenAPI)
	assert.NotNil(t, spec.Info)
	assert.Equal(t, "JFrog Xray API", spec.Info.Title)
}

func TestGetEmbeddedSpecVersion(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	version := provider.GetEmbeddedSpecVersion()
	assert.Equal(t, "v1.0.0", version)
}

func TestClose(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	err := provider.Close()
	assert.NoError(t, err)
}
