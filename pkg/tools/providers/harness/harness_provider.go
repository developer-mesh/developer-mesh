package harness

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

// Embed Harness OpenAPI spec as fallback
//
//go:embed harness-openapi.json
var harnessOpenAPISpecJSON []byte

// HarnessModule represents different Harness platform modules
type HarnessModule string

const (
	ModulePipeline  HarnessModule = "pipeline"
	ModuleProject   HarnessModule = "project"
	ModuleConnector HarnessModule = "connector"
	ModuleCCM       HarnessModule = "ccm"
	ModuleGitOps    HarnessModule = "gitops"
	ModuleIaCM      HarnessModule = "iacm"
	ModuleCV        HarnessModule = "cv"
	ModuleSTO       HarnessModule = "sto"
	ModuleFF        HarnessModule = "cf"
)

// HarnessProvider implements the StandardToolProvider interface for Harness
type HarnessProvider struct {
	*providers.BaseProvider
	specCache      repository.OpenAPICacheRepository
	specFallback   *openapi3.T
	httpClient     *http.Client
	enabledModules map[HarnessModule]bool
	accountID      string
	baseURL        string // Track the actual base URL for health checks
}

// NewHarnessProvider creates a new Harness provider instance
func NewHarnessProvider(logger observability.Logger) *HarnessProvider {
	base := providers.NewBaseProvider("harness", "v1", "https://app.harness.io", logger)

	// Load embedded spec as fallback
	var specFallback *openapi3.T
	if len(harnessOpenAPISpecJSON) > 0 {
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(harnessOpenAPISpecJSON)
		if err == nil {
			specFallback = spec
		}
	}

	provider := &HarnessProvider{
		BaseProvider: base,
		specFallback: specFallback,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://app.harness.io",
		enabledModules: map[HarnessModule]bool{
			ModulePipeline:  true,
			ModuleProject:   true,
			ModuleConnector: true,
			ModuleCCM:       true,
			ModuleGitOps:    true,
			ModuleCV:        true,
			ModuleSTO:       true,
			ModuleFF:        true,
			ModuleIaCM:      true,
		},
	}

	// Set operation mappings in base provider
	provider.SetOperationMappings(provider.GetOperationMappings())

	// Configure authentication type
	config := base.GetDefaultConfiguration()
	config.AuthType = "api_key"
	base.SetConfiguration(config)

	return provider
}

// NewHarnessProviderWithCache creates a new Harness provider with spec caching
func NewHarnessProviderWithCache(logger observability.Logger, specCache repository.OpenAPICacheRepository) *HarnessProvider {
	provider := NewHarnessProvider(logger)
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *HarnessProvider) GetProviderName() string {
	return "harness"
}

// GetSupportedVersions returns supported Harness API versions
func (p *HarnessProvider) GetSupportedVersions() []string {
	return []string{"v1", "v2", "ng"}
}

