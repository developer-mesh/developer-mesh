package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Pull Request Handlers

// GetPullRequestHandler handles getting a specific pull request
type GetPullRequestHandler struct {
	provider *GitHubProvider
}

func NewGetPullRequestHandler(p *GitHubProvider) *GetPullRequestHandler {
	return &GetPullRequestHandler{provider: p}
}

func (h *GetPullRequestHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_pull_request",
		Description: "Get a specific pull request from a GitHub repository",
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
				"pull_number": map[string]interface{}{
					"type":        "integer",
					"description": "Pull request number",
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number"},
		},
	}
}

func (h *GetPullRequestHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	pr, _, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get pull request: %v", err)), nil
	}

	data, _ := json.Marshal(pr)
	return NewToolResult(string(data)), nil
}

// ListPullRequestsHandler handles listing pull requests
type ListPullRequestsHandler struct {
	provider *GitHubProvider
}

func NewListPullRequestsHandler(p *GitHubProvider) *ListPullRequestsHandler {
	return &ListPullRequestsHandler{provider: p}
}

func (h *ListPullRequestsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_pull_requests",
		Description: "List pull requests in a GitHub repository",
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
				"state": map[string]interface{}{
					"type":        "string",
					"description": "PR state: open, closed, or all",
				},
				"head": map[string]interface{}{
					"type":        "string",
					"description": "Filter by head branch",
				},
				"base": map[string]interface{}{
					"type":        "string",
					"description": "Filter by base branch",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort by: created, updated, popularity",
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
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListPullRequestsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.PullRequestListOptions{}
	if state, ok := params["state"].(string); ok {
		opts.State = state
	}
	if head, ok := params["head"].(string); ok {
		opts.Head = head
	}
	if base, ok := params["base"].(string); ok {
		opts.Base = base
	}
	if sort, ok := params["sort"].(string); ok {
		opts.Sort = sort
	}
	if direction, ok := params["direction"].(string); ok {
		opts.Direction = direction
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	prs, _, err := client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list pull requests: %v", err)), nil
	}

	// Create simplified response to reduce token usage
	simplified := make([]map[string]interface{}, 0, len(prs))
	for _, pr := range prs {
		simplified = append(simplified, simplifyPullRequest(pr))
	}

	data, _ := json.Marshal(simplified)
	return NewToolResult(string(data)), nil
}

// GetPullRequestFilesHandler handles getting files changed in a pull request
type GetPullRequestFilesHandler struct {
	provider *GitHubProvider
}

func NewGetPullRequestFilesHandler(p *GitHubProvider) *GetPullRequestFilesHandler {
	return &GetPullRequestFilesHandler{provider: p}
}

func (h *GetPullRequestFilesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_pull_request_files",
		Description: "Get files changed in a pull request",
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
				"pull_number": map[string]interface{}{
					"type":        "integer",
					"description": "Pull request number",
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
			"required": []interface{}{"owner", "repo", "pull_number"},
		},
	}
}

func (h *GetPullRequestFilesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	opts := &github.ListOptions{}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	files, _, err := client.PullRequests.ListFiles(ctx, owner, repo, pullNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get pull request files: %v", err)), nil
	}

	data, _ := json.Marshal(files)
	return NewToolResult(string(data)), nil
}

// SearchPullRequestsHandler handles searching for pull requests
type SearchPullRequestsHandler struct {
	provider *GitHubProvider
}

func NewSearchPullRequestsHandler(p *GitHubProvider) *SearchPullRequestsHandler {
	return &SearchPullRequestsHandler{provider: p}
}

func (h *SearchPullRequestsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_pull_requests",
		Description: "Search for pull requests on GitHub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub PR search syntax",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort results by: created, updated, comments",
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

func (h *SearchPullRequestsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
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

	// Add type:pr to ensure we're searching for pull requests
	query = query + " type:pr"

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

	h.provider.logger.Info("Search pull requests pagination", map[string]interface{}{
		"query":    query,
		"per_page": perPage,
		"page":     page,
	})

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search pull requests: %v", err)), nil
	}

	// Return items with essential metadata only
	response := map[string]interface{}{
		"items":       result.Issues,
		"total_count": *result.Total,
		"has_more":    *result.Total > len(result.Issues),
		"page":        opts.Page,
		"per_page":    opts.PerPage,
	}

	data, _ := json.Marshal(response)
	return NewToolResult(string(data)), nil
}

