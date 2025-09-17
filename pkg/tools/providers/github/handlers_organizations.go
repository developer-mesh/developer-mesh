package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Organization and User Operations

// ListOrganizationsHandler handles listing organizations for a user
type ListOrganizationsHandler struct {
	provider *GitHubProvider
}

func NewListOrganizationsHandler(p *GitHubProvider) *ListOrganizationsHandler {
	return &ListOrganizationsHandler{provider: p}
}

func (h *ListOrganizationsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_organizations",
		Description: "List organizations for a user or the authenticated user. Returns organization details including name, description, and URLs.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Username to list organizations for. Leave empty to list organizations for the authenticated user.",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of organizations per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"read:org"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"organizations": []interface{}{
				map[string]interface{}{
					"login":       "github",
					"id":          9919,
					"url":         "https://api.github.com/orgs/github",
					"description": "How people build software",
					"avatar_url":  "https://avatars.githubusercontent.com/u/9919?v=4",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "User not found",
				"solution": "Verify the username exists on GitHub",
			},
		},
		ExtendedHelp: `Lists organizations that a user belongs to.

Use cases:
- Discover which organizations a user is member of
- List your own organizations
- Find organization details for further operations`,
	}
}

func (h *ListOrganizationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	pagination := ExtractPagination(params)
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	var orgs []*github.Organization
	var err error

	username := extractString(params, "username")
	if username != "" {
		orgs, _, err = client.Organizations.List(ctx, username, opts)
	} else {
		orgs, _, err = client.Organizations.List(ctx, "", opts)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list organizations: %v", err)), nil
	}

	data, _ := json.Marshal(orgs)
	return NewToolResult(string(data)), nil
}

// SearchOrganizationsHandler handles searching for organizations
type SearchOrganizationsHandler struct {
	provider *GitHubProvider
}

func NewSearchOrganizationsHandler(p *GitHubProvider) *SearchOrganizationsHandler {
	return &SearchOrganizationsHandler{provider: p}
}

func (h *SearchOrganizationsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_organizations",
		Description: "Search for organizations on GitHub using advanced search syntax. Returns matching organizations with metadata.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax. Examples: 'microsoft', 'location:seattle', 'repos:>100'",
					"example":     "microsoft type:org",
					"minLength":   1,
					"maxLength":   256,
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort field for results",
					"enum":        []interface{}{"repositories", "joined", "followers", ""},
					"example":     "repositories",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Sort order",
					"enum":        []interface{}{"asc", "desc"},
					"default":     "desc",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
			"required": []interface{}{"query"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{},
			"rateLimit": map[string]interface{}{
				"requests": 30,
				"period":   "minute",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"login":       "microsoft",
					"id":          6154722,
					"avatar_url":  "https://avatars.githubusercontent.com/u/6154722?v=4",
					"description": "Open source projects and samples from Microsoft",
					"type":        "Organization",
				},
			},
			"total_count": 1,
			"has_more":    false,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid search query syntax",
				"solution": "Check GitHub search syntax documentation",
			},
		},
		ExtendedHelp: `Search for organizations using GitHub's search syntax.

Search qualifiers:
- location:LOCATION - Organizations in a location
- repos:N - Organizations with N repositories
- followers:N - Organizations with N followers

Note: Rate limited to 30 requests per minute.`,
	}
}

func (h *SearchOrganizationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
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

	pagination := ExtractPagination(params)
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if sort := extractString(params, "sort"); sort != "" {
		opts.Sort = sort
	}
	if order := extractString(params, "order"); order != "" {
		opts.Order = order
	}

	// Search for organizations (uses users endpoint with type:org)
	searchQuery := fmt.Sprintf("%s type:org", query)
	result, _, err := client.Search.Users(ctx, searchQuery, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search organizations: %v", err)), nil
	}

	// Return items with essential metadata only
	response := map[string]interface{}{
		"items":       result.Users,
		"total_count": *result.Total,
		"has_more":    *result.Total > len(result.Users),
		"page":        opts.Page,
		"per_page":    opts.PerPage,
	}

	data, _ := json.Marshal(response)
	return NewToolResult(string(data)), nil
}

