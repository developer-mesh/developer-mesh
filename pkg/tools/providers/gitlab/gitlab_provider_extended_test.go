package gitlab

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

// TestGitLabProvider_ExtendedOperations tests all the new CRUD operations
func TestGitLabProvider_ExtendedOperations(t *testing.T) {
	tests := []struct {
		name           string
		operation      string
		params         map[string]interface{}
		expectedMethod string
		expectedPath   string
		mockResponse   interface{}
		expectedError  bool
	}{
		// Project operations
		{
			name:      "update project",
			operation: "projects/update",
			params: map[string]interface{}{
				"id":          "123",
				"description": "Updated description",
				"visibility":  "private",
			},
			expectedMethod: "PUT",
			expectedPath:   "/projects/123",
			mockResponse: map[string]interface{}{
				"id":          123,
				"description": "Updated description",
			},
		},
		{
			name:           "delete project",
			operation:      "projects/delete",
			params:         map[string]interface{}{"id": "123"},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123",
		},
		{
			name:      "fork project",
			operation: "projects/fork",
			params: map[string]interface{}{
				"id":        "123",
				"namespace": "my-namespace",
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/fork",
			mockResponse:   map[string]interface{}{"id": 456},
		},
		{
			name:           "star project",
			operation:      "projects/star",
			params:         map[string]interface{}{"id": "123"},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/star",
		},
		{
			name:           "archive project",
			operation:      "projects/archive",
			params:         map[string]interface{}{"id": "123"},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/archive",
		},

		// Issue operations
		{
			name:      "update issue",
			operation: "issues/update",
			params: map[string]interface{}{
				"id":          "123",
				"issue_iid":   "45",
				"title":       "Updated title",
				"state_event": "close",
			},
			expectedMethod: "PUT",
			expectedPath:   "/projects/123/issues/45",
			mockResponse: map[string]interface{}{
				"iid":   45,
				"title": "Updated title",
				"state": "closed",
			},
		},
		{
			name:           "delete issue",
			operation:      "issues/delete",
			params:         map[string]interface{}{"id": "123", "issue_iid": "45"},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123/issues/45",
		},

		// Merge Request operations
		{
			name:      "approve merge request",
			operation: "merge_requests/approve",
			params: map[string]interface{}{
				"id":                "123",
				"merge_request_iid": "67",
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/merge_requests/67/approve",
			mockResponse:   map[string]interface{}{"approved": true},
		},
		{
			name:      "merge merge request",
			operation: "merge_requests/merge",
			params: map[string]interface{}{
				"id":                          "123",
				"merge_request_iid":           "67",
				"merge_commit_message":        "Merging MR",
				"should_remove_source_branch": true,
			},
			expectedMethod: "PUT",
			expectedPath:   "/projects/123/merge_requests/67/merge",
			mockResponse:   map[string]interface{}{"state": "merged"},
		},
		{
			name:      "rebase merge request",
			operation: "merge_requests/rebase",
			params: map[string]interface{}{
				"id":                "123",
				"merge_request_iid": "67",
			},
			expectedMethod: "PUT",
			expectedPath:   "/projects/123/merge_requests/67/rebase",
			mockResponse:   map[string]interface{}{"rebase_in_progress": true},
		},

		// Pipeline operations
		{
			name:           "cancel pipeline",
			operation:      "pipelines/cancel",
			params:         map[string]interface{}{"id": "123", "pipeline_id": "789"},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/pipelines/789/cancel",
			mockResponse:   map[string]interface{}{"status": "canceled"},
		},
		{
			name:           "retry pipeline",
			operation:      "pipelines/retry",
			params:         map[string]interface{}{"id": "123", "pipeline_id": "789"},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/pipelines/789/retry",
			mockResponse:   map[string]interface{}{"status": "running"},
		},
		{
			name:           "delete pipeline",
			operation:      "pipelines/delete",
			params:         map[string]interface{}{"id": "123", "pipeline_id": "789"},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123/pipelines/789",
		},

		// Job operations
		{
			name:           "get job",
			operation:      "jobs/get",
			params:         map[string]interface{}{"id": "123", "job_id": "456"},
			expectedMethod: "GET",
			expectedPath:   "/projects/123/jobs/456",
			mockResponse:   map[string]interface{}{"id": 456, "status": "success"},
		},
		{
			name:           "cancel job",
			operation:      "jobs/cancel",
			params:         map[string]interface{}{"id": "123", "job_id": "456"},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/jobs/456/cancel",
			mockResponse:   map[string]interface{}{"status": "canceled"},
		},
		{
			name:           "play job",
			operation:      "jobs/play",
			params:         map[string]interface{}{"id": "123", "job_id": "456"},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/jobs/456/play",
			mockResponse:   map[string]interface{}{"status": "running"},
		},

		// File operations
		{
			name:      "create file",
			operation: "files/create",
			params: map[string]interface{}{
				"id":             "123",
				"file_path":      "README.md",
				"branch":         "main",
				"content":        "# Hello World",
				"commit_message": "Add README",
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/repository/files/README.md",
			mockResponse:   map[string]interface{}{"file_path": "README.md"},
		},
		{
			name:      "update file",
			operation: "files/update",
			params: map[string]interface{}{
				"id":             "123",
				"file_path":      "README.md",
				"branch":         "main",
				"content":        "# Updated",
				"commit_message": "Update README",
			},
			expectedMethod: "PUT",
			expectedPath:   "/projects/123/repository/files/README.md",
			mockResponse:   map[string]interface{}{"file_path": "README.md"},
		},
		{
			name:      "delete file",
			operation: "files/delete",
			params: map[string]interface{}{
				"id":             "123",
				"file_path":      "README.md",
				"branch":         "main",
				"commit_message": "Delete README",
			},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123/repository/files/README.md",
		},

		// Branch operations
		{
			name:      "create branch",
			operation: "branches/create",
			params: map[string]interface{}{
				"id":     "123",
				"branch": "feature/new",
				"ref":    "main",
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/repository/branches",
			mockResponse:   map[string]interface{}{"name": "feature/new"},
		},
		{
			name:           "delete branch",
			operation:      "branches/delete",
			params:         map[string]interface{}{"id": "123", "branch": "feature/old"},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123/repository/branches/feature/old",
		},
		{
			name:      "protect branch",
			operation: "branches/protect",
			params: map[string]interface{}{
				"id":                "123",
				"name":              "main",
				"push_access_level": 40,
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/protected_branches",
			mockResponse:   map[string]interface{}{"name": "main"},
		},

		// Tag operations
		{
			name:      "create tag",
			operation: "tags/create",
			params: map[string]interface{}{
				"id":       "123",
				"tag_name": "v1.0.0",
				"ref":      "main",
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/repository/tags",
			mockResponse:   map[string]interface{}{"name": "v1.0.0"},
		},
		{
			name:           "delete tag",
			operation:      "tags/delete",
			params:         map[string]interface{}{"id": "123", "tag_name": "v0.9.0"},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123/repository/tags/v0.9.0",
		},

		// Wiki operations
		{
			name:      "create wiki page",
			operation: "wikis/create",
			params: map[string]interface{}{
				"id":      "123",
				"title":   "Home",
				"content": "Welcome to wiki",
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/wikis",
			mockResponse:   map[string]interface{}{"slug": "home"},
		},
		{
			name:      "update wiki page",
			operation: "wikis/update",
			params: map[string]interface{}{
				"id":      "123",
				"slug":    "home",
				"content": "Updated content",
			},
			expectedMethod: "PUT",
			expectedPath:   "/projects/123/wikis/home",
			mockResponse:   map[string]interface{}{"slug": "home"},
		},

		// Member operations
		{
			name:      "add project member",
			operation: "members/add",
			params: map[string]interface{}{
				"id":           "123",
				"user_id":      "456",
				"access_level": 30,
			},
			expectedMethod: "POST",
			expectedPath:   "/projects/123/members",
			mockResponse:   map[string]interface{}{"id": 456, "access_level": 30},
		},
		{
			name:           "remove project member",
			operation:      "members/remove",
			params:         map[string]interface{}{"id": "123", "user_id": "456"},
			expectedMethod: "DELETE",
			expectedPath:   "/projects/123/members/456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify authentication header
				authHeader := r.Header.Get("Authorization")
				assert.Equal(t, "Bearer test-token", authHeader)

				// Verify method and path
				assert.Equal(t, tt.expectedMethod, r.Method)
				assert.Equal(t, tt.expectedPath, r.URL.Path)

				// Return mock response
				w.WriteHeader(http.StatusOK)
				if tt.mockResponse != nil {
					if err := json.NewEncoder(w).Encode(tt.mockResponse); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}
			}))
			defer server.Close()

			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewGitLabProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Create context with credentials
			ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			// Execute operation
			result, err := provider.ExecuteOperation(ctx, tt.operation, tt.params)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.mockResponse != nil {
					assert.NotNil(t, result)
				}
			}
		})
	}
}

// TestGitLabProvider_SpecialOperations tests operations that require special handling
func TestGitLabProvider_SpecialOperations(t *testing.T) {
	tests := []struct {
		name              string
		operation         string
		params            map[string]interface{}
		expectedOperation string
		expectedParams    map[string]interface{}
	}{
		{
			name:      "close issue",
			operation: "issues/close",
			params: map[string]interface{}{
				"id":        "123",
				"issue_iid": "45",
			},
			expectedOperation: "issues/update",
			expectedParams: map[string]interface{}{
				"id":          "123",
				"issue_iid":   "45",
				"state_event": "close",
			},
		},
		{
			name:      "reopen issue",
			operation: "issues/reopen",
			params: map[string]interface{}{
				"id":        "123",
				"issue_iid": "45",
			},
			expectedOperation: "issues/update",
			expectedParams: map[string]interface{}{
				"id":          "123",
				"issue_iid":   "45",
				"state_event": "reopen",
			},
		},
		{
			name:      "close merge request",
			operation: "merge_requests/close",
			params: map[string]interface{}{
				"id":                "123",
				"merge_request_iid": "67",
			},
			expectedOperation: "merge_requests/update",
			expectedParams: map[string]interface{}{
				"id":                "123",
				"merge_request_iid": "67",
				"state_event":       "close",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualOperation := ""
			actualParams := map[string]interface{}{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Capture the actual request
				switch tt.expectedOperation {
				case "issues/update":
					assert.Contains(t, r.URL.Path, "/issues/")
					assert.Equal(t, "PUT", r.Method)

					// Check that state_event was injected
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("Failed to decode request body: %v", err)
					}
					assert.Equal(t, tt.expectedParams["state_event"], body["state_event"])
				case "merge_requests/update":
					assert.Contains(t, r.URL.Path, "/merge_requests/")
					assert.Equal(t, "PUT", r.Method)

					// Check that state_event was injected
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("Failed to decode request body: %v", err)
					}
					assert.Equal(t, tt.expectedParams["state_event"], body["state_event"])
				}

				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": true}); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewGitLabProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			_, err := provider.ExecuteOperation(ctx, tt.operation, tt.params)
			assert.NoError(t, err)

			// The special operations should have been transformed
			_ = actualOperation
			_ = actualParams
		})
	}
}

// TestGitLabProvider_FilterOperationsByPermissions tests permission-based filtering
func TestGitLabProvider_ExtendedFilterOperationsByPermissions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	tests := []struct {
		name        string
		operations  []string
		permissions map[string]interface{}
		expected    []string
	}{
		{
			name: "guest access - read only",
			operations: []string{
				"projects/list", "projects/get", "projects/create", "projects/delete",
				"issues/list", "issues/create", "issues/update",
			},
			permissions: map[string]interface{}{
				"access_level": 10, // Guest
				"scopes":       []string{"read_api"},
			},
			expected: []string{
				"projects/list", "projects/get",
				"issues/list",
			},
		},
		{
			name: "reporter access - can create issues",
			operations: []string{
				"issues/list", "issues/create", "issues/update", "issues/delete",
				"merge_requests/create", "merge_requests/merge",
			},
			permissions: map[string]interface{}{
				"access_level": 20, // Reporter
				"scopes":       []string{"api"},
			},
			expected: []string{
				"issues/list", "issues/create", "issues/update",
				// merge_requests/create requires Developer access
			},
		},
		{
			name: "developer access - can manage code",
			operations: []string{
				"merge_requests/create", "merge_requests/merge", "merge_requests/approve",
				"branches/create", "branches/delete", "branches/protect",
				"files/create", "files/update", "files/delete",
			},
			permissions: map[string]interface{}{
				"access_level": 30, // Developer
				"scopes":       []string{"api", "write_repository"},
			},
			expected: []string{
				"merge_requests/create", "merge_requests/merge", "merge_requests/approve",
				"branches/create", // branches/delete requires Maintainer
				"files/create", "files/update", "files/delete",
			},
		},
		{
			name: "maintainer access - can protect branches",
			operations: []string{
				"branches/create", "branches/delete", "branches/protect", "branches/unprotect",
				"pipelines/delete", "jobs/erase",
				"projects/archive", "projects/delete",
			},
			permissions: map[string]interface{}{
				"access_level": 40, // Maintainer
				"scopes":       []string{"api"},
			},
			expected: []string{
				"branches/create", "branches/delete", "branches/protect", "branches/unprotect",
				"pipelines/delete", "jobs/erase",
				"projects/archive", // projects/delete requires Owner
			},
		},
		{
			name: "owner access - full control",
			operations: []string{
				"projects/delete", "projects/archive",
				"groups/update", "groups/delete",
			},
			permissions: map[string]interface{}{
				"access_level": 50, // Owner
				"scopes":       []string{"api"},
			},
			expected: []string{
				"projects/delete", "projects/archive",
				"groups/update", "groups/delete",
			},
		},
		{
			name: "api scope - write operations allowed",
			operations: []string{
				"projects/create", "issues/create", "merge_requests/create",
			},
			permissions: map[string]interface{}{
				"access_level": 30,
				"scopes":       []string{"api"},
			},
			expected: []string{
				"projects/create", "issues/create", "merge_requests/create",
			},
		},
		{
			name: "read_api scope - only read operations",
			operations: []string{
				"projects/list", "projects/get", "projects/create",
				"issues/list", "issues/get", "issues/create",
			},
			permissions: map[string]interface{}{
				"access_level": 50, // Even with Owner access
				"scopes":       []string{"read_api"},
			},
			expected: []string{
				"projects/list", "projects/get",
				"issues/list", "issues/get",
			},
		},
		{
			name: "write_repository scope - repository operations",
			operations: []string{
				"files/create", "files/update", "branches/create", "tags/create",
				"commits/create",
			},
			permissions: map[string]interface{}{
				"access_level": 30,
				"scopes":       []string{"write_repository"},
			},
			expected: []string{
				"files/create", "files/update", "branches/create", "tags/create",
				"commits/create",
			},
		},
		{
			name: "no permissions - read only fallback",
			operations: []string{
				"projects/list", "projects/create", "projects/delete",
				"issues/list", "issues/create",
			},
			permissions: nil,
			expected: []string{
				"projects/list",
				"issues/list",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := provider.FilterOperationsByPermissions(tt.operations, tt.permissions)
			assert.ElementsMatch(t, tt.expected, filtered)
		})
	}
}

