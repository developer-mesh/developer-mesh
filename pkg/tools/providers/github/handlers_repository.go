package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Repository Handlers

// ListRepositoriesHandler handles listing repositories
type ListRepositoriesHandler struct {
	provider *GitHubProvider
}

func NewListRepositoriesHandler(p *GitHubProvider) *ListRepositoriesHandler {
	return &ListRepositoriesHandler{provider: p}
}

func (h *ListRepositoriesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_repositories",
		Description: "List repositories for a user, organization, or authenticated user with pagination and filtering options",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type":        "string",
					"description": "Username to list repositories for (e.g., 'octocat', 'torvalds'). Leave empty for authenticated user repositories",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
				},
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name to list repositories for (e.g., 'github', 'microsoft'). Cannot be used with 'user' parameter",
					"example":     "github",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Filter repositories by type. Defaults to 'all'",
					"enum":        []string{"all", "owner", "public", "private", "member"},
					"default":     "all",
					"example":     "public",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort repositories by field. Defaults to 'updated'",
					"enum":        []string{"created", "updated", "pushed", "full_name"},
					"default":     "updated",
					"example":     "updated",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "Sort direction. Defaults to 'desc'",
					"enum":        []string{"asc", "desc"},
					"default":     "desc",
					"example":     "desc",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100). Defaults to 30",
					"minimum":     1,
					"maximum":     100,
					"default":     30,
					"example":     30,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve (1-based). Defaults to 1",
					"minimum":     1,
					"default":     1,
					"example":     1,
				},
			},
			"required": []interface{}{},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo", "public_repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"responseExample": map[string]interface{}{
				"success": []map[string]interface{}{
					{
						"id":        1296269,
						"name":      "Hello-World",
						"full_name": "octocat/Hello-World",
						"private":   false,
						"html_url":  "https://github.com/octocat/Hello-World",
					},
				},
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/repos/repos#list-repositories-for-a-user",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "User or organization not found or private",
					"solution": "Verify the username/org exists and you have permission to view their repositories",
				},
				{
					"code":     403,
					"reason":   "Rate limit exceeded or insufficient permissions",
					"solution": "Wait for rate limit reset or ensure proper authentication scopes",
				},
			},
			"extendedHelp": "This operation lists repositories based on visibility and your access level. For organizations, you need appropriate membership or public visibility. See https://docs.github.com/rest/repos/repos#list-repositories-for-a-user",
		},
	}
}

func (h *ListRepositoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	opts := &github.RepositoryListOptions{}
	if typ, ok := params["type"].(string); ok {
		opts.Type = typ
	}
	if sort, ok := params["sort"].(string); ok {
		opts.Sort = sort
	}
	if direction, ok := params["direction"].(string); ok {
		opts.Direction = direction
	}

	pagination := ExtractPagination(params)
	opts.ListOptions = github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	var repos []*github.Repository
	var err error

	if org, ok := params["org"].(string); ok && org != "" {
		// List organization repositories
		orgOpts := &github.RepositoryListByOrgOptions{
			Type:        opts.Type,
			Sort:        opts.Sort,
			Direction:   opts.Direction,
			ListOptions: opts.ListOptions,
		}
		repos, _, err = client.Repositories.ListByOrg(ctx, org, orgOpts)
	} else if user, ok := params["user"].(string); ok && user != "" {
		// List user repositories
		userOpts := &github.RepositoryListByUserOptions{
			Type:        opts.Type,
			Sort:        opts.Sort,
			Direction:   opts.Direction,
			ListOptions: opts.ListOptions,
		}
		repos, _, err = client.Repositories.ListByUser(ctx, user, userOpts)
	} else {
		// List authenticated user's repositories
		authOpts := &github.RepositoryListByAuthenticatedUserOptions{
			Type:        opts.Type,
			Sort:        opts.Sort,
			Direction:   opts.Direction,
			ListOptions: opts.ListOptions,
		}
		repos, _, err = client.Repositories.ListByAuthenticatedUser(ctx, authOpts)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list repositories: %v", err)), nil
	}

	data, _ := json.Marshal(repos)
	return NewToolResult(string(data)), nil
}

// GetRepositoryHandler handles getting a specific repository
type GetRepositoryHandler struct {
	provider *GitHubProvider
}

func NewGetRepositoryHandler(p *GitHubProvider) *GetRepositoryHandler {
	return &GetRepositoryHandler{provider: p}
}

