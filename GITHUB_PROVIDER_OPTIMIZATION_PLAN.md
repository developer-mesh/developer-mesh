# GitHub Provider Implementation Checklist

## Overview
Transform our basic GitHub provider into a comprehensive solution with 150+ operations matching the official GitHub MCP server capabilities.

## Step 1: Foundation Setup

### 1.1 Add Dependencies
- [ ] Add `github.com/google/go-github/v74` to go.mod
- [ ] Add `github.com/shurcooL/githubv4` for GraphQL support
- [ ] Run `go mod tidy` to update dependencies

**Implementation Details:**
```bash
# In go.mod, add:
require (
    github.com/google/go-github/v74 v74.0.0
    github.com/shurcooL/githubv4 v0.0.0-20240120211514-18a1ae0e79dc
)
```

### 1.2 Create GitHub Client Wrapper
- [ ] Create `pkg/tools/providers/github/github_client.go`
- [ ] Implement REST client initialization with auth
- [ ] Implement GraphQL client initialization
- [ ] Add rate limiter wrapper
- [ ] Add retry logic with exponential backoff

**Implementation Pattern (from official MCP):**
```go
// github_client.go structure:
type GitHubClients struct {
    REST     *github.Client
    GraphQL  *githubv4.Client
    Raw      *RawClient  // For raw file access
}

// Client creation pattern (from server.go):
func NewGitHubClients(token string, host string) (*GitHubClients, error) {
    // REST client setup
    restClient := github.NewClient(nil).WithAuthToken(token)
    restClient.UserAgent = "devops-mcp-github/1.0"
    
    // GraphQL client setup with bearer auth transport
    httpClient := &http.Client{
        Transport: &BearerAuthTransport{
            Transport: http.DefaultTransport,
            Token:     token,
        },
    }
    gqlClient := githubv4.NewEnterpriseClient(apiURL, httpClient)
    
    return &GitHubClients{
        REST:    restClient,
        GraphQL: gqlClient,
    }, nil
}
```

### 1.3 Refactor Provider Structure
- [ ] Create `pkg/tools/providers/github/github_toolsets.go`
- [ ] Define toolset structure with enable/disable capability
- [ ] Implement toolset registration pattern
- [ ] Add read-only mode support flag
- [ ] Update `github_provider.go` to use new client

**Implementation Pattern (from toolsets.go):**
```go
// Toolset structure pattern:
type GitHubToolset struct {
    Name        string
    Description string
    Enabled     bool
    ReadOnly    bool
    Tools       []ToolDefinition
}

// Tool handler pattern (each tool returns definition + handler):
func CreateIssue(getClient GetClientFn) (tool ToolDef, handler HandlerFunc) {
    return ToolDef{
        Name: "issues/create",
        Params: []Param{...},
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        // Handler implementation
    }
}
```

## Step 2: Core Tool Implementation

### 2.1 Repository Operations
- [x] repos/list
- [x] repos/get
- [x] repos/create
- [x] repos/update
- [x] repos/delete
- [ ] repos/fork
- [ ] repos/list_branches
- [ ] repos/create_branch
- [ ] repos/list_commits
- [ ] repos/get_commit
- [ ] repos/list_tags
- [ ] repos/get_tag
- [ ] repos/list_releases
- [ ] repos/get_latest_release
- [ ] repos/get_release_by_tag
- [ ] repos/create_or_update_file
- [ ] repos/delete_file
- [ ] repos/get_file_contents
- [ ] repos/push_files (multi-file operation)
- [ ] repos/search_repositories
- [ ] repos/search_code

**Implementation Pattern for Each Tool:**
```go
// Example: repos/get_commit implementation pattern
func GetCommit(getClient GetClientFn) (tool ToolDef, handler HandlerFunc) {
    return ToolDef{
        Name: "repos/get_commit",
        Description: "Get commit details",
        Params: []Param{
            {Name: "owner", Type: "string", Required: true},
            {Name: "repo", Type: "string", Required: true},
            {Name: "sha", Type: "string", Required: true},
            {Name: "include_diff", Type: "bool", Default: true},
        },
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        client, _ := getClient(ctx)
        
        // Use GitHub SDK directly
        commit, resp, err := client.Repositories.GetCommit(
            ctx, 
            params["owner"].(string),
            params["repo"].(string), 
            params["sha"].(string),
            nil,
        )
        
        // Handle GitHub-specific errors
        if err != nil {
            return handleGitHubError(resp, err)
        }
        
        // Convert to minimal response
        return convertToMinimalCommit(commit), nil
    }
}

// Multi-file operation pattern (repos/push_files):
func PushFiles(getClient GetClientFn) (tool ToolDef, handler HandlerFunc) {
    // Uses tree and commit API to push multiple files atomically
    // 1. Get current branch ref
    // 2. Create blobs for each file
    // 3. Create new tree with all changes
    // 4. Create commit pointing to new tree
    // 5. Update branch ref to new commit
}
```

