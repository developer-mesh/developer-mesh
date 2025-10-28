package jira

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/getkin/kin-openapi/openapi3"
)

// contextKey is a type for context keys to avoid string literals
type contextKey string

const (
	// jiraTokenKey is the context key for Jira authentication token
	jiraTokenKey contextKey = "jira_token"
)

// Embed Jira Cloud OpenAPI spec as fallback
//
//go:embed jira-cloud-openapi.json
var jiraOpenAPISpecJSON []byte

// ToolHandler interface for Jira tool operations
type ToolHandler interface {
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
	GetDefinition() ToolDefinition
}

// ToolDefinition describes a tool's schema
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Toolset represents a group of related tools
type Toolset struct {
	Name        string
	Description string
	Tools       []ToolHandler
	Enabled     bool
}

// JiraProvider implements the StandardToolProvider interface for Atlassian Jira Cloud
// Uses Jira REST API v3 (latest version) with offset/limit pagination
type JiraProvider struct {
	*providers.BaseProvider
	domain           string // The Atlassian domain (e.g., yourcompany.atlassian.net)
	specCache        repository.OpenAPICacheRepository
	specFallback     *openapi3.T
	httpClient       *http.Client
	encryptionSvc    *security.EncryptionService
	securityMgr      *JiraSecurityManager      // Epic 4, Story 4.1 - Security Features
	observabilityMgr *JiraObservabilityManager // Epic 4, Story 4.2 - Observability Features
	cacheManager     *JiraCacheManager         // Epic 4, Story 4.3 - Caching Layer

	// Handler registry
	toolRegistry    map[string]ToolHandler
	toolsetRegistry map[string]*Toolset
	enabledToolsets map[string]bool
	mutex           sync.RWMutex
}

// Helper functions for creating results
func NewToolResult(data interface{}) *ToolResult {
	return &ToolResult{
		Success: true,
		Data:    data,
	}
}

