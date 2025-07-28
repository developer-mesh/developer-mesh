package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ToolService manages tool lifecycle operations
type ToolService struct {
	db                *sqlx.DB
	credentialManager *CredentialManager
	logger            observability.Logger
}

// NewToolService creates a new tool service
func NewToolService(db *sqlx.DB, credentialManager *CredentialManager, logger observability.Logger) *ToolService {
	return &ToolService{
		db:                db,
		credentialManager: credentialManager,
		logger:            logger,
	}
}

// ToolConfigDB represents the database model for tool configurations
type ToolConfigDB struct {
	ID                   string          `db:"id"`
	TenantID             string          `db:"tenant_id"`
	ToolType             string          `db:"tool_type"`
	ToolName             string          `db:"tool_name"`
	DisplayName          sql.NullString  `db:"display_name"`
	Config               json.RawMessage `db:"config"`
	CredentialsEncrypted []byte          `db:"credentials_encrypted"`
	AuthType             string          `db:"auth_type"`
	RetryPolicy          json.RawMessage `db:"retry_policy"`
	Status               string          `db:"status"`
	HealthStatus         string          `db:"health_status"`
	LastHealthCheck      sql.NullTime    `db:"last_health_check"`
	CreatedAt            time.Time       `db:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"`
	CreatedBy            sql.NullString  `db:"created_by"`
}

// CreateTool registers a new tool for a tenant
func (s *ToolService) CreateTool(ctx context.Context, tenantID string, config *tool.ToolConfig, createdBy string) error {
	// Validate unique name for tenant
	exists, err := s.toolExists(ctx, tenantID, config.Name)
	if err != nil {
		return fmt.Errorf("failed to check tool existence: %w", err)
	}
	if exists {
		return fmt.Errorf("tool with name %s already exists for this tenant", config.Name)
	}

	// Encrypt credentials
	encryptedCreds, err := s.credentialManager.EncryptCredential(tenantID, config.Credential)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Serialize config and retry policy
	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var retryPolicyJSON json.RawMessage
	if config.RetryPolicy != nil {
		retryPolicyJSON, err = json.Marshal(config.RetryPolicy)
		if err != nil {
			return fmt.Errorf("failed to marshal retry policy: %w", err)
		}
	}

	// Generate ID
	id := uuid.New().String()

	// Insert into database
	query := `
		INSERT INTO tool_configurations (
			id, tenant_id, tool_type, tool_name, display_name,
			config, credentials_encrypted, auth_type, retry_policy,
			status, health_status, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)`

	_, err = s.db.ExecContext(ctx, query,
		id, tenantID, config.Type, config.Name,
		sql.NullString{String: config.DisplayName, Valid: config.DisplayName != ""},
		configJSON, encryptedCreds,
		config.Credential.Type, retryPolicyJSON,
		"active", "unknown",
		sql.NullString{String: createdBy, Valid: createdBy != ""},
	)

	if err != nil {
		return fmt.Errorf("failed to insert tool configuration: %w", err)
	}

	config.ID = id
	s.logger.Info("Tool created successfully", map[string]interface{}{
		"tool_id":   id,
		"tool_name": config.Name,
		"tenant_id": tenantID,
	})

	return nil
}

// GetTool retrieves a tool configuration
func (s *ToolService) GetTool(ctx context.Context, tenantID, toolName string) (*tool.ToolConfig, error) {
	var dbConfig ToolConfigDB

	query := `
		SELECT id, tenant_id, tool_type, tool_name, display_name,
		       config, credentials_encrypted, auth_type, retry_policy,
		       status, health_status, last_health_check,
		       created_at, updated_at, created_by
		FROM tool_configurations
		WHERE tenant_id = $1 AND tool_name = $2 AND status = 'active'`

	err := s.db.GetContext(ctx, &dbConfig, query, tenantID, toolName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool not found")
		}
		return nil, fmt.Errorf("failed to fetch tool configuration: %w", err)
	}

	return s.dbConfigToToolConfig(tenantID, &dbConfig)
}

// GetToolByType retrieves a tool by type (for migration compatibility)
func (s *ToolService) GetToolByType(ctx context.Context, tenantID, toolType string) (*tool.ToolConfig, error) {
	var dbConfig ToolConfigDB

	query := `
		SELECT id, tenant_id, tool_type, tool_name, display_name,
		       config, credentials_encrypted, auth_type, retry_policy,
		       status, health_status, last_health_check,
		       created_at, updated_at, created_by
		FROM tool_configurations
		WHERE tenant_id = $1 AND tool_type = $2 AND status = 'active'
		ORDER BY created_at ASC
		LIMIT 1`

	err := s.db.GetContext(ctx, &dbConfig, query, tenantID, toolType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool not found")
		}
		return nil, fmt.Errorf("failed to fetch tool configuration: %w", err)
	}

	return s.dbConfigToToolConfig(tenantID, &dbConfig)
}