### 2.2 Issue Operations
- [x] issues/list
- [x] issues/get
- [x] issues/create
- [x] issues/update
- [ ] issues/add_comment
- [ ] issues/get_comments
- [ ] issues/search
- [ ] issues/add_sub_issue
- [ ] issues/remove_sub_issue
- [ ] issues/reprioritize_sub_issue
- [ ] issues/list_sub_issues
- [ ] issues/list_issue_types
- [ ] issues/assign_copilot

**GraphQL Implementation Pattern (for complex queries):**
```go
// issues/list uses GraphQL for efficient filtering
func ListIssues(getGQLClient GetGQLClientFn) (tool ToolDef, handler HandlerFunc) {
    return ToolDef{
        Name: "issues/list",
        Params: []Param{
            {Name: "owner", Required: true},
            {Name: "repo", Required: true},
            {Name: "labels", Type: "[]string"},
            {Name: "state", Default: "open"},
            {Name: "after", Type: "string"}, // Cursor pagination
        },
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        // GraphQL query structure
        var query struct {
            Repository struct {
                Issues struct {
                    Nodes []IssueFragment
                    PageInfo struct {
                        HasNextPage bool
                        EndCursor   string
                    }
                    TotalCount int
                } `graphql:"issues(first: $first, after: $after, states: $states, labels: $labels)"`
            } `graphql:"repository(owner: $owner, name: $repo)"`
        }
        
        // Execute GraphQL query
        variables := map[string]interface{}{
            "owner": githubv4.String(params["owner"].(string)),
            "repo":  githubv4.String(params["repo"].(string)),
            "first": githubv4.Int(30),
            "after": (*githubv4.String)(nil), // Handle cursor
        }
        
        client, _ := getGQLClient(ctx)
        err := client.Query(ctx, &query, variables)
        
        // Return with pagination info
        return map[string]interface{}{
            "issues": query.Repository.Issues.Nodes,
            "pageInfo": query.Repository.Issues.PageInfo,
            "totalCount": query.Repository.Issues.TotalCount,
        }, err
    }
}
```

### 2.3 Pull Request Operations
- [x] pulls/list
- [x] pulls/get
- [x] pulls/create
- [x] pulls/merge
- [ ] pulls/update
- [ ] pulls/update_branch
- [ ] pulls/get_diff
- [ ] pulls/get_files
- [ ] pulls/get_reviews
- [ ] pulls/get_review_comments
- [ ] pulls/create_review
- [ ] pulls/submit_review
- [ ] pulls/add_review_comment
- [ ] pulls/request_copilot_review
- [ ] pulls/search

**PR Review Implementation Pattern:**
```go
// Complex PR review workflow using GraphQL
func CreatePullRequestReview(getGQLClient GetGQLClientFn) (tool ToolDef, handler HandlerFunc) {
    // GraphQL mutation for creating review
    return ToolDef{
        Name: "pulls/create_review",
        Params: []Param{
            {Name: "owner", Required: true},
            {Name: "repo", Required: true},
            {Name: "pull_number", Required: true},
            {Name: "event", Required: true}, // APPROVE, REQUEST_CHANGES, COMMENT
            {Name: "body", Type: "string"},
        },
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        // First, get PR node ID via REST or GraphQL
        // Then, create review using mutation
        var mutation struct {
            AddPullRequestReview struct {
                PullRequestReview struct {
                    ID string
                    State string
                    Body string
                }
            } `graphql:"addPullRequestReview(input: $input)"`
        }
        
        input := githubv4.AddPullRequestReviewInput{
            PullRequestID: pullRequestNodeID,
            Event: githubv4.PullRequestReviewEvent(params["event"].(string)),
            Body: githubv4.String(params["body"].(string)),
        }
        
        return executeMutation(ctx, &mutation, input)
    }
}
```

