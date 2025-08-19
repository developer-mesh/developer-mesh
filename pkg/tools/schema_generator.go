package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaGenerator generates MCP-compatible tool schemas from OpenAPI specs
// Enhanced with 2025 AI agent best practices for better tool discovery
type SchemaGenerator struct {
	// Configuration for schema generation
	MaxOperationsPerTool int
	GroupByTag           bool
	IncludeDeprecated    bool
	EnhanceForAI         bool // Enable AI-friendly enhancements

	// Operation grouper for multi-tool generation
	grouper *OperationGrouper
	logger  observability.Logger
}

// NewSchemaGenerator creates a new schema generator with default settings
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		MaxOperationsPerTool: 50,   // Limit operations per tool to avoid overwhelming agents
		GroupByTag:           true, // Group operations by tag for better organization
		IncludeDeprecated:    false,
		EnhanceForAI:         true, // Enable AI enhancements by default
		grouper:              NewOperationGrouper(),
		logger:               observability.NewStandardLogger("schema-generator"),
	}
}

// NewSchemaGeneratorWithLogger creates a schema generator with a custom logger
func NewSchemaGeneratorWithLogger(logger observability.Logger) *SchemaGenerator {
	gen := NewSchemaGenerator()
	gen.logger = logger
	return gen
}

// GenerateMCPSchema generates an MCP-compatible schema from an OpenAPI spec
// This returns a single unified schema that describes all available operations
func (g *SchemaGenerator) GenerateMCPSchema(spec *openapi3.T) (map[string]interface{}, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	// Build the MCP tool schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "The API operation to perform",
				"enum":        g.extractOperationIDs(spec),
			},
			"parameters": map[string]interface{}{
				"type":        "object",
				"description": "Parameters for the selected operation",
				"properties":  map[string]interface{}{},
			},
		},
		"required":             []string{"operation"},
		"additionalProperties": false,
	}

	// Add operation-specific parameter schemas
	operationSchemas := g.extractOperationSchemas(spec)
	if len(operationSchemas) > 0 {
		schema["allOf"] = []interface{}{
			map[string]interface{}{
				"if": map[string]interface{}{
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"const": "dynamic",
						},
					},
				},
				"then": map[string]interface{}{
					"properties": map[string]interface{}{
						"parameters": operationSchemas,
					},
				},
			},
		}
	}

	// Add metadata about available operations
	schema["x-operations"] = g.extractOperationMetadata(spec)

	return schema, nil
}

// GenerateOperationSchemas generates individual schemas for each operation
// This is useful when you want to expose each operation as a separate tool
func (g *SchemaGenerator) GenerateOperationSchemas(spec *openapi3.T) (map[string]interface{}, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	schemas := make(map[string]interface{})

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			// Generate operation ID if not present
			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			// Generate schema for this operation
			opSchema := g.generateOperationSchema(operation, method, path)
			schemas[operationID] = opSchema
		}
	}

	return schemas, nil
}

// extractOperationIDs extracts all operation IDs from the spec
func (g *SchemaGenerator) extractOperationIDs(spec *openapi3.T) []string {
	var operationIDs []string
	seen := make(map[string]bool)

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			if !seen[operationID] {
				operationIDs = append(operationIDs, operationID)
				seen[operationID] = true
			}

			// Stop if we've reached the max
			if len(operationIDs) >= g.MaxOperationsPerTool {
				break
			}
		}

		if len(operationIDs) >= g.MaxOperationsPerTool {
			break
		}
	}

	return operationIDs
}

// extractOperationMetadata extracts metadata about each operation
func (g *SchemaGenerator) extractOperationMetadata(spec *openapi3.T) map[string]interface{} {
	metadata := make(map[string]interface{})

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			operationID := operation.OperationID
			if operationID == "" {
				operationID = g.generateOperationID(method, path)
			}

			metadata[operationID] = map[string]interface{}{
				"method":      method,
				"path":        path,
				"summary":     operation.Summary,
				"description": operation.Description,
				"tags":        operation.Tags,
				"deprecated":  operation.Deprecated,
			}
		}
	}

	return metadata
}

