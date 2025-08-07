package adapters

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISanitizer_SanitizeJSON(t *testing.T) {
	logger := observability.NewNoopLogger()
	sanitizer := NewOpenAPISanitizer(logger)

	tests := []struct {
		name     string
		input    string
		expected string
		fixes    int
	}{
		{
			name: "Fix numeric example for string type",
			input: `{
				"openapi": "3.0.0",
				"components": {
					"schemas": {
						"Test": {
							"properties": {
								"version": {
									"type": "string",
									"example": 20.04
								}
							}
						}
					}
				}
			}`,
			expected: `"example": "20.04"`,
			fixes:    1,
		},
		{
			name: "Add missing OpenAPI version",
			input: `{
				"info": {
					"title": "Test API",
					"version": "1.0.0"
				}
			}`,
			expected: `"openapi": "3.0.0"`,
			fixes:    1,
		},
		{
			name: "Multiple fixes",
			input: `{
				"info": {
					"title": "Test",
					"version": "1.0"
				},
				"components": {
					"schemas": {
						"Runner": {
							"properties": {
								"os_version": {
									"type": "string",
									"example": 20.04
								},
								"cpu_count": {
									"type": "string", 
									"example": 8
								}
							}
						}
					}
				}
			}`,
			expected: `"openapi": "3.0.0"`,
			fixes:    2, // Missing openapi + quoted examples
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, fixes := sanitizer.SanitizeJSON([]byte(tt.input))
			resultStr := string(result)

			// Check if expected content is present
			assert.Contains(t, resultStr, tt.expected)

			// Check number of fixes
			assert.Len(t, fixes, tt.fixes)
		})
	}
}

func TestOpenAPISanitizer_FixExampleValue(t *testing.T) {
	logger := observability.NewNoopLogger()
	sanitizer := NewOpenAPISanitizer(logger)

	tests := []struct {
		name         string
		example      interface{}
		expectedType string
		expected     interface{}
		shouldFix    bool
	}{
		{
			name:         "Number to string",
			example:      20.04,
			expectedType: "string",
			expected:     "20.04",
			shouldFix:    true,
		},
		{
			name:         "Integer to string",
			example:      42,
			expectedType: "string",
			expected:     "42",
			shouldFix:    true,
		},
		{
			name:         "String to integer",
			example:      "42",
			expectedType: "integer",
			expected:     int64(42),
			shouldFix:    true,
		},
		{
			name:         "String to number",
			example:      "3.14",
			expectedType: "number",
			expected:     3.14,
			shouldFix:    true,
		},
		{
			name:         "String to boolean",
			example:      "true",
			expectedType: "boolean",
			expected:     true,
			shouldFix:    true,
		},
		{
			name:         "Already correct string",
			example:      "test",
			expectedType: "string",
			expected:     "test",
			shouldFix:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, fixed := sanitizer.fixExampleValue(tt.example, tt.expectedType)
			assert.Equal(t, tt.shouldFix, fixed)
			if tt.shouldFix {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestOpenAPISanitizer_ValidateAndSanitize(t *testing.T) {
	logger := observability.NewNoopLogger()
	sanitizer := NewOpenAPISanitizer(logger)
	ctx := context.Background()

	t.Run("Valid spec needs no sanitization", func(t *testing.T) {
		spec := &openapi3.T{
			OpenAPI: "3.0.0",
			Info: &openapi3.Info{
				Title:   "Valid API",
				Version: "1.0.0",
			},
			Paths: openapi3.NewPaths(),
		}

		err := sanitizer.ValidateAndSanitize(ctx, spec)
		assert.NoError(t, err)
	})

	t.Run("Invalid spec gets sanitized", func(t *testing.T) {
		stringType := openapi3.Types{"string"}
		spec := &openapi3.T{
			OpenAPI: "3.0.0",
			Info: &openapi3.Info{
				Title:   "Test API",
				Version: "1.0.0",
			},
			Paths: openapi3.NewPaths(),
			Components: &openapi3.Components{
				Schemas: openapi3.Schemas{
					"TestSchema": &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &stringType,
							Properties: openapi3.Schemas{
								"version": &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type:    &stringType,
										Example: 20.04, // Invalid: number for string type
									},
								},
							},
						},
					},
				},
			},
		}

		err := sanitizer.ValidateAndSanitize(ctx, spec)
		// Should return nil even if validation fails (partial success)
		assert.NoError(t, err)

		// Check if the example was fixed
		versionProp := spec.Components.Schemas["TestSchema"].Value.Properties["version"].Value
		assert.Equal(t, "20.04", versionProp.Example)
	})
}

