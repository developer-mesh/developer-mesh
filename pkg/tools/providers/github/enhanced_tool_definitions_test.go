package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsageExamplesStructure validates that all usage examples are properly structured
func TestUsageExamplesStructure(t *testing.T) {
	issueTools := GetEnhancedIssueToolDefinitions()
	prTools := GetEnhancedPullRequestToolDefinitions()

	allTools := append(issueTools, prTools...)

	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) {
			// Skip if no examples defined
			if len(tool.UsageExamples) == 0 {
				t.Skipf("No usage examples defined for %s", tool.Name)
			}

			// Validate each example
			for i, example := range tool.UsageExamples {
				t.Run(example.Name, func(t *testing.T) {
					// Validate required fields
					assert.NotEmpty(t, example.Name, "Example %d: name should not be empty", i)
					assert.NotEmpty(t, example.Description, "Example %d: description should not be empty", i)
					assert.NotNil(t, example.Input, "Example %d: input should not be nil", i)

					// Validate example name follows convention
					validNames := map[string]bool{
						"simple":     true,
						"complex":    true,
						"error_case": true,
						"advanced":   true,
						"minimal":    true,
					}
					assert.True(t, validNames[example.Name], "Example %d: name '%s' should be one of: simple, complex, error_case, advanced, minimal", i, example.Name)

					// Validate input has required fields from schema (skip for error cases)
					if example.Name != "error_case" {
						validateExampleInput(t, tool, example)
					}

					// Validate error cases have ExpectedError
					if example.Name == "error_case" {
						assert.NotNil(t, example.ExpectedError, "Error case example should have ExpectedError defined")
						if example.ExpectedError != nil {
							assert.NotZero(t, example.ExpectedError.Code, "ExpectedError should have a non-zero code")
							assert.NotEmpty(t, example.ExpectedError.Reason, "ExpectedError should have a reason")
							assert.NotEmpty(t, example.ExpectedError.Solution, "ExpectedError should have a solution")
						}
						assert.Nil(t, example.ExpectedOutput, "Error case should not have ExpectedOutput")
					} else {
						// Non-error cases should have ExpectedOutput
						assert.NotNil(t, example.ExpectedOutput, "Non-error example should have ExpectedOutput defined")
						assert.Nil(t, example.ExpectedError, "Non-error case should not have ExpectedError")
					}
				})
			}
		})
	}
}

// TestExampleCoverage ensures tools with examples have proper coverage
func TestExampleCoverage(t *testing.T) {
	issueTools := GetEnhancedIssueToolDefinitions()
	prTools := GetEnhancedPullRequestToolDefinitions()

	allTools := append(issueTools, prTools...)

	// Tools that are expected to have examples (for Story 2.1.2 demonstration)
	expectedToolsWithExamples := map[string]bool{
		"get_issue":        true,
		"list_issues":      true,
		"create_issue":     true,
		"get_pull_request": true,
	}

	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) {
			// Skip tools that aren't expected to have examples yet
			if !expectedToolsWithExamples[tool.Name] && len(tool.UsageExamples) == 0 {
				t.Skipf("Tool %s doesn't have examples yet (not required for Story 2.1.2)", tool.Name)
				return
			}

			// If a tool has examples, it should have at least 2
			if len(tool.UsageExamples) > 0 {
				assert.GreaterOrEqual(t, len(tool.UsageExamples), 2,
					"Tool %s should have at least 2 usage examples", tool.Name)

				// Check for required example types
				hasSimple := false
				hasComplex := false
				hasError := false

				for _, example := range tool.UsageExamples {
					switch example.Name {
					case "simple":
						hasSimple = true
					case "complex":
						hasComplex = true
					case "error_case":
						hasError = true
					}
				}

				assert.True(t, hasSimple, "Tool %s should have a 'simple' example", tool.Name)
				assert.True(t, hasComplex || hasError,
					"Tool %s should have either a 'complex' or 'error_case' example", tool.Name)
			}
		})
	}
}

// validateExampleInput validates that the example input matches the tool's input schema
func validateExampleInput(t *testing.T, tool EnhancedToolDefinition, example UsageExample) {
	// Get required fields from schema
	schemaMap := tool.InputSchema
	if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
		// Check required fields
		if required, ok := schemaMap["required"].([]interface{}); ok {
			for _, reqField := range required {
				if fieldName, ok := reqField.(string); ok {
					// Error cases might intentionally omit required fields
					if example.Name != "error_case" {
						assert.Contains(t, example.Input, fieldName,
							"Example '%s' is missing required field '%s'", example.Name, fieldName)
					}
				}
			}
		}

		// Validate field types match schema
		for key, value := range example.Input {
			if propSchema, exists := props[key]; exists {
				if propMap, ok := propSchema.(map[string]interface{}); ok {
					validateFieldType(t, key, value, propMap)
				}
			}
		}
	}
}

