# Artifactory & Xray Enhancement Plan

## Overview

This plan enhances our Artifactory provider and adds a new Xray provider based on features from the JFrog MCP. Research has been completed (see JFROG_API_RESEARCH.md) confirming most required endpoints already exist in our implementation.

## DevMesh Pattern Compliance

This plan follows established DevMesh patterns:
- **Provider Pattern:** Extends BaseProvider with StandardToolProvider interface
- **Operation Mappings:** Uses existing OperationMapping structure
- **Authentication:** Leverages BaseProvider's passthrough auth via context
- **Testing:** Follows testify/assert with httptest patterns
- **Error Handling:** Uses wrapped errors with context (`fmt.Errorf`)
- **Logging:** Uses structured logging via observability.Logger
- **No Magic Numbers:** Will use named constants
- **No Debug Statements:** Production-ready code only

## Implementation Approach

Following the GitHub provider pattern with operation mappings and passthrough authentication. All features inspired by JFrog MCP but implemented independently in Go.

## Prerequisites & Research Tasks

### ✅ Research Task 1: JFrog API Documentation Review (COMPLETE)
**Status:** Complete - See JFROG_API_RESEARCH.md
**Key Findings:**
- Most required endpoints already exist in our implementation
- No direct `/whoami` endpoint - use `/api/security/apiKey` then query user
- Xray APIs well documented at `/xray/api/v1/`
- Runtime APIs appear to be cloud-only features

### ✅ Research Task 2: Architecture Decision - Provider Structure (RESOLVED)
**Decision:** Separate providers for Artifactory and Xray
**Rationale:**
- Xray has distinct API patterns and response formats
- Not all installations have Xray (separate product)
- Easier to test and maintain separately
- Follows single responsibility principle

### ✅ Research Task 3: Validate Passthrough Authentication (CONFIRMED)
**Status:** Confirmed via research
**Findings:**
- JFrog Platform uses unified authentication
- Standard headers work: `Authorization: Bearer` or `X-JFrog-Art-Api`
- Same auth works across Artifactory and Xray services

## Epic 0: AI Agent Enablement (CRITICAL)

### Why This Epic Is Critical for AI Success

Without these stories, AI agents will fail because:
1. **Permission filtering doesn't work** - The infrastructure isn't integrated
2. **Authentication headers are incomplete** - Missing X-JFrog-Art-Api support
3. **User identity requires 2 API calls** - AI won't know to chain these calls
4. **Operation names are cryptic** - "repos/create" doesn't tell AI what repository types exist
5. **No way to know what's unavailable** - AI will repeatedly try cloud-only features
6. **AQL syntax is complex** - AI will generate malformed queries
7. **Errors don't suggest fixes** - AI gets stuck when operations fail

This epic makes the difference between 30% and 90%+ success rate.

### Story 0.0: Fix Permission Integration Infrastructure
**Points:** 5
**BLOCKING:** Must be completed first - without this, permission filtering won't work
**Acceptance Criteria:**
- Add permission discoverer integration to ArtifactoryProvider
- Implement operation filtering mechanism in provider
- Ensure filtered operations are returned to AI agents
- Test that operations are actually filtered based on permissions

**Technical Tasks:**
- Add fields to ArtifactoryProvider:
  ```go
  type ArtifactoryProvider struct {
      *providers.BaseProvider
      permissionDiscoverer *ArtifactoryPermissionDiscoverer  // NEW
      filteredOperations   map[string]providers.OperationMapping // NEW
      allOperations        map[string]providers.OperationMapping // NEW - cache all ops
  }
  ```
- Override GetOperationMappings to return filtered operations:
  ```go
  func (p *ArtifactoryProvider) GetOperationMappings() map[string]providers.OperationMapping {
      // Return filtered operations if available
      if p.filteredOperations != nil {
          return p.filteredOperations
      }
      // Otherwise return all operations (backward compatibility)
      return p.allOperations
  }
  ```
- Initialize permission discoverer in NewArtifactoryProvider:
  ```go
  func NewArtifactoryProvider(logger observability.Logger) *ArtifactoryProvider {
      provider := &ArtifactoryProvider{
          BaseProvider:         providers.NewBaseProvider("artifactory", logger),
          permissionDiscoverer: NewArtifactoryPermissionDiscoverer(logger),
          allOperations:        p.getAllOperationMappings(), // Store all ops
      }
      // Initialize filtered operations on first use
      return provider
  }
  ```
