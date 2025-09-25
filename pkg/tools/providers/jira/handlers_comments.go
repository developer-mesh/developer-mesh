package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// Comment Handlers

// GetCommentsHandler handles fetching comments for an issue
type GetCommentsHandler struct {
	provider *JiraProvider
}

func NewGetCommentsHandler(p *JiraProvider) *GetCommentsHandler {
	return &GetCommentsHandler{provider: p}
}

func (h *GetCommentsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_comments",
		Description: "Get comments for a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"startAt": map[string]interface{}{
					"type":        "integer",
					"description": "The starting index of the returned comments",
					"default":     0,
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"description": "The maximum number of comments to return",
					"default":     50,
					"maximum":     100,
				},
				"orderBy": map[string]interface{}{
					"type":        "string",
					"description": "Order the comments by created date ('created' or '-created')",
					"default":     "-created",
					"enum":        []interface{}{"created", "-created"},
				},
				"expand": map[string]interface{}{
					"type":        "string",
					"description": "Fields to expand (e.g., 'renderedBody')",
				},
			},
			"required": []interface{}{"issueIdOrKey"},
		},
	}
}

func (h *GetCommentsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
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

	// Pagination parameters
	if startAt, ok := params["startAt"].(float64); ok && startAt >= 0 {
		queryParams += fmt.Sprintf("%sstartAt=%d", separator, int(startAt))
		separator = "&"
	}

	maxResults := 50
	if max, ok := params["maxResults"].(float64); ok && max > 0 {
		if max > 100 {
			maxResults = 100
		} else {
			maxResults = int(max)
		}
	}
	queryParams += fmt.Sprintf("%smaxResults=%d", separator, maxResults)
	separator = "&"

	// Order by parameter
	if orderBy, ok := params["orderBy"].(string); ok && orderBy != "" {
		queryParams += fmt.Sprintf("%sorderBy=%s", separator, orderBy)
		separator = "&"
	} else {
		queryParams += fmt.Sprintf("%sorderBy=-created", separator) // Default to newest first
		separator = "&"
	}

	// Expand parameter
	if expand, ok := params["expand"].(string); ok && expand != "" {
		queryParams += fmt.Sprintf("%sexpand=%s", separator, expand)
	}

	// Build request URL
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/comment%s", issueIdOrKey, queryParams))

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
		return NewToolError(fmt.Sprintf("Failed to get comments: %v", err)), nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to get comments with status %d", resp.StatusCode)
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
		"operation":   "get_comments",
		"issue":       issueIdOrKey,
	}

	// Add pagination info
	if startAtVal, ok := result["startAt"].(float64); ok {
		if maxResultsVal, ok := result["maxResults"].(float64); ok {
			if total, ok := result["total"].(float64); ok {
				nextStart := startAtVal + maxResultsVal
				if nextStart < total {
					result["_metadata"].(map[string]interface{})["nextStartAt"] = nextStart
					result["_metadata"].(map[string]interface{})["hasMore"] = true
				} else {
					result["_metadata"].(map[string]interface{})["hasMore"] = false
				}
			}
		}
	}

	return NewToolResult(result), nil
}

// AddCommentHandler handles adding a comment to an issue
type AddCommentHandler struct {
	provider *JiraProvider
}

func NewAddCommentHandler(p *JiraProvider) *AddCommentHandler {
	return &AddCommentHandler{provider: p}
}

func (h *AddCommentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "add_comment",
		Description: "Add a comment to a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Plain text comment body (will be converted to ADF)",
				},
				"bodyADF": map[string]interface{}{
					"type":        "object",
					"description": "Rich text comment in Atlassian Document Format (ADF)",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":    "string",
							"default": "doc",
						},
						"version": map[string]interface{}{
							"type":    "integer",
							"default": 1,
						},
						"content": map[string]interface{}{
							"type":        "array",
							"description": "ADF content nodes",
						},
					},
				},
				"visibility": map[string]interface{}{
					"type":        "object",
					"description": "Comment visibility restriction",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Visibility type (group or role)",
							"enum":        []interface{}{"group", "role"},
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "The name of the group or role",
						},
						"identifier": map[string]interface{}{
							"type":        "string",
							"description": "The ID of the group or role (optional)",
						},
					},
				},
				"properties": map[string]interface{}{
					"type":        "array",
					"description": "Entity properties to set on the comment",
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
			"required": []interface{}{"issueIdOrKey"},
		},
	}
}

