package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Git Operations - Trees, Commits, Refs, Blobs

// GetBlobHandler handles getting a blob's content
type GetBlobHandler struct {
	provider *GitHubProvider
}

func NewGetBlobHandler(p *GitHubProvider) *GetBlobHandler {
	return &GetBlobHandler{provider: p}
}

func (h *GetBlobHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_blob",
		Description: "Get a blob's content from a repository",
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
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA of the blob",
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
		},
	}
}

func (h *GetBlobHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	sha := extractString(params, "sha")

	blob, _, err := client.Git.GetBlob(ctx, owner, repo, sha)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get blob: %v", err)), nil
	}

	// Decode content if it's base64 encoded
	content := blob.GetContent()
	if blob.GetEncoding() == "base64" && content != "" {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err == nil {
			content = string(decoded)
		}
	}

	return NewToolResult(map[string]interface{}{
		"sha":      blob.GetSHA(),
		"size":     blob.GetSize(),
		"encoding": blob.GetEncoding(),
		"content":  content,
		"url":      blob.GetURL(),
	}), nil
}

// CreateBlobHandler handles creating a new blob
type CreateBlobHandler struct {
	provider *GitHubProvider
}

func NewCreateBlobHandler(p *GitHubProvider) *CreateBlobHandler {
	return &CreateBlobHandler{provider: p}
}

func (h *CreateBlobHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_blob",
		Description: "Create a new blob in a repository",
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
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content of the blob",
				},
				"encoding": map[string]interface{}{
					"type":        "string",
					"description": "Encoding (utf-8 or base64, defaults to utf-8)",
				},
			},
			"required": []interface{}{"owner", "repo", "content"},
		},
	}
}

func (h *CreateBlobHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	content := extractString(params, "content")
	encoding := extractString(params, "encoding")
	
	if encoding == "" {
		encoding = "utf-8"
	}

	blob := &github.Blob{
		Content:  &content,
		Encoding: &encoding,
	}

	created, _, err := client.Git.CreateBlob(ctx, owner, repo, blob)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create blob: %v", err)), nil
	}

	return NewToolResult(map[string]interface{}{
		"sha": created.GetSHA(),
		"url": created.GetURL(),
	}), nil
}

// GetTreeHandler handles getting a tree
type GetTreeHandler struct {
	provider *GitHubProvider
}

func NewGetTreeHandler(p *GitHubProvider) *GetTreeHandler {
	return &GetTreeHandler{provider: p}
}

func (h *GetTreeHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_tree",
		Description: "Get a tree from a repository",
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
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA of the tree",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Get tree recursively",
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
		},
	}
}

func (h *GetTreeHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	sha := extractString(params, "sha")
	recursive, _ := params["recursive"].(bool)

	tree, _, err := client.Git.GetTree(ctx, owner, repo, sha, recursive)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get tree: %v", err)), nil
	}

	data, _ := json.Marshal(tree)
	return NewToolResult(string(data)), nil
}

// CreateTreeHandler handles creating a new tree
type CreateTreeHandler struct {
	provider *GitHubProvider
}

func NewCreateTreeHandler(p *GitHubProvider) *CreateTreeHandler {
	return &CreateTreeHandler{provider: p}
}

func (h *CreateTreeHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_tree",
		Description: "Create a new tree in a repository",
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
				"base_tree": map[string]interface{}{
					"type":        "string",
					"description": "SHA of tree to base this tree on",
				},
				"tree": map[string]interface{}{
					"type":        "array",
					"description": "Array of tree entries",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "File path",
							},
							"mode": map[string]interface{}{
								"type":        "string",
								"description": "File mode (100644, 100755, 040000, 160000, 120000)",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"description": "Object type (blob, tree, commit)",
							},
							"sha": map[string]interface{}{
								"type":        "string",
								"description": "SHA of the object",
							},
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Content (for new files)",
							},
						},
					},
				},
			},
			"required": []interface{}{"owner", "repo", "tree"},
		},
	}
}

