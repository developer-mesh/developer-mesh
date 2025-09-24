package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// Search Handlers

// SearchContentHandler handles searching for content using CQL
type SearchContentHandler struct {
	provider *ConfluenceProvider
}

func NewSearchContentHandler(p *ConfluenceProvider) *SearchContentHandler {
	return &SearchContentHandler{provider: p}
}

func (h *SearchContentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_content",
		Description: "Search Confluence content using CQL (Confluence Query Language)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cql": map[string]interface{}{
					"type":        "string",
					"description": "CQL query string (e.g., 'space = DEV AND type = page')",
				},
				"start": map[string]interface{}{
					"type":        "integer",
					"description": "Starting index for pagination",
					"default":     0,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return",
					"default":     25,
					"maximum":     100,
				},
				"expand": map[string]interface{}{
					"type":        "string",
					"description": "Properties to expand in results",
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

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("cql", cql)

	// Add pagination parameters
	if start, ok := params["start"].(float64); ok {
		queryParams.Set("start", fmt.Sprintf("%d", int(start)))
	}

	if limit, ok := params["limit"].(float64); ok {
		queryParams.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		queryParams.Set("limit", "25")
	}

	// Add expand parameter
	if expand, ok := params["expand"].(string); ok {
		queryParams.Set("expand", expand)
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

	// Apply space filter if configured
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Metadata != nil {
		if spaceFilter, ok := pctx.Metadata["CONFLUENCE_SPACES_FILTER"].(string); ok && spaceFilter != "" {
			// Filter results to only include content from allowed spaces
			if results, ok := result["results"].([]interface{}); ok {
				filtered := []interface{}{}
				for _, item := range results {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if space, ok := itemMap["space"].(map[string]interface{}); ok {
							if key, ok := space["key"].(string); ok {
								// Check if space is in filter
								if containsSpace(spaceFilter, key) {
									filtered = append(filtered, item)
								}
							}
						}
					}
				}
				result["results"] = filtered
				result["size"] = len(filtered)
			}
		}
	}

	return NewToolResult(result), nil
}

// Helper function to check if space is in filter
func containsSpace(filter, space string) bool {
	// Simple contains check - could be enhanced to support comma-separated lists
	return filter == "" || filter == space || filter == "*"
}