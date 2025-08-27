# GitLab Provider - Comprehensive Implementation Plan

## Executive Summary
The current GitLab provider implementation covers only basic READ operations and is missing critical CRUD functionality. This plan outlines the complete implementation to achieve feature parity with GitLab API v4 while maintaining security through pass-through authentication and permission-based visibility.

## Current State Analysis

### What We Have:
- **Projects**: List, Get, Create (missing: Update, Delete, Fork, Star, Archive)
- **Issues**: List, Get, Create (missing: Update, Close, Reopen, Move, Delete)
- **Merge Requests**: List, Get, Create (missing: Update, Approve, Merge, Close, Rebase)
- **Pipelines**: List, Get, Trigger (missing: Cancel, Retry, Delete)
- **Jobs**: List only (missing: Get, Cancel, Retry, Play, Artifacts)
- **Repository**: Branches/Commits/Tags list only (missing: Create, Delete, Protect)
- **Groups**: List, Get (missing: Create, Update, Delete, Members)
- **Users**: Current user only (missing: List, Get, Create, Update, Block)

### Critical Missing Features:
1. **No UPDATE operations** for any resource
2. **No DELETE operations** for any resource
3. **No state management** (closing issues, merging MRs)
4. **No permission-based filtering** of operations
5. **No wikis, snippets, deployments support**
6. **No protected branches/tags management**
7. **No file operations** (read, write, delete files)
8. **No container registry operations**

## Implementation Strategy

### Phase 1: Core CRUD Completion (Priority: HIGH)

#### 1.1 Projects Operations
```go
// Add to gitlab_provider.go
"projects/update": {
    Method: "PUT",
    PathTemplate: "/projects/{id}",
    RequiredParams: []string{"id"},
    OptionalParams: []string{"name", "description", "visibility", "default_branch"}
}
"projects/delete": {
    Method: "DELETE", 
    PathTemplate: "/projects/{id}",
    RequiredParams: []string{"id"}
}
"projects/fork": {
    Method: "POST",
    PathTemplate: "/projects/{id}/fork",
    RequiredParams: []string{"id"},
    OptionalParams: []string{"namespace", "path", "name"}
}
"projects/star": {
    Method: "POST",
    PathTemplate: "/projects/{id}/star",
    RequiredParams: []string{"id"}
}
"projects/unstar": {
    Method: "POST",
    PathTemplate: "/projects/{id}/unstar",
    RequiredParams: []string{"id"}
}
"projects/archive": {
    Method: "POST",
    PathTemplate: "/projects/{id}/archive",
    RequiredParams: []string{"id"}
}
"projects/unarchive": {
    Method: "POST",
    PathTemplate: "/projects/{id}/unarchive",
    RequiredParams: []string{"id"}
}
```

#### 1.2 Issues Operations
```go
"issues/update": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/issues/{issue_iid}",
    RequiredParams: []string{"id", "issue_iid"},
    OptionalParams: []string{"title", "description", "state_event", "assignee_ids", "labels", "milestone_id"}
}
"issues/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/issues/{issue_iid}",
    RequiredParams: []string{"id", "issue_iid"}
}
"issues/close": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/issues/{issue_iid}",
    RequiredParams: []string{"id", "issue_iid"},
    BodyParams: map[string]interface{}{"state_event": "close"}
}
"issues/reopen": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/issues/{issue_iid}",
    RequiredParams: []string{"id", "issue_iid"},
    BodyParams: map[string]interface{}{"state_event": "reopen"}
}
```

