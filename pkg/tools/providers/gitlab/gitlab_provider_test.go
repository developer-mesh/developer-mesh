package gitlab

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

func TestNewGitLabProvider(t *testing.T) {
	logger := &observability.NoopLogger{}

	provider := NewGitLabProvider(logger)

	assert.NotNil(t, provider)
	assert.Equal(t, "gitlab", provider.GetProviderName())
	assert.NotNil(t, provider.BaseProvider)
	assert.NotNil(t, provider.httpClient)
	assert.NotNil(t, provider.enabledModules)
	assert.True(t, provider.enabledModules[ModuleProjects])
	assert.True(t, provider.enabledModules[ModuleIssues])
	assert.True(t, provider.enabledModules[ModuleMergeRequests])
	assert.True(t, provider.enabledModules[ModulePipelines])
}

func TestGitLabProvider_GetSupportedVersions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	versions := provider.GetSupportedVersions()
	assert.Contains(t, versions, "v4")
}

func TestGitLabProvider_GetDefaultConfiguration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	config := provider.GetDefaultConfiguration()

	assert.Equal(t, "https://gitlab.com/api/v4", config.BaseURL)
	assert.Equal(t, "bearer", config.AuthType)
	assert.NotNil(t, config.DefaultHeaders)
	assert.Equal(t, "application/json", config.DefaultHeaders["Content-Type"])
	assert.Equal(t, "application/json", config.DefaultHeaders["Accept"])
	assert.Equal(t, 600, config.RateLimits.RequestsPerMinute)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.NotNil(t, config.RetryPolicy)
	assert.Equal(t, 3, config.RetryPolicy.MaxRetries)
	assert.True(t, config.RetryPolicy.RetryOnRateLimit)
	assert.True(t, config.RetryPolicy.RetryOnTimeout)
}

func TestGitLabProvider_GetToolDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	// Test with all modules enabled (default)
	tools := provider.GetToolDefinitions()
	assert.NotEmpty(t, tools)

	// Check that project tools are included
	hasProjectTool := false
	for _, tool := range tools {
		if tool.Name == "gitlab_projects" {
			hasProjectTool = true
			assert.NotEmpty(t, tool.Name)
			assert.NotEmpty(t, tool.Description)
			assert.NotEmpty(t, tool.Operation.ID)
			break
		}
	}
	assert.True(t, hasProjectTool, "Should have project tools when module is enabled")

	// Test with modules disabled
	provider.SetEnabledModules([]GitLabModule{})
	tools = provider.GetToolDefinitions()

	// Check that project tools are excluded
	hasProjectTool = false
	for _, tool := range tools {
		if tool.Name == "gitlab_projects" {
			hasProjectTool = true
			break
		}
	}
	assert.False(t, hasProjectTool, "Should not have project tools when module is disabled")
}

