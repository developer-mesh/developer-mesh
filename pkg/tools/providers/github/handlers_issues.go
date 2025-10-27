package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/utils"
	"github.com/google/go-github/v74/github"
	"github.com/shurcooL/githubv4"
)

// Issue Handlers

// GetIssueHandler handles getting a specific issue
type GetIssueHandler struct {
	provider *GitHubProvider
}

func NewGetIssueHandler(p *GitHubProvider) *GetIssueHandler {
	return &GetIssueHandler{provider: p}
}

func (h *GetIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_issue",
		Description: "Get issue details (title, body, state, labels, assignees, comments, timeline). Use when: investigating issue, checking status, reading discussion.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number (positive integer)",
					"example":     1234,
					"minimum":     1,
					"maximum":     999999999,
				},
			},
			"required": []interface{}{"owner", "repo", "issue_number"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
				"complexityLevel":   "simple",
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"id":     123456789,
					"number": 1234,
					"title":  "Bug: Something is broken",
					"state":  "open",
					"user": map[string]interface{}{
						"login": "octocat",
					},
					"labels": []interface{}{
						map[string]interface{}{
							"name":  "bug",
							"color": "d73a4a",
						},
					},
				},
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/issues/issues#get-an-issue",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "Issue not found or you lack access to the repository",
					"solution": "Verify the issue exists and you have read permission to the repository",
				},
				{
					"code":     403,
					"reason":   "Rate limit exceeded or insufficient permissions",
					"solution": "Wait for rate limit reset or check your authentication token permissions",
				},
				{
					"code":     401,
					"reason":   "Authentication required for private repository",
					"solution": "Provide a valid GitHub token with appropriate repository access",
				},
			},
			"extendedHelp": "This operation requires read access to the repository. Public repositories are accessible without authentication, but private repositories require appropriate permissions. See https://docs.github.com/en/rest/issues/issues#get-an-issue",
		},
	}
}

func (h *GetIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	issue, _, err := client.Issues.Get(ctx, owner, repo, issueNumber)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get issue: %v", err)), nil
	}

	data, _ := json.Marshal(issue)
	return NewToolResult(string(data)), nil
}

// SearchIssuesHandler handles searching for issues
type SearchIssuesHandler struct {
	provider *GitHubProvider
}

func NewSearchIssuesHandler(p *GitHubProvider) *SearchIssuesHandler {
	return &SearchIssuesHandler{provider: p}
}

func (h *SearchIssuesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_issues",
		Description: "Search issues by title, state, author, label, assignee, repo. Use when: finding bug, tracking work, filtering issues.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax (e.g., 'bug repo:facebook/react state:open')",
					"example":     "memory leak repo:microsoft/vscode state:open label:bug",
					"minLength":   1,
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort results by",
					"enum":        []interface{}{"created", "updated", "comments"},
					"default":     "created",
					"example":     "updated",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Sort order",
					"enum":        []interface{}{"asc", "desc"},
					"default":     "desc",
					"example":     "desc",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (1-100)",
					"minimum":     1,
					"maximum":     100,
					"default":     30,
					"example":     50,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number (1-based, max 100 for search results)",
					"minimum":     1,
					"maximum":     100,
					"default":     1,
					"example":     2,
				},
			},
			"required": []interface{}{"query"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"repo"},
				"minimumScopes":     []string{"public_repo"},
				"rateLimitCategory": "search",
				"requestsPerHour":   30,
				"complexityLevel":   "complex",
			},
			"responseExample": map[string]interface{}{
				"success": map[string]interface{}{
					"total_count": 2,
					"items": []interface{}{
						map[string]interface{}{
							"number": 1234,
							"title":  "Memory leak in component unmounting",
							"state":  "open",
							"repository": map[string]interface{}{
								"name":      "vscode",
								"full_name": "microsoft/vscode",
							},
						},
					},
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     422,
					"reason":   "Invalid search query syntax",
					"solution": "Check GitHub search syntax documentation and verify query format",
				},
				{
					"code":     403,
					"reason":   "Rate limit exceeded (search API has lower limits)",
					"solution": "Wait for rate limit reset - search API allows 30 requests per minute",
				},
			},
			"extendedHelp": "Uses GitHub's powerful search syntax. Supports qualifiers like 'repo:owner/name', 'state:open', 'label:bug', 'author:username', etc. See https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests",
		},
	}
}

