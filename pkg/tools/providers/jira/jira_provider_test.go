package jira

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJiraProvider(t *testing.T) {
	logger := &observability.NoopLogger{}

	tests := []struct {
		name            string
		domain          string
		expectedBaseURL string
	}{
		{
			name:            "domain without atlassian.net",
			domain:          "mycompany",
			expectedBaseURL: "https://mycompany.atlassian.net",
		},
		{
			name:            "domain with atlassian.net",
			domain:          "mycompany.atlassian.net",
			expectedBaseURL: "https://mycompany.atlassian.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewJiraProvider(logger, tt.domain)

			assert.NotNil(t, provider)
			assert.Equal(t, "jira", provider.GetProviderName())
			assert.Contains(t, provider.GetSupportedVersions(), "cloud")

			config := provider.GetDefaultConfiguration()
			assert.Equal(t, tt.expectedBaseURL, config.BaseURL)
			assert.Equal(t, "basic", config.AuthType)
		})
	}
}

func TestJiraProvider_GetToolDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	tools := provider.GetToolDefinitions()
	assert.NotEmpty(t, tools)

	// Check for expected tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["jira_issues"], "Should have jira_issues tool")
	assert.True(t, toolNames["jira_projects"], "Should have jira_projects tool")
	assert.True(t, toolNames["jira_boards"], "Should have jira_boards tool")
	assert.True(t, toolNames["jira_users"], "Should have jira_users tool")
}

func TestJiraProvider_GetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	mappings := provider.GetOperationMappings()
	assert.NotEmpty(t, mappings)

	// Check for key operations
	assert.Contains(t, mappings, "issues/search")
	assert.Contains(t, mappings, "issues/create")
	assert.Contains(t, mappings, "issues/get")
	assert.Contains(t, mappings, "issues/update")
	assert.Contains(t, mappings, "issues/delete")
	assert.Contains(t, mappings, "projects/list")
	assert.Contains(t, mappings, "boards/list")
	assert.Contains(t, mappings, "sprints/get")

	// Verify operation structure
	issueSearch := mappings["issues/search"]
	assert.Equal(t, "searchIssues", issueSearch.OperationID)
	assert.Equal(t, "GET", issueSearch.Method)
	assert.Equal(t, "/rest/api/3/search", issueSearch.PathTemplate)
	assert.Contains(t, issueSearch.OptionalParams, "jql")
}

