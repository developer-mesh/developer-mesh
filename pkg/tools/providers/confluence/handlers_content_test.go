package confluence

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

func TestCreatePageHandler_Execute(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse map[string]interface{}
		serverStatus   int
		providerCtx    *providers.ProviderContext
		expectedError  bool
		errorContains  string
		validateReq    func(*testing.T, *http.Request, map[string]interface{})
	}{
		{
			name: "successful page creation",
			params: map[string]interface{}{
				"title":    "Test Page",
				"spaceKey": "TESTSPACE",
				"content":  "<p>Test content</p>",
				"type":     "page",
			},
			serverResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Test Page",
				"space": map[string]interface{}{
					"key": "TESTSPACE",
				},
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request, body map[string]interface{}) {
				assert.Equal(t, "/wiki/rest/api/content", req.URL.Path)
				assert.Equal(t, "POST", req.Method)
				assert.Equal(t, "Test Page", body["title"])
				assert.Equal(t, "page", body["type"])

				space, ok := body["space"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "TESTSPACE", space["key"])

				bodyContent, ok := body["body"].(map[string]interface{})
				assert.True(t, ok)
				storage, ok := bodyContent["storage"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "<p>Test content</p>", storage["value"])
				assert.Equal(t, "storage", storage["representation"])
			},
		},
		{
			name: "page creation with parent",
			params: map[string]interface{}{
				"title":    "Child Page",
				"spaceKey": "TESTSPACE",
				"content":  "<p>Child content</p>",
				"parentId": "99999",
			},
			serverResponse: map[string]interface{}{
				"id":    "12346",
				"title": "Child Page",
			},
			serverStatus:  http.StatusCreated,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request, body map[string]interface{}) {
				ancestorsRaw, ok := body["ancestors"]
				assert.True(t, ok, "ancestors field should exist")
				ancestors, ok := ancestorsRaw.([]interface{})
				assert.True(t, ok, "ancestors should be an array")
				assert.Len(t, ancestors, 1)
				if len(ancestors) > 0 {
					ancestor, ok := ancestors[0].(map[string]interface{})
					assert.True(t, ok, "ancestor should be a map")
					assert.Equal(t, "99999", ancestor["id"])
				}
			},
		},
		{
			name: "read-only mode prevents creation",
			params: map[string]interface{}{
				"title":    "Test Page",
				"spaceKey": "TESTSPACE",
				"content":  "<p>Test content</p>",
			},
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"READ_ONLY": true,
				},
			},
			serverStatus:  0, // Server should not be called for read-only check
			expectedError: true,
			errorContains: "Cannot create pages in read-only mode",
		},
		{
			name: "missing title",
			params: map[string]interface{}{
				"spaceKey": "TESTSPACE",
				"content":  "<p>Test content</p>",
			},
			expectedError: true,
			errorContains: "title is required",
		},
		{
			name: "missing spaceKey",
			params: map[string]interface{}{
				"title":   "Test Page",
				"content": "<p>Test content</p>",
			},
			expectedError: true,
			errorContains: "spaceKey is required",
		},
		{
			name: "missing content",
			params: map[string]interface{}{
				"title":    "Test Page",
				"spaceKey": "TESTSPACE",
			},
			expectedError: true,
			errorContains: "content is required",
		},
		{
			name: "space not allowed",
			params: map[string]interface{}{
				"title":    "Test Page",
				"spaceKey": "FORBIDDEN",
				"content":  "<p>Test content</p>",
			},
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "ALLOWED,TESTSPACE",
				},
			},
			expectedError: true,
			errorContains: "Space FORBIDDEN is not in allowed spaces",
		},
		{
			name: "server error response",
			params: map[string]interface{}{
				"title":    "Test Page",
				"spaceKey": "TESTSPACE",
				"content":  "<p>Test content</p>",
			},
			serverResponse: map[string]interface{}{
				"message": "Internal server error",
			},
			serverStatus:  http.StatusInternalServerError,
			expectedError: true,
			errorContains: "Failed to create page: status 500 - Internal server error",
		},
		{
			name: "blog post creation",
			params: map[string]interface{}{
				"title":    "Test Blog Post",
				"spaceKey": "TESTSPACE",
				"content":  "<p>Blog content</p>",
				"type":     "blogpost",
			},
			serverResponse: map[string]interface{}{
				"id":    "12347",
				"title": "Test Blog Post",
				"type":  "blogpost",
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request, body map[string]interface{}) {
				assert.Equal(t, "blogpost", body["type"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate authentication
				auth := r.Header.Get("Authorization")
				assert.NotEmpty(t, auth)
				assert.Contains(t, auth, "Basic")

				// Validate headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Parse request body
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)

				// Run custom validation if provided
				if tt.validateReq != nil {
					tt.validateReq(t, r, body)
				}

				// Send response
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				if tt.serverResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create provider using test server URL
			provider := NewConfluenceProvider(&observability.NoopLogger{}, server.URL)
			provider.httpClient = &http.Client{}

			// Create handler
			handler := NewCreatePageHandler(provider)

			// Create context
			ctx := context.Background()
			if tt.providerCtx != nil {
				ctx = providers.WithContext(ctx, tt.providerCtx)
			}

			// Add auth to params
			if tt.params["email"] == nil {
				tt.params["email"] = "test@example.com"
			}
			if tt.params["api_token"] == nil {
				tt.params["api_token"] = "test-token"
			}

			// Execute
			result, err := handler.Execute(ctx, tt.params)

			// Assert
			if tt.expectedError {
				if result != nil && !result.Success {
					assert.False(t, result.Success)
					if tt.errorContains != "" {
						assert.Contains(t, result.Error, tt.errorContains)
					}
				} else {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected result but got nil")
				}
				if !result.Success {
					t.Fatalf("Expected success but got error: %s", result.Error)
				}
				assert.NotNil(t, result.Data)

				// Check metadata was added
				data := result.Data.(map[string]interface{})
				metadata, ok := data["_metadata"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "v1", metadata["api_version"])
				assert.Equal(t, "create_page", metadata["operation"])
			}
		})
	}
}

