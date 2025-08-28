package jira

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
)

// Embed Jira Cloud OpenAPI spec as fallback
//
//go:embed jira-cloud-openapi.json
var jiraOpenAPISpecJSON []byte

// JiraProvider implements the StandardToolProvider interface for Atlassian Jira Cloud
type JiraProvider struct {
	*providers.BaseProvider
	domain       string // The Atlassian domain (e.g., yourcompany.atlassian.net)
	specCache    repository.OpenAPICacheRepository
	specFallback *openapi3.T
	httpClient   *http.Client
}

// NewJiraProvider creates a new Jira Cloud provider instance
func NewJiraProvider(logger observability.Logger, domain string) *JiraProvider {
	// For Jira Cloud, the base URL includes the domain
	baseURL := fmt.Sprintf("https://%s.atlassian.net", domain)
	if strings.Contains(domain, "atlassian.net") {
		baseURL = fmt.Sprintf("https://%s", domain)
	}

	base := providers.NewBaseProvider("jira", "cloud", baseURL, logger)

	// Load embedded spec as fallback
	var specFallback *openapi3.T
	if len(jiraOpenAPISpecJSON) > 0 {
		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(jiraOpenAPISpecJSON)
		if err == nil {
			specFallback = spec
		}
	}

	provider := &JiraProvider{
		BaseProvider: base,
		domain:       domain,
		specFallback: specFallback,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Set operation mappings
	provider.SetOperationMappings(provider.GetOperationMappings())

	// Initialize configuration with proper base URL
	config := providers.ProviderConfig{
		BaseURL:  baseURL,
		AuthType: "basic",
		DefaultHeaders: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 60,
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnRateLimit: true,
		},
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "issues",
				DisplayName: "Issue Management",
				Description: "Operations for managing Jira issues",
				Operations:  []string{"issues/search", "issues/get", "issues/create", "issues/update", "issues/delete"},
			},
			{
				Name:        "projects",
				DisplayName: "Project Management",
				Description: "Operations for managing Jira projects",
				Operations:  []string{"projects/list", "projects/get", "projects/create", "projects/update"},
			},
			{
				Name:        "boards",
				DisplayName: "Agile Boards",
				Description: "Operations for Agile boards and sprints",
				Operations:  []string{"boards/list", "boards/get", "boards/sprints", "boards/backlog"},
			},
			{
				Name:        "users",
				DisplayName: "User Management",
				Description: "Operations for managing users and permissions",
				Operations:  []string{"users/search", "users/get", "users/groups"},
			},
			{
				Name:        "workflows",
				DisplayName: "Workflow Management",
				Description: "Operations for managing workflows and transitions",
				Operations:  []string{"workflows/list", "workflows/get"},
			},
		},
	}
	provider.SetConfiguration(config)

	return provider
}

// NewJiraProviderWithCache creates a new Jira provider with spec caching
func NewJiraProviderWithCache(logger observability.Logger, domain string, specCache repository.OpenAPICacheRepository) *JiraProvider {
	provider := NewJiraProvider(logger, domain)
	provider.specCache = specCache
	return provider
}

// GetProviderName returns the provider name
func (p *JiraProvider) GetProviderName() string {
	return "jira"
}

// GetSupportedVersions returns supported Jira API versions
func (p *JiraProvider) GetSupportedVersions() []string {
	return []string{"cloud", "3"}
}

