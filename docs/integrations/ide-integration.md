<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:37:11
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# IDE Integration with Developer Mesh

This guide demonstrates how modern IDEs like Windsurf and Cursor can integrate with the Developer Mesh platform to enhance developer productivity by providing seamless access to DevOps tools directly from the coding environment.

## Overview

Modern AI-powered IDEs can leverage the Developer Mesh protocol to:

1. Perform DevOps operations without leaving the editor
2. Get contextual information about repositories, CI/CD pipelines, and deployments
3. Enable AI assistants to suggest code with DevOps context
4. Automate workflows based on code changes

## Integration Architecture

There are two primary approaches to IDE integration:

1. **Extension/Plugin Architecture**: Creating dedicated extensions for IDEs like VS Code, JetBrains IDEs, etc.
2. **Native Integration**: Building MCP client support directly into IDEs like Windsurf and Cursor

## IDE Extension Example

Here's how Windsurf or Cursor could implement an MCP integration:

```javascript
// IDE extension for Developer Mesh integration
class MCPIntegration {
  constructor(apiKey, mcpUrl) {
    this.apiKey = apiKey;
    this.mcpUrl = mcpUrl || 'http://localhost:8080/api/v1';
    this.headers = {
      'Authorization': `Bearer ${this.apiKey}`,
      'Content-Type': 'application/json'
    };
  }

  // Connect to the MCP server and verify credentials
  async initialize() {
    try {
      const response = await fetch(`${this.mcpUrl}/status`, {
        method: 'GET',
        headers: this.headers
      });
      
      if (!response.ok) {
        throw new Error(`Failed to initialize MCP connection: ${response.statusText}`);
      }
      
      const data = await response.json();
      console.log('MCP connection established successfully:', data.version);
      return true;
    } catch (error) {
      console.error('MCP initialization failed:', error);
      return false;
    }
  }

  // Call any MCP tool with parameters
  async callTool(toolName, action, parameters) {
    try {
      const response = await fetch(`${this.mcpUrl}/tools/${toolName}/actions/${action}`, {
        method: 'POST',
        headers: this.headers,
        body: JSON.stringify(parameters)
      });
      
      if (!response.ok) {
        throw new Error(`Tool call failed: ${response.statusText}`);
      }
      
      return await response.json();
    } catch (error) {
      console.error(`Error calling ${toolName}/${action}:`, error);
      throw error;
    }
  }
  
  // Get contextual information about the current repository
  async getRepositoryContext(repoPath) {
    // Extract owner/repo from git remote URL
    const gitInfo = await this.extractGitInfo(repoPath);
    if (!gitInfo) return null;
    
    try {
      return await this.callTool('github', 'get_repository', {
        owner: gitInfo.owner,
        repo: gitInfo.repo
      });
    } catch (error) {
      console.error('Failed to get repository context:', error);
      return null;
    }
  }
  
  // Get CI/CD status for the current branch
  async getCIStatus(repoPath, branch) {
    const gitInfo = await this.extractGitInfo(repoPath);
    if (!gitInfo) return null;
    
    try {
      return await this.callTool('github', 'list_workflow_runs', {
        owner: gitInfo.owner,
        repo: gitInfo.repo,
        branch: branch || 'main'
      });
    } catch (error) {
      console.error('Failed to get CI status:', error);
      return null;
    }
  }
  
  // Helper to extract git info from repository
  async extractGitInfo(repoPath) {
    // Implementation to extract owner/repo from git remote
    // This would use the IDE's built-in git functionality
    return { owner: 'example-owner', repo: 'example-repo' };
  }
}

// Usage in IDE extension
async function activateExtension(context) {
  // Get API key from IDE settings
  const config = getConfiguration('devopsMcp');
  const apiKey = config.get('apiKey');
  const mcpUrl = config.get('serverUrl');
  
  if (!apiKey) {
    showNotification('Developer Mesh API key not configured');
    return;
  }
  
  // Initialize the MCP client
  const mcpClient = new MCPIntegration(apiKey, mcpUrl);
  const initialized = await mcpClient.initialize();
  
  if (!initialized) {
    showNotification('Failed to connect to Developer Mesh server');
    return;
  }
  
  // Register commands in the IDE
  registerCommand('devopsMcp.createIssue', createIssueHandler(mcpClient));
  registerCommand('devopsMcp.reviewPR', reviewPRHandler(mcpClient));
  registerCommand('devopsMcp.deployCode', deployCodeHandler(mcpClient));
  
  // Add status bar items
  const statusBarItem = createStatusBarItem();
  statusBarItem.text = 'Developer Mesh: Connected';
  statusBarItem.show();
  
  // Set up event handlers
  onDidSaveTextDocument(async (document) => {
    // Update vector embeddings for the document
    await updateCodeEmbedding(mcpClient, document);
  });
}
```