### 2.4 GitHub Actions Operations
- [x] actions/list-repo-workflows
- [x] actions/create-workflow-dispatch
- [x] actions/list-workflow-runs
- [x] actions/get-workflow-run
- [x] actions/list-workflow-run-jobs
- [x] actions/rerun-workflow
- [x] actions/list-runs-for-workflow
- [ ] actions/cancel_workflow_run
- [ ] actions/rerun_failed_jobs
- [ ] actions/get_job_logs
- [ ] actions/get_workflow_run_logs
- [ ] actions/get_workflow_run_usage
- [ ] actions/download_artifact
- [ ] actions/list_artifacts
- [ ] actions/delete_workflow_run_logs

### 2.5 User & Organization Operations
- [x] users/get (current implementation)
- [ ] users/search
- [ ] users/get_me
- [ ] orgs/list
- [ ] orgs/search
- [ ] teams/list
- [ ] teams/get_members

## Step 3: Advanced Features

### 3.1 Security Tools
- [ ] code_scanning/list_alerts
- [ ] code_scanning/get_alert
- [ ] dependabot/list_alerts
- [ ] dependabot/get_alert
- [ ] secret_scanning/list_alerts
- [ ] secret_scanning/get_alert
- [ ] security_advisories/list
- [ ] security_advisories/get
- [ ] security_advisories/list_global

**Security Tools Implementation Pattern:**
```go
// Security scanning implementation
func ListCodeScanningAlerts(getClient GetClientFn) (tool ToolDef, handler HandlerFunc) {
    return ToolDef{
        Name: "code_scanning/list_alerts",
        Params: []Param{
            {Name: "owner", Required: true},
            {Name: "repo", Required: true},
            {Name: "state", Default: "open"}, // open, closed, dismissed, fixed
            {Name: "severity"}, // critical, high, medium, low, warning, note, error
            {Name: "tool_name"}, // Filter by tool
        },
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        client, _ := getClient(ctx)
        
        opts := &github.AlertListOptions{
            State:    params["state"].(string),
            Severity: params["severity"].(string),
            Tool:     params["tool_name"].(string),
        }
        
        alerts, resp, err := client.CodeScanning.ListAlertsForRepo(
            ctx,
            params["owner"].(string),
            params["repo"].(string),
            opts,
        )
        
        // Convert to minimal format
        return convertSecurityAlerts(alerts), handleError(resp, err)
    }
}
```

### 3.2 Collaboration Tools
- [ ] discussions/list
- [ ] discussions/get
- [ ] discussions/get_comments
- [ ] discussions/list_categories
- [ ] notifications/list
- [ ] notifications/dismiss
- [ ] notifications/mark_all_read
- [ ] notifications/get_details
- [ ] notifications/manage_subscription
- [ ] gists/create
- [ ] gists/list
- [ ] gists/update

### 3.3 Git Operations
- [ ] git/create_blob
- [ ] git/get_blob
- [ ] git/create_tree
- [ ] git/get_tree
- [ ] git/create_commit
- [ ] git/get_commit
- [ ] git/create_ref
- [ ] git/update_ref
- [ ] git/delete_ref
- [ ] git/get_ref

## Step 4: MCP Protocol Enhancement

### 4.1 Resource Support
- [ ] Create `github_resources.go`
- [ ] Implement repository resource
- [ ] Implement user/team context resource
- [ ] Implement workflow status resource
- [ ] Add resource templates with URI patterns

