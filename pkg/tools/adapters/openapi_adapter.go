package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPIAdapter is a generic adapter that works with any OpenAPI specification
type OpenAPIAdapter struct {
	helper           *tools.OpenAPIHelper
	logger           observability.Logger
	discoveryService *DiscoveryService
	auth             *tools.DynamicAuthenticator
}

// NewOpenAPIAdapter creates a new OpenAPI adapter
func NewOpenAPIAdapter(logger observability.Logger) *OpenAPIAdapter {
	return &OpenAPIAdapter{
		helper:           tools.NewOpenAPIHelper(logger),
		logger:           logger,
		discoveryService: NewDiscoveryService(logger),
		auth:             tools.NewDynamicAuthenticator(),
	}
}

// NewOpenAPIAdapterWithDiscovery creates a new OpenAPI adapter with a custom discovery service
func NewOpenAPIAdapterWithDiscovery(logger observability.Logger, discoveryService *DiscoveryService) *OpenAPIAdapter {
	return &OpenAPIAdapter{
		helper:           tools.NewOpenAPIHelper(logger),
		logger:           logger,
		discoveryService: discoveryService,
		auth:             tools.NewDynamicAuthenticator(),
	}
}

// DiscoverAPIs discovers OpenAPI specifications
func (a *OpenAPIAdapter) DiscoverAPIs(ctx context.Context, config tools.ToolConfig) (*tools.DiscoveryResult, error) {
	// Use discovery service to find OpenAPI spec
	discoveryResult, err := a.discoveryService.DiscoverOpenAPISpec(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Parse and validate the spec
	if discoveryResult.OpenAPISpec != nil {
		// Extract capabilities from the spec
		capabilities := a.extractCapabilities(discoveryResult.OpenAPISpec)
		discoveryResult.Capabilities = capabilities

		// Add metadata
		if discoveryResult.OpenAPISpec.Info != nil {
			discoveryResult.Metadata["title"] = discoveryResult.OpenAPISpec.Info.Title
			discoveryResult.Metadata["version"] = discoveryResult.OpenAPISpec.Info.Version
			discoveryResult.Metadata["description"] = discoveryResult.OpenAPISpec.Info.Description
		}
	}

	return discoveryResult, nil
}

// GenerateTools generates tools from OpenAPI specification
func (a *OpenAPIAdapter) GenerateTools(config tools.ToolConfig, spec *openapi3.T) ([]*tools.Tool, error) {
	if spec == nil {
		return nil, fmt.Errorf("OpenAPI specification is nil")
	}

	generatedTools := []*tools.Tool{}

	// Get base URL from spec or config
	baseURL := config.BaseURL
	if len(spec.Servers) > 0 {
		baseURL = spec.Servers[0].URL
	}

	// Process each path and operation
	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Process each HTTP method
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, operation := range operations {
			if operation == nil {
				continue
			}

			// Generate tool from operation
			tool, err := a.generateToolFromOperation(
				operation.OperationID,
				path,
				method,
				operation,
				baseURL,
				config,
			)
			if err != nil {
				a.logger.Warn("Failed to generate tool from operation", map[string]interface{}{
					"path":   path,
					"method": method,
					"error":  err.Error(),
				})
				continue
			}

			generatedTools = append(generatedTools, tool)
		}
	}

	a.logger.Info("Generated tools from OpenAPI spec", map[string]interface{}{
		"count": len(generatedTools),
		"title": spec.Info.Title,
	})

	return generatedTools, nil
}

