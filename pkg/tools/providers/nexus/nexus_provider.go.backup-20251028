package nexus

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
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

// Embed Nexus OpenAPI spec as fallback
//
//go:embed nexus-openapi.json
var nexusOpenAPISpecJSON []byte

// NexusProvider implements the StandardToolProvider interface for Sonatype Nexus
type NexusProvider struct {
	*providers.BaseProvider
	specCache      repository.OpenAPICacheRepository // For caching the OpenAPI spec
	specFallback   *openapi3.T                       // Embedded fallback spec
	httpClient     *http.Client
	enabledModules map[string]bool // Module-based feature enablement
}

// NewNexusProvider creates a new Nexus provider instance
func NewNexusProvider(logger observability.Logger) *NexusProvider {
	base := providers.NewBaseProvider("nexus", "v1", "http://localhost:8081", logger)

	// Load embedded spec as fallback
	var specFallback *openapi3.T
	if len(nexusOpenAPISpecJSON) > 0 {
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(nexusOpenAPISpecJSON)
		if err == nil {
			specFallback = spec
		}
	}

	provider := &NexusProvider{
		BaseProvider: base,
		specFallback: specFallback,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		enabledModules: map[string]bool{
			"repositories": true,
			"components":   true,
			"assets":       true,
			"search":       true,
			"security":     true,
			"blobstores":   true,
			"cleanup":      true,
			"tasks":        true,
		},
	}

	// Set operation mappings in base provider
	provider.SetOperationMappings(provider.GetOperationMappings())
	// Set configuration to ensure auth type is configured
	provider.SetConfiguration(provider.GetDefaultConfiguration())
	return provider
}

// NewNexusProviderWithCache creates a new Nexus provider with spec caching
func NewNexusProviderWithCache(logger observability.Logger, specCache repository.OpenAPICacheRepository) *NexusProvider {
	provider := NewNexusProvider(logger)
	provider.specCache = specCache
	return provider
}

// SetBaseURL allows overriding the default base URL
func (p *NexusProvider) SetBaseURL(baseURL string) {
	config := p.GetDefaultConfiguration()
	config.BaseURL = baseURL
	if config.AuthType == "" {
		config.AuthType = "api_key" // Default to API key auth for tests
	}
	p.SetConfiguration(config)
}

// GetCurrentConfiguration returns the current configuration (after any modifications)
func (p *NexusProvider) GetCurrentConfiguration() providers.ProviderConfig {
	// BaseProvider.GetDefaultConfiguration actually returns the current config
	return p.BaseProvider.GetDefaultConfiguration()
}

// GetProviderName returns the provider name
func (p *NexusProvider) GetProviderName() string {
	return "nexus"
}

// GetSupportedVersions returns supported Nexus API versions
func (p *NexusProvider) GetSupportedVersions() []string {
	return []string{"v1", "3.83.1-03"}
}

// GetEnabledModules returns the list of enabled modules
func (p *NexusProvider) GetEnabledModules() []string {
	modules := make([]string, 0, len(p.enabledModules))
	for module, enabled := range p.enabledModules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}