func NewToolError(err string) *ToolResult {
	return &ToolResult{
		Success: false,
		Error:   err,
	}
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

	// Initialize security manager with default configuration
	securityConfig := GetDefaultJiraSecurityConfig()
	securityMgr, err := NewJiraSecurityManager(logger, securityConfig)
	if err != nil {
		logger.Warn("Failed to initialize Jira security manager", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Initialize observability manager with default configuration
	observabilityConfig := GetDefaultJiraObservabilityConfig()
	observabilityMgr := NewJiraObservabilityManager(observabilityConfig, logger)

	// Initialize cache manager with default configuration
	cacheConfig := GetDefaultJiraCacheConfig()
	cacheRepository := NewInMemoryJiraCacheRepository() // Use in-memory cache for now
	cacheManager := NewJiraCacheManager(cacheConfig, cacheRepository, logger)

	// Use secure HTTP client from security manager, or fallback to default
	var httpClient *http.Client
	if securityMgr != nil {
		httpClient = securityMgr.SecureHTTPClient()
	} else {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	provider := &JiraProvider{
		BaseProvider:     base,
		domain:           domain,
		specFallback:     specFallback,
		httpClient:       httpClient,
		encryptionSvc:    security.NewEncryptionService(""),
		securityMgr:      securityMgr,
		observabilityMgr: observabilityMgr,
		cacheManager:     cacheManager,
		toolRegistry:     make(map[string]ToolHandler),
		toolsetRegistry:  make(map[string]*Toolset),
		enabledToolsets:  make(map[string]bool),
	}

	// Register handlers and toolsets
	provider.registerHandlers()
	provider.enableDefaultToolsets()

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

// ValidateCredentials validates Jira credentials using passthrough authentication
// IMPORTANT: Following passthrough auth pattern - credentials are NOT stored, only validated
func (p *JiraProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
	// Build params with credentials for passthrough auth validation
	params := make(map[string]interface{})

	// Convert creds map to format expected by extractAuthToken
	// Support multiple formats for backward compatibility
	if email, hasEmail := creds["email"]; hasEmail {
		if apiToken, hasAPIToken := creds["api_token"]; hasAPIToken {
			// Build combined token for passthrough
			params["token"] = email + ":" + apiToken
		}
	} else if token, hasToken := creds["token"]; hasToken {
		params["token"] = token
	} else if accessToken, hasAccessToken := creds["access_token"]; hasAccessToken {
		// OAuth token - pass directly
		params["token"] = accessToken
	}

	// Extract authentication using passthrough pattern
	email, apiToken, err := p.extractAuthToken(ctx, params)
	if err != nil {
		// Try OAuth if basic auth extraction failed
		if token, ok := params["token"].(string); ok && !strings.Contains(token, ":") {
			// Likely an OAuth token, test it directly
			req, err := http.NewRequestWithContext(ctx, "GET", p.buildURL("/rest/api/3/myself"), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Accept", "application/json")

			resp, err := p.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to validate OAuth credentials: %w", err)
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return fmt.Errorf("invalid Jira OAuth credentials")
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("unexpected response from Jira API: %d - %s", resp.StatusCode, string(body))
			}
			return nil
		}
		return fmt.Errorf("failed to extract credentials for validation: %w", err)
	}

	// Test the credentials with the myself endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", p.buildURL("/rest/api/3/myself"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(email, apiToken))
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

// ExecuteOperation executes a Jira operation
func (p *JiraProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	// Apply configuration from context
	p.ConfigureFromContext(ctx)

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

	// Check for read-only mode
	if p.IsReadOnlyMode(ctx) && p.IsWriteOperation(operation) {
		return nil, fmt.Errorf("operation %s not allowed in read-only mode", operation)
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

// HealthCheck verifies the Jira API is accessible with comprehensive observability
func (p *JiraProvider) HealthCheck(ctx context.Context) error {
	// Use the observability manager for comprehensive health checking if available
	if p.observabilityMgr != nil {
		status := p.observabilityMgr.PerformHealthCheck(ctx, p.basicHealthCheck)
		if !status.Healthy {
			if len(status.Errors) > 0 {
				return fmt.Errorf("health check failed: %s", status.Errors[0])
			}
			return fmt.Errorf("health check failed")
		}
		return nil
	}

	// Fallback to basic health check if observability manager is not available
	return p.basicHealthCheck(ctx)
}

// basicHealthCheck performs the core health check logic
func (p *JiraProvider) basicHealthCheck(ctx context.Context) error {
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

// GetHealthStatus returns the current health status with detailed information
func (p *JiraProvider) GetHealthStatus() HealthStatus {
	if p.observabilityMgr != nil {
		return p.observabilityMgr.GetHealthStatus()
	}

	// Fallback status if observability manager is not available
	return HealthStatus{
		Healthy:     true,
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"observability_manager": "not_available",
			"basic_mode":            true,
		},
	}
}

// IsDebugMode returns whether debug mode is enabled
func (p *JiraProvider) IsDebugMode() bool {
	if p.observabilityMgr != nil {
		return p.observabilityMgr.IsDebugMode()
	}
	return false
}

// Close cleans up any resources
func (p *JiraProvider) Close() error {
	// Currently no resources to clean up
	return nil
}

// buildURL constructs the full URL for an API endpoint
// registerHandlers registers all tool handlers
func (p *JiraProvider) registerHandlers() {
	// Issue handlers
	issueHandlers := []ToolHandler{
		NewGetIssueHandler(p),
		NewCreateIssueHandler(p),
		NewUpdateIssueHandler(p),
		NewDeleteIssueHandler(p),
	}

	// Register issue handlers
	for _, handler := range issueHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}

	// Create issues toolset
	issueToolset := &Toolset{
		Name:        "issues",
		Description: "Jira issue management operations",
		Tools:       issueHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["issues"] = issueToolset

	// Search handlers
	searchHandlers := []ToolHandler{
		NewSearchIssuesHandler(p),
	}

	// Register search handlers
	for _, handler := range searchHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}

	// Create search toolset
	searchToolset := &Toolset{
		Name:        "search",
		Description: "Jira search and query operations",
		Tools:       searchHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["search"] = searchToolset

	// Comment handlers
	commentHandlers := []ToolHandler{
		NewGetCommentsHandler(p),
		NewAddCommentHandler(p),
		NewUpdateCommentHandler(p),
		NewDeleteCommentHandler(p),
	}
	// Register comment handlers
	for _, handler := range commentHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}
	// Create comments toolset
	commentToolset := &Toolset{
		Name:        "comments",
		Description: "Jira issue comment operations",
		Tools:       commentHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["comments"] = commentToolset

	// Workflow handlers
	workflowHandlers := []ToolHandler{
		NewGetTransitionsHandler(p),
		NewTransitionIssueHandler(p),
		NewGetWorkflowsHandler(p),
		NewAddWorkflowCommentHandler(p),
	}
	// Register workflow handlers
	for _, handler := range workflowHandlers {
		def := handler.GetDefinition()
		p.toolRegistry[def.Name] = handler
	}
	// Create workflow toolset
	workflowToolset := &Toolset{
		Name:        "workflow",
		Description: "Jira workflow and transition operations",
		Tools:       workflowHandlers,
		Enabled:     false,
	}
	p.toolsetRegistry["workflow"] = workflowToolset

	// TODO: Add project handlers after creating handlers_projects.go
}

// enableDefaultToolsets enables the default set of toolsets
func (p *JiraProvider) enableDefaultToolsets() {
	// Enable all toolsets by default
	for name := range p.toolsetRegistry {
		_ = p.EnableToolset(name)
	}
}

// EnableToolset enables a specific toolset
func (p *JiraProvider) EnableToolset(name string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	toolset, exists := p.toolsetRegistry[name]
	if !exists {
		return fmt.Errorf("toolset %s not found", name)
	}

	toolset.Enabled = true
	p.enabledToolsets[name] = true

	p.BaseProvider.GetLogger().Info("Enabled toolset", map[string]interface{}{
		"toolset": name,
	})

	return nil
}

// DisableToolset disables a specific toolset
func (p *JiraProvider) DisableToolset(name string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	toolset, exists := p.toolsetRegistry[name]
	if !exists {
		return fmt.Errorf("toolset %s not found", name)
	}

	toolset.Enabled = false
	delete(p.enabledToolsets, name)

	p.BaseProvider.GetLogger().Info("Disabled toolset", map[string]interface{}{
		"toolset": name,
	})

	return nil
}

// IsToolsetEnabled checks if a toolset is enabled
func (p *JiraProvider) IsToolsetEnabled(name string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.enabledToolsets[name]
}

// GetEnabledToolsets returns a list of enabled toolsets
func (p *JiraProvider) GetEnabledToolsets() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	toolsets := make([]string, 0, len(p.enabledToolsets))
	for name, enabled := range p.enabledToolsets {
		if enabled {
			toolsets = append(toolsets, name)
		}
	}
	return toolsets
}

// ConfigureFromContext applies configuration from provider context
func (p *JiraProvider) ConfigureFromContext(ctx context.Context) {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return
	}

	// Check for enabled tools configuration
	if enabledTools, ok := pctx.Metadata["ENABLED_TOOLS"].(string); ok && enabledTools != "" {
		// Disable all toolsets first
		for name := range p.toolsetRegistry {
			_ = p.DisableToolset(name) // Ignore errors when disabling during initialization
		}

		// Enable only specified toolsets
		tools := strings.Split(enabledTools, ",")
		for _, tool := range tools {
			tool = strings.TrimSpace(tool)
			if tool != "" {
				if err := p.EnableToolset(tool); err != nil {
					p.BaseProvider.GetLogger().Warn("Failed to enable toolset", map[string]interface{}{
						"toolset": tool,
						"error":   err.Error(),
					})
				}
			}
		}
	}

	// Log configuration applied
	if projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string); ok && projectFilter != "" {
		p.BaseProvider.GetLogger().Info("Applied project filter", map[string]interface{}{
			"filter": projectFilter,
		})
	}

	if readOnly, ok := pctx.Metadata["READ_ONLY"].(bool); ok && readOnly {
		p.BaseProvider.GetLogger().Info("Read-only mode enabled", nil)
	}
}

// IsReadOnlyMode checks if the provider is in read-only mode
func (p *JiraProvider) IsReadOnlyMode(ctx context.Context) bool {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return false
	}

	readOnly, ok := pctx.Metadata["READ_ONLY"].(bool)
	return ok && readOnly
}

// IsWriteOperation checks if an operation is a write operation
func (p *JiraProvider) IsWriteOperation(operation string) bool {
	writeOperations := []string{
		"create", "update", "delete", "transition", "assign",
		"add", "remove", "edit", "post", "submit", "move",
	}

	operation = strings.ToLower(operation)
	for _, writeOp := range writeOperations {
		if strings.Contains(operation, writeOp) {
			return true
		}
	}

	return false
}

// FilterProjectResults filters results based on JIRA_PROJECTS_FILTER
func (p *JiraProvider) FilterProjectResults(ctx context.Context, results interface{}) interface{} {
	pctx, ok := providers.FromContext(ctx)
	if !ok || pctx == nil || pctx.Metadata == nil {
		return results
	}

	projectFilter, ok := pctx.Metadata["JIRA_PROJECTS_FILTER"].(string)
	if !ok || projectFilter == "" || projectFilter == "*" {
		return results
	}

	// Apply filtering logic based on result type
	switch r := results.(type) {
	case map[string]interface{}:
		// Check if this is a list of issues
		if issues, ok := r["issues"].([]interface{}); ok {
			filtered := p.filterIssuesByProject(issues, projectFilter)
			r["issues"] = filtered
			r["total"] = len(filtered)
		}
		// Check if this is a list of projects
		if projects, ok := r["values"].([]interface{}); ok {
			filtered := p.filterProjects(projects, projectFilter)
			r["values"] = filtered
			r["total"] = len(filtered)
		}
	case []interface{}:
		// Direct array of items
		return p.filterItemsByProject(r, projectFilter)
	}

	return results
}

// Helper function to filter issues by project
func (p *JiraProvider) filterIssuesByProject(issues []interface{}, filter string) []interface{} {
	filtered := []interface{}{}
	allowedProjects := strings.Split(filter, ",")

	for _, issue := range issues {
		if issueMap, ok := issue.(map[string]interface{}); ok {
			if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
				if project, ok := fields["project"].(map[string]interface{}); ok {
					if key, ok := project["key"].(string); ok {
						if p.isProjectAllowed(key, allowedProjects) {
							filtered = append(filtered, issue)
						}
					}
				}
			}
		}
	}

	return filtered
}

// Helper function to filter projects
func (p *JiraProvider) filterProjects(projects []interface{}, filter string) []interface{} {
	filtered := []interface{}{}
	allowedProjects := strings.Split(filter, ",")

	for _, project := range projects {
		if projectMap, ok := project.(map[string]interface{}); ok {
			if key, ok := projectMap["key"].(string); ok {
				if p.isProjectAllowed(key, allowedProjects) {
					filtered = append(filtered, project)
				}
			}
		}
	}

	return filtered
}

// Helper function to filter generic items by project
func (p *JiraProvider) filterItemsByProject(items []interface{}, filter string) []interface{} {
	filtered := []interface{}{}
	allowedProjects := strings.Split(filter, ",")

	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Try different project key locations
			var projectKey string
			if key, ok := itemMap["projectKey"].(string); ok {
				projectKey = key
			} else if project, ok := itemMap["project"].(map[string]interface{}); ok {
				if key, ok := project["key"].(string); ok {
					projectKey = key
				}
			}

			if projectKey != "" && p.isProjectAllowed(projectKey, allowedProjects) {
				filtered = append(filtered, item)
			}
		}
	}

	return filtered
}