func (h *GetRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_repository",
		Description: "Get detailed information about a specific GitHub repository including metadata, permissions, and statistics",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'octocat', 'github')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'Hello-World', 'linux')",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
				},
			},
			"required": []interface{}{"owner", "repo"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo", "public_repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"id":                1296269,
					"name":              "Hello-World",
					"full_name":         "octocat/Hello-World",
					"private":           false,
					"html_url":          "https://github.com/octocat/Hello-World",
					"description":       "This your first repo!",
					"language":          "C",
					"stargazers_count":  80,
					"forks_count":       9,
					"open_issues_count": 0,
					"default_branch":    "main",
				},
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/repos/repos#get-a-repository",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "Repository not found, private, or you lack access",
					"solution": "Verify repository exists and you have read permission. For private repos, ensure proper authentication",
				},
				{
					"code":     403,
					"reason":   "Rate limit exceeded or repository access restricted",
					"solution": "Wait for rate limit reset or check if repository requires specific permissions",
				},
			},
			"extendedHelp": "This operation returns comprehensive repository information. For private repositories, you need appropriate access permissions. See https://docs.github.com/rest/repos/repos#get-a-repository",
		},
	}
}

func (h *GetRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Debug logging
	h.provider.logger.Debug("GetRepositoryHandler.Execute called", map[string]interface{}{
		"params":      params,
		"has_context": ctx != nil,
	})

	// Check cache first if enabled
	if h.provider.cacheEnabled && h.provider.cache != nil {
		owner := extractString(params, "owner")
		repo := extractString(params, "repo")
		cacheKey := BuildRepositoryCacheKey(owner, repo, "get")

		if cached, found := h.provider.cache.Get(cacheKey); found {
			h.provider.logger.Debug("Cache hit for repository", map[string]interface{}{
				"owner": owner,
				"repo":  repo,
			})
			if result, ok := cached.(*ToolResult); ok {
				return result, nil
			}
		}
	}

	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		h.provider.logger.Error("GitHub client not found in context", map[string]interface{}{
			"context_keys": fmt.Sprintf("%v", ctx),
		})
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get repository: %v", err)), nil
	}

	data, _ := json.Marshal(repository)
	result := NewToolResult(string(data))

	// Cache the successful result
	if h.provider.cacheEnabled && h.provider.cache != nil && !result.IsError {
		cacheKey := BuildRepositoryCacheKey(owner, repo, "get")
		h.provider.cache.Set(cacheKey, result, GetRecommendedTTL("repositories"))
		h.provider.logger.Debug("Cached repository", map[string]interface{}{
			"owner": owner,
			"repo":  repo,
			"ttl":   GetRecommendedTTL("repositories").String(),
		})
	}

	return result, nil
}

// UpdateRepositoryHandler handles updating repository settings
type UpdateRepositoryHandler struct {
	provider *GitHubProvider
}

func NewUpdateRepositoryHandler(p *GitHubProvider) *UpdateRepositoryHandler {
	return &UpdateRepositoryHandler{provider: p}
}

func (h *UpdateRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_repository",
		Description: "Update repository settings including name, description, visibility, and merge options (requires admin permissions)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'octocat', 'github')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Current repository name (e.g., 'Hello-World')",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "New repository name (must be unique for owner). Leave empty to keep current name",
					"example":     "my-renamed-repo",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"maxLength":   100,
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Repository description (max 350 characters)",
					"example":     "An updated description of my project",
					"maxLength":   350,
				},
				"homepage": map[string]interface{}{
					"type":        "string",
					"description": "Repository homepage URL (must be valid URL)",
					"example":     "https://example.com",
					"format":      "uri",
				},
				"private": map[string]interface{}{
					"type":        "boolean",
					"description": "Make repository private (requires paid plan) or public",
					"example":     false,
				},
				"has_issues": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable or disable GitHub Issues for this repository",
					"default":     true,
					"example":     true,
				},
				"has_projects": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable or disable GitHub Projects for this repository",
					"default":     true,
					"example":     false,
				},
				"has_wiki": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable or disable GitHub Wiki for this repository",
					"default":     true,
					"example":     true,
				},
				"default_branch": map[string]interface{}{
					"type":        "string",
					"description": "Default branch name (branch must exist before setting as default)",
					"example":     "main",
					"pattern":     "^[a-zA-Z0-9._/-]+$",
				},
				"allow_squash_merge": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow squash merging for pull requests",
					"default":     true,
					"example":     true,
				},
				"allow_merge_commit": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow merge commits for pull requests",
					"default":     true,
					"example":     false,
				},
				"allow_rebase_merge": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow rebase merging for pull requests",
					"default":     true,
					"example":     true,
				},
				"delete_branch_on_merge": map[string]interface{}{
					"type":        "boolean",
					"description": "Automatically delete head branches after pull requests are merged",
					"default":     false,
					"example":     true,
				},
				"archived": map[string]interface{}{
					"type":        "boolean",
					"description": "Archive repository (makes it read-only). Cannot be undone via API",
					"default":     false,
					"example":     false,
				},
			},
			"required": []interface{}{"owner", "repo"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo"},
				"minimumScopes":     []string{"repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"id":             1296269,
					"name":           "Hello-World-Updated",
					"full_name":      "octocat/Hello-World-Updated",
					"description":    "Updated description",
					"private":        false,
					"default_branch": "main",
				},
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/repos/repos#update-a-repository",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "Repository not found or insufficient permissions",
					"solution": "Verify repository exists and you have admin permissions to modify settings",
				},
				{
					"code":     422,
					"reason":   "Invalid repository name or settings",
					"solution": "Ensure new name is unique and follows GitHub naming conventions",
				},
				{
					"code":     403,
					"reason":   "Insufficient permissions to modify repository",
					"solution": "You need admin permissions on the repository to update settings",
				},
			},
			"extendedHelp": "Updates repository settings and metadata. Requires admin permissions. Some changes like making a repository private may require a paid plan. Archived repositories become read-only. See https://docs.github.com/rest/repos/repos#update-a-repository",
		},
	}
}