// GetToolDefinitions returns Nexus-specific tool definitions
func (p *NexusProvider) GetToolDefinitions() []providers.ToolDefinition {
	tools := []providers.ToolDefinition{}

	// Repository Management tools
	if p.enabledModules["repositories"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_repositories",
				DisplayName: "Nexus Repository Management",
				Description: "Manage Nexus repositories (hosted, proxy, group)",
				Category:    "artifact_management",
				Operation: providers.OperationDef{
					ID:           "repositories",
					Method:       "GET",
					PathTemplate: "/v1/repositories",
				},
				Parameters: []providers.ParameterDef{
					{Name: "format", In: "query", Type: "string", Required: false, Description: "Repository format (maven2, npm, docker, etc.)"},
					{Name: "type", In: "query", Type: "string", Required: false, Description: "Repository type (hosted, proxy, group)"},
				},
			},
			{
				Name:        "nexus_repository_create",
				DisplayName: "Create Nexus Repository",
				Description: "Create a new repository in Nexus",
				Category:    "artifact_management",
				Operation: providers.OperationDef{
					ID:           "repository_create",
					Method:       "POST",
					PathTemplate: "/v1/repositories/{format}/{type}",
				},
				Parameters: []providers.ParameterDef{
					{Name: "format", In: "path", Type: "string", Required: true, Description: "Repository format"},
					{Name: "type", In: "path", Type: "string", Required: true, Description: "Repository type"},
				},
			},
		}...)
	}

	// Component management tools
	if p.enabledModules["components"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_components",
				DisplayName: "Nexus Components",
				Description: "List and manage components in repositories",
				Category:    "artifact_management",
				Operation: providers.OperationDef{
					ID:           "components",
					Method:       "GET",
					PathTemplate: "/v1/components",
				},
				Parameters: []providers.ParameterDef{
					{Name: "repository", In: "query", Type: "string", Required: true, Description: "Repository name"},
					{Name: "continuationToken", In: "query", Type: "string", Required: false, Description: "Pagination token"},
				},
			},
			{
				Name:        "nexus_component_upload",
				DisplayName: "Upload Component",
				Description: "Upload a component to a repository",
				Category:    "artifact_management",
				Operation: providers.OperationDef{
					ID:           "component_upload",
					Method:       "POST",
					PathTemplate: "/v1/components",
				},
				Parameters: []providers.ParameterDef{
					{Name: "repository", In: "query", Type: "string", Required: true, Description: "Target repository"},
				},
			},
		}...)
	}

	// Asset management tools
	if p.enabledModules["assets"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_assets",
				DisplayName: "Nexus Assets",
				Description: "List and manage assets in repositories",
				Category:    "artifact_management",
				Operation: providers.OperationDef{
					ID:           "assets",
					Method:       "GET",
					PathTemplate: "/v1/assets",
				},
				Parameters: []providers.ParameterDef{
					{Name: "repository", In: "query", Type: "string", Required: true, Description: "Repository name"},
					{Name: "continuationToken", In: "query", Type: "string", Required: false, Description: "Pagination token"},
				},
			},
		}...)
	}

	// Search tools
	if p.enabledModules["search"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_search",
				DisplayName: "Search Nexus",
				Description: "Search for components across repositories",
				Category:    "search",
				Operation: providers.OperationDef{
					ID:           "search",
					Method:       "GET",
					PathTemplate: "/v1/search",
				},
				Parameters: []providers.ParameterDef{
					{Name: "q", In: "query", Type: "string", Required: false, Description: "Query string"},
					{Name: "repository", In: "query", Type: "string", Required: false, Description: "Repository to search"},
					{Name: "format", In: "query", Type: "string", Required: false, Description: "Component format"},
					{Name: "group", In: "query", Type: "string", Required: false, Description: "Component group"},
					{Name: "name", In: "query", Type: "string", Required: false, Description: "Component name"},
					{Name: "version", In: "query", Type: "string", Required: false, Description: "Component version"},
				},
			},
			{
				Name:        "nexus_search_assets",
				DisplayName: "Search Nexus Assets",
				Description: "Search for assets across repositories",
				Category:    "search",
				Operation: providers.OperationDef{
					ID:           "search_assets",
					Method:       "GET",
					PathTemplate: "/v1/search/assets",
				},
				Parameters: []providers.ParameterDef{
					{Name: "q", In: "query", Type: "string", Required: false, Description: "Query string"},
					{Name: "repository", In: "query", Type: "string", Required: false, Description: "Repository to search"},
					{Name: "format", In: "query", Type: "string", Required: false, Description: "Asset format"},
				},
			},
		}...)
	}

	// Security management tools
	if p.enabledModules["security"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_users",
				DisplayName: "Nexus Users",
				Description: "Manage Nexus users and permissions",
				Category:    "security",
				Operation: providers.OperationDef{
					ID:           "users",
					Method:       "GET",
					PathTemplate: "/v1/security/users",
				},
				Parameters: []providers.ParameterDef{
					{Name: "userId", In: "query", Type: "string", Required: false, Description: "Filter by user ID"},
					{Name: "source", In: "query", Type: "string", Required: false, Description: "User source"},
				},
			},
			{
				Name:        "nexus_roles",
				DisplayName: "Nexus Roles",
				Description: "Manage security roles",
				Category:    "security",
				Operation: providers.OperationDef{
					ID:           "roles",
					Method:       "GET",
					PathTemplate: "/v1/security/roles",
				},
				Parameters: []providers.ParameterDef{
					{Name: "source", In: "query", Type: "string", Required: false, Description: "Role source"},
				},
			},
			{
				Name:        "nexus_privileges",
				DisplayName: "Nexus Privileges",
				Description: "Manage security privileges",
				Category:    "security",
				Operation: providers.OperationDef{
					ID:           "privileges",
					Method:       "GET",
					PathTemplate: "/v1/security/privileges",
				},
				Parameters: []providers.ParameterDef{},
			},
		}...)
	}

	// Blob store management
	if p.enabledModules["blobstores"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_blobstores",
				DisplayName: "Nexus Blob Stores",
				Description: "Manage blob stores for artifact storage",
				Category:    "storage",
				Operation: providers.OperationDef{
					ID:           "blobstores",
					Method:       "GET",
					PathTemplate: "/v1/blobstores",
				},
				Parameters: []providers.ParameterDef{},
			},
		}...)
	}

	// Tasks management
	if p.enabledModules["tasks"] {
		tools = append(tools, []providers.ToolDefinition{
			{
				Name:        "nexus_tasks",
				DisplayName: "Nexus Tasks",
				Description: "Manage and monitor system tasks",
				Category:    "administration",
				Operation: providers.OperationDef{
					ID:           "tasks",
					Method:       "GET",
					PathTemplate: "/v1/tasks",
				},
				Parameters: []providers.ParameterDef{
					{Name: "type", In: "query", Type: "string", Required: false, Description: "Task type filter"},
				},
			},
		}...)
	}

	return tools
}

