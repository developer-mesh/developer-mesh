package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// OrganizationToolRepository defines the interface for organization tool storage
type OrganizationToolRepository interface {
	// Create stores a new organization tool
	Create(ctx context.Context, tool *models.OrganizationTool) error

	// GetByID retrieves an organization tool by ID
	GetByID(ctx context.Context, id string) (*models.OrganizationTool, error)

	// GetByInstanceName retrieves an organization tool by name and organization
	GetByInstanceName(ctx context.Context, orgID, instanceName string) (*models.OrganizationTool, error)

	// ListByOrganization retrieves tools for an organization
	ListByOrganization(ctx context.Context, orgID string) ([]*models.OrganizationTool, error)

	// ListByTenant retrieves tools for a tenant
	ListByTenant(ctx context.Context, tenantID string) ([]*models.OrganizationTool, error)

	// Update modifies an existing organization tool
	Update(ctx context.Context, tool *models.OrganizationTool) error

	// UpdateStatus updates the status of an organization tool
	UpdateStatus(ctx context.Context, id, status string) error

	// UpdateHealth updates the health status of an organization tool
	UpdateHealth(ctx context.Context, id string, healthStatus json.RawMessage, healthMessage string) error

	// Delete removes an organization tool
	Delete(ctx context.Context, id string) error
}

// organizationToolRepository is the SQL implementation
type organizationToolRepository struct {
	db *sqlx.DB
}

// NewOrganizationToolRepository creates a new organization tool repository
func NewOrganizationToolRepository(db *sqlx.DB) OrganizationToolRepository {
	return &organizationToolRepository{db: db}
}

// Create stores a new organization tool
func (r *organizationToolRepository) Create(ctx context.Context, tool *models.OrganizationTool) error {
	configJSON, err := json.Marshal(tool.InstanceConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal instance config: %w", err)
	}

	query := `
		INSERT INTO mcp.organization_tools (
			id, organization_id, tenant_id, template_id,
			instance_name, display_name, description,
			instance_config, credentials_encrypted, encryption_key_id,
			custom_mappings, enabled_features, disabled_operations,
			rate_limit_overrides, custom_headers,
			status, is_active, tags, metadata,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21
		)`

	_, err = r.db.ExecContext(ctx, query,
		tool.ID,
		tool.OrganizationID,
		tool.TenantID,
		tool.TemplateID,
		tool.InstanceName,
		tool.DisplayName,
		tool.Description,
		configJSON,
		tool.CredentialsEncrypted,
		tool.EncryptionKeyID,
		tool.CustomMappings,
		tool.EnabledFeatures,
		pq.Array(tool.DisabledOperations),
		tool.RateLimitOverrides,
		tool.CustomHeaders,
		tool.Status,
		tool.IsActive,
		pq.Array(tool.Tags),
		tool.Metadata,
		tool.CreatedAt,
		tool.UpdatedAt,
	)

	return err
}

