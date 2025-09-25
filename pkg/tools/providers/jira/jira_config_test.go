package jira

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJiraProvider_ConfigurationManagement(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-domain")

	t.Run("EnableToolset", func(t *testing.T) {
		// Enable a valid toolset
		err := provider.EnableToolset("issues")
		assert.NoError(t, err)
		assert.True(t, provider.IsToolsetEnabled("issues"))

		// Try to enable non-existent toolset
		err = provider.EnableToolset("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolset nonexistent not found")
	})

	t.Run("DisableToolset", func(t *testing.T) {
		// Enable then disable a toolset
		err := provider.EnableToolset("search")
		require.NoError(t, err)
		assert.True(t, provider.IsToolsetEnabled("search"))

		err = provider.DisableToolset("search")
		assert.NoError(t, err)
		assert.False(t, provider.IsToolsetEnabled("search"))

		// Try to disable non-existent toolset
		err = provider.DisableToolset("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolset nonexistent not found")
	})

	t.Run("GetEnabledToolsets", func(t *testing.T) {
		// Enable multiple toolsets
		_ = provider.EnableToolset("issues")
		_ = provider.EnableToolset("search")

		enabled := provider.GetEnabledToolsets()
		assert.Contains(t, enabled, "issues")
		assert.Contains(t, enabled, "search")

		// Disable one
		_ = provider.DisableToolset("search")
		enabled = provider.GetEnabledToolsets()
		assert.Contains(t, enabled, "issues")
		assert.NotContains(t, enabled, "search")
	})

	t.Run("ConfigureFromContext with ENABLED_TOOLS", func(t *testing.T) {
		// Enable all toolsets first
		_ = provider.EnableToolset("issues")
		_ = provider.EnableToolset("search")

		// Create context with ENABLED_TOOLS metadata
		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"ENABLED_TOOLS": "search",
			},
		})

		// Apply configuration
		provider.ConfigureFromContext(ctx)

		// Only search should be enabled
		assert.False(t, provider.IsToolsetEnabled("issues"))
		assert.True(t, provider.IsToolsetEnabled("search"))
	})

	t.Run("IsReadOnlyMode", func(t *testing.T) {
		// Test without READ_ONLY flag
		ctx := context.Background()
		assert.False(t, provider.IsReadOnlyMode(ctx))

		// Test with READ_ONLY flag set to false
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"READ_ONLY": false,
			},
		})
		assert.False(t, provider.IsReadOnlyMode(ctx))

		// Test with READ_ONLY flag set to true
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"READ_ONLY": true,
			},
		})
		assert.True(t, provider.IsReadOnlyMode(ctx))
	})

	t.Run("IsWriteOperation", func(t *testing.T) {
		// Test write operations
		assert.True(t, provider.IsWriteOperation("create"))
		assert.True(t, provider.IsWriteOperation("update"))
		assert.True(t, provider.IsWriteOperation("delete"))
		assert.True(t, provider.IsWriteOperation("issue/create"))
		assert.True(t, provider.IsWriteOperation("issue/update"))
		assert.True(t, provider.IsWriteOperation("transition"))
		assert.True(t, provider.IsWriteOperation("assign"))

		// Test read operations
		assert.False(t, provider.IsWriteOperation("get"))
		assert.False(t, provider.IsWriteOperation("list"))
		assert.False(t, provider.IsWriteOperation("search"))
		assert.False(t, provider.IsWriteOperation("find"))
		assert.False(t, provider.IsWriteOperation("issue/get"))
	})

	t.Run("FilterProjectResults", func(t *testing.T) {
		// Create test data with issues from different projects
		testData := map[string]interface{}{
			"issues": []interface{}{
				map[string]interface{}{
					"id": "1",
					"fields": map[string]interface{}{
						"project": map[string]interface{}{
							"key": "PROJ1",
						},
					},
				},
				map[string]interface{}{
					"id": "2",
					"fields": map[string]interface{}{
						"project": map[string]interface{}{
							"key": "PROJ2",
						},
					},
				},
				map[string]interface{}{
					"id": "3",
					"fields": map[string]interface{}{
						"project": map[string]interface{}{
							"key": "PROJ3",
						},
					},
				},
			},
			"total": 3,
		}

		// Test without filter
		ctx := context.Background()
		result := provider.FilterProjectResults(ctx, testData)
		resultMap := result.(map[string]interface{})
		assert.Len(t, resultMap["issues"], 3)

		// Test with filter allowing PROJ1 and PROJ2
		// Create a fresh copy of test data since it gets modified
		testDataCopy := map[string]interface{}{
			"issues": []interface{}{
				map[string]interface{}{
					"id": "1",
					"fields": map[string]interface{}{
						"project": map[string]interface{}{
							"key": "PROJ1",
						},
					},
				},
				map[string]interface{}{
					"id": "2",
					"fields": map[string]interface{}{
						"project": map[string]interface{}{
							"key": "PROJ2",
						},
					},
				},
				map[string]interface{}{
					"id": "3",
					"fields": map[string]interface{}{
						"project": map[string]interface{}{
							"key": "PROJ3",
						},
					},
				},
			},
			"total": 3,
		}
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"JIRA_PROJECTS_FILTER": "PROJ1,PROJ2",
			},
		})
		result = provider.FilterProjectResults(ctx, testDataCopy)
		resultMap = result.(map[string]interface{})
		assert.Len(t, resultMap["issues"], 2)
		assert.Equal(t, 2, resultMap["total"])

		// Test with wildcard filter
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"JIRA_PROJECTS_FILTER": "*",
			},
		})
		result = provider.FilterProjectResults(ctx, testData)
		resultMap = result.(map[string]interface{})
		assert.Len(t, resultMap["issues"], 3)
	})

	t.Run("ExecuteOperation with READ_ONLY mode", func(t *testing.T) {
		// Set up context with READ_ONLY mode
		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"READ_ONLY": true,
			},
		})

		// Try to execute a write operation that exists in mappings
		_, err := provider.ExecuteOperation(ctx, "issues/create", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed in read-only mode")

		// Read operations should be allowed (but may fail for other reasons)
		// Using issues/search which exists in the default mappings
		_, err = provider.ExecuteOperation(ctx, "issues/search", map[string]interface{}{})
		// This will fail because we don't have a handler, but NOT because of read-only
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "read-only mode")
	})
}