- Add method to trigger permission discovery and filtering:
  ```go
  func (p *ArtifactoryProvider) InitializeWithPermissions(ctx context.Context, apiKey string) error {
      permissions, err := p.permissionDiscoverer.DiscoverPermissions(ctx, apiKey)
      if err != nil {
          return fmt.Errorf("failed to discover permissions: %w", err)
      }

      // Filter operations based on permissions
      p.filteredOperations = p.permissionDiscoverer.FilterOperationsByPermissions(
          p.allOperations,
          permissions,
      )

      p.logger.Info("Initialized Artifactory provider with filtered operations", map[string]interface{}{
          "total_operations": len(p.allOperations),
          "allowed_operations": len(p.filteredOperations),
      })
      return nil
  }
  ```
- Create comprehensive tests to verify filtering works

### Story 0.1: Create AI-Friendly Operation Helpers
**Points:** 3
**Critical:** Required for AI agents to use the API effectively
**Acceptance Criteria:**
- Simplify complex multi-step operations into single callable operations
- Add clear operation discovery mechanisms
- Provide explicit feature availability responses
- Include actionable error messages

**Technical Tasks:**
- Create internal operation category for helper functions:
  ```go
  // Since operations are static mappings, not methods, we need a different approach
  // Add to operation mappings in GetOperationMappings():
  "internal/current-user": {
      Method: "INTERNAL",  // Special method type for internal operations
      PathTemplate: "",     // No external API call
      Description: "Get current authenticated user details",
      Handler: p.handleGetCurrentUser,  // NEW: Add handler field to OperationMapping
  },
  "internal/available-features": {
      Method: "INTERNAL",
      PathTemplate: "",
      Description: "Get list of available JFrog features and their status",
      Handler: p.handleGetAvailableFeatures,
  }
  ```
- Implement handler methods that encapsulate complex logic:
  ```go
  // Handler for internal/current-user operation
  func (p *ArtifactoryProvider) handleGetCurrentUser(ctx context.Context, params map[string]interface{}) (interface{}, error) {
      // 1. Call /api/security/apiKey to get context
      apiKeyResp, err := p.ExecuteAction(ctx, "security/apikey/get", params)
      if err != nil {
          return nil, fmt.Errorf("failed to get API key info: %w", err)
      }

      // 2. Extract username from response
      username := extractUsername(apiKeyResp)

      // 3. Call /api/security/users/{userName}
      userParams := map[string]interface{}{"userName": username}
      userResp, err := p.ExecuteAction(ctx, "users/get", userParams)
      if err != nil {
          return nil, fmt.Errorf("failed to get user details: %w", err)
      }

      // 4. Return structured response
      return userResp, nil
  }

  // Handler for internal/available-features operation
  func (p *ArtifactoryProvider) handleGetAvailableFeatures(ctx context.Context, params map[string]interface{}) (interface{}, error) {
      features := make(map[string]FeatureStatus)

      // Probe each feature endpoint
      features["xray"] = p.probeFeature(ctx, "/xray/api/v1/system/version")
      features["pipelines"] = p.probeFeature(ctx, "/pipelines/api/v1/system/info")
      features["mission-control"] = p.probeFeature(ctx, "/mc/api/v1/system/info")

      return features, nil
  }
  ```
- Modify BaseProvider.ExecuteAction to handle INTERNAL method type:
  ```go
  // In ExecuteAction, check for INTERNAL method:
  if mapping.Method == "INTERNAL" && mapping.Handler != nil {
      return mapping.Handler(ctx, parameters)
  }
  ```
- Wrap all errors with context: what failed, why, and suggested action

### Story 0.2: Enhance Operation Definitions for AI Discovery
**Points:** 3
**Critical:** AI agents need semantic understanding of operations
**Acceptance Criteria:**
- Add detailed descriptions to every operation mapping
- Include examples for complex operations
- Add semantic tags for operation discovery
- Provide parameter validation with clear error messages

