package github

import (
	"fmt"
	"regexp"
)

// EnhancedToolDefinition extends the basic ToolDefinition with AI-optimized metadata
type EnhancedToolDefinition struct {
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"displayName"`
	Description     string                 `json:"description"`
	ExtendedHelp    string                 `json:"extendedHelp,omitempty"`
	InputSchema     map[string]interface{} `json:"inputSchema"`
	Metadata        ToolMetadata           `json:"metadata,omitempty"`
	ResponseExample ResponseExample        `json:"responseExample,omitempty"`
	CommonErrors    []CommonError          `json:"commonErrors,omitempty"`
	SemanticTags    []string               `json:"semanticTags,omitempty"`
}

// ToolMetadata provides additional metadata for tools
type ToolMetadata struct {
	RequiredScopes    []string `json:"requiredScopes,omitempty"`
	MinimumScopes     []string `json:"minimumScopes,omitempty"`
	RateLimitCategory string   `json:"rateLimitCategory,omitempty"`
	RequestsPerHour   int      `json:"requestsPerHour,omitempty"`
	ComplexityLevel   string   `json:"complexityLevel,omitempty"` // simple, moderate, complex
}

// ResponseExample provides example responses for success and error cases
type ResponseExample struct {
	Success interface{} `json:"success,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// CommonError describes common error scenarios and solutions
type CommonError struct {
	Code     int    `json:"code"`
	Reason   string `json:"reason"`
	Solution string `json:"solution"`
}

// Enhanced parameter patterns and validation
var (
	// GitHub username/organization pattern
	GitHubUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

	// Repository name pattern
	RepoNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

	// SHA pattern (40 character hex)
	SHAPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

	// Short SHA pattern (7+ characters)
	ShortSHAPattern = regexp.MustCompile(`^[a-f0-9]{7,40}$`)
)

// GetEnhancedIssueToolDefinitions returns enhanced definitions for issue-related tools
func GetEnhancedIssueToolDefinitions() []EnhancedToolDefinition {
	return []EnhancedToolDefinition{
		{
			Name:         "get_issue",
			DisplayName:  "Get Issue",
			Description:  "Retrieve detailed information about a specific GitHub issue, including metadata, comments count, labels, and current state",
			ExtendedHelp: "This operation requires read access to the repository. Public repositories are accessible without authentication, but private repositories require appropriate permissions. See https://docs.github.com/en/rest/issues/issues#get-an-issue",
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
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "simple",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
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
				Error: map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/issues/issues#get-an-issue",
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     404,
					Reason:   "Issue not found or you lack access to the repository",
					Solution: "Verify the issue exists and you have read permission to the repository",
				},
				{
					Code:     403,
					Reason:   "Rate limit exceeded or insufficient permissions",
					Solution: "Wait for rate limit reset or check your authentication token permissions",
				},
				{
					Code:     401,
					Reason:   "Authentication required for private repository",
					Solution: "Provide a valid GitHub token with appropriate repository access",
				},
			},
			SemanticTags: []string{"github", "issues", "read", "fetch", "metadata"},
		},
		{
			Name:         "list_issues",
			DisplayName:  "List Issues",
			Description:  "Retrieve a paginated list of issues for a repository with comprehensive filtering options",
			ExtendedHelp: "Returns issues in descending order by creation date. Supports filtering by state, labels, assignee, and milestone. Use 'state=all' to get both open and closed issues.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
						"example":     "facebook",
						"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name (e.g., 'react', 'vscode')",
						"example":     "react",
						"pattern":     "^[a-zA-Z0-9._-]+$",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"description": "Issue state filter",
						"enum":        []interface{}{"open", "closed", "all"},
						"default":     "open",
						"example":     "open",
					},
					"labels": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated list of label names to filter by (e.g., 'bug,enhancement')",
						"example":     "bug,help wanted",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Username of assignee to filter by. Use 'none' for unassigned issues, '*' for any assignee",
						"example":     "octocat",
					},
					"creator": map[string]interface{}{
						"type":        "string",
						"description": "Username of issue creator to filter by",
						"example":     "octocat",
					},
					"mentioned": map[string]interface{}{
						"type":        "string",
						"description": "Username to filter issues where this user is mentioned",
						"example":     "octocat",
					},
					"milestone": map[string]interface{}{
						"type":        "string",
						"description": "Milestone title to filter by. Use 'none' for issues without milestone, '*' for any milestone",
						"example":     "v1.0.0",
					},
					"sort": map[string]interface{}{
						"type":        "string",
						"description": "Sort field for results",
						"enum":        []interface{}{"created", "updated", "comments"},
						"default":     "created",
						"example":     "updated",
					},
					"direction": map[string]interface{}{
						"type":        "string",
						"description": "Sort direction",
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
						"description": "Page number to retrieve (1-based)",
						"minimum":     1,
						"default":     1,
						"example":     2,
					},
				},
				"required": []interface{}{"owner", "repo"},
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "moderate",
			},
			ResponseExample: ResponseExample{
				Success: []interface{}{
					map[string]interface{}{
						"number": 1234,
						"title":  "Bug: Something is broken",
						"state":  "open",
						"labels": []interface{}{
							map[string]interface{}{
								"name": "bug",
							},
						},
					},
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     404,
					Reason:   "Repository not found or you lack access",
					Solution: "Verify repository exists and you have read permission",
				},
				{
					Code:     422,
					Reason:   "Invalid parameter values (e.g., invalid state, sort, or direction)",
					Solution: "Check that all parameters use valid enum values",
				},
			},
			SemanticTags: []string{"github", "issues", "list", "filter", "pagination"},
		},
		{
			Name:         "create_issue",
			DisplayName:  "Create Issue",
			Description:  "Create a new issue in a GitHub repository with title, body, labels, and assignees",
			ExtendedHelp: "Requires write access to the repository. Assignees must be collaborators on the repository. Labels must exist in the repository before they can be assigned.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner username or organization name",
						"example":     "octocat",
						"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
						"example":     "Hello-World",
						"pattern":     "^[a-zA-Z0-9._-]+$",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Issue title (required, max 256 characters)",
						"example":     "Found a bug in the login system",
						"minLength":   1,
						"maxLength":   256,
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Issue body content (Markdown supported)",
						"example":     "## Steps to reproduce\n1. Go to login page\n2. Enter credentials\n3. Error occurs",
					},
					"labels": map[string]interface{}{
						"type":        "array",
						"description": "Array of label names to assign to the issue",
						"items": map[string]interface{}{
							"type": "string",
						},
						"example": []interface{}{"bug", "priority-high"},
					},
					"assignees": map[string]interface{}{
						"type":        "array",
						"description": "Array of usernames to assign to the issue (must be collaborators)",
						"items": map[string]interface{}{
							"type":    "string",
							"pattern": "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
						},
						"example": []interface{}{"octocat"},
					},
					"milestone": map[string]interface{}{
						"type":        "integer",
						"description": "Milestone number to assign to the issue",
						"example":     1,
						"minimum":     1,
					},
				},
				"required": []interface{}{"owner", "repo", "title"},
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "moderate",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
					"id":       123456789,
					"number":   1234,
					"title":    "Found a bug in the login system",
					"state":    "open",
					"html_url": "https://github.com/octocat/Hello-World/issues/1234",
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     422,
					Reason:   "Validation failed - invalid assignee, label, or milestone",
					Solution: "Ensure assignees are collaborators, labels exist, and milestone is valid",
				},
				{
					Code:     403,
					Reason:   "Insufficient permissions to create issues",
					Solution: "Verify you have write access to the repository",
				},
			},
			SemanticTags: []string{"github", "issues", "create", "write", "collaboration"},
		},
		{
			Name:         "update_issue",
			DisplayName:  "Update Issue",
			Description:  "Update an existing GitHub issue's title, body, state, labels, assignees, or milestone",
			ExtendedHelp: "Requires write access to update issues. Only specified fields will be updated - omitted fields remain unchanged. State can be 'open' or 'closed'.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner username or organization name",
						"example":     "octocat",
						"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
						"example":     "Hello-World",
						"pattern":     "^[a-zA-Z0-9._-]+$",
					},
					"issue_number": map[string]interface{}{
						"type":        "integer",
						"description": "Issue number to update",
						"example":     1234,
						"minimum":     1,
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New issue title",
						"example":     "Updated: Bug in login system (fixed)",
						"maxLength":   256,
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "New issue body content",
						"example":     "## Update\nThis issue has been resolved in PR #1235",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"description": "Issue state",
						"enum":        []interface{}{"open", "closed"},
						"example":     "closed",
					},
					"labels": map[string]interface{}{
						"type":        "array",
						"description": "Array of label names (replaces all existing labels)",
						"items": map[string]interface{}{
							"type": "string",
						},
						"example": []interface{}{"bug", "fixed"},
					},
					"assignees": map[string]interface{}{
						"type":        "array",
						"description": "Array of usernames to assign (replaces all existing assignees)",
						"items": map[string]interface{}{
							"type":    "string",
							"pattern": "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
						},
						"example": []interface{}{"octocat"},
					},
					"milestone": map[string]interface{}{
						"type":        "integer",
						"description": "Milestone number (null to remove milestone)",
						"example":     1,
						"minimum":     1,
					},
				},
				"required": []interface{}{"owner", "repo", "issue_number"},
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "moderate",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
					"id":     123456789,
					"number": 1234,
					"title":  "Updated: Bug in login system (fixed)",
					"state":  "closed",
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     404,
					Reason:   "Issue not found or you lack access",
					Solution: "Verify the issue exists and you have write permission",
				},
				{
					Code:     422,
					Reason:   "Validation failed for labels, assignees, or milestone",
					Solution: "Ensure all labels exist, assignees are collaborators, and milestone is valid",
				},
			},
			SemanticTags: []string{"github", "issues", "update", "modify", "state-management"},
		},
		{
			Name:         "search_issues",
			DisplayName:  "Search Issues",
			Description:  "Search for issues across GitHub using advanced search syntax with filters for repository, state, labels, and more",
			ExtendedHelp: "Uses GitHub's powerful search syntax. Supports qualifiers like 'repo:owner/name', 'state:open', 'label:bug', 'author:username', etc. See https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests",
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
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "search",
				RequestsPerHour:   30,
				ComplexityLevel:   "complex",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
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
			CommonErrors: []CommonError{
				{
					Code:     422,
					Reason:   "Invalid search query syntax",
					Solution: "Check GitHub search syntax documentation and verify query format",
				},
				{
					Code:     403,
					Reason:   "Rate limit exceeded (search API has lower limits)",
					Solution: "Wait for rate limit reset - search API allows 30 requests per minute",
				},
			},
			SemanticTags: []string{"github", "issues", "search", "query", "discovery"},
		},
	}
}

