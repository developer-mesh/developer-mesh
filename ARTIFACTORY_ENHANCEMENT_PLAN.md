# Artifactory & Xray Enhancement Plan

## Critical Note: Research Required

This plan identifies features inspired by the JFrog MCP but **requires research** to determine actual JFrog API endpoints and implementation details. No assumptions have been made about API paths, request formats, or service boundaries. All technical implementation details marked "TBD from research" must be determined before implementation begins.

## Implementation Approach

Following the GitHub provider pattern with operation mappings and passthrough authentication. All features inspired by JFrog MCP but implemented independently in Go.

## Prerequisites & Research Tasks

### Research Task 1: JFrog API Documentation Review
**Points:** 3
**Required Before:** All implementation stories
**Deliverables:**
- Document actual JFrog REST API endpoints for each operation
- Map JFrog MCP tool names to actual API calls
- Identify authentication requirements for each service
- Document response formats and error codes

### Research Task 2: Architecture Decision - Provider Structure
**Points:** 2
**Required Before:** Epic 2
**Decision Required:**
- Single provider with both Artifactory and Xray operations OR
- Separate providers for Artifactory and Xray
- Consider: authentication sharing, code organization, testing isolation

### Research Task 3: Validate Passthrough Authentication
**Points:** 2
**Required Before:** All implementation
**Validation Required:**
- Confirm JFrog Platform unified tokens work across services
- Test passthrough auth with Artifactory API
- Test passthrough auth with Xray API (if accessible)
- Document header requirements (Authorization, X-JFrog-Art-Api, etc.)

## Epic 1: Enhance Core Artifactory Operations

### Story 1.1: Add AQL (Artifactory Query Language) Support
**Points:** 5
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Add operation mapping for AQL execution (actual endpoint TBD from research)
- Support complex JSON query bodies
- Handle paginated results if supported by API
- Add query validation based on JFrog documentation

**Technical Tasks:**
- Research actual AQL endpoint path and method from JFrog API docs
- Add AQL operation to `GetOperationMappings()` in artifactory_provider.go
- Implement query parameter transformation for complex JSON
- Add response parsing based on actual AQL response format
- Create unit tests for AQL query execution

### Story 1.2: Add Package Curation Operations
**Points:** 8
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Implement operations inspired by JFrog MCP's curation tools
- Support checking package curation status (`jfrog_get_package_curation_status` equivalent)
- Handle curation-specific response formats

**Technical Tasks:**
- Research if curation is part of Artifactory or Xray API
- Document actual endpoints for package curation
- Add operation mappings based on actual API
- Implement curation policy parameters if supported
- Create tests with appropriate mock data

### Story 1.3: Add Project-Based Operations
**Points:** 5
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Add project management operations if supported by JFrog API
- Support project-scoped repository access if available
- Handle project membership and permissions based on API capabilities

**Technical Tasks:**
- Research if JFrog Projects API exists and its endpoints
- Determine project operation names and parameters from API docs
- Implement project context only if supported by API
- Add project filtering only if API supports this feature
- Update permission operations based on actual API capabilities

## Epic 2: Add Xray Security Scanning Support

### Story 2.1: Implement Provider Structure for Xray
**Points:** 8
**Dependencies:** Research Task 2 (Architecture Decision)
**Acceptance Criteria:**
- Implementation based on architecture decision (separate provider or extend Artifactory)
- Implements StandardToolProvider interface if separate provider
- Follows same pattern as existing providers
- Supports passthrough authentication

**Technical Tasks:**
- Implement based on architecture decision:
  - Option A: Create new `pkg/tools/providers/xray/xray_provider.go`
  - Option B: Extend existing artifactory_provider.go with Xray operations
- If Option A: Register in `apps/rest-api/internal/api/providers_init.go`
- Add comprehensive tests following existing patterns

### Story 2.2: Implement Xray Scan Operations
**Points:** 13
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Add vulnerability scanning operations based on actual Xray API
- Support operations inspired by JFrog MCP's `jfrog_get_artifacts_summary`
- Handle scan results with severity grouping (Critical, High, Medium, Low, Unknown)
- Parse Xray-specific response formats

**Technical Tasks:**
- Research and document actual Xray API endpoints for:
  - Artifact scanning
  - Build scanning
  - Repository scanning
  - Retrieving scan results
  - Getting issues summary
- Add operation mappings using actual endpoint paths
- Implement severity categorization as per Xray API response
- Create response parsing based on actual Xray data structures
- Add unit tests with mocked Xray responses