// ValidateCredentials validates Nexus credentials
func (p *NexusProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// Nexus supports multiple auth methods
	username, hasUsername := creds["username"]
	password, hasPassword := creds["password"]
	apiKey, hasAPIKey := creds["api_key"]
	token, hasToken := creds["token"]

	// Build auth header based on available credentials
	var authHeader string
	if hasAPIKey {
		// NX-API-KEY auth
		authHeader = "NX-APIKEY " + apiKey
	} else if hasToken {
		// Bearer token auth
		authHeader = "Bearer " + token
	} else if hasUsername && hasPassword {
		// Basic auth
		auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		authHeader = "Basic " + auth
	} else {
		return fmt.Errorf("missing required credentials: provide api_key, token, or username/password")
	}

	// Test the credentials with a simple API call
	config := p.GetCurrentConfiguration()
	req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/v1/status", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid Nexus credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response from Nexus API: %d", resp.StatusCode)
	}

	return nil
}

// GetOperationMappings returns Nexus-specific operation mappings
func (p *NexusProvider) GetOperationMappings() map[string]providers.OperationMapping {
	mappings := map[string]providers.OperationMapping{
		// Repository operations
		"repositories/list": {
			OperationID:    "getRepositories_6",
			Method:         "GET",
			PathTemplate:   "/v1/repositories",
			RequiredParams: []string{},
			OptionalParams: []string{},
		},
		"repositories/get": {
			OperationID:    "getRepository_7",
			Method:         "GET",
			PathTemplate:   "/v1/repositories/{repositoryName}",
			RequiredParams: []string{"repositoryName"},
		},
		"repositories/delete": {
			OperationID:    "deleteRepository",
			Method:         "DELETE",
			PathTemplate:   "/v1/repositories/{repositoryName}",
			RequiredParams: []string{"repositoryName"},
		},

		// Component operations
		"components/list": {
			OperationID:    "getComponents",
			Method:         "GET",
			PathTemplate:   "/v1/components",
			RequiredParams: []string{"repository"},
			OptionalParams: []string{"continuationToken"},
		},
		"components/get": {
			OperationID:    "getComponentById",
			Method:         "GET",
			PathTemplate:   "/v1/components/{id}",
			RequiredParams: []string{"id"},
		},
		"components/upload": {
			OperationID:    "uploadComponent",
			Method:         "POST",
			PathTemplate:   "/v1/components",
			RequiredParams: []string{"repository"},
		},
		"components/delete": {
			OperationID:    "deleteComponent",
			Method:         "DELETE",
			PathTemplate:   "/v1/components/{id}",
			RequiredParams: []string{"id"},
		},

		// Asset operations
		"assets/list": {
			OperationID:    "getAssets",
			Method:         "GET",
			PathTemplate:   "/v1/assets",
			RequiredParams: []string{"repository"},
			OptionalParams: []string{"continuationToken"},
		},
		"assets/get": {
			OperationID:    "getAssetById",
			Method:         "GET",
			PathTemplate:   "/v1/assets/{id}",
			RequiredParams: []string{"id"},
		},
		"assets/delete": {
			OperationID:    "deleteAsset",
			Method:         "DELETE",
			PathTemplate:   "/v1/assets/{id}",
			RequiredParams: []string{"id"},
		},

		// Search operations
		"search/components": {
			OperationID:    "searchComponents",
			Method:         "GET",
			PathTemplate:   "/v1/search",
			RequiredParams: []string{},
			OptionalParams: []string{"q", "repository", "format", "group", "name", "version", "continuationToken"},
		},
		"search/assets": {
			OperationID:    "searchAssets",
			Method:         "GET",
			PathTemplate:   "/v1/search/assets",
			RequiredParams: []string{},
			OptionalParams: []string{"q", "repository", "format", "group", "name", "continuationToken"},
		},

		// Security: User operations
		"users/list": {
			OperationID:    "getUsers",
			Method:         "GET",
			PathTemplate:   "/v1/security/users",
			RequiredParams: []string{},
			OptionalParams: []string{"userId", "source"},
		},
		"users/create": {
			OperationID:    "createUser",
			Method:         "POST",
			PathTemplate:   "/v1/security/users",
			RequiredParams: []string{},
		},
		"users/update": {
			OperationID:    "updateUser",
			Method:         "PUT",
			PathTemplate:   "/v1/security/users/{userId}",
			RequiredParams: []string{"userId"},
		},
		"users/delete": {
			OperationID:    "deleteUser",
			Method:         "DELETE",
			PathTemplate:   "/v1/security/users/{userId}",
			RequiredParams: []string{"userId"},
		},

		// Security: Role operations
		"roles/list": {
			OperationID:    "getRoles",
			Method:         "GET",
			PathTemplate:   "/v1/security/roles",
			RequiredParams: []string{},
			OptionalParams: []string{"source"},
		},
		"roles/get": {
			OperationID:    "getRole",
			Method:         "GET",
			PathTemplate:   "/v1/security/roles/{id}",
			RequiredParams: []string{"id"},
		},
		"roles/create": {
			OperationID:    "create",
			Method:         "POST",
			PathTemplate:   "/v1/security/roles",
			RequiredParams: []string{},
		},
		"roles/update": {
			OperationID:    "update",
			Method:         "PUT",
			PathTemplate:   "/v1/security/roles/{id}",
			RequiredParams: []string{"id"},
		},
		"roles/delete": {
			OperationID:    "delete",
			Method:         "DELETE",
			PathTemplate:   "/v1/security/roles/{id}",
			RequiredParams: []string{"id"},
		},

		// Security: Privilege operations
		"privileges/list": {
			OperationID:    "getPrivileges",
			Method:         "GET",
			PathTemplate:   "/v1/security/privileges",
			RequiredParams: []string{},
		},
		"privileges/get": {
			OperationID:    "getPrivilege",
			Method:         "GET",
			PathTemplate:   "/v1/security/privileges/{privilegeName}",
			RequiredParams: []string{"privilegeName"},
		},
		"privileges/delete": {
			OperationID:    "deletePrivilege",
			Method:         "DELETE",
			PathTemplate:   "/v1/security/privileges/{privilegeName}",
			RequiredParams: []string{"privilegeName"},
		},

		// Blob store operations
		"blobstores/list": {
			OperationID:    "listBlobStores",
			Method:         "GET",
			PathTemplate:   "/v1/blobstores",
			RequiredParams: []string{},
		},
		"blobstores/get": {
			OperationID:    "getBlobStore",
			Method:         "GET",
			PathTemplate:   "/v1/blobstores/{name}",
			RequiredParams: []string{"name"},
		},
		"blobstores/delete": {
			OperationID:    "deleteBlobStore",
			Method:         "DELETE",
			PathTemplate:   "/v1/blobstores/{name}",
			RequiredParams: []string{"name"},
		},

		// Task operations
		"tasks/list": {
			OperationID:    "getTasks",
			Method:         "GET",
			PathTemplate:   "/v1/tasks",
			RequiredParams: []string{},
			OptionalParams: []string{"type"},
		},
		"tasks/get": {
			OperationID:    "getTaskById",
			Method:         "GET",
			PathTemplate:   "/v1/tasks/{id}",
			RequiredParams: []string{"id"},
		},
		"tasks/run": {
			OperationID:    "run",
			Method:         "POST",
			PathTemplate:   "/v1/tasks/{id}/run",
			RequiredParams: []string{"id"},
		},
		"tasks/stop": {
			OperationID:    "stop",
			Method:         "POST",
			PathTemplate:   "/v1/tasks/{id}/stop",
			RequiredParams: []string{"id"},
		},

		// Cleanup policy operations
		"cleanup/list": {
			OperationID:    "getAll",
			Method:         "GET",
			PathTemplate:   "/v1/cleanup-policies",
			RequiredParams: []string{},
		},
		"cleanup/get": {
			OperationID:    "get",
			Method:         "GET",
			PathTemplate:   "/v1/cleanup-policies/{policyName}",
			RequiredParams: []string{"policyName"},
		},
		"cleanup/create": {
			OperationID:    "create_5",
			Method:         "POST",
			PathTemplate:   "/v1/cleanup-policies",
			RequiredParams: []string{},
		},
		"cleanup/update": {
			OperationID:    "update_5",
			Method:         "PUT",
			PathTemplate:   "/v1/cleanup-policies/{policyName}",
			RequiredParams: []string{"policyName"},
		},
		"cleanup/delete": {
			OperationID:    "delete_6",
			Method:         "DELETE",
			PathTemplate:   "/v1/cleanup-policies/{policyName}",
			RequiredParams: []string{"policyName"},
		},
	}

	// Add format-specific repository create operations
	formats := []string{"maven", "npm", "docker", "nuget", "pypi", "raw", "rubygems", "helm", "apt", "yum"}
	types := []string{"hosted", "proxy", "group"}

	for _, format := range formats {
		for _, repoType := range types {
			key := fmt.Sprintf("repositories/create/%s/%s", format, repoType)
			mappings[key] = providers.OperationMapping{
				OperationID:    fmt.Sprintf("createRepository_%s_%s", format, repoType),
				Method:         "POST",
				PathTemplate:   fmt.Sprintf("/v1/repositories/%s/%s", format, repoType),
				RequiredParams: []string{},
			}
		}
	}

	return mappings
}

