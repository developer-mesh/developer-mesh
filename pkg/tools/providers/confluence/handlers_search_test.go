package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchContentHandler(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("GetDefinition", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewSearchContentHandler(provider)
		def := handler.GetDefinition()

		assert.Equal(t, "search_content", def.Name)
		assert.Contains(t, def.Description, "CQL")
		assert.Contains(t, def.Description, "v1 API")

		// Check required fields
		schema := def.InputSchema
		props := schema["properties"].(map[string]interface{})
		assert.Contains(t, props, "cql")
		assert.Contains(t, props, "cqlcontext")
		assert.Contains(t, props, "start")
		assert.Contains(t, props, "limit")
		assert.Contains(t, props, "expand")
		assert.Contains(t, props, "excerpt")
		assert.Contains(t, props, "includeArchivedSpaces")

		required := schema["required"].([]interface{})
		assert.Contains(t, required, "cql")
	})

	t.Run("Execute Basic Search", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/wiki/rest/api/content/search")
			assert.Equal(t, "Basic ZW1haWxAdGVzdC5jb206dG9rZW4=", r.Header.Get("Authorization"))

			// Check query parameters
			query := r.URL.Query()
			assert.Equal(t, "space = DEV AND type = page", query.Get("cql"))
			assert.Equal(t, "0", query.Get("start"))
			assert.Equal(t, "25", query.Get("limit"))
			assert.Equal(t, "highlight", query.Get("excerpt"))

			// Return mock response
			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Page 1",
						"type":  "page",
						"space": map[string]interface{}{
							"key":  "DEV",
							"name": "Development",
						},
					},
					map[string]interface{}{
						"id":    "2",
						"title": "Page 2",
						"type":  "page",
						"space": map[string]interface{}{
							"key": "DEV",
						},
					},
				},
				"start": 0,
				"limit": 25,
				"size":  2,
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		// Use test server URL as domain for testing
		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql":       "space = DEV AND type = page",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success, "Result should be successful")

		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		assert.Len(t, results, 2)

		// Check that query metadata was added
		queryMeta := data["_query"].(map[string]interface{})
		assert.Equal(t, "space = DEV AND type = page", queryMeta["cql"])
		assert.Equal(t, 0, queryMeta["start"])
		assert.Equal(t, 25, queryMeta["limit"])
	})

	t.Run("Execute with Pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			assert.Equal(t, "10", query.Get("start"))
			assert.Equal(t, "50", query.Get("limit"))

			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "11",
						"title": "Page 11",
						"space": map[string]interface{}{
							"key": "TEST",
						},
					},
				},
				"start": 10,
				"limit": 50,
				"size":  50, // Indicates there might be more
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql":       "type = page",
			"start":     float64(10),
			"limit":     float64(50),
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		// Check pagination links
		links := data["_links"].(map[string]interface{})
		assert.Contains(t, links, "self")
		assert.Contains(t, links, "next") // Should have next since size == limit
		assert.Contains(t, links, "prev") // Should have prev since start > 0
	})

	t.Run("Execute with Expand", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			assert.Equal(t, "space,history,body.view", query.Get("expand"))

			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Page with Body",
						"space": map[string]interface{}{
							"key":  "TEST",
							"name": "Test Space",
						},
						"body": map[string]interface{}{
							"view": map[string]interface{}{
								"value": "<p>Page content</p>",
							},
						},
						"history": map[string]interface{}{
							"createdDate": "2024-01-01T00:00:00.000Z",
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

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql":       "type = page",
			"expand":    []interface{}{"space", "history", "body.view"},
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)

		data := result.Data.(map[string]interface{})
		results := data["results"].([]interface{})
		page := results[0].(map[string]interface{})
		assert.Contains(t, page, "body")
		assert.Contains(t, page, "history")
	})

	t.Run("Execute with Space Filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Page in ALLOWED",
						"space": map[string]interface{}{
							"key": "ALLOWED",
						},
					},
					map[string]interface{}{
						"id":    "2",
						"title": "Page in NOTALLOWED",
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

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "ALLOWED",
			},
		})

		params := map[string]interface{}{
			"cql":       "type = page",
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
		assert.Equal(t, "Page in ALLOWED", page["title"])
	})

	t.Run("Execute with Archived Spaces", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			assert.Equal(t, "true", query.Get("includeArchivedSpaces"))

			response := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"id":    "1",
						"title": "Page in Archived Space",
						"space": map[string]interface{}{
							"key":    "ARCHIVED",
							"status": "archived",
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

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql":                   "type = page",
			"includeArchivedSpaces": true,
			"email":                 "email@test.com",
			"api_token":             "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("CQL Validation", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewSearchContentHandler(provider)

		testCases := []struct {
			name        string
			cql         string
			shouldError bool
			errorMsg    string
		}{
			{
				name:        "Empty CQL",
				cql:         "",
				shouldError: true,
				errorMsg:    "cql query is required",
			},
			{
				name:        "Valid CQL",
				cql:         "space = DEV AND type = page",
				shouldError: false,
			},
			{
				name:        "Unbalanced single quotes",
				cql:         "title ~ 'test",
				shouldError: true,
				errorMsg:    "unbalanced single quotes",
			},
			{
				name:        "Unbalanced double quotes",
				cql:         `title ~ "test`,
				shouldError: true,
				errorMsg:    "unbalanced double quotes",
			},
			{
				name:        "Unbalanced parentheses",
				cql:         "(space = DEV AND (type = page)",
				shouldError: true,
				errorMsg:    "unbalanced parentheses",
			},
			{
				name:        "SQL injection pattern",
				cql:         "space = DEV; DROP TABLE--;",
				shouldError: true,
				errorMsg:    "dangerous pattern",
			},
			{
				name:        "Complex valid query",
				cql:         `(space = "DEV" OR space = "TEST") AND type = page AND title ~ "search term"`,
				shouldError: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				params := map[string]interface{}{
					"cql":       tc.cql,
					"email":     "email@test.com",
					"api_token": "token",
				}

				ctx := context.Background()
				result, err := handler.Execute(ctx, params)
				require.NoError(t, err)

				if tc.shouldError {
					assert.False(t, result.Success)
					assert.Contains(t, result.Error, tc.errorMsg)
				} else {
					// Would succeed if there was a real server
					// Here it will fail with connection error but not validation error
					if !result.Success {
						assert.NotContains(t, result.Error, "Invalid CQL")
					}
				}
			})
		}
	})

	t.Run("Execute Missing CQL", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "cql query is required")
	})

	t.Run("Execute Authentication Failure", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql": "space = DEV",
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
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]interface{}{
				"message": "Invalid CQL syntax: unexpected token at position 10",
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql":       "invalid cql syntax",
			"email":     "email@test.com",
			"api_token": "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		assert.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "CQL query error")
		assert.Contains(t, result.Error, "Invalid CQL syntax")
	})

	t.Run("Execute with CQL Context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			assert.Equal(t, `{"spaceKey":"DEV"}`, query.Get("cqlcontext"))

			response := map[string]interface{}{
				"results": []interface{}{},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		params := map[string]interface{}{
			"cql":        "type = page",
			"cqlcontext": `{"spaceKey":"DEV"}`,
			"email":      "email@test.com",
			"api_token":  "token",
		}

		ctx := context.Background()
		result, err := handler.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("Execute with Different Excerpt Types", func(t *testing.T) {
		testCases := []struct {
			excerpt         string
			expectedExcerpt string
		}{
			{"highlight", "highlight"},
			{"indexed", "indexed"},
			{"none", "none"},
			{"", "highlight"}, // Default
		}

		for _, tc := range testCases {
			t.Run(tc.excerpt, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					query := r.URL.Query()
					assert.Equal(t, tc.expectedExcerpt, query.Get("excerpt"))

					response := map[string]interface{}{
						"results": []interface{}{},
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Logf("Failed to encode response: %v", err)
					}
				}))
				defer server.Close()

				provider := NewConfluenceProvider(logger, server.URL)
				handler := NewSearchContentHandler(provider)

				params := map[string]interface{}{
					"cql":       "type = page",
					"email":     "email@test.com",
					"api_token": "token",
				}
				if tc.excerpt != "" {
					params["excerpt"] = tc.excerpt
				}

				ctx := context.Background()
				result, err := handler.Execute(ctx, params)
				require.NoError(t, err)
				assert.True(t, result.Success)
			})
		}
	})

	t.Run("Limit Validation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			limit := query.Get("limit")
			// Limit should be capped at 100
			limitInt := 0
			_, _ = fmt.Sscanf(limit, "%d", &limitInt)
			assert.LessOrEqual(t, limitInt, 100)
			assert.GreaterOrEqual(t, limitInt, 1)

			response := map[string]interface{}{
				"results": []interface{}{},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		}))
		defer server.Close()

		provider := NewConfluenceProvider(logger, server.URL)
		handler := NewSearchContentHandler(provider)

		testCases := []struct {
			limit         float64
			expectedLimit int
		}{
			{-10, 1},   // Should be clamped to minimum
			{0, 1},     // Should be clamped to minimum
			{50, 50},   // Should be kept as-is
			{200, 100}, // Should be clamped to maximum
		}

		for _, tc := range testCases {
			params := map[string]interface{}{
				"cql":       "type = page",
				"limit":     tc.limit,
				"email":     "email@test.com",
				"api_token": "token",
			}

			ctx := context.Background()
			result, err := handler.Execute(ctx, params)
			require.NoError(t, err)
			assert.True(t, result.Success)
		}
	})
}

func TestSearchHandlerIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}

	t.Run("Search Toolset Registration", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")

		// Check that search toolset is registered
		assert.True(t, provider.IsToolsetEnabled("search"))

		// Check that handlers are in toolset
		enabled := provider.GetEnabledToolsets()
		assert.Contains(t, enabled, "search")

		// Verify search handler is registered
		searchHandler := NewSearchContentHandler(provider)
		assert.NotNil(t, searchHandler)
		assert.Equal(t, "search_content", searchHandler.GetDefinition().Name)
	})

	t.Run("Search Uses V1 API", func(t *testing.T) {
		provider := NewConfluenceProvider(logger, "test-domain")
		handler := NewSearchContentHandler(provider)

		// Check description mentions v1 API
		assert.Contains(t, handler.GetDefinition().Description, "v1 API")

		// Verify buildV1URL is used (by checking the URL pattern)
		v1URL := provider.buildV1URL("/content/search")
		assert.Contains(t, v1URL, "/wiki/rest/api/content/search")
		assert.NotContains(t, v1URL, "/wiki/api/v2") // Should NOT be v2
	})

	t.Run("V1 URL Builder for Test Servers", func(t *testing.T) {
		testServer := "http://localhost:8080"
		provider := NewConfluenceProvider(logger, testServer)

		v1URL := provider.buildV1URL("/content/search")
		assert.Equal(t, "http://localhost:8080/wiki/rest/api/content/search", v1URL)

		// Test with v2 suffix (should be removed)
		provider2 := NewConfluenceProvider(logger, testServer+"/wiki/api/v2")
		v1URL2 := provider2.buildV1URL("/content/search")
		assert.Equal(t, "http://localhost:8080/wiki/rest/api/content/search", v1URL2)
	})
}