// extractOperationSchemas creates a combined schema for all operations
func (g *SchemaGenerator) extractOperationSchemas(spec *openapi3.T) map[string]interface{} {
	properties := make(map[string]interface{})

	for _, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for _, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Skip deprecated operations if configured
			if !g.IncludeDeprecated && operation.Deprecated {
				continue
			}

			// Extract parameters for this operation
			params := g.extractOperationParameters(operation, pathItem.Parameters)
			if len(params) > 0 {
				for name, schema := range params {
					// Merge parameters from different operations
					if existing, ok := properties[name]; ok {
						// If parameter exists, try to merge schemas intelligently
						properties[name] = g.mergeSchemas(existing, schema)
					} else {
						properties[name] = schema
					}
				}
			}
		}
	}

	return properties
}

// generateOperationSchema generates a schema for a single operation
func (g *SchemaGenerator) generateOperationSchema(operation *openapi3.Operation, method, path string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":        "object",
		"description": g.getOperationDescription(operation),
		"properties":  make(map[string]interface{}),
		"required":    []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	required := []string{}

	// Add path parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.In == "path" {
			paramSchema := g.parameterToSchema(param.Value)
			properties[param.Value.Name] = paramSchema
			if param.Value.Required {
				required = append(required, param.Value.Name)
			}
		}
	}

	// Add query parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.In == "query" {
			paramSchema := g.parameterToSchema(param.Value)
			properties[param.Value.Name] = paramSchema
			if param.Value.Required {
				required = append(required, param.Value.Name)
			}
		}
	}

	// Add request body
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if jsonContent, ok := operation.RequestBody.Value.Content["application/json"]; ok {
			if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
				bodySchema := g.schemaToMCPSchema(jsonContent.Schema.Value)
				properties["body"] = bodySchema
				if operation.RequestBody.Value.Required {
					required = append(required, "body")
				}
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// extractOperationParameters extracts parameters from an operation
func (g *SchemaGenerator) extractOperationParameters(operation *openapi3.Operation, globalParams openapi3.Parameters) map[string]interface{} {
	params := make(map[string]interface{})

	// Process global parameters first
	for _, param := range globalParams {
		if param.Value != nil {
			params[param.Value.Name] = g.parameterToSchema(param.Value)
		}
	}

	// Process operation-specific parameters (override globals)
	for _, param := range operation.Parameters {
		if param.Value != nil {
			params[param.Value.Name] = g.parameterToSchema(param.Value)
		}
	}

	// Process request body
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if jsonContent, ok := operation.RequestBody.Value.Content["application/json"]; ok {
			if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
				// For request body, we prefix parameters with "body_" to avoid conflicts
				if jsonContent.Schema.Value.Properties != nil {
					for name, prop := range jsonContent.Schema.Value.Properties {
						if prop.Value != nil {
							params["body_"+name] = g.schemaToMCPSchema(prop.Value)
						}
					}
				}
			}
		}
	}

	return params
}

// parameterToSchema converts an OpenAPI parameter to MCP schema
// Enhanced with AI-friendly descriptions and examples
func (g *SchemaGenerator) parameterToSchema(param *openapi3.Parameter) map[string]interface{} {
	schema := map[string]interface{}{
		"description": g.enhanceParameterDescription(param),
	}

	if param.Schema != nil && param.Schema.Value != nil {
		// Copy type information
		schemaValue := param.Schema.Value
		if schemaValue.Type != nil {
			schema["type"] = g.getSchemaType(schemaValue)
		}

		// Add AI enhancements
		if g.EnhanceForAI {
			// Add example if available
			if param.Example != nil {
				schema["example"] = param.Example
			} else if param.Examples != nil && len(param.Examples) > 0 {
				// Extract first example
				for _, ex := range param.Examples {
					if ex.Value != nil && ex.Value.Value != nil {
						schema["example"] = ex.Value.Value
						break
					}
				}
			}

			// Add parameter location hint
			if param.In != "" {
				schema["x-parameter-location"] = param.In
			}
		}
		if len(schemaValue.Enum) > 0 {
			schema["enum"] = schemaValue.Enum
		}
		if schemaValue.Default != nil {
			schema["default"] = schemaValue.Default
		}
		if schemaValue.Pattern != "" {
			schema["pattern"] = schemaValue.Pattern
		}
		// MinLength and MaxLength are uint64 in openapi3, not pointers
		if schemaValue.MinLength > 0 {
			schema["minLength"] = schemaValue.MinLength
		}
		if schemaValue.MaxLength != nil && *schemaValue.MaxLength > 0 {
			schema["maxLength"] = *schemaValue.MaxLength
		}
	}

	return schema
}