## AI-Assistant Integration Use Case

Here's how an AI coding assistant in an IDE like Windsurf or Cursor could leverage MCP:

```javascript
class AIAssistantMCPIntegration {
  constructor(mcpClient) {
    this.mcpClient = mcpClient;
  }
  
  // Get relevant context before generating code
  async getDevOpsContext(codeSnippet, filepath) {
    // Current repository context
    const repoContext = await this.mcpClient.getRepositoryContext(getWorkspaceFolder());
    
    // Related issues/PRs based on the code content
    const relatedIssues = await this.searchRelatedIssues(codeSnippet);
    
    // CI/CD pipeline configuration
    const ciConfig = await this.getCIConfiguration();
    
    // Deployment history
    const deployments = await this.getRecentDeployments();
    
    return {
      repository: repoContext,
      issues: relatedIssues,
      ciConfig: ciConfig,
      deployments: deployments
    };
  }
  
  // Use vector embeddings to find related issues
  async searchRelatedIssues(codeSnippet) {
    try {
      // Create embedding for the code snippet
      const embedding = await this.generateEmbedding(codeSnippet);
      
      // Search for similar content using vector search
      return await this.mcpClient.callTool('vector', 'search_embeddings', {
        embedding: embedding,
        limit: 5,
        min_similarity: 0.7
      });
    } catch (error) {
      console.error('Failed to find related issues:', error);
      return [];
    }
  }
  
  // Generate code that integrates with DevOps workflows
  async generateDevOpsAwareCode(prompt, filepath, currentCode) {
    // Get DevOps context
    const context = await this.getDevOpsContext(currentCode, filepath);
    
    // Enhanced prompt with DevOps context
    const enhancedPrompt = {
      prompt: prompt,
      filepath: filepath,
      currentCode: currentCode,
      devOpsContext: context
    };
    
    // Call AI code generation with enhanced context
    return await this.generateCode(enhancedPrompt);
  }
  
  // Helper methods
  async generateEmbedding(text) {
    // Implementation to generate vector embedding
  }
  
  async generateCode(enhancedPrompt) {
    // Implementation to generate code with AI
  }
  
  async getCIConfiguration() {
    // Get CI configuration from repository
  }
  
  async getRecentDeployments() {
    // Get recent deployment history
  }
}
```

## UI Components for IDE Integration

An MCP-integrated IDE could provide these UI components:

1. **DevOps Panel**: A dedicated panel showing repository info, CI/CD status, and recent deployments
2. **Contextual Actions**: Right-click menu options to create issues, branches, or PRs based on code selection
3. **Status Indicators**: Inline status indicators showing deployment status, test coverage, or security scans
4. **Chat Interface**: AI assistant chat with DevOps context awareness

## Implementation Example: VS Code Extension

Here's a simplified example of how to implement this as a VS Code extension:

