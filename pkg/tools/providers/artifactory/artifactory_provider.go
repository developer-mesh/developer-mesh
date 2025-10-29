package artifactory

import (
	"context"
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

// ArtifactoryProvider implements the StandardToolProvider interface for JFrog Artifactory
type ArtifactoryProvider struct {
	*providers.BaseProvider
	specCache            repository.OpenAPICacheRepository // For caching the OpenAPI spec
	httpClient           *http.Client
	permissionDiscoverer *ArtifactoryPermissionDiscoverer // Permission discovery integration
	capabilityDiscoverer *CapabilityDiscoverer            // Capability reporting
}

// NewArtifactoryProvider creates a new Artifactory provider instance
func NewArtifactoryProvider(logger observability.Logger) *ArtifactoryProvider {
	// Defensive nil check for logger
	if logger == nil {
		// Create a no-op logger if none provided
		logger = &observability.NoopLogger{}
	}

	// Default to cloud instance, can be overridden via configuration
	base := providers.NewBaseProvider("artifactory", "v2", "https://mycompany.jfrog.io/artifactory", logger)

	provider := &ArtifactoryProvider{
		BaseProvider: base,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for large artifact operations
		},
		permissionDiscoverer: NewArtifactoryPermissionDiscoverer(logger, base.GetDefaultConfiguration().BaseURL),
		capabilityDiscoverer: NewCapabilityDiscoverer(logger),
	}

	// Set operation mappings in base provider
	provider.SetOperationMappings(provider.getAllOperationMappings())
	// Set configuration to ensure auth type is configured
	provider.SetConfiguration(provider.GetDefaultConfiguration())
	return provider
}

// NewArtifactoryProviderWithCache creates a new Artifactory provider with spec caching
func NewArtifactoryProviderWithCache(logger observability.Logger, specCache repository.OpenAPICacheRepository) *ArtifactoryProvider {
	// NewArtifactoryProvider handles nil logger check
	provider := NewArtifactoryProvider(logger)
	// Allow nil specCache - it's optional
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *ArtifactoryProvider) GetProviderName() string {
	return "artifactory"
}

// GetSupportedVersions returns supported Artifactory API versions
func (p *ArtifactoryProvider) GetSupportedVersions() []string {
	return []string{"v1", "v2"}
}

// GetToolDefinitions returns Artifactory-specific tool definitions
func (p *ArtifactoryProvider) GetToolDefinitions() []providers.ToolDefinition {
	// Defensive nil check
	if p == nil {
		return nil
	}

	return []providers.ToolDefinition{
		{
			Name:        "artifactory_repositories",
			DisplayName: "Artifactory Repositories",
			Description: "Manage Artifactory repositories",
			Category:    "repository_management",
		},
		{
			Name:        "artifactory_artifacts",
			DisplayName: "Artifactory Artifacts",
			Description: "Manage artifacts and packages",
			Category:    "artifact_management",
		},
		{
			Name:        "artifactory_builds",
			DisplayName: "Artifactory Builds",
			Description: "Manage build information",
			Category:    "ci_cd",
		},
		{
			Name:        "artifactory_users",
			DisplayName: "Artifactory Users",
			Description: "Manage users and permissions",
			Category:    "security",
		},
	}
}

// GetOperationMappings returns Artifactory-specific operation mappings
func (p *ArtifactoryProvider) GetOperationMappings() map[string]providers.OperationMapping {
	// Defensive nil check
	if p == nil {
		return nil
	}

	// Return all operations
	return p.getAllOperationMappings()
}

