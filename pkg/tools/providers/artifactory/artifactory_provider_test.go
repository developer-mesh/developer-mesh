package artifactory

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

	// Test security operations
	usersList, exists := mappings["users/list"]
	assert.True(t, exists)
	assert.Equal(t, "GET", usersList.Method)
	assert.Equal(t, "/api/security/users", usersList.PathTemplate)

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
	assert.Len(t, definitions, 5)

	// Check repositories definition
	reposDef := definitions[0]
	assert.Equal(t, "artifactory_repositories", reposDef.Name)
	assert.NotEmpty(t, reposDef.Description)
	assert.NotEmpty(t, reposDef.UsageExamples)
	assert.Contains(t, reposDef.SemanticTags, "repository")
	assert.Contains(t, reposDef.SemanticTags, "maven")

	// Check search definition
	var searchDef providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "artifactory_search" {
			searchDef = def
			break
		}
	}
	assert.Contains(t, searchDef.SemanticTags, "aql")
	assert.Contains(t, searchDef.SemanticTags, "checksum")

	// Check security definition
	var securityDef providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "artifactory_security" {
			securityDef = def
			break
		}
	}
	assert.Contains(t, securityDef.SemanticTags, "rbac")
	assert.Contains(t, securityDef.SemanticTags, "token")
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

func TestExecuteOperation_CreateUser(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/security/users/john.doe", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		// Check request body
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		assert.Equal(t, "john.doe", body["userName"])
		assert.Equal(t, "john.doe@example.com", body["email"])

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"john.doe","email":"john.doe@example.com","admin":false}`))
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
		"userName": "john.doe",
		"email":    "john.doe@example.com",
		"password": "SecurePass123!",
		"admin":    false,
	}

	result, err := provider.ExecuteOperation(ctx, "users/create", params)
	require.NoError(t, err)
	assert.NotNil(t, result)
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