func (h *UpdateRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repoName := extractString(params, "repo")

	repo := &github.Repository{}

	if name, ok := params["name"].(string); ok {
		repo.Name = &name
	}
	if description, ok := params["description"].(string); ok {
		repo.Description = &description
	}
	if homepage, ok := params["homepage"].(string); ok {
		repo.Homepage = &homepage
	}
	if private, ok := params["private"].(bool); ok {
		repo.Private = &private
	}
	if hasIssues, ok := params["has_issues"].(bool); ok {
		repo.HasIssues = &hasIssues
	}
	if hasProjects, ok := params["has_projects"].(bool); ok {
		repo.HasProjects = &hasProjects
	}
	if hasWiki, ok := params["has_wiki"].(bool); ok {
		repo.HasWiki = &hasWiki
	}
	if defaultBranch, ok := params["default_branch"].(string); ok {
		repo.DefaultBranch = &defaultBranch
	}
	if allowSquash, ok := params["allow_squash_merge"].(bool); ok {
		repo.AllowSquashMerge = &allowSquash
	}
	if allowMerge, ok := params["allow_merge_commit"].(bool); ok {
		repo.AllowMergeCommit = &allowMerge
	}
	if allowRebase, ok := params["allow_rebase_merge"].(bool); ok {
		repo.AllowRebaseMerge = &allowRebase
	}
	if deleteBranch, ok := params["delete_branch_on_merge"].(bool); ok {
		repo.DeleteBranchOnMerge = &deleteBranch
	}
	if archived, ok := params["archived"].(bool); ok {
		repo.Archived = &archived
	}

	updated, _, err := client.Repositories.Edit(ctx, owner, repoName, repo)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update repository: %v", err)), nil
	}

	data, _ := json.Marshal(updated)
	return NewToolResult(string(data)), nil
}

// DeleteRepositoryHandler handles deleting a repository
type DeleteRepositoryHandler struct {
	provider *GitHubProvider
}

func NewDeleteRepositoryHandler(p *GitHubProvider) *DeleteRepositoryHandler {
	return &DeleteRepositoryHandler{provider: p}
}

func (h *DeleteRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_repository",
		Description: "⚠️  DESTRUCTIVE: Permanently delete a repository and all its data (requires admin permissions). This action cannot be undone.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'octocat', 'github')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name to delete (e.g., 'Hello-World'). THIS WILL BE PERMANENTLY DELETED",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
				},
			},
			"required": []interface{}{"owner", "repo"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"delete_repo"},
				"minimumScopes":     []string{"delete_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
				"destructive":       true,
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"status":     "deleted",
					"repository": "octocat/Hello-World",
					"message":    "Repository successfully deleted",
				},
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/repos/repos#delete-a-repository",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "Repository not found or you lack admin access",
					"solution": "Verify repository exists and you have admin/owner permissions",
				},
				{
					"code":     403,
					"reason":   "Insufficient permissions to delete repository",
					"solution": "You need admin/owner permissions and the 'delete_repo' scope to delete repositories",
				},
				{
					"code":     422,
					"reason":   "Repository cannot be deleted (may have restrictions)",
					"solution": "Check for branch protection rules or organization policies preventing deletion",
				},
			},
			"extendedHelp": "🚨 WARNING: This permanently deletes the repository and ALL its data including code, issues, pull requests, and wiki. This action CANNOT be undone. Requires admin/owner permissions and 'delete_repo' OAuth scope. Consider archiving instead of deleting. See https://docs.github.com/rest/repos/repos#delete-a-repository",
		},
	}
}

func (h *DeleteRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	_, err := client.Repositories.Delete(ctx, owner, repo)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete repository: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status":     "deleted",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
	}), nil
}

// SearchRepositoriesHandler handles repository search
type SearchRepositoriesHandler struct {
	provider *GitHubProvider
}

func NewSearchRepositoriesHandler(p *GitHubProvider) *SearchRepositoriesHandler {
	return &SearchRepositoriesHandler{provider: p}
}

