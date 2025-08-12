import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

// Custom metrics
const messagesSent = new Counter('messages_sent');
const messagesReceived = new Counter('messages_received');
const toolsListLatency = new Trend('tools_list_latency_ms');
const toolCallLatency = new Trend('tool_call_latency_ms');
const initLatency = new Trend('init_latency_ms');
const wsConnections = new Counter('ws_connections');
const wsErrors = new Counter('ws_errors');

// Test configuration
export const options = {
  stages: [
    // Ramp-up phase
    { duration: '2m', target: 50 },   // Ramp up to 50 users
    { duration: '5m', target: 100 },  // Ramp up to 100 users
    { duration: '10m', target: 200 }, // Stay at 200 users
    { duration: '5m', target: 300 },  // Peak load at 300 users
    { duration: '10m', target: 300 }, // Maintain peak load
    { duration: '5m', target: 100 },  // Ramp down to 100 users
    { duration: '2m', target: 0 },    // Ramp down to 0
  ],
  thresholds: {
    // Response time thresholds
    'tools_list_latency_ms': ['p(95)<500', 'p(99)<1000'],
    'tool_call_latency_ms': ['p(95)<2000', 'p(99)<5000'],
    'init_latency_ms': ['p(95)<1000', 'p(99)<2000'],
    
    // Error rate thresholds
    'ws_errors': ['rate<0.01'], // Less than 1% error rate
    
    // WebSocket connection success
    'ws_connections': ['rate>0.99'], // More than 99% success rate
  },
};

// Test data
const testTools = [
  'agent.register',
  'agent.list',
  'workflow.create',
  'workflow.execute',
  'task.create',
  'task.update',
  'context.get',
  'context.set'
];

function generateMCPMessage(method, params = {}, id = null) {
  return JSON.stringify({
    jsonrpc: '2.0',
    method: method,
    params: params,
    id: id || Math.random().toString(36).substring(7)
  });
}

export default function () {
  const url = `ws://${__ENV.MCP_HOST || 'localhost:8080'}/ws`;
  const params = {
    headers: {
      'Authorization': `Bearer ${__ENV.API_KEY || 'test-key'}`,
      'X-Tenant-ID': `tenant-${__VU}` // Virtual user ID as tenant
    }
  };

  const startTime = Date.now();
  
  const res = ws.connect(url, params, function (socket) {
    wsConnections.add(1);
    
    // Event handlers
    socket.on('open', function open() {
      console.log(`VU ${__VU}: WebSocket connected`);
      
      // Send initialize message
      const initStart = Date.now();
      socket.send(generateMCPMessage('initialize', {
        protocolVersion: '1.0.0',
        capabilities: {
          tools: true,
          resources: true,
          prompts: false
        },
        agentInfo: {
          name: `load-test-agent-${__VU}`,
          version: '1.0.0'
        }
      }));
      
      // Track initialization time
      socket.on('message', function(data) {
        const msg = JSON.parse(data);
        if (msg.result && msg.result.protocolVersion) {
          initLatency.add(Date.now() - initStart);
          
          // After initialization, start sending test messages
          performLoadTest(socket);
        }
        messagesReceived.add(1);
      });
    });

    socket.on('error', function (e) {
      console.error(`VU ${__VU}: WebSocket error:`, e);
      wsErrors.add(1);
    });

    socket.on('close', function () {
      console.log(`VU ${__VU}: WebSocket closed`);
    });

    // Keep connection alive for test duration
    socket.setTimeout(function () {
      socket.close();
    }, 60000); // 60 seconds timeout
  });

  check(res, {
    'WebSocket connection established': (r) => r && r.status === 101,
  });
}

function performLoadTest(socket) {
  // Test 1: List tools
  const toolsListStart = Date.now();
  socket.send(generateMCPMessage('tools/list', {}));
  
  socket.on('message', function(data) {
    const msg = JSON.parse(data);
    
    // Track tools list response time
    if (msg.result && msg.result.tools) {
      toolsListLatency.add(Date.now() - toolsListStart);
      
      // Test 2: Call random tools
      performToolCalls(socket, msg.result.tools);
    }
  });
  
  // Test 3: Resources operations
  setTimeout(() => {
    testResourceOperations(socket);
  }, 2000);
  
  // Test 4: Continuous message flow
  setInterval(() => {
    sendRandomMessage(socket);
  }, 5000); // Send a message every 5 seconds
}

