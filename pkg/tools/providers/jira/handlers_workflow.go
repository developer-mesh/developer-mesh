package jira

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

// Workflow Handlers

// GetTransitionsHandler handles fetching available transitions for an issue
type GetTransitionsHandler struct {
	provider *JiraProvider
}

func NewGetTransitionsHandler(p *JiraProvider) *GetTransitionsHandler {
	return &GetTransitionsHandler{provider: p}
}

func (h *GetTransitionsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_transitions",
		Description: "Get available transitions for a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"expand": map[string]interface{}{
					"type":        "string",
					"description": "Use expand to include additional information about transitions in the response (e.g., 'transitions.fields')",
				},
				"transitionId": map[string]interface{}{
					"type":        "string",
					"description": "Return only transitions with specified ID (optional)",
				},
				"skipRemoteOnlyCondition": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether transitions with the 'RemoteOnlyCondition' are included in the response",
					"default":     false,
				},
				"includeUnavailableTransitions": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether details of transitions that fail a condition are returned",
					"default":     false,
				},
				"sortByOpsBarAndStatus": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the transitions are sorted by ops-bar sequence value first then category order",
					"default":     false,
				},
			},
			"required": []interface{}{"issueIdOrKey"},
		},
	}
}

func (h *GetTransitionsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Get issue ID or key
	issueIdOrKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueIdOrKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// Check project access if key is provided
	if strings.Contains(issueIdOrKey, "-") {
		// Extract project key from issue key
		parts := strings.Split(issueIdOrKey, "-")
		if len(parts) >= 1 {
			projectKey := parts[0]

			// Check project filter
			pctx, ok := providers.FromContext(ctx)
			if ok && pctx != nil && pctx.Metadata != nil {
				if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
					allowedProjects := strings.Split(projectFilter, ",")
					for i := range allowedProjects {
						allowedProjects[i] = strings.TrimSpace(allowedProjects[i])
					}
					if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
						return NewToolError(fmt.Sprintf("Issue belongs to project %s which is not in allowed projects: %s", projectKey, projectFilter)), nil
					}
				}
			}
		}
	}

	// Build query parameters
	queryParams := ""
	separator := "?"

	// Add expand parameter
	if expand, ok := params["expand"].(string); ok && expand != "" {
		queryParams += fmt.Sprintf("%sexpand=%s", separator, url.QueryEscape(expand))
		separator = "&"
	}

	// Add transitionId parameter
	if transitionId, ok := params["transitionId"].(string); ok && transitionId != "" {
		queryParams += fmt.Sprintf("%stransitionId=%s", separator, url.QueryEscape(transitionId))
		separator = "&"
	}

	// Add boolean parameters
	if skipRemoteOnlyCondition, ok := params["skipRemoteOnlyCondition"].(bool); ok && skipRemoteOnlyCondition {
		queryParams += fmt.Sprintf("%sskipRemoteOnlyCondition=true", separator)
		separator = "&"
	}

	if includeUnavailableTransitions, ok := params["includeUnavailableTransitions"].(bool); ok && includeUnavailableTransitions {
		queryParams += fmt.Sprintf("%sincludeUnavailableTransitions=true", separator)
		separator = "&"
	}

	if sortByOpsBarAndStatus, ok := params["sortByOpsBarAndStatus"].(bool); ok && sortByOpsBarAndStatus {
		queryParams += fmt.Sprintf("%ssortByOpsBarAndStatus=true", separator)
	}

	// Build request URL
	requestURL := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/transitions%s", issueIdOrKey, queryParams))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
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
		return NewToolError(fmt.Sprintf("Failed to get transitions: %v", err)), nil
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
		errorMsg := fmt.Sprintf("Failed to get transitions with status %d", resp.StatusCode)
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

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v3",
		"operation":   "get_transitions",
		"issue":       issueIdOrKey,
	}

	// Add transition count
	if transitions, ok := result["transitions"].([]interface{}); ok {
		result["_metadata"].(map[string]interface{})["transition_count"] = len(transitions)
	}

	return NewToolResult(result), nil
}

// TransitionIssueHandler handles executing a transition on an issue
type TransitionIssueHandler struct {
	provider *JiraProvider
}

func NewTransitionIssueHandler(p *JiraProvider) *TransitionIssueHandler {
	return &TransitionIssueHandler{provider: p}
}