func TestUpdatePageHandler_Execute(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		getResponse    map[string]interface{}
		getStatus      int
		updateResponse map[string]interface{}
		updateStatus   int
		providerCtx    *providers.ProviderContext
		expectedError  bool
		errorContains  string
		validateReq    func(*testing.T, *http.Request, map[string]interface{})
	}{
		{
			name: "successful page update",
			params: map[string]interface{}{
				"pageId":  "12345",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			getResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Existing Page",
				"type":  "page",
				"space": map[string]interface{}{
					"key": "TESTSPACE",
				},
				"version": map[string]interface{}{
					"number": float64(2),
				},
			},
			getStatus: http.StatusOK,
			updateResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Existing Page",
				"version": map[string]interface{}{
					"number": float64(3),
				},
			},
			updateStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request, body map[string]interface{}) {
				if req.Method == "PUT" {
					assert.Equal(t, "12345", body["id"])
					assert.Equal(t, "Existing Page", body["title"])

					version, ok := body["version"].(map[string]interface{})
					assert.True(t, ok)
					assert.Equal(t, float64(3), version["number"]) // Should be incremented
					assert.Equal(t, false, version["minorEdit"])

					bodyContent, ok := body["body"].(map[string]interface{})
					assert.True(t, ok)
					storage, ok := bodyContent["storage"].(map[string]interface{})
					assert.True(t, ok)
					assert.Equal(t, "<p>Updated content</p>", storage["value"])
				}
			},
		},
		{
			name: "update with title change",
			params: map[string]interface{}{
				"pageId":  "12345",
				"title":   "New Title",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			getResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Old Title",
				"type":  "page",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			getStatus: http.StatusOK,
			updateResponse: map[string]interface{}{
				"id":    "12345",
				"title": "New Title",
			},
			updateStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request, body map[string]interface{}) {
				if req.Method == "PUT" {
					assert.Equal(t, "New Title", body["title"])
				}
			},
		},
		{
			name: "minor edit",
			params: map[string]interface{}{
				"pageId":    "12345",
				"content":   "<p>Minor change</p>",
				"version":   float64(2),
				"minorEdit": true,
			},
			getResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Page",
				"type":  "page",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			getStatus: http.StatusOK,
			updateResponse: map[string]interface{}{
				"id": "12345",
			},
			updateStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request, body map[string]interface{}) {
				if req.Method == "PUT" {
					version, ok := body["version"].(map[string]interface{})
					assert.True(t, ok)
					assert.Equal(t, true, version["minorEdit"])
				}
			},
		},
		{
			name: "version conflict",
			params: map[string]interface{}{
				"pageId":  "12345",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			getResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Page",
				"type":  "page",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			getStatus:    http.StatusOK,
			updateStatus: http.StatusConflict,
			updateResponse: map[string]interface{}{
				"message": "Version conflict",
			},
			expectedError: true,
			errorContains: "Version conflict - page was modified by another user",
		},
		{
			name: "page not found",
			params: map[string]interface{}{
				"pageId":  "99999",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			getStatus:     http.StatusNotFound,
			expectedError: true,
			errorContains: "Page 99999 not found",
		},
		{
			name: "no permission",
			params: map[string]interface{}{
				"pageId":  "12345",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			getStatus:     http.StatusForbidden,
			expectedError: true,
			errorContains: "No permission to update page 12345",
		},
		{
			name: "read-only mode",
			params: map[string]interface{}{
				"pageId":  "12345",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"READ_ONLY": true,
				},
			},
			expectedError: true,
			errorContains: "Cannot update pages in read-only mode",
		},
		{
			name: "space not allowed",
			params: map[string]interface{}{
				"pageId":  "12345",
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			getResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Page",
				"type":  "page",
				"space": map[string]interface{}{
					"key": "FORBIDDEN",
				},
			},
			getStatus: http.StatusOK,
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "ALLOWED,TESTSPACE",
				},
			},
			expectedError: true,
			errorContains: "Page 12345 is in space FORBIDDEN which is not allowed",
		},
		{
			name: "missing pageId",
			params: map[string]interface{}{
				"content": "<p>Updated content</p>",
				"version": float64(2),
			},
			expectedError: true,
			errorContains: "pageId is required",
		},
		{
			name: "missing content",
			params: map[string]interface{}{
				"pageId":  "12345",
				"version": float64(2),
			},
			expectedError: true,
			errorContains: "content is required",
		},
		{
			name: "missing version",
			params: map[string]interface{}{
				"pageId":  "12345",
				"content": "<p>Updated content</p>",
			},
			expectedError: true,
			errorContains: "version is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++

				// First call is GET to check page
				if callCount == 1 {
					assert.Equal(t, "GET", r.Method)
					assert.Contains(t, r.URL.Path, "/wiki/rest/api/content/")
					assert.Contains(t, r.URL.Query().Get("expand"), "space")
					assert.Contains(t, r.URL.Query().Get("expand"), "version")

					w.WriteHeader(tt.getStatus)
					if tt.getResponse != nil {
						_ = json.NewEncoder(w).Encode(tt.getResponse)
					}
				} else {
					// Second call is PUT to update
					assert.Equal(t, "PUT", r.Method)

					// Parse request body
					var body map[string]interface{}
					_ = json.NewDecoder(r.Body).Decode(&body)

					// Run custom validation
					if tt.validateReq != nil {
						tt.validateReq(t, r, body)
					}

					w.WriteHeader(tt.updateStatus)
					if tt.updateResponse != nil {
						_ = json.NewEncoder(w).Encode(tt.updateResponse)
					}
				}
			}))
			defer server.Close()

			// Create provider using test server URL
			provider := NewConfluenceProvider(&observability.NoopLogger{}, server.URL)
			provider.httpClient = &http.Client{}

			// Create handler
			handler := NewUpdatePageHandler(provider)

			// Create context
			ctx := context.Background()
			if tt.providerCtx != nil {
				ctx = providers.WithContext(ctx, tt.providerCtx)
			}

			// Add auth to params
			if tt.params["email"] == nil {
				tt.params["email"] = "test@example.com"
			}
			if tt.params["api_token"] == nil {
				tt.params["api_token"] = "test-token"
			}

			// Execute
			result, err := handler.Execute(ctx, tt.params)

			// Assert
			if tt.expectedError {
				if result != nil && !result.Success {
					assert.False(t, result.Success)
					if tt.errorContains != "" {
						assert.Contains(t, result.Error, tt.errorContains)
					}
				} else {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected result but got nil")
				}
				if !result.Success {
					t.Fatalf("Expected success but got error: %s", result.Error)
				}
				assert.NotNil(t, result.Data)

				// Check metadata was added
				data := result.Data.(map[string]interface{})
				metadata, ok := data["_metadata"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "v1", metadata["api_version"])
				assert.Equal(t, "update_page", metadata["operation"])
			}
		})
	}
}