// enhanceParameterDescription creates an AI-friendly parameter description
func (g *SchemaGenerator) enhanceParameterDescription(param *openapi3.Parameter) string {
	var desc strings.Builder

	// Base description
	if param.Description != "" {
		desc.WriteString(param.Description)
	} else {
		desc.WriteString(fmt.Sprintf("%s parameter", strings.Title(param.Name)))
	}

	if !g.EnhanceForAI {
		return desc.String()
	}

	// Add location context for AI
	switch param.In {
	case "path":
		desc.WriteString(" (URL path parameter)")
	case "query":
		desc.WriteString(" (query string parameter)")
	case "header":
		desc.WriteString(" (HTTP header)")
	case "cookie":
		desc.WriteString(" (cookie value)")
	}

	// Add requirement status
	if param.Required {
		desc.WriteString(" [REQUIRED]")
	} else {
		desc.WriteString(" [OPTIONAL]")
	}

	// Add schema constraints if available
	if param.Schema != nil && param.Schema.Value != nil {
		schema := param.Schema.Value

		// Add enum values if present
		if len(schema.Enum) > 0 && len(schema.Enum) <= 10 {
			desc.WriteString(". Allowed values: ")
			enumStrs := make([]string, len(schema.Enum))
			for i, v := range schema.Enum {
				enumStrs[i] = fmt.Sprintf("%v", v)
			}
			desc.WriteString(strings.Join(enumStrs, ", "))
		}

		// Add format hint
		if schema.Format != "" {
			desc.WriteString(fmt.Sprintf(". Format: %s", schema.Format))
		}

		// Add pattern if present
		if schema.Pattern != "" && len(schema.Pattern) < 50 {
			desc.WriteString(fmt.Sprintf(". Pattern: %s", schema.Pattern))
		}
	}

	return desc.String()
}

// schemaToMCPSchema converts an OpenAPI schema to MCP schema
func (g *SchemaGenerator) schemaToMCPSchema(schema *openapi3.Schema) map[string]interface{} {
	// Handle composition schemas (oneOf, allOf, anyOf) by simplifying them
	// Claude's API doesn't support these at the top level
	if len(schema.OneOf) > 0 {
		// For oneOf, use the first schema as a fallback
		if schema.OneOf[0].Value != nil {
			return g.schemaToMCPSchema(schema.OneOf[0].Value)
		}
	}
	if len(schema.AllOf) > 0 {
		// For allOf, merge all schemas
		merged := map[string]interface{}{
			"type":        "object",
			"description": schema.Description,
			"properties":  make(map[string]interface{}),
		}
		for _, subSchema := range schema.AllOf {
			if subSchema.Value != nil {
				subMCP := g.schemaToMCPSchema(subSchema.Value)
				if props, ok := subMCP["properties"].(map[string]interface{}); ok {
					mergedProps := merged["properties"].(map[string]interface{})
					for k, v := range props {
						mergedProps[k] = v
					}
				}
			}
		}
		return merged
	}
	if len(schema.AnyOf) > 0 {
		// For anyOf, use the first schema as a fallback
		if schema.AnyOf[0].Value != nil {
			return g.schemaToMCPSchema(schema.AnyOf[0].Value)
		}
	}

	mcpSchema := map[string]interface{}{
		"type":        g.getSchemaType(schema),
		"description": schema.Description,
	}

	// Handle arrays
	if g.getSchemaType(schema) == "array" && schema.Items != nil && schema.Items.Value != nil {
		mcpSchema["items"] = g.schemaToMCPSchema(schema.Items.Value)
	}

	// Handle objects
	if g.getSchemaType(schema) == "object" && schema.Properties != nil {
		properties := make(map[string]interface{})
		for name, prop := range schema.Properties {
			if prop.Value != nil {
				properties[name] = g.schemaToMCPSchema(prop.Value)
			}
		}
		mcpSchema["properties"] = properties

		if len(schema.Required) > 0 {
			mcpSchema["required"] = schema.Required
		}
	}

	// Add constraints
	if len(schema.Enum) > 0 {
		mcpSchema["enum"] = schema.Enum
	}
	if schema.Default != nil {
		mcpSchema["default"] = schema.Default
	}
	if schema.Pattern != "" {
		mcpSchema["pattern"] = schema.Pattern
	}

	return mcpSchema
}