// Helper function to check if a project is allowed
func (p *JiraProvider) isProjectAllowed(projectKey string, allowedProjects []string) bool {
	for _, allowed := range allowedProjects {
		allowed = strings.TrimSpace(allowed)
		if allowed == projectKey || allowed == "*" {
			return true
		}
	}
	return false
}

// extractAuthToken extracts authentication token from context or params (passthrough auth)
// Following GitHub provider pattern exactly for passthrough authentication
func (p *JiraProvider) extractAuthToken(ctx context.Context, params map[string]interface{}) (string, string, error) {
	// Priority 1: Try from ProviderContext first (standard provider pattern)
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil {
		// Check for token in main Credentials field
		if pctx.Credentials != nil && pctx.Credentials.Token != "" {
			// For Jira, token might be in email:api_token format
			parts := strings.Split(pctx.Credentials.Token, ":")
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
			// Return error if token format is invalid
			if !strings.Contains(pctx.Credentials.Token, ":") {
				return "", "", fmt.Errorf("invalid token format, expected email:api_token")
			}
		}

		// Check Metadata (newer pattern)
		if pctx.Metadata != nil {
			// Check for token in metadata
			if token, ok := pctx.Metadata["token"].(string); ok && token != "" {
				parts := strings.Split(token, ":")
				if len(parts) == 2 {
					return parts[0], parts[1], nil
				}
			}
			// Check for separate email and api_token in metadata
			if email, ok := pctx.Metadata["email"].(string); ok && email != "" {
				if apiToken, ok := pctx.Metadata["api_token"].(string); ok && apiToken != "" {
					return email, apiToken, nil
				}
				// Check for OAuth access_token
				if accessToken, ok := pctx.Metadata["access_token"].(string); ok && accessToken != "" {
					return email, accessToken, nil
				}
			}
		}

		// Also check custom credentials map (legacy)
		if pctx.Credentials != nil && pctx.Credentials.Custom != nil {
			if token, ok := pctx.Credentials.Custom["token"]; ok && token != "" {
				parts := strings.Split(token, ":")
				if len(parts) == 2 {
					return parts[0], parts[1], nil
				}
			}
			// Check for Jira-specific keys
			if token, ok := pctx.Credentials.Custom["jira_token"]; ok && token != "" {
				parts := strings.Split(token, ":")
				if len(parts) == 2 {
					return parts[0], parts[1], nil
				}
			}
			// Check for separate email and api_token in custom
			if email, ok := pctx.Credentials.Custom["email"]; ok && email != "" {
				if apiToken, ok := pctx.Credentials.Custom["api_token"]; ok && apiToken != "" {
					return email, apiToken, nil
				}
			}
		}
	}

	// Priority 2: Try passthrough auth from params
	if auth, ok := params["__passthrough_auth"].(map[string]interface{}); ok {
		// Check for encrypted token
		if encryptedToken, ok := auth["encrypted_token"].(string); ok && encryptedToken != "" {
			// Extract tenant ID for decryption
			tenantID := p.extractTenantID(ctx, params)
			decrypted, err := p.encryptionSvc.DecryptCredential([]byte(encryptedToken), tenantID)
			if err != nil {
				return "", "", fmt.Errorf("failed to decrypt token: %w", err)
			}
			// Parse decrypted token (email:api_token format)
			parts := strings.Split(decrypted, ":")
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
		}

		// Check for plain token (development mode)
		if token, ok := auth["token"].(string); ok && token != "" {
			parts := strings.Split(token, ":")
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
		}

		// Try separate email and api_token in passthrough
		if email, ok := auth["email"].(string); ok && email != "" {
			if apiToken, ok := auth["api_token"].(string); ok && apiToken != "" {
				return email, apiToken, nil
			}
		}
	}

	// Priority 3: Try direct token from params (backward compatibility)
	if token, ok := params["token"].(string); ok && token != "" {
		parts := strings.Split(token, ":")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
		// Return specific error for invalid format
		if !strings.Contains(token, ":") {
			return "", "", fmt.Errorf("invalid token format, expected email:api_token")
		}
	}

	// Priority 4: Try direct email/api_token from params (backward compatibility)
	if email, ok := params["email"].(string); ok && email != "" {
		if apiToken, ok := params["api_token"].(string); ok && apiToken != "" {
			return email, apiToken, nil
		}
		// Check for OAuth access_token
		if accessToken, ok := params["access_token"].(string); ok && accessToken != "" {
			return email, accessToken, nil
		}
		// Return specific error if email exists but token is missing
		return "", "", fmt.Errorf("email provided but api_token missing")
	}

	// Priority 5: Try from context value
	if token, ok := ctx.Value(jiraTokenKey).(string); ok {
		parts := strings.Split(token, ":")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("no authentication credentials found")
}

// extractTenantID extracts tenant ID from context or params
func (p *JiraProvider) extractTenantID(ctx context.Context, params map[string]interface{}) string {
	// Try from ProviderContext
	if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.TenantID != "" {
		return pctx.TenantID
	}
	// Try from params
	if tenantID, ok := params["tenant_id"].(string); ok && tenantID != "" {
		return tenantID
	}
	// Try from passthrough auth
	if auth, ok := params["__passthrough_auth"].(map[string]interface{}); ok {
		if tenantID, ok := auth["tenant_id"].(string); ok && tenantID != "" {
			return tenantID
		}
	}
	// Try from context value
	if tenantID, ok := ctx.Value("tenant_id").(string); ok && tenantID != "" {
		return tenantID
	}
	return ""
}

func (p *JiraProvider) buildURL(path string) string {
	// Check if domain looks like a full URL (for testing)
	if strings.HasPrefix(p.domain, "http://") || strings.HasPrefix(p.domain, "https://") {
		baseURL := strings.TrimRight(p.domain, "/")
		path = strings.TrimLeft(path, "/")
		return fmt.Sprintf("%s/%s", baseURL, path)
	}

	// Normal flow for production
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

// secureHTTPDo performs an HTTP request with security and observability features
// Epic 4, Story 4.1 - Security (PII detection, sanitization, audit logging)
// Epic 4, Story 4.2 - Observability (operation tracking, metrics, error handling)
// Epic 4, Story 4.3 - Caching Layer (response caching, ETags support, cache invalidation)
func (p *JiraProvider) secureHTTPDo(ctx context.Context, req *http.Request, operation string) (*http.Response, error) {
	start := time.Now()

	// Start operation tracking with observability manager
	var operationFinish func(error)
	if p.observabilityMgr != nil {
		ctx, operationFinish = p.observabilityMgr.StartOperation(ctx, operation)
	}

	// Epic 4, Story 4.3 - Check cache for GET requests before making HTTP call
	var cacheEntry *CacheEntry
	if p.cacheManager != nil && p.cacheManager.IsCacheable(req.Method, operation) {
		// Convert headers to map[string]string for cache manager
		headerMap := make(map[string]string)
		for key, values := range req.Header {
			if len(values) > 0 {
				headerMap[key] = values[0] // Use first value
			}
		}

		// Check cache first
		var err error
		cacheEntry, err = p.cacheManager.Get(ctx, req.Method, req.URL.String(), operation, headerMap)
		if err == nil && cacheEntry != nil {
			// Cache hit - add conditional headers using cache manager
			p.cacheManager.AddConditionalHeaders(req, cacheEntry)
		}
	}

	// Apply request sanitization if security manager is available
	if p.securityMgr != nil {
		// Sanitize the request
		if err := p.securityMgr.SanitizeRequest(req); err != nil {
			// Log through embedded BaseProvider
			logger := p.GetLogger()
			logger.Warn("Request sanitization failed", map[string]interface{}{
				"error": err.Error(),
				"url":   req.URL.String(),
			})
		}

		// Log security event for the request
		p.securityMgr.LogSecurityEvent("http_request", map[string]interface{}{
			"operation": operation,
			"url":       req.URL.String(),
			"method":    req.Method,
		})
	}

	// Execute the HTTP request
	resp, err := p.httpClient.Do(req)
	duration := time.Since(start)

	// Handle errors with comprehensive categorization
	if err != nil {
		// Categorize error with observability manager
		categorizedErr := err
		if p.observabilityMgr != nil {
			categorizedErr = p.observabilityMgr.CategorizeError(err, operation, duration)
		}

		// Finish operation tracking
		if operationFinish != nil {
			operationFinish(categorizedErr)
		}

		// Security logging
		if p.securityMgr != nil {
			p.securityMgr.LogSecurityEvent("http_request_error", map[string]interface{}{
				"operation": operation,
				"url":       req.URL.String(),
				"error":     err.Error(),
			})
		}

		return resp, categorizedErr
	}

	// Epic 4, Story 4.3 - Handle 304 Not Modified responses (cache still valid)
	if resp.StatusCode == http.StatusNotModified && cacheEntry != nil && p.cacheManager != nil {
		// Use cache manager to handle conditional response
		cachedResp, _, isValid := p.cacheManager.HandleConditionalResponse(resp, cacheEntry)
		if isValid && cachedResp != nil {
			// Finish successful operation tracking
			if operationFinish != nil {
				operationFinish(nil)
			}

			return cachedResp, nil
		}
	}

	// Record HTTP metrics
	if p.observabilityMgr != nil {
		endpoint := req.URL.Path
		if endpoint == "" {
			endpoint = req.URL.String()
		}
		p.observabilityMgr.RecordHTTPMetrics(req.Method, endpoint, resp.StatusCode, duration)
	}

	// Apply response processing if security manager is available
	if p.securityMgr != nil {
		// Read response body for PII detection and sanitization
		if resp.Body != nil {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err := resp.Body.Close(); err != nil {
				logger := p.GetLogger()
				logger.Warn("Failed to close response body", map[string]interface{}{
					"error": err.Error(),
				})
			}
			if err != nil {
				logger := p.GetLogger()
				logger.Warn("Failed to read response body for security processing", map[string]interface{}{
					"error": err.Error(),
				})

				// Categorize and finish operation
				categorizedErr := err
				if p.observabilityMgr != nil {
					categorizedErr = p.observabilityMgr.CategorizeError(err, operation, duration)
				}
				if operationFinish != nil {
					operationFinish(categorizedErr)
				}

				return resp, categorizedErr
			}

			// Detect PII in response
			piiTypes, err := p.securityMgr.DetectPII(bodyBytes)
			if err != nil {
				logger := p.GetLogger()
				logger.Warn("PII detection failed", map[string]interface{}{
					"error": err.Error(),
				})
			} else if len(piiTypes) > 0 {
				p.securityMgr.LogSecurityEvent("response_pii_detected", map[string]interface{}{
					"operation": operation,
					"url":       req.URL.String(),
					"pii_types": piiTypes,
				})
			}

			// Sanitize response if needed
			sanitizedBody, err := p.securityMgr.SanitizeResponse(bodyBytes)
			if err != nil {
				logger := p.GetLogger()
				logger.Warn("Response sanitization failed", map[string]interface{}{
					"error": err.Error(),
				})
				sanitizedBody = bodyBytes // Use original if sanitization fails
			}

			// Replace response body with sanitized version
			resp.Body = io.NopCloser(strings.NewReader(string(sanitizedBody)))
			resp.ContentLength = int64(len(sanitizedBody))
		}

		// Log successful response
		p.securityMgr.LogSecurityEvent("http_response", map[string]interface{}{
			"operation":   operation,
			"url":         req.URL.String(),
			"status_code": resp.StatusCode,
		})
	}

	// Epic 4, Story 4.3 - Cache successful responses for GET requests
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && p.cacheManager != nil &&
		p.cacheManager.IsCacheable(req.Method, operation) {

		// Read response body for caching (need to preserve it for caller)
		if resp.Body != nil {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				// Convert headers to map[string]string
				headerMap := make(map[string]string)
				for key, values := range req.Header {
					if len(values) > 0 {
						headerMap[key] = values[0]
					}
				}

				// Cache the successful response
				if err := p.cacheManager.Set(ctx, req.Method, req.URL.String(), operation, headerMap, resp, bodyBytes); err != nil {
					// Log caching failure but don't fail the request
					logger := p.GetLogger()
					logger.Warn("Failed to cache response", map[string]interface{}{
						"error":     err.Error(),
						"operation": operation,
						"url":       req.URL.String(),
					})
				}

				// Replace the response body so caller can still read it
				resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
				resp.ContentLength = int64(len(bodyBytes))
			}
		}
	}

	// Handle HTTP errors (4xx, 5xx) with proper categorization
	if resp.StatusCode >= 400 {
		httpErr := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)

		// Categorize HTTP error
		var categorizedErr = httpErr
		if p.observabilityMgr != nil {
			categorizedErr = p.observabilityMgr.CategorizeError(httpErr, operation, duration)
		}

		// Finish operation tracking with error
		if operationFinish != nil {
			operationFinish(categorizedErr)
		}

		return resp, categorizedErr
	}

	// Epic 4, Story 4.3 - Cache invalidation for write operations
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && p.cacheManager != nil {
		// Invalidate cache for write operations (POST, PUT, DELETE, PATCH)
		if req.Method != "GET" && req.Method != "HEAD" && req.Method != "OPTIONS" {
			if err := p.cacheManager.InvalidateByOperation(ctx, operation); err != nil {
				// Log invalidation failure but don't fail the request
				logger := p.GetLogger()
				logger.Warn("Failed to invalidate cache", map[string]interface{}{
					"error":     err.Error(),
					"operation": operation,
					"method":    req.Method,
					"url":       req.URL.String(),
				})
			}
		}
	}

	// Finish successful operation tracking
	if operationFinish != nil {
		operationFinish(nil)
	}

	return resp, nil
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
