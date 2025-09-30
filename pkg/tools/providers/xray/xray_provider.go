package xray

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
)

// XrayProvider implements the StandardToolProvider interface for JFrog Xray
type XrayProvider struct {
	*providers.BaseProvider
	specCache            repository.OpenAPICacheRepository // For caching the OpenAPI spec
	httpClient           *http.Client
	permissionDiscoverer *XrayPermissionDiscoverer             // Permission discovery integration
	filteredOperations   map[string]providers.OperationMapping // Filtered operations based on permissions
	allOperations        map[string]providers.OperationMapping // Cache of all operations
}

// NewXrayProvider creates a new Xray provider instance
func NewXrayProvider(logger observability.Logger) *XrayProvider {
	// Defensive nil check for logger
	if logger == nil {
		// Create a no-op logger if none provided
		logger = &observability.NoopLogger{}
	}

	// Default to cloud instance, can be overridden via configuration
	base := providers.NewBaseProvider("xray", "v1", "https://mycompany.jfrog.io/xray", logger)

	provider := &XrayProvider{
		BaseProvider: base,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for scan operations
		},
		permissionDiscoverer: NewXrayPermissionDiscoverer(logger, base.GetDefaultConfiguration().BaseURL),
		allOperations:        nil, // Will be initialized when first accessed
	}

	// Store all operations for later filtering
	provider.allOperations = provider.getAllOperationMappings()

	// Set operation mappings in base provider (initially all operations)
	provider.SetOperationMappings(provider.allOperations)

	// Set configuration to ensure auth type is configured
	provider.SetConfiguration(provider.GetDefaultConfiguration())

	return provider
}

// NewXrayProviderWithCache creates a new Xray provider with spec caching
func NewXrayProviderWithCache(logger observability.Logger, specCache repository.OpenAPICacheRepository) *XrayProvider {
	// NewXrayProvider handles nil logger check
	provider := NewXrayProvider(logger)
	// Allow nil specCache - it's optional
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *XrayProvider) GetProviderName() string {
	return "xray"
}

// GetSupportedVersions returns supported Xray API versions
func (p *XrayProvider) GetSupportedVersions() []string {
	return []string{"v1", "v2"}
}

// GetToolDefinitions returns Xray-specific tool definitions
func (p *XrayProvider) GetToolDefinitions() []providers.ToolDefinition {
	// Defensive nil check
	if p == nil {
		return nil
	}

	return []providers.ToolDefinition{
		{
			Name:        "xray_scans",
			DisplayName: "Xray Security Scans",
			Description: "Perform security vulnerability scanning",
			Category:    "security",
		},
		{
			Name:        "xray_violations",
			DisplayName: "Xray Policy Violations",
			Description: "Manage security and license violations",
			Category:    "compliance",
		},
		{
			Name:        "xray_watches",
			DisplayName: "Xray Watches",
			Description: "Manage Xray watches for continuous monitoring",
			Category:    "monitoring",
		},
		{
			Name:        "xray_policies",
			DisplayName: "Xray Security Policies",
			Description: "Define and manage security policies",
			Category:    "policy_management",
		},
		{
			Name:        "xray_components",
			DisplayName: "Xray Component Intelligence",
			Description: "Get vulnerability information for components",
			Category:    "vulnerability_intelligence",
		},
		{
			Name:        "xray_reports",
			DisplayName: "Xray Reports",
			Description: "Generate security and compliance reports",
			Category:    "reporting",
		},
	}
}

