package openapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPIAdapter is the ONLY adapter needed for all tools
type OpenAPIAdapter struct {
	httpClient *http.Client
	parser     *openapi3.Loader
}

// NewOpenAPIAdapter creates a new OpenAPI adapter
func NewOpenAPIAdapter() *OpenAPIAdapter {
	return &OpenAPIAdapter{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		parser: openapi3.NewLoader(),
	}
}

// DiscoverAPIs attempts to discover OpenAPI specifications
func (a *OpenAPIAdapter) DiscoverAPIs(ctx context.Context, config tool.ToolConfig) (*tool.DiscoveryResult, error) {
	result := &tool.DiscoveryResult{
		Status:         "discovering",
		DiscoveredURLs: []string{},
	}

	// If OpenAPI URL is provided, use it directly
	if config.OpenAPIURL != "" {
		spec, err := a.loadOpenAPISpec(ctx, config.OpenAPIURL, config.Credential)
		if err == nil {
			result.Status = "success"
			result.OpenAPISpec = spec
			result.DiscoveredURLs = append(result.DiscoveredURLs, config.OpenAPIURL)
			result.Capabilities = a.extractCapabilities(spec)
			return result, nil
		}
		result.Error = err.Error()
	}

	// Try common OpenAPI paths
	baseURL := strings.TrimRight(config.BaseURL, "/")
	commonPaths := []string{
		// Standard OpenAPI/Swagger paths
		"/openapi.json",
		"/swagger.json",
		"/openapi.yaml",
		"/swagger.yaml",
		"/api/openapi.json",
		"/api/swagger.json",
		"/api-docs",
		"/api-docs.json",
		"/swagger-ui/swagger.json",
		"/.well-known/openapi.json",

		// Versioned paths
		"/v1/openapi.json",
		"/v2/openapi.json",
		"/v3/openapi.json",
		"/api/v1/openapi.json",
		"/api/v2/openapi.json",
		"/api/v3/openapi.json",
		"/api/v4/openapi.json",

		// Tool-specific patterns
		"/api/v3/openapi.json",          // GitHub
		"/api/v4/openapi.json",          // GitLab
		"/gateway/api/openapi.json",     // Harness
		"/api/openapi.json",             // Generic
		"/artifactory/api/openapi.json", // JFrog
		"/api/v2/openapi.json",          // Dynatrace
		"/web_api/api_description",      // SonarQube

		// Documentation paths
		"/docs/api",
		"/api/docs",
		"/developer/api",
		"/developers/api-docs",
	}

	for _, path := range commonPaths {
		url := baseURL + path
		spec, err := a.loadOpenAPISpec(ctx, url, config.Credential)
		if err == nil {
			result.Status = "success"
			result.OpenAPISpec = spec
			result.DiscoveredURLs = append(result.DiscoveredURLs, url)
			result.Capabilities = a.extractCapabilities(spec)
			return result, nil
		}
	}

	// Try subdomain discovery
	if parsed, err := url.Parse(baseURL); err == nil {
		subdomains := []string{"api", "apidocs", "docs", "developer"}
		for _, subdomain := range subdomains {
			parts := strings.Split(parsed.Host, ".")
			if len(parts) > 1 {
				// Replace first part with subdomain
				parts[0] = subdomain
				parsed.Host = strings.Join(parts, ".")
				for _, path := range []string{"/", "/openapi.json", "/swagger.json"} {
					url := parsed.String() + path
					spec, err := a.loadOpenAPISpec(ctx, url, config.Credential)
					if err == nil {
						result.Status = "success"
						result.OpenAPISpec = spec
						result.DiscoveredURLs = append(result.DiscoveredURLs, url)
						result.Capabilities = a.extractCapabilities(spec)
						return result, nil
					}
				}
			}
		}
	}

	// If documentation URL is provided, try it
	if config.DocumentationURL != "" {
		spec, err := a.loadOpenAPISpec(ctx, config.DocumentationURL, config.Credential)
		if err == nil {
			result.Status = "success"
			result.OpenAPISpec = spec
			result.DiscoveredURLs = append(result.DiscoveredURLs, config.DocumentationURL)
			result.Capabilities = a.extractCapabilities(spec)
			return result, nil
		}
	}

	result.Status = "failed"
	result.RequiresManual = true
	result.SuggestedActions = []string{
		"Provide the OpenAPI specification URL in the 'openapi_url' field",
		"Check if the API documentation is on a different subdomain",
		"Verify that the authentication credentials have access to the API documentation",
	}

	return result, nil
}

