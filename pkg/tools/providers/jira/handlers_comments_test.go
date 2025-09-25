package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
)

func TestGetCommentsHandler(t *testing.T) {
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
			name: "Get comments successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 2,
				"comments": [
					{
						"id": "10001",
						"author": {
							"accountId": "user1",
							"displayName": "User One"
						},
						"body": {
							"type": "doc",
							"version": 1,
							"content": [
								{
									"type": "paragraph",
									"content": [
										{"type": "text", "text": "First comment"}
									]
								}
							]
						},
						"created": "2024-01-01T10:00:00.000+0000",
						"updated": "2024-01-01T10:00:00.000+0000"
					},
					{
						"id": "10002",
						"author": {
							"accountId": "user2",
							"displayName": "User Two"
						},
						"body": {
							"type": "doc",
							"version": 1,
							"content": [
								{
									"type": "paragraph",
									"content": [
										{"type": "text", "text": "Second comment"}
									]
								}
							]
						},
						"created": "2024-01-02T10:00:00.000+0000",
						"updated": "2024-01-02T10:00:00.000+0000"
					}
				]
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "/rest/api/3/issue/PROJ-123/comment", u.Path)
				query := u.Query()
				assert.Equal(t, "50", query.Get("maxResults"))
				assert.Equal(t, "-created", query.Get("orderBy"))
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				assert.True(t, result.Success)
				data := result.Data.(map[string]interface{})
				assert.Equal(t, float64(2), data["total"])
				comments := data["comments"].([]interface{})
				assert.Len(t, comments, 2)

				// Check metadata
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, "v3", metadata["api_version"])
				assert.Equal(t, "get_comments", metadata["operation"])
				assert.Equal(t, "PROJ-123", metadata["issue"])
				assert.False(t, metadata["hasMore"].(bool))
			},
		},
		{
			name: "Get comments with pagination",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"startAt":      float64(10),
				"maxResults":   float64(25),
				"orderBy":      "created",
			},
			serverResponse: `{
				"startAt": 10,
				"maxResults": 25,
				"total": 100,
				"comments": []
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "/rest/api/3/issue/PROJ-456/comment", u.Path)
				query := u.Query()
				assert.Equal(t, "10", query.Get("startAt"))
				assert.Equal(t, "25", query.Get("maxResults"))
				assert.Equal(t, "created", query.Get("orderBy"))
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				assert.True(t, result.Success)
				data := result.Data.(map[string]interface{})
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, float64(35), metadata["nextStartAt"])
				assert.True(t, metadata["hasMore"].(bool))
			},
		},
		{
			name: "Get comments with expand parameter",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-789",
				"expand":       "renderedBody",
			},
			serverResponse: `{
				"startAt": 0,
				"maxResults": 50,
				"total": 1,
				"comments": [
					{
						"id": "10003",
						"body": {
							"type": "doc",
							"version": 1,
							"content": []
						},
						"renderedBody": "<p>Rendered HTML content</p>"
					}
				]
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "/rest/api/3/issue/PROJ-789/comment", u.Path)
				assert.Equal(t, "renderedBody", u.Query().Get("expand"))
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				assert.True(t, result.Success)
				data := result.Data.(map[string]interface{})
				comments := data["comments"].([]interface{})
				comment := comments[0].(map[string]interface{})
				assert.Contains(t, comment, "renderedBody")
			},
		},
		{
			name: "Get comments with project filter - allowed",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-100",
			},
			projectFilter:  "PROJ,OTHER",
			serverResponse: `{"comments": [], "total": 0, "startAt": 0, "maxResults": 50}`,
			statusCode:     http.StatusOK,
			validateResult: func(t *testing.T, result *ToolResult) {
				assert.True(t, result.Success)
			},
		},
		{
			name: "Get comments with project filter - denied",
			params: map[string]interface{}{
				"issueIdOrKey": "DENIED-100",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Issue belongs to project DENIED which is not in allowed projects",
		},
		{
			name: "Missing issueIdOrKey",
			params: map[string]interface{}{
				"startAt": float64(0),
			},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name: "Server error response",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-999",
			},
			serverResponse: `{
				"errorMessages": ["Issue does not exist or you do not have permission to see it"],
				"errors": {}
			}`,
			statusCode:    http.StatusNotFound,
			expectError:   true,
			errorContains: "Failed to get comments with status 404",
		},
		{
			name: "Max results exceeds limit",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-200",
				"maxResults":   float64(200),
			},
			serverResponse: `{"comments": [], "total": 0, "startAt": 0, "maxResults": 100}`,
			statusCode:     http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				// Should cap at 100
				assert.Equal(t, "100", u.Query().Get("maxResults"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")
				assert.Contains(t, r.URL.Path, "/comment")

				// Validate authentication
				authHeader := r.Header.Get("Authorization")
				assert.True(t, strings.HasPrefix(authHeader, "Basic "))

				// Custom URL validation
				if tt.validateURL != nil {
					tt.validateURL(t, r.URL)
				}

				// Return response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewGetCommentsHandler(provider)

			// Create context with credentials
			ctx := context.Background()
			pctx := &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"email":     "test@example.com",
					"api_token": "test-token",
				},
			}

			// Add project filter if specified
			if tt.projectFilter != "" {
				pctx.Metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}

			ctx = providers.WithContext(ctx, pctx)

			// Execute handler
			result, _ := handler.Execute(ctx, tt.params)

			// Check result
			if tt.expectError {
				assert.False(t, result.Success)
				if tt.errorContains != "" {
					assert.Contains(t, result.Error, tt.errorContains)
				}
			} else {
				assert.True(t, result.Success)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}
		})
	}
}

func TestAddCommentHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse string
		statusCode     int
		projectFilter  string
		readOnly       bool
		expectError    bool
		errorContains  string
		validateBody   func(t *testing.T, body map[string]interface{})
		validateResult func(t *testing.T, result *ToolResult)
	}{
		{
			name: "Add plain text comment",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"body":         "This is a test comment\n\nWith multiple paragraphs",
			},
			serverResponse: `{
				"id": "10001",
				"author": {
					"accountId": "user1",
					"displayName": "Test User"
				},
				"body": {
					"type": "doc",
					"version": 1,
					"content": [
						{
							"type": "paragraph",
							"content": [
								{"type": "text", "text": "This is a test comment"}
							]
						}
					]
				},
				"created": "2024-01-01T10:00:00.000+0000"
			}`,
			statusCode: http.StatusCreated,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				// Check ADF conversion
				adfBody := body["body"].(map[string]interface{})
				assert.Equal(t, "doc", adfBody["type"])
				assert.Equal(t, float64(1), adfBody["version"])
				content := adfBody["content"].([]interface{})
				assert.Len(t, content, 2) // Two paragraphs
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				assert.True(t, result.Success)
				data := result.Data.(map[string]interface{})
				assert.Equal(t, "10001", data["id"])

				// Check metadata
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, "v3", metadata["api_version"])
				assert.Equal(t, "add_comment", metadata["operation"])
				assert.Equal(t, "PROJ-123", metadata["issue"])
			},
		},
		{
			name: "Add ADF formatted comment",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"bodyADF": map[string]interface{}{
					"type":    "doc",
					"version": 1,
					"content": []interface{}{
						map[string]interface{}{
							"type": "paragraph",
							"content": []interface{}{
								map[string]interface{}{
									"type": "text",
									"text": "Bold text",
									"marks": []interface{}{
										map[string]interface{}{"type": "strong"},
									},
								},
							},
						},
					},
				},
			},
			serverResponse: `{
				"id": "10002",
				"body": {
					"type": "doc",
					"version": 1,
					"content": []
				}
			}`,
			statusCode: http.StatusCreated,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				// Should use ADF directly
				adfBody := body["body"].(map[string]interface{})
				assert.Equal(t, "doc", adfBody["type"])
				content := adfBody["content"].([]interface{})
				paragraph := content[0].(map[string]interface{})
				paragraphContent := paragraph["content"].([]interface{})
				text := paragraphContent[0].(map[string]interface{})
				assert.Contains(t, text, "marks")
			},
		},
		{
			name: "Add comment with visibility restriction",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-789",
				"body":         "Internal comment",
				"visibility": map[string]interface{}{
					"type":  "group",
					"value": "developers",
				},
			},
			serverResponse: `{"id": "10003"}`,
			statusCode:     http.StatusCreated,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				visibility := body["visibility"].(map[string]interface{})
				assert.Equal(t, "group", visibility["type"])
				assert.Equal(t, "developers", visibility["value"])
			},
		},
		{
			name: "Add comment with properties",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-100",
				"body":         "Comment with properties",
				"properties": []interface{}{
					map[string]interface{}{
						"key":   "sentiment",
						"value": "positive",
					},
				},
			},
			serverResponse: `{"id": "10004"}`,
			statusCode:     http.StatusCreated,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				properties := body["properties"].([]interface{})
				assert.Len(t, properties, 1)
				prop := properties[0].(map[string]interface{})
				assert.Equal(t, "sentiment", prop["key"])
			},
		},
		{
			name: "Read-only mode prevents adding",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-200",
				"body":         "Should not be added",
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "Cannot add comment in read-only mode",
		},
		{
			name: "Project filter denies access",
			params: map[string]interface{}{
				"issueIdOrKey": "DENIED-100",
				"body":         "Should be denied",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Issue belongs to project DENIED which is not in allowed projects",
		},
		{
			name: "Missing required body",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-300",
			},
			expectError:   true,
			errorContains: "Either 'body' (plain text) or 'bodyADF' (rich text) is required",
		},
		{
			name: "Server error response",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-999",
				"body":         "Test",
			},
			serverResponse: `{
				"errorMessages": ["You do not have permission to comment on this issue"],
				"errors": {}
			}`,
			statusCode:    http.StatusForbidden,
			expectError:   true,
			errorContains: "Failed to add comment with status 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]interface{}

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")
				assert.Contains(t, r.URL.Path, "/comment")
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Capture body
				decoder := json.NewDecoder(r.Body)
				_ = decoder.Decode(&receivedBody)

				// Custom body validation
				if tt.validateBody != nil {
					tt.validateBody(t, receivedBody)
				}

				// Return response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewAddCommentHandler(provider)

			// Create context with credentials
			ctx := context.Background()
			pctx := &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"email":     "test@example.com",
					"api_token": "test-token",
				},
			}

			// Add additional metadata
			if tt.projectFilter != "" {
				pctx.Metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}
			if tt.readOnly {
				pctx.Metadata["JIRA_READ_ONLY"] = true
			}

			ctx = providers.WithContext(ctx, pctx)

			// Execute handler
			result, _ := handler.Execute(ctx, tt.params)

			// Check result
			if tt.expectError {
				assert.False(t, result.Success)
				if tt.errorContains != "" {
					assert.Contains(t, result.Error, tt.errorContains)
				}
			} else {
				assert.True(t, result.Success)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}
		})
	}
}

func TestUpdateCommentHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse string
		statusCode     int
		readOnly       bool
		projectFilter  string
		expectError    bool
		errorContains  string
		validateURL    func(t *testing.T, u *url.URL)
	}{
		{
			name: "Update comment successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"commentId":    "10001",
				"body":         "Updated comment text",
			},
			serverResponse: `{
				"id": "10001",
				"body": {
					"type": "doc",
					"version": 1,
					"content": []
				},
				"updated": "2024-01-02T10:00:00.000+0000"
			}`,
			statusCode: http.StatusOK,
		},
		{
			name: "Update with notify users disabled",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"commentId":    "10002",
				"body":         "Silent update",
				"notifyUsers":  false,
			},
			serverResponse: `{"id": "10002"}`,
			statusCode:     http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "false", u.Query().Get("notifyUsers"))
			},
		},
		{
			name: "Update with ADF body",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-789",
				"commentId":    "10003",
				"bodyADF": map[string]interface{}{
					"type":    "doc",
					"version": 1,
					"content": []interface{}{},
				},
			},
			serverResponse: `{"id": "10003"}`,
			statusCode:     http.StatusOK,
		},
		{
			name: "Read-only mode prevents update",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-100",
				"commentId":    "10004",
				"body":         "Should not update",
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "Cannot update comment in read-only mode",
		},
		{
			name: "Project filter denies access",
			params: map[string]interface{}{
				"issueIdOrKey": "DENIED-100",
				"commentId":    "10005",
				"body":         "Should be denied",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Issue belongs to project DENIED which is not in allowed projects",
		},
		{
			name:          "Missing issueIdOrKey",
			params:        map[string]interface{}{"commentId": "10006", "body": "Test"},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name:          "Missing commentId",
			params:        map[string]interface{}{"issueIdOrKey": "PROJ-200", "body": "Test"},
			expectError:   true,
			errorContains: "commentId is required",
		},
		{
			name: "Missing body",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-300",
				"commentId":    "10007",
			},
			expectError:   true,
			errorContains: "Either 'body' (plain text) or 'bodyADF' (rich text) is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "PUT", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")
				assert.Contains(t, r.URL.Path, "/comment/")

				// Custom URL validation
				if tt.validateURL != nil {
					tt.validateURL(t, r.URL)
				}

				// Return response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewUpdateCommentHandler(provider)

			// Create context
			ctx := context.Background()
			pctx := &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"email":     "test@example.com",
					"api_token": "test-token",
				},
			}

			// Add additional metadata
			if tt.projectFilter != "" {
				pctx.Metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}
			if tt.readOnly {
				pctx.Metadata["JIRA_READ_ONLY"] = true
			}

			ctx = providers.WithContext(ctx, pctx)

			// Execute handler
			result, _ := handler.Execute(ctx, tt.params)

			// Check result
			if tt.expectError {
				assert.False(t, result.Success)
				if tt.errorContains != "" {
					assert.Contains(t, result.Error, tt.errorContains)
				}
			} else {
				assert.True(t, result.Success)
			}
		})
	}
}