func (h *SearchRepositoriesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_repositories",
		Description: "Search for repositories on GitHub using advanced search syntax with filters for language, stars, forks, and more",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax. Examples: 'react language:javascript', 'user:octocat stars:>100', 'machine learning created:>2020-01-01'",
					"example":     "react language:javascript stars:>1000",
					"minLength":   1,
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort search results by relevance, stars, forks, or updated date",
					"enum":        []string{"stars", "forks", "updated", ""},
					"default":     "",
					"example":     "stars",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Sort order for results. Defaults to 'desc'",
					"enum":        []string{"asc", "desc"},
					"default":     "desc",
					"example":     "desc",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100). Defaults to 30",
					"minimum":     1,
					"maximum":     100,
					"default":     30,
					"example":     30,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve (1-based). Defaults to 1",
					"minimum":     1,
					"default":     1,
					"example":     1,
				},
			},
			"required": []interface{}{"query"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"public_repo"},
				"minimumScopes":     []string{},
				"rateLimitCategory": "search",
				"requestsPerMinute": 30,
				"requestsPerHour":   1800,
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"total_count": 12345,
					"items": []map[string]interface{}{
						{
							"id":               3081286,
							"name":             "Tetris",
							"full_name":        "dtrupenn/Tetris",
							"description":      "A C implementation of Tetris using Pennsim through LC4",
							"language":         "C",
							"stargazers_count": 1,
						},
					},
				},
				"error": map[string]interface{}{
					"message":           "Validation Failed",
					"documentation_url": "https://docs.github.com/rest/search#search-repositories",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     422,
					"reason":   "Invalid search query syntax",
					"solution": "Check GitHub search syntax. Use 'language:python' not 'language=python'",
				},
				{
					"code":     403,
					"reason":   "Rate limit exceeded for search API",
					"solution": "Search API has lower rate limits (30 requests/minute). Wait before retrying",
				},
			},
			"searchHelp": map[string]interface{}{
				"commonPatterns": []string{
					"language:javascript - repositories written in JavaScript",
					"stars:>100 - repositories with more than 100 stars",
					"user:octocat - repositories owned by octocat",
					"org:github - repositories owned by GitHub organization",
					"created:>2020-01-01 - repositories created after Jan 1, 2020",
					"pushed:>2021-01-01 - repositories updated after Jan 1, 2021",
					"size:>1000 - repositories larger than 1000 KB",
					"fork:false - exclude forked repositories",
				},
			},
			"extendedHelp": "Searches repositories using GitHub's search syntax. Has lower rate limits than other APIs (30 requests/minute). Supports complex queries with filters for language, owner, stars, forks, size, and dates. See https://docs.github.com/search-github/searching-on-github/searching-for-repositories",
		},
	}
}

func (h *SearchRepositoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	// Check for both 'q' and 'query' for compatibility
	query := extractString(params, "q")
	if query == "" {
		query = extractString(params, "query")
	}
	if query == "" {
		return NewToolError("query parameter is required (use 'q' or 'query')"), nil
	}

	opts := &github.SearchOptions{}
	if sort := extractString(params, "sort"); sort != "" {
		opts.Sort = sort
	}
	if order := extractString(params, "order"); order != "" {
		opts.Order = order
	}

	// Use extractInt for pagination parameters with defaults
	perPage := extractInt(params, "per_page")
	if perPage == 0 {
		perPage = 30 // Default per_page
	} else if perPage > 100 {
		perPage = 100 // Max allowed by GitHub
	}
	opts.PerPage = perPage

	page := extractInt(params, "page")
	if page == 0 {
		page = 1 // Default to first page
	}
	opts.Page = page

	h.provider.logger.Info("Search repositories pagination", map[string]interface{}{
		"query":    query,
		"per_page": perPage,
		"page":     page,
	})

	result, _, err := client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search repositories: %v", err)), nil
	}

	h.provider.logger.Debug("Search repositories result", map[string]interface{}{
		"requested_per_page": opts.PerPage,
		"items_returned":     len(result.Repositories),
		"total_count":        *result.Total,
	})

	// Return simplified items with essential metadata only
	simplifiedRepos := make([]map[string]interface{}, 0, len(result.Repositories))
	for _, repo := range result.Repositories {
		simplifiedRepos = append(simplifiedRepos, simplifyRepository(repo))
	}

	response := map[string]interface{}{
		"items":       simplifiedRepos,
		"total_count": *result.Total,
		"has_more":    *result.Total > len(result.Repositories),
		"page":        opts.Page,
		"per_page":    opts.PerPage,
	}

	data, _ := json.Marshal(response)
	h.provider.logger.Debug("Response size", map[string]interface{}{
		"bytes": len(data),
	})
	return NewToolResult(string(data)), nil
}

// GetFileContentsHandler handles file content retrieval
type GetFileContentsHandler struct {
	provider *GitHubProvider
}

func NewGetFileContentsHandler(p *GitHubProvider) *GetFileContentsHandler {
	return &GetFileContentsHandler{provider: p}
}

