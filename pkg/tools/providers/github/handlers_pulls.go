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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	pullNumber := int(params["pull_number"].(float64))

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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

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

	data, _ := json.Marshal(prs)
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	pullNumber := int(params["pull_number"].(float64))

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

	query, ok := params["query"].(string)
	if !ok {
		return NewToolError("query parameter is required"), nil
	}

	// Add type:pr to ensure we're searching for pull requests
	query = query + " type:pr"

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
		Description: "Create a new pull request in a GitHub repository",
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
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Pull request title",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Pull request body",
				},
				"head": map[string]interface{}{
					"type":        "string",
					"description": "The name of the branch where changes are implemented",
				},
				"base": map[string]interface{}{
					"type":        "string",
					"description": "The name of the branch you want changes pulled into",
				},
				"draft": map[string]interface{}{
					"type":        "boolean",
					"description": "Create as draft pull request",
				},
			},
			"required": []interface{}{"owner", "repo", "title", "head", "base"},
		},
	}
}

func (h *CreatePullRequestHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	title, _ := params["title"].(string)
	head, _ := params["head"].(string)
	base, _ := params["base"].(string)

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
		Description: "Merge a pull request",
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
				"commit_title": map[string]interface{}{
					"type":        "string",
					"description": "Title for the merge commit",
				},
				"commit_message": map[string]interface{}{
					"type":        "string",
					"description": "Message for the merge commit",
				},
				"merge_method": map[string]interface{}{
					"type":        "string",
					"description": "Merge method: merge, squash, or rebase",
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number"},
		},
	}
}

func (h *MergePullRequestHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	pullNumber := int(params["pull_number"].(float64))

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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	pullNumber := int(params["pull_number"].(float64))

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
