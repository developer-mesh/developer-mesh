package github

// Common schema patterns for GitHub tools to avoid duplication and ensure consistency

// OwnerSchema returns the schema for repository owner parameter
func OwnerSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft', 'octocat')",
		"example":     "facebook",
		"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
		"minLength":   1,
		"maxLength":   39,
	}
}

// RepoSchema returns the schema for repository name parameter
func RepoSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": "Repository name (e.g., 'react', 'kubernetes', 'tensorflow')",
		"example":     "react",
		"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
		"minLength":   1,
		"maxLength":   100,
	}
}

// IssueNumberSchema returns the schema for issue number parameter
func IssueNumberSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": "Issue number (e.g., 123, 4567)",
		"example":     123,
		"minimum":     1,
	}
}

// PullNumberSchema returns the schema for pull request number parameter
func PullNumberSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": "Pull request number (e.g., 123, 4567)",
		"example":     123,
		"minimum":     1,
	}
}

// PaginationSchema returns common pagination parameters
func PaginationSchema() map[string]interface{} {
	return map[string]interface{}{
		"per_page": map[string]interface{}{
			"type":        "integer",
			"description": "Number of results per page (1-100). Defaults to 30",
			"minimum":     1,
			"maximum":     100,
			"default":     30,
			"example":     30,
		},
		"page": map[string]interface{}{
			"type":        "integer",
			"description": "Page number to retrieve (1-based). Defaults to 1",
			"minimum":     1,
			"default":     1,
			"example":     1,
		},
	}
}

// StateSchema returns schema for issue/PR state parameter
func StateSchema(defaultState string, states []string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": "Filter by state. Defaults to '" + defaultState + "'",
		"enum":        states,
		"default":     defaultState,
		"example":     states[0],
	}
}

// LabelsSchema returns schema for labels array parameter
func LabelsSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": "Filter by labels (e.g., ['bug', 'enhancement', 'documentation'])",
		"items": map[string]interface{}{
			"type":      "string",
			"pattern":   "^[a-zA-Z0-9][a-zA-Z0-9 ._-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
			"maxLength": 50,
		},
		"example":     []string{"bug", "enhancement"},
		"maxItems":    20,
		"uniqueItems": true,
	}
}

// AssigneesSchema returns schema for assignees array parameter
func AssigneesSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": "Usernames to assign (e.g., ['octocat', 'defunkt']. Note: assignees must be collaborators)",
		"items": map[string]interface{}{
			"type":      "string",
			"pattern":   "^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$",
			"maxLength": 39,
		},
		"example":     []string{"octocat", "defunkt"},
		"maxItems":    10,
		"uniqueItems": true,
	}
}

// SortDirectionSchema returns schema for sort direction parameter
func SortDirectionSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": "Sort direction. Defaults to 'desc' (newest first)",
		"enum":        []string{"asc", "desc"},
		"default":     "desc",
		"example":     "desc",
	}
}

// CommonMetadata returns common metadata for GitHub operations
func CommonMetadata(scopes []string, category string, requestsPerHour int) map[string]interface{} {
	minimumScopes := []string{"public_repo"}
	if len(scopes) > 0 {
		minimumScopes = []string{scopes[0]} // First scope is usually the minimum
	}

	return map[string]interface{}{
		"requiredScopes":    scopes,
		"minimumScopes":     minimumScopes,
		"rateLimitCategory": category,
		"requestsPerHour":   requestsPerHour,
	}
}

// CommonErrors returns common error responses for GitHub operations
func CommonErrors(operation string) []map[string]interface{} {
	errors := []map[string]interface{}{
		{
			"code":     401,
			"reason":   "Bad credentials or invalid token",
			"solution": "Check your GitHub token and ensure it has the required permissions",
		},
		{
			"code":     403,
			"reason":   "Rate limit exceeded or insufficient permissions",
			"solution": "Wait for rate limit reset or ensure proper authentication scopes",
		},
		{
			"code":     404,
			"reason":   "Repository, issue, or resource not found",
			"solution": "Verify the repository exists and you have permission to access it",
		},
	}

	// Add operation-specific errors
	switch operation {
	case "create", "update":
		errors = append(errors, map[string]interface{}{
			"code":     422,
			"reason":   "Validation failed - invalid or missing parameters",
			"solution": "Check required parameters and ensure values meet GitHub's validation rules",
		})
	case "search":
		errors = append(errors, map[string]interface{}{
			"code":     422,
			"reason":   "Validation failed - invalid search syntax",
			"solution": "Check search query syntax. See GitHub search documentation for valid operators",
		})
	}

	return errors
}