func (h *AddCommentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if in read-only mode
	pctx, ok := providers.FromContext(ctx)
	if ok && pctx != nil && pctx.Metadata != nil {
		if readOnly, ok := pctx.Metadata["JIRA_READ_ONLY"].(bool); ok && readOnly {
			return NewToolError("Cannot add comment in read-only mode"), nil
		}
	}

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

	// Build comment body
	commentBody := make(map[string]interface{})

	// Handle body content - prefer ADF if provided, otherwise convert plain text
	if bodyADF, ok := params["bodyADF"].(map[string]interface{}); ok {
		// Use provided ADF directly
		commentBody["body"] = bodyADF
	} else if bodyText, ok := params["body"].(string); ok && bodyText != "" {
		// Convert plain text to ADF
		commentBody["body"] = h.convertTextToADF(bodyText)
	} else {
		return NewToolError("Either 'body' (plain text) or 'bodyADF' (rich text) is required"), nil
	}

	// Add visibility if provided
	if visibility, ok := params["visibility"].(map[string]interface{}); ok {
		commentBody["visibility"] = visibility
	}

	// Add properties if provided
	if properties, ok := params["properties"].([]interface{}); ok {
		commentBody["properties"] = properties
	}

	// Marshal request body
	bodyJSON, err := json.Marshal(commentBody)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request body: %v", err)), nil
	}

	// Build request URL
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/comment", issueIdOrKey))

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
		return NewToolError(fmt.Sprintf("Failed to add comment: %v", err)), nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to add comment with status %d", resp.StatusCode)
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
		"operation":   "add_comment",
		"issue":       issueIdOrKey,
	}

	return NewToolResult(result), nil
}

// convertTextToADF converts plain text to Atlassian Document Format
func (h *AddCommentHandler) convertTextToADF(text string) map[string]interface{} {
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

// UpdateCommentHandler handles updating an existing comment
type UpdateCommentHandler struct {
	provider *JiraProvider
}

func NewUpdateCommentHandler(p *JiraProvider) *UpdateCommentHandler {
	return &UpdateCommentHandler{provider: p}
}

func (h *UpdateCommentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_comment",
		Description: "Update an existing comment on a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"commentId": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the comment to update",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Plain text comment body (will be converted to ADF)",
				},
				"bodyADF": map[string]interface{}{
					"type":        "object",
					"description": "Rich text comment in Atlassian Document Format (ADF)",
				},
				"visibility": map[string]interface{}{
					"type":        "object",
					"description": "Comment visibility restriction",
				},
				"notifyUsers": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to notify users about the comment update",
					"default":     true,
				},
			},
			"required": []interface{}{"issueIdOrKey", "commentId"},
		},
	}
}