func (h *GetFileContentsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_file_contents",
		Description: "Retrieve the raw contents of a file from a GitHub repository with support for different branches, tags, or commits",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'octocat', 'facebook')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'Hello-World', 'react')",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path within the repository (e.g., 'README.md', 'src/index.js', 'docs/api.md')",
					"example":     "README.md",
					"minLength":   1,
				},
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Git reference: branch name, tag, or commit SHA (defaults to repository's default branch)",
					"example":     "main",
					"default":     "",
				},
			},
			"required": []interface{}{"owner", "repo", "path"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo", "public_repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
				"maxFileSize":       "1MB",
			},
			"responseExample": map[string]interface{}{
				"success": "# Hello World\n\nThis is a sample README file.\n\n## Installation\n\nnpm install\n",
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/repos/contents#get-repository-content",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "File not found, repository not found, or no access",
					"solution": "Verify the file path exists in the specified branch/commit and you have read access to the repository",
				},
				{
					"code":     403,
					"reason":   "File too large or insufficient permissions",
					"solution": "Files larger than 1MB require Git Data API. Ensure you have read permissions for private repositories",
				},
				{
					"code":     422,
					"reason":   "Invalid reference (branch, tag, or SHA)",
					"solution": "Verify the branch name, tag, or commit SHA exists in the repository",
				},
			},
			"encodingInfo": map[string]interface{}{
				"textFiles":   "Returned as UTF-8 decoded text content",
				"binaryFiles": "Large binary files may return metadata only - use Git Data API for raw content",
				"maxSize":     "Files larger than 1MB require alternative API endpoints",
			},
			"extendedHelp": "Retrieves file content from any Git reference. For files larger than 1MB, use the Git Data API instead. Binary files are automatically detected and handled appropriately. See https://docs.github.com/rest/repos/contents#get-repository-content",
		},
	}
}

func (h *GetFileContentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	path := extractString(params, "path")

	opts := &github.RepositoryContentGetOptions{}
	if ref, ok := params["ref"].(string); ok {
		opts.Ref = ref
	}

	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get file contents: %v", err)), nil
	}

	if fileContent == nil {
		return NewToolError("File not found"), nil
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to decode file content: %v", err)), nil
	}

	return NewToolResult(content), nil
}

// ListCommitsHandler handles listing commits
type ListCommitsHandler struct {
	provider *GitHubProvider
}

func NewListCommitsHandler(p *GitHubProvider) *ListCommitsHandler {
	return &ListCommitsHandler{provider: p}
}

func (h *ListCommitsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_commits",
		Description: "List commits from a GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA or branch to start listing from",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Only commits containing this file path",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListCommitsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.CommitsListOptions{}
	if sha, ok := params["sha"].(string); ok {
		opts.SHA = sha
	}
	if path, ok := params["path"].(string); ok {
		opts.Path = path
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list commits: %v", err)), nil
	}

	// Create simplified response to reduce token usage
	simplified := make([]map[string]interface{}, 0, len(commits))
	for _, commit := range commits {
		simplified = append(simplified, simplifyCommit(commit))
	}

	data, _ := json.Marshal(simplified)
	return NewToolResult(string(data)), nil
}

// SearchCodeHandler handles code search
type SearchCodeHandler struct {
	provider *GitHubProvider
}

func NewSearchCodeHandler(p *GitHubProvider) *SearchCodeHandler {
	return &SearchCodeHandler{provider: p}
}

func (h *SearchCodeHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_code",
		Description: "Search for code on GitHub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub code search syntax",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort results by: indexed",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Order results: asc or desc",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"query"},
		},
	}
}

func (h *SearchCodeHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	// Check for both 'q' and 'query' for compatibility
	query := extractString(params, "q")
	if query == "" {
		query = extractString(params, "query")
	}
	if query == "" {
		return NewToolError("query parameter is required (use 'q' or 'query')"), nil
	}

	opts := &github.SearchOptions{}
	if sort := extractString(params, "sort"); sort != "" {
		opts.Sort = sort
	}
	if order := extractString(params, "order"); order != "" {
		opts.Order = order
	}

	// Use extractInt for pagination parameters with defaults
	perPage := extractInt(params, "per_page")
	if perPage == 0 {
		perPage = 30 // Default per_page
	} else if perPage > 100 {
		perPage = 100 // Max allowed by GitHub
	}
	opts.PerPage = perPage

	page := extractInt(params, "page")
	if page == 0 {
		page = 1 // Default to first page
	}
	opts.Page = page

	h.provider.logger.Info("Search code pagination", map[string]interface{}{
		"query":    query,
		"per_page": perPage,
		"page":     page,
	})

	result, _, err := client.Search.Code(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search code: %v", err)), nil
	}

	// Return simplified items with essential metadata only
	simplifiedResults := make([]map[string]interface{}, 0, len(result.CodeResults))
	for _, codeResult := range result.CodeResults {
		simplifiedResults = append(simplifiedResults, simplifyCodeResult(codeResult))
	}

	response := map[string]interface{}{
		"items":       simplifiedResults,
		"total_count": *result.Total,
		"has_more":    *result.Total > len(result.CodeResults),
		"page":        opts.Page,
		"per_page":    opts.PerPage,
	}

	data, _ := json.Marshal(response)
	return NewToolResult(string(data)), nil
}

