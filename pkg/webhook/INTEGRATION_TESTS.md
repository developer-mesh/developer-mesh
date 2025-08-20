# Context Lifecycle Integration Tests

This directory contains integration tests for the context lifecycle management system that use real Redis instances via testcontainers.

## Prerequisites

- Docker must be installed and running
- Go 1.24.6+ with testcontainers-go support

## Running the Tests

### Run all integration tests
```bash
make test-integration
```

### Run only lifecycle Redis tests
```bash
make test-redis-lifecycle
```

### Run specific test
```bash
cd pkg/webhook
go test -tags=integration -v -run TestWithRealRedis/DistributedLocking ./...
```

## Test Coverage

The integration tests cover:

1. **Distributed Locking**
   - Concurrent lock acquisition from multiple instances
   - Lock retry mechanism with backoff
   - Lock expiration and renewal
   - Proper cleanup on failure

2. **Batch Processing**
   - Large-scale batch transitions (200+ contexts)
   - Concurrent batch operations
   - Pipeline execution verification
   - Error handling and recovery

3. **Search Performance**
   - Sorted set query performance
   - Multi-tenant search operations
   - Time-based filtering efficiency
   - Large dataset handling (300+ contexts)

4. **Concurrent State Transitions**
   - Race condition handling
   - State consistency verification
   - Multi-manager coordination
   - Error recovery

## Benchmarks

Run performance benchmarks:
```bash
cd pkg/webhook
go test -tags=integration -bench=. -benchmem -run=^$ ./...
```

## Testcontainers Configuration

The tests use testcontainers-go to spin up Redis instances automatically. Each test suite gets a fresh Redis instance to ensure isolation.

### Container Configuration
- Image: `redis:7-alpine`
- Exposed Port: 6379
- Wait Strategy: Log message "Ready to accept connections"

## Debugging Failed Tests

1. **Container startup issues**
   - Check Docker is running: `docker ps`
   - Check Docker resources: `docker system df`
   - Clean up containers: `docker system prune`

2. **Connection timeouts**
   - The tests retry connection 10 times with 500ms delays
   - Check for port conflicts on your system
   - Ensure Docker networking is functional

3. **Test failures**
   - Enable debug logging: `export DEBUG=true`
   - Run specific test with verbose output
   - Check Redis logs: `docker logs <container-id>`

## CI/CD Integration

These tests are designed to run in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Integration Tests
  run: |
    make test-redis-lifecycle
```

## Performance Expectations

- Lock acquisition: < 10ms average
- Batch processing: > 100 contexts/second
- Search operations: < 100ms for 1000 contexts
- Concurrent operations: No deadlocks with 5+ managers

## Adding New Tests

When adding new integration tests:

1. Use the `//go:build integration` build tag
2. Create containers using the helper functions
3. Always defer container cleanup
4. Test with multiple concurrent managers
5. Verify final state consistency