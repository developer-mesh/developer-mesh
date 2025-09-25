package confluence

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// GetAIOptimizedDefinitions returns AI-friendly tool definitions for Confluence
func (p *ConfluenceProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	var definitions []providers.AIOptimizedToolDefinition

	// Page Management
	if toolset, exists := p.toolsetRegistry["pages"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "confluence_pages",
			DisplayName: "Confluence Pages",
			Category:    "Documentation",
			Subcategory: "Page Management",
			Description: "Create, read, update, and delete Confluence pages. Manage documentation, knowledge base articles, and collaborative content.",
			DetailedHelp: `Confluence Pages enable you to:
- Create new pages with rich content and formatting
- Update existing page content and properties
- Delete pages (with optional purge for permanent removal)
- List pages in spaces with filtering and search
- Manage page hierarchies with parent-child relationships
- Version control for page content
- Collaborate with team members through comments and @mentions`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new documentation page",
					Input: map[string]interface{}{
						"action":   "create",
						"title":    "API Documentation",
						"spaceKey": "DOCS",
						"content":  "<h1>API Overview</h1><p>This document describes our REST API...</p>",
						"type":     "page",
					},
					Explanation: "Creates a new page in the DOCS space with HTML content",
				},
				{
					Scenario: "List all pages in a space",
					Input: map[string]interface{}{
						"action":   "list",
						"spaceKey": "PROJECT",
						"limit":    25,
						"sort":     "title",
					},
					Explanation: "Retrieves the first 25 pages from the PROJECT space sorted by title",
				},
				{
					Scenario: "Update an existing page",
					Input: map[string]interface{}{
						"action":  "update",
						"pageId":  "12345",
						"title":   "Updated API Documentation",
						"content": "<h1>API v2 Overview</h1><p>Updated content...</p>",
						"version": 3,
					},
					Explanation: "Updates page 12345 with new content, requires current version for conflict detection",
				},
				{
					Scenario: "Delete a page permanently",
					Input: map[string]interface{}{
						"action": "delete",
						"pageId": "67890",
						"purge":  true,
					},
					Explanation: "Permanently deletes page 67890 (cannot be restored)",
				},
			},
			SemanticTags: []string{
				"page", "document", "wiki", "content", "documentation",
				"knowledge-base", "article", "confluence", "collaboration",
			},
			CommonPhrases: []string{
				"create page", "write documentation", "update wiki",
				"delete page", "list documents", "find page",
				"create child page", "move page", "copy page",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform (list, get, create, update, delete)",
						Examples:    []interface{}{"list", "create", "update", "delete"},
					},
					"pageId": {
						Type:        "string",
						Description: "Page ID for operations on specific pages",
						Examples:    []interface{}{"12345", "67890"},
					},
					"spaceKey": {
						Type:        "string",
						Description: "Space key where the page resides or will be created",
						Examples:    []interface{}{"DOCS", "PROJECT", "TEAM"},
					},
					"title": {
						Type:        "string",
						Description: "Page title",
						Examples:    []interface{}{"API Documentation", "Release Notes", "User Guide"},
					},
					"content": {
						Type:        "string",
						Description: "Page content in storage format (HTML)",
						Examples:    []interface{}{"<p>Content here</p>", "<h1>Title</h1>"},
					},
				},
				Required: []string{"action"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "pages"},
					{Action: "read", Resource: "pages"},
					{Action: "update", Resource: "pages"},
					{Action: "delete", Resource: "pages"},
					{Action: "list", Resource: "pages"},
					{Action: "move", Resource: "pages"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 100,
					Description:       "Standard Confluence Cloud rate limits apply",
				},
				DataAccess: &providers.DataAccessPattern{
					Pagination:       true,
					MaxResults:       250,
					SupportedFilters: []string{"space", "title", "created", "modified"},
					SupportedSorts:   []string{"title", "created", "modified"},
				},
			},
			ComplexityLevel: "simple",
		})
	}

	// Search Operations
	if toolset, exists := p.toolsetRegistry["search"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "confluence_search",
			DisplayName: "Confluence Search",
			Category:    "Search",
			Subcategory: "Content Discovery",
			Description: "Search Confluence content using CQL (Confluence Query Language). Find pages, spaces, attachments, and other content types.",
			DetailedHelp: `Confluence Search capabilities:
- Full-text search across all content
- CQL (Confluence Query Language) for advanced queries
- Filter by space, type, author, date ranges
- Search within specific content types (pages, blogs, attachments)
- Include archived content in searches
- Paginated results for large result sets
- Search with content expansion for detailed results`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Search for pages containing specific text",
					Input: map[string]interface{}{
						"action": "search",
						"cql":    "type = page AND text ~ 'API documentation'",
						"limit":  10,
					},
					Explanation: "Finds pages containing 'API documentation' text",
				},
				{
					Scenario: "Find all pages in a space created this week",
					Input: map[string]interface{}{
						"action": "search",
						"cql":    "space = DOCS AND type = page AND created >= startOfWeek()",
						"expand": []string{"space", "history"},
					},
					Explanation: "Searches for recently created pages in DOCS space with expanded metadata",
				},
				{
					Scenario: "Search for pages modified by a specific user",
					Input: map[string]interface{}{
						"action": "search",
						"cql":    "type = page AND lastModified = 'john.doe' AND created >= -30d",
					},
					Explanation: "Finds pages modified by john.doe in the last 30 days",
				},
				{
					Scenario: "Find attachments of a specific type",
					Input: map[string]interface{}{
						"action": "search",
						"cql":    "type = attachment AND mediaType = 'application/pdf'",
					},
					Explanation: "Searches for all PDF attachments",
				},
			},
			SemanticTags: []string{
				"search", "cql", "query", "find", "discover",
				"filter", "confluence-query-language", "content-search",
			},
			CommonPhrases: []string{
				"search pages", "find content", "query confluence",
				"search in space", "find attachments", "search by author",
				"find recent changes", "search archived",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Must be 'search' for search operations",
						Examples:    []interface{}{"search"},
					},
					"cql": {
						Type:        "string",
						Description: "Confluence Query Language expression",
						Examples:    []interface{}{"type = page", "space = DOCS AND text ~ 'api'"},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results to return",
						Examples:    []interface{}{10, 25, 50},
					},
					"start": {
						Type:        "integer",
						Description: "Starting index for pagination",
						Examples:    []interface{}{0, 25, 50},
					},
				},
				Required: []string{"action", "cql"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "search", Resource: "content"},
					{Action: "search", Resource: "pages"},
					{Action: "search", Resource: "spaces"},
					{Action: "search", Resource: "attachments"},
				},
				DataAccess: &providers.DataAccessPattern{
					Pagination:       true,
					MaxResults:       100,
					SupportedFilters: []string{"space", "type", "author", "created", "modified"},
				},
			},
			ComplexityLevel: "moderate",
		})
	}

	// Space Management
	if toolset, exists := p.toolsetRegistry["spaces"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "confluence_spaces",
			DisplayName: "Confluence Spaces",
			Category:    "Organization",
			Subcategory: "Space Management",
			Description: "Manage Confluence spaces - the containers for organizing related pages and content. List, filter, and navigate spaces.",
			DetailedHelp: `Confluence Space management:
- List all accessible spaces
- Filter spaces by type (global, personal)
- Filter by status (current, archived)
- Get space metadata and permissions
- Navigate space hierarchies
- Manage space settings and configurations`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List all active spaces",
					Input: map[string]interface{}{
						"action": "list",
						"status": "current",
						"type":   "global",
						"limit":  50,
					},
					Explanation: "Lists up to 50 active global spaces",
				},
				{
					Scenario: "Find archived spaces",
					Input: map[string]interface{}{
						"action": "list",
						"status": "archived",
					},
					Explanation: "Retrieves all archived spaces for cleanup or restoration",
				},
				{
					Scenario: "List personal spaces",
					Input: map[string]interface{}{
						"action": "list",
						"type":   "personal",
					},
					Explanation: "Lists all personal user spaces",
				},
			},
			SemanticTags: []string{
				"space", "workspace", "project", "organization",
				"container", "hierarchy", "structure",
			},
			CommonPhrases: []string{
				"list spaces", "find space", "show spaces",
				"get archived spaces", "personal spaces",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform (list)",
						Examples:    []interface{}{"list"},
					},
					"type": {
						Type:        "string",
						Description: "Space type filter",
						Examples:    []interface{}{"global", "personal"},
					},
					"status": {
						Type:        "string",
						Description: "Space status filter",
						Examples:    []interface{}{"current", "archived"},
					},
				},
				Required: []string{"action"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "list", Resource: "spaces"},
					{Action: "read", Resource: "spaces"},
				},
			},
			ComplexityLevel: "simple",
		})
	}

	// Content Management (V1 Operations)
	if toolset, exists := p.toolsetRegistry["content"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "confluence_content",
			DisplayName: "Confluence Content Operations",
			Category:    "Content",
			Subcategory: "Advanced Operations",
			Description: "Advanced content operations including page creation with full formatting, updates with version control, and attachment management.",
			DetailedHelp: `Advanced Content Operations (V1 API):
- Create pages with full storage format support
- Update pages with version conflict detection
- Manage attachments on pages
- Handle complex content structures
- Support for macros and rich formatting
- Parent-child page relationships
- Blog post creation and management`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a page with rich formatting",
					Input: map[string]interface{}{
						"action":   "create_page",
						"title":    "Technical Specifications",
						"spaceKey": "TECH",
						"content":  "<ac:structured-macro ac:name='toc'/><h1>Overview</h1><p>Content with macros...</p>",
						"parentId": "98765",
					},
					Explanation: "Creates a child page with table of contents macro",
				},
				{
					Scenario: "Update page with version control",
					Input: map[string]interface{}{
						"action":    "update_page",
						"pageId":    "12345",
						"content":   "<h1>Updated Content</h1><p>New version...</p>",
						"version":   5,
						"minorEdit": false,
					},
					Explanation: "Updates page content with version conflict detection",
				},
				{
					Scenario: "List page attachments",
					Input: map[string]interface{}{
						"action":    "get_attachments",
						"pageId":    "12345",
						"mediaType": "application/pdf",
					},
					Explanation: "Lists all PDF attachments on a specific page",
				},
				{
					Scenario: "Create a blog post",
					Input: map[string]interface{}{
						"action":   "create_page",
						"title":    "Product Release Announcement",
						"spaceKey": "BLOG",
						"content":  "<p>We're excited to announce...</p>",
						"type":     "blogpost",
					},
					Explanation: "Creates a blog post instead of a regular page",
				},
			},
			SemanticTags: []string{
				"content", "attachment", "version", "storage-format",
				"rich-text", "macro", "blog", "media",
			},
			CommonPhrases: []string{
				"create content", "update with version", "add attachment",
				"version conflict", "create blog post", "get attachments",
				"rich formatting", "add macro",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Content operation to perform",
						Examples:    []interface{}{"create_page", "update_page", "get_attachments"},
					},
					"pageId": {
						Type:        "string",
						Description: "Page ID for operations",
						Examples:    []interface{}{"12345"},
					},
					"version": {
						Type:        "integer",
						Description: "Current page version for conflict detection",
						Examples:    []interface{}{1, 2, 3},
					},
				},
				Required: []string{"action"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "content"},
					{Action: "update", Resource: "content"},
					{Action: "read", Resource: "attachments"},
				},
			},
			ComplexityLevel: "complex",
		})
	}

	// Label Management
	if toolset, exists := p.toolsetRegistry["labels"]; exists && toolset.Enabled {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "confluence_labels",
			DisplayName: "Confluence Labels",
			Category:    "Metadata",
			Subcategory: "Tagging",
			Description: "Manage labels on Confluence pages for categorization, organization, and improved searchability.",
			DetailedHelp: `Label Management features:
- Add labels to pages for categorization
- Remove labels from pages
- List all labels on a page
- Search content by labels
- Create consistent taxonomies
- Improve content discoverability`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Add labels to a page",
					Input: map[string]interface{}{
						"action": "add",
						"pageId": "12345",
						"labels": []string{"api", "documentation", "v2"},
					},
					Explanation: "Adds multiple labels to categorize the page",
				},
				{
					Scenario: "Get all labels on a page",
					Input: map[string]interface{}{
						"action": "get",
						"pageId": "12345",
					},
					Explanation: "Retrieves all labels currently applied to the page",
				},
				{
					Scenario: "Remove a label from a page",
					Input: map[string]interface{}{
						"action": "remove",
						"pageId": "12345",
						"label":  "outdated",
					},
					Explanation: "Removes a specific label from the page",
				},
				{
					Scenario: "Search pages by label",
					Input: map[string]interface{}{
						"action": "search",
						"cql":    "label = 'important' AND space = DOCS",
					},
					Explanation: "Finds all pages in DOCS space with 'important' label",
				},
			},
			SemanticTags: []string{
				"label", "tag", "category", "metadata",
				"taxonomy", "classification", "organization",
			},
			CommonPhrases: []string{
				"add label", "tag page", "categorize content",
				"remove label", "list tags", "search by label",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Label operation to perform",
						Examples:    []interface{}{"add", "remove", "get"},
					},
					"pageId": {
						Type:        "string",
						Description: "Page ID to manage labels on",
						Examples:    []interface{}{"12345"},
					},
					"label": {
						Type:        "string",
						Description: "Single label name",
						Examples:    []interface{}{"important", "draft", "reviewed"},
					},
					"labels": {
						Type:        "array",
						Description: "Multiple labels for batch operations",
						Examples:    []interface{}{[]string{"api", "v2"}, []string{"draft"}},
					},
				},
				Required: []string{"action", "pageId"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "labels"},
					{Action: "read", Resource: "labels"},
					{Action: "delete", Resource: "labels"},
				},
			},
			ComplexityLevel: "simple",
		})
	}

	// Error Handling and Best Practices
	definitions = append(definitions, providers.AIOptimizedToolDefinition{
		Name:        "confluence_errors",
		DisplayName: "Confluence Error Handling",
		Category:    "Help",
		Subcategory: "Error Resolution",
		Description: "Common Confluence API errors and how to handle them effectively.",
		DetailedHelp: `Common Error Scenarios and Solutions:

1. Authentication Errors (401):
   - Verify email and API token are correct
   - Check if API token has expired
   - Ensure user has access to Confluence

2. Permission Errors (403):
   - User lacks permission to perform the operation
   - Page or space restrictions prevent access
   - Read-only mode is enabled

3. Not Found Errors (404):
   - Page ID or space key doesn't exist
   - Content has been deleted
   - User doesn't have view permission

4. Version Conflict (409):
   - Page was modified by another user
   - Retrieve latest version and retry update
   - Use minorEdit flag for minor changes

5. Rate Limiting (429):
   - Too many requests in short time
   - Implement exponential backoff
   - Reduce request frequency

6. Validation Errors (400):
   - Invalid CQL syntax in search queries
   - Missing required parameters
   - Invalid HTML in content

7. Space Filter Restrictions:
   - Operation blocked by CONFLUENCE_SPACES_FILTER
   - Space not in allowed list
   - Contact administrator to update filter`,
		UsageExamples: []providers.Example{
			{
				Scenario: "Handle version conflict on update",
				Input: map[string]interface{}{
					"error":      "Version conflict - page was modified by another user",
					"resolution": "Get current version, merge changes, retry with new version number",
				},
				Explanation: "Version conflicts require fetching the latest version and retrying",
			},
			{
				Scenario: "Debug CQL syntax error",
				Input: map[string]interface{}{
					"error":      "Invalid CQL: unbalanced quotes",
					"resolution": "Check for matching quotes and parentheses in query",
				},
				Explanation: "CQL queries must have balanced quotes and valid syntax",
			},
			{
				Scenario: "Resolve space access restriction",
				Input: map[string]interface{}{
					"error":      "Space PRIVATE is not in allowed spaces",
					"resolution": "Request access to space or update CONFLUENCE_SPACES_FILTER",
				},
				Explanation: "Space filters restrict access to certain spaces",
			},
		},
		SemanticTags: []string{
			"error", "troubleshooting", "debugging", "resolution",
			"conflict", "permission", "authentication",
		},
		CommonPhrases: []string{
			"error handling", "fix error", "resolve conflict",
			"permission denied", "not found", "authentication failed",
		},
		ComplexityLevel: "moderate",
	})

	// Best Practices Guide
	definitions = append(definitions, providers.AIOptimizedToolDefinition{
		Name:        "confluence_bestpractices",
		DisplayName: "Confluence Best Practices",
		Category:    "Help",
		Subcategory: "Guidelines",
		Description: "Best practices for using Confluence APIs effectively and efficiently.",
		DetailedHelp: `Confluence API Best Practices:

1. Performance Optimization:
   - Use pagination for large result sets (limit parameter)
   - Minimize content expansions to needed fields only
   - Cache frequently accessed content
   - Use CQL to filter results at source

2. Content Management:
   - Always include version number when updating pages
   - Use storage format for rich content
   - Validate HTML content before submission
   - Implement retry logic for transient failures

3. Search Optimization:
   - Use specific CQL queries over broad searches
   - Index commonly searched fields with labels
   - Limit search scope to specific spaces when possible
   - Use appropriate date ranges in queries

4. Security Considerations:
   - Never hardcode API tokens
   - Use read-only mode for non-production environments
   - Implement space filters for multi-tenant setups
   - Regularly rotate API tokens

5. API Version Selection:
   - Use v2 API for simple page operations
   - Use v1 API for content creation/updates
   - v1 required for CQL search operations
   - Check _metadata.api_version in responses

6. Error Handling:
   - Implement exponential backoff for rate limits
   - Log all API errors with context
   - Handle version conflicts gracefully
   - Provide meaningful error messages to users

7. Label Management:
   - Establish consistent label taxonomy
   - Use labels for content categorization
   - Avoid special characters in labels
   - Keep label names short and descriptive`,
		UsageExamples: []providers.Example{
			{
				Scenario: "Efficient page listing with pagination",
				Input: map[string]interface{}{
					"action": "list",
					"limit":  50,
					"start":  0,
					"sort":   "modified",
				},
				Explanation: "Paginate through results to avoid timeouts",
			},
			{
				Scenario: "Safe page update with version check",
				Input: map[string]interface{}{
					"action":  "update",
					"pageId":  "12345",
					"version": 3,
					"content": "...",
				},
				Explanation: "Always include version to prevent overwriting changes",
			},
		},
		SemanticTags: []string{
			"best-practices", "guidelines", "optimization",
			"performance", "security", "efficiency",
		},
		CommonPhrases: []string{
			"best practice", "optimize performance", "improve efficiency",
			"secure usage", "proper implementation",
		},
		ComplexityLevel: "moderate",
	})

	return definitions
}
