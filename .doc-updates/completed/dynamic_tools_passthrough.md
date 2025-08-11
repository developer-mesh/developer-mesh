<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:28:14
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Dynamic Tools Passthrough Authentication

## Overview

Passthrough authentication allows users to authenticate with dynamic tools using their own personal access tokens instead of shared service accounts. This ensures that:

- Actions are performed with the user's permissions
- Audit trails accurately reflect who performed each action
- Users can only access resources they have permissions for
- No elevation of privileges through service accounts

## Architecture

The passthrough authentication system consists of several components:

### 1. Provider Mapping

Each tool is associated with a provider (e.g., GitHub, GitLab, Bitbucket) that determines which user tokens can be used:

```json
{
  "provider": "github",  // Maps to X-Token-Provider header
  "passthrough_config": {
    "mode": "optional",
    "fallback_to_service": true
  }
}
```

### 2. Token Flow

```
User Request → API Gateway → MCP Server → Dynamic Tool → External API
     ↓              ↓            ↓            ↓             ↓
X-User-Token   Validates    Extracts     Uses User    Authenticated
X-Token-Provider  Token     Credentials   Token        API Call
```

### 3. Authentication Middleware

The `GinMiddlewareWithPassthrough` extracts user tokens from requests:

```go
// Headers used for passthrough
X-User-Token: <user's personal access token>
X-Token-Provider: <provider name>
```

## Configuration

### Tool Configuration

When creating a tool, configure passthrough behavior:

```json
{
  "name": "my-github-tool",
  "provider": "github",
  "passthrough_config": {
    "mode": "optional",         // optional, required, disabled
    "fallback_to_service": true // Allow fallback to service account
  }
}
```

### Passthrough Modes

#### Optional Mode (Default)
- User tokens are used if provided
- Falls back to service account if no user token
- Best for tools where some users may not have credentials

```json
{
  "mode": "optional",
  "fallback_to_service": true
}
```

#### Required Mode
- User tokens are mandatory
- Requests without user tokens are rejected
- Use for sensitive tools requiring user attribution

```json
{
  "mode": "required",
  "fallback_to_service": false
}
```

#### Disabled Mode
- Only service account credentials are used
- User tokens are ignored even if provided
- Use for internal tools or when user tokens aren't supported

```json
{
  "mode": "disabled"
}
```

## Usage Examples

### Basic Usage

```bash
# Execute action with user token
curl -X POST http://localhost:8080/api/v1/tools/{toolId}/execute/create_issue \
  -H "Authorization: Bearer $GATEWAY_API_KEY" \
  -H "X-User-Token: ghp_usertoken123" \
  -H "X-Token-Provider: github" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "owner": "myorg",
      "repo": "myrepo",
      "title": "New Issue"
    }
  }'
```

### Client SDK Usage

```javascript
// JavaScript/TypeScript
const response = await mcpClient.executeTool({
  toolId: 'github-tool-id',
  action: 'create_issue',
  parameters: {
    owner: 'myorg',
    repo: 'myrepo',
    title: 'New Issue'
  },
  headers: {
    'X-User-Token': process.env.GITHUB_TOKEN,
    'X-Token-Provider': 'github'
  }
});
```

```python
# Python
response = mcp_client.execute_tool(
    tool_id='github-tool-id',
    action='create_issue',
    parameters={
        'owner': 'myorg',
        'repo': 'myrepo',
        'title': 'New Issue'
    },
    headers={
        'X-User-Token': os.environ['GITHUB_TOKEN'],
        'X-Token-Provider': 'github'
    }
)
```

## Provider Types

### Built-in Providers

1. **github**: GitHub and GitHub Enterprise
2. **gitlab**: GitLab and GitLab self-hosted
3. **bitbucket**: Bitbucket Cloud and Server
4. **custom**: Any other provider

### Provider Auto-Detection

If no provider is specified, the system attempts to detect it from the base URL:

```
https://api.github.com → github
https://gitlab.com → gitlab
https://api.bitbucket.org → bitbucket
https://other.com → custom
```

## Security Considerations

### Token Handling

1. **No Storage**: User tokens are never stored in the database
2. **Request Scope**: Tokens are only used for the specific request
3. **Memory Cleanup**: Tokens are cleared from memory after use
4. **No Logging**: Tokens are never logged or included in audit trails

### Provider Validation

The system validates that the token provider matches the tool's configured provider:

```
Tool Provider: github
User Token Provider: gitlab
Result: 403 Forbidden - Provider mismatch
```

### Audit Logging

All executions are logged with authentication method:

```json
{
  "event_type": "tool_execution",
  "tool_id": "github-123",
  "action": "create_issue",
  "auth_method": "passthrough",  // or "service_account"
  "user_id": "user-456",
  "success": true
}
```

## Error Handling

### Common Errors

#### Missing Required Token
```json
{
  "error": "passthrough token required",
  "details": "this tool requires user authentication via X-User-Token header"
}
```

#### Provider Mismatch
```json
{
  "error": "provider mismatch",
  "details": "tool requires github credentials but gitlab token provided"
}
```

#### Invalid Token
```json
{
  "error": "authentication failed",
  "details": "invalid or expired user token"
}
```

## Best Practices

### For Administrators

1. **Choose Appropriate Modes**: Use "required" for sensitive tools, "optional" for general tools
2. **Configure Providers**: Always specify the provider for better validation
3. **Monitor Usage**: Review audit logs to track authentication methods
4. **Document Requirements**: Clearly document which tools require user tokens

### For Developers

1. **Handle Errors**: Implement proper error handling for authentication failures
2. **Token Security**: Never log or store user tokens
3. **Provide Feedback**: Give clear error messages when authentication fails
4. **Test Both Paths**: Test with both user tokens and service accounts

### For Users

1. **Token Management**: Keep personal access tokens secure
2. **Correct Provider**: Ensure token provider matches the tool
3. **Token Permissions**: Ensure tokens have necessary permissions
4. **Token Rotation**: Regularly rotate personal access tokens

## Migration Guide

### From Legacy Tools

If migrating from the legacy GitHub tools that support passthrough:

1. **Create Dynamic Tool**: Configure with same provider
2. **Enable Passthrough**: Set mode to "optional" or "required"
3. **Update Client Code**: No changes needed if using same headers
4. **Test Migration**: Verify both authentication methods work

### Adding Passthrough to Existing Tools

1. **Database Migration**: Already applied with schema update
2. **Update Tool Config**: Set provider and passthrough_config
3. **Test Thoroughly**: Verify existing integrations still work
4. **Communicate Changes**: Notify users about new authentication option

## Troubleshooting

### Token Not Being Used

1. Check that headers are being sent correctly
2. Verify provider matches between token and tool
3. Ensure passthrough mode is not "disabled"
4. Check audit logs for authentication method used

### Authentication Failures

1. Verify token is valid and not expired
2. Check token has necessary permissions
3. Ensure provider configuration is correct
4. Review error messages for specific issues

### Fallback Not Working

1. Verify fallback_to_service is true
2. Check service account credentials are valid
3. Ensure passthrough mode is "optional"
4. Review logs for fallback attempts

## Metrics and Monitoring

The system tracks authentication methods in metrics:

```
dynamic_tools_executions{auth_method="passthrough"} 
dynamic_tools_executions{auth_method="service_account"}
```

Use these metrics to:
- Track adoption of user tokens
- Identify tools still using service accounts
- Monitor authentication failures