// GetDefaultConfiguration returns default Nexus configuration
func (p *NexusProvider) GetDefaultConfiguration() providers.ProviderConfig {
	return providers.ProviderConfig{
		BaseURL:  "http://localhost:8081/service/rest",
		AuthType: "basic", // Can be "basic", "bearer", or "api_key"
		DefaultHeaders: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 120,
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnRateLimit: true,
		},
		HealthEndpoint: "/v1/status",
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "repositories",
				DisplayName: "Repository Management",
				Description: "Operations for managing repositories",
				Operations:  []string{"repositories/list", "repositories/get", "repositories/delete"},
			},
			{
				Name:        "components",
				DisplayName: "Component Management",
				Description: "Operations for managing components",
				Operations:  []string{"components/list", "components/get", "components/upload", "components/delete"},
			},
			{
				Name:        "assets",
				DisplayName: "Asset Management",
				Description: "Operations for managing assets",
				Operations:  []string{"assets/list", "assets/get", "assets/delete"},
			},
			{
				Name:        "search",
				DisplayName: "Search",
				Description: "Search operations",
				Operations:  []string{"search/components", "search/assets"},
			},
			{
				Name:        "security",
				DisplayName: "Security Management",
				Description: "User, role, and privilege management",
				Operations:  []string{"users/list", "users/create", "roles/list", "roles/create", "privileges/list"},
			},
			{
				Name:        "administration",
				DisplayName: "Administration",
				Description: "System administration tasks",
				Operations:  []string{"tasks/list", "tasks/run", "blobstores/list", "cleanup/list"},
			},
		},
	}
}

