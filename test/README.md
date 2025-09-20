# Test Suite Documentation

This document describes the test setup and common patterns for the Developer Mesh project.

## Test Structure

The project has multiple test levels:
- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions with external services
- **Functional Tests**: End-to-end tests of complete features

## Running Tests

### Unit Tests
```bash
# Run all unit tests
make test

# Run tests for specific application
make test-edge-mcp
make test-rest-api
make test-worker

# Run specific test
go test -v -run "TestAgentCRUD" ./apps/rest-api/internal/api
```

### Integration Tests
```bash
# Run all integration tests (requires Docker)
make test-integration

# Run specific integration test
ENABLE_INTEGRATION_TESTS=true go test -tags=integration -v -run "TestGitHubAdapter" ./pkg/tests/integration
```

### Functional Tests
```bash
# Start services and run functional tests
make test-functional
```

## Common Test Patterns

### 1. Authentication in Tests

#### REST API Tests
Tests should use Bearer token authentication with a test middleware:
```go
router.Use(func(c *gin.Context) {
    if c.GetHeader("Authorization") == "Bearer test-token" {
        tenantID := c.GetHeader("X-Tenant-ID")
        if tenantID == "" {
            tenantID = "default-tenant"
        }
        c.Set("user", map[string]any{
            "id": "test-user",
            "tenant_id": tenantID,
        })
    }
    c.Next()
})
```

#### Integration Tests
Always provide authentication configuration, even for mock tests:
```go
config := github.DefaultConfig()
config.Auth.Token = "test-token"  // Required even with mock server
```

### 2. Database Setup

#### In-Memory SQLite for Unit Tests
```go
db, err := sqlx.Open("sqlite3", ":memory:")
require.NoError(t, err)

// Create tables matching your models
schema := `
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT,
    model_id TEXT,
    ...
);`
```

### 3. JSON-RPC Protocol Testing

For MCP server tests, ensure proper JSON-RPC 2.0 handling:
```go
// Set version header for all responses
c.Header("X-MCP-Version", "1.0")

// Handle streaming requests
if c.GetHeader("Accept") == "text/event-stream" {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    // ... handle streaming
}
```

### 4. Model Validation

Ensure test data matches actual model structures:
```go
// Check models.Agent struct for actual fields
payload := map[string]any{
    "name":     "test-agent",
    "model_id": "gpt-4",
    // Don't include fields that don't exist in the model
}
```

## Environment Configuration

See `test/config/test-env.yaml` for complete test environment configuration.

Key environment variables:
- `ENABLE_INTEGRATION_TESTS=true` - Enable integration tests
- `GO_TEST_TAGS=integration` - Required build tags

## Troubleshooting Common Issues

### Authentication Failures (401)
- Check that test middleware is setting user context correctly
- Ensure tenant_id is set in the user object, not just context
- Verify API key configuration in test setup

### Database Errors
- Ensure all required columns exist in test schema
- Match column names to model struct tags (db:"column_name")
- Use appropriate data types for SQLite

### Integration Test Failures
- Always provide auth configuration, even for mocks
- Check that adapter factories are properly initialized
- Ensure mock servers are running on correct endpoints

### JSON-RPC Protocol Errors
- Validate request has "jsonrpc": "2.0" field
- Return proper error responses for invalid requests
- Set appropriate HTTP status codes (400 for invalid, 200 for valid)

## Best Practices

1. **Use Production Code Patterns**: Tests should follow the same patterns as production code
2. **Avoid Shortcuts**: Don't bypass validation or security just for tests
3. **Mock External Services**: Use httptest servers for external API calls
4. **Isolate Tests**: Each test should be independent and not affect others
5. **Clear Error Messages**: Include context in test assertions for easier debugging