**Technical Tasks:**
- Update GetAIOptimizedDefinitions() to include:
  ```go
  "repos/create": {
      Description: "Create a new repository in Artifactory (local, remote, or virtual)",
      LongDescription: "Creates a repository for storing artifacts. Local repos store artifacts, remote repos proxy external sources, virtual repos aggregate multiple repos.",
      Examples: []Example{
          {
              Description: "Create a local Maven repository",
              Parameters: map[string]interface{}{
                  "key": "my-maven-local",
                  "rclass": "local",
                  "packageType": "maven",
              },
          },
      },
      SemanticTags: []string{"create", "repository", "storage", "artifact-management"},
      ParameterValidation: map[string]ValidationRule{
          "rclass": {Required: true, Values: []string{"local", "remote", "virtual"}},
          "packageType": {Required: true, Values: []string{"maven", "npm", "docker", "generic"}},
      },
  }
  ```
- Add similar detailed definitions for ALL operations
- Include error examples with resolution steps

### Story 0.3: Add AQL Query Builder for AI Agents
**Points:** 2
**Critical:** AQL syntax is complex for AI to construct
**Acceptance Criteria:**
- Provide structured AQL query builder
- Include common query templates
- Validate queries before execution
- Return clear error messages for invalid queries

**Technical Tasks:**
- Create AQL helper methods:
  ```go
  type AQLQueryBuilder struct {
      findClause   string
      includeFields []string
      sortBy       string
      limit        int
  }

  func (b *AQLQueryBuilder) FindItemsByName(pattern string) *AQLQueryBuilder
  func (b *AQLQueryBuilder) FindItemsByProperty(key, value string) *AQLQueryBuilder
  func (b *AQLQueryBuilder) Build() (string, error) // Returns valid AQL string
  ```
- Add common query templates:
  ```go
  AQLTemplates = map[string]string{
      "find-by-name": `items.find({"name":{"$match":"%s"}})`,
      "find-by-checksum": `items.find({"actual_sha1":"%s"})`,
      "find-recent": `items.find({"modified":{"$gt":"%s"}})`,
  }
  ```
- Include in operation definition with examples

### Story 0.4: Implement Capability Reporting
**Points:** 2
**Critical:** AI needs to know what's not available and why
**Acceptance Criteria:**
- Report unavailable operations with clear reasons
- Distinguish between: not installed, no permission, cloud-only
- Return structured capability report
- Cache capability discovery results

**Technical Tasks:**
- Add capability reporting to both providers:
  ```go
  type Capability struct {
      Available bool                   `json:"available"`
      Reason    string                 `json:"reason,omitempty"`
      Required  []string               `json:"required,omitempty"` // Required: ["Xray license", "Admin permission"]
  }

  type CapabilityReport struct {
      Operations map[string]Capability `json:"operations"`
      Features   map[string]Capability `json:"features"`
  }
  ```
- Return for unavailable operations:
  ```json
  {
      "error": "operation_unavailable",
      "operation": "xray/scan/artifact",
      "reason": "Xray is not installed or accessible",
      "resolution": "Ensure Xray is installed and your API key has access"
  }
  ```
- Include in health check response

### Story 0.5: Add JFrog-Specific Authentication Headers
**Points:** 2
**Critical:** Many JFrog installations require X-JFrog-Art-Api header
**Acceptance Criteria:**
- Support X-JFrog-Art-Api header for API key authentication
- Maintain backward compatibility with Bearer token auth
- Auto-detect which header to use based on credential type
- Test with both authentication methods

**Technical Tasks:**
- Extend BaseProvider's applyAuthentication method:
  ```go
  // In base_provider.go applyAuthentication method
  func (p *BaseProvider) applyAuthentication(req *http.Request, pctx *ProviderContext) error {
      // Check if this is JFrog provider (artifactory or xray)
      if p.Name == "artifactory" || p.Name == "xray" {
          // Use X-JFrog-Art-Api for API keys
          if pctx.Credentials.APIKey != "" {
              req.Header.Set("X-JFrog-Art-Api", pctx.Credentials.APIKey)
              return nil
          }
          // Fall back to Bearer token if available
          if pctx.Credentials.Token != "" {
              req.Header.Set("Authorization", "Bearer " + pctx.Credentials.Token)
              return nil
          }
      }

      // Standard authentication for other providers
      switch strings.ToLower(pctx.Credentials.Type) {
      case "bearer":
          if pctx.Credentials.Token != "" {
              req.Header.Set("Authorization", "Bearer " + pctx.Credentials.Token)
          }
      case "api_key":
          if pctx.Credentials.APIKey != "" {
              // For JFrog, this is handled above
              // For others, use standard API key header
              req.Header.Set("X-API-Key", pctx.Credentials.APIKey)
          }
      // ... other auth types
      }
      return nil
  }
  ```
