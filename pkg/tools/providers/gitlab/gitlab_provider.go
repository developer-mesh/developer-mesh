package gitlab

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
)

// Embed GitLab OpenAPI spec as fallback
//
//go:embed gitlab-openapi.json
var gitlabOpenAPISpecJSON []byte

// GitLabModule represents different GitLab modules/features
type GitLabModule string

const (
	ModuleProjects        GitLabModule = "projects"
	ModuleIssues          GitLabModule = "issues"
	ModuleMergeRequests   GitLabModule = "merge_requests"
	ModulePipelines       GitLabModule = "pipelines"
	ModuleJobs            GitLabModule = "jobs"
	ModuleRunners         GitLabModule = "runners"
	ModuleRepositories    GitLabModule = "repositories"
	ModuleWikis           GitLabModule = "wikis"
	ModuleSnippets        GitLabModule = "snippets"
	ModuleDeployments     GitLabModule = "deployments"
	ModuleEnvironments    GitLabModule = "environments"
	ModulePackages        GitLabModule = "packages"
	ModuleContainerReg    GitLabModule = "container_registry"
	ModuleSecurityReports GitLabModule = "security_reports"
	ModuleGroups          GitLabModule = "groups"
	ModuleUsers           GitLabModule = "users"
)

// GitLabProvider implements the StandardToolProvider interface for GitLab
type GitLabProvider struct {
	*providers.BaseProvider
	specCache      repository.OpenAPICacheRepository
	specFallback   *openapi3.T
	httpClient     *http.Client
	enabledModules map[GitLabModule]bool
	baseURL        string // Track the actual base URL for configuration
	instanceURL    string // GitLab instance URL (e.g., gitlab.com or self-hosted)
}

// NewGitLabProvider creates a new GitLab provider instance
func NewGitLabProvider(logger observability.Logger) *GitLabProvider {
	base := providers.NewBaseProvider("gitlab", "v4", "https://gitlab.com/api/v4", logger)

	// Load embedded spec as fallback
	var specFallback *openapi3.T
	if len(gitlabOpenAPISpecJSON) > 0 {
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(gitlabOpenAPISpecJSON)
		if err == nil {
			specFallback = spec
		}
	}

	provider := &GitLabProvider{
		BaseProvider: base,
		specFallback: specFallback,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:     "https://gitlab.com/api/v4",
		instanceURL: "https://gitlab.com",
		enabledModules: map[GitLabModule]bool{
			ModuleProjects:        true,
			ModuleIssues:          true,
			ModuleMergeRequests:   true,
			ModulePipelines:       true,
			ModuleJobs:            true,
			ModuleRepositories:    true,
			ModuleGroups:          true,
			ModuleUsers:           true,
			ModuleWikis:           true,
			ModuleSnippets:        true,
			ModuleDeployments:     true,
			ModuleEnvironments:    true,
			ModulePackages:        true,
			ModuleContainerReg:    true,
			ModuleSecurityReports: true,
			ModuleRunners:         true,
		},
	}

	// Set operation mappings in base provider
	provider.SetOperationMappings(provider.GetOperationMappings())
	return provider
}

// NewGitLabProviderWithCache creates a new GitLab provider with spec caching
func NewGitLabProviderWithCache(logger observability.Logger, specCache repository.OpenAPICacheRepository) *GitLabProvider {
	provider := NewGitLabProvider(logger)
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *GitLabProvider) GetProviderName() string {
	return "gitlab"
}

// GetSupportedVersions returns supported GitLab API versions
func (p *GitLabProvider) GetSupportedVersions() []string {
	return []string{"v4"}
}

