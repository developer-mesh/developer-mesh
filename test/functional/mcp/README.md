# MCP Functional Tests

This directory contains comprehensive functional tests for the MCP (Model Context Protocol) server implementation.

## Test Files

- **mcp_protocol_test.go** - Basic protocol compliance tests
- **mcp_server_comprehensive_test.go** - Comprehensive WebSocket connection and protocol tests
- **mcp_rest_api_test.go** - REST API endpoint tests
- **mcp_suite_test.go** - Ginkgo test suite setup
- **MCP_TEST_SUMMARY.md** - Detailed test coverage documentation

## Running Tests

### Prerequisites

1. Set environment variables:
```bash
export MCP_SERVER_URL=http://localhost:8080
export MCP_API_KEY=docker-admin-api-key
export MCP_TEST_MODE=true
```

2. Ensure MCP server is running:
```bash
make docker-compose-up
```

### Run All Tests

```bash
# Using Make targets
make test-functional-mcp

# Or directly with Ginkgo
cd test/functional
ginkgo -v ./mcp
```

### Run Specific Test Types

```bash
# WebSocket tests only
make test-functional-mcp-websocket

# REST API tests only
make test-functional-mcp-rest

# With coverage
make test-functional-mcp-coverage

# With specific focus
make test-functional-mcp-focus FOCUS="Tool Discovery"
```

### Development Mode

```bash
# Run tests in watch mode
make test-functional-mcp-watch
```

## Test Categories

1. **WebSocket Tests**: Test WebSocket connections, MCP protocol handshake, message handling
2. **REST API Tests**: Test HTTP endpoints, authentication, CRUD operations
3. **Integration Tests**: Test interaction with other services (contexts, vectors, etc.)
4. **Performance Tests**: Test latency, concurrent connections, throughput

## Configuration

Tests can be configured via environment variables:

- `MCP_SERVER_URL` - MCP server URL (default: `http://localhost:8080`)
- `MCP_API_KEY` - API key for authentication (default: `docker-admin-api-key`)
- `MCP_TEST_TIMEOUT` - Test timeout in seconds (default: 30)
- `MCP_TEST_MODE` - Enable test mode features (default: false)

## Adding New Tests

1. Add test functions to the appropriate test file
2. Use descriptive `Describe` and `It` blocks
3. Follow existing patterns for setup/teardown
4. Use environment variables for configuration
5. Add any new test scenarios to MCP_TEST_SUMMARY.md

## Troubleshooting

If tests fail:

1. Check server is running: `curl http://localhost:8080/health`
2. Verify API key: `echo $MCP_API_KEY`
3. Check logs: `make docker-compose-logs service=edge-mcp`
4. Enable debug mode: `export GINKGO_VERBOSE=true`