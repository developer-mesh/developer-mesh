package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

func TestDocumentRepository_CreateDocument(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close mock db: %v", closeErr)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	repo := NewDocumentRepository(sqlxDB)

	ctx := context.Background()
	doc := &models.Document{
		ID:          uuid.New(),
		TenantID:    uuid.New(),
		SourceID:    "github_main",
		SourceType:  models.SourceTypeGitHub,
		URL:         "https://github.com/test/repo",
		Title:       "README.md",
		ContentHash: "abc123",
		Metadata:    map[string]interface{}{"branch": "main"},
	}

	// Convert metadata to JSON for matching
	metadataJSON, err := json.Marshal(doc.Metadata)
	assert.NoError(t, err)

	// Expect the insert query
	mock.ExpectExec("INSERT INTO rag.documents").
		WithArgs(
			doc.ID, doc.TenantID, doc.SourceID, doc.SourceType,
			doc.URL, doc.Title, doc.ContentHash, metadataJSON,
			sqlmock.AnyArg(), sqlmock.AnyArg(), // timestamps
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute
	err = repo.CreateDocument(ctx, doc)
	assert.NoError(t, err)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDocumentRepository_DocumentExists(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close mock db: %v", closeErr)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	repo := NewDocumentRepository(sqlxDB)

	ctx := context.Background()
	hash := "abc123"

	// Test case: document exists
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(hash).
		WillReturnRows(rows)

	exists, err := repo.DocumentExists(ctx, hash)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDocumentRepository_CreateIngestionJob(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close mock db: %v", closeErr)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	repo := NewDocumentRepository(sqlxDB)

	ctx := context.Background()
	now := time.Now()
	job := &models.IngestionJob{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		SourceID:  "github_main",
		Status:    models.StatusPending,
		StartedAt: &now,
		Metadata:  map[string]interface{}{"trigger": "manual"},
	}

	// Convert metadata to JSON for matching
	metadataJSON, err := json.Marshal(job.Metadata)
	assert.NoError(t, err)

	// Expect the insert query
	mock.ExpectExec("INSERT INTO rag.ingestion_jobs").
		WithArgs(
			job.ID, job.TenantID, job.SourceID, job.Status,
			job.StartedAt, metadataJSON, sqlmock.AnyArg(), // created_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute
	err = repo.CreateIngestionJob(ctx, job)
	assert.NoError(t, err)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDocumentRepository_UpdateIngestionJob(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close mock db: %v", closeErr)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	repo := NewDocumentRepository(sqlxDB)

	ctx := context.Background()
	now := time.Now()
	job := &models.IngestionJob{
		ID:                 uuid.New(),
		Status:             models.StatusCompleted,
		CompletedAt:        &now,
		DocumentsProcessed: 10,
		ChunksCreated:      50,
		EmbeddingsCreated:  50,
		ErrorMessage:       "",
	}

	// Expect the update query
	mock.ExpectExec("UPDATE rag.ingestion_jobs").
		WithArgs(
			job.Status, job.CompletedAt, job.DocumentsProcessed,
			job.ChunksCreated, job.EmbeddingsCreated,
			job.ErrorMessage, job.ID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err = repo.UpdateIngestionJob(ctx, job)
	assert.NoError(t, err)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
