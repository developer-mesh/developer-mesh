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

	// Build query parameters
	queryParams := url.Values{}

	// Add prefix filter
	if prefix, ok := params["prefix"].(string); ok {
		queryParams.Set("prefix", prefix)
	}

	// Add sort parameter
	if sort, ok := params["sort"].(string); ok {
		queryParams.Set("sort", sort)
	}

	// Add limit
	if limit, ok := params["limit"].(float64); ok {
		queryParams.Set("limit", fmt.Sprintf("%d", int(limit)))
	} else {
		queryParams.Set("limit", "50")
	}

	// Add cursor for pagination
	if cursor, ok := params["cursor"].(string); ok {
		queryParams.Set("cursor", cursor)
	}

	// Build request URL - using v2 API
	labelsURL := h.provider.buildURL(fmt.Sprintf("/pages/%s/labels", pageId))
	if len(queryParams) > 0 {
		labelsURL += "?" + queryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", labelsURL, nil)
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
		return NewToolError(fmt.Sprintf("Failed to get labels: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to get labels: status %d", resp.StatusCode)), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
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
		return NewToolError(fmt.Sprintf("Failed to add label: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return NewToolError(fmt.Sprintf("Failed to add label: status %d", resp.StatusCode)), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
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

	// Build request URL - using v1 API for label deletion (v2 may not support DELETE)
	url := h.provider.buildV1URL(fmt.Sprintf("/content/%s/label/%s", pageId, label))

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
		return NewToolError(fmt.Sprintf("Failed to remove label: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to remove label: status %d", resp.StatusCode)), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Label '%s' removed from page %s", label, pageId),
	}), nil
}