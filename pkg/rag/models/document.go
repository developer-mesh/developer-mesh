// Package models defines core data models for the RAG system
package models

import (
	"time"

	"github.com/google/uuid"
)

// Document represents a document in the RAG system
type Document struct {
	ID          uuid.UUID              `json:"id" db:"id"`
	TenantID    uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	SourceID    string                 `json:"source_id" db:"source_id"`
	SourceType  string                 `json:"source_type" db:"source_type"`
	URL         string                 `json:"url" db:"url"`
	Title       string                 `json:"title" db:"title"`
	Content     string                 `json:"content"`
	ContentHash string                 `json:"content_hash" db:"content_hash"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`

	// Scoring fields for relevance ranking
	BaseScore       float64 `json:"base_score"`
	FreshnessScore  float64 `json:"freshness_score"`
	AuthorityScore  float64 `json:"authority_score"`
	PopularityScore float64 `json:"popularity_score"`
	QualityScore    float64 `json:"quality_score"`
	ImportanceScore float64 `json:"importance_score"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Chunk represents a chunk of a document
type Chunk struct {
	ID          uuid.UUID              `json:"id" db:"id"`
	DocumentID  uuid.UUID              `json:"document_id" db:"document_id"`
	ChunkIndex  int                    `json:"chunk_index" db:"chunk_index"`
	Content     string                 `json:"content" db:"content"`
	StartChar   int                    `json:"start_char" db:"start_char"`
	EndChar     int                    `json:"end_char" db:"end_char"`
	EmbeddingID *uuid.UUID             `json:"embedding_id" db:"embedding_id"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`

	// For in-memory processing (not stored in DB)
	Embedding []float32 `json:"-" db:"-"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// IngestionJob represents an ingestion job
type IngestionJob struct {
	ID                 uuid.UUID              `json:"id" db:"id"`
	TenantID           uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	SourceID           string                 `json:"source_id" db:"source_id"`
	Status             string                 `json:"status" db:"status"`
	StartedAt          *time.Time             `json:"started_at" db:"started_at"`
	CompletedAt        *time.Time             `json:"completed_at" db:"completed_at"`
	DocumentsProcessed int                    `json:"documents_processed" db:"documents_processed"`
	ChunksCreated      int                    `json:"chunks_created" db:"chunks_created"`
	EmbeddingsCreated  int                    `json:"embeddings_created" db:"embeddings_created"`
	ErrorMessage       string                 `json:"error_message" db:"error_message"`
	Metadata           map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt          time.Time              `json:"created_at" db:"created_at"`
}

// IngestionStatus represents the status of an ingestion job
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// SourceType represents the type of data source
const (
	SourceTypeGitHub     = "github"
	SourceTypeWeb        = "web"
	SourceTypeS3         = "s3"
	SourceTypeLocal      = "local"
	SourceTypeConfluence = "confluence"
	SourceTypeJira       = "jira"
)
