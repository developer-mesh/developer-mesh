package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-github/v74/github"
)

// Collaboration Operations - Notifications, Gists, Stars, Watching

// ListNotificationsHandler handles listing notifications
type ListNotificationsHandler struct {
	provider *GitHubProvider
}

func NewListNotificationsHandler(p *GitHubProvider) *ListNotificationsHandler {
	return &ListNotificationsHandler{provider: p}
}

func (h *ListNotificationsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_notifications",
		Description: "List notifications for the authenticated user",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "Show all notifications (including read)",
				},
				"participating": map[string]interface{}{
					"type":        "boolean",
					"description": "Show only notifications where user is participating",
				},
				"since": map[string]interface{}{
					"type":        "string",
					"description": "Show notifications updated after this time (ISO 8601 format)",
				},
				"before": map[string]interface{}{
					"type":        "string",
					"description": "Show notifications updated before this time (ISO 8601 format)",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
		},
	}
}

func (h *ListNotificationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	pagination := ExtractPagination(params)
	opts := &github.NotificationListOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if all, ok := params["all"].(bool); ok {
		opts.All = all
	}
	if participating, ok := params["participating"].(bool); ok {
		opts.Participating = participating
	}
	if since := extractString(params, "since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}
	if before := extractString(params, "before"); before != "" {
		if t, err := time.Parse(time.RFC3339, before); err == nil {
			opts.Before = t
		}
	}

	notifications, _, err := client.Activity.ListNotifications(ctx, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list notifications: %v", err)), nil
	}

	data, _ := json.Marshal(notifications)
	return NewToolResult(string(data)), nil
}

// MarkNotificationAsReadHandler handles marking a notification as read
type MarkNotificationAsReadHandler struct {
	provider *GitHubProvider
}

func NewMarkNotificationAsReadHandler(p *GitHubProvider) *MarkNotificationAsReadHandler {
	return &MarkNotificationAsReadHandler{provider: p}
}

func (h *MarkNotificationAsReadHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "mark_notification_as_read",
		Description: "Mark a notification as read",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"thread_id": map[string]interface{}{
					"type":        "string",
					"description": "Thread ID of the notification",
				},
			},
			"required": []interface{}{"thread_id"},
		},
	}
}

func (h *MarkNotificationAsReadHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	threadID := extractString(params, "thread_id")

	_, err := client.Activity.MarkThreadRead(ctx, threadID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to mark notification as read: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status":    "marked_as_read",
		"thread_id": threadID,
	}), nil
}

// ListGistsHandler handles listing gists
type ListGistsHandler struct {
	provider *GitHubProvider
}

func NewListGistsHandler(p *GitHubProvider) *ListGistsHandler {
	return &ListGistsHandler{provider: p}
}

func (h *ListGistsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_gists",
		Description: "List gists for a user or all public gists",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Username to list gists for (omit for authenticated user)",
				},
				"since": map[string]interface{}{
					"type":        "string",
					"description": "Show gists updated after this time (ISO 8601 format)",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
		},
	}
}

func (h *ListGistsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	pagination := ExtractPagination(params)
	opts := &github.GistListOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if since := extractString(params, "since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}

	var gists []*github.Gist
	var err error

	username := extractString(params, "username")
	if username != "" {
		gists, _, err = client.Gists.List(ctx, username, opts)
	} else {
		gists, _, err = client.Gists.List(ctx, "", opts)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list gists: %v", err)), nil
	}

	data, _ := json.Marshal(gists)
	return NewToolResult(string(data)), nil
}

// GetGistHandler handles getting a specific gist
type GetGistHandler struct {
	provider *GitHubProvider
}

func NewGetGistHandler(p *GitHubProvider) *GetGistHandler {
	return &GetGistHandler{provider: p}
}

func (h *GetGistHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_gist",
		Description: "Get a specific gist",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Gist ID",
				},
			},
			"required": []interface{}{"gist_id"},
		},
	}
}

func (h *GetGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	gistID := extractString(params, "gist_id")

	gist, _, err := client.Gists.Get(ctx, gistID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get gist: %v", err)), nil
	}

	data, _ := json.Marshal(gist)
	return NewToolResult(string(data)), nil
}

// CreateGistHandler handles creating a new gist
type CreateGistHandler struct {
	provider *GitHubProvider
}

func NewCreateGistHandler(p *GitHubProvider) *CreateGistHandler {
	return &CreateGistHandler{provider: p}
}

func (h *CreateGistHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_gist",
		Description: "Create a new gist",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Gist description",
				},
				"public": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the gist should be public",
				},
				"files": map[string]interface{}{
					"type":        "object",
					"description": "Map of filename to file content",
					"additionalProperties": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "File content",
							},
						},
					},
				},
			},
			"required": []interface{}{"files"},
		},
	}
}

func (h *CreateGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	gist := &github.Gist{}

	if desc := extractString(params, "description"); desc != "" {
		gist.Description = &desc
	}

	if public, ok := params["public"].(bool); ok {
		gist.Public = &public
	}

	// Parse files
	if filesRaw, ok := params["files"].(map[string]interface{}); ok {
		gist.Files = make(map[github.GistFilename]github.GistFile)
		for filename, fileData := range filesRaw {
			if fileMap, ok := fileData.(map[string]interface{}); ok {
				if content, ok := fileMap["content"].(string); ok {
					gist.Files[github.GistFilename(filename)] = github.GistFile{
						Content: &content,
					}
				}
			}
		}
	}

	created, _, err := client.Gists.Create(ctx, gist)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create gist: %v", err)), nil
	}

	data, _ := json.Marshal(created)
	return NewToolResult(string(data)), nil
}

