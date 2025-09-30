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

func TestProjectOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Test that project operations are included in operation mappings
	t.Run("ProjectOperationsExist", func(t *testing.T) {
		operations := provider.GetOperationMappings()

		// Check core project operations exist
		projectOps := []string{
			"projects/list",
			"projects/get",
			"projects/create",
			"projects/update",
			"projects/delete",
		}

		for _, op := range projectOps {
			_, exists := operations[op]
			assert.True(t, exists, "Operation %s should exist", op)
		}

		// Check project membership operations exist
		memberOps := []string{
			"projects/users/list",
			"projects/users/get",
			"projects/users/add",
			"projects/users/update",
			"projects/users/remove",
			"projects/groups/list",
			"projects/groups/get",
			"projects/groups/add",
			"projects/groups/update",
			"projects/groups/remove",
		}

		for _, op := range memberOps {
			_, exists := operations[op]
			assert.True(t, exists, "Operation %s should exist", op)
		}

		// Check project role operations exist
		roleOps := []string{
			"projects/roles/list",
			"projects/roles/get",
			"projects/roles/create",
			"projects/roles/update",
			"projects/roles/delete",
		}

		for _, op := range roleOps {
			_, exists := operations[op]
			assert.True(t, exists, "Operation %s should exist", op)
		}

		// Check project repository operations exist
		repoOps := []string{
			"projects/repos/assign",
			"projects/repos/unassign",
			"projects/repos/list",
		}

		for _, op := range repoOps {
			_, exists := operations[op]
			assert.True(t, exists, "Operation %s should exist", op)
		}
	})

	// Test project operations have correct configuration
	t.Run("ProjectOperationConfiguration", func(t *testing.T) {
		operations := provider.GetOperationMappings()

		// Test projects/list configuration
		listOp := operations["projects/list"]
		assert.Equal(t, "GET", listOp.Method)
		assert.Equal(t, "/access/api/v1/projects", listOp.PathTemplate)
		assert.Empty(t, listOp.RequiredParams)
		assert.Contains(t, listOp.OptionalParams, "pageNum")
		assert.Contains(t, listOp.OptionalParams, "numOfRows")
		assert.Contains(t, listOp.OptionalParams, "orderBy")

		// Test projects/create configuration
		createOp := operations["projects/create"]
		assert.Equal(t, "POST", createOp.Method)
		assert.Equal(t, "/access/api/v1/projects", createOp.PathTemplate)
		assert.Contains(t, createOp.RequiredParams, "projectKey")
		assert.Contains(t, createOp.RequiredParams, "displayName")
		assert.Contains(t, createOp.OptionalParams, "description")
		assert.Contains(t, createOp.OptionalParams, "storageQuotaBytes")

		// Test projects/users/add configuration
		addUserOp := operations["projects/users/add"]
		assert.Equal(t, "PUT", addUserOp.Method)
		assert.Equal(t, "/access/api/v1/projects/{projectKey}/users/{username}", addUserOp.PathTemplate)
		assert.Contains(t, addUserOp.RequiredParams, "projectKey")
		assert.Contains(t, addUserOp.RequiredParams, "username")
		assert.Contains(t, addUserOp.OptionalParams, "roles")

		// Test projects/repos/assign configuration
		assignRepoOp := operations["projects/repos/assign"]
		assert.Equal(t, "PUT", assignRepoOp.Method)
		assert.Equal(t, "/access/api/v1/projects/_/attach/repositories/{repoKey}/{projectKey}", assignRepoOp.PathTemplate)
		assert.Contains(t, assignRepoOp.RequiredParams, "repoKey")
		assert.Contains(t, assignRepoOp.RequiredParams, "projectKey")
		assert.Contains(t, assignRepoOp.OptionalParams, "force")
	})

	// Test project operations in operation groups
	t.Run("ProjectsInOperationGroups", func(t *testing.T) {
		config := provider.GetDefaultConfiguration()

		// Find the projects operation group
		var projectGroup *providers.OperationGroup
		for i, group := range config.OperationGroups {
			if group.Name == "projects" {
				projectGroup = &config.OperationGroups[i]
				break
			}
		}

		require.NotNil(t, projectGroup, "Projects operation group should exist")
		assert.Equal(t, "Project Management", projectGroup.DisplayName)
		assert.Contains(t, projectGroup.Description, "Pro/Enterprise feature")

		// Check that all project operations are listed
		expectedOps := []string{
			"projects/list", "projects/get", "projects/create",
			"projects/users/add", "projects/groups/add",
			"projects/roles/create", "projects/repos/assign",
		}

		for _, op := range expectedOps {
			assert.Contains(t, projectGroup.Operations, op, "Operation %s should be in projects group", op)
		}
	})
}