### Story 2.3: Implement Xray Component Intelligence
**Points:** 8
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Add component vulnerability lookup based on actual Xray API
- Support CVE queries if available in API
- Include license compliance if supported by API
- Handle component graph analysis if API provides this

**Technical Tasks:**
- Research actual Xray Component API endpoints
- Determine available component intelligence features in API
- Add operations based on actual API capabilities (not assumed names)
- Implement component version handling as per API specification
- Add support for package types actually supported by API

### Story 2.4: Implement Xray Reports and Metrics
**Points:** 5
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Add report generation operations if supported by Xray API
- Support report types available in actual API
- Include metrics based on API capabilities

**Technical Tasks:**
- Research actual Xray Reports API endpoints
- Determine available report types and formats from API
- Add operations based on actual API (not assumed operation names)
- Implement only supported report formats (verify JSON, PDF, CSV availability)
- Add filtering parameters as actually supported by API

## Epic 3: Add Runtime Management Features

### Story 3.1: Add Runtime Cluster Operations
**Points:** 8
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Implement operations inspired by JFrog MCP's runtime management tools
- Support listing runtime clusters (`list_jfrog_runtime_clusters` equivalent)
- Support getting specific cluster (`get_jfrog_runtime_specific_cluster` equivalent)

**Technical Tasks:**
- Research actual JFrog Runtime API endpoints
- Determine if Runtime is part of Artifactory API or separate service
- Add operation mappings based on actual API
- Implement cluster ID handling as per API specification
- Add cluster type filtering if supported by API

### Story 3.2: Add Container Image Runtime Monitoring
**Points:** 5
**Dependencies:** Research Task 1, Story 3.1
**Acceptance Criteria:**
- Implement operation inspired by JFrog MCP's `list_jfrog_running_images`
- Include security status for running images if available in API
- Parse runtime-specific response formats

**Technical Tasks:**
- Research actual endpoint for listing running container images
- Determine data structure for image security status
- Add operation mappings based on actual API
- Implement filtering parameters as supported by API
- Test with various container runtime scenarios

## Epic 4: Integration and Testing

### Story 4.1: Add Passthrough Authentication for Xray
**Points:** 3
**Dependencies:** Research Task 3
**Acceptance Criteria:**
- Xray provider supports same auth as Artifactory (if confirmed by research)
- Handle unified platform tokens (if they exist)
- Support instance-specific endpoints based on actual URL patterns

**Technical Tasks:**
- Implement auth based on Research Task 3 findings
- Add only validated authentication methods
- Support URL patterns as documented by JFrog
- Test only with confirmed auth methods

### Story 4.2: Create Integration Tests
**Points:** 8
**Dependencies:** All implementation stories
**Acceptance Criteria:**
- Mock server responses based on actual API responses
- End-to-end testing for implemented operations
- Performance testing based on actual API behavior

**Technical Tasks:**
- Extend mockserver based on actual API response formats
- Create test fixtures using documented response structures
- Add integration tests for actually implemented operations
- Test rate limiting based on documented limits (if any)

### Story 4.3: Update Documentation and Examples
**Points:** 3
**Acceptance Criteria:**
- Update provider documentation
- Add usage examples for new operations
- Include AI-optimized definitions

**Technical Tasks:**
- Update GetAIOptimizedDefinitions() for both providers
- Add semantic tags for new operations
- Create example workflows for common scenarios
- Document authentication requirements

## Epic 5: Enhanced Search and Discovery

### Story 5.1: Enhance Existing Search Operations
**Points:** 5
**Dependencies:** Research Task 1
**Note:** We already have search operations - this enhances them
**Acceptance Criteria:**
- Enhance existing search operations based on API documentation
- Add additional search parameters if supported by API
- Support batch operations if API allows

**Technical Tasks:**
- Review existing search operations against latest API docs
- Add any missing parameters to existing operations
- Implement batch search only if API supports it
- Add result aggregation only if natively supported
- No saved searches unless API provides this feature

### Story 5.2: Add Package Discovery Features
**Points:** 5
**Dependencies:** Research Task 1
**Acceptance Criteria:**
- Implement operations inspired by JFrog MCP's catalog tools:
  - `jfrog_get_package_info` equivalent
  - `jfrog_get_package_versions` equivalent
- Handle package catalog response formats