// getAllOperationMappings returns all Artifactory operation mappings (unfiltered)
func (p *ArtifactoryProvider) getAllOperationMappings() map[string]providers.OperationMapping {
	// Defensive nil check
	if p == nil {
		return nil
	}

	// Start with base operations
	operations := map[string]providers.OperationMapping{
		// Repository operations
		"repos/list": {
			OperationID:    "listRepositories",
			Method:         "GET",
			PathTemplate:   "/api/repositories",
			RequiredParams: []string{},
			OptionalParams: []string{"type", "packageType"},
		},
		"repos/get": {
			OperationID:    "getRepository",
			Method:         "GET",
			PathTemplate:   "/api/repositories/{repoKey}",
			RequiredParams: []string{"repoKey"},
		},
		"repos/create": {
			OperationID:    "createRepository",
			Method:         "PUT",
			PathTemplate:   "/api/repositories/{repoKey}",
			RequiredParams: []string{"repoKey", "rclass"},
			OptionalParams: []string{"packageType", "description", "notes", "includesPattern", "excludesPattern", "repoLayoutRef"},
		},
		"repos/update": {
			OperationID:    "updateRepository",
			Method:         "POST",
			PathTemplate:   "/api/repositories/{repoKey}",
			RequiredParams: []string{"repoKey"},
			OptionalParams: []string{"description", "notes", "includesPattern", "excludesPattern"},
		},
		"repos/delete": {
			OperationID:    "deleteRepository",
			Method:         "DELETE",
			PathTemplate:   "/api/repositories/{repoKey}",
			RequiredParams: []string{"repoKey"},
		},

		// Artifact operations
		"artifacts/upload": {
			OperationID:    "uploadArtifact",
			Method:         "PUT",
			PathTemplate:   "/{repoKey}/{itemPath}",
			RequiredParams: []string{"repoKey", "itemPath"},
			OptionalParams: []string{"properties"},
		},
		"artifacts/download": {
			OperationID:    "downloadArtifact",
			Method:         "GET",
			PathTemplate:   "/{repoKey}/{itemPath}",
			RequiredParams: []string{"repoKey", "itemPath"},
		},
		"artifacts/info": {
			OperationID:    "getArtifactInfo",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{itemPath}",
			RequiredParams: []string{"repoKey", "itemPath"},
		},
		"artifacts/copy": {
			OperationID:    "copyArtifact",
			Method:         "POST",
			PathTemplate:   "/api/copy/{srcRepoKey}/{srcItemPath}?to={targetRepoKey}/{targetItemPath}",
			RequiredParams: []string{"srcRepoKey", "srcItemPath", "targetRepoKey", "targetItemPath"},
			OptionalParams: []string{"dry", "suppressLayouts", "failFast"},
		},
		"artifacts/move": {
			OperationID:    "moveArtifact",
			Method:         "POST",
			PathTemplate:   "/api/move/{srcRepoKey}/{srcItemPath}?to={targetRepoKey}/{targetItemPath}",
			RequiredParams: []string{"srcRepoKey", "srcItemPath", "targetRepoKey", "targetItemPath"},
			OptionalParams: []string{"dry", "suppressLayouts", "failFast"},
		},
		"artifacts/delete": {
			OperationID:    "deleteArtifact",
			Method:         "DELETE",
			PathTemplate:   "/{repoKey}/{itemPath}",
			RequiredParams: []string{"repoKey", "itemPath"},
		},
		"artifacts/properties/set": {
			OperationID:    "setArtifactProperties",
			Method:         "PUT",
			PathTemplate:   "/api/storage/{repoKey}/{itemPath}?properties={properties}",
			RequiredParams: []string{"repoKey", "itemPath", "properties"},
			OptionalParams: []string{"recursive"},
		},
		"artifacts/properties/delete": {
			OperationID:    "deleteArtifactProperties",
			Method:         "DELETE",
			PathTemplate:   "/api/storage/{repoKey}/{itemPath}?properties={properties}",
			RequiredParams: []string{"repoKey", "itemPath", "properties"},
			OptionalParams: []string{"recursive"},
		},

		// Search operations are replaced below after basic operations

		// Build operations
		"builds/list": {
			OperationID:    "listBuilds",
			Method:         "GET",
			PathTemplate:   "/api/build",
			RequiredParams: []string{},
			OptionalParams: []string{"project"},
		},
		"builds/get": {
			OperationID:    "getBuildInfo",
			Method:         "GET",
			PathTemplate:   "/api/build/{buildName}/{buildNumber}",
			RequiredParams: []string{"buildName", "buildNumber"},
			OptionalParams: []string{"project"},
		},
		"builds/runs": {
			OperationID:    "getBuildRuns",
			Method:         "GET",
			PathTemplate:   "/api/build/{buildName}",
			RequiredParams: []string{"buildName"},
			OptionalParams: []string{"project"},
		},
		"builds/upload": {
			OperationID:    "uploadBuildInfo",
			Method:         "PUT",
			PathTemplate:   "/api/build",
			RequiredParams: []string{"buildInfo"},
			OptionalParams: []string{"project"},
		},
		"builds/promote": {
			OperationID:    "promoteBuild",
			Method:         "POST",
			PathTemplate:   "/api/build/promote/{buildName}/{buildNumber}",
			RequiredParams: []string{"buildName", "buildNumber", "targetRepo"},
			OptionalParams: []string{"status", "comment", "ciUser", "timestamp", "copy", "dependencies", "scopes", "properties", "failFast", "project"},
		},
		"builds/delete": {
			OperationID:    "deleteBuild",
			Method:         "DELETE",
			PathTemplate:   "/api/build/{buildName}",
			RequiredParams: []string{"buildName"},
			OptionalParams: []string{"buildNumbers", "artifacts", "project", "deleteAll"},
		},

		// User operations

		// Group operations

		// Permission operations

		// Token operations

		// Project operations - Enterprise/Pro feature
		// Note: Projects API is available at /access/api/v1/projects
		// Requires Platform Pro or Enterprise license

		// Project membership operations

		// Project group operations

		// Project roles operations

		// Project-scoped repository operations

		// System operations
		"system/info": {
			OperationID:    "getSystemInfo",
			Method:         "GET",
			PathTemplate:   "/api/system",
			RequiredParams: []string{},
		},
		"system/version": {
			OperationID:    "getVersion",
			Method:         "GET",
			PathTemplate:   "/api/system/version",
			RequiredParams: []string{},
		},
		"system/storage": {
			OperationID:    "getStorageInfo",
			Method:         "GET",
			PathTemplate:   "/api/storageinfo",
			RequiredParams: []string{},
		},
		"system/ping": {
			OperationID:    "ping",
			Method:         "GET",
			PathTemplate:   "/api/system/ping",
			RequiredParams: []string{},
		},
		"system/configuration": {
			OperationID:    "getConfiguration",
			Method:         "GET",
			PathTemplate:   "/api/system/configuration",
			RequiredParams: []string{},
		},

		// Docker operations
		"docker/repositories": {
			OperationID:    "listDockerRepositories",
			Method:         "GET",
			PathTemplate:   "/api/docker/{repoKey}/v2/_catalog",
			RequiredParams: []string{"repoKey"},
			OptionalParams: []string{"n", "last"},
		},
		"docker/tags": {
			OperationID:    "listDockerTags",
			Method:         "GET",
			PathTemplate:   "/api/docker/{repoKey}/v2/{imagePath}/tags/list",
			RequiredParams: []string{"repoKey", "imagePath"},
			OptionalParams: []string{"n", "last"},
		},

		// Internal operations for AI-friendly helpers
		"internal/current-user": {
			OperationID:    "getCurrentUser",
			Method:         "INTERNAL",
			PathTemplate:   "", // No external API call
			RequiredParams: []string{},
			Handler:        p.handleGetCurrentUser,
		},
		"internal/available-features": {
			OperationID:    "getAvailableFeatures",
			Method:         "INTERNAL",
			PathTemplate:   "", // No external API call
			RequiredParams: []string{},
			Handler:        p.handleGetAvailableFeatures,
		},
	}

	// Merge in enhanced search operations (Epic 4, Story 4.1)
	searchOps := p.getEnhancedSearchOperations()
	for key, op := range searchOps {
		operations[key] = op
	}

	// Merge in package discovery operations (Epic 4, Story 4.2)
	packageOps := p.getPackageDiscoveryOperations()
	for key, op := range packageOps {
		operations[key] = op
	}

	return operations
}