// CreatePullRequestHandler handles creating a new pull request
type CreatePullRequestHandler struct {
	provider *GitHubProvider
}

func NewCreatePullRequestHandler(p *GitHubProvider) *CreatePullRequestHandler {
	return &CreatePullRequestHandler{provider: p}
}

func (h *CreatePullRequestHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_pull_request",
		Description: "Create a new pull request from one branch to another, enabling code review and collaboration",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization (e.g., 'facebook', 'microsoft')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Pull request title (concise description of changes)",
					"example":     "Add user authentication feature",
					"minLength":   1,
					"maxLength":   256,
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Detailed description of changes, motivation, and testing notes (supports Markdown)",
					"example":     "## Summary\n\nThis PR adds OAuth2 authentication...\n\n## Changes\n- Added login endpoint\n- Integrated OAuth2\n\n## Testing\n- Unit tests added\n- Manual testing completed",
					"maxLength":   65536,
				},
				"head": map[string]interface{}{
					"type":        "string",
					"description": "Source branch containing changes (e.g., 'feature/auth', 'username:branch' for forks)",
					"example":     "feature/new-authentication",
					"minLength":   1,
				},
				"base": map[string]interface{}{
					"type":        "string",
					"description": "Target branch to merge changes into (typically 'main' or 'master')",
					"example":     "main",
					"default":     "main",
					"minLength":   1,
				},
				"draft": map[string]interface{}{
					"type":        "boolean",
					"description": "Create as draft PR (not ready for review, useful for work-in-progress)",
					"default":     false,
					"example":     false,
				},
			},
			"required": []interface{}{"owner", "repo", "title", "head", "base"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo", "public_repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"id":        1,
					"number":    100,
					"state":     "open",
					"title":     "Add user authentication feature",
					"html_url":  "https://github.com/octocat/Hello-World/pull/100",
					"mergeable": true,
					"draft":     false,
				},
				"error": map[string]interface{}{
					"message":           "Validation Failed",
					"documentation_url": "https://docs.github.com/rest/pulls/pulls#create-a-pull-request",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     422,
					"reason":   "Invalid head branch, base branch, or no changes between branches",
					"solution": "Ensure both branches exist, head has changes, and you have push access",
				},
				{
					"code":     404,
					"reason":   "Repository, head branch, or base branch not found",
					"solution": "Verify repository exists and both branches are pushed to GitHub",
				},
				{
					"code":     403,
					"reason":   "Insufficient permissions to create pull request",
					"solution": "Ensure you have write access to the repository or fork permissions",
				},
			},
			"extendedHelp": "Creates a pull request for code review. For cross-fork PRs, use 'username:branch' format for head. Draft PRs are useful for early feedback. See https://docs.github.com/rest/pulls/pulls#create-a-pull-request",
		},
	}
}

func (h *CreatePullRequestHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	title := extractString(params, "title")
	head := extractString(params, "head")
	base := extractString(params, "base")

	prRequest := &github.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
	}

	if body, ok := params["body"].(string); ok {
		prRequest.Body = &body
	}
	if draft, ok := params["draft"].(bool); ok {
		prRequest.Draft = &draft
	}

	pr, _, err := client.PullRequests.Create(ctx, owner, repo, prRequest)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create pull request: %v", err)), nil
	}

	data, _ := json.Marshal(pr)
	return NewToolResult(string(data)), nil
}

// MergePullRequestHandler handles merging a pull request
type MergePullRequestHandler struct {
	provider *GitHubProvider
}

func NewMergePullRequestHandler(p *GitHubProvider) *MergePullRequestHandler {
	return &MergePullRequestHandler{provider: p}
}

