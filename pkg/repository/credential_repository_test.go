package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	// Note: This test requires a real database connection for integration testing
	// For unit tests, use mocks as shown in credential_manager_test.go
)

func TestTenantCredential_Fields(t *testing.T) {
	// Test that all required fields exist and are properly tagged
	cred := &TenantCredential{
		ID:              uuid.New(),
		TenantID:        uuid.New(),
		CredentialName:  "test-cred",
		CredentialType:  "api_key",
		EncryptedValue:  "encrypted-value",
		IsActive:        true,
		Tags:            []string{"production", "ci/cd"},
		AllowedEdgeMcps: []string{"edge-1", "edge-2"},
	}

	assert.NotEqual(t, uuid.Nil, cred.ID)
	assert.NotEqual(t, uuid.Nil, cred.TenantID)
	assert.Equal(t, "test-cred", cred.CredentialName)
	assert.Equal(t, "api_key", cred.CredentialType)
	assert.True(t, cred.IsActive)
	assert.Len(t, cred.Tags, 2)
	assert.Len(t, cred.AllowedEdgeMcps, 2)
}

func TestTenantCredential_OAuthFields(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "encrypted-secret"
	refreshToken := "encrypted-refresh"
	tokenExpiry := time.Now().Add(1 * time.Hour)

	cred := &TenantCredential{
		ID:                         uuid.New(),
		TenantID:                   uuid.New(),
		CredentialType:             "oauth2",
		OAuthClientID:              &clientID,
		OAuthClientSecretEncrypted: &clientSecret,
		OAuthRefreshTokenEncrypted: &refreshToken,
		OAuthTokenExpiry:           &tokenExpiry,
	}

	assert.NotNil(t, cred.OAuthClientID)
	assert.Equal(t, "test-client-id", *cred.OAuthClientID)
	assert.NotNil(t, cred.OAuthClientSecretEncrypted)
	assert.NotNil(t, cred.OAuthRefreshTokenEncrypted)
	assert.NotNil(t, cred.OAuthTokenExpiry)
}

func TestTenantCredential_TimestampFields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(90 * 24 * time.Hour)
	lastUsedAt := now.Add(-1 * time.Hour)

	cred := &TenantCredential{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  &expiresAt,
		LastUsedAt: &lastUsedAt,
	}

	assert.False(t, cred.CreatedAt.IsZero())
	assert.False(t, cred.UpdatedAt.IsZero())
	assert.NotNil(t, cred.ExpiresAt)
	assert.NotNil(t, cred.LastUsedAt)
	assert.True(t, cred.ExpiresAt.After(now))
	assert.True(t, cred.LastUsedAt.Before(now))
}

func TestCredentialRepository_NewRepository(t *testing.T) {
	// This is a unit test that doesn't require a real DB
	// We're just testing that the constructor works
	db := &sqlx.DB{} // Empty DB for testing
	repo := NewCredentialRepository(db)

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.db)
}

// Integration tests below require a real PostgreSQL database with the schema
// They are skipped in short mode to allow unit tests to run without database

func TestCredentialRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require a real database connection
	// For actual integration testing, set up a test database:
	// db, err := sqlx.Connect("postgres", "postgres://...")
	// require.NoError(t, err)
	// defer db.Close()
	//
	// repo := NewCredentialRepository(db)
	// ctx := context.Background()
	//
	// cred := &TenantCredential{
	//     TenantID: uuid.New(),
	//     CredentialName: "test",
	//     CredentialType: "api_key",
	//     EncryptedValue: "encrypted",
	//     IsActive: true,
	// }
	//
	// err = repo.Create(ctx, cred)
	// require.NoError(t, err)
	// assert.NotEqual(t, uuid.Nil, cred.ID)
}

func TestCredentialRepository_GetByTenantAndName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Similar to Create_Integration test
	// This would test the GetByTenantAndName method with a real database
}

func TestCredentialRepository_ListExpiring_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test the ListExpiring method with a real database
	// Setup would include creating credentials with various expiry dates
}

func TestCredentialRepository_EnforceExpiry_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test the expiry enforcement with a real database
}

// Example of how to set up integration tests with a test database
// Uncomment and adapt when ready to run integration tests:
/*
func setupTestDB(t *testing.T) *sqlx.DB {
	// Connect to test database
	db, err := sqlx.Connect("postgres",
		"postgres://testuser:testpass@localhost:5432/devmesh_test?sslmode=disable")
	require.NoError(t, err)

	// Run migrations or create test schema
	// ...

	return db
}

func teardownTestDB(t *testing.T, db *sqlx.DB) {
	// Clean up test data
	db.Exec("TRUNCATE TABLE mcp.tenant_tool_credentials CASCADE")
	db.Close()
}

func TestCredentialRepository_FullLifecycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	repo := NewCredentialRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()

	// 1. Create credential
	cred := &TenantCredential{
		TenantID:       tenantID,
		CredentialName: "github-token",
		CredentialType: "api_key",
		EncryptedValue: "encrypted-token-value",
		IsActive:       true,
		Tags:           []string{"github", "ci"},
		ExpiresAt:      ptrTime(time.Now().Add(90 * 24 * time.Hour)),
	}

	err := repo.Create(ctx, cred)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, cred.ID)

	// 2. Get by ID
	retrieved, err := repo.Get(ctx, cred.ID)
	require.NoError(t, err)
	assert.Equal(t, cred.CredentialName, retrieved.CredentialName)

	// 3. Get by tenant and name
	byName, err := repo.GetByTenantAndName(ctx, tenantID, "github-token")
	require.NoError(t, err)
	assert.Equal(t, cred.ID, byName.ID)

	// 4. Update last used
	err = repo.UpdateLastUsed(ctx, cred.ID)
	require.NoError(t, err)

	// 5. List by tenant
	creds, err := repo.ListByTenant(ctx, tenantID, false)
	require.NoError(t, err)
	assert.Len(t, creds, 1)

	// 6. Deactivate
	err = repo.Deactivate(ctx, cred.ID)
	require.NoError(t, err)

	// 7. Verify deactivated
	deactivated, err := repo.Get(ctx, cred.ID)
	require.NoError(t, err)
	assert.False(t, deactivated.IsActive)

	// 8. Delete
	err = repo.Delete(ctx, cred.ID)
	require.NoError(t, err)

	// 9. Verify deleted
	_, err = repo.Get(ctx, cred.ID)
	assert.Error(t, err)
}
*/
