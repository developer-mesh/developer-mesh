package github

import (
	"context"

	"github.com/shurcooL/githubv4"
)

// CreatePullRequestGraphQLHandler creates a pull request using GraphQL
type CreatePullRequestGraphQLHandler struct {
	provider *GitHubProvider
}

// NewCreatePullRequestGraphQLHandler creates a new handler
func NewCreatePullRequestGraphQLHandler(p *GitHubProvider) *CreatePullRequestGraphQLHandler {
	return &CreatePullRequestGraphQLHandler{provider: p}
}

// Execute creates a pull request via GraphQL mutation
func (h *CreatePullRequestGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	title := extractString(params, "title")
	body := extractString(params, "body")
	head := extractString(params, "head")
	base := extractString(params, "base")
	draft := extractBool(params, "draft")

	if owner == "" || repo == "" || title == "" || head == "" || base == "" {
		return ErrorResult("owner, repo, title, head, and base are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// First, get the repository ID
	var repoQuery struct {
		Repository struct {
			ID string
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	err = client.Query(ctx, &repoQuery, map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	})
	if err != nil {
		return ErrorResult("Failed to get repository ID: %v", err), nil
	}

	// Create the pull request
	var mutation struct {
		CreatePullRequest struct {
			PullRequest struct {
				ID     string
				Number int
				Title  string
				URL    string
				State  string
			}
		} `graphql:"createPullRequest(input: $input)"`
	}

	input := githubv4.CreatePullRequestInput{
		RepositoryID: githubv4.ID(repoQuery.Repository.ID),
		Title:        githubv4.String(title),
		HeadRefName:  githubv4.String(head),
		BaseRefName:  githubv4.String(base),
	}

	if body != "" {
		input.Body = githubv4.NewString(githubv4.String(body))
	}

	if draft {
		input.Draft = githubv4.NewBoolean(githubv4.Boolean(draft))
	}

	err = client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return ErrorResult("Failed to create pull request: %v", err), nil
	}

	result := map[string]interface{}{
		"id":     mutation.CreatePullRequest.PullRequest.ID,
		"number": mutation.CreatePullRequest.PullRequest.Number,
		"title":  mutation.CreatePullRequest.PullRequest.Title,
		"url":    mutation.CreatePullRequest.PullRequest.URL,
		"state":  mutation.CreatePullRequest.PullRequest.State,
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *CreatePullRequestGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "pull_request_create_graphql",
		Description: "Create a pull request using GraphQL mutation",
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
					"description": "Pull request description",
				},
				"head": map[string]interface{}{
					"type":        "string",
					"description": "Head branch name",
				},
				"base": map[string]interface{}{
					"type":        "string",
					"description": "Base branch name",
				},
				"draft": map[string]interface{}{
					"type":        "boolean",
					"description": "Create as draft PR",
					"default":     false,
				},
			},
			"required": []string{"owner", "repo", "title", "head", "base"},
		},
	}
}

// AddPullRequestReviewGraphQLHandler adds a review to a PR using GraphQL
type AddPullRequestReviewGraphQLHandler struct {
	provider *GitHubProvider
}

// NewAddPullRequestReviewGraphQLHandler creates a new handler
func NewAddPullRequestReviewGraphQLHandler(p *GitHubProvider) *AddPullRequestReviewGraphQLHandler {
	return &AddPullRequestReviewGraphQLHandler{provider: p}
}