// generateToolFromOperation creates a tool from an OpenAPI operation
func (a *OpenAPIAdapter) generateToolFromOperation(
	operationID string,
	path string,
	method string,
	operation *openapi3.Operation,
	baseURL string,
	config tools.ToolConfig,
) (*tools.Tool, error) {
	// Generate tool from operation
	baseTool, err := a.helper.GenerateToolFromOperation(operationID, path, method, operation, baseURL)
	if err != nil {
		return nil, err
	}

	// Override handler with our dynamic implementation
	baseTool.Handler = a.createDynamicHandler(path, method, operation, baseURL, config)

	// Add tool type prefix to avoid naming conflicts
	if config.Name != "" {
		baseTool.Definition.Name = fmt.Sprintf("%s_%s",
			a.sanitizePrefix(config.Name),
			baseTool.Definition.Name,
		)
	}

	return baseTool, nil
}

// createDynamicHandler creates a handler that makes actual API calls
func (a *OpenAPIAdapter) createDynamicHandler(
	path string,
	method string,
	operation *openapi3.Operation,
	baseURL string,
	config tools.ToolConfig,
) tools.ToolHandler {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		// Build the request URL
		requestURL := a.buildRequestURL(baseURL, path, params)

		// Build request body for POST/PUT/PATCH
		var body io.Reader
		if method == "POST" || method == "PUT" || method == "PATCH" {
			bodyData := a.extractBodyParameters(params)
			if len(bodyData) > 0 {
				jsonBody, err := json.Marshal(bodyData)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal request body: %w", err)
				}
				body = strings.NewReader(string(jsonBody))
			}
		}

		// Determine which credentials to use
		credential := config.Credential // Default to service account

		// Check for user credentials in context
		if userCreds, ok := auth.GetToolCredentials(ctx); ok && config.Provider != "" {
			// Get credential for the specific provider
			var userCred *models.TokenCredential
			switch config.Provider {
			case "github":
				userCred = userCreds.GitHub
			case "gitlab":
				userCred = userCreds.GitLab
			default:
				if userCreds.Custom != nil {
					userCred = userCreds.Custom[config.Provider]
				}
			}

			// Use user credential if available
			if userCred != nil {
				credential = userCred
				a.logger.Debug("Using passthrough credentials for API request", map[string]interface{}{
					"provider": config.Provider,
					"tool_id":  config.ID,
				})
			} else if config.PassthroughConfig != nil && config.PassthroughConfig.Mode == "required" {
				return nil, fmt.Errorf("passthrough token required but not found for provider %s", config.Provider)
			}
		} else if config.PassthroughConfig != nil && config.PassthroughConfig.Mode == "required" {
			return nil, fmt.Errorf("passthrough authentication required but no user credentials provided")
		}

		// Validate we have credentials
		if credential == nil {
			return nil, fmt.Errorf("no credentials available for tool %s", config.Name)
		}

		// Make the request with selected credentials
		resp, err := a.helper.MakeAuthenticatedRequest(
			ctx,
			method,
			requestURL,
			body,
			credential,
		)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				// Log error but don't fail the operation
				a.logger.Debugf("failed to close response body: %v", err)
			}
		}()

		// Read response
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Parse response based on content type
		contentType := resp.Header.Get("Content-Type")

		// Handle non-2xx status codes
		if resp.StatusCode >= 400 {
			var errorResponse map[string]interface{}
			if strings.Contains(contentType, "application/json") {
				if err := json.Unmarshal(responseBody, &errorResponse); err != nil {
					// Log unmarshal error but continue with raw body
					a.logger.Debugf("failed to unmarshal error response: %v", err)
				}
			}

			return nil, fmt.Errorf("API error (status %d): %s",
				resp.StatusCode,
				a.formatErrorResponse(errorResponse, string(responseBody)),
			)
		}

		// Parse successful response
		if strings.Contains(contentType, "application/json") {
			var result interface{}
			if err := json.Unmarshal(responseBody, &result); err != nil {
				return nil, fmt.Errorf("failed to parse JSON response: %w", err)
			}
			return result, nil
		}

		// Return raw response for non-JSON
		return map[string]interface{}{
			"status": resp.StatusCode,
			"body":   string(responseBody),
		}, nil
	}
}

