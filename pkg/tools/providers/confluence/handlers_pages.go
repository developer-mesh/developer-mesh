package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Helper function for basic auth
func basicAuth(email, apiToken string) string {
	auth := email + ":" + apiToken
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// Page Handlers

// GetPageHandler handles getting a specific page
type GetPageHandler struct {
	provider *ConfluenceProvider
}

func NewGetPageHandler(p *ConfluenceProvider) *GetPageHandler {
	return &GetPageHandler{provider: p}
}

func (h *GetPageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_page",
		Description: "Retrieve detailed information about a specific Confluence page",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID",
				},
				"expand": map[string]interface{}{
					"type":        "array",
					"description": "Properties to expand (e.g., 'body.storage', 'version', 'ancestors')",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []interface{}{"pageId"},
		},
	}
}

func (h *GetPageHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	// Build request URL - using v2 API
	url := h.provider.buildURL(fmt.Sprintf("/pages/%s", pageId))

	// Add expand parameter
	if expands, ok := params["expand"].([]interface{}); ok && len(expands) > 0 {
		expandList := make([]string, len(expands))
		for i, e := range expands {
			if s, ok := e.(string); ok {
				expandList[i] = s
			}
		}
		if len(expandList) > 0 {
			url += "?expand=" + strings.Join(expandList, ",")
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		return NewToolError(fmt.Sprintf("Failed to get page: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to get page: status %d", resp.StatusCode)), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	return NewToolResult(result), nil
}

// ListPagesHandler handles listing pages
type ListPagesHandler struct {
	provider *ConfluenceProvider
}

func NewListPagesHandler(p *ConfluenceProvider) *ListPagesHandler {
	return &ListPagesHandler{provider: p}
}

func (h *ListPagesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_pages",
		Description: "List Confluence pages with filtering options",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"spaceId": map[string]interface{}{
					"type":        "string",
					"description": "Filter by space ID",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status (current, draft, archived)",
					"enum":        []interface{}{"current", "draft", "archived"},
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort order",
					"enum":        []interface{}{"id", "-id", "created-date", "-created-date", "modified-date", "-modified-date"},
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of pages to return",
					"default":     25,
					"maximum":     250,
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination",
				},
			},
		},
	}
}

func (h *ListPagesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Build query parameters
	queryParams := url.Values{}

	// Add space filter
	if spaceId, ok := params["spaceId"].(string); ok {
		queryParams.Set("space-id", spaceId)
	}

	// Add status filter
	if status, ok := params["status"].(string); ok {
		queryParams.Set("status", status)
	}

	// Add sort parameter
	if sort, ok := params["sort"].(string); ok {
		queryParams.Set("sort", sort)
	}

	// Add limit
	if limit, ok := params["limit"].(float64); ok {
		queryParams.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		queryParams.Set("limit", "25")
	}

	// Add cursor for pagination
	if cursor, ok := params["cursor"].(string); ok {
		queryParams.Set("cursor", cursor)
	}

	// Build request URL - using v2 API
	pagesURL := h.provider.buildURL("/pages")
	if len(queryParams) > 0 {
		pagesURL += "?" + queryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", pagesURL, nil)
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
		return NewToolError(fmt.Sprintf("Failed to list pages: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to list pages: status %d", resp.StatusCode)), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	return NewToolResult(result), nil
}

// DeletePageHandler handles deleting a page
type DeletePageHandler struct {
	provider *ConfluenceProvider
}

func NewDeletePageHandler(p *ConfluenceProvider) *DeletePageHandler {
	return &DeletePageHandler{provider: p}
}

func (h *DeletePageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_page",
		Description: "Delete a Confluence page",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID to delete",
				},
			},
			"required": []interface{}{"pageId"},
		},
	}
}

func (h *DeletePageHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	// Build request URL - using v2 API
	url := h.provider.buildURL(fmt.Sprintf("/pages/%s", pageId))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete page: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to delete page: status %d", resp.StatusCode)), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Page %s deleted successfully", pageId),
	}), nil
}

// buildURL helper method for ConfluenceProvider
func (p *ConfluenceProvider) buildURL(path string) string {
	// Use v2 API endpoint
	return fmt.Sprintf("https://%s.atlassian.net/wiki/api/v2%s", p.domain, path)
}