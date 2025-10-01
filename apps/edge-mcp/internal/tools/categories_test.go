package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryListByCategory tests category-based filtering
func TestRegistryListByCategory(t *testing.T) {
	registry := NewRegistry()

	// Create test tools with different categories
	tools := []ToolDefinition{
		{
			Name:        "repo_list",
			Description: "List repositories",
			Category:    string(CategoryRepository),
			Tags:        []string{string(CapabilityRead), string(CapabilityList)},
			Handler:     mockHandler,
		},
		{
			Name:        "repo_create",
			Description: "Create repository",
			Category:    string(CategoryRepository),
			Tags:        []string{string(CapabilityWrite), string(CapabilityValidation)},
			Handler:     mockHandler,
		},
		{
			Name:        "issue_list",
			Description: "List issues",
			Category:    string(CategoryIssues),
			Tags:        []string{string(CapabilityRead), string(CapabilityList)},
			Handler:     mockHandler,
		},
		{
			Name:        "workflow_execute",
			Description: "Execute workflow",
			Category:    string(CategoryWorkflow),
			Tags:        []string{string(CapabilityExecute), string(CapabilityAsync)},
			Handler:     mockHandler,
		},
	}

	// Register all tools
	for _, tool := range tools {
		registry.RegisterRemote(tool)
	}

	// Test ListByCategory
	t.Run("filter by repository category", func(t *testing.T) {
		filtered := registry.ListByCategory(string(CategoryRepository))
		assert.Len(t, filtered, 2)
		for _, tool := range filtered {
			assert.Equal(t, string(CategoryRepository), tool.Category)
		}
	})

	t.Run("filter by issues category", func(t *testing.T) {
		filtered := registry.ListByCategory(string(CategoryIssues))
		assert.Len(t, filtered, 1)
		assert.Equal(t, "issue_list", filtered[0].Name)
	})

	t.Run("filter by workflow category", func(t *testing.T) {
		filtered := registry.ListByCategory(string(CategoryWorkflow))
		assert.Len(t, filtered, 1)
		assert.Equal(t, "workflow_execute", filtered[0].Name)
	})

	t.Run("filter by non-existent category", func(t *testing.T) {
		filtered := registry.ListByCategory("non-existent")
		assert.Len(t, filtered, 0)
	})
}

// TestRegistryListByTags tests tag-based filtering
func TestRegistryListByTags(t *testing.T) {
	registry := NewRegistry()

	// Create test tools with different tags
	tools := []ToolDefinition{
		{
			Name:        "tool1",
			Description: "Read-only tool",
			Category:    string(CategoryGeneral),
			Tags:        []string{string(CapabilityRead)},
			Handler:     mockHandler,
		},
		{
			Name:        "tool2",
			Description: "Read and list tool",
			Category:    string(CategoryGeneral),
			Tags:        []string{string(CapabilityRead), string(CapabilityList)},
			Handler:     mockHandler,
		},
		{
			Name:        "tool3",
			Description: "Write tool",
			Category:    string(CategoryGeneral),
			Tags:        []string{string(CapabilityWrite), string(CapabilityValidation)},
			Handler:     mockHandler,
		},
		{
			Name:        "tool4",
			Description: "Async execution tool",
			Category:    string(CategoryGeneral),
			Tags:        []string{string(CapabilityExecute), string(CapabilityAsync)},
			Handler:     mockHandler,
		},
	}

	// Register all tools
	for _, tool := range tools {
		registry.RegisterRemote(tool)
	}

	// Test ListByTags
	t.Run("filter by single tag", func(t *testing.T) {
		filtered := registry.ListByTags([]string{string(CapabilityRead)})
		assert.Len(t, filtered, 2)
		for _, tool := range filtered {
			assert.Contains(t, tool.Tags, string(CapabilityRead))
		}
	})

	t.Run("filter by multiple tags", func(t *testing.T) {
		filtered := registry.ListByTags([]string{string(CapabilityRead), string(CapabilityList)})
		assert.Len(t, filtered, 1)
		assert.Equal(t, "tool2", filtered[0].Name)
	})

	t.Run("filter by write tag", func(t *testing.T) {
		filtered := registry.ListByTags([]string{string(CapabilityWrite)})
		assert.Len(t, filtered, 1)
		assert.Equal(t, "tool3", filtered[0].Name)
	})

	t.Run("filter by non-existent tag", func(t *testing.T) {
		filtered := registry.ListByTags([]string{"non-existent"})
		assert.Len(t, filtered, 0)
	})

	t.Run("empty tags filter", func(t *testing.T) {
		filtered := registry.ListByTags([]string{})
		// Empty tags should match all tools
		assert.Len(t, filtered, 4)
	})
}