func (h *CreateTreeHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	baseTree := extractString(params, "base_tree")

	var entries []*github.TreeEntry
	if treeArray, ok := params["tree"].([]interface{}); ok {
		for _, item := range treeArray {
			if entry, ok := item.(map[string]interface{}); ok {
				treeEntry := &github.TreeEntry{}
				
				if path := extractString(entry, "path"); path != "" {
					treeEntry.Path = &path
				}
				if mode := extractString(entry, "mode"); mode != "" {
					treeEntry.Mode = &mode
				}
				if entryType := extractString(entry, "type"); entryType != "" {
					treeEntry.Type = &entryType
				}
				if sha := extractString(entry, "sha"); sha != "" {
					treeEntry.SHA = &sha
				}
				if content := extractString(entry, "content"); content != "" {
					treeEntry.Content = &content
				}
				
				entries = append(entries, treeEntry)
			}
		}
	}

	created, _, err := client.Git.CreateTree(ctx, owner, repo, baseTree, entries)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create tree: %v", err)), nil
	}

	data, _ := json.Marshal(created)
	return NewToolResult(string(data)), nil
}

// GetGitCommitHandler handles getting a Git commit object
type GetGitCommitHandler struct {
	provider *GitHubProvider
}

func NewGetGitCommitHandler(p *GitHubProvider) *GetGitCommitHandler {
	return &GetGitCommitHandler{provider: p}
}

func (h *GetGitCommitHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_git_commit",
		Description: "Get a specific commit from a repository",
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
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA of the commit",
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
		},
	}
}

func (h *GetGitCommitHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	sha := extractString(params, "sha")

	commit, _, err := client.Git.GetCommit(ctx, owner, repo, sha)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get commit: %v", err)), nil
	}

	data, _ := json.Marshal(commit)
	return NewToolResult(string(data)), nil
}

// CreateCommitHandler handles creating a new commit
type CreateCommitHandler struct {
	provider *GitHubProvider
}

func NewCreateCommitHandler(p *GitHubProvider) *CreateCommitHandler {
	return &CreateCommitHandler{provider: p}
}

func (h *CreateCommitHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_commit",
		Description: "Create a new commit in a repository",
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
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Commit message",
				},
				"tree": map[string]interface{}{
					"type":        "string",
					"description": "SHA of the tree object",
				},
				"parents": map[string]interface{}{
					"type":        "array",
					"description": "SHAs of parent commits",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"author": map[string]interface{}{
					"type": "object",
					"description": "Author information",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"email": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			"required": []interface{}{"owner", "repo", "message", "tree"},
		},
	}
}

func (h *CreateCommitHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	message := extractString(params, "message")
	tree := extractString(params, "tree")

	commit := &github.Commit{
		Message: &message,
		Tree: &github.Tree{
			SHA: &tree,
		},
	}

	// Add parents
	if parentsArray, ok := params["parents"].([]interface{}); ok {
		var parents []*github.Commit
		for _, p := range parentsArray {
			if parentSHA, ok := p.(string); ok {
				sha := parentSHA
				parents = append(parents, &github.Commit{SHA: &sha})
			}
		}
		commit.Parents = parents
	}

	// Add author if provided
	if authorMap, ok := params["author"].(map[string]interface{}); ok {
		author := &github.CommitAuthor{}
		if name := extractString(authorMap, "name"); name != "" {
			author.Name = &name
		}
		if email := extractString(authorMap, "email"); email != "" {
			author.Email = &email
		}
		commit.Author = author
	}

	created, _, err := client.Git.CreateCommit(ctx, owner, repo, commit, nil)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create commit: %v", err)), nil
	}

	data, _ := json.Marshal(created)
	return NewToolResult(string(data)), nil
}

// GetRefHandler handles getting a reference
type GetRefHandler struct {
	provider *GitHubProvider
}

func NewGetRefHandler(p *GitHubProvider) *GetRefHandler {
	return &GetRefHandler{provider: p}
}

func (h *GetRefHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_ref",
		Description: "Get a reference from a repository",
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
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Reference path (e.g., heads/main, tags/v1.0.0)",
				},
			},
			"required": []interface{}{"owner", "repo", "ref"},
		},
	}
}