func TestProjectListExecution(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Create a mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle capability discovery endpoints
		switch r.URL.Path {
		case "/api/system/ping":
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("OK")); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
			return
		case "/xray/api/v1/system/version":
			// Xray not available
			w.WriteHeader(http.StatusNotFound)
			return
		case "/access/api/v1/projects":
			// Main test case - list projects
			assert.Equal(t, "GET", r.Method)

			// Check authentication header is present
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				authHeader = r.Header.Get("X-JFrog-Art-Api")
			}
			assert.NotEmpty(t, authHeader, "Authentication header should be present")

			// Check for optional query parameters
			query := r.URL.Query()
			pageNum := query.Get("pageNum")
			numOfRows := query.Get("numOfRows")

			// Return mock response
			response := map[string]interface{}{
				"projects": []map[string]interface{}{
					{
						"projectKey":  "proj1",
						"displayName": "Project One",
						"description": "First test project",
						"adminPrivileges": []string{
							"MANAGE_MEMBERS",
							"MANAGE_RESOURCES",
						},
					},
					{
						"projectKey":  "proj2",
						"displayName": "Project Two",
						"description": "Second test project",
					},
				},
				"totalCount": 2,
			}

			// Add pagination info if requested
			if pageNum != "" || numOfRows != "" {
				response["pageNum"] = pageNum
				response["numOfRows"] = numOfRows
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}
		default:
			// Handle other capability discovery endpoints
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Configure provider to use mock server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Test executing projects/list operation
	// Add credentials to context
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	params := map[string]interface{}{
		"pageNum":   "1",
		"numOfRows": "10",
	}

	result, err := provider.ExecuteOperation(ctx, "projects/list", params)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check result structure
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	projects, ok := resultMap["projects"].([]interface{})
	require.True(t, ok, "Projects should be an array")
	assert.Len(t, projects, 2)

	// Check first project
	proj1, ok := projects[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "proj1", proj1["projectKey"])
	assert.Equal(t, "Project One", proj1["displayName"])
}

