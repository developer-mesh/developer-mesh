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
		Description: "List repositories for a user or organization",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user": map[string]interface{}{
					"type":        "string",
					"description": "Username (leave empty for authenticated user)",
				},
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Type of repositories: all, owner, public, private, member",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort by: created, updated, pushed, full_name",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "Sort direction: asc or desc",
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
			"required": []interface{}{},
		},
	}
}

func (h *ListRepositoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Get information about a specific repository",
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
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *GetRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
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

	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
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
		Description: "Update repository settings",
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
				"name": map[string]interface{}{
					"type":        "string",
					"description": "New repository name",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Repository description",
				},
				"homepage": map[string]interface{}{
					"type":        "string",
					"description": "Repository homepage URL",
				},
				"private": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the repository is private",
				},
				"has_issues": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable issues",
				},
				"has_projects": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable projects",
				},
				"has_wiki": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable wiki",
				},
				"default_branch": map[string]interface{}{
					"type":        "string",
					"description": "Default branch name",
				},
				"allow_squash_merge": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow squash merging",
				},
				"allow_merge_commit": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow merge commits",
				},
				"allow_rebase_merge": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow rebase merging",
				},
				"delete_branch_on_merge": map[string]interface{}{
					"type":        "boolean",
					"description": "Automatically delete head branches",
				},
				"archived": map[string]interface{}{
					"type":        "boolean",
					"description": "Archive the repository",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *UpdateRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Delete a repository (requires admin permissions)",
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
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *DeleteRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Search for repositories on GitHub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort results by: stars, forks, updated",
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

func (h *SearchRepositoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	query, ok := params["query"].(string)
	if !ok {
		return NewToolError("query parameter is required"), nil
	}

	opts := &github.SearchOptions{}
	if sort, ok := params["sort"].(string); ok {
		opts.Sort = sort
	}
	if order, ok := params["order"].(string); ok {
		opts.Order = order
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	result, _, err := client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search repositories: %v", err)), nil
	}

	data, _ := json.Marshal(result)
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
		Description: "Get contents of a file from a GitHub repository",
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
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Branch, tag, or commit SHA",
				},
			},
			"required": []interface{}{"owner", "repo", "path"},
		},
	}
}

func (h *GetFileContentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	path, _ := params["path"].(string)

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
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

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

	data, _ := json.Marshal(commits)
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
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	query, ok := params["query"].(string)
	if !ok {
		return NewToolError("query parameter is required"), nil
	}

	opts := &github.SearchOptions{}
	if sort, ok := params["sort"].(string); ok {
		opts.Sort = sort
	}
	if order, ok := params["order"].(string); ok {
		opts.Order = order
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	result, _, err := client.Search.Code(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search code: %v", err)), nil
	}

	data, _ := json.Marshal(result)
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
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	sha, _ := params["sha"].(string)

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
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

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
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Create a new GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Repository description",
				},
				"private": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the repository is private",
				},
				"auto_init": map[string]interface{}{
					"type":        "boolean",
					"description": "Initialize with README",
				},
				"gitignore_template": map[string]interface{}{
					"type":        "string",
					"description": "Gitignore template name",
				},
				"license_template": map[string]interface{}{
					"type":        "string",
					"description": "License template name",
				},
			},
			"required": []interface{}{"name"},
		},
	}
}

func (h *CreateRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Fork a GitHub repository",
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
				"organization": map[string]interface{}{
					"type":        "string",
					"description": "Organization to fork to (optional)",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ForkRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
	client, ok := ctx.Value("github_client").(*github.Client)
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
	client, ok := ctx.Value("github_client").(*github.Client)
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
	client, ok := ctx.Value("github_client").(*github.Client)
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
