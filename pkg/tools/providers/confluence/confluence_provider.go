package confluence

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
)

// contextKey is a type for context keys to avoid string literals
type contextKey string

const (
	// confluenceTokenKey is the context key for Confluence authentication token
	confluenceTokenKey contextKey = "confluence_token"
)

// Embed Confluence OpenAPI spec as fallback
// This will be populated from Atlassian's official spec
//
//go:embed confluence-openapi.json
var confluenceOpenAPISpecJSON []byte

// ToolHandler interface for Confluence tool operations
type ToolHandler interface {
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
	GetDefinition() ToolDefinition
}

// ToolDefinition describes a tool's schema
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Toolset represents a group of related tools
type Toolset struct {
	Name        string
	Description string
	Tools       []ToolHandler
	Enabled     bool
}

// ConfluenceProvider implements the StandardToolProvider interface for Confluence Cloud
// API Version Strategy:
//   - v2 API: Used for pages, labels, and modern operations (cursor-based pagination)
//   - v1 API: Used for CQL search and legacy operations (offset/limit pagination)
//
// The provider automatically selects the appropriate API version for each operation
type ConfluenceProvider struct {
	*providers.BaseProvider
	specCache     repository.OpenAPICacheRepository // For caching the OpenAPI spec
	specFallback  *openapi3.T                       // Embedded fallback spec
	httpClient    *http.Client
	domain        string // e.g., "your-domain" for https://your-domain.atlassian.net
	encryptionSvc *security.EncryptionService

	// Handler registry
	toolRegistry    map[string]ToolHandler
	toolsetRegistry map[string]*Toolset
	enabledToolsets map[string]bool
	mutex           sync.RWMutex
}

// Helper functions for creating results
func NewToolResult(data interface{}) *ToolResult {
	return &ToolResult{
		Success: true,
		Data:    data,
	}
}

func NewToolError(err string) *ToolResult {
	return &ToolResult{
		Success: false,
		Error:   err,
	}
}

// NewConfluenceProvider creates a new Confluence provider instance
func NewConfluenceProvider(logger observability.Logger, domain string) *ConfluenceProvider {
	baseURL := fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api", domain)
	base := providers.NewBaseProvider("confluence", "v2", baseURL, logger)

	// Load embedded spec as fallback
	var specFallback *openapi3.T
	if len(confluenceOpenAPISpecJSON) > 0 {
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(confluenceOpenAPISpecJSON)
		if err == nil {
			specFallback = spec
		}
	}

	provider := &ConfluenceProvider{
		BaseProvider: base,
		specFallback: specFallback,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		domain:          domain,
		encryptionSvc:   security.NewEncryptionService(""),
		toolRegistry:    make(map[string]ToolHandler),
		toolsetRegistry: make(map[string]*Toolset),
		enabledToolsets: make(map[string]bool),
	}

	// Register handlers
	provider.registerHandlers()
	// Enable default toolsets
	provider.enableDefaultToolsets()

	// Set operation mappings in base provider
	provider.SetOperationMappings(provider.GetOperationMappings())
	// Set configuration to ensure auth type is configured
	provider.SetConfiguration(provider.GetDefaultConfiguration())
	return provider
}

// NewConfluenceProviderWithCache creates a new Confluence provider with spec caching
func NewConfluenceProviderWithCache(logger observability.Logger, domain string, specCache repository.OpenAPICacheRepository) *ConfluenceProvider {
	provider := NewConfluenceProvider(logger, domain)
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *ConfluenceProvider) GetProviderName() string {
	return "confluence"
}

// GetSupportedVersions returns supported Confluence API versions
func (p *ConfluenceProvider) GetSupportedVersions() []string {
	return []string{"v2", "v1"}
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *ConfluenceProvider) GetEmbeddedSpecVersion() string {
	return "2024.01" // Update this when updating the embedded spec
}

