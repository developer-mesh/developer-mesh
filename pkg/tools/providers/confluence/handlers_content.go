package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// Content Handlers - V1 API Operations
//
// These handlers implement critical Confluence operations that are not available or have
// limited support in the v2 API. While v2 is preferred for modern features, v1 remains
// essential for comprehensive content management.
//
// V1 API is required for:
// - Page Creation: v2 lacks full content creation support with storage format
// - Page Updates: v2 has limited update capabilities, v1 supports full content updates
// - Space Listing: v1 provides better filtering and metadata than v2
// - Attachments: v2 has minimal attachment support, v1 provides full CRUD operations
// - Search/CQL: v2 does not support Confluence Query Language (CQL)
//
// V2 API is used for:
// - Page Retrieval: GET /pages/{id} - better performance and modern features
// - Page Deletion: DELETE /pages/{id} - cleaner implementation
// - Label Reading: GET /pages/{id}/labels - modern label management
//
// API Version Strategy:
// 1. Use v2 when available and feature-complete
// 2. Fall back to v1 for operations not supported in v2
// 3. Document clearly which version is used for each operation
// 4. Include "_metadata.api_version" in all responses for transparency

// CreatePageHandler handles creating a new page using v1 API
// V1 Required: v2 API lacks support for creating pages with full storage format content
type CreatePageHandler struct {
	provider *ConfluenceProvider
}

func NewCreatePageHandler(p *ConfluenceProvider) *CreatePageHandler {
	return &CreatePageHandler{provider: p}
}

func (h *CreatePageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_page",
		Description: "Create a new Confluence page using v1 API (required for full content support)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Page title",
				},
				"spaceKey": map[string]interface{}{
					"type":        "string",
					"description": "Space key where page will be created",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Page content in storage format (HTML)",
				},
				"parentId": map[string]interface{}{
					"type":        "string",
					"description": "Parent page ID (optional, for child pages)",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Content type",
					"enum":        []interface{}{"page", "blogpost"},
					"default":     "page",
				},
			},
			"required": []interface{}{"title", "spaceKey", "content"},
		},
	}
}

func (h *CreatePageHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if read-only mode is enabled
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot create pages in read-only mode"), nil
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return NewToolError("title is required"), nil
	}

	spaceKey, ok := params["spaceKey"].(string)
	if !ok || spaceKey == "" {
		return NewToolError("spaceKey is required"), nil
	}

	content, ok := params["content"].(string)
	if !ok || content == "" {
		return NewToolError("content is required"), nil
	}

	// Check space filter
	if !h.provider.IsSpaceAllowed(ctx, spaceKey) {
		return NewToolError(fmt.Sprintf("Space %s is not in allowed spaces", spaceKey)), nil
	}

	// Build request body
	pageType := "page"
	if t, ok := params["type"].(string); ok {
		pageType = t
	}

	body := map[string]interface{}{
		"type":  pageType,
		"title": title,
		"space": map[string]interface{}{
			"key": spaceKey,
		},
		"body": map[string]interface{}{
			"storage": map[string]interface{}{
				"value":          content,
				"representation": "storage",
			},
		},
	}

	// Add parent if specified
	if parentId, ok := params["parentId"].(string); ok && parentId != "" {
		body["ancestors"] = []map[string]interface{}{
			{
				"id": parentId,
			},
		}
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	// Build request URL - v1 API required for content creation
	url := h.provider.buildV1URL("/content")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create page: %v", err)), nil
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errorMsg := fmt.Sprintf("Failed to create page: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v1",
		"operation":   "create_page",
	}

	return NewToolResult(result), nil
}

// UpdatePageHandler handles updating an existing page using v1 API
// V1 Required: v2 API has limited update support and doesn't handle version conflicts properly
type UpdatePageHandler struct {
	provider *ConfluenceProvider
}

func NewUpdatePageHandler(p *ConfluenceProvider) *UpdatePageHandler {
	return &UpdatePageHandler{provider: p}
}

func (h *UpdatePageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_page",
		Description: "Update an existing Confluence page using v1 API",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID to update",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "New page title (optional)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "New page content in storage format (HTML)",
				},
				"version": map[string]interface{}{
					"type":        "integer",
					"description": "Current version number (required for conflict detection)",
				},
				"minorEdit": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this is a minor edit",
					"default":     false,
				},
			},
			"required": []interface{}{"pageId", "content", "version"},
		},
	}
}