// GetAIOptimizedDefinitions returns AI-friendly tool definitions
func (p *NexusProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return []providers.AIOptimizedToolDefinition{
		{
			Name:        "nexus_repositories",
			DisplayName: "Nexus Repository Management",
			Category:    "Artifact Management",
			Description: "Manage Nexus repositories including Maven, npm, Docker, NuGet, PyPI, and more",
			UsageExamples: []providers.Example{
				{
					Scenario: "List all repositories",
					Input: map[string]interface{}{
						"action": "list",
					},
				},
				{
					Scenario: "Get details of a specific repository",
					Input: map[string]interface{}{
						"action":         "get",
						"repositoryName": "maven-central",
					},
				},
				{
					Scenario: "Create a new Maven hosted repository",
					Input: map[string]interface{}{
						"action": "create",
						"format": "maven",
						"type":   "hosted",
						"parameters": map[string]interface{}{
							"name": "my-maven-repo",
							"maven": map[string]interface{}{
								"versionPolicy": "RELEASE",
								"layoutPolicy":  "STRICT",
							},
						},
					},
				},
			},
			SemanticTags:  []string{"repository", "artifact", "package", "maven", "npm", "docker", "nexus"},
			CommonPhrases: []string{"create repository", "list artifacts", "upload component"},
		},
		{
			Name:        "nexus_components",
			DisplayName: "Nexus Components",
			Category:    "Artifact Management",
			Description: "Manage components (artifacts) in Nexus repositories",
			UsageExamples: []providers.Example{
				{
					Scenario: "List components in a repository",
					Input: map[string]interface{}{
						"action":     "list",
						"repository": "maven-releases",
					},
				},
				{
					Scenario: "Upload a component to a repository",
					Input: map[string]interface{}{
						"action":     "upload",
						"repository": "maven-releases",
						"parameters": map[string]interface{}{
							"groupId":    "com.example",
							"artifactId": "my-app",
							"version":    "1.0.0",
						},
					},
				},
				{
					Scenario: "Delete a specific component",
					Input: map[string]interface{}{
						"action": "delete",
						"id":     "component-id-123",
					},
				},
			},
			SemanticTags: []string{"component", "artifact", "package", "jar", "war", "pom"},
		},
		{
			Name:        "nexus_search",
			DisplayName: "Nexus Search",
			Category:    "Search",
			Description: "Search for components and assets across Nexus repositories",
			UsageExamples: []providers.Example{
				{
					Scenario: "Search for components by name",
					Input: map[string]interface{}{
						"action": "components",
						"parameters": map[string]interface{}{
							"name": "spring-boot",
						},
					},
				},
				{
					Scenario: "Search for components in a specific repository",
					Input: map[string]interface{}{
						"action": "components",
						"parameters": map[string]interface{}{
							"repository": "maven-central",
							"group":      "org.springframework.boot",
						},
					},
				},
				{
					Scenario: "Search for assets by format",
					Input: map[string]interface{}{
						"action": "assets",
						"parameters": map[string]interface{}{
							"format": "docker",
							"q":      "nginx",
						},
					},
				},
			},
			SemanticTags: []string{"search", "find", "query", "lookup", "discover"},
		},
		{
			Name:        "nexus_security",
			DisplayName: "Nexus Security Management",
			Category:    "Security",
			Description: "Manage users, roles, and privileges in Nexus",
			UsageExamples: []providers.Example{
				{
					Scenario: "List all users",
					Input: map[string]interface{}{
						"action": "users/list",
					},
				},
				{
					Scenario: "Create a new user",
					Input: map[string]interface{}{
						"action": "users/create",
						"parameters": map[string]interface{}{
							"userId":       "john.doe",
							"firstName":    "John",
							"lastName":     "Doe",
							"emailAddress": "john.doe@example.com",
							"roles":        []string{"nx-developer"},
						},
					},
				},
				{
					Scenario: "List security roles",
					Input: map[string]interface{}{
						"action": "roles/list",
						"source": "internal",
					},
				},
			},
			SemanticTags: []string{"security", "user", "role", "privilege", "permission", "access"},
		},
	}
}

