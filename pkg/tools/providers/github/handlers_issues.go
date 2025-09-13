package github

import (
	"context"
	"encoding/json"
	"fmt"

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
		Description: "Get a specific issue from a GitHub repository",
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

func (h *GetIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
		Description: "Search for issues on GitHub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub issue search syntax",
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

func (h *SearchIssuesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search issues: %v", err)), nil
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
		Description: "List issues from a GitHub repository",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

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

	data, _ := json.Marshal(issues)
	return NewToolResult(string(data)), nil
}

func (h *ListIssuesHandler) executeGraphQL(ctx context.Context, client *githubv4.Client, params map[string]interface{}) (*ToolResult, error) {
	// Extract owner and repo with safe type assertions
	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	if owner == "" || repo == "" {
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
		Description: "Get comments for a specific issue",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
		Description: "Create a new issue in a GitHub repository",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	title, _ := params["title"].(string)

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
		Description: "Add a comment to a GitHub issue",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))
	body, _ := params["body"].(string)

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
		Description: "Update an existing GitHub issue",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
		Description: "Lock an issue to prevent further comments",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
		Description: "Unlock an issue to allow comments",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
		Description: "Get events for a GitHub issue",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
		Description: "Get timeline events for a GitHub issue",
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

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	issueNumber := int(params["issue_number"].(float64))

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