func (h *TransitionIssueHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "transition_issue",
		Description: "Execute a transition on a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"transitionId": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the transition to execute",
				},
				"fields": map[string]interface{}{
					"type":        "object",
					"description": "Fields to update during the transition (e.g., resolution, assignee)",
					"properties": map[string]interface{}{
						"resolution": map[string]interface{}{
							"type":        "object",
							"description": "Resolution to set (for Done/Resolved transitions)",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "Resolution name (e.g., 'Done', 'Fixed', 'Won't Do')",
								},
								"id": map[string]interface{}{
									"type":        "string",
									"description": "Resolution ID",
								},
							},
						},
						"assignee": map[string]interface{}{
							"type":        "object",
							"description": "Assignee to set during transition",
							"properties": map[string]interface{}{
								"accountId": map[string]interface{}{
									"type":        "string",
									"description": "Account ID of the assignee",
								},
								"name": map[string]interface{}{
									"type":        "string",
									"description": "Username of the assignee (deprecated)",
								},
							},
						},
					},
				},
				"update": map[string]interface{}{
					"type":        "object",
					"description": "Fields to update using the edit operations (add, set, remove)",
				},
				"historyMetadata": map[string]interface{}{
					"type":        "object",
					"description": "Additional history metadata for the transition",
				},
				"properties": map[string]interface{}{
					"type":        "array",
					"description": "Entity properties to set",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"key": map[string]interface{}{
								"type": "string",
							},
							"value": map[string]interface{}{},
						},
					},
				},
			},
			"required": []interface{}{"issueIdOrKey", "transitionId"},
		},
	}
}

func (h *TransitionIssueHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if in read-only mode
	pctx, ok := providers.FromContext(ctx)
	if ok && pctx != nil && pctx.Metadata != nil {
		if readOnly, ok := pctx.Metadata["JIRA_READ_ONLY"].(bool); ok && readOnly {
			return NewToolError("Cannot transition issue in read-only mode"), nil
		}
	}

	// Get issue ID or key
	issueIdOrKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueIdOrKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	// Get transition ID
	transitionId, ok := params["transitionId"].(string)
	if !ok || transitionId == "" {
		return NewToolError("transitionId is required"), nil
	}

	// Validate transition ID format
	if err := h.validateTransitionId(transitionId); err != nil {
		return NewToolError(fmt.Sprintf("Invalid transitionId: %v", err)), nil
	}

	// Check project access if key is provided
	if strings.Contains(issueIdOrKey, "-") {
		// Extract project key from issue key
		parts := strings.Split(issueIdOrKey, "-")
		if len(parts) >= 1 {
			projectKey := parts[0]

			// Check project filter
			if pctx != nil && pctx.Metadata != nil {
				if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
					allowedProjects := strings.Split(projectFilter, ",")
					for i := range allowedProjects {
						allowedProjects[i] = strings.TrimSpace(allowedProjects[i])
					}
					if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
						return NewToolError(fmt.Sprintf("Issue belongs to project %s which is not in allowed projects: %s", projectKey, projectFilter)), nil
					}
				}
			}
		}
	}

	// Build transition request body
	transitionBody := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionId,
		},
	}

	// Add fields if provided
	if fields, ok := params["fields"].(map[string]interface{}); ok {
		transitionBody["fields"] = fields
	}

	// Add update operations if provided
	if update, ok := params["update"].(map[string]interface{}); ok {
		transitionBody["update"] = update
	}

	// Add history metadata if provided
	if historyMetadata, ok := params["historyMetadata"].(map[string]interface{}); ok {
		transitionBody["historyMetadata"] = historyMetadata
	}

	// Add properties if provided
	if properties, ok := params["properties"].([]interface{}); ok {
		transitionBody["properties"] = properties
	}

	// Marshal request body
	bodyJSON, err := json.Marshal(transitionBody)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request body: %v", err)), nil
	}

	// Build request URL
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueIdOrKey))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyJSON))
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
		return NewToolError(fmt.Sprintf("Failed to transition issue: %v", err)), nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Check response status (transitions typically return 204 No Content)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var errorResult map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errorResult)

		errorMsg := fmt.Sprintf("Failed to transition issue with status %d", resp.StatusCode)
		if errorMessages, ok := errorResult["errorMessages"].([]interface{}); ok && len(errorMessages) > 0 {
			errorMsg = fmt.Sprintf("%s: %v", errorMsg, errorMessages[0])
		}
		if errors, ok := errorResult["errors"].(map[string]interface{}); ok && len(errors) > 0 {
			errorDetails := []string{}
			for field, msg := range errors {
				errorDetails = append(errorDetails, fmt.Sprintf("%s: %v", field, msg))
			}
			errorMsg = fmt.Sprintf("%s - %s", errorMsg, strings.Join(errorDetails, ", "))
		}
		return NewToolError(errorMsg), nil
	}

	// Return success result (transitions typically don't return content)
	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s transitioned successfully using transition %s", issueIdOrKey, transitionId),
		"_metadata": map[string]interface{}{
			"api_version":   "v3",
			"operation":     "transition_issue",
			"issue":         issueIdOrKey,
			"transition_id": transitionId,
		},
	}

	return NewToolResult(result), nil
}

