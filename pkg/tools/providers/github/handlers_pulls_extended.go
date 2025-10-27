package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Pull Request Extended Operations

// UpdatePullRequestBranchHandler handles updating a PR branch with base branch
type UpdatePullRequestBranchHandler struct {
	provider *GitHubProvider
}

func NewUpdatePullRequestBranchHandler(p *GitHubProvider) *UpdatePullRequestBranchHandler {
	return &UpdatePullRequestBranchHandler{provider: p}
}

func (h *UpdatePullRequestBranchHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_pull_request_branch",
		Description: "Update PR branch with latest from base (merge base into head). Use when: resolving conflicts, updating stale PR, syncing with main.",
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

func (h *UpdatePullRequestBranchHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	_, _, err := client.PullRequests.UpdateBranch(ctx, owner, repo, pullNumber, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update pull request branch: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status":  "updated",
		"message": "Pull request branch updated with base branch",
	}), nil
}

// GetPullRequestDiffHandler handles getting PR diff
type GetPullRequestDiffHandler struct {
	provider *GitHubProvider
}

func NewGetPullRequestDiffHandler(p *GitHubProvider) *GetPullRequestDiffHandler {
	return &GetPullRequestDiffHandler{provider: p}
}

func (h *GetPullRequestDiffHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_pull_request_diff",
		Description: "Get PR unified diff (full patch text). Use when: detailed code review, analyzing changes, generating changelog.",
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

func (h *GetPullRequestDiffHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	// Get raw diff
	diff, _, err := client.PullRequests.GetRaw(ctx, owner, repo, pullNumber, github.RawOptions{Type: github.Diff})
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get pull request diff: %v", err)), nil
	}

	return NewToolResult(map[string]interface{}{
		"diff":        diff,
		"pull_number": pullNumber,
	}), nil
}

// GetPullRequestReviewsHandler handles listing PR reviews
type GetPullRequestReviewsHandler struct {
	provider *GitHubProvider
}

func NewGetPullRequestReviewsHandler(p *GitHubProvider) *GetPullRequestReviewsHandler {
	return &GetPullRequestReviewsHandler{provider: p}
}

func (h *GetPullRequestReviewsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_pull_request_reviews",
		Description: "Get PR reviews (reviewer, state, body, submitted_at). Use when: checking review status, seeing feedback, tracking approvals.",
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

func (h *GetPullRequestReviewsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	pagination := ExtractPagination(params)
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repo, pullNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list pull request reviews: %v", err)), nil
	}

	data, _ := json.Marshal(reviews)
	return NewToolResult(string(data)), nil
}

// GetPullRequestReviewCommentsHandler handles listing review comments
type GetPullRequestReviewCommentsHandler struct {
	provider *GitHubProvider
}

func NewGetPullRequestReviewCommentsHandler(p *GitHubProvider) *GetPullRequestReviewCommentsHandler {
	return &GetPullRequestReviewCommentsHandler{provider: p}
}

func (h *GetPullRequestReviewCommentsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_pull_request_review_comments",
		Description: "Get PR review comments (line, path, body, author, position). Use when: reading feedback, addressing comments, tracking discussion.",
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
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort by: created, updated",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "Sort direction: asc or desc",
				},
				"since": map[string]interface{}{
					"type":        "string",
					"description": "Only comments updated after this time (ISO 8601)",
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

func (h *GetPullRequestReviewCommentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	opts := &github.PullRequestListCommentsOptions{}
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

	comments, _, err := client.PullRequests.ListComments(ctx, owner, repo, pullNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list review comments: %v", err)), nil
	}

	data, _ := json.Marshal(comments)
	return NewToolResult(string(data)), nil
}

// CreatePullRequestReviewHandler handles creating a PR review
type CreatePullRequestReviewHandler struct {
	provider *GitHubProvider
}

func NewCreatePullRequestReviewHandler(p *GitHubProvider) *CreatePullRequestReviewHandler {
	return &CreatePullRequestReviewHandler{provider: p}
}

func (h *CreatePullRequestReviewHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_pull_request_review",
		Description: "Start PR review (create pending review, add comments). Use when: beginning code review, batching comments, structured review.",
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
				"event": map[string]interface{}{
					"type":        "string",
					"description": "Review action: APPROVE, REQUEST_CHANGES, COMMENT",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Review body text",
				},
				"comments": map[string]interface{}{
					"type":        "array",
					"description": "Draft review comments",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "File path",
							},
							"position": map[string]interface{}{
								"type":        "integer",
								"description": "Position in diff",
							},
							"body": map[string]interface{}{
								"type":        "string",
								"description": "Comment text",
							},
						},
					},
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number"},
		},
	}
}