// GetToolDefinitions returns Jira-specific tool definitions
func (p *JiraProvider) GetToolDefinitions() []providers.ToolDefinition {
	return []providers.ToolDefinition{
		{
			Name:        "jira_issues",
			DisplayName: "Jira Issues",
			Description: "Manage Jira issues including creation, updates, and searches",
			Category:    "issue_tracking",
			Operation: providers.OperationDef{
				ID:           "issues",
				Method:       "GET",
				PathTemplate: "/rest/api/3/search",
			},
			Parameters: []providers.ParameterDef{
				{Name: "jql", In: "query", Type: "string", Required: false, Description: "JQL query string"},
				{Name: "maxResults", In: "query", Type: "integer", Required: false, Description: "Maximum results", Default: 50},
				{Name: "startAt", In: "query", Type: "integer", Required: false, Description: "Starting index", Default: 0},
			},
		},
		{
			Name:        "jira_projects",
			DisplayName: "Jira Projects",
			Description: "Manage Jira projects and boards",
			Category:    "project_management",
			Operation: providers.OperationDef{
				ID:           "projects",
				Method:       "GET",
				PathTemplate: "/rest/api/3/project",
			},
			Parameters: []providers.ParameterDef{
				{Name: "expand", In: "query", Type: "string", Required: false, Description: "Additional fields to expand"},
			},
		},
		{
			Name:        "jira_users",
			DisplayName: "Jira Users",
			Description: "Manage Jira users and permissions",
			Category:    "identity",
			Operation: providers.OperationDef{
				ID:           "users",
				Method:       "GET",
				PathTemplate: "/rest/api/3/user/search",
			},
			Parameters: []providers.ParameterDef{
				{Name: "query", In: "query", Type: "string", Required: false, Description: "Search query"},
				{Name: "maxResults", In: "query", Type: "integer", Required: false, Description: "Maximum results", Default: 50},
			},
		},
		{
			Name:        "jira_boards",
			DisplayName: "Jira Boards",
			Description: "Manage Agile boards and sprints",
			Category:    "agile",
			Operation: providers.OperationDef{
				ID:           "boards",
				Method:       "GET",
				PathTemplate: "/rest/agile/1.0/board",
			},
			Parameters: []providers.ParameterDef{
				{Name: "type", In: "query", Type: "string", Required: false, Description: "Board type (scrum, kanban)"},
				{Name: "name", In: "query", Type: "string", Required: false, Description: "Board name filter"},
			},
		},
		{
			Name:        "jira_workflows",
			DisplayName: "Jira Workflows",
			Description: "Manage issue workflows and transitions",
			Category:    "automation",
			Operation: providers.OperationDef{
				ID:           "workflows",
				Method:       "GET",
				PathTemplate: "/rest/api/3/workflow",
			},
		},
	}
}

