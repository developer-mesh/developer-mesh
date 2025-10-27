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
		Description: "Get blob content (raw file data) by SHA. Use when: accessing specific Git object, low-level Git operations, blob inspection.",
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
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "The SHA hash of the blob object to retrieve",
					"example":     "3a0f86fb8db8eea7ccbb9a95f325ddbedfb25e15",
					"pattern":     "^[a-f0-9]{40}$",
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
		},
		Metadata: map[string]interface{}{
			"category":    "git_operations",
			"scopes":      []string{"repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"sha":      "3a0f86fb8db8eea7ccbb9a95f325ddbedfb25e15",
			"size":     1024,
			"encoding": "base64",
			"content":  "SGVsbG8gV29ybGQh",
			"url":      "https://api.github.com/repos/owner/repo/git/blobs/3a0f86fb",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the SHA exists. Use get_tree or list_commits to find valid blob SHAs.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have read access to the repository.",
			},
		},
		ExtendedHelp: "Retrieves raw file content from Git's object storage using the blob's SHA. Blobs represent file contents in Git. The content may be base64 encoded for binary files. Use get_file_contents for a simpler way to get file content by path.",
	}
}

func (h *GetBlobHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Create blob from content (returns SHA). Use when: low-level Git operations, building commits programmatically, storing file content.",
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
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to store in the blob (text or base64 encoded binary)",
					"example":     "Hello World!",
				},
				"encoding": map[string]interface{}{
					"type":        "string",
					"description": "Content encoding format",
					"enum":        []interface{}{"utf-8", "base64"},
					"default":     "utf-8",
					"example":     "utf-8",
				},
			},
			"required": []interface{}{"owner", "repo", "content"},
		},
		Metadata: map[string]interface{}{
			"category":    "git_operations",
			"scopes":      []string{"repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"sha": "3a0f86fb8db8eea7ccbb9a95f325ddbedfb25e15",
			"url": "https://api.github.com/repos/owner/repo/git/blobs/3a0f86fb",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have write access to the repository.",
			},
			{
				"error":    "422 Unprocessable Entity",
				"solution": "Check that the encoding matches the content format (use base64 for binary data).",
			},
		},
		ExtendedHelp: "Creates a blob object in Git's object storage. Blobs are the fundamental storage unit for file content in Git. This is a low-level operation typically used when constructing commits programmatically. For simple file operations, use create_or_update_file instead.",
	}
}

func (h *CreateBlobHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get tree object (files, dirs, SHAs, permissions). Use when: exploring repo structure, analyzing directory layout, traversing Git objects.",
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
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "The SHA hash of the tree object (get from commits or other trees)",
					"example":     "9fb037999f264ba9a7fc6274d15fa3ae2ab98312",
					"pattern":     "^[a-f0-9]{40}$",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Return tree recursively including all subdirectories (may be truncated for large trees)",
					"default":     false,
					"example":     true,
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
		},
		Metadata: map[string]interface{}{
			"category":    "git_operations",
			"scopes":      []string{"repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"sha":       "9fb037999f264ba9a7fc6274d15fa3ae2ab98312",
			"url":       "https://api.github.com/repos/owner/repo/git/trees/9fb037999f",
			"truncated": false,
			"tree": []map[string]interface{}{
				{
					"path": "README.md",
					"mode": "100644",
					"type": "blob",
					"sha":  "3a0f86fb8db8eea7ccbb9a95f325ddbedfb25e15",
					"size": 1024,
				},
				{
					"path": "src",
					"mode": "040000",
					"type": "tree",
					"sha":  "f93e3a1a1525fb5b91020git da86fe9fd1",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the tree SHA exists. Get valid tree SHAs from commits or parent trees.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have read access to the repository.",
			},
		},
		ExtendedHelp: "Retrieves a tree object representing a directory structure in Git. Trees contain references to blobs (files) and other trees (subdirectories). Use recursive=true to get the entire directory tree, but be aware that very large trees may be truncated.",
	}
}

