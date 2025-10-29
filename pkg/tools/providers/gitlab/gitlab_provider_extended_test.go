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

// BenchmarkGitLabProvider_GetExtendedMappings benchmarks extended mapping retrieval
func BenchmarkGitLabProvider_GetExtendedMappings(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewGitLabProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetOperationMappings()
	}
}