// ValidateCredentials validates the provided credentials for Xray
func (p *XrayProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// Use the system/ping endpoint to validate credentials
	// Use the configured base URL, not the default
	baseURL := p.BaseProvider.GetDefaultConfiguration().BaseURL
	if baseURL == "" {
		baseURL = p.GetDefaultConfiguration().BaseURL
	}
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/system/ping", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Apply authentication
	if apiKey, ok := creds["api_key"]; ok {
		// Detect auth type and apply appropriate header
		if p.isJFrogAPIKey(apiKey) {
			req.Header.Set("X-JFrog-Art-Api", apiKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	} else {
		return fmt.Errorf("api_key is required for Xray authentication")
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid credentials: authentication failed")
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to validate credentials: received status %d", resp.StatusCode)
	}

	return nil
}

// ExecuteOperation executes a specific operation with the given parameters
func (p *XrayProvider) ExecuteOperation(ctx context.Context, op string, params map[string]interface{}) (interface{}, error) {
	// Check if we have filtered operations and if the operation is allowed
	operations := p.GetOperationMappings()
	if _, ok := operations[op]; !ok {
		return nil, fmt.Errorf("operation '%s' not available (may require additional permissions or features)", op)
	}

	// Use BaseProvider's Execute which handles the actual API call
	return p.Execute(ctx, op, params)
}

// GetOperationMappings returns the operation ID to API endpoint mappings
// This method is overridden to return filtered operations if available
func (p *XrayProvider) GetOperationMappings() map[string]providers.OperationMapping {
	// Return filtered operations if available
	if p.filteredOperations != nil {
		return p.filteredOperations
	}
	// Otherwise return all operations (backward compatibility)
	return p.allOperations
}

// getAllOperationMappings returns all Xray operation mappings
func (p *XrayProvider) getAllOperationMappings() map[string]providers.OperationMapping {
	operations := map[string]providers.OperationMapping{
		// System operations
		"system/ping": {
			OperationID:  "SystemPing",
			Method:       "GET",
			PathTemplate: "/api/v1/system/ping",
		},
		"system/version": {
			OperationID:  "SystemVersion",
			Method:       "GET",
			PathTemplate: "/api/v1/system/version",
		},

		// Scan operations
		"scan/artifact": {
			OperationID:    "ScanArtifact",
			Method:         "POST",
			PathTemplate:   "/api/v1/scan/artifact",
			RequiredParams: []string{"componentId"},
		},
		"scan/build": {
			OperationID:    "ScanBuild",
			Method:         "POST",
			PathTemplate:   "/api/v1/scan/build",
			RequiredParams: []string{"buildName", "buildNumber"},
		},
		"scan/status": {
			OperationID:    "GetScanStatus",
			Method:         "GET",
			PathTemplate:   "/api/v1/scan/status/{scan_id}",
			RequiredParams: []string{"scan_id"},
		},

		// Summary operations
		"summary/artifact": {
			OperationID:    "GetArtifactSummary",
			Method:         "POST",
			PathTemplate:   "/api/v1/summary/artifact",
			RequiredParams: []string{"paths"},
		},
		"summary/build": {
			OperationID:    "GetBuildSummary",
			Method:         "GET",
			PathTemplate:   "/api/v1/summary/build",
			RequiredParams: []string{"build_name", "build_number"},
		},

		// Component operations
		"components/details": {
			OperationID:    "GetComponentDetails",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/details",
			RequiredParams: []string{"component_id"},
		},
		"components/search": {
			OperationID:    "SearchComponents",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/search",
			RequiredParams: []string{"query"},
		},

		// Violations operations
		"violations/list": {
			OperationID:  "ListViolations",
			Method:       "GET",
			PathTemplate: "/api/v2/violations",
			OptionalParams: []string{
				"type",        // security, license, operational_risk
				"severity",    // critical, major, minor
				"created",     // date range
				"watch_name",  // filter by watch
				"policy_name", // filter by policy
				"limit",       // pagination
				"offset",      // pagination
			},
		},
		"violations/artifact": {
			OperationID:    "GetArtifactViolations",
			Method:         "POST",
			PathTemplate:   "/api/v2/violations/artifact",
			RequiredParams: []string{"repo_key", "path"},
		},

		// Watch operations
		"watches/list": {
			OperationID:  "ListWatches",
			Method:       "GET",
			PathTemplate: "/api/v2/watches",
		},
		"watches/get": {
			OperationID:    "GetWatch",
			Method:         "GET",
			PathTemplate:   "/api/v2/watches/{name}",
			RequiredParams: []string{"name"},
		},
		"watches/create": {
			OperationID:    "CreateWatch",
			Method:         "POST",
			PathTemplate:   "/api/v2/watches",
			RequiredParams: []string{"name", "description"},
		},
		"watches/update": {
			OperationID:    "UpdateWatch",
			Method:         "PUT",
			PathTemplate:   "/api/v2/watches/{name}",
			RequiredParams: []string{"name"},
		},
		"watches/delete": {
			OperationID:    "DeleteWatch",
			Method:         "DELETE",
			PathTemplate:   "/api/v2/watches/{name}",
			RequiredParams: []string{"name"},
		},

		// Policy operations
		"policies/list": {
			OperationID:  "ListPolicies",
			Method:       "GET",
			PathTemplate: "/api/v2/policies",
		},
		"policies/get": {
			OperationID:    "GetPolicy",
			Method:         "GET",
			PathTemplate:   "/api/v2/policies/{name}",
			RequiredParams: []string{"name"},
		},
		"policies/create": {
			OperationID:    "CreatePolicy",
			Method:         "POST",
			PathTemplate:   "/api/v2/policies",
			RequiredParams: []string{"name", "type", "rules"},
		},
		"policies/update": {
			OperationID:    "UpdatePolicy",
			Method:         "PUT",
			PathTemplate:   "/api/v2/policies/{name}",
			RequiredParams: []string{"name"},
		},
		"policies/delete": {
			OperationID:    "DeletePolicy",
			Method:         "DELETE",
			PathTemplate:   "/api/v2/policies/{name}",
			RequiredParams: []string{"name"},
		},

		// Ignore rules operations
		"ignore-rules/list": {
			OperationID:  "ListIgnoreRules",
			Method:       "GET",
			PathTemplate: "/api/v1/ignore_rules",
		},
		"ignore-rules/create": {
			OperationID:    "CreateIgnoreRule",
			Method:         "POST",
			PathTemplate:   "/api/v1/ignore_rules",
			RequiredParams: []string{"vulnerability", "component"},
			OptionalParams: []string{"expiry_date", "reason"},
		},
		"ignore-rules/delete": {
			OperationID:    "DeleteIgnoreRule",
			Method:         "DELETE",
			PathTemplate:   "/api/v1/ignore_rules/{id}",
			RequiredParams: []string{"id"},
		},
	}

	// Merge component intelligence operations
	componentOps := p.AddComponentIntelligenceOperations()
	for key, op := range componentOps {
		operations[key] = op
	}

	// Merge reports and metrics operations
	reportsOps := p.AddReportsAndMetricsOperations()
	for key, op := range reportsOps {
		operations[key] = op
	}

	return operations
}

// GetDefaultConfiguration returns the default configuration for Xray
func (p *XrayProvider) GetDefaultConfiguration() providers.ProviderConfig {
	return providers.ProviderConfig{
		BaseURL:        "https://mycompany.jfrog.io/xray",
		AuthType:       "api_key",
		HealthEndpoint: "/api/v1/system/ping",
		RequiredScopes: []string{},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 60,
			RequestsPerHour:   3600,
		},
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "scanning",
				DisplayName: "Security Scanning",
				Description: "Scan artifacts and builds for vulnerabilities",
				Operations:  []string{"scan/artifact", "scan/build", "scan/status", "summary/artifact", "summary/build"},
			},
			{
				Name:        "violations",
				DisplayName: "Policy Violations",
				Description: "Monitor and manage security and license violations",
				Operations:  []string{"violations/list", "violations/artifact"},
			},
			{
				Name:        "watches",
				DisplayName: "Watch Management",
				Description: "Configure watches for continuous monitoring",
				Operations:  []string{"watches/list", "watches/get", "watches/create", "watches/update", "watches/delete"},
			},
			{
				Name:        "policies",
				DisplayName: "Policy Management",
				Description: "Define and manage security policies",
				Operations:  []string{"policies/list", "policies/get", "policies/create", "policies/update", "policies/delete"},
			},
			{
				Name:        "components",
				DisplayName: "Component Intelligence",
				Description: "Get detailed vulnerability information for components",
				Operations:  []string{"components/details", "components/search"},
			},
			{
				Name:        "reports",
				DisplayName: "Reporting",
				Description: "Generate security and compliance reports",
				Operations: []string{
					"reports/vulnerability", "reports/license", "reports/operational_risk",
					"reports/sbom", "reports/compliance", "reports/status", "reports/download",
					"reports/list", "reports/get", "reports/delete", "reports/schedule",
					"reports/schedule/list", "reports/schedule/delete",
					"reports/export/violations", "reports/export/inventory",
				},
			},
			{
				Name:        "metrics",
				DisplayName: "Metrics & Analytics",
				Description: "Security metrics and trend analysis",
				Operations: []string{
					"metrics/violations", "metrics/scans", "metrics/components",
					"metrics/exposure", "metrics/trends", "metrics/summary", "metrics/dashboard",
				},
			},
			{
				Name:        "system",
				DisplayName: "System",
				Description: "System health and version information",
				Operations:  []string{"system/ping", "system/version"},
			},
		},
		Timeout: 60 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnTimeout:   true,
			RetryOnRateLimit: true,
		},
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
	}
}

