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

// GetPageHandler handles getting a specific page using Confluence v2 API
type GetPageHandler struct {
	provider *ConfluenceProvider
}

func NewGetPageHandler(p *ConfluenceProvider) *GetPageHandler {
	return &GetPageHandler{provider: p}
}

func (h *GetPageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_page",
		Description: "Retrieve detailed information about a specific Confluence page using v2 API",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID",
				},
				"expand": map[string]interface{}{
					"type":        "array",
					"description": "Properties to expand in v2 API (e.g., 'body', 'version', 'ancestors', 'children', 'descendants', 'space', 'history')",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"version": map[string]interface{}{
					"type":        "integer",
					"description": "Version number to retrieve (optional, defaults to latest)",
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

	// Build query parameters
	queryParams := url.Values{}

	// Add expand parameter for v2 API
	if expands, ok := params["expand"].([]interface{}); ok && len(expands) > 0 {
		expandList := make([]string, len(expands))
		for i, e := range expands {
			if s, ok := e.(string); ok {
				expandList[i] = s
			}
		}
		if len(expandList) > 0 {
			queryParams.Set("expand", strings.Join(expandList, ","))
		}
	}

	// Add version parameter if specified
	if version, ok := params["version"].(float64); ok {
		queryParams.Set("version", fmt.Sprintf("%d", int(version)))
	}

	// Build request URL - using v2 API
	pageURL := h.provider.buildURL(fmt.Sprintf("/pages/%s", pageId))
	if len(queryParams) > 0 {
		pageURL += "?" + queryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Parse response body
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to get page: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Apply space filtering if configured
	if filtered := h.provider.FilterSpaceResults(ctx, result); filtered != nil {
		if filteredMap, ok := filtered.(map[string]interface{}); ok {
			result = filteredMap
		}
	}

	return NewToolResult(result), nil
}

// ListPagesHandler handles listing pages using Confluence v2 API with cursor-based pagination
type ListPagesHandler struct {
	provider *ConfluenceProvider
}

func NewListPagesHandler(p *ConfluenceProvider) *ListPagesHandler {
	return &ListPagesHandler{provider: p}
}

func (h *ListPagesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_pages",
		Description: "List Confluence pages using v2 API with cursor-based pagination",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"spaceId": map[string]interface{}{
					"type":        "string",
					"description": "Filter by space ID (will be overridden by CONFLUENCE_SPACES_FILTER if set)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status (current, draft, archived, trashed)",
					"enum":        []interface{}{"current", "draft", "archived", "trashed"},
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Filter by title (partial match)",
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort order for v2 API",
					"enum":        []interface{}{"id", "-id", "created-date", "-created-date", "modified-date", "-modified-date", "title", "-title"},
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of pages to return per request (max 250)",
					"default":     25,
					"minimum":     1,
					"maximum":     250,
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination (v2 API uses cursor-based pagination)",
				},
				"expand": map[string]interface{}{
					"type":        "array",
					"description": "Properties to expand (e.g., 'body', 'version', 'space')",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}
}

func (h *ListPagesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Build query parameters
	queryParams := url.Values{}

	// Add space filter (may be overridden by CONFLUENCE_SPACES_FILTER)
	if spaceId, ok := params["spaceId"].(string); ok && spaceId != "" {
		queryParams.Set("space-id", spaceId)
	}

	// Add status filter
	if status, ok := params["status"].(string); ok && status != "" {
		queryParams.Set("status", status)
	}

	// Add title filter
	if title, ok := params["title"].(string); ok && title != "" {
		queryParams.Set("title", title)
	}

	// Add sort parameter
	if sort, ok := params["sort"].(string); ok && sort != "" {
		queryParams.Set("sort", sort)
	}

	// Add limit with validation
	limit := 25
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
		if limit < 1 {
			limit = 1
		} else if limit > 250 {
			limit = 250
		}
	}
	queryParams.Set("limit", fmt.Sprintf("%d", limit))

	// Add cursor for pagination (v2 API)
	if cursor, ok := params["cursor"].(string); ok && cursor != "" {
		queryParams.Set("cursor", cursor)
	}

	// Add expand parameter
	if expands, ok := params["expand"].([]interface{}); ok && len(expands) > 0 {
		expandList := make([]string, len(expands))
		for i, e := range expands {
			if s, ok := e.(string); ok {
				expandList[i] = s
			}
		}
		if len(expandList) > 0 {
			queryParams.Set("expand", strings.Join(expandList, ","))
		}
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Parse response body
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to list pages: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Apply space filtering if configured
	if filtered := h.provider.FilterSpaceResults(ctx, result); filtered != nil {
		if filteredMap, ok := filtered.(map[string]interface{}); ok {
			result = filteredMap
		}
	}

	// Add pagination info to result if not present
	if _, hasLinks := result["_links"]; !hasLinks {
		result["_links"] = map[string]interface{}{
			"self": pagesURL,
		}
	}

	return NewToolResult(result), nil
}

// DeletePageHandler handles deleting a page using Confluence v2 API
type DeletePageHandler struct {
	provider *ConfluenceProvider
}

func NewDeletePageHandler(p *ConfluenceProvider) *DeletePageHandler {
	return &DeletePageHandler{provider: p}
}

func (h *DeletePageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_page",
		Description: "Delete a Confluence page using v2 API (moves to trash or permanently deletes)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID to delete",
				},
				"purge": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, permanently delete the page. If false, move to trash (default: false)",
					"default":     false,
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

	// Check if we should purge (permanently delete)
	purge := false
	if p, ok := params["purge"].(bool); ok {
		purge = p
	}

	// Build request URL - using v2 API
	deleteURL := h.provider.buildURL(fmt.Sprintf("/pages/%s", pageId))

	// Add purge parameter if needed
	if purge {
		deleteURL += "?purge=true"
	}

	// First, get the page to check if we can access it
	// This helps provide better error messages
	getReq, err := http.NewRequestWithContext(ctx, "GET", h.provider.buildURL(fmt.Sprintf("/pages/%s", pageId)), nil)
	if err == nil {
		// Add authentication for the check
		if email, token, err := h.provider.extractAuthToken(ctx, params); err == nil {
			getReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
			getReq.Header.Set("Accept", "application/json")

			if resp, err := h.provider.httpClient.Do(getReq); err == nil {
				defer func() {
					if err := resp.Body.Close(); err != nil {
						h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
							"error": err.Error(),
						})
					}
				}()

				switch resp.StatusCode {
				case http.StatusNotFound:
					return NewToolError(fmt.Sprintf("Page %s not found", pageId)), nil
				case http.StatusForbidden:
					return NewToolError(fmt.Sprintf("No permission to access page %s", pageId)), nil
				case http.StatusOK:
					// Parse the page to get its space for filtering check
					var pageData map[string]interface{}
					if err := json.NewDecoder(resp.Body).Decode(&pageData); err == nil {
						// Check if page is in an allowed space
						if space, ok := pageData["space"].(map[string]interface{}); ok {
							if spaceKey, ok := space["key"].(string); ok {
								// Check space filter
								if filtered := h.provider.FilterSpaceResults(ctx, map[string]interface{}{
									"results": []interface{}{pageData},
								}); filtered != nil {
									if results, ok := filtered.(map[string]interface{})["results"].([]interface{}); ok && len(results) == 0 {
										return NewToolError(fmt.Sprintf("Page %s is in space %s which is not allowed by CONFLUENCE_SPACES_FILTER", pageId, spaceKey)), nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Create delete request
	req, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, nil)
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
		return NewToolError(fmt.Sprintf("Failed to delete page: %v", err)), nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Check response - v2 API returns 204 No Content on success
	if resp.StatusCode == http.StatusNoContent {
		action := "moved to trash"
		if purge {
			action = "permanently deleted"
		}
		return NewToolResult(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Page %s %s successfully", pageId, action),
			"pageId":  pageId,
			"purged":  purge,
		}), nil
	}

	// Handle error responses
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		errorMsg := fmt.Sprintf("Failed to delete page: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	return NewToolError(fmt.Sprintf("Failed to delete page: status %d", resp.StatusCode)), nil
}