func (h *SearchIssuesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
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

	// Log pagination parameters for debugging
	h.provider.logger.Info("Search issues pagination", map[string]interface{}{
		"query":    query,
		"per_page": perPage,
		"page":     page,
	})

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search issues: %v", err)), nil
	}

	// Return simplified items with essential metadata only
	simplifiedIssues := make([]map[string]interface{}, 0, len(result.Issues))
	for _, issue := range result.Issues {
		simplifiedIssues = append(simplifiedIssues, simplifyIssue(issue))
	}

	response := map[string]interface{}{
		"items":       simplifiedIssues,
		"total_count": *result.Total,
		"has_more":    *result.Total > len(result.Issues),
		"page":        opts.Page,
		"per_page":    opts.PerPage,
	}

	data, _ := json.Marshal(response)
	return NewToolResult(string(data)), nil
}

// ListIssuesHandler handles listing issues (uses GraphQL for better performance)
type ListIssuesHandler struct {
	provider *GitHubProvider
}

func NewListIssuesHandler(p *GitHubProvider) *ListIssuesHandler {
	return &ListIssuesHandler{provider: p}
}

func (h *ListIssuesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_issues",
		Description: "List issues (number, title, state, labels, assignee, author). Use when: triaging issues, finding bugs, checking open work.",
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
					"description": "Issue state: open, closed, or all",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Filter by labels",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort by: created, updated, comments",
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

func (h *ListIssuesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Log incoming parameters for debugging (with sensitive data redacted)
	h.provider.logger.Info("ListIssuesHandler.Execute called", map[string]interface{}{
		"params":    utils.RedactSensitiveData(params),
		"has_owner": params["owner"] != nil,
		"has_repo":  params["repo"] != nil,
	})

	// For complex list operations, use GraphQL client for better performance
	gqlClient, ok := GetGitHubV4ClientFromContext(ctx)
	if ok {
		// GraphQL implementation for better performance
		return h.executeGraphQL(ctx, gqlClient, params)
	}

	// Fallback to REST API
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.IssueListByRepoOptions{}
	if state, ok := params["state"].(string); ok {
		opts.State = state
	}
	if labels, ok := params["labels"].([]interface{}); ok {
		var labelStrings []string
		for _, label := range labels {
			if str, ok := label.(string); ok {
				labelStrings = append(labelStrings, str)
			}
		}
		opts.Labels = labelStrings
	}
	if sort, ok := params["sort"].(string); ok {
		opts.Sort = sort
	}
	if direction, ok := params["direction"].(string); ok {
		opts.Direction = direction
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.ListOptions.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.ListOptions.Page = int(page)
	}

	issues, _, err := client.Issues.ListByRepo(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list issues: %v", err)), nil
	}

	// Create simplified response to reduce token usage
	simplified := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		simplified = append(simplified, simplifyIssue(issue))
	}

	data, _ := json.Marshal(simplified)
	return NewToolResult(string(data)), nil
}