// GetDefaultConfiguration returns default Artifactory configuration
func (p *ArtifactoryProvider) GetDefaultConfiguration() providers.ProviderConfig {
	// This method returns static config, but we add defensive check for consistency
	if p == nil {
		// Return minimal valid config if provider is nil
		return providers.ProviderConfig{
			BaseURL:  "https://mycompany.jfrog.io/artifactory",
			AuthType: "bearer",
		}
	}

	return providers.ProviderConfig{
		BaseURL:        "https://mycompany.jfrog.io/artifactory",
		AuthType:       "bearer", // Supports bearer tokens and API keys
		HealthEndpoint: "/api/system/ping",
		DefaultHeaders: map[string]string{
			"Accept":                  "application/json",
			"Content-Type":            "application/json",
			"X-JFrog-Art-Api-Version": "2",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 600, // Artifactory typically has higher rate limits
		},
		Timeout: 60 * time.Second, // Longer timeout for artifact operations
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnRateLimit: true,
		},
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "repositories",
				DisplayName: "Repository Management",
				Description: "Operations for managing Artifactory repositories",
				Operations:  []string{"repos/list", "repos/get", "repos/create", "repos/update", "repos/delete"},
			},
			{
				Name:        "artifacts",
				DisplayName: "Artifact Management",
				Description: "Operations for managing artifacts and packages",
				Operations: []string{
					"artifacts/upload", "artifacts/download", "artifacts/info",
					"artifacts/copy", "artifacts/move", "artifacts/delete",
					"artifacts/properties/set", "artifacts/properties/delete",
				},
			},
			{
				Name:        "search",
				DisplayName: "Search Operations",
				Description: "Various search capabilities",
				Operations: []string{
					"search/artifacts", "search/aql", "search/gavc",
					"search/property", "search/checksum", "search/pattern",
					"search/dates", "search/buildArtifacts", "search/dependency",
					"search/usage", "search/latestVersion", "search/stats",
					"search/badChecksum", "search/license", "search/metadata",
				},
			},
			{
				Name:        "builds",
				DisplayName: "Build Management",
				Description: "Operations for managing build information",
				Operations: []string{
					"builds/list", "builds/get", "builds/runs",
					"builds/upload", "builds/promote", "builds/delete",
				},
			},
			{
				Name:        "system",
				DisplayName: "System Operations",
				Description: "System information and configuration",
				Operations:  []string{"system/info", "system/version", "system/storage", "system/ping", "system/configuration"},
			},
			{
				Name:        "docker",
				DisplayName: "Docker Registry",
				Description: "Docker-specific operations",
				Operations:  []string{"docker/repositories", "docker/tags"},
			},
			{
				Name:        "packages",
				DisplayName: "Package Discovery",
				Description: "Simplified package discovery and version management",
				Operations: []string{
					"packages/info", "packages/versions", "packages/latest",
					"packages/stats", "packages/properties", "packages/search",
					"packages/dependencies", "packages/dependents",
					"packages/maven/info", "packages/maven/versions", "packages/maven/pom",
					"packages/npm/info", "packages/npm/versions", "packages/npm/tarball",
					"packages/docker/info", "packages/docker/tags", "packages/docker/layers",
					"packages/pypi/info", "packages/pypi/versions",
					"packages/nuget/info", "packages/nuget/versions",
				},
			},
		},
	}
}

// GetAIOptimizedDefinitions returns AI-friendly tool definitions
func (p *ArtifactoryProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	// Defensive nil check
	if p == nil {
		return nil
	}

	// Use the enhanced AI-optimized definitions from Story 0.2
	return p.GetEnhancedAIOptimizedDefinitions()
}

