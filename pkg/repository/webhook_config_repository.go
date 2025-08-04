package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// WebhookConfigRepository defines the interface for webhook configuration operations
type WebhookConfigRepository interface {
	// GetByOrganization retrieves a webhook configuration by organization name
	GetByOrganization(ctx context.Context, orgName string) (*models.WebhookConfig, error)

	// GetWebhookSecret retrieves just the webhook secret for an organization
	GetWebhookSecret(ctx context.Context, orgName string) (string, error)

	// Create creates a new webhook configuration
	Create(ctx context.Context, config *models.WebhookConfigCreate) (*models.WebhookConfig, error)

	// Update updates an existing webhook configuration
	Update(ctx context.Context, orgName string, config *models.WebhookConfigUpdate) (*models.WebhookConfig, error)

	// List lists all webhook configurations
	List(ctx context.Context, enabledOnly bool) ([]*models.WebhookConfig, error)

	// Delete deletes a webhook configuration
	Delete(ctx context.Context, orgName string) error
}

// webhookConfigRepository is the PostgreSQL implementation
type webhookConfigRepository struct {
	db *sqlx.DB
}

// NewWebhookConfigRepository creates a new webhook configuration repository
func NewWebhookConfigRepository(db *sqlx.DB) WebhookConfigRepository {
	return &webhookConfigRepository{db: db}
}

// GetByOrganization retrieves a webhook configuration by organization name
func (r *webhookConfigRepository) GetByOrganization(ctx context.Context, orgName string) (*models.WebhookConfig, error) {
	var config models.WebhookConfig
	query := `
		SELECT id, organization_name, webhook_secret, enabled, allowed_events, metadata, created_at, updated_at
		FROM mcp.webhook_configs
		WHERE organization_name = $1
	`

	err := r.db.GetContext(ctx, &config, query, orgName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Wrap(err, "webhook configuration not found")
		}
		return nil, errors.Wrap(err, "failed to get webhook configuration")
	}

	return &config, nil
}

// GetWebhookSecret retrieves just the webhook secret for an organization
func (r *webhookConfigRepository) GetWebhookSecret(ctx context.Context, orgName string) (string, error) {
	var secret string
	query := `
		SELECT webhook_secret
		FROM mcp.webhook_configs
		WHERE organization_name = $1 AND enabled = true
	`

	err := r.db.GetContext(ctx, &secret, query, orgName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.Wrap(err, "webhook configuration not found or disabled")
		}
		return "", errors.Wrap(err, "failed to get webhook secret")
	}

	return secret, nil
}

// Create creates a new webhook configuration
func (r *webhookConfigRepository) Create(ctx context.Context, config *models.WebhookConfigCreate) (*models.WebhookConfig, error) {
	newConfig := &models.WebhookConfig{
		ID:               uuid.New(),
		OrganizationName: config.OrganizationName,
		WebhookSecret:    config.WebhookSecret, // Should be encrypted by service layer
		Enabled:          true,
		AllowedEvents:    config.AllowedEvents,
		Metadata:         models.JSONMap(config.Metadata),
	}

	// Set default allowed events if not specified
	if len(newConfig.AllowedEvents) == 0 {
		newConfig.AllowedEvents = []string{"issues", "issue_comment", "pull_request", "push", "release"}
	}

	if err := newConfig.Validate(); err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	query := `
		INSERT INTO mcp.webhook_configs 
		(id, organization_name, webhook_secret, enabled, allowed_events, metadata)
		VALUES (:id, :organization_name, :webhook_secret, :enabled, :allowed_events, :metadata)
		RETURNING created_at, updated_at
	`

	rows, err := r.db.NamedQueryContext(ctx, query, newConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create webhook configuration")
	}
	defer func() {
		_ = rows.Close()
	}()

	if rows.Next() {
		err = rows.Scan(&newConfig.CreatedAt, &newConfig.UpdatedAt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan timestamps")
		}
	}

	return newConfig, nil
}

// Update updates an existing webhook configuration
func (r *webhookConfigRepository) Update(ctx context.Context, orgName string, config *models.WebhookConfigUpdate) (*models.WebhookConfig, error) {
	// Build dynamic update query
	updates := make(map[string]interface{})
	updates["organization_name"] = orgName

	query := "UPDATE mcp.webhook_configs SET "
	params := []string{}

	if config.Enabled != nil {
		params = append(params, "enabled = :enabled")
		updates["enabled"] = *config.Enabled
	}

	if config.WebhookSecret != nil {
		params = append(params, "webhook_secret = :webhook_secret")
		updates["webhook_secret"] = *config.WebhookSecret
	}

	if config.AllowedEvents != nil {
		params = append(params, "allowed_events = :allowed_events")
		updates["allowed_events"] = config.AllowedEvents
	}

	if config.Metadata != nil {
		params = append(params, "metadata = :metadata")
		updates["metadata"] = models.JSONMap(config.Metadata)
	}

	if len(params) == 0 {
		return nil, errors.New("no fields to update")
	}

	query += fmt.Sprintf("%s, updated_at = NOW() WHERE organization_name = :organization_name",
		joinParams(params))
	query += " RETURNING id, organization_name, webhook_secret, enabled, allowed_events, metadata, created_at, updated_at"

	rows, err := r.db.NamedQueryContext(ctx, query, updates)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update webhook configuration")
	}
	defer func() {
		_ = rows.Close()
	}()

	var updatedConfig models.WebhookConfig
	if rows.Next() {
		err = rows.StructScan(&updatedConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan updated configuration")
		}
	} else {
		return nil, errors.New("webhook configuration not found")
	}

	return &updatedConfig, nil
}

// List lists all webhook configurations
func (r *webhookConfigRepository) List(ctx context.Context, enabledOnly bool) ([]*models.WebhookConfig, error) {
	query := `
		SELECT id, organization_name, webhook_secret, enabled, allowed_events, metadata, created_at, updated_at
		FROM mcp.webhook_configs
	`

	if enabledOnly {
		query += " WHERE enabled = true"
	}

	query += " ORDER BY organization_name"

	var configs []*models.WebhookConfig
	err := r.db.SelectContext(ctx, &configs, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list webhook configurations")
	}

	return configs, nil
}

// Delete deletes a webhook configuration
func (r *webhookConfigRepository) Delete(ctx context.Context, orgName string) error {
	query := "DELETE FROM mcp.webhook_configs WHERE organization_name = $1"

	result, err := r.db.ExecContext(ctx, query, orgName)
	if err != nil {
		return errors.Wrap(err, "failed to delete webhook configuration")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("webhook configuration not found")
	}

	return nil
}

// joinParams joins parameter strings with commas
func joinParams(params []string) string {
	result := ""
	for i, p := range params {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
