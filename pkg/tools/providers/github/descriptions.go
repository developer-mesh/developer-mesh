package github

import "strings"

// GetOperationDescription returns a meaningful description for a GitHub operation
func GetOperationDescription(operationName string) string {
	descriptions := map[string]string{
		// Repository operations
		"get_repository":      "Retrieve detailed information about a GitHub repository including stats, settings, and metadata",
		"list_repositories":   "List repositories for a user or organization with filtering and pagination options",
		"create_repository":   "Create a new GitHub repository with specified settings and configuration",
		"update_repository":   "Update repository settings including description, visibility, and features",
		"delete_repository":   "Permanently delete a GitHub repository (requires admin permissions)",
		"fork_repository":     "Create a fork of a repository to your account or organization",
		"watch_repository":    "Subscribe to notifications for all repository activity",
		"unwatch_repository":  "Unsubscribe from repository notifications",
		"get_file_contents":   "Retrieve the contents of a file or directory from a repository",
		"search_repositories": "Search GitHub repositories using advanced query syntax",

		// Issue operations
		"create_issue":       "Create a new issue with title, body, labels, and assignees",
		"get_issue":          "Retrieve detailed information about a specific issue",
		"update_issue":       "Update issue properties including state, labels, and assignees",
		"list_issues":        "List repository issues with filtering by state, labels, and assignees",
		"search_issues":      "Search for issues across repositories using GitHub's search syntax",
		"lock_issue":         "Lock an issue to prevent further comments (requires write access)",
		"unlock_issue":       "Unlock a previously locked issue to allow comments",
		"add_issue_comment":  "Add a comment to an existing issue",
		"get_issue_comments": "Retrieve all comments for a specific issue",
		"get_issue_events":   "Get the event timeline showing all actions taken on an issue",
		"get_issue_timeline": "Get a detailed timeline of all issue activity including comments and state changes",

		// Pull request operations
		"create_pull_request":              "Create a new pull request from a branch to another branch",
		"get_pull_request":                 "Retrieve detailed information about a pull request",
		"update_pull_request":              "Update pull request title, body, or state",
		"list_pull_requests":               "List pull requests with filtering by state, head, and base",
		"search_pull_requests":             "Search for pull requests using GitHub's search syntax",
		"merge_pull_request":               "Merge a pull request using specified merge method",
		"get_pull_request_diff":            "Get the diff of changes in a pull request",
		"get_pull_request_files":           "List all files changed in a pull request",
		"get_pull_request_reviews":         "Get all reviews for a pull request",
		"get_pull_request_review_comments": "Get review comments on a pull request's code",
		"add_pull_request_review_comment":  "Add a review comment to specific lines in a pull request",
		"create_pull_request_review":       "Start a new review on a pull request",
		"submit_pull_request_review":       "Submit a pending pull request review with approval status",
		"update_pull_request_branch":       "Update a pull request's branch with the latest base branch changes",

		// GitHub Actions operations
		"list_workflows":           "List all GitHub Actions workflows in a repository",
		"get_workflow_run":         "Get details about a specific workflow run",
		"list_workflow_runs":       "List workflow runs with filtering by workflow, branch, and status",
		"list_workflow_jobs":       "List all jobs in a workflow run",
		"run_workflow":             "Manually trigger a workflow dispatch event",
		"cancel_workflow_run":      "Cancel a running workflow",
		"rerun_workflow_run":       "Re-run an entire workflow",
		"rerun_failed_jobs":        "Re-run only the failed jobs in a workflow",
		"get_workflow_run_logs":    "Download logs for a workflow run",
		"delete_workflow_run_logs": "Delete logs for a workflow run",
		"get_workflow_run_usage":   "Get billable usage minutes for a workflow run",
		"get_job_logs":             "Download logs for a specific job in a workflow run",
		"list_artifacts":           "List artifacts produced by workflow runs",
		"download_artifact":        "Download a specific workflow artifact",

		// Security operations
		"list_code_scanning_alerts":       "List code scanning security alerts for a repository",
		"get_code_scanning_alert":         "Get details about a specific code scanning alert",
		"update_code_scanning_alert":      "Update the status of a code scanning alert",
		"list_dependabot_alerts":          "List Dependabot security alerts for vulnerable dependencies",
		"get_dependabot_alert":            "Get details about a specific Dependabot alert",
		"update_dependabot_alert":         "Update the status of a Dependabot alert",
		"list_secret_scanning_alerts":     "List alerts for exposed secrets in a repository",
		"get_secret_scanning_alert":       "Get details about a specific secret scanning alert",
		"update_secret_scanning_alert":    "Update the status of a secret scanning alert",
		"list_secret_scanning_locations":  "List all locations where a secret was detected",
		"list_security_advisories":        "List security advisories for a repository",
		"list_global_security_advisories": "List global security advisories from GitHub's database",

		// Git operations
		"get_commit":     "Get detailed information about a specific commit",
		"list_commits":   "List commits with filtering by author, date range, and path",
		"create_commit":  "Create a new commit programmatically",
		"get_git_commit": "Get a raw Git commit object",
		"create_blob":    "Create a blob object containing file contents",
		"get_blob":       "Retrieve a blob object by SHA",
		"create_tree":    "Create a tree object representing a directory structure",
		"get_tree":       "Retrieve a tree object showing directory contents",
		"create_ref":     "Create a new Git reference (branch or tag)",
		"get_ref":        "Get information about a Git reference",
		"update_ref":     "Update a Git reference to point to a different commit",
		"delete_ref":     "Delete a Git reference",
		"list_refs":      "List all Git references in a repository",
		"push_files":     "Push multiple files to a repository in a single commit",

		// Branch and tag operations
		"list_branches":      "List all branches in a repository",
		"create_branch":      "Create a new branch from a specific commit or branch",
		"get_tag":            "Get information about a specific tag",
		"list_tags":          "List all tags in a repository",
		"create_release":     "Create a new GitHub release with release notes and assets",
		"list_releases":      "List all releases for a repository",
		"get_latest_release": "Get the latest published release for a repository",
		"get_release_by_tag": "Get a release by its tag name",

		// File operations
		"create_or_update_file": "Create a new file or update an existing file in a repository",
		"delete_file":           "Delete a file from a repository with a commit message",

		// Organization operations
		"get_organization":     "Get detailed information about a GitHub organization",
		"list_organizations":   "List organizations for the authenticated user",
		"search_organizations": "Search for organizations using GitHub's search syntax",
		"get_teams":            "List all teams in an organization",
		"list_teams":           "List teams with filtering and pagination",
		"get_team_members":     "List all members of a specific team",

		// User and collaboration operations
		"get_me":                    "Get information about the authenticated user",
		"search_users":              "Search for users using GitHub's search syntax",
		"list_notifications":        "List all notifications for the authenticated user",
		"mark_notification_as_read": "Mark a notification as read",
		"create_gist":               "Create a new GitHub Gist with one or more files",
		"get_gist":                  "Retrieve a specific Gist by ID",
		"update_gist":               "Update an existing Gist's files or description",
		"delete_gist":               "Delete a Gist permanently",
		"list_gists":                "List Gists for a user or the authenticated user",
		"star_gist":                 "Star a Gist to save it to your starred list",
		"unstar_gist":               "Remove a Gist from your starred list",

		// Search operations
		"search_code": "Search for code across GitHub repositories using advanced query syntax",

		// GraphQL operations
		"repository_details_graphql":      "Get comprehensive repository details using GitHub's GraphQL API",
		"issue_create_graphql":            "Create an issue using GitHub's GraphQL API for enhanced features",
		"issues_list_graphql":             "List issues with advanced filtering using GraphQL",
		"pull_request_create_graphql":     "Create a pull request using GraphQL for additional options",
		"pull_request_merge_graphql":      "Merge a pull request using GraphQL with advanced merge options",
		"pull_request_review_add_graphql": "Add a pull request review using GraphQL",
		"search_issues_prs_graphql":       "Search issues and pull requests using GraphQL for richer results",

		// Discussion operations
		"discussions_list":           "List discussions in a repository with category filtering",
		"discussion_get":             "Get detailed information about a specific discussion",
		"discussion_comments_get":    "Retrieve all comments for a discussion thread",
		"discussion_categories_list": "List all discussion categories for a repository",
	}

	if desc, ok := descriptions[operationName]; ok {
		return desc
	}

	// Fallback for unknown operations
	return "Execute " + operationName + " operation on GitHub"
}

