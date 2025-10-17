package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/models"
)

// SourceRepository handles tenant-aware source data access
type SourceRepository struct {
	db *sqlx.DB
}

// NewSourceRepository creates a new source repository instance
func NewSourceRepository(db *sqlx.DB) *SourceRepository {
	return &SourceRepository{db: db}
}

// BeginTx begins a new database transaction
func (r *SourceRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// CreateSource creates a new tenant source configuration
func (r *SourceRepository) CreateSource(ctx context.Context, tx *sqlx.Tx, source *models.TenantSource) error {
	query := `
		INSERT INTO rag.tenant_sources (
			id, tenant_id, source_id, source_type,
			config, schedule, enabled, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.ExecContext(ctx, query,
			source.ID, source.TenantID, source.SourceID, source.SourceType,
			source.Config, source.Schedule, source.Enabled, source.CreatedBy)
	} else {
		result, err = r.db.ExecContext(ctx, query,
			source.ID, source.TenantID, source.SourceID, source.SourceType,
			source.Config, source.Schedule, source.Enabled, source.CreatedBy)
	}

	if err != nil {
		return fmt.Errorf("failed to insert source: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows inserted")
	}

	return nil
}

// ListSourcesByTenant retrieves all sources for a tenant
func (r *SourceRepository) ListSourcesByTenant(ctx context.Context, tenantID uuid.UUID) ([]*models.TenantSource, error) {
	// Query with explicit tenant filter for application-level tenant isolation
	query := `
		SELECT
			id, tenant_id, source_id, source_type,
			config, schedule, enabled, sync_status,
			last_sync_at, next_sync_at, sync_error_count,
			created_at, updated_at, created_by, updated_by
		FROM rag.tenant_sources
		WHERE tenant_id = $1
		ORDER BY created_at DESC`

	var sources []*models.TenantSource
	err := r.db.SelectContext(ctx, &sources, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	return sources, nil
}

// GetSource retrieves a specific source by tenant and source ID
func (r *SourceRepository) GetSource(ctx context.Context, tenantID uuid.UUID, sourceID string) (*models.TenantSource, error) {
	var source models.TenantSource
	query := `
		SELECT
			id, tenant_id, source_id, source_type,
			config, schedule, enabled, sync_status,
			last_sync_at, next_sync_at, sync_error_count,
			created_at, updated_at, created_by, updated_by
		FROM rag.tenant_sources
		WHERE tenant_id = $1 AND source_id = $2`

	err := r.db.GetContext(ctx, &source, query, tenantID, sourceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("source not found")
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return &source, nil
}

// GetEnabledSourcesByTenant retrieves all enabled sources for a tenant
func (r *SourceRepository) GetEnabledSourcesByTenant(ctx context.Context, tenantID uuid.UUID) ([]*models.TenantSource, error) {
	query := `
		SELECT
			id, tenant_id, source_id, source_type,
			config, schedule, enabled, sync_status,
			last_sync_at, next_sync_at, sync_error_count,
			created_at, updated_at, created_by, updated_by
		FROM rag.tenant_sources
		WHERE tenant_id = $1 AND enabled = true
		ORDER BY created_at DESC`

	var sources []*models.TenantSource
	err := r.db.SelectContext(ctx, &sources, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled sources: %w", err)
	}

	return sources, nil
}

// UpdateSource updates a source configuration
func (r *SourceRepository) UpdateSource(ctx context.Context, source *models.TenantSource) error {
	query := `
		UPDATE rag.tenant_sources
		SET
			config = :config,
			schedule = :schedule,
			enabled = :enabled,
			sync_status = :sync_status,
			last_sync_at = :last_sync_at,
			next_sync_at = :next_sync_at,
			sync_error_count = :sync_error_count,
			updated_by = :updated_by
		WHERE tenant_id = :tenant_id AND source_id = :source_id`

	result, err := r.db.NamedExecContext(ctx, query, source)
	if err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("source not found")
	}

	return nil
}

// DeleteSource deletes a source and cascades to credentials and documents
func (r *SourceRepository) DeleteSource(ctx context.Context, tenantID uuid.UUID, sourceID string) error {
	// CASCADE DELETE will automatically remove:
	// - tenant_source_credentials
	// - tenant_documents
	// - tenant_sync_jobs
	query := `
		DELETE FROM rag.tenant_sources
		WHERE tenant_id = $1 AND source_id = $2`

	result, err := r.db.ExecContext(ctx, query, tenantID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("source not found")
	}

	return nil
}

// CreateSyncJob creates a new sync job record
func (r *SourceRepository) CreateSyncJob(ctx context.Context, job *models.TenantSyncJob) error {
	query := `
		INSERT INTO rag.tenant_sync_jobs (
			id, tenant_id, source_id, job_type, status, priority
		) VALUES (
			:id, :tenant_id, :source_id, :job_type, :status, :priority
		)`

	_, err := r.db.NamedExecContext(ctx, query, job)
	if err != nil {
		return fmt.Errorf("failed to create sync job: %w", err)
	}

	return nil
}

// UpdateSyncJob updates a sync job's status and statistics
func (r *SourceRepository) UpdateSyncJob(ctx context.Context, job *models.TenantSyncJob) error {
	query := `
		UPDATE rag.tenant_sync_jobs
		SET
			status = :status,
			started_at = :started_at,
			completed_at = :completed_at,
			documents_processed = :documents_processed,
			documents_added = :documents_added,
			documents_updated = :documents_updated,
			documents_deleted = :documents_deleted,
			chunks_created = :chunks_created,
			errors_count = :errors_count,
			error_message = :error_message,
			error_details = :error_details,
			duration_ms = :duration_ms,
			memory_used_mb = :memory_used_mb
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, job)
	if err != nil {
		return fmt.Errorf("failed to update sync job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("sync job not found")
	}

	return nil
}

// GetSyncJob retrieves a sync job by ID
func (r *SourceRepository) GetSyncJob(ctx context.Context, tenantID uuid.UUID, jobID uuid.UUID) (*models.TenantSyncJob, error) {
	var job models.TenantSyncJob
	query := `
		SELECT
			id, tenant_id, source_id, job_type, status, priority,
			started_at, completed_at,
			documents_processed, documents_added, documents_updated,
			documents_deleted, chunks_created, errors_count,
			error_message, error_details, duration_ms, memory_used_mb,
			created_at
		FROM rag.tenant_sync_jobs
		WHERE tenant_id = $1 AND id = $2`

	err := r.db.GetContext(ctx, &job, query, tenantID, jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sync job not found")
		}
		return nil, fmt.Errorf("failed to get sync job: %w", err)
	}

	return &job, nil
}

