package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// HealthCheckDBImpl implements the HealthCheckDB interface
type HealthCheckDBImpl struct {
	db            *sql.DB
	encryptionSvc *security.EncryptionService
}

// NewHealthCheckDBImpl creates a new health check database implementation
func NewHealthCheckDBImpl(db *sql.DB, encryptionSvc *security.EncryptionService) *HealthCheckDBImpl {
	return &HealthCheckDBImpl{
		db:            db,
		encryptionSvc: encryptionSvc,
	}
}

// GetActiveToolsForHealthCheck retrieves all active tools that need health checking
func (h *HealthCheckDBImpl) GetActiveToolsForHealthCheck(ctx context.Context) ([]tools.ToolConfig, error) {
	query := `
		SELECT 
			id, tenant_id, tool_name, base_url, documentation_url, 
			openapi_url, auth_type, encrypted_credentials, config, 
			retry_policy, health_config, created_at, updated_at
		FROM tool_configurations
		WHERE status = 'active'
		AND health_config IS NOT NULL
		AND health_config->>'mode' != 'disabled'
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active tools: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore error on close
	}()

	var configs []tools.ToolConfig
	for rows.Next() {
		var (
			config               tools.ToolConfig
			encryptedCreds       sql.NullString
			configJSON           sql.NullString
			retryPolicyJSON      sql.NullString
			healthConfigJSON     sql.NullString
			authType             sql.NullString
			documentationURL     sql.NullString
			openAPIURL           sql.NullString
			createdAt, updatedAt time.Time
		)

		err := rows.Scan(
			&config.ID,
			&config.TenantID,
			&config.Name,
			&config.BaseURL,
			&documentationURL,
			&openAPIURL,
			&authType,
			&encryptedCreds,
			&configJSON,
			&retryPolicyJSON,
			&healthConfigJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool row: %w", err)
		}

		// Set optional fields
		if documentationURL.Valid {
			config.DocumentationURL = documentationURL.String
		}
		if openAPIURL.Valid {
			config.OpenAPIURL = openAPIURL.String
		}

		// Parse JSON fields
		if configJSON.Valid {
			if err := json.Unmarshal([]byte(configJSON.String), &config.Config); err != nil {
				return nil, fmt.Errorf("failed to parse config JSON: %w", err)
			}
		} else {
			config.Config = make(map[string]interface{})
		}

		if retryPolicyJSON.Valid {
			var retryPolicy tools.ToolRetryPolicy
			if err := json.Unmarshal([]byte(retryPolicyJSON.String), &retryPolicy); err != nil {
				return nil, fmt.Errorf("failed to parse retry policy JSON: %w", err)
			}
			config.RetryPolicy = &retryPolicy
		}

		if healthConfigJSON.Valid {
			var healthConfig tools.HealthCheckConfig
			if err := json.Unmarshal([]byte(healthConfigJSON.String), &healthConfig); err != nil {
				return nil, fmt.Errorf("failed to parse health config JSON: %w", err)
			}
			config.HealthConfig = &healthConfig
		}

		// Decrypt credentials if present
		if encryptedCreds.Valid && authType.Valid {
			decrypted, err := h.encryptionSvc.DecryptCredential([]byte(encryptedCreds.String), config.TenantID)
			if err != nil {
				// Log error but continue - health check can work without auth
				// The health check itself will handle auth failures
				config.Credential = nil
			} else {
				// For now, create a simple token credential
				// In a real implementation, this would deserialize the full credential structure
				config.Credential = &models.TokenCredential{
					Type:  authType.String,
					Token: decrypted,
				}
			}
		}

		configs = append(configs, config)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configs, nil
}

// UpdateToolHealthStatus updates the health status of a tool in the database
func (h *HealthCheckDBImpl) UpdateToolHealthStatus(ctx context.Context, tenantID, toolID string, status *tools.HealthStatus) error {
	healthData, err := json.Marshal(map[string]interface{}{
		"is_healthy":    status.IsHealthy,
		"last_checked":  status.LastChecked,
		"response_time": status.ResponseTime,
		"error":         status.Error,
		"details":       status.Details,
		"version":       status.Version,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal health status: %w", err)
	}

	query := `
		UPDATE tool_configurations
		SET 
			health_status = $1,
			last_health_check = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND tenant_id = $4
	`

	result, err := h.db.ExecContext(ctx, query, string(healthData), status.LastChecked, toolID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to update health status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool not found: %s", toolID)
	}

	return nil
}