// GetCommitHandler handles getting a specific commit
type GetCommitHandler struct {
	provider *GitHubProvider
}

func NewGetCommitHandler(p *GitHubProvider) *GetCommitHandler {
	return &GetCommitHandler{provider: p}
}

func (h *GetCommitHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_commit",
		Description: "Get a specific commit from a GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "Commit SHA",
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
		},
	}
}

func (h *GetCommitHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	sha := extractString(params, "sha")

	commit, _, err := client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get commit: %v", err)), nil
	}

	data, _ := json.Marshal(commit)
	return NewToolResult(string(data)), nil
}

// ListBranchesHandler handles listing branches
type ListBranchesHandler struct {
	provider *GitHubProvider
}

func NewListBranchesHandler(p *GitHubProvider) *ListBranchesHandler {
	return &ListBranchesHandler{provider: p}
}

func (h *ListBranchesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_branches",
		Description: "List branches in a GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"protected": map[string]interface{}{
					"type":        "boolean",
					"description": "List only protected branches",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListBranchesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.BranchListOptions{}
	if protected, ok := params["protected"].(bool); ok {
		opts.Protected = &protected
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	branches, _, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list branches: %v", err)), nil
	}

	data, _ := json.Marshal(branches)
	return NewToolResult(string(data)), nil
}

// CreateOrUpdateFileHandler handles file creation/update
type CreateOrUpdateFileHandler struct {
	provider *GitHubProvider
}

func NewCreateOrUpdateFileHandler(p *GitHubProvider) *CreateOrUpdateFileHandler {
	return &CreateOrUpdateFileHandler{provider: p}
}

func (h *CreateOrUpdateFileHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_or_update_file",
		Description: "Create or update a file in a GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path in the repository",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Commit message",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "File content (base64 encoded)",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Branch to commit to",
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA of file being replaced (for updates)",
				},
			},
			"required": []interface{}{"owner", "repo", "path", "message", "content"},
		},
	}
}

func (h *CreateOrUpdateFileHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	path := extractString(params, "path")
	message := extractString(params, "message")
	content := extractString(params, "content")
	branch := extractString(params, "branch")
	sha := extractString(params, "sha")

	opts := &github.RepositoryContentFileOptions{
		Message: &message,
		Content: []byte(content),
	}

	if branch != "" {
		opts.Branch = &branch
	}
	if sha != "" {
		opts.SHA = &sha
	}

	result, _, err := client.Repositories.CreateFile(ctx, owner, repo, path, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create/update file: %v", err)), nil
	}

	return NewToolResult(marshalJSON(result)), nil
}

// CreateRepositoryHandler handles repository creation
type CreateRepositoryHandler struct {
	provider *GitHubProvider
}

func NewCreateRepositoryHandler(p *GitHubProvider) *CreateRepositoryHandler {
	return &CreateRepositoryHandler{provider: p}
}

func (h *CreateRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_repository",
		Description: "Create a new GitHub repository with optional templates and initialization settings",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (must be unique for the owner). Can contain alphanumeric characters, hyphens, periods, and underscores",
					"example":     "my-awesome-project",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"maxLength":   100,
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Brief description of the repository (optional but recommended)",
					"example":     "A web application for managing tasks",
					"maxLength":   350,
				},
				"private": map[string]interface{}{
					"type":        "boolean",
					"description": "Create as private repository (requires paid plan or organization). Defaults to false (public)",
					"default":     false,
					"example":     false,
				},
				"auto_init": map[string]interface{}{
					"type":        "boolean",
					"description": "Initialize repository with README.md file. Required for immediate cloning. Defaults to false",
					"default":     false,
					"example":     true,
				},
				"gitignore_template": map[string]interface{}{
					"type":        "string",
					"description": "Language-specific .gitignore template (e.g., 'Node', 'Python', 'Java', 'Go'). See GitHub's gitignore templates",
					"example":     "Node",
				},
				"license_template": map[string]interface{}{
					"type":        "string",
					"description": "License template (e.g., 'mit', 'apache-2.0', 'gpl-3.0'). See GitHub's license templates",
					"example":     "mit",
				},
			},
			"required": []interface{}{"name"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo", "public_repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"id":        1296269,
					"name":      "Hello-World",
					"full_name": "octocat/Hello-World",
					"private":   false,
					"html_url":  "https://github.com/octocat/Hello-World",
					"clone_url": "https://github.com/octocat/Hello-World.git",
				},
				"error": map[string]interface{}{
					"message":           "Repository creation failed",
					"documentation_url": "https://docs.github.com/rest/repos/repos#create-a-repository-for-the-authenticated-user",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     422,
					"reason":   "Repository name already exists or invalid",
					"solution": "Choose a unique repository name and ensure it follows naming conventions",
				},
				{
					"code":     403,
					"reason":   "Insufficient permissions or private repo limit exceeded",
					"solution": "Ensure you have repo creation permissions and haven't exceeded private repository limits",
				},
			},
			"extendedHelp": "Creates a new repository under the authenticated user's account. For private repositories, ensure your plan supports them. Repository names must be unique per owner. See https://docs.github.com/rest/repos/repos#create-a-repository-for-the-authenticated-user",
		},
	}
}