func (h *ListIssuesHandler) executeGraphQL(ctx context.Context, client *githubv4.Client, params map[string]interface{}) (*ToolResult, error) {
	// Extract owner and repo using the helper function to handle various types
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	if owner == "" || repo == "" {
		// Info logging to understand what parameters we received (with sensitive data redacted)
		h.provider.logger.Info("Missing owner or repo in executeGraphQL", map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"params": utils.RedactSensitiveData(params),
		})
		return NewToolError("owner and repo parameters are required"), nil
	}

	// GraphQL query for listing issues
	var query struct {
		Repository struct {
			Issues struct {
				Nodes []struct {
					Number int
					Title  string
					Body   string
					State  string
					Author struct {
						Login string
					}
					CreatedAt string
					UpdatedAt string
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
			} `graphql:"issues(first: $first, after: $after, states: $states)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	// Respect per_page parameter
	perPage := 30 // default
	if pp, ok := params["per_page"].(float64); ok && pp > 0 {
		perPage = int(pp)
		if perPage > 100 {
			perPage = 100 // GitHub's max
		}
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(repo),
		"first": githubv4.Int(perPage),
		"after": (*githubv4.String)(nil),
	}

	// Map state to GraphQL enum
	if state, ok := params["state"].(string); ok {
		switch state {
		case "open":
			variables["states"] = []githubv4.IssueState{githubv4.IssueStateOpen}
		case "closed":
			variables["states"] = []githubv4.IssueState{githubv4.IssueStateClosed}
		default:
			variables["states"] = []githubv4.IssueState{githubv4.IssueStateOpen, githubv4.IssueStateClosed}
		}
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list issues via GraphQL: %v", err)), nil
	}

	data, _ := json.Marshal(query.Repository.Issues)
	return NewToolResult(string(data)), nil
}

// GetIssueCommentsHandler handles getting issue comments
type GetIssueCommentsHandler struct {
	provider *GitHubProvider
}

func NewGetIssueCommentsHandler(p *GitHubProvider) *GetIssueCommentsHandler {
	return &GetIssueCommentsHandler{provider: p}
}

func (h *GetIssueCommentsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_issue_comments",
		Description: "Get issue comments (author, body, created_at, reactions). Use when: reading discussion, following conversation, checking feedback.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
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
			"required": []interface{}{"owner", "repo", "issue_number"},
		},
	}
}

func (h *GetIssueCommentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	opts := &github.IssueListCommentsOptions{}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	comments, _, err := client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get issue comments: %v", err)), nil
	}

	data, _ := json.Marshal(comments)
	return NewToolResult(string(data)), nil
}

// CreateIssueHandler handles creating a new issue
type CreateIssueHandler struct {
	provider *GitHubProvider
}

func NewCreateIssueHandler(p *GitHubProvider) *CreateIssueHandler {
	return &CreateIssueHandler{provider: p}
}

func (h *CreateIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_issue",
		Description: "Create issue with title, body, labels, assignees. Use when: reporting bug, requesting feature, documenting task.",
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
					"description": "Issue title",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Issue body",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Labels to apply",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"assignees": map[string]interface{}{
					"type":        "array",
					"description": "Users to assign",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []interface{}{"owner", "repo", "title"},
		},
	}
}

func (h *CreateIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	title := extractString(params, "title")

	issueRequest := &github.IssueRequest{
		Title: &title,
	}

	if body, ok := params["body"].(string); ok {
		issueRequest.Body = &body
	}

	if labels, ok := params["labels"].([]interface{}); ok {
		var labelStrings []string
		for _, label := range labels {
			if str, ok := label.(string); ok {
				labelStrings = append(labelStrings, str)
			}
		}
		issueRequest.Labels = &labelStrings
	}

	if assignees, ok := params["assignees"].([]interface{}); ok {
		var assigneeStrings []string
		for _, assignee := range assignees {
			if str, ok := assignee.(string); ok {
				assigneeStrings = append(assigneeStrings, str)
			}
		}
		issueRequest.Assignees = &assigneeStrings
	}

	issue, _, err := client.Issues.Create(ctx, owner, repo, issueRequest)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create issue: %v", err)), nil
	}

	data, _ := json.Marshal(issue)
	return NewToolResult(string(data)), nil
}

// AddIssueCommentHandler handles adding a comment to an issue
type AddIssueCommentHandler struct {
	provider *GitHubProvider
}

func NewAddIssueCommentHandler(p *GitHubProvider) *AddIssueCommentHandler {
	return &AddIssueCommentHandler{provider: p}
}

func (h *AddIssueCommentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "add_issue_comment",
		Description: "Add comment to issue. Use when: responding, asking question, providing update, linking PR.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Comment body",
				},
			},
			"required": []interface{}{"owner", "repo", "issue_number", "body"},
		},
	}
}

func (h *AddIssueCommentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")
	body := extractString(params, "body")

	comment := &github.IssueComment{
		Body: &body,
	}

	newComment, _, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, comment)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to add comment: %v", err)), nil
	}

	data, _ := json.Marshal(newComment)
	return NewToolResult(string(data)), nil
}

// UpdateIssueHandler handles updating an issue
type UpdateIssueHandler struct {
	provider *GitHubProvider
}

func NewUpdateIssueHandler(p *GitHubProvider) *UpdateIssueHandler {
	return &UpdateIssueHandler{provider: p}
}

func (h *UpdateIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_issue",
		Description: "Update issue (title, body, state, labels, assignees). Use when: refining description, changing status, reassigning work.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "New issue title",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "New issue body",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "Issue state: open or closed",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"description": "Labels to set",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"assignees": map[string]interface{}{
					"type":        "array",
					"description": "Users to assign",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []interface{}{"owner", "repo", "issue_number"},
		},
	}
}

func (h *UpdateIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	issueRequest := &github.IssueRequest{}

	if title, ok := params["title"].(string); ok {
		issueRequest.Title = &title
	}
	if body, ok := params["body"].(string); ok {
		issueRequest.Body = &body
	}
	if state, ok := params["state"].(string); ok {
		issueRequest.State = &state
	}

	if labels, ok := params["labels"].([]interface{}); ok {
		var labelStrings []string
		for _, label := range labels {
			if str, ok := label.(string); ok {
				labelStrings = append(labelStrings, str)
			}
		}
		issueRequest.Labels = &labelStrings
	}

	if assignees, ok := params["assignees"].([]interface{}); ok {
		var assigneeStrings []string
		for _, assignee := range assignees {
			if str, ok := assignee.(string); ok {
				assigneeStrings = append(assigneeStrings, str)
			}
		}
		issueRequest.Assignees = &assigneeStrings
	}

	issue, _, err := client.Issues.Edit(ctx, owner, repo, issueNumber, issueRequest)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update issue: %v", err)), nil
	}

	data, _ := json.Marshal(issue)
	return NewToolResult(string(data)), nil
}

// LockIssueHandler handles locking an issue to prevent further comments
type LockIssueHandler struct {
	provider *GitHubProvider
}

func NewLockIssueHandler(p *GitHubProvider) *LockIssueHandler {
	return &LockIssueHandler{provider: p}
}

func (h *LockIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "lock_issue",
		Description: "Lock issue (prevent comments) with reason. Use when: closing heated discussion, preventing spam, archiving resolved issue.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
				},
				"lock_reason": map[string]interface{}{
					"type":        "string",
					"description": "Reason for locking: off-topic, too heated, resolved, spam",
				},
			},
			"required": []interface{}{"owner", "repo", "issue_number"},
		},
	}
}

func (h *LockIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	opts := &github.LockIssueOptions{}
	if reason, ok := params["lock_reason"].(string); ok {
		opts.LockReason = reason
	}

	_, err := client.Issues.Lock(ctx, owner, repo, issueNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to lock issue: %v", err)), nil
	}

	return NewToolResult(`{"status": "locked"}`), nil
}

// UnlockIssueHandler handles unlocking an issue
type UnlockIssueHandler struct {
	provider *GitHubProvider
}

func NewUnlockIssueHandler(p *GitHubProvider) *UnlockIssueHandler {
	return &UnlockIssueHandler{provider: p}
}

func (h *UnlockIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "unlock_issue",
		Description: "Unlock issue (allow comments). Use when: reopening discussion, allowing feedback, reversing lock.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
				},
			},
			"required": []interface{}{"owner", "repo", "issue_number"},
		},
	}
}

func (h *UnlockIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	_, err := client.Issues.Unlock(ctx, owner, repo, issueNumber)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to unlock issue: %v", err)), nil
	}

	return NewToolResult(`{"status": "unlocked"}`), nil
}

// GetIssueEventsHandler handles getting events for an issue
type GetIssueEventsHandler struct {
	provider *GitHubProvider
}

func NewGetIssueEventsHandler(p *GitHubProvider) *GetIssueEventsHandler {
	return &GetIssueEventsHandler{provider: p}
}

func (h *GetIssueEventsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_issue_events",
		Description: "Get issue events (labeled, assigned, closed, reopened, etc.). Use when: tracking history, auditing changes, understanding timeline.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
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
			"required": []interface{}{"owner", "repo", "issue_number"},
		},
	}
}

func (h *GetIssueEventsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	opts := &github.ListOptions{}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	events, _, err := client.Issues.ListIssueEvents(ctx, owner, repo, issueNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get issue events: %v", err)), nil
	}

	data, _ := json.Marshal(events)
	return NewToolResult(string(data)), nil
}

// GetIssueTimelineHandler handles getting timeline events for an issue
type GetIssueTimelineHandler struct {
	provider *GitHubProvider
}

func NewGetIssueTimelineHandler(p *GitHubProvider) *GetIssueTimelineHandler {
	return &GetIssueTimelineHandler{provider: p}
}

func (h *GetIssueTimelineHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_issue_timeline",
		Description: "Get combined timeline (events + comments in order). Use when: full issue history, understanding context, comprehensive view.",
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
				"issue_number": map[string]interface{}{
					"type":        "integer",
					"description": "Issue number",
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
			"required": []interface{}{"owner", "repo", "issue_number"},
		},
	}
}

func (h *GetIssueTimelineHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	issueNumber := extractInt(params, "issue_number")

	opts := &github.ListOptions{}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	timeline, _, err := client.Issues.ListIssueTimeline(ctx, owner, repo, issueNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get issue timeline: %v", err)), nil
	}

	data, _ := json.Marshal(timeline)
	return NewToolResult(string(data)), nil
}
