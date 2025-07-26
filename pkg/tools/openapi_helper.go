package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPIHelper provides helper functions for OpenAPI operations
type OpenAPIHelper struct {
	httpClient *http.Client
	logger     observability.Logger
	auth       *DynamicAuthenticator
}

// NewOpenAPIHelper creates a new OpenAPI helper
func NewOpenAPIHelper(logger observability.Logger) *OpenAPIHelper {
	return &OpenAPIHelper{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
		auth:   NewDynamicAuthenticator(),
	}
}

// MakeAuthenticatedRequest makes an authenticated HTTP request
func (h *OpenAPIHelper) MakeAuthenticatedRequest(
	ctx context.Context,
	method, url string,
	body io.Reader,
	creds *models.TokenCredential,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication using dynamic authenticator
	if err := h.auth.ApplyAuthentication(req, creds); err != nil {
		return nil, fmt.Errorf("failed to authenticate request: %w", err)
	}

	// Add common headers
	req.Header.Set("User-Agent", "DevOps-MCP/1.0")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// GenerateToolFromOperation creates a tool from an OpenAPI operation
func (h *OpenAPIHelper) GenerateToolFromOperation(
	operationID string,
	path string,
	method string,
	operation *openapi3.Operation,
	baseURL string,
) (*Tool, error) {
	// Generate tool name
	toolName := h.generateToolName(operationID, path, method)

	// Generate description
	description := operation.Summary
	if description == "" {
		description = operation.Description
	}
	if description == "" {
		description = fmt.Sprintf("%s %s", method, path)
	}

	// Generate parameters schema
	paramSchema, required := h.generateParameterSchema(operation)

	// Generate return schema
	returnSchema := h.generateReturnSchema(operation)

	// Create tool definition
	definition := ToolDefinition{
		Name:        toolName,
		Description: description,
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: paramSchema,
			Required:   required,
		},
		Returns: returnSchema,
		Tags:    h.generateTags(operation),
	}

	// Create handler
	handler := h.createOperationHandler(path, method, operation, baseURL)

	return &Tool{
		Definition: definition,
		Handler:    handler,
	}, nil
}

// generateToolName generates a tool name from operation details
func (h *OpenAPIHelper) generateToolName(operationID, path, method string) string {
	if operationID != "" {
		return h.sanitizeName(operationID)
	}

	// Generate from path and method
	parts := strings.Split(strings.Trim(path, "/"), "/")
	name := strings.ToLower(method)

	for _, part := range parts {
		if !strings.HasPrefix(part, "{") {
			name += "_" + h.sanitizeName(part)
		}
	}

	return name
}

// sanitizeName sanitizes a string to be a valid tool name
func (h *OpenAPIHelper) sanitizeName(s string) string {
	// Replace non-alphanumeric characters with underscores
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, s)

	// Convert to lowercase and trim underscores
	result = strings.ToLower(result)
	result = strings.Trim(result, "_")

	// Replace multiple underscores with single
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	return result
}

// getSchemaType returns the type of an OpenAPI schema as a string
func getSchemaType(schema *openapi3.Schema) string {
	if schema.Type == nil {
		return "string" // default
	}

	// Type is a slice in OpenAPI 3.1
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

// generateParameterSchema generates parameter schema from OpenAPI operation
func (h *OpenAPIHelper) generateParameterSchema(operation *openapi3.Operation) (map[string]PropertySchema, []string) {
	properties := make(map[string]PropertySchema)
	required := []string{}

	// Add path parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.In == "path" {
			schema := h.schemaToPropertySchema(param.Value.Schema)
			schema.Description = param.Value.Description
			properties[param.Value.Name] = schema
			if param.Value.Required {
				required = append(required, param.Value.Name)
			}
		}
	}

	// Add query parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.In == "query" {
			schema := h.schemaToPropertySchema(param.Value.Schema)
			schema.Description = param.Value.Description
			properties[param.Value.Name] = schema
			if param.Value.Required {
				required = append(required, param.Value.Name)
			}
		}
	}

	// Add request body
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if jsonContent, ok := operation.RequestBody.Value.Content["application/json"]; ok {
			if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
				// Add body parameters
				if jsonContent.Schema.Value.Properties != nil {
					for name, prop := range jsonContent.Schema.Value.Properties {
						schema := h.schemaRefToPropertySchema(prop)
						properties["body_"+name] = schema

						// Check if required
						for _, req := range jsonContent.Schema.Value.Required {
							if req == name {
								required = append(required, "body_"+name)
							}
						}
					}
				}
			}
		}
	}

	return properties, required
}

// schemaToPropertySchema converts OpenAPI schema to property schema
func (h *OpenAPIHelper) schemaToPropertySchema(schema *openapi3.SchemaRef) PropertySchema {
	if schema == nil || schema.Value == nil {
		return PropertySchema{Type: "string"}
	}

	return h.schemaValueToPropertySchema(schema.Value)
}

// schemaRefToPropertySchema converts OpenAPI schema ref to property schema
func (h *OpenAPIHelper) schemaRefToPropertySchema(schema *openapi3.SchemaRef) PropertySchema {
	if schema == nil || schema.Value == nil {
		return PropertySchema{Type: "string"}
	}

	return h.schemaValueToPropertySchema(schema.Value)
}

