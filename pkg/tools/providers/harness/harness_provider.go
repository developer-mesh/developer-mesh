package harness

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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

// isTestEnvironment checks if we're running in a test environment
// This is used to skip loading the large embedded OpenAPI spec during tests
func isTestEnvironment() bool {
	// Check if the test flag is set (go test sets this)
	if flag.Lookup("test.v") != nil {
		return true
	}
	// Check for TEST environment variable as fallback
	if os.Getenv("TEST") == "true" {
		return true
	}
	return false
}

// HarnessModule represents different Harness platform modules
type HarnessModule string

// Harness API Namespace Documentation:
// The Harness platform uses different API namespaces for different generations and modules:
//
// /ng/api/        - Next Generation APIs (most current platform features)
// /pipeline/api/  - Pipeline-specific operations (executions, triggers, approvals)
// /pm/api/        - Policy Management APIs (governance, OPA policies)
// /ccm/api/       - Cloud Cost Management specific APIs
// /cf/admin/      - Feature Flag administration APIs
// /cv/api/        - Continuous Verification APIs
// /chaos/api/     - Chaos Engineering APIs
// /sto/api/       - Security Testing Orchestration APIs
// /gitops/api/    - GitOps specific APIs
// /idp/api/       - Internal Developer Portal APIs
// /iacm/api/      - Infrastructure as Code Management APIs
//
// Each namespace represents a different microservice or module boundary within
// the Harness platform architecture.
const (
	// Core Pipeline and Project Management
	ModulePipeline  HarnessModule = "pipeline"
	ModuleProject   HarnessModule = "project"
	ModuleConnector HarnessModule = "connector"
	// Cloud Cost Management (CCM) - FinOps and cost optimization
	ModuleCCM       HarnessModule = "ccm" // Cloud Cost Management
	ModuleCloudCost HarnessModule = "ccm" // Alias for clarity
	// Continuous Delivery and GitOps
	ModuleGitOps HarnessModule = "gitops"
	// Infrastructure as Code Management (IaCM) - Terraform, CloudFormation, etc.
	ModuleIaCM        HarnessModule = "iacm" // Infrastructure as Code Management
	ModuleInfraAsCode HarnessModule = "iacm" // Alias for clarity
	// Continuous Verification (CV) - APM and monitoring integration
	ModuleCV                     HarnessModule = "cv" // Continuous Verification
	ModuleContinuousVerification HarnessModule = "cv" // Alias for clarity
	// Security Testing Orchestration (STO) - Security scanning and testing
	ModuleSTO             HarnessModule = "sto" // Security Testing Orchestration
	ModuleSecurityTesting HarnessModule = "sto" // Alias for clarity
	// Feature Flags (FF) - Progressive delivery and feature management
	ModuleFF          HarnessModule = "cf" // Note: uses 'cf' internally for Feature Flags
	ModuleFeatureFlag HarnessModule = "cf" // Alias for clarity
	// Service Delivery Components
	ModuleService        HarnessModule = "service"
	ModuleEnvironment    HarnessModule = "environment"
	ModuleInfra          HarnessModule = "infrastructure"
	ModuleInfrastructure HarnessModule = "infrastructure" // Alias for clarity
	// Source Control and Artifact Management
	ModulePullRequest HarnessModule = "pullrequest"
	ModuleRepository  HarnessModule = "repository"
	ModuleRegistry    HarnessModule = "registry"
	// Observability and Monitoring
	ModuleDashboard HarnessModule = "dashboard"
	ModuleLogs      HarnessModule = "logs"
	ModuleAudit     HarnessModule = "audit"
	// Chaos Engineering - Resilience testing
	ModuleChaos            HarnessModule = "chaos"
	ModuleChaosEngineering HarnessModule = "chaos" // Alias for clarity
	// Supply Chain Security Assurance (SSCA) - SBOM and vulnerability management
	ModuleSSCA                HarnessModule = "ssca" // Supply Chain Security Assurance
	ModuleSupplyChainSecurity HarnessModule = "ssca" // Alias for clarity
	// Internal Developer Portal (IDP) - Service catalog and scorecards
	ModuleIDP             HarnessModule = "idp" // Internal Developer Portal
	ModuleDeveloperPortal HarnessModule = "idp" // Alias for clarity
	// Platform Core Components
	ModuleTemplate  HarnessModule = "template"
	ModuleDatabase  HarnessModule = "database"
	ModuleExecution HarnessModule = "execution"
	ModuleSecret    HarnessModule = "secret"
	// Identity and Access Management
	ModuleUser         HarnessModule = "user"
	ModuleDelegate     HarnessModule = "delegate"
	ModuleApproval     HarnessModule = "approval"
	ModuleNotification HarnessModule = "notification"
	ModuleWebhook      HarnessModule = "webhook"
	ModuleAPIKey       HarnessModule = "apikey"
	ModuleAccount      HarnessModule = "account"
	ModuleLicense      HarnessModule = "license"
	// Configuration Management
	ModuleVariable  HarnessModule = "variable"
	ModuleFileStore HarnessModule = "filestore"
	ModuleManifest  HarnessModule = "manifest"
	// Access Control and Governance
	ModuleResourceGroup   HarnessModule = "resourcegroup"
	ModuleRBACPolicy      HarnessModule = "rbacpolicy"
	ModuleDelegateProfile HarnessModule = "delegateprofile"
	ModuleGovernance      HarnessModule = "governance"
	// Pipeline Configuration
	ModuleTrigger      HarnessModule = "trigger"
	ModuleInputSet     HarnessModule = "inputset"
	ModuleFreezeWindow HarnessModule = "freezewindow"
)