// IssueResponseExample returns example response for issue operations
func IssueResponseExample() map[string]interface{} {
	return map[string]interface{}{
		"success": map[string]interface{}{
			"id":     1,
			"number": 1347,
			"title":  "Found a bug",
			"body":   "I'm having a problem with this.",
			"state":  "open",
			"user": map[string]interface{}{
				"login": "octocat",
			},
			"labels": []map[string]interface{}{
				{
					"name":  "bug",
					"color": "f29513",
				},
			},
			"assignees":  []interface{}{},
			"created_at": "2011-04-22T13:33:48Z",
			"updated_at": "2011-04-22T13:33:48Z",
			"html_url":   "https://github.com/octocat/Hello-World/issues/1347",
		},
		"error": map[string]interface{}{
			"message":           "Not Found",
			"documentation_url": "https://docs.github.com/rest/issues/issues#get-an-issue",
		},
	}
}

// PullRequestResponseExample returns example response for PR operations
func PullRequestResponseExample() map[string]interface{} {
	return map[string]interface{}{
		"success": map[string]interface{}{
			"id":     1,
			"number": 1347,
			"title":  "Amazing new feature",
			"body":   "Please pull these awesome changes in!",
			"state":  "open",
			"user": map[string]interface{}{
				"login": "octocat",
			},
			"head": map[string]interface{}{
				"ref": "new-feature",
			},
			"base": map[string]interface{}{
				"ref": "main",
			},
			"mergeable":  true,
			"merged":     false,
			"created_at": "2011-04-22T13:33:48Z",
			"updated_at": "2011-04-22T13:33:48Z",
			"html_url":   "https://github.com/octocat/Hello-World/pull/1347",
		},
		"error": map[string]interface{}{
			"message":           "Not Found",
			"documentation_url": "https://docs.github.com/rest/pulls/pulls#get-a-pull-request",
		},
	}
}

// RepositoryResponseExample returns example response for repository operations
func RepositoryResponseExample() map[string]interface{} {
	return map[string]interface{}{
		"success": map[string]interface{}{
			"id":        1296269,
			"name":      "Hello-World",
			"full_name": "octocat/Hello-World",
			"private":   false,
			"owner": map[string]interface{}{
				"login": "octocat",
			},
			"description": "This your first repo!",
			"fork":        false,
			"created_at":  "2011-01-26T19:01:12Z",
			"updated_at":  "2011-01-26T19:14:43Z",
			"html_url":    "https://github.com/octocat/Hello-World",
			"clone_url":   "https://github.com/octocat/Hello-World.git",
		},
		"error": map[string]interface{}{
			"message":           "Not Found",
			"documentation_url": "https://docs.github.com/rest/repos/repos#get-a-repository",
		},
	}
}

// ExtendedHelpText returns contextual help for operations
func ExtendedHelpText(operation, resource string) string {
	baseURL := "https://docs.github.com/rest"

	switch resource {
	case "issues":
		switch operation {
		case "get":
			return "This operation retrieves a single issue by number. Issues and pull requests share the same numbering system. See " + baseURL + "/issues/issues#get-an-issue"
		case "list":
			return "This operation lists repository issues with filtering and pagination. Use state, labels, and assignee filters to narrow results. See " + baseURL + "/issues/issues#list-repository-issues"
		case "search":
			return "This operation searches issues across GitHub using powerful query syntax. Use repo:owner/name to limit to specific repositories. See " + baseURL + "/search#search-issues-and-pull-requests"
		case "create":
			return "This operation creates a new issue. Requires write access to the repository. Labels and assignees must exist and be valid. See " + baseURL + "/issues/issues#create-an-issue"
		case "update":
			return "This operation updates an existing issue. Only provided fields are updated. Requires write access. See " + baseURL + "/issues/issues#update-an-issue"
		}
	case "pulls":
		switch operation {
		case "get":
			return "This operation retrieves a single pull request by number. Includes detailed merge status and review information. See " + baseURL + "/pulls/pulls#get-a-pull-request"
		case "list":
			return "This operation lists repository pull requests with filtering options. Use state, head, and base filters for targeted results. See " + baseURL + "/pulls/pulls#list-pull-requests"
		case "create":
			return "This operation creates a new pull request. Head and base branches must exist. Draft PRs can be created for work in progress. See " + baseURL + "/pulls/pulls#create-a-pull-request"
		case "merge":
			return "This operation merges a pull request. Choose merge method: merge (merge commit), squash (squash commits), or rebase (rebase and merge). See " + baseURL + "/pulls/pulls#merge-a-pull-request"
		}
	case "repos":
		switch operation {
		case "get":
			return "This operation retrieves repository information. Public repositories are accessible to anyone, private repositories require appropriate access. See " + baseURL + "/repos/repos#get-a-repository"
		case "list":
			return "This operation lists repositories for a user or organization. Visibility depends on your access level and repository settings. See " + baseURL + "/repos/repos#list-repositories-for-a-user"
		}
	}

	return "See GitHub REST API documentation at " + baseURL + " for detailed information about this operation."
}
