package cache

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// MigrationHelper assists with transitioning to tenant-aware cache
type MigrationHelper struct {
	legacy  *SemanticCache
	tenant  *TenantAwareCache
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewMigrationHelper creates a new migration helper
func NewMigrationHelper(
	legacy *SemanticCache,
	tenant *TenantAwareCache,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *MigrationHelper {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.migration")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &MigrationHelper{
		legacy:  legacy,
		tenant:  tenant,
		logger:  logger,
		metrics: metrics,
	}
}

// Get performs dual-read during migration
func (m *MigrationHelper) Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error) {
	tenantID := auth.GetTenantID(ctx)

	// Try tenant cache first
	if tenantID != uuid.Nil {
		entry, err := m.tenant.Get(ctx, query, embedding)
		if err == nil && entry != nil {
			m.metrics.IncrementCounterWithLabels("cache.migration.tenant_hit", 1, nil)
			return entry, nil
		}
	}

	// Fallback to legacy
	entry, err := m.legacy.Get(ctx, query, embedding)
	if err == nil && entry != nil {
		m.metrics.IncrementCounterWithLabels("cache.migration.legacy_hit", 1, nil)

		// Async copy to tenant cache
		if tenantID != uuid.Nil {
			go m.copyToTenantCache(context.Background(), tenantID, query, entry)
		}
	}

	return entry, err
}

// Set performs dual-write during migration
func (m *MigrationHelper) Set(ctx context.Context, query string, embedding []float32, results []CachedSearchResult) error {
	tenantID := auth.GetTenantID(ctx)

	var legacyErr, tenantErr error

	// Write to legacy cache
	legacyErr = m.legacy.Set(ctx, query, embedding, results)
	if legacyErr != nil {
		m.logger.Error("Failed to set in legacy cache", map[string]interface{}{
			"error": legacyErr.Error(),
			"query": query,
		})
	}

	// Write to tenant cache if tenant ID is available
	if tenantID != uuid.Nil {
		tenantErr = m.tenant.Set(ctx, query, embedding, results)
		if tenantErr != nil {
			m.logger.Error("Failed to set in tenant cache", map[string]interface{}{
				"error":     tenantErr.Error(),
				"query":     query,
				"tenant_id": tenantID.String(),
			})
		}
	}

	// Return tenant error if available, otherwise legacy error
	if tenantErr != nil {
		return tenantErr
	}
	return legacyErr
}

// Delete removes from both caches
func (m *MigrationHelper) Delete(ctx context.Context, query string) error {
	tenantID := auth.GetTenantID(ctx)

	// Delete from legacy
	if err := m.legacy.Delete(ctx, query); err != nil {
		m.logger.Warn("Failed to delete from legacy cache", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
	}

	// Delete from tenant cache if tenant ID is available
	if tenantID != uuid.Nil {
		if err := m.tenant.Delete(ctx, query); err != nil {
			m.logger.Warn("Failed to delete from tenant cache", map[string]interface{}{
				"error":     err.Error(),
				"query":     query,
				"tenant_id": tenantID.String(),
			})
		}
	}

	return nil
}

// copyToTenantCache copies an entry from legacy to tenant cache
func (m *MigrationHelper) copyToTenantCache(ctx context.Context, tenantID uuid.UUID, query string, entry *CacheEntry) {
	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Create a context with tenant ID
	tenantCtx := auth.WithTenantID(ctx, tenantID)

	// Copy to tenant cache
	if err := m.tenant.Set(tenantCtx, query, entry.Embedding, entry.Results); err != nil {
		m.logger.Error("Failed to copy to tenant cache", map[string]interface{}{
			"error":     err.Error(),
			"query":     query,
			"tenant_id": tenantID.String(),
		})
		m.metrics.IncrementCounterWithLabels("cache.migration.copy_failed", 1, map[string]string{
			"tenant_id": tenantID.String(),
		})
	} else {
		m.metrics.IncrementCounterWithLabels("cache.migration.copy_success", 1, map[string]string{
			"tenant_id": tenantID.String(),
		})
	}
}

// GetMigrationStats returns migration statistics
func (m *MigrationHelper) GetMigrationStats(ctx context.Context) map[string]interface{} {
	// This would typically query metrics to get:
	// - Number of legacy hits vs tenant hits
	// - Number of successful copies
	// - Number of failed copies
	// - Migration progress percentage

	return map[string]interface{}{
		"mode":           string(m.tenant.mode),
		"legacy_enabled": m.legacy != nil,
		"tenant_enabled": m.tenant != nil,
		// Additional stats would come from metrics
	}
}
