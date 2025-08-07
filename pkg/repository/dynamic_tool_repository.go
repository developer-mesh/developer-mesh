package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/jmoiron/sqlx"
)

// DynamicToolRepository defines the interface for dynamic tool storage
type DynamicToolRepository interface {
	// Create stores a new dynamic tool
	Create(ctx context.Context, tool *models.DynamicTool) error

	// GetByID retrieves a dynamic tool by ID
	GetByID(ctx context.Context, id string) (*models.DynamicTool, error)

	// GetByToolName retrieves a dynamic tool by name and tenant
	GetByToolName(ctx context.Context, tenantID, toolName string) (*models.DynamicTool, error)

	// List retrieves dynamic tools for a tenant
	List(ctx context.Context, tenantID string, status string) ([]*models.DynamicTool, error)

	// Update modifies an existing dynamic tool
	Update(ctx context.Context, tool *models.DynamicTool) error

	// Delete removes a dynamic tool
	Delete(ctx context.Context, id string) error

	// UpdateStatus updates the status of a dynamic tool
	UpdateStatus(ctx context.Context, id, status string) error
}

// dynamicToolRepository is the SQL implementation
type dynamicToolRepository struct {
	db *sqlx.DB
}

// NewDynamicToolRepository creates a new dynamic tool repository
func NewDynamicToolRepository(db *sqlx.DB) DynamicToolRepository {
	return &dynamicToolRepository{db: db}
}

