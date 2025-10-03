#!/usr/bin/env python3
"""
Edge MCP Python Client Example

This example demonstrates how to connect to Edge MCP and execute tools
using the WebSocket-based MCP protocol.

Requirements:
    pip install websockets asyncio
"""

import asyncio
import json
import logging
from typing import Any, Dict, List, Optional

import websockets

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
EDGE_MCP_URL = "ws://localhost:8082/ws"
API_KEY = "dev-admin-key-1234567890"


class EdgeMCPClient:
    """Edge MCP WebSocket client"""

    def __init__(self, url: str, api_key: str):
        self.url = url
        self.api_key = api_key
        self.websocket: Optional[websockets.WebSocketClientProtocol] = None
        self.request_id = 0

    def _next_id(self) -> int:
        """Generate next request ID"""
        self.request_id += 1
        return self.request_id

    async def connect(self) -> None:
        """Connect to Edge MCP WebSocket"""
        headers = {
            "Authorization": f"Bearer {self.api_key}"
        }
        self.websocket = await websockets.connect(self.url, extra_headers=headers)
        logger.info("Connected to Edge MCP")

    async def disconnect(self) -> None:
        """Disconnect from Edge MCP"""
        if self.websocket:
            await self.websocket.close()
            logger.info("Disconnected from Edge MCP")

    async def send_message(self, method: str, params: Optional[Dict] = None) -> Dict[str, Any]:
        """Send a JSON-RPC message and wait for response"""
        if not self.websocket:
            raise RuntimeError("Not connected to Edge MCP")

        request_id = self._next_id()
        message = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": params or {}
        }

        # Send message
        await self.websocket.send(json.dumps(message))
        logger.debug(f"Sent: {method} (id={request_id})")

        # Receive response
        response_str = await self.websocket.recv()
        response = json.loads(response_str)
        logger.debug(f"Received: {response}")

        # Check for error
        if "error" in response:
            error = response["error"]
            raise Exception(f"MCP Error ({error['code']}): {error['message']}")

        return response.get("result", {})

    async def initialize(self) -> Dict[str, Any]:
        """Initialize MCP session"""
        result = await self.send_message("initialize", {
            "protocolVersion": "2025-06-18",
            "clientInfo": {
                "name": "python-example-client",
                "version": "1.0.0"
            }
        })

        # Send initialized confirmation
        await self.send_message("initialized", {})

        logger.info(f"Initialized: {result['serverInfo']['name']} v{result['serverInfo']['version']}")
        return result

    async def list_tools(self) -> List[Dict[str, Any]]:
        """List all available tools"""
        result = await self.send_message("tools/list")
        tools = result.get("tools", [])
        logger.info(f"Found {len(tools)} tools")
        return tools

    async def call_tool(self, name: str, arguments: Dict[str, Any]) -> Any:
        """Execute a single tool"""
        result = await self.send_message("tools/call", {
            "name": name,
            "arguments": arguments
        })
        logger.info(f"Executed tool: {name}")
        return result

    async def batch_call_tools(
        self,
        tools: List[Dict[str, Any]],
        parallel: bool = True
    ) -> Dict[str, Any]:
        """Execute multiple tools in batch"""
        result = await self.send_message("tools/batch", {
            "tools": tools,
            "parallel": parallel
        })
        logger.info(
            f"Batch executed {len(tools)} tools: "
            f"{result['success_count']} succeeded, {result['error_count']} failed"
        )
        return result

    async def get_context(self) -> Dict[str, Any]:
        """Get current session context"""
        result = await self.send_message("context.get")
        return result

    async def update_context(self, context: Dict[str, Any], merge: bool = True) -> None:
        """Update session context"""
        await self.send_message("context.update", {
            "context": context,
            "merge": merge
        })
        logger.info("Context updated")


async def main():
    """Example usage of Edge MCP client"""

    # Create client
    client = EdgeMCPClient(EDGE_MCP_URL, API_KEY)

    try:
        # Connect and initialize
        await client.connect()
        await client.initialize()

        # List available tools
        tools = await client.list_tools()
        print(f"\nAvailable tools: {len(tools)}")
        for tool in tools[:5]:  # Show first 5
            print(f"  - {tool['name']}: {tool.get('description', '')}")

        # Execute a single tool
        print("\n--- Single Tool Execution ---")
        result = await client.call_tool("github_get_repository", {
            "owner": "developer-mesh",
            "repo": "developer-mesh"
        })
        print(f"Repository info: {json.dumps(result, indent=2)}")

        # Batch execute multiple tools
        print("\n--- Batch Tool Execution ---")
        batch_result = await client.batch_call_tools([
            {
                "id": "call-1",
                "name": "github_list_issues",
                "arguments": {
                    "owner": "developer-mesh",
                    "repo": "developer-mesh",
                    "state": "open"
                }
            },
            {
                "id": "call-2",
                "name": "github_list_pull_requests",
                "arguments": {
                    "owner": "developer-mesh",
                    "repo": "developer-mesh",
                    "state": "open"
                }
            }
        ], parallel=True)

        print(f"Batch completed in {batch_result['duration_ms']}ms:")
        print(f"  Success: {batch_result['success_count']}")
        print(f"  Errors: {batch_result['error_count']}")

        # Update context
        print("\n--- Context Management ---")
        await client.update_context({
            "project": "developer-mesh",
            "task": "api_documentation"
        })

        context = await client.get_context()
        print(f"Current context: {json.dumps(context, indent=2)}")

    except Exception as e:
        logger.error(f"Error: {e}")
        raise

    finally:
        # Disconnect
        await client.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
