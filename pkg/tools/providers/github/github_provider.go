package github

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

// Embed GitHub OpenAPI spec as fallback
// This will be populated from GitHub's official spec
//
//go:embed github-openapi-v3.json
var githubOpenAPISpecJSON []byte

// GitHubProvider implements the StandardToolProvider interface for GitHub
type GitHubProvider struct {
	*providers.BaseProvider
	specCache    repository.OpenAPICacheRepository // For caching the OpenAPI spec
	specFallback *openapi3.T                       // Embedded fallback spec
	httpClient   *http.Client
}

// NewGitHubProvider creates a new GitHub provider instance
func NewGitHubProvider(logger observability.Logger) *GitHubProvider {
	base := providers.NewBaseProvider("github", "v3", "https://api.github.com", logger)

	// Load embedded spec as fallback
	var specFallback *openapi3.T
	if len(githubOpenAPISpecJSON) > 0 {
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(githubOpenAPISpecJSON)
		if err == nil {
			specFallback = spec
		}
	}

	provider := &GitHubProvider{
		BaseProvider: base,
		specFallback: specFallback,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	// Set operation mappings in base provider
	provider.SetOperationMappings(provider.GetOperationMappings())
	// Set configuration to ensure auth type is configured
	provider.SetConfiguration(provider.GetDefaultConfiguration())
	return provider
}

// NewGitHubProviderWithCache creates a new GitHub provider with spec caching
func NewGitHubProviderWithCache(logger observability.Logger, specCache repository.OpenAPICacheRepository) *GitHubProvider {
	provider := NewGitHubProvider(logger)
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *GitHubProvider) GetProviderName() string {
	return "github"
}

// GetSupportedVersions returns supported GitHub API versions
func (p *GitHubProvider) GetSupportedVersions() []string {
	return []string{"v3", "2022-11-28"}
}

// GetToolDefinitions returns GitHub-specific tool definitions
func (p *GitHubProvider) GetToolDefinitions() []providers.ToolDefinition {
	return []providers.ToolDefinition{
		{
			Name:        "github_repos",
			DisplayName: "GitHub Repositories",
			Description: "Manage GitHub repositories",
			Category:    "version_control",
			Operation: providers.OperationDef{
				ID:           "repos",
				Method:       "GET",
				PathTemplate: "/repos/{owner}/{repo}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "owner", In: "path", Type: "string", Required: true, Description: "Repository owner"},
				{Name: "repo", In: "path", Type: "string", Required: true, Description: "Repository name"},
			},
		},
		{
			Name:        "github_issues",
			DisplayName: "GitHub Issues",
			Description: "Manage GitHub issues",
			Category:    "Issue Tracking",
			Operation: providers.OperationDef{
				ID:           "issues",
				Method:       "GET",
				PathTemplate: "/repos/{owner}/{repo}/issues",
			},
			Parameters: []providers.ParameterDef{
				{Name: "owner", In: "path", Type: "string", Required: true, Description: "Repository owner"},
				{Name: "repo", In: "path", Type: "string", Required: true, Description: "Repository name"},
				{Name: "state", In: "query", Type: "string", Required: false, Description: "Issue state", Default: "open"},
			},
		},
		{
			Name:        "github_pulls",
			DisplayName: "GitHub Pull Requests",
			Description: "Manage GitHub pull requests",
			Category:    "Code Review",
			Operation: providers.OperationDef{
				ID:           "pulls",
				Method:       "GET",
				PathTemplate: "/repos/{owner}/{repo}/pulls",
			},
			Parameters: []providers.ParameterDef{
				{Name: "owner", In: "path", Type: "string", Required: true, Description: "Repository owner"},
				{Name: "repo", In: "path", Type: "string", Required: true, Description: "Repository name"},
				{Name: "state", In: "query", Type: "string", Required: false, Description: "PR state", Default: "open"},
			},
		},
		{
			Name:        "github_actions",
			DisplayName: "GitHub Actions",
			Description: "Manage GitHub Actions workflows",
			Category:    "CI/CD",
			Operation: providers.OperationDef{
				ID:           "actions",
				Method:       "GET",
				PathTemplate: "/repos/{owner}/{repo}/actions/workflows",
			},
			Parameters: []providers.ParameterDef{
				{Name: "owner", In: "path", Type: "string", Required: true, Description: "Repository owner"},
				{Name: "repo", In: "path", Type: "string", Required: true, Description: "Repository name"},
			},
		},
		{
			Name:        "github_users",
			DisplayName: "GitHub Users",
			Description: "Manage GitHub users and organizations",
			Category:    "Identity",
			Operation: providers.OperationDef{
				ID:           "users",
				Method:       "GET",
				PathTemplate: "/users/{username}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "username", In: "path", Type: "string", Required: false, Description: "Username"},
			},
		},
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
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid GitHub credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response from GitHub API: %d", resp.StatusCode)
	}

	return nil
}

// GetOperationMappings returns GitHub-specific operation mappings
func (p *GitHubProvider) GetOperationMappings() map[string]providers.OperationMapping {
	return map[string]providers.OperationMapping{
		// Repository operations
		"repos/list": {
			OperationID:    "listRepos",
			Method:         "GET",
			PathTemplate:   "/user/repos",
			RequiredParams: []string{},
			OptionalParams: []string{"type", "sort", "direction", "per_page", "page"},
		},
		"repos/get": {
			OperationID:    "getRepo",
			Method:         "GET",
			PathTemplate:   "/repos/{owner}/{repo}",
			RequiredParams: []string{"owner", "repo"},
		},
		"repos/create": {
			OperationID:    "createRepo",
			Method:         "POST",
			PathTemplate:   "/user/repos",
			RequiredParams: []string{"name"},
			OptionalParams: []string{"description", "private", "auto_init"},
		},
		"repos/update": {
			OperationID:    "updateRepo",
			Method:         "PATCH",
			PathTemplate:   "/repos/{owner}/{repo}",
			RequiredParams: []string{"owner", "repo"},
			OptionalParams: []string{"name", "description", "private", "default_branch"},
		},
		"repos/delete": {
			OperationID:    "deleteRepo",
			Method:         "DELETE",
			PathTemplate:   "/repos/{owner}/{repo}",
			RequiredParams: []string{"owner", "repo"},
		},

		// Issue operations
		"issues/list": {
			OperationID:    "listIssues",
			Method:         "GET",
			PathTemplate:   "/repos/{owner}/{repo}/issues",
			RequiredParams: []string{"owner", "repo"},
			OptionalParams: []string{"state", "labels", "sort", "direction", "since", "per_page", "page"},
		},
		"issues/get": {
			OperationID:    "getIssue",
			Method:         "GET",
			PathTemplate:   "/repos/{owner}/{repo}/issues/{issue_number}",
			RequiredParams: []string{"owner", "repo", "issue_number"},
		},
		"issues/create": {
			OperationID:    "createIssue",
			Method:         "POST",
			PathTemplate:   "/repos/{owner}/{repo}/issues",
			RequiredParams: []string{"owner", "repo", "title"},
			OptionalParams: []string{"body", "labels", "assignees", "milestone"},
		},
		"issues/update": {
			OperationID:    "updateIssue",
			Method:         "PATCH",
			PathTemplate:   "/repos/{owner}/{repo}/issues/{issue_number}",
			RequiredParams: []string{"owner", "repo", "issue_number"},
			OptionalParams: []string{"title", "body", "state", "labels", "assignees"},
		},

		// Pull request operations
		"pulls/list": {
			OperationID:    "listPulls",
			Method:         "GET",
			PathTemplate:   "/repos/{owner}/{repo}/pulls",
			RequiredParams: []string{"owner", "repo"},
			OptionalParams: []string{"state", "head", "base", "sort", "direction", "per_page", "page"},
		},
		"pulls/get": {
			OperationID:    "getPull",
			Method:         "GET",
			PathTemplate:   "/repos/{owner}/{repo}/pulls/{pull_number}",
			RequiredParams: []string{"owner", "repo", "pull_number"},
		},
		"pulls/create": {
			OperationID:    "createPull",
			Method:         "POST",
			PathTemplate:   "/repos/{owner}/{repo}/pulls",
			RequiredParams: []string{"owner", "repo", "title", "head", "base"},
			OptionalParams: []string{"body", "draft"},
		},
		"pulls/merge": {
			OperationID:    "mergePull",
			Method:         "PUT",
			PathTemplate:   "/repos/{owner}/{repo}/pulls/{pull_number}/merge",
			RequiredParams: []string{"owner", "repo", "pull_number"},
			OptionalParams: []string{"commit_title", "commit_message", "merge_method"},
		},

		// Actions operations
		"actions/list-workflows": {
			OperationID:    "listWorkflows",
			Method:         "GET",
			PathTemplate:   "/repos/{owner}/{repo}/actions/workflows",
			RequiredParams: []string{"owner", "repo"},
			OptionalParams: []string{"per_page", "page"},
		},
		"actions/trigger-workflow": {
			OperationID:    "triggerWorkflow",
			Method:         "POST",
			PathTemplate:   "/repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches",
			RequiredParams: []string{"owner", "repo", "workflow_id", "ref"},
			OptionalParams: []string{"inputs"},
		},
	}
}

// GetDefaultConfiguration returns default GitHub configuration
func (p *GitHubProvider) GetDefaultConfiguration() providers.ProviderConfig {
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
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "repository",
				DisplayName: "Repository Management",
				Description: "Operations for managing repositories",
				Operations:  []string{"repos/list", "repos/get", "repos/create", "repos/update", "repos/delete"},
			},
			{
				Name:        "issues",
				DisplayName: "Issue Management",
				Description: "Operations for managing issues",
				Operations:  []string{"issues/list", "issues/get", "issues/create", "issues/update"},
			},
			{
				Name:        "pulls",
				DisplayName: "Pull Request Management",
				Description: "Operations for managing pull requests",
				Operations:  []string{"pulls/list", "pulls/get", "pulls/create", "pulls/merge"},
			},
			{
				Name:        "actions",
				DisplayName: "GitHub Actions",
				Description: "Operations for GitHub Actions workflows",
				Operations:  []string{"actions/list-workflows", "actions/trigger-workflow"},
			},
		},
	}
}

