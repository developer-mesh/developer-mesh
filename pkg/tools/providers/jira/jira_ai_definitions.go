package jira

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// GetAIOptimizedDefinitions returns AI-friendly tool definitions for Jira
func (p *JiraProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	var definitions []providers.AIOptimizedToolDefinition

	// Issue Management
	if toolset, exists := p.toolsetRegistry["issues"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "jira_issues",
			DisplayName: "Jira Issue Management",
			Category:    "Issue Tracking",
			Subcategory: "Issue Operations",
			Description: "Create, read, update, and delete Jira issues including stories, bugs, tasks, and epics. Manage issue fields, properties, and metadata.",
			DetailedHelp: `Jira Issue Management capabilities:
- Create new issues with required and optional fields
- Retrieve detailed issue information with expansion options
- Update existing issues with field modifications and operations
- Delete issues with optional subtask deletion
- Support for custom fields and issue types
- Handle issue linking and relationships
- Manage issue priorities, components, and versions
- Work with issue security and permission levels`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new bug report",
					Input: map[string]interface{}{
						"action": "create_issue",
						"fields": map[string]interface{}{
							"project":     map[string]interface{}{"key": "PROJ"},
							"issuetype":   map[string]interface{}{"name": "Bug"},
							"summary":     "Application crashes on login",
							"description": "When users click the login button, the application crashes with a null pointer exception.",
							"priority":    map[string]interface{}{"name": "High"},
							"assignee":    map[string]interface{}{"accountId": "12345"},
							"components":  []map[string]interface{}{{"name": "Authentication"}},
						},
					},
					Explanation: "Creates a high-priority bug in the PROJ project with detailed description and assignment",
				},
				{
					Scenario: "Get detailed issue information",
					Input: map[string]interface{}{
						"action":       "get_issue",
						"issueIdOrKey": "PROJ-123",
						"expand":       []string{"changelog", "renderedFields", "names", "transitions"},
					},
					Explanation: "Retrieves issue PROJ-123 with expanded change history, rendered fields, and available transitions",
				},
				{
					Scenario: "Update issue with fields and comments",
					Input: map[string]interface{}{
						"action":       "update_issue",
						"issueIdOrKey": "PROJ-456",
						"fields": map[string]interface{}{
							"summary": "Updated: Application crashes on login - investigating database connection",
						},
						"update": map[string]interface{}{
							"comment": []map[string]interface{}{
								{
									"add": map[string]interface{}{
										"body": "Investigation shows this might be related to database connection timeout. Looking into configuration.",
									},
								},
							},
						},
					},
					Explanation: "Updates the issue summary and adds a progress comment in a single operation",
				},
				{
					Scenario: "Create an epic with custom fields",
					Input: map[string]interface{}{
						"action": "create_issue",
						"fields": map[string]interface{}{
							"project":           map[string]interface{}{"key": "PROJ"},
							"issuetype":         map[string]interface{}{"name": "Epic"},
							"summary":           "User Authentication Overhaul",
							"description":       "Complete redesign of the authentication system to support SSO and 2FA",
							"customfield_10014": "AUTH-2024-Q2",
							"labels":            []string{"security", "authentication", "epic"},
						},
					},
					Explanation: "Creates an epic with custom Epic Name field and appropriate labels",
				},
			},
			SemanticTags: []string{
				"issue", "ticket", "bug", "story", "task", "epic", "jira",
				"create", "update", "delete", "get", "manage", "track",
			},
			CommonPhrases: []string{
				"create ticket", "update issue", "get issue details", "delete issue",
				"create bug report", "create story", "create task", "manage issues",
				"issue tracking", "bug tracking", "project management",
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "issues"},
					{Action: "read", Resource: "issues"},
					{Action: "update", Resource: "issues"},
					{Action: "delete", Resource: "issues"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 60,
					Description:       "Jira Cloud rate limits: 60 requests per minute for issue operations",
				},
			},
		})
	}

	// Search Operations
	if toolset, exists := p.toolsetRegistry["search"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "jira_search",
			DisplayName: "Jira Search & JQL",
			Category:    "Search",
			Subcategory: "Query Operations",
			Description: "Advanced search capabilities using JQL (Jira Query Language) to find issues, filter results, and generate reports across projects.",
			DetailedHelp: `Jira Search with JQL capabilities:
- Use JQL (Jira Query Language) for powerful, flexible queries
- Search across projects, issue types, statuses, assignees, and dates
- Filter by custom fields, labels, components, and versions
- Support for advanced JQL functions (currentUser(), startOfWeek(), etc.)
- Paginated results for large datasets
- Sort by various fields (created, updated, priority, rank)
- Validate JQL syntax before execution
- Include additional fields with expand parameter`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Find open bugs assigned to current user",
					Input: map[string]interface{}{
						"action":     "search_issues",
						"jql":        "assignee = currentUser() AND type = Bug AND status != Done",
						"maxResults": 25,
						"fields":     []string{"summary", "status", "priority", "created"},
					},
					Explanation: "Searches for open bugs assigned to the authenticated user with specific field selection",
				},
				{
					Scenario: "Find recently created high-priority issues",
					Input: map[string]interface{}{
						"action":  "search_issues",
						"jql":     "project = PROJ AND priority = High AND created >= -7d",
						"orderBy": "created DESC",
						"expand":  []string{"changelog"},
					},
					Explanation: "Finds high-priority issues created in the last 7 days, sorted by creation date with change history",
				},
				{
					Scenario: "Complex query with multiple conditions",
					Input: map[string]interface{}{
						"action":     "search_issues",
						"jql":        "project in (PROJ, TEAM) AND (type = Story OR type = Epic) AND sprint in openSprints() AND labels in (frontend, backend)",
						"maxResults": 50,
						"startAt":    0,
					},
					Explanation: "Advanced query finding stories and epics in active sprints across multiple projects with specific labels",
				},
				{
					Scenario: "Generate report of team activity",
					Input: map[string]interface{}{
						"action":  "search_issues",
						"jql":     "project = PROJ AND assignee in membersOf('jira-developers') AND updated >= -14d",
						"fields":  []string{"assignee", "status", "updated", "summary"},
						"orderBy": "updated DESC",
					},
					Explanation: "Generates a team activity report showing issues updated by team members in the last 2 weeks",
				},
			},
			SemanticTags: []string{
				"search", "jql", "query", "find", "filter", "report",
				"jira-query-language", "advanced-search", "issue-search",
			},
			CommonPhrases: []string{
				"search issues", "find tickets", "jql query", "advanced search",
				"filter issues", "search by assignee", "find open bugs",
				"search by date", "generate report", "query jira",
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "search", Resource: "issues"},
					{Action: "query", Resource: "issues"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 60,
					Description:       "Jira Cloud rate limits: 60 requests per minute for search operations",
				},
			},
		})
	}

	// Comment Management
	if toolset, exists := p.toolsetRegistry["comments"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "jira_comments",
			DisplayName: "Jira Comments",
			Category:    "Collaboration",
			Subcategory: "Issue Comments",
			Description: "Add, retrieve, update, and delete comments on Jira issues. Support for rich text formatting, visibility restrictions, and comment threading.",
			DetailedHelp: `Jira Comments capabilities:
- Add comments with plain text or ADF (Atlassian Document Format)
- Retrieve all comments for an issue with pagination
- Update existing comments with version control
- Delete comments with proper permissions
- Support for comment visibility (roles, groups)
- Rich text formatting and mentions (@username)
- Comment properties for additional metadata
- Automatic conversion from plain text to ADF format`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Add a progress update comment",
					Input: map[string]interface{}{
						"action":       "add_comment",
						"issueIdOrKey": "PROJ-123",
						"body":         "Investigation complete. The issue is caused by a race condition in the authentication module. Fix will be deployed in the next release.",
					},
					Explanation: "Adds a plain text comment with project progress information",
				},
				{
					Scenario: "Add a comment visible only to developers",
					Input: map[string]interface{}{
						"action":       "add_comment",
						"issueIdOrKey": "PROJ-456",
						"body":         "Technical details: The bug occurs when ThreadLocal variables are not properly cleaned up. Requires code review @john.doe",
						"visibility": map[string]interface{}{
							"type":  "role",
							"value": "Developers",
						},
					},
					Explanation: "Adds a comment with role-based visibility and mentions a specific user",
				},
				{
					Scenario: "Get all comments for an issue",
					Input: map[string]interface{}{
						"action":       "get_comments",
						"issueIdOrKey": "PROJ-789",
						"expand":       []string{"renderedBody", "properties"},
						"orderBy":      "created",
					},
					Explanation: "Retrieves all comments with rendered HTML and properties, ordered by creation time",
				},
				{
					Scenario: "Update an existing comment",
					Input: map[string]interface{}{
						"action":       "update_comment",
						"issueIdOrKey": "PROJ-123",
						"commentId":    "10001",
						"body":         "Updated: Investigation complete. The issue is resolved and deployed to production.",
						"visibility": map[string]interface{}{
							"type":  "group",
							"value": "jira-administrators",
						},
					},
					Explanation: "Updates comment content and changes visibility to administrators group",
				},
			},
			SemanticTags: []string{
				"comment", "discussion", "collaboration", "feedback",
				"update", "communication", "thread", "mention",
			},
			CommonPhrases: []string{
				"add comment", "update comment", "delete comment", "get comments",
				"comment on issue", "discuss issue", "provide feedback",
				"team discussion", "progress update", "mention user",
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "comments"},
					{Action: "read", Resource: "comments"},
					{Action: "update", Resource: "comments"},
					{Action: "delete", Resource: "comments"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 60,
					Description:       "Jira Cloud rate limits: 60 requests per minute for comment operations",
				},
			},
		})
	}

	// Workflow Management
	if toolset, exists := p.toolsetRegistry["workflow"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "jira_workflow",
			DisplayName: "Jira Workflow & Transitions",
			Category:    "Workflow",
			Subcategory: "Issue Transitions",
			Description: "Manage issue workflows, execute transitions, and query workflow configurations. Handle issue status changes, workflow schemes, and transition rules.",
			DetailedHelp: `Jira Workflow capabilities:
- Get available transitions for any issue
- Execute issue transitions with validation
- Query workflow schemes and configurations
- Execute transitions with simultaneous comment addition
- Support for conditional transitions and field requirements
- Workflow validation and permission checking
- Handle transition screens and required fields
- Support for post-function execution and automation rules`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Get available transitions for an issue",
					Input: map[string]interface{}{
						"action":       "get_transitions",
						"issueIdOrKey": "PROJ-123",
						"expand":       []string{"transitions.fields"},
					},
					Explanation: "Retrieves all possible status transitions for the issue with field requirements",
				},
				{
					Scenario: "Transition issue to 'In Progress' status",
					Input: map[string]interface{}{
						"action":       "transition_issue",
						"issueIdOrKey": "PROJ-456",
						"transition": map[string]interface{}{
							"id": "4",
						},
						"fields": map[string]interface{}{
							"assignee": map[string]interface{}{
								"accountId": "12345",
							},
						},
					},
					Explanation: "Transitions issue to In Progress and assigns it to a specific user",
				},
				{
					Scenario: "Complete issue with resolution and comment",
					Input: map[string]interface{}{
						"action":       "transition_with_comment",
						"issueIdOrKey": "PROJ-789",
						"transitionId": "31",
						"comment":      "Issue resolved. Deployed fix to production and verified working correctly.",
						"fields": map[string]interface{}{
							"resolution": map[string]interface{}{
								"name": "Fixed",
							},
						},
					},
					Explanation: "Transitions issue to Done status with resolution and adds a completion comment",
				},
				{
					Scenario: "Query workflow configurations",
					Input: map[string]interface{}{
						"action":     "get_workflows",
						"projectKey": "PROJ",
						"expand":     []string{"workflows.transitions", "workflows.statuses"},
					},
					Explanation: "Retrieves workflow schemes for the project with detailed transition and status information",
				},
			},
			SemanticTags: []string{
				"workflow", "transition", "status", "state", "progress",
				"complete", "resolve", "close", "move", "change",
			},
			CommonPhrases: []string{
				"move to in progress", "mark as done", "transition issue",
				"change status", "complete task", "resolve bug",
				"close issue", "workflow transition", "status change",
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "read", Resource: "transitions"},
					{Action: "execute", Resource: "transitions"},
					{Action: "read", Resource: "workflows"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 60,
					Description:       "Jira Cloud rate limits: 60 requests per minute for workflow operations",
				},
			},
		})
	}

	// Help and Best Practices
	definitions = append(definitions, providers.AIOptimizedToolDefinition{
		Name:        "jira_help",
		DisplayName: "Jira Help & Best Practices",
		Category:    "Help",
		Subcategory: "Usage Guidance",
		Description: "Common patterns, troubleshooting, and best practices for Jira operations",
		DetailedHelp: `Common Jira usage patterns and troubleshooting:

JQL Query Examples:
- Basic: 'project = PROJ AND status = Open'
- Date ranges: 'created >= -7d AND updated >= -1d'
- User queries: 'assignee = currentUser() OR reporter = currentUser()'
- Complex: 'project in (PROJ, TEAM) AND priority in (High, Highest) AND status != Done'

Common Field Names:
- project, issuetype, summary, description, assignee, reporter
- priority, status, resolution, labels, components, fixVersions
- created, updated, duedate, environment

Error Handling:
- Issue not found: Check issue key format (PROJECT-123)
- Permission denied: Verify project access and user permissions
- Invalid JQL: Use validateQuery=true to check syntax
- Field errors: Check required fields for issue type and project

Best Practices:
- Use specific field selection to improve performance
- Implement pagination for large result sets
- Validate JQL queries before execution
- Use project filters to limit scope
- Handle rate limits with appropriate delays`,
		UsageExamples: []providers.Example{
			{
				Scenario: "Validate JQL query syntax",
				Input: map[string]interface{}{
					"action":        "search_issues",
					"jql":           "project = PROJ AND invalid_field = value",
					"validateQuery": true,
					"maxResults":    0,
				},
				Explanation: "Tests JQL syntax without executing the search",
			},
		},
		SemanticTags: []string{
			"help", "troubleshooting", "best-practices", "examples",
			"jql-help", "field-reference", "error-handling",
		},
		CommonPhrases: []string{
			"jira help", "how to use jira", "jql examples",
			"jira best practices", "troubleshoot jira",
		},
	})

	return definitions
}