func (h *GetTreeHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Create tree from file/dir entries. Use when: advanced Git operations, programmatic commits, custom Git structures.",
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
				"base_tree": map[string]interface{}{
					"type":        "string",
					"description": "SHA of the tree object to use as the base for this tree. If omitted, creates a new tree without a base.",
					"example":     "9fb037999f264ba9a7fc6274d15fa3ae2ab98312",
					"pattern":     "^[a-f0-9]{40}$",
				},
				"tree": map[string]interface{}{
					"type":        "array",
					"description": "Array of tree entries to include in the new tree. Each entry represents a file or subdirectory.",
					"minItems":    1,
					"maxItems":    1000,
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "File or directory path relative to the tree root (e.g., 'README.md', 'src/index.js')",
								"example":     "src/components/Button.tsx",
								"maxLength":   4096,
							},
							"mode": map[string]interface{}{
								"type":        "string",
								"description": "File mode indicating the type and permissions of the entry",
								"enum":        []interface{}{"100644", "100755", "040000", "160000", "120000"},
								"example":     "100644",
								"enumDescriptions": []interface{}{
									"100644: Regular file (non-executable)",
									"100755: Executable file",
									"040000: Subdirectory (tree)",
									"160000: Submodule (commit)",
									"120000: Symbolic link",
								},
							},
							"type": map[string]interface{}{
								"type":        "string",
								"description": "The type of Git object this entry points to",
								"enum":        []interface{}{"blob", "tree", "commit"},
								"example":     "blob",
								"enumDescriptions": []interface{}{
									"blob: File content",
									"tree: Subdirectory",
									"commit: Submodule reference",
								},
							},
							"sha": map[string]interface{}{
								"type":        "string",
								"description": "SHA hash of an existing Git object to reference. Use this OR content, not both.",
								"example":     "3a0f86fb8db8eea7ccbb9a95f325ddbedfb25e15",
								"pattern":     "^[a-f0-9]{40}$",
							},
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Content for a new file to be created. GitHub will create a blob from this content. Use this OR sha, not both.",
								"example":     "console.log('Hello, World!');",
								"maxLength":   100000000, // 100MB limit for API
							},
						},
						"required": []interface{}{"path", "mode", "type"},
					},
				},
			},
			"required": []interface{}{"owner", "repo", "tree"},
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
			"sha": "cd8274d15fa3ae2ab983129fb037999f264ba9a7",
			"url": "https://api.github.com/repos/octocat/Hello-World/trees/cd8274d15fa3ae2ab983129fb037999f264ba9a7",
			"tree": []interface{}{
				map[string]interface{}{
					"path": "README.md",
					"mode": "100644",
					"type": "blob",
					"size": 132,
					"sha":  "7c258a9869f33c1e1e1f74fbb32f07c86cb5a75b",
					"url":  "https://api.github.com/repos/octocat/Hello-World/git/blobs/7c258a9869f33c1e1e1f74fbb32f07c86cb5a75b",
				},
			},
			"truncated": false,
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Repository or base tree SHA not found",
				"solution": "Verify the repository exists and the base_tree SHA is valid",
			},
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid tree entry (e.g., wrong mode for type, invalid SHA)",
				"solution": "Ensure mode matches type (100644 for blob, 040000 for tree) and SHAs exist",
			},
			{
				"error":    "409 Conflict",
				"cause":    "Git database temporarily unavailable",
				"solution": "Retry the request after a short delay",
			},
		},
		ExtendedHelp: `Creates a new tree object in the Git database. Trees are fundamental Git objects that represent directory structures.

Key concepts:
- Trees contain pointers to blobs (files) and other trees (subdirectories)
- Each tree entry has a mode that determines its type and permissions
- You can either reference existing objects by SHA or create new blobs inline with content
- Use base_tree to build upon an existing tree structure

Common use cases:
1. Creating a commit with multiple file changes
2. Building a new directory structure programmatically
3. Preparing changes for a pull request

Example workflow:
1. Create blobs for new file content (or use existing SHAs)
2. Create a tree with entries pointing to those blobs
3. Create a commit pointing to the tree
4. Update a reference to point to the commit`,
	}
}