// buildRequestURL builds the complete request URL with path parameters
func (a *OpenAPIAdapter) buildRequestURL(baseURL, path string, params map[string]interface{}) string {
	return a.helper.BuildRequestURL(baseURL, path, params)
}

// extractBodyParameters extracts body parameters from the params map
func (a *OpenAPIAdapter) extractBodyParameters(params map[string]interface{}) map[string]interface{} {
	return a.helper.ExtractBodyParameters(params)
}

// formatErrorResponse formats an error response for display
func (a *OpenAPIAdapter) formatErrorResponse(errorResponse map[string]interface{}, rawBody string) string {
	// Try common error message fields
	errorFields := []string{"message", "error", "error_description", "detail", "details"}

	for _, field := range errorFields {
		if msg, ok := errorResponse[field]; ok {
			return fmt.Sprintf("%v", msg)
		}
	}

	// Return raw body if no standard error field found
	if len(rawBody) > 200 {
		return rawBody[:200] + "..."
	}
	return rawBody
}

// extractCapabilities extracts capabilities from OpenAPI spec
func (a *OpenAPIAdapter) extractCapabilities(spec *openapi3.T) []tools.Capability {
	capabilities := []tools.Capability{}
	capMap := make(map[string][]string)

	// Analyze operations to determine capabilities
	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Categorize by resource type
		resourceType := a.extractResourceType(path)
		if resourceType == "" {
			continue
		}

		// Check operations
		if pathItem.Get != nil {
			capMap[resourceType] = append(capMap[resourceType], "read")
		}
		if pathItem.Post != nil {
			capMap[resourceType] = append(capMap[resourceType], "create")
		}
		if pathItem.Put != nil || pathItem.Patch != nil {
			capMap[resourceType] = append(capMap[resourceType], "update")
		}
		if pathItem.Delete != nil {
			capMap[resourceType] = append(capMap[resourceType], "delete")
		}
	}

	// Convert to capabilities
	for resource, actions := range capMap {
		capability := tools.Capability{
			Name:        fmt.Sprintf("%s_management", resource),
			Description: fmt.Sprintf("Manage %s resources", resource),
			Actions:     actions,
		}
		capabilities = append(capabilities, capability)
	}

	return capabilities
}

// extractResourceType extracts the resource type from a path
func (a *OpenAPIAdapter) extractResourceType(path string) string {
	// Remove leading slash and split
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// Look for resource names (usually plural nouns)
	for _, part := range parts {
		// Skip parameters and common prefixes
		if strings.Contains(part, "{") || part == "api" || part == "v1" || part == "v2" {
			continue
		}

		// Return the first meaningful part
		if len(part) > 2 {
			return strings.TrimSuffix(part, "s") // Simple depluralization
		}
	}

	return ""
}

// sanitizePrefix sanitizes a string to be used as a tool name prefix
func (a *OpenAPIAdapter) sanitizePrefix(s string) string {
	// Remove special characters and convert to lowercase
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, s)

	return strings.ToLower(strings.Trim(result, "_"))
}

// AuthenticateRequest adds authentication to HTTP requests based on OpenAPI security schemes
func (a *OpenAPIAdapter) AuthenticateRequest(req *http.Request, creds *models.TokenCredential, securitySchemes map[string]tools.SecurityScheme) error {
	// Use the dynamic authenticator to apply authentication
	return a.auth.ApplyAuthentication(req, creds)
}

// TestConnection tests the connection to the tool
func (a *OpenAPIAdapter) TestConnection(ctx context.Context, config tools.ToolConfig) error {
	// Use the OpenAPI helper to test connection
	return a.helper.TestConnection(ctx, config)
}

// ExtractSecuritySchemes extracts security schemes from OpenAPI spec
func (a *OpenAPIAdapter) ExtractSecuritySchemes(spec *openapi3.T) map[string]tools.SecurityScheme {
	// Use the dynamic authenticator to extract security schemes
	return a.auth.ExtractSecuritySchemes(spec)
}