func TestJiraProvider_ValidateCredentials(t *testing.T) {
	tests := []struct {
		name        string
		creds       map[string]string
		setupServer func(*httptest.Server)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid basic auth credentials",
			creds: map[string]string{
				"email":     "user@example.com",
				"api_token": "test-token",
			},
			setupServer: func(server *httptest.Server) {
				// Server setup is handled in the test body
			},
			expectError: false,
		},
		{
			name: "valid OAuth2 credentials",
			creds: map[string]string{
				"access_token": "oauth-token",
			},
			setupServer: func(server *httptest.Server) {
				// Server setup is handled in the test body
			},
			expectError: false,
		},
		{
			name:        "missing credentials",
			creds:       map[string]string{},
			expectError: true,
			errorMsg:    "no authentication credentials found",
		},
		{
			name: "invalid credentials",
			creds: map[string]string{
				"email":     "user@example.com",
				"api_token": "invalid",
			},
			setupServer: func(server *httptest.Server) {
				// Server will return 401
			},
			expectError: true,
			errorMsg:    "invalid Jira credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check the endpoint
				assert.Equal(t, "/rest/api/3/myself", r.URL.Path)

				// Check authorization header
				authHeader := r.Header.Get("Authorization")

				if tt.name == "invalid credentials" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				if tt.creds["access_token"] != "" {
					assert.Contains(t, authHeader, "Bearer")
				} else if tt.creds["email"] != "" && tt.creds["api_token"] != "" {
					assert.Contains(t, authHeader, "Basic")
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"accountId":"123","displayName":"Test User"}`))
			}))
			defer server.Close()

			// Create provider with test server URL
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, "test")

			// Override base URL to use test server
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Test credential validation
			ctx := context.Background()
			err := provider.ValidateCredentials(ctx, tt.creds)

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

func TestJiraProvider_NormalizeOperationName(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	tests := []struct {
		name      string
		operation string
		expected  string
	}{
		{
			name:      "simple search",
			operation: "search",
			expected:  "issues/search",
		},
		{
			name:      "simple create",
			operation: "create",
			expected:  "issues/create",
		},
		{
			name:      "already normalized",
			operation: "issues/create",
			expected:  "issues/create",
		},
		{
			name:      "hyphenated format",
			operation: "issues-create",
			expected:  "issues/create",
		},
		{
			name:      "underscore format",
			operation: "issues_create",
			expected:  "issues/create",
		},
		{
			name:      "transition action",
			operation: "transition",
			expected:  "issues/transition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.operation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJiraProvider_ResolveOperationFromContext(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	tests := []struct {
		name      string
		operation string
		params    map[string]interface{}
		expected  string
	}{
		{
			name:      "issue get with issueIdOrKey",
			operation: "get",
			params:    map[string]interface{}{"issueIdOrKey": "PROJ-123"},
			expected:  "issues/get",
		},
		{
			name:      "project get with projectIdOrKey",
			operation: "get",
			params:    map[string]interface{}{"projectIdOrKey": "PROJ"},
			expected:  "projects/get",
		},
		{
			name:      "board get with boardId",
			operation: "get",
			params:    map[string]interface{}{"boardId": "10"},
			expected:  "boards/get",
		},
		{
			name:      "sprint get with sprintId",
			operation: "get",
			params:    map[string]interface{}{"sprintId": "20"},
			expected:  "sprints/get",
		},
		{
			name:      "search with JQL",
			operation: "search",
			params:    map[string]interface{}{"jql": "project = PROJ"},
			expected:  "issues/search",
		},
		{
			name:      "no context match",
			operation: "unknown",
			params:    map[string]interface{}{},
			expected:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.resolveOperationFromContext(tt.operation, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJiraProvider_ExecuteOperation(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/search":
			// Handle JQL search
			jql := r.URL.Query().Get("jql")
			if jql != "" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"issues":[],"total":0}`))
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		case "/rest/api/3/issue":
			// Handle issue creation
			if r.Method == "POST" {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"10000","key":"PROJ-1"}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Override base URL
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Create context with credentials (Jira uses basic auth with email/API token)
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Username: "user@example.com",
			Password: "test-api-token",
		},
	})

	tests := []struct {
		name        string
		operation   string
		params      map[string]interface{}
		expectError bool
	}{
		{
			name:      "search with JQL",
			operation: "search",
			params: map[string]interface{}{
				"jql": "project = PROJ",
			},
			expectError: false,
		},
		{
			name:      "normalized search",
			operation: "issues/search",
			params: map[string]interface{}{
				"jql": "status = Open",
			},
			expectError: false,
		},
		{
			name:        "unknown operation",
			operation:   "invalid/operation",
			params:      map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestJiraProvider_GetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	definitions := provider.GetAIOptimizedDefinitions()
	assert.NotEmpty(t, definitions)

	// Should have help definition at minimum (since all toolsets are enabled by default)
	defNames := make(map[string]bool)
	for _, def := range definitions {
		defNames[def.Name] = true

		// Verify AI-specific fields
		assert.NotEmpty(t, def.Description)
		assert.NotEmpty(t, def.UsageExamples)
		assert.NotEmpty(t, def.SemanticTags)
		assert.NotEmpty(t, def.CommonPhrases)
	}

	// Should always have help definition
	assert.True(t, defNames["jira_help"], "Should have AI definition for help")

	// Should have toolset definitions if toolsets are enabled
	assert.True(t, defNames["jira_issues"], "Should have AI definition for issues")
	assert.True(t, defNames["jira_search"], "Should have AI definition for search")
	assert.True(t, defNames["jira_comments"], "Should have AI definition for comments")
	assert.True(t, defNames["jira_workflow"], "Should have AI definition for workflow")

	// Check detailed fields for issues definition
	for _, def := range definitions {
		if def.Name == "jira_issues" {
			assert.NotNil(t, def.Capabilities)
			assert.NotEmpty(t, def.Capabilities.Capabilities)
			assert.NotNil(t, def.Capabilities.RateLimits)
			assert.Equal(t, 60, def.Capabilities.RateLimits.RequestsPerMinute)
		}
	}
}

func TestJiraProvider_HealthCheck(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		expectError  bool
	}{
		{
			name:         "healthy server",
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "unhealthy server",
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/rest/api/3/serverInfo", r.URL.Path)
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					_, _ = w.Write([]byte(`{"version":"9.0.0","deploymentType":"Cloud"}`))
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, "test")

			// Override base URL
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
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

func TestJiraProvider_GetDefaultConfiguration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	config := provider.GetDefaultConfiguration()

	// Verify configuration
	assert.Equal(t, "basic", config.AuthType)
	assert.Equal(t, 60, config.RateLimits.RequestsPerMinute)
	assert.NotNil(t, config.RetryPolicy)
	assert.Equal(t, 3, config.RetryPolicy.MaxRetries)
	assert.True(t, config.RetryPolicy.RetryOnRateLimit)
	assert.NotEmpty(t, config.OperationGroups)

	// Check operation groups
	groupNames := make(map[string]bool)
	for _, group := range config.OperationGroups {
		groupNames[group.Name] = true
		assert.NotEmpty(t, group.Operations)
	}

	assert.True(t, groupNames["issues"])
	assert.True(t, groupNames["projects"])
	assert.True(t, groupNames["boards"])
	assert.True(t, groupNames["users"])
}

func TestJiraProvider_BasicAuth(t *testing.T) {
	username := "user@example.com"
	password := "api-token-123"

	result := basicAuth(username, password)

	// Should be base64 encoded
	assert.NotEmpty(t, result)

	// Decode and verify
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	assert.Equal(t, username+":"+password, string(decoded))
}

func TestJiraProvider_BuildURL(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "mycompany")

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "path without leading slash",
			path:     "rest/api/3/issue",
			expected: "https://mycompany.atlassian.net/rest/api/3/issue",
		},
		{
			name:     "path with leading slash",
			path:     "/rest/api/3/issue",
			expected: "https://mycompany.atlassian.net/rest/api/3/issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.buildURL(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Epic 4, Story 4.2 - Observability Features Tests

func TestJiraProvider_ObservabilityIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Verify observability manager was initialized
	assert.NotNil(t, provider.observabilityMgr)
	assert.False(t, provider.IsDebugMode()) // Should be false by default
}