// ListTools returns all tools for a tenant
func (s *ToolService) ListTools(ctx context.Context, tenantID string) ([]*tool.ToolConfig, error) {
	var dbConfigs []ToolConfigDB

	query := `
		SELECT id, tenant_id, tool_type, tool_name, display_name,
		       config, credentials_encrypted, auth_type, retry_policy,
		       status, health_status, last_health_check,
		       created_at, updated_at, created_by
		FROM tool_configurations
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY tool_name`

	err := s.db.SelectContext(ctx, &dbConfigs, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	configs := make([]*tool.ToolConfig, 0, len(dbConfigs))
	for _, dbConfig := range dbConfigs {
		config, err := s.dbConfigToToolConfig(tenantID, &dbConfig)
		if err != nil {
			s.logger.Error("Failed to convert DB config", map[string]interface{}{
				"error":   err.Error(),
				"tool_id": dbConfig.ID,
			})
			continue
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// UpdateTool updates a tool configuration
func (s *ToolService) UpdateTool(ctx context.Context, tenantID, toolName string, updates map[string]interface{}) error {
	// Get existing tool
	existing, err := s.GetTool(ctx, tenantID, toolName)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Update fields
	if newConfig, ok := updates["config"].(map[string]interface{}); ok {
		// Merge with existing config
		for k, v := range newConfig {
			existing.Config[k] = v
		}
	}

	if displayName, ok := updates["display_name"].(string); ok {
		existing.DisplayName = displayName
	}

	if creds, ok := updates["credential"].(*tool.TokenCredential); ok {
		encryptedCreds, err := s.credentialManager.EncryptCredential(tenantID, creds)
		if err != nil {
			return fmt.Errorf("failed to encrypt credentials: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE tool_configurations SET credentials_encrypted = $1, auth_type = $2 WHERE id = $3",
			encryptedCreds, creds.Type, existing.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update credentials: %w", err)
		}
	}

	// Update config
	configJSON, err := json.Marshal(existing.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE tool_configurations SET config = $1, display_name = $2 WHERE id = $3",
		configJSON,
		sql.NullString{String: existing.DisplayName, Valid: existing.DisplayName != ""},
		existing.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	return tx.Commit()
}

// DeleteTool marks a tool as deleted
func (s *ToolService) DeleteTool(ctx context.Context, tenantID, toolName string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE tool_configurations SET status = 'deleted' WHERE tenant_id = $1 AND tool_name = $2 AND status = 'active'",
		tenantID, toolName,
	)
	if err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("tool not found")
	}

	s.logger.Info("Tool deleted successfully", map[string]interface{}{
		"tool_name": toolName,
		"tenant_id": tenantID,
	})

	return nil
}

// UpdateHealthStatus updates the health status of a tool
func (s *ToolService) UpdateHealthStatus(ctx context.Context, toolID string, status *tool.HealthStatus) error {
	healthStatus := "healthy"
	if !status.IsHealthy {
		healthStatus = "unhealthy"
	}

	_, err := s.db.ExecContext(ctx,
		"UPDATE tool_configurations SET health_status = $1, last_health_check = $2 WHERE id = $3",
		healthStatus, status.LastChecked, toolID,
	)

	if err != nil {
		return fmt.Errorf("failed to update health status: %w", err)
	}

	return nil
}

// toolExists checks if a tool with the given name exists for a tenant
func (s *ToolService) toolExists(ctx context.Context, tenantID, toolName string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		"SELECT COUNT(*) FROM tool_configurations WHERE tenant_id = $1 AND tool_name = $2 AND status = 'active'",
		tenantID, toolName,
	)
	return count > 0, err
}

// dbConfigToToolConfig converts database model to domain model
func (s *ToolService) dbConfigToToolConfig(tenantID string, dbConfig *ToolConfigDB) (*tool.ToolConfig, error) {
	config := &tool.ToolConfig{
		ID:           dbConfig.ID,
		TenantID:     tenantID,
		Type:         dbConfig.ToolType,
		Name:         dbConfig.ToolName,
		Status:       dbConfig.Status,
		HealthStatus: dbConfig.HealthStatus,
	}

	if dbConfig.DisplayName.Valid {
		config.DisplayName = dbConfig.DisplayName.String
	}

	if dbConfig.LastHealthCheck.Valid {
		config.LastHealthCheck = &dbConfig.LastHealthCheck.Time
	}

	// Unmarshal config
	if len(dbConfig.Config) > 0 {
		if err := json.Unmarshal(dbConfig.Config, &config.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// Extract URLs from config
	if baseURL, ok := config.Config["base_url"].(string); ok {
		config.BaseURL = baseURL
	}
	if docURL, ok := config.Config["documentation_url"].(string); ok {
		config.DocumentationURL = docURL
	}
	if openAPIURL, ok := config.Config["openapi_url"].(string); ok {
		config.OpenAPIURL = openAPIURL
	}

	// Unmarshal retry policy
	if len(dbConfig.RetryPolicy) > 0 {
		var retryPolicy tool.ToolRetryPolicy
		if err := json.Unmarshal(dbConfig.RetryPolicy, &retryPolicy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
		}
		config.RetryPolicy = &retryPolicy
	}

	// Decrypt credentials
	if len(dbConfig.CredentialsEncrypted) > 0 {
		creds, err := s.credentialManager.DecryptCredential(tenantID, dbConfig.CredentialsEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}
		config.Credential = creds
	}

	return config, nil
}
