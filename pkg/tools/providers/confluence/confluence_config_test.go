package confluence

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfluenceProvider_ConfigurationManagement(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	t.Run("EnableToolset", func(t *testing.T) {
		// Enable a valid toolset
		err := provider.EnableToolset("pages")
		assert.NoError(t, err)
		assert.True(t, provider.IsToolsetEnabled("pages"))

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
		_ = provider.EnableToolset("pages")
		_ = provider.EnableToolset("search")
		_ = provider.EnableToolset("labels")

		enabled := provider.GetEnabledToolsets()
		assert.Contains(t, enabled, "pages")
		assert.Contains(t, enabled, "search")
		assert.Contains(t, enabled, "labels")

		// Disable one
		_ = provider.DisableToolset("search")
		enabled = provider.GetEnabledToolsets()
		assert.Contains(t, enabled, "pages")
		assert.NotContains(t, enabled, "search")
		assert.Contains(t, enabled, "labels")
	})

	t.Run("ConfigureFromContext with ENABLED_TOOLS", func(t *testing.T) {
		// Enable all toolsets first
		_ = provider.EnableToolset("pages")
		_ = provider.EnableToolset("search")
		_ = provider.EnableToolset("labels")

		// Create context with ENABLED_TOOLS metadata
		ctx := context.Background()
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"ENABLED_TOOLS": "pages,labels",
			},
		})

		// Apply configuration
		provider.ConfigureFromContext(ctx)

		// Only pages and labels should be enabled
		assert.True(t, provider.IsToolsetEnabled("pages"))
		assert.False(t, provider.IsToolsetEnabled("search"))
		assert.True(t, provider.IsToolsetEnabled("labels"))
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
		assert.True(t, provider.IsWriteOperation("page/create"))
		assert.True(t, provider.IsWriteOperation("page/update"))
		assert.True(t, provider.IsWriteOperation("publish"))
		assert.True(t, provider.IsWriteOperation("archive"))
		assert.True(t, provider.IsWriteOperation("upload"))
		assert.True(t, provider.IsWriteOperation("attach"))

		// Test read operations
		assert.False(t, provider.IsWriteOperation("get"))
		assert.False(t, provider.IsWriteOperation("list"))
		assert.False(t, provider.IsWriteOperation("search"))
		assert.False(t, provider.IsWriteOperation("find"))
		assert.False(t, provider.IsWriteOperation("page/get"))
	})

	t.Run("FilterSpaceResults", func(t *testing.T) {
		// Create test data with pages from different spaces
		testData := map[string]interface{}{
			"results": []interface{}{
				map[string]interface{}{
					"id": "1",
					"space": map[string]interface{}{
						"key": "SPACE1",
					},
				},
				map[string]interface{}{
					"id": "2",
					"space": map[string]interface{}{
						"key": "SPACE2",
					},
				},
				map[string]interface{}{
					"id": "3",
					"space": map[string]interface{}{
						"key": "SPACE3",
					},
				},
			},
			"size": 3,
		}

		// Test without filter
		ctx := context.Background()
		result := provider.FilterSpaceResults(ctx, testData)
		resultMap := result.(map[string]interface{})
		assert.Len(t, resultMap["results"], 3)

		// Test with filter allowing SPACE1 and SPACE2
		// Create a fresh copy of test data since it gets modified
		testDataCopy := map[string]interface{}{
			"results": []interface{}{
				map[string]interface{}{
					"id": "1",
					"space": map[string]interface{}{
						"key": "SPACE1",
					},
				},
				map[string]interface{}{
					"id": "2",
					"space": map[string]interface{}{
						"key": "SPACE2",
					},
				},
				map[string]interface{}{
					"id": "3",
					"space": map[string]interface{}{
						"key": "SPACE3",
					},
				},
			},
			"size": 3,
		}
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "SPACE1,SPACE2",
			},
		})
		result = provider.FilterSpaceResults(ctx, testDataCopy)
		resultMap = result.(map[string]interface{})
		assert.Len(t, resultMap["results"], 2)
		assert.Equal(t, 2, resultMap["size"])

		// Test with wildcard filter
		testDataCopy2 := map[string]interface{}{
			"results": []interface{}{
				map[string]interface{}{
					"id": "1",
					"space": map[string]interface{}{
						"key": "SPACE1",
					},
				},
				map[string]interface{}{
					"id": "2",
					"space": map[string]interface{}{
						"key": "SPACE2",
					},
				},
				map[string]interface{}{
					"id": "3",
					"space": map[string]interface{}{
						"key": "SPACE3",
					},
				},
			},
			"size": 3,
		}
		ctx = providers.WithContext(ctx, &providers.ProviderContext{
			Metadata: map[string]interface{}{
				"CONFLUENCE_SPACES_FILTER": "*",
			},
		})
		result = provider.FilterSpaceResults(ctx, testDataCopy2)
		resultMap = result.(map[string]interface{})
		assert.Len(t, resultMap["results"], 3)
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
		_, err := provider.ExecuteOperation(ctx, "content/create", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed in read-only mode")

		// Read operations should be allowed (but may fail for other reasons)
		// Using content/search which exists in the default mappings
		_, err = provider.ExecuteOperation(ctx, "content/search", map[string]interface{}{})
		// This will fail because we don't have a handler, but NOT because of read-only
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "read-only mode")
	})
}