func TestGitLabProvider_ValidateCredentials(t *testing.T) {
	tests := []struct {
		name           string
		credentials    map[string]string
		serverResponse int
		serverBody     string
		expectError    bool
		errorContains  string
	}{
		{
			name: "valid personal access token",
			credentials: map[string]string{
				"personal_access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
			},
			serverResponse: http.StatusOK,
			serverBody: `{
				"id": 1,
				"username": "testuser",
				"email": "test@example.com",
				"name": "Test User",
				"state": "active"
			}`,
			expectError: false,
		},
		{
			name: "valid private token (legacy) - not supported",
			credentials: map[string]string{
				"private_token": "xxxxxxxxxxxxxxxxxxxx",
			},
			expectError:   true,
			errorContains: "missing required credentials",
		},
		{
			name: "valid job token",
			credentials: map[string]string{
				"job_token": "xxxxxxxxxxxxxxxxxxxx",
			},
			serverResponse: http.StatusOK,
			serverBody: `{
				"id": 1,
				"username": "testuser",
				"email": "test@example.com",
				"name": "Test User",
				"state": "active"
			}`,
			expectError: false,
		},
		{
			name:          "missing credentials",
			credentials:   map[string]string{},
			expectError:   true,
			errorContains: "missing required credentials",
		},
		{
			name: "invalid token format",
			credentials: map[string]string{
				"personal_access_token": "invalid",
			},
			expectError:   true,
			errorContains: "invalid GitLab credentials",
		},
		{
			name: "unauthorized",
			credentials: map[string]string{
				"personal_access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
			},
			serverResponse: http.StatusUnauthorized,
			serverBody:     `{"message": "401 Unauthorized"}`,
			expectError:    true,
			errorContains:  "invalid GitLab credentials",
		},
		{
			name: "server error",
			credentials: map[string]string{
				"personal_access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
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
					// Verify request headers - check for proper auth header
					if pat := tt.credentials["personal_access_token"]; pat != "" {
						assert.Equal(t, "Bearer "+pat, r.Header.Get("Authorization"))
					} else if jobToken := tt.credentials["job_token"]; jobToken != "" {
						assert.Equal(t, "Bearer "+jobToken, r.Header.Get("Authorization"))
					}
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					// Check URL
					assert.Contains(t, r.URL.Path, "/user")

					w.WriteHeader(tt.serverResponse)
					if _, err := w.Write([]byte(tt.serverBody)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
				defer server.Close()
			}

			logger := &observability.NoopLogger{}

			provider := NewGitLabProvider(logger)
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

func TestGitLabProvider_ExecuteOperation(t *testing.T) {
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
			name:      "list projects",
			operation: "projects/list",
			params: map[string]interface{}{
				"per_page": 20,
				"page":     1,
			},
			serverResponse: http.StatusOK,
			serverBody: `[
				{"id": 1, "name": "Project 1", "path": "project1"},
				{"id": 2, "name": "Project 2", "path": "project2"}
			]`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/projects", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "20", r.URL.Query().Get("per_page"))
				assert.Equal(t, "1", r.URL.Query().Get("page"))
			},
		},
		{
			name:      "get project",
			operation: "projects/get",
			params: map[string]interface{}{
				"id": "123",
			},
			serverResponse: http.StatusOK,
			serverBody:     `{"id": 123, "name": "My Project", "path": "my-project"}`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/projects/123", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
			},
		},
		{
			name:      "create issue",
			operation: "issues/create",
			params: map[string]interface{}{
				"id":          "456",
				"title":       "Bug Report",
				"description": "Something is broken",
			},
			serverResponse: http.StatusCreated,
			serverBody:     `{"id": 789, "iid": 10, "title": "Bug Report"}`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/projects/456/issues", r.URL.Path)
				assert.Equal(t, "POST", r.Method)
			},
		},
		{
			name:      "list merge requests",
			operation: "merge_requests/list",
			params: map[string]interface{}{
				"id":    "789",
				"state": "opened",
			},
			serverResponse: http.StatusOK,
			serverBody: `[
				{"id": 1, "iid": 1, "title": "MR 1"},
				{"id": 2, "iid": 2, "title": "MR 2"}
			]`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/projects/789/merge_requests", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "opened", r.URL.Query().Get("state"))
			},
		},
		{
			name:      "trigger pipeline",
			operation: "pipelines/trigger",
			params: map[string]interface{}{
				"id":  "100",
				"ref": "main",
			},
			serverResponse: http.StatusCreated,
			serverBody:     `{"id": 999, "ref": "main", "status": "pending"}`,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/projects/100/pipeline", r.URL.Path)
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
					assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

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

			provider := NewGitLabProvider(logger)
			if server != nil {
				// Override base URL for testing
				config := provider.GetDefaultConfiguration()
				config.BaseURL = server.URL
				provider.SetConfiguration(config)
			}

			// Create context with credentials for ExecuteOperation
			pctx := &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
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

func TestGitLabProvider_GetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	mappings := provider.GetOperationMappings()

	assert.NotEmpty(t, mappings)

	// Test project mappings
	assert.Contains(t, mappings, "projects/list")
	assert.Contains(t, mappings, "projects/get")
	assert.Contains(t, mappings, "projects/create")

	// Test issue mappings
	assert.Contains(t, mappings, "issues/list")
	assert.Contains(t, mappings, "issues/get")
	assert.Contains(t, mappings, "issues/create")

	// Test merge request mappings
	assert.Contains(t, mappings, "merge_requests/list")
	assert.Contains(t, mappings, "merge_requests/get")
	assert.Contains(t, mappings, "merge_requests/create")

	// Verify mapping structure
	projectList := mappings["projects/list"]
	assert.Equal(t, "listProjects", projectList.OperationID)
	assert.Equal(t, "GET", projectList.Method)
	assert.Contains(t, projectList.PathTemplate, "/projects")
	assert.NotNil(t, projectList.OptionalParams)
}