// TestRegistryListWithFilter tests combined category and tag filtering
func TestRegistryListWithFilter(t *testing.T) {
	registry := NewRegistry()

	// Create test tools with different categories and tags
	tools := []ToolDefinition{
		{
			Name:        "repo_read",
			Description: "Read repository",
			Category:    string(CategoryRepository),
			Tags:        []string{string(CapabilityRead)},
			Handler:     mockHandler,
		},
		{
			Name:        "repo_write",
			Description: "Write repository",
			Category:    string(CategoryRepository),
			Tags:        []string{string(CapabilityWrite)},
			Handler:     mockHandler,
		},
		{
			Name:        "issue_read",
			Description: "Read issues",
			Category:    string(CategoryIssues),
			Tags:        []string{string(CapabilityRead)},
			Handler:     mockHandler,
		},
		{
			Name:        "issue_write",
			Description: "Write issues",
			Category:    string(CategoryIssues),
			Tags:        []string{string(CapabilityWrite)},
			Handler:     mockHandler,
		},
	}

	// Register all tools
	for _, tool := range tools {
		registry.RegisterRemote(tool)
	}

	// Test ListWithFilter
	t.Run("filter by category and tag", func(t *testing.T) {
		filtered := registry.ListWithFilter(string(CategoryRepository), []string{string(CapabilityRead)})
		assert.Len(t, filtered, 1)
		assert.Equal(t, "repo_read", filtered[0].Name)
	})

	t.Run("filter by category only", func(t *testing.T) {
		filtered := registry.ListWithFilter(string(CategoryRepository), []string{})
		assert.Len(t, filtered, 2)
		for _, tool := range filtered {
			assert.Equal(t, string(CategoryRepository), tool.Category)
		}
	})

	t.Run("filter by tags only", func(t *testing.T) {
		filtered := registry.ListWithFilter("", []string{string(CapabilityWrite)})
		assert.Len(t, filtered, 2)
		for _, tool := range filtered {
			assert.Contains(t, tool.Tags, string(CapabilityWrite))
		}
	})

	t.Run("no filters", func(t *testing.T) {
		filtered := registry.ListWithFilter("", []string{})
		assert.Len(t, filtered, 4)
	})

	t.Run("non-matching filters", func(t *testing.T) {
		filtered := registry.ListWithFilter(string(CategoryRepository), []string{string(CapabilityExecute)})
		assert.Len(t, filtered, 0)
	})
}

