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
		Description: GetOperationDescription("pull_request_create_graphql"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Pull request title",
					"example":     "Add new feature X",
					"minLength":   1,
					"maxLength":   256,
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Pull request description (supports Markdown)",
					"example":     "## Summary\n\nThis PR adds feature X which allows...\n\n## Changes\n- Added new component\n- Updated tests",
					"maxLength":   65536,
				},
				"head": map[string]interface{}{
					"type":        "string",
					"description": "Head branch name (source branch)",
					"example":     "feature-branch",
					"pattern":     "^[^\\s]+$",
					"minLength":   1,
					"maxLength":   255,
				},
				"base": map[string]interface{}{
					"type":        "string",
					"description": "Base branch name (target branch)",
					"example":     "main",
					"pattern":     "^[^\\s]+$",
					"minLength":   1,
					"maxLength":   255,
				},
				"draft": map[string]interface{}{
					"type":        "boolean",
					"description": "Create as draft pull request (not ready for review)",
					"default":     false,
					"example":     false,
				},
				"maintainer_can_modify": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow maintainers to edit this PR",
					"default":     true,
					"example":     true,
				},
			},
			"required": []string{"owner", "repo", "title", "head", "base"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "write:pull_request"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"mutation":     true,
		},
		ResponseExample: map[string]interface{}{
			"id":     "PR_kwDOABCD5M4Abcde",
			"number": 123,
			"title":  "Add new feature X",
			"url":    "https://github.com/octocat/Hello-World/pull/123",
			"state":  "OPEN",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Head branch not found",
				"cause":    "The specified head branch doesn't exist",
				"solution": "Ensure the head branch exists and is pushed to GitHub",
			},
			{
				"error":    "Base branch not found",
				"cause":    "The specified base branch doesn't exist",
				"solution": "Verify the base branch name (usually 'main' or 'master')",
			},
			{
				"error":    "No commits between branches",
				"cause":    "Head and base branches are identical",
				"solution": "Make commits to head branch before creating PR",
			},
			{
				"error":    "Pull request already exists",
				"cause":    "A PR from this head to base already exists",
				"solution": "Update the existing PR instead of creating a new one",
			},
		},
		ExtendedHelp: `The pull_request_create_graphql operation creates a new pull request using GitHub's GraphQL API.

Advantages over REST API:
- Returns GraphQL node ID for use in other mutations
- More consistent response format
- Can be combined with other GraphQL operations

Pull request best practices:
- Use descriptive titles that summarize the change
- Include context in the body (why, what, how)
- Reference related issues with #issue_number
- Use draft PRs for work in progress
- Add reviewers after creation

Markdown formatting in body:
- Use ## for sections
- Use - or * for bullet points
- Use triple backticks for code blocks
- Reference commits with SHA
- @mention users for attention

Examples:

# Basic PR
{
  "owner": "octocat",
  "repo": "Hello-World",
  "title": "Fix typo in README",
  "head": "fix-typo",
  "base": "main"
}

# PR with detailed description
{
  "owner": "octocat",
  "repo": "Hello-World",
  "title": "Add user authentication",
  "body": "## Summary\n\nImplements JWT-based authentication\n\n## Changes\n- Add login endpoint\n- Add JWT validation\n- Update user model\n\nFixes #42",
  "head": "feature/auth",
  "base": "develop"
}

# Draft PR
{
  "owner": "octocat",
  "repo": "Hello-World",
  "title": "WIP: Refactor database layer",
  "body": "Work in progress - do not review yet",
  "head": "refactor/db",
  "base": "main",
  "draft": true
}`,
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
	pullNumber := extractInt32(params, "pull_number")
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
		Description: GetOperationDescription("pull_request_review_add_graphql"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"pull_number": map[string]interface{}{
					"type":        "integer",
					"description": "Pull request number",
					"example":     42,
					"minimum":     1,
				},
				"event": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"},
					"description": "Review decision type",
					"example":     "APPROVE",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Review comment body (supports Markdown)",
					"example":     "LGTM! Great work on this feature.",
					"maxLength":   65536,
				},
			},
			"required": []string{"owner", "repo", "pull_number", "event"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "write:pull_request"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"mutation":     true,
		},
		ResponseExample: map[string]interface{}{
			"id":          "PRR_kwDOABCD5M4Abcde",
			"state":       "APPROVED",
			"body":        "LGTM! Great work on this feature.",
			"submittedAt": "2024-01-20T14:30:00Z",
			"author":      map[string]interface{}{"login": "reviewer1"},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Pull request not found",
				"cause":    "PR doesn't exist or you lack access",
				"solution": "Verify PR number and repository permissions",
			},
			{
				"error":    "Cannot approve own pull request",
				"cause":    "GitHub doesn't allow self-approval",
				"solution": "Have another team member review the PR",
			},
			{
				"error":    "Review already exists",
				"cause":    "You've already reviewed this PR",
				"solution": "Dismiss the previous review first if you want to re-review",
			},
		},
		ExtendedHelp: `The pull_request_review_add_graphql operation adds a review to a pull request using GraphQL.

Review types:
- APPROVE: Approve the PR for merging
- REQUEST_CHANGES: Request changes before approval
- COMMENT: Add general feedback without approval decision

Review guidelines:
- Cannot approve your own PR
- Only one pending review per user at a time
- Reviews can be dismissed by repo admins
- Required reviews block merging if configured

Examples:

# Approve a PR
{
  "owner": "octocat",
  "repo": "Hello-World",
  "pull_number": 42,
  "event": "APPROVE",
  "body": "LGTM! Ship it! 🚀"
}

# Request changes
{
  "owner": "octocat",
  "repo": "Hello-World",
  "pull_number": 42,
  "event": "REQUEST_CHANGES",
  "body": "Please address the following:\n- Fix the memory leak in line 42\n- Add unit tests"
}

# Add comment without approval
{
  "owner": "octocat",
  "repo": "Hello-World",
  "pull_number": 42,
  "event": "COMMENT",
  "body": "Nice approach! Consider using a Map instead of Object for better performance."
}`,
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
	pullNumber := extractInt32(params, "pull_number")
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
		Description: GetOperationDescription("pull_request_merge_graphql"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"pull_number": map[string]interface{}{
					"type":        "integer",
					"description": "Pull request number to merge",
					"example":     42,
					"minimum":     1,
				},
				"merge_method": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"MERGE", "SQUASH", "REBASE"},
					"description": "Method to use for merging (MERGE creates merge commit, SQUASH combines commits, REBASE replays commits)",
					"default":     "MERGE",
					"example":     "SQUASH",
				},
				"commit_title": map[string]interface{}{
					"type":        "string",
					"description": "Custom title for the merge commit (defaults to PR title)",
					"example":     "feat: Add user authentication (#42)",
					"maxLength":   256,
				},
				"commit_message": map[string]interface{}{
					"type":        "string",
					"description": "Custom body for the merge commit (defaults to PR description)",
					"example":     "Implements JWT-based authentication\n\nCo-authored-by: contributor <email>",
					"maxLength":   65536,
				},
			},
			"required": []string{"owner", "repo", "pull_number"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "write:pull_request"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"mutation":     true,
			"destructive":  false, // Merging is generally not considered destructive
		},
		ResponseExample: map[string]interface{}{
			"mergeCommit": map[string]interface{}{
				"oid":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
				"message": "Merge pull request #42 from feature-branch",
				"url":     "https://github.com/octocat/Hello-World/commit/6dcb09b5b57875f334f61aebed695e2e4193db5e",
			},
			"pullRequest": map[string]interface{}{
				"state":    "MERGED",
				"mergedAt": "2024-01-20T15:00:00Z",
				"mergedBy": map[string]interface{}{"login": "octocat"},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Pull request is not mergeable",
				"cause":    "Conflicts, failing checks, or missing reviews",
				"solution": "Resolve conflicts, fix failing checks, or get required approvals",
			},
			{
				"error":    "Merge method not allowed",
				"cause":    "Repository settings restrict certain merge methods",
				"solution": "Use an allowed merge method or update repository settings",
			},
			{
				"error":    "Base branch protection",
				"cause":    "Branch protection rules prevent merging",
				"solution": "Ensure all branch protection requirements are met",
			},
			{
				"error":    "Pull request already merged",
				"cause":    "PR has already been merged",
				"solution": "No action needed - PR is already merged",
			},
		},
		ExtendedHelp: `The pull_request_merge_graphql operation merges a pull request using GitHub's GraphQL API.

Merge methods:
- MERGE: Creates a merge commit with all commits preserved
- SQUASH: Combines all commits into a single commit
- REBASE: Replays commits on top of base branch

Pre-merge checklist:
- All CI checks passing
- Required reviews approved
- No merge conflicts
- Branch protection rules satisfied

Commit message formatting:
- Title: Keep under 72 characters
- Body: Add details, co-authors, issue references
- Use conventional commits format if required

Examples:

# Basic merge
{
  "owner": "octocat",
  "repo": "Hello-World",
  "pull_number": 42
}

# Squash merge with custom message
{
  "owner": "octocat",
  "repo": "Hello-World",
  "pull_number": 42,
  "merge_method": "SQUASH",
  "commit_title": "feat: Add authentication system (#42)",
  "commit_message": "Implements JWT-based auth with refresh tokens\n\nCloses #35\nCo-authored-by: alice <alice@example.com>"
}

# Rebase merge
{
  "owner": "octocat",
  "repo": "Hello-World",
  "pull_number": 42,
  "merge_method": "REBASE"
}

Post-merge actions:
- Delete feature branch (if configured)
- Trigger deployment workflows
- Update related issues
- Notify team members`,
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
		Description: GetOperationDescription("issue_create_graphql"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Issue title",
					"example":     "Bug: Login button not working on mobile",
					"minLength":   1,
					"maxLength":   256,
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Issue description (supports Markdown)",
					"example":     "## Description\n\nThe login button is unresponsive on mobile devices.\n\n## Steps to reproduce\n1. Open app on mobile\n2. Click login\n3. Nothing happens",
					"maxLength":   65536,
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Labels to add to the issue (must exist in repository)",
					"example":     []string{"bug", "mobile", "high-priority"},
					"maxItems":    100,
				},
				"assignees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "GitHub usernames to assign (users must have access to repository)",
					"example":     []string{"developer1", "developer2"},
					"maxItems":    10,
				},
				"milestone_number": map[string]interface{}{
					"type":        "integer",
					"description": "Milestone number to associate with the issue",
					"example":     3,
					"minimum":     1,
				},
			},
			"required": []string{"owner", "repo", "title"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "write:issue"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
			"mutation":     true,
		},
		ResponseExample: map[string]interface{}{
			"id":        "I_kwDOABCD5M4Abcde",
			"number":    42,
			"title":     "Bug: Login button not working on mobile",
			"body":      "## Description\n\nThe login button is unresponsive...",
			"state":     "OPEN",
			"url":       "https://github.com/octocat/Hello-World/issues/42",
			"createdAt": "2024-01-20T10:00:00Z",
			"author":    map[string]interface{}{"login": "octocat"},
			"labels": []map[string]interface{}{
				{"name": "bug", "color": "d73a4a"},
				{"name": "mobile", "color": "0366d6"},
			},
			"assignees": []map[string]interface{}{
				{"login": "developer1"},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Repository not found",
				"cause":    "Repository doesn't exist or you lack access",
				"solution": "Verify repository name and permissions",
			},
			{
				"error":    "Label not found",
				"cause":    "Specified label doesn't exist in repository",
				"solution": "Create the label first or use existing labels",
			},
			{
				"error":    "Cannot assign user",
				"cause":    "User doesn't have access to repository",
				"solution": "Only assign users who are collaborators or org members",
			},
			{
				"error":    "Issues disabled",
				"cause":    "Repository has issues disabled",
				"solution": "Enable issues in repository settings",
			},
		},
		ExtendedHelp: `The issue_create_graphql operation creates a new issue using GitHub's GraphQL API.

Issue writing tips:
- Use clear, descriptive titles
- Include reproduction steps for bugs
- Add relevant context and screenshots
- Link related issues/PRs with #number
- Use task lists with - [ ] for tracking

Markdown formatting:
- Headers: ##, ###
- Bold: **text**
- Code blocks: triple backticks with language
- Tables, lists, and quotes supported

Label strategies:
- bug: For defects
- enhancement: For improvements
- documentation: For docs
- good first issue: For newcomers
- help wanted: For community contributions

Examples:

# Simple bug report
{
  "owner": "octocat",
  "repo": "Hello-World",
  "title": "Button click not working",
  "body": "The submit button on the form page doesn't respond to clicks.",
  "labels": ["bug"]
}

# Feature request with details
{
  "owner": "octocat",
  "repo": "Hello-World",
  "title": "Add dark mode support",
  "body": "## Feature Request\n\n### Description\nAdd a toggle for dark mode in settings\n\n### Benefits\n- Better for night reading\n- Reduces eye strain\n\n### Implementation ideas\n- Use CSS variables\n- Store preference in localStorage",
  "labels": ["enhancement", "ui"],
  "assignees": ["designer1"]
}

# Task tracking issue
{
  "owner": "octocat",
  "repo": "Hello-World",
  "title": "Q1 2024 Roadmap",
  "body": "## Tasks\n- [ ] Implement auth\n- [ ] Add API endpoints\n- [ ] Write tests\n- [ ] Update documentation",
  "labels": ["epic", "planning"],
  "milestone_number": 5
}`,
	}
}