// GetAIOptimizedDefinitions returns AI-friendly tool definitions
func (p *GitHubProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return []providers.AIOptimizedToolDefinition{
		{
			Name:        "github_repos",
			Description: "Manage GitHub repositories including creation, updates, branches, tags, and releases",
			UsageExamples: []providers.Example{
				{
					Scenario: "List all repositories for a user",
					Input: map[string]interface{}{
						"action": "list",
						"parameters": map[string]interface{}{
							"type": "all",
							"sort": "updated",
						},
					},
				},
				{
					Scenario: "Get details of a specific repository",
					Input: map[string]interface{}{
						"action": "get",
						"owner":  "octocat",
						"repo":   "hello-world",
					},
				},
				{
					Scenario: "Create a new repository",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"name":        "my-new-repo",
							"description": "This is my new repository",
							"private":     false,
							"auto_init":   true,
						},
					},
				},
			},
			SemanticTags:  []string{"repository", "code", "version-control", "git"},
			CommonPhrases: []string{"clone repo", "fork repository", "create PR"},
		},
		{
			Name:        "github_issues",
			DisplayName: "GitHub Issues",
			Category:    "Issue Tracking",
			Description: "Manage GitHub issues including creation, updates, comments, and labels",
			UsageExamples: []providers.Example{
				{
					Scenario: "List open issues in a repository",
					Input: map[string]interface{}{
						"action": "list",
						"owner":  "octocat",
						"repo":   "hello-world",
						"parameters": map[string]interface{}{
							"state": "open",
						},
					},
				},
				{
					Scenario: "Create a new issue",
					Input: map[string]interface{}{
						"action": "create",
						"owner":  "octocat",
						"repo":   "hello-world",
						"parameters": map[string]interface{}{
							"title":  "Bug: Application crashes on startup",
							"body":   "The application crashes when...",
							"labels": []string{"bug", "high-priority"},
						},
					},
				},
			},
			SemanticTags: []string{"issue", "bug", "feature-request", "task"},
		},
		{
			Name:        "github_pulls",
			DisplayName: "GitHub Pull Requests",
			Category:    "Code Review",
			Description: "Manage GitHub pull requests including creation, reviews, and merging",
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a pull request",
					Input: map[string]interface{}{
						"action": "create",
						"owner":  "octocat",
						"repo":   "hello-world",
						"parameters": map[string]interface{}{
							"title": "Add new feature",
							"body":  "This PR adds...",
							"head":  "feature-branch",
							"base":  "main",
						},
					},
				},
			},
			SemanticTags: []string{"pull-request", "PR", "merge-request", "code-review"},
		},
	}
}

