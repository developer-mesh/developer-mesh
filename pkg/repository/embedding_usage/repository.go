package embedding_usage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// UsageRecord represents a single embedding usage record
type UsageRecord struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	TenantID            uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	AgentID             *uuid.UUID `db:"agent_id" json:"agent_id,omitempty"`
	ModelID             uuid.UUID  `db:"model_id" json:"model_id"`
	TokensUsed          int        `db:"tokens_used" json:"tokens_used"`
	CharactersProcessed *int       `db:"characters_processed" json:"characters_processed,omitempty"`
	EmbeddingsGenerated int        `db:"embeddings_generated" json:"embeddings_generated"`
	ActualCost          float64    `db:"actual_cost" json:"actual_cost"`
	BilledCost          float64    `db:"billed_cost" json:"billed_cost"`
	LatencyMs           *int       `db:"latency_ms" json:"latency_ms,omitempty"`
	ProviderLatencyMs   *int       `db:"provider_latency_ms" json:"provider_latency_ms,omitempty"`
	RequestID           *uuid.UUID `db:"request_id" json:"request_id,omitempty"`
	TaskType            *string    `db:"task_type" json:"task_type,omitempty"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
}

// UsageSummary represents aggregated usage statistics
type UsageSummary struct {
	TenantID         uuid.UUID    `json:"tenant_id"`
	ModelID          *uuid.UUID   `json:"model_id,omitempty"`
	PeriodStart      time.Time    `json:"period_start"`
	PeriodEnd        time.Time    `json:"period_end"`
	TotalTokens      int64        `json:"total_tokens"`
	TotalCharacters  int64        `json:"total_characters"`
	TotalEmbeddings  int          `json:"total_embeddings"`
	TotalRequests    int          `json:"total_requests"`
	TotalCost        float64      `json:"total_cost"`
	TotalBilledCost  float64      `json:"total_billed_cost"`
	AverageLatencyMs float64      `json:"average_latency_ms"`
	ModelBreakdown   []ModelUsage `json:"model_breakdown,omitempty"`
}

// ModelUsage represents usage for a specific model
type ModelUsage struct {
	ModelID         uuid.UUID `json:"model_id"`
	ModelIdentifier string    `json:"model_identifier"`
	TokensUsed      int64     `json:"tokens_used"`
	RequestCount    int       `json:"request_count"`
	TotalCost       float64   `json:"total_cost"`
}

// EmbeddingUsageRepository defines the interface for usage tracking operations
type EmbeddingUsageRepository interface {
	// TrackUsage records a new usage entry
	TrackUsage(ctx context.Context, record *UsageRecord) error

	// GetUsageSummary returns aggregated usage for a tenant in a time period
	GetUsageSummary(ctx context.Context, tenantID uuid.UUID, start, end time.Time) (*UsageSummary, error)

	// GetUsageByModel returns usage breakdown by model for a tenant
	GetUsageByModel(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]ModelUsage, error)

	// GetUsageByAgent returns usage for a specific agent
	GetUsageByAgent(ctx context.Context, agentID uuid.UUID, start, end time.Time) (*UsageSummary, error)

	// GetCurrentMonthUsage returns usage for the current month
	GetCurrentMonthUsage(ctx context.Context, tenantID uuid.UUID) (*UsageSummary, error)

	// GetCurrentDayUsage returns usage for the current day
	GetCurrentDayUsage(ctx context.Context, tenantID uuid.UUID) (*UsageSummary, error)

	// BulkTrackUsage records multiple usage entries in a transaction
	BulkTrackUsage(ctx context.Context, records []*UsageRecord) error

	// PurgeOldRecords removes records older than the retention period
	PurgeOldRecords(ctx context.Context, retentionDays int) error
}

// EmbeddingUsageRepositoryImpl implements EmbeddingUsageRepository
type EmbeddingUsageRepositoryImpl struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewEmbeddingUsageRepository creates a new EmbeddingUsageRepository instance
func NewEmbeddingUsageRepository(db *sqlx.DB) EmbeddingUsageRepository {
	return &EmbeddingUsageRepositoryImpl{
		db:     db,
		logger: observability.NewStandardLogger("embedding_usage_repository"),
	}
}

// TrackUsage records a new usage entry using the SQL function
func (r *EmbeddingUsageRepositoryImpl) TrackUsage(ctx context.Context, record *UsageRecord) error {
	query := `SELECT mcp.track_embedding_usage($1, $2, $3, $4, $5, $6, $7)`

	var usageID uuid.UUID
	err := r.db.QueryRowContext(ctx, query,
		record.TenantID,
		record.AgentID,
		record.ModelID,
		record.TokensUsed,
		record.CharactersProcessed,
		record.LatencyMs,
		record.TaskType,
	).Scan(&usageID)

	if err != nil {
		return fmt.Errorf("failed to track usage: %w", err)
	}

	record.ID = usageID
	r.logger.Debug("Usage tracked", map[string]interface{}{
		"usage_id":  usageID,
		"tenant_id": record.TenantID,
		"tokens":    record.TokensUsed,
	})

	return nil
}

// GetUsageSummary returns aggregated usage for a tenant in a time period
func (r *EmbeddingUsageRepositoryImpl) GetUsageSummary(ctx context.Context, tenantID uuid.UUID, start, end time.Time) (*UsageSummary, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COALESCE(SUM(characters_processed), 0) as total_characters,
			COALESCE(SUM(embeddings_generated), 0) as total_embeddings,
			COALESCE(SUM(actual_cost), 0) as total_cost,
			COALESCE(SUM(billed_cost), 0) as total_billed_cost,
			COALESCE(AVG(latency_ms), 0) as average_latency_ms
		FROM mcp.embedding_usage_tracking
		WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3`

	summary := &UsageSummary{
		TenantID:    tenantID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	err := r.db.QueryRowContext(ctx, query, tenantID, start, end).Scan(
		&summary.TotalRequests,
		&summary.TotalTokens,
		&summary.TotalCharacters,
		&summary.TotalEmbeddings,
		&summary.TotalCost,
		&summary.TotalBilledCost,
		&summary.AverageLatencyMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage summary: %w", err)
	}

	// Get model breakdown
	modelUsage, err := r.GetUsageByModel(ctx, tenantID, start, end)
	if err == nil {
		summary.ModelBreakdown = modelUsage
	}

	return summary, nil
}

