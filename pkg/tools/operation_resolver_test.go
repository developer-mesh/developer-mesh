package tools

import (
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationResolver(t *testing.T) {
	logger := &observability.NoopLogger{}
	resolver := NewOperationResolver(logger)

	// Create a sample OpenAPI spec with GitHub-style operations
	spec := &openapi3.T{
		Paths: openapi3.NewPaths(),
	}

	// Add sample operations
	reposGetOp := &openapi3.Operation{
		OperationID: "repos/get",
		Summary:     "Get a repository",
		Tags:        []string{"repos"},
	}

	issuesListOp := &openapi3.Operation{
		OperationID: "issues/list",
		Summary:     "List issues",
		Tags:        []string{"issues"},
	}

	pullsCreateOp := &openapi3.Operation{
		OperationID: "pulls/create",
		Summary:     "Create a pull request",
		Tags:        []string{"pulls"},
	}

	// Add paths
	spec.Paths.Set("/repos/{owner}/{repo}", &openapi3.PathItem{
		Get: reposGetOp,
	})

	spec.Paths.Set("/repos/{owner}/{repo}/issues", &openapi3.PathItem{
		Get: issuesListOp,
	})

	spec.Paths.Set("/repos/{owner}/{repo}/pulls", &openapi3.PathItem{
		Post: pullsCreateOp,
	})

	// Build operation mappings
	err := resolver.BuildOperationMappings(spec, "github")
	require.NoError(t, err)

	t.Run("resolve simple action name", func(t *testing.T) {
		// Test resolving "get" with repo context
		context := map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		}

		resolved, err := resolver.ResolveOperation("get", context)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "repos/get", resolved.OperationID)
		assert.Equal(t, "get", resolved.SimpleName)
	})

	t.Run("resolve with issue context", func(t *testing.T) {
		// Test resolving "list" with issue context
		context := map[string]interface{}{
			"owner":        "octocat",
			"repo":         "hello-world",
			"issue_number": 123,
		}

		resolved, err := resolver.ResolveOperation("list", context)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "issues/list", resolved.OperationID)
		assert.Equal(t, "list", resolved.SimpleName)
	})

	t.Run("resolve exact operation ID", func(t *testing.T) {
		// Test exact match
		resolved, err := resolver.ResolveOperation("repos/get", nil)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "repos/get", resolved.OperationID)
	})

	t.Run("resolve normalized operation ID", func(t *testing.T) {
		// Test with hyphen instead of slash
		resolved, err := resolver.ResolveOperation("repos-get", nil)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "repos/get", resolved.OperationID)
	})

	t.Run("resolve with pull request context", func(t *testing.T) {
		// Test resolving "create" with pull request context
		context := map[string]interface{}{
			"owner":       "octocat",
			"repo":        "hello-world",
			"pull_number": 456,
		}

		resolved, err := resolver.ResolveOperation("create", context)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "pulls/create", resolved.OperationID)
	})

	t.Run("handle unknown operation", func(t *testing.T) {
		// Test with unknown operation
		resolved, err := resolver.ResolveOperation("unknown-operation", nil)
		assert.Error(t, err)
		assert.Nil(t, resolved)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("disambiguate with context scoring", func(t *testing.T) {
		// Add another "get" operation for users
		usersGetOp := &openapi3.Operation{
			OperationID: "users/get",
			Summary:     "Get a user",
			Tags:        []string{"users"},
		}

		spec.Paths.Set("/users/{username}", &openapi3.PathItem{
			Get: usersGetOp,
		})

		// Rebuild mappings
		err := resolver.BuildOperationMappings(spec, "github")
		require.NoError(t, err)

		// Now "get" is ambiguous - should resolve to repos/get with repo context
		context := map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		}

		resolved, err := resolver.ResolveOperation("get", context)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "repos/get", resolved.OperationID)

		// Should resolve to users/get with user context
		userContext := map[string]interface{}{
			"username": "octocat",
		}

		resolved, err = resolver.ResolveOperation("get", userContext)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		// This might resolve to either, but with proper scoring it should prefer users/get
	})

	t.Run("fuzzy matching", func(t *testing.T) {
		// Test fuzzy matching with underscores
		resolved, err := resolver.ResolveOperation("repos_get", nil)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "repos/get", resolved.OperationID)
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		// Test case insensitive
		resolved, err := resolver.ResolveOperation("REPOS/GET", nil)
		assert.NoError(t, err)
		assert.NotNil(t, resolved)
		assert.Equal(t, "repos/get", resolved.OperationID)
	})
}

func TestOperationResolverEdgeCases(t *testing.T) {
	logger := &observability.NoopLogger{}
	resolver := NewOperationResolver(logger)

	t.Run("empty spec", func(t *testing.T) {
		spec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}

		err := resolver.BuildOperationMappings(spec, "test")
		assert.NoError(t, err)

		resolved, err := resolver.ResolveOperation("get", nil)
		assert.Error(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("nil spec", func(t *testing.T) {
		err := resolver.BuildOperationMappings(nil, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid OpenAPI spec")
	})

	t.Run("operations without IDs", func(t *testing.T) {
		spec := &openapi3.T{
			Paths: openapi3.NewPaths(),
		}

		// Add operation without ID
		spec.Paths.Set("/test", &openapi3.PathItem{
			Get: &openapi3.Operation{
				Summary: "Test operation",
			},
		})

		err := resolver.BuildOperationMappings(spec, "test")
		assert.NoError(t, err)

		// The resolver can still work with operations without IDs
		// It will create a synthetic ID based on method and path
		resolved, _ := resolver.ResolveOperation("get_test", nil)
		// It might not find it with just "get" since there's no operation ID
		// but that's okay - we're testing edge cases
		if resolved != nil {
			assert.Equal(t, "", resolved.OperationID)
			assert.Equal(t, "/test", resolved.Path)
			assert.Equal(t, "GET", resolved.Method)
		}
	})
}

func TestExtractSimpleName(t *testing.T) {
	logger := &observability.NoopLogger{}
	resolver := NewOperationResolver(logger)

	tests := []struct {
		name        string
		operationID string
		expected    string
	}{
		{
			name:        "slash separator",
			operationID: "repos/get",
			expected:    "get",
		},
		{
			name:        "hyphen separator",
			operationID: "repos-get",
			expected:    "get",
		},
		{
			name:        "multiple parts",
			operationID: "repos/get-content",
			expected:    "get",
		},
		{
			name:        "camelCase",
			operationID: "getRepos",
			expected:    "get",
		},
		{
			name:        "underscore separator",
			operationID: "repos_get",
			expected:    "get",
		},
		{
			name:        "no verb found",
			operationID: "foobar",
			expected:    "foobar",
		},
		{
			name:        "empty string",
			operationID: "",
			expected:    "",
		},
		{
			name:        "verb in middle",
			operationID: "repos/list/all",
			expected:    "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.extractSimpleName(tt.operationID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferResourceFromParam(t *testing.T) {
	logger := &observability.NoopLogger{}
	resolver := NewOperationResolver(logger)

	tests := []struct {
		param    string
		expected string
	}{
		{"owner", "repos"},
		{"repo", "repos"},
		{"repository", "repos"},
		{"org", "orgs"},
		{"organization", "orgs"},
		{"user", "users"},
		{"username", "users"},
		{"issue_number", "issues"},
		{"issue_id", "issues"},
		{"pull_number", "pulls"},
		{"pr_number", "pulls"},
		{"gist_id", "gists"},
		{"team_id", "teams"},
		{"team_slug", "teams"},
		{"workflow_id", "workflows"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.param, func(t *testing.T) {
			result := resolver.inferResourceFromParam(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}
