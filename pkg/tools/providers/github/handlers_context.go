package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
	"github.com/shurcooL/githubv4"
)

// Context Handlers

// GetMeHandler handles getting the current authenticated user
type GetMeHandler struct {
	provider *GitHubProvider
}

func NewGetMeHandler(p *GitHubProvider) *GetMeHandler {
	return &GetMeHandler{provider: p}
}

func (h *GetMeHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_me",
		Description: "Get information about the authenticated user",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

func (h *GetMeHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get authenticated user: %v", err)), nil
	}

	data, _ := json.Marshal(user)
	return NewToolResult(string(data)), nil
}

// GetTeamsHandler handles getting teams for the authenticated user
type GetTeamsHandler struct {
	provider *GitHubProvider
}

func NewGetTeamsHandler(p *GitHubProvider) *GetTeamsHandler {
	return &GetTeamsHandler{provider: p}
}

func (h *GetTeamsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_teams",
		Description: "Get teams for the authenticated user",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name (optional, returns all teams if not specified)",
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

func (h *GetTeamsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Try GraphQL client first for better performance
	if gqlClient, ok := ctx.Value("githubv4_client").(*githubv4.Client); ok {
		return h.executeGraphQL(ctx, gqlClient, params)
	}

	// Fallback to REST API
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	opts := &github.ListOptions{}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	var teams []*github.Team
	var err error

	if org, ok := params["org"].(string); ok && org != "" {
		// Get teams for specific organization
		teams, _, err = client.Teams.ListTeams(ctx, org, opts)
	} else {
		// Get all teams for authenticated user
		teams, _, err = client.Teams.ListUserTeams(ctx, opts)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get teams: %v", err)), nil
	}

	data, _ := json.Marshal(teams)
	return NewToolResult(string(data)), nil
}

func (h *GetTeamsHandler) executeGraphQL(ctx context.Context, client *githubv4.Client, params map[string]interface{}) (*ToolResult, error) {
	// GraphQL query for user teams
	var query struct {
		Viewer struct {
			Organizations struct {
				Nodes []struct {
					Name  string
					Teams struct {
						Nodes []struct {
							Name        string
							Description string
							Slug        string
							Privacy     string
							Members     struct {
								TotalCount int
							}
						}
					} `graphql:"teams(first: $first)"`
				}
			} `graphql:"organizations(first: 100)"`
		}
	}

	variables := map[string]interface{}{
		"first": githubv4.Int(100),
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get teams via GraphQL: %v", err)), nil
	}

	// If org is specified, filter results
	if org, ok := params["org"].(string); ok && org != "" {
		for _, orgNode := range query.Viewer.Organizations.Nodes {
			if orgNode.Name == org {
				data, _ := json.Marshal(orgNode.Teams.Nodes)
				return NewToolResult(string(data)), nil
			}
		}
		return NewToolResult("[]"), nil
	}

	// Return all teams from all organizations
	var allTeams []interface{}
	for _, orgNode := range query.Viewer.Organizations.Nodes {
		for _, team := range orgNode.Teams.Nodes {
			teamData := map[string]interface{}{
				"organization": orgNode.Name,
				"name":         team.Name,
				"description":  team.Description,
				"slug":         team.Slug,
				"privacy":      team.Privacy,
				"members":      team.Members.TotalCount,
			}
			allTeams = append(allTeams, teamData)
		}
	}

	data, _ := json.Marshal(allTeams)
	return NewToolResult(string(data)), nil
}