func (h *CreateTreeHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Retrieve a Git commit object from a repository. Returns the raw Git commit data including tree SHA, parent commits, author, committer, and message.",
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
				"sha": map[string]interface{}{
					"type":        "string",
					"description": "SHA hash of the commit object to retrieve. Can be full 40-character SHA or abbreviated.",
					"example":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					"pattern":     "^[a-f0-9]{4,40}$",
					"minLength":   4,
					"maxLength":   40,
				},
			},
			"required": []interface{}{"owner", "repo", "sha"},
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
			"sha":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
			"url":     "https://api.github.com/repos/octocat/Hello-World/git/commits/6dcb09b5b57875f334f61aebed695e2e4193db5e",
			"message": "Fix all the bugs",
			"author": map[string]interface{}{
				"name":  "Monalisa Octocat",
				"email": "support@github.com",
				"date":  "2014-11-07T22:01:45Z",
			},
			"committer": map[string]interface{}{
				"name":  "Monalisa Octocat",
				"email": "support@github.com",
				"date":  "2014-11-07T22:01:45Z",
			},
			"tree": map[string]interface{}{
				"sha": "691272480426f78a0138979dd3ce63b77f706feb",
				"url": "https://api.github.com/repos/octocat/Hello-World/trees/691272480426f78a0138979dd3ce63b77f706feb",
			},
			"parents": []interface{}{
				map[string]interface{}{
					"sha": "1acc419d4d6a9ce985db7be48c6349a0475975b5",
					"url": "https://api.github.com/repos/octocat/Hello-World/git/commits/1acc419d4d6a9ce985db7be48c6349a0475975b5",
				},
			},
			"verification": map[string]interface{}{
				"verified":  true,
				"reason":    "valid",
				"signature": "-----BEGIN PGP SIGNATURE-----...",
				"payload":   "tree 691272480426f78a0138979dd3ce63b77f706feb...",
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Commit SHA not found in repository",
				"solution": "Verify the SHA exists in the repository and hasn't been garbage collected",
			},
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid SHA format",
				"solution": "Ensure the SHA is a valid hexadecimal string (4-40 characters)",
			},
		},
		ExtendedHelp: `Retrieves the raw Git commit object from a repository. This is different from the Commits API endpoint - it returns the low-level Git commit data.

Key differences from regular commit endpoint:
- Returns raw Git commit object structure
- Includes tree SHA for the commit's file structure
- Shows parent commit SHAs for traversing history
- Contains GPG signature verification details if signed

The commit object includes:
- message: The full commit message
- tree: SHA of the tree object representing the file structure
- parents: Array of parent commit SHAs (empty for initial commit)
- author: Name, email, and timestamp of the author
- committer: Name, email, and timestamp of the committer
- verification: GPG signature details if the commit is signed

Use this when you need:
- Low-level Git object access
- To traverse the Git object graph
- GPG signature verification details
- To create new commits programmatically`,
	}
}

func (h *GetGitCommitHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Create commit with tree, parent(s), message. Use when: programmatic commits, automation, advanced Git operations.",
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
						"type":    "string",
						"pattern": "^[a-f0-9]{40}$",
						"example": "553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
					},
				},
				"author": map[string]interface{}{
					"type":        "object",
					"description": "Author information - the person who originally wrote the code",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Author's name",
							"example":     "Jane Doe",
							"minLength":   1,
							"maxLength":   256,
						},
						"email": map[string]interface{}{
							"type":        "string",
							"description": "Author's email address",
							"format":      "email",
							"example":     "jane@example.com",
						},
					},
				},
			},
			"required": []interface{}{"owner", "repo", "message", "tree"},
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
			"sha":     "7638417db6d59f3c431d3e1f261cc637155684cd",
			"url":     "https://api.github.com/repos/octocat/Hello-World/git/commits/7638417db6d59f3c431d3e1f261cc637155684cd",
			"message": "Fix critical bug in authentication",
			"author": map[string]interface{}{
				"name":  "Jane Doe",
				"email": "jane@example.com",
			},
			"tree": map[string]interface{}{
				"sha": "691272480426f78a0138979dd3ce63b77f706feb",
				"url": "https://api.github.com/repos/octocat/Hello-World/trees/691272480426f78a0138979dd3ce63b77f706feb",
			},
			"parents": []interface{}{
				map[string]interface{}{
					"sha": "553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
					"url": "https://api.github.com/repos/octocat/Hello-World/git/commits/553c2077f0edc3d5dc5d17262f6aa498e69d6f8e",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Tree SHA or parent commit SHA not found",
				"solution": "Verify all SHAs exist in the repository before creating the commit",
			},
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Invalid commit data (e.g., missing required fields, invalid SHA format)",
				"solution": "Ensure all SHAs are valid 40-character hex strings and required fields are present",
			},
		},
		ExtendedHelp: `Creates a new Git commit object in the repository's database. This is a low-level operation that creates the commit but doesn't update any references.

Typical workflow for creating a commit:
1. Create blobs for new/modified files
2. Create a tree that references these blobs
3. Create a commit that points to the tree
4. Update a reference (branch) to point to the new commit

Key concepts:
- Author is who wrote the code
- Parents: Most commits have one parent, merge commits have two, initial commit has none
- Tree: Represents the complete file structure at the time of the commit

Note: This creates the commit object but doesn't update any branch. Use update_ref to point a branch to the new commit.`,
	}
}