func (h *CreatePullRequestReviewHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")

	review := &github.PullRequestReviewRequest{}

	if event, ok := params["event"].(string); ok {
		review.Event = &event
	}
	if body, ok := params["body"].(string); ok {
		review.Body = &body
	}

	if comments, ok := params["comments"].([]interface{}); ok {
		var draftComments []*github.DraftReviewComment
		for _, c := range comments {
			if comment, ok := c.(map[string]interface{}); ok {
				draftComment := &github.DraftReviewComment{}
				if path, ok := comment["path"].(string); ok {
					draftComment.Path = &path
				}
				if position, ok := comment["position"].(float64); ok {
					pos := int(position)
					draftComment.Position = &pos
				}
				if body, ok := comment["body"].(string); ok {
					draftComment.Body = &body
				}
				draftComments = append(draftComments, draftComment)
			}
		}
		review.Comments = draftComments
	}

	createdReview, _, err := client.PullRequests.CreateReview(ctx, owner, repo, pullNumber, review)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create review: %v", err)), nil
	}

	data, _ := json.Marshal(createdReview)
	return NewToolResult(string(data)), nil
}

// SubmitPullRequestReviewHandler handles submitting a pending review
type SubmitPullRequestReviewHandler struct {
	provider *GitHubProvider
}

func NewSubmitPullRequestReviewHandler(p *GitHubProvider) *SubmitPullRequestReviewHandler {
	return &SubmitPullRequestReviewHandler{provider: p}
}

func (h *SubmitPullRequestReviewHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "submit_pull_request_review",
		Description: "Submit review with state (APPROVE, REQUEST_CHANGES, COMMENT). Use when: finishing review, approving PR, requesting changes.",
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
				"review_id": map[string]interface{}{
					"type":        "integer",
					"description": "Review ID",
				},
				"event": map[string]interface{}{
					"type":        "string",
					"description": "Review action: APPROVE, REQUEST_CHANGES, COMMENT",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Review body text",
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number", "review_id", "event"},
		},
	}
}

func (h *SubmitPullRequestReviewHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")
	reviewID := int64(extractInt(params, "review_id"))
	event := extractString(params, "event")

	review := &github.PullRequestReviewRequest{
		Event: &event,
	}

	if body, ok := params["body"].(string); ok {
		review.Body = &body
	}

	submittedReview, _, err := client.PullRequests.SubmitReview(ctx, owner, repo, pullNumber, reviewID, review)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to submit review: %v", err)), nil
	}

	data, _ := json.Marshal(submittedReview)
	return NewToolResult(string(data)), nil
}

// AddPullRequestReviewCommentHandler handles adding a review comment
type AddPullRequestReviewCommentHandler struct {
	provider *GitHubProvider
}

func NewAddPullRequestReviewCommentHandler(p *GitHubProvider) *AddPullRequestReviewCommentHandler {
	return &AddPullRequestReviewCommentHandler{provider: p}
}

func (h *AddPullRequestReviewCommentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "add_pull_request_review_comment",
		Description: "Add inline comment to PR file/line. Use when: commenting on code, suggesting change, asking question.",
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
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Comment text",
				},
				"commit_id": map[string]interface{}{
					"type":        "string",
					"description": "SHA of the commit to comment on",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to comment on",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number to comment on",
				},
				"side": map[string]interface{}{
					"type":        "string",
					"description": "Side of the diff: LEFT or RIGHT",
				},
			},
			"required": []interface{}{"owner", "repo", "pull_number", "body", "commit_id", "path"},
		},
	}
}

func (h *AddPullRequestReviewCommentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")
	body := extractString(params, "body")
	commitID := extractString(params, "commit_id")
	path := extractString(params, "path")

	comment := &github.PullRequestComment{
		Body:     &body,
		CommitID: &commitID,
		Path:     &path,
	}

	if line, ok := params["line"].(float64); ok {
		lineInt := int(line)
		comment.Line = &lineInt
	}
	if side, ok := params["side"].(string); ok {
		comment.Side = &side
	}

	createdComment, _, err := client.PullRequests.CreateComment(ctx, owner, repo, pullNumber, comment)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to add review comment: %v", err)), nil
	}

	data, _ := json.Marshal(createdComment)
	return NewToolResult(string(data)), nil
}
