package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Search Handlers

// SearchContentHandler handles searching for content using CQL via v1 API
// Note: v2 API does not support CQL, so v1 is required for search operations
type SearchContentHandler struct {
	provider *ConfluenceProvider
}

func NewSearchContentHandler(p *ConfluenceProvider) *SearchContentHandler {
	return &SearchContentHandler{provider: p}
}

func (h *SearchContentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_content",
		Description: "Search Confluence content using CQL (Confluence Query Language) via v1 API",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cql": map[string]interface{}{
					"type":        "string",
					"description": "CQL query string (e.g., 'space = DEV AND type = page', 'title ~ \"search term\"')",
				},
				"cqlcontext": map[string]interface{}{
					"type":        "string",
					"description": "The context for the CQL query (json string with spaceKey, contentId, etc.)",
				},
				"start": map[string]interface{}{
					"type":        "integer",
					"description": "Starting index for pagination (0-based)",
					"default":     0,
					"minimum":     0,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return per page",
					"default":     25,
					"minimum":     1,
					"maximum":     100,
				},
				"expand": map[string]interface{}{
					"type":        "array",
					"description": "Properties to expand in results (e.g., 'space', 'history', 'body.view', 'version')",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"excerpt": map[string]interface{}{
					"type":        "string",
					"description": "Type of excerpt to include (highlight, indexed, none)",
					"enum":        []interface{}{"highlight", "indexed", "none"},
					"default":     "highlight",
				},
				"includeArchivedSpaces": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include content from archived spaces",
					"default":     false,
				},
			},
			"required": []interface{}{"cql"},
		},
	}
}

func (h *SearchContentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Get CQL query
	cql, ok := params["cql"].(string)
	if !ok || cql == "" {
		return NewToolError("cql query is required"), nil
	}

	// Validate CQL query (basic validation)
	if err := h.validateCQL(cql); err != nil {
		return NewToolError(fmt.Sprintf("Invalid CQL query: %v", err)), nil
	}

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("cql", cql)

	// Add CQL context if provided
	if cqlContext, ok := params["cqlcontext"].(string); ok && cqlContext != "" {
		queryParams.Set("cqlcontext", cqlContext)
	}

	// Add pagination parameters with validation
	start := 0
	if s, ok := params["start"].(float64); ok {
		start = int(s)
		if start < 0 {
			start = 0
		}
	}
	queryParams.Set("start", fmt.Sprintf("%d", start))

	limit := 25
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
		if limit < 1 {
			limit = 1
		} else if limit > 100 {
			limit = 100
		}
	}
	queryParams.Set("limit", fmt.Sprintf("%d", limit))

	// Add expand parameter
	if expands, ok := params["expand"].([]interface{}); ok && len(expands) > 0 {
		expandList := make([]string, 0, len(expands))
		for _, e := range expands {
			if s, ok := e.(string); ok {
				expandList = append(expandList, s)
			}
		}
		if len(expandList) > 0 {
			queryParams.Set("expand", strings.Join(expandList, ","))
		}
	}

	// Add excerpt parameter
	if excerpt, ok := params["excerpt"].(string); ok && excerpt != "" {
		queryParams.Set("excerpt", excerpt)
	} else {
		queryParams.Set("excerpt", "highlight")
	}

	// Add includeArchivedSpaces parameter
	if includeArchived, ok := params["includeArchivedSpaces"].(bool); ok && includeArchived {
		queryParams.Set("includeArchivedSpaces", "true")
	}

	// Build request URL - using v1 API for CQL support
	searchURL := h.provider.buildV1URL("/content/search")
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
		return NewToolError(fmt.Sprintf("Failed to search content: %v", err)), nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Search failed with status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
			// Check for CQL syntax errors
			if strings.Contains(message, "CQL") || strings.Contains(message, "query") {
				errorMsg = fmt.Sprintf("CQL query error: %s", message)
			}
		}
		return NewToolError(errorMsg), nil
	}

	// Apply space filter if configured
	if filtered := h.provider.FilterSpaceResults(ctx, result); filtered != nil {
		if filteredMap, ok := filtered.(map[string]interface{}); ok {
			result = filteredMap
		}
	}

	// Add pagination metadata
	if _, hasSize := result["size"]; !hasSize {
		// Calculate size from results if not present
		if results, ok := result["results"].([]interface{}); ok {
			result["size"] = len(results)
		}
	}

	// Add pagination links for easier navigation
	if !h.hasLinks(result) {
		h.addPaginationLinks(result, start, limit)
	}

	// Add query metadata
	result["_query"] = map[string]interface{}{
		"cql":   cql,
		"start": start,
		"limit": limit,
	}

	return NewToolResult(result), nil
}

// validateCQL performs basic validation of CQL query
func (h *SearchContentHandler) validateCQL(cql string) error {
	// Check for empty query
	cql = strings.TrimSpace(cql)
	if cql == "" {
		return fmt.Errorf("CQL query cannot be empty")
	}

	// Check for basic SQL injection patterns (very basic)
	dangerousPatterns := []string{
		"--;",
		"/*",
		"*/",
		"xp_",
		"sp_",
	}

	lowerCQL := strings.ToLower(cql)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerCQL, pattern) {
			return fmt.Errorf("potentially dangerous pattern detected in CQL: %s", pattern)
		}
	}

	// Check for balanced quotes
	if strings.Count(cql, "'")%2 != 0 {
		return fmt.Errorf("unbalanced single quotes in CQL")
	}
	if strings.Count(cql, "\"")%2 != 0 {
		return fmt.Errorf("unbalanced double quotes in CQL")
	}

	// Check for balanced parentheses
	openParens := strings.Count(cql, "(")
	closeParens := strings.Count(cql, ")")
	if openParens != closeParens {
		return fmt.Errorf("unbalanced parentheses in CQL: %d open, %d close", openParens, closeParens)
	}

	return nil
}

// hasLinks checks if the result already has pagination links
func (h *SearchContentHandler) hasLinks(result map[string]interface{}) bool {
	_, hasLinks := result["_links"]
	if !hasLinks {
		_, hasLinks = result["links"]
	}
	return hasLinks
}

// addPaginationLinks adds pagination links to the result
func (h *SearchContentHandler) addPaginationLinks(result map[string]interface{}, start, limit int) {
	links := map[string]interface{}{}

	// Add self link
	links["self"] = fmt.Sprintf("/content/search?start=%d&limit=%d", start, limit)

	// Add next link if there might be more results
	// Check if the number of results equals the limit (indicating there might be more)
	hasMore := false
	if results, ok := result["results"].([]interface{}); ok {
		if len(results) == limit {
			hasMore = true
		}
	}
	// Also check if size field indicates there might be more
	if !hasMore {
		if size, ok := result["size"].(float64); ok && int(size) >= limit {
			hasMore = true
		} else if size, ok := result["size"].(int); ok && size >= limit {
			hasMore = true
		}
	}

	if hasMore {
		links["next"] = fmt.Sprintf("/content/search?start=%d&limit=%d", start+limit, limit)
	}

	// Add prev link if not at the beginning
	if start > 0 {
		prevStart := start - limit
		if prevStart < 0 {
			prevStart = 0
		}
		links["prev"] = fmt.Sprintf("/content/search?start=%d&limit=%d", prevStart, limit)
	}

	result["_links"] = links
}