// TestHasAllTags tests the hasAllTags helper function
func TestHasAllTags(t *testing.T) {
	tests := []struct {
		name         string
		toolTags     []string
		requiredTags []string
		expected     bool
	}{
		{
			name:         "has all required tags",
			toolTags:     []string{"read", "write", "delete"},
			requiredTags: []string{"read", "write"},
			expected:     true,
		},
		{
			name:         "missing required tag",
			toolTags:     []string{"read", "write"},
			requiredTags: []string{"read", "write", "delete"},
			expected:     false,
		},
		{
			name:         "empty required tags",
			toolTags:     []string{"read", "write"},
			requiredTags: []string{},
			expected:     true,
		},
		{
			name:         "empty tool tags",
			toolTags:     []string{},
			requiredTags: []string{"read"},
			expected:     false,
		},
		{
			name:         "both empty",
			toolTags:     []string{},
			requiredTags: []string{},
			expected:     true,
		},
		{
			name:         "exact match",
			toolTags:     []string{"read", "write"},
			requiredTags: []string{"read", "write"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllTags(tt.toolTags, tt.requiredTags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCategoryConstants tests that category constants are properly defined
func TestCategoryConstants(t *testing.T) {
	// Verify all categories have descriptions
	for category, info := range CategoryDescriptions {
		t.Run(string(category), func(t *testing.T) {
			assert.NotEmpty(t, info.Name, "Category %s should have a name", category)
			assert.NotEmpty(t, info.Description, "Category %s should have a description", category)
			assert.GreaterOrEqual(t, info.Priority, 0, "Category %s should have non-negative priority", category)
		})
	}

	// Verify specific categories exist
	expectedCategories := []ToolCategory{
		CategoryRepository,
		CategoryIssues,
		CategoryPullRequests,
		CategoryCICD,
		CategoryWorkflow,
		CategoryTask,
		CategoryAgent,
		CategoryContext,
		CategoryMonitoring,
		CategorySecurity,
		CategoryCollaboration,
		CategoryDocumentation,
		CategoryAnalytics,
		CategoryTemplate,
		CategoryConfiguration,
		CategoryGeneral,
	}

	for _, category := range expectedCategories {
		_, exists := CategoryDescriptions[category]
		assert.True(t, exists, "Category %s should have a description", category)
	}
}

// TestGetCategoriesForAgent tests agent-specific category recommendations
func TestGetCategoriesForAgent(t *testing.T) {
	tests := []struct {
		agentType          string
		expectedCategories []ToolCategory
	}{
		{
			agentType:          "code_reviewer",
			expectedCategories: []ToolCategory{CategoryPullRequests, CategoryRepository, CategoryIssues, CategorySecurity},
		},
		{
			agentType:          "deployer",
			expectedCategories: []ToolCategory{CategoryCICD, CategoryWorkflow, CategoryMonitoring, CategoryConfiguration},
		},
		{
			agentType:          "tester",
			expectedCategories: []ToolCategory{CategoryCICD, CategoryMonitoring, CategoryAnalytics, CategorySecurity},
		},
		{
			agentType:          "architect",
			expectedCategories: []ToolCategory{CategoryRepository, CategoryDocumentation, CategoryTemplate, CategoryConfiguration},
		},
		{
			agentType:          "project_manager",
			expectedCategories: []ToolCategory{CategoryTask, CategoryIssues, CategoryCollaboration, CategoryAnalytics},
		},
		{
			agentType:          "unknown",
			expectedCategories: []ToolCategory{CategoryGeneral},
		},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			categories := GetCategoriesForAgent(tt.agentType)
			assert.Equal(t, tt.expectedCategories, categories)
		})
	}
}

// TestGetCapabilitiesForOperation tests operation-specific capability recommendations
func TestGetCapabilitiesForOperation(t *testing.T) {
	tests := []struct {
		operation            string
		expectedCapabilities []ToolCapability
	}{
		{
			operation:            "get",
			expectedCapabilities: []ToolCapability{CapabilityRead},
		},
		{
			operation:            "list",
			expectedCapabilities: []ToolCapability{CapabilityRead, CapabilityList, CapabilityFilter, CapabilitySort, CapabilityPaginate},
		},
		{
			operation:            "create",
			expectedCapabilities: []ToolCapability{CapabilityWrite, CapabilityValidation},
		},
		{
			operation:            "update",
			expectedCapabilities: []ToolCapability{CapabilityUpdate, CapabilityValidation, CapabilityPreview},
		},
		{
			operation:            "delete",
			expectedCapabilities: []ToolCapability{CapabilityDelete, CapabilityDryRun},
		},
		{
			operation:            "execute",
			expectedCapabilities: []ToolCapability{CapabilityExecute, CapabilityAsync},
		},
		{
			operation:            "search",
			expectedCapabilities: []ToolCapability{CapabilityRead, CapabilitySearch, CapabilityFilter},
		},
		{
			operation:            "batch",
			expectedCapabilities: []ToolCapability{CapabilityBatch, CapabilityAsync},
		},
		{
			operation:            "unknown",
			expectedCapabilities: []ToolCapability{CapabilityRead},
		},
	}

	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			capabilities := GetCapabilitiesForOperation(tt.operation)
			assert.Equal(t, tt.expectedCapabilities, capabilities)
		})
	}
}

// mockHandler is a simple mock handler for testing
func mockHandler(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"status": "success"}, nil
}

// TestToolDefinitionSerialization tests that tools with categories and tags serialize correctly
func TestToolDefinitionSerialization(t *testing.T) {
	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Category:    string(CategoryRepository),
		Tags:        []string{string(CapabilityRead), string(CapabilityList)},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(tool)
	require.NoError(t, err)

	// Deserialize back
	var deserialized ToolDefinition
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, tool.Name, deserialized.Name)
	assert.Equal(t, tool.Description, deserialized.Description)
	assert.Equal(t, tool.Category, deserialized.Category)
	assert.Equal(t, tool.Tags, deserialized.Tags)

	// Verify JSON contains expected fields
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"category":"repository"`)
	assert.Contains(t, jsonStr, `"tags":["read","list"]`)
}