// ValidateCredentials validates Jira credentials
func (p *JiraProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// Jira Cloud supports multiple auth methods
	email, hasEmail := creds["email"]
	apiToken, hasAPIToken := creds["api_token"]

	// OAuth 2.0
	accessToken, hasAccessToken := creds["access_token"]

	// Validate based on available credentials
	var authHeader string
	if hasEmail && hasAPIToken {
		// Basic auth with email and API token
		authHeader = "Basic " + basicAuth(email, apiToken)
	} else if hasAccessToken {
		// OAuth 2.0
		authHeader = "Bearer " + accessToken
	} else {
		return fmt.Errorf("missing required credentials: either (email + api_token) or access_token required")
	}

	// Test the credentials with the myself endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", p.buildURL("/rest/api/3/myself"), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid Jira credentials")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from Jira API: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetOperationMappings returns Jira-specific operation mappings
func (p *JiraProvider) GetOperationMappings() map[string]providers.OperationMapping {
	return map[string]providers.OperationMapping{
		// Issue operations
		"issues/search": {
			OperationID:    "searchIssues",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/search",
			RequiredParams: []string{},
			OptionalParams: []string{"jql", "startAt", "maxResults", "fields", "expand"},
		},
		"issues/get": {
			OperationID:    "getIssue",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}",
			RequiredParams: []string{"issueIdOrKey"},
			OptionalParams: []string{"fields", "expand"},
		},
		"issues/create": {
			OperationID:    "createIssue",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/issue",
			RequiredParams: []string{"fields"},
			OptionalParams: []string{"updateHistory"},
		},
		"issues/update": {
			OperationID:    "updateIssue",
			Method:         "PUT",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}",
			RequiredParams: []string{"issueIdOrKey"},
			OptionalParams: []string{"fields", "notifyUsers"},
		},
		"issues/delete": {
			OperationID:    "deleteIssue",
			Method:         "DELETE",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}",
			RequiredParams: []string{"issueIdOrKey"},
			OptionalParams: []string{"deleteSubtasks"},
		},
		"issues/transitions": {
			OperationID:    "getTransitions",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/transitions",
			RequiredParams: []string{"issueIdOrKey"},
			OptionalParams: []string{"expand", "transitionId"},
		},
		"issues/transition": {
			OperationID:    "doTransition",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/transitions",
			RequiredParams: []string{"issueIdOrKey", "transition"},
		},
		"issues/assign": {
			OperationID:    "assignIssue",
			Method:         "PUT",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/assignee",
			RequiredParams: []string{"issueIdOrKey", "accountId"},
		},
		"issues/comments/list": {
			OperationID:    "getComments",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/comment",
			RequiredParams: []string{"issueIdOrKey"},
			OptionalParams: []string{"startAt", "maxResults", "expand"},
		},
		"issues/comments/add": {
			OperationID:    "addComment",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/comment",
			RequiredParams: []string{"issueIdOrKey", "body"},
			OptionalParams: []string{"visibility"},
		},
		"issues/attachments/add": {
			OperationID:    "addAttachment",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/attachments",
			RequiredParams: []string{"issueIdOrKey", "file"},
		},
		"issues/watchers/add": {
			OperationID:    "addWatcher",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/issue/{issueIdOrKey}/watchers",
			RequiredParams: []string{"issueIdOrKey", "accountId"},
		},

		// Project operations
		"projects/list": {
			OperationID:    "listProjects",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/project",
			RequiredParams: []string{},
			OptionalParams: []string{"expand", "startAt", "maxResults"},
		},
		"projects/get": {
			OperationID:    "getProject",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/project/{projectIdOrKey}",
			RequiredParams: []string{"projectIdOrKey"},
			OptionalParams: []string{"expand"},
		},
		"projects/create": {
			OperationID:    "createProject",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/project",
			RequiredParams: []string{"key", "name", "leadAccountId", "projectTypeKey"},
		},
		"projects/update": {
			OperationID:    "updateProject",
			Method:         "PUT",
			PathTemplate:   "/rest/api/3/project/{projectIdOrKey}",
			RequiredParams: []string{"projectIdOrKey"},
		},
		"projects/delete": {
			OperationID:    "deleteProject",
			Method:         "DELETE",
			PathTemplate:   "/rest/api/3/project/{projectIdOrKey}",
			RequiredParams: []string{"projectIdOrKey"},
		},
		"projects/versions": {
			OperationID:    "getProjectVersions",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/project/{projectIdOrKey}/versions",
			RequiredParams: []string{"projectIdOrKey"},
		},
		"projects/components": {
			OperationID:    "getProjectComponents",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/project/{projectIdOrKey}/components",
			RequiredParams: []string{"projectIdOrKey"},
		},

		// User operations
		"users/search": {
			OperationID:    "searchUsers",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/user/search",
			RequiredParams: []string{},
			OptionalParams: []string{"query", "username", "accountId", "startAt", "maxResults"},
		},
		"users/get": {
			OperationID:    "getUser",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/user",
			RequiredParams: []string{"accountId"},
			OptionalParams: []string{"expand"},
		},
		"users/groups": {
			OperationID:    "getUserGroups",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/user/groups",
			RequiredParams: []string{"accountId"},
		},
		"users/current": {
			OperationID:  "getCurrentUser",
			Method:       "GET",
			PathTemplate: "/rest/api/3/myself",
		},

		// Board operations (Agile)
		"boards/list": {
			OperationID:    "listBoards",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/board",
			RequiredParams: []string{},
			OptionalParams: []string{"startAt", "maxResults", "type", "name", "projectKeyOrId"},
		},
		"boards/get": {
			OperationID:    "getBoard",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/board/{boardId}",
			RequiredParams: []string{"boardId"},
		},
		"boards/backlog": {
			OperationID:    "getBoardBacklog",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/board/{boardId}/backlog",
			RequiredParams: []string{"boardId"},
			OptionalParams: []string{"startAt", "maxResults", "jql"},
		},
		"boards/sprints": {
			OperationID:    "getBoardSprints",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/board/{boardId}/sprint",
			RequiredParams: []string{"boardId"},
			OptionalParams: []string{"startAt", "maxResults", "state"},
		},
		"boards/issues": {
			OperationID:    "getBoardIssues",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/board/{boardId}/issue",
			RequiredParams: []string{"boardId"},
			OptionalParams: []string{"startAt", "maxResults", "jql"},
		},

		// Sprint operations
		"sprints/get": {
			OperationID:    "getSprint",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/sprint/{sprintId}",
			RequiredParams: []string{"sprintId"},
		},
		"sprints/create": {
			OperationID:    "createSprint",
			Method:         "POST",
			PathTemplate:   "/rest/agile/1.0/sprint",
			RequiredParams: []string{"name", "startDate", "endDate", "originBoardId"},
		},
		"sprints/update": {
			OperationID:    "updateSprint",
			Method:         "POST",
			PathTemplate:   "/rest/agile/1.0/sprint/{sprintId}",
			RequiredParams: []string{"sprintId"},
		},
		"sprints/issues": {
			OperationID:    "getSprintIssues",
			Method:         "GET",
			PathTemplate:   "/rest/agile/1.0/sprint/{sprintId}/issue",
			RequiredParams: []string{"sprintId"},
			OptionalParams: []string{"startAt", "maxResults", "jql"},
		},

		// Workflow operations
		"workflows/list": {
			OperationID:    "listWorkflows",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/workflow",
			RequiredParams: []string{},
			OptionalParams: []string{"startAt", "maxResults"},
		},
		"workflows/get": {
			OperationID:    "getWorkflow",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/workflow/{workflowName}",
			RequiredParams: []string{"workflowName"},
		},

		// Field operations
		"fields/list": {
			OperationID:  "listFields",
			Method:       "GET",
			PathTemplate: "/rest/api/3/field",
		},
		"fields/custom/create": {
			OperationID:    "createCustomField",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/field",
			RequiredParams: []string{"name", "fieldType"},
		},

		// Filter operations (JQL)
		"filters/list": {
			OperationID:    "listFilters",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/filter",
			OptionalParams: []string{"startAt", "maxResults", "expand"},
		},
		"filters/get": {
			OperationID:    "getFilter",
			Method:         "GET",
			PathTemplate:   "/rest/api/3/filter/{id}",
			RequiredParams: []string{"id"},
		},
		"filters/create": {
			OperationID:    "createFilter",
			Method:         "POST",
			PathTemplate:   "/rest/api/3/filter",
			RequiredParams: []string{"name", "jql"},
			OptionalParams: []string{"description", "sharePermissions"},
		},
	}
}