// GetAIOptimizedDefinitionsLegacy returns the original simpler definitions
// Kept for backward compatibility if needed
func (p *ArtifactoryProvider) GetAIOptimizedDefinitionsLegacy() []providers.AIOptimizedToolDefinition {
	// Defensive nil check
	if p == nil {
		return nil
	}

	return []providers.AIOptimizedToolDefinition{
		{
			Name:        "artifactory_repositories",
			Description: "Manage Artifactory repositories including local, remote, virtual, and federated repositories",
			UsageExamples: []providers.Example{
				{
					Scenario: "List all repositories",
					Input: map[string]interface{}{
						"action": "list",
					},
				},
				{
					Scenario: "Create a new Maven repository",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"repoKey":     "my-maven-local",
							"rclass":      "local",
							"packageType": "maven",
							"description": "Local Maven repository",
						},
					},
				},
				{
					Scenario: "Get repository configuration",
					Input: map[string]interface{}{
						"action":  "get",
						"repoKey": "my-maven-local",
					},
				},
			},
			SemanticTags: []string{
				"repository", "storage", "package-management", "maven", "npm", "docker",
				"pypi", "nuget", "helm", "go", "rpm", "debian", "generic",
			},
		},
		{
			Name:        "artifactory_artifacts",
			Description: "Upload, download, copy, move, and manage artifacts with properties and metadata",
			UsageExamples: []providers.Example{
				{
					Scenario: "Upload an artifact",
					Input: map[string]interface{}{
						"action": "upload",
						"parameters": map[string]interface{}{
							"repoKey":  "libs-release-local",
							"itemPath": "com/mycompany/myapp/1.0.0/myapp-1.0.0.jar",
							"file":     "@/path/to/myapp-1.0.0.jar",
						},
					},
				},
				{
					Scenario: "Download an artifact",
					Input: map[string]interface{}{
						"action": "download",
						"parameters": map[string]interface{}{
							"repoKey":  "libs-release-local",
							"itemPath": "com/mycompany/myapp/1.0.0/myapp-1.0.0.jar",
						},
					},
				},
				{
					Scenario: "Set artifact properties",
					Input: map[string]interface{}{
						"action": "properties/set",
						"parameters": map[string]interface{}{
							"repoKey":    "libs-release-local",
							"itemPath":   "com/mycompany/myapp/1.0.0/",
							"properties": "build.number=123;build.name=myapp",
							"recursive":  true,
						},
					},
				},
			},
			SemanticTags: []string{
				"artifact", "package", "binary", "upload", "download", "properties",
				"metadata", "copy", "move", "promote",
			},
		},
		{
			Name:        "artifactory_search",
			Description: "Search artifacts using various criteria including AQL, GAVC, properties, checksums, and patterns",
			UsageExamples: []providers.Example{
				{
					Scenario: "Search artifacts by name",
					Input: map[string]interface{}{
						"action": "artifacts",
						"parameters": map[string]interface{}{
							"name":  "*.jar",
							"repos": "libs-release-local,libs-snapshot-local",
						},
					},
				},
				{
					Scenario: "Advanced AQL search",
					Input: map[string]interface{}{
						"action": "aql",
						"parameters": map[string]interface{}{
							"query": `items.find({
								"repo": "libs-release-local",
								"size": {"$gt": 10000}
							})`,
						},
					},
				},
				{
					Scenario: "Search by Maven coordinates (GAVC)",
					Input: map[string]interface{}{
						"action": "gavc",
						"parameters": map[string]interface{}{
							"g":     "com.mycompany",
							"a":     "myapp",
							"v":     "1.0.*",
							"repos": "libs-release-local",
						},
					},
				},
			},
			SemanticTags: []string{
				"search", "query", "aql", "find", "locate", "discover",
				"gavc", "maven", "checksum", "sha256", "properties",
			},
		},
		{
			Name:        "artifactory_builds",
			Description: "Manage CI/CD build information, promote builds, and track build artifacts",
			UsageExamples: []providers.Example{
				{
					Scenario: "List all builds",
					Input: map[string]interface{}{
						"action": "list",
					},
				},
				{
					Scenario: "Get specific build info",
					Input: map[string]interface{}{
						"action": "get",
						"parameters": map[string]interface{}{
							"buildName":   "myapp",
							"buildNumber": "123",
						},
					},
				},
				{
					Scenario: "Promote build to production",
					Input: map[string]interface{}{
						"action": "promote",
						"parameters": map[string]interface{}{
							"buildName":   "myapp",
							"buildNumber": "123",
							"targetRepo":  "libs-prod-local",
							"status":      "Released",
							"comment":     "Promoted to production",
						},
					},
				},
			},
			SemanticTags: []string{
				"build", "ci", "cd", "pipeline", "jenkins", "promote",
				"release", "deployment", "build-info",
			},
		},
		{
			Name:        "artifactory_security",
			Description: "Manage users, groups, permissions, and access tokens for secure repository access",
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new user",
					Input: map[string]interface{}{
						"action": "users/create",
						"parameters": map[string]interface{}{
							"userName": "john.doe",
							"email":    "john.doe@company.com",
							"password": "SecurePass123!",
							"groups":   []string{"developers", "readers"},
						},
					},
				},
				{
					Scenario: "Create an access token",
					Input: map[string]interface{}{
						"action": "tokens/create",
						"parameters": map[string]interface{}{
							"username":    "ci-user",
							"scope":       "applied-permissions/user",
							"expires_in":  7776000, // 90 days
							"refreshable": true,
						},
					},
				},
				{
					Scenario: "Create repository permission",
					Input: map[string]interface{}{
						"action": "permissions/create",
						"parameters": map[string]interface{}{
							"name": "dev-team-permission",
							"repositories": map[string]interface{}{
								"libs-snapshot-local": []string{"read", "write", "delete"},
							},
							"groups": map[string]interface{}{
								"developers": []string{"read", "write"},
							},
						},
					},
				},
			},
			SemanticTags: []string{
				"security", "user", "group", "permission", "access", "token",
				"authentication", "authorization", "rbac", "acl",
			},
		},
	}
}