// GetOperationMetadata returns additional metadata for operations
func GetOperationMetadata(operationName string) map[string]interface{} {
	metadata := map[string]interface{}{
		"destructive": false,
		"scopes":      []string{},
		"rateLimit":   "standard",
	}

	// Mark destructive operations
	destructiveOps := []string{
		"delete_repository",
		"delete_file",
		"delete_ref",
		"delete_gist",
		"delete_workflow_run_logs",
	}

	for _, op := range destructiveOps {
		if op == operationName {
			metadata["destructive"] = true
			break
		}
	}

	// Add required scopes based on operation type
	switch operationName {
	case "create_repository", "delete_repository", "update_repository":
		metadata["scopes"] = []string{"repo"}
	case "create_issue", "update_issue", "add_issue_comment":
		metadata["scopes"] = []string{"repo", "public_repo"}
	case "create_pull_request", "merge_pull_request", "update_pull_request":
		metadata["scopes"] = []string{"repo", "public_repo"}
	case "run_workflow", "cancel_workflow_run":
		metadata["scopes"] = []string{"actions"}
	case "create_gist", "update_gist", "delete_gist":
		metadata["scopes"] = []string{"gist"}
	case "list_notifications", "mark_notification_as_read":
		metadata["scopes"] = []string{"notifications"}
	case "get_organization", "list_teams", "get_team_members":
		metadata["scopes"] = []string{"read:org"}
	default:
		metadata["scopes"] = []string{"repo"}
	}

	// Mark operations with higher rate limits
	searchOps := []string{
		"search_repositories",
		"search_issues",
		"search_pull_requests",
		"search_code",
		"search_users",
		"search_organizations",
	}

	for _, op := range searchOps {
		if op == operationName {
			metadata["rateLimit"] = "search"
			break
		}
	}

	return metadata
}