// getSchemaType returns the type of an OpenAPI schema as a string
func (g *SchemaGenerator) getSchemaType(schema *openapi3.Schema) string {
	if schema.Type == nil {
		return "string" // default
	}

	// Type is a *openapi3.Types in OpenAPI 3.1
	if schema.Type.Is("string") {
		return "string"
	} else if schema.Type.Is("number") {
		return "number"
	} else if schema.Type.Is("integer") {
		return "integer"
	} else if schema.Type.Is("boolean") {
		return "boolean"
	} else if schema.Type.Is("array") {
		return "array"
	} else if schema.Type.Is("object") {
		return "object"
	}

	return "string" // default
}

// generateOperationID generates an operation ID from method and path
// Enhanced to follow 2025 AI agent naming conventions (snake_case, max 60 chars)
func (g *SchemaGenerator) generateOperationID(method, path string) string {
	if !g.EnhanceForAI {
		// Original behavior
		parts := strings.Split(strings.Trim(path, "/"), "/")
		cleanParts := []string{strings.ToLower(method)}
		for _, part := range parts {
			if strings.HasPrefix(part, "{") || part == "v1" || part == "v2" || part == "api" {
				continue
			}
			cleanParts = append(cleanParts, part)
		}
		return strings.Join(cleanParts, "_")
	}

	// Enhanced AI-friendly operation ID generation
	// Following Google ADK and OpenAI standards
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var cleanParts []string

	// Use semantic verb mapping for common REST operations
	verb := g.mapMethodToSemanticVerb(method, path)
	cleanParts = append(cleanParts, verb)

	// Extract resource names from path
	for i, part := range parts {
		// Skip version indicators and generic terms
		if part == "v1" || part == "v2" || part == "api" || part == "rest" {
			continue
		}

		// Handle path parameters intelligently
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// Include context about what's being identified
			if i > 0 && i == len(parts)-1 {
				// Last segment parameter, likely an ID
				cleanParts = append(cleanParts, "by_id")
			}
			continue
		}

		// Convert to snake_case
		cleanParts = append(cleanParts, toSnakeCase(part))
	}

	// Generate ID and enforce length limit
	id := strings.Join(cleanParts, "_")
	if len(id) > 60 {
		id = id[:60]
	}

	return id
}

// mapMethodToSemanticVerb maps HTTP methods to semantic verbs for AI understanding
func (g *SchemaGenerator) mapMethodToSemanticVerb(method, path string) string {
	methodLower := strings.ToLower(method)
	pathLower := strings.ToLower(path)

	switch methodLower {
	case "get":
		if strings.Contains(pathLower, "search") || strings.Contains(pathLower, "query") {
			return "search"
		}
		if strings.Contains(pathLower, "list") || !strings.Contains(path, "{") {
			return "list"
		}
		return "get"
	case "post":
		if strings.Contains(pathLower, "search") {
			return "search"
		}
		if strings.Contains(pathLower, "execute") || strings.Contains(pathLower, "run") {
			return "execute"
		}
		return "create"
	case "put":
		return "update"
	case "patch":
		return "patch"
	case "delete":
		return "delete"
	default:
		return methodLower
	}
}