// validateTransitionId validates the transition ID format
func (h *TransitionIssueHandler) validateTransitionId(transitionId string) error {
	if transitionId == "" {
		return fmt.Errorf("transition ID cannot be empty")
	}

	// Transition IDs are typically numeric strings but can also be names in some contexts
	// Basic validation to prevent obvious injection attempts
	if len(transitionId) > 50 {
		return fmt.Errorf("transition ID too long (max 50 characters)")
	}

	// Check for dangerous characters that might indicate injection
	dangerousChars := []string{"<", ">", "\"", "'", "&", ";", "|", "`"}
	for _, char := range dangerousChars {
		if strings.Contains(transitionId, char) {
			return fmt.Errorf("transition ID contains invalid character: %s", char)
		}
	}

	return nil
}

// GetWorkflowsHandler handles fetching workflows for a project (optional advanced feature)
type GetWorkflowsHandler struct {
	provider *JiraProvider
}

func NewGetWorkflowsHandler(p *JiraProvider) *GetWorkflowsHandler {
	return &GetWorkflowsHandler{provider: p}
}

func (h *GetWorkflowsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_workflows",
		Description: "Get workflows for projects (requires admin permissions)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"projectKey": map[string]interface{}{
					"type":        "string",
					"description": "Project key to get workflows for (optional, gets all if not specified)",
				},
				"workflowName": map[string]interface{}{
					"type":        "string",
					"description": "Name of specific workflow to retrieve",
				},
				"expand": map[string]interface{}{
					"type":        "string",
					"description": "Use expand to include additional information (e.g., 'transitions', 'transitions.rules')",
				},
			},
			"required": []interface{}{},
		},
	}
}

func (h *GetWorkflowsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Build query parameters
	queryParams := ""
	separator := "?"

	// Add project key if provided
	if projectKey, ok := params["projectKey"].(string); ok && projectKey != "" {
		// Check project filter
		pctx, ok := providers.FromContext(ctx)
		if ok && pctx != nil && pctx.Metadata != nil {
			if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
				allowedProjects := strings.Split(projectFilter, ",")
				for i := range allowedProjects {
					allowedProjects[i] = strings.TrimSpace(allowedProjects[i])
				}
				if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
					return NewToolError(fmt.Sprintf("Project %s is not in allowed projects: %s", projectKey, projectFilter)), nil
				}
			}
		}

		queryParams += fmt.Sprintf("%sprojectKeys=%s", separator, url.QueryEscape(projectKey))
		separator = "&"
	}

	// Add workflow name if provided
	if workflowName, ok := params["workflowName"].(string); ok && workflowName != "" {
		queryParams += fmt.Sprintf("%sworkflowNames=%s", separator, url.QueryEscape(workflowName))
		separator = "&"
	}

	// Add expand parameter
	if expand, ok := params["expand"].(string); ok && expand != "" {
		queryParams += fmt.Sprintf("%sexpand=%s", separator, url.QueryEscape(expand))
	}

	// Build request URL
	requestURL := h.provider.buildURL(fmt.Sprintf("/rest/api/3/workflow/search%s", queryParams))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
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
		return NewToolError(fmt.Sprintf("Failed to get workflows: %v", err)), nil
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
		errorMsg := fmt.Sprintf("Failed to get workflows with status %d", resp.StatusCode)
		if errorMessages, ok := result["errorMessages"].([]interface{}); ok && len(errorMessages) > 0 {
			errorMsg = fmt.Sprintf("%s: %v", errorMsg, errorMessages[0])
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v3",
		"operation":   "get_workflows",
	}

	if projectKey, ok := params["projectKey"].(string); ok && projectKey != "" {
		result["_metadata"].(map[string]interface{})["project_key"] = projectKey
	}

	return NewToolResult(result), nil
}

// AddWorkflowCommentHandler handles adding comments during transitions
type AddWorkflowCommentHandler struct {
	provider *JiraProvider
}

func NewAddWorkflowCommentHandler(p *JiraProvider) *AddWorkflowCommentHandler {
	return &AddWorkflowCommentHandler{provider: p}
}

func (h *AddWorkflowCommentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "transition_with_comment",
		Description: "Execute a transition on an issue with a comment",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"transitionId": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the transition to execute",
				},
				"comment": map[string]interface{}{
					"type":        "string",
					"description": "Comment to add during the transition (plain text)",
				},
				"commentADF": map[string]interface{}{
					"type":        "object",
					"description": "Comment in ADF format for rich text",
				},
				"commentVisibility": map[string]interface{}{
					"type":        "object",
					"description": "Visibility restriction for the comment",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"group", "role"},
						},
						"value": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"fields": map[string]interface{}{
					"type":        "object",
					"description": "Fields to update during the transition",
				},
				"resolution": map[string]interface{}{
					"type":        "string",
					"description": "Resolution name to set (shorthand for fields.resolution)",
				},
			},
			"required": []interface{}{"issueIdOrKey", "transitionId"},
		},
	}
}

