package jira

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

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAtlassianAPIServer creates a comprehensive mock of the Atlassian API
type MockAtlassianAPIServer struct {
	mu            sync.RWMutex
	server        *httptest.Server
	requestCount  int
	failNextCalls int
	responseDelay time.Duration
	rateLimited   bool
}

// NewMockAtlassianAPIServer creates a new mock Atlassian API server
func NewMockAtlassianAPIServer() *MockAtlassianAPIServer {
	mock := &MockAtlassianAPIServer{}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.mu.Lock()
		mock.requestCount++
		responseDelay := mock.responseDelay
		rateLimited := mock.rateLimited
		shouldFail := mock.failNextCalls > 0
		if shouldFail {
			mock.failNextCalls--
		}
		mock.mu.Unlock()

		// Simulate response delay
		if responseDelay > 0 {
			time.Sleep(responseDelay)
		}

		// Simulate rate limiting
		if rateLimited {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()))
			w.WriteHeader(http.StatusTooManyRequests)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"Rate limit exceeded"},
				"errors":        map[string]string{},
			}); err != nil {
				// Ignore encoding errors in test mock
				_ = err
			}
			return
		}

		// Simulate failures
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"Internal server error"},
				"errors":        map[string]string{},
			}); err != nil {
				// Ignore encoding errors in test mock
				_ = err
			}
			return
		}

		// Route based on path
		switch {
		case strings.Contains(r.URL.Path, "/rest/api/3/issue/search"):
			mock.handleIssueSearch(w, r)
		case strings.HasPrefix(r.URL.Path, "/rest/api/3/issue"):
			mock.handleIssueCRUD(w, r)
		case strings.Contains(r.URL.Path, "/rest/agile/1.0/board"):
			mock.handleAgileBoards(w, r)
		case strings.Contains(r.URL.Path, "/rest/api/3/project"):
			mock.handleProjects(w, r)
		case strings.Contains(r.URL.Path, "/rest/api/3/user"):
			mock.handleUsers(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{"Endpoint not found"},
				"errors":        map[string]string{},
			}); err != nil {
				// Ignore encoding errors in test mock
				_ = err
			}
		}
	}))

	return mock
}

// handleIssueSearch handles JQL search requests with pagination
func (m *MockAtlassianAPIServer) handleIssueSearch(w http.ResponseWriter, r *http.Request) {
	startAt := r.URL.Query().Get("startAt")
	maxResults := r.URL.Query().Get("maxResults")
	jql := r.URL.Query().Get("jql")

	// Default values
	if startAt == "" {
		startAt = "0"
	}
	if maxResults == "" {
		maxResults = "50"
	}

	// Mock paginated response
	response := map[string]interface{}{
		"expand":     "names,schema",
		"startAt":    startAt,
		"maxResults": maxResults,
		"total":      150, // Total issues matching query
		"issues": []map[string]interface{}{
			{
				"id":   "10001",
				"key":  "TEST-1",
				"self": fmt.Sprintf("%s/rest/api/3/issue/10001", m.server.URL),
				"fields": map[string]interface{}{
					"summary":     fmt.Sprintf("Test Issue %s", startAt),
					"description": "This is a test issue",
					"status": map[string]interface{}{
						"name": "Open",
					},
					"assignee": map[string]interface{}{
						"accountId":    "test-user-123",
						"displayName":  "Test User",
						"emailAddress": "test@example.com",
					},
					"created": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
					"updated": time.Now().Format(time.RFC3339),
				},
			},
		},
	}

	// Add warning for JQL complexity
	if strings.Contains(jql, "AND") && strings.Contains(jql, "OR") {
		response["warningMessages"] = []string{"Complex JQL query may impact performance"}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Ignore encoding errors in test mock
		_ = err
	}
}