func TestProjectCreateExecution(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Create a mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints
		if handleCommonDiscoveryEndpoints(w, r) {
			return
		}

		// Check the actual request
		if r.URL.Path == "/access/api/v1/projects" {
			if r.Method == "GET" {
				// Handle GET request for projects list (might be checking if project exists)
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"projects": []map[string]interface{}{},
				}); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
				return
			}
			// For POST, we continue to create the project
			assert.Equal(t, "POST", r.Method)
		} else {
			t.Errorf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Parse request body
		var requestBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)

		// Check required fields
		assert.NotEmpty(t, requestBody["projectKey"])
		assert.NotEmpty(t, requestBody["displayName"])

		// Return created project
		response := map[string]interface{}{
			"projectKey":        requestBody["projectKey"],
			"displayName":       requestBody["displayName"],
			"description":       requestBody["description"],
			"storageQuotaBytes": requestBody["storageQuotaBytes"],
			"created":           "2025-01-28T10:00:00Z",
			"adminPrivileges": []string{
				"MANAGE_MEMBERS",
				"MANAGE_RESOURCES",
				"INDEX_RESOURCES",
				"ANNOTATE",
				"DELETE",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Configure provider to use mock server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Test executing projects/create operation
	// Add credentials to context
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	params := map[string]interface{}{
		"projectKey":        "test-project",
		"displayName":       "Test Project",
		"description":       "A test project for unit testing",
		"storageQuotaBytes": 1073741824, // 1GB
	}

	result, err := provider.ExecuteOperation(ctx, "projects/create", params)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check result
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")
	assert.Equal(t, "test-project", resultMap["projectKey"])
	assert.Equal(t, "Test Project", resultMap["displayName"])
	assert.NotEmpty(t, resultMap["created"])
}

func TestProjectUserManagement(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Create a mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints
		if handleCommonDiscoveryEndpoints(w, r) {
			return
		}

		// Handle GET for /access/api/v1/projects
		if r.URL.Path == "/access/api/v1/projects" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"projects": []map[string]interface{}{
					{"projectKey": "test-project"},
				},
			}); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}
			return
		}

		switch r.URL.Path {
		case "/access/api/v1/projects/test-project/users":
			if r.Method == "GET" {
				// List project users
				t.Logf("Handling GET request for project users")
				response := map[string]interface{}{
					"members": []map[string]interface{}{
						{
							"name":  "user1",
							"roles": []string{"Developer", "Viewer"},
						},
						{
							"name":  "user2",
							"roles": []string{"Project Admin"},
						},
					},
					"totalCount": 2,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
				return
			}

		case "/access/api/v1/projects/test-project/users/newuser":
			switch r.Method {
			case "PUT":
				// Add user to project
				var requestBody map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				response := map[string]interface{}{
					"name":  "newuser",
					"roles": requestBody["roles"],
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
				return
			case "DELETE":
				// Remove user from project - return a success response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "success",
					"message": "User removed from project",
				}); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
				return
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Configure provider to use mock server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Add credentials to context
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	// Test listing project users
	t.Run("ListProjectUsers", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "projects/users/list", map[string]interface{}{
			"projectKey": "test-project",
		})
		require.NoError(t, err)
		require.NotNil(t, result, "Result should not be nil")

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a map, got: %T", result)

		members, ok := resultMap["members"].([]interface{})
		require.True(t, ok, "Members field should be an array, got: %T", resultMap["members"])
		assert.Len(t, members, 2)
	})

	// Test adding user to project
	t.Run("AddProjectUser", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "projects/users/add", map[string]interface{}{
			"projectKey": "test-project",
			"username":   "newuser",
			"roles":      []string{"Developer"},
		})
		require.NoError(t, err)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "newuser", resultMap["name"])
	})

	// Test removing user from project
	t.Run("RemoveProjectUser", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "projects/users/remove", map[string]interface{}{
			"projectKey": "test-project",
			"username":   "newuser",
		})
		require.NoError(t, err)
		// DELETE typically returns no content, just ensure no error
		assert.NotNil(t, result)
	})
}