// GetDefaultConfiguration returns the default configuration for Confluence
func (p *ConfluenceProvider) GetDefaultConfiguration() providers.ProviderConfig {
	return providers.ProviderConfig{
		BaseURL:        fmt.Sprintf("https://%s.atlassian.net/wiki/api/v2", p.domain), // Using v2 API as default
		AuthType:       "basic",                                                       // Confluence uses basic auth with API tokens
		RequiredScopes: []string{"read:confluence-content.all", "write:confluence-content.all"},
		RateLimits: providers.RateLimitConfig{
			RequestsPerHour:    5000,
			RequestsPerMinute:  100,
			ConcurrentRequests: 10,
		},
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "content",
				DisplayName: "Content Management",
				Description: "Create, read, update, and delete Confluence pages and blog posts",
				Operations:  []string{"content/create", "content/get", "content/update", "content/delete", "content/search"},
			},
			{
				Name:        "space",
				DisplayName: "Space Management",
				Description: "Manage Confluence spaces",
				Operations:  []string{"space/list", "space/get", "space/create", "space/update"},
			},
			{
				Name:        "attachment",
				DisplayName: "Attachment Management",
				Description: "Manage attachments on Confluence pages",
				Operations:  []string{"attachment/create", "attachment/get", "attachment/update", "attachment/delete"},
			},
			{
				Name:        "comment",
				DisplayName: "Comment Management",
				Description: "Manage comments on Confluence content",
				Operations:  []string{"comment/create", "comment/get", "comment/update", "comment/delete"},
			},
			{
				Name:        "label",
				DisplayName: "Label Management",
				Description: "Manage labels on Confluence content",
				Operations:  []string{"label/add", "label/remove", "label/get"},
			},
		},
		DefaultHeaders: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnTimeout:   true,
			RetryOnRateLimit: true,
		},
		HealthEndpoint: fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api/space", p.domain),
	}
}

// GetToolDefinitions returns Confluence-specific tool definitions
func (p *ConfluenceProvider) GetToolDefinitions() []providers.ToolDefinition {
	return []providers.ToolDefinition{
		{
			Name:        "confluence_content",
			DisplayName: "Confluence Content",
			Description: "Manage Confluence pages, blog posts, and other content",
			Category:    "documentation",
			Operation: providers.OperationDef{
				ID:           "content",
				Method:       "GET",
				PathTemplate: "/content/{id}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: false, Description: "Content ID"},
				{Name: "spaceKey", In: "query", Type: "string", Required: false, Description: "Space key"},
				{Name: "title", In: "query", Type: "string", Required: false, Description: "Content title"},
				{Name: "type", In: "query", Type: "string", Required: false, Description: "Content type (page, blogpost)", Default: "page"},
			},
		},
		{
			Name:        "confluence_space",
			DisplayName: "Confluence Spaces",
			Description: "Manage Confluence spaces",
			Category:    "documentation",
			Operation: providers.OperationDef{
				ID:           "space",
				Method:       "GET",
				PathTemplate: "/space",
			},
			Parameters: []providers.ParameterDef{
				{Name: "spaceKey", In: "query", Type: "string", Required: false, Description: "Space key"},
				{Name: "type", In: "query", Type: "string", Required: false, Description: "Space type", Default: "global"},
			},
		},
		{
			Name:        "confluence_search",
			DisplayName: "Confluence Search",
			Description: "Search Confluence content using CQL (Confluence Query Language)",
			Category:    "search",
			Operation: providers.OperationDef{
				ID:           "search",
				Method:       "GET",
				PathTemplate: "/search",
			},
			Parameters: []providers.ParameterDef{
				{Name: "cql", In: "query", Type: "string", Required: true, Description: "Confluence Query Language string"},
				{Name: "limit", In: "query", Type: "integer", Required: false, Description: "Maximum number of results", Default: 25},
			},
		},
		{
			Name:        "confluence_attachment",
			DisplayName: "Confluence Attachments",
			Description: "Manage attachments on Confluence pages",
			Category:    "documentation",
			Operation: providers.OperationDef{
				ID:           "attachment",
				Method:       "GET",
				PathTemplate: "/content/{id}/child/attachment",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Content ID"},
			},
		},
		{
			Name:        "confluence_comment",
			DisplayName: "Confluence Comments",
			Description: "Manage comments on Confluence content",
			Category:    "collaboration",
			Operation: providers.OperationDef{
				ID:           "comment",
				Method:       "GET",
				PathTemplate: "/content/{id}/child/comment",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Content ID"},
			},
		},
		{
			Name:        "confluence_user",
			DisplayName: "Confluence Users",
			Description: "Manage Confluence users and permissions",
			Category:    "identity",
			Operation: providers.OperationDef{
				ID:           "user",
				Method:       "GET",
				PathTemplate: "/user",
			},
			Parameters: []providers.ParameterDef{
				{Name: "accountId", In: "query", Type: "string", Required: false, Description: "User account ID"},
				{Name: "username", In: "query", Type: "string", Required: false, Description: "Username (deprecated, use accountId)"},
			},
		},
	}
}