func TestJiraProvider_HealthCheckWithObservability(t *testing.T) {
	// Create a test server that responds to serverInfo endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/serverInfo" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"version": "8.0.0", "versionNumbers": [8,0,0]}`)); err != nil {
				t.Logf("Failed to write response: %v", err)
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Set the provider's baseURL to use the test server
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "basic",
	})

	ctx := context.Background()

	// Test successful health check
	err := provider.HealthCheck(ctx)
	assert.NoError(t, err)

	// Verify health status is updated
	status := provider.GetHealthStatus()
	assert.True(t, status.Healthy)
	assert.NotZero(t, status.LastChecked)
	assert.Greater(t, status.ResponseTime, time.Duration(0))
}

func TestJiraProvider_HealthCheckFailure(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(`{"error": "Internal server error"}`)); err != nil {
			t.Logf("Failed to write error response: %v", err)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Set the provider's baseURL to use the test server
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "basic",
	})

	ctx := context.Background()

	// Test failed health check
	err := provider.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check returned status 500")

	// Verify health status reflects failure
	status := provider.GetHealthStatus()
	assert.False(t, status.Healthy)
	assert.NotEmpty(t, status.Errors)
}

func TestJiraProvider_SecureHTTPDoWithObservability(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"message": "success", "data": "test"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Create a test request
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Execute secure HTTP request
	resp, err := provider.secureHTTPDo(ctx, req, "test_operation")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Clean up response
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()
}

func TestJiraProvider_SecureHTTPDoErrorHandling(t *testing.T) {
	// Create a test server that returns different error types
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/401":
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error": "Unauthorized"}`)); err != nil {
				t.Logf("Failed to write error response: %v", err)
			}
		case "/429":
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"error": "Rate limit exceeded"}`)); err != nil {
				t.Logf("Failed to write error response: %v", err)
			}
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"error": "Server error"}`)); err != nil {
				t.Logf("Failed to write error response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "unauthorized error",
			path:           "/401",
			expectedStatus: 401,
			expectedError:  "HTTP 401",
		},
		{
			name:           "rate limit error",
			path:           "/429",
			expectedStatus: 429,
			expectedError:  "HTTP 429",
		},
		{
			name:           "server error",
			path:           "/500",
			expectedStatus: 500,
			expectedError:  "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", server.URL+tt.path, nil)
			require.NoError(t, err)

			ctx := context.Background()
			resp, err := provider.secureHTTPDo(ctx, req, "test_error_operation")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			// Verify that the error is properly categorized as a JiraError
			var jiraErr *JiraError
			if errors.As(err, &jiraErr) {
				assert.NotEmpty(t, jiraErr.Type)
				assert.Equal(t, "test_error_operation", jiraErr.Operation)
			}

			if resp != nil {
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)
				defer func() {
					if err := resp.Body.Close(); err != nil {
						t.Logf("Failed to close response body: %v", err)
					}
				}()
			}
		})
	}
}

func TestJiraProvider_DebugModeIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Initially debug mode should be disabled
	assert.False(t, provider.IsDebugMode())

	// Enable debug mode
	if provider.observabilityMgr != nil {
		provider.observabilityMgr.config.DebugMode = true
		provider.observabilityMgr.debugLogger = logger.WithPrefix("[JIRA-DEBUG]")
	}

	assert.True(t, provider.IsDebugMode())
}

func TestJiraProvider_ObservabilityMetrics(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Verify observability manager provides metrics
	if provider.observabilityMgr != nil {
		metrics := provider.observabilityMgr.GetObservabilityMetrics()

		assert.Contains(t, metrics, "debug_mode")
		assert.Contains(t, metrics, "metrics_enabled")
		assert.Contains(t, metrics, "error_tracking_enabled")
		assert.Contains(t, metrics, "health_check_timeout")
		assert.Contains(t, metrics, "current_health_status")
	}
}
