package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
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
	queryParams := []string{}
	if fields, ok := params["fields"].([]interface{}); ok {
		fieldList := make([]string, len(fields))
		for i, f := range fields {
			if s, ok := f.(string); ok {
				fieldList[i] = s
			}
		}
		if len(fieldList) > 0 {
			queryParams = append(queryParams, "fields="+strings.Join(fieldList, ","))
		}
	}

	// Add expand parameter if provided
	if expand, ok := params["expand"].(string); ok && expand != "" {
		queryParams = append(queryParams, "expand="+expand)
	}

	if len(queryParams) > 0 {
		url += "?" + strings.Join(queryParams, "&")
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
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errorBody)
		errorMsg := fmt.Sprintf("Failed to get issue: status %d", resp.StatusCode)
		if msg, ok := errorBody["errorMessages"].([]interface{}); ok && len(msg) > 0 {
			errorMsg = fmt.Sprintf("%s - %v", errorMsg, msg[0])
		}
		return NewToolError(errorMsg), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Apply project filtering if configured
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Metadata != nil {
		if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
			// Check if issue belongs to allowed project
			if fields, ok := result["fields"].(map[string]interface{}); ok {
				if project, ok := fields["project"].(map[string]interface{}); ok {
					if projectKey, ok := project["key"].(string); ok {
						allowedProjects := strings.Split(projectFilter, ",")
						if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
							return NewToolError(fmt.Sprintf("Issue belongs to project %s which is not in allowed projects: %s", projectKey, projectFilter)), nil
						}
					}
				}
			}
		}
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v3",
		"operation":   "get_issue",
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
	// Check read-only mode
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot create issue: provider is in read-only mode"), nil
	}

	// Validate required fields
	fields, ok := params["fields"].(map[string]interface{})
	if !ok {
		return NewToolError("fields parameter is required"), nil
	}

	// Extract and validate project
	var projectKey string
	if project, ok := fields["project"].(map[string]interface{}); ok {
		if key, ok := project["key"].(string); ok {
			projectKey = key
		} else if id, ok := project["id"].(string); ok {
			// We have project ID but need to validate against key filter
			// For now, we'll proceed with ID
			projectKey = id
		}
	}

	// Check project filter if configured
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Metadata != nil {
		if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
			if projectKey != "" {
				allowedProjects := strings.Split(projectFilter, ",")
				if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
					return NewToolError(fmt.Sprintf("Cannot create issue in project %s: not in allowed projects (%s)", projectKey, projectFilter)), nil
				}
			}
		}
	}

	// Validate issue type
	if issueType, ok := fields["issuetype"].(map[string]interface{}); !ok {
		return NewToolError("issuetype is required in fields"), nil
	} else {
		if _, hasName := issueType["name"]; !hasName {
			if _, hasId := issueType["id"]; !hasId {
				return NewToolError("issuetype must have either 'name' or 'id'"), nil
			}
		}
	}

	// Validate summary
	if _, ok := fields["summary"].(string); !ok {
		return NewToolError("summary is required in fields"), nil
	}

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
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	if resp.StatusCode != http.StatusCreated {
		errorMsg := fmt.Sprintf("Failed to create issue: status %d", resp.StatusCode)
		if errors, ok := result["errors"].(map[string]interface{}); ok && len(errors) > 0 {
			// Build detailed error message from field errors
			errorDetails := []string{}
			for field, msg := range errors {
				errorDetails = append(errorDetails, fmt.Sprintf("%s: %v", field, msg))
			}
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, strings.Join(errorDetails, ", "))
		} else if msgs, ok := result["errorMessages"].([]interface{}); ok && len(msgs) > 0 {
			errorMsg = fmt.Sprintf("%s - %v", errorMsg, msgs[0])
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v3",
		"operation":   "create_issue",
		"project":     projectKey,
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
	// Check read-only mode
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot update issue: provider is in read-only mode"), nil
	}

	issueKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// First, get the issue to check project permissions
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Metadata != nil {
		if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
			// We need to fetch the issue first to check its project
			getURL := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s?fields=project", issueKey))
			getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
			if err == nil {
				email, token, _ := h.provider.extractAuthToken(ctx, params)
				getReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
				getReq.Header.Set("Accept", "application/json")

				if getResp, err := h.provider.httpClient.Do(getReq); err == nil {
					defer func() {
						_ = getResp.Body.Close()
					}()
					if getResp.StatusCode == http.StatusOK {
						var issueData map[string]interface{}
						if json.NewDecoder(getResp.Body).Decode(&issueData) == nil {
							if fields, ok := issueData["fields"].(map[string]interface{}); ok {
								if project, ok := fields["project"].(map[string]interface{}); ok {
									if projectKey, ok := project["key"].(string); ok {
										allowedProjects := strings.Split(projectFilter, ",")
										if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
											return NewToolError(fmt.Sprintf("Cannot update issue in project %s: not in allowed projects (%s)", projectKey, projectFilter)), nil
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Prepare update body
	updateBody := map[string]interface{}{}
	if fields, ok := params["fields"]; ok {
		updateBody["fields"] = fields
	}
	if update, ok := params["update"]; ok {
		updateBody["update"] = update
	}

	// Add notification settings
	queryParams := []string{}
	if notifyUsers, ok := params["notifyUsers"].(bool); ok && !notifyUsers {
		queryParams = append(queryParams, "notifyUsers=false")
	}

	body, err := json.Marshal(updateBody)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request: %v", err)), nil
	}

	// Create request
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s", issueKey))
	if len(queryParams) > 0 {
		url += "?" + strings.Join(queryParams, "&")
	}

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
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var errorBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errorBody)
		errorMsg := fmt.Sprintf("Failed to update issue: status %d", resp.StatusCode)
		if errors, ok := errorBody["errors"].(map[string]interface{}); ok && len(errors) > 0 {
			errorDetails := []string{}
			for field, msg := range errors {
				errorDetails = append(errorDetails, fmt.Sprintf("%s: %v", field, msg))
			}
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, strings.Join(errorDetails, ", "))
		} else if msgs, ok := errorBody["errorMessages"].([]interface{}); ok && len(msgs) > 0 {
			errorMsg = fmt.Sprintf("%s - %v", errorMsg, msgs[0])
		}
		return NewToolError(errorMsg), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s updated successfully", issueKey),
		"_metadata": map[string]interface{}{
			"api_version": "v3",
			"operation":   "update_issue",
		},
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
	// Check read-only mode
	if h.provider.IsReadOnlyMode(ctx) {
		return NewToolError("Cannot delete issue: provider is in read-only mode"), nil
	}

	issueKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// First, get the issue to check project permissions
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Metadata != nil {
		if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
			// We need to fetch the issue first to check its project
			getURL := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s?fields=project", issueKey))
			getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
			if err == nil {
				email, token, _ := h.provider.extractAuthToken(ctx, params)
				getReq.Header.Set("Authorization", "Basic "+basicAuth(email, token))
				getReq.Header.Set("Accept", "application/json")

				if getResp, err := h.provider.httpClient.Do(getReq); err == nil {
					defer func() {
						_ = getResp.Body.Close()
					}()
					if getResp.StatusCode == http.StatusOK {
						var issueData map[string]interface{}
						if json.NewDecoder(getResp.Body).Decode(&issueData) == nil {
							if fields, ok := issueData["fields"].(map[string]interface{}); ok {
								if project, ok := fields["project"].(map[string]interface{}); ok {
									if projectKey, ok := project["key"].(string); ok {
										allowedProjects := strings.Split(projectFilter, ",")
										if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
											return NewToolError(fmt.Sprintf("Cannot delete issue in project %s: not in allowed projects (%s)", projectKey, projectFilter)), nil
										}
									}
								}
							}
						}
					}
				}
			}
		}
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
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete issue: %v", err)), nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response
	if resp.StatusCode != http.StatusNoContent {
		var errorBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errorBody)
		errorMsg := fmt.Sprintf("Failed to delete issue: status %d", resp.StatusCode)
		if msgs, ok := errorBody["errorMessages"].([]interface{}); ok && len(msgs) > 0 {
			errorMsg = fmt.Sprintf("%s - %v", errorMsg, msgs[0])
		}
		return NewToolError(errorMsg), nil
	}

	return NewToolResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s deleted successfully", issueKey),
		"_metadata": map[string]interface{}{
			"api_version": "v3",
			"operation":   "delete_issue",
		},
	}), nil
}
