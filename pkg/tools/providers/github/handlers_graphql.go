package github

import (
	"context"

	"github.com/shurcooL/githubv4"
)

// GraphQL fragment definitions for common data structures

// IssueFragment represents the common issue fields we fetch
type IssueFragment struct {
	ID        string
	Number    int
	Title     string
	Body      string
	State     string
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	ClosedAt  *githubv4.DateTime
	Author    struct {
		Login string
	}
	Labels struct {
		Nodes []struct {
			Name  string
			Color string
		}
	} `graphql:"labels(first: 10)"`
	Assignees struct {
		Nodes []struct {
			Login string
		}
	} `graphql:"assignees(first: 10)"`
	Comments struct {
		TotalCount int
	}
	Milestone *struct {
		Title string
		State string
	}
}

// PullRequestFragment represents common PR fields
type PullRequestFragment struct {
	ID         string
	Number     int
	Title      string
	Body       string
	State      string
	IsDraft    bool
	Merged     bool
	MergedAt   *githubv4.DateTime
	CreatedAt  githubv4.DateTime
	UpdatedAt  githubv4.DateTime
	ClosedAt   *githubv4.DateTime
	HeadRefOid string
	BaseRefOid string
	Author     struct {
		Login string
	}
	HeadRef struct {
		Name   string
		Target struct {
			Oid string
		}
	}
	BaseRef struct {
		Name   string
		Target struct {
			Oid string
		}
	}
	Labels struct {
		Nodes []struct {
			Name  string
			Color string
		}
	} `graphql:"labels(first: 10)"`
	Reviews struct {
		TotalCount int
		Nodes      []struct {
			State  string
			Author struct {
				Login string
			}
		}
	} `graphql:"reviews(first: 10)"`
	Comments struct {
		TotalCount int
	}
	Commits struct {
		TotalCount int
	}
	ChangedFiles int
}

// RepositoryFragment represents common repository fields
type RepositoryFragment struct {
	ID               string
	Name             string
	NameWithOwner    string
	Description      *string
	IsPrivate        bool
	IsArchived       bool
	IsFork           bool
	IsTemplate       bool
	DefaultBranchRef *struct {
		Name string
	}
	PrimaryLanguage *struct {
		Name  string
		Color string
	}
	Languages struct {
		Nodes []struct {
			Name  string
			Color string
		}
	} `graphql:"languages(first: 10)"`
	StargazerCount  int
	ForkCount       int
	WatcherCount    int
	OpenIssuesCount int `graphql:"issues(states: OPEN) { totalCount }"`
	CreatedAt       githubv4.DateTime
	UpdatedAt       githubv4.DateTime
	PushedAt        *githubv4.DateTime
	LicenseInfo     *struct {
		Name     string
		SpdxId   string
		Nickname *string
	}
}

// Handler implementations using GraphQL

// ListIssuesGraphQLHandler lists issues using GraphQL for efficient filtering
type ListIssuesGraphQLHandler struct {
	provider *GitHubProvider
}

// NewListIssuesGraphQLHandler creates a new GraphQL issues list handler
func NewListIssuesGraphQLHandler(p *GitHubProvider) *ListIssuesGraphQLHandler {
	return &ListIssuesGraphQLHandler{provider: p}
}