func TestOpenAPISanitizer_FixArrayExample(t *testing.T) {
	logger := observability.NewNoopLogger()
	sanitizer := NewOpenAPISanitizer(logger)

	t.Run("Nested array with string integers", func(t *testing.T) {
		// Create a schema similar to contributor-activity.weeks
		intType := openapi3.Types{"integer"}
		itemsSchema := &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"w": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &intType}},
				"a": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &intType}},
				"d": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &intType}},
				"c": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &intType}},
			},
		}

		// Example with string values that should be integers
		example := []interface{}{
			map[string]interface{}{
				"w": "1367712000", // String that should be integer
				"a": 6898,         // Already correct
				"d": "77",         // String that should be integer
				"c": 10,           // Already correct
			},
		}

		fixed, wasFixed := sanitizer.fixArrayExample(example, itemsSchema)
		assert.True(t, wasFixed)

		// Check the fixed values
		fixedArray := fixed.([]interface{})
		firstItem := fixedArray[0].(map[string]interface{})

		// All values should be int64 (strings get converted, ints get normalized)
		assert.Equal(t, int64(1367712000), firstItem["w"])
		assert.Equal(t, int64(6898), firstItem["a"]) // Normalized to int64
		assert.Equal(t, int64(77), firstItem["d"])
		assert.Equal(t, int64(10), firstItem["c"]) // Normalized to int64
	})

	t.Run("Full contributor-activity schema", func(t *testing.T) {
		// Test the actual contributor-activity schema structure
		jsonData := `{
			"openapi": "3.0.3",
			"info": {
				"title": "Test API",
				"version": "1.0.0"
			},
			"components": {
				"schemas": {
					"contributor-activity": {
						"type": "object",
						"properties": {
							"weeks": {
								"type": "array",
								"example": [{"w": "1367712000", "a": 6898, "d": 77, "c": 10}],
								"items": {
									"type": "object",
									"properties": {
										"w": {"type": "integer"},
										"a": {"type": "integer"},
										"d": {"type": "integer"},
										"c": {"type": "integer"}
									}
								}
							}
						}
					}
				}
			}
		}`

		// Parse the spec
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData([]byte(jsonData))
		require.NoError(t, err)

		// Apply sanitization
		_, fixes := sanitizer.SanitizeSpec(spec)
		assert.Contains(t, fixes[0], "Fixed array example")

		// Check the fixed example
		weeksExample := spec.Components.Schemas["contributor-activity"].Value.Properties["weeks"].Value.Example
		require.NotNil(t, weeksExample)

		arr := weeksExample.([]interface{})
		firstWeek := arr[0].(map[string]interface{})

		// All values should now be int64 (our sanitizer normalizes all integers)
		assert.Equal(t, int64(1367712000), firstWeek["w"])
		assert.Equal(t, int64(6898), firstWeek["a"])
		assert.Equal(t, int64(77), firstWeek["d"])
		assert.Equal(t, int64(10), firstWeek["c"])
	})
}

func TestOpenAPISanitizer_RealWorldGitHub(t *testing.T) {
	logger := observability.NewNoopLogger()
	sanitizer := NewOpenAPISanitizer(logger)

	t.Run("GitHub-like schema issue", func(t *testing.T) {
		// Simulate the exact GitHub issue
		jsonData := `{
			"openapi": "3.0.3",
			"info": {
				"title": "GitHub API",
				"version": "1.0.0"
			},
			"components": {
				"schemas": {
					"actions-hosted-runner": {
						"properties": {
							"display_name": {
								"description": "Display name for this image.",
								"type": "string",
								"example": 20.04
							}
						}
					}
				}
			}
		}`

		// Pre-process JSON
		sanitized, fixes := sanitizer.SanitizeJSON([]byte(jsonData))
		require.NotEmpty(t, fixes)
		assert.Contains(t, fixes[0], "Quoted numeric examples")

		// Parse sanitized JSON
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(sanitized)
		require.NoError(t, err)

		// Validate should now pass
		ctx := context.Background()
		err = sanitizer.ValidateAndSanitize(ctx, spec)
		assert.NoError(t, err)
	})
}