// ValidateCredentials validates Confluence credentials using passthrough authentication
// IMPORTANT: Following passthrough auth pattern - credentials are NOT stored, only validated
func (p *ConfluenceProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// Build params with credentials for passthrough auth validation
	params := make(map[string]interface{})

	// Convert creds map to format expected by extractAuthToken
	// Support multiple formats for backward compatibility
	if email, hasEmail := creds["email"]; hasEmail {
		if apiToken, hasAPIToken := creds["api_token"]; hasAPIToken {
			// Build combined token for passthrough
			params["token"] = email + ":" + apiToken
		}
	} else if username, hasUsername := creds["username"]; hasUsername {
		if password, hasPassword := creds["password"]; hasPassword {
			// Legacy format - build combined token
			params["token"] = username + ":" + password
		}
	} else if token, hasToken := creds["token"]; hasToken {
		params["token"] = token
	}

	// Extract authentication using passthrough pattern
	email, apiToken, err := p.extractAuthToken(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to extract credentials for validation: %w", err)
	}

	// Test the credentials with the space endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api/space", p.domain), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid Confluence credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response from Confluence API: %d", resp.StatusCode)
	}

	return nil
}

// ExecuteOperation executes a Confluence operation
func (p *ConfluenceProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Apply configuration from context
	p.ConfigureFromContext(ctx)

	// Normalize operation name (handle different formats)
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	// Check for read-only mode
	if p.IsReadOnlyMode(ctx) && p.IsWriteOperation(operation) {
		return nil, fmt.Errorf("operation %s not allowed in read-only mode", operation)
	}

	// Use base provider's execution with Confluence-specific handling
	result, err := p.Execute(ctx, operation, params)
	if err != nil {
		return nil, err
	}

	// Apply space filtering to results
	result = p.FilterSpaceResults(ctx, result)

	return result, nil
}

// normalizeOperationName normalizes operation names to handle different formats
func (p *ConfluenceProvider) normalizeOperationName(operation string) string {
	// Handle different separators to normalize format
	normalized := strings.ReplaceAll(operation, "-", "/")
	normalized = strings.ReplaceAll(normalized, "_", "/")

	// If it already has a resource prefix (e.g., "content/create"), return it
	if strings.Contains(normalized, "/") {
		return normalized
	}

	// Simple action defaults for backward compatibility
	simpleActions := map[string]string{
		"list":   "content/list",
		"get":    "content/get",
		"create": "content/create",
		"update": "content/update",
		"delete": "content/delete",
		"search": "content/search",
	}

	if defaultOp, ok := simpleActions[normalized]; ok {
		return defaultOp
	}

	return normalized
}

