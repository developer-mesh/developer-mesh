<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:38:04
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# IDE Pass-Through Authentication Example

This example demonstrates how IDE plugins can use the pass-through authentication feature to allow users to provide their own Personal Access Tokens (PATs) for backend tools.

## IDE Plugin Configuration

```typescript
// IDE Plugin settings (e.g., VS Code settings.json)
{
  "devopsMcp": {
    "serverUrl": "https://mcp.company.com/api/v1",
    "apiKey": "mcp_1234567890abcdef", // MCP API key for authentication to MCP server
    
    // User's personal access tokens for backend tools
    "toolCredentials": {
      "github": {
        "token": "ghp_yourGitHubPersonalAccessToken",
        "type": "pat"
      },
      "jira": {
        "token": "your-jira-api-token",
        "baseUrl": "https://company.atlassian.net"
      },
      "sonarqube": {
        "token": "sqp_yourSonarQubeToken",
        "baseUrl": "https://sonarqube.company.com"
      },
      "artifactory": {
        "token": "AKCp_yourArtifactoryToken",
        "baseUrl": "https://artifactory.company.com"
      }
    },
    
    // How to handle credentials
    "credentialStorage": "local", // "none" | "local" | "server"
    "secureMode": true
  }
}
```

## Making Authenticated Requests

### TypeScript/JavaScript Example

```typescript
import { MCPClient } from '@developer-mesh/ide-client';

// Initialize client with configuration
const client = new MCPClient({
  serverUrl: 'https://mcp.company.com/api/v1',
  apiKey: process.env.MCP_API_KEY,
  toolCredentials: {
    github: {
      token: process.env.GITHUB_PAT,
      type: 'pat'
    },
    jira: {
      token: process.env.JIRA_TOKEN,
      baseUrl: 'https://company.atlassian.net'
    }
  }
});

// Example 1: Create a GitHub issue with user's PAT
async function createGitHubIssue() {
  try {
    const result = await client.callTool('github', 'create_issue', {
      owner: 'myorg',
      repo: 'myrepo',
      title: 'Bug: Login button not working',
      body: 'The login button is unresponsive on mobile devices.',
      labels: ['bug', 'mobile']
    });
    
    console.log(`Created issue #${result.number}`);
  } catch (error) {
    if (error.code === 'AUTH_FAILED') {
      console.error('GitHub authentication failed. Please check your PAT.');
    }
  }
}

// Example 2: Query Jira with user's credentials
async function getMyJiraIssues() {
  const result = await client.callTool('jira', 'search_issues', {
    jql: 'assignee = currentUser() AND status != Done',
    fields: ['summary', 'status', 'priority']
  });
  
  console.log(`Found ${result.issues.length} open issues assigned to you`);
}

// Example 3: Check SonarQube quality gate
async function checkCodeQuality(projectKey: string) {
  const result = await client.callTool('sonarqube', 'get_project_status', {
    projectKey: projectKey
  });
  
  if (result.projectStatus.status === 'ERROR') {
    console.error('Quality gate failed!');
  }
}
```

## Request Format

Here's what the actual HTTP request looks like:

```http
POST https://mcp.company.com/api/v1/tools/github/actions/create_issue
Authorization: Bearer mcp_1234567890abcdef
Content-Type: application/json

{
  "action": "create_issue",
  "parameters": {
    "owner": "myorg",
    "repo": "myrepo",
    "title": "Bug: Login button not working",
    "body": "The login button is unresponsive on mobile devices."
  },
  "credentials": {
    "github": {
      "token": "ghp_yourGitHubPersonalAccessToken",
      "type": "pat"
    }
  }
}
```

## Response Handling

```typescript
// Success response
{
  "id": 12345,
  "number": 42,
  "title": "Bug: Login button not working",
  "html_url": "https://github.com/myorg/myrepo/issues/42",
  "created_at": "2024-01-15T10:30:00Z"
}

// Authentication failure
{
  "error": "GitHub authentication failed",
  "details": "Bad credentials",
  "code": "AUTH_FAILED"
}
```

## Security Best Practices

### 1. Never Log Credentials

```typescript
// BAD - Don't do this
console.log(`Making request with token: ${credentials.github.token}`);

