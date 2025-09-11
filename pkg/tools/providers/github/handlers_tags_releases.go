package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Tags and Releases Handlers

// ListTagsHandler handles listing repository tags
type ListTagsHandler struct {
	provider *GitHubProvider
}

func NewListTagsHandler(p *GitHubProvider) *ListTagsHandler {
	return &ListTagsHandler{provider: p}
}

func (h *ListTagsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_tags",
		Description: "List tags in a repository",
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
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListTagsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	pagination := ExtractPagination(params)
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list tags: %v", err)), nil
	}

	data, _ := json.Marshal(tags)
	return NewToolResult(string(data)), nil
}

// GetTagHandler handles getting a specific tag
type GetTagHandler struct {
	provider *GitHubProvider
}

func NewGetTagHandler(p *GitHubProvider) *GetTagHandler {
	return &GetTagHandler{provider: p}
}

func (h *GetTagHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_tag",
		Description: "Get a specific tag from a repository",
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
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag name",
				},
			},
			"required": []interface{}{"owner", "repo", "tag"},
		},
	}
}

func (h *GetTagHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	tagName := extractString(params, "tag")

	// Get tag reference
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "tags/"+tagName)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get tag: %v", err)), nil
	}

	// If it's an annotated tag, get the tag object
	if ref.Object.Type != nil && *ref.Object.Type == "tag" {
		tag, _, err := client.Git.GetTag(ctx, owner, repo, *ref.Object.SHA)
		if err != nil {
			return NewToolError(fmt.Sprintf("Failed to get tag object: %v", err)), nil
		}
		data, _ := json.Marshal(tag)
		return NewToolResult(string(data)), nil
	}

	// Return ref for lightweight tags
	data, _ := json.Marshal(ref)
	return NewToolResult(string(data)), nil
}

// ListReleasesHandler handles listing repository releases
type ListReleasesHandler struct {
	provider *GitHubProvider
}

func NewListReleasesHandler(p *GitHubProvider) *ListReleasesHandler {
	return &ListReleasesHandler{provider: p}
}

func (h *ListReleasesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_releases",
		Description: "List releases in a repository",
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
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListReleasesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	pagination := ExtractPagination(params)
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	releases, _, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list releases: %v", err)), nil
	}

	data, _ := json.Marshal(releases)
	return NewToolResult(string(data)), nil
}

// GetLatestReleaseHandler handles getting the latest release
type GetLatestReleaseHandler struct {
	provider *GitHubProvider
}

func NewGetLatestReleaseHandler(p *GitHubProvider) *GetLatestReleaseHandler {
	return &GetLatestReleaseHandler{provider: p}
}

func (h *GetLatestReleaseHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_latest_release",
		Description: "Get the latest release from a repository",
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

func (h *GetLatestReleaseHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get latest release: %v", err)), nil
	}

	data, _ := json.Marshal(release)
	return NewToolResult(string(data)), nil
}

// GetReleaseByTagHandler handles getting a release by tag
type GetReleaseByTagHandler struct {
	provider *GitHubProvider
}

func NewGetReleaseByTagHandler(p *GitHubProvider) *GetReleaseByTagHandler {
	return &GetReleaseByTagHandler{provider: p}
}

func (h *GetReleaseByTagHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_release_by_tag",
		Description: "Get a release by its tag name",
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
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag name",
				},
			},
			"required": []interface{}{"owner", "repo", "tag"},
		},
	}
}

func (h *GetReleaseByTagHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	tag := extractString(params, "tag")

	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get release by tag: %v", err)), nil
	}

	data, _ := json.Marshal(release)
	return NewToolResult(string(data)), nil
}

// CreateReleaseHandler handles creating a new release
type CreateReleaseHandler struct {
	provider *GitHubProvider
}

func NewCreateReleaseHandler(p *GitHubProvider) *CreateReleaseHandler {
	return &CreateReleaseHandler{provider: p}
}

func (h *CreateReleaseHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_release",
		Description: "Create a new release",
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
				"tag_name": map[string]interface{}{
					"type":        "string",
					"description": "Tag name for the release",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Release name",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Release description",
				},
				"target_commitish": map[string]interface{}{
					"type":        "string",
					"description": "Target branch or commit SHA",
				},
				"draft": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this is a draft release",
				},
				"prerelease": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this is a prerelease",
				},
				"generate_release_notes": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to generate release notes automatically",
				},
			},
			"required": []interface{}{"owner", "repo", "tag_name"},
		},
	}
}

func (h *CreateReleaseHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	tagName := extractString(params, "tag_name")
	releaseRequest := &github.RepositoryRelease{
		TagName: &tagName,
	}
	
	if name := extractString(params, "name"); name != "" {
		releaseRequest.Name = &name
	}
	
	if body := extractString(params, "body"); body != "" {
		releaseRequest.Body = &body
	}
	
	if targetCommitish := extractString(params, "target_commitish"); targetCommitish != "" {
		releaseRequest.TargetCommitish = &targetCommitish
	}
	
	if draft, ok := params["draft"].(bool); ok {
		releaseRequest.Draft = &draft
	}
	
	if prerelease, ok := params["prerelease"].(bool); ok {
		releaseRequest.Prerelease = &prerelease
	}
	
	if generateNotes, ok := params["generate_release_notes"].(bool); ok {
		releaseRequest.GenerateReleaseNotes = &generateNotes
	}

	release, _, err := client.Repositories.CreateRelease(ctx, owner, repo, releaseRequest)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create release: %v", err)), nil
	}

	data, _ := json.Marshal(release)
	return NewToolResult(string(data)), nil
}