// GetOpenAPISpec returns the OpenAPI specification for Confluence
func (p *ConfluenceProvider) GetOpenAPISpec() (*openapi3.T, error) {
	ctx := context.Background()

	// Try cache first if available
	if p.specCache != nil {
		spec, err := p.specCache.Get(ctx, "confluence-v2")
		if err == nil && spec != nil {
			return spec, nil
		}
	}

	// Try fetching from Atlassian with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	spec, err := p.fetchAndCacheSpec(fetchCtx)
	if err != nil {
		// Use embedded fallback if available
		if p.specFallback != nil {
			p.BaseProvider.GetLogger().Warn("Using embedded OpenAPI spec fallback", map[string]interface{}{
				"error": err.Error(),
			})
			return p.specFallback, nil
		}
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	return spec, nil
}

// HealthCheck verifies the Confluence API is accessible
func (p *ConfluenceProvider) HealthCheck(ctx context.Context) error {
	// Use the /wiki/rest/api/space endpoint for health check
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api/space", p.domain), nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("confluence API health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// For Confluence, 401 means auth is required but API is accessible
	// 200 or 401 both indicate the API is reachable
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("confluence API returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *ConfluenceProvider) Close() error {
	// Currently no resources to clean up
	// If we add connection pools or other resources, clean them up here
	return nil
}

// fetchAndCacheSpec fetches the OpenAPI spec from Atlassian and caches it
func (p *ConfluenceProvider) fetchAndCacheSpec(ctx context.Context) (*openapi3.T, error) {
	// Atlassian provides OpenAPI specs at developer.atlassian.com
	// This is a known URL for the Confluence Cloud REST API v2 spec
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://developer.atlassian.com/cloud/confluence/openapi/v2/openapi.json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Cache the spec if cache is available
	if p.specCache != nil {
		_ = p.specCache.Set(ctx, "confluence-v2", spec, 24*time.Hour) // Cache for 24 hours
	}

	return spec, nil
}

// registerHandlers registers all available handlers
func (p *ConfluenceProvider) registerHandlers() {
	// Page handlers
	pageHandlers := []ToolHandler{
		NewGetPageHandler(p),
		NewListPagesHandler(p),
		NewDeletePageHandler(p),
	}

	pageToolset := &Toolset{
		Name:        "pages",
		Description: "Confluence page operations",
		Tools:       pageHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["pages"] = pageToolset

	// Register page handlers
	for _, handler := range pageHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}

	// Search handlers
	searchHandlers := []ToolHandler{
		NewSearchContentHandler(p),
	}

	searchToolset := &Toolset{
		Name:        "search",
		Description: "Confluence search operations",
		Tools:       searchHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["search"] = searchToolset

	// Register search handlers
	for _, handler := range searchHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}

	// Label handlers
	labelHandlers := []ToolHandler{
		NewGetPageLabelsHandler(p),
		NewAddLabelHandler(p),
		NewRemoveLabelHandler(p),
	}

	labelToolset := &Toolset{
		Name:        "labels",
		Description: "Confluence label operations",
		Tools:       labelHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["labels"] = labelToolset

	// Register label handlers
	for _, handler := range labelHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}

	// Content handlers (v1 API operations)
	contentHandlers := []ToolHandler{
		NewCreatePageHandler(p),
		NewUpdatePageHandler(p),
		NewGetAttachmentsHandler(p),
	}

	contentToolset := &Toolset{
		Name:        "content",
		Description: "Confluence content operations (v1 API)",
		Tools:       contentHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["content"] = contentToolset

	// Register content handlers
	for _, handler := range contentHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}

	// Space handlers (v1 API operations)
	spaceHandlers := []ToolHandler{
		NewListSpacesHandler(p),
	}

	spaceToolset := &Toolset{
		Name:        "spaces",
		Description: "Confluence space operations (v1 API)",
		Tools:       spaceHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["spaces"] = spaceToolset

	// Register space handlers
	for _, handler := range spaceHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}
}

// enableDefaultToolsets enables the default toolsets for Confluence
func (p *ConfluenceProvider) enableDefaultToolsets() {
	// Enable common toolsets by default
	// Note: content and spaces use v1 API for operations not available in v2
	defaultToolsets := []string{"pages", "search", "labels", "content", "spaces"}
	for _, name := range defaultToolsets {
		if err := p.EnableToolset(name); err != nil {
			// Log error but continue with other toolsets
			p.BaseProvider.GetLogger().Warn("Failed to enable default toolset", map[string]interface{}{
				"toolset": name,
				"error":   err.Error(),
			})
		}
	}
}

// EnableToolset enables a specific toolset
func (p *ConfluenceProvider) EnableToolset(name string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	toolset, exists := p.toolsetRegistry[name]
	if !exists {
		return fmt.Errorf("toolset %s not found", name)
	}

	toolset.Enabled = true
	p.enabledToolsets[name] = true

	p.BaseProvider.GetLogger().Info("Enabled toolset", map[string]interface{}{
		"toolset": name,
	})

	return nil
}

// DisableToolset disables a specific toolset
func (p *ConfluenceProvider) DisableToolset(name string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	toolset, exists := p.toolsetRegistry[name]
	if !exists {
		return fmt.Errorf("toolset %s not found", name)
	}

	toolset.Enabled = false
	delete(p.enabledToolsets, name)

	p.BaseProvider.GetLogger().Info("Disabled toolset", map[string]interface{}{
		"toolset": name,
	})

	return nil
}

// IsToolsetEnabled checks if a toolset is enabled
func (p *ConfluenceProvider) IsToolsetEnabled(name string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.enabledToolsets[name]
}

// GetEnabledToolsets returns a list of enabled toolsets
func (p *ConfluenceProvider) GetEnabledToolsets() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	toolsets := make([]string, 0, len(p.enabledToolsets))
	for name, enabled := range p.enabledToolsets {
		if enabled {
			toolsets = append(toolsets, name)
		}
	}
	return toolsets
}

// ConfigureFromContext applies configuration from provider context
func (p *ConfluenceProvider) ConfigureFromContext(ctx context.Context) {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return
	}

	// Check for enabled tools configuration
	if enabledTools, ok := pctx.Metadata["ENABLED_TOOLS"].(string); ok && enabledTools != "" {
		// Disable all toolsets first
		for name := range p.toolsetRegistry {
			_ = p.DisableToolset(name) // Ignore errors when disabling during initialization
		}

		// Enable only specified toolsets
		tools := strings.Split(enabledTools, ",")
		for _, tool := range tools {
			tool = strings.TrimSpace(tool)
			if tool != "" {
				if err := p.EnableToolset(tool); err != nil {
					p.BaseProvider.GetLogger().Warn("Failed to enable toolset", map[string]interface{}{
						"toolset": tool,
						"error":   err.Error(),
					})
				}
			}
		}
	}

	// Log configuration applied
	if spaceFilter, ok := pctx.Metadata["CONFLUENCE_SPACES_FILTER"].(string); ok && spaceFilter != "" {
		p.BaseProvider.GetLogger().Info("Applied space filter", map[string]interface{}{
			"filter": spaceFilter,
		})
	}

	if readOnly, ok := pctx.Metadata["READ_ONLY"].(bool); ok && readOnly {
		p.BaseProvider.GetLogger().Info("Read-only mode enabled", nil)
	}
}

