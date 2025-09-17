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
		Description: "List repository tags, typically used for versioning and releases. Returns tags with their commit information.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of tags per page (1-100)",
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
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"repo"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"tags": []interface{}{
				map[string]interface{}{
					"name":        "v1.0.0",
					"zipball_url": "https://api.github.com/repos/octocat/Hello-World/zipball/v1.0.0",
					"tarball_url": "https://api.github.com/repos/octocat/Hello-World/tarball/v1.0.0",
					"commit": map[string]interface{}{
						"sha": "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
						"url": "https://api.github.com/repos/octocat/Hello-World/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					},
					"node_id": "MDM6VGFnOTQwYmQzMzYyNDhlZmFlMGY5ZWU1YmM3YjJkNWM5ODU4ODdiMTZhYw==",
				},
				map[string]interface{}{
					"name":        "v0.9.0",
					"zipball_url": "https://api.github.com/repos/octocat/Hello-World/zipball/v0.9.0",
					"tarball_url": "https://api.github.com/repos/octocat/Hello-World/tarball/v0.9.0",
					"commit": map[string]interface{}{
						"sha": "553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
						"url": "https://api.github.com/repos/octocat/Hello-World/commits/553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
					},
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Repository not found or no access",
				"solution": "Verify the repository exists and you have read access",
			},
		},
		ExtendedHelp: `Lists all tags in a repository. Tags are typically used for marking release points or versions.

The response includes:
- name: The tag name (e.g., 'v1.0.0')
- commit: Information about the commit the tag points to
- zipball_url: URL to download the source code as a zip file
- tarball_url: URL to download the source code as a tar file

Common use cases:
- List all versions/releases of a project
- Find the latest tag for deployment
- Verify tag naming conventions
- Download source code for specific versions

Note: Tags are different from releases. A tag is a Git reference, while a release is a GitHub feature built on top of tags with additional metadata.`,
	}
}

func (h *ListTagsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get detailed information about a specific tag in a repository, including the Git object it points to.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag name to retrieve (e.g., 'v1.0.0', 'release-2024')",
					"example":     "v1.0.0",
					"minLength":   1,
					"maxLength":   255,
				},
			},
			"required": []interface{}{"owner", "repo", "tag"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"repo"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
		},
		ResponseExample: map[string]interface{}{
			"tag": "v1.0.0",
			"sha": "940bd336248efae0f9ee5bc7b2d5c985887b16ac",
			"url": "https://api.github.com/repos/octocat/Hello-World/git/tags/940bd336248efae0f9ee5bc7b2d5c985887b16ac",
			"tagger": map[string]interface{}{
				"name":  "Monalisa Octocat",
				"email": "octocat@github.com",
				"date":  "2014-11-07T22:01:45Z",
			},
			"object": map[string]interface{}{
				"type": "commit",
				"sha":  "c3d0be41ecbe669545ee3e94d31ed9a4bc91ee3c",
				"url":  "https://api.github.com/repos/octocat/Hello-World/git/commits/c3d0be41ecbe669545ee3e94d31ed9a4bc91ee3c",
			},
			"message": "Release version 1.0.0\n\n### Features\n- Initial stable release\n- Full API implementation\n",
			"verification": map[string]interface{}{
				"verified":  true,
				"reason":    "valid",
				"signature": "-----BEGIN PGP SIGNATURE-----...",
				"payload":   "object c3d0be41ecbe669545ee3e94d31ed9a4bc91ee3c...",
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Tag not found in repository",
				"solution": "Verify the tag name is correct and exists in the repository",
			},
		},
		ExtendedHelp: `Retrieves detailed information about a specific tag, including annotated tag data if available.

Two types of tags:
1. Lightweight tags: Simple references to commits
2. Annotated tags: Include tagger info, date, message, and can be signed

The response includes:
- tag: The tag name
- object: The Git object the tag points to (usually a commit)
- tagger: Information about who created the tag (for annotated tags)
- message: Tag message (for annotated tags)
- verification: GPG signature verification (if signed)

Use cases:
- Get detailed release information
- Verify tag signatures
- Check who created a tag and when
- Read release notes from tag messages`,
	}
}