// GetDefaultConfiguration returns default Jira configuration
func (p *JiraProvider) GetDefaultConfiguration() providers.ProviderConfig {
	// Get the stored configuration from BaseProvider
	config := p.BaseProvider.GetDefaultConfiguration()
	if config.BaseURL != "" {
		return config
	}

	// Return default configuration if not yet configured
	return providers.ProviderConfig{
		BaseURL:  fmt.Sprintf("https://%s.atlassian.net", p.domain),
		AuthType: "basic", // Default to basic auth with API token
		DefaultHeaders: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		RateLimits: providers.RateLimitConfig{
			RequestsPerMinute: 60,
		},
		Timeout: 30 * time.Second,
		RetryPolicy: &providers.RetryPolicy{
			MaxRetries:       3,
			InitialDelay:     1 * time.Second,
			MaxDelay:         10 * time.Second,
			Multiplier:       2.0,
			RetryOnRateLimit: true,
		},
		OperationGroups: []providers.OperationGroup{
			{
				Name:        "issues",
				DisplayName: "Issue Management",
				Description: "Operations for managing Jira issues",
				Operations:  []string{"issues/search", "issues/get", "issues/create", "issues/update", "issues/delete"},
			},
			{
				Name:        "projects",
				DisplayName: "Project Management",
				Description: "Operations for managing Jira projects",
				Operations:  []string{"projects/list", "projects/get", "projects/create", "projects/update"},
			},
			{
				Name:        "boards",
				DisplayName: "Agile Boards",
				Description: "Operations for Agile boards and sprints",
				Operations:  []string{"boards/list", "boards/get", "boards/sprints", "boards/backlog"},
			},
			{
				Name:        "users",
				DisplayName: "User Management",
				Description: "Operations for managing users and permissions",
				Operations:  []string{"users/search", "users/get", "users/groups"},
			},
			{
				Name:        "workflows",
				DisplayName: "Workflow Management",
				Description: "Operations for managing workflows and transitions",
				Operations:  []string{"workflows/list", "workflows/get"},
			},
		},
	}
}