#### 1.3 Merge Requests Operations
```go
"merge_requests/update": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}",
    RequiredParams: []string{"id", "merge_request_iid"},
    OptionalParams: []string{"title", "description", "state_event", "assignee_ids", "labels"}
}
"merge_requests/approve": {
    Method: "POST",
    PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}/approve",
    RequiredParams: []string{"id", "merge_request_iid"}
}
"merge_requests/unapprove": {
    Method: "POST",
    PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}/unapprove",
    RequiredParams: []string{"id", "merge_request_iid"}
}
"merge_requests/merge": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}/merge",
    RequiredParams: []string{"id", "merge_request_iid"},
    OptionalParams: []string{"merge_commit_message", "squash", "should_remove_source_branch"}
}
"merge_requests/close": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}",
    RequiredParams: []string{"id", "merge_request_iid"},
    BodyParams: map[string]interface{}{"state_event": "close"}
}
"merge_requests/rebase": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/merge_requests/{merge_request_iid}/rebase",
    RequiredParams: []string{"id", "merge_request_iid"}
}
```

### Phase 2: Repository Operations (Priority: HIGH)

#### 2.1 File Operations
```go
"files/get": {
    Method: "GET",
    PathTemplate: "/projects/{id}/repository/files/{file_path}",
    RequiredParams: []string{"id", "file_path", "ref"},
}
"files/create": {
    Method: "POST",
    PathTemplate: "/projects/{id}/repository/files/{file_path}",
    RequiredParams: []string{"id", "file_path", "branch", "content", "commit_message"},
    OptionalParams: []string{"author_email", "author_name"}
}
"files/update": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/repository/files/{file_path}",
    RequiredParams: []string{"id", "file_path", "branch", "content", "commit_message"},
    OptionalParams: []string{"last_commit_id", "author_email", "author_name"}
}
"files/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/repository/files/{file_path}",
    RequiredParams: []string{"id", "file_path", "branch", "commit_message"}
}
```

#### 2.2 Branch Operations
```go
"branches/create": {
    Method: "POST",
    PathTemplate: "/projects/{id}/repository/branches",
    RequiredParams: []string{"id", "branch", "ref"}
}
"branches/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/repository/branches/{branch}",
    RequiredParams: []string{"id", "branch"}
}
"branches/protect": {
    Method: "POST",
    PathTemplate: "/projects/{id}/protected_branches",
    RequiredParams: []string{"id", "name"},
    OptionalParams: []string{"push_access_level", "merge_access_level", "allow_force_push"}
}
"branches/unprotect": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/protected_branches/{name}",
    RequiredParams: []string{"id", "name"}
}
```

#### 2.3 Tag Operations
```go
"tags/create": {
    Method: "POST",
    PathTemplate: "/projects/{id}/repository/tags",
    RequiredParams: []string{"id", "tag_name", "ref"},
    OptionalParams: []string{"message", "release_description"}
}
"tags/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/repository/tags/{tag_name}",
    RequiredParams: []string{"id", "tag_name"}
}
```

### Phase 3: CI/CD Operations (Priority: MEDIUM)

#### 3.1 Pipeline Management
```go
"pipelines/cancel": {
    Method: "POST",
    PathTemplate: "/projects/{id}/pipelines/{pipeline_id}/cancel",
    RequiredParams: []string{"id", "pipeline_id"}
}
"pipelines/retry": {
    Method: "POST",
    PathTemplate: "/projects/{id}/pipelines/{pipeline_id}/retry",
    RequiredParams: []string{"id", "pipeline_id"}
}
"pipelines/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/pipelines/{pipeline_id}",
    RequiredParams: []string{"id", "pipeline_id"}
}
```

#### 3.2 Job Management
```go
"jobs/get": {
    Method: "GET",
    PathTemplate: "/projects/{id}/jobs/{job_id}",
    RequiredParams: []string{"id", "job_id"}
}
"jobs/cancel": {
    Method: "POST",
    PathTemplate: "/projects/{id}/jobs/{job_id}/cancel",
    RequiredParams: []string{"id", "job_id"}
}
"jobs/retry": {
    Method: "POST",
    PathTemplate: "/projects/{id}/jobs/{job_id}/retry",
    RequiredParams: []string{"id", "job_id"}
}
"jobs/play": {
    Method: "POST",
    PathTemplate: "/projects/{id}/jobs/{job_id}/play",
    RequiredParams: []string{"id", "job_id"}
}
"jobs/artifacts": {
    Method: "GET",
    PathTemplate: "/projects/{id}/jobs/{job_id}/artifacts",
    RequiredParams: []string{"id", "job_id"}
}
```