function performToolCalls(socket, availableTools) {
  // Select random tools to call
  const numCalls = Math.floor(Math.random() * 5) + 1; // 1-5 calls
  
  for (let i = 0; i < numCalls; i++) {
    const tool = testTools[Math.floor(Math.random() * testTools.length)];
    const toolCallStart = Date.now();
    
    const params = generateToolParams(tool);
    
    socket.send(generateMCPMessage('tools/call', {
      name: tool,
      arguments: params
    }));
    
    socket.on('message', function(data) {
      const msg = JSON.parse(data);
      if (msg.result && msg.result.content) {
        toolCallLatency.add(Date.now() - toolCallStart);
      }
    });
    
    messagesSent.add(1);
    sleep(Math.random() * 2); // Random delay between calls
  }
}

function generateToolParams(toolName) {
  // Generate appropriate parameters based on tool type
  switch(toolName) {
    case 'agent.register':
      return {
        name: `agent-${__VU}-${Date.now()}`,
        capabilities: ['code', 'analysis'],
        model: 'claude-3'
      };
    case 'workflow.create':
      return {
        name: `workflow-${Date.now()}`,
        description: 'Load test workflow',
        steps: [
          { type: 'analyze', params: {} },
          { type: 'execute', params: {} }
        ]
      };
    case 'task.create':
      return {
        title: `Task ${Date.now()}`,
        description: 'Load test task',
        priority: Math.random() > 0.5 ? 'high' : 'normal'
      };
    case 'context.set':
      return {
        key: `context-${__VU}`,
        value: { 
          data: 'test data',
          timestamp: Date.now()
        }
      };
    default:
      return {};
  }
}

function testResourceOperations(socket) {
  // Test resource listing
  socket.send(generateMCPMessage('resources/list', {}));
  messagesSent.add(1);
  
  // Test resource reading
  setTimeout(() => {
    socket.send(generateMCPMessage('resources/read', {
      uri: 'metrics://system/performance'
    }));
    messagesSent.add(1);
  }, 1000);
}

function sendRandomMessage(socket) {
  const messageTypes = [
    () => generateMCPMessage('tools/list', {}),
    () => generateMCPMessage('resources/list', {}),
    () => generateMCPMessage('tools/call', {
      name: testTools[Math.floor(Math.random() * testTools.length)],
      arguments: {}
    })
  ];
  
  const message = messageTypes[Math.floor(Math.random() * messageTypes.length)]();
  socket.send(message);
  messagesSent.add(1);
}

// Export summary for better reporting
export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(data),
    'summary.html': htmlReport(data)
  };
}

function textSummary(data, options) {
  // Generate text summary
  let summary = '\n=== MCP Load Test Results ===\n\n';
  
  summary += `Total Duration: ${data.state.testRunDurationMs}ms\n`;
  summary += `VUs Max: ${data.metrics.vus.max}\n`;
  summary += `Messages Sent: ${data.metrics.messages_sent.count}\n`;
  summary += `Messages Received: ${data.metrics.messages_received.count}\n`;
  summary += `WebSocket Connections: ${data.metrics.ws_connections.count}\n`;
  summary += `WebSocket Errors: ${data.metrics.ws_errors.count}\n\n`;
  
  summary += '=== Latency Metrics ===\n';
  summary += `Tools List P95: ${data.metrics.tools_list_latency_ms.p95}ms\n`;
  summary += `Tools List P99: ${data.metrics.tools_list_latency_ms.p99}ms\n`;
  summary += `Tool Call P95: ${data.metrics.tool_call_latency_ms.p95}ms\n`;
  summary += `Tool Call P99: ${data.metrics.tool_call_latency_ms.p99}ms\n\n`;
  
  summary += '=== Threshold Results ===\n';
  for (const [key, value] of Object.entries(data.thresholds)) {
    summary += `${key}: ${value.passes ? '✓ PASS' : '✗ FAIL'}\n`;
  }
  
  return summary;
}

function htmlReport(data) {
  // Generate HTML report
  return `
    <!DOCTYPE html>
    <html>
    <head>
      <title>MCP Load Test Report</title>
      <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .metric { margin: 10px 0; padding: 10px; background: #f0f0f0; }
        .pass { color: green; }
        .fail { color: red; }
        h2 { color: #333; }
      </style>
    </head>
    <body>
      <h1>MCP Load Test Report</h1>
      <div class="metric">
        <h2>Summary</h2>
        <p>Duration: ${data.state.testRunDurationMs}ms</p>
        <p>Max VUs: ${data.metrics.vus.max}</p>
        <p>Messages: ${data.metrics.messages_sent.count} sent, ${data.metrics.messages_received.count} received</p>
      </div>
      <div class="metric">
        <h2>Latency</h2>
        <p>Tools List P95: ${data.metrics.tools_list_latency_ms.p95}ms</p>
        <p>Tool Call P95: ${data.metrics.tool_call_latency_ms.p95}ms</p>
      </div>
    </body>
    </html>
  `;
}