func (h *GetTagHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "List GitHub releases for a repository. Releases are deployable software iterations with release notes and assets.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of releases per page (1-100)",
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
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"repo"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
			"pagination": true,
		},
		ResponseExample: map[string]interface{}{
			"releases": []interface{}{
				map[string]interface{}{
					"id":               1,
					"tag_name":         "v1.0.0",
					"target_commitish": "main",
					"name":             "Version 1.0.0",
					"body":             "## What's Changed\n* Feature A by @user in #123\n* Fix B by @user in #124\n\n**Full Changelog**: https://github.com/octocat/Hello-World/compare/v0.9.0...v1.0.0",
					"draft":            false,
					"prerelease":       false,
					"created_at":       "2024-01-15T10:00:00Z",
					"published_at":     "2024-01-15T10:30:00Z",
					"author": map[string]interface{}{
						"login":      "octocat",
						"id":         1,
						"avatar_url": "https://github.com/images/error/octocat_happy.gif",
						"type":       "User",
					},
					"assets": []interface{}{
						map[string]interface{}{
							"name":                 "app-v1.0.0.zip",
							"size":                 1024000,
							"download_count":       42,
							"browser_download_url": "https://github.com/octocat/Hello-World/releases/download/v1.0.0/app-v1.0.0.zip",
						},
					},
					"tarball_url": "https://api.github.com/repos/octocat/Hello-World/tarball/v1.0.0",
					"zipball_url": "https://api.github.com/repos/octocat/Hello-World/zipball/v1.0.0",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Repository not found or no releases",
				"solution": "Verify the repository exists and has published releases",
			},
		},
		ExtendedHelp: `Lists all releases for a repository, ordered by creation date (newest first).

Releases vs Tags:
- Releases are GitHub-specific features built on top of Git tags
- Include release notes, binary assets, and metadata
- Can be drafts or pre-releases
- Tags are Git references that releases are based on

The response includes:
- tag_name: The Git tag associated with the release
- name: The release title
- body: Release notes/description
- draft: Whether it's a draft (not visible to public)
- prerelease: Whether it's marked as a pre-release
- assets: Downloadable files attached to the release
- author: Who created the release

Use cases:
- Display changelog to users
- Download release assets
- Track deployment history
- Automate release notifications`,
	}
}

func (h *ListReleasesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get the latest published release for a repository. Returns the most recent non-prerelease, non-draft release.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"repo"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
		},
		ResponseExample: map[string]interface{}{
			"id":               1,
			"tag_name":         "v1.0.0",
			"target_commitish": "main",
			"name":             "Version 1.0.0 - Stable Release",
			"body":             "## Major Features\n\n- Feature A\n- Feature B\n\n## Bug Fixes\n\n- Fixed issue #123\n\n## Breaking Changes\n\n- API endpoint renamed",
			"draft":            false,
			"prerelease":       false,
			"created_at":       "2024-01-15T10:00:00Z",
			"published_at":     "2024-01-15T10:30:00Z",
			"html_url":         "https://github.com/octocat/Hello-World/releases/tag/v1.0.0",
			"assets": []interface{}{
				map[string]interface{}{
					"name":                 "app-v1.0.0.zip",
					"size":                 1024000,
					"download_count":       42,
					"browser_download_url": "https://github.com/octocat/Hello-World/releases/download/v1.0.0/app-v1.0.0.zip",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "No published releases found",
				"solution": "Verify the repository has at least one published, non-draft release",
			},
		},
		ExtendedHelp: `Gets the latest published release for a repository.

Selection criteria:
- Only published releases (not drafts)
- Only full releases (not pre-releases) by default
- Sorted by published date, not creation date

Use cases:
- Get current stable version for deployment
- Check for updates in automated systems
- Display "What's New" to users
- Download latest release assets

Note: This endpoint specifically returns the latest full release. Use list_releases to get pre-releases or drafts.`,
	}
}

func (h *GetLatestReleaseHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get a specific release by its associated tag name. Useful for retrieving release details for a specific version.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag name associated with the release (e.g., 'v1.0.0', 'release-2024')",
					"example":     "v1.0.0",
					"minLength":   1,
					"maxLength":   255,
				},
			},
			"required": []interface{}{"owner", "repo", "tag"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"repo"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion": "2022-11-28",
		},
		ResponseExample: map[string]interface{}{
			"id":               1,
			"tag_name":         "v1.0.0",
			"target_commitish": "main",
			"name":             "Version 1.0.0",
			"body":             "Release notes for version 1.0.0\n\n### Features\n- New feature A\n- New feature B",
			"draft":            false,
			"prerelease":       false,
			"created_at":       "2024-01-15T10:00:00Z",
			"published_at":     "2024-01-15T10:30:00Z",
			"author": map[string]interface{}{
				"login":      "octocat",
				"id":         1,
				"avatar_url": "https://github.com/images/error/octocat_happy.gif",
			},
			"assets": []interface{}{
				map[string]interface{}{
					"name":                 "app-v1.0.0.zip",
					"size":                 1024000,
					"browser_download_url": "https://github.com/octocat/Hello-World/releases/download/v1.0.0/app-v1.0.0.zip",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "No release found for the specified tag",
				"solution": "Verify the tag exists and has an associated release (not all tags have releases)",
			},
		},
		ExtendedHelp: `Retrieves a release by its tag name.

Important notes:
- Not all tags have associated releases
- The tag must match exactly (case-sensitive)
- Returns draft releases if you have permission

Use cases:
- Get release notes for a specific version
- Download assets for a particular release
- Verify release metadata before deployment
- Check if a tag has been released

Difference from get_tag:
- get_tag: Returns Git tag information
- get_release_by_tag: Returns GitHub release information (notes, assets, metadata)`,
	}
}