### Phase 4: Additional Resources (Priority: MEDIUM)

#### 4.1 Wiki Operations
```go
"wikis/list": {
    Method: "GET",
    PathTemplate: "/projects/{id}/wikis",
    RequiredParams: []string{"id"}
}
"wikis/get": {
    Method: "GET",
    PathTemplate: "/projects/{id}/wikis/{slug}",
    RequiredParams: []string{"id", "slug"}
}
"wikis/create": {
    Method: "POST",
    PathTemplate: "/projects/{id}/wikis",
    RequiredParams: []string{"id", "title", "content"},
    OptionalParams: []string{"format"}
}
"wikis/update": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/wikis/{slug}",
    RequiredParams: []string{"id", "slug"},
    OptionalParams: []string{"title", "content", "format"}
}
"wikis/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/wikis/{slug}",
    RequiredParams: []string{"id", "slug"}
}
```

#### 4.2 Snippet Operations
```go
"snippets/list": {
    Method: "GET",
    PathTemplate: "/projects/{id}/snippets",
    RequiredParams: []string{"id"}
}
"snippets/get": {
    Method: "GET",
    PathTemplate: "/projects/{id}/snippets/{snippet_id}",
    RequiredParams: []string{"id", "snippet_id"}
}
"snippets/create": {
    Method: "POST",
    PathTemplate: "/projects/{id}/snippets",
    RequiredParams: []string{"id", "title", "content", "visibility"},
    OptionalParams: []string{"description", "file_name"}
}
"snippets/update": {
    Method: "PUT",
    PathTemplate: "/projects/{id}/snippets/{snippet_id}",
    RequiredParams: []string{"id", "snippet_id"},
    OptionalParams: []string{"title", "content", "visibility", "description"}
}
"snippets/delete": {
    Method: "DELETE",
    PathTemplate: "/projects/{id}/snippets/{snippet_id}",
    RequiredParams: []string{"id", "snippet_id"}
}
```

### Phase 5: Permission-Based Filtering (Priority: CRITICAL)

#### 5.1 Enhanced Permission Discovery
```go
// Update permission_discoverer.go
func (d *GitLabPermissionDiscoverer) FilterOperationsByPermissions(
    operations []string,
    permissions map[string]interface{},
) []string {
    filtered := []string{}
    
    // Extract token scopes
    scopes := extractScopes(permissions)
    accessLevel := extractAccessLevel(permissions)
    
    for _, op := range operations {
        if d.isOperationAllowed(op, scopes, accessLevel) {
            filtered = append(filtered, op)
        }
    }
    
    return filtered
}

func (d *GitLabPermissionDiscoverer) isOperationAllowed(
    operation string,
    scopes []string,
    accessLevel int,
) bool {
    // Define operation requirements
    operationRequirements := map[string]struct{
        minAccessLevel int
        requiredScopes []string
    }{
        "projects/delete": {40, []string{"api"}}, // Maintainer
        "projects/update": {30, []string{"api", "write_repository"}}, // Developer
        "issues/create":   {20, []string{"api", "write_repository"}}, // Reporter
        "issues/update":   {20, []string{"api", "write_repository"}}, // Reporter
        "merge_requests/merge": {30, []string{"api"}}, // Developer
        // ... more mappings
    }
    
    req, exists := operationRequirements[operation]
    if !exists {
        // Default: allow read operations
        if strings.Contains(operation, "/list") || 
           strings.Contains(operation, "/get") {
            return hasReadScope(scopes)
        }
        return false
    }
    
    // Check access level
    if accessLevel < req.minAccessLevel {
        return false
    }
    
    // Check scopes
    return hasAnyScope(scopes, req.requiredScopes)
}
```

