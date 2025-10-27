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
		Description: "List notifications (repo activity, mentions, updates, state). Use when: checking inbox, finding unread items, tracking repo activity.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "Show all notifications including already read ones",
					"default":     false,
				},
				"participating": map[string]interface{}{
					"type":        "boolean",
					"description": "Show only notifications where you're directly participating or mentioned",
					"default":     false,
				},
				"since": map[string]interface{}{
					"type":        "string",
					"description": "Show notifications updated after this time (ISO 8601 format)",
					"format":      "date-time",
					"example":     "2024-01-01T00:00:00Z",
				},
				"before": map[string]interface{}{
					"type":        "string",
					"description": "Show notifications updated before this time (ISO 8601 format)",
					"format":      "date-time",
					"example":     "2024-12-31T23:59:59Z",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"notifications"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"notifications": []interface{}{
				map[string]interface{}{
					"id":         "1",
					"repository": map[string]interface{}{"full_name": "octocat/Hello-World"},
					"subject": map[string]interface{}{
						"title": "Greetings",
						"type":  "Issue",
					},
					"reason":     "subscribed",
					"unread":     true,
					"updated_at": "2024-01-01T00:00:00Z",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "401 Unauthorized",
				"cause":    "Invalid or missing authentication",
				"solution": "Ensure valid authentication token with notifications scope",
			},
		},
		ExtendedHelp: `Lists GitHub notifications for activity you're involved in.

Notification reasons:
- subscribed: Watching a repository
- manual: Manually subscribed to thread
- author: You created the thread
- comment: You commented on the thread
- mention: You were @mentioned
- team_mention: Your team was mentioned
- state_change: You changed the thread state`,
	}
}

func (h *ListNotificationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Mark a specific GitHub notification thread as read.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"thread_id": map[string]interface{}{
					"type":        "string",
					"description": "Thread ID of the notification to mark as read",
					"example":     "1234567890",
					"pattern":     "^[0-9]+$",
				},
			},
			"required": []interface{}{"thread_id"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"notifications"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion":  "2022-11-28",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"status":    "marked_as_read",
			"thread_id": "1234567890",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Thread ID not found",
				"solution": "Verify the thread ID exists and you have access",
			},
		},
		ExtendedHelp: `Marks a notification thread as read. This removes the notification from your unread list.

Use this after you've reviewed a notification to clear it from your inbox.`,
	}
}

func (h *MarkNotificationAsReadHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "List gists (description, files, visibility, created date). Use when: browsing code snippets, finding shared code, checking public gists.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Username to list gists for. Leave empty for authenticated user's gists.",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"since": map[string]interface{}{
					"type":        "string",
					"description": "Show gists updated after this time (ISO 8601 format)",
					"format":      "date-time",
					"example":     "2024-01-01T00:00:00Z",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number for pagination",
					"default":     1,
					"minimum":     1,
				},
			},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"gist"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"gists": []interface{}{
				map[string]interface{}{
					"id":          "aa5a315d61ae9438b18d",
					"description": "Hello World Examples",
					"public":      true,
					"files": map[string]interface{}{
						"hello_world.rb": map[string]interface{}{
							"filename": "hello_world.rb",
							"type":     "application/x-ruby",
							"language": "Ruby",
							"size":     167,
						},
					},
					"created_at": "2010-04-14T02:15:15Z",
					"updated_at": "2011-06-20T11:34:15Z",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "User not found",
				"solution": "Verify the username exists",
			},
		},
		ExtendedHelp: `Lists GitHub Gists - shareable code snippets.

Gists can be:
- Public: Discoverable by anyone
- Secret: Only accessible via direct URL

Use cases:
- Share code snippets
- Create embeddable code examples
- Store configuration files
- Quick code backup`,
	}
}

func (h *ListGistsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get gist details (files, content, revisions, comments). Use when: viewing snippet, checking version history, reading shared code.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the gist to retrieve",
					"example":     "aa5a315d61ae9438b18d",
					"pattern":     "^[a-f0-9]+$",
					"minLength":   1,
					"maxLength":   32,
				},
			},
			"required": []interface{}{"gist_id"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
		},
		ResponseExample: map[string]interface{}{
			"id":          "aa5a315d61ae9438b18d",
			"description": "Hello World Examples",
			"public":      true,
			"files": map[string]interface{}{
				"hello_world.rb": map[string]interface{}{
					"filename": "hello_world.rb",
					"type":     "application/x-ruby",
					"language": "Ruby",
					"raw_url":  "https://gist.githubusercontent.com/raw/...",
					"size":     167,
					"content":  "class HelloWorld\n  def initialize(name)\n    @name = name\n  end\nend",
				},
			},
			"owner": map[string]interface{}{
				"login": "octocat",
				"id":    1,
			},
			"forks":    []interface{}{},
			"history":  []interface{}{},
			"comments": 0,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Gist not found or private",
				"solution": "Verify the gist ID and ensure you have access",
			},
		},
		ExtendedHelp: `Retrieves complete details about a gist including all file contents.

The response includes:
- All files with their content
- Fork and revision history
- Comments count
- Owner information

Use this to read gist contents or check metadata.`,
	}
}