// GetToolDefinitions returns Harness-specific tool definitions
func (p *HarnessProvider) GetToolDefinitions() []providers.ToolDefinition {
	var tools []providers.ToolDefinition

	// Pipeline tools
	if p.enabledModules[ModulePipeline] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "harness_pipelines",
			DisplayName: "Harness Pipelines",
			Description: "Manage CI/CD pipelines in Harness",
			Category:    "ci_cd",
			Operation: providers.OperationDef{
				ID:           "pipelines",
				Method:       "GET",
				PathTemplate: "/v1/orgs/{org}/projects/{project}/pipelines",
			},
			Parameters: []providers.ParameterDef{
				{Name: "org", In: "path", Type: "string", Required: true, Description: "Organization identifier"},
				{Name: "project", In: "path", Type: "string", Required: true, Description: "Project identifier"},
				{Name: "page", In: "query", Type: "integer", Required: false, Description: "Page number", Default: 0},
				{Name: "limit", In: "query", Type: "integer", Required: false, Description: "Items per page", Default: 30},
			},
		})
	}

	// Project tools
	if p.enabledModules[ModuleProject] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "harness_projects",
			DisplayName: "Harness Projects",
			Description: "Manage projects in Harness organizations",
			Category:    "Platform",
			Operation: providers.OperationDef{
				ID:           "projects",
				Method:       "GET",
				PathTemplate: "/v1/orgs/{org}/projects",
			},
			Parameters: []providers.ParameterDef{
				{Name: "org", In: "path", Type: "string", Required: true, Description: "Organization identifier"},
				{Name: "has_module", In: "query", Type: "string", Required: false, Description: "Filter by module"},
			},
		})
	}

	// Connector tools
	if p.enabledModules[ModuleConnector] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "harness_connectors",
			DisplayName: "Harness Connectors",
			Description: "Manage connectors for external services",
			Category:    "Integration",
			Operation: providers.OperationDef{
				ID:           "connectors",
				Method:       "GET",
				PathTemplate: "/v1/orgs/{org}/projects/{project}/connectors",
			},
			Parameters: []providers.ParameterDef{
				{Name: "org", In: "path", Type: "string", Required: true, Description: "Organization identifier"},
				{Name: "project", In: "path", Type: "string", Required: true, Description: "Project identifier"},
				{Name: "type", In: "query", Type: "string", Required: false, Description: "Connector type filter"},
			},
		})
	}

	// GitOps tools
	if p.enabledModules[ModuleGitOps] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "harness_gitops",
			DisplayName: "Harness GitOps",
			Description: "Manage GitOps applications and deployments",
			Category:    "GitOps",
			Operation: providers.OperationDef{
				ID:           "gitops",
				Method:       "GET",
				PathTemplate: "/gitops/api/v1/agents",
			},
			Parameters: []providers.ParameterDef{
				{Name: "accountIdentifier", In: "query", Type: "string", Required: true, Description: "Account identifier"},
				{Name: "orgIdentifier", In: "query", Type: "string", Required: false, Description: "Organization identifier"},
				{Name: "projectIdentifier", In: "query", Type: "string", Required: false, Description: "Project identifier"},
			},
		})
	}

	// Cloud Cost Management tools
	if p.enabledModules[ModuleCCM] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "harness_ccm",
			DisplayName: "Harness Cloud Cost Management",
			Description: "Monitor and optimize cloud costs",
			Category:    "FinOps",
			Operation: providers.OperationDef{
				ID:           "ccm",
				Method:       "POST",
				PathTemplate: "/ccm/api/graphql",
			},
			Parameters: []providers.ParameterDef{
				{Name: "query", In: "body", Type: "string", Required: true, Description: "GraphQL query"},
			},
		})
	}

	// Security Testing Orchestration tools
	if p.enabledModules[ModuleSTO] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "harness_sto",
			DisplayName: "Harness Security Testing",
			Description: "Orchestrate security scans and vulnerability management",
			Category:    "Security",
			Operation: providers.OperationDef{
				ID:           "sto",
				Method:       "GET",
				PathTemplate: "/sto/api/v2/scans",
			},
			Parameters: []providers.ParameterDef{
				{Name: "accountIdentifier", In: "query", Type: "string", Required: true, Description: "Account identifier"},
				{Name: "orgIdentifier", In: "query", Type: "string", Required: true, Description: "Organization identifier"},
				{Name: "projectIdentifier", In: "query", Type: "string", Required: true, Description: "Project identifier"},
			},
		})
	}

	return tools
}