#### 5.2 Access Level Mapping
```go
const (
    NoAccess          = 0
    MinimalAccess     = 5
    GuestAccess       = 10
    ReporterAccess    = 20
    DeveloperAccess   = 30
    MaintainerAccess  = 40
    OwnerAccess       = 50
)

// Map GitLab access levels to allowed operations
var accessLevelOperations = map[int][]string{
    GuestAccess: {
        "projects/list", "projects/get",
        "issues/list", "issues/get",
        "merge_requests/list", "merge_requests/get",
    },
    ReporterAccess: {
        // Inherits Guest permissions plus:
        "issues/create", "issues/update",
        "pipelines/list", "jobs/list",
    },
    DeveloperAccess: {
        // Inherits Reporter permissions plus:
        "merge_requests/create", "merge_requests/update",
        "branches/create", "tags/create",
        "files/create", "files/update",
    },
    MaintainerAccess: {
        // Inherits Developer permissions plus:
        "merge_requests/merge", "merge_requests/approve",
        "branches/protect", "branches/delete",
        "pipelines/cancel", "pipelines/retry",
    },
    OwnerAccess: {
        // All operations including:
        "projects/delete", "projects/archive",
        "branches/unprotect", "tags/delete",
    },
}
```

### Phase 6: Pass-Through Authentication (Priority: CRITICAL)

#### 6.1 Maintain Existing Pass-Through Model
```go
// ValidateCredentials already properly handles pass-through
func (p *GitLabProvider) ValidateCredentials(ctx context.Context, creds map[string]string) error {
    // Check for GitLab-specific tokens
    token := ""
    authType := ""
    
    if pat := creds["personal_access_token"]; pat != "" {
        token = pat
        authType = "bearer"
    } else if apiKey := creds["api_key"]; apiKey != "" {
        token = apiKey  
        authType = "bearer"
    } else if t := creds["token"]; t != "" {
        token = t
        authType = "bearer"
    } else if jobToken := creds["job_token"]; jobToken != "" {
        token = jobToken
        authType = "job-token"
    }
    
    // Validate token format
    if token != "" && strings.HasPrefix(token, "glpat-") {
        if len(token) < 20 {
            return fmt.Errorf("invalid GitLab personal access token format")
        }
    }
    
    // Store in context for pass-through
    pctx := &providers.ProviderContext{
        Credentials: &providers.ProviderCredentials{
            Token: token,
        },
    }
    
    // Test with actual API call
    ctx = providers.WithContext(ctx, pctx)
    return p.testAuthentication(ctx)
}
```

#### 6.2 Ensure All Operations Use Pass-Through Context
```go
// ExecuteOperation must use credentials from context
func (p *GitLabProvider) ExecuteOperation(
    ctx context.Context,
    operation string,
    params map[string]interface{},
) (interface{}, error) {
    // Get credentials from context (pass-through)
    pctx, ok := providers.FromContext(ctx)
    if !ok || pctx.Credentials == nil {
        return nil, fmt.Errorf("no credentials in context")
    }
    
    // Map operation to endpoint
    mapping, exists := p.GetOperationMappings()[operation]
    if !exists {
        return nil, fmt.Errorf("operation %s not found", operation)
    }
    
    // Build request with pass-through credentials
    req := p.buildRequest(mapping, params)
    
    // Apply authentication from context
    p.applyAuthentication(ctx, req)
    
    // Execute with user's credentials
    return p.executeRequest(ctx, req)
}
```

### Phase 7: Testing Strategy

#### 7.1 Unit Tests for Each Operation
```go
func TestGitLabProvider_UpdateOperations(t *testing.T) {
    tests := []struct {
        name      string
        operation string
        params    map[string]interface{}
        expected  string
    }{
        {
            name:      "update project",
            operation: "projects/update",
            params:    map[string]interface{}{
                "id": "123",
                "description": "Updated description",
            },
            expected: "PUT /projects/123",
        },
        // ... more test cases
    }
}
```