func (h *AddWorkflowCommentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if in read-only mode
	pctx, ok := providers.FromContext(ctx)
	if ok && pctx != nil && pctx.Metadata != nil {
		if readOnly, ok := pctx.Metadata["JIRA_READ_ONLY"].(bool); ok && readOnly {
			return NewToolError("Cannot transition issue in read-only mode"), nil
		}
	}

	// Get required parameters
	issueIdOrKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueIdOrKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	transitionId, ok := params["transitionId"].(string)
	if !ok || transitionId == "" {
		return NewToolError("transitionId is required"), nil
	}

	// Check project access
	if strings.Contains(issueIdOrKey, "-") {
		parts := strings.Split(issueIdOrKey, "-")
		if len(parts) >= 1 {
			projectKey := parts[0]
			if pctx != nil && pctx.Metadata != nil {
				if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" && projectFilter != "*" {
					allowedProjects := strings.Split(projectFilter, ",")
					for i := range allowedProjects {
						allowedProjects[i] = strings.TrimSpace(allowedProjects[i])
					}
					if !h.provider.isProjectAllowed(projectKey, allowedProjects) {
						return NewToolError(fmt.Sprintf("Issue belongs to project %s which is not in allowed projects: %s", projectKey, projectFilter)), nil
					}
				}
			}
		}
	}

	// Build transition request
	transitionBody := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionId,
		},
	}

	// Add update section for comment
	updateSection := make(map[string]interface{})

	// Handle comment
	if commentText, ok := params["comment"].(string); ok && commentText != "" {
		// Convert to ADF
		commentADF := h.convertTextToADF(commentText)
		comment := map[string]interface{}{
			"add": map[string]interface{}{
				"body": commentADF,
			},
		}

		// Add visibility if provided
		if visibility, ok := params["commentVisibility"].(map[string]interface{}); ok {
			comment["add"].(map[string]interface{})["visibility"] = visibility
		}

		updateSection["comment"] = []interface{}{comment}
	} else if commentADF, ok := params["commentADF"].(map[string]interface{}); ok {
		comment := map[string]interface{}{
			"add": map[string]interface{}{
				"body": commentADF,
			},
		}

		// Add visibility if provided
		if visibility, ok := params["commentVisibility"].(map[string]interface{}); ok {
			comment["add"].(map[string]interface{})["visibility"] = visibility
		}

		updateSection["comment"] = []interface{}{comment}
	}

	if len(updateSection) > 0 {
		transitionBody["update"] = updateSection
	}

	// Add fields if provided
	if fields, ok := params["fields"].(map[string]interface{}); ok {
		transitionBody["fields"] = fields
	} else if resolution, ok := params["resolution"].(string); ok && resolution != "" {
		// Shorthand for resolution
		transitionBody["fields"] = map[string]interface{}{
			"resolution": map[string]interface{}{
				"name": resolution,
			},
		}
	}

	// Marshal and execute the request
	bodyJSON, err := json.Marshal(transitionBody)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request body: %v", err)), nil
	}

	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueIdOrKey))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	email, token, err := h.provider.extractAuthToken(ctx, params)
	if err != nil {
		return NewToolError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(email, token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.provider.httpClient.Do(req)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to transition issue: %v", err)), nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.provider.GetLogger().Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var errorResult map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errorResult)

		errorMsg := fmt.Sprintf("Failed to transition issue with status %d", resp.StatusCode)
		if errorMessages, ok := errorResult["errorMessages"].([]interface{}); ok && len(errorMessages) > 0 {
			errorMsg = fmt.Sprintf("%s: %v", errorMsg, errorMessages[0])
		}
		return NewToolError(errorMsg), nil
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Issue %s transitioned successfully with comment", issueIdOrKey),
		"_metadata": map[string]interface{}{
			"api_version":   "v3",
			"operation":     "transition_with_comment",
			"issue":         issueIdOrKey,
			"transition_id": transitionId,
		},
	}

	return NewToolResult(result), nil
}

// convertTextToADF converts plain text to Atlassian Document Format
func (h *AddWorkflowCommentHandler) convertTextToADF(text string) map[string]interface{} {
	// Split text into paragraphs
	paragraphs := strings.Split(text, "\n\n")
	content := make([]interface{}, 0, len(paragraphs))

	for _, para := range paragraphs {
		if para = strings.TrimSpace(para); para != "" {
			// Create paragraph node
			paragraphNode := map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": para,
					},
				},
			}
			content = append(content, paragraphNode)
		}
	}

	// If no content, add empty paragraph
	if len(content) == 0 {
		content = append(content, map[string]interface{}{
			"type":    "paragraph",
			"content": []interface{}{},
		})
	}

	// Return ADF document
	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}