// ValidateCredentials validates Harness credentials
func (p *HarnessProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	apiKey, hasAPIKey := creds["api_key"]
	token, hasToken := creds["token"]
	pat, hasPAT := creds["personal_access_token"]

	// Accept any of these credential types
	authToken := ""
	if hasAPIKey {
		authToken = apiKey
	} else if hasToken {
		authToken = token
	} else if hasPAT {
		authToken = pat
	} else {
		return fmt.Errorf("missing required credentials: api_key, token, or personal_access_token")
	}

	// Validate API key format if it looks like a PAT
	if authToken != "" && !strings.HasPrefix(authToken, "pat.") && len(authToken) < 20 {
		return fmt.Errorf("invalid Harness API key format")
	}

	// Test the credentials with account info endpoint
	accountID := creds["account_id"]
	if accountID == "" {
		// Try to get account ID from the API key format (pat.ACCOUNT_ID.xxx)
		if strings.HasPrefix(authToken, "pat.") {
			parts := strings.Split(authToken, ".")
			if len(parts) >= 3 {
				accountID = parts[1]
			}
		}
	}

	// Build the URL using the configured base URL
	testPath := "gateway/ng/api/user/currentUser"
	if accountID != "" {
		testPath = fmt.Sprintf("%s?accountIdentifier=%s", testPath, accountID)
	}

	// Create a proper context with credentials for authentication
	pctx := &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: authToken,
		},
	}
	ctx = providers.WithContext(ctx, pctx)

	// Use ExecuteHTTPRequest which handles base URL properly
	resp, err := p.ExecuteHTTPRequest(ctx, "GET", testPath, nil, nil)

	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid Harness credentials")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from Harness API: %d - %s", resp.StatusCode, string(body))
	}

	// Store the account ID if we extracted it
	if accountID != "" {
		p.accountID = accountID
	}

	return nil
}

