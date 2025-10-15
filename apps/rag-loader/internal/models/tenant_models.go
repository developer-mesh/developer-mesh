package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TenantSource represents a tenant's data source configuration
type TenantSource struct {
	ID             uuid.UUID       `db:"id" json:"id"`
	TenantID       uuid.UUID       `db:"tenant_id" json:"tenant_id"`
	SourceID       string          `db:"source_id" json:"source_id"`
	SourceType     string          `db:"source_type" json:"source_type"`
	Enabled        bool            `db:"enabled" json:"enabled"`
	Schedule       string          `db:"schedule" json:"schedule,omitempty"`
	Config         json.RawMessage `db:"config" json:"config"`
	LastSyncAt     *time.Time      `db:"last_sync_at" json:"last_sync_at,omitempty"`
	NextSyncAt     *time.Time      `db:"next_sync_at" json:"next_sync_at,omitempty"`
	SyncStatus     string          `db:"sync_status" json:"sync_status"`
	SyncErrorCount int             `db:"sync_error_count" json:"sync_error_count"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at" json:"updated_at"`
	CreatedBy      uuid.UUID       `db:"created_by" json:"created_by,omitempty"`
	UpdatedBy      *uuid.UUID      `db:"updated_by" json:"updated_by,omitempty"`
}

// TenantSourceCredential represents encrypted credentials for a source
type TenantSourceCredential struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	TenantID       uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	SourceID       string     `db:"source_id" json:"source_id"`
	CredentialType string     `db:"credential_type" json:"credential_type"`
	EncryptedValue string     `db:"encrypted_value" json:"-"` // Never expose in JSON
	KMSKeyID       *string    `db:"kms_key_id" json:"kms_key_id,omitempty"`
	ExpiresAt      *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	LastRotatedAt  time.Time  `db:"last_rotated_at" json:"last_rotated_at"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

// TenantDocument represents a document in the RAG system
type TenantDocument struct {
	ID              uuid.UUID              `db:"id" json:"id"`
	TenantID        uuid.UUID              `db:"tenant_id" json:"tenant_id"`
	SourceID        string                 `db:"source_id" json:"source_id"`
	DocumentID      string                 `db:"document_id" json:"document_id"`
	ParentID        *string                `db:"parent_id" json:"parent_id,omitempty"`
	DocumentType    string                 `db:"document_type" json:"document_type"`
	Title           *string                `db:"title" json:"title,omitempty"`
	Content         string                 `db:"content" json:"content"`
	ContentHash     string                 `db:"content_hash" json:"content_hash"`
	Metadata        map[string]interface{} `db:"metadata" json:"metadata"`
	ChunkIndex      int                    `db:"chunk_index" json:"chunk_index"`
	ChunkTotal      int                    `db:"chunk_total" json:"chunk_total"`
	TokenCount      *int                   `db:"token_count" json:"token_count,omitempty"`
	Language        *string                `db:"language" json:"language,omitempty"`
	ImportanceScore float64                `db:"importance_score" json:"importance_score"`
	IndexedAt       time.Time              `db:"indexed_at" json:"indexed_at"`
	CreatedAt       time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time              `db:"updated_at" json:"updated_at"`
}

// TenantSyncJob represents a sync job for a tenant source
type TenantSyncJob struct {
	ID                 uuid.UUID        `db:"id" json:"id"`
	TenantID           uuid.UUID        `db:"tenant_id" json:"tenant_id"`
	SourceID           string           `db:"source_id" json:"source_id"`
	JobType            string           `db:"job_type" json:"job_type"`
	Status             string           `db:"status" json:"status"`
	Priority           int              `db:"priority" json:"priority"`
	StartedAt          *time.Time       `db:"started_at" json:"started_at,omitempty"`
	CompletedAt        *time.Time       `db:"completed_at" json:"completed_at,omitempty"`
	DocumentsProcessed int              `db:"documents_processed" json:"documents_processed"`
	DocumentsAdded     int              `db:"documents_added" json:"documents_added"`
	DocumentsUpdated   int              `db:"documents_updated" json:"documents_updated"`
	DocumentsDeleted   int              `db:"documents_deleted" json:"documents_deleted"`
	ChunksCreated      int              `db:"chunks_created" json:"chunks_created"`
	ErrorsCount        int              `db:"errors_count" json:"errors_count"`
	ErrorMessage       *string          `db:"error_message" json:"error_message,omitempty"`
	ErrorDetails       *json.RawMessage `db:"error_details" json:"error_details,omitempty"`
	DurationMs         *int             `db:"duration_ms" json:"duration_ms,omitempty"`
	MemoryUsedMb       *int             `db:"memory_used_mb" json:"memory_used_mb,omitempty"`
	CreatedAt          time.Time        `db:"created_at" json:"created_at"`
}