// GenerateTools creates tool definitions from an OpenAPI spec
func (a *OpenAPIAdapter) GenerateTools(config tool.ToolConfig, spec *openapi3.T) ([]*tool.DynamicTool, error) {
	var tools []*tool.DynamicTool

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			tool := a.operationToTool(config.Name, path, method, operation)
			if tool != nil {
				tools = append(tools, tool)
			}
		}
	}

	return tools, nil
}

// AuthenticateRequest adds authentication to a request based on the credential type
func (a *OpenAPIAdapter) AuthenticateRequest(req *http.Request, creds *tool.TokenCredential) error {
	if creds == nil {
		return nil
	}

	switch creds.Type {
	case "bearer":
		prefix := "Bearer"
		if creds.HeaderPrefix != "" {
			prefix = creds.HeaderPrefix
		}
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", prefix, creds.Token))

	case "api_key":
		headerName := creds.HeaderName
		if headerName == "" {
			headerName = "X-API-Key"
		}
		req.Header.Set(headerName, creds.APIKey)

	case "basic":
		req.SetBasicAuth(creds.Username, creds.Password)

	case "token":
		// Some APIs use custom token headers
		headerName := creds.HeaderName
		if headerName == "" {
			headerName = "Authorization"
		}
		req.Header.Set(headerName, creds.Token)

	default:
		return fmt.Errorf("unsupported authentication type: %s", creds.Type)
	}

	return nil
}

// TestConnection verifies that the tool is accessible
func (a *OpenAPIAdapter) TestConnection(ctx context.Context, config tool.ToolConfig) error {
	// Try to fetch the OpenAPI spec or make a simple API call
	req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := a.AuthenticateRequest(req, config.Credential); err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ExtractAuthSchemes extracts authentication schemes from the OpenAPI spec
func (a *OpenAPIAdapter) ExtractAuthSchemes(spec *openapi3.T) []tool.SecurityScheme {
	var schemes []tool.SecurityScheme

	if spec.Components == nil || spec.Components.SecuritySchemes == nil {
		return schemes
	}

	for name, schemeRef := range spec.Components.SecuritySchemes {
		if schemeRef == nil || schemeRef.Value == nil {
			continue
		}

		scheme := schemeRef.Value
		ts := tool.SecurityScheme{
			Type:        scheme.Type,
			Description: scheme.Description,
		}

		switch scheme.Type {
		case "http":
			ts.Scheme = scheme.Scheme
			ts.BearerFormat = scheme.BearerFormat
		case "apiKey":
			ts.Name = scheme.Name
			ts.In = scheme.In
		case "oauth2":
			// Handle OAuth2 if needed in future
		}

		// Use the key name if no other name is set
		if ts.Name == "" {
			ts.Name = name
		}

		schemes = append(schemes, ts)
	}

	return schemes
}

// loadOpenAPISpec loads and parses an OpenAPI specification
func (a *OpenAPIAdapter) loadOpenAPISpec(ctx context.Context, specURL string, creds *tool.TokenCredential) (*openapi3.T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
	if err != nil {
		return nil, err
	}

	if err := a.AuthenticateRequest(req, creds); err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON first
	doc, err := a.parser.LoadFromData(body)
	if err != nil {
		// Try YAML if JSON fails
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	return doc, nil
}

// extractCapabilities analyzes the OpenAPI spec to determine tool capabilities
func (a *OpenAPIAdapter) extractCapabilities(spec *openapi3.T) []tool.Capability {
	capabilities := make(map[string]*tool.Capability)

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Extract category from tags or path
			category := "general"
			if len(operation.Tags) > 0 {
				category = operation.Tags[0]
			} else {
				// Extract from path (e.g., /api/v1/users -> users)
				parts := strings.Split(strings.Trim(path, "/"), "/")
				for _, part := range parts {
					if !strings.HasPrefix(part, "v") && !strings.Contains(part, "{") {
						category = part
						break
					}
				}
			}

			if _, exists := capabilities[category]; !exists {
				capabilities[category] = &tool.Capability{
					Name:     category,
					Category: category,
					Actions:  []string{},
				}
			}

			actionName := fmt.Sprintf("%s:%s", strings.ToLower(method), path)
			if operation.OperationID != "" {
				actionName = operation.OperationID
			}

			capabilities[category].Actions = append(capabilities[category].Actions, actionName)
		}
	}

	// Convert map to slice
	var result []tool.Capability
	for _, cap := range capabilities {
		result = append(result, *cap)
	}

	return result
}

