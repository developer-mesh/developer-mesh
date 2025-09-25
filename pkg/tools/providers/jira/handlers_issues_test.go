package jira

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

func TestGetIssueHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse string
		statusCode     int
		projectFilter  string
		expectError    bool
		errorContains  string
	}{
		{
			name: "Get issue successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"fields":       []interface{}{"summary", "description", "status"},
			},
			serverResponse: `{
				"id": "10000",
				"key": "PROJ-123",
				"fields": {
					"summary": "Test Issue",
					"description": "Test Description",
					"status": {"name": "Open"},
					"project": {"key": "PROJ", "name": "Project"}
				}
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name: "Get issue with expand parameter",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"expand":       "changelog,transitions",
			},
			serverResponse: `{
				"id": "10000",
				"key": "PROJ-123",
				"fields": {"project": {"key": "PROJ"}},
				"changelog": {},
				"transitions": []
			}`,
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name: "Missing issue key",
			params: map[string]interface{}{
				"fields": []interface{}{"summary"},
			},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name: "Issue not found",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-999",
			},
			serverResponse: `{"errorMessages":["Issue does not exist or you do not have permission to see it."]}`,
			statusCode:     http.StatusNotFound,
			expectError:    true,
			errorContains:  "status 404",
		},
		{
			name: "Project filter blocks access",
			params: map[string]interface{}{
				"issueIdOrKey": "BLOCKED-123",
			},
			serverResponse: `{
				"id": "10001",
				"key": "BLOCKED-123",
				"fields": {"project": {"key": "BLOCKED"}}
			}`,
			statusCode:    http.StatusOK,
			projectFilter: "PROJ,ALLOWED",
			expectError:   true,
			errorContains: "not in allowed projects",
		},
		{
			name: "Project filter allows access",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
			},
			serverResponse: `{
				"id": "10000",
				"key": "PROJ-123",
				"fields": {"project": {"key": "PROJ"}}
			}`,
			statusCode:    http.StatusOK,
			projectFilter: "PROJ,OTHER",
			expectError:   false,
		},
		{
			name: "Wildcard project filter allows all",
			params: map[string]interface{}{
				"issueIdOrKey": "ANY-123",
			},
			serverResponse: `{
				"id": "10000",
				"key": "ANY-123",
				"fields": {"project": {"key": "ANY"}}
			}`,
			statusCode:    http.StatusOK,
			projectFilter: "*",
			expectError:   false,
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
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")

				// Check query parameters
				if tt.params["fields"] != nil {
					assert.Contains(t, r.URL.Query().Get("fields"), "summary")
				}
				if expand, ok := tt.params["expand"].(string); ok {
					assert.Equal(t, expand, r.URL.Query().Get("expand"))
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
			// Log the server URL for debugging
			t.Logf("Test server URL: %s", server.URL)
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
			handler := NewGetIssueHandler(provider)
			result, _ := handler.Execute(ctx, tt.params)

			// Check results
			if tt.expectError {
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tt.errorContains)
			} else {
				require.NotNil(t, result)
				if !result.Success {
					t.Errorf("Expected success but got error: %s", result.Error)
				}
				assert.True(t, result.Success)
				assert.NotNil(t, result.Data)

				// Only check metadata if we have data
				if result.Data != nil {
					data := result.Data.(map[string]interface{})
					metadata := data["_metadata"].(map[string]interface{})
					assert.Equal(t, "v3", metadata["api_version"])
					assert.Equal(t, "get_issue", metadata["operation"])
				}
			}
		})
	}
}

func TestCreateIssueHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse string
		statusCode     int
		projectFilter  string
		readOnly       bool
		expectError    bool
		errorContains  string
	}{
		{
			name: "Create issue successfully",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project":     map[string]interface{}{"key": "PROJ"},
					"issuetype":   map[string]interface{}{"name": "Bug"},
					"summary":     "Test Bug",
					"description": "Test Description",
				},
			},
			serverResponse: `{
				"id": "10000",
				"key": "PROJ-123",
				"self": "https://example.atlassian.net/rest/api/3/issue/10000"
			}`,
			statusCode:  http.StatusCreated,
			expectError: false,
		},
		{
			name: "Read-only mode blocks creation",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project":   map[string]interface{}{"key": "PROJ"},
					"issuetype": map[string]interface{}{"name": "Bug"},
					"summary":   "Test Bug",
				},
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "read-only mode",
		},
		{
			name: "Missing fields parameter",
			params: map[string]interface{}{
				"summary": "Test",
			},
			expectError:   true,
			errorContains: "fields parameter is required",
		},
		{
			name: "Missing required field - summary",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project":   map[string]interface{}{"key": "PROJ"},
					"issuetype": map[string]interface{}{"name": "Bug"},
				},
			},
			expectError:   true,
			errorContains: "summary is required",
		},
		{
			name: "Missing required field - issuetype",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project": map[string]interface{}{"key": "PROJ"},
					"summary": "Test",
				},
			},
			expectError:   true,
			errorContains: "issuetype is required",
		},
		{
			name: "Project filter blocks creation",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project":   map[string]interface{}{"key": "BLOCKED"},
					"issuetype": map[string]interface{}{"name": "Bug"},
					"summary":   "Test Bug",
				},
			},
			projectFilter: "PROJ,ALLOWED",
			expectError:   true,
			errorContains: "not in allowed projects",
		},
		{
			name: "Server error with field details",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project":   map[string]interface{}{"key": "PROJ"},
					"issuetype": map[string]interface{}{"name": "Bug"},
					"summary":   "Test",
				},
			},
			serverResponse: `{
				"errors": {
					"priority": "Priority is required",
					"components": "Component 'Invalid' is not valid"
				}
			}`,
			statusCode:    http.StatusBadRequest,
			expectError:   true,
			errorContains: "priority: Priority is required",
		},
		{
			name: "Custom fields support",
			params: map[string]interface{}{
				"fields": map[string]interface{}{
					"project":           map[string]interface{}{"key": "PROJ"},
					"issuetype":         map[string]interface{}{"name": "Story"},
					"summary":           "User Story",
					"customfield_10001": "Custom Value",
					"customfield_10002": map[string]interface{}{
						"value": "Option A",
					},
				},
			},
			serverResponse: `{
				"id": "10001",
				"key": "PROJ-124"
			}`,
			statusCode:  http.StatusCreated,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method and path
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue")

				// Verify content type
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Parse request body
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Logf("Failed to decode request body: %v", err)
				}

				// Verify fields were sent
				if fields, ok := body["fields"].(map[string]interface{}); ok {
					if tt.name == "Custom fields support" {
						assert.Contains(t, fields, "customfield_10001")
					}
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

			// Set up context
			ctx := context.Background()
			metadata := make(map[string]interface{})
			if tt.projectFilter != "" {
				metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}
			if tt.readOnly {
				metadata["READ_ONLY"] = true
			}
			if len(metadata) > 0 {
				ctx = providers.WithContext(ctx, &providers.ProviderContext{
					Metadata: metadata,
				})
			}

			// Add authentication to params
			tt.params["token"] = "test@example.com:test-token"

			// Create and execute handler
			handler := NewCreateIssueHandler(provider)
			result, _ := handler.Execute(ctx, tt.params)

			// Check results
			if tt.expectError {
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tt.errorContains)
			} else {
				require.NotNil(t, result)
				assert.True(t, result.Success)
				assert.NotNil(t, result.Data)

				// Verify metadata
				data := result.Data.(map[string]interface{})
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, "v3", metadata["api_version"])
				assert.Equal(t, "create_issue", metadata["operation"])
			}
		})
	}
}

func TestUpdateIssueHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		getResponse    string
		updateResponse string
		getStatusCode  int
		statusCode     int
		projectFilter  string
		readOnly       bool
		expectError    bool
		errorContains  string
	}{
		{
			name: "Update issue successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"fields": map[string]interface{}{
					"summary":     "Updated Summary",
					"description": "Updated Description",
				},
			},
			getResponse:   `{"fields": {"project": {"key": "PROJ"}}}`,
			getStatusCode: http.StatusOK,
			statusCode:    http.StatusNoContent,
			expectError:   false,
		},
		{
			name: "Update with notification settings",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"fields": map[string]interface{}{
					"priority": map[string]interface{}{"name": "High"},
				},
				"notifyUsers": false,
			},
			getResponse:   `{"fields": {"project": {"key": "PROJ"}}}`,
			getStatusCode: http.StatusOK,
			statusCode:    http.StatusNoContent,
			expectError:   false,
		},
		{
			name: "Read-only mode blocks update",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"fields":       map[string]interface{}{"summary": "New"},
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "read-only mode",
		},
		{
			name: "Missing issue key",
			params: map[string]interface{}{
				"fields": map[string]interface{}{"summary": "New"},
			},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name: "Project filter blocks update",
			params: map[string]interface{}{
				"issueIdOrKey": "BLOCKED-123",
				"fields":       map[string]interface{}{"summary": "New"},
			},
			getResponse:   `{"fields": {"project": {"key": "BLOCKED"}}}`,
			getStatusCode: http.StatusOK,
			projectFilter: "PROJ,ALLOWED",
			expectError:   true,
			errorContains: "not in allowed projects",
		},
		{
			name: "Field validation error",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"fields": map[string]interface{}{
					"fixVersion": map[string]interface{}{"name": "Invalid"},
				},
			},
			getResponse:   `{"fields": {"project": {"key": "PROJ"}}}`,
			getStatusCode: http.StatusOK,
			updateResponse: `{
				"errors": {
					"fixVersion": "Version 'Invalid' does not exist"
				}
			}`,
			statusCode:    http.StatusBadRequest,
			expectError:   true,
			errorContains: "fixVersion: Version 'Invalid' does not exist",
		},
		{
			name: "Update with advanced update operations",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"update": map[string]interface{}{
					"labels": []map[string]interface{}{
						{"add": "backend"},
						{"add": "urgent"},
					},
					"components": []map[string]interface{}{
						{"set": []map[string]interface{}{
							{"name": "Component A"},
						}},
					},
				},
			},
			getResponse:   `{"fields": {"project": {"key": "PROJ"}}}`,
			getStatusCode: http.StatusOK,
			statusCode:    http.StatusNoContent,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getRequestCount := 0
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case "GET":
					// This is the permission check request
					getRequestCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.getStatusCode)
					if tt.getResponse != "" {
						if _, err := w.Write([]byte(tt.getResponse)); err != nil {
							t.Logf("Failed to write response: %v", err)
						}
					}
				case "PUT":
					// This is the update request
					assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")

					// Check notification parameter
					if notifyUsers, ok := tt.params["notifyUsers"].(bool); ok && !notifyUsers {
						assert.Equal(t, "false", r.URL.Query().Get("notifyUsers"))
					}

					// Parse and verify body
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Logf("Failed to decode request body: %v", err)
					}
					if tt.params["update"] != nil {
						assert.Contains(t, body, "update")
					}

					// Return response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					if tt.updateResponse != "" {
						if _, err := w.Write([]byte(tt.updateResponse)); err != nil {
							t.Logf("Failed to write response: %v", err)
						}
					}
				}
			}))
			defer server.Close()

			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)

			// Set up context
			ctx := context.Background()
			metadata := make(map[string]interface{})
			if tt.projectFilter != "" {
				metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}
			if tt.readOnly {
				metadata["READ_ONLY"] = true
			}
			if len(metadata) > 0 {
				ctx = providers.WithContext(ctx, &providers.ProviderContext{
					Metadata: metadata,
				})
			}

			// Add authentication to params
			tt.params["token"] = "test@example.com:test-token"

			// Create and execute handler
			handler := NewUpdateIssueHandler(provider)
			result, _ := handler.Execute(ctx, tt.params)

			// Check results
			if tt.expectError {
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tt.errorContains)
			} else {
				require.NotNil(t, result)
				assert.True(t, result.Success)
				assert.NotNil(t, result.Data)

				// Verify success message
				data := result.Data.(map[string]interface{})
				assert.True(t, data["success"].(bool))
				assert.Contains(t, data["message"].(string), "updated successfully")

				// Verify metadata
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, "v3", metadata["api_version"])
				assert.Equal(t, "update_issue", metadata["operation"])
			}

			// Verify GET request was made for project filter cases
			if tt.projectFilter != "" && !tt.readOnly && tt.params["issueIdOrKey"] != nil {
				assert.Greater(t, getRequestCount, 0, "Should have made GET request for project check")
			}
		})
	}
}

func TestDeleteIssueHandler(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]interface{}
		getResponse   string
		getStatusCode int
		deleteStatus  int
		projectFilter string
		readOnly      bool
		expectError   bool
		errorContains string
	}{
		{
			name: "Delete issue successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
			},
			getResponse:   `{"fields": {"project": {"key": "PROJ"}}}`,
			getStatusCode: http.StatusOK,
			deleteStatus:  http.StatusNoContent,
			expectError:   false,
		},
		{
			name: "Delete with subtasks",
			params: map[string]interface{}{
				"issueIdOrKey":   "PROJ-123",
				"deleteSubtasks": true,
			},
			getResponse:   `{"fields": {"project": {"key": "PROJ"}}}`,
			getStatusCode: http.StatusOK,
			deleteStatus:  http.StatusNoContent,
			expectError:   false,
		},
		{
			name: "Read-only mode blocks deletion",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "read-only mode",
		},
		{
			name:          "Missing issue key",
			params:        map[string]interface{}{},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name: "Project filter blocks deletion",
			params: map[string]interface{}{
				"issueIdOrKey": "BLOCKED-123",
			},
			getResponse:   `{"fields": {"project": {"key": "BLOCKED"}}}`,
			getStatusCode: http.StatusOK,
			projectFilter: "PROJ,ALLOWED",
			expectError:   true,
			errorContains: "not in allowed projects",
		},
		{
			name: "Issue not found",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-999",
			},
			getResponse:   `{"errorMessages": ["Issue does not exist"]}`,
			getStatusCode: http.StatusOK,
			deleteStatus:  http.StatusNotFound,
			expectError:   true,
			errorContains: "status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getRequestCount := 0
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case "GET":
					// This is the permission check request
					getRequestCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.getStatusCode)
					if tt.getResponse != "" {
						if _, err := w.Write([]byte(tt.getResponse)); err != nil {
							t.Logf("Failed to write response: %v", err)
						}
					}
				case "DELETE":
					// This is the delete request
					assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")

					// Check deleteSubtasks parameter
					if deleteSubtasks, ok := tt.params["deleteSubtasks"].(bool); ok && deleteSubtasks {
						assert.Equal(t, "true", r.URL.Query().Get("deleteSubtasks"))
					}

					// Return response
					w.WriteHeader(tt.deleteStatus)
					if tt.deleteStatus != http.StatusNoContent {
						w.Header().Set("Content-Type", "application/json")
						if _, err := w.Write([]byte(`{"errorMessages": ["Issue does not exist"]}`)); err != nil {
							t.Logf("Failed to write error response: %v", err)
						}
					}
				}
			}))
			defer server.Close()

			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)

			// Set up context
			ctx := context.Background()
			metadata := make(map[string]interface{})
			if tt.projectFilter != "" {
				metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}
			if tt.readOnly {
				metadata["READ_ONLY"] = true
			}
			if len(metadata) > 0 {
				ctx = providers.WithContext(ctx, &providers.ProviderContext{
					Metadata: metadata,
				})
			}

			// Add authentication to params
			tt.params["token"] = "test@example.com:test-token"

			// Create and execute handler
			handler := NewDeleteIssueHandler(provider)
			result, _ := handler.Execute(ctx, tt.params)

			// Check results
			if tt.expectError {
				require.NotNil(t, result)
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tt.errorContains)
			} else {
				require.NotNil(t, result)
				assert.True(t, result.Success)
				assert.NotNil(t, result.Data)

				// Verify success message
				data := result.Data.(map[string]interface{})
				assert.True(t, data["success"].(bool))
				assert.Contains(t, data["message"].(string), "deleted successfully")

				// Verify metadata
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, "v3", metadata["api_version"])
				assert.Equal(t, "delete_issue", metadata["operation"])
			}

			// Verify GET request was made for project filter cases
			if tt.projectFilter != "" && !tt.readOnly && tt.params["issueIdOrKey"] != nil {
				assert.Greater(t, getRequestCount, 0, "Should have made GET request for project check")
			}
		})
	}
}

func TestIssueHandlersIntegration(t *testing.T) {
	t.Run("Handler Registration", func(t *testing.T) {
		logger := &observability.NoopLogger{}
		provider := NewJiraProvider(logger, "https://test.atlassian.net")

		// Register handlers
		provider.registerHandlers()

		// Verify all issue handlers are registered
		operations := []string{"get_issue", "create_issue", "update_issue", "delete_issue"}
		for _, op := range operations {
			handler, exists := provider.toolRegistry[op]
			assert.True(t, exists, "Handler %s should be registered", op)
			assert.NotNil(t, handler)

			// Verify handler definition
			def := handler.GetDefinition()
			assert.Equal(t, op, def.Name)
			assert.NotEmpty(t, def.Description)
			assert.NotNil(t, def.InputSchema)
		}
	})

	t.Run("Issues Toolset", func(t *testing.T) {
		logger := &observability.NoopLogger{}
		provider := NewJiraProvider(logger, "https://test.atlassian.net")

		// Register handlers
		provider.registerHandlers()

		// Verify issues toolset exists
		toolset, exists := provider.toolsetRegistry["issues"]
		assert.True(t, exists)
		assert.NotNil(t, toolset)
		assert.Equal(t, "issues", toolset.Name)
		assert.Contains(t, toolset.Description, "issue management")

		// Verify all handlers are in the toolset
		assert.Equal(t, 4, len(toolset.Tools))

		// Verify each tool handler
		for _, tool := range toolset.Tools {
			def := tool.GetDefinition()
			assert.Contains(t, []string{"get_issue", "create_issue", "update_issue", "delete_issue"}, def.Name)
		}
	})

	t.Run("Custom Fields Documentation", func(t *testing.T) {
		logger := &observability.NoopLogger{}
		provider := NewJiraProvider(logger, "https://test.atlassian.net")

		// Get create handler definition
		handler := NewCreateIssueHandler(provider)
		def := handler.GetDefinition()

		// Verify schema supports custom fields
		schema := def.InputSchema
		props := schema["properties"].(map[string]interface{})
		fields := props["fields"].(map[string]interface{})

		// The description should mention that custom fields are supported
		assert.Contains(t, fields["description"].(string), "project, issuetype, summary, description")
	})
}
