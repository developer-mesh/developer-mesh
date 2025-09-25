package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Label Handlers

// GetPageLabelsHandler handles getting labels for a page
type GetPageLabelsHandler struct {
	provider *ConfluenceProvider
}

func NewGetPageLabelsHandler(p *ConfluenceProvider) *GetPageLabelsHandler {
	return &GetPageLabelsHandler{provider: p}
}

func (h *GetPageLabelsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_page_labels",
		Description: "Get all labels associated with a Confluence page",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID",
				},
				"prefix": map[string]interface{}{
					"type":        "string",
					"description": "Filter labels by prefix (e.g., 'global', 'my', 'team')",
					"enum":        []interface{}{"global", "my", "team"},
				},
				"sort": map[string]interface{}{
					"type":        "string",
					"description": "Sort order for labels",
					"enum":        []interface{}{"created-date", "-created-date", "id", "-id", "name", "-name"},
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of labels to return",
					"default":     50,
					"maximum":     200,
				},
				"cursor": map[string]interface{}{
					"type":        "string",
					"description": "Cursor for pagination",
				},
			},
			"required": []interface{}{"pageId"},
		},
	}
}

func (h *GetPageLabelsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	// First, check if the page is accessible (permission check)
	// We need to get the page details to check its space for filtering
	pageURL := h.provider.buildURL(fmt.Sprintf("/pages/%s", pageId))
	pageReq, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create page request: %v", err)), nil
	}

	// Add authentication for page check
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	pageReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	pageReq.Header.Set("Accept", "application/json")

	// Check page access
	pageResp, err := h.provider.httpClient.Do(pageReq)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to check page access: %v", err)), nil
	}
	defer func() {
		_ = pageResp.Body.Close()
	}()

	// Check permissions
	if pageResp.StatusCode == http.StatusNotFound {
		return NewToolError(fmt.Sprintf("Page %s not found", pageId)), nil
	} else if pageResp.StatusCode == http.StatusForbidden {
		return NewToolError(fmt.Sprintf("No permission to access page %s", pageId)), nil
	} else if pageResp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to access page %s: status %d", pageId, pageResp.StatusCode)), nil
	}

	// Parse page response to check space filtering
	var pageData map[string]interface{}
	if err := json.NewDecoder(pageResp.Body).Decode(&pageData); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse page response: %v", err)), nil
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

	// Build query parameters for labels
	queryParams := url.Values{}

	// Add prefix filter
	if prefix, ok := params["prefix"].(string); ok {
		queryParams.Set("prefix", prefix)
	}

	// Add sort parameter
	if sort, ok := params["sort"].(string); ok {
		queryParams.Set("sort", sort)
	}

	// Add limit with validation
	limit := 50
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
		if limit < 1 {
			limit = 1
		} else if limit > 200 {
			limit = 200
		}
	}
	queryParams.Set("limit", fmt.Sprintf("%d", limit))

	// Add cursor for pagination
	if cursor, ok := params["cursor"].(string); ok && cursor != "" {
		queryParams.Set("cursor", cursor)
	}

	// Build request URL - using v2 API
	labelsURL := h.provider.buildURL(fmt.Sprintf("/pages/%s/labels", pageId))
	if len(queryParams) > 0 {
		labelsURL += "?" + queryParams.Encode()
	}

	// Create request for labels
	req, err := http.NewRequestWithContext(ctx, "GET", labelsURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get labels: %v", err)), nil
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

	// Check response status after parsing (to get error message if available)
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to get labels: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata about the request
	result["_metadata"] = map[string]interface{}{
		"pageId": pageId,
		"limit":  limit,
	}
	if prefix, ok := params["prefix"].(string); ok {
		result["_metadata"].(map[string]interface{})["prefix"] = prefix
	}

	return NewToolResult(result), nil
}

// AddLabelHandler handles adding a label to a page
type AddLabelHandler struct {
	provider *ConfluenceProvider
}

func NewAddLabelHandler(p *ConfluenceProvider) *AddLabelHandler {
	return &AddLabelHandler{provider: p}
}

func (h *AddLabelHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "add_page_label",
		Description: "Add a label to a Confluence page",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID",
				},
				"label": map[string]interface{}{
					"type":        "string",
					"description": "Label to add",
				},
				"prefix": map[string]interface{}{
					"type":        "string",
					"description": "Label prefix (defaults to 'global')",
					"enum":        []interface{}{"global", "my", "team"},
					"default":     "global",
				},
			},
			"required": []interface{}{"pageId", "label"},
		},
	}
}