func (h *GetReleaseByTagHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Create a new GitHub release. Creates a tag if it doesn't exist and publishes release notes with optional assets.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*$",
					"minLength":   1,
					"maxLength":   100,
				},
				"tag_name": map[string]interface{}{
					"type":        "string",
					"description": "Tag name for the release. Will be created if it doesn't exist (e.g., 'v1.0.0', 'release-2024')",
					"example":     "v1.0.0",
					"minLength":   1,
					"maxLength":   255,
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Release title. Defaults to tag_name if not specified.",
					"example":     "Version 1.0.0 - Stable Release",
					"maxLength":   125,
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Release notes/description in Markdown format. Leave empty to auto-generate.",
					"example":     "## What's Changed\n\n### Features\n- New feature A (#123)\n- New feature B (#124)\n\n### Bug Fixes\n- Fixed issue with authentication (#125)\n\n**Full Changelog**: https://github.com/owner/repo/compare/v0.9.0...v1.0.0",
					"maxLength":   125000,
				},
				"target_commitish": map[string]interface{}{
					"type":        "string",
					"description": "Branch name or commit SHA to create the tag from. Defaults to the default branch.",
					"example":     "main",
				},
				"draft": map[string]interface{}{
					"type":        "boolean",
					"description": "Create as draft (unpublished) release. Drafts are only visible to users with push access.",
					"default":     false,
					"example":     false,
				},
				"prerelease": map[string]interface{}{
					"type":        "boolean",
					"description": "Mark as pre-release (e.g., beta, alpha). Pre-releases are marked as not ready for production.",
					"default":     false,
					"example":     false,
				},
				"generate_release_notes": map[string]interface{}{
					"type":        "boolean",
					"description": "Auto-generate release notes from commits and pull requests. GitHub will create notes based on merged PRs.",
					"default":     false,
					"example":     true,
				},
			},
			"required": []interface{}{"owner", "repo", "tag_name"},
		},
		Metadata: map[string]interface{}{
			"scopes": []interface{}{"repo"},
			"rateLimit": map[string]interface{}{
				"requests": 5000,
				"period":   "hour",
			},
			"apiVersion":  "2022-11-28",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"id":               1,
			"tag_name":         "v1.0.0",
			"target_commitish": "main",
			"name":             "Version 1.0.0 - Stable Release",
			"body":             "## What's Changed\n\n### Features\n- New feature A by @user in #123\n",
			"draft":            false,
			"prerelease":       false,
			"created_at":       "2024-01-15T10:00:00Z",
			"published_at":     "2024-01-15T10:00:00Z",
			"html_url":         "https://github.com/octocat/Hello-World/releases/tag/v1.0.0",
			"upload_url":       "https://uploads.github.com/repos/octocat/Hello-World/releases/1/assets{?name,label}",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Release already exists for this tag",
				"solution": "Delete the existing release first or use a different tag name",
			},
			{
				"error":    "404 Not Found",
				"cause":    "Target branch or commit not found",
				"solution": "Verify target_commitish exists in the repository",
			},
		},
		ExtendedHelp: `Creates a new GitHub release with release notes and optional file attachments.

Release workflow:
1. Create release (this operation)
2. Upload assets (separate API calls)
3. Publish release (if created as draft)

Key features:
- Auto-creates tag if it doesn't exist
- Can generate release notes from PRs automatically
- Supports markdown in release body
- Can create as draft for review before publishing
- Pre-release flag for beta/alpha versions

Best practices:
- Use semantic versioning for tags (e.g., v1.0.0)
- Include changelog in release body
- Mark unstable versions as pre-release
- Create as draft if you need to upload assets before publishing

Note: After creating a release, use the upload_url to attach binary assets.`,
	}
}

func (h *CreateReleaseHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