```typescript
// vscode-mcp-extension.ts
import * as vscode from 'vscode';
import { MCPIntegration } from './mcp-integration';

export function activate(context: vscode.ExtensionContext) {
  const apiKey = vscode.workspace.getConfiguration('devopsMcp').get<string>('apiKey');
  const mcpUrl = vscode.workspace.getConfiguration('devopsMcp').get<string>('serverUrl');
  
  const mcpClient = new MCPIntegration(apiKey, mcpUrl);
  
  // Register DevOps Panel WebView
  context.subscriptions.push(
    vscode.commands.registerCommand('devopsMcp.openPanel', () => {
      const panel = vscode.window.createWebviewPanel(
        'devopsMcpPanel',
        'Developer Mesh',
        vscode.ViewColumn.Beside,
        {}
      );
      
      updatePanelContent(panel, mcpClient);
    })
  );
  
  // Register right-click menu commands
  context.subscriptions.push(
    vscode.commands.registerCommand('devopsMcp.createIssueFromSelection', async () => {
      const editor = vscode.window.activeTextEditor;
      if (editor) {
        const selection = editor.document.getText(editor.selection);
        const title = await vscode.window.showInputBox({ prompt: 'Issue Title' });
        
        if (title) {
          try {
            const repoInfo = await mcpClient.extractGitInfo(editor.document.uri.fsPath);
            const issue = await mcpClient.callTool('github', 'create_issue', {
              owner: repoInfo.owner,
              repo: repoInfo.repo,
              title: title,
              body: `Code Reference:\n\`\`\`\n${selection}\n\`\`\``
            });
            
            vscode.window.showInformationMessage(`Issue #${issue.number} created`);
          } catch (error) {
            vscode.window.showErrorMessage(`Failed to create issue: ${error.message}`);
          }
        }
      }
    })
  );
  
  // Status bar integration
  const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
  statusBarItem.text = '$(sync) MCP';
  statusBarItem.command = 'devopsMcp.openPanel';
  statusBarItem.show();
  context.subscriptions.push(statusBarItem);
  
  // Initialize and show connection status
  mcpClient.initialize().then(success => {
    if (success) {
      statusBarItem.text = '$(check) MCP';
      vscode.window.showInformationMessage('Connected to Developer Mesh server');
    } else {
      statusBarItem.text = '$(error) MCP';
      vscode.window.showErrorMessage('Failed to connect to Developer Mesh server');
    }
  });
}

async function updatePanelContent(panel, mcpClient) {
  try {
    const activeDocumentPath = vscode.window.activeTextEditor?.document.uri.fsPath;
    if (!activeDocumentPath) return;
    
    const repoInfo = await mcpClient.extractGitInfo(activeDocumentPath);
    const [repoData, ciStatus, issues] = await Promise.all([
      mcpClient.getRepositoryContext(activeDocumentPath),
      mcpClient.getCIStatus(activeDocumentPath, 'main'),
      mcpClient.callTool('github', 'list_issues', { 
        owner: repoInfo.owner, 
        repo: repoInfo.repo,
        state: 'open',
        limit: 5 
      })
    ]);
    
    panel.webview.html = generatePanelHtml(repoData, ciStatus, issues);
  } catch (error) {
    panel.webview.html = `<h1>Error loading DevOps data</h1><p>${error.message}</p>`;
  }
}

function generatePanelHtml(repoData, ciStatus, issues) {
  // Generate HTML for the webview panel
  // ...
}
```

## Cursor/Windsurf Native Integration

For AI-native IDEs like Cursor or Windsurf, a deeper integration pattern can be implemented:

1. **Contextualized AI Coding**:
   - Provide DevOps context (issues, PRs, CI status) to the AI model during code generation
   - Automatically reference relevant issue numbers in commit messages
   - Suggest tests based on CI requirements

2. **Workflow Automation**:
   - Automatically create branches for new features based on issue assignments
   - Trigger PR creation after a series of commits
   - Run pre-commit hooks via MCP for linting and security scanning

3. **Knowledge Base Integration**:
   - Use vector search to find similar code reviews and issues
   - Provide relevant documentation during coding
   - Build organizational knowledge from code and issue history

## Example Backend: Cursor IDE Integration

Here's a conceptual example of how Cursor IDE could integrate with MCP:

```typescript
// cursor-mcp-integration.ts

interface CodeGenerationRequest {
  prompt: string;
  filepath: string;
  contextLines: string[];
  fileTree: string[];
}

interface DevOpsContext {
  repositoryInfo: any;
  relatedIssues: any[];
  buildStatus: any;
  deploymentHistory: any[];
  codeReviewComments: any[];
  testResults: any;
}

