import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

// Custom metrics
const messagesSent = new Counter('messages_sent');
const messagesReceived = new Counter('messages_received');
const connectionsSuccess = new Rate('connections_success');
const toolsListLatency = new Trend('tools_list_latency_ms');
const toolCallLatency = new Trend('tool_call_latency_ms');
const initLatency = new Trend('init_latency_ms');
const wsErrors = new Counter('ws_errors');

// Test configuration for comprehensive testing
export const options = {
  scenarios: {
    // Scenario 1: Smoke test
    smoke: {
      executor: 'constant-vus',
      vus: 2,
      duration: '1m',
      startTime: '0s',
    },
    // Scenario 2: Load test
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 20 },  // Ramp up to 20 users
        { duration: '5m', target: 20 },  // Stay at 20 users
        { duration: '2m', target: 0 },   // Ramp down to 0
      ],
      startTime: '1m',
    },
    // Scenario 3: Stress test
    stress: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 50 },  // Ramp up to 50 users
        { duration: '5m', target: 100 }, // Ramp up to 100 users
        { duration: '2m', target: 0 },   // Ramp down to 0
      ],
      startTime: '10m',
    },
  },
  thresholds: {
    // Connection success rate
    'connections_success': ['rate>0.95'],
    
    // Response time thresholds
    'tools_list_latency_ms': ['p(95)<1000', 'p(99)<2000'],
    'tool_call_latency_ms': ['p(95)<3000', 'p(99)<5000'],
    'init_latency_ms': ['p(95)<1500', 'p(99)<3000'],
    
    // Error rate threshold
    'ws_errors': ['rate<0.05'], // Less than 5% error rate
    
    // WebSocket checks
    'checks': ['rate>0.9'], // 90% of checks should pass
  },
};

// Test data
const testTools = [
  'github.list_repos',
  'github.get_issue',
  'workflow.create',
  'workflow.execute',
  'agent.register',
  'agent.list',
  'context.save',
  'context.retrieve'
];

const testResources = [
  'metrics://system/cpu',
  'metrics://system/memory',
  'config://app/settings',
  'logs://app/recent'
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
  const url = `ws://localhost:8080/ws`;
  const params = {
    headers: {
      'Authorization': 'Bearer dev-admin-key-1234567890',
      'X-Request-ID': `test-${__VU}-${__ITER}`,
      'X-Tenant-ID': `tenant-${(__VU % 10) + 1}` // Distribute across 10 tenants
    },
    timeout: '30s',
  };

  let connectionSuccess = false;
  let initSuccess = false;
  let messagesReceivedCount = 0;

  const res = ws.connect(url, params, function (socket) {
    
    socket.on('open', function open() {
      connectionSuccess = true;
      console.log(`VU ${__VU}: WebSocket connected`);
      
      // Send initialize message
      const initStart = Date.now();
      const initMsg = generateMCPMessage('initialize', {
        protocolVersion: '1.0.0',
        capabilities: {
          tools: true,
          resources: true,
          prompts: false,
          completion: false
        },
        clientInfo: {
          name: `k6-test-client-${__VU}`,
          version: '1.0.0'
        }
      }, 'init-1');
      
      socket.send(initMsg);
      messagesSent.add(1);
      
      // Handle initialization response
      socket.on('message', function(data) {
        messagesReceived.add(1);
        messagesReceivedCount++;
        
        try {
          const msg = JSON.parse(data);
          
          // Check if this is the init response
          if (msg.id === 'init-1' && msg.result) {
            initSuccess = true;
            initLatency.add(Date.now() - initStart);
            console.log(`VU ${__VU}: Initialized successfully`);
            
            // After init, perform test operations
            performTestOperations(socket);
          }
          
          // Handle tool list response
          if (msg.id && msg.id.startsWith('tools-list-') && msg.result) {
            const latency = Date.now() - parseInt(msg.id.split('-')[2]);
            toolsListLatency.add(latency);
          }
          
          // Handle tool call response
          if (msg.id && msg.id.startsWith('tool-call-') && msg.result) {
            const latency = Date.now() - parseInt(msg.id.split('-')[2]);
            toolCallLatency.add(latency);
          }
          
        } catch (e) {
          console.error(`VU ${__VU}: Failed to parse message:`, e);
          wsErrors.add(1);
        }
      });
    });

    socket.on('error', function (e) {
      console.error(`VU ${__VU}: WebSocket error:`, e);
      wsErrors.add(1);
    });

    socket.on('close', function () {
      console.log(`VU ${__VU}: WebSocket closed`);
    });

    // Perform test operations
    function performTestOperations(socket) {
      // Test 1: List tools
      const toolsListStart = Date.now();
      socket.send(generateMCPMessage('tools/list', {}, `tools-list-${toolsListStart}`));
      messagesSent.add(1);
      
      // Test 2: Call random tools (after delay)
      setTimeout(() => {
        for (let i = 0; i < 3; i++) {
          const tool = testTools[Math.floor(Math.random() * testTools.length)];
          const toolCallStart = Date.now();
          
          socket.send(generateMCPMessage('tools/call', {
            name: tool,
            arguments: generateToolArgs(tool)
          }, `tool-call-${toolCallStart}`));
          
          messagesSent.add(1);
          sleep(0.5 + Math.random()); // Random delay between calls
        }
      }, 1000);
      
      // Test 3: List resources
      setTimeout(() => {
        socket.send(generateMCPMessage('resources/list', {}, 'resources-list-1'));
        messagesSent.add(1);
      }, 2000);
      
      // Test 4: Read resources
      setTimeout(() => {
        const resource = testResources[Math.floor(Math.random() * testResources.length)];
        socket.send(generateMCPMessage('resources/read', {
          uri: resource
        }, 'resource-read-1'));
        messagesSent.add(1);
      }, 3000);
      
      // Test 5: Subscribe to resource updates
      setTimeout(() => {
        socket.send(generateMCPMessage('resources/subscribe', {
          uri: 'metrics://system/cpu'
        }, 'resource-sub-1'));
        messagesSent.add(1);
      }, 4000);
      
      // Keep connection alive for test duration
      setTimeout(() => {
        socket.close();
      }, 20000 + Math.random() * 10000); // 20-30 seconds
    }

    // Set overall timeout
    socket.setTimeout(function () {
      console.log(`VU ${__VU}: Timeout reached, closing connection`);
      socket.close();
    }, 60000); // 60 seconds timeout
  });

  // Record connection success
  connectionsSuccess.add(connectionSuccess);

  // Checks
  check(res, {
    'WebSocket connection established': (r) => connectionSuccess,
    'MCP initialization successful': (r) => initSuccess,
    'Received messages': (r) => messagesReceivedCount > 0,
  });
  
  // Sleep between iterations
  sleep(1 + Math.random() * 2);
}

