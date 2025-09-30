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

func TestEnhancedSearchOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Get all operations to verify search operations are included
	ops := provider.getAllOperationMappings()

	// Test that all enhanced search operations are present
	expectedSearchOps := []string{
		"search/artifacts",
		"search/aql",
		"search/gavc",
		"search/property",
		"search/checksum",
		"search/pattern",
		"search/dates",
		"search/buildArtifacts",
		"search/dependency",
		"search/usage",
		"search/latestVersion",
		"search/stats",
		"search/badChecksum",
		"search/license",
		"search/metadata",
	}

	for _, opName := range expectedSearchOps {
		t.Run(opName, func(t *testing.T) {
			op, exists := ops[opName]
			assert.True(t, exists, "Operation %s should exist", opName)
			assert.NotEmpty(t, op.OperationID, "Operation %s should have OperationID", opName)
			assert.NotEmpty(t, op.Method, "Operation %s should have Method", opName)
			assert.NotEmpty(t, op.PathTemplate, "Operation %s should have PathTemplate", opName)
		})
	}
}

func TestSearchParameterValidation(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	testCases := []struct {
		name      string
		operation string
		params    map[string]interface{}
		wantError bool
		errorMsg  string
	}{
		{
			name:      "search/artifacts - no parameters",
			operation: "search/artifacts",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "at least one search parameter required",
		},
		{
			name:      "search/artifacts - with name",
			operation: "search/artifacts",
			params: map[string]interface{}{
				"name": "*.jar",
			},
			wantError: false,
		},
		{
			name:      "search/property - no properties",
			operation: "search/property",
			params: map[string]interface{}{
				"repos": "libs-release",
			},
			wantError: true,
			errorMsg:  "property search requires property parameters",
		},
		{
			name:      "search/property - with properties",
			operation: "search/property",
			params: map[string]interface{}{
				"p":     "build.name=my-app",
				"repos": "libs-release",
			},
			wantError: false,
		},
		{
			name:      "search/dates - no dates",
			operation: "search/dates",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "dates search requires 'from' or 'to' parameter",
		},
		{
			name:      "search/dates - with from",
			operation: "search/dates",
			params: map[string]interface{}{
				"from": "2024-01-01T00:00:00Z",
			},
			wantError: false,
		},
		{
			name:      "search/dates - invalid dateFields",
			operation: "search/dates",
			params: map[string]interface{}{
				"from":       "2024-01-01T00:00:00Z",
				"dateFields": "invalid",
			},
			wantError: true,
			errorMsg:  "invalid dateFields value",
		},
		{
			name:      "search/buildArtifacts - no buildName",
			operation: "search/buildArtifacts",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "buildArtifacts search requires 'buildName' parameter",
		},
		{
			name:      "search/buildArtifacts - with buildName",
			operation: "search/buildArtifacts",
			params: map[string]interface{}{
				"buildName": "my-app",
			},
			wantError: false,
		},
		{
			name:      "search/dependency - no sha1",
			operation: "search/dependency",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "dependency search requires 'sha1' parameter",
		},
		{
			name:      "search/dependency - with sha1",
			operation: "search/dependency",
			params: map[string]interface{}{
				"sha1": "abc123def456",
			},
			wantError: false,
		},
		{
			name:      "search/usage - no dates",
			operation: "search/usage",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "usage search requires 'notUsedSince' or 'createdBefore' parameter",
		},
		{
			name:      "search/usage - with notUsedSince",
			operation: "search/usage",
			params: map[string]interface{}{
				"notUsedSince": "2023-07-01T00:00:00Z",
			},
			wantError: false,
		},
		{
			name:      "search/latestVersion - no group",
			operation: "search/latestVersion",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "latestVersion search requires 'g' (group) parameter",
		},
		{
			name:      "search/latestVersion - no artifact",
			operation: "search/latestVersion",
			params: map[string]interface{}{
				"g": "com.example",
			},
			wantError: true,
			errorMsg:  "latestVersion search requires 'a' (artifact) parameter",
		},
		{
			name:      "search/latestVersion - complete",
			operation: "search/latestVersion",
			params: map[string]interface{}{
				"g": "com.example",
				"a": "my-library",
			},
			wantError: false,
		},
		{
			name:      "search/badChecksum - invalid type",
			operation: "search/badChecksum",
			params: map[string]interface{}{
				"type": "invalid",
			},
			wantError: true,
			errorMsg:  "invalid checksum type",
		},
		{
			name:      "search/badChecksum - valid type",
			operation: "search/badChecksum",
			params: map[string]interface{}{
				"type": "sha256",
			},
			wantError: false,
		},
		{
			name:      "search/license - no parameters",
			operation: "search/license",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "license search requires 'license', 'approved', or 'unknown' parameter",
		},
		{
			name:      "search/license - with license",
			operation: "search/license",
			params: map[string]interface{}{
				"license": "Apache-2.0",
			},
			wantError: false,
		},
		{
			name:      "search/metadata - no metadata",
			operation: "search/metadata",
			params:    map[string]interface{}{},
			wantError: true,
			errorMsg:  "metadata search requires 'metadata' parameter",
		},
		{
			name:      "search/metadata - with metadata",
			operation: "search/metadata",
			params: map[string]interface{}{
				"metadata": "key1=value1",
			},
			wantError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.validateSearchParameters(tc.operation, tc.params)
			if tc.wantError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSearchOperationExecution(t *testing.T) {
	// Create test server to mock Artifactory API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/artifact":
			// Mock artifact search response
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"uri":          "http://localhost/artifactory/api/storage/libs-release-local/test.jar",
						"downloadUri":  "http://localhost/artifactory/libs-release-local/test.jar",
						"lastModified": "2024-01-01T00:00:00.000Z",
						"size":         "1024",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/search/gavc":
			// Mock GAVC search response
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"uri": "http://localhost/artifactory/api/storage/libs-release-local/com/example/my-lib/1.0.0/my-lib-1.0.0.jar",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/search/dates":
			// Mock dates search response
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"uri":     "http://localhost/artifactory/api/storage/libs-release-local/recent.jar",
						"created": "2024-01-01T00:00:00.000Z",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/search/buildArtifacts":
			// Mock build artifacts search response
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"uri":         "http://localhost/artifactory/api/storage/libs-release-local/build-artifact.jar",
						"buildName":   "my-app",
						"buildNumber": "42",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/search/latestVersion":
			// Mock latest version response - returns JSON with version
			response := map[string]interface{}{
				"version": "1.2.3",
				"uri":     "http://localhost/artifactory/api/storage/libs-release-local/com/example/my-lib/1.2.3",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider with test server URL
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "bearer",
	})

	// Create context with credentials
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-api-key",
		},
	})

	// Test artifact search
	t.Run("search/artifacts", func(t *testing.T) {
		params := map[string]interface{}{
			"name":  "*.jar",
			"repos": "libs-release-local",
		}
		result, err := provider.ExecuteOperation(ctx, "search/artifacts", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	// Test GAVC search
	t.Run("search/gavc", func(t *testing.T) {
		params := map[string]interface{}{
			"g":     "com.example",
			"a":     "my-lib",
			"repos": "libs-release-local",
		}
		result, err := provider.ExecuteOperation(ctx, "search/gavc", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	// Test dates search
	t.Run("search/dates", func(t *testing.T) {
		params := map[string]interface{}{
			"from":       "2024-01-01T00:00:00Z",
			"dateFields": "created",
			"repos":      "libs-release-local",
		}
		result, err := provider.ExecuteOperation(ctx, "search/dates", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	// Test build artifacts search
	t.Run("search/buildArtifacts", func(t *testing.T) {
		params := map[string]interface{}{
			"buildName":   "my-app",
			"buildNumber": "42",
			"repos":       "libs-release-local",
		}
		result, err := provider.ExecuteOperation(ctx, "search/buildArtifacts", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	// Test latest version search
	t.Run("search/latestVersion", func(t *testing.T) {
		params := map[string]interface{}{
			"g":     "com.example",
			"a":     "my-lib",
			"repos": "libs-release-local",
		}
		result, err := provider.ExecuteOperation(ctx, "search/latestVersion", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestFormatSearchURL(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	testCases := []struct {
		name     string
		baseURL  string
		params   map[string]interface{}
		expected string
	}{
		{
			name:     "No parameters",
			baseURL:  "/api/search/artifact",
			params:   map[string]interface{}{},
			expected: "/api/search/artifact",
		},
		{
			name:    "Single parameter",
			baseURL: "/api/search/artifact",
			params: map[string]interface{}{
				"name": "*.jar",
			},
			expected: "/api/search/artifact?name=%2A.jar",
		},
		{
			name:    "Multiple parameters",
			baseURL: "/api/search/artifact",
			params: map[string]interface{}{
				"name":  "*.jar",
				"repos": "libs-release,libs-snapshot",
			},
			expected: "/api/search/artifact?name=%2A.jar&repos=libs-release%2Clibs-snapshot",
		},
		{
			name:    "Boolean parameter",
			baseURL: "/api/search/artifact",
			params: map[string]interface{}{
				"includeRemote": true,
			},
			expected: "/api/search/artifact?includeRemote=true",
		},
		{
			name:    "Empty values ignored",
			baseURL: "/api/search/artifact",
			params: map[string]interface{}{
				"name":  "*.jar",
				"repos": "",
			},
			expected: "/api/search/artifact?name=%2A.jar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := provider.formatSearchURL(tc.baseURL, tc.params)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetSearchExamples(t *testing.T) {
	examples := GetSearchExamples()

	// Verify examples exist for key search operations
	expectedOps := []string{
		"search/artifacts",
		"search/property",
		"search/dates",
		"search/usage",
		"search/buildArtifacts",
		"search/latestVersion",
	}

	for _, op := range expectedOps {
		t.Run(op, func(t *testing.T) {
			exampleList, exists := examples[op]
			assert.True(t, exists, "Examples should exist for %s", op)
			assert.NotEmpty(t, exampleList, "Examples list should not be empty for %s", op)

			// Check first example has required fields
			if len(exampleList) > 0 {
				example := exampleList[0]
				assert.NotEmpty(t, example["description"], "Example should have description")
				assert.NotNil(t, example["params"], "Example should have params")
			}
		})
	}
}

func TestEnhancedSearchIntegration(t *testing.T) {
	// Test that enhanced search operations integrate properly with the provider
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Get default configuration to check operation groups
	config := provider.GetDefaultConfiguration()

	// Find search operation group
	var searchGroup *providers.OperationGroup
	for _, group := range config.OperationGroups {
		if group.Name == "search" {
			searchGroup = &group
			break
		}
	}

	require.NotNil(t, searchGroup, "Search operation group should exist")

	// Verify all enhanced search operations are in the group
	expectedOps := []string{
		"search/artifacts", "search/aql", "search/gavc",
		"search/property", "search/checksum", "search/pattern",
		"search/dates", "search/buildArtifacts", "search/dependency",
		"search/usage", "search/latestVersion", "search/stats",
		"search/badChecksum", "search/license", "search/metadata",
	}

	assert.ElementsMatch(t, expectedOps, searchGroup.Operations,
		"Search group should contain all enhanced search operations")
}