// GetByID retrieves an organization tool by ID
func (r *organizationToolRepository) GetByID(ctx context.Context, id string) (*models.OrganizationTool, error) {
	var tool models.OrganizationTool
	var configJSON []byte
	var displayName, description, encryptionKeyID, healthMessage sql.NullString
	var customMappings, enabledFeatures, rateLimitOverrides, customHeaders, healthStatus, metadata sql.NullString
	var lastHealthCheck, lastUsedAt sql.NullTime

	query := `
		SELECT 
			id, organization_id, tenant_id, template_id,
			instance_name, display_name, description,
			instance_config, credentials_encrypted, encryption_key_id,
			custom_mappings, enabled_features, disabled_operations,
			rate_limit_overrides, custom_headers,
			status, is_active, last_health_check, health_status, health_message,
			last_used_at, usage_count, error_count,
			tags, metadata,
			created_at, updated_at
		FROM mcp.organization_tools
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tool.ID,
		&tool.OrganizationID,
		&tool.TenantID,
		&tool.TemplateID,
		&tool.InstanceName,
		&displayName,
		&description,
		&configJSON,
		&tool.CredentialsEncrypted,
		&encryptionKeyID,
		&customMappings,
		&enabledFeatures,
		pq.Array(&tool.DisabledOperations),
		&rateLimitOverrides,
		&customHeaders,
		&tool.Status,
		&tool.IsActive,
		&lastHealthCheck,
		&healthStatus,
		&healthMessage,
		&lastUsedAt,
		&tool.UsageCount,
		&tool.ErrorCount,
		pq.Array(&tool.Tags),
		&metadata,
		&tool.CreatedAt,
		&tool.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("organization tool not found")
	}
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if displayName.Valid {
		tool.DisplayName = displayName.String
	}
	if description.Valid {
		tool.Description = description.String
	}
	if encryptionKeyID.Valid {
		tool.EncryptionKeyID = &encryptionKeyID.String
	}
	if lastHealthCheck.Valid {
		tool.LastHealthCheck = &lastHealthCheck.Time
	}
	if healthStatus.Valid {
		rawMsg := json.RawMessage(healthStatus.String)
		tool.HealthStatus = &rawMsg
	}
	if healthMessage.Valid {
		tool.HealthMessage = healthMessage.String
	}
	if lastUsedAt.Valid {
		tool.LastUsedAt = &lastUsedAt.Time
	}
	if customMappings.Valid {
		rawMsg := json.RawMessage(customMappings.String)
		tool.CustomMappings = &rawMsg
	}
	if enabledFeatures.Valid {
		rawMsg := json.RawMessage(enabledFeatures.String)
		tool.EnabledFeatures = &rawMsg
	}
	if rateLimitOverrides.Valid {
		rawMsg := json.RawMessage(rateLimitOverrides.String)
		tool.RateLimitOverrides = &rawMsg
	}
	if customHeaders.Valid {
		rawMsg := json.RawMessage(customHeaders.String)
		tool.CustomHeaders = &rawMsg
	}
	if metadata.Valid {
		rawMsg := json.RawMessage(metadata.String)
		tool.Metadata = &rawMsg
	}

	// Unmarshal config
	if err := json.Unmarshal(configJSON, &tool.InstanceConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instance config: %w", err)
	}

	return &tool, nil
}

// GetByInstanceName retrieves an organization tool by name and organization
func (r *organizationToolRepository) GetByInstanceName(ctx context.Context, orgID, instanceName string) (*models.OrganizationTool, error) {
	var tool models.OrganizationTool
	var configJSON []byte
	var displayName, description, encryptionKeyID, healthMessage sql.NullString
	var customMappings, enabledFeatures, rateLimitOverrides, customHeaders, healthStatus, metadata sql.NullString
	var lastHealthCheck, lastUsedAt sql.NullTime

	query := `
		SELECT 
			id, organization_id, tenant_id, template_id,
			instance_name, display_name, description,
			instance_config, credentials_encrypted, encryption_key_id,
			custom_mappings, enabled_features, disabled_operations,
			rate_limit_overrides, custom_headers,
			status, is_active, last_health_check, health_status, health_message,
			last_used_at, usage_count, error_count,
			tags, metadata,
			created_at, updated_at
		FROM mcp.organization_tools
		WHERE organization_id = $1 AND instance_name = $2`

	err := r.db.QueryRowContext(ctx, query, orgID, instanceName).Scan(
		&tool.ID,
		&tool.OrganizationID,
		&tool.TenantID,
		&tool.TemplateID,
		&tool.InstanceName,
		&displayName,
		&description,
		&configJSON,
		&tool.CredentialsEncrypted,
		&encryptionKeyID,
		&customMappings,
		&enabledFeatures,
		pq.Array(&tool.DisabledOperations),
		&rateLimitOverrides,
		&customHeaders,
		&tool.Status,
		&tool.IsActive,
		&lastHealthCheck,
		&healthStatus,
		&healthMessage,
		&lastUsedAt,
		&tool.UsageCount,
		&tool.ErrorCount,
		pq.Array(&tool.Tags),
		&metadata,
		&tool.CreatedAt,
		&tool.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("organization tool not found")
	}
	if err != nil {
		return nil, err
	}

	// Handle nullable fields (same as GetByID)
	if displayName.Valid {
		tool.DisplayName = displayName.String
	}
	if description.Valid {
		tool.Description = description.String
	}
	if encryptionKeyID.Valid {
		tool.EncryptionKeyID = &encryptionKeyID.String
	}
	if lastHealthCheck.Valid {
		tool.LastHealthCheck = &lastHealthCheck.Time
	}
	if healthStatus.Valid {
		rawMsg := json.RawMessage(healthStatus.String)
		tool.HealthStatus = &rawMsg
	}
	if healthMessage.Valid {
		tool.HealthMessage = healthMessage.String
	}
	if lastUsedAt.Valid {
		tool.LastUsedAt = &lastUsedAt.Time
	}
	if customMappings.Valid {
		rawMsg := json.RawMessage(customMappings.String)
		tool.CustomMappings = &rawMsg
	}
	if enabledFeatures.Valid {
		rawMsg := json.RawMessage(enabledFeatures.String)
		tool.EnabledFeatures = &rawMsg
	}
	if rateLimitOverrides.Valid {
		rawMsg := json.RawMessage(rateLimitOverrides.String)
		tool.RateLimitOverrides = &rawMsg
	}
	if customHeaders.Valid {
		rawMsg := json.RawMessage(customHeaders.String)
		tool.CustomHeaders = &rawMsg
	}
	if metadata.Valid {
		rawMsg := json.RawMessage(metadata.String)
		tool.Metadata = &rawMsg
	}

	// Unmarshal config
	if err := json.Unmarshal(configJSON, &tool.InstanceConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instance config: %w", err)
	}

	return &tool, nil
}

// ListByOrganization retrieves tools for an organization
func (r *organizationToolRepository) ListByOrganization(ctx context.Context, orgID string) ([]*models.OrganizationTool, error) {
	query := `
		SELECT 
			id, organization_id, tenant_id, template_id,
			instance_name, display_name, status, is_active,
			last_health_check, usage_count, error_count,
			created_at, updated_at
		FROM mcp.organization_tools
		WHERE organization_id = $1 AND is_active = true
		ORDER BY instance_name`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var tools []*models.OrganizationTool
	for rows.Next() {
		var tool models.OrganizationTool
		var displayName sql.NullString
		var lastHealthCheck sql.NullTime

		err := rows.Scan(
			&tool.ID,
			&tool.OrganizationID,
			&tool.TenantID,
			&tool.TemplateID,
			&tool.InstanceName,
			&displayName,
			&tool.Status,
			&tool.IsActive,
			&lastHealthCheck,
			&tool.UsageCount,
			&tool.ErrorCount,
			&tool.CreatedAt,
			&tool.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if displayName.Valid {
			tool.DisplayName = displayName.String
		}
		if lastHealthCheck.Valid {
			tool.LastHealthCheck = &lastHealthCheck.Time
		}

		tools = append(tools, &tool)
	}

	return tools, nil
}

// ListByTenant retrieves tools for a tenant
func (r *organizationToolRepository) ListByTenant(ctx context.Context, tenantID string) ([]*models.OrganizationTool, error) {
	query := `
		SELECT 
			id, organization_id, tenant_id, template_id,
			instance_name, display_name, status, is_active,
			last_health_check, usage_count, error_count,
			created_at, updated_at
		FROM mcp.organization_tools
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY instance_name`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var tools []*models.OrganizationTool
	for rows.Next() {
		var tool models.OrganizationTool
		var displayName sql.NullString
		var lastHealthCheck sql.NullTime

		err := rows.Scan(
			&tool.ID,
			&tool.OrganizationID,
			&tool.TenantID,
			&tool.TemplateID,
			&tool.InstanceName,
			&displayName,
			&tool.Status,
			&tool.IsActive,
			&lastHealthCheck,
			&tool.UsageCount,
			&tool.ErrorCount,
			&tool.CreatedAt,
			&tool.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if displayName.Valid {
			tool.DisplayName = displayName.String
		}
		if lastHealthCheck.Valid {
			tool.LastHealthCheck = &lastHealthCheck.Time
		}

		tools = append(tools, &tool)
	}

	return tools, nil
}

// Update modifies an existing organization tool
func (r *organizationToolRepository) Update(ctx context.Context, tool *models.OrganizationTool) error {
	configJSON, err := json.Marshal(tool.InstanceConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal instance config: %w", err)
	}

	tool.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.organization_tools SET
			display_name = $2,
			description = $3,
			instance_config = $4,
			custom_mappings = $5,
			enabled_features = $6,
			disabled_operations = $7,
			rate_limit_overrides = $8,
			custom_headers = $9,
			status = $10,
			is_active = $11,
			tags = $12,
			metadata = $13,
			updated_at = $14
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		tool.ID,
		tool.DisplayName,
		tool.Description,
		configJSON,
		tool.CustomMappings,
		tool.EnabledFeatures,
		pq.Array(tool.DisabledOperations),
		tool.RateLimitOverrides,
		tool.CustomHeaders,
		tool.Status,
		tool.IsActive,
		pq.Array(tool.Tags),
		tool.Metadata,
		tool.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("organization tool not found")
	}

	return nil
}

// UpdateStatus updates the status of an organization tool
func (r *organizationToolRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE mcp.organization_tools 
		SET status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("organization tool not found")
	}

	return nil
}

// UpdateHealth updates the health status of an organization tool
func (r *organizationToolRepository) UpdateHealth(ctx context.Context, id string, healthStatus json.RawMessage, healthMessage string) error {
	query := `
		UPDATE mcp.organization_tools 
		SET 
			health_status = $2,
			health_message = $3,
			last_health_check = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, healthStatus, healthMessage)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("organization tool not found")
	}

	return nil
}

// Delete removes an organization tool
func (r *organizationToolRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.organization_tools WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("organization tool not found")
	}

	return nil
}