// GetOperationMappings returns Harness-specific operation mappings
func (p *HarnessProvider) GetOperationMappings() map[string]providers.OperationMapping {
	mappings := make(map[string]providers.OperationMapping)

	// Pipeline operations
	if p.enabledModules[ModulePipeline] {
		mappings["pipelines/list"] = providers.OperationMapping{
			OperationID:    "listPipelines",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{"page", "limit", "sort", "order", "module", "filter_identifier"},
		}
		mappings["pipelines/get"] = providers.OperationMapping{
			OperationID:    "getPipeline",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}",
			RequiredParams: []string{"org", "project", "pipeline"},
			OptionalParams: []string{"branch", "repo_identifier", "get_default_from_other_repo"},
		}
		mappings["pipelines/create"] = providers.OperationMapping{
			OperationID:    "createPipeline",
			Method:         "POST",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{"branch", "repo_identifier", "root_folder", "file_path"},
		}
		mappings["pipelines/update"] = providers.OperationMapping{
			OperationID:    "updatePipeline",
			Method:         "PUT",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}",
			RequiredParams: []string{"org", "project", "pipeline"},
			OptionalParams: []string{"branch", "repo_identifier", "root_folder", "file_path"},
		}
		mappings["pipelines/delete"] = providers.OperationMapping{
			OperationID:    "deletePipeline",
			Method:         "DELETE",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}",
			RequiredParams: []string{"org", "project", "pipeline"},
			OptionalParams: []string{"branch", "repo_identifier"},
		}
		mappings["pipelines/execute"] = providers.OperationMapping{
			OperationID:    "executePipeline",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipeline/execute/{identifier}",
			RequiredParams: []string{"identifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "module_type", "branch"},
		}
		mappings["pipelines/validate"] = providers.OperationMapping{
			OperationID:    "validatePipeline",
			Method:         "POST",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}/validate",
			RequiredParams: []string{"org", "project", "pipeline"},
			OptionalParams: []string{"branch", "repo_identifier"},
		}
	}

	// Project operations
	if p.enabledModules[ModuleProject] {
		mappings["projects/list"] = providers.OperationMapping{
			OperationID:    "listProjects",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects",
			RequiredParams: []string{"org"},
			OptionalParams: []string{"has_module", "page", "limit", "sort", "order"},
		}
		mappings["projects/get"] = providers.OperationMapping{
			OperationID:    "getProject",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}",
			RequiredParams: []string{"org", "project"},
		}
		mappings["projects/create"] = providers.OperationMapping{
			OperationID:    "createProject",
			Method:         "POST",
			PathTemplate:   "/v1/orgs/{org}/projects",
			RequiredParams: []string{"org"},
			OptionalParams: []string{},
		}
		mappings["projects/update"] = providers.OperationMapping{
			OperationID:    "updateProject",
			Method:         "PUT",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{},
		}
		mappings["projects/delete"] = providers.OperationMapping{
			OperationID:    "deleteProject",
			Method:         "DELETE",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{},
		}
		mappings["orgs/list"] = providers.OperationMapping{
			OperationID:    "listOrganizations",
			Method:         "GET",
			PathTemplate:   "/v1/orgs",
			RequiredParams: []string{},
			OptionalParams: []string{"page", "limit", "sort", "order"},
		}
		mappings["orgs/get"] = providers.OperationMapping{
			OperationID:    "getOrganization",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}",
			RequiredParams: []string{"org"},
		}
	}

	// Connector operations
	if p.enabledModules[ModuleConnector] {
		mappings["connectors/list"] = providers.OperationMapping{
			OperationID:    "listConnectors",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/connectors",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{"type", "category", "page", "limit", "search_term"},
		}
		mappings["connectors/get"] = providers.OperationMapping{
			OperationID:    "getConnector",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/connectors/{connector}",
			RequiredParams: []string{"org", "project", "connector"},
			OptionalParams: []string{"branch", "repo_identifier"},
		}
		mappings["connectors/create"] = providers.OperationMapping{
			OperationID:    "createConnector",
			Method:         "POST",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/connectors",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{"branch", "repo_identifier"},
		}
		mappings["connectors/update"] = providers.OperationMapping{
			OperationID:    "updateConnector",
			Method:         "PUT",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/connectors/{connector}",
			RequiredParams: []string{"org", "project", "connector"},
			OptionalParams: []string{"branch", "repo_identifier"},
		}
		mappings["connectors/delete"] = providers.OperationMapping{
			OperationID:    "deleteConnector",
			Method:         "DELETE",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/connectors/{connector}",
			RequiredParams: []string{"org", "project", "connector"},
			OptionalParams: []string{"branch", "repo_identifier"},
		}
		mappings["connectors/validate"] = providers.OperationMapping{
			OperationID:    "validateConnector",
			Method:         "POST",
			PathTemplate:   "/ng/api/connectors/testConnection",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "identifier"},
		}
	}

	// GitOps operations
	if p.enabledModules[ModuleGitOps] {
		mappings["gitops/agents/list"] = providers.OperationMapping{
			OperationID:    "listGitOpsAgents",
			Method:         "GET",
			PathTemplate:   "/gitops/api/v1/agents",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "identifier", "name", "type"},
		}
		mappings["gitops/applications/list"] = providers.OperationMapping{
			OperationID:    "listGitOpsApplications",
			Method:         "GET",
			PathTemplate:   "/gitops/api/v1/applications",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit"},
		}
		mappings["gitops/applications/sync"] = providers.OperationMapping{
			OperationID:    "syncGitOpsApplication",
			Method:         "POST",
			PathTemplate:   "/gitops/api/v1/applications/{app_name}/sync",
			RequiredParams: []string{"app_name"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "agentIdentifier"},
		}
		mappings["gitops/applications/rollback"] = providers.OperationMapping{
			OperationID:    "rollbackGitOpsApplication",
			Method:         "POST",
			PathTemplate:   "/gitops/api/v1/applications/{app_name}/rollback",
			RequiredParams: []string{"app_name"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "agentIdentifier", "targetRevision"},
		}
	}

	// Security Testing Orchestration operations
	if p.enabledModules[ModuleSTO] {
		mappings["sto/scans/list"] = providers.OperationMapping{
			OperationID:    "listSecurityScans",
			Method:         "GET",
			PathTemplate:   "/sto/api/v2/scans",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit"},
		}
		mappings["sto/vulnerabilities/list"] = providers.OperationMapping{
			OperationID:    "listVulnerabilities",
			Method:         "GET",
			PathTemplate:   "/sto/api/v2/issues",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "severity", "page", "limit"},
		}
		mappings["sto/exemptions/create"] = providers.OperationMapping{
			OperationID:    "createSecurityExemption",
			Method:         "POST",
			PathTemplate:   "/sto/api/v2/exemptions",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
	}

	// Cloud Cost Management operations
	if p.enabledModules[ModuleCCM] {
		mappings["ccm/costs/overview"] = providers.OperationMapping{
			OperationID:    "getCostOverview",
			Method:         "POST",
			PathTemplate:   "/ccm/api/graphql",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier"},
		}
		mappings["ccm/budgets/list"] = providers.OperationMapping{
			OperationID:    "listBudgets",
			Method:         "GET",
			PathTemplate:   "/ccm/api/budgets",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "page", "limit"},
		}
		mappings["ccm/recommendations/list"] = providers.OperationMapping{
			OperationID:    "listCostRecommendations",
			Method:         "POST",
			PathTemplate:   "/ccm/api/recommendation/overview/list",
			RequiredParams: []string{"accountIdentifier"},
			OptionalParams: []string{},
		}
		mappings["ccm/anomalies/list"] = providers.OperationMapping{
			OperationID:    "listCostAnomalies",
			Method:         "POST",
			PathTemplate:   "/ccm/api/anomaly",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier"},
		}
	}

	return mappings
}

