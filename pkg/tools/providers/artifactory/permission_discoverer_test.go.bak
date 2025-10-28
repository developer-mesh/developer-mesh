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

func TestArtifactoryPermissionDiscoverer_DiscoverPermissions(t *testing.T) {
	logger := &observability.NoopLogger{}

	tests := []struct {
		name          string
		responses     map[string]mockResponse
		expectedPerms *ArtifactoryPermissions
		expectedError bool
	}{
		{
			name: "admin user with all permissions",
			responses: map[string]mockResponse{
				"/api/security/apiKey": {
					status: http.StatusOK,
					body:   map[string]interface{}{"username": "admin"},
				},
				"/api/security/users/admin": {
					status: http.StatusOK,
					body: map[string]interface{}{
						"name":   "Admin User",
						"email":  "admin@example.com",
						"admin":  true,
						"groups": []string{"admins"},
					},
				},
				"/api/system/configuration": {
					status: http.StatusOK,
					body:   map[string]interface{}{"type": "config"},
				},
				"/api/repositories": {
					status: http.StatusOK,
					body: []map[string]interface{}{
						{"key": "libs-release-local", "type": "LOCAL"},
						{"key": "libs-snapshot-local", "type": "LOCAL"},
					},
				},
			},
			expectedPerms: &ArtifactoryPermissions{
				UserInfo: map[string]interface{}{
					"name":   "Admin User",
					"email":  "admin@example.com",
					"admin":  true,
					"groups": []interface{}{"admins"},
				},
				IsAdmin: true,
				Repositories: map[string][]string{
					"libs-release-local":  {"read"},
					"libs-snapshot-local": {"read"},
				},
			},
		},
		{
			name: "regular user with limited permissions",
			responses: map[string]mockResponse{
				"/api/security/apiKey": {
					status: http.StatusOK,
					body:   map[string]interface{}{"username": "developer"},
				},
				"/api/security/users/developer": {
					status: http.StatusOK,
					body: map[string]interface{}{
						"name":   "Developer User",
						"email":  "developer@example.com",
						"admin":  false,
						"groups": []string{"developers"},
					},
				},
				"/api/system/configuration": {
					status: http.StatusForbidden,
					body:   map[string]interface{}{"error": "Access denied"},
				},
				"/api/repositories": {
					status: http.StatusOK,
					body: []map[string]interface{}{
						{"key": "libs-release-local", "type": "LOCAL"},
					},
				},
			},
			expectedPerms: &ArtifactoryPermissions{
				UserInfo: map[string]interface{}{
					"name":   "Developer User",
					"email":  "developer@example.com",
					"admin":  false,
					"groups": []interface{}{"developers"},
				},
				IsAdmin: false,
				Repositories: map[string][]string{
					"libs-release-local": {"read"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check authentication header
				assert.NotEmpty(t, r.Header.Get("X-JFrog-Art-Api"), "Expected X-JFrog-Art-Api header")

				// Return appropriate response based on path
				if response, ok := tt.responses[r.URL.Path]; ok {
					w.WriteHeader(response.status)
					if response.body != nil {
						if err := json.NewEncoder(w).Encode(response.body); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
						}
					}
					return
				}

				// Default response for unhandled paths (e.g., permission probes)
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			// Create discoverer with test server URL
			discoverer := NewArtifactoryPermissionDiscoverer(logger, server.URL)

			// Discover permissions
			perms, err := discoverer.DiscoverPermissions(context.Background(), "test-api-key")

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPerms.IsAdmin, perms.IsAdmin)
				assert.Equal(t, len(tt.expectedPerms.Repositories), len(perms.Repositories))
			}
		})
	}
}