**MCP Resource Implementation Pattern:**
```go
// github_resources.go

// Resource template for repository browsing
func GetRepositoryResourceContent(getClient GetClientFn) ResourceTemplate {
    return ResourceTemplate{
        URITemplate: "github://repos/{owner}/{repo}/{path}",
        Name: "Repository File Browser",
        Description: "Browse repository files and directories",
        MimeTypes: ["text/plain", "application/json"],
    }, func(ctx context.Context, uri string) (*ResourceContent, error) {
        // Parse URI to extract owner, repo, path
        parts := parseGitHubURI(uri)
        
        client, _ := getClient(ctx)
        
        // Get file content or directory listing
        if isDirectory(parts.Path) {
            contents, _, err := client.Repositories.GetContents(
                ctx, parts.Owner, parts.Repo, parts.Path, nil,
            )
            return &ResourceContent{
                URI: uri,
                MimeType: "application/json",
                Content: marshalDirectoryListing(contents),
            }, err
        }
        
        // Get file content
        file, _, err := client.Repositories.DownloadContents(
            ctx, parts.Owner, parts.Repo, parts.Path, nil,
        )
        return &ResourceContent{
            URI: uri,
            MimeType: detectMimeType(parts.Path),
            Content: file,
        }, err
    }
}

// Context resource for current user/session
func GetContextResource(getClient GetClientFn) ResourceTemplate {
    return ResourceTemplate{
        URITemplate: "github://context/me",
        Name: "Current User Context",
    }, func(ctx context.Context, uri string) (*ResourceContent, error) {
        client, _ := getClient(ctx)
        user, _, err := client.Users.Get(ctx, "")
        
        return &ResourceContent{
            URI: uri,
            MimeType: "application/json",
            Content: marshalUser(user),
        }, err
    }
}
```

### 4.2 Prompt Support
- [ ] Create `github_prompts.go`
- [ ] Add workflow creation prompts
- [ ] Add issue triage prompts
- [ ] Add PR review prompts
- [ ] Add release management prompts

**MCP Prompt Implementation Pattern:**
```go
// github_prompts.go

func IssueToFixWorkflowPrompt() PromptDefinition {
    return PromptDefinition{
        Name: "issue_to_fix_workflow",
        Description: "Convert an issue into a fix workflow",
        Arguments: []PromptArgument{
            {Name: "issue_number", Required: true},
            {Name: "owner", Required: true},
            {Name: "repo", Required: true},
        },
    }, func(args map[string]interface{}) (*PromptResult, error) {
        // Get issue details
        issue := getIssue(args["owner"], args["repo"], args["issue_number"])
        
        // Generate workflow prompt
        return &PromptResult{
            Messages: []Message{
                {
                    Role: "system",
                    Content: "You are a GitHub workflow expert...",
                },
                {
                    Role: "user", 
                    Content: fmt.Sprintf(
                        "Create a fix for issue #%d: %s\n\nDescription: %s",
                        issue.Number, issue.Title, issue.Body,
                    ),
                },
            },
        }, nil
    }
}
```

### 4.3 Tool Annotations
- [ ] Add ReadOnlyHint to all read operations
- [ ] Add user-friendly titles to all tools
- [ ] Add detailed descriptions with examples
- [ ] Add parameter validation schemas

**Tool Annotation Pattern:**
```go
// Pattern for tool with full MCP annotations
func CreateIssue(getClient GetClientFn) (tool ToolDef, handler HandlerFunc) {
    return ToolDef{
        Name: "issues/create",
        Description: "Create a new issue in a GitHub repository",
        Annotations: ToolAnnotation{
            Title: "Create Issue",
            ReadOnlyHint: false, // Write operation
            Examples: []Example{
                {
                    Input: map[string]interface{}{
                        "owner": "octocat",
                        "repo": "hello-world",
                        "title": "Bug: Application crashes",
                        "body": "Steps to reproduce...",
                    },
                    Output: "Created issue #123",
                },
            },
        },
        InputSchema: JSONSchema{
            Type: "object",
            Properties: map[string]Schema{
                "owner": {Type: "string", Pattern: "^[a-zA-Z0-9-]+$"},
                "repo": {Type: "string", Pattern: "^[a-zA-Z0-9-_.]+$"},
                "title": {Type: "string", MinLength: 1, MaxLength: 256},
                "body": {Type: "string"},
                "labels": {Type: "array", Items: {Type: "string"}},
                "assignees": {Type: "array", Items: {Type: "string"}},
            },
            Required: []string{"owner", "repo", "title"},
        },
    }, handler
}

// Helper to mark read-only operations
func ToBoolPtr(b bool) *bool {
    return &b
}
```

## Step 5: Infrastructure Features

### 5.1 Pagination Support
- [ ] Create `github_pagination.go`
- [ ] Implement cursor-based pagination for GraphQL
- [ ] Implement offset-based pagination for REST
- [ ] Add PageInfo to all list responses
- [ ] Add default page size constants