func (h *GetGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Create gist with files (public/private). Use when: sharing code snippet, creating example, documenting solution.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the gist",
					"example":     "Example of hello world in multiple languages",
					"maxLength":   256,
				},
				"public": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the gist should be public (true) or secret (false)",
					"default":     false,
				},
				"files": map[string]interface{}{
					"type":        "object",
					"description": "Map of filename to file content. At least one file is required.",
					"example": map[string]interface{}{
						"hello.py": map[string]interface{}{"content": "print('Hello, World!')"},
						"hello.js": map[string]interface{}{"content": "console.log('Hello, World!');"},
					},
					"additionalProperties": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Content of the file",
								"maxLength":   1048576, // 1MB limit
							},
						},
						"required": []interface{}{"content"},
					},
				},
			},
			"required": []interface{}{"files"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"gist"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion":  "2022-11-28",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"id":          "aa5a315d61ae9438b18d",
			"html_url":    "https://gist.github.com/aa5a315d61ae9438b18d",
			"description": "Example of hello world in multiple languages",
			"public":      false,
			"files": map[string]interface{}{
				"hello.py": map[string]interface{}{
					"filename": "hello.py",
					"content":  "print('Hello, World!')",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid file content or missing files",
				"solution": "Ensure at least one file with content is provided",
			},
		},
		ExtendedHelp: `Creates a new GitHub Gist with one or more files.

Public vs Secret:
- Public: Discoverable via search and profile
- Secret: Only accessible via direct URL

Both types can be shared and embedded.

Use cases:
- Share code snippets
- Store configuration files
- Create quick backups
- Embed code in blogs/documentation`,
	}
}

func (h *CreateGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: GetOperationDescription("update_gist"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Unique identifier of the gist to update",
					"example":     "aa5a315d61ae9438b18d",
					"pattern":     "^[a-f0-9]+$",
					"minLength":   20,
					"maxLength":   32,
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "New description for the gist (optional)",
					"example":     "Updated: Configuration files for my application",
					"maxLength":   1024,
				},
				"files": map[string]interface{}{
					"type":        "object",
					"description": "Map of filename to file changes. Set content to null to delete a file, use filename property to rename",
					"example": map[string]interface{}{
						"config.json": map[string]interface{}{
							"content": "{\"updated\": true}",
						},
						"old_file.txt": map[string]interface{}{
							"content": nil, // Delete this file
						},
						"renamed.txt": map[string]interface{}{
							"filename": "new_name.txt", // Rename the file
							"content":  "Updated content",
						},
					},
					"additionalProperties": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "New content for the file (set to null to delete the file)",
								"maxLength":   1048576, // 1MB limit per file
								"nullable":    true,
							},
							"filename": map[string]interface{}{
								"type":        "string",
								"description": "New filename for renaming the file",
								"pattern":     "^[^/]+$", // No directory separators
								"maxLength":   255,
							},
						},
					},
				},
			},
			"required": []interface{}{"gist_id"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"gist"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
		},
		ResponseExample: map[string]interface{}{
			"id":          "aa5a315d61ae9438b18d",
			"url":         "https://gist.github.com/aa5a315d61ae9438b18d",
			"description": "Updated configuration files",
			"public":      true,
			"owner":       map[string]interface{}{"login": "octocat"},
			"files": map[string]interface{}{
				"config.json": map[string]interface{}{
					"filename": "config.json",
					"size":     42,
					"content":  "{\"updated\": true}",
				},
			},
			"created_at": "2024-01-01T10:00:00Z",
			"updated_at": "2024-01-20T15:30:00Z",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Gist does not exist or you don't have access",
				"solution": "Verify the gist ID and ensure you have permission to edit it",
			},
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid file content or structure",
				"solution": "Check file content is valid UTF-8 and within size limits",
			},
			{
				"error":    "403 Forbidden",
				"cause":    "Not the owner of the gist",
				"solution": "Only gist owners can update their gists",
			},
		},
		ExtendedHelp: `The update_gist operation allows you to modify an existing GitHub Gist.

Key features:
- Update the gist description
- Add new files to the gist
- Update content of existing files
- Delete files by setting content to null
- Rename files using the filename property

File operations:
1. Update content: Provide new content string
2. Delete file: Set content to null
3. Rename file: Use filename property with new name
4. Add new file: Include new filename in the files map

Examples:

# Update only description
{
  "gist_id": "aa5a315d61ae9438b18d",
  "description": "Updated configuration files"
}

# Add and update files
{
  "gist_id": "aa5a315d61ae9438b18d",
  "files": {
    "config.json": {
      "content": "{\"version\": 2}"
    },
    "new_file.txt": {
      "content": "This is a new file"
    }
  }
}

# Delete and rename files
{
  "gist_id": "aa5a315d61ae9438b18d",
  "files": {
    "old_file.txt": {
      "content": null
    },
    "config.yaml": {
      "filename": "config.yml",
      "content": "version: 2"
    }
  }
}

Note: Only the gist owner can update it. Fork the gist if you want to make changes to someone else's gist.`,
	}
}

