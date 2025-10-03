# Edge MCP Interactive Examples

This directory contains working examples demonstrating Edge MCP functionality. Each example is fully functional and can be run against a local Edge MCP instance.

## 📁 Directory Structure

```
examples/
├── README.md                     # This file
├── workflows/                    # Common workflow examples
│   ├── github_operations.go      # GitHub repository operations
│   ├── harness_pipelines.go      # Harness pipeline management
│   ├── agent_orchestration.go    # Agent task management
│   ├── batch_operations.go       # Batch execution patterns
│   └── context_management.go     # Session context handling
├── errors/                       # Error scenario examples
│   ├── tool_not_found.go         # Handling missing tools
│   ├── authentication.go         # Auth error recovery
│   ├── rate_limiting.go          # Rate limit handling
│   ├── timeouts.go               # Timeout and retry patterns
│   └── validation.go             # Parameter validation errors
├── performance/                  # Performance optimization examples
│   ├── batch_parallel.go         # Parallel batch execution
│   ├── streaming.go              # Streaming large responses
│   └── caching.go                # Cache utilization
├── test/                         # Test harness
│   ├── harness.go                # Test framework
│   ├── integration_test.go       # Integration tests
│   └── benchmark_test.go         # Performance benchmarks
└── common/                       # Shared utilities
    ├── client.go                 # Reusable MCP client
    └── helpers.go                # Helper functions
```

## 🚀 Prerequisites

Before running examples, ensure Edge MCP is running:

```bash
# Start Edge MCP with Docker Compose
docker-compose -f docker-compose.local.yml up -d

# Verify Edge MCP is healthy
curl http://localhost:8085/health/ready
```

## 🏃 Running Examples

Each example can be run independently:

### Common Workflows

```bash
# GitHub operations
cd examples/workflows
go run github_operations.go

# Harness pipeline management
go run harness_pipelines.go

# Agent orchestration
go run agent_orchestration.go

# Batch operations
go run batch_operations.go

# Context management
go run context_management.go
```

### Error Scenarios

```bash
cd examples/errors

# Demonstrates error handling for various scenarios
go run tool_not_found.go      # Tool discovery and alternatives
go run authentication.go       # Auth failures and recovery
go run rate_limiting.go        # Rate limit backoff strategies
go run timeouts.go             # Timeout handling and retries
go run validation.go           # Parameter validation errors
```

### Performance Examples

```bash
cd examples/performance

# Performance optimization patterns
go run batch_parallel.go       # Parallel execution performance
go run streaming.go            # Streaming large payloads
go run caching.go              # Cache hit rate optimization
```

### Test Harness

```bash
cd examples/test

# Run all integration tests
go test -v ./...

# Run specific test suite
go test -v -run TestWorkflows

# Run benchmarks
go test -bench=. -benchmem
```

## 📋 Example Descriptions

### Workflows

| Example | Description | Key Features |
|---------|-------------|--------------|
| **github_operations.go** | GitHub repository management | List repos, get issues, create PRs |
| **harness_pipelines.go** | CI/CD pipeline operations | Get pipeline status, trigger executions |
| **agent_orchestration.go** | Multi-agent task management | Create tasks, assign agents, track progress |
| **batch_operations.go** | Batch execution patterns | Parallel/sequential execution, partial failures |
| **context_management.go** | Session state management | Create sessions, update context, retrieve state |

### Error Scenarios

| Example | Error Type | Recovery Strategy |
|---------|-----------|-------------------|
| **tool_not_found.go** | `TOOL_NOT_FOUND` | Fuzzy search, alternative tools, category search |
| **authentication.go** | `AUTH_FAILED`, `PERMISSION_DENIED` | Validate credentials, check permissions, refresh tokens |
| **rate_limiting.go** | `RATE_LIMIT_EXCEEDED` | Exponential backoff, respect retry_after, quota management |
| **timeouts.go** | `OPERATION_TIMEOUT` | Retry with backoff, circuit breaker, streaming for large ops |
| **validation.go** | `PARAMETER_VALIDATION_FAILED` | Type conversion, format validation, schema validation |

### Performance

| Example | Optimization | Impact |
|---------|-------------|--------|
| **batch_parallel.go** | Parallel execution | 5-10x faster for independent operations |
| **streaming.go** | Response streaming | Reduced memory, faster first byte |
| **caching.go** | Cache utilization | 50-80% reduction in external API calls |