// ExecuteOperation executes a Nexus operation
func (p *NexusProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize operation name (handle different formats)
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	// Use base provider's execution
	return p.Execute(ctx, operation, params)
}

// Execute overrides BaseProvider's Execute to handle Nexus-specific responses
func (p *NexusProvider) Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	mapping, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("operation %s not found", operation)
	}

	// Build path with parameters
	path := mapping.PathTemplate
	queryParams := make(map[string]string)

	// Replace path parameters
	for _, param := range mapping.RequiredParams {
		if value, ok := params[param]; ok {
			placeholder := "{" + param + "}"
			if strings.Contains(path, placeholder) {
				path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", value))
			} else {
				// If not a path param, add to query params for GET
				if mapping.Method == "GET" || mapping.Method == "HEAD" {
					queryParams[param] = fmt.Sprintf("%v", value)
				}
			}
		}
	}

	// Add optional parameters as query params for GET requests
	if mapping.Method == "GET" || mapping.Method == "HEAD" {
		for _, param := range mapping.OptionalParams {
			if value, ok := params[param]; ok {
				queryParams[param] = fmt.Sprintf("%v", value)
			}
		}
	}

	// Build query string
	if len(queryParams) > 0 {
		query := make([]string, 0, len(queryParams))
		for k, v := range queryParams {
			query = append(query, fmt.Sprintf("%s=%s", k, v))
		}
		path = path + "?" + strings.Join(query, "&")
	}

	// Prepare body for POST/PUT/PATCH methods
	var body interface{}
	if mapping.Method == "POST" || mapping.Method == "PUT" || mapping.Method == "PATCH" {
		// For Nexus, the body is typically the entire params minus path parameters
		bodyParams := make(map[string]interface{})
		for k, v := range params {
			// Skip path parameters
			isPathParam := false
			for _, pathParam := range mapping.RequiredParams {
				if strings.Contains(mapping.PathTemplate, "{"+pathParam+"}") && k == pathParam {
					isPathParam = true
					break
				}
			}
			if !isPathParam {
				bodyParams[k] = v
			}
		}
		if len(bodyParams) > 0 {
			body = bodyParams
		}
	}

	// Execute HTTP request using BaseProvider
	resp, err := p.ExecuteHTTPRequest(ctx, mapping.Method, path, body, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle 204 No Content responses
	if resp.StatusCode == http.StatusNoContent {
		return map[string]interface{}{"success": true, "status": 204}, nil
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nexus API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle empty response body
	if len(responseBody) == 0 {
		return map[string]interface{}{"success": true, "status": resp.StatusCode}, nil
	}

	// Parse JSON response
	var result interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		// If it's not JSON, return as string
		return map[string]interface{}{
			"data":   string(responseBody),
			"status": resp.StatusCode,
		}, nil
	}

	return result, nil
}

