package nexus

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

func TestNexusProvider_GetProviderName(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	assert.Equal(t, "nexus", provider.GetProviderName())
}

func TestNexusProvider_GetSupportedVersions(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	versions := provider.GetSupportedVersions()
	assert.Contains(t, versions, "v1")
	assert.Contains(t, versions, "3.83.1-03")
}

func TestNexusProvider_GetToolDefinitions(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	tools := provider.GetToolDefinitions()
	assert.NotEmpty(t, tools)

	// Check for key tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["nexus_repositories"])
	assert.True(t, toolNames["nexus_components"])
	assert.True(t, toolNames["nexus_assets"])
	assert.True(t, toolNames["nexus_search"])
	assert.True(t, toolNames["nexus_users"])
}

func TestNexusProvider_ValidateCredentials(t *testing.T) {
	tests := []struct {
		name        string
		creds       map[string]string
		serverSetup func(w http.ResponseWriter, r *http.Request)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid API Key",
			creds: map[string]string{
				"api_key": "valid-api-key",
			},
			serverSetup: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "NX-APIKEY valid-api-key", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "healthy",
				})
			},
			expectError: false,
		},
		{
			name: "Valid Bearer Token",
			creds: map[string]string{
				"token": "valid-token",
			},
			serverSetup: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer valid-token", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "healthy",
				})
			},
			expectError: false,
		},
		{
			name: "Valid Basic Auth",
			creds: map[string]string{
				"username": "admin",
				"password": "admin123",
			},
			serverSetup: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Basic YWRtaW46YWRtaW4xMjM=", r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "healthy",
				})
			},
			expectError: false,
		},
		{
			name: "Invalid Credentials",
			creds: map[string]string{
				"api_key": "invalid-key",
			},
			serverSetup: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			expectError: true,
			errorMsg:    "invalid Nexus credentials",
		},
		{
			name:        "Missing Credentials",
			creds:       map[string]string{},
			expectError: true,
			errorMsg:    "missing required credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := observability.NewNoopLogger()
			provider := NewNexusProvider(logger)

			if tt.serverSetup != nil {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/service/rest/v1/status", r.URL.Path)
					tt.serverSetup(w, r)
				}))
				defer server.Close()
				provider.SetBaseURL(server.URL + "/service/rest")
			}

			err := provider.ValidateCredentials(context.Background(), tt.creds)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNexusProvider_GetOperationMappings(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	mappings := provider.GetOperationMappings()
	assert.NotEmpty(t, mappings)

	// Check key operations exist
	assert.Contains(t, mappings, "repositories/list")
	assert.Contains(t, mappings, "components/list")
	assert.Contains(t, mappings, "assets/list")
	assert.Contains(t, mappings, "search/components")
	assert.Contains(t, mappings, "users/list")
	assert.Contains(t, mappings, "roles/list")
	assert.Contains(t, mappings, "tasks/list")

	// Check format-specific repository operations
	assert.Contains(t, mappings, "repositories/create/maven/hosted")
	assert.Contains(t, mappings, "repositories/create/npm/proxy")
	assert.Contains(t, mappings, "repositories/create/docker/group")
}