// HealthCheck performs a health check on the Artifactory instance
func (p *ArtifactoryProvider) HealthCheck(ctx context.Context) error {
	// Defensive nil checks
	if ctx == nil {
		return fmt.Errorf("artifactory health check: context cannot be nil")
	}
	if p == nil || p.BaseProvider == nil {
		return fmt.Errorf("artifactory health check: provider not properly initialized")
	}

	// Use the ping endpoint for health check
	baseURL := p.BaseProvider.GetDefaultConfiguration().BaseURL
	if baseURL == "" {
		baseURL = "<not configured>"
	}

	resp, err := p.ExecuteHTTPRequest(ctx, "GET", "/api/system/ping", nil, nil)
	if err != nil {
		return fmt.Errorf("artifactory health check failed for %s: %w", baseURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artifactory health check for %s returned unexpected status %d (expected 200 OK)", baseURL, resp.StatusCode)
	}

	return nil
}

// HealthCheckWithCapabilities performs a health check and returns capability information
func (p *ArtifactoryProvider) HealthCheckWithCapabilities(ctx context.Context) (map[string]interface{}, error) {
	// Defensive nil checks
	if ctx == nil {
		return nil, fmt.Errorf("artifactory health check: context cannot be nil")
	}
	if p == nil || p.BaseProvider == nil {
		return nil, fmt.Errorf("artifactory health check: provider not properly initialized")
	}

	// Perform basic health check
	healthErr := p.HealthCheck(ctx)

	result := map[string]interface{}{
		"provider": "artifactory",
		"healthy":  healthErr == nil,
		"baseURL":  p.BaseProvider.GetDefaultConfiguration().BaseURL,
	}

	if healthErr != nil {
		result["error"] = healthErr.Error()
	}

	// Include capability report if available
	if p.capabilityDiscoverer != nil {
		report, err := p.capabilityDiscoverer.DiscoverCapabilities(ctx, p)
		if err == nil {
			result["capabilities"] = map[string]interface{}{
				"features":           report.Features,
				"operations_summary": p.summarizeOperationCapabilities(report.Operations),
				"cache_valid":        report.CacheValid,
				"timestamp":          report.Timestamp,
			}
		} else {
			result["capability_error"] = fmt.Sprintf("Failed to discover capabilities: %v", err)
		}
	}

	return result, healthErr
}

// summarizeOperationCapabilities creates a summary of operation availability
func (p *ArtifactoryProvider) summarizeOperationCapabilities(operations map[string]Capability) map[string]interface{} {
	total := len(operations)
	available := 0
	unavailable := 0
	categories := make(map[string]int)

	for _, cap := range operations {
		if cap.Available {
			available++
		} else {
			unavailable++
			// Categorize unavailable reasons
			if strings.Contains(cap.Reason, "license") {
				categories["license_required"]++
			} else if strings.Contains(cap.Reason, "permission") {
				categories["permission_required"]++
			} else if strings.Contains(cap.Reason, "not installed") {
				categories["not_installed"]++
			} else if strings.Contains(cap.Reason, "cloud-only") {
				categories["cloud_only"]++
			} else {
				categories["other"]++
			}
		}
	}

	return map[string]interface{}{
		"total":                  total,
		"available":              available,
		"unavailable":            unavailable,
		"unavailable_categories": categories,
	}
}

// executeAQLQuery handles AQL query execution with text/plain content type
func (p *ArtifactoryProvider) executeAQLQuery(ctx context.Context, mapping providers.OperationMapping, params map[string]interface{}) (interface{}, error) {
	// Extract the query parameter - it should be a plain text AQL query
	var queryText string

	// Handle different ways the query might be provided
	if query, ok := params["query"]; ok {
		switch v := query.(type) {
		case string:
			queryText = v
		case map[string]interface{}:
			// If a map is provided, try to convert it to AQL format
			queryText = p.formatAQLFromMap(v)
		default:
			return nil, fmt.Errorf("invalid query format: expected string or map, got %T", v)
		}
	} else {
		return nil, fmt.Errorf("missing required parameter 'query' for AQL operation")
	}

	// Validate the query
	if err := p.validateAQLQuery(queryText); err != nil {
		return nil, fmt.Errorf("invalid AQL query: %w", err)
	}

	// Execute the AQL query with text/plain content type
	headers := map[string]string{
		"Content-Type": "text/plain",
		"Accept":       "application/json",
	}

	// Use ExecuteHTTPRequest directly with plain text body
	resp, err := p.ExecuteHTTPRequest(ctx, mapping.Method, mapping.PathTemplate, queryText, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to execute AQL query: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read and parse response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read AQL response: %w", err)
	}

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AQL query failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse AQL response - it returns results in a specific format
	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse AQL response: %w", err)
	}

	// Handle pagination if results are paginated
	if results, ok := result["results"].([]interface{}); ok {
		// Check if there's a limit in the query for pagination support
		if limit, hasLimit := params["limit"]; hasLimit {
			if limitInt, ok := limit.(int); ok && limitInt < len(results) {
				result["results"] = results[:limitInt]
				result["has_more"] = true
			}
		}

		// Set total_count to the actual number of results being returned
		if finalResults, ok := result["results"].([]interface{}); ok {
			result["total_count"] = len(finalResults)
		} else {
			result["total_count"] = len(results)
		}
	}

	return result, nil
}

// formatAQLFromMap converts a map-based query structure to AQL text format
func (p *ArtifactoryProvider) formatAQLFromMap(queryMap map[string]interface{}) string {
	// Basic AQL query builder from map structure
	// Example: {"type": "file", "repo": "my-repo"} -> items.find({"type": "file", "repo": "my-repo"})

	jsonBytes, err := json.Marshal(queryMap)
	if err != nil {
		// Fallback to basic query
		return `items.find({})`
	}

	return fmt.Sprintf("items.find(%s)", string(jsonBytes))
}

