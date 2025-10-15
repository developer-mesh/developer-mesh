# RAG Loader API Integration Tests

## Overview

This directory contains comprehensive integration tests for the multi-tenant RAG loader API, focusing on:

- **Tenant Isolation**: Verifying that tenants cannot access each other's data
- **Credential Encryption**: Ensuring tenant-specific encryption with AAD protection
- **Authentication**: Testing JWT validation and authorization
- **Row Level Security (RLS)**: Confirming database-level tenant isolation

## Prerequisites

### 1. Database Setup

The integration tests require a PostgreSQL database with the following:

- PostgreSQL 14+ with `pgvector` extension
- Database: `devmesh_development` (configurable via `DATABASE_NAME`)
- User: `devmesh` with appropriate permissions (configurable via `DATABASE_USER`)

### 2. Run Migrations

Before running tests, ensure all migrations are applied:

```bash
# From project root
cd apps/rag-loader

# Apply migrations
migrate -database "postgresql://devmesh:devmesh@localhost:5432/devmesh_development?sslmode=disable" \
    -path migrations up
```

Or using Docker:

```bash
# Start the database
docker-compose -f docker-compose.local.yml up database -d

# Wait for database to be ready
sleep 5

# Run migrations
make migrate-up
```

### 3. Environment Variables

Set the following environment variables (or use defaults):

```bash
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=devmesh
export DATABASE_PASSWORD=devmesh
export DATABASE_NAME=devmesh_development
export DATABASE_SSL_MODE=disable
```

## Running Tests

### Run All Tests

```bash
go test -v ./internal/api/...
```

### Run Specific Tests

```bash
# Test tenant isolation
go test -v ./internal/api/... -run TestTenantIsolation

# Test credential encryption
go test -v ./internal/api/... -run TestCredentialEncryption

# Test authentication
go test -v ./internal/api/... -run TestAPIWithoutToken
```

### Run with Coverage

```bash
go test -v -cover ./internal/api/...
```

## Test Structure

### Helper Functions

- **`setupTestDatabase(t)`**: Creates test database connection with cleanup
- **`setupTestApp(db)`**: Creates Gin application with all middleware and routes
- **`createTestTenant(t, db, tenantID, name)`**: Creates a test tenant with automatic cleanup
- **`generateTestToken(tenantID, userID)`**: Generates valid JWT tokens for testing

### Test Cases

#### 1. TestTenantIsolation

**Purpose**: Verify that one tenant cannot access another tenant's data

**What it tests**:
- Tenant 1 can create and list their own sources
- Tenant 2 cannot see Tenant 1's sources via API
- Tenant 2 cannot access Tenant 1's sources by ID
- Database RLS policies enforce isolation at the SQL level

**Expected behavior**:
- HTTP 201 when tenant creates their own source
- HTTP 200 with empty list when different tenant lists sources
- HTTP 404 when tenant tries to access another tenant's source
- Database queries return 0 rows for different tenants

#### 2. TestCredentialEncryption

**Purpose**: Verify tenant-specific credential encryption

**What it tests**:
- Same secret encrypts differently for different tenants
- Each tenant can decrypt their own credentials
- Cross-tenant decryption fails (AAD protection)
- Encryption uses AES-256-GCM with tenant-specific keys

**Expected behavior**:
- Encrypted values differ between tenants
- Decryption succeeds for correct tenant
- Decryption fails with "decryption failed" error for wrong tenant

#### 3. TestAPIWithoutToken

**Purpose**: Verify authentication is required

**What it tests**:
- API endpoints reject requests without Authorization header
- Returns HTTP 401 Unauthorized
- Error message indicates missing authentication

#### 4. TestAPIWithInvalidToken

**Purpose**: Verify invalid tokens are rejected

**What it tests**:
- API endpoints reject malformed or invalid JWT tokens
- Returns HTTP 401 Unauthorized
- Error message indicates invalid token

#### 5. TestInactiveTenant

**Purpose**: Verify inactive tenants cannot access API

**What it tests**:
- Tenants with `is_active = false` are rejected
- Returns HTTP 403 Forbidden
- Error message indicates tenant not authorized

#### 6. TestCredentialManagerPanicsOnInvalidKeySize

**Purpose**: Verify master key validation

**What it tests**:
- CredentialManager panics with invalid key size (not 32 bytes)
- Accepts valid 32-byte keys for AES-256

#### 7. TestGetAllCredentials

**Purpose**: Verify bulk credential retrieval

**What it tests**:
- Multiple credentials can be stored per source
- All credentials retrieved correctly in one call
- Decryption works for all credential types

#### 8. TestDeleteCredentials

**Purpose**: Verify credential deletion

**What it tests**:
- Credentials can be deleted
- Deleted credentials cannot be retrieved
- Returns `sql.ErrNoRows` for deleted credentials

## Troubleshooting

### Error: "relation 'mcp.tenants' does not exist"

**Solution**: Run database migrations first (see Prerequisites above)

### Error: "connection refused"

**Solution**: Ensure PostgreSQL is running and accessible:

```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Or check local PostgreSQL
pg_isready -h localhost -p 5432
```

### Error: "invalid token"

**Solution**: This is expected for invalid token tests. If occurring in isolation tests, check JWT secret matches in test setup.

### Tests are slow

**Reason**: Integration tests connect to real database

**Solution**: Use `-short` flag to skip integration tests during development:

```bash
go test -short ./internal/api/...
```

Then mark tests as integration tests:

```go
func TestTenantIsolation(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // ... test code
}
```

## Test Data Cleanup

All tests use `t.Cleanup()` to ensure proper teardown:

- Database connections are closed
- Test tenants are deleted (cascading to sources, credentials, documents)
- No test data persists between test runs

## CI/CD Integration

For CI/CD pipelines, ensure:

1. PostgreSQL service is available
2. Migrations are applied before tests
3. Test database is isolated from production
4. Environment variables are set correctly

Example GitHub Actions:

```yaml
name: Integration Tests

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: ankane/pgvector:latest
        env:
          POSTGRES_PASSWORD: devmesh
          POSTGRES_USER: devmesh
          POSTGRES_DB: devmesh_development
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Run migrations
        run: |
          migrate -database "postgresql://devmesh:devmesh@localhost:5432/devmesh_development?sslmode=disable" \
            -path apps/rag-loader/migrations up

      - name: Run tests
        env:
          DATABASE_HOST: localhost
          DATABASE_PORT: 5432
          DATABASE_USER: devmesh
          DATABASE_PASSWORD: devmesh
          DATABASE_NAME: devmesh_development
        run: |
          cd apps/rag-loader
          go test -v -cover ./internal/api/...
```

## Security Notes

### Test JWT Secret

The tests use a hardcoded JWT secret: `test-secret-key-32-bytes-long!!`

**⚠️ WARNING**: This is for testing only. Never use this secret in production.

### Test Master Key

Tests generate random 32-byte master keys for credential encryption.

Each test run uses different keys to ensure isolation.

### Credential Testing

GitHub credential validation is mocked/skipped in tests because:
- Tests shouldn't require real GitHub tokens
- Credential testing is for connectivity, not security
- Integration tests focus on tenant isolation, not external API validation

## Further Reading

- [Multi-Tenant Implementation Guide](../../../../docs/rag-loader-multi-tenant-implementation.md)
- [Security Best Practices](../../../../docs/security/tenant-isolation.md)
- [Database Schema](../../migrations/README.md)