// toSnakeCase converts a string to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// getOperationDescription gets a description for an operation
// Enhanced with AI-friendly descriptions following 2025 best practices
func (g *SchemaGenerator) getOperationDescription(operation *openapi3.Operation) string {
	if !g.EnhanceForAI {
		// Original behavior
		if operation.Summary != "" {
			return operation.Summary
		}
		if operation.Description != "" {
			// Truncate long descriptions
			if len(operation.Description) > 200 {
				return operation.Description[:197] + "..."
			}
			return operation.Description
		}
		return "API operation"
	}

	// Enhanced AI-friendly description
	var desc strings.Builder

	// Primary description from summary or description
	if operation.Summary != "" {
		desc.WriteString(operation.Summary)
	} else if operation.Description != "" {
		lines := strings.Split(operation.Description, "\n")
		if len(lines) > 0 {
			desc.WriteString(lines[0])
		}
	} else {
		desc.WriteString("Perform API operation")
	}

	// Add parameter context for AI understanding
	if len(operation.Parameters) > 0 {
		requiredParams := []string{}
		optionalParams := []string{}
		for _, param := range operation.Parameters {
			if param.Value != nil {
				if param.Value.Required {
					requiredParams = append(requiredParams, param.Value.Name)
				} else {
					optionalParams = append(optionalParams, param.Value.Name)
				}
			}
		}
		if len(requiredParams) > 0 {
			desc.WriteString(". Required: ")
			desc.WriteString(strings.Join(requiredParams, ", "))
		}
		if len(optionalParams) > 0 {
			desc.WriteString(". Optional: ")
			desc.WriteString(strings.Join(optionalParams, ", "))
		}
	}

	// Add response context
	if operation.Responses != nil && operation.Responses.Status(200) != nil {
		resp200 := operation.Responses.Status(200)
		if resp200.Value != nil && resp200.Value.Description != nil && *resp200.Value.Description != "" {
			desc.WriteString(". Returns: ")
			desc.WriteString(*resp200.Value.Description)
		}
	}

	return desc.String()
}

// mergeSchemas attempts to merge two schemas intelligently
func (g *SchemaGenerator) mergeSchemas(existing, new interface{}) interface{} {
	// For now, just return the existing schema
	// In a more sophisticated implementation, we could merge enums, types, etc.
	return existing
}

// GenerateGroupedSchemas generates schemas for operation groups
// This is the main method for creating multiple tools from an OpenAPI spec
func (g *SchemaGenerator) GenerateGroupedSchemas(spec *openapi3.T) (map[string]GroupedToolSchema, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	// Group operations using the grouper
	groups, err := g.grouper.GroupOperations(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to group operations: %w", err)
	}

	// Generate schemas for each group
	schemas := make(map[string]GroupedToolSchema)

	for groupName, group := range groups {
		schema := g.generateGroupSchema(group)
		schemas[groupName] = GroupedToolSchema{
			Name:        groupName,
			DisplayName: group.DisplayName,
			Description: group.Description,
			Schema:      schema,
			Operations:  g.extractGroupOperationInfo(group),
			Priority:    group.Priority,
		}
	}

	return schemas, nil
}

// GroupedToolSchema represents a schema for a grouped tool
type GroupedToolSchema struct {
	Name        string                 // Tool name (e.g., "github_repos")
	DisplayName string                 // Human-friendly name
	Description string                 // Tool description
	Schema      map[string]interface{} // MCP-compatible schema
	Operations  []OperationInfo        // Information about operations
	Priority    int                    // Priority for ordering
}

// OperationInfo contains metadata about an operation
type OperationInfo struct {
	ID          string
	Method      string
	Path        string
	Summary     string
	Description string
}

// generateGroupSchema generates a schema for an operation group
func (g *SchemaGenerator) generateGroupSchema(group *OperationGroup) map[string]interface{} {
	// Collect all unique parameters from all operations in the group
	allParameters := make(map[string]interface{})
	operationParams := make(map[string][]string) // Track which params belong to which operation

	// Extract parameters from each operation
	for opID, op := range group.Operations {
		opSchema := g.generateOperationSchema(op.Operation, op.Method, op.Path)
		if props, ok := opSchema["properties"].(map[string]interface{}); ok {
			operationParams[opID] = make([]string, 0)
			for paramName, paramSchema := range props {
				// Add operation info to parameter description
				if paramDesc, ok := paramSchema.(map[string]interface{}); ok {
					if desc, hasDesc := paramDesc["description"].(string); hasDesc {
						paramDesc["description"] = fmt.Sprintf("[%s] %s", opID, desc)
					} else {
						paramDesc["description"] = fmt.Sprintf("Parameter for %s operation", opID)
					}
				}
				allParameters[paramName] = paramSchema
				operationParams[opID] = append(operationParams[opID], paramName)
			}
		}
	}

	// Build the MCP tool schema for this group
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": fmt.Sprintf("The %s operation to perform", group.DisplayName),
				"enum":        g.extractGroupOperationIDs(group),
			},
			"parameters": map[string]interface{}{
				"type":                 "object",
				"description":          "Parameters for the selected operation",
				"properties":           allParameters,
				"additionalProperties": false,
			},
		},
		"required":             []string{"operation"},
		"additionalProperties": false,
	}

	// Add metadata about operations and their parameters
	schema["x-operations"] = g.extractGroupOperationMetadata(group)
	schema["x-operation-params"] = operationParams

	return schema
}