// GetToolDefinitions returns GitLab-specific tool definitions
func (p *GitLabProvider) GetToolDefinitions() []providers.ToolDefinition {
	var tools []providers.ToolDefinition

	// Project tools
	if p.enabledModules[ModuleProjects] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_projects",
			DisplayName: "GitLab Projects",
			Description: "Manage GitLab projects",
			Category:    "projects",
			Operation: providers.OperationDef{
				ID:           "projects",
				Method:       "GET",
				PathTemplate: "/projects/{id}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_projects_list",
			DisplayName: "List GitLab Projects",
			Description: "List all GitLab projects accessible to the user",
			Category:    "projects",
			Operation: providers.OperationDef{
				ID:           "projects/list",
				Method:       "GET",
				PathTemplate: "/projects",
			},
			Parameters: []providers.ParameterDef{
				{Name: "owned", In: "query", Type: "boolean", Required: false, Description: "Limit to owned projects"},
				{Name: "membership", In: "query", Type: "boolean", Required: false, Description: "Limit to projects user is member of"},
				{Name: "search", In: "query", Type: "string", Required: false, Description: "Search projects"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
				{Name: "page", In: "query", Type: "integer", Required: false, Description: "Page number"},
			},
		})
	}

	// Issues tools
	if p.enabledModules[ModuleIssues] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_issues",
			DisplayName: "GitLab Issues",
			Description: "Manage GitLab issues",
			Category:    "Issue Tracking",
			Operation: providers.OperationDef{
				ID:           "issues",
				Method:       "GET",
				PathTemplate: "/projects/{id}/issues/{issue_iid}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "issue_iid", In: "path", Type: "integer", Required: true, Description: "Issue IID"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_issues_list",
			DisplayName: "List GitLab Issues",
			Description: "List issues in a project",
			Category:    "Issue Tracking",
			Operation: providers.OperationDef{
				ID:           "issues/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/issues",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "state", In: "query", Type: "string", Required: false, Description: "State of issues (opened, closed, all)"},
				{Name: "labels", In: "query", Type: "string", Required: false, Description: "Comma-separated label names"},
				{Name: "milestone", In: "query", Type: "string", Required: false, Description: "Milestone title"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_issues_create",
			DisplayName: "Create GitLab Issue",
			Description: "Create a new issue in a project",
			Category:    "Issue Tracking",
			Operation: providers.OperationDef{
				ID:           "issues/create",
				Method:       "POST",
				PathTemplate: "/projects/{id}/issues",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "title", In: "body", Type: "string", Required: true, Description: "Issue title"},
				{Name: "description", In: "body", Type: "string", Required: false, Description: "Issue description"},
				{Name: "labels", In: "body", Type: "string", Required: false, Description: "Comma-separated label names"},
				{Name: "assignee_ids", In: "body", Type: "array", Required: false, Description: "User IDs to assign"},
			},
		})
	}

	// Merge Requests tools
	if p.enabledModules[ModuleMergeRequests] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_merge_requests",
			DisplayName: "GitLab Merge Requests",
			Description: "Manage GitLab merge requests",
			Category:    "Merge Requests",
			Operation: providers.OperationDef{
				ID:           "merge_requests",
				Method:       "GET",
				PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "merge_request_iid", In: "path", Type: "integer", Required: true, Description: "Merge request IID"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_merge_requests_list",
			DisplayName: "List GitLab Merge Requests",
			Description: "List merge requests in a project",
			Category:    "Merge Requests",
			Operation: providers.OperationDef{
				ID:           "merge_requests/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/merge_requests",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "state", In: "query", Type: "string", Required: false, Description: "State (opened, closed, locked, merged, all)"},
				{Name: "source_branch", In: "query", Type: "string", Required: false, Description: "Source branch name"},
				{Name: "target_branch", In: "query", Type: "string", Required: false, Description: "Target branch name"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_merge_requests_create",
			DisplayName: "Create GitLab Merge Request",
			Description: "Create a new merge request",
			Category:    "Merge Requests",
			Operation: providers.OperationDef{
				ID:           "merge_requests/create",
				Method:       "POST",
				PathTemplate: "/projects/{id}/merge_requests",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "source_branch", In: "body", Type: "string", Required: true, Description: "Source branch"},
				{Name: "target_branch", In: "body", Type: "string", Required: true, Description: "Target branch"},
				{Name: "title", In: "body", Type: "string", Required: true, Description: "MR title"},
				{Name: "description", In: "body", Type: "string", Required: false, Description: "MR description"},
			},
		})
	}

	// Pipeline tools
	if p.enabledModules[ModulePipelines] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_pipelines",
			DisplayName: "GitLab Pipelines",
			Description: "Manage GitLab CI/CD pipelines",
			Category:    "ci_cd",
			Operation: providers.OperationDef{
				ID:           "pipelines",
				Method:       "GET",
				PathTemplate: "/projects/{id}/pipelines/{pipeline_id}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "pipeline_id", In: "path", Type: "integer", Required: true, Description: "Pipeline ID"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_pipelines_list",
			DisplayName: "List GitLab Pipelines",
			Description: "List pipelines in a project",
			Category:    "ci_cd",
			Operation: providers.OperationDef{
				ID:           "pipelines/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/pipelines",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "status", In: "query", Type: "string", Required: false, Description: "Pipeline status"},
				{Name: "ref", In: "query", Type: "string", Required: false, Description: "Git ref"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_pipelines_trigger",
			DisplayName: "Trigger GitLab Pipeline",
			Description: "Trigger a new pipeline",
			Category:    "ci_cd",
			Operation: providers.OperationDef{
				ID:           "pipelines/trigger",
				Method:       "POST",
				PathTemplate: "/projects/{id}/pipeline",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "ref", In: "body", Type: "string", Required: true, Description: "Git ref to run pipeline on"},
				{Name: "variables", In: "body", Type: "object", Required: false, Description: "Pipeline variables"},
			},
		})
	}

	// Jobs tools
	if p.enabledModules[ModuleJobs] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_jobs_list",
			DisplayName: "List GitLab Jobs",
			Description: "List jobs in a project",
			Category:    "ci_cd",
			Operation: providers.OperationDef{
				ID:           "jobs/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/jobs",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "scope", In: "query", Type: "array", Required: false, Description: "Job scope (created, pending, running, failed, success, canceled, skipped, manual)"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
			},
		})
	}

	// Repository tools
	if p.enabledModules[ModuleRepositories] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_branches_list",
			DisplayName: "List GitLab Branches",
			Description: "List repository branches",
			Category:    "Repository",
			Operation: providers.OperationDef{
				ID:           "branches/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/repository/branches",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "search", In: "query", Type: "string", Required: false, Description: "Search pattern"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_commits_list",
			DisplayName: "List GitLab Commits",
			Description: "List repository commits",
			Category:    "Repository",
			Operation: providers.OperationDef{
				ID:           "commits/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/repository/commits",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "ref_name", In: "query", Type: "string", Required: false, Description: "Branch or tag name"},
				{Name: "since", In: "query", Type: "string", Required: false, Description: "Since date (ISO 8601)"},
				{Name: "until", In: "query", Type: "string", Required: false, Description: "Until date (ISO 8601)"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_tags_list",
			DisplayName: "List GitLab Tags",
			Description: "List repository tags",
			Category:    "Repository",
			Operation: providers.OperationDef{
				ID:           "tags/list",
				Method:       "GET",
				PathTemplate: "/projects/{id}/repository/tags",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Project ID or URL-encoded path"},
				{Name: "search", In: "query", Type: "string", Required: false, Description: "Search pattern"},
			},
		})
	}

	// Groups tools
	if p.enabledModules[ModuleGroups] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_groups_list",
			DisplayName: "List GitLab Groups",
			Description: "List all groups",
			Category:    "Groups",
			Operation: providers.OperationDef{
				ID:           "groups/list",
				Method:       "GET",
				PathTemplate: "/groups",
			},
			Parameters: []providers.ParameterDef{
				{Name: "search", In: "query", Type: "string", Required: false, Description: "Search groups"},
				{Name: "owned", In: "query", Type: "boolean", Required: false, Description: "Limit to owned groups"},
				{Name: "per_page", In: "query", Type: "integer", Required: false, Description: "Number of items per page"},
			},
		})

		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_groups_get",
			DisplayName: "Get GitLab Group",
			Description: "Get group details",
			Category:    "Groups",
			Operation: providers.OperationDef{
				ID:           "groups/get",
				Method:       "GET",
				PathTemplate: "/groups/{id}",
			},
			Parameters: []providers.ParameterDef{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Group ID or URL-encoded path"},
				{Name: "with_projects", In: "query", Type: "boolean", Required: false, Description: "Include group projects"},
			},
		})
	}

	// Users tools
	if p.enabledModules[ModuleUsers] {
		tools = append(tools, providers.ToolDefinition{
			Name:        "gitlab_users_current",
			DisplayName: "Get Current GitLab User",
			Description: "Get current authenticated user",
			Category:    "Users",
			Operation: providers.OperationDef{
				ID:           "users/current",
				Method:       "GET",
				PathTemplate: "/user",
			},
			Parameters: []providers.ParameterDef{},
		})
	}

	return tools
}