// Execute adds a review via GraphQL mutation
func (h *AddPullRequestReviewGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")
	event := extractString(params, "event") // APPROVE, REQUEST_CHANGES, COMMENT
	body := extractString(params, "body")

	if owner == "" || repo == "" || pullNumber == 0 || event == "" {
		return ErrorResult("owner, repo, pull_number, and event are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// Get the pull request node ID
	var prQuery struct {
		Repository struct {
			PullRequest struct {
				ID string
			} `graphql:"pullRequest(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	err = client.Query(ctx, &prQuery, map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(pullNumber),
	})
	if err != nil {
		return ErrorResult("Failed to get pull request ID: %v", err), nil
	}

	// Create the review
	var mutation struct {
		AddPullRequestReview struct {
			PullRequestReview struct {
				ID     string
				State  string
				Body   string
				Author struct {
					Login string
				}
				SubmittedAt *githubv4.DateTime
			}
		} `graphql:"addPullRequestReview(input: $input)"`
	}

	// Map event string to GraphQL enum
	var reviewEvent githubv4.PullRequestReviewEvent
	switch event {
	case "APPROVE":
		reviewEvent = githubv4.PullRequestReviewEventApprove
	case "REQUEST_CHANGES":
		reviewEvent = githubv4.PullRequestReviewEventRequestChanges
	case "COMMENT":
		reviewEvent = githubv4.PullRequestReviewEventComment
	default:
		return ErrorResult("Invalid event type. Must be APPROVE, REQUEST_CHANGES, or COMMENT"), nil
	}

	input := githubv4.AddPullRequestReviewInput{
		PullRequestID: githubv4.ID(prQuery.Repository.PullRequest.ID),
		Event:         &reviewEvent,
	}

	if body != "" {
		input.Body = githubv4.NewString(githubv4.String(body))
	}

	err = client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return ErrorResult("Failed to add review: %v", err), nil
	}

	result := map[string]interface{}{
		"id":     mutation.AddPullRequestReview.PullRequestReview.ID,
		"state":  mutation.AddPullRequestReview.PullRequestReview.State,
		"body":   mutation.AddPullRequestReview.PullRequestReview.Body,
		"author": mutation.AddPullRequestReview.PullRequestReview.Author.Login,
	}

	if mutation.AddPullRequestReview.PullRequestReview.SubmittedAt != nil {
		result["submitted_at"] = mutation.AddPullRequestReview.PullRequestReview.SubmittedAt.Time
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *AddPullRequestReviewGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "pull_request_review_add_graphql",
		Description: "Add a review to a pull request using GraphQL",
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
					"enum":        []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"},
					"description": "Review event type",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Review comment body",
				},
			},
			"required": []string{"owner", "repo", "pull_number", "event"},
		},
	}
}

// MergePullRequestGraphQLHandler merges a PR using GraphQL
type MergePullRequestGraphQLHandler struct {
	provider *GitHubProvider
}

// NewMergePullRequestGraphQLHandler creates a new handler
func NewMergePullRequestGraphQLHandler(p *GitHubProvider) *MergePullRequestGraphQLHandler {
	return &MergePullRequestGraphQLHandler{provider: p}
}

// Execute merges a pull request via GraphQL mutation
func (h *MergePullRequestGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	pullNumber := extractInt(params, "pull_number")
	mergeMethod := extractString(params, "merge_method") // MERGE, SQUASH, REBASE
	commitTitle := extractString(params, "commit_title")
	commitMessage := extractString(params, "commit_message")

	if owner == "" || repo == "" || pullNumber == 0 {
		return ErrorResult("owner, repo, and pull_number are required"), nil
	}

	if mergeMethod == "" {
		mergeMethod = "MERGE"
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// Get the pull request node ID
	var prQuery struct {
		Repository struct {
			PullRequest struct {
				ID         string
				Mergeable  githubv4.MergeableState
				HeadRefOid string
			} `graphql:"pullRequest(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	err = client.Query(ctx, &prQuery, map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(pullNumber),
	})
	if err != nil {
		return ErrorResult("Failed to get pull request: %v", err), nil
	}

	// Check if mergeable
	if prQuery.Repository.PullRequest.Mergeable != githubv4.MergeableStateMergeable {
		return ErrorResult("Pull request is not mergeable. Current state: %s", prQuery.Repository.PullRequest.Mergeable), nil
	}

	// Merge the pull request
	var mutation struct {
		MergePullRequest struct {
			PullRequest struct {
				ID       string
				Number   int
				State    string
				Merged   bool
				MergedAt *githubv4.DateTime
				MergedBy struct {
					Login string
				}
			}
		} `graphql:"mergePullRequest(input: $input)"`
	}

	// Map merge method string to GraphQL enum
	var method githubv4.PullRequestMergeMethod
	switch mergeMethod {
	case "MERGE":
		method = githubv4.PullRequestMergeMethodMerge
	case "SQUASH":
		method = githubv4.PullRequestMergeMethodSquash
	case "REBASE":
		method = githubv4.PullRequestMergeMethodRebase
	default:
		return ErrorResult("Invalid merge method. Must be MERGE, SQUASH, or REBASE"), nil
	}

	input := githubv4.MergePullRequestInput{
		PullRequestID: githubv4.ID(prQuery.Repository.PullRequest.ID),
		MergeMethod:   &method,
	}

	if commitTitle != "" {
		input.CommitHeadline = githubv4.NewString(githubv4.String(commitTitle))
	}

	if commitMessage != "" {
		input.CommitBody = githubv4.NewString(githubv4.String(commitMessage))
	}

	// Set the expected head OID to ensure we're merging the expected state
	input.ExpectedHeadOid = githubv4.NewGitObjectID(githubv4.GitObjectID(prQuery.Repository.PullRequest.HeadRefOid))

	err = client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return ErrorResult("Failed to merge pull request: %v", err), nil
	}

	result := map[string]interface{}{
		"id":        mutation.MergePullRequest.PullRequest.ID,
		"number":    mutation.MergePullRequest.PullRequest.Number,
		"state":     mutation.MergePullRequest.PullRequest.State,
		"merged":    mutation.MergePullRequest.PullRequest.Merged,
		"merged_by": mutation.MergePullRequest.PullRequest.MergedBy.Login,
	}

	if mutation.MergePullRequest.PullRequest.MergedAt != nil {
		result["merged_at"] = mutation.MergePullRequest.PullRequest.MergedAt.Time
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *MergePullRequestGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "pull_request_merge_graphql",
		Description: "Merge a pull request using GraphQL",
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
				"merge_method": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"MERGE", "SQUASH", "REBASE"},
					"description": "Merge method to use",
					"default":     "MERGE",
				},
				"commit_title": map[string]interface{}{
					"type":        "string",
					"description": "Title for the merge commit",
				},
				"commit_message": map[string]interface{}{
					"type":        "string",
					"description": "Body for the merge commit",
				},
			},
			"required": []string{"owner", "repo", "pull_number"},
		},
	}
}