class CursorMCPIntegration {
  private mcpClient: MCPIntegration;
  
  constructor(apiKey: string, mcpUrl: string) {
    this.mcpClient = new MCPIntegration(apiKey, mcpUrl);
  }
  
  async initialize(): Promise<boolean> {
    return await this.mcpClient.initialize();
  }
  
  // Enhance code generation with DevOps context
  async enhanceCodeGeneration(request: CodeGenerationRequest): Promise<CodeGenerationRequest> {
    const devOpsContext = await this.getDevOpsContext(request.filepath, request.contextLines.join('\n'));
    
    // Add DevOps context to the prompt
    const enhancedPrompt = this.buildEnhancedPrompt(request.prompt, devOpsContext);
    
    return {
      ...request,
      prompt: enhancedPrompt
    };
  }
  
  // Get all relevant DevOps context for the current coding task
  private async getDevOpsContext(filepath: string, context: string): Promise<DevOpsContext> {
    const gitInfo = await this.mcpClient.extractGitInfo(filepath);
    
    // Parallel fetch of all relevant context
    const [
      repositoryInfo,
      relatedIssues,
      buildStatus,
      deploymentHistory,
      codeReviewComments,
      testResults
    ] = await Promise.all([
      this.mcpClient.getRepositoryContext(filepath),
      this.findRelatedIssues(context, gitInfo),
      this.getLatestBuildStatus(gitInfo),
      this.getRecentDeployments(gitInfo),
      this.getRelevantCodeReviews(context, gitInfo),
      this.getTestResults(filepath, gitInfo)
    ]);
    
    return {
      repositoryInfo,
      relatedIssues,
      buildStatus,
      deploymentHistory,
      codeReviewComments,
      testResults
    };
  }
  
  // Find issues related to the current code using vector search
  private async findRelatedIssues(codeContext: string, gitInfo: any): Promise<any[]> {
    try {
      // Generate embedding for the code
      const embedding = await this.generateEmbedding(codeContext);
      
      // Search for related issues using vector similarity
      return await this.mcpClient.callTool('vector', 'search_embeddings', {
        embedding: embedding,
        metadata: {
          repository: `${gitInfo.owner}/${gitInfo.repo}`,
          type: 'issue'
        },
        limit: 5
      });
    } catch (error) {
      console.error('Failed to find related issues:', error);
      return [];
    }
  }
  
  // Build enhanced prompt with DevOps context
  private buildEnhancedPrompt(originalPrompt: string, context: DevOpsContext): string {
    let enhancedPrompt = originalPrompt;
    
    // Add repository information
    enhancedPrompt += `\n\nRepository: ${context.repositoryInfo?.full_name || 'Unknown'}`;
    
    // Add related issues if available
    if (context.relatedIssues && context.relatedIssues.length > 0) {
      enhancedPrompt += '\n\nRelated Issues:';
      context.relatedIssues.forEach(issue => {
        enhancedPrompt += `\n- #${issue.number}: ${issue.title}`;
      });
    }
    
    // Add build status context
    if (context.buildStatus) {
      enhancedPrompt += `\n\nLatest Build: ${context.buildStatus.status}`;
    }
    
    // Add code review comments if available
    if (context.codeReviewComments && context.codeReviewComments.length > 0) {
      enhancedPrompt += '\n\nRelevant Code Review Comments:';
      context.codeReviewComments.slice(0, 3).forEach(comment => {
        enhancedPrompt += `\n- ${comment.body}`;
      });
    }
    
    return enhancedPrompt;
  }
  
  // Additional helper methods
  private async generateEmbedding(text: string): Promise<number[]> {
    // Implementation to generate vector embedding
    // ...
  }
  
  private async getLatestBuildStatus(gitInfo: any): Promise<any> {
    // Get latest build status
    // ...
  }
  
  private async getRecentDeployments(gitInfo: any): Promise<any[]> {
    // Get recent deployments
    // ...
  }
  
  private async getRelevantCodeReviews(codeContext: string, gitInfo: any): Promise<any[]> {
    // Get relevant code reviews
    // ...
  }
  