// schemaValueToPropertySchema converts OpenAPI schema value to property schema
func (h *OpenAPIHelper) schemaValueToPropertySchema(schema *openapi3.Schema) PropertySchema {
	prop := PropertySchema{
		Type:        getSchemaType(schema),
		Description: schema.Description,
	}

	// Handle enums
	if len(schema.Enum) > 0 {
		prop.Enum = make([]interface{}, len(schema.Enum))
		copy(prop.Enum, schema.Enum)
	}

	// Handle defaults
	if schema.Default != nil {
		prop.Default = schema.Default
	}

	// Handle arrays
	if getSchemaType(schema) == "array" && schema.Items != nil {
		itemSchema := h.schemaRefToPropertySchema(schema.Items)
		prop.Items = &itemSchema
	}

	// Handle objects
	if getSchemaType(schema) == "object" && len(schema.Properties) > 0 {
		prop.Properties = make(map[string]PropertySchema)
		for name, propSchema := range schema.Properties {
			prop.Properties[name] = h.schemaRefToPropertySchema(propSchema)
		}
	}

	return prop
}

// generateReturnSchema generates return schema from OpenAPI operation
func (h *OpenAPIHelper) generateReturnSchema(operation *openapi3.Operation) ReturnSchema {
	// Look for 200 OK response
	if operation.Responses != nil && operation.Responses.Map() != nil {
		responses := operation.Responses.Map()
		if resp200, ok := responses["200"]; ok && resp200.Value != nil {
			if jsonContent, ok := resp200.Value.Content["application/json"]; ok {
				if jsonContent.Schema != nil && jsonContent.Schema.Value != nil {
					schema := jsonContent.Schema.Value
					desc := ""
					if resp200.Value.Description != nil {
						desc = *resp200.Value.Description
					}
					return ReturnSchema{
						Type:        getSchemaType(schema),
						Description: desc,
					}
				}
			}
		}
	}

	// Default return schema
	return ReturnSchema{
		Type:        "object",
		Description: "Operation response",
	}
}

// generateTags generates tags for the tool
func (h *OpenAPIHelper) generateTags(operation *openapi3.Operation) []string {
	tags := []string{"openapi"}

	// Add operation tags
	tags = append(tags, operation.Tags...)

	// Add capability-based tags
	if strings.Contains(strings.ToLower(operation.OperationID), "create") {
		tags = append(tags, "write")
	} else if strings.Contains(strings.ToLower(operation.OperationID), "get") ||
		strings.Contains(strings.ToLower(operation.OperationID), "list") {
		tags = append(tags, "read")
	}

	return tags
}

// createOperationHandler creates a handler for an OpenAPI operation
func (h *OpenAPIHelper) createOperationHandler(
	path string,
	method string,
	operation *openapi3.Operation,
	baseURL string,
) ToolHandler {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		// This will be implemented by the OpenAPIAdapter
		// The handler needs access to credentials and other config
		return map[string]interface{}{
			"status":  "handler_not_implemented",
			"message": fmt.Sprintf("Handler for %s %s needs to be created by OpenAPIAdapter", method, path),
		}, nil
	}
}

// TestConnection provides a basic connection test
func (h *OpenAPIHelper) TestConnection(ctx context.Context, config ToolConfig) error {
	// Make a simple authenticated request to test connectivity
	resp, err := h.MakeAuthenticatedRequest(ctx, "GET", config.BaseURL, nil, config.Credential)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			h.logger.Debugf("failed to close response body: %v", err)
		}
	}()

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed: status %d", resp.StatusCode)
	}

	// Any non-5xx response is considered successful for connection test
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: status %d", resp.StatusCode)
	}

	return nil
}

// BuildRequestURL builds the complete request URL with path parameters
func (h *OpenAPIHelper) BuildRequestURL(baseURL, path string, params map[string]interface{}) string {
	// Replace path parameters
	reqURL := baseURL + path

	// Replace {param} style parameters
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		reqURL = strings.Replace(reqURL, placeholder, fmt.Sprintf("%v", value), 1)
	}

	// Add query parameters
	queryParams := url.Values{}
	for key, value := range params {
		// Skip if it was a path parameter
		if strings.Contains(path, fmt.Sprintf("{%s}", key)) {
			continue
		}
		// Skip body parameters
		if strings.HasPrefix(key, "body_") {
			continue
		}
		// Add as query parameter
		queryParams.Set(key, fmt.Sprintf("%v", value))
	}

	if len(queryParams) > 0 {
		reqURL += "?" + queryParams.Encode()
	}

	return reqURL
}

// ExtractBodyParameters extracts body parameters from the params map
func (h *OpenAPIHelper) ExtractBodyParameters(params map[string]interface{}) map[string]interface{} {
	body := make(map[string]interface{})

	for key, value := range params {
		if strings.HasPrefix(key, "body_") {
			// Remove the "body_" prefix
			actualKey := strings.TrimPrefix(key, "body_")
			body[actualKey] = value
		}
	}

	return body
}
