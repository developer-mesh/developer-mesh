# Jira and Confluence Tools Overhaul - Engineering Plan

## Overview
This plan outlines the engineering work needed to overhaul our Jira and Confluence Go providers to implement core API operations while following our established patterns from GitHub and Harness providers.

## Current State
- **Jira Provider**: 1001 lines of Go code with basic operations
- **Confluence Provider**: 455 lines of Go code with limited functionality
- **Authentication**: API token only
- **API Version**: Confluence v2 base URL, Jira version unspecified
- **Pattern**: Basic provider structure without handler pattern

## Target State (Following Our Established Patterns)
- **Core Confluence operations** with handler-based architecture (pages, spaces, content)
- **Core Jira operations** with toolset grouping (issues, workflows, search)
- **Passthrough Authentication Only**: Following GitHub/Harness pattern (no config-based auth)
- **Modern API**: Confluence v2 REST API with v1 for search/CQL, Jira v3 REST API
- **Enhanced features**: Pagination, per-request credentials
- **Architecture**: Handler pattern, AI definitions, module constants (following GitHub/Harness)

---

## Architecture Patterns to Follow

Based on our GitHub and Harness implementations:

1. **Handler Pattern**: Each operation gets its own handler struct implementing `ToolHandler` interface
2. **File Organization**:
   - Handler files group related operations (following GitHub pattern)
   - `ai_definitions.go` - AI-optimized tool definitions
3. **Module Constants**: Define constants for Jira/Confluence modules and features
4. **Toolsets**: Group related tools with enable/disable capability
5. **Permission Filtering**:
   - Toolset-based enabling/disabling (GitHub pattern)
   - Tenant-specific configurations via ProviderContext
   - Resource-level filters (projects/spaces)
6. **Client Management**: Store REST clients in context (no GraphQL needed for Atlassian APIs)
7. **Parameter Aliasing**: Leverage BaseProvider's built-in aliasing
8. **Error Enhancement**: Use provider-specific error enhancement (like Harness)

## Engineering Stories

### Epic 1: Core Infrastructure Updates
**Goal**: Establish foundation following GitHub/Harness patterns

#### Story 1.1: Refactor to Handler Architecture ✅ **COMPLETED**
- Create handler interface matching GitHub's `ToolHandler` ✅
- Implement base handler struct with common functionality ✅
- Migrate existing operations to handler pattern ✅
- Set up toolset registry for grouping related handlers ✅
- **Acceptance Criteria**:
  - All operations use handler pattern ✅
  - Handlers implement consistent interface ✅
  - Toolset grouping works ✅
- **Completion Date**: 2025-01-24
- **Implementation Details**:
  - Created handler files for both Jira (handlers_issues.go, handlers_search.go) and Confluence (handlers_pages.go, handlers_search.go, handlers_labels.go)
  - Implemented ToolHandler interface with Execute() and GetDefinition() methods
  - Added toolset registry with enable/disable capability
  - All existing tests pass without modification

#### Story 1.2: Implement Passthrough Authentication (Following GitHub/Harness Pattern) ✅ **COMPLETED**
- **Remove all hardcoded authentication** - no config-based tokens ✅
- Implement passthrough-only authentication like GitHub provider: ✅
  - Primary: Check ProviderContext for credentials (Token or Custom\["token"\]) ✅
  - Secondary: Check `__passthrough_auth` parameter with encrypted_token ✅
  - Use EncryptionService.DecryptCredential for encrypted tokens ✅
  - Support plain token in `__passthrough_auth` for development ✅
  - Direct token parameter fallback for compatibility ✅
- Create `extractAuthToken()` method following GitHub's pattern ✅
- Support both API tokens and PATs through same passthrough mechanism ✅
- **Acceptance Criteria**:
  - NO hardcoded or config-based authentication ✅
  - All auth comes from request context/parameters ✅
  - Encrypted token decryption works correctly ✅
  - Authentication method matches GitHub/Harness providers exactly ✅
  - Each request can have different credentials ✅
- **Completion Date**: 2025-01-24
- **Implementation Details**:
  - Created extractAuthToken() method with 5-priority authentication levels
  - Supports ProviderContext, __passthrough_auth, and direct params
  - Handles both email:api_token and username:password formats
  - Checks Metadata in addition to Custom fields
  - Returns specific error messages for validation issues
  - Comprehensive test coverage for all authentication scenarios
  - Updated extractTenantID to check passthrough auth

#### Story 1.3: Upgrade to Modern API Clients ✅ **COMPLETED**
- Migrate Confluence operations to v2 REST API where available ✅
- Implement v1 fallback for operations not yet in v2 (including search) ✅
- **Update Jira client to use v3 REST API** (current version, not v2) ✅
- Implement cursor-based pagination for Confluence v2 APIs ✅
- Maintain offset/limit pagination for v1 APIs and Jira v3 ✅
- **Acceptance Criteria**:
  - Confluence uses v2 where available, v1 fallback for missing operations ✅
  - Jira uses v3 REST API (latest version) ✅
  - Pagination works for large result sets in both APIs ✅