// GetAIOptimizedDefinitions returns AI-optimized definitions for Xray operations
func (p *XrayProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	// Will be implemented in a separate file for better organization
	return getXrayAIOptimizedDefinitions()
}

// GetOpenAPISpec returns the OpenAPI specification for Xray
func (p *XrayProvider) GetOpenAPISpec() (*openapi3.T, error) {
	// For now, return a basic spec
	// In production, this would load from embedded spec or fetch from API
	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "JFrog Xray API",
			Version: "v1",
		},
	}
	return spec, nil
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *XrayProvider) GetEmbeddedSpecVersion() string {
	return "v1.0.0"
}

// HealthCheck verifies the Xray service is accessible and functioning
func (p *XrayProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.GetDefaultConfiguration().BaseURL+"/api/v1/system/ping", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Note: Health check should work without authentication in some cases
	// But we'll apply auth if available
	if p.BaseProvider != nil {
		// The BaseProvider will handle authentication through context
		resp, err := p.ExecuteHTTPRequest(ctx, "GET", "/api/v1/system/ping", nil, nil)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer func() {
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
		}()
		// Don't treat auth errors as health check failures
		// The service is still healthy even if we can't authenticate
		if resp.StatusCode >= 500 {
			return fmt.Errorf("health check failed with status %d", resp.StatusCode)
		}
		return nil
	}

	// Fallback to direct HTTP call
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *XrayProvider) Close() error {
	// No persistent connections to close
	return nil
}

