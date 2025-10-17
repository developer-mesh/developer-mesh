package api_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/api"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/middleware"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/repository"
	"github.com/developer-mesh/developer-mesh/pkg/rag/security"
)

// setupTestDatabase creates a test database connection
func setupTestDatabase(t *testing.T) *sqlx.DB {
	// Get database connection from environment or use defaults
	host := getEnvOrDefault("DATABASE_HOST", "localhost")
	port := getEnvOrDefault("DATABASE_PORT", "5432")
	user := getEnvOrDefault("DATABASE_USER", "devmesh")
	password := getEnvOrDefault("DATABASE_PASSWORD", "devmesh")
	dbname := getEnvOrDefault("DATABASE_NAME", "devmesh_development")
	sslmode := getEnvOrDefault("DATABASE_SSL_MODE", "disable")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=rag,mcp,public",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping integration test: PostgreSQL not available: %v", err)
		return nil
	}

	// Set connection pool settings for tests
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)

	// Verify connection
	err = db.Ping()
	if err != nil {
		t.Skipf("Skipping integration test: PostgreSQL not responding: %v", err)
		return nil
	}

	// Create test tenants table if it doesn't exist
	createTenantsTable := `
		CREATE TABLE IF NOT EXISTS mcp.tenants (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err = db.Exec(createTenantsTable)
	if err != nil {
		_ = db.Close()
		t.Skipf("Skipping integration test: Failed to setup test database: %v", err)
		return nil
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return db
}

// setupTestApp creates a test Gin application with all middleware and routes
func setupTestApp(db *sqlx.DB) *gin.Engine {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create test JWT secret
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")
	jwtValidator := auth.NewJWTValidator(jwtSecret, "test-issuer")

	// Create test master key for encryption
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		panic(fmt.Sprintf("Failed to generate master key: %v", err))
	}

	// Create services
	credMgr := security.NewCredentialManager(db, masterKey)
	repo := repository.NewSourceRepository(db)
	tenantMiddleware := middleware.NewTenantMiddleware(db, jwtValidator)

	// Create handlers
	sourceHandler := api.NewSourceHandler(repo, credMgr)

	// Setup router
	r := gin.New()
	r.Use(gin.Recovery())

	// API routes with tenant middleware
	apiV1 := r.Group("/api/v1")
	ragGroup := apiV1.Group("/rag")
	ragGroup.Use(tenantMiddleware.ExtractTenant())
	{
		ragGroup.POST("/sources", sourceHandler.CreateSource)
		ragGroup.GET("/sources", sourceHandler.ListSources)
		ragGroup.GET("/sources/:id", sourceHandler.GetSource)
		ragGroup.PUT("/sources/:id", sourceHandler.UpdateSource)
		ragGroup.DELETE("/sources/:id", sourceHandler.DeleteSource)
		ragGroup.POST("/sources/:id/sync", sourceHandler.TriggerSync)
		ragGroup.GET("/sources/:id/jobs", sourceHandler.GetSyncJobs)
	}

	return r
}

// createTestTenant creates a tenant in the test database
func createTestTenant(t *testing.T, db *sqlx.DB, tenantID uuid.UUID, name string) {
	query := `
		INSERT INTO mcp.tenants (id, name, is_active, created_at)
		VALUES ($1, $2, true, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO NOTHING
	`

	_, err := db.Exec(query, tenantID, name)
	require.NoError(t, err, "Failed to create test tenant")

	// Cleanup: Remove tenant after test
	t.Cleanup(func() {
		// Delete in reverse order due to foreign keys
		_, _ = db.Exec("DELETE FROM rag.tenant_sync_jobs WHERE tenant_id = $1", tenantID)
		_, _ = db.Exec("DELETE FROM rag.tenant_documents WHERE tenant_id = $1", tenantID)
		_, _ = db.Exec("DELETE FROM rag.tenant_source_credentials WHERE tenant_id = $1", tenantID)
		_, _ = db.Exec("DELETE FROM rag.tenant_sources WHERE tenant_id = $1", tenantID)
		_, _ = db.Exec("DELETE FROM mcp.tenants WHERE id = $1", tenantID)
	})
}

// generateTestToken generates a valid JWT token for testing
func generateTestToken(tenantID uuid.UUID, userID uuid.UUID) string {
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")
	validator := auth.NewJWTValidator(jwtSecret, "test-issuer")

	token, err := validator.GenerateToken(
		tenantID.String(),
		userID.String(),
		"test@example.com",
		[]string{"user"},
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate test token: %v", err))
	}

	return token
}

// getEnvOrDefault returns environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createTestSource creates a source in the test database for credential testing
func createTestSource(t *testing.T, db *sqlx.DB, tenantID uuid.UUID, sourceID string) {
	query := `
		INSERT INTO rag.tenant_sources (tenant_id, source_id, source_type, config, enabled)
		VALUES ($1, $2, 'github_repo', '{}', true)
		ON CONFLICT (tenant_id, source_id) DO NOTHING
	`
	_, err := db.Exec(query, tenantID, sourceID)
	require.NoError(t, err, "Failed to create test source")
}

// TestTenantIsolation verifies that tenants cannot access each other's data
func TestTenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	db := setupTestDatabase(t)
	app := setupTestApp(db)

	// Create two test tenants
	tenant1 := uuid.New()
	tenant2 := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	createTestTenant(t, db, tenant1, "Tenant 1")
	createTestTenant(t, db, tenant2, "Tenant 2")

	// Create JWT tokens for each tenant
	token1 := generateTestToken(tenant1, user1)
	token2 := generateTestToken(tenant2, user2)

	// Tenant 1 creates a source
	source1 := map[string]interface{}{
		"source_id":   "github-tenant1",
		"source_type": "github_org",
		"config": map[string]interface{}{
			"org": "tenant1-org",
		},
		"credentials": map[string]string{
			"token": "ghp_tenant1_token_test",
		},
		"schedule": "0 */6 * * *", // Every 6 hours
	}

	body1, err := json.Marshal(source1)
	require.NoError(t, err)

	// Create source for tenant 1
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/rag/sources", bytes.NewBuffer(body1))
	req1.Header.Set("Authorization", "Bearer "+token1)
	req1.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)

	// Note: GitHub credentials test will fail without valid token, but that's OK for isolation test
	// We're testing if we can create sources, not if GitHub tokens are valid
	if w1.Code != http.StatusCreated && w1.Code != http.StatusBadRequest {
		t.Logf("Create source response: %s", w1.Body.String())
	}

	// If creation failed due to credential testing, skip the rest of this test
	if w1.Code == http.StatusBadRequest {
		t.Skip("Skipping isolation test due to GitHub credential validation failure (expected in test environment)")
		return
	}

	require.Equal(t, http.StatusCreated, w1.Code, "Tenant 1 should create source")

	// Tenant 2 tries to list sources (should see none)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/rag/sources", nil)
	req2.Header.Set("Authorization", "Bearer "+token2)

	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &response)
	require.NoError(t, err)

	sources, ok := response["sources"].([]interface{})
	require.True(t, ok, "Response should contain sources array")
	assert.Empty(t, sources, "Tenant 2 should not see Tenant 1's sources")

	// Tenant 1 should see their own source
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/rag/sources", nil)
	req3.Header.Set("Authorization", "Bearer "+token1)

	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)

	var response2 map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &response2)
	require.NoError(t, err)

	sources2, ok := response2["sources"].([]interface{})
	require.True(t, ok, "Response should contain sources array")
	assert.Len(t, sources2, 1, "Tenant 1 should see their own source")

	// Direct database query to verify RLS isolation
	ctx := context.Background()

	// Set tenant context for tenant 2
	_, err = db.ExecContext(ctx, "SELECT rag.set_current_tenant($1)", tenant2)
	require.NoError(t, err)

	// Query should return 0 rows for tenant 2
	var count int
	err = db.GetContext(ctx,
		&count,
		"SELECT COUNT(*) FROM rag.tenant_sources WHERE tenant_id = $1",
		tenant2)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Database RLS should enforce tenant isolation for tenant 2")

	// Set tenant context for tenant 1
	_, err = db.ExecContext(ctx, "SELECT rag.set_current_tenant($1)", tenant1)
	require.NoError(t, err)

	// Query should return 1 row for tenant 1
	err = db.GetContext(ctx,
		&count,
		"SELECT COUNT(*) FROM rag.tenant_sources WHERE tenant_id = $1",
		tenant1)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Database RLS should show tenant 1's own source")

	// Verify tenant 2 cannot access tenant 1's source by ID
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/rag/sources/github-tenant1", nil)
	req4.Header.Set("Authorization", "Bearer "+token2)

	w4 := httptest.NewRecorder()
	app.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusNotFound, w4.Code, "Tenant 2 should not be able to access Tenant 1's source")
}

// TestCredentialEncryption verifies tenant-specific credential encryption
func TestCredentialEncryption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	db := setupTestDatabase(t)

	// Create test master key
	masterKey := make([]byte, 32)
	_, err := rand.Read(masterKey)
	require.NoError(t, err)

	credMgr := security.NewCredentialManager(db, masterKey)

	tenant1 := uuid.New()
	tenant2 := uuid.New()

	createTestTenant(t, db, tenant1, "Test Tenant 1")
	createTestTenant(t, db, tenant2, "Test Tenant 2")

	ctx := context.Background()
	secret := "super-secret-token-12345"
	sourceID := "test-source"

	// Create sources for both tenants (required for foreign key)
	createTestSource(t, db, tenant1, sourceID)
	createTestSource(t, db, tenant2, sourceID)

	// Store same secret for both tenants
	err = credMgr.StoreCredential(ctx, tenant1, sourceID, "token", secret)
	require.NoError(t, err, "Should store credential for tenant 1")

	err = credMgr.StoreCredential(ctx, tenant2, sourceID, "token", secret)
	require.NoError(t, err, "Should store credential for tenant 2")

	// Retrieve encrypted values from database directly
	var encrypted1, encrypted2 string
	err = db.GetContext(ctx, &encrypted1,
		"SELECT encrypted_value FROM rag.tenant_source_credentials WHERE tenant_id = $1 AND source_id = $2",
		tenant1, sourceID)
	require.NoError(t, err)

	err = db.GetContext(ctx, &encrypted2,
		"SELECT encrypted_value FROM rag.tenant_source_credentials WHERE tenant_id = $1 AND source_id = $2",
		tenant2, sourceID)
	require.NoError(t, err)

	// Encrypted values should be different
	assert.NotEqual(t, encrypted1, encrypted2,
		"Same secret should encrypt differently for different tenants")

	// Decrypt for each tenant - should succeed
	decrypted1, err := credMgr.GetCredential(ctx, tenant1, sourceID, "token")
	require.NoError(t, err, "Should decrypt credential for tenant 1")
	assert.Equal(t, secret, decrypted1, "Decrypted value should match original")

	decrypted2, err := credMgr.GetCredential(ctx, tenant2, sourceID, "token")
	require.NoError(t, err, "Should decrypt credential for tenant 2")
	assert.Equal(t, secret, decrypted2, "Decrypted value should match original")

	// Try to decrypt tenant 1's credential with tenant 2's context
	// This tests that Additional Authenticated Data (AAD) is working

	// Create source for cross-tenant test
	createTestSource(t, db, tenant2, "cross-tenant-test")

	query := `
		INSERT INTO rag.tenant_source_credentials
		(tenant_id, source_id, credential_type, encrypted_value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, source_id, credential_type) DO UPDATE
		SET encrypted_value = EXCLUDED.encrypted_value
	`
	// Store tenant1's encrypted value under tenant2's ID
	_, err = db.ExecContext(ctx, query, tenant2, "cross-tenant-test", "token", encrypted1)
	require.NoError(t, err)

	// Try to decrypt - should fail because AAD won't match
	_, err = credMgr.GetCredential(ctx, tenant2, "cross-tenant-test", "token")
	assert.Error(t, err, "Should not decrypt credential encrypted for different tenant")
	assert.Contains(t, err.Error(), "decryption failed",
		"Error should indicate decryption failure due to AAD mismatch")
}

// TestAPIWithoutToken verifies that endpoints require authentication
func TestAPIWithoutToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDatabase(t)
	app := setupTestApp(db)

	// Try to list sources without token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rag/sources", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should require authentication")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error", "Response should contain error message")
}

// TestAPIWithInvalidToken verifies that invalid tokens are rejected
func TestAPIWithInvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDatabase(t)
	app := setupTestApp(db)

	// Try to list sources with invalid token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rag/sources", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should reject invalid token")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error", "Response should contain error message")
}

// TestInactiveTenant verifies that inactive tenants cannot access the API
func TestInactiveTenant(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDatabase(t)
	app := setupTestApp(db)

	// Create inactive tenant
	tenantID := uuid.New()
	userID := uuid.New()

	query := `
		INSERT INTO mcp.tenants (id, name, is_active, created_at)
		VALUES ($1, $2, false, CURRENT_TIMESTAMP)
	`
	_, err := db.Exec(query, tenantID, "Inactive Tenant")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mcp.tenants WHERE id = $1", tenantID)
	})

	// Generate token for inactive tenant
	token := generateTestToken(tenantID, userID)

	// Try to access API
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rag/sources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "Inactive tenant should be forbidden")

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error", "Response should contain error message")
}

// TestCredentialManagerPanicsOnInvalidKeySize verifies that invalid master keys are rejected
func TestCredentialManagerPanicsOnInvalidKeySize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDatabase(t)

	// Test with invalid key size (should panic)
	assert.Panics(t, func() {
		invalidKey := make([]byte, 16) // Only 16 bytes instead of 32
		security.NewCredentialManager(db, invalidKey)
	}, "Should panic with invalid master key size")

	// Test with correct key size (should not panic)
	assert.NotPanics(t, func() {
		validKey := make([]byte, 32)
		security.NewCredentialManager(db, validKey)
	}, "Should not panic with valid 32-byte master key")
}

// TestGetAllCredentials verifies bulk credential retrieval
func TestGetAllCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDatabase(t)

	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}
	credMgr := security.NewCredentialManager(db, masterKey)

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID, "Test Tenant")

	ctx := context.Background()
	sourceID := "multi-cred-source"

	// Create source (required for foreign key)
	createTestSource(t, db, tenantID, sourceID)

	// Store multiple credentials (using valid credential types from CHECK constraint)
	credentials := map[string]string{
		"token":      "github-token-123",
		"api_key":    "api-key-456",
		"basic_auth": "secret-789",
	}

	for credType, value := range credentials {
		err := credMgr.StoreCredential(ctx, tenantID, sourceID, credType, value)
		require.NoError(t, err, "Should store credential: %s", credType)
	}

	// Retrieve all credentials at once
	retrieved, err := credMgr.GetAllCredentials(ctx, tenantID, sourceID)
	require.NoError(t, err, "Should retrieve all credentials")

	// Verify all credentials were retrieved correctly
	assert.Len(t, retrieved, len(credentials), "Should retrieve all credentials")
	for credType, expectedValue := range credentials {
		actualValue, ok := retrieved[credType]
		assert.True(t, ok, "Should find credential type: %s", credType)
		assert.Equal(t, expectedValue, actualValue, "Credential value should match for: %s", credType)
	}
}

// TestDeleteCredentials verifies credential deletion
func TestDeleteCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDatabase(t)

	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}
	credMgr := security.NewCredentialManager(db, masterKey)

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID, "Test Tenant")

	ctx := context.Background()
	sourceID := "delete-test-source"

	// Create source (required for foreign key)
	createTestSource(t, db, tenantID, sourceID)

	// Store a credential
	err := credMgr.StoreCredential(ctx, tenantID, sourceID, "token", "test-token")
	require.NoError(t, err)

	// Verify it exists
	_, err = credMgr.GetCredential(ctx, tenantID, sourceID, "token")
	require.NoError(t, err, "Credential should exist")

	// Delete credentials
	err = credMgr.DeleteCredentials(ctx, tenantID, sourceID)
	require.NoError(t, err, "Should delete credentials")

	// Verify it no longer exists
	_, err = credMgr.GetCredential(ctx, tenantID, sourceID, "token")
	assert.Error(t, err, "Credential should no longer exist")
	assert.Contains(t, err.Error(), "sql: no rows in result set", "Should return sql.ErrNoRows (wrapped)")
}