func TestCQLValidation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")
	handler := NewSearchContentHandler(provider)

	t.Run("Valid CQL Queries", func(t *testing.T) {
		validQueries := []string{
			"space = DEV",
			"type = page",
			"space = DEV AND type = page",
			"space = DEV OR space = TEST",
			"(space = DEV OR space = TEST) AND type = page",
			`title ~ "search term"`,
			`text ~ "complex search" AND space = DEV`,
			"lastmodified > now('-7d')",
			"creator = currentUser()",
		}

		for _, cql := range validQueries {
			err := handler.validateCQL(cql)
			assert.NoError(t, err, "CQL should be valid: %s", cql)
		}
	})

	t.Run("Invalid CQL Queries", func(t *testing.T) {
		invalidQueries := []struct {
			cql      string
			errorMsg string
		}{
			{"", "cannot be empty"},
			{"   ", "cannot be empty"},
			{"title ~ 'unbalanced", "unbalanced single quotes"},
			{`title ~ "unbalanced`, "unbalanced double quotes"},
			{"(space = DEV", "unbalanced parentheses"},
			{"space = DEV)", "unbalanced parentheses"},
			{"space = DEV--; DROP TABLE", "dangerous pattern"},
			{"/* comment */ space = DEV", "dangerous pattern"},
		}

		for _, tc := range invalidQueries {
			err := handler.validateCQL(tc.cql)
			assert.Error(t, err, "CQL should be invalid: %s", tc.cql)
			assert.Contains(t, err.Error(), tc.errorMsg)
		}
	})
}
