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
	UsageExamples   []UsageExample         `json:"usageExamples,omitempty"`
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

// UsageExample provides concrete examples of how to use the tool
type UsageExample struct {
	Name           string                 `json:"name"`                     // Short name for the example (e.g., "simple", "complex", "error_case")
	Description    string                 `json:"description"`              // What this example demonstrates
	Input          map[string]interface{} `json:"input"`                    // Example input parameters
	ExpectedOutput interface{}            `json:"expectedOutput,omitempty"` // Expected successful output (simplified)
	ExpectedError  *CommonError           `json:"expectedError,omitempty"`  // Expected error for error case examples
	Notes          string                 `json:"notes,omitempty"`          // Additional context or tips
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
			UsageExamples: []UsageExample{
				{
					Name:        "simple",
					Description: "Get basic issue information for a public repository",
					Input: map[string]interface{}{
						"owner":        "facebook",
						"repo":         "react",
						"issue_number": 1234,
					},
					ExpectedOutput: map[string]interface{}{
						"number": 1234,
						"title":  "Bug: Something is broken",
						"state":  "open",
						"user": map[string]interface{}{
							"login": "octocat",
						},
					},
					Notes: "This is the simplest use case - fetching a single issue from a public repository",
				},
				{
					Name:        "complex",
					Description: "Get issue with full details including labels, assignees, and milestone",
					Input: map[string]interface{}{
						"owner":        "microsoft",
						"repo":         "vscode",
						"issue_number": 98765,
					},
					ExpectedOutput: map[string]interface{}{
						"number": 98765,
						"title":  "Feature: Add support for new language",
						"state":  "open",
						"labels": []interface{}{
							map[string]interface{}{"name": "enhancement"},
							map[string]interface{}{"name": "help wanted"},
						},
						"assignees": []interface{}{
							map[string]interface{}{"login": "developer1"},
						},
						"milestone": map[string]interface{}{
							"title": "v1.75.0",
						},
					},
					Notes: "Complex issues often have multiple labels, assignees, and are part of milestones",
				},
				{
					Name:        "error_case",
					Description: "Attempting to access a non-existent issue",
					Input: map[string]interface{}{
						"owner":        "github",
						"repo":         "hub",
						"issue_number": 999999999,
					},
					ExpectedError: &CommonError{
						Code:     404,
						Reason:   "Issue not found or you lack access to the repository",
						Solution: "Verify the issue exists and you have read permission to the repository",
					},
					Notes: "Always handle 404 errors gracefully - the issue might be deleted or in a private repo",
				},
			},
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
			UsageExamples: []UsageExample{
				{
					Name:        "simple",
					Description: "List all open issues in a repository",
					Input: map[string]interface{}{
						"owner": "golang",
						"repo":  "go",
					},
					ExpectedOutput: []interface{}{
						map[string]interface{}{
							"number": 58901,
							"title":  "proposal: add generic constraints",
							"state":  "open",
						},
						map[string]interface{}{
							"number": 58902,
							"title":  "cmd/go: improve error messages",
							"state":  "open",
						},
					},
					Notes: "By default, returns only open issues sorted by creation date (newest first)",
				},
				{
					Name:        "complex",
					Description: "List issues with multiple filters and custom pagination",
					Input: map[string]interface{}{
						"owner":     "kubernetes",
						"repo":      "kubernetes",
						"state":     "open",
						"labels":    "bug,priority/P0",
						"assignee":  "johndoe",
						"sort":      "updated",
						"direction": "desc",
						"per_page":  50,
						"page":      2,
					},
					ExpectedOutput: []interface{}{
						map[string]interface{}{
							"number": 111234,
							"title":  "Critical: Pod scheduling failure",
							"labels": []interface{}{
								map[string]interface{}{"name": "bug"},
								map[string]interface{}{"name": "priority/P0"},
							},
							"assignees": []interface{}{
								map[string]interface{}{"login": "johndoe"},
							},
						},
					},
					Notes: "Complex queries can filter by multiple criteria and control pagination for large result sets",
				},
				{
					Name:        "error_case",
					Description: "Invalid filter parameter value",
					Input: map[string]interface{}{
						"owner": "torvalds",
						"repo":  "linux",
						"state": "invalid_state",
					},
					ExpectedError: &CommonError{
						Code:     422,
						Reason:   "Invalid parameter values (e.g., invalid state, sort, or direction)",
						Solution: "Check that all parameters use valid enum values",
					},
					Notes: "The 'state' parameter only accepts 'open', 'closed', or 'all' values",
				},
			},
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
			UsageExamples: []UsageExample{
				{
					Name:        "simple",
					Description: "Create a basic issue with just title and body",
					Input: map[string]interface{}{
						"owner": "myorg",
						"repo":  "myapp",
						"title": "Bug: Login button not working",
						"body":  "The login button on the homepage is not responding to clicks.\n\n**Steps to reproduce:**\n1. Go to homepage\n2. Click login button\n3. Nothing happens",
					},
					ExpectedOutput: map[string]interface{}{
						"number":   5678,
						"title":    "Bug: Login button not working",
						"state":    "open",
						"html_url": "https://github.com/myorg/myapp/issues/5678",
					},
					Notes: "Minimal required fields are owner, repo, and title. Body is optional but recommended.",
				},
				{
					Name:        "complex",
					Description: "Create issue with labels, assignees, and milestone",
					Input: map[string]interface{}{
						"owner":     "kubernetes",
						"repo":      "kubernetes",
						"title":     "Feature: Add support for custom resource validation",
						"body":      "## Description\nWe need to add validation for custom resources...\n\n## Acceptance Criteria\n- [ ] Schema validation\n- [ ] Webhook validation\n- [ ] Documentation",
						"labels":    []interface{}{"enhancement", "priority/P1", "sig/api-machinery"},
						"assignees": []interface{}{"developer1", "developer2"},
						"milestone": 42,
					},
					ExpectedOutput: map[string]interface{}{
						"number": 112233,
						"title":  "Feature: Add support for custom resource validation",
						"labels": []interface{}{
							map[string]interface{}{"name": "enhancement"},
							map[string]interface{}{"name": "priority/P1"},
							map[string]interface{}{"name": "sig/api-machinery"},
						},
						"assignees": []interface{}{
							map[string]interface{}{"login": "developer1"},
							map[string]interface{}{"login": "developer2"},
						},
						"milestone": map[string]interface{}{
							"number": 42,
							"title":  "v1.28",
						},
					},
					Notes: "Labels must exist in the repo, assignees must be collaborators, milestone must be valid",
				},
				{
					Name:        "error_case",
					Description: "Creating issue with invalid assignee",
					Input: map[string]interface{}{
						"owner":     "nodejs",
						"repo":      "node",
						"title":     "Test issue",
						"assignees": []interface{}{"nonexistent-user-12345"},
					},
					ExpectedError: &CommonError{
						Code:     422,
						Reason:   "Validation failed - invalid assignee, label, or milestone",
						Solution: "Ensure assignees are collaborators, labels exist, and milestone is valid",
					},
					Notes: "GitHub validates that assignees must have push access to the repository",
				},
			},
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
			UsageExamples: []UsageExample{
				{
					Name:        "simple",
					Description: "Get basic pull request information",
					Input: map[string]interface{}{
						"owner":       "golang",
						"repo":        "go",
						"pull_number": 45678,
					},
					ExpectedOutput: map[string]interface{}{
						"number":    45678,
						"title":     "cmd/go: add module graph pruning",
						"state":     "open",
						"draft":     false,
						"mergeable": true,
					},
					Notes: "Basic PR info includes state, draft status, and mergeability",
				},
				{
					Name:        "complex",
					Description: "Get PR with review status and CI checks information",
					Input: map[string]interface{}{
						"owner":       "kubernetes",
						"repo":        "kubernetes",
						"pull_number": 108234,
					},
					ExpectedOutput: map[string]interface{}{
						"number":          108234,
						"title":           "Add ephemeral containers support",
						"mergeable_state": "blocked",
						"reviews": []interface{}{
							map[string]interface{}{
								"user":  map[string]interface{}{"login": "reviewer1"},
								"state": "APPROVED",
							},
							map[string]interface{}{
								"user":  map[string]interface{}{"login": "reviewer2"},
								"state": "CHANGES_REQUESTED",
							},
						},
						"status_checks": []interface{}{
							map[string]interface{}{
								"context": "continuous-integration/jenkins",
								"state":   "success",
							},
						},
					},
					Notes: "Complex PRs include review decisions, CI status, and merge conflict info",
				},
				{
					Name:        "error_case",
					Description: "Attempting to access non-existent pull request",
					Input: map[string]interface{}{
						"owner":       "torvalds",
						"repo":        "linux",
						"pull_number": 999999999,
					},
					ExpectedError: &CommonError{
						Code:     404,
						Reason:   "Pull request not found or you lack access to the repository",
						Solution: "Verify the PR exists and you have read permission to the repository",
					},
					Notes: "PRs in the Linux kernel repo are managed differently - this would return 404",
				},
			},
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
