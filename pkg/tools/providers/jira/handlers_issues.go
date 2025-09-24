package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Issue Handlers

// GetIssueHandler handles getting a specific issue
type GetIssueHandler struct {
	provider *JiraProvider
}

func NewGetIssueHandler(p *JiraProvider) *GetIssueHandler {
	return &GetIssueHandler{provider: p}
}

func (h *GetIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_issue",
		Description: "Retrieve detailed information about a specific Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "Issue ID or key (e.g., 'PROJ-123')",
					"pattern":     "^[A-Z][A-Z0-9_]*-[1-9][0-9]*$|^[0-9]+$",
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
					"description": "Fields to expand (e.g., 'changelog', 'transitions')",
				},
			},
			"required": []interface{}{"issueIdOrKey"},
		},
	}
}

func (h *GetIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	issueKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// Build request URL
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s", issueKey))

	// Add query parameters
	if fields, ok := params["fields"].([]interface{}); ok {
		fieldList := make([]string, len(fields))
		for i, f := range fields {
			if s, ok := f.(string); ok {
				fieldList[i] = s
			}
		}
		if len(fieldList) > 0 {
			url += "?fields=" + strings.Join(fieldList, ",")
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
		return NewToolError(fmt.Sprintf("Failed to get issue: %v", err)), nil
	}
	defer resp.Body.Close()

	// Parse response
	if resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to get issue: status %d", resp.StatusCode)), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	return NewToolResult(result), nil
}

// CreateIssueHandler handles creating a new issue
type CreateIssueHandler struct {
	provider *JiraProvider
}

func NewCreateIssueHandler(p *JiraProvider) *CreateIssueHandler {
	return &CreateIssueHandler{provider: p}
}

func (h *CreateIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_issue",
		Description: "Create a new Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"fields": map[string]interface{}{
					"type":        "object",
					"description": "Issue fields including project, issuetype, summary, description",
					"properties": map[string]interface{}{
						"project": map[string]interface{}{
							"type":        "object",
							"description": "Project key or id",
							"properties": map[string]interface{}{
								"key": map[string]interface{}{
									"type": "string",
								},
								"id": map[string]interface{}{
									"type": "string",
								},
							},
						},
						"issuetype": map[string]interface{}{
							"type":        "object",
							"description": "Issue type",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type": "string",
								},
								"id": map[string]interface{}{
									"type": "string",
								},
							},
						},
						"summary": map[string]interface{}{
							"type":        "string",
							"description": "Issue summary",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Issue description",
						},
					},
					"required": []interface{}{"project", "issuetype", "summary"},
				},
			},
			"required": []interface{}{"fields"},
		},
	}
}

func (h *CreateIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Prepare request body
	body, err := json.Marshal(params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	// Create request
	url := h.provider.buildURL("/rest/api/3/issue")
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
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
		return NewToolError(fmt.Sprintf("Failed to create issue: %v", err)), nil
	}
	defer resp.Body.Close()

	// Parse response
	if resp.StatusCode != http.StatusCreated {
		return NewToolError(fmt.Sprintf("Failed to create issue: status %d", resp.StatusCode)), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	return NewToolResult(result), nil
}

// UpdateIssueHandler handles updating an existing issue
type UpdateIssueHandler struct {
	provider *JiraProvider
}

func NewUpdateIssueHandler(p *JiraProvider) *UpdateIssueHandler {
	return &UpdateIssueHandler{provider: p}
}

func (h *UpdateIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_issue",
		Description: "Update an existing Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "Issue ID or key to update",
				},
				"fields": map[string]interface{}{
					"type":        "object",
					"description": "Fields to update",
				},
				"notifyUsers": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to notify users",
					"default":     true,
				},
			},
			"required": []interface{}{"issueIdOrKey"},
		},
	}
}

func (h *UpdateIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	issueKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// Prepare update body
	updateBody := map[string]interface{}{}
	if fields, ok := params["fields"]; ok {
		updateBody["fields"] = fields
	}

	body, err := json.Marshal(updateBody)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	// Create request
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s", issueKey))
	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(body)))
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
		return NewToolError(fmt.Sprintf("Failed to update issue: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return NewToolError(fmt.Sprintf("Failed to update issue: status %d", resp.StatusCode)), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s updated successfully", issueKey),
	}), nil
}

// DeleteIssueHandler handles deleting an issue
type DeleteIssueHandler struct {
	provider *JiraProvider
}

func NewDeleteIssueHandler(p *JiraProvider) *DeleteIssueHandler {
	return &DeleteIssueHandler{provider: p}
}

func (h *DeleteIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_issue",
		Description: "Delete a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "Issue ID or key to delete",
				},
				"deleteSubtasks": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to delete subtasks",
					"default":     false,
				},
			},
			"required": []interface{}{"issueIdOrKey"},
		},
	}
}

func (h *DeleteIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	issueKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// Create request
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s", issueKey))
	if deleteSubtasks, ok := params["deleteSubtasks"].(bool); ok && deleteSubtasks {
		url += "?deleteSubtasks=true"
	}

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
		return NewToolError(fmt.Sprintf("Failed to delete issue: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusNoContent {
		return NewToolError(fmt.Sprintf("Failed to delete issue: status %d", resp.StatusCode)), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s deleted successfully", issueKey),
	}), nil
}

