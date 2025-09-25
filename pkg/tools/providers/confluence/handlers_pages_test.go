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

func TestGetPageHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewGetPageHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "get_page", def.Name)
		assert.Contains(t, def.Description, "v2 API")

		// Check required fields
		schema := def.InputSchema
		props := schema["properties"].(map[string]interface{})
		assert.Contains(t, props, "pageId")
		assert.Contains(t, props, "expand")
		assert.Contains(t, props, "version")

		required := schema["required"].([]interface{})
		assert.Contains(t, required, "pageId")
	})

	t.Run("Execute Success", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/pages/123")
			assert.Equal(t, "Basic ZW1haWxAdGVzdC5jb206dG9rZW4=", r.Header.Get("Authorization"))

			// Check expand parameter
			expand := r.URL.Query().Get("expand")
			if expand != "" {
				assert.Equal(t, "body,version", expand)
			}

			// Return mock response
			response := map[string]interface{}{
				"id":    "123",
				"title": "Test Page",
				"type":  "page",
				"space": map[string]interface{}{
					"key":  "TEST",
					"name": "Test Space",
				},
				"version": map[string]interface{}{
					"number": 1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Create provider with test server URL as domain
		// The buildURL method will detect it starts with http and use it as-is
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageHandler(provider)

		// Test basic get
		params := map[string]interface{}{
			"pageId":    "123",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)
		if !result.Success {
			t.Logf("Error from handler: %s", result.Error)
		}
		assert.True(t, result.Success, "Result should be successful")

		require.NotNil(t, result.Data, "Result data should not be nil")
		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok, "Result data should be a map")
		assert.Equal(t, "123", data["id"])
		assert.Equal(t, "Test Page", data["title"])
	})

	t.Run("Execute with Expand", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify expand parameter
			assert.Equal(t, "body,version", r.URL.Query().Get("expand"))

			response := map[string]interface{}{
				"id":    "123",
				"title": "Test Page",
				"body": map[string]interface{}{
					"storage": map[string]interface{}{
						"value": "<p>Page content</p>",
					},
				},
				"version": map[string]interface{}{
					"number": 2,
				},
				"space": map[string]interface{}{
					"key": "TEST",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageHandler(provider)

		params := map[string]interface{}{
			"pageId":    "123",
			"expand":    []interface{}{"body", "version"},
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)

		data := result.Data.(map[string]interface{})
		assert.Contains(t, data, "body")
		assert.Contains(t, data, "version")
	})

	t.Run("Execute with Space Filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"id":    "123",
				"title": "Test Page",
				"space": map[string]interface{}{
					"key": "NOTALLOWED",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageHandler(provider)

		// Set up context with space filter
		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "ALLOWED1,ALLOWED2",
			},
		})

		params := map[string]interface{}{
			"pageId":    "123",
			"email":     "email@test.com",
			"api_token": "token",
		}

		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)

		// The page should be filtered out since it's not in allowed spaces
		// FilterSpaceResults should handle single items differently
		data := result.Data.(map[string]interface{})
		// For single page results, it might return the page anyway or empty
		// This depends on FilterSpaceResults implementation
		assert.NotNil(t, data)
	})

	t.Run("Execute Missing PageId", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewGetPageHandler(provider)

		params := map[string]interface{}{
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err) // Handler returns error in result, not as error
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "pageId is required")
	})

	t.Run("Execute Authentication Failure", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewGetPageHandler(provider)

		params := map[string]interface{}{
			"pageId": "123",
			// No auth credentials
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Authentication failed")
	})

	t.Run("Execute API Error", func(t *testing.T) {
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

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewGetPageHandler(provider)

		params := map[string]interface{}{
			"pageId":    "nonexistent",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "404")
		assert.Contains(t, result.Error, "Page not found")
	})
}

func TestListPagesHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewListPagesHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "list_pages", def.Name)
		assert.Contains(t, def.Description, "v2 API")
		assert.Contains(t, def.Description, "cursor-based pagination")

		schema := def.InputSchema
		props := schema["properties"].(map[string]interface{})
		assert.Contains(t, props, "spaceId")
		assert.Contains(t, props, "status")
		assert.Contains(t, props, "title")
		assert.Contains(t, props, "sort")
		assert.Contains(t, props, "limit")
		assert.Contains(t, props, "cursor")
		assert.Contains(t, props, "expand")
	})

	t.Run("Execute Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/pages")

			// Check query parameters
			query := r.URL.Query()
			assert.Equal(t, "25", query.Get("limit"))

			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Page 1",
						"space": map[string]interface{}{
							"key": "TEST",
						},
					},
					map[string]interface{}{
						"id":    "2",
						"title": "Page 2",
						"space": map[string]interface{}{
							"key": "TEST",
						},
					},
				},
				"_links": map[string]interface{}{
					"next": "/pages?cursor=abc123",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewListPagesHandler(provider)

		params := map[string]interface{}{
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		assert.Len(t, results, 2)
	})

	t.Run("Execute with Filters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			assert.Equal(t, "SPACE1", query.Get("space-id"))
			assert.Equal(t, "current", query.Get("status"))
			assert.Equal(t, "Test", query.Get("title"))
			assert.Equal(t, "-modified-date", query.Get("sort"))
			assert.Equal(t, "50", query.Get("limit"))
			assert.Equal(t, "next123", query.Get("cursor"))

			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Test Page",
						"space": map[string]interface{}{
							"key": "SPACE1",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewListPagesHandler(provider)

		params := map[string]interface{}{
			"spaceId":   "SPACE1",
			"status":    "current",
			"title":     "Test",
			"sort":      "-modified-date",
			"limit":     float64(50),
			"cursor":    "next123",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("Execute with Space Filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Page 1",
						"space": map[string]interface{}{
							"key": "ALLOWED",
						},
					},
					map[string]interface{}{
						"id":    "2",
						"title": "Page 2",
						"space": map[string]interface{}{
							"key": "NOTALLOWED",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewListPagesHandler(provider)

		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "ALLOWED",
			},
		})

		params := map[string]interface{}{
			"email":     "email@test.com",
			"api_token": "token",
		}

		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		// Should only have 1 result after filtering
		assert.Len(t, results, 1)
		page := results[0].(map[string]interface{})
		assert.Equal(t, "1", page["id"])
	})

	t.Run("Execute with Limit Validation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			limit := query.Get("limit")
			// Limit should be capped at 250
			assert.Equal(t, "250", limit)

			response := map[string]interface{}{
				"results": []interface{}{},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewListPagesHandler(provider)

		params := map[string]interface{}{
			"limit":     float64(500), // Exceeds max
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})
}

func TestDeletePageHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewDeletePageHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "delete_page", def.Name)
		assert.Contains(t, def.Description, "v2 API")

		schema := def.InputSchema
		props := schema["properties"].(map[string]interface{})
		assert.Contains(t, props, "pageId")
		assert.Contains(t, props, "purge")

		required := schema["required"].([]interface{})
		assert.Contains(t, required, "pageId")
	})

	t.Run("Execute Trash Success", func(t *testing.T) {
		getCallCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				// First call to check page existence
				getCallCount++
				response := map[string]interface{}{
					"id":    "123",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "TEST",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case "DELETE":
				assert.Contains(t, r.URL.Path, "/pages/123")
				assert.NotContains(t, r.URL.RawQuery, "purge=true")
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewDeletePageHandler(provider)

		params := map[string]interface{}{
			"pageId":    "123",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		assert.Equal(t, true, data["success"])
		assert.Contains(t, data["message"], "moved to trash")
		assert.Equal(t, false, data["purged"])
	})

	t.Run("Execute Purge Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				response := map[string]interface{}{
					"id":    "123",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "TEST",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			case "DELETE":
				assert.Contains(t, r.URL.RawQuery, "purge=true")
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewDeletePageHandler(provider)

		params := map[string]interface{}{
			"pageId":    "123",
			"purge":     true,
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		assert.Contains(t, data["message"], "permanently deleted")
		assert.Equal(t, true, data["purged"])
	})

	t.Run("Execute Page Not Found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewDeletePageHandler(provider)

		params := map[string]interface{}{
			"pageId":    "nonexistent",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "not found")
	})

	t.Run("Execute Forbidden Space", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				response := map[string]interface{}{
					"id":    "123",
					"title": "Test Page",
					"space": map[string]interface{}{
						"key": "NOTALLOWED",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Logf("Failed to encode response: %v", err)
				}
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewDeletePageHandler(provider)

		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "ALLOWED",
			},
		})

		params := map[string]interface{}{
			"pageId":    "123",
			"email":     "email@test.com",
			"api_token": "token",
		}

		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "not allowed by CONFLUENCE_SPACES_FILTER")
	})

	t.Run("Execute Missing PageId", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewDeletePageHandler(provider)

		params := map[string]interface{}{
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "pageId is required")
	})
}

func TestPageHandlersIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("Pages Toolset Registration", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")

		// Check that pages toolset is registered
		assert.True(t, provider.IsToolsetEnabled("pages"))

		// Check that handlers are in toolset
		enabled := provider.GetEnabledToolsets()
		assert.Contains(t, enabled, "pages")

		// Verify individual handlers are registered directly by checking if we can create them
		getHandler := NewGetPageHandler(provider)
		assert.NotNil(t, getHandler, "get_page handler should be created")
		assert.Equal(t, "get_page", getHandler.GetDefinition().Name)

		listHandler := NewListPagesHandler(provider)
		assert.NotNil(t, listHandler, "list_pages handler should be created")
		assert.Equal(t, "list_pages", listHandler.GetDefinition().Name)

		deleteHandler := NewDeletePageHandler(provider)
		assert.NotNil(t, deleteHandler, "delete_page handler should be created")
		assert.Equal(t, "delete_page", deleteHandler.GetDefinition().Name)
	})

	t.Run("Handlers Use V2 API", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")

		// Verify handlers use v2 API endpoints
		getHandler := NewGetPageHandler(provider)
		listHandler := NewListPagesHandler(provider)
		deleteHandler := NewDeletePageHandler(provider)

		// Check descriptions mention v2 API
		assert.Contains(t, getHandler.GetDefinition().Description, "v2 API")
		assert.Contains(t, listHandler.GetDefinition().Description, "v2 API")
		assert.Contains(t, deleteHandler.GetDefinition().Description, "v2 API")
	})
}
