#!/bin/bash

# Integration Test Script for Edge MCP

set -e  # Exit on error

# Set current directory to project root
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

echo "Starting integration tests from $PROJECT_ROOT"

# Check if server and mockserver are already running
SERVER_RUNNING=false
MOCKSERVER_RUNNING=false

if pgrep -f "./edge-mcp" > /dev/null; then
    echo "Edge MCP Server is already running"
    SERVER_RUNNING=true
fi

if pgrep -f "./mockserver" > /dev/null; then
    echo "Mock Server is already running"
    MOCKSERVER_RUNNING=true
fi

# Function to clean up resources
cleanup() {
    echo "Cleaning up resources..."

    if [ "$SERVER_RUNNING" = false ] && pgrep -f "./edge-mcp" > /dev/null; then
        echo "Stopping Edge MCP Server..."
        pkill -f "./edge-mcp"
    fi

    if [ "$MOCKSERVER_RUNNING" = false ] && pgrep -f "./mockserver" > /dev/null; then
        echo "Stopping Mock Server..."
        pkill -f "./mockserver"
    fi

    echo "Cleanup completed"
}

# Register cleanup function to be called on script exit
trap cleanup EXIT

# Check if containers are running
if ! docker-compose -f docker-compose.local.yml ps postgres | grep -q "Up"; then
    echo "Starting PostgreSQL container..."
    docker-compose -f docker-compose.local.yml up -d postgres
    
    # Wait for PostgreSQL to start
    echo "Waiting for PostgreSQL to be ready..."
    sleep 5
fi

if ! docker-compose -f docker-compose.local.yml ps redis | grep -q "Up"; then
    echo "Starting Redis container..."
    docker-compose -f docker-compose.local.yml up -d redis
    
    # Wait for Redis to start
    echo "Waiting for Redis to be ready..."
    sleep 3
fi

if ! docker-compose -f docker-compose.local.yml ps mockserver | grep -q "Up"; then
    echo "Starting Mock Server container..."
    docker-compose -f docker-compose.local.yml up -d mockserver
    
    # Wait for Mock Server to start
    echo "Waiting for Mock Server to be ready..."
    sleep 3
fi

# Build if needed
if [ ! -f "./edge-mcp" ] || [ ! -f "./mockserver" ]; then
    echo "Building Edge MCP Server and Mock Server..."
    make build
    make mockserver-build
fi

# Start the server if not already running
if [ "$SERVER_RUNNING" = false ]; then
    echo "Starting Edge MCP Server..."
    ./edge-mcp &

    # Wait for server to start
    echo "Waiting for Edge MCP Server to be ready..."
    sleep 5
fi

# Set environment variables for the tests
export EDGE_MCP_ADDR="http://localhost:8085"
export MCP_API_KEY="local-admin-api-key"
export ENABLE_INTEGRATION_TESTS="true"

# Run integration tests
echo "Running integration tests..."
/usr/local/go/bin/go test -tags=integration -v ./test/integration/...

echo "Integration tests completed successfully!"
