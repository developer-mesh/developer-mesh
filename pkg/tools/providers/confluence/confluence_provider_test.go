package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTransport is a custom transport that redirects all requests to a test server
type testTransport struct {
	serverURL  string
	httpClient *http.Client
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the URL with our test server URL
	testReq, err := http.NewRequest(req.Method, t.serverURL+req.URL.Path+"?"+req.URL.RawQuery, req.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	for k, v := range req.Header {
		testReq.Header[k] = v
	}

	// Copy basic auth if present
	if username, password, ok := req.BasicAuth(); ok {
		testReq.SetBasicAuth(username, password)
	}

	return t.httpClient.Transport.RoundTrip(testReq)
}

func TestNewConfluenceProvider(t *testing.T) {
	logger := &observability.NoopLogger{}

	provider := NewConfluenceProvider(logger, "test-domain")

	assert.NotNil(t, provider)
	assert.Equal(t, "confluence", provider.GetProviderName())
	assert.Contains(t, provider.GetSupportedVersions(), "v2")
	assert.Equal(t, "test-domain", provider.domain)
	assert.Equal(t, "https://test-domain.atlassian.net/wiki/api/v2", provider.BaseProvider.GetDefaultConfiguration().BaseURL)
}

func TestGetToolDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	tools := provider.GetToolDefinitions()

	assert.NotEmpty(t, tools)

	// Check for expected tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["confluence_content"])
	assert.True(t, toolNames["confluence_space"])
	assert.True(t, toolNames["confluence_search"])
	assert.True(t, toolNames["confluence_attachment"])
	assert.True(t, toolNames["confluence_comment"])
	assert.True(t, toolNames["confluence_user"])
}

func TestGetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	mappings := provider.GetOperationMappings()

	assert.NotEmpty(t, mappings)

	// Test content operations
	assert.Contains(t, mappings, "content/list")
	assert.Contains(t, mappings, "content/get")
	assert.Contains(t, mappings, "content/create")
	assert.Contains(t, mappings, "content/update")
	assert.Contains(t, mappings, "content/delete")
	assert.Contains(t, mappings, "content/search")

	// Test space operations
	assert.Contains(t, mappings, "space/list")
	assert.Contains(t, mappings, "space/get")
	assert.Contains(t, mappings, "space/create")

	// Test search operation
	searchMapping := mappings["content/search"]
	assert.Equal(t, "GET", searchMapping.Method)
	assert.Equal(t, "/search", searchMapping.PathTemplate)
	assert.Contains(t, searchMapping.RequiredParams, "cql")
}

func TestGetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	assert.NotEmpty(t, definitions)

	// Check for comprehensive AI definitions
	var contentDef *providers.AIOptimizedToolDefinition
	for i, def := range definitions {
		if def.Name == "confluence_content" {
			contentDef = &definitions[i]
			break
		}
	}

	require.NotNil(t, contentDef)
	assert.Equal(t, "Confluence Content Operations", contentDef.DisplayName)
	assert.NotEmpty(t, contentDef.UsageExamples)
	assert.NotEmpty(t, contentDef.SemanticTags)
	assert.NotEmpty(t, contentDef.CommonPhrases)
	assert.NotNil(t, contentDef.Capabilities)
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name        string
		creds       map[string]string
		setupServer func(*httptest.Server)
		wantErr     bool
		errContains string
	}{
		{
			name: "valid email and api token",
			creds: map[string]string{
				"email":     "user@example.com",
				"api_token": "test-token",
			},
			setupServer: func(server *httptest.Server) {
				// Server will return 200 OK
			},
			wantErr: false,
		},
		{
			name: "invalid credentials",
			creds: map[string]string{
				"email":     "user@example.com",
				"api_token": "invalid-token",
			},
			setupServer: func(server *httptest.Server) {
				// Return 401 Unauthorized
			},
			wantErr:     true,
			errContains: "invalid Confluence credentials",
		},
		{
			name:  "missing credentials",
			creds: map[string]string{},
			setupServer: func(server *httptest.Server) {
			},
			wantErr:     true,
			errContains: "no authentication credentials found",
		},
		{
			name: "username and password (legacy)",
			creds: map[string]string{
				"username": "testuser",
				"password": "testpass",
			},
			setupServer: func(server *httptest.Server) {
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check that the URL path is correct
				if !strings.HasSuffix(r.URL.Path, "/wiki/rest/api/space") {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				// Check basic auth
				username, password, ok := r.BasicAuth()
				if !ok {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Check for valid credentials based on test case
				if tt.name == "invalid credentials" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Valid credentials
				if (username == "user@example.com" && password == "test-token") ||
					(username == "testuser" && password == "testpass") {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"results": []interface{}{},
					})
				} else {
					w.WriteHeader(http.StatusUnauthorized)
				}
			}))
			defer server.Close()

			// Create provider with test server URL
			logger := &observability.NoopLogger{}
			provider := NewConfluenceProvider(logger, "test-domain")

			// Create a custom HTTP client that redirects requests to our test server
			provider.httpClient = &http.Client{
				Transport: &testTransport{
					serverURL:  server.URL,
					httpClient: server.Client(),
				},
			}

			err := provider.ValidateCredentials(context.Background(), tt.creds)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeOperationName(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	tests := []struct {
		input    string
		expected string
	}{
		{"content-create", "content/create"},
		{"content_create", "content/create"},
		{"content/create", "content/create"},
		{"list", "content/list"},
		{"get", "content/get"},
		{"create", "content/create"},
		{"search", "content/search"},
		{"space-list", "space/list"},
		{"attachment_upload", "attachment/upload"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecuteOperation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wiki/rest/api/content":
			// List content
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id":    "123456",
						"type":  "page",
						"title": "Test Page",
					},
				},
				"size":  1,
				"start": 0,
				"limit": 25,
			})
		case "/wiki/rest/api/content/123456":
			// Get specific content
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":    "123456",
				"type":  "page",
				"title": "Test Page",
				"space": map[string]interface{}{
					"key": "TEST",
				},
			})
		case "/wiki/rest/api/search":
			// Search
			cql := r.URL.Query().Get("cql")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"content": map[string]interface{}{
							"id":    "789012",
							"type":  "page",
							"title": "Search Result",
						},
					},
				},
				"cql": cql,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Configure provider to use test server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL + "/wiki/rest/api"
	config.AuthType = "basic"
	provider.SetConfiguration(config)

	// Create context with credentials
	ctx := context.Background()
	ctx = providers.WithContext(ctx, &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Username: "test",
			Password: "test",
		},
	})

	t.Run("list content", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "content/list", map[string]interface{}{
			"spaceKey": "TEST",
			"limit":    10,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, resultMap, "results")
	})

	t.Run("get content", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "content/get", map[string]interface{}{
			"id": "123456",
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "123456", resultMap["id"])
	})

	t.Run("search content", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "content/search", map[string]interface{}{
			"cql":   "type = page",
			"limit": 10,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, resultMap, "results")
	})

	t.Run("unknown operation", func(t *testing.T) {
		_, err := provider.ExecuteOperation(ctx, "unknown/operation", map[string]interface{}{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown operation")
	})
}

func TestGetDefaultConfiguration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	config := provider.GetDefaultConfiguration()

	assert.Equal(t, "https://test-domain.atlassian.net/wiki/api/v2", config.BaseURL)
	assert.Equal(t, "basic", config.AuthType)
	assert.Contains(t, config.RequiredScopes, "read:confluence-content.all")
	assert.Contains(t, config.RequiredScopes, "write:confluence-content.all")
	assert.NotNil(t, config.RateLimits)
	assert.Equal(t, 5000, config.RateLimits.RequestsPerHour)
	assert.NotEmpty(t, config.OperationGroups)
	assert.NotNil(t, config.RetryPolicy)
}

func TestGetEnabledModules(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	modules := provider.GetEnabledModules()

	assert.NotEmpty(t, modules)
	assert.Contains(t, modules, "content")
	assert.Contains(t, modules, "space")
	assert.Contains(t, modules, "attachment")
	assert.Contains(t, modules, "comment")
	assert.Contains(t, modules, "label")
	assert.Contains(t, modules, "search")
}

func TestGetEmbeddedSpecVersion(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	version := provider.GetEmbeddedSpecVersion()

	assert.NotEmpty(t, version)
	assert.Equal(t, "2024.01", version)
}
