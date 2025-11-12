//go:build integration

package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// getTestDatabaseDSN returns the database DSN from environment variables
func getTestDatabaseDSN() string {
	// Look for the environment variable MCP_DATABASE_DSN
	dsn := os.Getenv("MCP_DATABASE_DSN")
	if dsn == "" {
		// Default test database DSN for development
		dsn = "postgres://postgres:postgres@localhost:5432/mcp_test?sslmode=disable"
	}
	return dsn
}

// SkipIfNoDatabase skips the test if database connection cannot be established
func SkipIfNoDatabase(t *testing.T) {
	if os.Getenv("ENABLE_INTEGRATION_TESTS") != "true" && os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set ENABLE_INTEGRATION_TESTS=true or INTEGRATION_TEST=true to run.")
	}
}

// CreateTestDatabaseConnection creates a standardized database connection for tests
// with consistent connection pool settings
func CreateTestDatabaseConnection(t *testing.T) *sqlx.DB {
	// Get database DSN from environment
	dsn := getTestDatabaseDSN()

	// Connect to the database
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db
}

// CleanupTestDatabase performs cleanup operations on the test database
func CleanupTestDatabase(t *testing.T, db *sqlx.DB, contextID string) {
	if contextID != "" {
		// Delete test context data if applicable
		_, err := db.Exec(fmt.Sprintf("DELETE FROM embeddings WHERE context_id = '%s'", contextID))
		if err != nil {
			t.Logf("Warning: Failed to clean up test data for context %s: %v", contextID, err)
		}
	}

	// Close the database connection
	if err := db.Close(); err != nil {
		t.Logf("Warning: Failed to close database connection: %v", err)
	}
}