- Add authentication type detection:
  ```go
  func (p *ArtifactoryProvider) detectAuthType(credentials Credentials) string {
      // API keys that start with certain patterns
      if strings.HasPrefix(credentials.APIKey, "AKC") ||
         strings.HasPrefix(credentials.APIKey, "cmVm") {
          return "artifactory_api_key"
      }
      // Access tokens
      if strings.Contains(credentials.Token, ".") &&
         len(credentials.Token) > 50 {
          return "access_token"
      }
      return "unknown"
  }
  ```
- Test authentication with both headers:
  ```go
  // Test with X-JFrog-Art-Api header
  testWithAPIKey(t, "X-JFrog-Art-Api", apiKey)

  // Test with Bearer token
  testWithBearer(t, "Authorization", "Bearer " + token)
  ```
- Add logging for authentication method used:
  ```go
  p.logger.Debug("Using JFrog authentication", map[string]interface{}{
      "method": authMethod,
      "header": headerName,
  })
  ```

## Epic 1: Enhance Core Artifactory Operations

### Story 1.0: Add Permission-Based Operation Filtering
**Points:** 10 (increased - includes integration work)
**Critical:** Must be implemented for security compliance
**Dependencies:** Story 0.0 must be completed first
**Acceptance Criteria:**
- Create `ArtifactoryPermissionDiscoverer` following Harness pattern
- Implement permission discovery using existing endpoints
- Integrate discoverer into ArtifactoryProvider (critical)
- Filter operations based on discovered permissions
- Verify operations are actually filtered when returned
- Support graceful degradation for limited permissions
- Cache permission discovery results

**Technical Tasks:**
- Create permission discoverer following Harness pattern:
  ```go
  // pkg/tools/providers/artifactory/permission_discoverer.go
  type ArtifactoryPermissionDiscoverer struct {
      logger     observability.Logger
      httpClient *http.Client
  }

  type ArtifactoryPermissions struct {
      UserInfo       map[string]interface{}
      Repositories   map[string][]string  // repo -> permissions (read/write/admin)
      EnabledFeatures map[string]bool     // feature -> enabled
      IsAdmin        bool
      Scopes         []string            // For compatibility with base
  }
  ```
- Implement DiscoverPermissions method:
  ```go
  func (d *ArtifactoryPermissionDiscoverer) DiscoverPermissions(ctx context.Context, apiKey string) (*ArtifactoryPermissions, error) {
      perms := &ArtifactoryPermissions{
          UserInfo:        make(map[string]interface{}),
          Repositories:    make(map[string][]string),
          EnabledFeatures: make(map[string]bool),
      }

      // 1. Get user identity (2-step process)
      if err := d.getUserInfo(ctx, apiKey, perms); err != nil {
          d.logger.Debug("Failed to get user info", map[string]interface{}{
              "error": err.Error(),
          })
      }

      // 2. Probe repository access
      d.probeRepositoryAccess(ctx, apiKey, perms)

      // 3. Check admin capabilities
      d.checkAdminAccess(ctx, apiKey, perms)

      // 4. Detect available features
      d.detectFeatures(ctx, apiKey, perms)

      return perms, nil
  }
  ```
- Implement FilterOperationsByPermissions:
  ```go
  func (d *ArtifactoryPermissionDiscoverer) FilterOperationsByPermissions(
      operations map[string]providers.OperationMapping,
      permissions *ArtifactoryPermissions,
  ) map[string]providers.OperationMapping {
      filtered := make(map[string]providers.OperationMapping)

      for opID, op := range operations {
          allowed := false

          // Check based on operation type
          switch {
          case strings.Contains(opID, "admin"):
              allowed = permissions.IsAdmin
          case strings.Contains(opID, "create") || strings.Contains(opID, "update"):
              allowed = d.hasWritePermission(permissions, opID)
          case strings.Contains(opID, "xray"):
              allowed = permissions.EnabledFeatures["xray"]
          default:
              // Read operations generally allowed if authenticated
              allowed = true
          }

          if allowed {
              filtered[opID] = op
          }
      }

      d.logger.Info("Filtered operations", map[string]interface{}{
          "total": len(operations),
          "allowed": len(filtered),
      })

      return filtered
  }
  ```