// Helper function to generate tool arguments
function generateToolArgs(toolName) {
  switch(toolName) {
    case 'github.list_repos':
      return {
        org: 'developer-mesh',
        limit: 10
      };
    case 'github.get_issue':
      return {
        owner: 'developer-mesh',
        repo: 'devops-mcp',
        number: Math.floor(Math.random() * 100) + 1
      };
    case 'workflow.create':
      return {
        name: `test-workflow-${Date.now()}`,
        description: 'Load test workflow',
        steps: [
          { action: 'analyze', params: { target: 'code' } },
          { action: 'report', params: { format: 'json' } }
        ]
      };
    case 'agent.register':
      return {
        name: `agent-${__VU}-${Date.now()}`,
        capabilities: ['code_analysis', 'testing'],
        model: 'claude-3-opus'
      };
    case 'context.save':
      return {
        key: `context-${__VU}`,
        value: {
          data: 'test context data',
          timestamp: Date.now(),
          metadata: { source: 'k6-test' }
        }
      };
    default:
      return {};
  }
}

// Export summary handler for better reporting
export function handleSummary(data) {
  const summary = {
    'Total Duration': `${data.state.testRunDurationMs}ms`,
    'Total VUs': data.metrics.vus ? data.metrics.vus.max : 0,
    'Messages Sent': data.metrics.messages_sent ? data.metrics.messages_sent.count : 0,
    'Messages Received': data.metrics.messages_received ? data.metrics.messages_received.count : 0,
    'Connection Success Rate': data.metrics.connections_success ? `${(data.metrics.connections_success.rate * 100).toFixed(2)}%` : '0%',
    'WebSocket Errors': data.metrics.ws_errors ? data.metrics.ws_errors.count : 0,
    'Latencies': {
      'Init P95': data.metrics.init_latency_ms ? `${data.metrics.init_latency_ms.p95}ms` : 'N/A',
      'Tools List P95': data.metrics.tools_list_latency_ms ? `${data.metrics.tools_list_latency_ms.p95}ms` : 'N/A',
      'Tool Call P95': data.metrics.tool_call_latency_ms ? `${data.metrics.tool_call_latency_ms.p95}ms` : 'N/A',
    },
    'Thresholds': {}
  };
  
  // Add threshold results
  for (const [key, value] of Object.entries(data.thresholds || {})) {
    summary.Thresholds[key] = value.passes ? '✓ PASS' : '✗ FAIL';
  }
  
  return {
    'stdout': JSON.stringify(summary, null, 2),
    'summary.json': JSON.stringify(data)
  };
}