func TestArtifactoryPermissionDiscoverer_FilterOperationsByPermissions(t *testing.T) {
	logger := &observability.NoopLogger{}
	discoverer := NewArtifactoryPermissionDiscoverer(logger, "https://test.jfrog.io/artifactory")

	operations := map[string]providers.OperationMapping{
		"repos/list": {
			OperationID: "listRepositories",
			Method:      "GET",
		},
		"repos/create": {
			OperationID: "createRepository",
			Method:      "PUT",
		},
		"repos/delete": {
			OperationID: "deleteRepository",
			Method:      "DELETE",
		},
		"artifacts/upload": {
			OperationID: "uploadArtifact",
			Method:      "PUT",
		},
		"artifacts/download": {
			OperationID: "downloadArtifact",
			Method:      "GET",
		},
		"system/configuration": {
			OperationID: "getConfiguration",
			Method:      "GET",
		},
		"users/create": {
			OperationID: "createUser",
			Method:      "PUT",
		},
		"search/artifacts": {
			OperationID: "searchArtifacts",
			Method:      "GET",
		},
	}

	tests := []struct {
		name        string
		permissions *ArtifactoryPermissions
		expected    []string // Operations that should be allowed
		blocked     []string // Operations that should be blocked
	}{
		{
			name: "admin user gets all operations",
			permissions: &ArtifactoryPermissions{
				IsAdmin: true,
				Repositories: map[string][]string{
					"test-repo": {"read", "write", "admin"},
				},
			},
			expected: []string{
				"repos/list",
				"repos/create",
				"repos/delete",
				"artifacts/upload",
				"artifacts/download",
				"system/configuration",
				"users/create",
				"search/artifacts",
			},
			blocked: []string{},
		},
		{
			name: "regular user with read-only access",
			permissions: &ArtifactoryPermissions{
				IsAdmin: false,
				Repositories: map[string][]string{
					"test-repo": {"read"},
				},
			},
			expected: []string{
				"repos/list",
				"artifacts/download",
				"search/artifacts",
			},
			blocked: []string{
				"repos/create",
				"repos/delete",
				"artifacts/upload",
				"system/configuration",
				"users/create",
			},
		},
		{
			name: "user with write permissions",
			permissions: &ArtifactoryPermissions{
				IsAdmin: false,
				Repositories: map[string][]string{
					"test-repo": {"read", "write"},
				},
			},
			expected: []string{
				"repos/list",
				"artifacts/upload",
				"artifacts/download",
				"search/artifacts",
			},
			blocked: []string{
				"repos/create",
				"repos/delete",
				"system/configuration",
				"users/create",
			},
		},
		{
			name: "user with no repository access",
			permissions: &ArtifactoryPermissions{
				IsAdmin:      false,
				Repositories: map[string][]string{},
			},
			expected: []string{}, // Very limited access
			blocked: []string{
				"repos/list",
				"repos/create",
				"repos/delete",
				"artifacts/upload",
				"artifacts/download",
				"search/artifacts",
				"users/create",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := discoverer.FilterOperationsByPermissions(operations, tt.permissions)

			// Check expected operations are allowed
			for _, op := range tt.expected {
				assert.Contains(t, filtered, op, "Expected operation %s to be allowed", op)
			}

			// Check blocked operations are not allowed
			for _, op := range tt.blocked {
				assert.NotContains(t, filtered, op, "Expected operation %s to be blocked", op)
			}

			// Verify the count
			assert.Equal(t, len(tt.expected), len(filtered), "Unexpected number of filtered operations")
		})
	}
}

func TestArtifactoryProvider_InitializeWithPermissions(t *testing.T) {
	logger := &observability.NoopLogger{}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/security/apiKey":
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"username": "testuser",
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/api/security/users/testuser":
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"name":   "Test User",
				"email":  "test@example.com",
				"admin":  false,
				"groups": []string{"developers"},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/api/repositories":
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode([]map[string]interface{}{
				{"key": "test-repo", "type": "LOCAL"},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/api/system/configuration":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider
	provider := NewArtifactoryProvider(logger)

	// Update base URL to use test server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Update permission discoverer to use test server
	provider.permissionDiscoverer = NewArtifactoryPermissionDiscoverer(logger, server.URL)

	// Test initialization with permissions
	ctx := context.Background()
	err := provider.InitializeWithPermissions(ctx, "test-api-key")
	require.NoError(t, err)

	// Verify operations are filtered
	operations := provider.GetOperationMappings()
	assert.NotNil(t, operations)

	// Should have filtered operations
	assert.NotNil(t, provider.filteredOperations)
	assert.Greater(t, len(provider.allOperations), len(provider.filteredOperations),
		"Filtered operations should be less than all operations for non-admin user")

	// Verify specific operations
	// Read operations should be allowed
	_, hasReposList := operations["repos/list"]
	assert.True(t, hasReposList, "repos/list should be allowed")

	// Admin operations should be blocked
	_, hasReposCreate := operations["repos/create"]
	assert.False(t, hasReposCreate, "repos/create should not be allowed for non-admin")
}

func TestArtifactoryProvider_GetOperationMappings(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	t.Run("returns all operations when no filtering", func(t *testing.T) {
		operations := provider.GetOperationMappings()
		assert.NotNil(t, operations)
		assert.Greater(t, len(operations), 0, "Should have operations")

		// Verify some key operations exist
		assert.Contains(t, operations, "repos/list")
		assert.Contains(t, operations, "repos/create")
		assert.Contains(t, operations, "artifacts/upload")
	})

	t.Run("returns filtered operations after initialization", func(t *testing.T) {
		// Simulate filtered operations
		provider.filteredOperations = map[string]providers.OperationMapping{
			"repos/list": {
				OperationID: "listRepositories",
				Method:      "GET",
			},
			"artifacts/download": {
				OperationID: "downloadArtifact",
				Method:      "GET",
			},
		}

		operations := provider.GetOperationMappings()
		assert.Equal(t, 2, len(operations), "Should return only filtered operations")
		assert.Contains(t, operations, "repos/list")
		assert.Contains(t, operations, "artifacts/download")
		assert.NotContains(t, operations, "repos/create", "Create operation should be filtered out")
	})
}

// Helper type for mock responses
type mockResponse struct {
	status int
	body   interface{}
}
