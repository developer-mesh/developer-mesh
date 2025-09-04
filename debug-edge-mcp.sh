#!/bin/bash

# Debug wrapper for Edge MCP to understand connection issues

echo "Edge MCP Debug Wrapper Starting at $(date)" >&2
echo "Environment Variables:" >&2
echo "  DEV_MESH_URL: $DEV_MESH_URL" >&2
echo "  DEV_MESH_API_KEY: ${DEV_MESH_API_KEY:0:20}..." >&2
echo "  GITHUB_TOKEN: ${GITHUB_TOKEN:+SET}" >&2
echo "" >&2

# Start Edge MCP with verbose output
exec edge-mcp --stdio --log-level debug 2>&1 | while IFS= read -r line; do
    # Output to stderr so it appears in Claude logs
    echo "[EDGE-MCP] $line" >&2
    # Also output to stdout for normal operation
    echo "$line"
done