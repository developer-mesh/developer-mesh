/**
 * Edge MCP TypeScript Client Example
 *
 * This example demonstrates how to connect to Edge MCP and execute tools
 * using the WebSocket-based MCP protocol.
 *
 * Requirements:
 *   npm install ws
 *   npm install @types/ws --save-dev
 */

import WebSocket from 'ws';

// Configuration
const EDGE_MCP_URL = 'ws://localhost:8082/ws';
const API_KEY = 'dev-admin-key-1234567890';

// Types
interface MCPMessage {
  jsonrpc: '2.0';
  id?: number | string;
  method?: string;
  params?: any;
  result?: any;
  error?: MCPError;
}

interface MCPError {
  code: number;
  message: string;
  data?: any;
}

interface ToolDefinition {
  name: string;
  description: string;
  category?: string;
  tags?: string[];
  inputSchema: any;
  examples?: any[];
}

interface BatchToolCall {
  id: string;
  name: string;
  arguments: Record<string, any>;
}

interface BatchResult {
  results: Array<{
    id: string;
    index: number;
    status: 'success' | 'error';
    result?: any;
    error?: MCPError;
    duration_ms: number;
  }>;
  duration_ms: number;
  success_count: number;
  error_count: number;
  parallel: boolean;
}

/**
 * Edge MCP WebSocket Client
 */
class EdgeMCPClient {
  private ws: WebSocket | null = null;
  private requestId = 0;
  private pendingRequests = new Map<number, {
    resolve: (value: any) => void;
    reject: (error: Error) => void;
  }>();

  constructor(private url: string, private apiKey: string) {}

  /**
   * Connect to Edge MCP WebSocket
   */
  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url, {
        headers: {
          'Authorization': `Bearer ${this.apiKey}`
        }
      });

      this.ws.on('open', () => {
        console.log('Connected to Edge MCP');
        resolve();
      });

      this.ws.on('error', (error) => {
        console.error('WebSocket error:', error);
        reject(error);
      });

      this.ws.on('message', (data) => {
        this.handleMessage(data.toString());
      });

      this.ws.on('close', () => {
        console.log('Disconnected from Edge MCP');
      });
    });
  }

  /**
   * Disconnect from Edge MCP
   */
  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  /**
   * Handle incoming message
   */
  private handleMessage(data: string): void {
    try {
      const message: MCPMessage = JSON.parse(data);

      // Handle response
      if (message.id !== undefined) {
        const pending = this.pendingRequests.get(Number(message.id));
        if (pending) {
          this.pendingRequests.delete(Number(message.id));

          if (message.error) {
            pending.reject(new Error(`MCP Error (${message.error.code}): ${message.error.message}`));
          } else {
            pending.resolve(message.result);
          }
        }
      }

      // Handle notification (id is null or undefined)
      if (message.id === null || message.id === undefined) {
        console.log('Notification:', message);
      }
    } catch (error) {
      console.error('Failed to parse message:', error);
    }
  }

  /**
   * Send a message and wait for response
   */
  private sendMessage(method: string, params?: any): Promise<any> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new Error('Not connected to Edge MCP'));
        return;
      }

      const id = ++this.requestId;
      const message: MCPMessage = {
        jsonrpc: '2.0',
        id,
        method,
        params: params || {}
      };

      this.pendingRequests.set(id, { resolve, reject });

      this.ws.send(JSON.stringify(message), (error) => {
        if (error) {
          this.pendingRequests.delete(id);
          reject(error);
        }
      });

      console.log(`Sent: ${method} (id=${id})`);
    });
  }

  /**
   * Initialize MCP session
   */
  async initialize(): Promise<any> {
    const result = await this.sendMessage('initialize', {
      protocolVersion: '2025-06-18',
      clientInfo: {
        name: 'typescript-example-client',
        version: '1.0.0'
      }
    });

    // Send initialized confirmation
    await this.sendMessage('initialized', {});

    console.log(`Initialized: ${result.serverInfo.name} v${result.serverInfo.version}`);
    return result;
  }

  /**
   * List all available tools
   */
  async listTools(): Promise<ToolDefinition[]> {
    const result = await this.sendMessage('tools/list');
    const tools = result.tools || [];
    console.log(`Found ${tools.length} tools`);
    return tools;
  }

  /**
   * Execute a single tool
   */
  async callTool(name: string, args: Record<string, any>): Promise<any> {
    const result = await this.sendMessage('tools/call', {
      name,
      arguments: args
    });
    console.log(`Executed tool: ${name}`);
    return result;
  }

  /**
   * Execute multiple tools in batch
   */
  async batchCallTools(
    tools: BatchToolCall[],
    parallel: boolean = true
  ): Promise<BatchResult> {
    const result = await this.sendMessage('tools/batch', {
      tools,
      parallel
    });
    console.log(
      `Batch executed ${tools.length} tools: ` +
      `${result.success_count} succeeded, ${result.error_count} failed`
    );
    return result;
  }

  /**
   * Get current session context
   */
  async getContext(): Promise<any> {
    return await this.sendMessage('context.get');
  }

  /**
   * Update session context
   */
  async updateContext(context: Record<string, any>, merge: boolean = true): Promise<void> {
    await this.sendMessage('context.update', {
      context,
      merge
    });
    console.log('Context updated');
  }
}

/**
 * Example usage
 */
async function main() {
  const client = new EdgeMCPClient(EDGE_MCP_URL, API_KEY);

  try {
    // Connect and initialize
    await client.connect();
    await client.initialize();

    // List available tools
    const tools = await client.listTools();
    console.log(`\nAvailable tools: ${tools.length}`);
    tools.slice(0, 5).forEach(tool => {
      console.log(`  - ${tool.name}: ${tool.description || ''}`);
    });

    // Execute a single tool
    console.log('\n--- Single Tool Execution ---');
    const result = await client.callTool('github_get_repository', {
      owner: 'developer-mesh',
      repo: 'developer-mesh'
    });
    console.log('Repository info:', JSON.stringify(result, null, 2));

    // Batch execute multiple tools
    console.log('\n--- Batch Tool Execution ---');
    const batchResult = await client.batchCallTools([
      {
        id: 'call-1',
        name: 'github_list_issues',
        arguments: {
          owner: 'developer-mesh',
          repo: 'developer-mesh',
          state: 'open'
        }
      },
      {
        id: 'call-2',
        name: 'github_list_pull_requests',
        arguments: {
          owner: 'developer-mesh',
          repo: 'developer-mesh',
          state: 'open'
        }
      }
    ], true);

    console.log(`Batch completed in ${batchResult.duration_ms}ms:`);
    console.log(`  Success: ${batchResult.success_count}`);
    console.log(`  Errors: ${batchResult.error_count}`);

    // Update context
    console.log('\n--- Context Management ---');
    await client.updateContext({
      project: 'developer-mesh',
      task: 'api_documentation'
    });

    const context = await client.getContext();
    console.log('Current context:', JSON.stringify(context, null, 2));

  } catch (error) {
    console.error('Error:', error);
    throw error;
  } finally {
    // Disconnect
    client.disconnect();
  }
}

// Run example
main().catch(console.error);