// operationToTool converts an OpenAPI operation to a tool definition
func (a *OpenAPIAdapter) operationToTool(prefix, path, method string, operation *openapi3.Operation) *tool.DynamicTool {
	if operation == nil {
		return nil
	}

	// Generate tool name
	name := fmt.Sprintf("%s_%s", prefix, operation.OperationID)
	if operation.OperationID == "" {
		// Generate from method and path
		cleanPath := strings.ReplaceAll(path, "/", "_")
		cleanPath = strings.ReplaceAll(cleanPath, "{", "")
		cleanPath = strings.ReplaceAll(cleanPath, "}", "")
		name = fmt.Sprintf("%s_%s%s", prefix, strings.ToLower(method), cleanPath)
	}

	// Build parameter schema
	params := &tool.ParameterSchema{
		Type:       "object",
		Properties: make(map[string]tool.PropertySchema),
		Required:   []string{},
	}

	// Add path parameters
	for _, param := range operation.Parameters {
		if param.Value == nil {
			continue
		}
		p := param.Value
		if p.In == "path" || p.In == "query" {
			paramSchema := tool.PropertySchema{
				Type:        getSchemaType(p.Schema),
				Description: p.Description,
			}
			params.Properties[p.Name] = paramSchema
			if p.Required {
				params.Required = append(params.Required, p.Name)
			}
		}
	}

	// Add request body parameters
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if content, ok := operation.RequestBody.Value.Content["application/json"]; ok && content.Schema != nil {
			if content.Schema.Value != nil && content.Schema.Value.Properties != nil {
				for propName, propSchema := range content.Schema.Value.Properties {
					if propSchema.Value != nil {
						params.Properties[propName] = tool.PropertySchema{
							Type:        getSchemaType(propSchema),
							Description: propSchema.Value.Description,
						}
					}
				}
				// Add required fields from request body
				params.Required = append(params.Required, content.Schema.Value.Required...)
			}
		}
	}

	description := operation.Summary
	if description == "" {
		description = operation.Description
	}
	if description == "" {
		description = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
	}

	return &tool.DynamicTool{
		ID:          name,
		Name:        name,
		Description: description,
		Method:      method,
		Path:        path,
		OperationID: operation.OperationID,
		Parameters:  params,
	}
}

// getSchemaType extracts the type from an OpenAPI schema
func getSchemaType(schemaRef *openapi3.SchemaRef) string {
	if schemaRef == nil || schemaRef.Value == nil {
		return "string"
	}

	schema := schemaRef.Value
	if schema.Type != nil && len(*schema.Type) > 0 {
		// Type is a slice, return the first type
		return (*schema.Type)[0]
	}

	// Default to string if type is not specified
	return "string"
}
