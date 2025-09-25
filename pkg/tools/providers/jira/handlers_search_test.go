package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIssuesHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse string
		statusCode     int
		projectFilter  string
		expectError    bool
		errorContains  string
		validateURL    func(t *testing.T, u *url.URL)
		validateResult func(t *testing.T, result *ToolResult)
	}{
		{
			name: "Search with simple JQL",
			params: map[string]interface{}{
				"jql": "project = PROJ AND status = Open",
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 2,
				"issues": [
					{
						"id": "10000",
						"key": "PROJ-1",
						"fields": {
							"summary": "Issue 1",
							"project": {"key": "PROJ"}
						}
					},
					{
						"id": "10001",
						"key": "PROJ-2",
						"fields": {
							"summary": "Issue 2",
							"project": {"key": "PROJ"}
						}
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Contains(t, u.Query().Get("jql"), "project = PROJ")
				assert.Contains(t, u.Query().Get("jql"), "status = Open")
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				data := result.Data.(map[string]interface{})
				issues := data["issues"].([]interface{})
				assert.Equal(t, 2, len(issues))
			},
		},
		{
			name: "Search with pagination",
			params: map[string]interface{}{
				"jql":        "type = Bug",
				"startAt":    20.0,
				"maxResults": 10.0,
			},
			serverResponse: `{
				"startAt": 20,
				"maxResults": 10,
				"total": 50,
				"issues": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "20", u.Query().Get("startAt"))
				assert.Equal(t, "10", u.Query().Get("maxResults"))
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				data := result.Data.(map[string]interface{})
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, 20, metadata["startAt"])
				assert.Equal(t, 10, metadata["maxResults"])
				assert.Equal(t, 30, metadata["nextStartAt"])
				assert.True(t, metadata["hasMore"].(bool))
			},
		},
		{
			name: "Search with fields selection",
			params: map[string]interface{}{
				"fields": []interface{}{"summary", "status", "assignee"},
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 1,
				"issues": [
					{
						"id": "10000",
						"key": "TEST-1",
						"fields": {
							"summary": "Test Issue",
							"status": {"name": "Open"},
							"assignee": {"displayName": "John Doe"},
							"project": {"key": "TEST"}
						}
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "summary,status,assignee", u.Query().Get("fields"))
			},
		},
		{
			name: "Search with expand parameter",
			params: map[string]interface{}{
				"jql":    "id = TEST-1",
				"expand": "changelog,transitions",
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 1,
				"issues": [
					{
						"id": "10000",
						"key": "TEST-1",
						"fields": {"project": {"key": "TEST"}},
						"changelog": {},
						"transitions": []
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "changelog,transitions", u.Query().Get("expand"))
			},
		},
		{
			name:   "Default search (no JQL provided)",
			params: map[string]interface{}{},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 10,
				"issues": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "ORDER BY created DESC", u.Query().Get("jql"))
			},
		},
		{
			name: "JQL validation - SQL injection attempt",
			params: map[string]interface{}{
				"jql": "project = 'TEST'; DROP TABLE issues;",
			},
			expectError:   true,
			errorContains: "potentially dangerous pattern detected",
		},
		{
			name: "JQL validation - unbalanced parentheses",
			params: map[string]interface{}{
				"jql": "project = TEST AND (status = Open",
			},
			expectError:   true,
			errorContains: "unbalanced parentheses",
		},
		{
			name: "JQL validation - unbalanced quotes",
			params: map[string]interface{}{
				"jql": "project = 'TEST",
			},
			expectError:   true,
			errorContains: "unbalanced single quotes",
		},
		{
			name: "JQL validation - script injection",
			params: map[string]interface{}{
				"jql": "summary ~ '<script>alert(1)</script>'",
			},
			expectError:   true,
			errorContains: "potentially dangerous pattern detected",
		},
		{
			name: "Max results enforced",
			params: map[string]interface{}{
				"maxResults": 200.0, // Exceeds maximum
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 100,
				"total": 150,
				"issues": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "100", u.Query().Get("maxResults"))
			},
		},
		{
			name: "Project filter applied to JQL",
			params: map[string]interface{}{
				"jql": "status = Open",
			},
			projectFilter: "PROJ1,PROJ2",
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 2,
				"issues": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				jql := u.Query().Get("jql")
				assert.Contains(t, jql, "project in")
				assert.Contains(t, jql, "PROJ1")
				assert.Contains(t, jql, "PROJ2")
				assert.Contains(t, jql, "status = Open")
			},
		},
		{
			name: "Project filter with empty JQL",
			params: map[string]interface{}{
				"jql": "",
			},
			projectFilter: "PROJ",
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 5,
				"issues": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "project = \"PROJ\" ORDER BY created DESC", u.Query().Get("jql"))
			},
		},
		{
			name: "Results filtered by project",
			params: map[string]interface{}{
				"jql": "type = Bug",
			},
			projectFilter: "ALLOWED",
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 3,
				"issues": [
					{
						"id": "1",
						"key": "ALLOWED-1",
						"fields": {"project": {"key": "ALLOWED"}}
					},
					{
						"id": "2",
						"key": "BLOCKED-1",
						"fields": {"project": {"key": "BLOCKED"}}
					},
					{
						"id": "3",
						"key": "ALLOWED-2",
						"fields": {"project": {"key": "ALLOWED"}}
					}
				]
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateResult: func(t *testing.T, result *ToolResult) {
				data := result.Data.(map[string]interface{})
				issues := data["issues"].([]interface{})
				assert.Equal(t, 2, len(issues), "Should filter out BLOCKED project")

				// Check that only ALLOWED issues remain
				for _, issue := range issues {
					issueMap := issue.(map[string]interface{})
					key := issueMap["key"].(string)
					assert.Contains(t, key, "ALLOWED")
				}

				// Check metadata
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, 3.0, metadata["originalTotal"])
				assert.Equal(t, 2, data["total"])
			},
		},
		{
			name: "Server error with details",
			params: map[string]interface{}{
				"jql": "invalid ] syntax",
			},
			serverResponse: `{
				"errorMessages": ["The JQL query is invalid"],
				"errors": {
					"jql": "Unexpected character ']' at position 8"
				}
			}`,
			statusCode:    http.StatusBadRequest,
			expectError:   true,
			errorContains: "Search failed with status 400",
		},
		{
			name: "Validate query disabled",
			params: map[string]interface{}{
				"jql":           "project = TEST",
				"validateQuery": false,
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 0,
				"issues": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "false", u.Query().Get("validateQuery"))
			},
		},
		{
			name: "Last page (no more results)",
			params: map[string]interface{}{
				"jql":        "type = Story",
				"startAt":    40.0,
				"maxResults": 10.0,
			},
			serverResponse: `{
				"startAt": 40,
				"maxResults": 10,
				"total": 45,
				"issues": [1, 2, 3, 4, 5]
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
			validateResult: func(t *testing.T, result *ToolResult) {
				data := result.Data.(map[string]interface{})
				metadata := data["_metadata"].(map[string]interface{})
				assert.False(t, metadata["hasMore"].(bool))
				assert.Nil(t, metadata["nextStartAt"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify authentication header
				authHeader := r.Header.Get("Authorization")
				assert.NotEmpty(t, authHeader)

				// Check URL and method
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/search")

				// Run custom URL validation if provided
				if tt.validateURL != nil {
					tt.validateURL(t, r.URL)
				}

				// Return response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != "" {
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)

			// Set up context with project filter if needed
			ctx := context.Background()
			if tt.projectFilter != "" {
				ctx = providers.WithContext(ctx, &providers.ProviderContext{
					Metadata: map[string]interface{}{
						"JIRA_PROJECTS_FILTER": tt.projectFilter,
					},
				})
			}

			// Add authentication to params
			tt.params["token"] = "test@example.com:test-token"

			// Create and execute handler
			handler := NewSearchIssuesHandler(provider)
			result, _ := handler.Execute(ctx, tt.params)

			// Check results
			if tt.expectError {
				require.NotNil(t, result)
				assert.False(t, result.Success)
				if tt.errorContains != "" {
					assert.Contains(t, result.Error, tt.errorContains)
				}
			} else {
				require.NotNil(t, result)
				if !result.Success {
					t.Errorf("Expected success but got error: %s", result.Error)
				}
				assert.True(t, result.Success)
				assert.NotNil(t, result.Data)

				// Run custom result validation if provided
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}

				// Always check for metadata
				if result.Data != nil {
					data := result.Data.(map[string]interface{})
					metadata := data["_metadata"].(map[string]interface{})
					assert.Equal(t, "v3", metadata["api_version"])
					assert.Equal(t, "search_issues", metadata["operation"])
				}
			}
		})
	}
}

func TestSearchIssuesHandler_JQLValidation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-domain")
	handler := NewSearchIssuesHandler(provider)

	tests := []struct {
		name        string
		jql         string
		shouldError bool
		errorMsg    string
	}{
		{"Empty JQL", "", false, ""},
		{"Valid simple JQL", "project = TEST", false, ""},
		{"Valid complex JQL", "project = TEST AND (status = Open OR priority = High)", false, ""},
		{"SQL injection attempt 1", "project = 'TEST'; DROP TABLE issues;", true, "potentially dangerous"},
		{"SQL injection attempt 2", "summary ~ 'test' OR '1'='1'", true, "potentially dangerous"},
		{"Script injection", "description ~ '<script>alert(1)</script>'", true, "potentially dangerous"},
		{"Unbalanced parentheses", "project = TEST AND (status = Open", true, "unbalanced parentheses"},
		{"Unbalanced single quotes", "project = 'TEST", true, "unbalanced single quotes"},
		{"Unbalanced double quotes", "summary ~ \"incomplete", true, "unbalanced double quotes"},
		{"Too long JQL", string(make([]byte, 5001)), true, "too long"},
		{"SQL comment injection", "project = TEST /* comment */ AND status = Open", true, "potentially dangerous"},
		{"Exec command injection", "project = TEST exec xp_cmdshell", true, "potentially dangerous"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateJQL(tt.jql)
			if tt.shouldError {
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

func TestSearchIssuesHandler_ProjectFilterApplication(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-domain")
	handler := NewSearchIssuesHandler(provider)

	tests := []struct {
		name          string
		originalJQL   string
		projectFilter string
		expectedJQL   string
	}{
		{
			name:          "No filter",
			originalJQL:   "status = Open",
			projectFilter: "",
			expectedJQL:   "status = Open",
		},
		{
			name:          "Wildcard filter",
			originalJQL:   "status = Open",
			projectFilter: "*",
			expectedJQL:   "status = Open",
		},
		{
			name:          "Single project filter",
			originalJQL:   "status = Open",
			projectFilter: "PROJ",
			expectedJQL:   "(project = \"PROJ\") AND (status = Open)",
		},
		{
			name:          "Multiple projects filter",
			originalJQL:   "type = Bug",
			projectFilter: "PROJ1,PROJ2,PROJ3",
			expectedJQL:   "(project in (\"PROJ1\", \"PROJ2\", \"PROJ3\")) AND (type = Bug)",
		},
		{
			name:          "Filter with empty JQL",
			originalJQL:   "",
			projectFilter: "PROJ",
			expectedJQL:   "project = \"PROJ\" ",
		},
		{
			name:          "Filter with ORDER BY only",
			originalJQL:   "ORDER BY created DESC",
			projectFilter: "PROJ",
			expectedJQL:   "project = \"PROJ\" ORDER BY created DESC",
		},
		{
			name:          "Filter with spaces in project list",
			originalJQL:   "status = Open",
			projectFilter: " PROJ1 , PROJ2 , PROJ3 ",
			expectedJQL:   "(project in (\"PROJ1\", \"PROJ2\", \"PROJ3\")) AND (status = Open)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.projectFilter != "" {
				ctx = providers.WithContext(ctx, &providers.ProviderContext{
					Metadata: map[string]interface{}{
						"JIRA_PROJECTS_FILTER": tt.projectFilter,
					},
				})
			}

			result := handler.applyProjectFilterToJQL(ctx, tt.originalJQL)
			assert.Equal(t, tt.expectedJQL, result)
		})
	}
}

func TestSearchIssuesHandler_Integration(t *testing.T) {
	t.Run("Handler Registration", func(t *testing.T) {
		logger := &observability.NoopLogger{}
		provider := NewJiraProvider(logger, "test-domain")

		// Register handlers
		provider.registerHandlers()

		// Verify search handler is registered
		handler, exists := provider.toolRegistry["search_issues"]
		assert.True(t, exists, "search_issues handler should be registered")
		assert.NotNil(t, handler)

		// Verify handler definition
		def := handler.GetDefinition()
		assert.Equal(t, "search_issues", def.Name)
		assert.Contains(t, def.Description, "JQL")
		assert.NotNil(t, def.InputSchema)
	})

	t.Run("Search Toolset", func(t *testing.T) {
		logger := &observability.NoopLogger{}
		provider := NewJiraProvider(logger, "test-domain")

		// Register handlers
		provider.registerHandlers()

		// Verify search toolset exists
		toolset, exists := provider.toolsetRegistry["search"]
		assert.True(t, exists)
		assert.NotNil(t, toolset)
		assert.Equal(t, "search", toolset.Name)
		assert.Contains(t, toolset.Description, "search")

		// Verify handler is in the toolset
		assert.Equal(t, 1, len(toolset.Tools))
	})

	t.Run("Pagination Calculation", func(t *testing.T) {
		// Test pagination metadata calculation
		testCases := []struct {
			total       float64
			startAt     int
			maxResults  int
			expectMore  bool
			nextStartAt int
		}{
			{100, 0, 50, true, 50},
			{100, 50, 50, false, 0},
			{100, 90, 20, false, 0},
			{100, 80, 10, true, 90},
			{10, 0, 50, false, 0},
		}

		for _, tc := range testCases {
			result := map[string]interface{}{
				"total": tc.total,
				"_metadata": map[string]interface{}{
					"startAt":    tc.startAt,
					"maxResults": tc.maxResults,
				},
			}

			// Calculate pagination
			nextStart := tc.startAt + tc.maxResults
			if float64(nextStart) < tc.total {
				result["_metadata"].(map[string]interface{})["nextStartAt"] = nextStart
				result["_metadata"].(map[string]interface{})["hasMore"] = true
			} else {
				result["_metadata"].(map[string]interface{})["hasMore"] = false
			}

			metadata := result["_metadata"].(map[string]interface{})
			assert.Equal(t, tc.expectMore, metadata["hasMore"])
			if tc.expectMore {
				assert.Equal(t, tc.nextStartAt, metadata["nextStartAt"])
			}
		}
	})
}