// GetAIOptimizedDefinitions returns AI-friendly tool definitions for Jira
func (p *JiraProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return []providers.AIOptimizedToolDefinition{
		{
			Name:         "jira_issues",
			DisplayName:  "Jira Issue Management",
			Category:     "issue_tracking",
			Description:  "Search, create, and manage Jira issues, stories, bugs, and tasks. Supports JQL queries, transitions, comments, and attachments.",
			DetailedHelp: "Use JQL (Jira Query Language) for powerful searches. Examples: 'project = PROJ AND status = Open', 'assignee = currentUser() AND created >= -7d'",
			UsageExamples: []providers.Example{
				{
					Scenario: "Search for open bugs in a project",
					Input: map[string]interface{}{
						"action": "search",
						"parameters": map[string]interface{}{
							"jql": "project = PROJ AND type = Bug AND status != Closed",
						},
					},
				},
				{
					Scenario: "Create a new story",
					Input: map[string]interface{}{
						"action": "create",
						"parameters": map[string]interface{}{
							"fields": map[string]interface{}{
								"project":     map[string]interface{}{"key": "PROJ"},
								"summary":     "As a user, I want to...",
								"description": "Detailed description here",
								"issuetype":   map[string]interface{}{"name": "Story"},
								"priority":    map[string]interface{}{"name": "Medium"},
							},
						},
					},
				},
				{
					Scenario: "Transition an issue to Done",
					Input: map[string]interface{}{
						"action":       "transition",
						"issueIdOrKey": "PROJ-123",
						"parameters": map[string]interface{}{
							"transition": map[string]interface{}{
								"id": "31", // Done transition ID
							},
						},
					},
				},
			},
			SemanticTags:  []string{"issue", "ticket", "bug", "story", "task", "epic", "JQL"},
			CommonPhrases: []string{"create ticket", "find bugs", "update issue", "close story", "assign task"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "issues"},
					{Action: "read", Resource: "issues"},
					{Action: "update", Resource: "issues"},
					{Action: "delete", Resource: "issues"},
					{Action: "search", Resource: "issues"},
					{Action: "transition", Resource: "issues"},
					{Action: "comment", Resource: "issues"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 60,
					Description:       "Jira Cloud rate limits vary by endpoint and tier",
				},
				DataAccess: &providers.DataAccessPattern{
					Pagination:       true,
					MaxResults:       100,
					SupportedFilters: []string{"JQL"},
					SupportedSorts:   []string{"created", "updated", "priority", "rank"},
				},
			},
		},
		{
			Name:        "jira_projects",
			DisplayName: "Jira Project Management",
			Category:    "project_management",
			Description: "Manage Jira projects, components, versions, and project settings",
			UsageExamples: []providers.Example{
				{
					Scenario: "List all projects",
					Input: map[string]interface{}{
						"action": "list",
					},
				},
				{
					Scenario: "Get project details with versions",
					Input: map[string]interface{}{
						"action":         "get",
						"projectIdOrKey": "PROJ",
						"parameters": map[string]interface{}{
							"expand": "lead,description,projectKeys",
						},
					},
				},
			},
			SemanticTags:  []string{"project", "component", "version", "release"},
			CommonPhrases: []string{"list projects", "create project", "project settings"},
		},
		{
			Name:        "jira_boards",
			DisplayName: "Jira Agile Boards",
			Category:    "agile",
			Description: "Manage Scrum and Kanban boards, sprints, backlogs, and board configurations",
			UsageExamples: []providers.Example{
				{
					Scenario: "List Scrum boards for a project",
					Input: map[string]interface{}{
						"action": "list",
						"parameters": map[string]interface{}{
							"type":           "scrum",
							"projectKeyOrId": "PROJ",
						},
					},
				},
				{
					Scenario: "Get active sprint for a board",
					Input: map[string]interface{}{
						"action":  "sprints",
						"boardId": "10",
						"parameters": map[string]interface{}{
							"state": "active",
						},
					},
				},
			},
			SemanticTags:  []string{"board", "sprint", "backlog", "scrum", "kanban", "agile"},
			CommonPhrases: []string{"active sprint", "board backlog", "sprint planning"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "read", Resource: "boards"},
					{Action: "update", Resource: "boards"},
					{Action: "create", Resource: "sprints"},
					{Action: "read", Resource: "sprints"},
					{Action: "update", Resource: "sprints"},
				},
			},
		},
	}
}