func TestDeleteCommentHandler(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]interface{}
		statusCode    int
		readOnly      bool
		projectFilter string
		expectError   bool
		errorContains string
	}{
		{
			name: "Delete comment successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"commentId":    "10001",
			},
			statusCode: http.StatusNoContent,
		},
		{
			name: "Read-only mode prevents deletion",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"commentId":    "10002",
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "Cannot delete comment in read-only mode",
		},
		{
			name: "Project filter denies access",
			params: map[string]interface{}{
				"issueIdOrKey": "DENIED-100",
				"commentId":    "10003",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Issue belongs to project DENIED which is not in allowed projects",
		},
		{
			name:          "Missing issueIdOrKey",
			params:        map[string]interface{}{"commentId": "10004"},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name:          "Missing commentId",
			params:        map[string]interface{}{"issueIdOrKey": "PROJ-789"},
			expectError:   true,
			errorContains: "commentId is required",
		},
		{
			name: "Server returns not found",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-999",
				"commentId":    "99999",
			},
			statusCode:    http.StatusNotFound,
			expectError:   true,
			errorContains: "Failed to delete comment with status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "DELETE", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")
				assert.Contains(t, r.URL.Path, "/comment/")

				// Return response
				w.WriteHeader(tt.statusCode)
				if tt.statusCode >= 400 {
					_, _ = w.Write([]byte(`{"errorMessages": ["Comment not found"]}`))
				}
			}))
			defer server.Close()

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewDeleteCommentHandler(provider)

			// Create context
			ctx := context.Background()
			pctx := &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"email":     "test@example.com",
					"api_token": "test-token",
				},
			}

			// Add additional metadata
			if tt.projectFilter != "" {
				pctx.Metadata["JIRA_PROJECTS_FILTER"] = tt.projectFilter
			}
			if tt.readOnly {
				pctx.Metadata["JIRA_READ_ONLY"] = true
			}

			ctx = providers.WithContext(ctx, pctx)

			// Execute handler
			result, _ := handler.Execute(ctx, tt.params)

			// Check result
			if tt.expectError {
				assert.False(t, result.Success)
				if tt.errorContains != "" {
					assert.Contains(t, result.Error, tt.errorContains)
				}
			} else {
				assert.True(t, result.Success)
				data := result.Data.(map[string]interface{})
				assert.True(t, data["success"].(bool))
				assert.Contains(t, data["message"], "deleted successfully")
			}
		})
	}
}