func TestListSpacesHandler_Execute(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		serverResponse map[string]interface{}
		serverStatus   int
		providerCtx    *providers.ProviderContext
		expectedError  bool
		errorContains  string
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name:   "list all spaces",
			params: map[string]interface{}{},
			serverResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"key":  "SPACE1",
						"name": "Space 1",
					},
					map[string]interface{}{
						"key":  "SPACE2",
						"name": "Space 2",
					},
				},
				"size": float64(2),
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "/wiki/rest/api/space", req.URL.Path)
				assert.Equal(t, "current", req.URL.Query().Get("status"))
				assert.Equal(t, "0", req.URL.Query().Get("start"))
				assert.Equal(t, "25", req.URL.Query().Get("limit"))
			},
		},
		{
			name: "filter by type",
			params: map[string]interface{}{
				"type": "global",
			},
			serverResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"key":  "GLOBAL1",
						"name": "Global Space",
						"type": "global",
					},
				},
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "global", req.URL.Query().Get("type"))
			},
		},
		{
			name: "filter by status",
			params: map[string]interface{}{
				"status": "archived",
			},
			serverResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"key":    "ARCHIVED1",
						"name":   "Archived Space",
						"status": "archived",
					},
				},
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "archived", req.URL.Query().Get("status"))
			},
		},
		{
			name: "with pagination",
			params: map[string]interface{}{
				"start": float64(10),
				"limit": float64(50),
			},
			serverResponse: map[string]interface{}{
				"results": []interface{}{},
				"start":   float64(10),
				"limit":   float64(50),
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "10", req.URL.Query().Get("start"))
				assert.Equal(t, "50", req.URL.Query().Get("limit"))
			},
		},
		{
			name: "limit validation - too low",
			params: map[string]interface{}{
				"limit": float64(0),
			},
			serverResponse: map[string]interface{}{
				"results": []interface{}{},
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "1", req.URL.Query().Get("limit"))
			},
		},
		{
			name: "limit validation - too high",
			params: map[string]interface{}{
				"limit": float64(150),
			},
			serverResponse: map[string]interface{}{
				"results": []interface{}{},
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "100", req.URL.Query().Get("limit"))
			},
		},
		{
			name: "start validation - negative",
			params: map[string]interface{}{
				"start": float64(-10),
			},
			serverResponse: map[string]interface{}{
				"results": []interface{}{},
			},
			serverStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "0", req.URL.Query().Get("start"))
			},
		},
		{
			name:   "space filtering applied",
			params: map[string]interface{}{},
			serverResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"key":  "ALLOWED",
						"name": "Allowed Space",
					},
					map[string]interface{}{
						"key":  "FORBIDDEN",
						"name": "Forbidden Space",
					},
				},
			},
			serverStatus: http.StatusOK,
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "ALLOWED",
				},
			},
			expectedError: false,
		},
		{
			name:   "server error",
			params: map[string]interface{}{},
			serverResponse: map[string]interface{}{
				"message": "Internal server error",
			},
			serverStatus:  http.StatusInternalServerError,
			expectedError: true,
			errorContains: "Failed to list spaces: status 500 - Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				if tt.validateReq != nil {
					tt.validateReq(t, r)
				}

				// Validate headers
				assert.Contains(t, r.Header.Get("Authorization"), "Basic")
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Send response
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				if tt.serverResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create provider using test server URL
			provider := NewConfluenceProvider(&observability.NoopLogger{}, server.URL)
			provider.httpClient = &http.Client{}

			// Create handler
			handler := NewListSpacesHandler(provider)

			// Create context
			ctx := context.Background()
			if tt.providerCtx != nil {
				ctx = providers.WithContext(ctx, tt.providerCtx)
			}

			// Add auth to params
			if tt.params["email"] == nil {
				tt.params["email"] = "test@example.com"
			}
			if tt.params["api_token"] == nil {
				tt.params["api_token"] = "test-token"
			}

			// Execute
			result, err := handler.Execute(ctx, tt.params)

			// Assert
			if tt.expectedError {
				if result != nil && !result.Success {
					assert.False(t, result.Success)
					if tt.errorContains != "" {
						assert.Contains(t, result.Error, tt.errorContains)
					}
				} else {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected result but got nil")
				}
				if !result.Success {
					t.Fatalf("Expected success but got error: %s", result.Error)
				}
				assert.NotNil(t, result.Data)

				// Check metadata was added
				data := result.Data.(map[string]interface{})
				metadata, ok := data["_metadata"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "v1", metadata["api_version"])
				assert.Equal(t, "list_spaces", metadata["operation"])
			}
		})
	}
}