// normalizeOperationName normalizes operation names to handle different formats
func (p *NexusProvider) normalizeOperationName(operation string) string {
	// Handle different separators
	normalized := strings.ReplaceAll(operation, "-", "/")
	normalized = strings.ReplaceAll(normalized, "_", "/")

	// If it already has a resource prefix, return it
	if strings.Contains(normalized, "/") {
		return normalized
	}

	// Map simple operations to their full form
	simpleActions := map[string]string{
		"list":   "repositories/list",
		"get":    "repositories/get",
		"create": "repositories/create",
		"update": "repositories/update",
		"delete": "repositories/delete",
		"search": "search/components",
		"upload": "components/upload",
	}

	if defaultOp, ok := simpleActions[normalized]; ok {
		return defaultOp
	}

	return normalized
}

// GetOpenAPISpec returns the OpenAPI specification for Nexus
func (p *NexusProvider) GetOpenAPISpec() (*openapi3.T, error) {
	ctx := context.Background()

	// Try cache first if available
	if p.specCache != nil {
		spec, err := p.specCache.Get(ctx, "nexus-v1")
		if err == nil && spec != nil {
			return spec, nil
		}
	}

	// Use embedded fallback if available
	if p.specFallback != nil {
		return p.specFallback, nil
	}

	// Try fetching from Nexus if it supports OpenAPI endpoint
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	spec, err := p.fetchAndCacheSpec(fetchCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	return spec, nil
}

// fetchAndCacheSpec fetches the OpenAPI spec from Nexus and caches it
func (p *NexusProvider) fetchAndCacheSpec(ctx context.Context) (*openapi3.T, error) {
	// Nexus doesn't typically expose OpenAPI directly, use embedded spec
	if p.specFallback != nil {
		// Cache the spec if cache is available
		if p.specCache != nil {
			_ = p.specCache.Set(ctx, "nexus-v1", p.specFallback, 24*time.Hour)
		}
		return p.specFallback, nil
	}

	return nil, fmt.Errorf("no OpenAPI spec available")
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *NexusProvider) GetEmbeddedSpecVersion() string {
	return "3.83.1-03"
}

// HealthCheck verifies the Nexus API is accessible
func (p *NexusProvider) HealthCheck(ctx context.Context) error {
	config := p.GetCurrentConfiguration()
	req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+config.HealthEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nexus API health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nexus API health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *NexusProvider) Close() error {
	// Currently no resources to clean up
	return nil
}

// FilterOperationsByPermissions filters operations based on user permissions
func (p *NexusProvider) FilterOperationsByPermissions(operations []string, permissions map[string]interface{}) []string {
	// Extract user privileges from permissions
	privileges, ok := permissions["privileges"].([]string)
	if !ok {
		// No privileges info, return all operations
		return operations
	}

	// Map Nexus privileges to allowed operations
	allowedOps := make(map[string]bool)

	for _, priv := range privileges {
		switch {
		case strings.Contains(priv, "repository-view"):
			// Read access to repositories
			allowedOps["repositories/list"] = true
			allowedOps["repositories/get"] = true
			allowedOps["components/list"] = true
			allowedOps["components/get"] = true
			allowedOps["assets/list"] = true
			allowedOps["assets/get"] = true
			allowedOps["search/components"] = true
			allowedOps["search/assets"] = true

		case strings.Contains(priv, "repository-admin"):
			// Full repository management
			allowedOps["repositories/list"] = true
			allowedOps["repositories/get"] = true
			allowedOps["repositories/create"] = true
			allowedOps["repositories/delete"] = true
			allowedOps["components/list"] = true
			allowedOps["components/get"] = true
			allowedOps["components/upload"] = true
			allowedOps["components/delete"] = true
			allowedOps["assets/list"] = true
			allowedOps["assets/get"] = true
			allowedOps["assets/delete"] = true

		case strings.Contains(priv, "security-admin"):
			// Security administration
			allowedOps["users/list"] = true
			allowedOps["users/create"] = true
			allowedOps["users/update"] = true
			allowedOps["users/delete"] = true
			allowedOps["roles/list"] = true
			allowedOps["roles/create"] = true
			allowedOps["roles/update"] = true
			allowedOps["roles/delete"] = true
			allowedOps["privileges/list"] = true
			allowedOps["privileges/get"] = true

		case strings.Contains(priv, "nexus:*"):
			// Admin - all operations
			return operations
		}
	}

	// Filter operations based on allowed set
	filtered := []string{}
	for _, op := range operations {
		normalized := p.normalizeOperationName(op)
		if allowedOps[normalized] {
			filtered = append(filtered, op)
		}
	}

	return filtered
}