// Create stores a new dynamic tool
func (r *dynamicToolRepository) Create(ctx context.Context, tool *models.DynamicTool) error {
	configJSON, err := json.Marshal(tool.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	webhookConfigJSON, err := json.Marshal(tool.WebhookConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook config: %w", err)
	}

	retryPolicyJSON, err := json.Marshal(tool.RetryPolicy)
	if err != nil {
		return fmt.Errorf("failed to marshal retry policy: %w", err)
	}

	passthroughConfigJSON, err := json.Marshal(tool.PassthroughConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal passthrough config: %w", err)
	}

	query := `
		INSERT INTO mcp.tool_configurations (
			id, tool_name, display_name, base_url, provider,
			config, webhook_config, retry_policy, passthrough_config,
			auth_type, credentials_encrypted, status, tenant_id, 
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)`

	_, err = r.db.ExecContext(ctx, query,
		tool.ID, tool.ToolName, tool.DisplayName, tool.BaseURL, tool.Provider,
		configJSON, webhookConfigJSON, retryPolicyJSON, passthroughConfigJSON,
		tool.AuthType, tool.CredentialsEncrypted, tool.Status, tool.TenantID,
		tool.CreatedAt, tool.UpdatedAt,
	)

	return err
}

// GetByID retrieves a dynamic tool by ID
func (r *dynamicToolRepository) GetByID(ctx context.Context, id string) (*models.DynamicTool, error) {
	var tool models.DynamicTool
	var configJSON, webhookConfigJSON, retryPolicyJSON, passthroughConfigJSON, healthStatusJSON []byte

	query := `
		SELECT 
			id, tool_name, display_name, base_url, provider,
			config, webhook_config, retry_policy, passthrough_config,
			auth_type, credentials_encrypted, status, health_status,
			last_health_check, tenant_id, created_at, updated_at
		FROM mcp.tool_configurations 
		WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tool.ID, &tool.ToolName, &tool.DisplayName, &tool.BaseURL, &tool.Provider,
		&configJSON, &webhookConfigJSON, &retryPolicyJSON, &passthroughConfigJSON,
		&tool.AuthType, &tool.CredentialsEncrypted, &tool.Status, &healthStatusJSON,
		&tool.LastHealthCheck, &tool.TenantID, &tool.CreatedAt, &tool.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tool not found")
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if configJSON != nil {
		if err := json.Unmarshal(configJSON, &tool.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}
	if webhookConfigJSON != nil {
		if err := json.Unmarshal(webhookConfigJSON, &tool.WebhookConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal webhook config: %w", err)
		}
	}
	if retryPolicyJSON != nil {
		if err := json.Unmarshal(retryPolicyJSON, &tool.RetryPolicy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
		}
	}
	if passthroughConfigJSON != nil {
		if err := json.Unmarshal(passthroughConfigJSON, &tool.PassthroughConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal passthrough config: %w", err)
		}
	}
	if healthStatusJSON != nil {
		rawMsg := json.RawMessage(healthStatusJSON)
		tool.HealthStatus = &rawMsg
	}

	return &tool, nil
}

// GetByToolName retrieves a dynamic tool by name and tenant
func (r *dynamicToolRepository) GetByToolName(ctx context.Context, tenantID, toolName string) (*models.DynamicTool, error) {
	var tool models.DynamicTool
	var configJSON, webhookConfigJSON, retryPolicyJSON, passthroughConfigJSON, healthStatusJSON []byte

	query := `
		SELECT 
			id, tool_name, display_name, base_url, provider,
			config, webhook_config, retry_policy, passthrough_config,
			auth_type, credentials_encrypted, status, health_status,
			last_health_check, tenant_id, created_at, updated_at
		FROM mcp.tool_configurations 
		WHERE tenant_id = $1 AND tool_name = $2`

	err := r.db.QueryRowContext(ctx, query, tenantID, toolName).Scan(
		&tool.ID, &tool.ToolName, &tool.DisplayName, &tool.BaseURL, &tool.Provider,
		&configJSON, &webhookConfigJSON, &retryPolicyJSON, &passthroughConfigJSON,
		&tool.AuthType, &tool.CredentialsEncrypted, &tool.Status, &healthStatusJSON,
		&tool.LastHealthCheck, &tool.TenantID, &tool.CreatedAt, &tool.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tool not found")
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if configJSON != nil {
		if err := json.Unmarshal(configJSON, &tool.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}
	if webhookConfigJSON != nil {
		if err := json.Unmarshal(webhookConfigJSON, &tool.WebhookConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal webhook config: %w", err)
		}
	}
	if retryPolicyJSON != nil {
		if err := json.Unmarshal(retryPolicyJSON, &tool.RetryPolicy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
		}
	}
	if passthroughConfigJSON != nil {
		if err := json.Unmarshal(passthroughConfigJSON, &tool.PassthroughConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal passthrough config: %w", err)
		}
	}
	if healthStatusJSON != nil {
		rawMsg := json.RawMessage(healthStatusJSON)
		tool.HealthStatus = &rawMsg
	}

	return &tool, nil
}

// List retrieves dynamic tools for a tenant
func (r *dynamicToolRepository) List(ctx context.Context, tenantID string, status string) ([]*models.DynamicTool, error) {
	query := `
		SELECT 
			id, tool_name, display_name, base_url, provider,
			config, webhook_config, retry_policy, passthrough_config,
			auth_type, credentials_encrypted, status, health_status,
			last_health_check, tenant_id, created_at, updated_at
		FROM mcp.tool_configurations 
		WHERE tenant_id = $1`

	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log error but don't fail the operation
			_ = err
		}
	}()

	var tools []*models.DynamicTool
	for rows.Next() {
		var tool models.DynamicTool
		var configJSON, webhookConfigJSON, retryPolicyJSON, passthroughConfigJSON, healthStatusJSON []byte

		err := rows.Scan(
			&tool.ID, &tool.ToolName, &tool.DisplayName, &tool.BaseURL, &tool.Provider,
			&configJSON, &webhookConfigJSON, &retryPolicyJSON, &passthroughConfigJSON,
			&tool.AuthType, &tool.CredentialsEncrypted, &tool.Status, &healthStatusJSON,
			&tool.LastHealthCheck, &tool.TenantID, &tool.CreatedAt, &tool.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		if configJSON != nil {
			if err := json.Unmarshal(configJSON, &tool.Config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		}
		if webhookConfigJSON != nil {
			if err := json.Unmarshal(webhookConfigJSON, &tool.WebhookConfig); err != nil {
				return nil, fmt.Errorf("failed to unmarshal webhook config: %w", err)
			}
		}
		if retryPolicyJSON != nil {
			if err := json.Unmarshal(retryPolicyJSON, &tool.RetryPolicy); err != nil {
				return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
			}
		}
		if passthroughConfigJSON != nil {
			if err := json.Unmarshal(passthroughConfigJSON, &tool.PassthroughConfig); err != nil {
				return nil, fmt.Errorf("failed to unmarshal passthrough config: %w", err)
			}
		}
		if healthStatusJSON != nil {
			rawMsg := json.RawMessage(healthStatusJSON)
			tool.HealthStatus = &rawMsg
		}

		tools = append(tools, &tool)
	}

	return tools, nil
}

// Update modifies an existing dynamic tool
func (r *dynamicToolRepository) Update(ctx context.Context, tool *models.DynamicTool) error {
	configJSON, err := json.Marshal(tool.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	webhookConfigJSON, err := json.Marshal(tool.WebhookConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook config: %w", err)
	}

	retryPolicyJSON, err := json.Marshal(tool.RetryPolicy)
	if err != nil {
		return fmt.Errorf("failed to marshal retry policy: %w", err)
	}

	passthroughConfigJSON, err := json.Marshal(tool.PassthroughConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal passthrough config: %w", err)
	}

	tool.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.tool_configurations SET
			tool_name = $2, display_name = $3, base_url = $4, provider = $5,
			config = $6, webhook_config = $7, retry_policy = $8,
			passthrough_config = $9, auth_type = $10, credentials_encrypted = $11,
			status = $12, updated_at = $13
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		tool.ID, tool.ToolName, tool.DisplayName, tool.BaseURL, tool.Provider,
		configJSON, webhookConfigJSON, retryPolicyJSON, passthroughConfigJSON,
		tool.AuthType, tool.CredentialsEncrypted, tool.Status, tool.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tool not found")
	}

	return nil
}

// Delete removes a dynamic tool
func (r *dynamicToolRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.tool_configurations WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tool not found")
	}

	return nil
}

// UpdateStatus updates the status of a dynamic tool
func (r *dynamicToolRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE mcp.tool_configurations 
		SET status = $2, updated_at = $3
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, status, time.Now())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tool not found")
	}

	return nil
}
