package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// Search Handlers

// SearchIssuesHandler handles searching for issues using JQL
type SearchIssuesHandler struct {
	provider *JiraProvider
}

func NewSearchIssuesHandler(p *JiraProvider) *SearchIssuesHandler {
	return &SearchIssuesHandler{provider: p}
}

func (h *SearchIssuesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_issues",
		Description: "Search for Jira issues using JQL (Jira Query Language)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"jql": map[string]interface{}{
					"type":        "string",
					"description": "JQL query string (e.g., 'project = PROJ AND status = Open')",
				},
				"startAt": map[string]interface{}{
					"type":        "integer",
					"description": "Starting index for pagination",
					"default":     0,
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return",
					"default":     50,
					"maximum":     100,
				},
				"fields": map[string]interface{}{
					"type":        "array",
					"description": "List of fields to return",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"expand": map[string]interface{}{
					"type":        "string",
					"description": "Fields to expand",
				},
				"validateQuery": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to validate the JQL query",
					"default":     true,
				},
			},
			"required": []interface{}{},
		},
	}
}

func (h *SearchIssuesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Build query parameters
	queryParams := url.Values{}

	// Add JQL query
	if jql, ok := params["jql"].(string); ok && jql != "" {
		queryParams.Set("jql", jql)
	} else {
		// Default to all issues in the project if no JQL provided
		queryParams.Set("jql", "ORDER BY created DESC")
	}

	// Add pagination parameters
	if startAt, ok := params["startAt"].(float64); ok {
		queryParams.Set("startAt", fmt.Sprintf("%d", int(startAt)))
	}

	if maxResults, ok := params["maxResults"].(float64); ok {
		queryParams.Set("maxResults", fmt.Sprintf("%d", int(maxResults)))
	} else {
		queryParams.Set("maxResults", "50")
	}

	// Add fields parameter
	if fields, ok := params["fields"].([]interface{}); ok && len(fields) > 0 {
		fieldList := ""
		for i, f := range fields {
			if s, ok := f.(string); ok {
				if i > 0 {
					fieldList += ","
				}
				fieldList += s
			}
		}
		if fieldList != "" {
			queryParams.Set("fields", fieldList)
		}
	}

	// Add expand parameter
	if expand, ok := params["expand"].(string); ok {
		queryParams.Set("expand", expand)
	}

	// Add validateQuery parameter
	if validateQuery, ok := params["validateQuery"].(bool); ok {
		queryParams.Set("validateQuery", fmt.Sprintf("%t", validateQuery))
	}

	// Build request URL
	searchURL := h.provider.buildURL("/rest/api/3/search")
	if len(queryParams) > 0 {
		searchURL += "?" + queryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to search issues: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Search failed with status %d", resp.StatusCode)), nil
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Apply project filter if configured
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Metadata != nil {
		if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" {
			// Filter results to only include issues from allowed projects
			if issues, ok := result["issues"].([]interface{}); ok {
				filtered := []interface{}{}
				for _, issue := range issues {
					if issueMap, ok := issue.(map[string]interface{}); ok {
						if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
							if project, ok := fields["project"].(map[string]interface{}); ok {
								if key, ok := project["key"].(string); ok {
									// Check if project is in filter
									if contains(projectFilter, key) {
										filtered = append(filtered, issue)
									}
								}
							}
						}
					}
				}
				result["issues"] = filtered
				result["total"] = len(filtered)
			}
		}
	}

	return NewToolResult(result), nil
}

// Helper function to check if project is in filter
func contains(filter, project string) bool {
	// Simple contains check - could be enhanced to support comma-separated lists
	return filter == "" || filter == project || filter == "*"
}