// SearchUsersHandler handles searching for users
type SearchUsersHandler struct {
	provider *GitHubProvider
}

func NewSearchUsersHandler(p *GitHubProvider) *SearchUsersHandler {
	return &SearchUsersHandler{provider: p}
}

func (h *SearchUsersHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_users",
		Description: "Search for users on GitHub using advanced search syntax. Find developers by location, language, followers, and more.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax. Examples: 'location:seattle', 'language:python', 'followers:>1000'",
					"example":     "location:seattle language:go",
					"minLength":   1,
					"maxLength":   256,
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort field for results",
					"enum":        []interface{}{"followers", "repositories", "joined", ""},
					"example":     "followers",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Sort order",
					"enum":        []interface{}{"asc", "desc"},
					"default":     "desc",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
			"required": []interface{}{"query"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{},
			"rateLimit": map[string]interface{}{
				"requests": 30,
				"period":   "minute",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"login":      "octocat",
					"id":         1,
					"avatar_url": "https://github.com/images/error/octocat_happy.gif",
					"type":       "User",
					"score":      1.0,
				},
			},
			"total_count": 1,
			"has_more":    false,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid search query syntax",
				"solution": "Check GitHub search syntax documentation",
			},
		},
		ExtendedHelp: `Search for GitHub users with advanced filters.

Search qualifiers:
- location:LOCATION - Users in a location
- language:LANG - Users who code in a language
- followers:N - Users with N followers
- repos:N - Users with N repositories
- type:user or type:org - Filter by account type

Note: Rate limited to 30 requests per minute.`,
	}
}

func (h *SearchUsersHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
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

	pagination := ExtractPagination(params)
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if sort := extractString(params, "sort"); sort != "" {
		opts.Sort = sort
	}
	if order := extractString(params, "order"); order != "" {
		opts.Order = order
	}

	result, _, err := client.Search.Users(ctx, query, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search users: %v", err)), nil
	}

	// Return items with essential metadata only
	response := map[string]interface{}{
		"items":       result.Users,
		"total_count": *result.Total,
		"has_more":    *result.Total > len(result.Users),
		"page":        opts.Page,
		"per_page":    opts.PerPage,
	}

	data, _ := json.Marshal(response)
	return NewToolResult(string(data)), nil
}

// GetTeamMembersHandler handles getting members of a team
type GetTeamMembersHandler struct {
	provider *GitHubProvider
}

func NewGetTeamMembersHandler(p *GitHubProvider) *GetTeamMembersHandler {
	return &GetTeamMembersHandler{provider: p}
}

