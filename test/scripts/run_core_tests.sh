#!/bin/bash

# This script runs key tests for the Edge MCP Server
# focusing on core functionality

set -e  # Exit on error

# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "Starting core tests from $PROJECT_ROOT"

# Set specific path to Go
GO_PATH="/usr/local/go/bin/go"

# Check if Go is installed
if ! command -v $GO_PATH &> /dev/null; then
    echo "Error: Go is not installed at $GO_PATH"
    exit 1
fi

# Clean up any build artifacts
echo "Cleaning up previous build artifacts..."
make clean

# Build the server
echo "Building Edge MCP server..."
cd ../../apps/edge-mcp && $GO_PATH build -o edge-mcp ./cmd/server && cd ../../test/scripts

# Run key tests that don't require Docker
echo "Running core tests..."

echo "1. Testing API functionality..."
$GO_PATH test ./internal/api -run="TestHealthHandler|TestRequestLogger|TestMetricsMiddleware|TestCORSMiddleware|TestBasicRouting" -v

echo "2. Testing Cache functionality..."
$GO_PATH test ./internal/cache -run="TestNewCache" -v

echo "3. Testing Config functionality..."
$GO_PATH test ./internal/config -run="TestSetDefaults|TestLoad" -v

echo "4. Testing Metrics functionality..."
$GO_PATH test ./internal/metrics -run="TestNewClient|TestNoopClient" -v

echo "5. Testing Repository functionality..."
$GO_PATH test ./internal/repository -run="TestStoreEmbedding|TestGetSupportedModels" -v

echo "6. Testing Safety functionality..."
$GO_PATH test ./internal/safety -run="TestGitHubChecker|TestGetCheckerForAdapter" -v

echo "7. Testing Context functionality..."
$GO_PATH test ./internal/core/context/tests -run="TestTruncateOldestFirst|TestTruncatePreservingUser" -v

echo "Core tests completed!"