// ValidateCredentials validates GitLab credentials
func (p *GitLabProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// GitLab supports personal access tokens, OAuth tokens, and job tokens
	token := ""
	if pat, ok := creds["personal_access_token"]; ok && pat != "" {
		token = pat
	} else if apiKey, ok := creds["api_key"]; ok && apiKey != "" {
		token = apiKey
	} else if t, ok := creds["token"]; ok && t != "" {
		token = t
	} else if jobToken, ok := creds["job_token"]; ok && jobToken != "" {
		token = jobToken
	} else {
		return fmt.Errorf("missing required credentials: personal_access_token, api_key, token, or job_token")
	}

	// Test the credentials by getting user info
	testPath := "user"

	// Create a proper context with credentials for authentication
	pctx := &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token:       token,
			AccessToken: token,
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

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid GitLab credentials")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from GitLab API: %d - %s", resp.StatusCode, string(body))
	}

	// Parse user info to get user details
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err == nil {
		// Store user ID if available
		if id, ok := userInfo["id"]; ok {
			// Get current config and update metadata without losing other fields
			config := p.BaseProvider.GetDefaultConfiguration()
			config.Metadata = map[string]interface{}{
				"user_id":  id,
				"username": userInfo["username"],
				"email":    userInfo["email"],
			}
			p.BaseProvider.SetConfiguration(config)
		}
	}

	return nil
}

