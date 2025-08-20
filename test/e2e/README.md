# Developer Mesh End-to-End Tests

This directory contains comprehensive end-to-end tests for the Developer Mesh platform, designed to test real AI agent interactions with the production environment.

## Overview

The E2E test suite validates:
- Single agent lifecycle and operations
- Multi-agent collaboration and coordination
- Performance under load and stress conditions
- Binary protocol efficiency
- Error handling and recovery
- Security and authentication

## Architecture

```
test/e2e/
├── agent/                 # Test agent implementations
│   ├── agent.go          # Base test agent framework
│   └── specialized_agents.go  # Specialized agent types
├── connection/           # WebSocket connection utilities
│   └── connection.go     # Connection management
├── data/                 # Test data management
│   └── data.go          # Test data generation and fixtures
├── reporting/            # Test reporting framework
│   └── reporter.go      # Multi-format test reporting
├── utils/               # Common utilities
│   ├── helpers.go       # Helper functions
│   └── isolation.go     # Test isolation mechanisms
├── scenarios/           # Test scenarios
│   ├── single_agent_test.go    # Single agent tests
│   ├── multi_agent_test.go     # Multi-agent collaboration tests
│   └── performance_test.go     # Performance and stress tests
├── suite_test.go        # Main test suite runner
├── Makefile            # Build and test commands
└── README.md           # This file
```

## Prerequisites

1. Go 1.24.6 or higher
2. Ginkgo test framework
3. Access to Developer Mesh production environment
4. Valid API credentials

## Installation

```bash
# Install dependencies
make install-deps

# Validate setup
make validate
```

## Configuration

### Environment Variables

```bash
# Required
export MCP_BASE_URL=mcp.dev-mesh.io
export API_BASE_URL=api.dev-mesh.io
export E2E_API_KEY=your-api-key

# Optional
export E2E_TENANT_ID=e2e-test-tenant
export E2E_DEBUG=true
export E2E_PARALLEL_TESTS=5
export E2E_TEST_TIMEOUT=30m
export E2E_REPORT_DIR=test-results
```

### Configuration File

Create a `.env` file in the test/e2e directory:

```env
MCP_BASE_URL=mcp.dev-mesh.io
API_BASE_URL=api.dev-mesh.io
E2E_API_KEY=your-api-key
E2E_TENANT_ID=e2e-test-tenant
```

## Running Tests

### Run All Tests
```bash
make test
```

### Run Specific Test Suites
```bash
# Single agent tests
make test-single

# Multi-agent collaboration tests
make test-multi

# Performance tests
make test-performance
```

### Run Tests Against Local Environment
```bash
make test-local
```

### Run Specific Test
```bash
make test-specific TEST_NAME="should handle reconnection"
```

### Watch Mode (Development)
```bash
make watch
```

### CI Mode
```bash
make test-ci
```

## Test Scenarios

### Single Agent Tests
- Agent lifecycle (connect, register, disconnect)
- Tool discovery and execution
- Context management
- Session management
- Error handling
- Authentication

### Multi-Agent Collaboration Tests
- Concurrent agent connections
- Code review workflow
- Parallel task execution
- Consensus mechanisms
- MapReduce patterns
- Deployment pipeline coordination

### Performance Tests
- 50+ concurrent agents
- Agent churn handling
- High message throughput
- Large payload handling
- Sustained load testing
- Binary protocol efficiency

## Reporting

Test results are generated in multiple formats:

- **JSON**: Machine-readable results (`test-results/report.json`)
- **JUnit XML**: CI/CD integration (`test-results/junit.xml`)
- **HTML**: Human-readable report (`test-results/report.html`)

View the latest test report:
```bash
make report
```

## Test Data Management

The test suite uses isolated test data:
- Each test creates its own namespace
- Test data is automatically cleaned up
- No interference between parallel tests
- Production-safe testing

## Writing New Tests

### 1. Create a New Test File

```go
package scenarios

import (
    "testing"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("My New Test", func() {
    It("should do something", func() {
        // Test implementation
    })
})
```

### 2. Use Test Agents

```go
// Create specialized agent
codeAgent := agent.NewCodeAnalysisAgent(apiKey, baseURL)

// Connect
err := codeAgent.Connect(ctx)
Expect(err).NotTo(HaveOccurred())
defer codeAgent.Close()

// Execute operations
resp, err := codeAgent.AnalyzeCode(ctx, repoURL, options)
```

### WebSocket Connection Requirements

**IMPORTANT**: All WebSocket connections to the MCP server MUST specify the `mcp.v1` subprotocol. Failure to do so will result in a 426 Upgrade Required error.

```go
// Correct WebSocket connection
dialOpts := &websocket.DialOptions{
    Subprotocols: []string{"mcp.v1"},  // REQUIRED
    HTTPHeader: http.Header{
        "Authorization": []string{"Bearer " + apiKey},
    },
}
```

### 3. Use Test Isolation

```go
// Create isolated namespace
namespace, err := isolation.CreateNamespace("my-test")
Expect(err).NotTo(HaveOccurred())
defer isolation.DeleteNamespace(namespace.ID)
```

## CI/CD Integration

### GitHub Actions

The test suite includes GitHub Actions workflow:
- Runs on every PR
- Nightly runs against production
- Performance monitoring
- Automatic test reports

### Running in CI

```yaml
- name: Run E2E tests
  run: |
    cd test/e2e
    make test-ci
```

## Debugging

### Enable Debug Logging
```bash
E2E_DEBUG=true make test
```

### Run Single Test with Verbose Output
```bash
ginkgo -v --focus="specific test name" .
```

### Analyze Failed Tests
```bash
# Check JSON report for details
jq '.testsuite[] | select(.failures > 0)' test-results/report.json
```

## Performance Benchmarks

### Expected Performance Targets

- **Connection Time**: < 100ms
- **Message Latency**: < 50ms (avg), < 100ms (p99)
- **Throughput**: > 100 msg/sec per agent
- **Concurrent Agents**: 50+ supported
- **Error Rate**: < 1%

### Run Benchmarks
```bash
make benchmark
```

## Troubleshooting

### Connection Issues
```bash
# Test connectivity
curl -I https://mcp.dev-mesh.io/health
curl -I https://api.dev-mesh.io/health

# Check API key
echo $E2E_API_KEY
```

### Test Timeouts
```bash
# Increase timeout
E2E_TEST_TIMEOUT=60m make test
```

### Clean Test Artifacts
```bash
make clean
```

## Contributing

1. Write tests following existing patterns
2. Ensure tests are idempotent
3. Use proper test isolation
4. Add appropriate assertions
5. Document new test scenarios
6. Run full test suite before submitting

## License

See the main project LICENSE file.