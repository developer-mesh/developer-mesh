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
		Description: "List organizations for a user",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Username to list organizations for (omit for authenticated user)",
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
		},
	}
}

func (h *ListOrganizationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Search for organizations on GitHub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort field (repositories, joined, followers)",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Sort order (asc or desc)",
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

func (h *SearchOrganizationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	query := extractString(params, "query")

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

	data, _ := json.Marshal(result)
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
		Description: "Search for users on GitHub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query using GitHub search syntax",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort field (followers, repositories, joined)",
				},
				"order": map[string]interface{}{
					"type":        "string",
					"description": "Sort order (asc or desc)",
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

func (h *SearchUsersHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	query := extractString(params, "query")

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

	data, _ := json.Marshal(result)
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
		Description: "Get members of a team",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name",
				},
				"team_slug": map[string]interface{}{
					"type":        "string",
					"description": "Team slug",
				},
				"role": map[string]interface{}{
					"type":        "string",
					"description": "Filter by role (member, maintainer, all)",
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
			"required": []interface{}{"org", "team_slug"},
		},
	}
}

func (h *GetTeamMembersHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "List teams in an organization",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name",
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
			"required": []interface{}{"org"},
		},
	}
}

func (h *ListTeamsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
		Description: "Get details about an organization",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name",
				},
			},
			"required": []interface{}{"org"},
		},
	}
}

func (h *GetOrganizationHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
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