func TestConfluenceProvider_SpaceFiltering(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	t.Run("filterSpaces", func(t *testing.T) {
		spaces := []interface{}{
			map[string]interface{}{"key": "SPACE1", "name": "Space 1"},
			map[string]interface{}{"key": "SPACE2", "name": "Space 2"},
			map[string]interface{}{"key": "SPACE3", "name": "Space 3"},
		}

		// Filter to only SPACE1 and SPACE3
		filtered := provider.filterSpaces(spaces, "SPACE1,SPACE3")
		assert.Len(t, filtered, 2)

		// Filter with wildcard
		filtered = provider.filterSpaces(spaces, "*")
		assert.Len(t, filtered, 3)

		// Filter with single space
		filtered = provider.filterSpaces(spaces, "SPACE2")
		assert.Len(t, filtered, 1)
	})

	t.Run("filterPagesBySpace", func(t *testing.T) {
		pages := []interface{}{
			map[string]interface{}{
				"id":    "1",
				"title": "Page 1",
				"space": map[string]interface{}{"key": "SPACE1"},
			},
			map[string]interface{}{
				"id":    "2",
				"title": "Page 2",
				"space": map[string]interface{}{"key": "SPACE2"},
			},
			map[string]interface{}{
				"id":    "3",
				"title": "Page 3",
				"space": map[string]interface{}{"key": "SPACE3"},
			},
		}

		// Filter to only SPACE1 and SPACE3
		filtered := provider.filterPagesBySpace(pages, "SPACE1,SPACE3")
		assert.Len(t, filtered, 2)
	})

	t.Run("isSpaceAllowed", func(t *testing.T) {
		allowed := []string{"SPACE1", "SPACE2", "SPACE3"}

		assert.True(t, provider.isSpaceAllowed("SPACE1", allowed))
		assert.True(t, provider.isSpaceAllowed("SPACE2", allowed))
		assert.False(t, provider.isSpaceAllowed("SPACE4", allowed))

		// Test with wildcard
		allowedWithWildcard := []string{"*"}
		assert.True(t, provider.isSpaceAllowed("ANY_SPACE", allowedWithWildcard))
	})

	t.Run("filterItemsBySpace", func(t *testing.T) {
		// Test with spaceKey field
		items := []interface{}{
			map[string]interface{}{"id": "1", "spaceKey": "SPACE1"},
			map[string]interface{}{"id": "2", "spaceKey": "SPACE2"},
			map[string]interface{}{"id": "3", "spaceKey": "SPACE3"},
		}

		filtered := provider.filterItemsBySpace(items, "SPACE1,SPACE2")
		assert.Len(t, filtered, 2)

		// Test with nested space field
		itemsNested := []interface{}{
			map[string]interface{}{
				"id":    "1",
				"space": map[string]interface{}{"key": "SPACE1"},
			},
			map[string]interface{}{
				"id":    "2",
				"space": map[string]interface{}{"key": "SPACE2"},
			},
			map[string]interface{}{
				"id":    "3",
				"space": map[string]interface{}{"key": "SPACE3"},
			},
		}

		filteredNested := provider.filterItemsBySpace(itemsNested, "SPACE1,SPACE3")
		assert.Len(t, filteredNested, 2)
	})
}