// GetOperationMappings returns GitLab-specific operation mappings
func (p *GitLabProvider) GetOperationMappings() map[string]providers.OperationMapping {
	// Get basic mappings
	basicMappings := p.getBasicOperationMappings()

	// Get extended mappings from gitlab_operations.go
	extendedMappings := getExtendedOperationMappings()

	// Merge them together
	return mergeOperationMappings(basicMappings, extendedMappings)
}

// getBasicOperationMappings returns the basic set of GitLab operations
func (p *GitLabProvider) getBasicOperationMappings() map[string]providers.OperationMapping {
	mappings := make(map[string]providers.OperationMapping)

	// Projects mappings
	mappings["projects/list"] = providers.OperationMapping{
		OperationID:    "listProjects",
		Method:         "GET",
		PathTemplate:   "/projects",
		RequiredParams: []string{},
		OptionalParams: []string{"owned", "membership", "search", "per_page", "page"},
	}
	mappings["projects/get"] = providers.OperationMapping{
		OperationID:    "getProject",
		Method:         "GET",
		PathTemplate:   "/projects/{id}",
		RequiredParams: []string{"id"},
		OptionalParams: []string{},
	}
	mappings["projects/create"] = providers.OperationMapping{
		OperationID:    "createProject",
		Method:         "POST",
		PathTemplate:   "/projects",
		RequiredParams: []string{"name"},
		OptionalParams: []string{"path", "namespace_id", "description", "visibility"},
	}

	// Issues mappings
	mappings["issues/list"] = providers.OperationMapping{
		OperationID:    "listIssues",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/issues",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"state", "labels", "milestone", "per_page"},
	}
	mappings["issues/get"] = providers.OperationMapping{
		OperationID:    "getIssue",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/issues/{issue_iid}",
		RequiredParams: []string{"id", "issue_iid"},
		OptionalParams: []string{},
	}
	mappings["issues/create"] = providers.OperationMapping{
		OperationID:    "createIssue",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/issues",
		RequiredParams: []string{"id", "title"},
		OptionalParams: []string{"description", "labels", "assignee_ids"},
	}

	// Merge Requests mappings
	mappings["merge_requests/list"] = providers.OperationMapping{
		OperationID:    "listMergeRequests",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/merge_requests",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"state", "source_branch", "target_branch", "per_page"},
	}
	mappings["merge_requests/get"] = providers.OperationMapping{
		OperationID:    "getMergeRequest",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/merge_requests/{merge_request_iid}",
		RequiredParams: []string{"id", "merge_request_iid"},
		OptionalParams: []string{},
	}
	mappings["merge_requests/create"] = providers.OperationMapping{
		OperationID:    "createMergeRequest",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/merge_requests",
		RequiredParams: []string{"id", "source_branch", "target_branch", "title"},
		OptionalParams: []string{"description"},
	}

	// Pipelines mappings
	mappings["pipelines/list"] = providers.OperationMapping{
		OperationID:    "listPipelines",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/pipelines",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"status", "ref", "per_page"},
	}
	mappings["pipelines/get"] = providers.OperationMapping{
		OperationID:    "getPipeline",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/pipelines/{pipeline_id}",
		RequiredParams: []string{"id", "pipeline_id"},
		OptionalParams: []string{},
	}
	mappings["pipelines/trigger"] = providers.OperationMapping{
		OperationID:    "triggerPipeline",
		Method:         "POST",
		PathTemplate:   "/projects/{id}/pipeline",
		RequiredParams: []string{"id", "ref"},
		OptionalParams: []string{"variables"},
	}

	// Jobs mappings
	mappings["jobs/list"] = providers.OperationMapping{
		OperationID:    "listJobs",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/jobs",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"scope", "per_page"},
	}

	// Repository mappings
	mappings["branches/list"] = providers.OperationMapping{
		OperationID:    "listBranches",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/branches",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"search"},
	}
	mappings["commits/list"] = providers.OperationMapping{
		OperationID:    "listCommits",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/commits",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"ref_name", "since", "until", "per_page"},
	}
	mappings["tags/list"] = providers.OperationMapping{
		OperationID:    "listTags",
		Method:         "GET",
		PathTemplate:   "/projects/{id}/repository/tags",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"search"},
	}

	// Groups mappings
	mappings["groups/list"] = providers.OperationMapping{
		OperationID:    "listGroups",
		Method:         "GET",
		PathTemplate:   "/groups",
		RequiredParams: []string{},
		OptionalParams: []string{"search", "owned", "per_page"},
	}
	mappings["groups/get"] = providers.OperationMapping{
		OperationID:    "getGroup",
		Method:         "GET",
		PathTemplate:   "/groups/{id}",
		RequiredParams: []string{"id"},
		OptionalParams: []string{"with_projects"},
	}

	// Users mappings
	mappings["users/current"] = providers.OperationMapping{
		OperationID:    "getCurrentUser",
		Method:         "GET",
		PathTemplate:   "/user",
		RequiredParams: []string{},
		OptionalParams: []string{},
	}

	return mappings
}

