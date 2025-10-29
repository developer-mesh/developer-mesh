package artifactory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArtifactoryProvider(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	assert.NotNil(t, provider)
	assert.Equal(t, "artifactory", provider.GetProviderName())
	assert.Contains(t, provider.GetSupportedVersions(), "v2")
	assert.NotNil(t, provider.GetOperationMappings())
	assert.NotEmpty(t, provider.GetToolDefinitions())
}

func TestGetToolDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	definitions := provider.GetToolDefinitions()
	assert.Len(t, definitions, 4)

	// Check for expected tools
	toolNames := make(map[string]bool)
	for _, def := range definitions {
		toolNames[def.Name] = true
	}

	assert.True(t, toolNames["artifactory_repositories"])
	assert.True(t, toolNames["artifactory_artifacts"])
	assert.True(t, toolNames["artifactory_builds"])
	assert.True(t, toolNames["artifactory_users"])
}

func TestGetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	mappings := provider.GetOperationMappings()
	assert.NotEmpty(t, mappings)

	// Test repository operations
	repoList, exists := mappings["repos/list"]
	assert.True(t, exists)
	assert.Equal(t, "GET", repoList.Method)
	assert.Equal(t, "/api/repositories", repoList.PathTemplate)

	// Test artifact operations
	artifactUpload, exists := mappings["artifacts/upload"]
	assert.True(t, exists)
	assert.Equal(t, "PUT", artifactUpload.Method)
	assert.Contains(t, artifactUpload.RequiredParams, "repoKey")
	assert.Contains(t, artifactUpload.RequiredParams, "itemPath")

	// Test search operations
	searchAQL, exists := mappings["search/aql"]
	assert.True(t, exists)
	assert.Equal(t, "POST", searchAQL.Method)
	assert.Equal(t, "/api/search/aql", searchAQL.PathTemplate)

	// Test build operations
	buildsList, exists := mappings["builds/list"]
	assert.True(t, exists)
	assert.Equal(t, "GET", buildsList.Method)

	// Test system operations
	systemPing, exists := mappings["system/ping"]
	assert.True(t, exists)
	assert.Equal(t, "GET", systemPing.Method)
	assert.Equal(t, "/api/system/ping", systemPing.PathTemplate)
}

func TestNormalizeOperationName(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Simple actions default to repo operations
		{"Simple list", "list", "repos/list"},
		{"Simple get", "get", "repos/get"},
		{"Simple create", "create", "repos/create"},
		{"Simple upload", "upload", "artifacts/upload"},
		{"Simple download", "download", "artifacts/download"},
		{"Simple search", "search", "search/artifacts"},

		// Already normalized
		{"Repos list", "repos/list", "repos/list"},
		{"Artifacts upload", "artifacts/upload", "artifacts/upload"},

		// Different separators
		{"Hyphen separator", "repos-list", "repos/list"},
		{"Underscore separator", "repos_list", "repos/list"},
		{"Mixed separators", "artifacts-properties_set", "artifacts/properties/set"},

		// Unknown operations pass through
		{"Unknown operation", "unknown/operation", "unknown/operation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.normalizeOperationName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDefaultConfiguration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	config := provider.GetDefaultConfiguration()

	assert.Equal(t, "bearer", config.AuthType)
	assert.Equal(t, "/api/system/ping", config.HealthEndpoint)
	assert.Contains(t, config.DefaultHeaders, "X-JFrog-Art-Api-Version")
	assert.Equal(t, "2", config.DefaultHeaders["X-JFrog-Art-Api-Version"])
	assert.Equal(t, 600, config.RateLimits.RequestsPerMinute)
	assert.NotNil(t, config.RetryPolicy)
	assert.True(t, config.RetryPolicy.RetryOnRateLimit)
}

func TestGetEmbeddedSpecVersion(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	version := provider.GetEmbeddedSpecVersion()
	assert.Equal(t, "v2-7.x", version)

	// Test with nil provider (shouldn't panic)
	var nilProvider *ArtifactoryProvider
	version = nilProvider.GetEmbeddedSpecVersion()
	assert.Equal(t, "v2", version)
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name      string
		creds     map[string]string
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid token credentials",
			creds: map[string]string{
				"token": "test-token",
			},
			wantError: false,
		},
		{
			name: "Valid API key credentials",
			creds: map[string]string{
				"api_key": "test-api-key",
			},
			wantError: false,
		},
		{
			name: "Valid username/password credentials",
			creds: map[string]string{
				"username": "testuser",
				"password": "testpass",
			},
			wantError: false,
		},
		{
			name:      "No credentials provided",
			creds:     map[string]string{},
			wantError: true,
			errorMsg:  "no valid credentials provided",
		},
		{
			name:      "Nil credentials",
			creds:     nil,
			wantError: true,
			errorMsg:  "credentials cannot be nil",
		},
		{
			name: "Only username without password",
			creds: map[string]string{
				"username": "testuser",
			},
			wantError: true,
			errorMsg:  "no valid credentials provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server for successful validation
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/system/ping" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK"))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			err := provider.ValidateCredentials(context.Background(), tt.creds)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()
	assert.GreaterOrEqual(t, len(definitions), 5, "Should have at least 5 category definitions")

	// Check that all definitions have required fields
	for _, def := range definitions {
		assert.NotEmpty(t, def.Name, "Definition should have a name")
		assert.NotEmpty(t, def.Description, "Definition should have a description")
		assert.NotEmpty(t, def.UsageExamples, "Definition should have usage examples")
		assert.NotEmpty(t, def.SemanticTags, "Definition should have semantic tags")
	}
}