// TestGitLabProvider_PassThroughAuthentication tests that credentials are properly passed through
func TestGitLabProvider_PassThroughAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]string
		operation   string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid personal access token",
			credentials: map[string]string{
				"personal_access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
			},
			operation: "projects/list",
		},
		{
			name: "valid api key",
			credentials: map[string]string{
				"api_key": "test-api-key",
			},
			operation: "issues/list",
		},
		{
			name: "valid token",
			credentials: map[string]string{
				"token": "test-token",
			},
			operation: "merge_requests/list",
		},
		{
			name:        "no credentials",
			credentials: map[string]string{},
			operation:   "projects/create",
			expectError: true,
			errorMsg:    "no credentials found in context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedAuth string
			authCaptured := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Capture the authorization header for non-validation requests
				if r.URL.Path != "/user" {
					capturedAuth = r.Header.Get("Authorization")
					authCaptured = true
					t.Logf("Request to %s with Authorization: %s", r.URL.Path, capturedAuth)
				}

				// For validation endpoint
				if r.URL.Path == "/user" {
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(map[string]interface{}{
						"id":       1,
						"username": "testuser",
					}); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
					return
				}

				// For other endpoints
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode([]interface{}{}); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewGitLabProvider(logger)
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			t.Logf("Config AuthType: %s", config.AuthType)
			provider.SetConfiguration(config)

			// First validate credentials if provided
			if len(tt.credentials) > 0 {
				err := provider.ValidateCredentials(context.Background(), tt.credentials)
				require.NoError(t, err)
			}

			// Create context with credentials
			var ctx context.Context
			if len(tt.credentials) > 0 {
				token := ""
				if t := tt.credentials["personal_access_token"]; t != "" {
					token = t
				} else if t := tt.credentials["api_key"]; t != "" {
					token = t
				} else if t := tt.credentials["token"]; t != "" {
					token = t
				}

				ctx = providers.WithContext(context.Background(), &providers.ProviderContext{
					Credentials: &providers.ProviderCredentials{
						Token:  token,
						APIKey: token, // Set both Token and APIKey for compatibility
					},
				})
			} else {
				ctx = context.Background()
			}

			// Execute operation
			// Provide required parameters for operations that need them
			params := map[string]interface{}{}
			if tt.operation == "issues/list" || tt.operation == "merge_requests/list" {
				params["id"] = "test-project"
			}
			// Debug: check context
			if pctx, ok := providers.FromContext(ctx); ok && pctx.Credentials != nil {
				t.Logf("Context has credentials: token=%v, apikey=%v", pctx.Credentials.Token != "", pctx.Credentials.APIKey != "")
			} else {
				t.Logf("Context missing credentials")
			}
			_, err := provider.ExecuteOperation(ctx, tt.operation, params)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				// Verify the token was passed through
				if authCaptured {
					assert.NotEmpty(t, capturedAuth, "Authorization header should not be empty")
					assert.Contains(t, capturedAuth, "Bearer", "Authorization header should contain Bearer")
				}
			}
		})
	}
}