func (h *GetRefHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	ref := extractString(params, "ref")

	reference, _, err := client.Git.GetRef(ctx, owner, repo, ref)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get reference: %v", err)), nil
	}

	data, _ := json.Marshal(reference)
	return NewToolResult(string(data)), nil
}

// ListRefsHandler handles listing references
type ListRefsHandler struct {
	provider *GitHubProvider
}

func NewListRefsHandler(p *GitHubProvider) *ListRefsHandler {
	return &ListRefsHandler{provider: p}
}

func (h *ListRefsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_refs",
		Description: "List references in a repository",
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
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Type of refs to list (heads, tags, or empty for all)",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListRefsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	refType := extractString(params, "type")

	var refs []*github.Reference
	var err error

	switch refType {
	case "heads":
		refs, _, err = client.Git.ListMatchingRefs(ctx, owner, repo, &github.ReferenceListOptions{
			Ref: "heads/",
		})
	case "tags":
		refs, _, err = client.Git.ListMatchingRefs(ctx, owner, repo, &github.ReferenceListOptions{
			Ref: "tags/",
		})
	default:
		refs, _, err = client.Git.ListMatchingRefs(ctx, owner, repo, nil)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list references: %v", err)), nil
	}

	data, _ := json.Marshal(refs)
	return NewToolResult(string(data)), nil
}

// CreateRefHandler handles creating a reference
type CreateRefHandler struct {
	provider *GitHubProvider
}

func NewCreateRefHandler(p *GitHubProvider) *CreateRefHandler {
	return &CreateRefHandler{provider: p}
}

func (h *CreateRefHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_ref",
		Description: "Create a new reference in a repository",
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
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Reference name (must start with refs/)",
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA to point the reference to",
				},
			},
			"required": []interface{}{"owner", "repo", "ref", "sha"},
		},
	}
}

func (h *CreateRefHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	ref := extractString(params, "ref")
	sha := extractString(params, "sha")

	reference := &github.Reference{
		Ref: &ref,
		Object: &github.GitObject{
			SHA: &sha,
		},
	}

	created, _, err := client.Git.CreateRef(ctx, owner, repo, reference)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to create reference: %v", err)), nil
	}

	data, _ := json.Marshal(created)
	return NewToolResult(string(data)), nil
}

// UpdateRefHandler handles updating a reference
type UpdateRefHandler struct {
	provider *GitHubProvider
}

func NewUpdateRefHandler(p *GitHubProvider) *UpdateRefHandler {
	return &UpdateRefHandler{provider: p}
}

func (h *UpdateRefHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_ref",
		Description: "Update an existing reference in a repository",
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
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Reference path (e.g., heads/main)",
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "New SHA to point the reference to",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Force update even if not fast-forward",
				},
			},
			"required": []interface{}{"owner", "repo", "ref", "sha"},
		},
	}
}

func (h *UpdateRefHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	refPath := extractString(params, "ref")
	sha := extractString(params, "sha")
	force, _ := params["force"].(bool)

	reference := &github.Reference{
		Ref: &refPath,
		Object: &github.GitObject{
			SHA: &sha,
		},
	}

	updated, _, err := client.Git.UpdateRef(ctx, owner, repo, reference, force)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update reference: %v", err)), nil
	}

	data, _ := json.Marshal(updated)
	return NewToolResult(string(data)), nil
}

// DeleteRefHandler handles deleting a reference
type DeleteRefHandler struct {
	provider *GitHubProvider
}

func NewDeleteRefHandler(p *GitHubProvider) *DeleteRefHandler {
	return &DeleteRefHandler{provider: p}
}

func (h *DeleteRefHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_ref",
		Description: "Delete a reference from a repository",
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
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Reference path (e.g., heads/feature-branch)",
				},
			},
			"required": []interface{}{"owner", "repo", "ref"},
		},
	}
}

func (h *DeleteRefHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	ref := extractString(params, "ref")

	_, err := client.Git.DeleteRef(ctx, owner, repo, ref)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete reference: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status": "deleted",
		"ref":    ref,
	}), nil
}