// GetDefaultConfiguration returns default Harness configuration
func (p *HarnessProvider) GetDefaultConfiguration() providers.ProviderConfig {
	return providers.ProviderConfig{
		BaseURL:  "https://app.harness.io",
		AuthType: "api_key",
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 100, // Conservative default, varies by tier
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnRateLimit: true,
			RetryOnTimeout:   true,
		},
		OperationGroups: p.getOperationGroups(),
	}
}

// getOperationGroups returns operation groups for Harness
func (p *HarnessProvider) getOperationGroups() []providers.OperationGroup {
	var groups []providers.OperationGroup

	if p.enabledModules[ModulePipeline] {
		groups = append(groups, providers.OperationGroup{
			Name:        "pipelines",
			DisplayName: "Pipeline Management",
			Description: "Create, manage, and execute CI/CD pipelines",
			Operations: []string{
				"pipelines/list", "pipelines/get", "pipelines/create",
				"pipelines/update", "pipelines/delete", "pipelines/execute",
				"pipelines/validate",
			},
		})
	}

	if p.enabledModules[ModuleProject] {
		groups = append(groups, providers.OperationGroup{
			Name:        "projects",
			DisplayName: "Project & Organization Management",
			Description: "Manage organizations and projects",
			Operations: []string{
				"projects/list", "projects/get", "projects/create",
				"projects/update", "projects/delete",
				"orgs/list", "orgs/get",
			},
		})
	}

	if p.enabledModules[ModuleConnector] {
		groups = append(groups, providers.OperationGroup{
			Name:        "connectors",
			DisplayName: "Connector Management",
			Description: "Configure connections to external services",
			Operations: []string{
				"connectors/list", "connectors/get", "connectors/create",
				"connectors/update", "connectors/delete", "connectors/validate",
			},
		})
	}

	if p.enabledModules[ModuleGitOps] {
		groups = append(groups, providers.OperationGroup{
			Name:        "gitops",
			DisplayName: "GitOps Management",
			Description: "Manage GitOps agents and applications",
			Operations: []string{
				"gitops/agents/list", "gitops/applications/list",
				"gitops/applications/sync", "gitops/applications/rollback",
			},
		})
	}

	if p.enabledModules[ModuleSTO] {
		groups = append(groups, providers.OperationGroup{
			Name:        "security",
			DisplayName: "Security Testing",
			Description: "Security scanning and vulnerability management",
			Operations: []string{
				"sto/scans/list", "sto/vulnerabilities/list", "sto/exemptions/create",
			},
		})
	}

	if p.enabledModules[ModuleCCM] {
		groups = append(groups, providers.OperationGroup{
			Name:        "cost",
			DisplayName: "Cloud Cost Management",
			Description: "Monitor and optimize cloud costs",
			Operations: []string{
				"ccm/costs/overview", "ccm/budgets/list",
				"ccm/recommendations/list", "ccm/anomalies/list",
			},
		})
	}

	return groups
}