// ExecuteOperation executes a Jira operation
func (p *JiraProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Normalize operation name (handle different formats)
	operation = p.normalizeOperationName(operation)

	// Get operation mapping
	_, exists := p.GetOperationMappings()[operation]
	if !exists {
		// Try to resolve the operation using context
		operation = p.resolveOperationFromContext(operation, params)
		_, exists = p.GetOperationMappings()[operation]
		if !exists {
			return nil, fmt.Errorf("unknown operation: %s", operation)
		}
	}

	// Special handling for JQL search - ensure proper parameter structure
	if operation == "issues/search" {
		if jql, ok := params["jql"].(string); ok && jql != "" {
			// JQL is already properly formatted
		} else if query, ok := params["query"].(string); ok {
			// Convert simple query to JQL
			params["jql"] = query
			delete(params, "query")
		}
	}

	// Use base provider's execution with Jira-specific handling
	return p.Execute(ctx, operation, params)
}

// normalizeOperationName normalizes operation names to handle different formats
func (p *JiraProvider) normalizeOperationName(operation string) string {
	// First, handle different separators to normalize format
	normalized := strings.ReplaceAll(operation, "-", "/")
	normalized = strings.ReplaceAll(normalized, "_", "/")

	// If it already has a resource prefix (e.g., "issues/create"), return it
	if strings.Contains(normalized, "/") {
		return normalized
	}

	// Map simple actions to default operations
	simpleActions := map[string]string{
		"search":     "issues/search",
		"list":       "issues/search", // Default list to search
		"get":        "issues/get",
		"create":     "issues/create",
		"update":     "issues/update",
		"delete":     "issues/delete",
		"comment":    "issues/comments/add",
		"transition": "issues/transition",
		"assign":     "issues/assign",
	}

	if defaultOp, ok := simpleActions[normalized]; ok {
		return defaultOp
	}

	return normalized
}

