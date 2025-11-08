package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
)

// GitHubToolsetProvider provides GitHub toolsets using the new toolset pattern
type GitHubToolsetProvider struct {
	adapter *GitHubAdapter
}

// NewGitHubToolsetProvider creates a new GitHub toolset provider
func NewGitHubToolsetProvider(adapter *GitHubAdapter) *GitHubToolsetProvider {
	return &GitHubToolsetProvider{
		adapter: adapter,
	}
}

// GetToolsets returns all GitHub toolsets
func (p *GitHubToolsetProvider) GetToolsets() []mcptools.Toolset {
	return []mcptools.Toolset{
		{
			Name:          "github_repos",
			DisplayName:   "GitHub Repositories",
			Description:   "Repository management operations: get, list, create, fork, and manage repositories",
			Category:      string(mcptools.CategoryRepository),
			Icon:          "📦",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "repository", "read", "write"},
			ToolGenerator: p.generateReposTools,
		},
		{
			Name:          "github_issues",
			DisplayName:   "GitHub Issues",
			Description:   "Issue tracking and management: create, update, list, comment, and manage issues",
			Category:      string(mcptools.CategoryIssues),
			Icon:          "🐛",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "issues", "tracking", "read", "write"},
			ToolGenerator: p.generateIssuesTools,
		},
		{
			Name:          "github_pulls",
			DisplayName:   "GitHub Pull Requests",
			Description:   "Pull request operations: create, review, merge, and manage pull requests",
			Category:      string(mcptools.CategoryPullRequests),
			Icon:          "🔄",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "pull_requests", "review", "read", "write"},
			ToolGenerator: p.generatePullsTools,
		},
		{
			Name:          "github_workflows",
			DisplayName:   "GitHub Actions Workflows",
			Description:   "CI/CD workflow operations: run, monitor, and manage GitHub Actions workflows",
			Category:      string(mcptools.CategoryCICD),
			Icon:          "🚀",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "workflows", "ci_cd", "execute", "read"},
			ToolGenerator: p.generateWorkflowsTools,
		},
		{
			Name:          "github_security",
			DisplayName:   "GitHub Security",
			Description:   "Security scanning and alerts: Dependabot, code scanning, secret scanning, security advisories",
			Category:      string(mcptools.CategorySecurity),
			Icon:          "🔐",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "security", "scanning", "read"},
			ToolGenerator: p.generateSecurityTools,
		},
		{
			Name:          "github_code",
			DisplayName:   "GitHub Code",
			Description:   "File and code operations: read, write, update, and delete files in repositories",
			Category:      string(mcptools.CategoryRepository),
			Icon:          "📝",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "files", "code", "read", "write"},
			ToolGenerator: p.generateCodeTools,
		},
		{
			Name:          "github_branches",
			DisplayName:   "GitHub Branches",
			Description:   "Branch management: create, list, and manage repository branches",
			Category:      string(mcptools.CategoryRepository),
			Icon:          "🌿",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "branches", "read", "write"},
			ToolGenerator: p.generateBranchesTools,
		},
		{
			Name:          "github_releases",
			DisplayName:   "GitHub Releases",
			Description:   "Release management: create, list, and manage repository releases",
			Category:      string(mcptools.CategoryRepository),
			Icon:          "🎉",
			Provider:      "github",
			Version:       "1.0.0",
			Tags:          []string{"github", "releases", "read", "write"},
			ToolGenerator: p.generateReleasesTools,
		},
	}
}

// GetToolsetByName returns a specific toolset by name
func (p *GitHubToolsetProvider) GetToolsetByName(name string) (mcptools.Toolset, bool) {
	toolsets := p.GetToolsets()
	for _, ts := range toolsets {
		if ts.Name == name {
			return ts, true
		}
	}
	return mcptools.Toolset{}, false
}

