package harness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
)

func TestNewHarnessProvider(t *testing.T) {
	logger := &observability.NoopLogger{}

	provider := NewHarnessProvider(logger)

	assert.NotNil(t, provider)
	assert.Equal(t, "harness", provider.GetProviderName())
	assert.NotNil(t, provider.BaseProvider)
	assert.NotNil(t, provider.httpClient)
	assert.NotNil(t, provider.enabledModules)
	assert.True(t, provider.enabledModules[ModulePipeline])
	assert.True(t, provider.enabledModules[ModuleProject])
	assert.True(t, provider.enabledModules[ModuleConnector])
}

func TestHarnessProvider_GetSupportedVersions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	versions := provider.GetSupportedVersions()
	assert.Contains(t, versions, "v1")
	assert.Contains(t, versions, "v2")
	assert.Contains(t, versions, "ng")
}

func TestHarnessProvider_GetDefaultConfiguration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	config := provider.GetDefaultConfiguration()

	assert.Equal(t, "https://app.harness.io", config.BaseURL)
	assert.Equal(t, "api_key", config.AuthType)
	assert.NotNil(t, config.DefaultHeaders)
	assert.Equal(t, "application/json", config.DefaultHeaders["Content-Type"])
	assert.Equal(t, "application/json", config.DefaultHeaders["Accept"])
	assert.Equal(t, 100, config.RateLimits.RequestsPerMinute)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.NotNil(t, config.RetryPolicy)
	assert.Equal(t, 3, config.RetryPolicy.MaxRetries)
	assert.True(t, config.RetryPolicy.RetryOnRateLimit)
	assert.True(t, config.RetryPolicy.RetryOnTimeout)
}

func TestHarnessProvider_GetToolDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	// Test with all modules enabled (default)
	tools := provider.GetToolDefinitions()
	assert.NotEmpty(t, tools)

	// Check that pipeline tools are included
	hasPipelineTool := false
	for _, tool := range tools {
		if tool.Category == "ci_cd" && tool.Name == "harness_pipelines" {
			hasPipelineTool = true
			assert.NotEmpty(t, tool.Name)
			assert.NotEmpty(t, tool.Description)
			assert.NotEmpty(t, tool.Operation.ID)
			break
		}
	}
	assert.True(t, hasPipelineTool, "Should have pipeline tools when module is enabled")

	// Test with modules disabled
	provider.SetEnabledModules([]HarnessModule{})
	tools = provider.GetToolDefinitions()

	// Check that pipeline tools are excluded
	hasPipelineTool = false
	for _, tool := range tools {
		if tool.Category == "ci_cd" && tool.Name == "harness_pipelines" {
			hasPipelineTool = true
			break
		}
	}
	assert.False(t, hasPipelineTool, "Should not have pipeline tools when module is disabled")
}