// ExecuteOperation executes a Harness operation
func (p *HarnessProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize operation name
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	// Add account ID if available and not provided
	if p.accountID != "" {
		if _, hasAccount := params["accountIdentifier"]; !hasAccount {
			params["accountIdentifier"] = p.accountID
		}
	}

	// Use base provider's execution with Harness-specific handling
	return p.Execute(ctx, operation, params)
}

// normalizeOperationName normalizes operation names to handle different formats
func (p *HarnessProvider) normalizeOperationName(operation string) string {
	// Handle different separators
	operation = strings.ReplaceAll(operation, "-", "/")
	operation = strings.ReplaceAll(operation, "_", "/")

	// Handle simple action names that need module context
	// This would need to be enhanced with actual parameter context
	// For now, just return the normalized operation
	return operation
}

// GetOpenAPISpec returns the OpenAPI specification for Harness
func (p *HarnessProvider) GetOpenAPISpec() (*openapi3.T, error) {
	ctx := context.Background()

	// Try cache first if available
	if p.specCache != nil {
		spec, err := p.specCache.Get(ctx, "harness-v1")
		if err == nil && spec != nil {
			return spec, nil
		}
	}

	// Use embedded spec as it's comprehensive
	if p.specFallback != nil {
		return p.specFallback, nil
	}

	// If no fallback, try to load embedded spec
	if len(harnessOpenAPISpecJSON) > 0 {
		// The Harness spec has some compatibility issues with kin-openapi
		// For now, we'll return a minimal spec that allows the provider to work
		// TODO: Fix the embedded spec or use a different parser
		loader := openapi3.NewLoader()
		// Create a minimal valid spec for testing
		minimalSpec := []byte(`{
			"openapi": "3.0.0",
			"info": {
				"title": "Harness API",
				"version": "1.0.0"
			},
			"paths": {},
			"components": {
				"schemas": {}
			}
		}`)
		spec, err := loader.LoadFromData(minimalSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
		}

		// Cache the spec if cache is available
		if p.specCache != nil {
			_ = p.specCache.Set(ctx, "harness-v1", spec, 24*time.Hour)
		}

		return spec, nil
	}

	return nil, fmt.Errorf("no OpenAPI spec available")
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *HarnessProvider) GetEmbeddedSpecVersion() string {
	return "v1.0-2024"
}

// SetConfiguration sets the provider configuration
func (p *HarnessProvider) SetConfiguration(config providers.ProviderConfig) {
	p.BaseProvider.SetConfiguration(config)
	if config.BaseURL != "" {
		p.baseURL = config.BaseURL
	}
}

// HealthCheck verifies the Harness API is accessible
func (p *HarnessProvider) HealthCheck(ctx context.Context) error {
	// For Harness, we need to check the health endpoint directly
	// because BaseProvider only treats 5xx as errors
	healthPath := "gateway/health"
	healthURL := strings.TrimRight(p.baseURL, "/") + "/" + strings.TrimLeft(healthPath, "/")

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("harness API health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// For Harness, treat anything other than 200 as unhealthy
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("harness API health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *HarnessProvider) Close() error {
	// Currently no resources to clean up
	return nil
}

// SetEnabledModules configures which Harness modules are enabled
func (p *HarnessProvider) SetEnabledModules(modules []HarnessModule) {
	// Reset all modules
	for module := range p.enabledModules {
		p.enabledModules[module] = false
	}
	// Enable specified modules
	for _, module := range modules {
		p.enabledModules[module] = true
	}
}

// GetEnabledModules returns the currently enabled modules
func (p *HarnessProvider) GetEnabledModules() []HarnessModule {
	var modules []HarnessModule
	for module, enabled := range p.enabledModules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}
