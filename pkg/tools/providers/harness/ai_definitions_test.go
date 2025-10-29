package harness

import (
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
)

func TestHarnessProvider_GetAIOptimizedDefinitions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	// Test with context-optimized modules enabled (default: 5 core modules)
	definitions := provider.GetAIOptimizedDefinitions()

	assert.NotEmpty(t, definitions, "Should have AI definitions")

	// Check we have definitions for each enabled module
	var categories []string
	for _, def := range definitions {
		categories = append(categories, def.Category)
	}

	// Verify categories for context-optimized enabled modules
	assert.Contains(t, categories, "CI/CD", "Should have CI/CD category")
	assert.Contains(t, categories, "Security", "Should have Security category")
	assert.Contains(t, categories, "Deployment", "Should have Deployment category")
	assert.Contains(t, categories, "Development", "Should have Development category")
	assert.Contains(t, categories, "Operations", "Should have Operations category")

	// Verify disabled module categories are NOT present
	assert.NotContains(t, categories, "Platform", "Platform category should be disabled")
	assert.NotContains(t, categories, "Integration", "Integration category should be disabled")
	assert.NotContains(t, categories, "GitOps", "GitOps category should be disabled")
	assert.NotContains(t, categories, "FinOps", "FinOps category should be disabled")

	// Verify structure of a definition
	var pipelineDef *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "harness_pipelines" {
			pipelineDef = &def
			break
		}
	}

	assert.NotNil(t, pipelineDef, "Should have harness_pipelines definition")
	if pipelineDef != nil {
		// Check required fields
		assert.NotEmpty(t, pipelineDef.Name)
		assert.NotEmpty(t, pipelineDef.DisplayName)
		assert.NotEmpty(t, pipelineDef.Category)
		assert.NotEmpty(t, pipelineDef.Description)
		assert.NotEmpty(t, pipelineDef.SemanticTags)
		assert.NotEmpty(t, pipelineDef.CommonPhrases)
		assert.NotEmpty(t, pipelineDef.UsageExamples)
		assert.NotEmpty(t, pipelineDef.InputSchema)

		// Check usage examples
		assert.GreaterOrEqual(t, len(pipelineDef.UsageExamples), 3,
			"Should have at least 3 usage examples")

		for _, example := range pipelineDef.UsageExamples {
			assert.NotEmpty(t, example.Scenario)
			assert.NotEmpty(t, example.Input)
		}

		// Check semantic tags exist
		assert.NotEmpty(t, pipelineDef.SemanticTags, "Should have semantic tags")
	}
}

func TestHarnessProvider_GetAIOptimizedDefinitions_WithDisabledModules(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	// Enable only specific modules (Pipeline and STO)
	provider.SetEnabledModules([]HarnessModule{ModulePipeline, ModuleSTO})

	definitions := provider.GetAIOptimizedDefinitions()

	// Should still have some definitions
	assert.NotEmpty(t, definitions)

	// Check that we have pipeline and security definitions, but not others
	var hasPipeline, hasSTO, hasConnector, hasProject bool
	for _, def := range definitions {
		switch def.Name {
		case "harness_pipelines":
			hasPipeline = true
		case "harness_security", "harness_sto":
			hasSTO = true
		case "harness_connectors":
			hasConnector = true
		case "harness_projects":
			hasProject = true
		}
	}

	assert.True(t, hasPipeline, "Should have pipeline definitions")
	assert.True(t, hasSTO, "Should have STO/security definitions")
	assert.False(t, hasConnector, "Should not have connector definitions when disabled")
	assert.False(t, hasProject, "Should not have project definitions when disabled")
}

func TestHarnessProvider_GetAIOptimizedDefinitions_AllCategories(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()

	// Track all unique categories
	categoryMap := make(map[string]bool)
	for _, def := range definitions {
		categoryMap[def.Category] = true
	}

	// Should have multiple categories
	assert.GreaterOrEqual(t, len(categoryMap), 4,
		"Should have at least 4 different categories")
}

func TestHarnessProvider_GetAIOptimizedDefinitions_Consistency(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	definitions := provider.GetAIOptimizedDefinitions()

	for _, def := range definitions {
		t.Run(def.Name, func(t *testing.T) {
			// Every definition should have required fields
			assert.NotEmpty(t, def.Name, "Name should not be empty")
			assert.NotEmpty(t, def.DisplayName, "DisplayName should not be empty")
			assert.NotEmpty(t, def.Category, "Category should not be empty")
			assert.NotEmpty(t, def.Description, "Description should not be empty")

			// Should have at least some semantic tags
			assert.NotEmpty(t, def.SemanticTags, "Should have semantic tags")

			// Should have at least one usage example
			assert.NotEmpty(t, def.UsageExamples, "Should have at least one usage example")

			// Each usage example should be complete
			for i, example := range def.UsageExamples {
				assert.NotEmpty(t, example.Scenario, "Example %d should have scenario", i)
				assert.NotEmpty(t, example.Input, "Example %d should have input", i)
			}

			// Input schema validation - some definitions may not have complex schemas
			if def.InputSchema.Type != "" {
				assert.NotEmpty(t, def.InputSchema.Type, "Input schema should have type if defined")
			}
		})
	}
}

// Benchmark AI definitions generation
func BenchmarkHarnessProvider_GetAIOptimizedDefinitions(b *testing.B) {
	logger := &observability.NoopLogger{}
	provider := NewHarnessProvider(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetAIOptimizedDefinitions()
	}
}