// validateAQLQuery performs basic validation on an AQL query string
func (p *ArtifactoryProvider) validateAQLQuery(query string) error {
	// Basic validation - ensure it's not empty and has basic AQL structure
	query = strings.TrimSpace(query)

	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Check for basic AQL keywords
	lowerQuery := strings.ToLower(query)
	hasValidPrefix := strings.Contains(lowerQuery, "items.find") ||
		strings.Contains(lowerQuery, "builds.find") ||
		strings.Contains(lowerQuery, "entries.find") ||
		strings.Contains(lowerQuery, "artifacts.find")

	if !hasValidPrefix {
		return fmt.Errorf("query must contain a valid AQL domain (items, builds, entries, or artifacts) with .find()")
	}

	// Check for balanced parentheses and braces
	if !p.hasBalancedBrackets(query) {
		return fmt.Errorf("query has unbalanced parentheses or braces")
	}

	return nil
}

// hasBalancedBrackets checks if a string has balanced brackets
func (p *ArtifactoryProvider) hasBalancedBrackets(s string) bool {
	stack := []rune{}
	pairs := map[rune]rune{
		')': '(',
		'}': '{',
		']': '[',
	}

	for _, ch := range s {
		switch ch {
		case '(', '{', '[':
			stack = append(stack, ch)
		case ')', '}', ']':
			if len(stack) == 0 || stack[len(stack)-1] != pairs[ch] {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}

	return len(stack) == 0
}

// ExecuteOperation executes an Artifactory operation
func (p *ArtifactoryProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Defensive nil checks
	if ctx == nil {
		return nil, fmt.Errorf("artifactory execute operation: context cannot be nil")
	}
	if p == nil || p.BaseProvider == nil {
		return nil, fmt.Errorf("artifactory execute operation: provider not properly initialized")
	}
	if params == nil {
		params = make(map[string]interface{})
	}

	// Normalize operation name (handle different formats)
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	mappings := p.GetOperationMappings()
	mapping, exists := mappings[operation]
	if !exists {
		// Build list of available operations for better error message
		availableOps := make([]string, 0, len(mappings))
		for op := range mappings {
			availableOps = append(availableOps, op)
		}
		// Limit to first 10 operations for readability
		if len(availableOps) > 10 {
			availableOps = append(availableOps[:10], "...")
		}
		return nil, fmt.Errorf("artifactory: unknown operation '%s'. Available operations include: %v", operation, availableOps)
	}

	// Validate search operation parameters (Epic 4, Story 4.1)
	if strings.HasPrefix(operation, "search/") {
		if err := p.validateSearchParameters(operation, params); err != nil {
			return nil, fmt.Errorf("artifactory: invalid search parameters: %w", err)
		}
	}

	// Check capability if discoverer is available
	if p.capabilityDiscoverer != nil {
		// Try to get cached report first
		report := p.capabilityDiscoverer.GetCachedReport()
		if report == nil {
			// Perform discovery if no cached report
			var err error
			report, err = p.capabilityDiscoverer.DiscoverCapabilities(ctx, p)
			if err != nil {
				// Log error but continue - we don't want to block operations due to capability check failures
				if p.GetLogger() != nil {
					p.GetLogger().Warn("Failed to discover capabilities", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}
		}

		// Check if operation is available
		if report != nil && report.Operations != nil {
			if capability, exists := report.Operations[operation]; exists {
				if !capability.Available {
					// Return structured error for unavailable operations
					return FormatCapabilityError(operation, capability), nil
				}
			}
		}
	}

	// Special handling for AQL operations - requires text/plain content type
	if operation == "search/aql" {
		return p.executeAQLQuery(ctx, mapping, params)
	}

	// Use base provider's execution for other operations
	return p.Execute(ctx, operation, params)
}

// normalizeOperationName normalizes operation names to handle different formats
func (p *ArtifactoryProvider) normalizeOperationName(operation string) string {
	// Defensive check
	if operation == "" {
		return ""
	}

	// Special case: internal operations should not be normalized
	if strings.HasPrefix(operation, "internal/") {
		return operation
	}

	// First, handle different separators to normalize format
	normalized := strings.ReplaceAll(operation, "-", "/")
	normalized = strings.ReplaceAll(normalized, "_", "/")

	// If it already has a resource prefix (e.g., "repos/create"), return it
	if strings.Contains(normalized, "/") {
		return normalized
	}

	// Only apply simple action defaults if no resource is specified
	simpleActions := map[string]string{
		"list":     "repos/list",
		"get":      "repos/get",
		"create":   "repos/create",
		"update":   "repos/update",
		"delete":   "repos/delete",
		"upload":   "artifacts/upload",
		"download": "artifacts/download",
		"search":   "search/artifacts",
	}

	if defaultOp, ok := simpleActions[normalized]; ok {
		return defaultOp
	}

	return normalized
}

// GetOpenAPISpec returns the OpenAPI specification for Artifactory
// Note: Artifactory doesn't provide a public OpenAPI spec, so this returns nil
func (p *ArtifactoryProvider) GetOpenAPISpec() (*openapi3.T, error) {
	// Defensive nil check
	if p == nil {
		return nil, fmt.Errorf("artifactory GetOpenAPISpec: provider not initialized")
	}

	// Artifactory doesn't provide a public OpenAPI specification
	// Operations are defined manually based on documentation
	return nil, fmt.Errorf("artifactory provider: OpenAPI specification not available (operations are defined manually based on JFrog documentation)")
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
// Since Artifactory doesn't provide a public OpenAPI spec, we return the API version
func (p *ArtifactoryProvider) GetEmbeddedSpecVersion() string {
	// Defensive nil check
	if p == nil {
		return "v2"
	}
	// Return the Artifactory API version we're implementing
	return "v2-7.x"
}

// ValidateCredentials validates Artifactory credentials
func (p *ArtifactoryProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// Defensive nil checks
	if ctx == nil {
		return fmt.Errorf("artifactory validate credentials: context cannot be nil")
	}
	if p == nil || p.BaseProvider == nil {
		return fmt.Errorf("artifactory validate credentials: provider not properly initialized")
	}
	if creds == nil {
		return fmt.Errorf("artifactory validate credentials: credentials cannot be nil")
	}

	// Check for required credentials
	token, hasToken := creds["token"]
	apiKey, hasAPIKey := creds["api_key"]
	username, hasUsername := creds["username"]
	password, hasPassword := creds["password"]

	// Artifactory supports multiple auth methods
	if !hasToken && !hasAPIKey && (!hasUsername || !hasPassword) {
		return fmt.Errorf("artifactory validate credentials: no valid credentials provided (requires token, api_key, or username/password)")
	}

	// Create a temporary context with the provided credentials
	providerCreds := &providers.ProviderCredentials{}

	// Store the current auth type and temporarily change it if needed
	originalAuthType := p.BaseProvider.GetDefaultConfiguration().AuthType
	tempConfig := p.BaseProvider.GetDefaultConfiguration()

	if hasToken {
		providerCreds.Token = token
		tempConfig.AuthType = "bearer"
	} else if hasAPIKey {
		providerCreds.APIKey = apiKey
		tempConfig.AuthType = "bearer" // Artifactory uses Bearer auth for API keys too
	} else if hasUsername && hasPassword {
		providerCreds.Username = username
		providerCreds.Password = password
		tempConfig.AuthType = "basic"
	}

	// Temporarily set the auth type
	p.SetConfiguration(tempConfig)
	// Restore original auth type after validation
	defer func() {
		restoreConfig := p.GetDefaultConfiguration()
		restoreConfig.AuthType = originalAuthType
		p.SetConfiguration(restoreConfig)
	}()

	// Use the context with credentials
	ctx = providers.WithContext(ctx, &providers.ProviderContext{
		Credentials: providerCreds,
	})

	// Validate by performing a health check
	err := p.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("artifactory validate credentials failed: %w", err)
	}

	return nil
}

// handleGetCurrentUser handles the internal/current-user operation
// This encapsulates the complex 2-step process of getting user details
func (p *ArtifactoryProvider) handleGetCurrentUser(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Defensive nil checks
	if ctx == nil {
		return nil, fmt.Errorf("handleGetCurrentUser: context cannot be nil")
	}
	if p == nil || p.BaseProvider == nil {
		return nil, fmt.Errorf("handleGetCurrentUser: provider not initialized")
	}

	// Step 1: Call /api/security/apiKey to get API key context
	// This doesn't exist directly, but we can use the permission target endpoint
	// Actually, we need to discover this from the context we have
	// Let's use a different approach - try to get user info from the context

	// Try to extract credentials from context to determine user
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx.Credentials == nil {
		return nil, fmt.Errorf("no credentials found in context for user identification")
	}

	// We can try to list users and find ourselves, or use the permissions endpoint
	// The most reliable way is to use the security endpoint to check what we can access
	// For now, let's use a simpler approach - try to get our own user details by checking permissions

	// Call the permissions endpoint to get user context
	permissionsResp, err := p.Execute(ctx, "permissions/list", params)
	if err != nil {
		// If we can't list permissions, try a different approach
		// We may not have permission to list all permissions
		// Try to get system info which often contains user info
		sysInfo, sysErr := p.Execute(ctx, "system/info", params)
		if sysErr != nil {
			return nil, fmt.Errorf("failed to identify current user: permissions error: %w, system info error: %w", err, sysErr)
		}
		// Try to extract user info from system response
		if sysMap, ok := sysInfo.(map[string]interface{}); ok {
			// Look for user info in the response
			userInfo := map[string]interface{}{
				"source": "system_info",
				"data":   sysMap,
			}
			return userInfo, nil
		}
		return nil, fmt.Errorf("failed to extract user info from system response")
	}

	// Extract user information from permissions response
	// The permissions response typically contains information about the current user
	userInfo := map[string]interface{}{
		"source":      "permissions",
		"permissions": permissionsResp,
		"message":     "User identification through permissions endpoint",
	}

	return userInfo, nil
}

// handleGetAvailableFeatures handles the internal/available-features operation
// This probes various endpoints to determine what features are available
func (p *ArtifactoryProvider) handleGetAvailableFeatures(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Defensive nil checks
	if ctx == nil {
		return nil, fmt.Errorf("handleGetAvailableFeatures: context cannot be nil")
	}
	if p == nil || p.BaseProvider == nil {
		return nil, fmt.Errorf("handleGetAvailableFeatures: provider not initialized")
	}

	features := make(map[string]interface{})

	// Check core Artifactory features
	features["artifactory"] = p.probeFeature(ctx, "/api/system/ping")

	// Check for Xray integration
	features["xray"] = p.probeFeature(ctx, "/xray/api/v1/system/version")

	// Check for Pipelines
	features["pipelines"] = p.probeFeature(ctx, "/pipelines/api/v1/system/info")

	// Check for Mission Control
	features["mission_control"] = p.probeFeature(ctx, "/mc/api/v1/system/info")

	// Check for Distribution
	features["distribution"] = p.probeFeature(ctx, "/distribution/api/v1/system/info")

	// Check for Access (usually always available with Artifactory)
	features["access"] = p.probeFeature(ctx, "/access/api/v1/system/ping")

	// Check repository types available
	repoTypes := p.checkRepositoryTypes(ctx)
	features["repository_types"] = repoTypes

	// Check available package types
	features["package_types"] = []string{
		"maven", "gradle", "ivy", "sbt",
		"npm", "bower", "yarn",
		"nuget",
		"gems", "bundler",
		"pypi", "conda",
		"docker", "helm",
		"go", "cargo",
		"conan", "opkg",
		"debian", "rpm", "yum",
		"vagrant", "gitlfs",
		"generic",
	}

	// Include information about operations available
	operations := p.GetOperationMappings()
	features["operations_count"] = len(operations)

	return features, nil
}

// probeFeature checks if a feature endpoint is available
func (p *ArtifactoryProvider) probeFeature(ctx context.Context, endpoint string) interface{} {
	// Try to call the endpoint
	resp, err := p.ExecuteHTTPRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return map[string]interface{}{
			"available": false,
			"reason":    fmt.Sprintf("probe failed: %v", err),
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check the status code
	switch resp.StatusCode {
	case http.StatusOK:
		return map[string]interface{}{
			"available": true,
			"status":    "active",
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		return map[string]interface{}{
			"available": false,
			"reason":    "no permission to access this feature",
			"status":    resp.StatusCode,
		}
	case http.StatusNotFound:
		return map[string]interface{}{
			"available": false,
			"reason":    "feature not installed or not available",
		}
	default:
		return map[string]interface{}{
			"available": false,
			"reason":    fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
			"status":    resp.StatusCode,
		}
	}
}

// checkRepositoryTypes checks what repository types are available
func (p *ArtifactoryProvider) checkRepositoryTypes(ctx context.Context) map[string]interface{} {
	result := make(map[string]interface{})

	// Try to list repositories to see what types exist
	repos, err := p.Execute(ctx, "repos/list", map[string]interface{}{})
	if err != nil {
		result["error"] = fmt.Sprintf("failed to list repositories: %v", err)
		return result
	}

	// Extract repository classes from response
	localRepos := 0
	remoteRepos := 0
	virtualRepos := 0
	federatedRepos := 0

	if repoList, ok := repos.([]interface{}); ok {
		for _, repo := range repoList {
			if repoMap, ok := repo.(map[string]interface{}); ok {
				if rclass, ok := repoMap["type"].(string); ok {
					switch rclass {
					case "LOCAL":
						localRepos++
					case "REMOTE":
						remoteRepos++
					case "VIRTUAL":
						virtualRepos++
					case "FEDERATED":
						federatedRepos++
					}
				}
			}
		}
	}

	result["local"] = map[string]interface{}{
		"supported": true,
		"count":     localRepos,
	}
	result["remote"] = map[string]interface{}{
		"supported": true,
		"count":     remoteRepos,
	}
	result["virtual"] = map[string]interface{}{
		"supported": true,
		"count":     virtualRepos,
	}
	if federatedRepos > 0 {
		result["federated"] = map[string]interface{}{
			"supported": true,
			"count":     federatedRepos,
		}
	}

	return result
}

// GetCapabilityReport returns the current capability report for this provider
func (p *ArtifactoryProvider) GetCapabilityReport(ctx context.Context) (*CapabilityReport, error) {
	// Defensive nil checks
	if ctx == nil {
		return nil, fmt.Errorf("GetCapabilityReport: context cannot be nil")
	}
	if p == nil {
		return nil, fmt.Errorf("GetCapabilityReport: provider not initialized")
	}
	if p.capabilityDiscoverer == nil {
		return nil, fmt.Errorf("GetCapabilityReport: capability discoverer not initialized")
	}

	// Try cached report first
	report := p.capabilityDiscoverer.GetCachedReport()
	if report != nil {
		return report, nil
	}

	// Perform discovery
	return p.capabilityDiscoverer.DiscoverCapabilities(ctx, p)
}

// InvalidateCapabilityCache forces the next capability discovery to refresh
func (p *ArtifactoryProvider) InvalidateCapabilityCache() {
	if p != nil && p.capabilityDiscoverer != nil {
		p.capabilityDiscoverer.InvalidateCache()
	}
}

// discoverOperations discovers available operations based on user permissions
// Currently unused but kept for future implementation of dynamic operation discovery
//
//nolint:unused // Reserved for future use when we implement dynamic operation discovery
func (p *ArtifactoryProvider) discoverOperations(ctx context.Context) ([]providers.OperationMapping, error) {
	// Defensive nil checks
	if ctx == nil {
		return nil, fmt.Errorf("artifactory discoverOperations: context cannot be nil")
	}
	if p == nil {
		return nil, fmt.Errorf("artifactory discoverOperations: provider not initialized")
	}

	// This could be implemented to discover available operations based on:
	// 1. User permissions
	// 2. Artifactory version
	// 3. Enabled features/modules

	// For now, return all operations
	mappings := p.GetOperationMappings()
	result := make([]providers.OperationMapping, 0, len(mappings))
	for _, mapping := range mappings {
		result = append(result, mapping)
	}
	return result, nil
}