// ExecuteOperation executes a GitHub operation
func (p *GitHubProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize operation name (handle different formats)
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	// Use base provider's execution with GitHub-specific handling
	return p.Execute(ctx, operation, params)
}

// normalizeOperationName normalizes operation names to handle different formats
func (p *GitHubProvider) normalizeOperationName(operation string) string {
	// First, handle different separators to normalize format
	normalized := strings.ReplaceAll(operation, "-", "/")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	
	// If it already has a resource prefix (e.g., "issues/create"), return it
	if strings.Contains(normalized, "/") {
		return normalized
	}
	
	// Only apply simple action defaults if no resource is specified
	simpleActions := map[string]string{
		"list":   "repos/list",
		"get":    "repos/get",
		"create": "repos/create",
		"update": "repos/update",
		"delete": "repos/delete",
	}

	if defaultOp, ok := simpleActions[normalized]; ok {
		return defaultOp
	}

	return normalized
}

// GetOpenAPISpec returns the OpenAPI specification for GitHub
// This implements the StandardToolProvider interface requirement
func (p *GitHubProvider) GetOpenAPISpec() (*openapi3.T, error) {
	ctx := context.Background()

	// Try cache first if available
	if p.specCache != nil {
		spec, err := p.specCache.Get(ctx, "github-v3")
		if err == nil && spec != nil {
			return spec, nil
		}
	}

	// Try fetching from GitHub with timeout
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

// fetchAndCacheSpec fetches the OpenAPI spec from GitHub and caches it
func (p *GitHubProvider) fetchAndCacheSpec(ctx context.Context) (*openapi3.T, error) {
	// GitHub doesn't provide a standard OpenAPI endpoint, but we can use the REST API description
	// For now, we'll return the embedded spec or fetch from a known location
	req, err := http.NewRequestWithContext(ctx, "GET", "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json", nil)
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
		_ = p.specCache.Set(ctx, "github-v3", spec, 24*time.Hour) // Cache for 24 hours
	}

	return spec, nil
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *GitHubProvider) GetEmbeddedSpecVersion() string {
	// This would typically be set from the embedded spec metadata
	// For now, return the API version we're using
	return "v3-2022-11-28"
}

// HealthCheck verifies the GitHub API is accessible
func (p *GitHubProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *GitHubProvider) Close() error {
	// Currently no resources to clean up
	// If we add connection pools or other resources, clean them up here
	return nil
}