func (h *CreateRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	repo := &github.Repository{
		Name:        ToStringPtr(extractString(params, "name")),
		Description: ToStringPtr(extractString(params, "description")),
		Private:     ToBoolPtr(extractBool(params, "private")),
		AutoInit:    ToBoolPtr(extractBool(params, "auto_init")),
	}

	if gitignore := extractString(params, "gitignore_template"); gitignore != "" {
		repo.GitignoreTemplate = &gitignore
	}
	if license := extractString(params, "license_template"); license != "" {
		repo.LicenseTemplate = &license
	}

	result, _, err := client.Repositories.Create(ctx, "", repo)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create repository: %v", err)), nil
	}

	return NewToolResult(marshalJSON(result)), nil
}

// ForkRepositoryHandler handles repository forking
type ForkRepositoryHandler struct {
	provider *GitHubProvider
}

func NewForkRepositoryHandler(p *GitHubProvider) *ForkRepositoryHandler {
	return &ForkRepositoryHandler{provider: p}
}

func (h *ForkRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "fork_repository",
		Description: GetOperationDescription("fork_repository"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization) to fork from",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name to fork",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"organization": map[string]interface{}{
					"type":        "string",
					"description": "Organization to fork the repository to (optional). If not specified, forks to your personal account",
					"example":     "my-org",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name for the fork (optional). Defaults to the same name as the parent repository",
					"example":     "my-react-fork",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"default_branch_only": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to fork only the default branch instead of all branches",
					"default":     false,
					"example":     false,
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "public_repo"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
			"async":        true, // Forking can be asynchronous for large repos
		},
		ResponseExample: map[string]interface{}{
			"id":        1296269,
			"name":      "Hello-World",
			"full_name": "your-username/Hello-World",
			"owner": map[string]interface{}{
				"login": "your-username",
				"type":  "User",
			},
			"private":    false,
			"fork":       true,
			"created_at": "2024-01-15T12:00:00Z",
			"updated_at": "2024-01-15T12:00:00Z",
			"parent": map[string]interface{}{
				"full_name": "octocat/Hello-World",
				"owner": map[string]interface{}{
					"login": "octocat",
				},
			},
			"html_url":       "https://github.com/your-username/Hello-World",
			"clone_url":      "https://github.com/your-username/Hello-World.git",
			"ssh_url":        "git@github.com:your-username/Hello-World.git",
			"default_branch": "main",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Source repository not found or private without access",
				"solution": "Verify the repository exists and you have read access",
			},
			{
				"error":    "403 Forbidden",
				"cause":    "Cannot fork to specified organization or forking disabled",
				"solution": "Ensure you have permission to create repos in the target organization",
			},
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Fork already exists or repository cannot be forked",
				"solution": "Check if you already have a fork of this repository",
			},
			{
				"error":    "202 Accepted",
				"cause":    "Fork creation is in progress (for large repositories)",
				"solution": "This is normal for large repos. Check back in a few minutes",
			},
		},
		ExtendedHelp: `The fork_repository operation creates a complete copy of a repository.

Forking creates:
- Complete copy of all branches (or just default branch if specified)
- All tags and releases
- Full commit history
- Connection to the parent repository for pull requests

Forking restrictions:
- Cannot fork your own repositories
- Cannot fork a fork into the same organization
- Private repos require appropriate permissions
- Some organizations may disable forking

Async behavior:
- Small repos fork immediately
- Large repos (>500MB) fork asynchronously
- API returns 202 Accepted for async forks
- Check fork status by polling the fork URL

Examples:

# Fork to personal account
{
  "owner": "facebook",
  "repo": "react"
}

# Fork to organization
{
  "owner": "facebook",
  "repo": "react",
  "organization": "my-company"
}

# Fork with custom name
{
  "owner": "facebook",
  "repo": "react",
  "name": "react-fork"
}

# Fork only default branch (faster for large repos)
{
  "owner": "facebook",
  "repo": "react",
  "default_branch_only": true
}

After forking:
1. Clone your fork locally
2. Add the original repo as 'upstream' remote
3. Keep your fork synced with upstream
4. Create feature branches for changes
5. Submit pull requests to contribute back`,
	}
}

func (h *ForkRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.RepositoryCreateForkOptions{}
	if org := extractString(params, "organization"); org != "" {
		opts.Organization = org
	}

	result, _, err := client.Repositories.CreateFork(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to fork repository: %v", err)), nil
	}

	return NewToolResult(marshalJSON(result)), nil
}

// CreateBranchHandler handles branch creation
type CreateBranchHandler struct {
	provider *GitHubProvider
}

func NewCreateBranchHandler(p *GitHubProvider) *CreateBranchHandler {
	return &CreateBranchHandler{provider: p}
}

