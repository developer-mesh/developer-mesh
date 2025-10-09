package github

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/developer-mesh/developer-mesh/pkg/utils"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-github/v74/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// contextKeyGitHubClient is the context key for GitHub REST client
	contextKeyGitHubClient contextKey = "github_client"
	// contextKeyGitHubV4Client is the context key for GitHub GraphQL client
	contextKeyGitHubV4Client contextKey = "githubv4_client"
)

// GetGitHubClientFromContext retrieves the GitHub REST client from context
func GetGitHubClientFromContext(ctx context.Context) (*github.Client, bool) {
	client, ok := ctx.Value(contextKeyGitHubClient).(*github.Client)
	return client, ok
}

// GetGitHubV4ClientFromContext retrieves the GitHub GraphQL client from context
func GetGitHubV4ClientFromContext(ctx context.Context) (*githubv4.Client, bool) {
	client, ok := ctx.Value(contextKeyGitHubV4Client).(*githubv4.Client)
	return client, ok
}

// GitHubProvider implements comprehensive GitHub integration
type GitHubProvider struct {
	logger         observability.Logger
	encryptionSvc  *security.EncryptionService
	circuitBreaker *resilience.CircuitBreaker
	rateLimiter    *resilience.RateLimiter
	retryPolicy    *resilience.RetryPolicy

	// Client management
	clientCache map[string]*clientSet
	clientMutex sync.RWMutex

	// Tool registry
	toolRegistry    map[string]ToolHandler
	toolsetRegistry map[string]*Toolset
	enabledToolsets map[string]bool

	// Configuration
	config providers.ProviderConfig

	// Caching
	cache        *GitHubCache
	cacheEnabled bool
}