// extractGroupOperationIDs extracts operation IDs from a group
func (g *SchemaGenerator) extractGroupOperationIDs(group *OperationGroup) []string {
	ids := make([]string, 0, len(group.Operations))
	for id := range group.Operations {
		ids = append(ids, id)
	}
	// Sort for consistency
	sort.Strings(ids)
	return ids
}

// extractGroupOperationMetadata extracts metadata for operations in a group
func (g *SchemaGenerator) extractGroupOperationMetadata(group *OperationGroup) map[string]interface{} {
	metadata := make(map[string]interface{})

	for opID, op := range group.Operations {
		metadata[opID] = map[string]interface{}{
			"method":      op.Method,
			"path":        op.Path,
			"summary":     op.Operation.Summary,
			"description": op.Operation.Description,
			"tags":        op.Operation.Tags,
			"deprecated":  op.Operation.Deprecated,
		}
	}

	return metadata
}

// extractGroupOperationInfo extracts operation information for documentation
func (g *SchemaGenerator) extractGroupOperationInfo(group *OperationGroup) []OperationInfo {
	info := make([]OperationInfo, 0, len(group.Operations))

	for opID, op := range group.Operations {
		info = append(info, OperationInfo{
			ID:          opID,
			Method:      op.Method,
			Path:        op.Path,
			Summary:     op.Operation.Summary,
			Description: op.Operation.Description,
		})
	}

	// Sort by operation ID for consistency
	sort.Slice(info, func(i, j int) bool {
		return info[i].ID < info[j].ID
	})

	return info
}

// ConfigureGrouping configures the operation grouping strategy
func (g *SchemaGenerator) ConfigureGrouping(strategy GroupingStrategy, maxPerGroup int) {
	if g.grouper != nil {
		g.grouper.GroupingStrategy = strategy
		g.grouper.MaxOperationsPerGroup = maxPerGroup
	}
}

// GenerateAIEnhancedSchema generates a schema with AI agent enhancements
// This method provides the best possible schema for AI agents to understand tools
func (g *SchemaGenerator) GenerateAIEnhancedSchema(spec *openapi3.T, toolName string) (map[string]interface{}, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI spec is nil")
	}

	// Ensure AI enhancements are enabled
	originalEnhance := g.EnhanceForAI
	g.EnhanceForAI = true
	defer func() { g.EnhanceForAI = originalEnhance }()

	// Extract tool metadata
	toolInfo := g.extractToolMetadata(spec, toolName)

	// Generate operation schemas
	operations, err := g.GenerateOperationSchemas(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to generate operation schemas: %w", err)
	}

	// Build enhanced schema following MCP standards
	schema := map[string]interface{}{
		"name":        toolInfo["name"],
		"description": toolInfo["description"],
		"version":     toolInfo["version"],
		"operations":  operations,
	}

	// Add semantic categories for better AI understanding
	if categories := g.extractSemanticCategories(spec); len(categories) > 0 {
		schema["categories"] = categories
	}

	// Add usage examples if available
	if examples := g.extractUsageExamples(spec); len(examples) > 0 {
		schema["examples"] = examples
	}

	// Add authentication requirements
	if authInfo := g.extractAuthenticationInfo(spec); authInfo != nil {
		schema["authentication"] = authInfo
	}

	return schema, nil
}