**Pagination Implementation Pattern (from official MCP):**
```go
// github_pagination.go

// Pagination types
type PaginationParams struct {
    Page    int
    PerPage int
    After   string // For cursor pagination
}

type CursorPaginationParams struct {
    PerPage int
    After   string
}

// Helper functions pattern from server.go
func WithPagination() []ParamDef {
    return []ParamDef{
        {Name: "page", Type: "number", Min: 1},
        {Name: "perPage", Type: "number", Min: 1, Max: 100},
    }
}

func WithCursorPagination() []ParamDef {
    return []ParamDef{
        {Name: "after", Type: "string", Description: "Cursor for pagination"},
        {Name: "perPage", Type: "number", Min: 1, Max: 100},
    }
}

// Extract pagination from params
func ExtractPagination(params map[string]interface{}) PaginationParams {
    page := 1
    if p, ok := params["page"].(float64); ok {
        page = int(p)
    }
    
    perPage := 30 // Default
    if pp, ok := params["perPage"].(float64); ok {
        perPage = int(pp)
    }
    
    after := ""
    if a, ok := params["after"].(string); ok {
        after = a
    }
    
    return PaginationParams{Page: page, PerPage: perPage, After: after}
}

// GraphQL pagination helper
func (p CursorPaginationParams) ToGraphQLParams() map[string]interface{} {
    vars := map[string]interface{}{
        "first": githubv4.Int(p.PerPage),
    }
    if p.After != "" {
        vars["after"] = githubv4.String(p.After)
    }
    return vars
}
```

### 5.2 Error Handling
- [ ] Create `github_errors.go`
- [ ] Define GitHub-specific error types
- [ ] Add rate limit error handling
- [ ] Add authentication error handling
- [ ] Add validation error handling

**Error Handling Pattern (from errors package):**
```go
// github_errors.go

// GitHub error response wrapper
func NewGitHubAPIErrorResponse(ctx context.Context, message string, resp *github.Response, err error) error {
    if resp == nil {
        return fmt.Errorf("%s: %w", message, err)
    }
    
    // Check for specific error types
    var rateLimitErr *github.RateLimitError
    if errors.As(err, &rateLimitErr) {
        return &RateLimitError{
            Message: message,
            ResetAt: rateLimitErr.Rate.Reset.Time,
        }
    }
    
    var abuseRateErr *github.AbuseRateLimitError
    if errors.As(err, &abuseRateErr) {
        return &AbuseRateLimitError{
            Message:     message,
            RetryAfter:  abuseRateErr.RetryAfter,
        }
    }
    
    // Generic error with status code
    return &GitHubAPIError{
        Message:    message,
        StatusCode: resp.StatusCode,
        Body:       extractBody(resp),
    }
}

// Custom error types
type RateLimitError struct {
    Message string
    ResetAt time.Time
}

type AbuseRateLimitError struct {
    Message    string
    RetryAfter time.Duration
}

// Helper to check if error is retryable
func IsRetryableError(err error) bool {
    var rateLimitErr *RateLimitError
    var abuseErr *AbuseRateLimitError
    return errors.As(err, &rateLimitErr) || errors.As(err, &abuseErr)
}
```

### 5.3 Search Implementation
- [ ] Create `github_search.go`
- [ ] Implement code search with syntax support
- [ ] Implement issue/PR search with filters
- [ ] Implement user/org search
- [ ] Implement repository search

**Search Implementation Pattern:**
```go
// github_search.go

func SearchCode(getClient GetClientFn) (tool ToolDef, handler HandlerFunc) {
    return ToolDef{
        Name: "search/code",
        Params: []Param{
            {Name: "query", Required: true, Description: "GitHub search syntax"},
            {Name: "sort", Type: "string"}, // indexed
            {Name: "order", Type: "string"}, // asc, desc
        },
    }, func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
        client, _ := getClient(ctx)
        
        opts := &github.SearchOptions{
            Sort:  params["sort"].(string),
            Order: params["order"].(string),
            ListOptions: github.ListOptions{
                PerPage: 30,
            },
        }
        
        // Use GitHub's search API
        results, resp, err := client.Search.Code(
            ctx,
            params["query"].(string),
            opts,
        )
        
        // Return with search metadata
        return map[string]interface{}{
            "total_count": results.GetTotalCount(),
            "incomplete": results.GetIncompleteResults(),
            "items": convertCodeResults(results.CodeResults),
        }, handleError(resp, err)
    }
}

// Search query builder helper
func BuildSearchQuery(filters map[string]interface{}) string {
    parts := []string{}
    
    // Add type qualifiers
    if repo, ok := filters["repo"].(string); ok {
        parts = append(parts, fmt.Sprintf("repo:%s", repo))
    }
    if lang, ok := filters["language"].(string); ok {
        parts = append(parts, fmt.Sprintf("language:%s", lang))
    }
    if user, ok := filters["user"].(string); ok {
        parts = append(parts, fmt.Sprintf("user:%s", user))
    }
    
    // Add the main query
    if q, ok := filters["q"].(string); ok {
        parts = append(parts, q)
    }
    
    return strings.Join(parts, " ")
}
```