func (h *UpdatePageHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if read-only mode is enabled
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot update pages in read-only mode"), nil
	}

	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	content, ok := params["content"].(string)
	if !ok || content == "" {
		return NewToolError("content is required"), nil
	}

	version, ok := params["version"].(float64)
	if !ok {
		return NewToolError("version is required"), nil
	}

	// First, get the current page to check permissions and space
	getURL := h.provider.buildV1URL(fmt.Sprintf("/content/%s?expand=space,version", pageId))
	getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	getReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	getReq.Header.Set("Accept", "application/json")

	// Get current page
	getResp, err := h.provider.httpClient.Do(getReq)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get page: %v", err)), nil
	}
	defer func() {
		_ = getResp.Body.Close()
	}()

	if getResp.StatusCode == http.StatusNotFound {
		return NewToolError(fmt.Sprintf("Page %s not found", pageId)), nil
	} else if getResp.StatusCode == http.StatusForbidden {
		return NewToolError(fmt.Sprintf("No permission to update page %s", pageId)), nil
	} else if getResp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to get page: status %d", getResp.StatusCode)), nil
	}

	var currentPage map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&currentPage); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse page: %v", err)), nil
	}

	// Check space filter
	if space, ok := currentPage["space"].(map[string]interface{}); ok {
		if spaceKey, ok := space["key"].(string); ok {
			if !h.provider.IsSpaceAllowed(ctx, spaceKey) {
				return NewToolError(fmt.Sprintf("Page %s is in space %s which is not allowed", pageId, spaceKey)), nil
			}
		}
	}

	// Build update request body
	body := map[string]interface{}{
		"id":    pageId,
		"type":  currentPage["type"],
		"title": currentPage["title"], // Use existing title by default
		"space": currentPage["space"],
		"body": map[string]interface{}{
			"storage": map[string]interface{}{
				"value":          content,
				"representation": "storage",
			},
		},
		"version": map[string]interface{}{
			"number":    int(version) + 1,
			"minorEdit": false,
		},
	}

	// Update title if provided
	if title, ok := params["title"].(string); ok && title != "" {
		body["title"] = title
	}

	// Set minor edit flag
	if minorEdit, ok := params["minorEdit"].(bool); ok {
		body["version"].(map[string]interface{})["minorEdit"] = minorEdit
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	// Build request URL
	updateURL := h.provider.buildV1URL(fmt.Sprintf("/content/%s", pageId))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "PUT", updateURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add headers
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update page: %v", err)), nil
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
	if resp.StatusCode == http.StatusConflict {
		return NewToolError("Version conflict - page was modified by another user"), nil
	} else if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to update page: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v1",
		"operation":   "update_page",
	}

	return NewToolResult(result), nil
}

// ListSpacesHandler handles listing spaces using v1 API
// V1 Preferred: v1 provides better space metadata and filtering options than v2
type ListSpacesHandler struct {
	provider *ConfluenceProvider
}

func NewListSpacesHandler(p *ConfluenceProvider) *ListSpacesHandler {
	return &ListSpacesHandler{provider: p}
}

func (h *ListSpacesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_spaces",
		Description: "List all accessible Confluence spaces using v1 API",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by space type",
					"enum":        []interface{}{"global", "personal"},
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by space status",
					"enum":        []interface{}{"current", "archived"},
					"default":     "current",
				},
				"start": map[string]interface{}{
					"type":        "integer",
					"description": "Starting index for pagination",
					"default":     0,
					"minimum":     0,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results",
					"default":     25,
					"minimum":     1,
					"maximum":     100,
				},
			},
		},
	}
}