- Use existing endpoints for discovery:
  - User identity: Call `/api/security/apiKey` then `/api/security/users/{userName}`
  - Repository access: Use `/api/repositories` (already implemented)
  - Permissions: Use `/api/v2/security/permissions` (already implemented)
  - Admin check: Probe `/api/system/configuration` (403 = not admin)
  - Xray detection: Probe `/xray/api/v1/system/version`
- CRITICAL: Wire up in provider (see Story 0.0 for integration)
- Add comprehensive tests:
  ```go
  func TestPermissionFiltering(t *testing.T) {
      // Test that admin operations are filtered for non-admin
      // Test that write operations are filtered without write perms
      // Test that Xray operations are filtered when not available
      // CRITICAL: Test that GetOperationMappings returns filtered ops
  }
  ```

## Epic 1: Enhance Core Artifactory Operations (continued)

### Story 1.1: Enhance AQL (Artifactory Query Language) Support
**Points:** 2 (reduced - endpoint exists)
**Note:** Endpoint `/api/search/aql` already exists in our implementation
**Acceptance Criteria:**
- Enhance existing AQL operation mapping for better parameter handling
- Support complex AQL query strings (text/plain content type)
- Handle paginated results if supported
- Add query validation

**Technical Tasks:**
- Update existing `search/aql` operation in `GetOperationMappings()`
- Change content type to `text/plain` for AQL queries
- Implement proper AQL query string formatting
- Add response parsing for AQL-specific format
- Create unit tests for complex AQL queries

### Story 1.2: Add Project-Based Operations
**Points:** 3 (reduced - may use existing repo APIs)
**Note:** Research indicates Projects may be enterprise-only
**Acceptance Criteria:**
- Verify if JFrog Projects API is available in self-hosted versions
- If available, add project management operations
- Support project-scoped repository access if available
- Handle project membership and permissions

**Technical Tasks:**
- Check if Projects API exists in target JFrog version
- If available, implement using existing repository patterns
- Add project context to repository operations
- Update permission operations if project-scoped
- Create tests with project scenarios

## Epic 2: Add Xray Security Scanning Support

### Story 2.1: Implement Xray Provider Structure
**Points:** 5
**Acceptance Criteria:**
- Create separate Xray provider (architecture decision made)
- Implements StandardToolProvider interface
- Includes permission-based filtering from the start
- Follows same pattern as existing providers
- Supports passthrough authentication

**Technical Tasks:**
- Create new `pkg/tools/providers/xray/xray_provider.go`
- Register in `apps/rest-api/internal/api/providers_init.go`:
  ```go
  xrayProvider := xray.NewXrayProvider(logger)
  if err := registry.RegisterProvider(xrayProvider); err != nil {
      logger.Error("Failed to register Xray provider", map[string]interface{}{
          "error": err.Error(),
      })
  }
  ```
- Implement all StandardToolProvider interface methods
- Create `XrayPermissionDiscoverer` similar to Artifactory
- Add GetAIOptimizedDefinitions() for AI-friendly definitions
- Add comprehensive tests in same package (xray_provider_test.go)

### Story 2.2: Implement Xray Scan Operations
**Points:** 8 (reduced - endpoints documented)
**Acceptance Criteria:**
- Add vulnerability scanning operations using Xray API
- Support artifact summary endpoint (`/xray/api/v1/summary/artifact`)
- Handle scan results with severity grouping (Critical, High, Medium, Low)
- Parse Xray-specific response formats

**Technical Tasks:**
- Implement operation mappings for:
  - `xray/summary/artifact`: POST `/xray/api/v1/summary/artifact`
  - `xray/scan/artifact`: POST `/xray/api/v1/scan/artifact`
  - `xray/scan/status`: GET `/xray/api/v1/scan/status/{scan_id}`
  - `xray/scan/build`: POST `/xray/api/v1/scan/build`
- Implement severity categorization from response
- Create response parsing for Xray format (different from Artifactory)
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

## Epic 3: Integration and Testing

### Story 3.1: Add Passthrough Authentication for Xray
**Points:** 2 (reduced - confirmed unified auth)
**Acceptance Criteria:**
- Xray provider uses same auth as Artifactory (confirmed)
- Handle unified platform tokens
- Support instance-specific endpoints

**Technical Tasks:**
- Extend BaseProvider for Xray (inherits auth handling)
- Use same headers: `Authorization: Bearer` or `X-JFrog-Art-Api`
- Support custom base URLs for Xray endpoints
- Test with both token types