func TestProjectRepositoryAssignment(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Create a mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle common discovery endpoints
		if handleCommonDiscoveryEndpoints(w, r) {
			return
		}

		// Handle GET for /access/api/v1/projects
		if r.URL.Path == "/access/api/v1/projects" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"projects": []map[string]interface{}{
					{"projectKey": "test-project"},
				},
			}); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}
			return
		}

		switch {
		case r.URL.Path == "/access/api/v1/projects/_/attach/repositories/my-repo/test-project":
			if r.Method == "PUT" {
				// Assign repository to project
				response := map[string]interface{}{
					"repoKey":    "my-repo",
					"projectKey": "test-project",
					"assigned":   true,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}

		case r.URL.Path == "/access/api/v1/projects/_/attach/repositories/my-repo":
			if r.Method == "DELETE" {
				// Unassign repository from project - return success response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "success",
					"message": "Repository unassigned from project",
				}); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
				return
			}

		case r.URL.Path == "/api/repositories" && r.URL.Query().Get("project") == "test-project":
			if r.Method == "GET" {
				// List project repositories
				response := []map[string]interface{}{
					{
						"key":         "my-repo",
						"type":        "LOCAL",
						"packageType": "maven",
						"projectKey":  "test-project",
					},
					{
						"key":         "another-repo",
						"type":        "REMOTE",
						"packageType": "npm",
						"projectKey":  "test-project",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Configure provider to use mock server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Add credentials to context
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-token",
		},
	})

	// Test assigning repository to project
	t.Run("AssignRepoToProject", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "projects/repos/assign", map[string]interface{}{
			"repoKey":    "my-repo",
			"projectKey": "test-project",
		})
		require.NoError(t, err)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, resultMap["assigned"])
	})

	// Test listing project repositories
	t.Run("ListProjectRepos", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "projects/repos/list", map[string]interface{}{
			"projectKey": "test-project",
		})
		require.NoError(t, err)

		repos, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, repos, 2)

		// Check first repo
		repo1, ok := repos[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "my-repo", repo1["key"])
		assert.Equal(t, "test-project", repo1["projectKey"])
	})

	// Test unassigning repository from project
	t.Run("UnassignRepoFromProject", func(t *testing.T) {
		result, err := provider.ExecuteOperation(ctx, "projects/repos/unassign", map[string]interface{}{
			"repoKey": "my-repo",
		})
		require.NoError(t, err)
		// DELETE typically returns no content, just ensure no error
		assert.NotNil(t, result)
	})
}

func TestProjectCapabilityFiltering(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Test that project operations are properly filtered when projects feature is not available
	t.Run("ProjectOperationsFilteredWhenUnavailable", func(t *testing.T) {
		// Create a mock capability discoverer that reports projects as unavailable
		capabilityDiscoverer := NewCapabilityDiscoverer(logger)
		provider.capabilityDiscoverer = capabilityDiscoverer

		// Create mock server that returns 403 for projects endpoint
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle common discovery endpoints first, except for projects
			if r.URL.Path == "/access/api/v1/projects" {
				// This should return forbidden to test capability filtering
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []map[string]interface{}{
						{
							"code":    "FORBIDDEN",
							"message": "Projects feature requires Platform Pro or Enterprise license",
						},
					},
				}); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
				return
			}

			// Handle all other discovery endpoints
			if handleCommonDiscoveryEndpoints(w, r) {
				return
			}
		}))
		defer server.Close()

		// Configure provider to use mock server
		config := provider.GetDefaultConfiguration()
		config.BaseURL = server.URL
		provider.SetConfiguration(config)

		// Add credentials to context
		ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
			Credentials: &providers.ProviderCredentials{
				Token: "test-token",
			},
		})

		// Discover capabilities
		report, err := provider.GetCapabilityReport(ctx)
		require.NoError(t, err)
		require.NotNil(t, report)

		// Check that projects feature is marked as unavailable
		projectsFeature, exists := report.Features["projects"]
		assert.True(t, exists, "Projects feature should be in capability report")
		assert.False(t, projectsFeature.Available, "Projects should be marked as unavailable")
		assert.Contains(t, projectsFeature.Reason, "Projects feature requires")
		assert.Contains(t, projectsFeature.Required, "Platform Pro or Enterprise")

		// Check that project operations are marked as unavailable
		for opID := range provider.GetOperationMappings() {
			if opID[:8] == "projects" {
				capability, exists := report.Operations[opID]
				assert.True(t, exists, "Operation %s should be in capability report", opID)
				assert.False(t, capability.Available, "Operation %s should be unavailable", opID)
				assert.Contains(t, capability.Reason, "Projects feature is not available")
			}
		}
	})
}

// Helper function to handle common discovery endpoints in mock servers
func handleCommonDiscoveryEndpoints(w http.ResponseWriter, r *http.Request) bool {
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
		// (project-specific queries should be handled by the test's mock)
		if r.URL.Query().Get("project") == "" {
			// Repository list for permission discovery
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"key": "test-repo", "type": "LOCAL"},
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
	}
	return false
}