func (h *UpdateCommentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if in read-only mode
	pctx, ok := providers.FromContext(ctx)
	if ok && pctx != nil && pctx.Metadata != nil {
		if readOnly, ok := pctx.Metadata["JIRA_READ_ONLY"].(bool); ok && readOnly {
			return NewToolError("Cannot update comment in read-only mode"), nil
		}
	}

	// Get parameters
	issueIdOrKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueIdOrKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	commentId, ok := params["commentId"].(string)
	if !ok || commentId == "" {
		return NewToolError("commentId is required"), nil
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

	// Build update body
	updateBody := make(map[string]interface{})

	// Handle body content
	if bodyADF, ok := params["bodyADF"].(map[string]interface{}); ok {
		updateBody["body"] = bodyADF
	} else if bodyText, ok := params["body"].(string); ok && bodyText != "" {
		updateBody["body"] = h.convertTextToADF(bodyText)
	} else {
		return NewToolError("Either 'body' (plain text) or 'bodyADF' (rich text) is required"), nil
	}

	// Add visibility if provided
	if visibility, ok := params["visibility"].(map[string]interface{}); ok {
		updateBody["visibility"] = visibility
	}

	// Marshal request body
	bodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to marshal request body: %v", err)), nil
	}

	// Build query params
	queryParams := ""
	if notifyUsers, ok := params["notifyUsers"].(bool); ok && !notifyUsers {
		queryParams = "?notifyUsers=false"
	}

	// Build request URL
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/comment/%s%s", issueIdOrKey, commentId, queryParams))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(bodyJSON))
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
		return NewToolError(fmt.Sprintf("Failed to update comment: %v", err)), nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NewToolError(fmt.Sprintf("Failed to parse response: %v", err)), nil
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to update comment with status %d", resp.StatusCode)
		if errorMessages, ok := result["errorMessages"].([]interface{}); ok && len(errorMessages) > 0 {
			errorMsg = fmt.Sprintf("%s: %v", errorMsg, errorMessages[0])
		}
		return NewToolError(errorMsg), nil
	}

	// Add metadata
	result["_metadata"] = map[string]interface{}{
		"api_version": "v3",
		"operation":   "update_comment",
		"issue":       issueIdOrKey,
		"comment_id":  commentId,
	}

	return NewToolResult(result), nil
}

// convertTextToADF converts plain text to Atlassian Document Format
func (h *UpdateCommentHandler) convertTextToADF(text string) map[string]interface{} {
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

// DeleteCommentHandler handles deleting a comment
type DeleteCommentHandler struct {
	provider *JiraProvider
}

func NewDeleteCommentHandler(p *JiraProvider) *DeleteCommentHandler {
	return &DeleteCommentHandler{provider: p}
}

func (h *DeleteCommentHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_comment",
		Description: "Delete a comment from a Jira issue",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issueIdOrKey": map[string]interface{}{
					"type":        "string",
					"description": "The ID or key of the issue",
				},
				"commentId": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the comment to delete",
				},
			},
			"required": []interface{}{"issueIdOrKey", "commentId"},
		},
	}
}

func (h *DeleteCommentHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Check if in read-only mode
	pctx, ok := providers.FromContext(ctx)
	if ok && pctx != nil && pctx.Metadata != nil {
		if readOnly, ok := pctx.Metadata["JIRA_READ_ONLY"].(bool); ok && readOnly {
			return NewToolError("Cannot delete comment in read-only mode"), nil
		}
	}

	// Get parameters
	issueIdOrKey, ok := params["issueIdOrKey"].(string)
	if !ok || issueIdOrKey == "" {
		return NewToolError("issueIdOrKey is required"), nil
	}

	commentId, ok := params["commentId"].(string)
	if !ok || commentId == "" {
		return NewToolError("commentId is required"), nil
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

	// Build request URL
	url := h.provider.buildURL(fmt.Sprintf("/rest/api/3/issue/%s/comment/%s", issueIdOrKey, commentId))

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
		return NewToolError(fmt.Sprintf("Failed to delete comment: %v", err)), nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&result)

		errorMsg := fmt.Sprintf("Failed to delete comment with status %d", resp.StatusCode)
		if errorMessages, ok := result["errorMessages"].([]interface{}); ok && len(errorMessages) > 0 {
			errorMsg = fmt.Sprintf("%s: %v", errorMsg, errorMessages[0])
		}
		return NewToolError(errorMsg), nil
	}

	// Return success result
	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Comment %s deleted successfully from issue %s", commentId, issueIdOrKey),
		"_metadata": map[string]interface{}{
			"api_version": "v3",
			"operation":   "delete_comment",
			"issue":       issueIdOrKey,
			"comment_id":  commentId,
		},
	}

	return NewToolResult(result), nil
}