// TestGitLabProvider_GetExtendedOperationMappings tests that all extended operations are registered
func TestGitLabProvider_GetExtendedOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	mappings := provider.GetOperationMappings()

	// Test that extended operations exist
	extendedOps := []string{
		// Projects
		"projects/update", "projects/delete", "projects/fork", "projects/star",
		"projects/unstar", "projects/archive", "projects/unarchive",
		// Issues
		"issues/update", "issues/delete", "issues/close", "issues/reopen",
		// Merge Requests
		"merge_requests/update", "merge_requests/approve", "merge_requests/unapprove",
		"merge_requests/merge", "merge_requests/close", "merge_requests/rebase",
		// Pipelines
		"pipelines/cancel", "pipelines/retry", "pipelines/delete",
		// Jobs
		"jobs/get", "jobs/cancel", "jobs/retry", "jobs/play", "jobs/artifacts", "jobs/erase",
		// Files
		"files/get", "files/raw", "files/create", "files/update", "files/delete",
		// Branches
		"branches/get", "branches/create", "branches/delete", "branches/protect", "branches/unprotect",
		// Tags
		"tags/get", "tags/create", "tags/delete",
		// Commits
		"commits/get", "commits/create", "commits/diff", "commits/comments", "commits/comment",
		// Groups
		"groups/create", "groups/update", "groups/delete",
		// Wikis
		"wikis/list", "wikis/get", "wikis/create", "wikis/update", "wikis/delete",
		// Snippets
		"snippets/list", "snippets/get", "snippets/create", "snippets/update", "snippets/delete",
		// Deployments
		"deployments/list", "deployments/get", "deployments/create", "deployments/update",
		// Members
		"members/list", "members/get", "members/add", "members/update", "members/remove",
	}

	for _, op := range extendedOps {
		t.Run(op, func(t *testing.T) {
			mapping, exists := mappings[op]
			assert.True(t, exists, "Operation %s should exist", op)
			assert.NotEmpty(t, mapping.Method, "Operation %s should have HTTP method", op)
			assert.NotEmpty(t, mapping.PathTemplate, "Operation %s should have path template", op)
			assert.NotEmpty(t, mapping.OperationID, "Operation %s should have operation ID", op)
		})
	}
}