func TestConvertTextToADF(t *testing.T) {
	handler := &AddCommentHandler{}

	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "Single paragraph",
			input: "This is a single paragraph",
			expected: map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "This is a single paragraph",
							},
						},
					},
				},
			},
		},
		{
			name:  "Multiple paragraphs",
			input: "First paragraph\n\nSecond paragraph\n\nThird paragraph",
			expected: map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "First paragraph",
							},
						},
					},
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Second paragraph",
							},
						},
					},
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Third paragraph",
							},
						},
					},
				},
			},
		},
		{
			name:  "Empty string",
			input: "",
			expected: map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []interface{}{
					map[string]interface{}{
						"type":    "paragraph",
						"content": []interface{}{},
					},
				},
			},
		},
		{
			name:  "Whitespace only",
			input: "   \n\n   ",
			expected: map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []interface{}{
					map[string]interface{}{
						"type":    "paragraph",
						"content": []interface{}{},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.convertTextToADF(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommentHandlerDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-domain")

	t.Run("GetCommentsHandler definition", func(t *testing.T) {
		handler := NewGetCommentsHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "get_comments", def.Name)
		assert.Equal(t, "Get comments for a Jira issue", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")

		// Check properties
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "issueIdOrKey")
		assert.Contains(t, properties, "startAt")
		assert.Contains(t, properties, "maxResults")
		assert.Contains(t, properties, "orderBy")
		assert.Contains(t, properties, "expand")

		// Check orderBy enum
		orderByProp := properties["orderBy"].(map[string]interface{})
		orderByEnum := orderByProp["enum"].([]interface{})
		assert.Contains(t, orderByEnum, "created")
		assert.Contains(t, orderByEnum, "-created")
	})

	t.Run("AddCommentHandler definition", func(t *testing.T) {
		handler := NewAddCommentHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "add_comment", def.Name)
		assert.Equal(t, "Add a comment to a Jira issue", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")

		// Check properties
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "issueIdOrKey")
		assert.Contains(t, properties, "body")
		assert.Contains(t, properties, "bodyADF")
		assert.Contains(t, properties, "visibility")
		assert.Contains(t, properties, "properties")

		// Check visibility enum
		visibilityProp := properties["visibility"].(map[string]interface{})
		visibilityProps := visibilityProp["properties"].(map[string]interface{})
		typeProp := visibilityProps["type"].(map[string]interface{})
		typeEnum := typeProp["enum"].([]interface{})
		assert.Contains(t, typeEnum, "group")
		assert.Contains(t, typeEnum, "role")
	})

	t.Run("UpdateCommentHandler definition", func(t *testing.T) {
		handler := NewUpdateCommentHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "update_comment", def.Name)
		assert.Equal(t, "Update an existing comment on a Jira issue", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")
		assert.Contains(t, required, "commentId")

		// Check properties
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "notifyUsers")
		notifyUsersProp := properties["notifyUsers"].(map[string]interface{})
		assert.Equal(t, true, notifyUsersProp["default"])
	})

	t.Run("DeleteCommentHandler definition", func(t *testing.T) {
		handler := NewDeleteCommentHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "delete_comment", def.Name)
		assert.Equal(t, "Delete a comment from a Jira issue", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")
		assert.Contains(t, required, "commentId")
	})
}
