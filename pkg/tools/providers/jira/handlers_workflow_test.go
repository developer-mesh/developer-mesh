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

func TestGetTransitionsHandler(t *testing.T) {
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
			name: "Get transitions successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
			},
			serverResponse: `{
				"expand": "transitions",
				"transitions": [
					{
						"id": "21",
						"name": "In Progress",
						"to": {
							"self": "https://your-domain.atlassian.net/rest/api/3/status/3",
							"description": "This issue is being actively worked on at the moment by the assignee.",
							"iconUrl": "https://your-domain.atlassian.net/images/icons/statuses/inprogress.png",
							"name": "In Progress",
							"id": "3",
							"statusCategory": {
								"self": "https://your-domain.atlassian.net/rest/api/3/statuscategory/4",
								"id": 4,
								"key": "indeterminate",
								"colorName": "yellow",
								"name": "In Progress"
							}
						},
						"hasScreen": false,
						"isGlobal": true,
						"isInitial": false,
						"isAvailable": true,
						"isConditional": false,
						"fields": {}
					},
					{
						"id": "31",
						"name": "Done",
						"to": {
							"self": "https://your-domain.atlassian.net/rest/api/3/status/10001",
							"description": "A resolution has been taken, and it is awaiting verification by reporter.",
							"iconUrl": "https://your-domain.atlassian.net/images/icons/statuses/resolved.png",
							"name": "Done",
							"id": "10001",
							"statusCategory": {
								"self": "https://your-domain.atlassian.net/rest/api/3/statuscategory/3",
								"id": 3,
								"key": "done",
								"colorName": "green",
								"name": "Done"
							}
						},
						"hasScreen": false,
						"isGlobal": true,
						"isInitial": false,
						"isAvailable": true,
						"isConditional": false,
						"fields": {
							"resolution": {
								"required": true,
								"schema": {
									"type": "resolution",
									"system": "resolution"
								},
								"name": "Resolution",
								"key": "resolution",
								"hasDefaultValue": false,
								"operations": ["set"],
								"allowedValues": [
									{
										"self": "https://your-domain.atlassian.net/rest/api/3/resolution/1",
										"id": "1",
										"name": "Fixed",
										"description": "A fix for this issue is checked into the tree and tested."
									}
								]
							}
						}
					}
				]
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "/rest/api/3/issue/PROJ-123/transitions", u.Path)
			},
			validateResult: func(t *testing.T, result *ToolResult) {
				assert.True(t, result.Success)
				data := result.Data.(map[string]interface{})
				transitions := data["transitions"].([]interface{})
				assert.Len(t, transitions, 2)

				// Check metadata
				metadata := data["_metadata"].(map[string]interface{})
				assert.Equal(t, "v3", metadata["api_version"])
				assert.Equal(t, "get_transitions", metadata["operation"])
				assert.Equal(t, "PROJ-123", metadata["issue"])
				assert.Equal(t, 2, metadata["transition_count"])
			},
		},
		{
			name: "Get transitions with expand parameter",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"expand":       "transitions.fields",
			},
			serverResponse: `{
				"transitions": [
					{
						"id": "21",
						"name": "In Progress",
						"fields": {
							"assignee": {
								"required": false,
								"schema": {"type": "user"}
							}
						}
					}
				]
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "transitions.fields", u.Query().Get("expand"))
			},
		},
		{
			name: "Get specific transition by ID",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-789",
				"transitionId": "21",
			},
			serverResponse: `{
				"transitions": [
					{
						"id": "21",
						"name": "In Progress"
					}
				]
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "21", u.Query().Get("transitionId"))
			},
		},
		{
			name: "Get transitions with boolean parameters",
			params: map[string]interface{}{
				"issueIdOrKey":                  "PROJ-100",
				"skipRemoteOnlyCondition":       true,
				"includeUnavailableTransitions": true,
				"sortByOpsBarAndStatus":         true,
			},
			serverResponse: `{"transitions": []}`,
			statusCode:     http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "true", u.Query().Get("skipRemoteOnlyCondition"))
				assert.Equal(t, "true", u.Query().Get("includeUnavailableTransitions"))
				assert.Equal(t, "true", u.Query().Get("sortByOpsBarAndStatus"))
			},
		},
		{
			name: "Project filter allows access",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-200",
			},
			projectFilter:  "PROJ,OTHER",
			serverResponse: `{"transitions": []}`,
			statusCode:     http.StatusOK,
		},
		{
			name: "Project filter denies access",
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
				"expand": "transitions.fields",
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
			errorContains: "Failed to get transitions with status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/issue/")
				assert.Contains(t, r.URL.Path, "/transitions")

				// Custom URL validation
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

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewGetTransitionsHandler(provider)

			// Create context
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

func TestTransitionIssueHandler(t *testing.T) {
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
	}{
		{
			name: "Transition issue successfully",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"transitionId": "21",
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				transition := body["transition"].(map[string]interface{})
				assert.Equal(t, "21", transition["id"])
			},
		},
		{
			name: "Transition with fields",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"transitionId": "31",
				"fields": map[string]interface{}{
					"resolution": map[string]interface{}{
						"name": "Fixed",
					},
					"assignee": map[string]interface{}{
						"accountId": "557058:f58131cb-b67d-43c7-b30d-6b58d40bd077",
					},
				},
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				fields := body["fields"].(map[string]interface{})
				resolution := fields["resolution"].(map[string]interface{})
				assert.Equal(t, "Fixed", resolution["name"])
				assignee := fields["assignee"].(map[string]interface{})
				assert.Equal(t, "557058:f58131cb-b67d-43c7-b30d-6b58d40bd077", assignee["accountId"])
			},
		},
		{
			name: "Transition with update operations",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-789",
				"transitionId": "21",
				"update": map[string]interface{}{
					"labels": []interface{}{
						map[string]interface{}{
							"add": "bug-fix",
						},
					},
				},
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				update := body["update"].(map[string]interface{})
				labels := update["labels"].([]interface{})
				assert.Len(t, labels, 1)
			},
		},
		{
			name: "Transition with history metadata",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-100",
				"transitionId": "31",
				"historyMetadata": map[string]interface{}{
					"type":           "myplugin:type",
					"description":    "text description",
					"descriptionKey": "plugin.changereason.i18.key",
				},
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				historyMetadata := body["historyMetadata"].(map[string]interface{})
				assert.Equal(t, "myplugin:type", historyMetadata["type"])
			},
		},
		{
			name: "Read-only mode prevents transition",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-200",
				"transitionId": "21",
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "Cannot transition issue in read-only mode",
		},
		{
			name: "Project filter denies access",
			params: map[string]interface{}{
				"issueIdOrKey": "DENIED-100",
				"transitionId": "21",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Issue belongs to project DENIED which is not in allowed projects",
		},
		{
			name:          "Missing issueIdOrKey",
			params:        map[string]interface{}{"transitionId": "21"},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name:          "Missing transitionId",
			params:        map[string]interface{}{"issueIdOrKey": "PROJ-300"},
			expectError:   true,
			errorContains: "transitionId is required",
		},
		{
			name: "Invalid transition ID",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-400",
				"transitionId": "invalid<script>alert('xss')</script>",
			},
			expectError:   true,
			errorContains: "Invalid transitionId",
		},
		{
			name: "Server error response",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-999",
				"transitionId": "21",
			},
			serverResponse: `{
				"errorMessages": ["Transition not available"],
				"errors": {"transition": "Transition '21' is not available for the current issue status."}
			}`,
			statusCode:    http.StatusBadRequest,
			expectError:   true,
			errorContains: "Failed to transition issue with status 400",
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
				assert.Contains(t, r.URL.Path, "/transitions")
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Capture body
				if r.Body != nil {
					decoder := json.NewDecoder(r.Body)
					_ = decoder.Decode(&receivedBody)

					// Custom body validation
					if tt.validateBody != nil {
						tt.validateBody(t, receivedBody)
					}
				}

				// Return response
				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != "" {
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewTransitionIssueHandler(provider)

			// Create context
			ctx := context.Background()
			pctx := &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"email":     "test@example.com",
					"api_token": "test-token",
				},
			}

			// Add metadata
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
				assert.Contains(t, data["message"], "transitioned successfully")
			}
		})
	}
}

func TestGetWorkflowsHandler(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse string
		statusCode     int
		projectFilter  string
		expectError    bool
		errorContains  string
		validateURL    func(t *testing.T, u *url.URL)
	}{
		{
			name:   "Get all workflows",
			params: map[string]interface{}{},
			serverResponse: `{
				"values": [
					{
						"id": {
							"name": "classic default workflow",
							"draft": false
						},
						"description": "The default workflow that comes with Jira.",
						"transitions": []
					}
				],
				"maxResults": 50,
				"startAt": 0,
				"total": 1,
				"isLast": true
			}`,
			statusCode: http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "/rest/api/3/workflow/search", u.Path)
			},
		},
		{
			name: "Get workflows for specific project",
			params: map[string]interface{}{
				"projectKey": "PROJ",
			},
			serverResponse: `{"values": [], "total": 0}`,
			statusCode:     http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "PROJ", u.Query().Get("projectKeys"))
			},
		},
		{
			name: "Get specific workflow",
			params: map[string]interface{}{
				"workflowName": "My Custom Workflow",
			},
			serverResponse: `{"values": [], "total": 0}`,
			statusCode:     http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "My Custom Workflow", u.Query().Get("workflowNames"))
			},
		},
		{
			name: "Get workflows with expand",
			params: map[string]interface{}{
				"expand": "transitions.rules",
			},
			serverResponse: `{"values": [], "total": 0}`,
			statusCode:     http.StatusOK,
			validateURL: func(t *testing.T, u *url.URL) {
				assert.Equal(t, "transitions.rules", u.Query().Get("expand"))
			},
		},
		{
			name: "Project filter denies access",
			params: map[string]interface{}{
				"projectKey": "DENIED",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Project DENIED is not in allowed projects",
		},
		{
			name: "Server error",
			params: map[string]interface{}{
				"projectKey": "NONEXISTENT",
			},
			serverResponse: `{
				"errorMessages": ["No project could be found with key 'NONEXISTENT'"]
			}`,
			statusCode:    http.StatusNotFound,
			expectError:   true,
			errorContains: "Failed to get workflows with status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/rest/api/3/workflow/search")

				// Custom URL validation
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

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewGetWorkflowsHandler(provider)

			// Create context
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
				if !result.Success {
					t.Logf("Error: %s", result.Error)
				}
				assert.True(t, result.Success)
			}
		})
	}
}

func TestAddWorkflowCommentHandler(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]interface{}
		statusCode    int
		readOnly      bool
		projectFilter string
		expectError   bool
		errorContains string
		validateBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "Transition with plain text comment",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-123",
				"transitionId": "21",
				"comment":      "Moving to In Progress",
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				transition := body["transition"].(map[string]interface{})
				assert.Equal(t, "21", transition["id"])

				update := body["update"].(map[string]interface{})
				comment := update["comment"].([]interface{})[0].(map[string]interface{})
				add := comment["add"].(map[string]interface{})
				bodyADF := add["body"].(map[string]interface{})
				assert.Equal(t, "doc", bodyADF["type"])
			},
		},
		{
			name: "Transition with ADF comment",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-456",
				"transitionId": "31",
				"commentADF": map[string]interface{}{
					"type":    "doc",
					"version": 1,
					"content": []interface{}{
						map[string]interface{}{
							"type": "paragraph",
							"content": []interface{}{
								map[string]interface{}{
									"type": "text",
									"text": "Closing as completed",
									"marks": []interface{}{
										map[string]interface{}{"type": "strong"},
									},
								},
							},
						},
					},
				},
			},
			statusCode: http.StatusNoContent,
		},
		{
			name: "Transition with comment visibility",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-789",
				"transitionId": "21",
				"comment":      "Internal note",
				"commentVisibility": map[string]interface{}{
					"type":  "group",
					"value": "developers",
				},
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				update := body["update"].(map[string]interface{})
				comment := update["comment"].([]interface{})[0].(map[string]interface{})
				add := comment["add"].(map[string]interface{})
				visibility := add["visibility"].(map[string]interface{})
				assert.Equal(t, "group", visibility["type"])
				assert.Equal(t, "developers", visibility["value"])
			},
		},
		{
			name: "Transition with resolution shorthand",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-100",
				"transitionId": "31",
				"comment":      "Fixed the issue",
				"resolution":   "Fixed",
			},
			statusCode: http.StatusNoContent,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				fields := body["fields"].(map[string]interface{})
				resolution := fields["resolution"].(map[string]interface{})
				assert.Equal(t, "Fixed", resolution["name"])
			},
		},
		{
			name: "Read-only mode prevents transition",
			params: map[string]interface{}{
				"issueIdOrKey": "PROJ-200",
				"transitionId": "21",
				"comment":      "Should not work",
			},
			readOnly:      true,
			expectError:   true,
			errorContains: "Cannot transition issue in read-only mode",
		},
		{
			name: "Project filter denies access",
			params: map[string]interface{}{
				"issueIdOrKey": "DENIED-100",
				"transitionId": "21",
				"comment":      "Should be denied",
			},
			projectFilter: "PROJ,OTHER",
			expectError:   true,
			errorContains: "Issue belongs to project DENIED which is not in allowed projects",
		},
		{
			name:          "Missing issueIdOrKey",
			params:        map[string]interface{}{"transitionId": "21", "comment": "Test"},
			expectError:   true,
			errorContains: "issueIdOrKey is required",
		},
		{
			name:          "Missing transitionId",
			params:        map[string]interface{}{"issueIdOrKey": "PROJ-300", "comment": "Test"},
			expectError:   true,
			errorContains: "transitionId is required",
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
				assert.Contains(t, r.URL.Path, "/transitions")

				// Capture body
				if r.Body != nil {
					decoder := json.NewDecoder(r.Body)
					_ = decoder.Decode(&receivedBody)

					// Custom body validation
					if tt.validateBody != nil {
						tt.validateBody(t, receivedBody)
					}
				}

				// Return response
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Create provider and handler
			logger := &observability.NoopLogger{}
			provider := NewJiraProvider(logger, server.URL)
			handler := NewAddWorkflowCommentHandler(provider)

			// Create context
			ctx := context.Background()
			pctx := &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"email":     "test@example.com",
					"api_token": "test-token",
				},
			}

			// Add metadata
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
				assert.Contains(t, data["message"], "transitioned successfully with comment")
			}
		})
	}
}

func TestTransitionValidation(t *testing.T) {
	handler := &TransitionIssueHandler{}

	tests := []struct {
		name          string
		transitionId  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "Valid numeric transition ID",
			transitionId: "21",
			expectError:  false,
		},
		{
			name:         "Valid alphanumeric transition ID",
			transitionId: "transition-123",
			expectError:  false,
		},
		{
			name:          "Empty transition ID",
			transitionId:  "",
			expectError:   true,
			errorContains: "transition ID cannot be empty",
		},
		{
			name:          "Transition ID too long",
			transitionId:  strings.Repeat("a", 51),
			expectError:   true,
			errorContains: "transition ID too long",
		},
		{
			name:          "Transition ID with dangerous characters",
			transitionId:  "21<script>alert('xss')</script>",
			expectError:   true,
			errorContains: "transition ID contains invalid character",
		},
		{
			name:          "Transition ID with quotes",
			transitionId:  "21\"OR 1=1--",
			expectError:   true,
			errorContains: "transition ID contains invalid character",
		},
		{
			name:          "Transition ID with pipe",
			transitionId:  "21|rm -rf /",
			expectError:   true,
			errorContains: "transition ID contains invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateTransitionId(tt.transitionId)

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

func TestWorkflowHandlerDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-domain")

	t.Run("GetTransitionsHandler definition", func(t *testing.T) {
		handler := NewGetTransitionsHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "get_transitions", def.Name)
		assert.Equal(t, "Get available transitions for a Jira issue", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")

		// Check properties
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "issueIdOrKey")
		assert.Contains(t, properties, "expand")
		assert.Contains(t, properties, "transitionId")
		assert.Contains(t, properties, "skipRemoteOnlyCondition")
		assert.Contains(t, properties, "includeUnavailableTransitions")
		assert.Contains(t, properties, "sortByOpsBarAndStatus")
	})

	t.Run("TransitionIssueHandler definition", func(t *testing.T) {
		handler := NewTransitionIssueHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "transition_issue", def.Name)
		assert.Equal(t, "Execute a transition on a Jira issue", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")
		assert.Contains(t, required, "transitionId")

		// Check properties
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "issueIdOrKey")
		assert.Contains(t, properties, "transitionId")
		assert.Contains(t, properties, "fields")
		assert.Contains(t, properties, "update")
		assert.Contains(t, properties, "historyMetadata")
		assert.Contains(t, properties, "properties")

		// Check resolution field structure
		fields := properties["fields"].(map[string]interface{})
		fieldsProps := fields["properties"].(map[string]interface{})
		assert.Contains(t, fieldsProps, "resolution")
		assert.Contains(t, fieldsProps, "assignee")
	})

	t.Run("GetWorkflowsHandler definition", func(t *testing.T) {
		handler := NewGetWorkflowsHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "get_workflows", def.Name)
		assert.Equal(t, "Get workflows for projects (requires admin permissions)", def.Description)

		// Check properties
		schema := def.InputSchema
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "projectKey")
		assert.Contains(t, properties, "workflowName")
		assert.Contains(t, properties, "expand")

		// Should have no required fields
		required := schema["required"].([]interface{})
		assert.Len(t, required, 0)
	})

	t.Run("AddWorkflowCommentHandler definition", func(t *testing.T) {
		handler := NewAddWorkflowCommentHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "transition_with_comment", def.Name)
		assert.Equal(t, "Execute a transition on an issue with a comment", def.Description)

		// Check required fields
		schema := def.InputSchema
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "issueIdOrKey")
		assert.Contains(t, required, "transitionId")

		// Check properties
		properties := schema["properties"].(map[string]interface{})
		assert.Contains(t, properties, "comment")
		assert.Contains(t, properties, "commentADF")
		assert.Contains(t, properties, "commentVisibility")
		assert.Contains(t, properties, "fields")
		assert.Contains(t, properties, "resolution")

		// Check comment visibility enum
		commentVisibility := properties["commentVisibility"].(map[string]interface{})
		visibilityProps := commentVisibility["properties"].(map[string]interface{})
		typeProp := visibilityProps["type"].(map[string]interface{})
		typeEnum := typeProp["enum"].([]interface{})
		assert.Contains(t, typeEnum, "group")
		assert.Contains(t, typeEnum, "role")
	})
}