// ExecuteOperation executes a GitLab operation
func (p *GitLabProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize operation name
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	// Handle special operations that need parameter injection
	switch operation {
	case "issues/close":
		// Inject state_event parameter for closing issue
		if params == nil {
			params = make(map[string]interface{})
		}
		params["state_event"] = "close"
		operation = "issues/update" // Use update endpoint with state_event

	case "issues/reopen":
		// Inject state_event parameter for reopening issue
		if params == nil {
			params = make(map[string]interface{})
		}
		params["state_event"] = "reopen"
		operation = "issues/update" // Use update endpoint with state_event

	case "merge_requests/close":
		// Inject state_event parameter for closing merge request
		if params == nil {
			params = make(map[string]interface{})
		}
		params["state_event"] = "close"
		operation = "merge_requests/update" // Use update endpoint with state_event
	}

	// Ensure pass-through authentication is maintained
	// The context should already have credentials from ValidateCredentials
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx.Credentials == nil {
		return nil, fmt.Errorf("no credentials found in context for pass-through authentication")
	}

	// Use base provider's execution which handles auth from context
	return p.Execute(ctx, operation, params)
}

// Execute overrides BaseProvider's Execute to handle GitLab-specific responses like 204 No Content
func (p *GitLabProvider) Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	mappings := p.GetOperationMappings()
	mapping, exists := mappings[operation]
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
			}
		}
	}

	// For GET requests, collect query parameters
	if mapping.Method == "GET" || mapping.Method == "HEAD" {
		// Add optional parameters as query params
		for _, param := range mapping.OptionalParams {
			if value, ok := params[param]; ok {
				queryParams[param] = fmt.Sprintf("%v", value)
			}
		}

		// Also check for common pagination parameters even if not in OptionalParams
		for _, param := range []string{"per_page", "page", "limit", "offset", "sort", "direction"} {
			if value, ok := params[param]; ok {
				queryParams[param] = fmt.Sprintf("%v", value)
			}
		}

		// Build query string with proper URL encoding
		if len(queryParams) > 0 {
			values := url.Values{}
			for k, v := range queryParams {
				values.Add(k, v)
			}
			path = path + "?" + values.Encode()
		}
	}

	// Prepare body for POST/PUT/PATCH methods
	var body interface{}
	if mapping.Method == "POST" || mapping.Method == "PUT" || mapping.Method == "PATCH" {
		body = params
	}

	// Execute HTTP request
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
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle empty response body (some operations return empty 200 OK)
	if len(responseBody) == 0 {
		return map[string]interface{}{"success": true, "status": resp.StatusCode}, nil
	}

	// Parse JSON response
	var result interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// GetDefaultConfiguration returns the default GitLab configuration
func (p *GitLabProvider) GetDefaultConfiguration() providers.ProviderConfig {
	return providers.ProviderConfig{
		BaseURL:  p.baseURL,
		AuthType: "bearer", // GitLab uses Bearer token authentication
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 600, // GitLab allows 600 requests per minute for authenticated users
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnRateLimit: true,
			RetryOnTimeout:   true,
			RetryableErrors:  []string{"500", "502", "503", "504"},
		},
		OperationGroups: p.getOperationGroups(),
	}
}

