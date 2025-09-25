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
	"github.com/stretchr/testify/require"
)

func TestGetPageLabelsHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		handler := NewGetPageLabelsHandler(nil)
		def := handler.GetDefinition()

		assert.Equal(t, "get_page_labels", def.Name)
		assert.Contains(t, def.Description, "labels")
		assert.Contains(t, def.Description, "page")

		// Check required parameters
		required := def.InputSchema["required"].([]interface{})
		assert.Contains(t, required, "pageId")

		// Check properties
		props := def.InputSchema["properties"].(map[string]interface{})
		assert.Contains(t, props, "pageId")
		assert.Contains(t, props, "prefix")
		assert.Contains(t, props, "sort")
		assert.Contains(t, props, "limit")
		assert.Contains(t, props, "cursor")
	})

	t.Run("Execute Basic Get Labels", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// First request: page permission check
				assert.Equal(t, "/pages/12345", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "TEST",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Second request: get labels
				assert.Equal(t, "/pages/12345/labels", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				response := map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{
							"prefix": "global",
							"name":   "important",
							"id":     "label1",
						},
						map[string]interface{}{
							"prefix": "global",
							"name":   "documentation",
							"id":     "label2",
						},
					},
					"size": 2,
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		assert.Len(t, results, 2)

		// Check metadata
		assert.Contains(t, data, "_metadata")
		metadata := data["_metadata"].(map[string]interface{})
		assert.Equal(t, "12345", metadata["pageId"])
		assert.Equal(t, 50, metadata["limit"])
	})

	t.Run("Execute with Filters", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// First request: page permission check
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "TEST",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Second request: get labels with filters
				query := r.URL.Query()
				assert.Equal(t, "my", query.Get("prefix"))
				assert.Equal(t, "-created-date", query.Get("sort"))
				assert.Equal(t, "25", query.Get("limit"))
				assert.Equal(t, "cursor123", query.Get("cursor"))

				response := map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{
							"prefix": "my",
							"name":   "personal-label",
							"id":     "label1",
						},
					},
					"size": 1,
					"_links": map[string]interface{}{
						"next": "/pages/12345/labels?cursor=next123",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"prefix":    "my",
			"sort":      "-created-date",
			"limit":     float64(25),
			"cursor":    "cursor123",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		assert.Len(t, results, 1)

		// Check metadata
		metadata := data["_metadata"].(map[string]interface{})
		assert.Equal(t, "my", metadata["prefix"])
		assert.Equal(t, 25, metadata["limit"])
	})

	t.Run("Execute Page Not Found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			response := map[string]interface{}{
				"message": "Page not found",
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId":    "nonexistent",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Page nonexistent not found")
	})

	t.Run("Execute No Permission", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			response := map[string]interface{}{
				"message": "Forbidden",
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "No permission to access page")
	})

	t.Run("Execute Space Filter", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			if hitCount == 1 {
				// First request: page permission check
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "BLOCKED",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		// Create context with space filter
		ctx := context.Background()
		pctx := &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "ALLOWED",
			},
		}
		ctx = providers.WithContext(ctx, pctx)

		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "not in an allowed space")
	})

	t.Run("Missing PageId", func(t *testing.T) {
		handler := NewGetPageLabelsHandler(nil)

		params := map[string]interface{}{
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "pageId is required")
	})

	t.Run("Authentication Failure", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId": "12345",
			// Missing authentication
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Authentication failed")
	})

	t.Run("Limit Validation", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// Page check
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Labels request
				query := r.URL.Query()
				// Should be capped at 200
				assert.Equal(t, "200", query.Get("limit"))

				response := map[string]interface{}{"results": []interface{}{}}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageLabelsHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"limit":     float64(500), // Over max
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		metadata := data["_metadata"].(map[string]interface{})
		assert.Equal(t, 200, metadata["limit"])
	})
}

func TestAddLabelHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		handler := NewAddLabelHandler(nil)
		def := handler.GetDefinition()

		assert.Equal(t, "add_page_label", def.Name)
		assert.Contains(t, def.Description, "Add")
		assert.Contains(t, def.Description, "label")

		// Check required parameters
		required := def.InputSchema["required"].([]interface{})
		assert.Contains(t, required, "pageId")
		assert.Contains(t, required, "label")

		// Check properties
		props := def.InputSchema["properties"].(map[string]interface{})
		assert.Contains(t, props, "pageId")
		assert.Contains(t, props, "label")
		assert.Contains(t, props, "prefix")
	})

	t.Run("Execute Add Label", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// First request: page permission check
				assert.Equal(t, "/wiki/rest/api/content/12345", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "TEST",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Second request: add label
				assert.Equal(t, "/wiki/rest/api/content/12345/label", r.URL.Path)
				assert.Equal(t, "POST", r.Method)

				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "global", body["prefix"])
				assert.Equal(t, "important", body["name"])

				response := map[string]interface{}{
					"prefix": "global",
					"name":   "important",
					"id":     "label123",
				}
				w.WriteHeader(http.StatusCreated)
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewAddLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "important",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		assert.Equal(t, "important", data["name"])
		assert.Contains(t, data, "_operation")

		operation := data["_operation"].(map[string]interface{})
		assert.Equal(t, "add_label", operation["action"])
		assert.Equal(t, "12345", operation["pageId"])
		assert.Equal(t, "important", operation["label"])
	})

	t.Run("Execute with Custom Prefix", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// Page check - v1 API
				assert.Equal(t, "/wiki/rest/api/content/12345", r.URL.Path)
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Add label - v1 API
				assert.Equal(t, "/wiki/rest/api/content/12345/label", r.URL.Path)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "team", body["prefix"])
				assert.Equal(t, "project-x", body["name"])

				response := map[string]interface{}{
					"prefix": "team",
					"name":   "project-x",
					"id":     "label456",
				}
				w.WriteHeader(http.StatusCreated)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewAddLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "project-x",
			"prefix":    "team",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		operation := data["_operation"].(map[string]interface{})
		assert.Equal(t, "team", operation["prefix"])
	})

	t.Run("Read Only Mode", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewAddLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "test",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		// Create context with read-only mode
		ctx := context.Background()
		pctx := &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"READ_ONLY": true,
			},
		}
		ctx = providers.WithContext(ctx, pctx)

		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "read-only mode")
	})

	t.Run("Label Validation", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewAddLabelHandler(provider)

		testCases := []struct {
			name     string
			label    interface{}
			expected string
		}{
			{
				name:     "Empty label",
				label:    "",
				expected: "label is required",
			},
			{
				name:     "Whitespace only",
				label:    "   ",
				expected: "label cannot be empty",
			},
			{
				name:     "Too long",
				label:    string(make([]byte, 256)),
				expected: "cannot exceed 255 characters",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				params := map[string]interface{}{
					"pageId":    "12345",
					"label":     tc.label,
					"email":     "test@example.com",
					"api_token": "token123",
				}

				ctx := context.Background()
				result, err := handler.Execute(ctx, params)
				require.NoError(t, err)
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.expected)
			})
		}
	})

	t.Run("Missing Parameters", func(t *testing.T) {
		handler := NewAddLabelHandler(nil)

		testCases := []struct {
			name   string
			params map[string]interface{}
			error  string
		}{
			{
				name: "Missing pageId",
				params: map[string]interface{}{
					"label":     "test",
					"email":     "test@example.com",
					"api_token": "token123",
				},
				error: "pageId is required",
			},
			{
				name: "Missing label",
				params: map[string]interface{}{
					"pageId":    "12345",
					"email":     "test@example.com",
					"api_token": "token123",
				},
				error: "label is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				result, err := handler.Execute(ctx, tc.params)
				require.NoError(t, err)
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.error)
			})
		}
	})
}

func TestRemoveLabelHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		handler := NewRemoveLabelHandler(nil)
		def := handler.GetDefinition()

		assert.Equal(t, "remove_page_label", def.Name)
		assert.Contains(t, def.Description, "Remove")
		assert.Contains(t, def.Description, "label")

		// Check required parameters
		required := def.InputSchema["required"].([]interface{})
		assert.Contains(t, required, "pageId")
		assert.Contains(t, required, "label")
	})

	t.Run("Execute Remove Label", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// First request: page permission check
				assert.Equal(t, "/wiki/rest/api/content/12345", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "TEST",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Second request: remove label
				assert.Equal(t, "/wiki/rest/api/content/12345/label/outdated", r.URL.Path)
				assert.Equal(t, "DELETE", r.Method)

				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewRemoveLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "outdated",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		assert.True(t, data["success"].(bool))
		assert.Contains(t, data["message"].(string), "removed")

		operation := data["_operation"].(map[string]interface{})
		assert.Equal(t, "remove_label", operation["action"])
		assert.Equal(t, "12345", operation["pageId"])
		assert.Equal(t, "outdated", operation["label"])
	})

	t.Run("Execute Label with Special Characters", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// Page check
				assert.Equal(t, "/wiki/rest/api/content/12345", r.URL.Path)
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Remove label - URL encoding handled by server
				// The server decodes %26 to & in the path
				assert.Equal(t, "/wiki/rest/api/content/12345/label/test+&+label", r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewRemoveLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "test & label",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("Label Not Found", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// Page check
				assert.Equal(t, "/wiki/rest/api/content/12345", r.URL.Path)
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// Label not found
				assert.Equal(t, "/wiki/rest/api/content/12345/label/nonexistent", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				response := map[string]interface{}{
					"message": "Label not found",
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewRemoveLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "nonexistent",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Label 'nonexistent' not found")
	})

	t.Run("Read Only Mode", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewRemoveLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "test",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		// Create context with read-only mode
		ctx := context.Background()
		pctx := &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"READ_ONLY": true,
			},
		}
		ctx = providers.WithContext(ctx, pctx)

		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "read-only mode")
	})

	t.Run("No Permission", func(t *testing.T) {
		hitCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hitCount++
			switch hitCount {
			case 1:
				// Page check
				assert.Equal(t, "/wiki/rest/api/content/12345", r.URL.Path)
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case 2:
				// No permission to remove
				assert.Equal(t, "/wiki/rest/api/content/12345/label/protected", r.URL.Path)
				w.WriteHeader(http.StatusForbidden)
				response := map[string]interface{}{
					"message": "No permission",
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewRemoveLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "protected",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "No permission to remove label")
	})

	t.Run("Space Filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Page in blocked space
			response := map[string]interface{}{
				"id":    "12345",
				"title": "Test Page",
				"space": map[string]interface{}{
					"key": "BLOCKED",
				},
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewRemoveLabelHandler(provider)

		params := map[string]interface{}{
			"pageId":    "12345",
			"label":     "test",
			"email":     "test@example.com",
			"api_token": "token123",
		}

		// Create context with space filter
		ctx := context.Background()
		pctx := &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "ALLOWED",
			},
		}
		ctx = providers.WithContext(ctx, pctx)

		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "not in an allowed space")
	})
}

func TestLabelHandlersIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("Full Label Lifecycle", func(t *testing.T) {
		labelAdded := false
		labelRemoved := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle different endpoints
			switch {
			case r.URL.Path == "/pages/12345":
				// Page permission check for GetPageLabelsHandler
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}

			case r.URL.Path == "/wiki/rest/api/content/12345":
				// Page permission check for Add/RemoveLabelHandler
				response := map[string]interface{}{
					"id":    "12345",
					"title": "Test Page",
					"space": map[string]interface{}{"key": "TEST"},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}

			case r.URL.Path == "/pages/12345/labels" && r.Method == "GET":
				// Get labels
				labels := []interface{}{}
				if labelAdded && !labelRemoved {
					labels = append(labels, map[string]interface{}{
						"prefix": "global",
						"name":   "test-label",
						"id":     "label123",
					})
				}
				response := map[string]interface{}{
					"results": labels,
					"size":    len(labels),
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}

			case r.URL.Path == "/wiki/rest/api/content/12345/label" && r.Method == "POST":
				// Add label
				labelAdded = true
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				response := map[string]interface{}{
					"prefix": body["prefix"],
					"name":   body["name"],
					"id":     "label123",
				}
				w.WriteHeader(http.StatusCreated)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}

			case r.URL.Path == "/wiki/rest/api/content/12345/label/test-label" && r.Method == "DELETE":
				// Remove label
				if labelAdded {
					labelRemoved = true
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		ctx := context.Background()

		// Step 1: Get initial labels (should be empty)
		getHandler := NewGetPageLabelsHandler(provider)
		result, err := getHandler.Execute(ctx, map[string]interface{}{
			"pageId":    "12345",
			"email":     "test@example.com",
			"api_token": "token123",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		assert.Len(t, results, 0)

		// Step 2: Add a label
		addHandler := NewAddLabelHandler(provider)
		result, err = addHandler.Execute(ctx, map[string]interface{}{
			"pageId":    "12345",
			"label":     "test-label",
			"email":     "test@example.com",
			"api_token": "token123",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)

		// Step 3: Get labels again (should have one)
		result, err = getHandler.Execute(ctx, map[string]interface{}{
			"pageId":    "12345",
			"email":     "test@example.com",
			"api_token": "token123",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
		data = result.Data.(map[string]interface{})
		results = data["results"].([]interface{})
		assert.Len(t, results, 1)

		// Step 4: Remove the label
		removeHandler := NewRemoveLabelHandler(provider)
		result, err = removeHandler.Execute(ctx, map[string]interface{}{
			"pageId":    "12345",
			"label":     "test-label",
			"email":     "test@example.com",
			"api_token": "token123",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)

		// Step 5: Get labels again (should be empty)
		result, err = getHandler.Execute(ctx, map[string]interface{}{
			"pageId":    "12345",
			"email":     "test@example.com",
			"api_token": "token123",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
		data = result.Data.(map[string]interface{})
		results = data["results"].([]interface{})
		assert.Len(t, results, 0)
	})
}