- **Completion Date**: 2025-01-24
- **Implementation Details**:
  - Centralized URL building methods in Confluence provider (buildURL for v2, buildV1URL for v1)
  - Verified Jira already uses v3 REST API throughout
  - Updated default Confluence BaseURL from v1 to v2 format
  - Documented API version strategy in code comments
  - All existing tests updated and passing

#### Story 1.4: Add Configuration Management with Permission Filtering ✅ **COMPLETED**
- Implement toolset enable/disable pattern (following GitHub provider): ✅
  - `EnableToolset(name string)` / `DisableToolset(name string)` methods ✅
  - `enabledToolsets` map to track active toolsets ✅
  - Default toolsets enabled on initialization ✅
- Add tenant-specific tool filtering: ✅
  - Use ProviderContext.TenantID for per-tenant configurations ✅
  - Support ENABLED_TOOLS environment variable for selective enabling ✅
- Add resource filters: ✅
  - JIRA_PROJECTS_FILTER to limit accessible projects ✅
  - CONFLUENCE_SPACES_FILTER to limit accessible spaces ✅
- Implement READ_ONLY mode to prevent write operations ✅
- **Acceptance Criteria**:
  - Only enabled toolsets are exposed to users ✅
  - Tenant-specific configurations respected ✅
  - Filters correctly limit data access ✅
  - Read-only mode prevents all write operations ✅
- **Completion Date**: 2025-01-24
- **Implementation Details**:
  - Added comprehensive configuration management to both Jira and Confluence providers
  - Implemented toolset enable/disable with mutex-protected thread safety
  - Added ConfigureFromContext() to apply settings from ProviderContext
  - Implemented project/space filtering with support for wildcards
  - Added read-only mode enforcement in ExecuteOperation()
  - Created comprehensive test coverage for all configuration features
  - All tests passing for both providers

---

### Epic 2: Confluence Provider Enhancement
**Goal**: Implement core Confluence v2 API operations using handler pattern

#### Story 2.1: Implement Page Handlers
- Create handler file for page operations (following GitHub pattern)
- `GetPageHandler` - GET /pages/{id} (v2)
- `ListPagesHandler` - GET /pages (v2)
- `DeletePageHandler` - DELETE /pages/{id} (v2)
- Create toolset to group page operations
- **Acceptance Criteria**:
  - All handlers implement ToolHandler interface
  - Operations use Confluence v2 REST API
  - Proper error handling and pagination