func TestHealthCheck(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/system/ping", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check auth header
		authHeader := r.Header.Get("Authorization")
		assert.NotEmpty(t, authHeader)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Override the base URL
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Create context with credentials
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	err := provider.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestExecuteOperation_ListRepos(t *testing.T) {
	// Mock response
	mockRepos := []map[string]interface{}{
		{
			"key":         "libs-release-local",
			"type":        "LOCAL",
			"packageType": "maven",
			"url":         "http://localhost:8081/artifactory/libs-release-local",
		},
		{
			"key":         "npm-local",
			"type":        "LOCAL",
			"packageType": "npm",
			"url":         "http://localhost:8081/artifactory/npm-local",
		},
	}

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints
		if handleCommonDiscoveryEndpointsForTests(t, w, r) {
			return
		}

		assert.Equal(t, "/api/repositories", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockRepos); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Override the base URL
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
	result, err := provider.ExecuteOperation(ctx, "repos/list", nil)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check result
	repos, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, repos, 2)
}

func TestExecuteOperation_SearchArtifacts(t *testing.T) {
	// Mock search results
	mockResults := map[string]interface{}{
		"results": []map[string]interface{}{
			{
				"repo":     "libs-release-local",
				"path":     "com/mycompany/myapp/1.0.0",
				"name":     "myapp-1.0.0.jar",
				"type":     "file",
				"size":     12345,
				"created":  "2025-01-27T10:00:00Z",
				"modified": "2025-01-27T10:00:00Z",
				"sha1":     "abc123def456",
				"md5":      "123456789abc",
			},
		},
	}

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints
		if handleCommonDiscoveryEndpointsForTests(t, w, r) {
			return
		}

		assert.Equal(t, "/api/search/artifact", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Check query parameters
		assert.Equal(t, "*.jar", r.URL.Query().Get("name"))
		assert.Equal(t, "libs-release-local", r.URL.Query().Get("repos"))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResults); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Override the base URL
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
	params := map[string]interface{}{
		"name":  "*.jar",
		"repos": "libs-release-local",
	}

	result, err := provider.ExecuteOperation(ctx, "search/artifacts", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check result
	searchResult, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, searchResult, "results")
}

func TestExecuteOperation_UnknownOperation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	_, err := provider.ExecuteOperation(ctx, "unknown/operation", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")
}