// validateFieldType validates that a field value matches its schema type
func validateFieldType(t *testing.T, fieldName string, value interface{}, schema map[string]interface{}) {
	expectedType, hasType := schema["type"].(string)
	if !hasType {
		return
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		assert.True(t, ok, "Field '%s' should be a string, got %T", fieldName, value)

	case "integer":
		switch v := value.(type) {
		case int, int32, int64:
			// Valid integer types
			if min, hasMin := schema["minimum"].(float64); hasMin {
				assert.GreaterOrEqual(t, float64(v.(int)), min,
					"Field '%s' value %v is below minimum %v", fieldName, v, min)
			}
			if max, hasMax := schema["maximum"].(float64); hasMax {
				assert.LessOrEqual(t, float64(v.(int)), max,
					"Field '%s' value %v is above maximum %v", fieldName, v, max)
			}
		default:
			assert.Fail(t, "Field '%s' should be an integer, got %T", fieldName, value)
		}

	case "array":
		_, ok := value.([]interface{})
		assert.True(t, ok, "Field '%s' should be an array, got %T", fieldName, value)

	case "object":
		_, ok := value.(map[string]interface{})
		assert.True(t, ok, "Field '%s' should be an object, got %T", fieldName, value)
	}
}

// TestExampleConsistency checks that examples follow consistent patterns
func TestExampleConsistency(t *testing.T) {
	issueTools := GetEnhancedIssueToolDefinitions()
	prTools := GetEnhancedPullRequestToolDefinitions()

	allTools := append(issueTools, prTools...)

	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) {
			for _, example := range tool.UsageExamples {
				// Check that Notes field provides value
				if example.Notes != "" {
					assert.Greater(t, len(example.Notes), 10,
						"Notes should be meaningful (>10 chars) for example '%s'", example.Name)
				}

				// Check ExpectedOutput structure for non-error cases
				if example.ExpectedOutput != nil {
					switch output := example.ExpectedOutput.(type) {
					case map[string]interface{}:
						// Single object output - should have some fields
						assert.NotEmpty(t, output, "ExpectedOutput should not be empty")
					case []interface{}:
						// Array output - should have at least one item
						assert.NotEmpty(t, output, "ExpectedOutput array should not be empty")
					default:
						// Simple types are OK too
					}
				}
			}
		})
	}
}

// TestExampleDocumentation verifies examples can be used for documentation
func TestExampleDocumentation(t *testing.T) {
	issueTools := GetEnhancedIssueToolDefinitions()

	// Just test one tool as an example
	if len(issueTools) > 0 {
		tool := issueTools[0]
		require.NotEmpty(t, tool.UsageExamples, "First tool should have examples")

		// Verify we can generate documentation from examples
		for _, example := range tool.UsageExamples {
			// Description should be clear and complete
			assert.Contains(t, example.Description, " ",
				"Description should be a complete sentence")

			// Input should be documentable
			assert.IsType(t, map[string]interface{}{}, example.Input,
				"Input should be a map for documentation")
		}
	}
}

// TestErrorExamples specifically validates error case examples
func TestErrorExamples(t *testing.T) {
	issueTools := GetEnhancedIssueToolDefinitions()
	prTools := GetEnhancedPullRequestToolDefinitions()

	allTools := append(issueTools, prTools...)

	foundErrorExample := false
	for _, tool := range allTools {
		for _, example := range tool.UsageExamples {
			if example.Name == "error_case" {
				foundErrorExample = true

				// Validate error example structure
				assert.NotNil(t, example.ExpectedError,
					"Error example in %s should have ExpectedError", tool.Name)

				if example.ExpectedError != nil {
					// Check that error code matches common errors if applicable
					foundMatchingError := false
					for _, commonErr := range tool.CommonErrors {
						if commonErr.Code == example.ExpectedError.Code {
							foundMatchingError = true
							break
						}
					}
					assert.True(t, foundMatchingError,
						"Error code %d in example should match a CommonError",
						example.ExpectedError.Code)
				}
			}
		}
	}

	assert.True(t, foundErrorExample, "Should have at least one error_case example")
}
