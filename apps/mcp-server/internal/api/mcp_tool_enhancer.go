package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jmoiron/sqlx"
)

// MCPToolEnhancer enhances MCP tool schemas with AI-friendly information
type MCPToolEnhancer struct {
	db            *sqlx.DB
	restClient    clients.RESTAPIClient
	logger        observability.Logger
	schemaCache   map[string]*EnhancedToolSchema
	cacheRepo     repository.OpenAPICacheRepository
}

// EnhancedToolSchema contains enhanced schema information for MCP tools
type EnhancedToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Examples    []UsageExample         `json:"examples,omitempty"`
	Operations  []OperationInfo        `json:"operations,omitempty"`
	Hints       map[string]interface{} `json:"hints,omitempty"`
}

// UsageExample represents a usage example for a tool
type UsageExample struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Arguments   map[string]interface{} `json:"arguments"`
	Explanation string                 `json:"explanation"`
}

// OperationInfo contains information about an available operation
type OperationInfo struct {
	ID          string                 `json:"id"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Summary     string                 `json:"summary"`
	Parameters  []ParameterInfo        `json:"parameters"`
}

// ParameterInfo contains parameter information
type ParameterInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Example     interface{} `json:"example,omitempty"`
}

// NewMCPToolEnhancer creates a new MCP tool enhancer
func NewMCPToolEnhancer(db *sqlx.DB, restClient clients.RESTAPIClient, logger observability.Logger) *MCPToolEnhancer {
	return &MCPToolEnhancer{
		db:          db,
		restClient:  restClient,
		logger:      logger,
		schemaCache: make(map[string]*EnhancedToolSchema),
		cacheRepo:   repository.NewOpenAPICacheRepository(db),
	}
}

// GenerateEnhancedSchema generates an enhanced schema for a tool
func (e *MCPToolEnhancer) GenerateEnhancedSchema(ctx context.Context, tool *models.DynamicTool) (*EnhancedToolSchema, error) {
	// Check cache first
	if cached, ok := e.schemaCache[tool.ID]; ok {
		return cached, nil
	}

	// Get OpenAPI spec for the tool
	spec, err := e.getOpenAPISpec(ctx, tool)
	if err != nil {
		// Fall back to basic schema if we can't get the spec
		return e.generateBasicSchema(tool), nil
	}

	// Generate enhanced schema
	schema := &EnhancedToolSchema{
		Name:        tool.ToolName,
		Description: e.generateEnhancedDescription(tool, spec),
		InputSchema: e.generateEnhancedInputSchema(tool, spec),
		Examples:    e.generateUsageExamples(tool, spec),
		Operations:  e.extractOperations(spec),
		Hints:       e.generateHints(tool, spec),
	}

	// Cache the result
	e.schemaCache[tool.ID] = schema

	return schema, nil
}

// generateEnhancedDescription creates a comprehensive description
func (e *MCPToolEnhancer) generateEnhancedDescription(tool *models.DynamicTool, spec *openapi3.T) string {
	var desc strings.Builder
	
	// Base description
	if tool.Description != nil && *tool.Description != "" {
		desc.WriteString(*tool.Description)
	} else if tool.DisplayName != "" {
		desc.WriteString(fmt.Sprintf("%s integration", tool.DisplayName))
	} else {
		desc.WriteString(fmt.Sprintf("%s API integration", tool.ToolName))
	}

	// Add available operations summary
	if spec != nil && spec.Paths != nil {
		operationCount := 0
		resourceTypes := make(map[string]bool)
		
		for path, pathItem := range spec.Paths.Map() {
			if pathItem != nil {
				operationCount += len(pathItem.Operations())
				// Extract resource type from path
				parts := strings.Split(strings.Trim(path, "/"), "/")
				for _, part := range parts {
					if !strings.HasPrefix(part, "{") && part != "v1" && part != "v2" && part != "api" {
						resourceTypes[part] = true
					}
				}
			}
		}
		
		if operationCount > 0 {
			desc.WriteString(fmt.Sprintf(". Provides %d operations", operationCount))
			if len(resourceTypes) > 0 {
				resources := make([]string, 0, len(resourceTypes))
				for r := range resourceTypes {
					resources = append(resources, r)
				}
				if len(resources) <= 5 {
					desc.WriteString(fmt.Sprintf(" for: %s", strings.Join(resources, ", ")))
				}
			}
		}
	}

	return desc.String()
}

// generateEnhancedInputSchema creates an AI-friendly input schema
func (e *MCPToolEnhancer) generateEnhancedInputSchema(tool *models.DynamicTool, spec *openapi3.T) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{},
		"required": []string{},
	}

	properties := schema["properties"].(map[string]interface{})

	// If we have an OpenAPI spec, extract available operations
	if spec != nil && spec.Paths != nil {
		operations := []string{}
		operationDetails := make(map[string]string)
		
		for _, pathItem := range spec.Paths.Map() {
			if pathItem == nil {
				continue
			}
			for _, op := range pathItem.Operations() {
				if op != nil && op.OperationID != "" {
					operations = append(operations, op.OperationID)
					if op.Summary != "" {
						operationDetails[op.OperationID] = op.Summary
					}
				}
			}
		}

		if len(operations) > 0 {
			// Add operation/action parameter with enum of available operations
			operationSchema := map[string]interface{}{
				"type":        "string",
				"description": "The operation to perform",
			}
			
			// Only add enum if we have a reasonable number of operations
			if len(operations) <= 50 {
				operationSchema["enum"] = operations
				
				// Add operation descriptions as x-enumDescriptions for AI understanding
				if len(operationDetails) > 0 {
					descriptions := make(map[string]string)
					for opID, desc := range operationDetails {
						descriptions[opID] = desc
					}
					operationSchema["x-enumDescriptions"] = descriptions
				}
			} else {
				// For large APIs, provide examples instead of full enum
				examples := operations
				if len(examples) > 10 {
					examples = operations[:10]
				}
				operationSchema["examples"] = examples
				operationSchema["description"] = fmt.Sprintf("The operation to perform. %d operations available", len(operations))
			}
			
			properties["operation"] = operationSchema
			schema["required"] = append(schema["required"].([]string), "operation")
		}
	} else {
		// Fallback to generic action parameter
		properties["action"] = map[string]interface{}{
			"type":        "string",
			"description": "The action to perform (e.g., 'list', 'get', 'create', 'update', 'delete')",
			"examples":    []string{"list", "get", "create", "update", "delete"},
		}
	}

	// Add parameters object for operation-specific parameters
	properties["parameters"] = map[string]interface{}{
		"type":                 "object",
		"description":          "Parameters specific to the chosen operation",
		"additionalProperties": true,
	}

	// Add common parameters based on tool type
	e.addCommonParameters(tool, properties)

	return schema
}

// addCommonParameters adds common parameters based on tool patterns
func (e *MCPToolEnhancer) addCommonParameters(tool *models.DynamicTool, properties map[string]interface{}) {
	toolNameLower := strings.ToLower(tool.ToolName)

	// GitHub/GitLab/Bitbucket patterns
	if strings.Contains(toolNameLower, "github") || strings.Contains(toolNameLower, "gitlab") || strings.Contains(toolNameLower, "bitbucket") {
		properties["owner"] = map[string]interface{}{
			"type":        "string",
			"description": "Repository owner or organization",
			"examples":    []string{"octocat", "my-org"},
		}
		properties["repo"] = map[string]interface{}{
			"type":        "string",
			"description": "Repository name",
			"examples":    []string{"my-repo", "hello-world"},
		}
	}

	// Snyk patterns
	if strings.Contains(toolNameLower, "snyk") {
		properties["org_id"] = map[string]interface{}{
			"type":        "string",
			"description": "Snyk organization ID",
		}
		properties["project_id"] = map[string]interface{}{
			"type":        "string",
			"description": "Snyk project ID",
		}
		properties["version"] = map[string]interface{}{
			"type":        "string",
			"description": "API version",
			"default":     "2024-10-15",
			"examples":    []string{"2024-10-15", "2024-01-01"},
		}
	}

	// Jira patterns
	if strings.Contains(toolNameLower, "jira") {
		properties["project_key"] = map[string]interface{}{
			"type":        "string",
			"description": "Jira project key",
			"examples":    []string{"PROJ", "DEV"},
		}
		properties["issue_key"] = map[string]interface{}{
			"type":        "string",
			"description": "Jira issue key",
			"examples":    []string{"PROJ-123", "DEV-456"},
		}
	}
}

// generateUsageExamples creates usage examples for the tool
func (e *MCPToolEnhancer) generateUsageExamples(tool *models.DynamicTool, spec *openapi3.T) []UsageExample {
	examples := []UsageExample{}
	toolNameLower := strings.ToLower(tool.ToolName)

	// Tool-specific examples
	if strings.Contains(toolNameLower, "snyk") {
		examples = append(examples, UsageExample{
			Title:       "Scan a project for vulnerabilities",
			Description: "Run a security scan on a Snyk project",
			Arguments: map[string]interface{}{
				"operation":  "test_project",
				"parameters": map[string]interface{}{
					"org_id":     "your-org-id",
					"project_id": "your-project-id",
					"version":    "2024-10-15",
				},
			},
			Explanation: "This scans the project and returns vulnerability information",
		})
	} else if strings.Contains(toolNameLower, "github") {
		examples = append(examples, UsageExample{
			Title:       "List repository issues",
			Description: "Get all open issues for a repository",
			Arguments: map[string]interface{}{
				"operation": "issues/list",
				"parameters": map[string]interface{}{
					"owner": "octocat",
					"repo":  "hello-world",
					"state": "open",
				},
			},
			Explanation: "Returns a list of open issues for the specified repository",
		})
	}

	// Add generic examples if we have operations from spec
	if spec != nil && len(examples) == 0 {
		for _, pathItem := range spec.Paths.Map() {
			if pathItem == nil {
				continue
			}
			for method, op := range pathItem.Operations() {
				if op != nil && op.OperationID != "" && strings.ToUpper(method) == "GET" {
					examples = append(examples, UsageExample{
						Title:       fmt.Sprintf("Example: %s", op.OperationID),
						Description: op.Summary,
						Arguments: map[string]interface{}{
							"operation":  op.OperationID,
							"parameters": map[string]interface{}{},
						},
						Explanation: "Execute this operation to retrieve data",
					})
					if len(examples) >= 2 {
						break
					}
				}
			}
			if len(examples) >= 2 {
				break
			}
		}
	}

	return examples
}

// extractOperations extracts operation information from the spec
func (e *MCPToolEnhancer) extractOperations(spec *openapi3.T) []OperationInfo {
	operations := []OperationInfo{}

	if spec == nil || spec.Paths == nil {
		return operations
	}

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, op := range pathItem.Operations() {
			if op == nil {
				continue
			}

			opInfo := OperationInfo{
				ID:      op.OperationID,
				Method:  strings.ToUpper(method),
				Path:    path,
				Summary: op.Summary,
			}

			// Extract parameters
			for _, param := range op.Parameters {
				if param.Value != nil {
					opInfo.Parameters = append(opInfo.Parameters, ParameterInfo{
						Name:        param.Value.Name,
						Type:        e.getParameterType(param.Value),
						Required:    param.Value.Required,
						Description: param.Value.Description,
						Example:     param.Value.Example,
					})
				}
			}

			operations = append(operations, opInfo)

			// Limit operations to prevent huge schemas
			if len(operations) >= 20 {
				break
			}
		}
	}

	return operations
}

// generateHints generates AI hints for better tool usage
func (e *MCPToolEnhancer) generateHints(tool *models.DynamicTool, spec *openapi3.T) map[string]interface{} {
	hints := make(map[string]interface{})
	toolNameLower := strings.ToLower(tool.ToolName)

	// Authentication hints
	if tool.AuthType != "" {
		hints["authentication"] = map[string]interface{}{
			"type":   tool.AuthType,
			"hint":   e.getAuthHint(tool.AuthType),
		}
	}

	// Rate limiting hints
	hints["rateLimiting"] = "Be aware of rate limits. Implement exponential backoff on 429 errors."

	// Tool-specific hints
	if strings.Contains(toolNameLower, "snyk") {
		hints["domain"] = "security"
		hints["terminology"] = []string{
			"vulnerability", "CVE", "CVSS", "severity",
			"dependency", "license", "remediation", "patch",
		}
		hints["bestPractices"] = []string{
			"Always include version parameter for API stability",
			"Use project_id for project-specific operations",
			"Check org_id is correct before operations",
		}
	} else if strings.Contains(toolNameLower, "github") {
		hints["domain"] = "development"
		hints["terminology"] = []string{
			"repository", "issue", "pull request", "commit",
			"branch", "release", "workflow", "action",
		}
		hints["bestPractices"] = []string{
			"Use owner and repo parameters for repository operations",
			"Include state parameter when listing issues or PRs",
			"Paginate results for large datasets",
		}
	}

	return hints
}

// getAuthHint provides authentication hints based on type
func (e *MCPToolEnhancer) getAuthHint(authType string) string {
	switch authType {
	case "bearer":
		return "Include Bearer token in Authorization header"
	case "api_key":
		return "Include API key as configured in tool settings"
	case "oauth2":
		return "Use OAuth2 flow for authentication"
	case "basic":
		return "Use Basic authentication with username:password"
	default:
		return "Follow API documentation for authentication"
	}
}

// getParameterType extracts parameter type from OpenAPI parameter
func (e *MCPToolEnhancer) getParameterType(param *openapi3.Parameter) string {
	if param.Schema != nil && param.Schema.Value != nil {
		if param.Schema.Value.Type != nil {
			// Type is openapi3.Types which is a slice
			if param.Schema.Value.Type.Is("string") {
				return "string"
			} else if param.Schema.Value.Type.Is("integer") {
				return "integer"
			} else if param.Schema.Value.Type.Is("number") {
				return "number"
			} else if param.Schema.Value.Type.Is("boolean") {
				return "boolean"
			} else if param.Schema.Value.Type.Is("array") {
				return "array"
			} else if param.Schema.Value.Type.Is("object") {
				return "object"
			}
		}
	}
	return "string"
}

// getOpenAPISpec retrieves the OpenAPI spec for a tool
func (e *MCPToolEnhancer) getOpenAPISpec(ctx context.Context, tool *models.DynamicTool) (*openapi3.T, error) {
	// If we have the spec in the tool
	if tool.OpenAPISpec != nil {
		var specData json.RawMessage
		if err := json.Unmarshal(*tool.OpenAPISpec, &specData); err == nil {
			loader := openapi3.NewLoader()
			spec, err := loader.LoadFromData([]byte(specData))
			if err == nil {
				return spec, nil
			}
		}
		
		// Try treating it as direct JSON
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData([]byte(*tool.OpenAPISpec))
		if err == nil {
			return spec, nil
		}
	}

	// Try to fetch from URL if available
	if tool.OpenAPIURL != nil && *tool.OpenAPIURL != "" {
		// This would involve fetching the spec from the URL
		// For now, return nil to use fallback
	}

	return nil, fmt.Errorf("no OpenAPI spec available")
}

// generateBasicSchema generates a basic schema when no spec is available
func (e *MCPToolEnhancer) generateBasicSchema(tool *models.DynamicTool) *EnhancedToolSchema {
	return &EnhancedToolSchema{
		Name:        tool.ToolName,
		Description: fmt.Sprintf("%s API integration", tool.DisplayName),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "The action to perform",
					"examples":    []string{"list", "get", "create", "update", "delete"},
				},
				"parameters": map[string]interface{}{
					"type":                 "object",
					"description":          "Action parameters",
					"additionalProperties": true,
				},
			},
			"required": []string{"action"},
		},
	}
}