// InitializeWithPermissions initializes the provider with permission-based filtering
func (p *XrayProvider) InitializeWithPermissions(ctx context.Context, apiKey string) error {
	permissions, err := p.permissionDiscoverer.DiscoverPermissions(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("failed to discover permissions: %w", err)
	}

	// Filter operations based on permissions
	p.filteredOperations = p.permissionDiscoverer.FilterOperationsByPermissions(
		p.allOperations,
		permissions,
	)

	if p.BaseProvider != nil && p.GetLogger() != nil {
		p.GetLogger().Info("Initialized Xray provider with filtered operations", map[string]interface{}{
			"total_operations":   len(p.allOperations),
			"allowed_operations": len(p.filteredOperations),
		})
	}
	return nil
}

// isJFrogAPIKey detects if the provided credential is a JFrog API key vs access token
func (p *XrayProvider) isJFrogAPIKey(apiKey string) bool {
	// JFrog API keys are typically 64-73 character base64 strings
	// Access tokens are JWTs starting with "ey"
	if len(apiKey) >= 64 && len(apiKey) <= 73 && !isJWT(apiKey) {
		return true
	}
	return false
}

// isJWT checks if a string looks like a JWT token
func isJWT(token string) bool {
	// JWTs start with "ey" (base64 encoded "{")
	return len(token) > 2 && token[:2] == "ey"
}
