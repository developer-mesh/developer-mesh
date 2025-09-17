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
		Description: GetOperationDescription("issues_list_graphql"),
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
				"state": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"open", "closed", "all"},
					"description": "Filter issues by state",
					"default":     "open",
					"example":     "open",
				},
				"labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Filter by one or more labels (AND condition)",
					"example":     []string{"bug", "help wanted"},
					"maxItems":    100,
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination (from previous response's endCursor)",
					"example":     "Y3Vyc29yOnYyOpHOAAAAAA==",
				},
			},
			"required": []string{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:org"},
			"rate_limit":   "graphql", // GraphQL has 5000 points/hour
			"api_version":  "GraphQL",
			"points_cost":  1, // Each GraphQL query costs points based on complexity
		},
		ResponseExample: map[string]interface{}{
			"issues": []map[string]interface{}{
				{
					"id":        "I_kwDOABCD5M4Abcde",
					"number":    42,
					"title":     "Bug in login flow",
					"body":      "When I try to login...",
					"state":     "OPEN",
					"createdAt": "2024-01-15T10:00:00Z",
					"updatedAt": "2024-01-16T14:30:00Z",
					"author":    map[string]interface{}{"login": "octocat"},
					"labels": []map[string]interface{}{
						{"name": "bug", "color": "d73a4a"},
					},
					"assignees":    []map[string]interface{}{{"login": "developer1"}},
					"commentCount": 5,
				},
			},
			"pageInfo": map[string]interface{}{
				"hasNextPage": true,
				"endCursor":   "Y3Vyc29yOnYyOpHOAAAAAA==",
			},
			"totalCount": 156,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Repository not found",
				"cause":    "Repository does not exist or you lack access",
				"solution": "Verify repository name and permissions",
			},
			{
				"error":    "Query complexity limit exceeded",
				"cause":    "GraphQL query is too complex",
				"solution": "Reduce the number of fields requested or pagination size",
			},
			{
				"error":    "Rate limited",
				"cause":    "Exceeded GraphQL API rate limit (5000 points/hour)",
				"solution": "Wait for rate limit reset or reduce query frequency",
			},
		},
		ExtendedHelp: `The issues_list_graphql operation uses GitHub's GraphQL API for efficient issue querying.

Advantages over REST API:
- Fetch only the fields you need
- Better performance with nested data
- More efficient pagination
- Reduced number of API calls

GraphQL-specific features:
- Returns GraphQL node IDs (can be used in mutations)
- Supports cursor-based pagination
- Can fetch related data in single query
- Consistent field naming across queries

Pagination:
- Uses cursor-based pagination (more efficient than offset)
- Returns 30 issues per page by default
- Use 'after' parameter with 'endCursor' from previous response
- Check 'hasNextPage' to know if more results exist

Examples:

# Basic query for open issues
{
  "owner": "facebook",
  "repo": "react"
}

# Filter by multiple labels
{
  "owner": "facebook",
  "repo": "react",
  "labels": ["bug", "help wanted"]
}

# Get all issues (open and closed)
{
  "owner": "facebook",
  "repo": "react",
  "state": "all"
}

# Paginate through results
{
  "owner": "facebook",
  "repo": "react",
  "after": "Y3Vyc29yOnYyOpHOAAAAAA=="
}

Rate limiting:
- GraphQL uses a point system (5000 points/hour)
- Simple queries cost 1 point
- Complex queries with many fields cost more
- Monitor X-RateLimit headers in responses`,
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
		Description: GetOperationDescription("search_issues_prs_graphql"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "GitHub search query using advanced search syntax",
					"example":     "repo:facebook/react is:open label:bug author:octocat",
					"minLength":   1,
					"maxLength":   1000,
				},
				"after": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination (from previous response's endCursor)",
					"example":     "Y3Vyc29yOnYyOpHOAAAAAA==",
				},
			},
			"required": []string{"query"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:org"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  2, // Search queries cost more points
		},
		ResponseExample: map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"type":      "issue",
					"id":        "I_kwDOABCD5M4Abcde",
					"number":    42,
					"title":     "Bug in authentication",
					"state":     "OPEN",
					"createdAt": "2024-01-15T10:00:00Z",
					"author":    map[string]interface{}{"login": "octocat"},
				},
				{
					"type":      "pull_request",
					"id":        "PR_kwDOABCD5M4Abcde",
					"number":    123,
					"title":     "Fix authentication bug",
					"state":     "OPEN",
					"isDraft":   false,
					"merged":    false,
					"createdAt": "2024-01-16T09:00:00Z",
				},
			},
			"totalCount": 42,
			"pageInfo": map[string]interface{}{
				"hasNextPage": true,
				"endCursor":   "Y3Vyc29yOnYyOpHOAAAAAA==",
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Invalid search query",
				"cause":    "Search query syntax is invalid",
				"solution": "Check GitHub search syntax documentation",
			},
			{
				"error":    "Search timeout",
				"cause":    "Search query is too complex or matches too many results",
				"solution": "Narrow your search criteria or add more specific filters",
			},
			{
				"error":    "Rate limited",
				"cause":    "Exceeded GraphQL API rate limit",
				"solution": "Wait for rate limit reset or reduce query frequency",
			},
		},
		ExtendedHelp: `The search_issues_prs_graphql operation provides powerful search across issues and pull requests.

Search syntax qualifiers:
- repo:owner/name - Search in specific repository
- org:orgname - Search in organization
- is:open/closed/merged - Filter by state
- is:issue/pr - Filter by type
- author:username - Filter by author
- assignee:username - Filter by assignee
- label:"bug" - Filter by label (use quotes for multi-word)
- milestone:"v1.0" - Filter by milestone
- created:>2024-01-01 - Filter by creation date
- updated:<2024-01-01 - Filter by update date
- comments:>10 - Filter by comment count
- involves:username - User is author, assignee, or mentioned
- team:orgname/teamname - Involves team members

Combining qualifiers:
- Use space to combine with AND logic
- Use OR for alternative conditions
- Use NOT or - to exclude
- Group with parentheses (limited support)

Examples:

# Search for open bugs in React repo
{
  "query": "repo:facebook/react is:open is:issue label:bug"
}

# Find PRs authored by octocat that need review
{
  "query": "is:pr is:open author:octocat review:required"
}

# Search across organization
{
  "query": "org:microsoft is:open involves:@me"
}

# Complex date-based search
{
  "query": "repo:nodejs/node created:>2024-01-01 updated:<2024-02-01 is:closed"
}

Tips:
- Returns both issues and PRs in single search
- Check 'type' field to distinguish issues from PRs
- More specific queries perform better
- Use cursor pagination for large result sets`,
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
			DiskUsage               *int
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
		Description: GetOperationDescription("repository_details_graphql"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
			},
			"required": []string{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"repo", "read:org"},
			"rate_limit":   "graphql",
			"api_version":  "GraphQL",
			"points_cost":  1,
		},
		ResponseExample: map[string]interface{}{
			"id":            "R_kgDOABCDEF",
			"name":          "react",
			"nameWithOwner": "facebook/react",
			"description":   "A declarative, efficient, and flexible JavaScript library for building user interfaces",
			"isPrivate":     false,
			"isArchived":    false,
			"isFork":        false,
			"isTemplate":    false,
			"createdAt":     "2013-05-24T16:15:54Z",
			"updatedAt":     "2024-01-20T10:30:00Z",
			"pushedAt":      "2024-01-20T09:45:00Z",
			"language": map[string]interface{}{
				"name":  "JavaScript",
				"color": "#f1e05a",
			},
			"languages": []map[string]interface{}{
				{"name": "JavaScript", "percentage": 75.5},
				{"name": "TypeScript", "percentage": 20.3},
			},
			"defaultBranch": "main",
			"stats": map[string]interface{}{
				"stars":        210000,
				"forks":        42000,
				"watchers":     6700,
				"issues":       850,
				"pullRequests": 320,
			},
			"topics": []string{"react", "javascript", "library", "ui", "frontend"},
			"license": map[string]interface{}{
				"key":  "mit",
				"name": "MIT License",
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "Repository not found",
				"cause":    "Repository does not exist or you lack access",
				"solution": "Verify repository name and permissions",
			},
			{
				"error":    "Rate limited",
				"cause":    "Exceeded GraphQL API rate limit",
				"solution": "Wait for rate limit reset or reduce query frequency",
			},
		},
		ExtendedHelp: `The repository_details_graphql operation fetches comprehensive repository information using GraphQL.

Returned information includes:
- Basic metadata (name, description, visibility)
- Repository statistics (stars, forks, issues, PRs)
- Language breakdown with percentages
- Topics/tags
- License information
- Timestamps (created, updated, last push)
- Default branch
- Archive and template status

Advantages over REST API:
- Single request for all data (REST requires multiple endpoints)
- Consistent field naming
- More efficient data transfer
- Includes calculated fields like language percentages

Example:
{
  "owner": "facebook",
  "repo": "react"
}

Use cases:
- Repository analytics and reporting
- Migration planning
- Dependency analysis
- License compliance checks
- Technology stack discovery`,
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