func (h *UpdateGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: GetOperationDescription("delete_gist"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Unique identifier of the gist to delete",
					"example":     "aa5a315d61ae9438b18d",
					"pattern":     "^[a-f0-9]+$",
					"minLength":   20,
					"maxLength":   32,
				},
			},
			"required": []interface{}{"gist_id"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"gist"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
			"destructive":  true,
		},
		ResponseExample: map[string]interface{}{
			"status":  "deleted",
			"gist_id": "aa5a315d61ae9438b18d",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Gist does not exist or already deleted",
				"solution": "Verify the gist ID exists",
			},
			{
				"error":    "403 Forbidden",
				"cause":    "Not the owner of the gist",
				"solution": "Only gist owners can delete their gists",
			},
		},
		ExtendedHelp: `The delete_gist operation permanently removes a GitHub Gist.

⚠️ WARNING: This operation is destructive and cannot be undone!

Requirements:
- You must be the owner of the gist
- The gist_id must be valid

Considerations:
- All files in the gist will be permanently deleted
- All comments on the gist will be lost
- Starred and forked counts will be reset
- URLs to the gist will return 404

Example:
{
  "gist_id": "aa5a315d61ae9438b18d"
}

Alternatives to deletion:
- Make the gist secret instead of deleting
- Update the content to remove sensitive information
- Archive the content elsewhere before deletion`,
	}
}

func (h *DeleteGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: GetOperationDescription("star_gist"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Unique identifier of the gist to star",
					"example":     "aa5a315d61ae9438b18d",
					"pattern":     "^[a-f0-9]+$",
					"minLength":   20,
					"maxLength":   32,
				},
			},
			"required": []interface{}{"gist_id"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"gist"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
			"idempotent":   true,
		},
		ResponseExample: map[string]interface{}{
			"status":  "starred",
			"gist_id": "aa5a315d61ae9438b18d",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Gist does not exist",
				"solution": "Verify the gist ID is correct",
			},
			{
				"error":    "304 Not Modified",
				"cause":    "Gist is already starred",
				"solution": "No action needed - gist is already in your starred list",
			},
		},
		ExtendedHelp: `The star_gist operation adds a GitHub Gist to your starred gists list.

Starring a gist:
- Adds it to your personal starred gists collection
- Makes it easier to find later via your starred gists page
- Shows your appreciation for the gist
- Increases the gist's star count

Notes:
- This operation is idempotent - starring an already starred gist has no effect
- You can star your own gists
- Starred gists are public - anyone can see what you've starred
- There's no limit to how many gists you can star

Example:
{
  "gist_id": "aa5a315d61ae9438b18d"
}

To view your starred gists:
- Visit https://gist.github.com/starred
- Or use the list_gists operation with starred=true`,
	}
}

