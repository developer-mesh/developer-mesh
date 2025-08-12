import ws from 'k6/ws';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '10s',
};

export default function () {
  const url = 'ws://localhost:8080/ws';
  const params = {
    headers: {
      'Authorization': 'Bearer dev-admin-key-1234567890',
    },
  };

  const res = ws.connect(url, params, function (socket) {
    socket.on('open', () => {
      console.log('Connected to WebSocket');
      
      // Send MCP initialize message
      socket.send(JSON.stringify({
        jsonrpc: '2.0',
        method: 'initialize',
        params: {
          protocolVersion: '1.0.0',
          capabilities: {
            tools: true,
            resources: true
          }
        },
        id: '1'
      }));
    });

    socket.on('message', (data) => {
      console.log('Received message:', data);
      
      // Parse and check if it's the init response
      try {
        const msg = JSON.parse(data);
        if (msg.id === '1' && msg.result) {
          console.log('Initialization successful');
          
          // Try listing tools
          socket.send(JSON.stringify({
            jsonrpc: '2.0',
            method: 'tools/list',
            params: {},
            id: '2'
          }));
        }
      } catch (e) {
        console.error('Failed to parse message:', e);
      }
    });

    socket.on('error', (e) => {
      console.error('WebSocket error:', e);
    });

    socket.on('close', () => {
      console.log('WebSocket connection closed');
    });

    // Keep connection open for 5 seconds
    socket.setTimeout(() => {
      console.log('Closing connection');
      socket.close();
    }, 5000);
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}