// GetEnhancedPullRequestToolDefinitions returns enhanced definitions for PR-related tools
func GetEnhancedPullRequestToolDefinitions() []EnhancedToolDefinition {
	return []EnhancedToolDefinition{
		{
			Name:         "get_pull_request",
			DisplayName:  "Get Pull Request",
			Description:  "Retrieve detailed information about a specific GitHub pull request, including review status, checks, and merge state",
			ExtendedHelp: "Returns comprehensive PR data including diff stats, review decisions, status checks, and merge conflict information. Requires read access to the repository.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
						"example":     "facebook",
						"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name (e.g., 'react', 'vscode')",
						"example":     "react",
						"pattern":     "^[a-zA-Z0-9._-]+$",
					},
					"pull_number": map[string]interface{}{
						"type":        "integer",
						"description": "Pull request number (positive integer)",
						"example":     1234,
						"minimum":     1,
						"maximum":     999999999,
					},
				},
				"required": []interface{}{"owner", "repo", "pull_number"},
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "simple",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
					"id":              123456789,
					"number":          1234,
					"title":           "Fix memory leak in useEffect cleanup",
					"state":           "open",
					"draft":           false,
					"mergeable":       true,
					"mergeable_state": "clean",
					"base": map[string]interface{}{
						"ref": "main",
					},
					"head": map[string]interface{}{
						"ref": "fix/memory-leak",
					},
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     404,
					Reason:   "Pull request not found or you lack access to the repository",
					Solution: "Verify the PR exists and you have read permission to the repository",
				},
			},
			SemanticTags: []string{"github", "pull-requests", "read", "code-review"},
		},
		{
			Name:         "create_pull_request",
			DisplayName:  "Create Pull Request",
			Description:  "Create a new pull request to propose changes from one branch to another",
			ExtendedHelp: "Creates a PR between two branches. The head branch contains your changes, and the base branch is where you want your changes merged. Both branches must exist.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner username or organization name",
						"example":     "octocat",
						"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
						"example":     "Hello-World",
						"pattern":     "^[a-zA-Z0-9._-]+$",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Pull request title (required, max 256 characters)",
						"example":     "Fix memory leak in useEffect cleanup",
						"minLength":   1,
						"maxLength":   256,
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Pull request body content (Markdown supported)",
						"example":     "## Changes\n- Fixed memory leak by properly cleaning up subscriptions\n- Added unit tests\n\nFixes #1234",
					},
					"head": map[string]interface{}{
						"type":        "string",
						"description": "The branch containing your changes (can include fork owner like 'username:branch')",
						"example":     "fix/memory-leak",
					},
					"base": map[string]interface{}{
						"type":        "string",
						"description": "The branch you want your changes merged into",
						"example":     "main",
						"default":     "main",
					},
					"draft": map[string]interface{}{
						"type":        "boolean",
						"description": "Create as draft pull request (cannot be merged until marked ready)",
						"example":     false,
						"default":     false,
					},
					"maintainer_can_modify": map[string]interface{}{
						"type":        "boolean",
						"description": "Allow maintainers to modify the pull request",
						"example":     true,
						"default":     true,
					},
				},
				"required": []interface{}{"owner", "repo", "title", "head", "base"},
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "moderate",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
					"id":       123456789,
					"number":   1234,
					"title":    "Fix memory leak in useEffect cleanup",
					"state":    "open",
					"draft":    false,
					"html_url": "https://github.com/octocat/Hello-World/pull/1234",
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     422,
					Reason:   "Validation failed - branches don't exist or no commits between them",
					Solution: "Ensure both head and base branches exist and there are commits to compare",
				},
				{
					Code:     403,
					Reason:   "Insufficient permissions to create pull requests",
					Solution: "Verify you have write access to the repository",
				},
			},
			SemanticTags: []string{"github", "pull-requests", "create", "collaboration", "code-review"},
		},
		{
			Name:         "merge_pull_request",
			DisplayName:  "Merge Pull Request",
			Description:  "Merge a pull request using the specified merge method (merge, squash, or rebase)",
			ExtendedHelp: "Requires write access and the PR must be mergeable. Different merge methods: 'merge' creates merge commit, 'squash' combines all commits into one, 'rebase' applies commits individually.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{
						"type":        "string",
						"description": "Repository owner username or organization name",
						"example":     "octocat",
						"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					},
					"repo": map[string]interface{}{
						"type":        "string",
						"description": "Repository name",
						"example":     "Hello-World",
						"pattern":     "^[a-zA-Z0-9._-]+$",
					},
					"pull_number": map[string]interface{}{
						"type":        "integer",
						"description": "Pull request number to merge",
						"example":     1234,
						"minimum":     1,
					},
					"commit_title": map[string]interface{}{
						"type":        "string",
						"description": "Custom merge commit title (optional)",
						"example":     "Fix memory leak in useEffect cleanup (#1234)",
					},
					"commit_message": map[string]interface{}{
						"type":        "string",
						"description": "Custom merge commit message (optional)",
						"example":     "This PR fixes a memory leak issue that was causing performance problems.",
					},
					"merge_method": map[string]interface{}{
						"type":        "string",
						"description": "Merge method to use",
						"enum":        []interface{}{"merge", "squash", "rebase"},
						"default":     "merge",
						"example":     "squash",
					},
					"sha": map[string]interface{}{
						"type":        "string",
						"description": "SHA that pull request head must match (optional safety check)",
						"example":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
						"pattern":     "^[a-f0-9]{40}$",
					},
				},
				"required": []interface{}{"owner", "repo", "pull_number"},
			},
			Metadata: ToolMetadata{
				RequiredScopes:    []string{"repo"},
				MinimumScopes:     []string{"public_repo"},
				RateLimitCategory: "core",
				RequestsPerHour:   5000,
				ComplexityLevel:   "complex",
			},
			ResponseExample: ResponseExample{
				Success: map[string]interface{}{
					"sha":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					"merged":  true,
					"message": "Pull Request successfully merged",
				},
			},
			CommonErrors: []CommonError{
				{
					Code:     405,
					Reason:   "Pull request is not mergeable (conflicts, failed checks, or already merged)",
					Solution: "Resolve merge conflicts, wait for checks to pass, or verify PR isn't already merged",
				},
				{
					Code:     409,
					Reason:   "SHA mismatch - head commit has changed",
					Solution: "Refresh the PR data and use the current head SHA",
				},
				{
					Code:     403,
					Reason:   "Insufficient permissions to merge pull requests",
					Solution: "Verify you have write access and branch protection allows merging",
				},
			},
			SemanticTags: []string{"github", "pull-requests", "merge", "integration", "version-control"},
		},
	}
}

