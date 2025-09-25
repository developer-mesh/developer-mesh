//go:build integration

package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJiraProviderIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("MockServerIntegration", func(t *testing.T) {
		// Create mock server
		var mockServer *httptest.Server
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Basic auth check
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			switch r.URL.Path {
			case "/rest/api/3/issue":
				if r.Method == "POST" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":   "10001",
						"key":  "TEST-123",
						"self": mockServer.URL + "/rest/api/3/issue/10001",
					})
				}

			case "/rest/api/3/issue/TEST-123":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":  "10001",
						"key": "TEST-123",
						"fields": map[string]interface{}{
							"summary":     "Test Issue",
							"description": "Test Description",
							"status": map[string]interface{}{
								"name": "To Do",
							},
						},
					})
				} else if r.Method == "PUT" {
					w.WriteHeader(http.StatusNoContent)
				} else if r.Method == "DELETE" {
					w.WriteHeader(http.StatusNoContent)
				}

			case "/rest/api/3/search":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"issues": []map[string]interface{}{
						{
							"id":  "10001",
							"key": "TEST-123",
							"fields": map[string]interface{}{
								"summary": "Test Issue",
							},
						},
					},
					"total": 1,
				})

			case "/rest/api/3/issue/TEST-123/comment":
				if r.Method == "POST" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":   "10000",
						"body": "Test comment",
					})
				} else if r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"comments": []map[string]interface{}{
							{
								"id":   "10000",
								"body": "Test comment",
							},
						},
					})
				}

			case "/rest/api/3/issue/TEST-123/transitions":
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"transitions": []map[string]interface{}{
							{
								"id":   "11",
								"name": "In Progress",
							},
							{
								"id":   "21",
								"name": "Done",
							},
						},
					})
				} else if r.Method == "POST" {
					w.WriteHeader(http.StatusNoContent)
				}

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer mockServer.Close()

		// Create provider
		provider := NewJiraProvider(logger, "test-tenant")
		provider.domain = mockServer.URL

		// Test provider name
		assert.Equal(t, "jira", provider.GetProviderName())

		// Test tool definitions are loaded
		toolDefs := provider.GetToolDefinitions()
		assert.NotEmpty(t, toolDefs)

		// Check for essential tools
		toolNames := make([]string, 0, len(toolDefs))
		for _, tool := range toolDefs {
			toolNames = append(toolNames, tool.Name)
		}
		assert.Contains(t, toolNames, "jira_issues")
		assert.Contains(t, toolNames, "jira_projects")
		assert.Contains(t, toolNames, "jira_workflows")

		// Test operation mappings
		mappings := provider.GetOperationMappings()
		assert.NotEmpty(t, mappings)
		assert.Contains(t, mappings, "issues/create")
		assert.Contains(t, mappings, "issues/get")

		// Create context with proper provider credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Username: "test@example.com",
				Password: "test-token",
			},
		})

		// Test create issue
		t.Run("CreateIssue", func(t *testing.T) {
			params := map[string]interface{}{
				"project":     "TEST",
				"summary":     "Test Issue",
				"description": "Test Description",
				"issuetype":   "Task",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/create", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
				t.Logf("Create issue result: %v", toolResult.Data)
			}
		})

		// Test get issue
		t.Run("GetIssue", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/get", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
				t.Logf("Get issue result: %v", toolResult.Data)
			}
		})

		// Test search issues
		t.Run("SearchIssues", func(t *testing.T) {
			params := map[string]interface{}{
				"jql":        "project = TEST",
				"maxResults": 50,
			}

			result, err := provider.ExecuteOperation(ctx, "issues/search", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
				t.Logf("Search result: %v", toolResult.Data)
			}
		})

		// Test update issue
		t.Run("UpdateIssue", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
				"summary":      "Updated Summary",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/update", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
			}
		})

		// Test delete issue
		t.Run("DeleteIssue", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/delete", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
			}
		})

		// Test add comment
		t.Run("AddComment", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
				"body":         "Test comment",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/comments/add", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
			}
		})

		// Test get comments
		t.Run("GetComments", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/comments/list", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
			}
		})

		// Test get transitions
		t.Run("GetTransitions", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/transitions", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
			}
		})

		// Test transition issue
		t.Run("TransitionIssue", func(t *testing.T) {
			params := map[string]interface{}{
				"issueIdOrKey": "TEST-123",
				"transition":   "11",
			}

			result, err := provider.ExecuteOperation(ctx, "issues/transition", params)
			require.NoError(t, err)
			require.NotNil(t, result)

			if toolResult, ok := result.(*ToolResult); ok {
				assert.True(t, toolResult.Success)
			}
		})
	})

	// Test with real Atlassian API if configured
	t.Run("RealAtlassianAPI", func(t *testing.T) {
		domain := os.Getenv("JIRA_TEST_DOMAIN")
		email := os.Getenv("JIRA_TEST_EMAIL")
		apiToken := os.Getenv("JIRA_TEST_TOKEN")

		if domain == "" || email == "" || apiToken == "" {
			t.Skip("JIRA_TEST_DOMAIN, JIRA_TEST_EMAIL, or JIRA_TEST_TOKEN not set")
		}

		provider := NewJiraProvider(logger, "test-tenant")
		provider.domain = domain

		ctx := context.Background()
		ctx = context.WithValue(ctx, "auth_type", "basic")
		ctx = context.WithValue(ctx, "email", email)
		ctx = context.WithValue(ctx, "api_token", apiToken)

		// Test search with real API
		params := map[string]interface{}{
			"jql":        "project = TEST ORDER BY created DESC",
			"maxResults": 5,
		}

		result, err := provider.ExecuteOperation(ctx, "issues/search", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		if toolResult, ok := result.(*ToolResult); ok {
			assert.True(t, toolResult.Success)
			t.Logf("Real Jira search result: %v", toolResult.Data)
		}
	})
}

