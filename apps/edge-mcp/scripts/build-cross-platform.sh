#!/bin/bash
# Build Edge MCP for multiple platforms

set -e

echo "Building Edge MCP for multiple platforms..."
echo

# Create build directory
mkdir -p build

# Build for current platform
echo "Building for current platform ($(go env GOOS)/$(go env GOARCH))..."
go build -o build/edge-mcp ./cmd/server
echo "✅ Built: build/edge-mcp"

# Build for macOS (Intel)
echo "Building for macOS/amd64..."
GOOS=darwin GOARCH=amd64 go build -o build/edge-mcp-darwin-amd64 ./cmd/server
echo "✅ Built: build/edge-mcp-darwin-amd64"

# Build for macOS (Apple Silicon)
echo "Building for macOS/arm64..."
GOOS=darwin GOARCH=arm64 go build -o build/edge-mcp-darwin-arm64 ./cmd/server
echo "✅ Built: build/edge-mcp-darwin-arm64"

# Build for Linux amd64
echo "Building for Linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o build/edge-mcp-linux-amd64 ./cmd/server
echo "✅ Built: build/edge-mcp-linux-amd64"

# Build for Linux arm64
echo "Building for Linux/arm64..."
GOOS=linux GOARCH=arm64 go build -o build/edge-mcp-linux-arm64 ./cmd/server
echo "✅ Built: build/edge-mcp-linux-arm64"

# Build for Windows amd64
echo "Building for Windows/amd64..."
GOOS=windows GOARCH=amd64 go build -o build/edge-mcp-windows-amd64.exe ./cmd/server
echo "✅ Built: build/edge-mcp-windows-amd64.exe"

echo
echo "Cross-platform builds complete!"
echo
echo "Build artifacts:"
ls -lh build/
echo
echo "Platform-specific features:"
echo "- Unix systems (macOS/Linux): Full process group support, all shell commands"
echo "- Windows: Adapted command mappings (ls→dir, cat→type, etc.), no process groups"
echo "- All platforms: Secure command execution, path validation, API compatibility"