func (h *CreateBranchHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_branch",
		Description: "Create a new branch in a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "New branch name",
				},
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Source branch or SHA",
				},
			},
			"required": []interface{}{"owner", "repo", "branch", "from"},
		},
	}
}

func (h *CreateBranchHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	branch := extractString(params, "branch")
	from := extractString(params, "from")

	// Get the reference to branch from
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "heads/"+from)
	if err != nil {
		// Try as SHA
		ref, _, err = client.Git.GetRef(ctx, owner, repo, from)
		if err != nil {
			return NewToolError(fmt.Sprintf("Failed to get source reference: %v", err)), nil
		}
	}

	// Create new branch reference
	newRef := &github.Reference{
		Ref: ToStringPtr("refs/heads/" + branch),
		Object: &github.GitObject{
			SHA: ref.Object.SHA,
		},
	}

	result, _, err := client.Git.CreateRef(ctx, owner, repo, newRef)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create branch: %v", err)), nil
	}

	return NewToolResult(marshalJSON(result)), nil
}

// PushFilesHandler handles multi-file push operations
type PushFilesHandler struct {
	provider *GitHubProvider
}

func NewPushFilesHandler(p *GitHubProvider) *PushFilesHandler {
	return &PushFilesHandler{provider: p}
}

func (h *PushFilesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "push_files",
		Description: "Push multiple files to a repository atomically",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Branch to push to",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Commit message",
				},
				"files": map[string]interface{}{
					"type":        "array",
					"description": "Files to push",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "File path",
							},
							"content": map[string]interface{}{
								"type":        "string",
								"description": "File content",
							},
						},
					},
				},
			},
			"required": []interface{}{"owner", "repo", "branch", "message", "files"},
		},
	}
}

func (h *PushFilesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	branch := extractString(params, "branch")
	message := extractString(params, "message")

	// Get current branch ref
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "heads/"+branch)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get branch reference: %v", err)), nil
	}

	// Get current commit
	commit, _, err := client.Git.GetCommit(ctx, owner, repo, *ref.Object.SHA)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get commit: %v", err)), nil
	}

	// Create blobs for each file
	entries := []github.TreeEntry{}
	if files, ok := params["files"].([]interface{}); ok {
		for _, file := range files {
			if fileMap, ok := file.(map[string]interface{}); ok {
				path := extractString(fileMap, "path")
				content := extractString(fileMap, "content")

				blob, _, err := client.Git.CreateBlob(ctx, owner, repo, &github.Blob{
					Content:  &content,
					Encoding: ToStringPtr("utf-8"),
				})
				if err != nil {
					return NewToolError(fmt.Sprintf("Failed to create blob for %s: %v", path, err)), nil
				}

				entries = append(entries, github.TreeEntry{
					Path: &path,
					Mode: ToStringPtr("100644"),
					Type: ToStringPtr("blob"),
					SHA:  blob.SHA,
				})
			}
		}
	}

	// Create new tree (convert []TreeEntry to []*TreeEntry)
	treeEntries := make([]*github.TreeEntry, len(entries))
	for i := range entries {
		entry := entries[i]
		treeEntries[i] = &entry
	}
	tree, _, err := client.Git.CreateTree(ctx, owner, repo, *commit.Tree.SHA, treeEntries)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create tree: %v", err)), nil
	}

	// Create new commit
	newCommit, _, err := client.Git.CreateCommit(ctx, owner, repo, &github.Commit{
		Message: &message,
		Tree:    tree,
		Parents: []*github.Commit{commit},
	}, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create commit: %v", err)), nil
	}

	// Update branch reference
	ref.Object.SHA = newCommit.SHA
	_, _, err = client.Git.UpdateRef(ctx, owner, repo, ref, false)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update branch reference: %v", err)), nil
	}

	return NewToolResult(marshalJSON(map[string]interface{}{
		"commit":  newCommit.SHA,
		"message": message,
		"files":   len(entries),
	})), nil
}

// DeleteFileHandler handles file deletion
type DeleteFileHandler struct {
	provider *GitHubProvider
}

func NewDeleteFileHandler(p *GitHubProvider) *DeleteFileHandler {
	return &DeleteFileHandler{provider: p}
}

func (h *DeleteFileHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_file",
		Description: "Delete a file from a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Commit message",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Branch to delete from",
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA of file being deleted",
				},
			},
			"required": []interface{}{"owner", "repo", "path", "message", "sha"},
		},
	}
}

func (h *DeleteFileHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	path := extractString(params, "path")
	message := extractString(params, "message")
	sha := extractString(params, "sha")
	branch := extractString(params, "branch")

	opts := &github.RepositoryContentFileOptions{
		Message: &message,
		SHA:     &sha,
	}

	if branch != "" {
		opts.Branch = &branch
	}

	result, _, err := client.Repositories.DeleteFile(ctx, owner, repo, path, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete file: %v", err)), nil
	}

	return NewToolResult(marshalJSON(result)), nil
}
