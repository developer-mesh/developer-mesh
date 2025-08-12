import ws from 'k6/ws';
import { check } from 'k6';

// Quick test configuration
export const options = {
  vus: 5,
  duration: '30s',
};

export default function () {
  const url = `ws://localhost:8080/ws`;
  const params = {
    headers: {
      'Authorization': 'Bearer test-key',
    }
  };

  const res = ws.connect(url, params, function (socket) {
    socket.on('open', function open() {
      console.log(`Connected`);
      
      // Send initialize message
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
      
      socket.on('message', function(data) {
        console.log('Received:', data);
      });
    });

    socket.on('error', function (e) {
      console.error('WebSocket error:', e);
    });

    socket.setTimeout(function () {
      socket.close();
    }, 10000);
  });

  check(res, {
    'WebSocket connected': (r) => r && r.status === 101,
  });
}