## 🔧 Configuration

Examples use environment variables for configuration:

```bash
# Edge MCP connection
export EDGE_MCP_URL="ws://localhost:8085/ws"
export EDGE_MCP_API_KEY="devmesh_ab80cbb2438dbb43339c0e3317ab2fc6dd0e046f3b50360df06abb5bae31a210"

# Optional: GitHub passthrough authentication
export GITHUB_TOKEN="ghp_your_token_here"

# Optional: Harness passthrough authentication
export HARNESS_API_KEY="your_harness_api_key"
export HARNESS_ACCOUNT_ID="your_account_id"

# Run examples
go run examples/workflows/github_operations.go
```

## 🧪 Test Harness

The test harness provides automated validation of all examples:

```go
// Run integration tests
cd examples/test
go test -v ./...

// Output:
// === RUN   TestWorkflows
// === RUN   TestWorkflows/GitHub_Operations
// === RUN   TestWorkflows/Harness_Pipelines
// === RUN   TestWorkflows/Agent_Orchestration
// === RUN   TestWorkflows/Batch_Operations
// === RUN   TestWorkflows/Context_Management
// --- PASS: TestWorkflows (2.34s)
// === RUN   TestErrorScenarios
// === RUN   TestErrorScenarios/Tool_Not_Found
// === RUN   TestErrorScenarios/Authentication
// === RUN   TestErrorScenarios/Rate_Limiting
// === RUN   TestErrorScenarios/Timeouts
// === RUN   TestErrorScenarios/Validation
// --- PASS: TestErrorScenarios (1.56s)
// === RUN   TestPerformance
// === RUN   TestPerformance/Batch_Parallel
// === RUN   TestPerformance/Streaming
// === RUN   TestPerformance/Caching
// --- PASS: TestPerformance (3.12s)
// PASS
// ok      github.com/developer-mesh/developer-mesh/examples/test  6.023s
```

## 📊 Benchmarks

Performance benchmarks for common operations:

```bash
cd examples/test
go test -bench=. -benchmem

# Expected output:
# BenchmarkSingleToolCall-8          1000  1234567 ns/op   2048 B/op  10 allocs/op
# BenchmarkBatchParallel-8            500  2345678 ns/op   4096 B/op  25 allocs/op
# BenchmarkBatchSequential-8          200  5678901 ns/op   4096 B/op  25 allocs/op
# BenchmarkStreamingResponse-8        300  3456789 ns/op   8192 B/op  15 allocs/op
```

## 🐛 Troubleshooting

### Connection Issues

```bash
# Check Edge MCP is running
curl http://localhost:8085/health/ready

# Check WebSocket connectivity
wscat -c ws://localhost:8085/ws \
  -H "Authorization: Bearer devmesh_ab80cbb2438dbb43339c0e3317ab2fc6dd0e046f3b50360df06abb5bae31a210"
```

### Authentication Errors

```bash
# Verify API key
export EDGE_MCP_API_KEY="devmesh_ab80cbb2438dbb43339c0e3317ab2fc6dd0e046f3b50360df06abb5bae31a210"

# Test authentication
curl -H "Authorization: Bearer $EDGE_MCP_API_KEY" \
  http://localhost:8085/health
```

### Rate Limiting

Examples demonstrate proper rate limit handling with exponential backoff. If you encounter rate limits:

1. Reduce concurrent requests
2. Add delays between batches
3. Use batch operations instead of individual calls
4. Monitor quota usage via metrics

## 📚 Additional Resources

- [Quick Start Guide](../docs/quickstart.md)
- [API Documentation](../docs/openapi/edge-mcp.yaml)
- [Integration Guides](../docs/integrations/)
- [Troubleshooting Guide](../docs/integrations/troubleshooting.md)

## 🤝 Contributing

When adding new examples:

1. Follow the existing structure
2. Include comprehensive error handling
3. Add examples to test harness
4. Document expected behavior
5. Update this README

## 📝 Notes

- All examples use the MCP protocol (JSON-RPC 2.0 over WebSocket)
- Examples are standalone and don't require Core Platform
- Each example includes cleanup code
- Errors are handled gracefully with recovery guidance
- All examples are tested in CI/CD pipeline