// TestGitLabProvider_AccessLevelExtraction tests access level extraction from permissions
func TestGitLabProvider_AccessLevelExtraction(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	tests := []struct {
		name        string
		permissions map[string]interface{}
		expected    int
	}{
		{
			name:        "numeric access level",
			permissions: map[string]interface{}{"access_level": 30},
			expected:    30,
		},
		{
			name:        "float64 access level",
			permissions: map[string]interface{}{"access_level": float64(40)},
			expected:    40,
		},
		{
			name:        "string owner",
			permissions: map[string]interface{}{"access_level": "owner"},
			expected:    50,
		},
		{
			name:        "string maintainer",
			permissions: map[string]interface{}{"access_level": "maintainer"},
			expected:    40,
		},
		{
			name:        "string master (legacy)",
			permissions: map[string]interface{}{"access_level": "master"},
			expected:    40,
		},
		{
			name:        "string developer",
			permissions: map[string]interface{}{"access_level": "developer"},
			expected:    30,
		},
		{
			name:        "string reporter",
			permissions: map[string]interface{}{"access_level": "reporter"},
			expected:    20,
		},
		{
			name:        "string guest",
			permissions: map[string]interface{}{"access_level": "guest"},
			expected:    10,
		},
		{
			name:        "unknown string",
			permissions: map[string]interface{}{"access_level": "unknown"},
			expected:    0,
		},
		{
			name:        "no access level",
			permissions: map[string]interface{}{},
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := provider.extractAccessLevel(tt.permissions)
			assert.Equal(t, tt.expected, level)
		})
	}
}