// CreateIssueGraphQLHandler creates an issue using GraphQL
type CreateIssueGraphQLHandler struct {
	provider *GitHubProvider
}

// NewCreateIssueGraphQLHandler creates a new handler
func NewCreateIssueGraphQLHandler(p *GitHubProvider) *CreateIssueGraphQLHandler {
	return &CreateIssueGraphQLHandler{provider: p}
}

// Execute creates an issue via GraphQL mutation
func (h *CreateIssueGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	title := extractString(params, "title")
	body := extractString(params, "body")

	if owner == "" || repo == "" || title == "" {
		return ErrorResult("owner, repo, and title are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// Get repository ID and label IDs if provided
	var repoQuery struct {
		Repository struct {
			ID     string
			Labels struct {
				Nodes []struct {
					ID   string
					Name string
				}
			} `graphql:"labels(first: 100, query: $labelQuery)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Build label query if labels are provided
	labelQuery := ""
	if labelsRaw, ok := params["labels"]; ok {
		if labels, ok := labelsRaw.([]string); ok && len(labels) > 0 {
			// GraphQL doesn't support exact label matching in query, so we fetch all and filter
			labelQuery = ""
		}
	}

	err = client.Query(ctx, &repoQuery, map[string]interface{}{
		"owner":      githubv4.String(owner),
		"repo":       githubv4.String(repo),
		"labelQuery": githubv4.String(labelQuery),
	})
	if err != nil {
		return ErrorResult("Failed to get repository: %v", err), nil
	}

	// Create the issue
	var mutation struct {
		CreateIssue struct {
			Issue struct {
				ID     string
				Number int
				Title  string
				Body   string
				State  string
				URL    string
				Author struct {
					Login string
				}
			}
		} `graphql:"createIssue(input: $input)"`
	}

	input := githubv4.CreateIssueInput{
		RepositoryID: githubv4.ID(repoQuery.Repository.ID),
		Title:        githubv4.String(title),
	}

	if body != "" {
		input.Body = githubv4.NewString(githubv4.String(body))
	}

	// Add label IDs if labels were provided
	if labelsRaw, ok := params["labels"]; ok {
		if labels, ok := labelsRaw.([]string); ok && len(labels) > 0 {
			labelIDs := []githubv4.ID{}
			for _, labelName := range labels {
				for _, repoLabel := range repoQuery.Repository.Labels.Nodes {
					if repoLabel.Name == labelName {
						labelIDs = append(labelIDs, githubv4.ID(repoLabel.ID))
						break
					}
				}
			}
			if len(labelIDs) > 0 {
				input.LabelIDs = &labelIDs
			}
		}
	}

	// Add assignees if provided
	// Note: GraphQL API requires user node IDs for assignees
	// For simplicity, we're omitting assignees as it requires additional queries
	// In production, you'd query for user IDs first
	// TODO: Implement assignee support with proper user ID resolution

	err = client.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return ErrorResult("Failed to create issue: %v", err), nil
	}

	result := map[string]interface{}{
		"id":     mutation.CreateIssue.Issue.ID,
		"number": mutation.CreateIssue.Issue.Number,
		"title":  mutation.CreateIssue.Issue.Title,
		"body":   mutation.CreateIssue.Issue.Body,
		"state":  mutation.CreateIssue.Issue.State,
		"url":    mutation.CreateIssue.Issue.URL,
		"author": mutation.CreateIssue.Issue.Author.Login,
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *CreateIssueGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "issue_create_graphql",
		Description: "Create an issue using GraphQL",
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
					"items":       map[string]interface{}{"type": "string"},
					"description": "Labels to add to the issue",
				},
				"assignees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Usernames to assign (note: may require additional permissions)",
				},
			},
			"required": []string{"owner", "repo", "title"},
		},
	}
}