// getOperationGroups returns operation groups for GitLab
func (p *GitLabProvider) getOperationGroups() []providers.OperationGroup {
	var groups []providers.OperationGroup

	if p.enabledModules[ModuleProjects] {
		groups = append(groups, providers.OperationGroup{
			Name:        "projects",
			DisplayName: "Project Management",
			Description: "Manage GitLab projects",
			Operations: []string{
				"projects/list", "projects/get", "projects/create",
			},
		})
	}

	if p.enabledModules[ModuleIssues] {
		groups = append(groups, providers.OperationGroup{
			Name:        "issues",
			DisplayName: "Issue Management",
			Description: "Manage GitLab issues",
			Operations: []string{
				"issues/list", "issues/get", "issues/create",
			},
		})
	}

	if p.enabledModules[ModuleMergeRequests] {
		groups = append(groups, providers.OperationGroup{
			Name:        "merge_requests",
			DisplayName: "Merge Request Management",
			Description: "Manage GitLab merge requests",
			Operations: []string{
				"merge_requests/list", "merge_requests/get", "merge_requests/create",
			},
		})
	}

	if p.enabledModules[ModulePipelines] {
		groups = append(groups, providers.OperationGroup{
			Name:        "pipelines",
			DisplayName: "CI/CD Pipelines",
			Description: "Manage GitLab CI/CD pipelines",
			Operations: []string{
				"pipelines/list", "pipelines/get", "pipelines/trigger",
			},
		})
	}

	if p.enabledModules[ModuleJobs] {
		groups = append(groups, providers.OperationGroup{
			Name:        "jobs",
			DisplayName: "CI/CD Jobs",
			Description: "Manage CI/CD jobs",
			Operations: []string{
				"jobs/list",
			},
		})
	}

	if p.enabledModules[ModuleRepositories] {
		groups = append(groups, providers.OperationGroup{
			Name:        "repository",
			DisplayName: "Repository Management",
			Description: "Manage repository branches, tags, and commits",
			Operations: []string{
				"branches/list", "commits/list", "tags/list",
			},
		})
	}

	if p.enabledModules[ModuleGroups] {
		groups = append(groups, providers.OperationGroup{
			Name:        "groups",
			DisplayName: "Group Management",
			Description: "Manage GitLab groups",
			Operations: []string{
				"groups/list", "groups/get",
			},
		})
	}

	if p.enabledModules[ModuleUsers] {
		groups = append(groups, providers.OperationGroup{
			Name:        "users",
			DisplayName: "User Management",
			Description: "Manage GitLab users",
			Operations: []string{
				"users/current",
			},
		})
	}

	return groups
}

// GetAIOptimizedDefinitions returns AI-optimized definitions for GitLab tools
func (p *GitLabProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return GetGitLabAIDefinitions(p.enabledModules)
}

