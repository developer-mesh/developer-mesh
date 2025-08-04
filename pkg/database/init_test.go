package database

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeTables(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(mock sqlmock.Sqlmock)
		expectedError string
	}{
		{
			name: "successful initialization - no-op implementation",
			setupMock: func(mock sqlmock.Sqlmock) {
				// InitializeTables is now a no-op function that delegates to migrations
				// No database queries are expected
			},
		},
		{
			name: "multiple calls are idempotent",
			setupMock: func(mock sqlmock.Sqlmock) {
				// No database queries are expected even with multiple calls
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = mockDB.Close() }()

			// Setup expectations
			tt.setupMock(mock)

			// Create Database instance
			db := &Database{
				db: sqlx.NewDb(mockDB, "postgres"),
			}

			// Execute InitializeTables
			err = db.InitializeTables(context.Background())

			// Check expectations
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)

				// For idempotent test, call again
				if tt.name == "multiple calls are idempotent" {
					err = db.InitializeTables(context.Background())
					assert.NoError(t, err)
				}
			}

			// Verify all expectations were met
			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestInitializeTables_NoTableCreation(t *testing.T) {
	// This test verifies that InitializeTables does NOT create tables
	// Tables should only be created by migrations
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	// No expectations - InitializeTables should not make any database queries
	// If InitializeTables tries to query or create tables, this test will fail

	db := &Database{
		db: sqlx.NewDb(mockDB, "postgres"),
	}

	err = db.InitializeTables(context.Background())
	assert.NoError(t, err)

	// Verify all expectations were met and no extra queries were executed
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestInitializeTables_Documentation(t *testing.T) {
	// This test documents the behavior of InitializeTables
	t.Run("delegates to migrations", func(t *testing.T) {
		// InitializeTables is a no-op function that relies on database migrations
		// to create and manage schema. This design ensures:
		// 1. Schema is version controlled through migration files
		// 2. Schema changes are applied consistently across environments
		// 3. No hardcoded DDL statements in application code
		// 4. Clean separation of concerns between app and schema management

		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = mockDB.Close() }()

		db := &Database{
			db: sqlx.NewDb(mockDB, "postgres"),
		}

		// Should complete without any database operations
		err = db.InitializeTables(context.Background())
		assert.NoError(t, err)

		// Verify no unexpected database operations occurred
		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}