#### Story 2.2: Implement Search Handlers (v1 API required)
- Create handler file for search operations
- `SearchContentHandler` - Uses v1 API /content/search with CQL
- Implement CQL (Confluence Query Language) support via v1
- Create toolset to group search operations
- **Acceptance Criteria**:
  - Search uses v1 API (v2 doesn't support CQL)
  - CQL queries properly validated
  - Results properly paginated

#### Story 2.3: Implement Label Handlers
- Create handler file for label operations
- `GetPageLabelsHandler` - GET /pages/{id}/labels (v2)
- Create toolset to group label operations
- **Acceptance Criteria**:
  - Label operations use v2 API
  - Proper permission checking

#### Story 2.4: Implement Additional v1 Operations (if needed)
- Identify operations not available in v2
- Create handlers for critical v1-only operations
- Document which operations require v1
- **Acceptance Criteria**:
  - Clear documentation of v1 vs v2 usage
  - Smooth fallback between API versions

#### Story 2.5: Create Confluence AI Definitions (`confluence_ai_definitions.go`)
- Implement `GetAIOptimizedDefinitions()` following Harness pattern
- Group operations by category (Pages, Spaces, Search, Content)
- Add detailed usage examples for each operation
- Include common error scenarios and handling
- **Acceptance Criteria**:
  - AI definitions match GitHub/Harness quality
  - Examples cover common use cases
  - Categories align with toolsets

---

### Epic 3: Jira Provider Enhancement
**Goal**: Implement core Jira v3 API operations using handler pattern

#### Story 3.1: Implement Issue Handlers
- Create handler file for issue operations (following GitHub pattern)
- `GetIssueHandler` - Get issue details (v3)
- `CreateIssueHandler` - Create issues (v3)
- `UpdateIssueHandler` - Update issue fields (v3)
- `DeleteIssueHandler` - Delete issues (v3)
- Create toolset to group issue operations
- **Acceptance Criteria**:
  - All handlers implement ToolHandler interface
  - Operations use Jira v3 REST API
  - Custom fields properly supported
  - JIRA_PROJECTS_FILTER applied in handlers

#### Story 3.2: Implement Search Handlers
- Create handler file for search operations
- `SearchIssuesHandler` - JQL search endpoint (v3)
- Implement JQL validation
- Create toolset to group search operations
- **Acceptance Criteria**:
  - Search uses Jira v3 API
  - JQL properly validated
  - Results filtered by JIRA_PROJECTS_FILTER
  - Pagination works correctly

#### Story 3.3: Implement Comment Handlers
- Create handler file for comment operations
- `AddCommentHandler` - Add comments (v3)
- `GetCommentsHandler` - Get comments (v3)
- Create toolset to group comment operations
- **Acceptance Criteria**:
  - Comments use v3 API
  - Rich text formatting supported

#### Story 3.4: Implement Workflow Handlers
- Create handler file for workflow operations
- `GetTransitionsHandler` - Get available transitions (v3)
- `TransitionIssueHandler` - Execute transitions (v3)
- Create toolset to group workflow operations
- **Acceptance Criteria**:
  - Transitions properly validated
  - Permission-aware operations

#### Story 3.5: Create Jira AI Definitions (`jira_ai_definitions.go`)
- Implement `GetAIOptimizedDefinitions()` following Harness pattern
- Group operations by category (Issues, Search, Workflow, Activity)
- Add detailed usage examples for each operation
- Include JQL query examples and templates
- **Acceptance Criteria**:
  - AI definitions match GitHub/Harness quality
  - Examples cover common use cases
  - JQL examples provided
  - Categories align with toolsets

---

### Epic 4: Shared Features and Utilities
**Goal**: Implement cross-cutting concerns

#### Story 4.1: Implement Security Features
- Add PII filtering for sensitive data
- Implement SSL verification controls
- Add request/response sanitization
- Create security audit logging
- **Acceptance Criteria**:
  - PII properly filtered from logs
  - SSL controls configurable

#### Story 4.2: Add Observability
- Implement comprehensive error handling
- Add operation metrics and timing
- Create debug mode with verbose logging
- Add health check endpoints
- **Acceptance Criteria**:
  - All errors properly categorized
  - Metrics exported to observability system

#### Story 4.3: Implement Caching Layer
- Add response caching for read operations
- Implement cache invalidation strategy
- Add ETags support
- **Acceptance Criteria**:
  - Cache improves performance
  - Invalidation prevents stale data

---

### Epic 5: Testing and Documentation
**Goal**: Ensure quality and usability

#### Story 5.1: Unit Testing
- Write unit tests for all new operations
- Mock Atlassian API responses
- Test error scenarios
- Test pagination edge cases
- **Acceptance Criteria**:
  - 80% code coverage minimum
  - All operations have tests

#### Story 5.2: Integration Testing
- Create integration test suite
- Test against Atlassian Cloud sandbox
- Test Server/Data Center compatibility
- Test multi-user scenarios
- **Acceptance Criteria**:
  - Integration tests pass
  - Compatibility verified

#### Story 5.3: Documentation
- Document all operations with examples
- Create migration guide from current providers
- Document configuration options
- Add troubleshooting guide
- **Acceptance Criteria**:
  - All operations documented
  - Examples work correctly

---

## Implementation Order

### Phase 1: Foundation (2 weeks)
1. Story 1.1: Handler Architecture Refactor
2. Story 1.2: Passthrough Authentication Implementation
3. Story 1.3: Modern API Clients
4. Story 1.4: Configuration Management

### Phase 2: Core Operations (3 weeks)
1. Story 2.1: Confluence Page Operations
2. Story 3.1: Jira Issue Operations
3. Story 4.1: Security Features

### Phase 3: Search and Query (2 weeks)
1. Story 2.2: Confluence Search
2. Story 3.2: Jira Search and Query
3. Story 4.2: Observability

### Phase 4: Advanced Features (3 weeks)
1. Story 2.3-2.5: Confluence Advanced Features
2. Story 3.3-3.4: Jira Workflow and Comments
3. Story 4.3: Caching Layer

### Phase 5: Quality Assurance (2 weeks)
1. Story 5.1: Unit Testing
2. Story 5.2: Integration Testing
3. Story 5.3: Documentation

---

## Success Metrics
- Core Confluence operations implemented (pages, labels, search via v1)
- Core Jira operations implemented (issues, comments, transitions, search)
- Support for Cloud deployments (Server/Data Center if API compatible)
- Passthrough authentication working with encrypted credentials
- Confluence v2 API used where available, v1 for search/CQL
- Jira v3 API used throughout
- 80% test coverage achieved
- Performance: < 500ms average response time for read operations

## Risks and Mitigations
- **Risk**: API version differences between Cloud and Server
  - **Mitigation**: Implement version detection and conditional logic
- **Risk**: Token encryption/decryption failures
  - **Mitigation**: Clear error messages, support plain tokens in dev
- **Risk**: Breaking changes for existing users
  - **Mitigation**: Maintain parameter compatibility where possible

---

## Technical Notes
- **API Versions**:
  - Jira: v3 REST API (current version)
  - Confluence: v2 API for pages/labels, v1 API required for search/CQL
- **Authentication**: Passthrough-only pattern matching GitHub provider's `extractAuthToken()` method
- **Handler Pattern**: Each operation implements ToolHandler interface with Execute() and GetDefinition()
- **Handler Files**: Follow GitHub's pattern but specific names determined during implementation
- **Encryption**: Uses EncryptionService.DecryptCredential() with tenant-scoped keys
- **Verified Operations**:
  - Confluence v2: GET/DELETE /pages, GET /pages/{id}/labels
  - Confluence v1: Required for CQL search operations
  - Jira v3: Full CRUD on issues, comments, transitions, JQL search
- Each story is sized for 1-2 developers to complete in a sprint
- Dependencies are minimized to allow parallel work where possible