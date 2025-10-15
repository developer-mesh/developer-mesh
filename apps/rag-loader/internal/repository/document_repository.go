// Package repository implements data access for the RAG loader
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// DocumentRepository handles document data access
type DocumentRepository struct {
	db *sqlx.DB
}

// NewDocumentRepository creates a new document repository
func NewDocumentRepository(db *sqlx.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// CreateDocument creates a new document record
func (r *DocumentRepository) CreateDocument(ctx context.Context, doc *models.Document) error {
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = time.Now()
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO rag.documents (
			id, tenant_id, source_id, source_type, url, title,
			content_hash, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err = r.db.ExecContext(ctx, query,
		doc.ID, doc.TenantID, doc.SourceID, doc.SourceType,
		doc.URL, doc.Title, doc.ContentHash, metadataJSON,
		doc.CreatedAt, doc.UpdatedAt,
	)
	if err != nil {
		// Check for unique constraint violation on content_hash
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return fmt.Errorf("document already exists with hash %s", doc.ContentHash)
		}
		return fmt.Errorf("failed to create document: %w", err)
	}

	return nil
}

// GetDocument retrieves a document by ID
func (r *DocumentRepository) GetDocument(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	var doc models.Document
	query := `
		SELECT id, tenant_id, source_id, source_type, url, title,
		       content_hash, metadata, created_at, updated_at
		FROM rag.documents
		WHERE id = $1`

	err := r.db.GetContext(ctx, &doc, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &doc, nil
}

// GetDocumentByHash retrieves a document by content hash
func (r *DocumentRepository) GetDocumentByHash(ctx context.Context, hash string) (*models.Document, error) {
	var doc models.Document
	query := `
		SELECT id, tenant_id, source_id, source_type, url, title,
		       content_hash, metadata, created_at, updated_at
		FROM rag.documents
		WHERE content_hash = $1`

	err := r.db.GetContext(ctx, &doc, query, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Document doesn't exist
		}
		return nil, fmt.Errorf("failed to get document by hash: %w", err)
	}

	return &doc, nil
}

// DocumentExists checks if a document exists by content hash
func (r *DocumentRepository) DocumentExists(ctx context.Context, hash string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM rag.documents WHERE content_hash = $1)`

	err := r.db.GetContext(ctx, &exists, query, hash)
	if err != nil {
		return false, fmt.Errorf("failed to check document existence: %w", err)
	}

	return exists, nil
}

// CreateChunk creates a new document chunk
func (r *DocumentRepository) CreateChunk(ctx context.Context, chunk *models.Chunk) error {
	if chunk.ID == uuid.Nil {
		chunk.ID = uuid.New()
	}
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now()
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(chunk.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO rag.document_chunks (
			id, document_id, chunk_index, content, start_char, end_char,
			embedding_id, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)`

	_, err = r.db.ExecContext(ctx, query,
		chunk.ID, chunk.DocumentID, chunk.ChunkIndex, chunk.Content,
		chunk.StartChar, chunk.EndChar, chunk.EmbeddingID,
		metadataJSON, chunk.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create chunk: %w", err)
	}

	return nil
}

// UpdateChunkEmbedding updates the embedding ID for a chunk
func (r *DocumentRepository) UpdateChunkEmbedding(ctx context.Context, chunkID uuid.UUID, embeddingID uuid.UUID) error {
	query := `
		UPDATE rag.document_chunks
		SET embedding_id = $1
		WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, embeddingID, chunkID)
	if err != nil {
		return fmt.Errorf("failed to update chunk embedding: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("chunk not found: %s", chunkID)
	}

	return nil
}

// GetChunksByDocument retrieves all chunks for a document
func (r *DocumentRepository) GetChunksByDocument(ctx context.Context, documentID uuid.UUID) ([]*models.Chunk, error) {
	var chunks []*models.Chunk
	query := `
		SELECT id, document_id, chunk_index, content, start_char, end_char,
		       embedding_id, metadata, created_at
		FROM rag.document_chunks
		WHERE document_id = $1
		ORDER BY chunk_index`

	err := r.db.SelectContext(ctx, &chunks, query, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}

	return chunks, nil
}

// CreateIngestionJob creates a new ingestion job
func (r *DocumentRepository) CreateIngestionJob(ctx context.Context, job *models.IngestionJob) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(job.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO rag.ingestion_jobs (
			id, tenant_id, source_id, status, started_at, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)`

	_, err = r.db.ExecContext(ctx, query,
		job.ID, job.TenantID, job.SourceID, job.Status,
		job.StartedAt, metadataJSON, job.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create ingestion job: %w", err)
	}

	return nil
}

// UpdateIngestionJob updates an ingestion job's status and statistics
func (r *DocumentRepository) UpdateIngestionJob(ctx context.Context, job *models.IngestionJob) error {
	query := `
		UPDATE rag.ingestion_jobs
		SET status = $1,
		    completed_at = $2,
		    documents_processed = $3,
		    chunks_created = $4,
		    embeddings_created = $5,
		    error_message = $6
		WHERE id = $7`

	result, err := r.db.ExecContext(ctx, query,
		job.Status, job.CompletedAt, job.DocumentsProcessed,
		job.ChunksCreated, job.EmbeddingsCreated,
		job.ErrorMessage, job.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update ingestion job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ingestion job not found: %s", job.ID)
	}

	return nil
}

// GetIngestionJob retrieves an ingestion job by ID
func (r *DocumentRepository) GetIngestionJob(ctx context.Context, id uuid.UUID) (*models.IngestionJob, error) {
	var job models.IngestionJob
	query := `
		SELECT id, tenant_id, source_id, status, started_at, completed_at,
		       documents_processed, chunks_created, embeddings_created,
		       error_message, metadata, created_at
		FROM rag.ingestion_jobs
		WHERE id = $1`

	err := r.db.GetContext(ctx, &job, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ingestion job not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get ingestion job: %w", err)
	}

	return &job, nil
}

// GetLastSuccessfulIngestion gets the last successful ingestion for a source
func (r *DocumentRepository) GetLastSuccessfulIngestion(ctx context.Context, sourceID string) (*models.IngestionJob, error) {
	var job models.IngestionJob
	query := `
		SELECT id, tenant_id, source_id, status, started_at, completed_at,
		       documents_processed, chunks_created, embeddings_created,
		       error_message, metadata, created_at
		FROM rag.ingestion_jobs
		WHERE source_id = $1 AND status = $2
		ORDER BY completed_at DESC
		LIMIT 1`

	err := r.db.GetContext(ctx, &job, query, sourceID, models.StatusCompleted)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No successful ingestion found
		}
		return nil, fmt.Errorf("failed to get last successful ingestion: %w", err)
	}

	return &job, nil
}
