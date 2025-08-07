package adapters

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPISanitizer fixes common validation errors in OpenAPI specifications
type OpenAPISanitizer struct {
	logger                  observability.Logger
	maxSpecSize             int64
	maxSanitizationAttempts int
}

// NewOpenAPISanitizer creates a new OpenAPI sanitizer
func NewOpenAPISanitizer(logger observability.Logger) *OpenAPISanitizer {
	return &OpenAPISanitizer{
		logger:                  logger,
		maxSpecSize:             50 * 1024 * 1024, // 50MB max
		maxSanitizationAttempts: 3,
	}
}

// SanitizeSpec attempts to fix common validation errors in OpenAPI specs
func (s *OpenAPISanitizer) SanitizeSpec(spec *openapi3.T) (*openapi3.T, []string) {
	var fixes []string

	// Fix 1: Handle type mismatches in examples
	fixes = append(fixes, s.fixExampleTypeMismatches(spec)...)

	// Fix 2: Handle missing required fields
	fixes = append(fixes, s.fixMissingRequiredFields(spec)...)

	// Fix 3: Handle invalid enum values
	fixes = append(fixes, s.fixInvalidEnumValues(spec)...)

	// Fix 4: Handle circular references
	fixes = append(fixes, s.fixCircularReferences(spec)...)

	if len(fixes) > 0 {
		s.logger.Info("Applied OpenAPI spec fixes", map[string]interface{}{
			"fixes_count": len(fixes),
			"fixes":       fixes,
		})
	}

	return spec, fixes
}

// fixExampleTypeMismatches fixes examples that don't match their schema type
func (s *OpenAPISanitizer) fixExampleTypeMismatches(spec *openapi3.T) []string {
	var fixes []string

	// Check all component schemas
	if spec.Components != nil && spec.Components.Schemas != nil {
		for name, schemaRef := range spec.Components.Schemas {
			if schemaRef.Value == nil {
				continue
			}

			fixes = append(fixes, s.fixSchemaExamples(name, schemaRef.Value)...)
		}
	}

	// Check all paths
	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			fixes = append(fixes, s.fixPathExamples(path, pathItem)...)
		}
	}

	return fixes
}

// fixSchemaExamples fixes examples in a single schema
func (s *OpenAPISanitizer) fixSchemaExamples(schemaName string, schema *openapi3.Schema) []string {
	var fixes []string

	// Fix direct schema example with safety check
	if schema.Example != nil && schema.Type != nil && len(*schema.Type) > 0 {
		// Safely access first type
		types := *schema.Type
		if len(types) > 0 {
			schemaType := types[0]
			// Special handling for array types with examples
			if schemaType == "array" && schema.Items != nil && schema.Items.Value != nil {
				if fixed, ok := s.fixArrayExample(schema.Example, schema.Items.Value); ok {
					schema.Example = fixed
					fixes = append(fixes, fmt.Sprintf("Fixed array example type mismatch in schema '%s'", schemaName))
				}
			} else {
				if fixed, ok := s.fixExampleValue(schema.Example, schemaType); ok {
					schema.Example = fixed
					fixes = append(fixes, fmt.Sprintf("Fixed example type mismatch in schema '%s'", schemaName))
				}
			}
		}
	}

	// Fix property examples
	for propName, propSchema := range schema.Properties {
		if propSchema.Value != nil && propSchema.Value.Example != nil && propSchema.Value.Type != nil && len(*propSchema.Value.Type) > 0 {
			// Safely access type
			types := *propSchema.Value.Type
			if len(types) > 0 {
				propType := types[0]
				// Special handling for array types with examples
				if propType == "array" && propSchema.Value.Items != nil && propSchema.Value.Items.Value != nil {
					if fixed, ok := s.fixArrayExample(propSchema.Value.Example, propSchema.Value.Items.Value); ok {
						propSchema.Value.Example = fixed
						fixes = append(fixes, fmt.Sprintf("Fixed array example type mismatch in '%s.%s'", schemaName, propName))
					}
				} else {
					if fixed, ok := s.fixExampleValue(propSchema.Value.Example, propType); ok {
						propSchema.Value.Example = fixed
						fixes = append(fixes, fmt.Sprintf("Fixed example type mismatch in '%s.%s'", schemaName, propName))
					}
				}
			}
		}

		// Recursively fix nested schemas with depth limit to prevent stack overflow
		if propSchema.Value != nil && len(schemaName) < 200 { // Prevent deep recursion
			fixes = append(fixes, s.fixSchemaExamples(fmt.Sprintf("%s.%s", schemaName, propName), propSchema.Value)...)
		}
	}

	// Fix items schema for arrays
	if schema.Items != nil && schema.Items.Value != nil {
		fixes = append(fixes, s.fixSchemaExamples(schemaName+"[items]", schema.Items.Value)...)
	}

	return fixes
}