// Execute runs the GraphQL query to list issues
func (h *ListIssuesGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	if owner == "" || repo == "" {
		return ErrorResult("owner and repo are required"), nil
	}

	// Get GraphQL client
	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// Build the query
	var query struct {
		Repository struct {
			Issues struct {
				Nodes    []IssueFragment
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				TotalCount int
			} `graphql:"issues(first: $first, after: $after, states: $states, labels: $labels, orderBy: $orderBy)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Build variables
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
		"first": githubv4.Int(30),
	}

	// Handle optional parameters
	if after := extractString(params, "after"); after != "" {
		variables["after"] = githubv4.String(after)
	} else {
		variables["after"] = (*githubv4.String)(nil)
	}

	// Handle state filter
	state := extractString(params, "state")
	if state == "" {
		state = "open"
	}
	states := []githubv4.IssueState{}
	switch state {
	case "open":
		states = append(states, githubv4.IssueStateOpen)
	case "closed":
		states = append(states, githubv4.IssueStateClosed)
	case "all":
		states = append(states, githubv4.IssueStateOpen, githubv4.IssueStateClosed)
	}
	variables["states"] = states

	// Handle labels filter
	if labelsRaw, ok := params["labels"]; ok {
		if labels, ok := labelsRaw.([]string); ok && len(labels) > 0 {
			labelStrings := make([]githubv4.String, len(labels))
			for i, label := range labels {
				labelStrings[i] = githubv4.String(label)
			}
			variables["labels"] = labelStrings
		} else {
			variables["labels"] = (*[]githubv4.String)(nil)
		}
	} else {
		variables["labels"] = (*[]githubv4.String)(nil)
	}

	// Handle order
	variables["orderBy"] = githubv4.IssueOrder{
		Field:     githubv4.IssueOrderFieldUpdatedAt,
		Direction: githubv4.OrderDirectionDesc,
	}

	// Execute query
	err = client.Query(ctx, &query, variables)
	if err != nil {
		return ErrorResult("GraphQL query failed: %v", err), nil
	}

	// Convert to response format
	issues := make([]map[string]interface{}, 0, len(query.Repository.Issues.Nodes))
	for _, issue := range query.Repository.Issues.Nodes {
		issues = append(issues, convertIssueFragment(issue))
	}

	result := map[string]interface{}{
		"issues": issues,
		"pageInfo": map[string]interface{}{
			"hasNextPage": query.Repository.Issues.PageInfo.HasNextPage,
			"endCursor":   string(query.Repository.Issues.PageInfo.EndCursor),
		},
		"totalCount": query.Repository.Issues.TotalCount,
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *ListIssuesGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "issues_list_graphql",
		Description: "List repository issues using GraphQL for efficient filtering and pagination",
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
					"enum":        []string{"open", "closed", "all"},
					"description": "Issue state filter",
					"default":     "open",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Filter by labels",
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination",
				},
			},
			"required": []string{"owner", "repo"},
		},
	}
}

// SearchIssuesAndPRsGraphQLHandler searches issues and PRs using GraphQL
type SearchIssuesAndPRsGraphQLHandler struct {
	provider *GitHubProvider
}

// NewSearchIssuesAndPRsGraphQLHandler creates a new search handler
func NewSearchIssuesAndPRsGraphQLHandler(p *GitHubProvider) *SearchIssuesAndPRsGraphQLHandler {
	return &SearchIssuesAndPRsGraphQLHandler{provider: p}
}

// Execute runs the GraphQL search query
func (h *SearchIssuesAndPRsGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	query := extractString(params, "query")
	if query == "" {
		return ErrorResult("query is required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// Search query structure
	var searchQuery struct {
		Search struct {
			IssueCount int
			PageInfo   struct {
				HasNextPage bool
				EndCursor   githubv4.String
			}
			Nodes []struct {
				Typename string `graphql:"__typename"`
				Issue    struct {
					IssueFragment
				} `graphql:"... on Issue"`
				PullRequest struct {
					PullRequestFragment
				} `graphql:"... on PullRequest"`
			}
		} `graphql:"search(query: $query, type: ISSUE, first: $first, after: $after)"`
	}

	variables := map[string]interface{}{
		"query": githubv4.String(query),
		"first": githubv4.Int(30),
	}

	if after := extractString(params, "after"); after != "" {
		variables["after"] = githubv4.String(after)
	} else {
		variables["after"] = (*githubv4.String)(nil)
	}

	err = client.Query(ctx, &searchQuery, variables)
	if err != nil {
		return ErrorResult("GraphQL search failed: %v", err), nil
	}

	// Process results
	results := make([]map[string]interface{}, 0, len(searchQuery.Search.Nodes))
	for _, node := range searchQuery.Search.Nodes {
		switch node.Typename {
		case "Issue":
			item := convertIssueFragment(node.Issue.IssueFragment)
			item["type"] = "issue"
			results = append(results, item)
		case "PullRequest":
			item := convertPullRequestFragment(node.PullRequest.PullRequestFragment)
			item["type"] = "pull_request"
			results = append(results, item)
		}
	}

	result := map[string]interface{}{
		"items":      results,
		"totalCount": searchQuery.Search.IssueCount,
		"pageInfo": map[string]interface{}{
			"hasNextPage": searchQuery.Search.PageInfo.HasNextPage,
			"endCursor":   string(searchQuery.Search.PageInfo.EndCursor),
		},
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *SearchIssuesAndPRsGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_issues_prs_graphql",
		Description: "Search issues and pull requests using GitHub's GraphQL search API",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "GitHub search query (e.g., 'repo:owner/name is:open label:bug')",
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination",
				},
			},
			"required": []string{"query"},
		},
	}
}

// GetRepositoryDetailsGraphQLHandler gets detailed repository info using GraphQL
type GetRepositoryDetailsGraphQLHandler struct {
	provider *GitHubProvider
}

// NewGetRepositoryDetailsGraphQLHandler creates a new handler
func NewGetRepositoryDetailsGraphQLHandler(p *GitHubProvider) *GetRepositoryDetailsGraphQLHandler {
	return &GetRepositoryDetailsGraphQLHandler{provider: p}
}

// Execute fetches repository details via GraphQL
func (h *GetRepositoryDetailsGraphQLHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	if owner == "" || repo == "" {
		return ErrorResult("owner and repo are required"), nil
	}

	client, err := h.provider.getGraphQLClient(ctx)
	if err != nil {
		return ErrorResult("Failed to get GraphQL client: %v", err), nil
	}

	// Repository query with extended information
	var query struct {
		Repository struct {
			RepositoryFragment
			Collaborators struct {
				TotalCount int
			} `graphql:"collaborators(affiliation: ALL)"`
			Releases struct {
				TotalCount int
			}
			Tags struct {
				TotalCount int
			} `graphql:"refs(refPrefix: \"refs/tags/\")"`
			Branches struct {
				TotalCount int
			} `graphql:"refs(refPrefix: \"refs/heads/\")"`
			PullRequests struct {
				TotalCount int
			} `graphql:"pullRequests(states: OPEN)"`
			DiskUsage       *int
			IsSecurityPolicyEnabled bool
			VulnerabilityAlerts     *struct {
				TotalCount int
			}
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}

	err = client.Query(ctx, &query, variables)
	if err != nil {
		return ErrorResult("GraphQL query failed: %v", err), nil
	}

	// Convert to response
	result := convertRepositoryFragment(query.Repository.RepositoryFragment)
	
	// Add extended information
	result["collaborators_count"] = query.Repository.Collaborators.TotalCount
	result["releases_count"] = query.Repository.Releases.TotalCount
	result["tags_count"] = query.Repository.Tags.TotalCount
	result["branches_count"] = query.Repository.Branches.TotalCount
	result["open_pull_requests_count"] = query.Repository.PullRequests.TotalCount
	
	if query.Repository.DiskUsage != nil {
		result["disk_usage_kb"] = *query.Repository.DiskUsage
	}
	
	result["security_policy_enabled"] = query.Repository.IsSecurityPolicyEnabled
	
	if query.Repository.VulnerabilityAlerts != nil {
		result["vulnerability_alerts_count"] = query.Repository.VulnerabilityAlerts.TotalCount
	}

	return SuccessResult(result), nil
}

// GetDefinition returns the tool definition
func (h *GetRepositoryDetailsGraphQLHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "repository_details_graphql",
		Description: "Get comprehensive repository details using GraphQL",
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
			"required": []string{"owner", "repo"},
		},
	}
}

// Helper functions to convert GraphQL fragments to maps

func convertIssueFragment(issue IssueFragment) map[string]interface{} {
	result := map[string]interface{}{
		"id":         issue.ID,
		"number":     issue.Number,
		"title":      issue.Title,
		"body":       issue.Body,
		"state":      issue.State,
		"created_at": issue.CreatedAt.Time,
		"updated_at": issue.UpdatedAt.Time,
		"author":     issue.Author.Login,
		"comments":   issue.Comments.TotalCount,
	}

	if issue.ClosedAt != nil {
		result["closed_at"] = issue.ClosedAt.Time
	}

	// Add labels
	labels := make([]map[string]interface{}, 0, len(issue.Labels.Nodes))
	for _, label := range issue.Labels.Nodes {
		labels = append(labels, map[string]interface{}{
			"name":  label.Name,
			"color": label.Color,
		})
	}
	result["labels"] = labels

	// Add assignees
	assignees := make([]string, 0, len(issue.Assignees.Nodes))
	for _, assignee := range issue.Assignees.Nodes {
		assignees = append(assignees, assignee.Login)
	}
	result["assignees"] = assignees

	// Add milestone
	if issue.Milestone != nil {
		result["milestone"] = map[string]interface{}{
			"title": issue.Milestone.Title,
			"state": issue.Milestone.State,
		}
	}

	return result
}

func convertPullRequestFragment(pr PullRequestFragment) map[string]interface{} {
	result := map[string]interface{}{
		"id":            pr.ID,
		"number":        pr.Number,
		"title":         pr.Title,
		"body":          pr.Body,
		"state":         pr.State,
		"is_draft":      pr.IsDraft,
		"merged":        pr.Merged,
		"created_at":    pr.CreatedAt.Time,
		"updated_at":    pr.UpdatedAt.Time,
		"author":        pr.Author.Login,
		"head_ref":      pr.HeadRef.Name,
		"base_ref":      pr.BaseRef.Name,
		"head_sha":      pr.HeadRefOid,
		"base_sha":      pr.BaseRefOid,
		"comments":      pr.Comments.TotalCount,
		"commits":       pr.Commits.TotalCount,
		"changed_files": pr.ChangedFiles,
	}

	if pr.MergedAt != nil {
		result["merged_at"] = pr.MergedAt.Time
	}
	if pr.ClosedAt != nil {
		result["closed_at"] = pr.ClosedAt.Time
	}

	// Add labels
	labels := make([]map[string]interface{}, 0, len(pr.Labels.Nodes))
	for _, label := range pr.Labels.Nodes {
		labels = append(labels, map[string]interface{}{
			"name":  label.Name,
			"color": label.Color,
		})
	}
	result["labels"] = labels

	// Add reviews summary
	reviews := make([]map[string]interface{}, 0, len(pr.Reviews.Nodes))
	for _, review := range pr.Reviews.Nodes {
		reviews = append(reviews, map[string]interface{}{
			"state":  review.State,
			"author": review.Author.Login,
		})
	}
	result["reviews"] = reviews
	result["reviews_count"] = pr.Reviews.TotalCount

	return result
}

func convertRepositoryFragment(repo RepositoryFragment) map[string]interface{} {
	result := map[string]interface{}{
		"id":                repo.ID,
		"name":              repo.Name,
		"full_name":         repo.NameWithOwner,
		"is_private":        repo.IsPrivate,
		"is_archived":       repo.IsArchived,
		"is_fork":           repo.IsFork,
		"is_template":       repo.IsTemplate,
		"stargazers_count":  repo.StargazerCount,
		"forks_count":       repo.ForkCount,
		"watchers_count":    repo.WatcherCount,
		"open_issues_count": repo.OpenIssuesCount,
		"created_at":        repo.CreatedAt.Time,
		"updated_at":        repo.UpdatedAt.Time,
	}

	if repo.Description != nil {
		result["description"] = *repo.Description
	}

	if repo.DefaultBranchRef != nil {
		result["default_branch"] = repo.DefaultBranchRef.Name
	}

	if repo.PrimaryLanguage != nil {
		result["primary_language"] = map[string]interface{}{
			"name":  repo.PrimaryLanguage.Name,
			"color": repo.PrimaryLanguage.Color,
		}
	}

	// Add all languages
	languages := make([]map[string]interface{}, 0, len(repo.Languages.Nodes))
	for _, lang := range repo.Languages.Nodes {
		languages = append(languages, map[string]interface{}{
			"name":  lang.Name,
			"color": lang.Color,
		})
	}
	result["languages"] = languages

	if repo.PushedAt != nil {
		result["pushed_at"] = repo.PushedAt.Time
	}

	if repo.LicenseInfo != nil {
		license := map[string]interface{}{
			"name":    repo.LicenseInfo.Name,
			"spdx_id": repo.LicenseInfo.SpdxId,
		}
		if repo.LicenseInfo.Nickname != nil {
			license["nickname"] = *repo.LicenseInfo.Nickname
		}
		result["license"] = license
	}

	return result
}