func TestNexusProvider_normalizeOperationName(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	tests := []struct {
		input    string
		expected string
	}{
		{"list", "repositories/list"},
		{"get", "repositories/get"},
		{"create", "repositories/create"},
		{"search", "search/components"},
		{"upload", "components/upload"},
		{"components_list", "components/list"},
		{"assets/get", "assets/get"},
		{"repositories/create/maven/hosted", "repositories/create/maven/hosted"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNexusProvider_ExecuteOperation(t *testing.T) {
	tests := []struct {
		name           string
		operation      string
		params         map[string]interface{}
		serverResponse interface{}
		serverStatus   int
		expectError    bool
	}{
		{
			name:      "List Repositories",
			operation: "repositories/list",
			params:    map[string]interface{}{},
			serverResponse: []map[string]interface{}{
				{
					"name":   "maven-central",
					"format": "maven2",
					"type":   "proxy",
					"url":    "https://repo1.maven.org/maven2/",
				},
				{
					"name":   "maven-releases",
					"format": "maven2",
					"type":   "hosted",
				},
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:      "Get Repository",
			operation: "repositories/get",
			params: map[string]interface{}{
				"repositoryName": "maven-central",
			},
			serverResponse: map[string]interface{}{
				"name":   "maven-central",
				"format": "maven2",
				"type":   "proxy",
				"url":    "https://repo1.maven.org/maven2/",
				"online": true,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:      "List Components",
			operation: "components/list",
			params: map[string]interface{}{
				"repository": "maven-releases",
			},
			serverResponse: map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":         "component-1",
						"repository": "maven-releases",
						"format":     "maven2",
						"group":      "com.example",
						"name":       "my-app",
						"version":    "1.0.0",
					},
				},
				"continuationToken": nil,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:      "Search Components",
			operation: "search/components",
			params: map[string]interface{}{
				"q":      "spring",
				"format": "maven2",
			},
			serverResponse: map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":      "spring-boot-starter",
						"group":   "org.springframework.boot",
						"name":    "spring-boot-starter",
						"version": "2.5.0",
					},
				},
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check authentication
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			logger := observability.NewNoopLogger()
			provider := NewNexusProvider(logger)

			// Override base URL and auth type
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			config.AuthType = "api_key"
			provider.SetConfiguration(config)

			// Add auth credentials to context
			ctx := context.Background()
			ctx = providers.WithContext(ctx, &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					APIKey: "test-api-key",
				},
			})

			result, err := provider.ExecuteOperation(ctx, tt.operation, tt.params)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.serverStatus == http.StatusNoContent {
					// For 204 responses, expect a success indicator
					resultMap, ok := result.(map[string]interface{})
					assert.True(t, ok)
					assert.Equal(t, true, resultMap["success"])
					assert.Equal(t, 204, resultMap["status"])
				} else if tt.serverResponse != nil {
					assert.NotNil(t, result)
				}
			}
		})
	}
}

func TestNexusProvider_GetAIOptimizedDefinitions(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()
	assert.NotEmpty(t, definitions)

	// Check for key AI definitions
	defNames := make(map[string]bool)
	for _, def := range definitions {
		defNames[def.Name] = true

		// Verify each definition has required fields
		assert.NotEmpty(t, def.Description)
		assert.NotEmpty(t, def.UsageExamples)
		assert.NotEmpty(t, def.SemanticTags)
	}

	assert.True(t, defNames["nexus_repositories"])
	assert.True(t, defNames["nexus_components"])
	assert.True(t, defNames["nexus_search"])
	assert.True(t, defNames["nexus_security"])
}

func TestNexusProvider_GetDefaultConfiguration(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	config := provider.GetDefaultConfiguration()

	assert.Equal(t, "http://localhost:8081/service/rest", config.BaseURL)
	assert.Equal(t, "basic", config.AuthType)
	assert.NotNil(t, config.RetryPolicy)
	assert.Equal(t, 3, config.RetryPolicy.MaxRetries)
	assert.Equal(t, 120, config.RateLimits.RequestsPerMinute)
	assert.NotEmpty(t, config.OperationGroups)
}

func TestNexusProvider_HealthCheck(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		expectError  bool
	}{
		{
			name:         "Healthy",
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "Unhealthy",
			serverStatus: http.StatusServiceUnavailable,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/service/rest/v1/status", r.URL.Path)
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"status": "healthy",
					})
				}
			}))
			defer server.Close()

			logger := observability.NewNoopLogger()
			provider := NewNexusProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL + "/service/rest"
			provider.SetConfiguration(config)

			err := provider.HealthCheck(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNexusProvider_GetEmbeddedSpecVersion(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	version := provider.GetEmbeddedSpecVersion()
	assert.Equal(t, "3.83.1-03", version)
}

func TestNexusProvider_GetOpenAPISpec(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	spec, err := provider.GetOpenAPISpec()

	// Should use embedded spec
	require.NoError(t, err)
	assert.NotNil(t, spec)
}

func TestNexusProvider_SetBaseURL(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	newURL := "https://nexus.example.com"
	provider.SetBaseURL(newURL)

	config := provider.GetCurrentConfiguration()
	assert.Equal(t, newURL, config.BaseURL)
}

func TestNexusProvider_GetEnabledModules(t *testing.T) {
	logger := observability.NewNoopLogger()
	provider := NewNexusProvider(logger)

	modules := provider.GetEnabledModules()
	assert.Contains(t, modules, "repositories")
	assert.Contains(t, modules, "components")
	assert.Contains(t, modules, "assets")
	assert.Contains(t, modules, "search")
	assert.Contains(t, modules, "security")
	assert.Contains(t, modules, "tasks")
}
