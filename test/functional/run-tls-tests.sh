#!/bin/bash
# Script to run functional tests with TLS enabled

set -e

echo "=== TLS Functional Test Runner ==="
echo

# Check if certificates exist
CERT_DIR="${CERT_DIR:-./certs}"
if [ ! -f "$CERT_DIR/server.crt" ] || [ ! -f "$CERT_DIR/server.key" ]; then
    echo "❌ TLS certificates not found in $CERT_DIR"
    echo "Please run: ./scripts/certs/generate-dev-certs.sh"
    exit 1
fi

# Export TLS environment variables
export TEST_TLS_ENABLED=true
export TLS_CERT_FILE="$CERT_DIR/server.crt"
export TLS_KEY_FILE="$CERT_DIR/server.key"
export TLS_CA_FILE="$CERT_DIR/ca.crt"

# Export test URLs with HTTPS/WSS
export TEST_TLS_API_URL="https://localhost:8443"
export TEST_TLS_WS_URL="wss://localhost:8443/ws"

# Use the TLS configuration
export MCP_CONFIG_FILE="./test/functional/configs/config.tls.yaml"

echo "✅ TLS configuration:"
echo "   - Certificate: $TLS_CERT_FILE"
echo "   - Key: $TLS_KEY_FILE"
echo "   - CA: $TLS_CA_FILE"
echo "   - API URL: $TEST_TLS_API_URL"
echo "   - WebSocket URL: $TEST_TLS_WS_URL"
echo

# Check if services are running with TLS
echo "Checking TLS endpoints..."
if ! curl -k --connect-timeout 2 "$TEST_TLS_API_URL/health" >/dev/null 2>&1; then
    echo "⚠️  Warning: TLS API endpoint not reachable at $TEST_TLS_API_URL"
    echo "Make sure services are running with TLS enabled:"
    echo "  MCP_CONFIG_FILE=./test/functional/configs/config.tls.yaml ./apps/edge-mcp/edge-mcp"
    echo "  MCP_CONFIG_FILE=./test/functional/configs/config.tls.yaml ./apps/rest-api/api"
fi

# Run TLS-specific tests
echo
echo "Running TLS functional tests..."
cd test/functional

if command -v ginkgo >/dev/null 2>&1; then
    # Use ginkgo if available
    ginkgo -v --focus="TLS Configuration Tests" ./api/...
else
    # Fallback to go test
    go test -v ./api/... -ginkgo.focus="TLS Configuration Tests"
fi

echo
echo "=== TLS Test Summary ==="
echo "To run all functional tests with TLS:"
echo "  TEST_TLS_ENABLED=true make test-functional"
echo
echo "To test specific TLS features:"
echo "  - Minimum TLS version enforcement"
echo "  - TLS 1.3 preference"
echo "  - Secure cipher suite requirements"
echo "  - Certificate validation"