// IsReadOnlyMode checks if the provider is in read-only mode
func (p *ConfluenceProvider) IsReadOnlyMode(ctx context.Context) bool {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return false
	}

	readOnly, ok := pctx.Metadata["READ_ONLY"].(bool)
	return ok && readOnly
}

// IsWriteOperation checks if an operation is a write operation
func (p *ConfluenceProvider) IsWriteOperation(operation string) bool {
	writeOperations := []string{
		"create", "update", "delete", "add", "remove",
		"edit", "post", "submit", "move", "publish",
		"upload", "attach", "restore", "archive",
	}

	operation = strings.ToLower(operation)
	for _, writeOp := range writeOperations {
		if strings.Contains(operation, writeOp) {
			return true
		}
	}

	return false
}

// FilterSpaceResults filters results based on CONFLUENCE_SPACES_FILTER
func (p *ConfluenceProvider) FilterSpaceResults(ctx context.Context, results interface{}) interface{} {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return results
	}

	spaceFilter, ok := pctx.Metadata["CONFLUENCE_SPACES_FILTER"].(string)
	if !ok || spaceFilter == "" || spaceFilter == "*" {
		return results
	}

	// Apply filtering logic based on result type
	switch r := results.(type) {
	case map[string]interface{}:
		// Check if this is a list of pages
		if results, ok := r["results"].([]interface{}); ok {
			filtered := p.filterPagesBySpace(results, spaceFilter)
			r["results"] = filtered
			r["size"] = len(filtered)
		}
		// Check if this is a list of spaces
		if spaces, ok := r["spaces"].([]interface{}); ok {
			filtered := p.filterSpaces(spaces, spaceFilter)
			r["spaces"] = filtered
			r["size"] = len(filtered)
		}
	case []interface{}:
		// Direct array of items
		return p.filterItemsBySpace(r, spaceFilter)
	}

	return results
}

// Helper function to filter pages by space
func (p *ConfluenceProvider) filterPagesBySpace(pages []interface{}, filter string) []interface{} {
	filtered := []interface{}{}
	allowedSpaces := strings.Split(filter, ",")

	for _, page := range pages {
		if pageMap, ok := page.(map[string]interface{}); ok {
			if space, ok := pageMap["space"].(map[string]interface{}); ok {
				if key, ok := space["key"].(string); ok {
					if p.isSpaceAllowed(key, allowedSpaces) {
						filtered = append(filtered, page)
					}
				}
			}
		}
	}

	return filtered
}