func TestGitLabProvider_GetOpenAPISpec(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("with embedded spec", func(t *testing.T) {
		provider := NewGitLabProvider(logger)

		spec, err := provider.GetOpenAPISpec()
		assert.NoError(t, err)
		assert.NotNil(t, spec)
	})
}

func TestGitLabProvider_GetEmbeddedSpecVersion(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	version := provider.GetEmbeddedSpecVersion()
	assert.NotEmpty(t, version)
	assert.Contains(t, version, "2024") // Assuming the spec is from 2024
}

func TestGitLabProvider_HealthCheck(t *testing.T) {
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
				// Check that it's hitting the health endpoint
				assert.Contains(t, r.URL.Path, "/version")
				w.WriteHeader(tt.serverResponse)
				if tt.serverResponse == http.StatusOK {
					if _, err := w.Write([]byte(`{"version": "16.5.0", "revision": "abc123"}`)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewGitLabProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Add credentials to context for health check
			pctx := &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			}
			ctx := providers.WithContext(context.Background(), pctx)
			err := provider.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGitLabProvider_SetEnabledModules(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	// Initially all core modules are enabled
	assert.True(t, provider.enabledModules[ModuleProjects])
	assert.True(t, provider.enabledModules[ModuleIssues])
	assert.True(t, provider.enabledModules[ModuleMergeRequests])

	// Disable all modules then enable only specific ones
	provider.SetEnabledModules([]GitLabModule{ModuleProjects, ModuleWikis})

	assert.True(t, provider.enabledModules[ModuleProjects])
	assert.False(t, provider.enabledModules[ModuleIssues])
	assert.False(t, provider.enabledModules[ModuleMergeRequests])
	assert.True(t, provider.enabledModules[ModuleWikis])
	assert.False(t, provider.enabledModules[ModulePipelines])
}

func TestGitLabProvider_GetEnabledModules(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	modules := provider.GetEnabledModules()
	assert.NotNil(t, modules)
	assert.Contains(t, modules, ModuleProjects)
	assert.Contains(t, modules, ModuleIssues)
	assert.Contains(t, modules, ModuleMergeRequests)
}

func TestGitLabProvider_Close(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	err := provider.Close()
	assert.NoError(t, err)
}

func TestGitLabProvider_normalizeOperationName(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	tests := []struct {
		input    string
		expected string
	}{
		{"projects-list", "projects/list"},
		{"projects_list", "projects/list"},
		{"projects/list", "projects/list"},
		{"PROJECTS/LIST", "PROJECTS/LIST"},
		{"merge-requests-approve", "merge/requests/approve"},
		{"merge_requests_approve", "merge_requests/approve"}, // merge_requests is preserved as a single entity
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitLabProvider_GetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()
	assert.NotEmpty(t, definitions)

	// Check for key definitions
	hasProjectsDef := false
	hasIssuesDef := false
	hasMRsDef := false

	for _, def := range definitions {
		switch def.Name {
		case "gitlab_projects":
			hasProjectsDef = true
			assert.Equal(t, "Projects", def.Category)
			assert.NotEmpty(t, def.UsageExamples)
			assert.NotEmpty(t, def.SemanticTags)
		case "gitlab_issues":
			hasIssuesDef = true
			assert.Equal(t, "Issue Tracking", def.Category)
		case "gitlab_merge_requests":
			hasMRsDef = true
			assert.Equal(t, "Code Review", def.Category)
		}
	}

	assert.True(t, hasProjectsDef, "Should have projects definition")
	assert.True(t, hasIssuesDef, "Should have issues definition")
	assert.True(t, hasMRsDef, "Should have merge requests definition")
}

// TestGitLabProvider_FilterOperationsByPermissions tests permission-based filtering
// Note: This functionality may be implemented in the future
func TestGitLabProvider_FilterOperationsByPermissions(t *testing.T) {
	t.Skip("FilterOperationsByPermissions not yet implemented for GitLab provider")
}

// Benchmark tests
func BenchmarkGitLabProvider_GetToolDefinitions(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetToolDefinitions()
	}
}

func BenchmarkGitLabProvider_GetOperationMappings(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetOperationMappings()
	}
}

func BenchmarkGitLabProvider_GetAIOptimizedDefinitions(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetAIOptimizedDefinitions()
	}
}