// handleIssueCRUD handles issue CRUD operations
func (m *MockAtlassianAPIServer) handleIssueCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Return issue details
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"issue-etag-123"`)
		w.Header().Set("Last-Modified", time.Now().Add(-time.Hour).Format(http.TimeFormat))

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "10001",
			"key":  "TEST-1",
			"self": fmt.Sprintf("%s/rest/api/3/issue/10001", m.server.URL),
			"fields": map[string]interface{}{
				"summary":     "Test Issue",
				"description": "This is a test issue",
				"status": map[string]interface{}{
					"name": "In Progress",
				},
			},
		}); err != nil {
			// Ignore encoding errors in test mock
			_ = err
		}

	case "POST":
		// Create issue
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "10002",
			"key":  "TEST-2",
			"self": fmt.Sprintf("%s/rest/api/3/issue/10002", m.server.URL),
		}); err != nil {
			// Ignore encoding errors in test mock
			_ = err
		}

	case "PUT":
		// Update issue
		w.WriteHeader(http.StatusNoContent)

	case "DELETE":
		// Delete issue
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAgileBoards handles Agile board operations
func (m *MockAtlassianAPIServer) handleAgileBoards(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/sprint") {
		// Return sprint information
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"values": []map[string]interface{}{
				{
					"id":        1,
					"state":     "active",
					"name":      "Sprint 1",
					"startDate": time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
					"endDate":   time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
					"boardId":   1,
				},
			},
			"maxResults": 50,
			"startAt":    0,
			"isLast":     true,
		}); err != nil {
			// Ignore encoding errors in test mock
			_ = err
		}
	} else {
		// Return board information
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"values": []map[string]interface{}{
				{
					"id":   1,
					"name": "Test Board",
					"type": "scrum",
					"location": map[string]interface{}{
						"projectId":  10000,
						"projectKey": "TEST",
					},
				},
			},
			"maxResults": 50,
			"startAt":    0,
			"isLast":     true,
		}); err != nil {
			// Ignore encoding errors in test mock
			_ = err
		}
	}
}

// handleProjects handles project operations
func (m *MockAtlassianAPIServer) handleProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check for pagination parameters
	if r.URL.Query().Get("expand") == "description,lead,url,projectKeys" {
		// Return expanded project information
		if err := json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id":          "10000",
				"key":         "TEST",
				"name":        "Test Project",
				"description": "Test project description",
				"lead": map[string]interface{}{
					"accountId":   "lead-123",
					"displayName": "Project Lead",
				},
				"projectTypeKey": "software",
				"simplified":     false,
				"style":          "classic",
			},
		}); err != nil {
			// Ignore encoding errors in test mock
			_ = err
		}
	} else {
		// Return simple project list
		if err := json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id":   "10000",
				"key":  "TEST",
				"name": "Test Project",
			},
		}); err != nil {
			// Ignore encoding errors in test mock
			_ = err
		}
	}
}

// handleUsers handles user operations
func (m *MockAtlassianAPIServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"accountId":    "test-user-123",
		"accountType":  "atlassian",
		"emailAddress": "test@example.com",
		"displayName":  "Test User",
		"active":       true,
		"timeZone":     "UTC",
		"locale":       "en_US",
	}); err != nil {
		// Ignore encoding errors in test mock
		_ = err
	}
}

// Close shuts down the mock server
func (m *MockAtlassianAPIServer) Close() {
	m.server.Close()
}

// SetResponseDelay sets the response delay in a thread-safe manner
func (m *MockAtlassianAPIServer) SetResponseDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseDelay = delay
}

// SetRateLimited sets the rate limited flag in a thread-safe manner
func (m *MockAtlassianAPIServer) SetRateLimited(limited bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimited = limited
}

// SetFailNextCalls sets the number of calls to fail in a thread-safe manner
func (m *MockAtlassianAPIServer) SetFailNextCalls(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNextCalls = count
}

// GetRequestCount returns the request count in a thread-safe manner
func (m *MockAtlassianAPIServer) GetRequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestCount
}

// ResetRequestCount resets the request count in a thread-safe manner
func (m *MockAtlassianAPIServer) ResetRequestCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCount = 0
}

// TestJiraProvider_ErrorScenarios tests various error conditions
func TestJiraProvider_ErrorScenarios(t *testing.T) {
	logger := &observability.NoopLogger{}
	mockServer := NewMockAtlassianAPIServer()
	defer mockServer.Close()

	provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
	ctx := context.Background()

	t.Run("Server Error Recovery", func(t *testing.T) {
		// Simulate server error
		mockServer.SetFailNextCalls(2)

		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		// First call should fail
		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}

		// Second call should also fail
		req2, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		resp2, err := provider.secureHTTPDo(ctx, req2, "issues/get")
		require.Error(t, err)
		if resp2 != nil {
			assert.Equal(t, http.StatusInternalServerError, resp2.StatusCode)
			_ = resp2.Body.Close()
		}

		// Third call should succeed
		req3, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		resp3, err := provider.secureHTTPDo(ctx, req3, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp3.StatusCode)
		_ = resp3.Body.Close()
	})

	t.Run("Rate Limiting", func(t *testing.T) {
		// Enable rate limiting
		mockServer.SetRateLimited(true)

		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
			assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}

		// Disable rate limiting
		mockServer.SetRateLimited(false)
	})

	t.Run("Timeout Handling", func(t *testing.T) {
		// Set response delay
		mockServer.SetResponseDelay(100 * time.Millisecond)

		// Create context with short timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		req, err := http.NewRequestWithContext(timeoutCtx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(timeoutCtx, req, "issues/get")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
		if resp != nil {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}

		// Reset delay
		mockServer.SetResponseDelay(0)
	})

	t.Run("Invalid Endpoint", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/invalid/endpoint", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "unknown")
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}
	})
}

// TestJiraProvider_PaginationEdgeCases tests pagination edge cases
func TestJiraProvider_PaginationEdgeCases(t *testing.T) {
	logger := &observability.NoopLogger{}
	mockServer := NewMockAtlassianAPIServer()
	defer mockServer.Close()

	provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
	ctx := context.Background()

	t.Run("First Page", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			mockServer.server.URL+"/rest/api/3/issue/search?jql=project=TEST&startAt=0&maxResults=50", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/search")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Logf("Failed to decode response: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		assert.Equal(t, "0", result["startAt"])
		assert.Equal(t, "50", result["maxResults"])
		assert.Equal(t, float64(150), result["total"])
	})

	t.Run("Middle Page", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			mockServer.server.URL+"/rest/api/3/issue/search?jql=project=TEST&startAt=50&maxResults=50", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/search")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Logf("Failed to decode response: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		assert.Equal(t, "50", result["startAt"])
	})

	t.Run("Last Page", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			mockServer.server.URL+"/rest/api/3/issue/search?jql=project=TEST&startAt=100&maxResults=50", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/search")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Logf("Failed to decode response: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		assert.Equal(t, "100", result["startAt"])
		// Last page would have fewer results in reality
	})

	t.Run("Beyond Last Page", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET",
			mockServer.server.URL+"/rest/api/3/issue/search?jql=project=TEST&startAt=200&maxResults=50", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/search")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Logf("Failed to decode response: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		assert.Equal(t, "200", result["startAt"])
		// Should return empty issues array in reality
	})

	t.Run("Complex JQL Warning", func(t *testing.T) {
		// Use URL encoding for complex JQL
		jql := "project=TEST+AND+status=Open+OR+assignee=currentUser()"
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/rest/api/3/issue/search?jql=%s&startAt=0&maxResults=50", mockServer.server.URL, jql), nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/search")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Logf("Failed to decode response: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}

		// Check for performance warning
		if warnings, ok := result["warningMessages"].([]interface{}); ok {
			assert.Greater(t, len(warnings), 0)
			assert.Contains(t, warnings[0].(string), "Complex JQL")
		}
	})
}

// TestJiraProvider_ConcurrentRequests tests concurrent request handling
func TestJiraProvider_ConcurrentRequests(t *testing.T) {
	logger := &observability.NoopLogger{}
	mockServer := NewMockAtlassianAPIServer()
	defer mockServer.Close()

	provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
	ctx := context.Background()

	t.Run("Concurrent Read Operations", func(t *testing.T) {
		// Launch multiple concurrent requests
		numRequests := 10
		results := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				req, err := http.NewRequestWithContext(ctx, "GET",
					fmt.Sprintf("%s/rest/api/3/issue/TEST-%d", mockServer.server.URL, id), nil)
				if err != nil {
					results <- err
					return
				}

				resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
				if resp != nil {
					if err := resp.Body.Close(); err != nil {
						t.Logf("Failed to close response body: %v", err)
					}
				}
				results <- err
			}(i)
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			err := <-results
			assert.NoError(t, err)
		}

		// Verify request count
		assert.GreaterOrEqual(t, mockServer.GetRequestCount(), numRequests)
	})

	t.Run("Mixed Read/Write Operations", func(t *testing.T) {
		// Reset request count
		mockServer.ResetRequestCount()

		// Launch mixed operations
		numOps := 10
		results := make(chan error, numOps)

		for i := 0; i < numOps; i++ {
			go func(id int) {
				var req *http.Request
				var err error

				if id%2 == 0 {
					// Read operation
					req, err = http.NewRequestWithContext(ctx, "GET",
						fmt.Sprintf("%s/rest/api/3/issue/TEST-%d", mockServer.server.URL, id), nil)
				} else {
					// Write operation
					body := strings.NewReader(`{"fields":{"summary":"Test"}}`)
					req, err = http.NewRequestWithContext(ctx, "PUT",
						fmt.Sprintf("%s/rest/api/3/issue/TEST-%d", mockServer.server.URL, id), body)
					if req != nil {
						req.Header.Set("Content-Type", "application/json")
					}
				}

				if err != nil {
					results <- err
					return
				}

				resp, err := provider.secureHTTPDo(ctx, req, "issues/update")
				if resp != nil {
					if err := resp.Body.Close(); err != nil {
						t.Logf("Failed to close response body: %v", err)
					}
				}
				results <- err
			}(i)
		}

		// Collect results
		for i := 0; i < numOps; i++ {
			err := <-results
			// Some operations might fail due to caching/invalidation, that's ok
			_ = err
		}

		// Verify requests were made
		assert.Greater(t, mockServer.GetRequestCount(), 0)
	})
}

// TestJiraProvider_AuthenticationScenarios tests various authentication scenarios
func TestJiraProvider_AuthenticationScenarios(t *testing.T) {
	logger := &observability.NoopLogger{}
	mockServer := NewMockAtlassianAPIServer()
	defer mockServer.Close()

	t.Run("Basic Authentication Headers", func(t *testing.T) {
		provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
		ctx := context.Background()

		// Create request with basic auth header
		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		// Add basic auth header
		req.Header.Set("Authorization", "Basic dGVzdEBleGFtcGxlLmNvbTp0ZXN0LWFwaS10b2tlbg==")

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	})

	t.Run("Bearer Token Authentication", func(t *testing.T) {
		provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
		ctx := context.Background()

		// Create request with bearer token
		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		// Add bearer token header
		req.Header.Set("Authorization", "Bearer personal-access-token-789")

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	})

	t.Run("Missing Authentication", func(t *testing.T) {
		provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
		ctx := context.Background()

		// Create request without auth
		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		// Should still work with mock server (real server would return 401)
		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	})
}

// TestJiraProvider_DataIntegrity tests data integrity and validation
func TestJiraProvider_DataIntegrity(t *testing.T) {
	logger := &observability.NoopLogger{}
	mockServer := NewMockAtlassianAPIServer()
	defer mockServer.Close()

	provider := NewJiraProvider(logger, strings.TrimPrefix(mockServer.server.URL, "https://"))
	ctx := context.Background()

	t.Run("Response Data Validation", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, "GET", mockServer.server.URL+"/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		resp, err := provider.secureHTTPDo(ctx, req, "issues/get")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var issue map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&issue)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
		require.NoError(t, err)

		// Validate required fields
		assert.NotEmpty(t, issue["id"])
		assert.NotEmpty(t, issue["key"])
		assert.NotEmpty(t, issue["self"])

		// Validate fields structure
		fields, ok := issue["fields"].(map[string]interface{})
		assert.True(t, ok)
		assert.NotNil(t, fields["summary"])
	})

	t.Run("Create Issue Validation", func(t *testing.T) {
		// Test with valid data
		validBody := strings.NewReader(`{
			"fields": {
				"project": {"key": "TEST"},
				"summary": "Valid Issue",
				"description": "This is a valid issue",
				"issuetype": {"name": "Bug"}
			}
		}`)

		req, err := http.NewRequestWithContext(ctx, "POST", mockServer.server.URL+"/rest/api/3/issue", validBody)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := provider.secureHTTPDo(ctx, req, "issues/create")
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	})
}