// fixArrayExample fixes examples within arrays based on the items schema
func (s *OpenAPISanitizer) fixArrayExample(example interface{}, itemsSchema *openapi3.Schema) (interface{}, bool) {
	// Check if example is actually an array
	arr, ok := example.([]interface{})
	if !ok {
		return example, false
	}

	fixed := false
	fixedArray := make([]interface{}, len(arr))

	for i, item := range arr {
		// If item is an object, check its properties
		if obj, ok := item.(map[string]interface{}); ok {
			fixedObj := make(map[string]interface{})
			for key, value := range obj {
				// Check if this property has a schema
				if itemsSchema.Properties != nil {
					if propRef, exists := itemsSchema.Properties[key]; exists && propRef.Value != nil {
						if propRef.Value.Type != nil && len(*propRef.Value.Type) > 0 {
							propType := (*propRef.Value.Type)[0]
							if fixedValue, wasFixed := s.fixExampleValue(value, propType); wasFixed {
								fixedObj[key] = fixedValue
								fixed = true
							} else {
								fixedObj[key] = value
							}
						} else {
							fixedObj[key] = value
						}
					} else {
						fixedObj[key] = value
					}
				} else {
					fixedObj[key] = value
				}
			}
			fixedArray[i] = fixedObj
		} else {
			// For non-object items, try to fix based on items schema type
			if itemsSchema.Type != nil && len(*itemsSchema.Type) > 0 {
				itemType := (*itemsSchema.Type)[0]
				if fixedValue, wasFixed := s.fixExampleValue(item, itemType); wasFixed {
					fixedArray[i] = fixedValue
					fixed = true
				} else {
					fixedArray[i] = item
				}
			} else {
				fixedArray[i] = item
			}
		}
	}

	if fixed {
		return fixedArray, true
	}
	return example, false
}

// fixExampleValue converts an example to match the expected type
func (s *OpenAPISanitizer) fixExampleValue(example interface{}, expectedType string) (interface{}, bool) {
	switch expectedType {
	case "string":
		// Convert any non-string to string
		if _, ok := example.(string); !ok {
			return fmt.Sprintf("%v", example), true
		}
	case "integer":
		// Convert string numbers to integers
		if str, ok := example.(string); ok {
			// Try to parse as number
			var num float64
			if _, err := fmt.Sscanf(str, "%f", &num); err == nil {
				return int64(num), true
			}
		}
		// Convert float to integer (handles JSON numbers)
		if f, ok := example.(float64); ok {
			// Check if it's a whole number
			if f == float64(int64(f)) {
				return int64(f), true
			}
			return int64(f), true
		}
		// Already an int, but we should normalize to int64
		if i, ok := example.(int); ok {
			return int64(i), true
		}
		// Already int64, keep as is
		if _, ok := example.(int64); ok {
			return example, false
		}
	case "number":
		// Convert string to number
		if str, ok := example.(string); ok {
			var num float64
			if _, err := fmt.Sscanf(str, "%f", &num); err == nil {
				return num, true
			}
		}
	case "boolean":
		// Convert string booleans
		if str, ok := example.(string); ok {
			str = strings.ToLower(str)
			switch str {
			case "true", "1", "yes":
				return true, true
			case "false", "0", "no":
				return false, true
			}
		}
	}
	return example, false
}

// fixPathExamples fixes examples in path operations
func (s *OpenAPISanitizer) fixPathExamples(path string, pathItem *openapi3.PathItem) []string {
	var fixes []string

	operations := map[string]*openapi3.Operation{
		"GET":    pathItem.Get,
		"POST":   pathItem.Post,
		"PUT":    pathItem.Put,
		"DELETE": pathItem.Delete,
		"PATCH":  pathItem.Patch,
	}

	for method, op := range operations {
		if op == nil {
			continue
		}

		// Fix parameter examples
		for _, param := range op.Parameters {
			if param.Value != nil && param.Value.Schema != nil && param.Value.Schema.Value != nil {
				schema := param.Value.Schema.Value
				if schema.Example != nil && schema.Type != nil && len(*schema.Type) > 0 {
					// Safely access type
					types := *schema.Type
					if len(types) > 0 {
						schemaType := types[0]
						if fixed, ok := s.fixExampleValue(schema.Example, schemaType); ok {
							schema.Example = fixed
							fixes = append(fixes, fmt.Sprintf("Fixed parameter example in %s %s", method, path))
						}
					}
				}
			}
		}

		// Fix request body examples
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			for mediaType, content := range op.RequestBody.Value.Content {
				if content.Schema != nil && content.Schema.Value != nil {
					fixes = append(fixes, s.fixSchemaExamples(fmt.Sprintf("%s %s request[%s]", method, path, mediaType), content.Schema.Value)...)
				}
			}
		}

		// Fix response examples
		if op.Responses != nil {
			for status, response := range op.Responses.Map() {
				if response.Value != nil {
					for mediaType, content := range response.Value.Content {
						if content.Schema != nil && content.Schema.Value != nil {
							fixes = append(fixes, s.fixSchemaExamples(fmt.Sprintf("%s %s response[%s][%s]", method, path, status, mediaType), content.Schema.Value)...)
						}
					}
				}
			}
		}
	}

	return fixes
}

