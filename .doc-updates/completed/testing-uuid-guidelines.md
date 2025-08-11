<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:44:39
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# UUID Testing Guidelines

## Overview

All test IDs in the Developer Mesh codebase must use proper UUID v4 format. This ensures consistency and prevents type mismatches between test and production code.

## Standard Test UUIDs

The project provides standard test UUIDs in `pkg/testutil/uuid_helpers.go`:

```go
// Standard test UUIDs
TestTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
TestUserID   = uuid.MustParse("22222222-2222-2222-2222-222222222222")

// Helper functions
func TestTenantIDString() string // Returns "11111111-1111-1111-1111-111111111111"
func TestUserIDString() string   // Returns "22222222-2222-2222-2222-222222222222"
```

## Usage Guidelines

### 1. Import the testutil package

```go
import "github.com/developer-mesh/developer-mesh/pkg/testutil"
```

### 2. Use standard UUIDs for common test scenarios

```go
// For tenant IDs
tenantID := testutil.TestTenantID         // uuid.UUID type
tenantIDStr := testutil.TestTenantIDString() // string type

// For user IDs
userID := testutil.TestUserID             // uuid.UUID type
userIDStr := testutil.TestUserIDString()     // string type
```

### 3. Generate random UUIDs for unique test data

```go
// When you need unique IDs in tests
agentID := uuid.New().String()
modelID := uuid.New().String()
```

### 4. Never use hardcoded non-UUID strings

```go
// ❌ BAD - Do not use
tenantID := "tenant-123"
userID := "test-user"

// ✅ GOOD - Use proper UUIDs
tenantID := testutil.TestTenantIDString()
userID := testutil.TestUserIDString()
```

## Test Categories

### Unit Tests
- Run with `make test`
- Should not require external services
- Use mocks for database and Redis dependencies
- Exclude with `-short` flag

### Integration Tests
- Run with `make test-int`
- Require Redis and PostgreSQL
- Use build tag `//go:build integration`
- Test real database interactions

### Service Tests
- Run with `make test-with-services`
- Unit tests that require Redis/PostgreSQL
- For testing cache implementations, repositories, etc.

## Test Data Factory

Use the test data factory for consistent test models:

```go
import "github.com/developer-mesh/developer-mesh/pkg/testutil"

// Create test models with defaults
agent := testutil.Factory.CreateTestAgent()
model := testutil.Factory.CreateTestModel()
tool := testutil.Factory.CreateTestDynamicTool()

// Customize with options
agent := testutil.Factory.CreateTestAgent(
    testutil.WithAgentName("Custom Agent"),
    testutil.WithAgentTenantID(customTenantID),
)
```

## Migration Checklist

When updating tests to use proper UUIDs:

1. Replace hardcoded string IDs with UUID constants or generated UUIDs
2. Update mock expectations to use UUID strings
3. Fix type mismatches between `uuid.UUID` and `string`
4. Add integration build tags where appropriate
5. Verify tests pass with `make test`

### Automated Migration

Use the provided migration script to automatically fix common hardcoded IDs:

```bash
# Run the migration script
./scripts/fix_test_uuids.sh

# Verify tests still pass
make test
```

The script will:
- Replace common test IDs like "tenant-123" with proper UUIDs
- Update string literals, variable assignments, and function calls
- Report any remaining non-UUID patterns for manual review

## Common Patterns

### Auth Context Setup

```go
// In test middleware
c.Set("user", map[string]any{
    "id":        testutil.TestUserIDString(),
    "tenant_id": testutil.TestTenantIDString(),
})
```

### API Key Configuration

```go
APIKeys: map[string]auth.APIKeySettings{
    "test-key-1234567890": {
        Role:     "admin",
        Scopes:   []string{"read", "write", "admin"},
        TenantID: testutil.TestTenantIDString(),
    },
}
```

### Repository Tests

```go
// Use proper UUIDs in test data
agent := &models.Agent{
    ID:       uuid.New().String(),
    TenantID: testutil.TestTenantID,
    Name:     "Test Agent",
}
```

## Troubleshooting

### Type Mismatch Errors

If you see errors like:
```
Not equal: expected: string("test-user")
          actual: uuid.UUID(uuid.UUID{...})
```

Fix by:
1. Converting UUID to string: `user.ID.String()`
2. Using the string helper: `testutil.TestUserIDString()`

### Database Schema Issues

For SQLite tests with PostgreSQL schemas:
```sql
-- Use quoted table names for SQLite
CREATE TABLE IF NOT EXISTS "mcp.agents" (...)
```

### Missing Services

If tests fail with "dial tcp" errors:
- Run `make test-with-services` for tests requiring Redis/PostgreSQL
- Add appropriate build tags to exclude from unit tests

## Mock Repositories

Use the provided mock repositories in `pkg/testutil/mock_repositories.go` for database-dependent tests:

```go
import "github.com/developer-mesh/developer-mesh/pkg/testutil"

// Create mock repository
mockAgentRepo := testutil.NewMockAgentRepository()

// Optionally set error for testing error paths
mockAgentRepo.SetError(errors.New("database error"))

// Use in service tests
service := NewAgentService(mockAgentRepo, logger)
```

Available mock repositories:
- `MockAgentRepository` - For agent CRUD operations
- `MockModelRepository` - For model management
- `MockDynamicToolRepository` - For dynamic tool operations

## UUID Format Requirements

All IDs in the Developer Mesh codebase must follow UUID v4 format:
- Format: `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`
- Example: `11111111-1111-1111-1111-111111111111`
- Use `uuid.New()` for random UUIDs
- Use `uuid.MustParse()` for known UUIDs in tests

Never use:
- Sequential IDs: "1", "2", "3"
- Prefixed strings: "tenant-123", "user-456"
- Short identifiers: "abc", "test"

Always use:
- Proper UUID format for all entity IDs
- Type `uuid.UUID` for ID fields in structs