#### 7.2 Integration Tests with Mock Server
```go
func TestGitLabProvider_PassThroughAuth(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify pass-through token is used
        authHeader := r.Header.Get("Authorization")
        assert.Equal(t, "Bearer user-token", authHeader)
        
        // Respond based on endpoint
        switch r.URL.Path {
        case "/user":
            w.Write([]byte(`{"id": 1, "username": "test"}`))
        }
    }))
    
    // Test with user's token
    ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
        Credentials: &providers.ProviderCredentials{
            Token: "user-token",
        },
    })
    
    provider := NewGitLabProvider(logger)
    provider.SetConfiguration(providers.ProviderConfig{
        BaseURL: server.URL,
    })
    
    err := provider.ValidateCredentials(ctx, map[string]string{
        "personal_access_token": "user-token",
    })
    assert.NoError(t, err)
}
```

### Phase 8: AI Definitions Update

#### 8.1 Update AI Definitions with New Operations
```go
// In ai_definitions.go
func GetGitLabAIDefinitions(enabledModules map[GitLabModule]bool) []providers.AIOptimizedToolDefinition {
    definitions = append(definitions, providers.AIOptimizedToolDefinition{
        Name:        "gitlab_projects_management",
        DisplayName: "GitLab Project Management",
        Category:    "Projects",
        Subcategory: "CRUD Operations",
        Description: "Complete project lifecycle management including create, update, delete, fork, star, and archive operations",
        UsageExamples: []providers.Example{
            {
                Scenario: "Update project settings",
                Input: map[string]interface{}{
                    "action": "update",
                    "id": "123",
                    "parameters": map[string]interface{}{
                        "description": "Updated description",
                        "visibility": "private",
                    },
                },
                Explanation: "Updates project configuration",
            },
            {
                Scenario: "Delete a project",
                Input: map[string]interface{}{
                    "action": "delete",
                    "id": "123",
                },
                Explanation: "Permanently deletes a project (requires owner access)",
            },
        },
        Capabilities: &providers.ToolCapabilities{
            Capabilities: []providers.Capability{
                {Action: "create", Resource: "projects"},
                {Action: "read", Resource: "projects"},
                {Action: "update", Resource: "projects"},
                {Action: "delete", Resource: "projects"},
                {Action: "fork", Resource: "projects"},
                {Action: "archive", Resource: "projects"},
            },
        },
    })
}
```

## Implementation Timeline

1. **Week 1**: Core CRUD operations (Projects, Issues, MRs)
2. **Week 2**: Repository operations (Files, Branches, Tags)
3. **Week 3**: CI/CD operations and Permission filtering
4. **Week 4**: Testing, documentation, and edge cases

## Security Considerations

1. **Token Validation**: Always validate token format before use
2. **Permission Checks**: Filter operations based on actual API permissions
3. **Audit Logging**: Log all destructive operations
4. **Rate Limiting**: Respect GitLab's rate limits (600 req/min)
5. **Sensitive Data**: Never log tokens or credentials
6. **HTTPS Only**: Ensure all API calls use HTTPS

## Success Criteria

1. ✅ All CRUD operations implemented for core resources
2. ✅ Pass-through authentication working for all operations
3. ✅ Permission-based filtering accurately reflects user's access
4. ✅ All operations tested with >80% coverage
5. ✅ AI definitions updated for all new operations
6. ✅ No regression in existing functionality
7. ✅ Documentation complete and accurate

## Risk Mitigation

1. **API Changes**: Use versioned endpoints, test against multiple GitLab versions
2. **Rate Limiting**: Implement exponential backoff, respect rate limit headers
3. **Token Expiry**: Handle token refresh for OAuth tokens
4. **Large Responses**: Implement pagination for list operations
5. **Network Failures**: Implement retry logic with circuit breaker pattern