// resolveOperationFromContext attempts to resolve operation based on parameters
func (p *JiraProvider) resolveOperationFromContext(operation string, params map[string]interface{}) string {
	// Check for specific parameters that indicate the resource type
	if _, hasIssueKey := params["issueIdOrKey"]; hasIssueKey {
		// It's an issue operation
		switch operation {
		case "get":
			return "issues/get"
		case "update":
			return "issues/update"
		case "delete":
			return "issues/delete"
		case "comment":
			return "issues/comments/add"
		case "transition":
			return "issues/transition"
		case "assign":
			return "issues/assign"
		}
	}

	if _, hasProjectKey := params["projectIdOrKey"]; hasProjectKey {
		// It's a project operation
		switch operation {
		case "get":
			return "projects/get"
		case "update":
			return "projects/update"
		case "delete":
			return "projects/delete"
		case "versions":
			return "projects/versions"
		case "components":
			return "projects/components"
		}
	}

	if _, hasBoardId := params["boardId"]; hasBoardId {
		// It's a board operation
		switch operation {
		case "get":
			return "boards/get"
		case "backlog":
			return "boards/backlog"
		case "sprints":
			return "boards/sprints"
		case "issues":
			return "boards/issues"
		}
	}

	if _, hasSprintId := params["sprintId"]; hasSprintId {
		// It's a sprint operation
		switch operation {
		case "get":
			return "sprints/get"
		case "update":
			return "sprints/update"
		case "issues":
			return "sprints/issues"
		}
	}

	// Check for JQL parameter indicating search
	if _, hasJQL := params["jql"]; hasJQL && (operation == "search" || operation == "list") {
		return "issues/search"
	}

	return operation
}

// GetOpenAPISpec returns the OpenAPI specification for Jira
func (p *JiraProvider) GetOpenAPISpec() (*openapi3.T, error) {
	ctx := context.Background()

	// Try cache first if available
	if p.specCache != nil {
		spec, err := p.specCache.Get(ctx, "jira-cloud")
		if err == nil && spec != nil {
			return spec, nil
		}
	}

	// Try fetching from Atlassian with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	spec, err := p.fetchAndCacheSpec(fetchCtx)
	if err != nil {
		// Use embedded fallback if available
		if p.specFallback != nil {
			p.BaseProvider.GetLogger().Warn("Using embedded OpenAPI spec fallback", map[string]interface{}{
				"error": err.Error(),
			})
			return p.specFallback, nil
		}
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	return spec, nil
}

// fetchAndCacheSpec fetches the OpenAPI spec from Atlassian and caches it
func (p *JiraProvider) fetchAndCacheSpec(ctx context.Context) (*openapi3.T, error) {
	// Atlassian provides OpenAPI specs at a known location
	req, err := http.NewRequestWithContext(ctx, "GET", "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Cache the spec if cache is available
	if p.specCache != nil {
		_ = p.specCache.Set(ctx, "jira-cloud", spec, 24*time.Hour) // Cache for 24 hours
	}

	return spec, nil
}

// GetEmbeddedSpecVersion returns the version of the embedded OpenAPI spec
func (p *JiraProvider) GetEmbeddedSpecVersion() string {
	return "cloud-2024"
}

// HealthCheck verifies the Jira API is accessible
func (p *JiraProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.buildURL("/rest/api/3/serverInfo"), nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira API health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jira API health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up any resources
func (p *JiraProvider) Close() error {
	// Currently no resources to clean up
	return nil
}

// buildURL constructs the full URL for an API endpoint
func (p *JiraProvider) buildURL(path string) string {
	config := p.BaseProvider.GetDefaultConfiguration()
	baseURL := config.BaseURL
	if baseURL == "" {
		// Fallback to domain-based URL
		baseURL = fmt.Sprintf("https://%s.atlassian.net", p.domain)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/%s", baseURL, path)
}

// basicAuth creates a basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64Encode(auth)
}

// base64Encode encodes a string to base64
func base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