### 5.4 Caching Layer
- [ ] Create `github_cache.go`
- [ ] Implement in-memory cache for frequently accessed data
- [ ] Add TTL management
- [ ] Add cache invalidation logic
- [ ] Add cache metrics

**Cache Implementation Pattern:**
```go
// github_cache.go

type GitHubCache struct {
    mu    sync.RWMutex
    items map[string]*CacheItem
    ttl   time.Duration
}

type CacheItem struct {
    Value     interface{}
    ExpiresAt time.Time
}

func NewGitHubCache(defaultTTL time.Duration) *GitHubCache {
    cache := &GitHubCache{
        items: make(map[string]*CacheItem),
        ttl:   defaultTTL,
    }
    
    // Start cleanup goroutine
    go cache.cleanupExpired()
    return cache
}

func (c *GitHubCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    item, ok := c.items[key]
    if !ok || time.Now().After(item.ExpiresAt) {
        return nil, false
    }
    return item.Value, true
}

func (c *GitHubCache) Set(key string, value interface{}, ttl ...time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    expiry := c.ttl
    if len(ttl) > 0 {
        expiry = ttl[0]
    }
    
    c.items[key] = &CacheItem{
        Value:     value,
        ExpiresAt: time.Now().Add(expiry),
    }
}

// Cache key builder for consistent keys
func BuildCacheKey(resource string, params ...string) string {
    parts := append([]string{resource}, params...)
    return strings.Join(parts, ":")
}
```

## Step 6: Enterprise Features

### 6.1 GitHub Enterprise Support
- [ ] Add host configuration option
- [ ] Support custom API endpoints
- [ ] Handle enterprise-specific auth
- [ ] Add GHE.com data residency support

### 6.2 Copilot Integration
- [ ] Implement create_pull_request_with_copilot
- [ ] Add copilot assignment to issues
- [ ] Add copilot review requests
- [ ] Add copilot suggestions API

### 6.3 Advanced Workflows
- [ ] Add GitHub Projects support
- [ ] Add GitHub Packages operations
- [ ] Add webhook management
- [ ] Add deployment operations

## Step 7: Testing & Documentation

### 7.1 Unit Tests
- [ ] Test each tool handler
- [ ] Test pagination logic
- [ ] Test error handling
- [ ] Test operation resolver
- [ ] Test cache functionality

### 7.2 Integration Tests
- [ ] Create mock GitHub API server
- [ ] Test complete workflows
- [ ] Test rate limit handling
- [ ] Test auth flows
- [ ] Test GraphQL queries

### 7.3 Documentation
- [ ] Document each tool with examples
- [ ] Create migration guide
- [ ] Add troubleshooting guide
- [ ] Document configuration options
- [ ] Add performance tuning guide

## Step 8: Optimization

### 8.1 Performance
- [ ] Optimize GraphQL queries
- [ ] Implement batch operations
- [ ] Add connection pooling
- [ ] Profile and optimize hot paths
- [ ] Add metrics collection

### 8.2 Configuration
- [ ] Add dynamic toolset discovery
- [ ] Implement i18n support
- [ ] Add configuration overrides
- [ ] Support environment variables
- [ ] Add feature flags

## Integration Guide

### Putting It All Together