// TestGitLabProvider_ScopeExtraction tests scope extraction from permissions
func TestGitLabProvider_ScopeExtraction(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	tests := []struct {
		name        string
		permissions map[string]interface{}
		expected    []string
	}{
		{
			name:        "string array scopes",
			permissions: map[string]interface{}{"scopes": []string{"api", "read_api"}},
			expected:    []string{"api", "read_api"},
		},
		{
			name:        "interface array scopes",
			permissions: map[string]interface{}{"scopes": []interface{}{"api", "write_repository"}},
			expected:    []string{"api", "write_repository"},
		},
		{
			name:        "single string scope",
			permissions: map[string]interface{}{"scopes": "api"},
			expected:    []string{"api"},
		},
		{
			name:        "no scopes",
			permissions: map[string]interface{}{},
			expected:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopes := provider.extractScopes(tt.permissions)
			assert.ElementsMatch(t, tt.expected, scopes)
		})
	}
}

// BenchmarkGitLabProvider_FilterOperations benchmarks permission filtering
func BenchmarkGitLabProvider_FilterOperations(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	operations := []string{
		"projects/list", "projects/get", "projects/create", "projects/update", "projects/delete",
		"issues/list", "issues/get", "issues/create", "issues/update", "issues/delete",
		"merge_requests/list", "merge_requests/get", "merge_requests/create", "merge_requests/merge",
		"files/get", "files/create", "files/update", "files/delete",
		"branches/list", "branches/create", "branches/protect",
	}

	permissions := map[string]interface{}{
		"access_level": 30,
		"scopes":       []string{"api", "write_repository"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.FilterOperationsByPermissions(operations, permissions)
	}
}

// BenchmarkGitLabProvider_GetExtendedMappings benchmarks extended mapping retrieval
func BenchmarkGitLabProvider_GetExtendedMappings(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetOperationMappings()
	}
}