// GetOpenAPISpec returns the GitLab OpenAPI specification
func (p *GitLabProvider) GetOpenAPISpec() (*openapi3.T, error) {
	// Try cache first
	if p.specCache != nil {
		ctx := context.Background()
		spec, err := p.specCache.Get(ctx, "gitlab-v4")
		if err == nil && spec != nil {
			return spec, nil
		}
	}

	// Use embedded spec as it's comprehensive
	if p.specFallback != nil {
		return p.specFallback, nil
	}

	// If no fallback, try to load embedded spec
	if len(gitlabOpenAPISpecJSON) > 0 {
		// The GitLab spec may have compatibility issues with kin-openapi
		// For now, we'll return a minimal spec that allows the provider to work
		loader := openapi3.NewLoader()
		// Create a minimal valid spec for testing
		minimalSpec := []byte(`{
			"openapi": "3.0.0",
			"info": {
				"title": "GitLab API",
				"version": "v4"
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
			ctx := context.Background()
			_ = p.specCache.Set(ctx, "gitlab-v4", spec, 24*time.Hour)
		}

		return spec, nil
	}

	return nil, fmt.Errorf("no OpenAPI spec available")
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *GitLabProvider) GetEmbeddedSpecVersion() string {
	return "v4-2024"
}

// SetConfiguration sets the provider configuration
func (p *GitLabProvider) SetConfiguration(config providers.ProviderConfig) {
	p.BaseProvider.SetConfiguration(config)
	if config.BaseURL != "" {
		p.baseURL = config.BaseURL
		// Extract instance URL from base URL
		if strings.Contains(config.BaseURL, "/api/") {
			parts := strings.Split(config.BaseURL, "/api/")
			if len(parts) > 0 {
				p.instanceURL = parts[0]
			}
		}
	}
}

// HealthCheck verifies the GitLab API is accessible
func (p *GitLabProvider) HealthCheck(ctx context.Context) error {
	// Use ExecuteHTTPRequest which properly handles the base URL
	resp, err := p.ExecuteHTTPRequest(ctx, "GET", "version", nil, nil)
	if err != nil {
		return fmt.Errorf("GitLab API health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// For GitLab, anything other than 200 is considered unhealthy
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitLab API health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *GitLabProvider) Close() error {
	// Currently no resources to clean up
	return nil
}

// FilterOperationsByPermissions filters operations based on user's permissions
func (p *GitLabProvider) FilterOperationsByPermissions(operations []string, permissions map[string]interface{}) []string {
	if permissions == nil {
		// If no permissions provided, return read-only operations
		return p.filterReadOnlyOperations(operations)
	}

	// Extract scopes and access level from permissions
	scopes := p.extractScopes(permissions)
	accessLevel := p.extractAccessLevel(permissions)

	filtered := []string{}
	for _, op := range operations {
		if p.isOperationAllowed(op, scopes, accessLevel) {
			filtered = append(filtered, op)
		}
	}

	return filtered
}

// extractScopes extracts OAuth scopes from permissions
func (p *GitLabProvider) extractScopes(permissions map[string]interface{}) []string {
	scopes := []string{}

	if scopesRaw, ok := permissions["scopes"]; ok {
		switch v := scopesRaw.(type) {
		case []string:
			scopes = v
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					scopes = append(scopes, str)
				}
			}
		case string:
			// Single scope as string
			scopes = []string{v}
		}
	}

	return scopes
}

// extractAccessLevel extracts the user's access level from permissions
func (p *GitLabProvider) extractAccessLevel(permissions map[string]interface{}) int {
	if accessLevelRaw, ok := permissions["access_level"]; ok {
		switch v := accessLevelRaw.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// Map string access levels to numeric values
			switch v {
			case "owner":
				return 50
			case "maintainer", "master":
				return 40
			case "developer":
				return 30
			case "reporter":
				return 20
			case "guest":
				return 10
			default:
				return 0
			}
		}
	}
	return 0 // No access by default
}

// isOperationAllowed checks if an operation is allowed based on scopes and access level
func (p *GitLabProvider) isOperationAllowed(operation string, scopes []string, accessLevel int) bool {
	// Define operation requirements
	operationRequirements := map[string]struct {
		minAccessLevel int
		requiredScopes []string
	}{
		// Project operations
		"projects/list":      {0, []string{"read_api", "api"}},
		"projects/get":       {0, []string{"read_api", "api"}},
		"projects/create":    {0, []string{"api"}},
		"projects/update":    {30, []string{"api"}}, // Developer
		"projects/delete":    {50, []string{"api"}}, // Owner
		"projects/fork":      {0, []string{"api"}},
		"projects/star":      {0, []string{"api"}},
		"projects/unstar":    {0, []string{"api"}},
		"projects/archive":   {40, []string{"api"}}, // Maintainer
		"projects/unarchive": {40, []string{"api"}}, // Maintainer

		// Issue operations
		"issues/list":   {0, []string{"read_api", "api"}},
		"issues/get":    {0, []string{"read_api", "api"}},
		"issues/create": {10, []string{"api"}}, // Guest
		"issues/update": {20, []string{"api"}}, // Reporter
		"issues/close":  {20, []string{"api"}}, // Reporter
		"issues/reopen": {20, []string{"api"}}, // Reporter
		"issues/delete": {40, []string{"api"}}, // Maintainer

		// Merge Request operations
		"merge_requests/list":      {0, []string{"read_api", "api"}},
		"merge_requests/get":       {0, []string{"read_api", "api"}},
		"merge_requests/create":    {30, []string{"api"}}, // Developer
		"merge_requests/update":    {30, []string{"api"}}, // Developer
		"merge_requests/approve":   {30, []string{"api"}}, // Developer
		"merge_requests/unapprove": {30, []string{"api"}}, // Developer
		"merge_requests/merge":     {30, []string{"api"}}, // Developer (with merge permissions)
		"merge_requests/close":     {30, []string{"api"}}, // Developer
		"merge_requests/rebase":    {30, []string{"api"}}, // Developer
		"merge_requests/delete":    {40, []string{"api"}}, // Maintainer

		// Pipeline operations
		"pipelines/list":    {0, []string{"read_api", "api"}},
		"pipelines/get":     {0, []string{"read_api", "api"}},
		"pipelines/trigger": {30, []string{"api"}}, // Developer
		"pipelines/cancel":  {30, []string{"api"}}, // Developer
		"pipelines/retry":   {30, []string{"api"}}, // Developer
		"pipelines/delete":  {40, []string{"api"}}, // Maintainer

		// Job operations
		"jobs/list":      {0, []string{"read_api", "api"}},
		"jobs/get":       {0, []string{"read_api", "api"}},
		"jobs/cancel":    {30, []string{"api"}},             // Developer
		"jobs/retry":     {30, []string{"api"}},             // Developer
		"jobs/play":      {30, []string{"api"}},             // Developer
		"jobs/artifacts": {20, []string{"read_api", "api"}}, // Reporter
		"jobs/erase":     {40, []string{"api"}},             // Maintainer

		// Repository operations
		"branches/list":      {0, []string{"read_repository", "api"}},
		"branches/get":       {0, []string{"read_repository", "api"}},
		"branches/create":    {30, []string{"write_repository", "api"}}, // Developer
		"branches/delete":    {40, []string{"api"}},                     // Maintainer
		"branches/protect":   {40, []string{"api"}},                     // Maintainer
		"branches/unprotect": {40, []string{"api"}},                     // Maintainer

		"commits/list":     {0, []string{"read_repository", "api"}},
		"commits/get":      {0, []string{"read_repository", "api"}},
		"commits/create":   {30, []string{"write_repository", "api"}}, // Developer
		"commits/diff":     {0, []string{"read_repository", "api"}},
		"commits/comments": {0, []string{"read_repository", "api"}},
		"commits/comment":  {20, []string{"api"}}, // Reporter

		"tags/list":   {0, []string{"read_repository", "api"}},
		"tags/get":    {0, []string{"read_repository", "api"}},
		"tags/create": {30, []string{"write_repository", "api"}}, // Developer
		"tags/delete": {40, []string{"api"}},                     // Maintainer

		"files/get":    {0, []string{"read_repository", "api"}},
		"files/raw":    {0, []string{"read_repository", "api"}},
		"files/create": {30, []string{"write_repository", "api"}}, // Developer
		"files/update": {30, []string{"write_repository", "api"}}, // Developer
		"files/delete": {30, []string{"write_repository", "api"}}, // Developer

		// Group operations
		"groups/list":   {0, []string{"read_api", "api"}},
		"groups/get":    {0, []string{"read_api", "api"}},
		"groups/create": {0, []string{"api"}},  // Any authenticated user can create groups
		"groups/update": {50, []string{"api"}}, // Owner
		"groups/delete": {50, []string{"api"}}, // Owner

		// Wiki operations
		"wikis/list":   {0, []string{"read_wiki", "api"}},
		"wikis/get":    {0, []string{"read_wiki", "api"}},
		"wikis/create": {30, []string{"api"}}, // Developer
		"wikis/update": {30, []string{"api"}}, // Developer
		"wikis/delete": {30, []string{"api"}}, // Developer

		// Snippet operations
		"snippets/list":   {0, []string{"read_api", "api"}},
		"snippets/get":    {0, []string{"read_api", "api"}},
		"snippets/create": {20, []string{"api"}}, // Reporter
		"snippets/update": {20, []string{"api"}}, // Reporter
		"snippets/delete": {20, []string{"api"}}, // Reporter

		// User operations
		"users/current": {0, []string{"read_user", "api"}},
	}

	req, exists := operationRequirements[operation]
	if !exists {
		// Default: allow read operations with read scopes
		if strings.Contains(operation, "/list") || strings.Contains(operation, "/get") {
			return p.hasAnyScope(scopes, []string{"read_api", "api"})
		}
		return false
	}

	// Check access level
	if accessLevel < req.minAccessLevel {
		return false
	}

	// Check scopes
	return p.hasAnyScope(scopes, req.requiredScopes)
}

// hasAnyScope checks if the user has any of the required scopes
func (p *GitLabProvider) hasAnyScope(userScopes []string, requiredScopes []string) bool {
	for _, required := range requiredScopes {
		for _, userScope := range userScopes {
			if userScope == required {
				return true
			}
		}
	}
	return false
}

// filterReadOnlyOperations filters to only read operations
func (p *GitLabProvider) filterReadOnlyOperations(operations []string) []string {
	readOnly := []string{}
	for _, op := range operations {
		if strings.Contains(op, "/list") || strings.Contains(op, "/get") ||
			op == "users/current" {
			readOnly = append(readOnly, op)
		}
	}
	return readOnly
}

// SetEnabledModules configures which GitLab modules are enabled
func (p *GitLabProvider) SetEnabledModules(modules []GitLabModule) {
	// Disable all modules first
	for module := range p.enabledModules {
		p.enabledModules[module] = false
	}

	// Enable specified modules
	for _, module := range modules {
		p.enabledModules[module] = true
	}
}

// GetEnabledModules returns the list of enabled modules
func (p *GitLabProvider) GetEnabledModules() []GitLabModule {
	var modules []GitLabModule
	for module, enabled := range p.enabledModules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}

// normalizeOperationName normalizes operation names to handle various formats
func (p *GitLabProvider) normalizeOperationName(op string) string {
	// GitLab uses specific naming patterns like "merge_requests" as a single entity
	// We need to preserve these while normalizing the operation format

	// First, handle known GitLab entities that should not be split
	// These are compound words that represent single resources in GitLab
	preservedEntities := []string{
		"merge_requests",
		"project_members",
		"group_members",
		"protected_branches",
		"protected_tags",
		"container_registry",
		"security_reports",
	}

	// Temporarily replace preserved entities with placeholders
	placeholders := make(map[string]string)
	for i, entity := range preservedEntities {
		placeholder := fmt.Sprintf("§PLACEHOLDER%d§", i) // Use § to avoid conflicts
		if strings.Contains(op, entity) {
			op = strings.ReplaceAll(op, entity, placeholder)
			placeholders[placeholder] = entity
		}
	}

	// Now normalize: replace hyphens and remaining underscores with forward slashes
	normalized := strings.ReplaceAll(op, "-", "/")
	normalized = strings.ReplaceAll(normalized, "_", "/")

	// Restore preserved entities
	for placeholder, entity := range placeholders {
		normalized = strings.ReplaceAll(normalized, placeholder, entity)
	}

	return normalized
}