### Story 3.2: Create Integration Tests
**Points:** 5
**Dependencies:** All implementation stories
**Acceptance Criteria:**
- Mock server responses based on actual API responses
- End-to-end testing for implemented operations
- Performance testing

**Technical Tasks:**
- Extend mockserver with Xray response formats
- Create test fixtures using documented response structures
- Add integration tests for new operations
- Test with both Artifactory and Xray providers

### Story 3.3: Update Documentation and Examples
**Points:** 2
**Acceptance Criteria:**
- Update provider documentation
- Add usage examples for new operations
- Include AI-optimized definitions

**Technical Tasks:**
- Update GetAIOptimizedDefinitions() for both providers
- Add semantic tags for new operations
- Create example workflows for common scenarios
- Document authentication requirements

## Epic 4: Enhanced Search and Discovery

### Story 4.1: Enhance Existing Search Operations
**Points:** 2
**Note:** We already have search operations - minor enhancements only
**Acceptance Criteria:**
- Review existing search operations for completeness
- Add any missing search parameters
- Ensure all search types are covered

**Technical Tasks:**
- Verify existing operations: artifact, AQL, GAVC, property, checksum
- Add any missing query parameters
- Ensure proper error handling for search operations
- Update tests for search enhancements

### Story 4.2: Simplify Package Discovery
**Points:** 2 (reduced - uses existing endpoints)
**Acceptance Criteria:**
- Use existing storage API for package operations
- Support package info and version listing
- Handle standard repository layouts

**Technical Tasks:**
- Use `/api/storage/{repoKey}/{packagePath}` for package info
- Use `/api/storage/{repoKey}/{packageName}?list&deep=1` for versions
- Add convenience operations for common package queries
- Test with different package types (Maven, npm, Docker, etc.)

## Implementation Order

**Phase 0 (Pre-Implementation):** ✅ COMPLETE
- Research Task 1: JFrog API Documentation Review ✅
- Research Task 2: Architecture Decision - Provider Structure ✅
- Research Task 3: Validate Passthrough Authentication ✅

**Phase 1 (Sprint 1) - Critical Infrastructure & AI Foundation:** 🚨🤖
- Story 0.0: Fix Permission Integration Infrastructure (5 points) - **BLOCKING**
- Story 0.5: Add JFrog-Specific Authentication Headers (2 points) - **CRITICAL**
- Story 0.1: Create AI-Friendly Operation Helpers (3 points) - CRITICAL
- Story 0.2: Enhance Operation Definitions for AI Discovery (3 points) - CRITICAL
- Story 0.3: Add AQL Query Builder for AI Agents (2 points)
- Story 0.4: Implement Capability Reporting (2 points)
Total: 17 points

**Phase 2 (Sprint 2) - Permission Discovery & Core:**
- Story 1.0: Add Permission-Based Operation Filtering (10 points) - CRITICAL
- Story 1.1: Enhance AQL Support (2 points)
Total: 12 points

**Phase 3 (Sprint 3) - Xray Provider:**
- Story 2.1: Implement Xray Provider Structure (5 points)
- Story 3.1: Add Passthrough Authentication for Xray (2 points)
- Story 2.2: Implement Xray Scan Operations (8 points)
Total: 15 points

**Phase 4 (Sprint 4) - Search & Advanced Features:**
- Story 4.1: Enhance Existing Search Operations (2 points)
- Story 2.3: Xray Component Intelligence (8 points)
- Story 1.2: Add Project-Based Operations (3 points) - If available
Total: 13 points

**Phase 5 (Sprint 5) - Reports and Discovery:**
- Story 2.4: Xray Reports and Metrics (5 points)
- Story 4.2: Simplify Package Discovery (2 points)
- Story 3.2: Create Integration Tests (5 points)
Total: 12 points

**Phase 6 (Sprint 6) - Documentation & Polish:**
- Story 3.3: Update Documentation (2 points)
- Performance optimization
- Bug fixes
Total: 2+ points

## Technical Considerations

### Provider Structure (Decided)
- **Decision:** Separate providers for Artifactory and Xray
- Xray provider structure:
  ```go
  type XrayProvider struct {
      *providers.BaseProvider
      permissionDiscoverer *XrayPermissionDiscoverer
      httpClient *http.Client
  }
  ```
