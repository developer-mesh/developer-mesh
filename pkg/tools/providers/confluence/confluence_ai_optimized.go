package confluence

import (
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// GetAIOptimizedDefinitions returns AI-optimized tool definitions for Confluence
func (p *ConfluenceProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return []providers.AIOptimizedToolDefinition{
		{
			Name:        "confluence_content",
			DisplayName: "Confluence Content Management",
			Category:    "documentation",
			Subcategory: "wiki",
			Description: "Create, read, update, and delete Confluence pages and blog posts. Manage documentation, knowledge base articles, and team wikis.",
			DetailedHelp: `The Confluence content tool allows you to:
- Create new pages or blog posts in specific spaces
- Update existing documentation with new information
- Search for content using Confluence Query Language (CQL)
- Organize content hierarchically with parent-child relationships
- Manage content versions and restore previous versions
- Control who can view or edit content with restrictions`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new documentation page",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"type":  "page",
							"title": "API Documentation",
							"space": map[string]interface{}{
								"key": "DEV",
							},
							"body": map[string]interface{}{
								"storage": map[string]interface{}{
									"value":          "<h1>API Documentation</h1><p>This page contains our API documentation.</p>",
									"representation": "storage",
								},
							},
						},
					},
					Explanation: "Creates a new page titled 'API Documentation' in the DEV space with initial HTML content",
				},
				{
					Scenario: "Search for all pages about deployment",
					Input: map[string]interface{}{
						"action": "search",
						"parameters": map[string]interface{}{
							"cql":   "text ~ \"deployment\" AND type = page",
							"limit": 10,
						},
					},
					Explanation: "Searches for up to 10 pages containing the word 'deployment' using CQL",
				},
				{
					Scenario: "Update an existing page with new content",
					Input: map[string]interface{}{
						"action": "update",
						"parameters": map[string]interface{}{
							"id":    "123456",
							"title": "Updated API Documentation",
							"version": map[string]interface{}{
								"number": 2,
							},
							"body": map[string]interface{}{
								"storage": map[string]interface{}{
									"value":          "<h1>Updated API Documentation</h1><p>New content here.</p>",
									"representation": "storage",
								},
							},
						},
					},
					Explanation: "Updates page with ID 123456 to version 2 with new title and content",
				},
			},
			SemanticTags: []string{
				"documentation", "wiki", "knowledge-base", "page", "blog-post",
				"content", "article", "confluence", "atlassian", "collaborate",
			},
			CommonPhrases: []string{
				"create a confluence page", "update documentation", "search wiki",
				"write a blog post", "document this in confluence", "add to knowledge base",
			},
			RelatedTools: []string{"jira", "bitbucket", "slack"},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "The action to perform (create, get, update, delete, search, list)",
						Examples:    []interface{}{"create", "search", "update"},
					},
					"id": {
						Type:        "string",
						Description: "Content ID for operations on existing content",
						Examples:    []interface{}{"123456", "789012"},
					},
					"spaceKey": {
						Type:        "string",
						Description: "The key of the Confluence space",
						Examples:    []interface{}{"DEV", "PROJ", "TEAM"},
						Aliases:     []string{"space", "space_key"},
					},
					"title": {
						Type:        "string",
						Description: "The title of the page or blog post",
						Examples:    []interface{}{"API Documentation", "Release Notes v2.0"},
					},
					"type": {
						Type:        "string",
						Description: "Content type (page, blogpost, comment)",
						Examples:    []interface{}{"page", "blogpost"},
					},
					"cql": {
						Type:        "string",
						Description: "Confluence Query Language string for searching",
						Examples:    []interface{}{"type = page AND space = DEV", "text ~ \"api\" AND lastmodified > now(\"-7d\")"},
						Template:    "field operator value [AND|OR field operator value]",
					},
				},
				Required: []string{"action"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "pages", Constraints: []string{"requires_space_key", "requires_title"}},
					{Action: "read", Resource: "pages", Constraints: []string{"public_or_permitted"}},
					{Action: "update", Resource: "pages", Constraints: []string{"requires_edit_permission", "version_conflict_check"}},
					{Action: "delete", Resource: "pages", Constraints: []string{"requires_delete_permission"}},
					{Action: "search", Resource: "all_content", Constraints: []string{"cql_syntax_required"}},
				},
				Limitations: []providers.Limitation{
					{Description: "Cannot modify restricted content without proper permissions", Workaround: "Request access from space admin"},
					{Description: "File attachments require separate API calls", Workaround: "Use attachment operations after content creation"},
					{Description: "CQL has limited operators compared to SQL", Workaround: "Combine multiple queries or filter results client-side"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerHour:   5000,
					RequestsPerMinute: 100,
					Description:       "Confluence Cloud API rate limits per user",
				},
				DataAccess: &providers.DataAccessPattern{
					Pagination:       true,
					MaxResults:       100,
					SupportedFilters: []string{"space", "type", "status", "created", "lastmodified"},
					SupportedSorts:   []string{"created", "modified", "title"},
				},
			},
			ComplexityLevel: "moderate",
		},
		{
			Name:        "confluence_space",
			DisplayName: "Confluence Space Management",
			Category:    "documentation",
			Subcategory: "organization",
			Description: "Manage Confluence spaces - the containers for organizing related pages and content.",
			DetailedHelp: `Spaces in Confluence are used to organize content by team, project, or topic. This tool allows you to:
- Create new spaces for teams or projects
- Configure space settings and permissions
- List and search available spaces
- Manage space templates and themes`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new team space",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"key":         "TEAMX",
							"name":        "Team X Documentation",
							"description": "Documentation space for Team X projects and processes",
						},
					},
				},
				{
					Scenario: "List all available spaces",
					Input: map[string]interface{}{
						"action": "list",
						"parameters": map[string]interface{}{
							"type":  "global",
							"limit": 50,
						},
					},
				},
			},
			SemanticTags:  []string{"space", "workspace", "organization", "container", "project-space"},
			CommonPhrases: []string{"create a space", "organize documentation", "set up team wiki"},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "The action to perform (create, get, list, update, delete)",
						Examples:    []interface{}{"create", "list", "get"},
					},
					"key": {
						Type:        "string",
						Description: "Unique space key (uppercase letters, no spaces)",
						Examples:    []interface{}{"DEV", "PROJ", "TEAMX"},
						Template:    "[A-Z]+",
					},
					"name": {
						Type:        "string",
						Description: "Human-readable space name",
						Examples:    []interface{}{"Development Team", "Project Phoenix"},
					},
				},
			},
			ComplexityLevel: "simple",
		},
		{
			Name:        "confluence_attachment",
			DisplayName: "Confluence Attachments",
			Category:    "documentation",
			Subcategory: "files",
			Description: "Manage file attachments on Confluence pages - upload, download, and organize files.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Upload a file to a page",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"id":      "123456",
							"file":    "design-doc.pdf",
							"comment": "Latest design document version",
						},
					},
				},
			},
			SemanticTags:    []string{"attachment", "file", "upload", "document", "media"},
			CommonPhrases:   []string{"attach file", "upload document", "add attachment"},
			ComplexityLevel: "simple",
		},
		{
			Name:        "confluence_search",
			DisplayName: "Confluence Search",
			Category:    "search",
			Description: "Search Confluence content using CQL (Confluence Query Language) for powerful, structured queries.",
			DetailedHelp: `CQL allows you to search for content based on various criteria:
- Text content: text ~ "search term"
- Content type: type = page OR type = blogpost
- Space: space = DEV
- Date ranges: created > now("-7d")
- Labels: label = "important"
- Author: creator = "john.doe"`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Find all pages modified in the last week",
					Input: map[string]interface{}{
						"action": "search",
						"parameters": map[string]interface{}{
							"cql":   "type = page AND lastmodified > now(\"-7d\")",
							"limit": 20,
						},
					},
				},
				{
					Scenario: "Search for pages with specific label in a space",
					Input: map[string]interface{}{
						"action": "search",
						"parameters": map[string]interface{}{
							"cql": "space = DEV AND label = \"api\" AND type = page",
						},
					},
				},
			},
			SemanticTags:  []string{"search", "query", "find", "cql", "filter"},
			CommonPhrases: []string{"search confluence", "find pages", "query content", "look up documentation"},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"cql": {
						Type:        "string",
						Description: "Confluence Query Language string",
						Examples: []interface{}{
							"text ~ \"deployment\"",
							"type = page AND space = DEV",
							"label = \"important\" AND created > now(\"-30d\")",
						},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results to return",
						Examples:    []interface{}{10, 25, 100},
					},
				},
				Required: []string{"cql"},
			},
			ComplexityLevel: "complex",
		},
		{
			Name:        "confluence_comment",
			DisplayName: "Confluence Comments",
			Category:    "collaboration",
			Description: "Manage comments on Confluence pages for discussions, feedback, and collaboration.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Add a comment to a page",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"id": "123456",
							"body": map[string]interface{}{
								"storage": map[string]interface{}{
									"value":          "<p>Great documentation! One suggestion...</p>",
									"representation": "storage",
								},
							},
						},
					},
				},
			},
			SemanticTags:    []string{"comment", "discussion", "feedback", "reply", "conversation"},
			CommonPhrases:   []string{"add comment", "leave feedback", "discuss", "reply to comment"},
			ComplexityLevel: "simple",
		},
		{
			Name:        "confluence_label",
			DisplayName: "Confluence Labels",
			Category:    "organization",
			Description: "Manage labels on Confluence content for categorization and organization.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Add labels to a page",
					Input: map[string]interface{}{
						"action": "add",
						"parameters": map[string]interface{}{
							"id": "123456",
							"labels": []map[string]string{
								{"name": "important"},
								{"name": "api-docs"},
								{"name": "v2"},
							},
						},
					},
				},
			},
			SemanticTags:    []string{"label", "tag", "category", "organize", "classify"},
			CommonPhrases:   []string{"add label", "tag page", "categorize content", "organize with labels"},
			ComplexityLevel: "simple",
		},
		{
			Name:        "confluence_user",
			DisplayName: "Confluence User Management",
			Category:    "identity",
			Description: "Manage Confluence users, permissions, and watch settings.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Get current user information",
					Input: map[string]interface{}{
						"action": "current",
					},
				},
				{
					Scenario: "Watch a page for updates",
					Input: map[string]interface{}{
						"action": "watch",
						"parameters": map[string]interface{}{
							"contentId": "123456",
							"accountId": "557058:12345678-90ab-cdef-1234-567890abcdef",
						},
					},
				},
			},
			SemanticTags:    []string{"user", "account", "profile", "permissions", "watch"},
			CommonPhrases:   []string{"get user", "watch page", "check permissions", "user profile"},
			ComplexityLevel: "simple",
		},
		{
			Name:        "confluence_template",
			DisplayName: "Confluence Templates",
			Category:    "documentation",
			Subcategory: "templates",
			Description: "Manage page templates to standardize content creation and maintain consistency.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a meeting notes template",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"name":         "Meeting Notes Template",
							"templateType": "page",
							"body": map[string]interface{}{
								"storage": map[string]interface{}{
									"value": "<h2>Meeting Notes - [Date]</h2><h3>Attendees</h3><ul><li></li></ul><h3>Agenda</h3><ol><li></li></ol><h3>Action Items</h3><ul><li></li></ul>",
								},
							},
						},
					},
				},
			},
			SemanticTags:    []string{"template", "blueprint", "boilerplate", "pattern", "standard"},
			CommonPhrases:   []string{"create template", "use template", "page template", "standard format"},
			ComplexityLevel: "moderate",
		},
		{
			Name:        "confluence_permission",
			DisplayName: "Confluence Permissions",
			Category:    "security",
			Description: "Manage content restrictions and permissions to control access to sensitive information.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Check if user can edit a page",
					Input: map[string]interface{}{
						"action": "check",
						"parameters": map[string]interface{}{
							"id": "123456",
							"subject": map[string]interface{}{
								"type":       "user",
								"identifier": "557058:12345678-90ab-cdef-1234-567890abcdef",
							},
							"operation": "update",
						},
					},
				},
			},
			SemanticTags:    []string{"permission", "restriction", "access", "security", "authorization"},
			CommonPhrases:   []string{"check permissions", "restrict access", "grant permission", "view restrictions"},
			ComplexityLevel: "moderate",
		},
	}
}
