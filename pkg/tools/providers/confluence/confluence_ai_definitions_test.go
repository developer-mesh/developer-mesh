package confluence

import (
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfluenceProvider_GetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	// Initialize provider to register handlers

	// Get AI definitions with default enabled toolsets
	definitions := provider.GetAIOptimizedDefinitions()

	assert.NotEmpty(t, definitions, "Should have AI definitions")

	// Track which categories and tools we found
	foundCategories := make(map[string]bool)
	foundTools := make(map[string]bool)

	for _, def := range definitions {
		foundCategories[def.Category] = true
		foundTools[def.Name] = true
	}

	// Should have definitions for major categories
	assert.True(t, foundCategories["Documentation"], "Should have Documentation category")
	assert.True(t, foundCategories["Search"], "Should have Search category")
	assert.True(t, foundCategories["Organization"], "Should have Organization category")
	assert.True(t, foundCategories["Content"], "Should have Content category")
	assert.True(t, foundCategories["Metadata"], "Should have Metadata category")
	assert.True(t, foundCategories["Help"], "Should have Help category")

	// Should have specific tool definitions
	assert.True(t, foundTools["confluence_pages"], "Should have pages definition")
	assert.True(t, foundTools["confluence_search"], "Should have search definition")
	assert.True(t, foundTools["confluence_spaces"], "Should have spaces definition")
	assert.True(t, foundTools["confluence_content"], "Should have content definition")
	assert.True(t, foundTools["confluence_labels"], "Should have labels definition")
	assert.True(t, foundTools["confluence_errors"], "Should have error handling definition")
	assert.True(t, foundTools["confluence_bestpractices"], "Should have best practices definition")
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_PageDefinition(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	// Find the pages definition
	var pagesDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "confluence_pages" {
			pagesDef = &def
			break
		}
	}

	require.NotNil(t, pagesDef, "Should have confluence_pages definition")

	// Verify structure
	assert.Equal(t, "confluence_pages", pagesDef.Name)
	assert.Equal(t, "Confluence Pages", pagesDef.DisplayName)
	assert.Equal(t, "Documentation", pagesDef.Category)
	assert.Equal(t, "Page Management", pagesDef.Subcategory)
	assert.NotEmpty(t, pagesDef.Description)
	assert.NotEmpty(t, pagesDef.DetailedHelp)
	assert.Contains(t, pagesDef.DetailedHelp, "Create new pages")

	// Check usage examples
	assert.GreaterOrEqual(t, len(pagesDef.UsageExamples), 4, "Should have at least 4 usage examples")

	for _, example := range pagesDef.UsageExamples {
		assert.NotEmpty(t, example.Scenario, "Example should have scenario")
		assert.NotEmpty(t, example.Input, "Example should have input")
		assert.NotEmpty(t, example.Explanation, "Example should have explanation")

		// Verify input structure
		assert.NotEmpty(t, example.Input["action"], "Input should have action")
	}

	// Check semantic tags
	assert.NotEmpty(t, pagesDef.SemanticTags)
	assert.Contains(t, pagesDef.SemanticTags, "page")
	assert.Contains(t, pagesDef.SemanticTags, "documentation")
	assert.Contains(t, pagesDef.SemanticTags, "confluence")

	// Check common phrases
	assert.NotEmpty(t, pagesDef.CommonPhrases)
	assert.Contains(t, pagesDef.CommonPhrases, "create page")
	assert.Contains(t, pagesDef.CommonPhrases, "update wiki")

	// Check input schema
	assert.Equal(t, "object", pagesDef.InputSchema.Type)
	assert.NotEmpty(t, pagesDef.InputSchema.Properties)
	assert.Contains(t, pagesDef.InputSchema.Properties, "action")
	assert.Contains(t, pagesDef.InputSchema.Properties, "pageId")
	assert.Contains(t, pagesDef.InputSchema.Properties, "spaceKey")
	assert.Contains(t, pagesDef.InputSchema.Required, "action")

	// Check capabilities
	assert.NotNil(t, pagesDef.Capabilities)
	assert.NotEmpty(t, pagesDef.Capabilities.Capabilities)

	// Should have CRUD capabilities for pages
	hasCreate, hasRead, hasUpdate, hasDelete := false, false, false, false
	for _, cap := range pagesDef.Capabilities.Capabilities {
		if cap.Resource == "pages" {
			switch cap.Action {
			case "create":
				hasCreate = true
			case "read":
				hasRead = true
			case "update":
				hasUpdate = true
			case "delete":
				hasDelete = true
			}
		}
	}
	assert.True(t, hasCreate, "Should have create capability")
	assert.True(t, hasRead, "Should have read capability")
	assert.True(t, hasUpdate, "Should have update capability")
	assert.True(t, hasDelete, "Should have delete capability")

	// Check complexity level
	assert.Equal(t, "simple", pagesDef.ComplexityLevel)
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_SearchDefinition(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	// Find the search definition
	var searchDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "confluence_search" {
			searchDef = &def
			break
		}
	}

	require.NotNil(t, searchDef, "Should have confluence_search definition")

	// Verify search-specific details
	assert.Equal(t, "Search", searchDef.Category)
	assert.Equal(t, "Content Discovery", searchDef.Subcategory)
	assert.Contains(t, searchDef.Description, "CQL")
	assert.Contains(t, searchDef.DetailedHelp, "Confluence Query Language")

	// Check CQL examples
	assert.GreaterOrEqual(t, len(searchDef.UsageExamples), 3, "Should have CQL examples")

	// Verify at least one example contains CQL
	hasCQLExample := false
	for _, example := range searchDef.UsageExamples {
		if cql, exists := example.Input["cql"]; exists && cql != nil {
			hasCQLExample = true
			break
		}
	}
	assert.True(t, hasCQLExample, "Should have at least one CQL example")

	// Check that CQL is in the input schema
	assert.Contains(t, searchDef.InputSchema.Properties, "cql")
	assert.Contains(t, searchDef.InputSchema.Required, "cql")

	// Check complexity level (search is more complex)
	assert.Equal(t, "moderate", searchDef.ComplexityLevel)
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_ErrorHandling(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	// Find the error handling definition
	var errorDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "confluence_errors" {
			errorDef = &def
			break
		}
	}

	require.NotNil(t, errorDef, "Should have confluence_errors definition")

	// Verify error handling content
	assert.Equal(t, "Help", errorDef.Category)
	assert.Equal(t, "Error Resolution", errorDef.Subcategory)
	assert.Contains(t, errorDef.DetailedHelp, "401")
	assert.Contains(t, errorDef.DetailedHelp, "403")
	assert.Contains(t, errorDef.DetailedHelp, "404")
	assert.Contains(t, errorDef.DetailedHelp, "409")
	assert.Contains(t, errorDef.DetailedHelp, "Version Conflict")

	// Check error examples
	assert.NotEmpty(t, errorDef.UsageExamples)

	// Verify error resolution examples
	for _, example := range errorDef.UsageExamples {
		assert.NotEmpty(t, example.Input["error"], "Error example should have error field")
		assert.NotEmpty(t, example.Input["resolution"], "Error example should have resolution")
	}
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_WithDisabledToolsets(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	// Disable some toolsets
	_ = provider.DisableToolset("pages")
	_ = provider.DisableToolset("search")

	definitions := provider.GetAIOptimizedDefinitions()

	// Should still have some definitions (labels, content, help, etc.)
	assert.NotEmpty(t, definitions)

	// Check that disabled toolsets are not in definitions
	for _, def := range definitions {
		assert.NotEqual(t, "confluence_pages", def.Name, "Pages should be disabled")
		assert.NotEqual(t, "confluence_search", def.Name, "Search should be disabled")
	}

	// Should still have enabled toolsets
	hasLabels := false
	hasContent := false
	for _, def := range definitions {
		if def.Name == "confluence_labels" {
			hasLabels = true
		}
		if def.Name == "confluence_content" {
			hasContent = true
		}
	}
	assert.True(t, hasLabels, "Labels should still be enabled")
	assert.True(t, hasContent, "Content should still be enabled")
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_ContentOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	// Find the content operations definition
	var contentDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "confluence_content" {
			contentDef = &def
			break
		}
	}

	require.NotNil(t, contentDef, "Should have confluence_content definition")

	// Verify it's marked as complex (v1 operations)
	assert.Equal(t, "complex", contentDef.ComplexityLevel)
	assert.Equal(t, "Advanced Operations", contentDef.Subcategory)

	// Should mention v1 API
	assert.Contains(t, contentDef.DetailedHelp, "V1 API")

	// Should have attachment examples
	hasAttachmentExample := false
	for _, example := range contentDef.UsageExamples {
		if action, ok := example.Input["action"].(string); ok && action == "get_attachments" {
			hasAttachmentExample = true
			break
		}
	}
	assert.True(t, hasAttachmentExample, "Should have attachment example")
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_BestPractices(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	// Find the best practices definition
	var bestPracticesDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "confluence_bestpractices" {
			bestPracticesDef = &def
			break
		}
	}

	require.NotNil(t, bestPracticesDef, "Should have confluence_bestpractices definition")

	// Verify best practices content
	assert.Equal(t, "Help", bestPracticesDef.Category)
	assert.Equal(t, "Guidelines", bestPracticesDef.Subcategory)

	// Should cover key areas
	assert.Contains(t, bestPracticesDef.DetailedHelp, "Performance")
	assert.Contains(t, bestPracticesDef.DetailedHelp, "Security")
	assert.Contains(t, bestPracticesDef.DetailedHelp, "Error Handling")
	assert.Contains(t, bestPracticesDef.DetailedHelp, "API Version")
	assert.Contains(t, bestPracticesDef.DetailedHelp, "v1")
	assert.Contains(t, bestPracticesDef.DetailedHelp, "v2")
}

func TestConfluenceProvider_GetAIOptimizedDefinitions_LabelOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewConfluenceProvider(logger, "test-domain")

	definitions := provider.GetAIOptimizedDefinitions()

	// Find the label operations definition
	var labelDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "confluence_labels" {
			labelDef = &def
			break
		}
	}

	require.NotNil(t, labelDef, "Should have confluence_labels definition")

	// Verify label-specific details
	assert.Equal(t, "Metadata", labelDef.Category)
	assert.Equal(t, "Tagging", labelDef.Subcategory)

	// Check for label operations
	assert.Contains(t, labelDef.Description, "categorization")
	assert.Contains(t, labelDef.DetailedHelp, "taxonomies")

	// Should have add/remove/get examples
	hasAdd, hasRemove, hasGet := false, false, false
	for _, example := range labelDef.UsageExamples {
		if action, ok := example.Input["action"].(string); ok {
			switch action {
			case "add":
				hasAdd = true
			case "remove":
				hasRemove = true
			case "get":
				hasGet = true
			}
		}
	}
	assert.True(t, hasAdd, "Should have add label example")
	assert.True(t, hasRemove, "Should have remove label example")
	assert.True(t, hasGet, "Should have get labels example")
}