// GetOperationResponseExample returns example response for operations
func GetOperationResponseExample(operationName string) interface{} {
	examples := map[string]interface{}{
		// Repository operations
		"get_repository": map[string]interface{}{
			"id":               123456789,
			"name":             "example-repo",
			"full_name":        "owner/example-repo",
			"private":          false,
			"description":      "An example repository",
			"language":         "Go",
			"stargazers_count": 42,
			"forks_count":      10,
		},
		"list_repositories": []map[string]interface{}{
			{
				"id":       123456789,
				"name":     "repo1",
				"private":  false,
				"language": "Go",
			},
			{
				"id":       987654321,
				"name":     "repo2",
				"private":  true,
				"language": "Python",
			},
		},
		"create_repository": map[string]interface{}{
			"id":         123456789,
			"name":       "new-repo",
			"full_name":  "owner/new-repo",
			"private":    true,
			"created_at": "2024-01-01T00:00:00Z",
		},

		// Issue operations
		"create_issue": map[string]interface{}{
			"id":     1,
			"number": 42,
			"state":  "open",
			"title":  "Example issue",
			"body":   "Issue description",
			"user": map[string]interface{}{
				"login": "username",
			},
		},
		"list_issues": []map[string]interface{}{
			{
				"id":     1,
				"number": 42,
				"state":  "open",
				"title":  "Bug report",
			},
			{
				"id":     2,
				"number": 43,
				"state":  "closed",
				"title":  "Feature request",
			},
		},

		// Pull request operations
		"create_pull_request": map[string]interface{}{
			"id":     1,
			"number": 100,
			"state":  "open",
			"title":  "Add new feature",
			"head": map[string]interface{}{
				"ref": "feature-branch",
			},
			"base": map[string]interface{}{
				"ref": "main",
			},
		},
		"merge_pull_request": map[string]interface{}{
			"sha":     "6dcb09b5b57875f334f61aebed695e2e4193db5e",
			"merged":  true,
			"message": "Pull Request successfully merged",
		},

		// Workflow operations
		"list_workflows": map[string]interface{}{
			"total_count": 2,
			"workflows": []map[string]interface{}{
				{
					"id":    161335,
					"name":  "CI",
					"state": "active",
				},
				{
					"id":    161336,
					"name":  "Deploy",
					"state": "active",
				},
			},
		},
		"run_workflow": map[string]interface{}{
			"message": "Workflow dispatch event triggered successfully",
		},

		// Search operations
		"search_repositories": map[string]interface{}{
			"total_count": 40,
			"items": []map[string]interface{}{
				{
					"id":       123,
					"name":     "awesome-project",
					"language": "Go",
					"stars":    1000,
				},
			},
		},
		"search_code": map[string]interface{}{
			"total_count": 5,
			"items": []map[string]interface{}{
				{
					"name": "example.go",
					"path": "src/example.go",
					"repository": map[string]interface{}{
						"name": "example-repo",
					},
				},
			},
		},

		// Generic list response for operations without specific examples
		"default_list": []map[string]interface{}{
			{"id": 1, "name": "item1"},
			{"id": 2, "name": "item2"},
		},

		// Generic single item response
		"default_get": map[string]interface{}{
			"id":   1,
			"name": "example",
		},

		// Generic success response
		"default_success": map[string]interface{}{
			"success": true,
			"message": "Operation completed successfully",
		},
	}

	// Return specific example if available
	if example, ok := examples[operationName]; ok {
		return example
	}

	// Return appropriate default based on operation type
	if strings.HasPrefix(operationName, "list_") || strings.HasPrefix(operationName, "search_") {
		return examples["default_list"]
	}
	if strings.HasPrefix(operationName, "get_") {
		return examples["default_get"]
	}
	if strings.HasPrefix(operationName, "create_") || strings.HasPrefix(operationName, "update_") {
		return examples["default_success"]
	}

	// Default response
	return examples["default_success"]
}