- Must implement StandardToolProvider interface methods
- Operation mappings will use documented Xray API endpoints

### Authentication (Confirmed)
- JFrog Platform uses unified authentication across services
- Required headers:
  - `Authorization: Bearer <TOKEN>` (recommended)
  - `X-JFrog-Art-Api: <API_KEY>` (legacy)
- Same authentication works for both Artifactory and Xray

### API Integration Details
- **Artifactory endpoints:** Already implemented at `/api/`
- **Xray endpoints:** Documented at `/xray/api/v1/`
- **Response formats:** JSON for both services (different structures)
- **Error codes:** Standard HTTP codes plus JFrog-specific messages

### Testing Approach

Following DevMesh testing patterns:
- **Unit Tests:** In same package as code (not `_test` package)
- **Test Framework:** Always use testify/assert and testify/mock
- **Table Tests:** Preferred for multiple scenarios
- **Coverage:** Minimum 80% for new code
- **Mock Pattern:**
  ```go
  func TestOperationName(t *testing.T) {
      logger := &observability.NoopLogger{}
      provider := NewXrayProvider(logger)

      // Test with httptest server
      server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          // Mock response
      }))
      defer server.Close()
  }
  ```
- **Integration Tests:** Use existing mockserver in `apps/mockserver`
- **No external dependencies:** All tests must work without JFrog instance

## JFrog MCP Operations Mapping

Operations from JFrog MCP and their implementation status:

### Already Implemented ✅
- `check_jfrog_availability` → Use existing health check
- `create_local_repository` → `repos/create`
- `create_remote_repository` → `repos/create` with rclass
- `create_virtual_repository` → `repos/create` with rclass
- `list_repositories` → `repos/list`
- `set_folder_property` → `artifacts/properties/set`
- `list_jfrog_builds` → `builds/list`
- `get_specific_build` → `builds/get`

### To Be Enhanced 🔧
- `execute_aql_query` → **Story 1.1** (endpoint exists, needs enhancement)

### New Xray Operations 🆕
- `jfrog_get_artifacts_summary` → **Story 2.2** (Xray provider)
- `jfrog_get_package_version_vulnerabilities` → **Story 2.3** (Xray provider)
- `jfrog_get_vulnerability_info` → **Story 2.3** (Xray provider)

### Optional/Enterprise Features ⚠️
- `list_jfrog_projects` → **Story 1.2** (may be enterprise-only)
- `jfrog_get_package_info` → **Story 4.2** (uses existing storage API)
- `jfrog_get_package_versions` → **Story 4.2** (uses existing storage API)

### Deferred (Cloud-Only) ❌
- `list_jfrog_runtime_clusters` - Cloud/Edge feature
- `get_jfrog_runtime_specific_cluster` - Cloud/Edge feature
- `list_jfrog_running_images` - Cloud/Edge feature
- `jfrog_get_package_curation_status` - Unverified API

## Success Metrics
- ✅ **AI agents can successfully use all operations** (Epic 0)
- ✅ Permission-based operation filtering implemented for security
- ✅ Xray provider created and integrated
- ✅ Enhanced operations inspired by JFrog MCP where APIs exist
- ✅ Both providers pass StandardToolProvider interface tests
- ✅ No breaking changes to existing Artifactory operations
- ✅ Test coverage of 80%+ for new code
- ✅ Clear separation between Artifactory and Xray concerns
- ✅ **Clear error messages and capability reporting for AI agents**
- ✅ **Single-method helpers for complex operations**

## Summary

This plan addresses AI agent limitations by adding **Epic 0: AI Agent Enablement** with 4 critical stories that:
- Simplify complex multi-step operations (like user identity lookup)
- Provide detailed operation definitions with examples and semantic tags
- Add structured query builders for complex syntax (AQL)
- Report capabilities explicitly (what's available, what's not, and why)

**Key Changes:**
- Added Epic 0 with 4 AI enablement stories (10 points)
- Removed cloud-only Runtime features (6 stories removed)
- Leveraging existing endpoints (reduced complexity)
- Focusing on verified, documented APIs

**Total Stories:** 20 (6 AI/Infrastructure enablement + 14 feature stories)
**Total Points:** ~74 (19 AI/Infrastructure + 55 features)
**Estimated Timeline:** 6 sprints
**AI Success Rate:** Expected to increase from 30% to 90%+ (with infrastructure fixes)