// Helper function to filter spaces
func (p *ConfluenceProvider) filterSpaces(spaces []interface{}, filter string) []interface{} {
	filtered := []interface{}{}
	allowedSpaces := strings.Split(filter, ",")

	for _, space := range spaces {
		if spaceMap, ok := space.(map[string]interface{}); ok {
			if key, ok := spaceMap["key"].(string); ok {
				if p.isSpaceAllowed(key, allowedSpaces) {
					filtered = append(filtered, space)
				}
			}
		}
	}

	return filtered
}

// Helper function to filter generic items by space
func (p *ConfluenceProvider) filterItemsBySpace(items []interface{}, filter string) []interface{} {
	filtered := []interface{}{}
	allowedSpaces := strings.Split(filter, ",")

	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Try different space key locations
			var spaceKey string
			if key, ok := itemMap["spaceKey"].(string); ok {
				spaceKey = key
			} else if space, ok := itemMap["space"].(map[string]interface{}); ok {
				if key, ok := space["key"].(string); ok {
					spaceKey = key
				}
			}

			if spaceKey != "" && p.isSpaceAllowed(spaceKey, allowedSpaces) {
				filtered = append(filtered, item)
			}
		}
	}

	return filtered
}

// Helper function to check if a space is allowed
func (p *ConfluenceProvider) isSpaceAllowed(spaceKey string, allowedSpaces []string) bool {
	for _, allowed := range allowedSpaces {
		allowed = strings.TrimSpace(allowed)
		if allowed == spaceKey || allowed == "*" {
			return true
		}
	}
	return false
}

