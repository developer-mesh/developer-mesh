package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

	// Get JQL query
	jql := ""
	if jqlParam, ok := params["jql"].(string); ok && jqlParam != "" {
		jql = jqlParam
	} else {
		// Default to all issues ordered by creation date
		jql = "ORDER BY created DESC"
	}

	// Validate JQL for potential injection
	if err := h.validateJQL(jql); err != nil {
		return NewToolError(fmt.Sprintf("Invalid JQL query: %v", err)), nil
	}

	// Apply project filter to JQL if configured
	jql = h.applyProjectFilterToJQL(ctx, jql)
	queryParams.Set("jql", jql)

	// Add pagination parameters
	startAt := 0
	if start, ok := params["startAt"].(float64); ok && start >= 0 {
		startAt = int(start)
	}
	queryParams.Set("startAt", fmt.Sprintf("%d", startAt))

	maxResults := 50
	if max, ok := params["maxResults"].(float64); ok && max > 0 {
		if max > 100 {
			maxResults = 100 // Enforce maximum
		} else {
			maxResults = int(max)
		}
	}
	queryParams.Set("maxResults", fmt.Sprintf("%d", maxResults))

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
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Search failed with status %d", resp.StatusCode)
		if errorMessages, ok := result["errorMessages"].([]interface{}); ok && len(errorMessages) > 0 {
			errorMsg = fmt.Sprintf("%s: %v", errorMsg, errorMessages[0])
		}
		if errors, ok := result["errors"].(map[string]interface{}); ok && len(errors) > 0 {
			errorDetails := []string{}
			for field, msg := range errors {
				errorDetails = append(errorDetails, fmt.Sprintf("%s: %v", field, msg))
			}
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, strings.Join(errorDetails, ", "))
		}
		return NewToolError(errorMsg), nil
	}

	// Apply additional project filtering if configured (in case JQL filter wasn't applied)
	h.filterSearchResults(ctx, &result)

	// Add metadata (preserve any existing metadata from filtering)
	var metadata map[string]interface{}
	if existingMetadata, exists := result["_metadata"].(map[string]interface{}); exists {
		metadata = existingMetadata
	} else {
		metadata = make(map[string]interface{})
	}

	metadata["api_version"] = "v3"
	metadata["operation"] = "search_issues"
	metadata["jql"] = jql
	metadata["startAt"] = startAt
	metadata["maxResults"] = maxResults

	result["_metadata"] = metadata

	// Add pagination info
	if total, ok := result["total"].(float64); ok {
		nextStart := startAt + maxResults
		if float64(nextStart) < total {
			result["_metadata"].(map[string]interface{})["nextStartAt"] = nextStart
			result["_metadata"].(map[string]interface{})["hasMore"] = true
		} else {
			result["_metadata"].(map[string]interface{})["hasMore"] = false
		}
	}

	return NewToolResult(result), nil
}

// validateJQL validates the JQL query for potential security issues
func (h *SearchIssuesHandler) validateJQL(jql string) error {
	if jql == "" {
		return nil
	}

	// Check for potential SQL injection patterns
	dangerousPatterns := []string{
		"';", "'; ", "';--", "' OR ", "' AND ",
		"/*", "*/", "xp_", "sp_", "exec ",
		"<script", "</script>", "javascript:",
		"DROP ", "DELETE ", "TRUNCATE ", "ALTER ", "CREATE ",
	}

	jqlUpper := strings.ToUpper(jql)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(jqlUpper, strings.ToUpper(pattern)) {
			return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
		}
	}

	// Check for reasonable length
	if len(jql) > 5000 {
		return fmt.Errorf("JQL query too long (max 5000 characters)")
	}

	// Basic syntax validation - check for balanced parentheses
	openCount := strings.Count(jql, "(")
	closeCount := strings.Count(jql, ")")
	if openCount != closeCount {
		return fmt.Errorf("unbalanced parentheses in JQL query")
	}

	// Check for balanced quotes
	singleQuotes := strings.Count(jql, "'")
	if singleQuotes%2 != 0 {
		return fmt.Errorf("unbalanced single quotes in JQL query")
	}

	doubleQuotes := strings.Count(jql, "\"")
	if doubleQuotes%2 != 0 {
		return fmt.Errorf("unbalanced double quotes in JQL query")
	}

	return nil
}

// applyProjectFilterToJQL adds project filter to JQL if configured
func (h *SearchIssuesHandler) applyProjectFilterToJQL(ctx context.Context, jql string) string {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return jql
	}

	projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string)
	if !ok || projectFilter == "" || projectFilter == "*" {
		return jql
	}

	// Parse allowed projects
	allowedProjects := strings.Split(projectFilter, ",")
	for i := range allowedProjects {
		allowedProjects[i] = strings.TrimSpace(allowedProjects[i])
	}

	// Build project filter clause
	projectClause := ""
	if len(allowedProjects) == 1 {
		projectClause = fmt.Sprintf("project = \"%s\"", allowedProjects[0])
	} else {
		projectList := []string{}
		for _, project := range allowedProjects {
			projectList = append(projectList, fmt.Sprintf("\"%s\"", project))
		}
		projectClause = fmt.Sprintf("project in (%s)", strings.Join(projectList, ", "))
	}

	// Combine with existing JQL
	if jql == "" || strings.HasPrefix(strings.TrimSpace(strings.ToUpper(jql)), "ORDER BY") {
		// If no JQL or only ORDER BY, add project filter
		return projectClause + " " + jql
	} else {
		// Otherwise, add project filter as AND condition
		return fmt.Sprintf("(%s) AND (%s)", projectClause, jql)
	}
}

// filterSearchResults applies additional filtering to search results
func (h *SearchIssuesHandler) filterSearchResults(ctx context.Context, result *map[string]interface{}) {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return
	}

	projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string)
	if !ok || projectFilter == "" || projectFilter == "*" {
		return
	}

	// Parse allowed projects
	allowedProjects := strings.Split(projectFilter, ",")
	for i := range allowedProjects {
		allowedProjects[i] = strings.TrimSpace(allowedProjects[i])
	}

	// Filter results to only include issues from allowed projects
	if issues, ok := (*result)["issues"].([]interface{}); ok {
		filtered := []interface{}{}
		for _, issue := range issues {
			if issueMap, ok := issue.(map[string]interface{}); ok {
				if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
					if project, ok := fields["project"].(map[string]interface{}); ok {
						if key, ok := project["key"].(string); ok {
							if h.provider.isProjectAllowed(key, allowedProjects) {
								filtered = append(filtered, issue)
							}
						}
					}
				}
			}
		}
		(*result)["issues"] = filtered
		// Update total to reflect filtered count
		if _, hasTotal := (*result)["total"]; hasTotal {
			// Ensure _metadata exists and is the correct type
			if (*result)["_metadata"] == nil {
				(*result)["_metadata"] = make(map[string]interface{})
			}
			if metadata, ok := (*result)["_metadata"].(map[string]interface{}); ok {
				metadata["originalTotal"] = (*result)["total"]
			}
			(*result)["total"] = len(filtered)
		}
	}
}