func (h *AddLabelHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	label, ok := params["label"].(string)
	if !ok || label == "" {
		return NewToolError("label is required"), nil
	}

	// Validate label (basic validation)
	label = strings.TrimSpace(label)
	if label == "" {
		return NewToolError("label cannot be empty or whitespace"), nil
	}
	if len(label) > 255 {
		return NewToolError("label cannot exceed 255 characters"), nil
	}

	// Check if read-only mode is enabled
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot add labels in read-only mode"), nil
	}

	// First, check if the page is accessible (permission check)
	pageURL := h.provider.buildV1URL(fmt.Sprintf("/content/%s", pageId))
	pageReq, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create page request: %v", err)), nil
	}

	// Add authentication for page check
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	pageReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	pageReq.Header.Set("Accept", "application/json")

	// Check page access
	pageResp, err := h.provider.httpClient.Do(pageReq)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to check page access: %v", err)), nil
	}
	defer func() {
		_ = pageResp.Body.Close()
	}()

	// Check permissions
	if pageResp.StatusCode == http.StatusNotFound {
		return NewToolError(fmt.Sprintf("Page %s not found", pageId)), nil
	} else if pageResp.StatusCode == http.StatusForbidden {
		return NewToolError(fmt.Sprintf("No permission to modify page %s", pageId)), nil
	} else if pageResp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to access page %s: status %d", pageId, pageResp.StatusCode)), nil
	}

	// Parse page response to check space filtering
	var pageData map[string]interface{}
	if err := json.NewDecoder(pageResp.Body).Decode(&pageData); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse page response: %v", err)), nil
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

	prefix := "global"
	if p, ok := params["prefix"].(string); ok {
		prefix = p
	}

	// Build request body
	body := map[string]interface{}{
		"prefix": prefix,
		"name":   label,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	// Build request URL - using v1 API for label creation (v2 may not support POST)
	url := h.provider.buildV1URL(fmt.Sprintf("/content/%s/label", pageId))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to add label: %v", err)), nil
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

	// Check response status after parsing (to get error message if available)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errorMsg := fmt.Sprintf("Failed to add label: status %d", resp.StatusCode)
		if message, ok := result["message"].(string); ok {
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, message)
		}
		return NewToolError(errorMsg), nil
	}

	// Add operation metadata
	result["_operation"] = map[string]interface{}{
		"action": "add_label",
		"pageId": pageId,
		"label":  label,
		"prefix": prefix,
	}

	return NewToolResult(result), nil
}

// RemoveLabelHandler handles removing a label from a page
type RemoveLabelHandler struct {
	provider *ConfluenceProvider
}

func NewRemoveLabelHandler(p *ConfluenceProvider) *RemoveLabelHandler {
	return &RemoveLabelHandler{provider: p}
}

func (h *RemoveLabelHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "remove_page_label",
		Description: "Remove a label from a Confluence page",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pageId": map[string]interface{}{
					"type":        "string",
					"description": "Page ID",
				},
				"label": map[string]interface{}{
					"type":        "string",
					"description": "Label to remove",
				},
			},
			"required": []interface{}{"pageId", "label"},
		},
	}
}

func (h *RemoveLabelHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pageId, ok := params["pageId"].(string)
	if !ok || pageId == "" {
		return NewToolError("pageId is required"), nil
	}

	label, ok := params["label"].(string)
	if !ok || label == "" {
		return NewToolError("label is required"), nil
	}

	// Validate label
	label = strings.TrimSpace(label)
	if label == "" {
		return NewToolError("label cannot be empty or whitespace"), nil
	}

	// Check if read-only mode is enabled
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot remove labels in read-only mode"), nil
	}

	// First, check if the page is accessible (permission check)
	pageURL := h.provider.buildV1URL(fmt.Sprintf("/content/%s", pageId))
	pageReq, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create page request: %v", err)), nil
	}

	// Add authentication for page check
	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	pageReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	pageReq.Header.Set("Accept", "application/json")

	// Check page access
	pageResp, err := h.provider.httpClient.Do(pageReq)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to check page access: %v", err)), nil
	}
	defer func() {
		_ = pageResp.Body.Close()
	}()

	// Check permissions
	if pageResp.StatusCode == http.StatusNotFound {
		return NewToolError(fmt.Sprintf("Page %s not found", pageId)), nil
	} else if pageResp.StatusCode == http.StatusForbidden {
		return NewToolError(fmt.Sprintf("No permission to modify page %s", pageId)), nil
	} else if pageResp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to access page %s: status %d", pageId, pageResp.StatusCode)), nil
	}

	// Parse page response to check space filtering
	var pageData map[string]interface{}
	if err := json.NewDecoder(pageResp.Body).Decode(&pageData); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse page response: %v", err)), nil
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

	// Build request URL - using v1 API for label deletion (v2 may not support DELETE)
	// URL encode the label to handle special characters
	url := h.provider.buildV1URL(fmt.Sprintf("/content/%s/label/%s", pageId, url.QueryEscape(label)))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	// Add authentication
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to remove label: %v", err)), nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Check response
	if resp.StatusCode == http.StatusNotFound {
		return NewToolError(fmt.Sprintf("Label '%s' not found on page %s", label, pageId)), nil
	} else if resp.StatusCode == http.StatusForbidden {
		return NewToolError(fmt.Sprintf("No permission to remove label from page %s", pageId)), nil
	} else if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		// Try to parse error message
		var errorData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorData); err == nil {
			if message, ok := errorData["message"].(string); ok {
				return NewToolError(fmt.Sprintf("Failed to remove label: %s", message)), nil
			}
		}
		return NewToolError(fmt.Sprintf("Failed to remove label: status %d", resp.StatusCode)), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Label '%s' removed from page %s", label, pageId),
		"_operation": map[string]interface{}{
			"action": "remove_label",
			"pageId": pageId,
			"label":  label,
		},
	}), nil
}