// GetEnhancedOperationDescription returns enhanced description for operations
// Note: The main GetOperationDescription is in descriptions.go
func GetEnhancedOperationDescription(operationName string) string {
	descriptions := map[string]string{
		"get_issue":           "Retrieve detailed information about a specific GitHub issue, including metadata, comments count, labels, and current state",
		"list_issues":         "Retrieve a paginated list of issues for a repository with comprehensive filtering options",
		"create_issue":        "Create a new issue in a GitHub repository with title, body, labels, and assignees",
		"update_issue":        "Update an existing GitHub issue's title, body, state, labels, assignees, or milestone",
		"search_issues":       "Search for issues across GitHub using advanced search syntax with filters for repository, state, labels, and more",
		"get_pull_request":    "Retrieve detailed information about a specific GitHub pull request, including review status, checks, and merge state",
		"create_pull_request": "Create a new pull request to propose changes from one branch to another",
		"merge_pull_request":  "Merge a pull request using the specified merge method (merge, squash, or rebase)",
		"list_pull_requests":  "Retrieve a paginated list of pull requests for a repository with filtering by state, head, base, and more",
	}
	return descriptions[operationName]
}

// ValidateParameters provides parameter validation for enhanced tools
func ValidateParameters(toolName string, params map[string]interface{}) error {
	switch toolName {
	case "get_issue", "update_issue":
		if owner, ok := params["owner"].(string); ok {
			if !GitHubUsernamePattern.MatchString(owner) {
				return fmt.Errorf("invalid owner format: %s", owner)
			}
		}
		if repo, ok := params["repo"].(string); ok {
			if !RepoNamePattern.MatchString(repo) {
				return fmt.Errorf("invalid repository name format: %s", repo)
			}
		}
	case "merge_pull_request":
		if sha, ok := params["sha"].(string); ok && sha != "" {
			if !SHAPattern.MatchString(sha) {
				return fmt.Errorf("invalid SHA format: %s (must be 40 character hex)", sha)
			}
		}
	}
	return nil
}