**Main Provider Integration Pattern:**
```go
// github_provider.go - Updated structure

type GitHubProvider struct {
    *providers.BaseProvider
    clients     *GitHubClients
    toolsets    *ToolsetManager
    cache       *GitHubCache
    rateLimiter *RateLimiter
}

func (p *GitHubProvider) Initialize(ctx context.Context, config ProviderConfig) error {
    // 1. Initialize clients
    p.clients = NewGitHubClients(config.Token, config.Host)
    
    // 2. Setup toolsets
    p.toolsets = NewToolsetManager(p.clients)
    p.toolsets.EnableToolsets(config.EnabledToolsets)
    
    // 3. Initialize infrastructure
    p.cache = NewGitHubCache(5 * time.Minute)
    p.rateLimiter = NewRateLimiter(config.RateLimits)
    
    // 4. Register all tools with dynamic system
    for _, toolset := range p.toolsets.GetEnabled() {
        for _, tool := range toolset.GetTools() {
            p.RegisterTool(tool)
        }
    }
    
    return nil
}

// ExecuteOperation - Main entry point
func (p *GitHubProvider) ExecuteOperation(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
    // 1. Check cache
    cacheKey := BuildCacheKey(operation, params)
    if cached, ok := p.cache.Get(cacheKey); ok {
        return cached, nil
    }
    
    // 2. Rate limit check
    if err := p.rateLimiter.Wait(ctx); err != nil {
        return nil, err
    }
    
    // 3. Find and execute tool
    tool, handler := p.toolsets.GetTool(operation)
    if tool == nil {
        return nil, fmt.Errorf("unknown operation: %s", operation)
    }
    
    // 4. Execute with error handling
    result, err := handler(ctx, params)
    if err != nil {
        return nil, NewGitHubAPIErrorResponse(ctx, err)
    }
    
    // 5. Cache successful results
    if IsReadOperation(operation) {
        p.cache.Set(cacheKey, result)
    }
    
    return result, nil
}
```

### Operation Mapping Integration

**Connecting to Existing Operation Resolver:**
```go
// Update operation_resolver.go to handle GitHub patterns

func (r *OperationResolver) ResolveGitHubOperation(action string, params map[string]interface{}) (*ResolvedOperation, error) {
    // Special handling for GitHub operation patterns
    
    // 1. Handle namespaced operations (issues/create, pulls/merge)
    if strings.Contains(action, "/") {
        return r.operationCache[action], nil
    }
    
    // 2. Handle GraphQL operations
    if strings.HasPrefix(action, "gql:") {
        return &ResolvedOperation{
            OperationID: action,
            Method: "GRAPHQL",
        }, nil
    }
    
    // 3. Use standard resolution
    return r.ResolveOperation(action, params)
}
```

## Completion Tracking

### Progress Summary
- **Total Tasks**: 180+
- **Completed**: 22
- **In Progress**: 0
- **Remaining**: 158+

### Priority Order
1. Foundation Setup (Step 1) - **Required First**
2. Core Tools (Step 2) - **High Priority**
3. MCP Protocol (Step 4) - **High Priority**
4. Infrastructure (Step 5) - **Medium Priority**
5. Advanced Features (Step 3) - **Medium Priority**
6. Enterprise (Step 6) - **Low Priority**
7. Testing (Step 7) - **Ongoing**
8. Optimization (Step 8) - **Final Phase**

### Critical Implementation Notes

**Must-Have Patterns from Official MCP:**
1. **Tool Handler Separation**: Each tool returns both definition and handler function
2. **GraphQL for Complex Queries**: Use GraphQL for list operations with filtering
3. **Minimal Response Objects**: Convert GitHub SDK objects to minimal formats
4. **Error Context**: Always wrap errors with GitHub-specific context
5. **Pagination by Default**: All list operations must support pagination
6. **Read-Only Hints**: Mark all read operations with ReadOnlyHint: true

**Common Pitfalls to Avoid:**
1. Don't use OpenAPI spec parsing - use GitHub SDK directly
2. Don't forget to handle GitHub's node IDs for GraphQL operations
3. Don't expose full GitHub objects - create minimal response types
4. Don't ignore rate limits - implement proper backoff
5. Don't hardcode page sizes - use configurable defaults

### Next Actions
1. [ ] Add GitHub client dependencies to go.mod
2. [ ] Create github_client.go with auth handling
3. [ ] Create github_toolsets.go with toolset definitions
4. [ ] Implement first toolset (repos) completely
5. [ ] Test with real GitHub API
6. [ ] Add pagination to all list operations
7. [ ] Implement error handling wrapper
8. [ ] Add caching for read operations

---

**Last Updated**: January 2025  
**Status**: Implementation Ready - All patterns documented from official GitHub MCP