// UpdateGistHandler handles updating a gist
type UpdateGistHandler struct {
	provider *GitHubProvider
}

func NewUpdateGistHandler(p *GitHubProvider) *UpdateGistHandler {
	return &UpdateGistHandler{provider: p}
}

func (h *UpdateGistHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_gist",
		Description: "Update an existing gist",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Gist ID",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "New gist description",
				},
				"files": map[string]interface{}{
					"type":        "object",
					"description": "Map of filename to file content (set content to null to delete)",
					"additionalProperties": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "File content (null to delete)",
							},
							"filename": map[string]interface{}{
								"type":        "string",
								"description": "New filename (for renaming)",
							},
						},
					},
				},
			},
			"required": []interface{}{"gist_id"},
		},
	}
}

func (h *UpdateGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	gistID := extractString(params, "gist_id")
	gist := &github.Gist{}

	if desc := extractString(params, "description"); desc != "" {
		gist.Description = &desc
	}

	// Parse files
	if filesRaw, ok := params["files"].(map[string]interface{}); ok {
		gist.Files = make(map[github.GistFilename]github.GistFile)
		for filename, fileData := range filesRaw {
			gistFile := github.GistFile{}
			if fileMap, ok := fileData.(map[string]interface{}); ok {
				if content, ok := fileMap["content"].(string); ok {
					gistFile.Content = &content
				}
				if newFilename, ok := fileMap["filename"].(string); ok {
					gistFile.Filename = &newFilename
				}
			}
			gist.Files[github.GistFilename(filename)] = gistFile
		}
	}

	updated, _, err := client.Gists.Edit(ctx, gistID, gist)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update gist: %v", err)), nil
	}

	data, _ := json.Marshal(updated)
	return NewToolResult(string(data)), nil
}

// DeleteGistHandler handles deleting a gist
type DeleteGistHandler struct {
	provider *GitHubProvider
}

func NewDeleteGistHandler(p *GitHubProvider) *DeleteGistHandler {
	return &DeleteGistHandler{provider: p}
}

func (h *DeleteGistHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_gist",
		Description: "Delete a gist",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Gist ID",
				},
			},
			"required": []interface{}{"gist_id"},
		},
	}
}

func (h *DeleteGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	gistID := extractString(params, "gist_id")

	_, err := client.Gists.Delete(ctx, gistID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete gist: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status":  "deleted",
		"gist_id": gistID,
	}), nil
}

// StarGistHandler handles starring a gist
type StarGistHandler struct {
	provider *GitHubProvider
}

func NewStarGistHandler(p *GitHubProvider) *StarGistHandler {
	return &StarGistHandler{provider: p}
}

func (h *StarGistHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "star_gist",
		Description: "Star a gist",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Gist ID",
				},
			},
			"required": []interface{}{"gist_id"},
		},
	}
}

func (h *StarGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	gistID := extractString(params, "gist_id")

	_, err := client.Gists.Star(ctx, gistID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to star gist: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status":  "starred",
		"gist_id": gistID,
	}), nil
}

// UnstarGistHandler handles unstarring a gist
type UnstarGistHandler struct {
	provider *GitHubProvider
}

func NewUnstarGistHandler(p *GitHubProvider) *UnstarGistHandler {
	return &UnstarGistHandler{provider: p}
}

func (h *UnstarGistHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "unstar_gist",
		Description: "Unstar a gist",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Gist ID",
				},
			},
			"required": []interface{}{"gist_id"},
		},
	}
}

func (h *UnstarGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	gistID := extractString(params, "gist_id")

	_, err := client.Gists.Unstar(ctx, gistID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to unstar gist: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status":  "unstarred",
		"gist_id": gistID,
	}), nil
}

// WatchRepositoryHandler handles watching a repository
type WatchRepositoryHandler struct {
	provider *GitHubProvider
}

func NewWatchRepositoryHandler(p *GitHubProvider) *WatchRepositoryHandler {
	return &WatchRepositoryHandler{provider: p}
}

func (h *WatchRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "watch_repository",
		Description: "Watch a repository for notifications",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"subscribed": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to subscribe to notifications",
				},
				"ignored": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to ignore notifications",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *WatchRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	sub := &github.Subscription{}
	if subscribed, ok := params["subscribed"].(bool); ok {
		sub.Subscribed = &subscribed
	}
	if ignored, ok := params["ignored"].(bool); ok {
		sub.Ignored = &ignored
	}

	subscription, _, err := client.Activity.SetRepositorySubscription(ctx, owner, repo, sub)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to watch repository: %v", err)), nil
	}

	data, _ := json.Marshal(subscription)
	return NewToolResult(string(data)), nil
}

// UnwatchRepositoryHandler handles unwatching a repository
type UnwatchRepositoryHandler struct {
	provider *GitHubProvider
}

func NewUnwatchRepositoryHandler(p *GitHubProvider) *UnwatchRepositoryHandler {
	return &UnwatchRepositoryHandler{provider: p}
}

func (h *UnwatchRepositoryHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "unwatch_repository",
		Description: "Unwatch a repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *UnwatchRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	_, err := client.Activity.DeleteRepositorySubscription(ctx, owner, repo)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to unwatch repository: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status": "unwatched",
		"owner":  owner,
		"repo":   repo,
	}), nil
}
