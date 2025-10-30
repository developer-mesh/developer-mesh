#!/bin/bash
# Edge MCP Development Build Script
# This script builds Edge MCP with proper version injection for development

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Navigate to script directory then to project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

# Get version information
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Parse arguments
OUTPUT_DIR="${1:-${PROJECT_ROOT}/bin}"
OUTPUT_NAME="${2:-edge-mcp}"

echo -e "${YELLOW}Building Edge MCP...${NC}"
echo "  Version:    ${VERSION}"
echo "  Commit:     ${COMMIT}"
echo "  Build Time: ${BUILD_TIME}"
echo "  Output:     ${OUTPUT_DIR}/${OUTPUT_NAME}"

# Create output directory if it doesn't exist
mkdir -p "${OUTPUT_DIR}"

# Build with version injection
cd apps/edge-mcp && go build \
    -ldflags="-s -w \
        -X 'main.version=${VERSION}' \
        -X 'main.commit=${COMMIT}' \
        -X 'main.buildTime=${BUILD_TIME}'" \
    -o "${OUTPUT_DIR}/${OUTPUT_NAME}" \
    ./cmd/server

echo -e "${GREEN}✅ Edge MCP built successfully!${NC}"

# Verify the build
if command -v "${OUTPUT_DIR}/${OUTPUT_NAME}" &> /dev/null || [ -f "${OUTPUT_DIR}/${OUTPUT_NAME}" ]; then
    echo -e "\n${YELLOW}Version Info:${NC}"
    "${OUTPUT_DIR}/${OUTPUT_NAME}" --version
else
    echo -e "\n${YELLOW}Binary location:${NC} ${OUTPUT_DIR}/${OUTPUT_NAME}"
fi

echo -e "\n${YELLOW}Quick Commands:${NC}"
echo "  Run:     ${OUTPUT_DIR}/${OUTPUT_NAME} --port 8082"
echo "  Stdio:   ${OUTPUT_DIR}/${OUTPUT_NAME} --stdio"
echo "  Version: ${OUTPUT_DIR}/${OUTPUT_NAME} --version"
echo "  Help:    ${OUTPUT_DIR}/${OUTPUT_NAME} --help"