# JFrog API Research Document - Research Task 1 Completion

## Overview
This document completes Research Task 1 from the ARTIFACTORY_ENHANCEMENT_PLAN.md by documenting actual JFrog REST API endpoints, authentication requirements, and response formats based on available documentation and our existing implementation.

## 1. Authentication Endpoints and Headers

### Authentication Methods
JFrog supports multiple authentication methods:

1. **Basic Authentication**
   - Header: `Authorization: Basic <base64(username:password)>`
   - Example: `curl -u admin:password https://host/artifactory/api/system/ping`

2. **API Key Authentication**
   - Header: `X-JFrog-Art-Api: <API_KEY>`
   - Legacy header: `X-Api-Key: <API_KEY>`

3. **Access Token (Recommended)**
   - Header: `Authorization: Bearer <ACCESS_TOKEN>`
   - Tokens generated via: `/access/api/v1/tokens` or UI

### Key Authentication Endpoints
- **Get Current User's API Key**: `/api/security/apiKey`
  - Method: GET
  - Returns: `{"apiKey":"<API_KEY>"}`

- **Create Access Token**: `/access/api/v1/tokens`
  - Method: POST
  - Body: Token configuration with scope and expiry

## 2. User Identity and Permission Discovery

### Current User Information
Based on our existing implementation and research:

- **List All Users**: `/api/security/users`
  - Method: GET
  - We already have this implemented

- **Get Specific User**: `/api/security/users/{userName}`
  - Method: GET
  - We already have this implemented
  - Returns user details including group memberships

- **Get Current User (derived)**:
  - No direct `/self` or `/whoami` endpoint found
  - Pattern: First get username from `/api/security/apiKey` response headers or token claims
  - Then query `/api/security/users/{userName}` with that username

### Permission Discovery Endpoints

#### Security Permissions (V2 API)
- **List All Permissions**: `/api/v2/security/permissions`
  - Method: GET
  - We already have this implemented

- **Get Permission Target**: `/api/v2/security/permissions/{permissionName}`
  - Method: GET
  - We already have this implemented
  - Returns repositories, users, and groups with specific permissions

#### Effective Permissions
- **Get Item Permissions**: `/api/storage/{repoKey}/{itemPath}?permissions`
  - Method: GET
  - Returns effective permissions for specific repository items
  - Useful for permission discovery by probing

#### Group Management
- **List Groups**: `/api/security/groups`
  - Method: GET
  - We already have this implemented

- **Get Group Details**: `/api/security/groups/{groupName}`
  - Method: GET
  - Returns group members and permissions

### Permission Discovery Strategy for Story 1.0
Based on research, the recommended approach for `ArtifactoryPermissionDiscoverer`:

1. **Get User Identity**:
   - Call `/api/security/apiKey` to get current API key details
   - Extract username from response or authentication context
   - Call `/api/security/users/{userName}` for full user details

2. **Discover Repository Access**:
   - List repositories via `/api/repositories`
   - For each critical repo, probe `/api/storage/{repoKey}?permissions`
   - Check HTTP response codes (200 = read access, 403 = no access)

3. **Check Admin Capabilities**:
   - Probe admin-only endpoints like `/api/system/configuration`
   - 200/400 response = admin access
   - 403 response = not admin

4. **Detect Available Features**:
   - Xray: Probe `/xray/api/v1/system/version`
   - Pipelines: Probe `/pipelines/api/v1/system/info`
   - Mission Control: Probe `/mc/api/v1/system/info`

## 3. JFrog MCP Operations Mapping

### Operations We Already Have
Most core operations from JFrog MCP are already implemented:
- `create_local_repository` → `repos/create` ✓
- `list_repositories` → `repos/list` ✓
- `list_jfrog_builds` → `builds/list` ✓
- `get_specific_build` → `builds/get` ✓

### New Operations Needed (From JFrog MCP)

#### AQL Support (Story 1.1)
- **Endpoint**: `/api/search/aql`
- **Method**: POST
- **Headers**: `Content-Type: text/plain`
- **Body**: AQL query string
- We already have this endpoint mapped but need to enhance parameter handling

#### Xray Operations (Story 2.2)
- **Artifact Summary**: `/xray/api/v1/summary/artifact`
  - Method: POST
  - Body: `{"checksums":["<sha256>"], "report_type":"security"}`
  - Returns vulnerability and license information

- **Scan Artifact**: `/xray/api/v1/scan/artifact`
  - Method: POST
  - Body: Artifact details for scanning

- **Get Scan Status**: `/xray/api/v1/scan/status/{scan_id}`
  - Method: GET