**Technical Tasks:**
- Research actual JFrog Package Catalog API endpoints
- Determine if this is part of Artifactory or separate service
- Add operation mappings based on actual API
- Implement package type handling as per API
- Add version listing and filtering support

## Implementation Order

**Phase 0 (Pre-Implementation):**
- Research Task 1: JFrog API Documentation Review
- Research Task 2: Architecture Decision - Provider Structure
- Research Task 3: Validate Passthrough Authentication

**Phase 1 (Sprint 1-2):**
- Story 2.1: Implement Provider Structure for Xray
- Story 1.1: Add AQL Support
- Story 4.1: Passthrough Authentication for Xray

**Phase 2 (Sprint 3-4):**
- Story 2.2: Implement Xray Scan Operations
- Story 1.3: Add Project-Based Operations
- Story 4.2: Create Integration Tests

**Phase 3 (Sprint 5-6):**
- Story 2.3: Component Intelligence
- Story 1.2: Package Curation Operations
- Story 3.1: Runtime Cluster Operations

**Phase 4 (Sprint 7-8):**
- Story 2.4: Reports and Metrics
- Story 3.2: Container Runtime Monitoring
- Story 5.1: Advanced Search

**Phase 5 (Sprint 9):**
- Story 5.2: Package Discovery
- Story 4.3: Documentation
- Performance optimization based on actual API behavior
- Bug fixes as discovered during testing

## Technical Considerations

### Provider Structure Decision Points
- **Architecture Decision Required:** Single provider vs separate providers for Xray
- If separate provider, follow pattern from existing providers:
  ```go
  type XrayProvider struct {
      *providers.BaseProvider
      httpClient *http.Client
  }
  ```
- Operation mappings will use actual JFrog API endpoints (TBD from research)

### Authentication Requirements
- **Must Validate:** Passthrough authentication works with JFrog Platform
- **Must Document:** Required headers for each service
- Expected headers based on pattern:
  - Authorization (Bearer token or Basic auth)
  - X-JFrog-Art-Api (if using API key)
  - Additional headers TBD from API documentation

### API Integration Unknowns
- **Actual endpoint paths** - Must be determined from JFrog API documentation
- **Request/Response formats** - Need to match actual JFrog API
- **Pagination approach** - Does Xray API support pagination?
- **Rate limiting** - What are the actual limits for each service?
- **Error codes** - Document JFrog-specific error responses

### Testing Approach
- Unit tests with mocked responses based on actual API
- Integration tests require either:
  - Access to JFrog test instance
  - Comprehensive mock server updates
- Contract validation against official API documentation

## JFrog MCP Operations Mapping

Operations from JFrog MCP that need equivalents in our implementation:
1. `check_jfrog_availability` - Use existing health check
2. `create_local_repository` - Already have `repos/create`
3. `create_remote_repository` - Already have `repos/create` with rclass parameter
4. `create_virtual_repository` - Already have `repos/create` with rclass parameter
5. `list_repositories` - Already have `repos/list`
6. `set_folder_property` - Already have `artifacts/properties/set`
7. `execute_aql_query` - **NEW: Story 1.1**
8. `list_jfrog_builds` - Already have `builds/list`
9. `get_specific_build` - Already have `builds/get`
10. `list_jfrog_runtime_clusters` - **NEW: Story 3.1**
11. `get_jfrog_runtime_specific_cluster` - **NEW: Story 3.1**
12. `list_jfrog_running_images` - **NEW: Story 3.2**
13. `list_jfrog_environments` - **NEW: Story 1.3 (Projects)**
14. `list_jfrog_projects` - **NEW: Story 1.3**
15. `get_specific_project` - **NEW: Story 1.3**
16. `create_project` - **NEW: Story 1.3**
17. `jfrog_get_package_info` - **NEW: Story 5.2**
18. `jfrog_get_package_versions` - **NEW: Story 5.2**
19. `jfrog_get_package_version_vulnerabilities` - **NEW: Story 2.3**
20. `jfrog_get_vulnerability_info` - **NEW: Story 2.3**
21. `jfrog_get_package_curation_status` - **NEW: Story 1.2**
22. `jfrog_get_artifacts_summary` - **NEW: Story 2.2**

## Success Metrics
- Operations inspired by JFrog MCP are implemented where API support exists
- Provider(s) pass all StandardToolProvider interface tests
- No breaking changes to existing Artifactory operations
- Response times align with actual API performance characteristics
- Test coverage target of 80%+ (may adjust based on complexity)
- Clear documentation of what was implemented vs what was not possible