func TestGetAttachmentsHandler_Execute(t *testing.T) {
	tests := []struct {
		name           string
		params         map[string]interface{}
		pageResponse   map[string]interface{}
		pageStatus     int
		attachResponse map[string]interface{}
		attachStatus   int
		providerCtx    *providers.ProviderContext
		expectedError  bool
		errorContains  string
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "get all attachments",
			params: map[string]interface{}{
				"pageId": "12345",
			},
			pageResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Test Page",
				"space": map[string]interface{}{
					"key": "TESTSPACE",
				},
			},
			pageStatus: http.StatusOK,
			attachResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":       "att1",
						"title":    "attachment1.pdf",
						"fileSize": float64(1024),
					},
					map[string]interface{}{
						"id":       "att2",
						"title":    "image.png",
						"fileSize": float64(2048),
					},
				},
				"size": float64(2),
			},
			attachStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.URL.Path == "/wiki/rest/api/content/12345/child/attachment" {
					assert.Equal(t, "0", req.URL.Query().Get("start"))
					assert.Equal(t, "25", req.URL.Query().Get("limit"))
				}
			},
		},
		{
			name: "filter by filename",
			params: map[string]interface{}{
				"pageId":   "12345",
				"filename": "document.pdf",
			},
			pageResponse: map[string]interface{}{
				"id":    "12345",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			pageStatus: http.StatusOK,
			attachResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "att1",
						"title": "document.pdf",
					},
				},
			},
			attachStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.URL.Path == "/wiki/rest/api/content/12345/child/attachment" {
					assert.Equal(t, "document.pdf", req.URL.Query().Get("filename"))
				}
			},
		},
		{
			name: "filter by media type",
			params: map[string]interface{}{
				"pageId":    "12345",
				"mediaType": "image/png",
			},
			pageResponse: map[string]interface{}{
				"id":    "12345",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			pageStatus: http.StatusOK,
			attachResponse: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":        "att1",
						"title":     "image.png",
						"mediaType": "image/png",
					},
				},
			},
			attachStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.URL.Path == "/wiki/rest/api/content/12345/child/attachment" {
					assert.Equal(t, "image/png", req.URL.Query().Get("mediaType"))
				}
			},
		},
		{
			name: "with pagination",
			params: map[string]interface{}{
				"pageId": "12345",
				"start":  float64(5),
				"limit":  float64(10),
			},
			pageResponse: map[string]interface{}{
				"id":    "12345",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			pageStatus: http.StatusOK,
			attachResponse: map[string]interface{}{
				"results": []interface{}{},
				"start":   float64(5),
				"limit":   float64(10),
			},
			attachStatus:  http.StatusOK,
			expectedError: false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.URL.Path == "/wiki/rest/api/content/12345/child/attachment" {
					assert.Equal(t, "5", req.URL.Query().Get("start"))
					assert.Equal(t, "10", req.URL.Query().Get("limit"))
				}
			},
		},
		{
			name: "page not found",
			params: map[string]interface{}{
				"pageId": "99999",
			},
			pageStatus:    http.StatusNotFound,
			expectedError: true,
			errorContains: "Page 99999 not found",
		},
		{
			name: "no permission to access page",
			params: map[string]interface{}{
				"pageId": "12345",
			},
			pageStatus:    http.StatusForbidden,
			expectedError: true,
			errorContains: "No permission to access page 12345",
		},
		{
			name: "page in forbidden space",
			params: map[string]interface{}{
				"pageId": "12345",
			},
			pageResponse: map[string]interface{}{
				"id":    "12345",
				"title": "Test Page",
				"space": map[string]interface{}{
					"key": "FORBIDDEN",
				},
			},
			pageStatus: http.StatusOK,
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "ALLOWED,TESTSPACE",
				},
			},
			expectedError: true,
			errorContains: "Page 12345 is not in an allowed space",
		},
		{
			name:          "missing pageId",
			params:        map[string]interface{}{},
			expectedError: true,
			errorContains: "pageId is required",
		},
		{
			name: "attachment request fails",
			params: map[string]interface{}{
				"pageId": "12345",
			},
			pageResponse: map[string]interface{}{
				"id":    "12345",
				"space": map[string]interface{}{"key": "TESTSPACE"},
			},
			pageStatus: http.StatusOK,
			attachResponse: map[string]interface{}{
				"message": "Failed to retrieve attachments",
			},
			attachStatus:  http.StatusInternalServerError,
			expectedError: true,
			errorContains: "Failed to get attachments: status 500 - Failed to retrieve attachments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++

				// First call is GET to check page
				if callCount == 1 {
					assert.Equal(t, "GET", r.Method)
					assert.Contains(t, r.URL.Path, "/wiki/rest/api/content/")
					assert.NotContains(t, r.URL.Path, "/child/attachment")

					w.WriteHeader(tt.pageStatus)
					if tt.pageResponse != nil {
						if err := json.NewEncoder(w).Encode(tt.pageResponse); err != nil {
							t.Logf("Failed to encode page response: %v", err)
						}
					}
				} else {
					// Second call is GET for attachments
					assert.Equal(t, "GET", r.Method)
					assert.Contains(t, r.URL.Path, "/child/attachment")

					// Run custom validation
					if tt.validateReq != nil {
						tt.validateReq(t, r)
					}

					w.WriteHeader(tt.attachStatus)
					if tt.attachResponse != nil {
						if err := json.NewEncoder(w).Encode(tt.attachResponse); err != nil {
							t.Logf("Failed to encode attach response: %v", err)
						}
					}
				}
			}))
			defer server.Close()

			// Create provider using test server URL
			provider := NewConfluenceProvider(&observability.NoopLogger{}, server.URL)
			provider.httpClient = &http.Client{}

			// Create handler
			handler := NewGetAttachmentsHandler(provider)

			// Create context
			ctx := context.Background()
			if tt.providerCtx != nil {
				ctx = providers.WithContext(ctx, tt.providerCtx)
			}

			// Add auth to params
			if tt.params["email"] == nil {
				tt.params["email"] = "test@example.com"
			}
			if tt.params["api_token"] == nil {
				tt.params["api_token"] = "test-token"
			}

			// Execute
			result, err := handler.Execute(ctx, tt.params)

			// Assert
			if tt.expectedError {
				if result != nil && !result.Success {
					assert.False(t, result.Success)
					if tt.errorContains != "" {
						assert.Contains(t, result.Error, tt.errorContains)
					}
				} else {
					assert.Error(t, err)
					if tt.errorContains != "" {
						assert.Contains(t, err.Error(), tt.errorContains)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected result but got nil")
				}
				if !result.Success {
					t.Fatalf("Expected success but got error: %s", result.Error)
				}
				assert.NotNil(t, result.Data)

				// Check metadata was added
				data := result.Data.(map[string]interface{})
				metadata, ok := data["_metadata"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "v1", metadata["api_version"])
				assert.Equal(t, "get_attachments", metadata["operation"])
				assert.Equal(t, "12345", metadata["pageId"])
			}
		})
	}
}