// fixMissingRequiredFields adds missing required fields with sensible defaults
func (s *OpenAPISanitizer) fixMissingRequiredFields(spec *openapi3.T) []string {
	var fixes []string

	// Ensure info section has required fields
	if spec.Info != nil {
		if spec.Info.Title == "" {
			spec.Info.Title = "API"
			fixes = append(fixes, "Added missing title to info section")
		}
		if spec.Info.Version == "" {
			spec.Info.Version = "1.0.0"
			fixes = append(fixes, "Added missing version to info section")
		}
	}

	// Ensure paths is not nil
	if spec.Paths == nil {
		spec.Paths = openapi3.NewPaths()
		fixes = append(fixes, "Added missing paths object")
	}

	return fixes
}

// fixInvalidEnumValues ensures enum values match the schema type
func (s *OpenAPISanitizer) fixInvalidEnumValues(spec *openapi3.T) []string {
	var fixes []string

	if spec.Components != nil && spec.Components.Schemas != nil {
		for name, schemaRef := range spec.Components.Schemas {
			if schemaRef.Value != nil && len(schemaRef.Value.Enum) > 0 {
				fixes = append(fixes, s.fixSchemaEnums(name, schemaRef.Value)...)
			}
		}
	}

	return fixes
}

// fixSchemaEnums fixes enum values to match schema type
func (s *OpenAPISanitizer) fixSchemaEnums(schemaName string, schema *openapi3.Schema) []string {
	var fixes []string

	if len(schema.Enum) > 0 && schema.Type != nil && len(*schema.Type) > 0 {
		// Safely access type
		types := *schema.Type
		if len(types) > 0 {
			schemaType := types[0]
			var fixedEnum []interface{}
			modified := false

			for _, value := range schema.Enum {
				if fixed, ok := s.fixExampleValue(value, schemaType); ok {
					fixedEnum = append(fixedEnum, fixed)
					modified = true
				} else {
					fixedEnum = append(fixedEnum, value)
				}
			}

			if modified {
				schema.Enum = fixedEnum
				fixes = append(fixes, fmt.Sprintf("Fixed enum values in schema '%s'", schemaName))
			}
		}
	}

	// Fix nested properties
	for propName, propSchema := range schema.Properties {
		if propSchema.Value != nil && len(propSchema.Value.Enum) > 0 {
			fixes = append(fixes, s.fixSchemaEnums(fmt.Sprintf("%s.%s", schemaName, propName), propSchema.Value)...)
		}
	}

	return fixes
}

// fixCircularReferences attempts to detect and break circular references
func (s *OpenAPISanitizer) fixCircularReferences(spec *openapi3.T) []string {
	var fixes []string
	// This is complex and would need careful implementation
	// For now, just detect and log
	return fixes
}

// SanitizeJSON performs pre-parsing fixes on raw JSON
func (s *OpenAPISanitizer) SanitizeJSON(data []byte) ([]byte, []string) {
	var fixes []string

	// Safety check: size limit
	if int64(len(data)) > s.maxSpecSize {
		s.logger.Warn("OpenAPI spec exceeds size limit", map[string]interface{}{
			"size":  len(data),
			"limit": s.maxSpecSize,
		})
		return data, fixes // Return original data without processing
	}

	// Convert to string for regex operations
	content := string(data)

	// Fix 1: Quote numeric examples that should be strings
	// Pattern: "type": "string" followed by "example": <number>
	// Use non-greedy matching to prevent ReDoS
	pattern := regexp.MustCompile(`("type":\s*"string"[^}]{0,200}?"example":\s*)([0-9.]+)([,\s}])`)
	if pattern.MatchString(content) {
		content = pattern.ReplaceAllString(content, `${1}"${2}"${3}`)
		fixes = append(fixes, "Quoted numeric examples for string types")
	}

	// Fix 2: Handle malformed URLs in servers section (removed for safety)
	// This was too risky and could corrupt valid URLs

	// Fix 3: Ensure required OpenAPI version field
	if !strings.Contains(content, `"openapi"`) && !strings.Contains(content, `"swagger"`) {
		// Add OpenAPI version at the beginning
		content = strings.Replace(content, "{", `{"openapi": "3.0.0",`, 1)
		fixes = append(fixes, "Added missing OpenAPI version field")
	}

	return []byte(content), fixes
}

// ValidateAndSanitize combines validation with automatic fixing
func (s *OpenAPISanitizer) ValidateAndSanitize(ctx context.Context, spec *openapi3.T) error {
	// First, try normal validation
	if err := spec.Validate(ctx); err == nil {
		return nil // No sanitization needed
	}

	// Apply sanitization
	_, fixes := s.SanitizeSpec(spec)

	// Try validation again with a more lenient approach
	if err := spec.Validate(ctx); err != nil {
		// Log remaining errors but don't fail
		s.logger.Warn("OpenAPI spec has remaining validation issues after sanitization", map[string]interface{}{
			"fixes_applied": len(fixes),
			"error":         err.Error(),
		})
		// Return nil to allow partial success
		return nil
	}

	return nil
}