// extractAuthToken extracts authentication token from context or parameters
// Following GitHub provider pattern exactly for passthrough authentication
func (p *ConfluenceProvider) extractAuthToken(ctx context.Context, params map[string]interface{}) (string, string, error) {
	// Priority 1: Try from ProviderContext first (standard provider pattern)
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil {
		// Check for token in main Credentials field
		if pctx.Credentials != nil && pctx.Credentials.Token != "" {
			// For Confluence, token might be in email:api_token format
			parts := strings.SplitN(pctx.Credentials.Token, ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
			// Return error if token format is invalid
			if !strings.Contains(pctx.Credentials.Token, ":") {
				return "", "", fmt.Errorf("invalid token format, expected email:api_token or username:password")
			}
		}

		// Check for username/password (legacy support)
		if pctx.Credentials != nil && pctx.Credentials.Username != "" && pctx.Credentials.Password != "" {
			return pctx.Credentials.Username, pctx.Credentials.Password, nil
		}

		// Check Metadata (newer pattern)
		if pctx.Metadata != nil {
			// Check for token in metadata
			if token, ok := pctx.Metadata["token"].(string); ok && token != "" {
				parts := strings.SplitN(token, ":", 2)
				if len(parts) == 2 {
					return parts[0], parts[1], nil
				}
			}
			// Check for separate email and api_token in metadata
			if email, ok := pctx.Metadata["email"].(string); ok && email != "" {
				if apiToken, ok := pctx.Metadata["api_token"].(string); ok && apiToken != "" {
					return email, apiToken, nil
				}
			}
			// Check for username/password in metadata
			if username, ok := pctx.Metadata["username"].(string); ok && username != "" {
				if password, ok := pctx.Metadata["password"].(string); ok && password != "" {
					return username, password, nil
				}
			}
		}

		// Also check custom credentials map (legacy)
		if pctx.Credentials != nil && pctx.Credentials.Custom != nil {
			if token, ok := pctx.Credentials.Custom["token"]; ok && token != "" {
				parts := strings.SplitN(token, ":", 2)
				if len(parts) == 2 {
					return parts[0], parts[1], nil
				}
			}
			// Check for Confluence-specific keys
			if token, ok := pctx.Credentials.Custom["confluence_token"]; ok && token != "" {
				parts := strings.SplitN(token, ":", 2)
				if len(parts) == 2 {
					return parts[0], parts[1], nil
				}
			}
			// Check for separate email and api_token in custom
			if email, ok := pctx.Credentials.Custom["email"]; ok && email != "" {
				if apiToken, ok := pctx.Credentials.Custom["api_token"]; ok && apiToken != "" {
					return email, apiToken, nil
				}
			}
		}
	}

	// Priority 2: Try passthrough auth from params
	if auth, ok := params["__passthrough_auth"].(map[string]interface{}); ok {
		// Check for encrypted token
		if encryptedToken, ok := auth["encrypted_token"].(string); ok && encryptedToken != "" {
			// Extract tenant ID for decryption
			tenantID := p.extractTenantID(ctx, params)
			decrypted, err := p.encryptionSvc.DecryptCredential([]byte(encryptedToken), tenantID)
			if err != nil {
				return "", "", fmt.Errorf("failed to decrypt token: %w", err)
			}
			// Parse decrypted token (email:api_token format)
			parts := strings.SplitN(decrypted, ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
		}

		// Check for plain token (development mode)
		if token, ok := auth["token"].(string); ok && token != "" {
			parts := strings.SplitN(token, ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
		}

		// Try separate email and api_token in passthrough
		if email, ok := auth["email"].(string); ok && email != "" {
			if apiToken, ok := auth["api_token"].(string); ok && apiToken != "" {
				return email, apiToken, nil
			}
		}

		// Try username/password in passthrough
		if username, ok := auth["username"].(string); ok && username != "" {
			if password, ok := auth["password"].(string); ok && password != "" {
				return username, password, nil
			}
		}
	}

	// Priority 3: Try direct token from params (backward compatibility)
	if token, ok := params["token"].(string); ok && token != "" {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
		// Return specific error for invalid format
		if !strings.Contains(token, ":") {
			return "", "", fmt.Errorf("invalid token format, expected email:api_token or username:password")
		}
	}

	// Priority 4: Try direct email/api_token or username/password from params (backward compatibility)
	if email, ok := params["email"].(string); ok && email != "" {
		if apiToken, ok := params["api_token"].(string); ok && apiToken != "" {
			return email, apiToken, nil
		}
		// Return specific error if email exists but token is missing
		return "", "", fmt.Errorf("email provided but api_token missing")
	}
	if username, ok := params["username"].(string); ok && username != "" {
		if password, ok := params["password"].(string); ok && password != "" {
			return username, password, nil
		}
		// Return specific error if username exists but password is missing
		return "", "", fmt.Errorf("username provided but password missing")
	}

	// Priority 5: Try from context value
	if token, ok := ctx.Value(confluenceTokenKey).(string); ok {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("no authentication credentials found")
}

// extractTenantID extracts tenant ID from context or params
func (p *ConfluenceProvider) extractTenantID(ctx context.Context, params map[string]interface{}) string {
	// Try from ProviderContext
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.TenantID != "" {
		return pctx.TenantID
	}
	// Try from params
	if tenantID, ok := params["tenant_id"].(string); ok && tenantID != "" {
		return tenantID
	}
	// Try from passthrough auth
	if auth, ok := params["__passthrough_auth"].(map[string]interface{}); ok {
		if tenantID, ok := auth["tenant_id"].(string); ok && tenantID != "" {
			return tenantID
		}
	}
	// Try from context value
	if tenantID, ok := ctx.Value("tenant_id").(string); ok && tenantID != "" {
		return tenantID
	}
	return ""
}

// buildURL builds a Confluence v2 API URL
// Use this for modern operations that are available in v2 API
func (p *ConfluenceProvider) buildURL(path string) string {
	// Check if domain is a test URL (starts with http)
	if strings.HasPrefix(p.domain, "http") {
		// For testing, domain contains the full base URL
		return p.domain + path
	}
	// Use v2 API endpoint (modern API with cursor-based pagination)
	return fmt.Sprintf("https://%s.atlassian.net/wiki/api/v2%s", p.domain, path)
}

// buildV1URL builds a Confluence v1 API URL
// Use this for operations that are not yet available in v2 (e.g., CQL search)
func (p *ConfluenceProvider) buildV1URL(path string) string {
	// Check if domain is a test URL (starts with http)
	if strings.HasPrefix(p.domain, "http") {
		// For testing, domain contains the full base URL
		// Remove /wiki/api/v2 if present and add /wiki/rest/api
		baseURL := strings.TrimSuffix(p.domain, "/wiki/api/v2")
		baseURL = strings.TrimSuffix(baseURL, "/")
		return baseURL + "/wiki/rest/api" + path
	}
	// Use v1 API endpoint (required for CQL support and some legacy operations)
	return fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api%s", p.domain, path)
}