// extractToolMetadata extracts metadata about the tool from the spec
func (g *SchemaGenerator) extractToolMetadata(spec *openapi3.T, toolName string) map[string]interface{} {
	metadata := map[string]interface{}{
		"name": toolName,
	}

	if spec.Info != nil {
		if spec.Info.Title != "" {
			metadata["display_name"] = spec.Info.Title
		}

		// Generate comprehensive description
		var desc strings.Builder
		if spec.Info.Description != "" {
			desc.WriteString(spec.Info.Description)
		} else if spec.Info.Title != "" {
			desc.WriteString(fmt.Sprintf("API for %s", spec.Info.Title))
		}

		// Add contact info if available
		if spec.Info.Contact != nil {
			if spec.Info.Contact.URL != "" {
				desc.WriteString(fmt.Sprintf(". Documentation: %s", spec.Info.Contact.URL))
			}
		}

		metadata["description"] = desc.String()

		if spec.Info.Version != "" {
			metadata["version"] = spec.Info.Version
		}
	}

	return metadata
}

// extractSemanticCategories extracts categories from tags and paths
func (g *SchemaGenerator) extractSemanticCategories(spec *openapi3.T) []map[string]interface{} {
	categories := make(map[string]map[string]interface{})

	// Extract from tags
	if spec.Tags != nil {
		for _, tag := range spec.Tags {
			if tag.Name != "" {
				categories[tag.Name] = map[string]interface{}{
					"name":        tag.Name,
					"description": tag.Description,
				}
			}
		}
	}

	// Extract from paths
	for path := range spec.Paths.Map() {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) > 0 {
			category := parts[0]
			// Skip version indicators
			if category == "v1" || category == "v2" || category == "api" {
				if len(parts) > 1 {
					category = parts[1]
				}
			}

			if _, exists := categories[category]; !exists {
				categories[category] = map[string]interface{}{
					"name":        category,
					"description": fmt.Sprintf("Operations related to %s", category),
				}
			}
		}
	}

	// Convert to slice
	result := make([]map[string]interface{}, 0, len(categories))
	for _, cat := range categories {
		result = append(result, cat)
	}

	return result
}

// extractUsageExamples extracts examples from the spec
func (g *SchemaGenerator) extractUsageExamples(spec *openapi3.T) []map[string]interface{} {
	examples := []map[string]interface{}{}

	// Look for examples in a few common operations
	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil || operation.RequestBody == nil {
				continue
			}

			if content := operation.RequestBody.Value.Content.Get("application/json"); content != nil {
				if content.Example != nil {
					examples = append(examples, map[string]interface{}{
						"operation":   operation.OperationID,
						"method":      method,
						"path":        path,
						"description": operation.Summary,
						"request":     content.Example,
					})
				}

				if len(examples) >= 3 {
					break // Limit examples
				}
			}
		}

		if len(examples) >= 3 {
			break
		}
	}

	return examples
}

// extractAuthenticationInfo extracts authentication requirements
func (g *SchemaGenerator) extractAuthenticationInfo(spec *openapi3.T) map[string]interface{} {
	if spec.Components == nil || len(spec.Components.SecuritySchemes) == 0 {
		return nil
	}

	authInfo := make(map[string]interface{})
	schemes := []map[string]interface{}{}

	for name, schemeRef := range spec.Components.SecuritySchemes {
		if schemeRef.Value == nil {
			continue
		}

		scheme := schemeRef.Value
		schemeInfo := map[string]interface{}{
			"name": name,
			"type": scheme.Type,
		}

		if scheme.Description != "" {
			schemeInfo["description"] = scheme.Description
		}

		switch scheme.Type {
		case "apiKey":
			schemeInfo["in"] = scheme.In
			schemeInfo["name"] = scheme.Name
		case "http":
			schemeInfo["scheme"] = scheme.Scheme
		case "oauth2":
			if scheme.Flows != nil {
				schemeInfo["flows"] = "OAuth2"
			}
		}

		schemes = append(schemes, schemeInfo)
	}

	if len(schemes) > 0 {
		authInfo["schemes"] = schemes
		authInfo["required"] = len(spec.Security) > 0
	}

	return authInfo
}