// GOOD - Log sanitized information
console.log(`Making request with GitHub credentials (***${credentials.github.token.slice(-4)})`);
```

### 2. Secure Storage in IDE

```typescript
// Use IDE's secure storage API
async function storeCredentials(tool: string, token: string) {
  // VS Code example
  await context.secrets.store(`mcp.${tool}.token`, token);
}

async function getCredentials(tool: string): Promise<string | undefined> {
  return await context.secrets.get(`mcp.${tool}.token`);
}
```

### 3. Validate Credentials

```typescript
// Test credentials before storing
async function validateGitHubToken(token: string): Promise<boolean> {
  try {
    const result = await client.callTool('github', 'get_authenticated_user', {}, {
      github: { token }
    });
    return true;
  } catch (error) {
    return false;
  }
}
```

## IDE UI Integration

### Settings UI Example

```typescript
// Register configuration UI in VS Code
vscode.commands.registerCommand('devopsMcp.configureCredentials', async () => {
  const tools = ['github', 'jira', 'sonarqube', 'artifactory'];
  
  for (const tool of tools) {
    const token = await vscode.window.showInputBox({
      prompt: `Enter your ${tool} token`,
      password: true,
      placeHolder: 'Token will be stored securely'
    });
    
    if (token) {
      // Validate token
      const isValid = await validateToolToken(tool, token);
      if (isValid) {
        await storeCredentials(tool, token);
        vscode.window.showInformationMessage(`${tool} credentials saved`);
      } else {
        vscode.window.showErrorMessage(`Invalid ${tool} token`);
      }
    }
  }
});
```

### Status Bar Integration

```typescript
// Show authentication status in status bar
const statusBarItem = vscode.window.createStatusBarItem();

async function updateAuthStatus() {
  const hasGitHub = await hasValidCredentials('github');
  const hasJira = await hasValidCredentials('jira');
  
  if (hasGitHub && hasJira) {
    statusBarItem.text = '$(check) MCP: Authenticated';
    statusBarItem.color = 'green';
  } else {
    statusBarItem.text = '$(warning) MCP: Missing Credentials';
    statusBarItem.color = 'yellow';
  }
  
  statusBarItem.show();
}
```

## Fallback Behavior

When user credentials are not provided, the system can fall back to service accounts:

```typescript
// Server-side configuration
{
  "auth": {
    "passthrough": {
      "enabled": true,
      "fallback_to_service_account": true,
      "service_accounts": {
        "github": true,
        "jira": false,  // No service account for Jira
        "sonarqube": true
      }
    }
  }
}

// Client behavior
async function callToolWithFallback(tool: string, action: string, params: any) {
  try {
    // First try with user credentials
    return await client.callTool(tool, action, params);
  } catch (error) {
    if (error.code === 'NO_CREDENTIALS' && error.service_account_available) {
      // Inform user that service account will be used
      console.log(`Using service account for ${tool}`);
      return await client.callTool(tool, action, params, { use_service_account: true });
    }
    throw error;
  }
}
```

## Benefits of Pass-Through Authentication

1. **User Permissions**: Actions happen with the user's actual permissions
2. **Audit Trail**: Backend tools see the real user, not a service account
3. **No Shared Secrets**: Each user manages their own credentials
4. **Compliance**: Meets security requirements for credential management
5. **Flexibility**: Users can use different credentials for different environments

## Troubleshooting

### Common Issues

1. **401 Unauthorized**
   - Check if the token is valid
   - Ensure the token has required scopes/permissions
   - Verify the token hasn't expired

2. **Rate Limiting**
   - Each user's rate limits apply to their tokens
   - Service account fallback may have different limits

3. **Token Rotation**
   - Implement token refresh reminders
   - Support multiple tokens for rotation

### Debug Mode

```typescript
// Enable debug logging
const client = new MCPClient({
  // ... config ...
  debug: true,
  onDebugLog: (level, message, data) => {
    // Custom debug handling
    if (level === 'error' && data.tool === 'github') {
      console.error('GitHub API Error:', data);
    }
  }
});
```