func (h *StarGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: GetOperationDescription("unstar_gist"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gist_id": map[string]interface{}{
					"type":        "string",
					"description": "Unique identifier of the gist to unstar",
					"example":     "aa5a315d61ae9438b18d",
					"pattern":     "^[a-f0-9]+$",
					"minLength":   20,
					"maxLength":   32,
				},
			},
			"required": []interface{}{"gist_id"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"gist"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
			"idempotent":   true,
		},
		ResponseExample: map[string]interface{}{
			"status":  "unstarred",
			"gist_id": "aa5a315d61ae9438b18d",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Gist does not exist",
				"solution": "Verify the gist ID is correct",
			},
			{
				"error":    "304 Not Modified",
				"cause":    "Gist is not starred",
				"solution": "No action needed - gist is not in your starred list",
			},
		},
		ExtendedHelp: `The unstar_gist operation removes a GitHub Gist from your starred gists list.

Unstarring a gist:
- Removes it from your starred gists collection
- Decreases the gist's star count
- Does not affect the gist itself or your access to it

Notes:
- This operation is idempotent - unstarring a non-starred gist has no effect
- Does not affect your ability to view or fork the gist
- The gist owner is not notified when you unstar

Example:
{
  "gist_id": "aa5a315d61ae9438b18d"
}

Use cases:
- Clean up your starred gists list
- Remove outdated or no longer relevant gists
- Organize your starred content`,
	}
}

func (h *UnstarGistHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: GetOperationDescription("watch_repository"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"subscribed": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to receive notifications for all activity (true) or only participating/mentions (false)",
					"default":     true,
					"example":     true,
				},
				"ignored": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to ignore all notifications from this repository",
					"default":     false,
					"example":     false,
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"notifications", "repo"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
			"idempotent":   true,
		},
		ResponseExample: map[string]interface{}{
			"subscribed":     true,
			"ignored":        false,
			"reason":         "manual",
			"created_at":     "2024-01-15T12:00:00Z",
			"url":            "https://api.github.com/repos/octocat/Hello-World/subscription",
			"repository_url": "https://api.github.com/repos/octocat/Hello-World",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Repository does not exist or is private without access",
				"solution": "Verify repository exists and you have read access",
			},
			{
				"error":    "403 Forbidden",
				"cause":    "Missing required OAuth scope",
				"solution": "Ensure your token has 'notifications' or 'repo' scope",
			},
		},
		ExtendedHelp: `The watch_repository operation manages your notification subscription for a GitHub repository.

Subscription levels:
1. Not watching: No notifications except when participating or @mentioned
2. Watching (subscribed=false): Notifications for participating and @mentions only
3. Watching all (subscribed=true): Notifications for all repository activity
4. Ignoring (ignored=true): Never receive notifications

Notification triggers when watching all:
- New issues and pull requests
- Comments on issues and PRs
- Commits and releases
- Wiki updates
- Discussions (if enabled)

Examples:

# Watch for all activity
{
  "owner": "octocat",
  "repo": "Hello-World",
  "subscribed": true
}

# Watch only participating/mentions
{
  "owner": "octocat",
  "repo": "Hello-World",
  "subscribed": false
}

# Ignore all notifications
{
  "owner": "octocat",
  "repo": "Hello-World",
  "ignored": true
}

Notes:
- Watching a repository does not automatically star it
- You automatically watch repositories you create or are added as a collaborator
- Organization owners can configure default watch settings for members`,
	}
}

func (h *WatchRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: GetOperationDescription("unwatch_repository"),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner (username or organization)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"oauth_scopes": []string{"notifications", "repo"},
			"rate_limit":   "core",
			"api_version":  "2022-11-28",
			"idempotent":   true,
		},
		ResponseExample: map[string]interface{}{
			"status": "unwatched",
			"owner":  "octocat",
			"repo":   "Hello-World",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Repository does not exist or subscription not found",
				"solution": "Verify repository exists and you were watching it",
			},
			{
				"error":    "304 Not Modified",
				"cause":    "Repository is not being watched",
				"solution": "No action needed - you're not watching this repository",
			},
		},
		ExtendedHelp: `The unwatch_repository operation removes your notification subscription from a GitHub repository.

Effects of unwatching:
- Stops all automatic notifications from the repository
- You'll still receive notifications when:
  - You're @mentioned
  - You're participating in a discussion
  - You're assigned to an issue or PR
  - You author an issue or PR

Notes:
- This is idempotent - unwatching an already unwatched repo has no effect
- Does not affect your starred repositories
- Does not affect your repository access permissions
- You can re-watch the repository at any time

Example:
{
  "owner": "octocat",
  "repo": "Hello-World"
}

Common use cases:
- Reduce notification noise from active repositories
- Stop watching after a project is completed
- Clean up watch list for repositories you no longer follow
- Temporarily mute notifications while keeping star status`,
	}
}

func (h *UnwatchRepositoryHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
