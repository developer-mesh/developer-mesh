# Organization Tool Refresh Feature

## Overview

The Organization Tool Refresh feature allows you to update an organization's tool configuration with the latest capabilities from the provider. This is particularly useful when:

- A provider adds new operations or toolsets
- The provider's API surface changes
- You've enabled additional toolsets in the provider configuration
- Tool definitions need to be synchronized with the latest provider state

## Problem It Solves

Previously, when a provider like GitHub expanded its capabilities (e.g., enabling additional toolsets), organization tools would continue using the snapshot of operations from when they were first registered. This meant organizations couldn't access new features without unregistering and re-registering the tool, which was disruptive.

## How It Works

The refresh mechanism:
1. Fetches the current provider state with all enabled toolsets
2. Updates the tool template with the latest operation mappings
3. Preserves existing configuration and credentials
4. Clears caches to ensure the new operations are immediately available

## API Endpoint

### Refresh Organization Tool

```http
PUT /api/v1/organizations/{orgId}/tools/{toolId}?action=refresh
```

**Headers:**
- `Authorization: Bearer <api_key>`
- `Content-Type: application/json`

**Response:**
```json
{
  "message": "Tool refreshed successfully",
  "tool_id": "uuid",
  "org_id": "uuid"
}
```

## Using the Refresh Script

A convenience script is provided for refreshing organization tools:

```bash
# Refresh a specific tool
./scripts/refresh-organization-tool.sh <org_id> <tool_id>

# With environment variables
export ORG_ID="your-org-id"
export TOOL_ID="your-tool-id"
./scripts/refresh-organization-tool.sh
```

## Implementation Details

### Components Modified

1. **OrganizationToolAdapter** (`pkg/adapters/organization_tool_adapter.go`)
   - Added `RefreshOrganizationTool` method
   - Syncs provider state with tool template
   - Clears provider cache after refresh

2. **EnhancedToolRegistry** (`pkg/services/enhanced_tool_registry.go`)
   - Added `RefreshOrganizationTool` method
   - Creates updated template from current provider state
   - Updates both template and organization tool records

3. **EnhancedToolsAPI** (`apps/rest-api/internal/api/enhanced_tools_api.go`)
   - Modified `UpdateOrganizationTool` to handle refresh action
   - Added `RefreshOrganizationTool` handler

### Database Impact

The refresh updates:
- `tool_templates` table: Updates `operation_mappings`, `ai_definitions`, and `updated_at`
- `organization_tools` table: Updates `updated_at` timestamp

### Cache Management

After a refresh:
- Provider cache is cleared for the affected provider
- Tenant tool name cache is cleared
- Operation cache remains valid (will be updated on next use)

## Example Use Case: GitHub Provider

### Scenario
You registered GitHub to your organization when only 5 toolsets were enabled (resulting in 51 tools). Later, you enabled all 11 toolsets in the provider configuration (now 114 tools available).

### Before Refresh
```bash
# Only 51 tools visible
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8081/api/v1/organizations/$ORG_ID/tools" | jq '.tools | length'
# Output: 51
```

### Perform Refresh
```bash
./scripts/refresh-organization-tool.sh $ORG_ID $GITHUB_TOOL_ID
# Output: ✅ Tool refreshed successfully!
```

### After Refresh
```bash
# All 114 tools now visible
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8081/api/v1/organizations/$ORG_ID/tools" | jq '.tools | length'
# Output: 114
```

## Best Practices

1. **Refresh After Provider Changes**: Always refresh organization tools after modifying provider configurations
2. **Monitor Operation Count**: Check the operation count in logs to verify the refresh worked
3. **Test in Staging**: Test refresh operations in staging before production
4. **Audit Trail**: Refresh operations are logged for audit purposes

## Limitations

- Refresh preserves existing credentials and configuration
- Custom mappings and disabled operations are preserved
- Rate limits and other customizations remain unchanged
- Only updates operations; doesn't change authentication or base configuration

## Future Enhancements

Potential improvements for this feature:
1. Automatic refresh detection when provider changes
2. Bulk refresh for all tools using a provider
3. Webhook notifications when new operations become available
4. UI for viewing and triggering refreshes
5. Rollback capability to previous template versions