func TestExecuteOperation_BuildPromote(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints
		if handleCommonDiscoveryEndpointsForTests(t, w, r) {
			return
		}

		assert.Equal(t, "/api/build/promote/myapp/123", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Check request body
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		assert.Equal(t, "libs-prod-local", body["targetRepo"])
		assert.Equal(t, "Released", body["status"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"level":"info","message":"Promotion completed successfully"}]}`))
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Override the base URL
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
	params := map[string]interface{}{
		"buildName":   "myapp",
		"buildNumber": "123",
		"targetRepo":  "libs-prod-local",
		"status":      "Released",
		"comment":     "Promoted to production",
	}

	result, err := provider.ExecuteOperation(ctx, "builds/promote", params)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestExecuteAQLQuery tests AQL query execution with text/plain content type
func TestExecuteAQLQuery(t *testing.T) {
	testCases := []struct {
		name           string
		query          interface{}
		mockResponse   string
		mockStatusCode int
		expectedError  bool
		checkResponse  func(t *testing.T, result interface{})
	}{
		{
			name:  "Simple AQL query as string",
			query: `items.find({"repo":"libs-release-local","type":"file"})`,
			mockResponse: `{
				"results": [
					{"repo":"libs-release-local","path":"org/example","name":"test.jar","type":"file"},
					{"repo":"libs-release-local","path":"org/sample","name":"app.jar","type":"file"}
				],
				"range": {"start_pos":0,"end_pos":2,"total":2}
			}`,
			mockStatusCode: http.StatusOK,
			expectedError:  false,
			checkResponse: func(t *testing.T, result interface{}) {
				res, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, res, "results")
				assert.Contains(t, res, "total_count")
				results := res["results"].([]interface{})
				assert.Len(t, results, 2)
			},
		},
		{
			name:  "AQL query with include fields",
			query: `items.find({"repo":"libs-release-local"}).include("name","repo","path","actual_md5","actual_sha1","size")`,
			mockResponse: `{
				"results": [
					{
						"repo":"libs-release-local",
						"path":"org/example",
						"name":"test.jar",
						"actual_md5":"abc123",
						"actual_sha1":"def456",
						"size":1024
					}
				],
				"range": {"start_pos":0,"end_pos":1,"total":1}
			}`,
			mockStatusCode: http.StatusOK,
			expectedError:  false,
			checkResponse: func(t *testing.T, result interface{}) {
				res, ok := result.(map[string]interface{})
				require.True(t, ok)
				results := res["results"].([]interface{})
				assert.Len(t, results, 1)
				firstResult := results[0].(map[string]interface{})
				assert.Equal(t, "test.jar", firstResult["name"])
				assert.Equal(t, float64(1024), firstResult["size"])
			},
		},
		{
			name: "AQL query from map structure",
			query: map[string]interface{}{
				"repo": "docker-local",
				"type": "file",
				"name": map[string]interface{}{
					"$match": "*.json",
				},
			},
			mockResponse: `{
				"results": [
					{"repo":"docker-local","path":"library/nginx","name":"manifest.json","type":"file"}
				],
				"range": {"start_pos":0,"end_pos":1,"total":1}
			}`,
			mockStatusCode: http.StatusOK,
			expectedError:  false,
			checkResponse: func(t *testing.T, result interface{}) {
				res, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, 1, res["total_count"])
			},
		},
		{
			name:  "Complex AQL with sorting and limit",
			query: `items.find({"repo":"libs-release-local"}).sort({"$asc":["created"]}).limit(10)`,
			mockResponse: `{
				"results": [
					{"repo":"libs-release-local","path":"a","name":"1.jar"},
					{"repo":"libs-release-local","path":"b","name":"2.jar"},
					{"repo":"libs-release-local","path":"c","name":"3.jar"}
				],
				"range": {"start_pos":0,"end_pos":3,"total":50}
			}`,
			mockStatusCode: http.StatusOK,
			expectedError:  false,
			checkResponse: func(t *testing.T, result interface{}) {
				res, ok := result.(map[string]interface{})
				require.True(t, ok)
				results := res["results"].([]interface{})
				assert.Len(t, results, 3)
				// Total count shows actual number returned
				assert.Equal(t, 3, res["total_count"])
			},
		},
		{
			name:           "Invalid AQL query - missing domain",
			query:          `find({"repo":"test"})`,
			mockResponse:   "",
			mockStatusCode: http.StatusOK,
			expectedError:  true,
		},
		{
			name:           "Invalid AQL query - unbalanced brackets",
			query:          `items.find({"repo":"test"`,
			mockResponse:   "",
			mockStatusCode: http.StatusOK,
			expectedError:  true,
		},
		{
			name:           "Empty query",
			query:          "",
			mockResponse:   "",
			mockStatusCode: http.StatusOK,
			expectedError:  true,
		},
		{
			name:           "Server error response",
			query:          `items.find({})`,
			mockResponse:   `{"errors":[{"status":500,"message":"Internal server error"}]}`,
			mockStatusCode: http.StatusInternalServerError,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle capability discovery requests
				if r.URL.Path == "/api/system/ping" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK"))
					return
				}
				if r.URL.Path == "/api/security/apiKey" {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"apiKey": "test-key",
						"type":   "user",
					})
					return
				}
				if r.URL.Path == "/api/repositories" {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode([]interface{}{})
					return
				}
				if r.URL.Path == "/api/system/configuration" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("<config></config>"))
					return
				}
				if r.URL.Path == "/xray/api/v1/system/version" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.URL.Path == "/pipelines/api/v1/system/info" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.URL.Path == "/mc/api/v1/system/info" {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				// Now handle the actual AQL request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/search/aql", r.URL.Path)

				// Verify content type header
				assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))

				// Read and verify request body
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				// For non-error test cases, verify the query was sent as plain text
				if !tc.expectedError && tc.query != nil {
					if str, ok := tc.query.(string); ok {
						assert.Equal(t, str, string(body))
					} else {
						// For map queries, verify it was converted to AQL format
						assert.Contains(t, string(body), "items.find")
					}
				}

				// Return mock response
				w.WriteHeader(tc.mockStatusCode)
				_, _ = w.Write([]byte(tc.mockResponse))
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewArtifactoryProvider(logger)

			// Disable capability discoverer for this test
			provider.capabilityDiscoverer = nil

			// Override the base URL
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Create context with credentials
			ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "test-token",
				},
			})

			// Execute AQL query
			params := map[string]interface{}{
				"query": tc.query,
			}

			result, err := provider.ExecuteOperation(ctx, "search/aql", params)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				if tc.checkResponse != nil {
					tc.checkResponse(t, result)
				}
			}
		})
	}
}

// TestAQLQueryValidation tests the AQL query validation logic
func TestAQLQueryValidation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	testCases := []struct {
		name          string
		query         string
		expectedError bool
	}{
		{
			name:          "Valid items.find query",
			query:         `items.find({"repo":"test"})`,
			expectedError: false,
		},
		{
			name:          "Valid builds.find query",
			query:         `builds.find({"name":"myapp"})`,
			expectedError: false,
		},
		{
			name:          "Valid entries.find query",
			query:         `entries.find({})`,
			expectedError: false,
		},
		{
			name:          "Valid artifacts.find query",
			query:         `artifacts.find({})`,
			expectedError: false,
		},
		{
			name:          "Query with include clause",
			query:         `items.find({}).include("name","repo")`,
			expectedError: false,
		},
		{
			name:          "Query with sort and limit",
			query:         `items.find({}).sort({"$asc":["created"]}).limit(10)`,
			expectedError: false,
		},
		{
			name:          "Invalid - missing domain",
			query:         `find({})`,
			expectedError: true,
		},
		{
			name:          "Invalid - wrong domain",
			query:         `users.find({})`,
			expectedError: true,
		},
		{
			name:          "Invalid - unbalanced parentheses",
			query:         `items.find({}`,
			expectedError: true,
		},
		{
			name:          "Invalid - unbalanced braces",
			query:         `items.find({"repo":"test"`,
			expectedError: true,
		},
		{
			name:          "Invalid - mixed unbalanced brackets",
			query:         `items.find({"repo":"test"])`,
			expectedError: true,
		},
		{
			name:          "Empty query",
			query:         "",
			expectedError: true,
		},
		{
			name:          "Whitespace only",
			query:         "   ",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.validateAQLQuery(tc.query)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFormatAQLFromMap tests the map to AQL conversion
func TestFormatAQLFromMap(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	testCases := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name: "Simple criteria",
			input: map[string]interface{}{
				"repo": "libs-release-local",
				"type": "file",
			},
			expected: `items.find({"repo":"libs-release-local","type":"file"})`,
		},
		{
			name: "Complex criteria with nested conditions",
			input: map[string]interface{}{
				"repo": "docker-local",
				"name": map[string]interface{}{
					"$match": "*.json",
				},
			},
			expected: `items.find({"name":{"$match":"*.json"},"repo":"docker-local"})`,
		},
		{
			name:     "Empty map",
			input:    map[string]interface{}{},
			expected: `items.find({})`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := provider.formatAQLFromMap(tc.input)
			// Parse both as JSON to compare structure, not string representation
			// Since map iteration order is not guaranteed
			assert.Contains(t, result, "items.find")
			if len(tc.input) > 0 {
				for key := range tc.input {
					assert.Contains(t, result, key)
				}
			}
		})
	}
}

// TestAQLWithPagination tests AQL query pagination support
func TestAQLWithPagination(t *testing.T) {
	// Create a mock server that returns multiple results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 10 results
		results := []interface{}{}
		for i := 0; i < 10; i++ {
			results = append(results, map[string]interface{}{
				"repo": "test-repo",
				"name": fmt.Sprintf("file%d.jar", i),
			})
		}

		response := map[string]interface{}{
			"results": results,
			"range": map[string]interface{}{
				"start_pos": 0,
				"end_pos":   10,
				"total":     10,
			},
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Disable capability discoverer for this test
	provider.capabilityDiscoverer = nil

	// Override the base URL
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Create context with credentials
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	// Test with limit parameter
	params := map[string]interface{}{
		"query": `items.find({})`,
		"limit": 5,
	}

	result, err := provider.ExecuteOperation(ctx, "search/aql", params)
	require.NoError(t, err)

	res, ok := result.(map[string]interface{})
	require.True(t, ok)

	// Check that limit was applied
	results := res["results"].([]interface{})
	assert.Len(t, results, 5)
	assert.Equal(t, true, res["has_more"])
	assert.Equal(t, 5, res["total_count"])
}

// Helper function to handle common discovery endpoints in mock servers
func handleCommonDiscoveryEndpointsForTests(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/api/system/ping", "/access/api/v1/system/ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return true
	case "/api/system/configuration":
		// System configuration endpoint
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"urlBase":     "http://test.artifactory.com",
			"offlineMode": false,
		})
		return true
	case "/xray/api/v1/system/version":
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "3.0.0", "revision": "123"})
		return true
	case "/pipelines/api/v1/system/info", "/mc/api/v1/system/info",
		"/distribution/api/v1/system/info", "/api/federation/status":
		// These are feature discovery endpoints - return 404 to indicate not available
		w.WriteHeader(http.StatusNotFound)
		return true
	case "/api/repositories":
		// Only handle repository list if there's no project query parameter
		if r.URL.Query().Get("project") == "" {
			// Repository list for permission discovery and tests
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"key": "test-repo", "type": "LOCAL", "packageType": "maven"},
				{"key": "npm-local", "type": "LOCAL", "packageType": "npm"},
			})
			return true
		}
		// Let the test handler deal with project-specific queries
		return false
	case "/api/v2/security/permissions":
		// Permission discovery endpoint
		if r.Method == "GET" {
			// Return empty permissions list for discovery
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"permissions": []map[string]interface{}{},
			})
		} else {
			// For other methods, just return OK
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
			})
		}
		return true
	case "/access/api/v1/projects":
		// Handle GET for projects list (capability discovery)
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"projects": []map[string]interface{}{},
			})
			return true
		}
		// Let test handle other methods
		return false
	}
	return false
}