func (h *ListSpacesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Build query parameters
	queryParams := url.Values{}

	// Add type filter
	if spaceType, ok := params["type"].(string); ok {
		queryParams.Set("type", spaceType)
	}

	// Add status filter
	status := "current"
	if s, ok := params["status"].(string); ok {
		status = s
	}
	queryParams.Set("status", status)

	// Add pagination
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

	// Build request URL - v1 API has better space listing support
	spacesURL := h.provider.buildV1URL("/space")
	if len(queryParams) > 0 {
		spacesURL += "?" + queryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", spacesURL, nil)
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
		return NewToolError(fmt.Sprintf("Failed to list spaces: %v", err)), nil
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
		errorMsg := fmt.Sprintf("Failed to list spaces: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Apply space filter
	if filtered := h.provider.FilterSpaceResults(ctx, result); filtered != nil {
		if filteredMap, ok := filtered.(map[string]interface{}); ok {
			result = filteredMap
		}
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v1",
		"operation":   "list_spaces",
		"start":       start,
		"limit":       limit,
	}

	return NewToolResult(result), nil
}

// GetAttachmentsHandler handles getting attachments for a page using v1 API
// V1 Required: v2 API has minimal attachment support, v1 provides full attachment management
type GetAttachmentsHandler struct {
	provider *ConfluenceProvider
}

func NewGetAttachmentsHandler(p *ConfluenceProvider) *GetAttachmentsHandler {
	return &GetAttachmentsHandler{provider: p}
}

func (h *GetAttachmentsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_attachments",
		Description: "Get attachments for a Confluence page using v1 API",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID to get attachments for",
				},
				"filename": map[string]interface{}{
					"type":        "string",
					"description": "Filter by specific filename",
				},
				"mediaType": map[string]interface{}{
					"type":        "string",
					"description": "Filter by media type",
				},
				"start": map[string]interface{}{
					"type":        "integer",
					"description": "Starting index for pagination",
					"default":     0,
					"minimum":     0,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results",
					"default":     25,
					"minimum":     1,
					"maximum":     100,
				},
			},
			"required": []interface{}{"pageId"},
		},
	}
}

func (h *GetAttachmentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	// First check if page is accessible
	pageURL := h.provider.buildV1URL(fmt.Sprintf("/content/%s", pageId))
	pageReq, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	pageReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	pageReq.Header.Set("Accept", "application/json")

	// Check page access
	pageResp, err := h.provider.httpClient.Do(pageReq)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to check page: %v", err)), nil
	}
	defer func() {
		_ = pageResp.Body.Close()
	}()

	if pageResp.StatusCode == http.StatusNotFound {
		return NewToolError(fmt.Sprintf("Page %s not found", pageId)), nil
	} else if pageResp.StatusCode == http.StatusForbidden {
		return NewToolError(fmt.Sprintf("No permission to access page %s", pageId)), nil
	} else if pageResp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to access page: status %d", pageResp.StatusCode)), nil
	}

	// Parse page to check space filter
	var pageData map[string]interface{}
	if err := json.NewDecoder(pageResp.Body).Decode(&pageData); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse page: %v", err)), nil
	}

	// Check space filter
	if filtered := h.provider.FilterSpaceResults(ctx, map[string]interface{}{
		"results": []interface{}{pageData},
	}); filtered != nil {
		if filteredMap, ok := filtered.(map[string]interface{}); ok {
			if results, ok := filteredMap["results"].([]interface{}); ok && len(results) == 0 {
				return NewToolError(fmt.Sprintf("Page %s is not in an allowed space", pageId)), nil
			}
		}
	}

	// Build query parameters for attachments
	queryParams := url.Values{}

	// Add filename filter
	if filename, ok := params["filename"].(string); ok && filename != "" {
		queryParams.Set("filename", filename)
	}

	// Add media type filter
	if mediaType, ok := params["mediaType"].(string); ok && mediaType != "" {
		queryParams.Set("mediaType", mediaType)
	}

	// Add pagination
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

	// Build request URL - v1 API for attachment support
	attachURL := h.provider.buildV1URL(fmt.Sprintf("/content/%s/child/attachment", pageId))
	if len(queryParams) > 0 {
		attachURL += "?" + queryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", attachURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add headers
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get attachments: %v", err)), nil
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
		errorMsg := fmt.Sprintf("Failed to get attachments: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v1",
		"operation":   "get_attachments",
		"pageId":      pageId,
		"start":       start,
		"limit":       limit,
	}

	return NewToolResult(result), nil
}

// IsSpaceAllowed checks if a space is in the allowed list
func (p *ConfluenceProvider) IsSpaceAllowed(ctx context.Context, spaceKey string) bool {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return true // No filter configured
	}

	spaceFilter, ok := pctx.Metadata["CONFLUENCE_SPACES_FILTER"].(string)
	if !ok || spaceFilter == "" || spaceFilter == "*" {
		return true // No filter or wildcard allows all
	}

	// Check if space is in allowed list
	allowedSpaces := strings.Split(spaceFilter, ",")
	for _, allowed := range allowedSpaces {
		allowed = strings.TrimSpace(allowed)
		if allowed == spaceKey {
			return true
		}
		// Support wildcards
		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(spaceKey, prefix) {
				return true
			}
		}
	}

	return false
}