func (h *CreateCommitHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Get ref details (SHA, type, object). Use when: validating ref exists, checking HEAD position, resolving tag.",
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
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "The reference path to retrieve. Format: 'heads/branch-name' for branches, 'tags/tag-name' for tags.",
					"example":     "heads/main",
					"pattern":     "^(heads|tags)/[a-zA-Z0-9._/-]+$",
					"minLength":   7,
					"maxLength":   256,
				},
			},
			"required": []interface{}{"owner", "repo", "ref"},
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
			"ref": "refs/heads/main",
			"url": "https://api.github.com/repos/octocat/Hello-World/git/refs/heads/main",
			"object": map[string]interface{}{
				"type": "commit",
				"sha":  "aa218f56b14c9653891f9e74264a383fa43fefbd",
				"url":  "https://api.github.com/repos/octocat/Hello-World/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd",
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"cause":    "Reference not found",
				"solution": "Verify the reference exists and use correct format (heads/branch or tags/tag)",
			},
			{
				"error":    "409 Conflict",
				"cause":    "Git database temporarily unavailable",
				"solution": "Retry the request after a short delay",
			},
		},
		ExtendedHelp: `Retrieves a Git reference (branch or tag) from a repository.

Reference format:
- For branches: 'heads/branch-name' (e.g., 'heads/main', 'heads/feature/new-feature')
- For tags: 'tags/tag-name' (e.g., 'tags/v1.0.0', 'tags/release-2024')

The response includes:
- ref: The full reference path (e.g., 'refs/heads/main')
- object: The Git object the reference points to (usually a commit)

Use cases:
- Get the current commit of a branch
- Verify a tag exists and see what it points to
- Check branch state before operations`,
	}
}

func (h *GetRefHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "List all refs with SHA (branches, tags, notes). Use when: auditing refs, finding all branches, listing tags.",
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
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Create ref (branch, tag) pointing to SHA. Use when: creating tag, creating branch programmatically, marking release.",
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
					"description": "SHA of the commit to point the new reference to",
					"example":     "aa218f56b14c9653891f9e74264a383fa43fefbd",
					"pattern":     "^[a-f0-9]{40}$",
				},
			},
			"required": []interface{}{"owner", "repo", "ref", "sha"},
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
			"ref": "refs/heads/feature-branch",
			"url": "https://api.github.com/repos/octocat/Hello-World/git/refs/heads/feature-branch",
			"object": map[string]interface{}{
				"type": "commit",
				"sha":  "aa218f56b14c9653891f9e74264a383fa43fefbd",
				"url":  "https://api.github.com/repos/octocat/Hello-World/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd",
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "422 Unprocessable Entity",
				"cause":    "Reference already exists",
				"solution": "Use update_ref to modify an existing reference, or choose a different name",
			},
			{
				"error":    "404 Not Found",
				"cause":    "SHA not found in repository",
				"solution": "Verify the commit SHA exists in the repository",
			},
			{
				"error":    "422 Invalid Reference",
				"cause":    "Invalid reference format",
				"solution": "Ensure ref starts with 'refs/heads/' for branches or 'refs/tags/' for tags",
			},
		},
		ExtendedHelp: `Creates a new Git reference (branch or tag) in a repository.

Reference format:
- For branches: 'refs/heads/branch-name'
- For tags: 'refs/tags/tag-name'

Common use cases:
- Create a new branch from a commit
- Create a new tag for a release
- Create feature branches programmatically

Note: The reference must not already exist. Use update_ref to modify existing references.`,
	}
}

func (h *CreateRefHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Update ref to point to different SHA. Use when: force-pushing branch, updating tag, changing HEAD.",
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
	client, ok := GetGitHubClientFromContext(ctx)
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
		Description: "Delete ref (branch, tag). Use when: cleaning up merged branches, removing old tags, deleting stale refs.",
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
	client, ok := GetGitHubClientFromContext(ctx)
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
