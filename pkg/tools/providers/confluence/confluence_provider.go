package confluence

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
)

// Embed Confluence OpenAPI spec as fallback
// This will be populated from Atlassian's official spec
//
//go:embed confluence-openapi.json
var confluenceOpenAPISpecJSON []byte

// ConfluenceProvider implements the StandardToolProvider interface for Confluence Cloud
type ConfluenceProvider struct {
	*providers.BaseProvider
	specCache    repository.OpenAPICacheRepository // For caching the OpenAPI spec
	specFallback *openapi3.T                       // Embedded fallback spec
	httpClient   *http.Client
	domain       string // e.g., "your-domain" for https://your-domain.atlassian.net
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
		domain: domain,
	}
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
		BaseURL:        fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api", p.domain),
		AuthType:       "basic", // Confluence uses basic auth with API tokens
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

// ValidateCredentials validates Confluence credentials
func (p *ConfluenceProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	email, hasEmail := creds["email"]
	apiToken, hasAPIToken := creds["api_token"]
	username, hasUsername := creds["username"]
	password, hasPassword := creds["password"]

	// Check for Confluence Cloud API token auth (preferred)
	if hasEmail && hasAPIToken {
		// Test with email and API token
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
			return fmt.Errorf("invalid Confluence credentials (email/api_token)")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected response from Confluence API: %d", resp.StatusCode)
		}
		return nil
	}

	// Check for basic auth (legacy, not recommended for Cloud)
	if hasUsername && hasPassword {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api/space", p.domain), nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(username, password)
		req.Header.Set("Accept", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to validate credentials: %w", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("invalid Confluence credentials (username/password)")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected response from Confluence API: %d", resp.StatusCode)
		}
		return nil
	}

	return fmt.Errorf("missing required credentials: email/api_token or username/password")
}

// ExecuteOperation executes a Confluence operation
func (p *ConfluenceProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize operation name (handle different formats)
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	// Use base provider's execution with Confluence-specific handling
	return p.Execute(ctx, operation, params)
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