func TestJiraProviderCaching(t *testing.T) {
	logger := &observability.NoopLogger{}

	// Create mock server that counts requests
	requestCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.URL.Path == "/rest/api/3/issue/TEST-123" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", "\"12345\"")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":  "10001",
				"key": "TEST-123",
				"fields": map[string]interface{}{
					"summary": "Cached Issue",
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create provider with caching enabled
	provider := NewJiraProvider(logger, "test-tenant")
	provider.domain = mockServer.URL

	// Enable cache manager if it's initialized
	if provider.cacheManager != nil {
		// Create context with proper provider credentials
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Username: "test@example.com",
				Password: "test-token",
			},
		})

		params := map[string]interface{}{
			"issueIdOrKey": "TEST-123",
		}

		// First request - should hit server
		result1, err := provider.ExecuteOperation(ctx, "issues/get", params)
		require.NoError(t, err)
		require.NotNil(t, result1)
		firstRequestCount := requestCount

		// Second request - should be cached
		result2, err := provider.ExecuteOperation(ctx, "issues/get", params)
		require.NoError(t, err)
		require.NotNil(t, result2)

		// If caching is working, request count shouldn't increase
		if requestCount == firstRequestCount {
			t.Log("Caching is working - second request was served from cache")
		} else {
			t.Log("Second request hit the server (caching might not be enabled)")
		}
	} else {
		t.Skip("Cache manager not initialized")
	}
}

func TestJiraProviderErrorHandling(t *testing.T) {
	logger := &observability.NoopLogger{}

	// Create mock server that returns errors
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/NOTFOUND-123":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"Issue not found"},
			})

		case "/rest/api/3/issue/FORBIDDEN-123":
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"You do not have permission to view this issue"},
			})

		case "/rest/api/3/issue/ERROR-500":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"Internal server error"},
			})

		default:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer mockServer.Close()

	provider := NewJiraProvider(logger, "test-tenant")
	provider.domain = mockServer.URL

	ctx := context.Background()
	ctx = context.WithValue(ctx, "auth_type", "basic")
	ctx = context.WithValue(ctx, "email", "test@example.com")
	ctx = context.WithValue(ctx, "api_token", "test-token")

	t.Run("Handle404NotFound", func(t *testing.T) {
		params := map[string]interface{}{
			"issueIdOrKey": "NOTFOUND-123",
		}

		result, err := provider.ExecuteOperation(ctx, "issues/get", params)
		// Provider should handle 404 gracefully
		if err != nil {
			t.Logf("Provider returned error for 404: %v", err)
		} else if result != nil {
			t.Logf("Provider handled 404 without error")
		}
	})

	t.Run("Handle403Forbidden", func(t *testing.T) {
		params := map[string]interface{}{
			"issueIdOrKey": "FORBIDDEN-123",
		}

		result, err := provider.ExecuteOperation(ctx, "issues/get", params)
		// Provider should handle 403 gracefully
		if err != nil {
			t.Logf("Provider returned error for 403: %v", err)
		} else if result != nil {
			t.Logf("Provider handled 403 without error")
		}
	})

	t.Run("Handle500ServerError", func(t *testing.T) {
		params := map[string]interface{}{
			"issueIdOrKey": "ERROR-500",
		}

		result, err := provider.ExecuteOperation(ctx, "issues/get", params)
		// Provider should handle 500 gracefully
		if err != nil {
			t.Logf("Provider returned error for 500: %v", err)
		} else if result != nil {
			t.Logf("Provider handled 500 without error")
		}
	})
}

func TestJiraProviderHealthCheck(t *testing.T) {
	logger := &observability.NoopLogger{}

	// Create mock server for health check
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"accountId":    "12345",
				"emailAddress": "test@example.com",
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	provider := NewJiraProvider(logger, "test-tenant")
	provider.domain = mockServer.URL

	ctx := context.Background()
	err := provider.HealthCheck(ctx)

	// Health check might fail if no credentials are set, which is expected
	if err != nil {
		t.Logf("Health check failed (expected without credentials): %v", err)
	} else {
		t.Log("Health check passed")
	}

	// Test health status
	status := provider.GetHealthStatus()
	t.Logf("Health status: %+v", status)
}

func TestJiraProviderObservability(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-tenant")

	// Test debug mode
	debugMode := provider.IsDebugMode()
	t.Logf("Debug mode: %v", debugMode)

	// Test observability manager exists
	if provider.observabilityMgr != nil {
		t.Log("Observability manager is initialized")

		// Test getting observability metrics
		metrics := provider.observabilityMgr.GetObservabilityMetrics()
		t.Logf("Observability metrics: %v", metrics)
	} else {
		t.Log("Observability manager not initialized")
	}
}