// Parameter Type Documentation:
// Common parameter types and their expected formats across Harness APIs:
//
// Identifiers (string):
//   - accountIdentifier: Account ID (e.g., "kmpySmUISimoRrJL6NL73w")
//   - orgIdentifier: Organization identifier (alphanumeric, hyphens, underscores)
//   - projectIdentifier: Project identifier (alphanumeric, hyphens, underscores)
//   - pipelineIdentifier: Pipeline identifier
//   - serviceIdentifier: Service identifier
//
// Pagination (integer):
//   - page: Page number (0-based), default: 0
//   - limit: Items per page, default: 20, max: 100
//   - pageSize: Alternative to 'limit' in some APIs
//   - offset: Number of items to skip
//
// Time Parameters (ISO 8601 string or Unix timestamp):
//   - startTime: Start time for date range queries (e.g., "2024-01-01T00:00:00Z" or 1704067200000)
//   - endTime: End time for date range queries
//   - period: Time period (e.g., "LAST_30_DAYS", "LAST_QUARTER")
//
// Boolean Parameters (string "true" or "false"):
//   - enable: Enable/disable flag
//   - merge: Whether to merge or replace
//   - replace_all: Replace all occurrences
//
// Status/State Parameters (enum string):
//   - status: Entity status (e.g., "ACTIVE", "INACTIVE", "PENDING")
//   - severity: Issue severity (e.g., "HIGH", "MEDIUM", "LOW", "CRITICAL")
//   - type: Entity type specific to each API
//
// Search and Filter (string):
//   - searchTerm: Text search query
//   - filter: Complex filter expression (API-specific format)
//   - query: GraphQL query string (for GraphQL endpoints)
//
// Arrays (comma-separated string or JSON array):
//   - inputSetIdentifiers: Comma-separated list of IDs
//   - userJourneyIdentifiers: Array of journey IDs
//
// Error Response Patterns:
//   - 200: Success
//   - 201: Created
//   - 204: No Content (successful delete)
//   - 400: Bad Request (invalid parameters)
//   - 401: Unauthorized (invalid API key)
//   - 403: Forbidden (insufficient permissions)
//   - 404: Not Found
//   - 409: Conflict (resource already exists)
//   - 429: Rate Limited
//   - 500: Internal Server Error
//   - 503: Service Unavailable
//
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
	// Skip loading during tests to avoid performance issues with large spec file (12MB)
	var specFallback *openapi3.T
	if !isTestEnvironment() && len(harnessOpenAPISpecJSON) > 0 {
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
			// Core CI/CD and deployment workflow (5 modules, ~52 tools)
			ModulePipeline:    true, // Pipeline execution and management (~12 tools)
			ModuleExecution:   true, // Execution logs, status, abort (~10 tools)
			ModulePullRequest: true, // PR operations, code review (~10 tools)
			ModuleSTO:         true, // Security scanning in pipelines (~12 tools)
			ModuleService:     true, // Service definitions for deployments (~8 tools)

			// Disabled for context optimization (15 modules, ~244 tools):
			// - ModuleRepository: Repository management - use GitHub/GitLab
			// - ModuleSSCA: Supply chain security - security team tool
			// - ModuleChaos: Chaos engineering - SRE/platform team
			// - ModuleGitOps: GitOps operations - use kubectl/ArgoCD
			// - ModuleIaCM: Terraform operations - use Terraform CLI
			// - ModuleEnvironment: Environment management - operations
			// - ModuleInfra: Infrastructure definitions - platform admin
			// - ModuleManifest: K8s manifests - operations
			// - ModuleVariable: Variable management - configuration admin
			// - ModuleSecret: Secret management - security admin
			// - ModuleConnector: Connector setup - one-time platform admin
			// - ModuleFileStore: File storage - operations
			// - ModuleLogs: Log management - observability team
			// - ModuleFF: Feature flags - product management
			// - ModuleApproval: Approval workflows - operations
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
		// Project tools
		// Project operations - always build for dynamic filtering
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
		// Connector tools
		// Connector operations - always build for dynamic filtering
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
		// GitOps tools
		// GitOps operations - always build for dynamic filtering
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
		// Cloud Cost Management tools
		// CCM operations - always build for dynamic filtering
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
		// Security Testing Orchestration tools
		// STO operations - always build for dynamic filtering
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
// Each operation mapping defines how to interact with a specific Harness API endpoint.
// Operations are organized by module and follow RESTful conventions.
func (p *HarnessProvider) GetOperationMappings() map[string]providers.OperationMapping {
	mappings := make(map[string]providers.OperationMapping)
	// IMPORTANT: Build ALL operation mappings regardless of enabled modules
	// Permission filtering happens at the REST API level during tool expansion
	// This ensures operations are available when permissions are discovered dynamically
	// Pipeline operations - Core CI/CD pipeline management
	// API Namespace: /v1/ (legacy) and /pipeline/api/ (current)
	// Build all pipeline operations for dynamic permission filtering
	{
		// List all pipelines in a project
		// Returns: Array of pipeline summaries with basic metadata
		// Pagination: Use page (0-based) and limit (max 100)
		// Filtering: filter_identifier for saved filters, module for specific modules
		mappings["pipelines/list"] = providers.OperationMapping{
			OperationID:    "listPipelines",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines",
			RequiredParams: []string{"org", "project"},
			OptionalParams: []string{"page", "limit", "sort", "order", "module", "filter_identifier"},
		}
		// Get detailed pipeline configuration
		// Returns: Complete pipeline YAML and metadata
		// Git-aware: Use branch and repo_identifier for remote pipelines
		mappings["pipelines/get"] = providers.OperationMapping{
			OperationID:    "getPipeline",
			Method:         "GET",
			PathTemplate:   "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}",
			RequiredParams: []string{"org", "project", "pipeline"},
			OptionalParams: []string{"branch", "repo_identifier", "get_default_from_other_repo"},
		}
		// Create a new pipeline
		// Body: Pipeline YAML configuration required
		// Git-aware: Specify branch, repo, and file path for remote storage
		// Returns: 201 Created with pipeline details
		// Errors: 409 if identifier exists, 400 for invalid YAML
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
		mappings["pipelines/execution-url"] = providers.OperationMapping{
			OperationID:    "fetchExecutionUrl",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/execution/{planExecutionId}/url",
			RequiredParams: []string{"planExecutionId"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		// Project operations
		// Project operations - always build for dynamic filtering
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
		// Connector operations
		// Connector operations - always build for dynamic filtering
		// GitOps operations
		// GitOps operations - always build for dynamic filtering
		// Security Testing Orchestration operations
		// STO operations - always build for dynamic filtering
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
		// Cloud Cost Management (CCM) operations - FinOps and cost optimization
		// API Namespace: /ccm/api/ - Dedicated cost management service
		// Note: Many CCM operations use GraphQL for complex queries
		// CCM operations - always build for dynamic filtering
		// Get cost overview using GraphQL
		// Body: GraphQL query for cost aggregation (e.g., by service, environment, cluster)
		// Returns: Cost breakdown based on query parameters
		// Common queries: Total spend, cost by service, cost trends
		// Note: This is a GraphQL endpoint - query structure varies by use case
		// List all budget configurations
		// Returns: Array of budget definitions with thresholds and alerts
		// Pagination: Use page (0-based) and limit (max 100)
		// Use case: Monitor spending against predefined budgets
		// Get AI-powered cost optimization recommendations
		// Returns: Prioritized list of cost-saving opportunities
		// Categories: Idle resources, rightsizing, reserved instances, spot usage
		// Note: Requires accountIdentifier for multi-account setups
		// Potential savings calculated based on historical usage patterns
		// Service operations
		// Service operations - always build for dynamic filtering
		mappings["services/list"] = providers.OperationMapping{
			OperationID:    "listServices",
			Method:         "GET",
			PathTemplate:   "/ng/api/servicesV2",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "size", "sort"},
		}
		mappings["services/get"] = providers.OperationMapping{
			OperationID:    "getService",
			Method:         "GET",
			PathTemplate:   "/ng/api/servicesV2/{serviceIdentifier}",
			RequiredParams: []string{"serviceIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["services/create"] = providers.OperationMapping{
			OperationID:    "createService",
			Method:         "POST",
			PathTemplate:   "/ng/api/servicesV2",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["services/update"] = providers.OperationMapping{
			OperationID:    "updateService",
			Method:         "PUT",
			PathTemplate:   "/ng/api/servicesV2/{serviceIdentifier}",
			RequiredParams: []string{"serviceIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["services/delete"] = providers.OperationMapping{
			OperationID:    "deleteService",
			Method:         "DELETE",
			PathTemplate:   "/ng/api/servicesV2/{serviceIdentifier}",
			RequiredParams: []string{"serviceIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		// Environment operations
		// Environment operations - always build for dynamic filtering
		mappings["environments/list"] = providers.OperationMapping{
			OperationID:    "listEnvironments",
			Method:         "GET",
			PathTemplate:   "/ng/api/environmentsV2",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "size", "sort"},
		}
		mappings["environments/get"] = providers.OperationMapping{
			OperationID:    "getEnvironment",
			Method:         "GET",
			PathTemplate:   "/ng/api/environmentsV2/{environmentIdentifier}",
			RequiredParams: []string{"environmentIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["environments/create"] = providers.OperationMapping{
			OperationID:    "createEnvironment",
			Method:         "POST",
			PathTemplate:   "/ng/api/environmentsV2",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["environments/update"] = providers.OperationMapping{
			OperationID:    "updateEnvironment",
			Method:         "PUT",
			PathTemplate:   "/ng/api/environmentsV2/{environmentIdentifier}",
			RequiredParams: []string{"environmentIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["environments/delete"] = providers.OperationMapping{
			OperationID:    "deleteEnvironment",
			Method:         "DELETE",
			PathTemplate:   "/ng/api/environmentsV2/{environmentIdentifier}",
			RequiredParams: []string{"environmentIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["environments/move-configs"] = providers.OperationMapping{
			OperationID:    "moveEnvironmentConfigs",
			Method:         "POST",
			PathTemplate:   "/ng/api/environmentsV2/{environmentIdentifier}/move-configs",
			RequiredParams: []string{"environmentIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "targetEnvironment"},
		}
		// Infrastructure operations
		// Infrastructure operations - always build for dynamic filtering
		mappings["infrastructures/list"] = providers.OperationMapping{
			OperationID:    "listInfrastructures",
			Method:         "GET",
			PathTemplate:   "/ng/api/infrastructures",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "environmentIdentifier", "page", "size"},
		}
		mappings["infrastructures/get"] = providers.OperationMapping{
			OperationID:    "getInfrastructure",
			Method:         "GET",
			PathTemplate:   "/ng/api/infrastructures/{infrastructureIdentifier}",
			RequiredParams: []string{"infrastructureIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "environmentIdentifier"},
		}
		mappings["infrastructures/create"] = providers.OperationMapping{
			OperationID:    "createInfrastructure",
			Method:         "POST",
			PathTemplate:   "/ng/api/infrastructures",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["infrastructures/update"] = providers.OperationMapping{
			OperationID:    "updateInfrastructure",
			Method:         "PUT",
			PathTemplate:   "/ng/api/infrastructures/{infrastructureIdentifier}",
			RequiredParams: []string{"infrastructureIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["infrastructures/delete"] = providers.OperationMapping{
			OperationID:    "deleteInfrastructure",
			Method:         "DELETE",
			PathTemplate:   "/ng/api/infrastructures/{infrastructureIdentifier}",
			RequiredParams: []string{"infrastructureIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["infrastructures/move-configs"] = providers.OperationMapping{
			OperationID:    "moveInfrastructureConfigs",
			Method:         "POST",
			PathTemplate:   "/ng/api/infrastructures/{infrastructureIdentifier}/move-configs",
			RequiredParams: []string{"infrastructureIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "targetInfrastructure"},
		}
		// Pull Request operations
		// PullRequest operations - always build for dynamic filtering
		mappings["pullrequests/list"] = providers.OperationMapping{
			OperationID:    "listPullRequests",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq",
			RequiredParams: []string{"repoIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit", "state"},
		}
		mappings["pullrequests/get"] = providers.OperationMapping{
			OperationID:    "getPullRequest",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq/{pullRequestNumber}",
			RequiredParams: []string{"repoIdentifier", "pullRequestNumber"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["pullrequests/create"] = providers.OperationMapping{
			OperationID:    "createPullRequest",
			Method:         "POST",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq",
			RequiredParams: []string{"repoIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["pullrequests/merge"] = providers.OperationMapping{
			OperationID:    "mergePullRequest",
			Method:         "POST",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq/{pullRequestNumber}/merge",
			RequiredParams: []string{"repoIdentifier", "pullRequestNumber"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["pullrequests/review"] = providers.OperationMapping{
			OperationID:    "reviewPullRequest",
			Method:         "POST",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq/{pullRequestNumber}/reviews",
			RequiredParams: []string{"repoIdentifier", "pullRequestNumber"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["pullrequests/checks"] = providers.OperationMapping{
			OperationID:    "getPullRequestChecks",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq/{pullRequestNumber}/checks",
			RequiredParams: []string{"repoIdentifier", "pullRequestNumber"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["pullrequests/activities"] = providers.OperationMapping{
			OperationID:    "getPullRequestActivities",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/pullreq/{pullRequestNumber}/activities",
			RequiredParams: []string{"repoIdentifier", "pullRequestNumber"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit"},
		}
		// Repository operations
		// Repository operations - always build for dynamic filtering
		mappings["repositories/list"] = providers.OperationMapping{
			OperationID:    "listRepositories",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit"},
		}
		mappings["repositories/get"] = providers.OperationMapping{
			OperationID:    "getRepository",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}",
			RequiredParams: []string{"repoIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["repositories/branches"] = providers.OperationMapping{
			OperationID:    "listBranches",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/branches",
			RequiredParams: []string{"repoIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit"},
		}
		mappings["repositories/commits"] = providers.OperationMapping{
			OperationID:    "listCommits",
			Method:         "GET",
			PathTemplate:   "/code/api/v1/repos/{repoIdentifier}/commits",
			RequiredParams: []string{"repoIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "branch", "since", "until", "page", "limit"},
		}
		// Registry operations
		// Registry operations - always build for dynamic filtering
		// Dashboard operations
		// Dashboard operations - always build for dynamic filtering
		// Chaos Engineering operations
		// Chaos operations - always build for dynamic filtering
		// Supply Chain Security operations
		// SSCA operations - always build for dynamic filtering
		mappings["ssca/sbom/generate"] = providers.OperationMapping{
			OperationID:    "generateSBOM",
			Method:         "POST",
			PathTemplate:   "/ssca/api/sbom/generate",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "artifactId"},
		}
		mappings["ssca/sbom/get"] = providers.OperationMapping{
			OperationID:    "getSBOM",
			Method:         "GET",
			PathTemplate:   "/ssca/api/sbom/{sbomId}",
			RequiredParams: []string{"sbomId"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["ssca/artifacts/scan"] = providers.OperationMapping{
			OperationID:    "scanArtifact",
			Method:         "POST",
			PathTemplate:   "/ssca/api/artifacts/scan",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["ssca/vulnerabilities/list"] = providers.OperationMapping{
			OperationID:    "listSSCAVulnerabilities",
			Method:         "GET",
			PathTemplate:   "/ssca/api/vulnerabilities",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "severity", "page", "limit"},
		}
		// Logs operations
		// Logs operations - always build for dynamic filtering
		mappings["logs/download"] = providers.OperationMapping{
			OperationID:    "downloadLogs",
			Method:         "GET",
			PathTemplate:   "/log-service/stream",
			RequiredParams: []string{},
			OptionalParams: []string{"accountID", "key", "prefix"},
		}
		mappings["logs/stream"] = providers.OperationMapping{
			OperationID:    "streamLogs",
			Method:         "GET",
			PathTemplate:   "/log-service/stream/v2",
			RequiredParams: []string{},
			OptionalParams: []string{"accountID", "key", "prefix", "follow"},
		}
		// Template operations
		// Template operations - always build for dynamic filtering
		// IDP operations
		// IDP operations - always build for dynamic filtering
		// Audit operations
		// Audit operations - always build for dynamic filtering
		// Database operations
		// Database operations - always build for dynamic filtering
		// Feature Flag operations
		// Feature Flag operations - always build for dynamic filtering
		// Continuous Verification operations
		// CV operations - always build for dynamic filtering
		// IaCM operations
		// IaCM operations - always build for dynamic filtering
		// Execution operations
		// Execution operations - always build for dynamic filtering
		mappings["executions/list"] = providers.OperationMapping{
			OperationID:    "listExecutions",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipelines/execution/summary",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier", "page", "size", "status"},
		}
		mappings["executions/get"] = providers.OperationMapping{
			OperationID:    "getExecution",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/execution/{planExecutionId}/summary",
			RequiredParams: []string{"planExecutionId"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["executions/status"] = providers.OperationMapping{
			OperationID:    "getExecutionStatus",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/execution/{planExecutionId}/status",
			RequiredParams: []string{"planExecutionId"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["executions/rollback"] = providers.OperationMapping{
			OperationID:    "rollbackExecution",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipelines/execution/{planExecutionId}/rollback",
			RequiredParams: []string{"planExecutionId"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["executions/abort"] = providers.OperationMapping{
			OperationID:    "abortExecution",
			Method:         "PUT",
			PathTemplate:   "/pipeline/api/pipelines/execution/{planExecutionId}/abort",
			RequiredParams: []string{"planExecutionId"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		// Secret operations
		// Secret operations - always build for dynamic filtering
		// User and Role operations
		// User operations - always build for dynamic filtering
		// Delegate operations
		// Delegate operations - always build for dynamic filtering
		// Approval operations
		// Approval operations - always build for dynamic filtering
		// Notification operations
		// Notification operations - always build for dynamic filtering
		// Webhook operations
		// Webhook operations - always build for dynamic filtering
		// API Key operations
		// API Key operations - always build for dynamic filtering
		// Account operations
		// Account operations - always build for dynamic filtering
		// License operations
		// License operations - always build for dynamic filtering
		// Variable operations
		// Variable operations - always build for dynamic filtering
		// File Store operations
		// FileStore operations - always build for dynamic filtering
		// Resource Group operations
		// ResourceGroup operations - always build for dynamic filtering
		// RBAC Policy operations
		// RBACPolicy operations - always build for dynamic filtering
		// Delegate Profile operations
		// DelegateProfile operations - always build for dynamic filtering
		// Governance Policy operations
		// Governance operations - always build for dynamic filtering
		// Manifest operations
		// Manifest operations - always build for dynamic filtering
		mappings["manifests/list"] = providers.OperationMapping{
			OperationID:    "listManifests",
			Method:         "GET",
			PathTemplate:   "/ng/api/manifests",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "serviceIdentifier", "page", "limit"},
		}
		mappings["manifests/get"] = providers.OperationMapping{
			OperationID:    "getManifest",
			Method:         "GET",
			PathTemplate:   "/ng/api/manifests/{manifestIdentifier}",
			RequiredParams: []string{"manifestIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "serviceIdentifier"},
		}
		mappings["manifests/create"] = providers.OperationMapping{
			OperationID:    "createManifest",
			Method:         "POST",
			PathTemplate:   "/ng/api/manifests",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "serviceIdentifier"},
		}
		mappings["manifests/update"] = providers.OperationMapping{
			OperationID:    "updateManifest",
			Method:         "PUT",
			PathTemplate:   "/ng/api/manifests/{manifestIdentifier}",
			RequiredParams: []string{"manifestIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "serviceIdentifier"},
		}
		mappings["manifests/delete"] = providers.OperationMapping{
			OperationID:    "deleteManifest",
			Method:         "DELETE",
			PathTemplate:   "/ng/api/manifests/{manifestIdentifier}",
			RequiredParams: []string{"manifestIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "serviceIdentifier"},
		}
		// Trigger operations
		// Trigger operations - always build for dynamic filtering
		mappings["triggers/list"] = providers.OperationMapping{
			OperationID:    "listTriggers",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/triggers",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier", "page", "limit"},
		}
		mappings["triggers/get"] = providers.OperationMapping{
			OperationID:    "getTrigger",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/triggers/{triggerIdentifier}",
			RequiredParams: []string{"triggerIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier"},
		}
		mappings["triggers/create"] = providers.OperationMapping{
			OperationID:    "createTrigger",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipelines/triggers",
			RequiredParams: []string{},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier"},
		}
		mappings["triggers/update"] = providers.OperationMapping{
			OperationID:    "updateTrigger",
			Method:         "PUT",
			PathTemplate:   "/pipeline/api/pipelines/triggers/{triggerIdentifier}",
			RequiredParams: []string{"triggerIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier"},
		}
		mappings["triggers/delete"] = providers.OperationMapping{
			OperationID:    "deleteTrigger",
			Method:         "DELETE",
			PathTemplate:   "/pipeline/api/pipelines/triggers/{triggerIdentifier}",
			RequiredParams: []string{"triggerIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier"},
		}
		mappings["triggers/execute"] = providers.OperationMapping{
			OperationID:    "executeTrigger",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipelines/triggers/{triggerIdentifier}/execute",
			RequiredParams: []string{"triggerIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "pipelineIdentifier"},
		}
		// Input Set operations
		// InputSet operations - always build for dynamic filtering
		mappings["inputsets/list"] = providers.OperationMapping{
			OperationID:    "listInputSets",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/{pipelineIdentifier}/inputsets",
			RequiredParams: []string{"pipelineIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "page", "limit"},
		}
		mappings["inputsets/get"] = providers.OperationMapping{
			OperationID:    "getInputSet",
			Method:         "GET",
			PathTemplate:   "/pipeline/api/pipelines/{pipelineIdentifier}/inputsets/{inputSetIdentifier}",
			RequiredParams: []string{"pipelineIdentifier", "inputSetIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["inputsets/create"] = providers.OperationMapping{
			OperationID:    "createInputSet",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipelines/{pipelineIdentifier}/inputsets",
			RequiredParams: []string{"pipelineIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["inputsets/update"] = providers.OperationMapping{
			OperationID:    "updateInputSet",
			Method:         "PUT",
			PathTemplate:   "/pipeline/api/pipelines/{pipelineIdentifier}/inputsets/{inputSetIdentifier}",
			RequiredParams: []string{"pipelineIdentifier", "inputSetIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["inputsets/delete"] = providers.OperationMapping{
			OperationID:    "deleteInputSet",
			Method:         "DELETE",
			PathTemplate:   "/pipeline/api/pipelines/{pipelineIdentifier}/inputsets/{inputSetIdentifier}",
			RequiredParams: []string{"pipelineIdentifier", "inputSetIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier"},
		}
		mappings["inputsets/merge"] = providers.OperationMapping{
			OperationID:    "mergeInputSets",
			Method:         "POST",
			PathTemplate:   "/pipeline/api/pipelines/{pipelineIdentifier}/inputsets/merge",
			RequiredParams: []string{"pipelineIdentifier"},
			OptionalParams: []string{"accountIdentifier", "orgIdentifier", "projectIdentifier", "inputSetIdentifiers"},
		}
		// Freeze Window operations
		// FreezeWindow operations - always build for dynamic filtering
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
		// Project operations - always build for dynamic filtering
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
		// Connector operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "connectors",
			DisplayName: "Connector Management",
			Description: "Configure connections to external services",
			Operations: []string{
				"connectors/list", "connectors/get", "connectors/create",
				"connectors/update", "connectors/delete", "connectors/validate",
			},
		})
		// GitOps operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "gitops",
			DisplayName: "GitOps Management",
			Description: "Manage GitOps agents and applications",
			Operations: []string{
				"gitops/agents/list", "gitops/applications/list",
				"gitops/applications/sync", "gitops/applications/rollback",
			},
		})
		// STO operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "security",
			DisplayName: "Security Testing",
			Description: "Security scanning and vulnerability management",
			Operations: []string{
				"sto/scans/list", "sto/vulnerabilities/list", "sto/exemptions/create",
			},
		})
		// CCM operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "cost",
			DisplayName: "Cloud Cost Management",
			Description: "Monitor and optimize cloud costs",
			Operations: []string{
				"ccm/costs/overview", "ccm/budgets/list",
				"ccm/recommendations/list", "ccm/anomalies/list",
			},
		})
		// Service and Deployment groups
		// Service operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "services",
			DisplayName: "Service Management",
			Description: "Manage application services and configurations",
			Operations: []string{
				"services/list", "services/get", "services/create",
				"services/update", "services/delete",
			},
		})
		// Environment operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "environments",
			DisplayName: "Environment Management",
			Description: "Configure deployment environments",
			Operations: []string{
				"environments/list", "environments/get", "environments/create",
				"environments/update", "environments/delete",
			},
		})
		// Infrastructure operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "infrastructure",
			DisplayName: "Infrastructure Definitions",
			Description: "Define and manage infrastructure configurations",
			Operations: []string{
				"infrastructures/list", "infrastructures/get", "infrastructures/create",
				"infrastructures/update", "infrastructures/delete",
			},
		})
		// Code and Repository groups
		// PullRequest operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "pullrequests",
			DisplayName: "Pull Request Management",
			Description: "Create and manage pull requests",
			Operations: []string{
				"pullrequests/list", "pullrequests/get", "pullrequests/create",
				"pullrequests/merge", "pullrequests/review",
			},
		})
		// Repository operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "repositories",
			DisplayName: "Repository Management",
			Description: "Manage code repositories",
			Operations: []string{
				"repositories/list", "repositories/get",
				"repositories/branches", "repositories/commits",
			},
		})
		// Registry operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "registries",
			DisplayName: "Artifact Registries",
			Description: "Manage artifact registries and images",
			Operations: []string{
				"registries/list", "registries/get", "registries/artifacts",
			},
		})
		// Monitoring and Analytics groups
		// Dashboard operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "dashboards",
			DisplayName: "Dashboards",
			Description: "View and manage dashboards",
			Operations: []string{
				"dashboards/list", "dashboards/get", "dashboards/data",
			},
		})
		// Chaos operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "chaos",
			DisplayName: "Chaos Engineering",
			Description: "Run chaos experiments for resilience testing",
			Operations: []string{
				"chaos/experiments/list", "chaos/experiments/get",
				"chaos/experiments/run", "chaos/experiments/results",
			},
		})
		// SSCA operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "ssca",
			DisplayName: "Supply Chain Security",
			Description: "Manage supply chain security and compliance",
			Operations: []string{
				"ssca/sbom/generate", "ssca/sbom/get",
				"ssca/artifacts/scan", "ssca/vulnerabilities/list",
			},
		})
		// Logs operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "logs",
			DisplayName: "Logs Management",
			Description: "Access and stream execution logs",
			Operations: []string{
				"logs/download", "logs/stream",
			},
		})
		// Platform and Configuration groups
		// Template operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "templates",
			DisplayName: "Template Management",
			Description: "Create and manage reusable templates",
			Operations: []string{
				"templates/list", "templates/get", "templates/create",
				"templates/update", "templates/delete",
			},
		})
		// IDP operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "idp",
			DisplayName: "Internal Developer Portal",
			Description: "Manage developer portal entities and scorecards",
			Operations: []string{
				"idp/entities/list", "idp/entities/get",
				"idp/scorecards/list", "idp/scorecards/get",
				"idp/catalog/list",
			},
		})
		// Audit operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "audit",
			DisplayName: "Audit Trail",
			Description: "Track user activities and changes",
			Operations: []string{
				"audit/events/list", "audit/events/get",
			},
		})
		// Database operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "database",
			DisplayName: "Database Operations",
			Description: "Manage database schemas and migrations",
			Operations: []string{
				"database/schema/list", "database/schema/get",
				"database/migrations/list",
			},
		})
		// Feature Management groups
		// Feature Flag operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "featureflags",
			DisplayName: "Feature Flags",
			Description: "Manage feature flags and targeting",
			Operations: []string{
				"featureflags/list", "featureflags/get", "featureflags/create",
				"featureflags/update", "featureflags/toggle",
			},
		})
		// CV operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "cv",
			DisplayName: "Continuous Verification",
			Description: "Monitor service health and SLOs",
			Operations: []string{
				"cv/monitored-services/list", "cv/monitored-services/get",
				"cv/health-sources/list", "cv/sli/list", "cv/slo/list",
			},
		})
		// IaCM operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "iacm",
			DisplayName: "Infrastructure as Code",
			Description: "Manage Terraform/OpenTofu workspaces",
			Operations: []string{
				"iacm/workspaces/list", "iacm/workspaces/get", "iacm/workspaces/create",
				"iacm/stacks/list", "iacm/cost-estimation",
			},
		})
		// Execution and Operations groups
		// Execution operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "executions",
			DisplayName: "Execution Management",
			Description: "Track and manage pipeline executions",
			Operations: []string{
				"executions/list", "executions/get", "executions/status",
				"executions/rollback", "executions/abort",
			},
		})
		// Secret operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "secrets",
			DisplayName: "Secrets Management",
			Description: "Manage secrets and credentials",
			Operations: []string{
				"secrets/list", "secrets/get", "secrets/create",
				"secrets/update", "secrets/delete",
			},
		})
		// User operations - always build for dynamic filtering
		groups = append(groups, providers.OperationGroup{
			Name:        "users",
			DisplayName: "User & Access Management",
			Description: "Manage users, groups, roles and permissions",
			Operations: []string{
				"users/list", "users/get",
				"usergroups/list", "usergroups/get",
				"roles/list", "roles/get",
				"permissions/list",
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
	result, err := p.Execute(ctx, operation, params)

	// Enhance error messages for common issues
	if err != nil {
		return nil, p.enhanceErrorMessage(err, operation, params)
	}

	return result, nil
}

// enhanceErrorMessage provides more helpful error messages for common Harness API errors
func (p *HarnessProvider) enhanceErrorMessage(err error, operation string, params map[string]interface{}) error {
	errMsg := err.Error()

	// Module not enabled errors
	if strings.Contains(errMsg, "Not Implemented") || strings.Contains(errMsg, "status:500") {
		moduleHints := map[string]string{
			"gitops": "GitOps module may not be enabled in your Harness account",
			"ccm":    "Cloud Cost Management (CCM) module may not be enabled in your Harness account",
			"sto":    "Security Testing Orchestration (STO) module may not be enabled in your Harness account",
			"chaos":  "Chaos Engineering module may not be enabled in your Harness account",
			"iacm":   "Infrastructure as Code Management module may not be enabled in your Harness account",
		}

		for module, hint := range moduleHints {
			if strings.Contains(operation, module) {
				return fmt.Errorf("%s. %s", err.Error(), hint)
			}
		}
	}

	// Project/Org not found errors
	if strings.Contains(errMsg, "not found") {
		if org, hasOrg := params["orgIdentifier"]; hasOrg {
			if project, hasProject := params["projectIdentifier"]; hasProject {
				return fmt.Errorf("%s. Please verify that org '%s' and project '%s' exist in your Harness account", err.Error(), org, project)
			}
		}
	}

	// Permission errors
	if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "Forbidden") {
		return fmt.Errorf("%s. Check that your API token has the necessary permissions for operation '%s'", err.Error(), operation)
	}

	// Token errors
	if strings.Contains(errMsg, "Token is not valid") || strings.Contains(errMsg, "401") {
		return fmt.Errorf("%s. Please verify your Harness API token is valid and not expired", err.Error())
	}

	return err
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