func (h *MergePullRequestHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "merge_pull_request",
		Description: "Merge an approved pull request into the base branch using specified merge strategy",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization (e.g., 'facebook', 'microsoft')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
				},
				"pull_number": map[string]interface{}{
					"type":        "integer",
					"description": "Pull request number to merge",
					"example":     42,
					"minimum":     1,
				},
				"commit_title": map[string]interface{}{
					"type":        "string",
					"description": "Custom merge commit title (defaults to PR title)",
					"example":     "Merge PR #42: Add authentication feature",
					"maxLength":   256,
				},
				"commit_message": map[string]interface{}{
					"type":        "string",
					"description": "Custom merge commit message body (defaults to PR description)",
					"example":     "This adds OAuth2 authentication with comprehensive test coverage",
					"maxLength":   65536,
				},
				"merge_method": map[string]interface{}{
					"type":        "string",
					"description": "How to merge the pull request",
					"enum":        []interface{}{"merge", "squash", "rebase"},
					"default":     "merge",
					"example":     "squash",
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo"},
				"minimumScopes":     []string{"repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"mergeMethodDetails": map[string]interface{}{
				"merge":  "Creates a merge commit with all individual commits preserved",
				"squash": "Combines all commits into a single commit on the base branch",
				"rebase": "Adds commits onto the base branch without a merge commit",
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"sha":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					"merged":  true,
					"message": "Pull Request successfully merged",
				},
				"error": map[string]interface{}{
					"message":           "Pull Request is not mergeable",
					"documentation_url": "https://docs.github.com/rest/pulls/pulls#merge-a-pull-request",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     405,
					"reason":   "Pull request is not mergeable (conflicts or checks failing)",
					"solution": "Resolve merge conflicts, ensure CI checks pass, and PR is approved",
				},
				{
					"code":     404,
					"reason":   "Pull request not found",
					"solution": "Verify the pull request number and repository are correct",
				},
				{
					"code":     403,
					"reason":   "Insufficient permissions or branch protection rules",
					"solution": "Ensure you have write access and meet branch protection requirements",
				},
				{
					"code":     422,
					"reason":   "Invalid merge method for repository settings",
					"solution": "Check repository settings for allowed merge methods",
				},
			},
			"extendedHelp": "Merges a PR after review. Ensure checks pass and approvals are met. Repository settings control available merge methods. See https://docs.github.com/rest/pulls/pulls#merge-a-pull-request",
		},
	}
}

func (h *MergePullRequestHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	mergeOptions := &github.PullRequestOptions{}
	if commitTitle, ok := params["commit_title"].(string); ok {
		mergeOptions.CommitTitle = commitTitle
	}
	if mergeMethod, ok := params["merge_method"].(string); ok {
		mergeOptions.MergeMethod = mergeMethod
	}

	result, _, err := client.PullRequests.Merge(ctx, owner, repo, pullNumber, "", mergeOptions)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to merge pull request: %v", err)), nil
	}

	data, _ := json.Marshal(result)
	return NewToolResult(string(data)), nil
}

// UpdatePullRequestHandler handles updating a pull request
type UpdatePullRequestHandler struct {
	provider *GitHubProvider
}

func NewUpdatePullRequestHandler(p *GitHubProvider) *UpdatePullRequestHandler {
	return &UpdatePullRequestHandler{provider: p}
}

func (h *UpdatePullRequestHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_pull_request",
		Description: "Update an existing pull request",
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
				"pull_number": map[string]interface{}{
					"type":        "integer",
					"description": "Pull request number",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "New pull request title",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "New pull request body",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State: open or closed",
				},
				"base": map[string]interface{}{
					"type":        "string",
					"description": "New base branch",
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number"},
		},
	}
}

func (h *UpdatePullRequestHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	prRequest := &github.PullRequest{}

	if title, ok := params["title"].(string); ok {
		prRequest.Title = &title
	}
	if body, ok := params["body"].(string); ok {
		prRequest.Body = &body
	}
	if state, ok := params["state"].(string); ok {
		prRequest.State = &state
	}
	if base, ok := params["base"].(string); ok {
		prRequest.Base = &github.PullRequestBranch{
			Ref: &base,
		}
	}

	pr, _, err := client.PullRequests.Edit(ctx, owner, repo, pullNumber, prRequest)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update pull request: %v", err)), nil
	}

	data, _ := json.Marshal(pr)
	return NewToolResult(string(data)), nil
}