func (h *GetTeamMembersHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_team_members",
		Description: "List members of a GitHub organization team. Shows team membership with role information.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name (e.g., 'github', 'microsoft')",
					"example":     "github",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"team_slug": map[string]interface{}{
					"type":        "string",
					"description": "Team slug (URL-friendly team name)",
					"example":     "engineering",
					"pattern":     "^[a-z0-9][a-z0-9-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
				"role": map[string]interface{}{
					"type":        "string",
					"description": "Filter members by role",
					"enum":        []interface{}{"member", "maintainer", "all"},
					"default":     "all",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
			"required": []interface{}{"org", "team_slug"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"read:org"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"members": []interface{}{
				map[string]interface{}{
					"login":      "octocat",
					"id":         1,
					"avatar_url": "https://github.com/images/error/octocat_happy.gif",
					"type":       "User",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Team not found or no access",
				"solution": "Verify team slug and organization name are correct",
			},
		},
		ExtendedHelp: `Lists members of an organization team.

Roles:
- member: Regular team member
- maintainer: Team maintainer with additional permissions
- all: Both members and maintainers (default)

Note: Requires organization membership to view teams.`,
	}
}

func (h *GetTeamMembersHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	org := extractString(params, "org")
	teamSlug := extractString(params, "team_slug")

	pagination := ExtractPagination(params)
	opts := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if role := extractString(params, "role"); role != "" {
		opts.Role = role
	}

	members, _, err := client.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get team members: %v", err)), nil
	}

	data, _ := json.Marshal(members)
	return NewToolResult(string(data)), nil
}

// ListTeamsHandler handles listing teams in an organization
type ListTeamsHandler struct {
	provider *GitHubProvider
}

func NewListTeamsHandler(p *GitHubProvider) *ListTeamsHandler {
	return &ListTeamsHandler{provider: p}
}

func (h *ListTeamsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_teams",
		Description: "List all teams in a GitHub organization. Returns team details including name, description, and permissions.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name (e.g., 'github', 'microsoft')",
					"example":     "github",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
			"required": []interface{}{"org"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"read:org"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"teams": []interface{}{
				map[string]interface{}{
					"id":          1,
					"name":        "Engineering",
					"slug":        "engineering",
					"description": "Engineering team",
					"privacy":     "closed",
					"permission":  "pull",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Organization not found or no access",
				"solution": "Verify organization name and access permissions",
			},
		},
		ExtendedHelp: `Lists all teams in an organization.

Team privacy levels:
- closed: Visible to all organization members
- secret: Only visible to organization owners and team members

Note: Requires organization membership to view teams.`,
	}
}

func (h *ListTeamsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	org := extractString(params, "org")

	pagination := ExtractPagination(params)
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	teams, _, err := client.Teams.ListTeams(ctx, org, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list teams: %v", err)), nil
	}

	data, _ := json.Marshal(teams)
	return NewToolResult(string(data)), nil
}

// GetOrganizationHandler handles getting an organization
type GetOrganizationHandler struct {
	provider *GitHubProvider
}

func NewGetOrganizationHandler(p *GitHubProvider) *GetOrganizationHandler {
	return &GetOrganizationHandler{provider: p}
}

func (h *GetOrganizationHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_organization",
		Description: "Get detailed information about a GitHub organization including description, location, blog, and repository counts.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name (e.g., 'github', 'microsoft')",
					"example":     "github",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
			},
			"required": []interface{}{"org"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
		},
		ResponseExample: map[string]interface{}{
			"login":                           "github",
			"id":                              9919,
			"description":                     "How people build software",
			"name":                            "GitHub",
			"company":                         "GitHub, Inc.",
			"blog":                            "https://github.blog",
			"location":                        "San Francisco, CA",
			"email":                           "support@github.com",
			"twitter_username":                "github",
			"is_verified":                     true,
			"has_organization_projects":       true,
			"has_repository_projects":         true,
			"public_repos":                    410,
			"public_gists":                    0,
			"followers":                       5000,
			"following":                       0,
			"html_url":                        "https://github.com/github",
			"created_at":                      "2008-05-11T04:37:31Z",
			"type":                            "Organization",
			"total_private_repos":             100,
			"owned_private_repos":             100,
			"private_gists":                   0,
			"disk_usage":                      10000,
			"collaborators":                   0,
			"billing_email":                   "billing@github.com",
			"default_repository_permission":   "read",
			"members_can_create_repositories": true,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Organization not found",
				"solution": "Verify the organization name is correct",
			},
		},
		ExtendedHelp: `Retrieves detailed information about an organization.

Public vs Authenticated:
- Public: Basic information visible to everyone
- Authenticated: Additional fields like billing email, private repos

Use cases:
- Get organization metadata
- Check organization settings
- Verify organization features
- Get repository counts`,
	}
}

func (h *GetOrganizationHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	org := extractString(params, "org")

	organization, _, err := client.Organizations.Get(ctx, org)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get organization: %v", err)), nil
	}

	data, _ := json.Marshal(organization)
	return NewToolResult(string(data)), nil
}
