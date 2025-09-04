# Claude Code Configuration with Pass-Through Authentication

## Prerequisites

1. DevMesh account with organization API key (obtained during registration)
2. Edge MCP installed and in PATH
3. Your personal access tokens for services you want to use (GitHub, AWS, etc.)

## Configuration

### Method 1: Using Claude Code CLI (Recommended)

The easiest way to add the DevMesh MCP server is using the Claude Code CLI:

```bash
# Remove any existing devmesh configuration
claude mcp remove devmesh

# Add the DevMesh MCP server with your credentials
claude mcp add devmesh \
  --env DEV_MESH_URL=http://localhost:8081 \
  --env DEV_MESH_API_KEY=devmesh_YOUR_API_KEY_HERE \
  --env GITHUB_TOKEN=ghp_YOUR_GITHUB_TOKEN_HERE \
  -- edge-mcp --stdio
```

Replace the placeholders:
- `YOUR_API_KEY_HERE`: Your DevMesh organization API key (starts with `devmesh_`)
- `YOUR_GITHUB_TOKEN_HERE`: Your GitHub personal access token (optional)

For production environments, use:
- `DEV_MESH_URL=https://api.devmesh.io`

### Method 2: Manual Configuration (Alternative)

If you prefer manual configuration, the Claude Code CLI command above will create the following configuration in your project's `.claude.json` file:

```json
{
  "mcpServers": {
    "devmesh": {
      "type": "stdio",
      "command": "edge-mcp",
      "args": ["--stdio"],
      "env": {
        "DEV_MESH_URL": "http://localhost:8081",
        "DEV_MESH_API_KEY": "devmesh_YOUR_API_KEY_HERE",
        "GITHUB_TOKEN": "ghp_YOUR_GITHUB_TOKEN_HERE"
      }
    }
  }
}
```

**Note**: Edge MCP supports two modes:
- **Stdio mode** (default for Claude Code): Use `--stdio` flag
- **WebSocket mode**: Use `--port 8082` for WebSocket server on specified port

## Verifying the Connection

After adding the server, verify it's connected:

```bash
# List all MCP servers and their status
claude mcp list

# You should see:
# devmesh: edge-mcp --stdio - ✓ Connected
```

If you see "✗ Failed to connect", check:
1. Edge MCP is installed: `which edge-mcp`
2. DevMesh services are running: `docker-compose -f docker-compose.local.yml ps`
3. API endpoint is accessible: `curl http://localhost:8081/health`

## Environment Setup

Set these environment variables before starting Claude Code:

### Required: DevMesh Platform Credentials
```bash
export DEV_MESH_URL="https://api.devmesh.io"
export DEV_MESH_API_KEY="devmesh_xxx..."  # Your API key from organization registration
```

**Note**: Your organization's tenant ID is automatically determined from your API key. You no longer need to provide it separately.

### Optional but Recommended: Personal Access Tokens

These tokens allow Edge MCP to perform actions as YOU, not as a service account:

```bash
# GitHub Personal Access Token
export GITHUB_TOKEN="ghp_your_personal_access_token"

# AWS Credentials (for your personal AWS account)
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_REGION="us-east-1"  # Optional

# Other service tokens (as needed)
export SLACK_TOKEN="xoxb-your-slack-token"
export JIRA_TOKEN="your-jira-api-token"
export GITLAB_TOKEN="glpat-your-gitlab-token"
```

## Getting Your API Key

If you haven't registered your organization yet:

1. Register your organization:
```bash
curl -X POST https://api.devmesh.io/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Your Company",
    "organization_slug": "your-company",
    "admin_email": "admin@company.com",
    "admin_name": "Your Name",
    "admin_password": "SecurePass123"
  }'
```

2. Save the `api_key` from the response - this is your `DEV_MESH_API_KEY`

If you already have an account, you can find your API key in the DevMesh dashboard or contact your organization admin.

## How Pass-Through Authentication Works

1. **Without Personal Tokens**: Actions are performed using DevMesh's service credentials
   - PRs created by "DevMesh Bot"
   - AWS resources created by service account
   - Less personal attribution

2. **With Personal Tokens**: Actions are performed as YOU
   - PRs created by your GitHub account
   - AWS resources created in your account
   - Full audit trail with your identity
   - Respects your personal permissions/limits

## Verification

After restarting Claude Code:

1. Check Edge MCP logs for "Extracted passthrough authentication"
2. Test with a GitHub command - the PR should be created as you
3. Verify in GitHub/AWS/etc. that actions show your username

## Security Notes

- Your API key identifies your organization and provides access to your tenant's resources
- Personal tokens are only held in memory during your session
- Tokens are never logged or stored persistently
- Tokens are only sent to DevMesh Platform over TLS
- Each action is logged with full attribution

## Available Tools

Tools are dynamically discovered from your organization's DevMesh configuration. With pass-through auth, you can use:

- **GitHub**: Create PRs, issues, and releases as yourself
- **AWS**: Manage resources in your personal or work AWS account
- **Slack**: Send messages as yourself (not as a bot)
- **Jira**: Create and update issues with your identity
- Plus any custom tools configured in your organization

## Troubleshooting

### "Actions still showing as bot/service account"
- Verify your personal token is set: `echo $GITHUB_TOKEN`
- Check Edge MCP logs for "Found GitHub passthrough token"
- Ensure token has required scopes (repo, write, etc.)

### "Authentication failed with Core Platform"
- Verify your API key is correct: `echo $DEV_MESH_API_KEY`
- Ensure your API key starts with `devmesh_`
- Check that your organization account is active
- Your API key automatically identifies your organization - no tenant ID needed

### "Tool execution failed"
- Check if your personal account has access to the resource
- Verify you're not hitting personal rate limits
- Some tools may not support pass-through auth yet

## Token Management Best Practices

1. **API Key Security**: 
   - Never share your DevMesh API key
   - Store it securely (use environment variables or secret manager)
   - Each team member should have their own user account

2. **Personal Tokens**:
   - Use minimal scopes - only grant permissions you need
   - Rotate regularly - update tokens every 30-90 days
   - Use separate tokens - don't reuse tokens across services
   - Store securely - use a password manager or secure keychain
   - Monitor usage - check your GitHub/AWS audit logs regularly

## Example: Creating a PR as Yourself

```json
// When Edge MCP executes this tool:
{
  "tool": "github.create_pr",
  "arguments": {
    "repo": "myorg/myrepo",
    "title": "Fix: Update documentation",
    "body": "This PR updates the README"
  }
}

// With GITHUB_TOKEN set, the PR will show:
// - Author: Your GitHub username
// - Signed commits (if GPG configured)
// - Your profile picture
// - Counts toward your contribution graph
```

## Backward Compatibility Note

If you're using an older version of Edge MCP that still requires `TENANT_ID`, you can leave it empty:
```bash
export TENANT_ID=""  # Not needed with organization API keys
```

The tenant ID is now automatically determined from your API key when Edge MCP authenticates with the Core Platform.