// GetUsageByModel returns usage breakdown by model for a tenant
func (r *EmbeddingUsageRepositoryImpl) GetUsageByModel(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]ModelUsage, error) {
	query := `
		SELECT 
			u.model_id,
			c.model_id as model_identifier,
			SUM(u.tokens_used) as tokens_used,
			COUNT(*) as request_count,
			SUM(u.actual_cost) as total_cost
		FROM mcp.embedding_usage_tracking u
		JOIN mcp.embedding_model_catalog c ON c.id = u.model_id
		WHERE u.tenant_id = $1 AND u.created_at >= $2 AND u.created_at < $3
		GROUP BY u.model_id, c.model_id
		ORDER BY tokens_used DESC`

	var usage []ModelUsage
	err := r.db.SelectContext(ctx, &usage, query, tenantID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage by model: %w", err)
	}

	return usage, nil
}

// GetUsageByAgent returns usage for a specific agent
func (r *EmbeddingUsageRepositoryImpl) GetUsageByAgent(ctx context.Context, agentID uuid.UUID, start, end time.Time) (*UsageSummary, error) {
	query := `
		SELECT 
			tenant_id,
			COUNT(*) as total_requests,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COALESCE(SUM(characters_processed), 0) as total_characters,
			COALESCE(SUM(embeddings_generated), 0) as total_embeddings,
			COALESCE(SUM(actual_cost), 0) as total_cost,
			COALESCE(SUM(billed_cost), 0) as total_billed_cost,
			COALESCE(AVG(latency_ms), 0) as average_latency_ms
		FROM mcp.embedding_usage_tracking
		WHERE agent_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY tenant_id`

	summary := &UsageSummary{
		PeriodStart: start,
		PeriodEnd:   end,
	}

	err := r.db.QueryRowContext(ctx, query, agentID, start, end).Scan(
		&summary.TenantID,
		&summary.TotalRequests,
		&summary.TotalTokens,
		&summary.TotalCharacters,
		&summary.TotalEmbeddings,
		&summary.TotalCost,
		&summary.TotalBilledCost,
		&summary.AverageLatencyMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent usage: %w", err)
	}

	return summary, nil
}

// GetCurrentMonthUsage returns usage for the current month
func (r *EmbeddingUsageRepositoryImpl) GetCurrentMonthUsage(ctx context.Context, tenantID uuid.UUID) (*UsageSummary, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	return r.GetUsageSummary(ctx, tenantID, start, end)
}

// GetCurrentDayUsage returns usage for the current day
func (r *EmbeddingUsageRepositoryImpl) GetCurrentDayUsage(ctx context.Context, tenantID uuid.UUID) (*UsageSummary, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	return r.GetUsageSummary(ctx, tenantID, start, end)
}

// BulkTrackUsage records multiple usage entries in a transaction
func (r *EmbeddingUsageRepositoryImpl) BulkTrackUsage(ctx context.Context, records []*UsageRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			r.logger.Warn("Failed to rollback transaction", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `SELECT mcp.track_embedding_usage($1, $2, $3, $4, $5, $6, $7)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			r.logger.Warn("Failed to close statement", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	for _, record := range records {
		var usageID uuid.UUID
		err := stmt.QueryRowContext(ctx,
			record.TenantID,
			record.AgentID,
			record.ModelID,
			record.TokensUsed,
			record.CharactersProcessed,
			record.LatencyMs,
			record.TaskType,
		).Scan(&usageID)

		if err != nil {
			return fmt.Errorf("failed to track usage for record: %w", err)
		}
		record.ID = usageID
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("Bulk usage tracked", map[string]interface{}{
		"count": len(records),
	})

	return nil
}

// PurgeOldRecords removes records older than the retention period
func (r *EmbeddingUsageRepositoryImpl) PurgeOldRecords(ctx context.Context, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	query := `DELETE FROM mcp.embedding_usage_tracking WHERE created_at < $1`

	result, err := r.db.ExecContext(ctx, query, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to purge old records: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("Old usage records purged", map[string]interface{}{
		"rows_deleted":   rows,
		"retention_days": retentionDays,
		"cutoff_date":    cutoffDate,
	})

	return nil
}