// generateReposTools generates repository toolset tools
func (p *GitHubToolsetProvider) generateReposTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	// Full schema for the unified repos tool
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Repository operation to perform",
				"enum":        []string{"get", "list", "create", "fork", "search"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner (required for get, fork)",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name (required for get, fork)",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Repository name (required for create)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Repository description (optional for create)",
			},
			"private": map[string]interface{}{
				"type":        "boolean",
				"description": "Make repository private (optional for create)",
				"default":     false,
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (required for search)",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_repos",
		Description: "Repository operations: get repository details, list repositories, create new repository, fork repository, or search repositories",
		Category:    string(mcptools.CategoryRepository),
		Tags:        []string{"github", "repository", "read", "write"},
		Handler:     p.createUnifiedHandler("repos"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		// Return minimal schema
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	case mcptools.DetailLevelName:
		// No schema needed
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generateIssuesTools generates issue toolset tools
func (p *GitHubToolsetProvider) generateIssuesTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Issue operation to perform",
				"enum":        []string{"get", "list", "create", "update", "add_comment", "lock", "unlock", "search"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
			"issue_number": map[string]interface{}{
				"type":        "integer",
				"description": "Issue number (required for get, update, add_comment, lock, unlock)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Issue title (required for create, optional for update)",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "Issue body/description",
			},
			"state": map[string]interface{}{
				"type":        "string",
				"description": "Issue state (for update)",
				"enum":        []string{"open", "closed"},
			},
			"comment": map[string]interface{}{
				"type":        "string",
				"description": "Comment text (for add_comment)",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_issues",
		Description: "Issue operations: get issue details, list issues, create issue, update issue, add comments, lock/unlock issues, or search issues",
		Category:    string(mcptools.CategoryIssues),
		Tags:        []string{"github", "issues", "tracking", "read", "write"},
		Handler:     p.createUnifiedHandler("issues"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generatePullsTools generates pull request toolset tools
func (p *GitHubToolsetProvider) generatePullsTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Pull request operation to perform",
				"enum":        []string{"get", "list", "create", "update", "merge", "create_review", "get_files", "get_diff", "search"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
			"pull_number": map[string]interface{}{
				"type":        "integer",
				"description": "Pull request number",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "PR title (for create)",
			},
			"head": map[string]interface{}{
				"type":        "string",
				"description": "Head branch (for create)",
			},
			"base": map[string]interface{}{
				"type":        "string",
				"description": "Base branch (for create)",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "PR description",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_pulls",
		Description: "Pull request operations: get PR details, list PRs, create PR, update PR, merge PR, create reviews, get files/diff, or search PRs",
		Category:    string(mcptools.CategoryPullRequests),
		Tags:        []string{"github", "pull_requests", "review", "read", "write"},
		Handler:     p.createUnifiedHandler("pulls"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generateWorkflowsTools generates workflow toolset tools
func (p *GitHubToolsetProvider) generateWorkflowsTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Workflow operation to perform",
				"enum":        []string{"list_workflows", "list_runs", "get_run", "run", "cancel", "rerun", "get_logs"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
			"workflow_id": map[string]interface{}{
				"type":        "string",
				"description": "Workflow ID or filename",
			},
			"run_id": map[string]interface{}{
				"type":        "integer",
				"description": "Workflow run ID",
			},
			"ref": map[string]interface{}{
				"type":        "string",
				"description": "Git ref (branch/tag) to run workflow on",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_workflows",
		Description: "GitHub Actions workflow operations: list workflows, list runs, get run details, trigger workflow, cancel run, rerun workflow, or get logs",
		Category:    string(mcptools.CategoryCICD),
		Tags:        []string{"github", "workflows", "ci_cd", "execute", "read"},
		Handler:     p.createUnifiedHandler("workflows"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generateSecurityTools generates security toolset tools
func (p *GitHubToolsetProvider) generateSecurityTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Security operation to perform",
				"enum":        []string{"list_dependabot", "list_code_scanning", "list_secret_scanning", "list_advisories", "get_dependabot", "get_code_scanning", "get_secret_scanning"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
			"alert_number": map[string]interface{}{
				"type":        "integer",
				"description": "Alert number (for get operations)",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_security",
		Description: "Security scanning operations: list and view Dependabot alerts, code scanning alerts, secret scanning alerts, and security advisories",
		Category:    string(mcptools.CategorySecurity),
		Tags:        []string{"github", "security", "scanning", "read"},
		Handler:     p.createUnifiedHandler("security"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generateCodeTools generates code/file toolset tools
func (p *GitHubToolsetProvider) generateCodeTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "File operation to perform",
				"enum":        []string{"get_file", "create_file", "update_file", "delete_file", "push_files", "search_code"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path in repository",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "File content (for create/update)",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Branch name",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (for search_code)",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_code",
		Description: "File and code operations: get file contents, create file, update file, delete file, push multiple files, or search code",
		Category:    string(mcptools.CategoryRepository),
		Tags:        []string{"github", "files", "code", "read", "write"},
		Handler:     p.createUnifiedHandler("code"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generateBranchesTools generates branch toolset tools
func (p *GitHubToolsetProvider) generateBranchesTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Branch operation to perform",
				"enum":        []string{"list", "create", "get"},
			},
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "Repository owner",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Branch name",
			},
			"from_branch": map[string]interface{}{
				"type":        "string",
				"description": "Source branch for create",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_branches",
		Description: "Branch operations: list branches, create branch, or get branch details",
		Category:    string(mcptools.CategoryRepository),
		Tags:        []string{"github", "branches", "read", "write"},
		Handler:     p.createUnifiedHandler("branches"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// generateReleasesTools generates release toolset tools
func (p *GitHubToolsetProvider) generateReleasesTools(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
	fullSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"action", "owner", "repo"},
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Release operation to perform",
				"enum":        []string{"list", "get_latest", "get_by_tag", "create"},
			},
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
				"description": "Release tag",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Release name (for create)",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "Release description (for create)",
			},
		},
	}

	tool := mcptools.ToolDefinition{
		Name:        "github_releases",
		Description: "Release operations: list releases, get latest release, get release by tag, or create release",
		Category:    string(mcptools.CategoryRepository),
		Tags:        []string{"github", "releases", "read", "write"},
		Handler:     p.createUnifiedHandler("releases"),
	}

	switch detailLevel {
	case mcptools.DetailLevelFull:
		tool.InputSchema = fullSchema
	case mcptools.DetailLevelDescription:
		tool.InputSchema = map[string]interface{}{
			"type":     "object",
			"required": []string{"action"},
			"properties": map[string]interface{}{
				"action": fullSchema["properties"].(map[string]interface{})["action"],
			},
		}
	}

	return []mcptools.ToolDefinition{tool}, nil
}

// createUnifiedHandler creates a unified handler that routes actions to the appropriate GitHub operation
func (p *GitHubToolsetProvider) createUnifiedHandler(toolsetName string) mcptools.ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		// Parse common parameters
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		action, ok := params["action"].(string)
		if !ok {
			return nil, fmt.Errorf("action parameter is required")
		}

		// Get context ID if present
		contextID, _ := params["context_id"].(string)

		// Route to appropriate handler based on toolset and action
		return p.routeAction(ctx, contextID, toolsetName, action, params)
	}
}

// routeAction routes an action to the appropriate GitHub operation
func (p *GitHubToolsetProvider) routeAction(ctx context.Context, contextID, toolset, action string, params map[string]interface{}) (interface{}, error) {
	// Map toolset + action to the existing ExecuteAction operations
	operationMap := map[string]string{
		// Repos
		"repos/get":    "getRepository",
		"repos/list":   "listRepositories",
		"repos/create": "createRepository",
		"repos/fork":   "forkRepository",
		"repos/search": "searchRepositories",

		// Issues
		"issues/get":         "getIssue",
		"issues/list":        "listIssues",
		"issues/create":      "createIssue",
		"issues/update":      "updateIssue",
		"issues/add_comment": "addIssueComment",
		"issues/lock":        "lockIssue",
		"issues/unlock":      "unlockIssue",
		"issues/search":      "searchIssues",

		// Pulls
		"pulls/get":           "getPullRequest",
		"pulls/list":          "listPullRequests",
		"pulls/create":        "createPullRequest",
		"pulls/update":        "updatePullRequest",
		"pulls/merge":         "mergePullRequest",
		"pulls/create_review": "createPullRequestReview",
		"pulls/get_files":     "getPullRequestFiles",
		"pulls/get_diff":      "getPullRequestDiff",
		"pulls/search":        "searchPullRequests",

		// Workflows
		"workflows/list_workflows": "listWorkflows",
		"workflows/list_runs":      "listWorkflowRuns",
		"workflows/get_run":        "getWorkflowRun",
		"workflows/run":            "runWorkflow",
		"workflows/cancel":         "cancelWorkflowRun",
		"workflows/rerun":          "rerunWorkflowRun",
		"workflows/get_logs":       "getWorkflowRunLogs",

		// Security
		"security/list_dependabot":      "listDependabotAlerts",
		"security/list_code_scanning":   "listCodeScanningAlerts",
		"security/list_secret_scanning": "listSecretScanningAlerts",
		"security/list_advisories":      "listSecurityAdvisories",
		"security/get_dependabot":       "getDependabotAlert",
		"security/get_code_scanning":    "getCodeScanningAlert",
		"security/get_secret_scanning":  "getSecretScanningAlert",

		// Code
		"code/get_file":    "getFileContents",
		"code/create_file": "createOrUpdateFile",
		"code/update_file": "createOrUpdateFile",
		"code/delete_file": "deleteFile",
		"code/push_files":  "pushFiles",
		"code/search_code": "searchCode",

		// Branches
		"branches/list":   "listBranches",
		"branches/create": "createBranch",
		"branches/get":    "getBranch",

		// Releases
		"releases/list":       "listReleases",
		"releases/get_latest": "getLatestRelease",
		"releases/get_by_tag": "getReleaseByTag",
		"releases/create":     "createRelease",
	}

	// Lookup the operation
	key := toolset + "/" + action
	operation, exists := operationMap[key]
	if !exists {
		return nil, fmt.Errorf("unknown action '%s' for toolset '%s'", action, toolset)
	}

	// Call the existing ExecuteAction method
	return p.adapter.ExecuteAction(ctx, contextID, operation, params)
}