// ListSyncJobs lists sync jobs for a source with pagination
func (r *SourceRepository) ListSyncJobs(ctx context.Context, tenantID uuid.UUID, sourceID string, limit int) ([]*models.TenantSyncJob, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			id, tenant_id, source_id, job_type, status, priority,
			started_at, completed_at,
			documents_processed, documents_added, documents_updated,
			documents_deleted, chunks_created, errors_count,
			error_message, error_details, duration_ms, memory_used_mb,
			created_at
		FROM rag.tenant_sync_jobs
		WHERE tenant_id = $1 AND source_id = $2
		ORDER BY created_at DESC
		LIMIT $3`

	var jobs []*models.TenantSyncJob
	err := r.db.SelectContext(ctx, &jobs, query, tenantID, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sync jobs: %w", err)
	}

	return jobs, nil
}

// GetSourceCredentials retrieves all credentials for a source
func (r *SourceRepository) GetSourceCredentials(ctx context.Context, tenantID uuid.UUID, sourceID string) ([]*models.TenantSourceCredential, error) {
	query := `
		SELECT
			id, tenant_id, source_id, credential_type,
			encrypted_value, kms_key_id, expires_at,
			last_rotated_at, created_at, updated_at
		FROM rag.tenant_source_credentials
		WHERE tenant_id = $1 AND source_id = $2`

	var creds []*models.TenantSourceCredential
	err := r.db.SelectContext(ctx, &creds, query, tenantID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	return creds, nil
}

// GetQueuedJobs retrieves all jobs with status "queued" ordered by priority and creation time
func (r *SourceRepository) GetQueuedJobs(ctx context.Context, limit int) ([]*models.TenantSyncJob, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			id, tenant_id, source_id, job_type, status, priority,
			started_at, completed_at,
			documents_processed, documents_added, documents_updated,
			documents_deleted, chunks_created, errors_count,
			error_message, error_details, duration_ms, memory_used_mb,
			created_at
		FROM rag.tenant_sync_jobs
		WHERE status = 'queued'
		ORDER BY priority DESC, created_at ASC
		LIMIT $1`

	var jobs []*models.TenantSyncJob
	err := r.db.SelectContext(ctx, &jobs, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get queued jobs: %w", err)
	}

	return jobs, nil
}