  private async getTestResults(filepath: string, gitInfo: any): Promise<any> {
    // Get test results
    // ...
  }
}
```

## Integration with Vector Search for Context-Aware Coding

One of the most powerful features of MCP is its vector search capability. Here's how an IDE can leverage it:

```typescript
class VectorSearchCodeContext {
  private mcpClient: MCPIntegration;
  
  constructor(mcpClient: MCPIntegration) {
    this.mcpClient = mcpClient;
  }
  
  // Store code embeddings for later reference
  async storeCodeEmbedding(filepath: string, codeBlock: string, metadata: any = {}): Promise<void> {
    try {
      const embedding = await this.generateEmbedding(codeBlock);
      
      await this.mcpClient.callTool('vector', 'store_embedding', {
        content: codeBlock,
        embedding: embedding,
        metadata: {
          ...metadata,
          filepath: filepath,
          timestamp: new Date().toISOString()
        }
      });
    } catch (error) {
      console.error('Failed to store code embedding:', error);
    }
  }
  
  // Find similar code across the codebase
  async findSimilarCode(codeSnippet: string): Promise<any[]> {
    try {
      const embedding = await this.generateEmbedding(codeSnippet);
      
      return await this.mcpClient.callTool('vector', 'search_embeddings', {
        embedding: embedding,
        metadata: {
          type: 'code'
        },
        limit: 10
      });
    } catch (error) {
      console.error('Failed to find similar code:', error);
      return [];
    }
  }
  
  // Find relevant documentation
  async findRelevantDocs(codeSnippet: string): Promise<any[]> {
    try {
      const embedding = await this.generateEmbedding(codeSnippet);
      
      return await this.mcpClient.callTool('vector', 'search_embeddings', {
        embedding: embedding,
        metadata: {
          type: 'documentation'
        },
        limit: 5
      });
    } catch (error) {
      console.error('Failed to find relevant docs:', error);
      return [];
    }
  }
  
  // Helper to generate embeddings
  private async generateEmbedding(text: string): Promise<number[]> {
    // Implementation to generate vector embedding
    // This could call an external embedding API or use MCP's embedding service
    // ...
  }
}
```

## Best Practices for IDE Integration

1. **Authentication and Security**:
   - Use secure storage for API keys
   - Support token rotation and expiration
   - Implement proper error handling for authentication failures

2. **Performance Optimization**:
   - Implement caching for repository context
   - Use debouncing for real-time updates
   - Run resource-intensive operations in background threads

3. **User Experience**:
   - Provide clear status indicators
   - Implement progressive disclosure of complex features
   - Allow customization of integration behavior

4. **Error Handling**:
   - Gracefully degrade when MCP server is unavailable
   - Provide clear error messages
   - Implement automatic retry with backoff

## Configuration Options

IDE extensions should provide these configuration options:

```json
{
  "devopsMcp.serverUrl": "http://localhost:8080/api/v1",
  "devopsMcp.apiKey": "",
  "devopsMcp.features": {
    "vectorSearch": true,
    "githubIntegration": true,
    "cicdStatus": true,
    "aiAssistant": true
  },
  "devopsMcp.refreshInterval": 60,
  "devopsMcp.logging": {
    "level": "info",
    "console": true,
    "file": false
  }
}
```

## Getting Started

To integrate your IDE with Developer Mesh:

1. **Set up the Developer Mesh server** following the [quick start guide](../getting-started/quick-start-guide.md)
2. **Generate an API key** for your IDE integration
3. **Install or build the extension** for your target IDE
4. **Configure the extension** with your MCP server URL and API key
5. **Start using the DevOps features** directly in your IDE

## Conclusion

By integrating Developer Mesh with modern IDEs like Windsurf and Cursor, developers gain seamless access to DevOps tools and contextual information without leaving their coding environment. This integration is particularly powerful for AI-assisted coding, as it enables the AI to incorporate DevOps context into its suggestions and automate routine DevOps tasks.

The adapter pattern used throughout the Developer Mesh codebase makes it especially well-suited for IDE integration, as it provides a consistent interface that abstracts away the complexities of individual DevOps tools.