func TestJiraProvider_ProjectFiltering(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test-domain")

	t.Run("filterProjects", func(t *testing.T) {
		projects := []interface{}{
			map[string]interface{}{"key": "PROJ1", "name": "Project 1"},
			map[string]interface{}{"key": "PROJ2", "name": "Project 2"},
			map[string]interface{}{"key": "PROJ3", "name": "Project 3"},
		}

		// Filter to only PROJ1 and PROJ3
		filtered := provider.filterProjects(projects, "PROJ1,PROJ3")
		assert.Len(t, filtered, 2)

		// Filter with wildcard
		filtered = provider.filterProjects(projects, "*")
		assert.Len(t, filtered, 3)

		// Filter with single project
		filtered = provider.filterProjects(projects, "PROJ2")
		assert.Len(t, filtered, 1)
	})

	t.Run("isProjectAllowed", func(t *testing.T) {
		allowed := []string{"PROJ1", "PROJ2", "PROJ3"}

		assert.True(t, provider.isProjectAllowed("PROJ1", allowed))
		assert.True(t, provider.isProjectAllowed("PROJ2", allowed))
		assert.False(t, provider.isProjectAllowed("PROJ4", allowed))

		// Test with wildcard
		allowedWithWildcard := []string{"*"}
		assert.True(t, provider.isProjectAllowed("ANY_PROJECT", allowedWithWildcard))
	})
}
