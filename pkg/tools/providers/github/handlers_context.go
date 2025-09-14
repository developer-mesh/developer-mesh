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
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get teams for the authenticated user across all organizations or from a specific organization.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"org": map[string]interface{}{
					"type":        "string",
					"description": "Organization name to filter teams. Leave empty to get teams from all organizations.",
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
					"id":           1,
					"name":         "Justice League",
					"slug":         "justice-league",
					"description":  "A great team.",
					"privacy":      "closed",
					"permission":   "admin",
					"organization": "github",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Organization not found",
				"solution": "Verify the organization name if specified",
			},
			{
				"error":    "403 Forbidden",
				"cause":    "Insufficient permissions",
				"solution": "Ensure you have read:org scope",
			},
		},
		ExtendedHelp: `Lists teams that the authenticated user belongs to.

This endpoint uses GraphQL for better performance when available.

Team permissions:
- admin: Can add/remove members and change team settings
- push: Can push to team repositories
- pull: Can pull from team repositories

Use cases:
- List all your teams across organizations
- Filter teams by organization
- Check team membership and permissions`,
	}
}

func (h *GetTeamsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Try GraphQL client first for better performance
	if gqlClient, ok := GetGitHubV4ClientFromContext(ctx); ok {
		return h.executeGraphQL(ctx, gqlClient, params)
	}

	// Fallback to REST API
	client, ok := GetGitHubClientFromContext(ctx)
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
