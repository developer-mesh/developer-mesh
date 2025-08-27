package gitlab

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// GetGitLabAIDefinitions returns AI-optimized definitions for GitLab tools
func GetGitLabAIDefinitions(enabledModules map[GitLabModule]bool) []providers.AIOptimizedToolDefinition {
	var definitions []providers.AIOptimizedToolDefinition

	// Projects definitions
	if enabledModules[ModuleProjects] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_projects",
			DisplayName: "GitLab Projects",
			Category:    "Projects",
			Subcategory: "Management",
			Description: "Manage GitLab projects including creation, configuration, and search operations",
			DetailedHelp: `GitLab Projects enable you to:
- Create and configure new projects
- Search and list projects
- Manage project settings and visibility
- Control access and permissions
- Configure project features
- Archive or delete projects`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List all projects you own",
					Input: map[string]interface{}{
						"action": "list",
						"owned":  true,
					},
					Explanation: "Retrieves all projects where you are the owner",
				},
				{
					Scenario: "Search for API projects",
					Input: map[string]interface{}{
						"action": "list",
						"search": "api",
					},
					Explanation: "Searches for projects containing 'api' in name or description",
				},
				{
					Scenario: "Get specific project details",
					Input: map[string]interface{}{
						"action": "get",
						"id":     "gitlab-org/gitlab",
					},
					Explanation: "Retrieves detailed information about the gitlab-org/gitlab project",
				},
			},
			SemanticTags: []string{
				"project", "repository", "repo", "gitlab", "version-control",
				"source-code", "codebase", "workspace",
			},
			CommonPhrases: []string{
				"list projects", "find project", "search repositories",
				"create project", "get project info", "my projects",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform (list, get, create)",
						Examples:    []interface{}{"list", "get", "create"},
					},
					"id": {
						Type:        "string",
						Description: "Project ID or URL-encoded path",
						Examples:    []interface{}{"123", "group/project"},
					},
					"search": {
						Type:        "string",
						Description: "Search term for filtering projects",
						Examples:    []interface{}{"api", "frontend"},
					},
					"owned": {
						Type:        "boolean",
						Description: "Filter to only owned projects",
						Examples:    []interface{}{true, false},
					},
				},
				Required: []string{"action"},
			},
		})
	}

	// Issues definitions
	if enabledModules[ModuleIssues] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_issues",
			DisplayName: "GitLab Issues",
			Category:    "Issue Tracking",
			Subcategory: "Management",
			Description: "Create, manage, and track issues in GitLab projects",
			DetailedHelp: `GitLab Issues enable you to:
- Create new issues for bugs, features, or tasks
- Track issue status and progress
- Assign issues to team members
- Label and categorize issues
- Link issues to merge requests
- Add comments and discussions`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List open bugs in a project",
					Input: map[string]interface{}{
						"action": "list",
						"id":     "my-project",
						"state":  "opened",
						"labels": "bug",
					},
					Explanation: "Retrieves all open issues labeled as bugs",
				},
				{
					Scenario: "Create a new bug report",
					Input: map[string]interface{}{
						"action":      "create",
						"id":          "my-project",
						"title":       "Login fails with special characters",
						"description": "Users cannot login when password contains @",
						"labels":      "bug,authentication",
					},
					Explanation: "Creates a new issue reporting a login bug",
				},
			},
			SemanticTags: []string{
				"issue", "bug", "ticket", "problem", "task",
				"feature-request", "tracking", "gitlab",
			},
			CommonPhrases: []string{
				"create issue", "report bug", "list issues",
				"open issues", "track problem", "file ticket",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform (list, get, create, update)",
						Examples:    []interface{}{"list", "create", "get"},
					},
					"id": {
						Type:        "string",
						Description: "Project ID or URL-encoded path",
						Examples:    []interface{}{"123", "group/project"},
					},
					"issue_iid": {
						Type:        "integer",
						Description: "Issue internal ID",
						Examples:    []interface{}{42, 123},
					},
					"title": {
						Type:        "string",
						Description: "Issue title",
						Examples:    []interface{}{"Bug: Login fails", "Feature: Dark mode"},
					},
					"description": {
						Type:        "string",
						Description: "Detailed issue description",
						Examples:    []interface{}{"Steps to reproduce:\n1. Go to login\n2. Enter password with @"},
					},
					"state": {
						Type:        "string",
						Description: "Issue state filter",
						Examples:    []interface{}{"opened", "closed", "all"},
					},
					"labels": {
						Type:        "string",
						Description: "Comma-separated label names",
						Examples:    []interface{}{"bug", "feature,ui"},
					},
				},
				Required: []string{"action", "id"},
			},
		})
	}

	// Merge Requests definitions
	if enabledModules[ModuleMergeRequests] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_merge_requests",
			DisplayName: "GitLab Merge Requests",
			Category:    "Code Review",
			Subcategory: "Management",
			Description: "Create and manage merge requests for code review and integration",
			DetailedHelp: `GitLab Merge Requests enable you to:
- Submit code for review
- Request code merges between branches
- Review and comment on code changes
- Approve or reject changes
- Monitor CI/CD pipeline status
- Merge code with various strategies`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List open merge requests for main branch",
					Input: map[string]interface{}{
						"action":        "list",
						"id":            "my-project",
						"state":         "opened",
						"target_branch": "main",
					},
					Explanation: "Shows all open MRs targeting the main branch",
				},
				{
					Scenario: "Create a merge request",
					Input: map[string]interface{}{
						"action":        "create",
						"id":            "my-project",
						"source_branch": "feature/new-api",
						"target_branch": "main",
						"title":         "Add new authentication API",
						"description":   "Implements OAuth 2.0 authentication",
					},
					Explanation: "Creates an MR to merge feature branch into main",
				},
			},
			SemanticTags: []string{
				"merge-request", "pull-request", "mr", "pr",
				"code-review", "merge", "branch", "gitlab",
			},
			CommonPhrases: []string{
				"create merge request", "open MR", "submit PR",
				"review code", "merge branch", "list MRs",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform",
						Examples:    []interface{}{"list", "create", "get", "merge"},
					},
					"id": {
						Type:        "string",
						Description: "Project ID or URL-encoded path",
						Examples:    []interface{}{"123", "group/project"},
					},
					"merge_request_iid": {
						Type:        "integer",
						Description: "Merge request internal ID",
						Examples:    []interface{}{42, 123},
					},
					"source_branch": {
						Type:        "string",
						Description: "Source branch name",
						Examples:    []interface{}{"feature/new-api", "fix/bug-123"},
					},
					"target_branch": {
						Type:        "string",
						Description: "Target branch name",
						Examples:    []interface{}{"main", "develop"},
					},
					"title": {
						Type:        "string",
						Description: "MR title",
						Examples:    []interface{}{"Add OAuth support", "Fix memory leak"},
					},
					"state": {
						Type:        "string",
						Description: "MR state filter",
						Examples:    []interface{}{"opened", "closed", "merged"},
					},
				},
				Required: []string{"action", "id"},
			},
		})
	}

	// CI/CD Pipeline definitions
	if enabledModules[ModulePipelines] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_pipelines",
			DisplayName: "GitLab CI/CD Pipelines",
			Category:    "CI/CD",
			Subcategory: "Pipeline Management",
			Description: "Manage and monitor GitLab CI/CD pipelines for automated builds and deployments",
			DetailedHelp: `GitLab Pipelines enable you to:
- View pipeline execution status
- Trigger manual pipeline runs
- Monitor build and deployment stages
- Review pipeline logs and artifacts
- Retry failed pipelines
- Cancel running pipelines`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List recent pipeline runs",
					Input: map[string]interface{}{
						"action": "list",
						"id":     "my-project",
						"ref":    "main",
					},
					Explanation: "Shows recent pipeline executions for main branch",
				},
				{
					Scenario: "Trigger a deployment pipeline",
					Input: map[string]interface{}{
						"action": "trigger",
						"id":     "my-project",
						"ref":    "main",
						"variables": map[string]string{
							"DEPLOY_ENV": "production",
						},
					},
					Explanation: "Manually triggers a pipeline with deployment variables",
				},
			},
			SemanticTags: []string{
				"pipeline", "ci", "cd", "build", "deploy",
				"automation", "gitlab-ci", "continuous-integration",
			},
			CommonPhrases: []string{
				"run pipeline", "trigger build", "deploy to production",
				"check pipeline status", "view builds", "CI/CD status",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform",
						Examples:    []interface{}{"list", "get", "trigger", "cancel"},
					},
					"id": {
						Type:        "string",
						Description: "Project ID or URL-encoded path",
						Examples:    []interface{}{"123", "group/project"},
					},
					"pipeline_id": {
						Type:        "integer",
						Description: "Pipeline ID",
						Examples:    []interface{}{12345},
					},
					"ref": {
						Type:        "string",
						Description: "Git ref (branch/tag)",
						Examples:    []interface{}{"main", "v1.0.0"},
					},
					"status": {
						Type:        "string",
						Description: "Pipeline status filter",
						Examples:    []interface{}{"success", "failed", "running"},
					},
					"variables": {
						Type:        "object",
						Description: "Pipeline variables",
						Examples:    []interface{}{map[string]string{"ENV": "prod"}},
					},
				},
				Required: []string{"action", "id"},
			},
		})
	}

	// Repository definitions
	if enabledModules[ModuleRepositories] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_repository",
			DisplayName: "GitLab Repository",
			Category:    "Version Control",
			Subcategory: "Repository Management",
			Description: "Manage GitLab repository branches, tags, and commits",
			DetailedHelp: `GitLab Repository management enables you to:
- List and search branches
- View commit history
- Manage tags and releases
- Compare branches and commits
- View file contents
- Manage branch protection rules`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List all branches",
					Input: map[string]interface{}{
						"action": "list_branches",
						"id":     "my-project",
					},
					Explanation: "Retrieves all branches in the repository",
				},
				{
					Scenario: "Get recent commits",
					Input: map[string]interface{}{
						"action":   "list_commits",
						"id":       "my-project",
						"ref_name": "main",
					},
					Explanation: "Shows recent commits on the main branch",
				},
			},
			SemanticTags: []string{
				"repository", "git", "branch", "commit", "tag",
				"version-control", "source-control", "gitlab",
			},
			CommonPhrases: []string{
				"list branches", "show commits", "get tags",
				"branch history", "recent changes", "code history",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Repository operation",
						Examples:    []interface{}{"list_branches", "list_commits", "list_tags"},
					},
					"id": {
						Type:        "string",
						Description: "Project ID or URL-encoded path",
						Examples:    []interface{}{"123", "group/project"},
					},
					"ref_name": {
						Type:        "string",
						Description: "Branch or tag name",
						Examples:    []interface{}{"main", "develop"},
					},
					"search": {
						Type:        "string",
						Description: "Search pattern",
						Examples:    []interface{}{"feature", "v1"},
					},
				},
				Required: []string{"action", "id"},
			},
		})
	}

	// Jobs definitions
	if enabledModules[ModuleJobs] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_jobs",
			DisplayName: "GitLab CI/CD Jobs",
			Category:    "CI/CD",
			Subcategory: "Job Management",
			Description: "Monitor and manage CI/CD job execution in GitLab pipelines",
			DetailedHelp: `GitLab Jobs enable you to:
- View job execution status
- Review job logs and artifacts
- Retry failed jobs
- Cancel running jobs
- Download job artifacts
- Monitor job performance`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List failed jobs",
					Input: map[string]interface{}{
						"action": "list",
						"id":     "my-project",
						"scope":  []string{"failed"},
					},
					Explanation: "Shows all failed jobs in the project",
				},
			},
			SemanticTags: []string{
				"job", "build", "test", "deploy", "ci",
				"task", "execution", "gitlab-ci",
			},
			CommonPhrases: []string{
				"check job status", "failed jobs", "running jobs",
				"job logs", "retry job", "build status",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Job operation",
						Examples:    []interface{}{"list", "get", "retry", "cancel"},
					},
					"id": {
						Type:        "string",
						Description: "Project ID or URL-encoded path",
						Examples:    []interface{}{"123", "group/project"},
					},
					"job_id": {
						Type:        "integer",
						Description: "Job ID",
						Examples:    []interface{}{98765},
					},
					"scope": {
						Type:        "array",
						Description: "Job status scope",
						Examples:    []interface{}{[]string{"failed"}, []string{"running", "pending"}},
					},
				},
				Required: []string{"action", "id"},
			},
		})
	}

	// Groups definitions
	if enabledModules[ModuleGroups] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_groups",
			DisplayName: "GitLab Groups",
			Category:    "Organization",
			Subcategory: "Group Management",
			Description: "Manage GitLab groups and organizational structure",
			DetailedHelp: `GitLab Groups enable you to:
- Organize projects into groups
- Manage group membership
- Control group-level permissions
- Share resources across projects
- Configure group settings
- Create subgroups and hierarchies`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List all groups",
					Input: map[string]interface{}{
						"action": "list",
					},
					Explanation: "Retrieves all accessible groups",
				},
				{
					Scenario: "Get group details with projects",
					Input: map[string]interface{}{
						"action":        "get",
						"id":            "my-group",
						"with_projects": true,
					},
					Explanation: "Shows group information including its projects",
				},
			},
			SemanticTags: []string{
				"group", "organization", "team", "namespace",
				"hierarchy", "gitlab",
			},
			CommonPhrases: []string{
				"list groups", "my groups", "group projects",
				"organization structure", "team groups",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Group operation",
						Examples:    []interface{}{"list", "get", "create"},
					},
					"id": {
						Type:        "string",
						Description: "Group ID or URL-encoded path",
						Examples:    []interface{}{"123", "my-group"},
					},
					"search": {
						Type:        "string",
						Description: "Search groups by name",
						Examples:    []interface{}{"engineering", "devops"},
					},
					"owned": {
						Type:        "boolean",
						Description: "Filter to owned groups only",
						Examples:    []interface{}{true, false},
					},
					"with_projects": {
						Type:        "boolean",
						Description: "Include group projects",
						Examples:    []interface{}{true, false},
					},
				},
				Required: []string{"action"},
			},
		})
	}

	// Users definitions
	if enabledModules[ModuleUsers] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "gitlab_user",
			DisplayName: "GitLab User",
			Category:    "Users",
			Subcategory: "User Management",
			Description: "Get information about GitLab users",
			DetailedHelp: `GitLab User operations enable you to:
- Get current user information
- Verify authentication status
- View user profile details
- Check user permissions`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Get current user info",
					Input: map[string]interface{}{
						"action": "current",
					},
					Explanation: "Retrieves information about the authenticated user",
				},
			},
			SemanticTags: []string{
				"user", "profile", "account", "authentication",
				"current-user", "gitlab",
			},
			CommonPhrases: []string{
				"who am i", "current user", "my profile",
				"user info", "account details",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "User operation",
						Examples:    []interface{}{"current", "get"},
					},
				},
				Required: []string{"action"},
			},
		})
	}

	return definitions
}