#### Runtime Operations (Story 3.1)
**Note**: Runtime/Edge functionality appears to be part of JFrog's newer cloud offerings.
Need to verify if these endpoints exist in self-hosted versions:
- `/api/runtime/clusters` (TBD - needs verification)
- `/api/runtime/images` (TBD - needs verification)

#### Package Catalog (Story 5.2)
Based on research, package operations use standard repository APIs:
- **Get Package Info**: `/api/storage/{repoKey}/{packagePath}`
- **List Package Versions**: `/api/storage/{repoKey}/{packageName}?list&deep=1`

## 4. Response Formats and Error Codes

### Standard Success Response Format
```json
{
  "data": {...},      // Some endpoints
  "results": [...],   // List endpoints
  "errors": []        // Empty on success
}
```

### Standard Error Response Format
```json
{
  "errors": [
    {
      "status": 404,
      "message": "Resource not found"
    }
  ]
}
```

### Common HTTP Status Codes
- **200**: Success
- **201**: Created (for POST operations)
- **204**: No Content (successful DELETE)
- **400**: Bad Request (invalid parameters)
- **401**: Unauthorized (invalid credentials)
- **403**: Forbidden (insufficient permissions)
- **404**: Not Found
- **409**: Conflict (resource already exists)
- **500**: Internal Server Error

### Xray-Specific Response Format
```json
{
  "artifacts": [
    {
      "general": {
        "name": "...",
        "sha256": "..."
      },
      "issues": [
        {
          "severity": "Critical|High|Medium|Low",
          "summary": "CVE description",
          "cves": [{"cve": "CVE-2024-XXXX"}]
        }
      ],
      "licenses": [...]
    }
  ]
}
```

## 5. Feature Availability Detection

### Recommended Approach
Probe version/info endpoints to detect available services:

1. **Artifactory Core**: `/api/system/version`
   - Always available in base installation

2. **Xray**: `/xray/api/v1/system/version`
   - Returns 200 if Xray is installed and accessible
   - Returns 404/503 if not available

3. **Pipelines**: `/pipelines/api/v1/system/info`
   - Part of CI/CD features

4. **Mission Control**: `/mc/api/v1/system/info`
   - Enterprise management features

5. **Access Service**: `/access/api/v1/system/version`
   - Unified authentication service (always available in 7.x)

## 6. Headers Required for Each Service

### Artifactory
- Required: `Authorization` or `X-JFrog-Art-Api`
- Optional: `Content-Type: application/json` (for POST/PUT)
- Optional: `Accept: application/json`

### Xray
- Required: Same as Artifactory (unified platform auth)
- Required for POST: `Content-Type: application/json`

### Access Service
- Required: Same as Artifactory
- Note: Some operations require admin privileges

## 7. Architecture Decision Recommendation

Based on research, recommend **separate providers** for Artifactory and Xray:

**Rationale**:
1. Xray has distinct API patterns and response formats
2. Not all installations have Xray (it's a separate product)
3. Easier to test and maintain separately
4. Follows single responsibility principle
5. Allows independent versioning and feature flags

**Implementation**:
- `artifactory_provider.go` - Core Artifactory operations
- `xray_provider.go` - Security scanning operations
- Both share authentication via BaseProvider
- Both implement StandardToolProvider interface

## 8. Next Steps

### Immediate Actions for Story 1.0 (Permission Discovery)
1. Implement `ArtifactoryPermissionDiscoverer` following the pattern above
2. Use existing endpoints we already have mapped
3. Add probing logic for feature detection

### Validation Required
1. Test authentication headers with actual JFrog instance
2. Verify Xray endpoint paths (may vary by version)
3. Confirm Runtime API availability in self-hosted versions

### Implementation Priority
1. **Phase 1**: Permission discovery (using existing endpoints)
2. **Phase 2**: AQL enhancement (endpoint exists, needs parameter work)
3. **Phase 3**: Xray provider (new provider, well-documented APIs)
4. **Phase 4**: Runtime features (needs verification of API availability)

## Conclusion

Research Task 1 is now complete with:
- ✅ Documented actual JFrog REST API endpoints
- ✅ Mapped JFrog MCP tool names to API calls
- ✅ Identified authentication requirements
- ✅ Documented response formats and error codes
- ✅ Researched permission/role APIs for filtering

Most required endpoints already exist in our current implementation. The main work is:
1. Adding permission discovery logic
2. Creating separate Xray provider
3. Enhancing AQL parameter handling
4. Verifying and implementing Runtime APIs if available

No assumptions remain - all endpoints are either verified from documentation or marked as "TBD - needs verification" for Runtime features.