func TestIsSpaceAllowed(t *testing.T) {
	tests := []struct {
		name        string
		spaceKey    string
		providerCtx *providers.ProviderContext
		expected    bool
	}{
		{
			name:     "no context - allow all",
			spaceKey: "ANYSPACE",
			expected: true,
		},
		{
			name:        "no filter configured - allow all",
			spaceKey:    "ANYSPACE",
			providerCtx: &providers.ProviderContext{},
			expected:    true,
		},
		{
			name:     "empty filter - allow all",
			spaceKey: "ANYSPACE",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "",
				},
			},
			expected: true,
		},
		{
			name:     "wildcard filter - allow all",
			spaceKey: "ANYSPACE",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "*",
				},
			},
			expected: true,
		},
		{
			name:     "exact match allowed",
			spaceKey: "ALLOWED",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "ALLOWED",
				},
			},
			expected: true,
		},
		{
			name:     "in list allowed",
			spaceKey: "SPACE2",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "SPACE1,SPACE2,SPACE3",
				},
			},
			expected: true,
		},
		{
			name:     "not in list - forbidden",
			spaceKey: "FORBIDDEN",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "ALLOWED,TESTSPACE",
				},
			},
			expected: false,
		},
		{
			name:     "wildcard prefix match",
			spaceKey: "PROJ123",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "PROJ*",
				},
			},
			expected: true,
		},
		{
			name:     "wildcard prefix no match",
			spaceKey: "TEAM123",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "PROJ*",
				},
			},
			expected: false,
		},
		{
			name:     "mixed wildcards and exact",
			spaceKey: "PROJ456",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": "EXACT,PROJ*,TEAM*",
				},
			},
			expected: true,
		},
		{
			name:     "spaces with trimming",
			spaceKey: "SPACE2",
			providerCtx: &providers.ProviderContext{
				Metadata: map[string]interface{}{
					"CONFLUENCE_SPACES_FILTER": " SPACE1 , SPACE2 , SPACE3 ",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &ConfluenceProvider{}

			ctx := context.Background()
			if tt.providerCtx != nil {
				ctx = providers.WithContext(ctx, tt.providerCtx)
			}

			result := provider.IsSpaceAllowed(ctx, tt.spaceKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}