func TestHarnessProvider_ValidateCredentials(t *testing.T) {
	tests := []struct {
		name           string
		credentials    map[string]string
		serverResponse int
		serverBody     string
		expectError    bool
		errorContains  string
	}{
		{
			name: "valid credentials",
			credentials: map[string]string{
				"api_key":    "pat.account123.key456",
				"account_id": "account123",
			},
			serverResponse: http.StatusOK,
			serverBody: `{
				"data": {
					"email": "user@example.com",
					"name": "Test User",
					"uuid": "user-uuid-123"
				}
			}`,
			expectError: false,
		},
		{
			name: "missing api key",
			credentials: map[string]string{
				"account_id": "account123",
			},
			expectError:   true,
			errorContains: "missing required credentials",
		},
		{
			name: "invalid api key format",
			credentials: map[string]string{
				"api_key": "invalid-key",
			},
			expectError:   true,
			errorContains: "invalid Harness API key format",
		},
		{
			name: "unauthorized",
			credentials: map[string]string{
				"api_key": "pat.account123.key456",
			},
			serverResponse: http.StatusUnauthorized,
			serverBody:     `{"message": "Unauthorized"}`,
			expectError:    true,
			errorContains:  "invalid Harness credentials",
		},
		{
			name: "server error",
			credentials: map[string]string{
				"api_key": "pat.account123.key456",
			},
			serverResponse: http.StatusInternalServerError,
			serverBody:     `{"message": "Internal Server Error"}`,
			expectError:    true,
			errorContains:  "request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverResponse > 0 {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify request headers - look for any auth token
					authToken := tt.credentials["api_key"]
					if authToken == "" {
						authToken = tt.credentials["token"]
					}
					if authToken == "" {
						authToken = tt.credentials["personal_access_token"]
					}
					assert.Equal(t, authToken, r.Header.Get("x-api-key"))
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					// Check URL
					assert.Contains(t, r.URL.Path, "/api/user/currentUser")

					w.WriteHeader(tt.serverResponse)
					if _, err := w.Write([]byte(tt.serverBody)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
				defer server.Close()
			}

			logger := &observability.NoopLogger{}

			provider := NewHarnessProvider(logger)
			if server != nil {
				// Override base URL for testing but preserve other config
				config := provider.GetDefaultConfiguration()
				config.BaseURL = server.URL
				provider.SetConfiguration(config)
			}

			ctx := context.Background()
			err := provider.ValidateCredentials(ctx, tt.credentials)

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

func TestHarnessProvider_ExecuteOperation(t *testing.T) {
	tests := []struct {
		name            string
		operation       string
		params          map[string]interface{}
		serverResponse  int
		serverBody      string
		expectError     bool
		validateRequest func(*testing.T, *http.Request)
	}{
		{
			name:      "list pipelines",
			operation: "pipelines/list",
			params: map[string]interface{}{
				"org":     "my-org",
				"project": "my-project",
			},
			serverResponse: http.StatusOK,
			serverBody: `{
				"pipelines": [
					{"identifier": "pipeline1", "name": "Pipeline 1"},
					{"identifier": "pipeline2", "name": "Pipeline 2"}
				]
			}`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/v1/orgs/my-org/projects/my-project/pipelines", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
			},
		},
		{
			name:      "create pipeline",
			operation: "pipelines/create",
			params: map[string]interface{}{
				"org":     "my-org",
				"project": "my-project",
				"name":    "New Pipeline",
				"yaml":    "pipeline: yaml content",
			},
			serverResponse: http.StatusCreated,
			serverBody:     `{"identifier": "new-pipeline", "name": "New Pipeline"}`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/v1/orgs/my-org/projects/my-project/pipelines", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
			},
		},
		{
			name:      "execute pipeline",
			operation: "pipelines/execute",
			params: map[string]interface{}{
				"identifier":        "my-pipeline",
				"accountIdentifier": "account123",
				"orgIdentifier":     "my-org",
				"projectIdentifier": "my-project",
			},
			serverResponse: http.StatusOK,
			serverBody:     `{"planExecutionId": "exec-123", "status": "Running"}`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/pipeline/api/pipeline/execute/my-pipeline", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
			},
		},
		{
			name:        "unknown operation",
			operation:   "unknown/operation",
			params:      map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverResponse > 0 {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify authentication header
					assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

					if tt.validateRequest != nil {
						tt.validateRequest(t, r)
					}
					w.WriteHeader(tt.serverResponse)
					if _, err := w.Write([]byte(tt.serverBody)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
				defer server.Close()
			}

			logger := &observability.NoopLogger{}

			provider := NewHarnessProvider(logger)
			if server != nil {
				// Override base URL for testing
				config := provider.GetDefaultConfiguration()
				config.BaseURL = server.URL
				provider.SetConfiguration(config)
			}

			// Create context with credentials for ExecuteOperation
			pctx := &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					APIKey: "test-api-key",
				},
			}
			ctx := providers.WithContext(context.Background(), pctx)
			result, err := provider.ExecuteOperation(ctx, tt.operation, tt.params)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestHarnessProvider_GetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	mappings := provider.GetOperationMappings()

	assert.NotEmpty(t, mappings)

	// Test pipeline mappings
	assert.Contains(t, mappings, "pipelines/list")
	assert.Contains(t, mappings, "pipelines/get")
	assert.Contains(t, mappings, "pipelines/create")
	assert.Contains(t, mappings, "pipelines/update")
	assert.Contains(t, mappings, "pipelines/delete")
	assert.Contains(t, mappings, "pipelines/execute")

	// Verify mapping structure
	pipelineList := mappings["pipelines/list"]
	assert.Equal(t, "listPipelines", pipelineList.OperationID)
	assert.Equal(t, "GET", pipelineList.Method)
	assert.Contains(t, pipelineList.PathTemplate, "/pipelines")
	assert.Contains(t, pipelineList.RequiredParams, "org")
	assert.Contains(t, pipelineList.RequiredParams, "project")
}

func TestHarnessProvider_GetOpenAPISpec(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("with embedded spec", func(t *testing.T) {
		provider := NewHarnessProvider(logger)

		spec, err := provider.GetOpenAPISpec()
		assert.NoError(t, err)
		assert.NotNil(t, spec)
	})
}

func TestHarnessProvider_GetEmbeddedSpecVersion(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	version := provider.GetEmbeddedSpecVersion()
	assert.NotEmpty(t, version)
	assert.Contains(t, version, "2024") // Assuming the spec is from 2024
}

func TestHarnessProvider_HealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "healthy",
			serverResponse: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "unauthorized",
			serverResponse: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "server error",
			serverResponse: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewHarnessProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			ctx := context.Background()
			err := provider.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHarnessProvider_SetEnabledModules(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	// Initially all modules are enabled
	assert.True(t, provider.enabledModules[ModulePipeline])
	assert.True(t, provider.enabledModules[ModuleProject])

	// Disable all modules then enable only specific ones
	provider.SetEnabledModules([]HarnessModule{ModuleProject, ModuleGitOps})

	assert.False(t, provider.enabledModules[ModulePipeline])
	assert.True(t, provider.enabledModules[ModuleProject])
	assert.False(t, provider.enabledModules[ModuleConnector])
	assert.True(t, provider.enabledModules[ModuleGitOps])
}

func TestHarnessProvider_GetEnabledModules(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	modules := provider.GetEnabledModules()
	assert.NotNil(t, modules)
	assert.Contains(t, modules, ModulePipeline)
	assert.Contains(t, modules, ModuleProject)
}

func TestHarnessProvider_Close(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	err := provider.Close()
	assert.NoError(t, err)
}

func TestHarnessProvider_normalizeOperationName(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	tests := []struct {
		input    string
		expected string
	}{
		{"pipelines-list", "pipelines/list"},
		{"pipelines_list", "pipelines/list"},
		{"pipelines/list", "pipelines/list"},
		{"PIPELINES/LIST", "PIPELINES/LIST"},
		{"pipelines-get-by-id", "pipelines/get/by/id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests
func BenchmarkHarnessProvider_GetToolDefinitions(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetToolDefinitions()
	}
}

func BenchmarkHarnessProvider_GetOperationMappings(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetOperationMappings()
	}
}
