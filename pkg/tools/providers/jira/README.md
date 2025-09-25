# Jira Provider Documentation

## Overview

The Jira provider enables comprehensive integration with Atlassian Jira for issue tracking, project management, and agile workflows. This provider supports both Jira Cloud and Jira Server/Data Center deployments through the Atlassian REST API v3.

## Table of Contents

- [Features](#features)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [Operations](#operations)
  - [Issue Operations](#issue-operations)
  - [Comment Operations](#comment-operations)
  - [Workflow Operations](#workflow-operations)
  - [Search Operations](#search-operations)
- [Advanced Features](#advanced-features)
- [Migration Guide](#migration-guide)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

## Features

- ✅ Full CRUD operations for Jira issues
- ✅ Comment management on issues
- ✅ Workflow transitions and status management
- ✅ Advanced JQL search capabilities
- ✅ Built-in security features (PII detection, credential encryption)
- ✅ Response caching for improved performance
- ✅ Comprehensive observability and metrics
- ✅ Support for both Cloud and Server/Data Center deployments
- ✅ Context-aware authentication with credential passthrough
- ✅ Rate limiting and retry logic
- ✅ Health monitoring and status checks

## Configuration

### Basic Configuration

```go
provider := NewJiraProvider(logger, "tenant-id")

// Configure domain
provider.domain = "https://your-company.atlassian.net"
```

### Environment Variables

The provider supports configuration through environment variables:

```bash
# Required for authentication
export JIRA_DOMAIN="https://your-company.atlassian.net"
export JIRA_EMAIL="your-email@company.com"
export JIRA_API_TOKEN="your-api-token"

# Optional configuration
export JIRA_CACHE_TTL="5m"              # Cache TTL (default: 5 minutes)
export JIRA_ENABLE_DEBUG="true"         # Enable debug logging
export JIRA_READ_ONLY="false"           # Enable read-only mode
export JIRA_PROJECT_FILTER="PROJ1,PROJ2" # Filter to specific projects
```

### Programmatic Configuration

```go
// Via context (recommended for passthrough authentication)
ctx := context.Background()
ctx = context.WithValue(ctx, "auth_type", "basic")
ctx = context.WithValue(ctx, "email", "user@example.com")
ctx = context.WithValue(ctx, "api_token", "encrypted-token")
ctx = context.WithValue(ctx, "tenant_id", "tenant-123")

// Optional configurations
ctx = context.WithValue(ctx, "read_only", true)
ctx = context.WithValue(ctx, "project_filter", "PROJ1,PROJ2")
ctx = context.WithValue(ctx, "debug_mode", true)
```

## Authentication

### Basic Authentication (API Token)

The provider uses basic authentication with email and API token:

```go
ctx := context.WithValue(ctx, "auth_type", "basic")
ctx = context.WithValue(ctx, "email", "user@example.com")
ctx = context.WithValue(ctx, "api_token", "your-api-token")
```

### OAuth 2.0

For OAuth authentication (typically for apps):

```go
ctx := context.WithValue(ctx, "auth_type", "oauth")
ctx = context.WithValue(ctx, "access_token", "oauth-access-token")
```

### Personal Access Token (PAT)

For Jira Server/Data Center with PAT:

```go
ctx := context.WithValue(ctx, "auth_type", "pat")
ctx = context.WithValue(ctx, "pat_token", "personal-access-token")
```

### Encrypted Credentials

The provider supports encrypted credentials for secure storage:

```go
// Credentials are automatically decrypted when prefixed with "encrypted:"
ctx = context.WithValue(ctx, "api_token", "encrypted:base64-encrypted-token")
```

## Operations

### Issue Operations

#### Get Issue

Retrieve a single issue by its key or ID.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "expand": "changelog,comments", // Optional expansions
    "fields": "summary,status,assignee", // Optional field filtering
}

result, err := provider.ExecuteOperation(ctx, "issues/get", params)
```

**Response:**
```json
{
  "id": "10001",
  "key": "PROJ-123",
  "fields": {
    "summary": "Issue summary",
    "status": {
      "name": "In Progress"
    },
    "assignee": {
      "displayName": "John Doe",
      "emailAddress": "john@example.com"
    }
  }
}
```

#### Create Issue

Create a new issue in Jira.

```go
params := map[string]interface{}{
    "project": "PROJ",
    "issuetype": "Bug",
    "summary": "Critical bug in production",
    "description": "Detailed description of the issue",
    "priority": "High",
    "labels": []string{"bug", "production"},
    "components": []string{"Backend"},
    "assignee": "john.doe",
    "reporter": "jane.smith",
    "customfield_10001": "Custom value", // Custom fields
}

result, err := provider.ExecuteOperation(ctx, "issues/create", params)
```

**Response:**
```json
{
  "id": "10002",
  "key": "PROJ-124",
  "self": "https://your-company.atlassian.net/rest/api/3/issue/10002"
}
```

#### Update Issue

Update an existing issue.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "summary": "Updated summary",
    "description": "Updated description",
    "priority": "Critical",
    "labels": []string{"urgent", "bug"},
    "fixVersions": []string{"1.0.0"},
}

result, err := provider.ExecuteOperation(ctx, "issues/update", params)
```

#### Delete Issue

Delete an issue (requires appropriate permissions).

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "deleteSubtasks": true, // Optional: delete subtasks
}

result, err := provider.ExecuteOperation(ctx, "issues/delete", params)
```

### Comment Operations

#### Get Comments

Retrieve all comments for an issue.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "startAt": 0,      // Pagination
    "maxResults": 50,
    "orderBy": "created", // created or -created
    "expand": "properties,renderedBody",
}

result, err := provider.ExecuteOperation(ctx, "comments/get", params)
```

#### Add Comment

Add a comment to an issue.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "body": "This is a comment with *formatted* text",
    "visibility": map[string]interface{}{
        "type": "group",
        "value": "developers",
    },
    "properties": []map[string]interface{}{
        {
            "key": "comment-type",
            "value": "internal-note",
        },
    },
}

result, err := provider.ExecuteOperation(ctx, "comments/add", params)
```

#### Update Comment

Update an existing comment.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "commentId": "10001",
    "body": "Updated comment text",
}

result, err := provider.ExecuteOperation(ctx, "comments/update", params)
```

#### Delete Comment

Delete a comment from an issue.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "commentId": "10001",
}

result, err := provider.ExecuteOperation(ctx, "comments/delete", params)
```

### Workflow Operations

#### Get Transitions

Get available transitions for an issue.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "expand": "transitions.fields",
    "transitionId": "", // Optional: filter by specific transition
}

result, err := provider.ExecuteOperation(ctx, "workflow/transitions", params)
```

**Response:**
```json
{
  "transitions": [
    {
      "id": "11",
      "name": "In Progress",
      "to": {
        "name": "In Progress",
        "id": "3"
      }
    },
    {
      "id": "21",
      "name": "Done",
      "to": {
        "name": "Done",
        "id": "10001"
      }
    }
  ]
}
```

#### Transition Issue

Move an issue to a different status.

```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "transition": "21", // Transition ID or name
    "comment": "Moving to Done status",
    "fields": map[string]interface{}{
        "resolution": map[string]interface{}{
            "name": "Fixed",
        },
    },
}

result, err := provider.ExecuteOperation(ctx, "issues/transition", params)
```

### Search Operations

#### Search Issues with JQL

Search for issues using Jira Query Language (JQL).

```go
params := map[string]interface{}{
    "jql": "project = PROJ AND status = 'In Progress' AND assignee = currentUser()",
    "startAt": 0,
    "maxResults": 50,
    "fields": []string{"summary", "status", "assignee", "created"},
    "expand": []string{"changelog", "operations"},
    "validateQuery": true,
}

result, err := provider.ExecuteOperation(ctx, "issues/search", params)
```

**Response:**
```json
{
  "expand": "schema,names",
  "startAt": 0,
  "maxResults": 50,
  "total": 23,
  "issues": [
    {
      "id": "10001",
      "key": "PROJ-123",
      "fields": {
        "summary": "Issue summary",
        "status": {
          "name": "In Progress"
        },
        "assignee": {
          "displayName": "John Doe"
        },
        "created": "2024-01-15T10:30:00.000+0000"
      }
    }
  ]
}
```

#### Advanced JQL Examples

```go
// Find bugs assigned to me in current sprint
jql := "type = Bug AND assignee = currentUser() AND sprint in openSprints()"

// Find high priority issues created in last week
jql := "priority in (High, Critical) AND created >= -1w"

// Find issues with specific labels in multiple projects
jql := "project in (PROJ1, PROJ2) AND labels in (security, urgent)"

// Find unresolved issues updated by specific user
jql := "resolution = Unresolved AND updatedBy = 'john.doe' AND updated >= -30d"
```

## Advanced Features

### Caching

The provider includes intelligent caching for improved performance:

```go
// Caching is automatic for GET operations
// Cache TTL can be configured
ctx = context.WithValue(ctx, "cache_ttl", "10m")

// Force cache refresh
ctx = context.WithValue(ctx, "force_refresh", true)

// Disable caching for specific request
ctx = context.WithValue(ctx, "no_cache", true)
```

### Batch Operations

Process multiple operations efficiently:

```go
// Batch create issues
issues := []map[string]interface{}{
    {
        "project": "PROJ",
        "summary": "Issue 1",
        "issuetype": "Task",
    },
    {
        "project": "PROJ",
        "summary": "Issue 2",
        "issuetype": "Bug",
    },
}

for _, issue := range issues {
    result, err := provider.ExecuteOperation(ctx, "issues/create", issue)
    // Handle result
}
```

### Error Handling

The provider includes comprehensive error categorization:

```go
result, err := provider.ExecuteOperation(ctx, "issues/get", params)
if err != nil {
    if jiraErr, ok := err.(*JiraError); ok {
        switch jiraErr.Type {
        case ErrorTypeAuthentication:
            // Handle authentication error
        case ErrorTypeRateLimit:
            // Handle rate limiting
            time.Sleep(jiraErr.RetryAfter)
        case ErrorTypeNotFound:
            // Handle not found
        default:
            // Handle other errors
        }
    }
}
```

### Health Monitoring

Monitor provider health and status:

```go
// Perform health check
err := provider.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Get detailed health status
status := provider.GetHealthStatus()
log.Printf("Provider healthy: %v", status.Healthy)
log.Printf("Last checked: %v", status.LastChecked)
log.Printf("Response time: %v", status.ResponseTime)
```

### Observability

Access metrics and debug information:

```go
// Enable debug mode
provider.observabilityMgr.config.DebugMode = true

// Get metrics
metrics := provider.observabilityMgr.GetObservabilityMetrics()
log.Printf("Metrics: %+v", metrics)

// Start operation with tracking
ctx, finish := provider.observabilityMgr.StartOperation(ctx, "custom_operation")
defer finish(nil) // Pass error if operation fails
```

## Migration Guide

### Migrating from Legacy Jira Provider

If you're migrating from an older Jira provider implementation, follow these steps:

#### 1. Update Authentication

**Old method:**
```go
// Legacy authentication
client := jira.NewClient("username", "password", "https://jira.example.com")
```

**New method:**
```go
// New authentication
provider := NewJiraProvider(logger, "tenant-id")
ctx := context.WithValue(ctx, "email", "user@example.com")
ctx := context.WithValue(ctx, "api_token", "api-token")
```

#### 2. Update Operation Calls

**Old method:**
```go
// Legacy operation
issue, err := client.GetIssue("PROJ-123")
```

**New method:**
```go
// New operation
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
}
result, err := provider.ExecuteOperation(ctx, "issues/get", params)
```

#### 3. Update Error Handling

**Old method:**
```go
// Legacy error handling
if err != nil {
    if err.StatusCode == 401 {
        // Handle auth error
    }
}
```

**New method:**
```go
// New error handling
if err != nil {
    if jiraErr, ok := err.(*JiraError); ok {
        if jiraErr.Type == ErrorTypeAuthentication {
            // Handle auth error
        }
    }
}
```

#### 4. Update Configuration

**Old configuration file:**
```yaml
jira:
  url: https://jira.example.com
  username: user
  password: pass
```

**New configuration:**
```yaml
jira:
  domain: https://your-company.atlassian.net
  auth:
    type: basic
    email: user@example.com
    api_token: ${JIRA_API_TOKEN}
  cache:
    enabled: true
    ttl: 5m
  observability:
    debug: false
    metrics: true
```

### Breaking Changes

1. **Authentication**: Username/password authentication is deprecated. Use API tokens.
2. **API Version**: Now uses Jira REST API v3 (previously v2)
3. **Operation Names**: Operations now use path-based naming (e.g., `issues/get` instead of `GetIssue`)
4. **Response Format**: Responses now return `ToolResult` with structured data
5. **Context Required**: All operations now require a context parameter

## Troubleshooting

### Common Issues and Solutions

#### Authentication Failures

**Problem:** Getting 401 Unauthorized errors
```
Error: authentication: Invalid API token or email
```

**Solution:**
1. Verify your API token is correct and not expired
2. Ensure email address matches the token owner
3. For Jira Cloud, use API tokens, not passwords
4. Check if token needs to be base64 encoded

```go
// Correct format for API token authentication
ctx = context.WithValue(ctx, "email", "user@example.com")
ctx = context.WithValue(ctx, "api_token", "ATATT3xFfGF0...") // Raw token, not base64
```

#### Rate Limiting

**Problem:** Getting 429 Too Many Requests errors
```
Error: rate_limit: API rate limit exceeded
```

**Solution:**
1. Implement exponential backoff
2. Use caching to reduce API calls
3. Batch operations where possible

```go
// Handle rate limiting with retry
result, err := provider.ExecuteOperation(ctx, "issues/search", params)
if err != nil {
    if jiraErr, ok := err.(*JiraError); ok && jiraErr.Type == ErrorTypeRateLimit {
        time.Sleep(jiraErr.RetryAfter)
        // Retry operation
        result, err = provider.ExecuteOperation(ctx, "issues/search", params)
    }
}
```

#### Permission Errors

**Problem:** Getting 403 Forbidden errors
```
Error: authorization: You do not have permission to perform this operation
```

**Solution:**
1. Verify user has required project permissions
2. Check if operation requires specific Jira permissions
3. Ensure API token has necessary scopes

```go
// Check if operating in read-only mode
if provider.IsReadOnlyMode(ctx) {
    // Skip write operations
    return nil
}
```

#### Network and Connectivity Issues

**Problem:** Connection timeouts or network errors
```
Error: network: connection timeout after 30s
```

**Solution:**
1. Check network connectivity to Jira instance
2. Verify firewall rules allow HTTPS traffic
3. Increase timeout for slow networks

```go
// Configure custom timeout
provider.httpClient.Timeout = 60 * time.Second
```

#### Invalid JQL Queries

**Problem:** JQL validation errors
```
Error: validation: The JQL query is invalid
```

**Solution:**
1. Validate JQL syntax before execution
2. Use field names, not display names
3. Escape special characters properly

```go
// Validate JQL before search
params := map[string]interface{}{
    "jql": "project = PROJ AND status = 'In Progress'", // Use quotes for multi-word values
    "validateQuery": true,
}
```

#### Cache Issues

**Problem:** Stale data being returned
```
Warning: Cache may contain outdated information
```

**Solution:**
1. Force cache refresh for critical operations
2. Reduce cache TTL for frequently changing data
3. Clear cache after write operations

```go
// Force fresh data
ctx = context.WithValue(ctx, "force_refresh", true)
result, err := provider.ExecuteOperation(ctx, "issues/get", params)
```

### Debug Mode

Enable debug mode for detailed logging:

```go
// Enable debug logging
ctx = context.WithValue(ctx, "debug_mode", true)

// Or via environment variable
os.Setenv("JIRA_ENABLE_DEBUG", "true")
```

Debug output includes:
- Full HTTP requests and responses
- Authentication headers (sanitized)
- Cache hit/miss information
- Performance metrics
- Error stack traces

### Performance Optimization

#### 1. Use Field Filtering

Only request fields you need:
```go
params := map[string]interface{}{
    "issueIdOrKey": "PROJ-123",
    "fields": "summary,status,assignee", // Only needed fields
}
```

#### 2. Enable Caching

Caching is enabled by default for GET operations:
```go
// Configure cache TTL
ctx = context.WithValue(ctx, "cache_ttl", "10m")
```

#### 3. Use Pagination

For large result sets, use pagination:
```go
params := map[string]interface{}{
    "jql": "project = PROJ",
    "startAt": 0,
    "maxResults": 50, // Limit results per request
}
```

#### 4. Batch Operations

Group related operations:
```go
// Instead of multiple individual requests
for _, key := range issueKeys {
    // Individual request for each issue
}

// Use JQL to fetch multiple issues
jql := fmt.Sprintf("key in (%s)", strings.Join(issueKeys, ","))
params := map[string]interface{}{
    "jql": jql,
}
```

## Examples

### Complete Issue Lifecycle Example

```go
package main

import (
    "context"
    "log"
    "github.com/developer-mesh/developer-mesh/pkg/tools/providers/jira"
)

func main() {
    // Initialize provider
    logger := &observability.NoopLogger{}
    provider := jira.NewJiraProvider(logger, "tenant-123")
    provider.domain = "https://your-company.atlassian.net"

    // Setup authentication
    ctx := context.Background()
    ctx = context.WithValue(ctx, "email", "user@example.com")
    ctx = context.WithValue(ctx, "api_token", "your-api-token")

    // 1. Create an issue
    createParams := map[string]interface{}{
        "project": "PROJ",
        "issuetype": "Task",
        "summary": "Implement new feature",
        "description": "As a user, I want to...",
        "priority": "Medium",
    }

    result, err := provider.ExecuteOperation(ctx, "issues/create", createParams)
    if err != nil {
        log.Fatalf("Failed to create issue: %v", err)
    }

    issueKey := result.(*jira.ToolResult).Data.(map[string]interface{})["key"].(string)
    log.Printf("Created issue: %s", issueKey)

    // 2. Add a comment
    commentParams := map[string]interface{}{
        "issueIdOrKey": issueKey,
        "body": "Starting work on this feature",
    }

    _, err = provider.ExecuteOperation(ctx, "comments/add", commentParams)
    if err != nil {
        log.Printf("Failed to add comment: %v", err)
    }

    // 3. Transition to In Progress
    transitionParams := map[string]interface{}{
        "issueIdOrKey": issueKey,
        "transition": "In Progress",
    }

    _, err = provider.ExecuteOperation(ctx, "issues/transition", transitionParams)
    if err != nil {
        log.Printf("Failed to transition issue: %v", err)
    }

    // 4. Update the issue
    updateParams := map[string]interface{}{
        "issueIdOrKey": issueKey,
        "summary": "Implement new feature - In Progress",
        "labels": []string{"in-progress", "feature"},
    }

    _, err = provider.ExecuteOperation(ctx, "issues/update", updateParams)
    if err != nil {
        log.Printf("Failed to update issue: %v", err)
    }

    log.Println("Issue lifecycle completed successfully")
}
```

### Search and Bulk Operations Example

```go
// Search for all unresolved bugs
searchParams := map[string]interface{}{
    "jql": "project = PROJ AND type = Bug AND resolution = Unresolved",
    "fields": []string{"key", "summary", "priority", "assignee"},
}

result, err := provider.ExecuteOperation(ctx, "issues/search", searchParams)
if err != nil {
    log.Fatalf("Search failed: %v", err)
}

searchResult := result.(*jira.ToolResult).Data.(map[string]interface{})
issues := searchResult["issues"].([]interface{})

// Process each bug
for _, issue := range issues {
    issueData := issue.(map[string]interface{})
    fields := issueData["fields"].(map[string]interface{})

    // Auto-assign high priority bugs
    if priority := fields["priority"].(map[string]interface{});
       priority["name"] == "High" && fields["assignee"] == nil {

        assignParams := map[string]interface{}{
            "issueIdOrKey": issueData["key"].(string),
            "assignee": "team-lead",
        }

        provider.ExecuteOperation(ctx, "issues/update", assignParams)
    }
}
```

### Error Handling Example

```go
// Comprehensive error handling
result, err := provider.ExecuteOperation(ctx, "issues/get", params)
if err != nil {
    switch e := err.(type) {
    case *jira.JiraError:
        switch e.Type {
        case jira.ErrorTypeAuthentication:
            log.Fatal("Authentication failed. Please check your credentials.")
        case jira.ErrorTypeAuthorization:
            log.Printf("Permission denied for operation: %s", e.Operation)
        case jira.ErrorTypeNotFound:
            log.Printf("Issue not found: %v", params["issueIdOrKey"])
        case jira.ErrorTypeRateLimit:
            log.Printf("Rate limited. Retry after %v", e.RetryAfter)
            time.Sleep(e.RetryAfter)
            // Retry operation
        case jira.ErrorTypeValidation:
            log.Printf("Validation error: %s", e.Message)
        default:
            log.Printf("Jira error: %v", e)
        }
    default:
        log.Printf("Unexpected error: %v", err)
    }
    return
}
```

## API Reference

For complete API documentation, refer to:
- [Atlassian Jira Cloud REST API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [Jira Server REST API](https://docs.atlassian.com/software/jira/docs/api/REST/latest/)

## Support

For issues, questions, or contributions:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review existing issues in the repository
3. Create a new issue with detailed information
4. Include debug logs when reporting problems

## License

This provider is part of the Developer Mesh platform and follows the same licensing terms.