// clientSet holds both REST and GraphQL clients for a tenant
type clientSet struct {
	restClient   *github.Client
	gqlClient    *githubv4.Client
	lastAccessed time.Time
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Content interface{} `json:"content"`
	IsError bool        `json:"isError,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ToolHandler defines the interface for tool implementations
type ToolHandler interface {
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
	GetDefinition() ToolDefinition
}

// ToolDefinition describes a tool's schema
type ToolDefinition struct {
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	InputSchema     map[string]interface{}   `json:"inputSchema"`
	Metadata        map[string]interface{}   `json:"metadata,omitempty"`
	ResponseExample map[string]interface{}   `json:"responseExample,omitempty"`
	CommonErrors    []map[string]interface{} `json:"commonErrors,omitempty"`
	ExtendedHelp    string                   `json:"extendedHelp,omitempty"`
}

// Toolset represents a group of related tools
type Toolset struct {
	Name        string
	Description string
	Tools       []ToolHandler
	Enabled     bool
}

// NewGitHubProvider creates a new GitHub provider with comprehensive tooling
func NewGitHubProvider(
	logger observability.Logger,
	encryptionSvc *security.EncryptionService,
	circuitBreaker *resilience.CircuitBreaker,
	rateLimiter *resilience.RateLimiter,
	retryPolicy *resilience.RetryPolicy,
) *GitHubProvider {
	p := &GitHubProvider{
		logger:          logger,
		encryptionSvc:   encryptionSvc,
		circuitBreaker:  circuitBreaker,
		rateLimiter:     rateLimiter,
		retryPolicy:     retryPolicy,
		clientCache:     make(map[string]*clientSet),
		toolRegistry:    make(map[string]ToolHandler),
		toolsetRegistry: make(map[string]*Toolset),
		enabledToolsets: make(map[string]bool),
		config:          getDefaultConfig(),
		cache:           NewGitHubCache(DefaultCacheConfig()),
		cacheEnabled:    true, // Enable caching by default
	}

	// Initialize toolsets
	p.initializeToolsets()

	// Enable default toolsets
	p.enableDefaultToolsets()

	return p
}

// GetProviderName returns the provider name
func (p *GitHubProvider) GetProviderName() string {
	return "github"
}

// GetSupportedVersions returns supported GitHub API versions
func (p *GitHubProvider) GetSupportedVersions() []string {
	return []string{"v3", "v4", "2022-11-28"}
}

// initializeToolsets creates all available toolsets
func (p *GitHubProvider) initializeToolsets() {
	// Repository toolset
	repoTools := &Toolset{
		Name:        "repos",
		Description: "GitHub Repository related tools",
		Tools: []ToolHandler{
			NewListRepositoriesHandler(p),
			NewGetRepositoryHandler(p),
			NewUpdateRepositoryHandler(p),
			NewDeleteRepositoryHandler(p),
			NewSearchRepositoriesHandler(p),
			NewGetFileContentsHandler(p),
			NewListCommitsHandler(p),
			NewSearchCodeHandler(p),
			NewGetCommitHandler(p),
			NewListBranchesHandler(p),
			NewCreateOrUpdateFileHandler(p),
			NewCreateRepositoryHandler(p),
			NewForkRepositoryHandler(p),
			NewCreateBranchHandler(p),
			NewPushFilesHandler(p),
			NewDeleteFileHandler(p),
			NewListTagsHandler(p),
			NewGetTagHandler(p),
			NewListReleasesHandler(p),
			NewGetLatestReleaseHandler(p),
			NewGetReleaseByTagHandler(p),
			NewCreateReleaseHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["repos"] = repoTools

	// Issues toolset
	issueTools := &Toolset{
		Name:        "issues",
		Description: "GitHub Issues related tools",
		Tools: []ToolHandler{
			NewGetIssueHandler(p),
			NewSearchIssuesHandler(p),
			NewListIssuesHandler(p),
			NewGetIssueCommentsHandler(p),
			NewCreateIssueHandler(p),
			NewAddIssueCommentHandler(p),
			NewUpdateIssueHandler(p),
			NewLockIssueHandler(p),
			NewUnlockIssueHandler(p),
			NewGetIssueEventsHandler(p),
			NewGetIssueTimelineHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["issues"] = issueTools

	// Pull Requests toolset
	prTools := &Toolset{
		Name:        "pull_requests",
		Description: "GitHub Pull Request related tools",
		Tools: []ToolHandler{
			NewGetPullRequestHandler(p),
			NewListPullRequestsHandler(p),
			NewGetPullRequestFilesHandler(p),
			NewSearchPullRequestsHandler(p),
			NewCreatePullRequestHandler(p),
			NewMergePullRequestHandler(p),
			NewUpdatePullRequestHandler(p),
			NewUpdatePullRequestBranchHandler(p),
			NewGetPullRequestDiffHandler(p),
			NewGetPullRequestReviewsHandler(p),
			NewGetPullRequestReviewCommentsHandler(p),
			NewCreatePullRequestReviewHandler(p),
			NewSubmitPullRequestReviewHandler(p),
			NewAddPullRequestReviewCommentHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["pull_requests"] = prTools

	// Actions toolset
	actionsTools := &Toolset{
		Name:        "actions",
		Description: "GitHub Actions workflows and CI/CD operations",
		Tools: []ToolHandler{
			NewListWorkflowsHandler(p),
			NewListWorkflowRunsHandler(p),
			NewGetWorkflowRunHandler(p),
			NewListWorkflowJobsHandler(p),
			NewRunWorkflowHandler(p),
			NewRerunWorkflowRunHandler(p),
			NewCancelWorkflowRunHandler(p),
			// Extended actions operations
			NewRerunFailedJobsHandler(p),
			NewGetJobLogsHandler(p),
			NewGetWorkflowRunLogsHandler(p),
			NewGetWorkflowRunUsageHandler(p),
			NewListArtifactsHandler(p),
			NewDownloadArtifactHandler(p),
			NewDeleteWorkflowRunLogsHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["actions"] = actionsTools

	// Context toolset - always enabled
	contextTools := &Toolset{
		Name:        "context",
		Description: "Tools that provide context about the current user and GitHub context",
		Tools: []ToolHandler{
			NewGetMeHandler(p),
			NewGetTeamsHandler(p),
		},
		Enabled: true,
	}
	p.toolsetRegistry["context"] = contextTools

	// Security toolset - code scanning, dependabot, secret scanning
	securityTools := &Toolset{
		Name:        "security",
		Description: "Security scanning and vulnerability management",
		Tools: []ToolHandler{
			// Code scanning
			NewListCodeScanningAlertsHandler(p),
			NewGetCodeScanningAlertHandler(p),
			NewUpdateCodeScanningAlertHandler(p),
			// Dependabot
			NewListDependabotAlertsHandler(p),
			NewGetDependabotAlertHandler(p),
			NewUpdateDependabotAlertHandler(p),
			// Secret scanning
			NewListSecretScanningAlertsHandler(p),
			NewGetSecretScanningAlertHandler(p),
			NewUpdateSecretScanningAlertHandler(p),
			NewListSecretScanningLocationsHandler(p),
			// Security advisories
			NewListSecurityAdvisoriesHandler(p),
			NewListGlobalSecurityAdvisoriesHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["security"] = securityTools

	// Collaboration toolset - notifications, gists, watching
	collaborationTools := &Toolset{
		Name:        "collaboration",
		Description: "Collaboration features including notifications, gists, and watching",
		Tools: []ToolHandler{
			// Notifications
			NewListNotificationsHandler(p),
			NewMarkNotificationAsReadHandler(p),
			// Gists
			NewListGistsHandler(p),
			NewGetGistHandler(p),
			NewCreateGistHandler(p),
			NewUpdateGistHandler(p),
			NewDeleteGistHandler(p),
			NewStarGistHandler(p),
			NewUnstarGistHandler(p),
			// Watching
			NewWatchRepositoryHandler(p),
			NewUnwatchRepositoryHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["collaboration"] = collaborationTools

	// Git toolset - low-level Git operations
	gitTools := &Toolset{
		Name:        "git",
		Description: "Low-level Git operations for trees, blobs, commits, and refs",
		Tools: []ToolHandler{
			// Blobs
			NewGetBlobHandler(p),
			NewCreateBlobHandler(p),
			// Trees
			NewGetTreeHandler(p),
			NewCreateTreeHandler(p),
			// Commits
			NewGetGitCommitHandler(p),
			NewCreateCommitHandler(p),
			// References
			NewGetRefHandler(p),
			NewListRefsHandler(p),
			NewCreateRefHandler(p),
			NewUpdateRefHandler(p),
			NewDeleteRefHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["git"] = gitTools

	// Organizations toolset - organization and team management
	orgsTools := &Toolset{
		Name:        "organizations",
		Description: "Organization, team, and user management",
		Tools: []ToolHandler{
			NewListOrganizationsHandler(p),
			NewGetOrganizationHandler(p),
			NewSearchOrganizationsHandler(p),
			NewSearchUsersHandler(p),
			NewListTeamsHandler(p),
			NewGetTeamMembersHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["organizations"] = orgsTools

	// GraphQL toolset - advanced GraphQL operations
	graphqlTools := &Toolset{
		Name:        "graphql",
		Description: "Advanced GraphQL-based operations for efficient data fetching",
		Tools: []ToolHandler{
			// Query operations
			NewListIssuesGraphQLHandler(p),
			NewSearchIssuesAndPRsGraphQLHandler(p),
			NewGetRepositoryDetailsGraphQLHandler(p),
			// Mutation operations
			NewCreateIssueGraphQLHandler(p),
			NewCreatePullRequestGraphQLHandler(p),
			NewAddPullRequestReviewGraphQLHandler(p),
			NewMergePullRequestGraphQLHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["graphql"] = graphqlTools

	// Discussions toolset
	discussionsTools := &Toolset{
		Name:        "discussions",
		Description: "GitHub Discussions management tools",
		Tools: []ToolHandler{
			NewListDiscussionsHandler(p),
			NewGetDiscussionHandler(p),
			NewGetDiscussionCommentsHandler(p),
			NewListDiscussionCategoriesHandler(p),
		},
		Enabled: false,
	}
	p.toolsetRegistry["discussions"] = discussionsTools

	// Register all tools in the main registry with caching for read operations
	for _, toolset := range p.toolsetRegistry {
		for i, tool := range toolset.Tools {
			def := tool.GetDefinition()

			// Wrap read-only operations with cache if caching is enabled
			if p.cache != nil && isReadOnlyOperation(def.Name) {
				toolset.Tools[i] = WrapWithCache(p, tool)
				tool = toolset.Tools[i]
			}

			p.toolRegistry[def.Name] = tool
		}
	}
}

// enableDefaultToolsets enables the default set of toolsets
func (p *GitHubProvider) enableDefaultToolsets() {
	// Enable context toolset by default
	p.enabledToolsets["context"] = true

	// Enable all toolsets by default to expose full GitHub functionality
	defaultToolsets := []string{
		"repos",
		"issues",
		"pull_requests",
		"actions",
		"security",
		"collaboration",
		"git",
		"organizations",
		"graphql",     // New GraphQL operations
		"discussions", // New Discussions API
	}
	for _, name := range defaultToolsets {
		if err := p.EnableToolset(name); err != nil {
			p.logger.Warn("Failed to enable default toolset", map[string]interface{}{
				"toolset": name,
				"error":   err.Error(),
			})
		}
	}
}

// EnableToolset enables a specific toolset
func (p *GitHubProvider) EnableToolset(name string) error {
	toolset, exists := p.toolsetRegistry[name]
	if !exists {
		return fmt.Errorf("toolset %s not found", name)
	}

	toolset.Enabled = true
	p.enabledToolsets[name] = true

	p.logger.Info("Enabled toolset", map[string]interface{}{
		"toolset": name,
	})

	return nil
}

// DisableToolset disables a specific toolset
func (p *GitHubProvider) DisableToolset(name string) error {
	toolset, exists := p.toolsetRegistry[name]
	if !exists {
		return fmt.Errorf("toolset %s not found", name)
	}

	// Don't allow disabling context toolset
	if name == "context" {
		return fmt.Errorf("cannot disable context toolset")
	}

	toolset.Enabled = false
	delete(p.enabledToolsets, name)

	p.logger.Info("Disabled toolset", map[string]interface{}{
		"toolset": name,
	})

	return nil
}

// GetToolDefinitions returns all available tool definitions
func (p *GitHubProvider) GetToolDefinitions() []providers.ToolDefinition {
	var definitions []providers.ToolDefinition

	for _, toolset := range p.toolsetRegistry {
		if !toolset.Enabled {
			continue
		}

		for _, tool := range toolset.Tools {
			def := tool.GetDefinition()

			// Convert to providers.ToolDefinition
			providerDef := providers.ToolDefinition{
				Name:        def.Name,
				DisplayName: def.Name,
				Description: def.Description,
				Category:    toolset.Name,
				Parameters:  p.extractParameters(def.InputSchema),
			}

			definitions = append(definitions, providerDef)
		}
	}

	return definitions
}

// extractParameters extracts parameter definitions from input schema
func (p *GitHubProvider) extractParameters(schema map[string]interface{}) []providers.ParameterDef {
	var params []providers.ParameterDef

	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		required := make(map[string]bool)
		if reqList, ok := schema["required"].([]interface{}); ok {
			for _, req := range reqList {
				if reqStr, ok := req.(string); ok {
					required[reqStr] = true
				}
			}
		}

		for name, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				param := providers.ParameterDef{
					Name:        name,
					Type:        p.getParameterType(propMap),
					Required:    required[name],
					Description: p.getParameterDescription(propMap),
				}
				params = append(params, param)
			}
		}
	}

	return params
}

// getParameterType extracts the type from a property definition
func (p *GitHubProvider) getParameterType(prop map[string]interface{}) string {
	if typ, ok := prop["type"].(string); ok {
		return typ
	}
	return "string"
}

// getParameterDescription extracts the description from a property definition
func (p *GitHubProvider) getParameterDescription(prop map[string]interface{}) string {
	if desc, ok := prop["description"].(string); ok {
		return desc
	}
	return ""
}

// ExecuteOperation executes a GitHub operation
func (p *GitHubProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize parameters: handle MCP-style nested parameters
	// MCP tools may send parameters in a nested structure where some params (owner, repo)
	// are at the top level while others are nested in a "parameters" object
	p.logger.Info("ExecuteOperation: before normalization", map[string]interface{}{
		"operation":  operation,
		"raw_params": utils.RedactSensitiveData(params),
	})
	normalizedParams := p.normalizeParameters(params)
	p.logger.Info("ExecuteOperation: after normalization", map[string]interface{}{
		"operation":         operation,
		"normalized_params": utils.RedactSensitiveData(normalizedParams),
	})

	// Extract tenant ID from context or normalized params
	tenantID := p.extractTenantID(ctx, normalizedParams)

	// Get or create client for tenant
	clients, err := p.getOrCreateClients(ctx, tenantID, normalizedParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	// Update context with clients
	ctx = context.WithValue(ctx, contextKeyGitHubClient, clients.restClient)
	ctx = context.WithValue(ctx, contextKeyGitHubV4Client, clients.gqlClient)

	// Debug: verify clients are in context
	if testClient, ok := GetGitHubClientFromContext(ctx); ok {
		p.logger.Debug("GitHub client successfully set in context", map[string]interface{}{
			"operation":  operation,
			"has_client": testClient != nil,
		})
	} else {
		p.logger.Error("Failed to set GitHub client in context", map[string]interface{}{
			"operation": operation,
		})
	}

	// Find and execute the tool
	tool, exists := p.toolRegistry[operation]
	if !exists {
		// Try to resolve operation name
		resolvedOp := p.resolveOperationName(operation, normalizedParams)
		tool, exists = p.toolRegistry[resolvedOp]
		if !exists {
			return nil, fmt.Errorf("unknown operation: %s", operation)
		}
	}

	// Execute with resilience patterns
	var result *ToolResult
	var execErr error

	// Apply circuit breaker if configured
	if p.circuitBreaker != nil {
		cbResult, cbErr := p.circuitBreaker.Execute(ctx, func() (interface{}, error) {
			res, err := tool.Execute(ctx, normalizedParams)
			return res, err
		})
		if cbErr != nil {
			return nil, cbErr
		}
		result = cbResult.(*ToolResult)
	} else {
		// Execute directly without circuit breaker
		result, execErr = tool.Execute(ctx, normalizedParams)
		if execErr != nil {
			return nil, execErr
		}
	}

	// Convert result to expected format
	if result.IsError {
		return nil, fmt.Errorf("tool execution failed: %v", result.Error)
	}

	return result.Content, nil
}

// getOrCreateClients gets or creates GitHub clients for a tenant
func (p *GitHubProvider) getOrCreateClients(ctx context.Context, tenantID string, params map[string]interface{}) (*clientSet, error) {
	p.clientMutex.RLock()
	if clients, exists := p.clientCache[tenantID]; exists {
		clients.lastAccessed = time.Now()
		p.clientMutex.RUnlock()
		return clients, nil
	}
	p.clientMutex.RUnlock()

	// Create new clients
	p.clientMutex.Lock()
	defer p.clientMutex.Unlock()

	// Double-check after acquiring write lock
	if clients, exists := p.clientCache[tenantID]; exists {
		clients.lastAccessed = time.Now()
		return clients, nil
	}

	// Extract authentication
	token, err := p.extractAuthToken(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to extract auth token: %w", err)
	}

	// Create OAuth2 token source
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(ctx, ts)

	// Create REST client
	restClient := github.NewClient(httpClient)

	// Create GraphQL client
	gqlClient := githubv4.NewClient(httpClient)

	clients := &clientSet{
		restClient:   restClient,
		gqlClient:    gqlClient,
		lastAccessed: time.Now(),
	}

	p.clientCache[tenantID] = clients

	// Start cleanup goroutine if needed
	go p.cleanupOldClients()

	return clients, nil
}

// getGraphQLClient gets or creates a GraphQL client for the tenant
func (p *GitHubProvider) getGraphQLClient(ctx context.Context) (*githubv4.Client, error) {
	// Get tenant ID
	tenantID := p.extractTenantID(ctx, nil)

	// Try to get from cache
	p.clientMutex.RLock()
	if clients, ok := p.clientCache[tenantID]; ok {
		p.clientMutex.RUnlock()
		clients.lastAccessed = time.Now()
		if clients.gqlClient != nil {
			return clients.gqlClient, nil
		}
	} else {
		p.clientMutex.RUnlock()
	}

	// Create new client set
	clients, err := p.getOrCreateClients(ctx, tenantID, nil)
	if err != nil {
		return nil, err
	}

	if clients.gqlClient == nil {
		return nil, fmt.Errorf("failed to create GraphQL client")
	}

	return clients.gqlClient, nil
}

// extractTenantID extracts tenant ID from context or params
func (p *GitHubProvider) extractTenantID(ctx context.Context, params map[string]interface{}) string {
	// Try from context first
	if tenantID, ok := ctx.Value("tenant_id").(string); ok {
		return tenantID
	}

	// Try from params
	if params != nil {
		if tenantID, ok := params["tenant_id"].(string); ok {
			return tenantID
		}
	}

	// Default tenant
	return "default"
}

// normalizeParameters handles MCP-style nested parameters
// MCP tools may send parameters in a nested structure where some params (owner, repo, etc.)
// are at the top level while operation-specific parameters are nested in a "parameters" object.
// This method flattens the structure for handlers that expect all parameters at the top level.
func (p *GitHubProvider) normalizeParameters(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return make(map[string]interface{})
	}

	// Create a new map with all top-level parameters
	normalized := make(map[string]interface{})
	for k, v := range params {
		normalized[k] = v
	}

	// If there's a nested "parameters" object, merge its contents into the top level
	if nestedParams, ok := params["parameters"].(map[string]interface{}); ok {
		p.logger.Info("Normalizing nested parameters", map[string]interface{}{
			"nested_count":    len(nestedParams),
			"nested_params":   utils.RedactSensitiveData(nestedParams),
			"top_level_count": len(params),
			"has_owner":       normalized["owner"] != nil,
			"has_repo":        normalized["repo"] != nil,
		})

		// Merge nested parameters into normalized map
		// Top-level parameters take precedence over nested ones
		for k, v := range nestedParams {
			if _, exists := normalized[k]; !exists {
				normalized[k] = v
				p.logger.Info("Added nested param to normalized", map[string]interface{}{
					"key":   k,
					"value": utils.SanitizeLogValue(k, v),
				})
			}
		}

		// Remove the nested "parameters" key since we've flattened it
		delete(normalized, "parameters")
	}

	return normalized
}

// extractAuthToken extracts authentication token from context or params
func (p *GitHubProvider) extractAuthToken(ctx context.Context, params map[string]interface{}) (string, error) {
	// Try from ProviderContext first (standard provider pattern)
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Credentials != nil {
		if pctx.Credentials.Token != "" {
			return pctx.Credentials.Token, nil
		}
		// Also check custom credentials map
		if token, ok := pctx.Credentials.Custom["token"]; ok && token != "" {
			return token, nil
		}
		// Check for GitHub-specific key
		if token, ok := pctx.Credentials.Custom["github_token"]; ok && token != "" {
			return token, nil
		}
	}

	// Try passthrough auth from params
	if auth, ok := params["__passthrough_auth"].(map[string]interface{}); ok {
		if encryptedToken, ok := auth["encrypted_token"].(string); ok {
			// Decrypt the token - assuming encrypted_token is base64 encoded
			// Extract tenant ID for decryption
			tenantID := p.extractTenantID(ctx, params)
			token, err := p.encryptionSvc.DecryptCredential([]byte(encryptedToken), tenantID)
			if err != nil {
				return "", fmt.Errorf("failed to decrypt token: %w", err)
			}
			return token, nil
		}

		if token, ok := auth["token"].(string); ok {
			return token, nil
		}
	}

	// Try direct token from params
	if token, ok := params["token"].(string); ok {
		return token, nil
	}

	// Try from context
	if token, ok := ctx.Value("github_token").(string); ok {
		return token, nil
	}

	return "", fmt.Errorf("no authentication token found")
}

// resolveOperationName resolves operation names to tool names
func (p *GitHubProvider) resolveOperationName(operation string, params map[string]interface{}) string {
	// Handle format variations
	operation = strings.ReplaceAll(operation, "-", "_")
	operation = strings.ReplaceAll(operation, "/", "_")

	// Handle action-based names
	if action, ok := params["action"].(string); ok {
		// Try to determine resource type from params
		resource := p.inferResourceType(params)
		if resource != "" {
			return fmt.Sprintf("%s_%s", resource, action)
		}
	}

	return operation
}

// inferResourceType infers the resource type from parameters
func (p *GitHubProvider) inferResourceType(params map[string]interface{}) string {
	// Check for explicit resource indicators
	if _, hasRepo := params["repo"]; hasRepo {
		if _, hasIssue := params["issue_number"]; hasIssue {
			return "issues"
		}
		if _, hasPR := params["pull_number"]; hasPR {
			return "pulls"
		}
		if _, hasWorkflow := params["workflow_id"]; hasWorkflow {
			return "actions"
		}
		return "repos"
	}

	if _, hasOrg := params["org"]; hasOrg {
		return "orgs"
	}

	if _, hasUser := params["username"]; hasUser {
		return "users"
	}

	return ""
}

// cleanupOldClients removes clients that haven't been accessed recently
func (p *GitHubProvider) cleanupOldClients() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.clientMutex.Lock()
		cutoff := time.Now().Add(-30 * time.Minute)

		for tenantID, clients := range p.clientCache {
			if clients.lastAccessed.Before(cutoff) {
				delete(p.clientCache, tenantID)
				p.logger.Debug("Cleaned up idle client", map[string]interface{}{
					"tenant_id": tenantID,
				})
			}
		}
		p.clientMutex.Unlock()
	}
}

// ValidateCredentials validates GitHub credentials
func (p *GitHubProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	token, hasToken := creds["token"]
	apiKey, hasAPIKey := creds["api_key"]
	pat, hasPAT := creds["personal_access_token"]

	// Accept any of these credential types
	authToken := ""
	if hasToken {
		authToken = token
	} else if hasAPIKey {
		authToken = apiKey
	} else if hasPAT {
		authToken = pat
	} else {
		return fmt.Errorf("missing required credentials: token, api_key, or personal_access_token")
	}

	// Test the credentials with a simple API call
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	)
	httpClient := oauth2.NewClient(ctx, ts)
	client := github.NewClient(httpClient)

	_, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}

	return nil
}

// HealthCheck verifies the GitHub API is accessible
func (p *GitHubProvider) HealthCheck(ctx context.Context) error {
	// Use an unauthenticated client for health check
	client := github.NewClient(nil)

	_, _, err := client.Meta.Get(ctx)
	if err != nil {
		return fmt.Errorf("GitHub API health check failed: %w", err)
	}

	return nil
}

// Close cleans up resources
func (p *GitHubProvider) Close() error {
	p.clientMutex.Lock()
	defer p.clientMutex.Unlock()

	// Clear client cache
	p.clientCache = make(map[string]*clientSet)

	return nil
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() providers.ProviderConfig {
	return providers.ProviderConfig{
		BaseURL:  "https://api.github.com",
		AuthType: "bearer",
		DefaultHeaders: map[string]string{
			"Accept":               "application/vnd.github.v3+json",
			"X-GitHub-Api-Version": "2022-11-28",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 60,
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:   3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
		},
	}
}

// GetOperationMappings returns operation mappings for compatibility
func (p *GitHubProvider) GetOperationMappings() map[string]providers.OperationMapping {
	mappings := make(map[string]providers.OperationMapping)

	// Generate mappings from registered tools
	for name, tool := range p.toolRegistry {
		def := tool.GetDefinition()

		// Extract parameters from schema
		var required []string
		var optional []string

		if schema, ok := def.InputSchema["properties"].(map[string]interface{}); ok {
			if reqList, ok := def.InputSchema["required"].([]interface{}); ok {
				for _, req := range reqList {
					if reqStr, ok := req.(string); ok {
						required = append(required, reqStr)
					}
				}
			}

			for param := range schema {
				found := false
				for _, req := range required {
					if req == param {
						found = true
						break
					}
				}
				if !found {
					optional = append(optional, param)
				}
			}
		}

		mappings[name] = providers.OperationMapping{
			OperationID:    name,
			RequiredParams: required,
			OptionalParams: optional,
		}
	}

	return mappings
}

// GetDefaultConfiguration returns default configuration
func (p *GitHubProvider) GetDefaultConfiguration() providers.ProviderConfig {
	return p.config
}

// Cache management methods

// EnableCache enables caching for the provider
func (p *GitHubProvider) EnableCache() {
	p.cacheEnabled = true
	p.logger.Info("GitHub provider cache enabled", nil)
}

// DisableCache disables caching for the provider
func (p *GitHubProvider) DisableCache() {
	p.cacheEnabled = false
	p.logger.Info("GitHub provider cache disabled", nil)
}

// IsCacheEnabled returns whether caching is enabled
func (p *GitHubProvider) IsCacheEnabled() bool {
	return p.cacheEnabled
}

// GetCacheStats returns cache statistics
func (p *GitHubProvider) GetCacheStats() map[string]interface{} {
	if p.cache == nil {
		return map[string]interface{}{
			"enabled": false,
			"error":   "cache not initialized",
		}
	}

	stats := p.cache.Stats()
	stats["enabled"] = p.cacheEnabled
	return stats
}

// ClearCache clears all cached items
func (p *GitHubProvider) ClearCache() {
	if p.cache != nil {
		p.cache.Clear()
		p.logger.Info("GitHub provider cache cleared", nil)
	}
}

// InvalidateCachePattern invalidates cache entries matching a pattern
func (p *GitHubProvider) InvalidateCachePattern(pattern string) int {
	if p.cache == nil {
		return 0
	}

	count := p.cache.InvalidatePattern(pattern)
	p.logger.Info("Invalidated cache entries", map[string]interface{}{
		"pattern": pattern,
		"count":   count,
	})
	return count
}

// SetCacheTTL sets the default cache TTL
func (p *GitHubProvider) SetCacheTTL(ttl time.Duration) {
	if p.cache != nil {
		p.cache.ttl = ttl
		p.logger.Info("Updated cache TTL", map[string]interface{}{
			"ttl": ttl.String(),
		})
	}
}

// GetAIOptimizedDefinitions returns AI-optimized tool definitions
func (p *GitHubProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	tools := p.GetToolDefinitions()
	optimized := make([]providers.AIOptimizedToolDefinition, 0, len(tools))

	for _, tool := range tools {
		// Convert standard tool to AI-optimized format
		// Use enhanced description if available
		description := GetOperationDescription(tool.Name)
		if description == "" {
			description = tool.Description
		}

		aiTool := providers.AIOptimizedToolDefinition{
			Name:        tool.Name,
			DisplayName: tool.Name,
			Category:    "GitHub",
			Description: description,

			// Add semantic tags based on operation type
			SemanticTags: getSemanticTags(tool.Name),

			// Set complexity level
			ComplexityLevel: getComplexityLevel(tool.Name),

			// Convert input schema
			InputSchema: providers.AIParameterSchema{
				Type:       "object",
				Properties: make(map[string]providers.AIPropertySchema),
			},
		}

		// Add subcategory based on toolset
		if strings.Contains(tool.Name, "issue") {
			aiTool.Subcategory = "Issues"
		} else if strings.Contains(tool.Name, "pull") || strings.Contains(tool.Name, "pr_") {
			aiTool.Subcategory = "Pull Requests"
		} else if strings.Contains(tool.Name, "workflow") || strings.Contains(tool.Name, "action") {
			aiTool.Subcategory = "Actions"
		} else if strings.Contains(tool.Name, "repository") || strings.Contains(tool.Name, "repo_") {
			aiTool.Subcategory = "Repositories"
		} else if strings.Contains(tool.Name, "security") || strings.Contains(tool.Name, "scan") {
			aiTool.Subcategory = "Security"
		}

		optimized = append(optimized, aiTool)
	}

	return optimized
}

// getSemanticTags returns semantic tags for a tool based on its name
func getSemanticTags(toolName string) []string {
	tags := []string{"github", "version-control"}

	if strings.HasPrefix(toolName, "get_") {
		tags = append(tags, "read", "fetch", "retrieve")
	}
	if strings.HasPrefix(toolName, "list_") {
		tags = append(tags, "read", "enumerate", "collection")
	}
	if strings.HasPrefix(toolName, "create_") {
		tags = append(tags, "write", "new", "add")
	}
	if strings.HasPrefix(toolName, "update_") {
		tags = append(tags, "write", "modify", "edit")
	}
	if strings.HasPrefix(toolName, "delete_") {
		tags = append(tags, "write", "remove", "destroy")
	}
	if strings.Contains(toolName, "search") {
		tags = append(tags, "query", "find", "filter")
	}

	return tags
}

// getComplexityLevel returns the complexity level of a tool
func getComplexityLevel(toolName string) string {
	// Simple operations
	if strings.HasPrefix(toolName, "get_") || strings.HasPrefix(toolName, "list_") {
		return "simple"
	}

	// Complex operations
	if strings.Contains(toolName, "merge") || strings.Contains(toolName, "workflow") ||
		strings.Contains(toolName, "tree") || strings.Contains(toolName, "commit") {
		return "complex"
	}

	// Default to moderate
	return "moderate"
}

// GetOpenAPISpec returns the OpenAPI specification
func (p *GitHubProvider) GetOpenAPISpec() (*openapi3.T, error) {
	// GitHub doesn't provide a standard OpenAPI spec in the format we need
	// Return a minimal spec for compatibility
	loader := openapi3.NewLoader()
	minimalSpec := []byte(`{
		"openapi": "3.0.0",
		"info": {
			"title": "GitHub API",
			"version": "v3"
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
	return spec, nil
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *GitHubProvider) GetEmbeddedSpecVersion() string {
	return "github-v3-2024"
}
