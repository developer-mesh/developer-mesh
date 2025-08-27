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

// ToolTemplateRepository defines the interface for tool template storage
type ToolTemplateRepository interface {
	// Create stores a new tool template
	Create(ctx context.Context, template *models.ToolTemplate) error

	// Upsert creates or updates a tool template
	Upsert(ctx context.Context, template *models.ToolTemplate) error

	// GetByID retrieves a tool template by ID
	GetByID(ctx context.Context, id string) (*models.ToolTemplate, error)

	// GetByProviderName retrieves the latest active template for a provider
	GetByProviderName(ctx context.Context, providerName string) (*models.ToolTemplate, error)

	// List retrieves all active public templates
	List(ctx context.Context) ([]*models.ToolTemplate, error)

	// ListByCategory retrieves templates by category
	ListByCategory(ctx context.Context, category string) ([]*models.ToolTemplate, error)

	// Update modifies an existing template
	Update(ctx context.Context, template *models.ToolTemplate) error

	// Delete removes a template (soft delete by marking inactive)
	Delete(ctx context.Context, id string) error
}

// toolTemplateRepository is the SQL implementation
type toolTemplateRepository struct {
	db *sqlx.DB
}

// NewToolTemplateRepository creates a new tool template repository
func NewToolTemplateRepository(db *sqlx.DB) ToolTemplateRepository {
	return &toolTemplateRepository{db: db}
}

// Create stores a new tool template
func (r *toolTemplateRepository) Create(ctx context.Context, template *models.ToolTemplate) error {
	query := `
		INSERT INTO mcp.tool_templates (
			id, provider_name, provider_version, display_name, description,
			icon_url, category, default_config, operation_groups, operation_mappings,
			ai_definitions, customization_schema, required_credentials,
			optional_credentials, optional_features, features, tags,
			documentation_url, api_documentation_url, example_configurations,
			is_public, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24
		)`

	// Marshal JSON fields
	defaultConfigJSON, err := json.Marshal(template.DefaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	operationGroupsJSON, err := json.Marshal(template.OperationGroups)
	if err != nil {
		return fmt.Errorf("failed to marshal operation groups: %w", err)
	}

	operationMappingsJSON, err := json.Marshal(template.OperationMappings)
	if err != nil {
		return fmt.Errorf("failed to marshal operation mappings: %w", err)
	}

	featuresJSON, err := json.Marshal(template.Features)
	if err != nil {
		return fmt.Errorf("failed to marshal features: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		template.ID,
		template.ProviderName,
		template.ProviderVersion,
		template.DisplayName,
		template.Description,
		template.IconURL,
		template.Category,
		defaultConfigJSON,
		operationGroupsJSON,
		operationMappingsJSON,
		template.AIDefinitions,
		template.CustomizationSchema,
		pq.Array(template.RequiredCredentials),
		pq.Array(template.OptionalCredentials),
		template.OptionalFeatures,
		featuresJSON,
		pq.Array(template.Tags),
		template.DocumentationURL,
		template.APIDocumentationURL,
		template.ExampleConfigurations,
		template.IsPublic,
		template.IsActive,
		template.CreatedAt,
		template.UpdatedAt,
	)

	return err
}

// Upsert creates or updates a tool template
func (r *toolTemplateRepository) Upsert(ctx context.Context, template *models.ToolTemplate) error {
	query := `
		INSERT INTO mcp.tool_templates (
			id, provider_name, provider_version, display_name, description,
			icon_url, category, default_config, operation_groups, operation_mappings,
			ai_definitions, customization_schema, required_credentials,
			optional_credentials, optional_features, features, tags,
			documentation_url, api_documentation_url, example_configurations,
			is_public, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24
		)
		ON CONFLICT (provider_name, provider_version) 
		DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			default_config = EXCLUDED.default_config,
			operation_groups = EXCLUDED.operation_groups,
			operation_mappings = EXCLUDED.operation_mappings,
			ai_definitions = EXCLUDED.ai_definitions,
			features = EXCLUDED.features,
			updated_at = CURRENT_TIMESTAMP`

	// Marshal JSON fields
	defaultConfigJSON, err := json.Marshal(template.DefaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	operationGroupsJSON, err := json.Marshal(template.OperationGroups)
	if err != nil {
		return fmt.Errorf("failed to marshal operation groups: %w", err)
	}

	operationMappingsJSON, err := json.Marshal(template.OperationMappings)
	if err != nil {
		return fmt.Errorf("failed to marshal operation mappings: %w", err)
	}

	featuresJSON, err := json.Marshal(template.Features)
	if err != nil {
		return fmt.Errorf("failed to marshal features: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		template.ID,
		template.ProviderName,
		template.ProviderVersion,
		template.DisplayName,
		template.Description,
		template.IconURL,
		template.Category,
		defaultConfigJSON,
		operationGroupsJSON,
		operationMappingsJSON,
		template.AIDefinitions,
		template.CustomizationSchema,
		pq.Array(template.RequiredCredentials),
		pq.Array(template.OptionalCredentials),
		template.OptionalFeatures,
		featuresJSON,
		pq.Array(template.Tags),
		template.DocumentationURL,
		template.APIDocumentationURL,
		template.ExampleConfigurations,
		template.IsPublic,
		template.IsActive,
		template.CreatedAt,
		template.UpdatedAt,
	)

	return err
}

// GetByID retrieves a tool template by ID
func (r *toolTemplateRepository) GetByID(ctx context.Context, id string) (*models.ToolTemplate, error) {
	var template models.ToolTemplate
	var defaultConfigJSON, operationGroupsJSON, operationMappingsJSON, featuresJSON []byte

	query := `
		SELECT 
			id, provider_name, provider_version, display_name, description,
			icon_url, category, default_config, operation_groups, operation_mappings,
			ai_definitions, customization_schema, required_credentials,
			optional_credentials, optional_features, features, tags,
			documentation_url, api_documentation_url, example_configurations,
			is_public, is_active, is_deprecated, deprecated_at, deprecated_message,
			created_at, updated_at
		FROM mcp.tool_templates
		WHERE id = $1 AND is_active = true`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&template.ID,
		&template.ProviderName,
		&template.ProviderVersion,
		&template.DisplayName,
		&template.Description,
		&template.IconURL,
		&template.Category,
		&defaultConfigJSON,
		&operationGroupsJSON,
		&operationMappingsJSON,
		&template.AIDefinitions,
		&template.CustomizationSchema,
		pq.Array(&template.RequiredCredentials),
		pq.Array(&template.OptionalCredentials),
		&template.OptionalFeatures,
		&featuresJSON,
		pq.Array(&template.Tags),
		&template.DocumentationURL,
		&template.APIDocumentationURL,
		&template.ExampleConfigurations,
		&template.IsPublic,
		&template.IsActive,
		&template.IsDeprecated,
		&template.DeprecatedAt,
		&template.DeprecatedMessage,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found")
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(defaultConfigJSON, &template.DefaultConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default config: %w", err)
	}

	if err := json.Unmarshal(operationGroupsJSON, &template.OperationGroups); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation groups: %w", err)
	}

	if err := json.Unmarshal(operationMappingsJSON, &template.OperationMappings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation mappings: %w", err)
	}

	if err := json.Unmarshal(featuresJSON, &template.Features); err != nil {
		return nil, fmt.Errorf("failed to unmarshal features: %w", err)
	}

	return &template, nil
}

// GetByProviderName retrieves the latest active template for a provider
func (r *toolTemplateRepository) GetByProviderName(ctx context.Context, providerName string) (*models.ToolTemplate, error) {
	var template models.ToolTemplate
	var defaultConfigJSON, operationGroupsJSON, operationMappingsJSON, featuresJSON []byte

	query := `
		SELECT 
			id, provider_name, provider_version, display_name, description,
			icon_url, category, default_config, operation_groups, operation_mappings,
			ai_definitions, customization_schema, required_credentials,
			optional_credentials, optional_features, features, tags,
			documentation_url, api_documentation_url, example_configurations,
			is_public, is_active, is_deprecated, deprecated_at, deprecated_message,
			created_at, updated_at
		FROM mcp.tool_templates
		WHERE provider_name = $1 
			AND is_active = true 
			AND is_deprecated = false
		ORDER BY provider_version DESC
		LIMIT 1`

	err := r.db.QueryRowContext(ctx, query, providerName).Scan(
		&template.ID,
		&template.ProviderName,
		&template.ProviderVersion,
		&template.DisplayName,
		&template.Description,
		&template.IconURL,
		&template.Category,
		&defaultConfigJSON,
		&operationGroupsJSON,
		&operationMappingsJSON,
		&template.AIDefinitions,
		&template.CustomizationSchema,
		pq.Array(&template.RequiredCredentials),
		pq.Array(&template.OptionalCredentials),
		&template.OptionalFeatures,
		&featuresJSON,
		pq.Array(&template.Tags),
		&template.DocumentationURL,
		&template.APIDocumentationURL,
		&template.ExampleConfigurations,
		&template.IsPublic,
		&template.IsActive,
		&template.IsDeprecated,
		&template.DeprecatedAt,
		&template.DeprecatedMessage,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found for provider %s", providerName)
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(defaultConfigJSON, &template.DefaultConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default config: %w", err)
	}

	if err := json.Unmarshal(operationGroupsJSON, &template.OperationGroups); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation groups: %w", err)
	}

	if err := json.Unmarshal(operationMappingsJSON, &template.OperationMappings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation mappings: %w", err)
	}

	if err := json.Unmarshal(featuresJSON, &template.Features); err != nil {
		return nil, fmt.Errorf("failed to unmarshal features: %w", err)
	}

	return &template, nil
}

// List retrieves all active public templates
func (r *toolTemplateRepository) List(ctx context.Context) ([]*models.ToolTemplate, error) {
	query := `
		SELECT 
			id, provider_name, provider_version, display_name, description,
			icon_url, category, tags, is_public, is_active,
			created_at, updated_at
		FROM mcp.tool_templates
		WHERE is_active = true AND is_public = true AND is_deprecated = false
		ORDER BY display_name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var templates []*models.ToolTemplate
	for rows.Next() {
		var template models.ToolTemplate
		err := rows.Scan(
			&template.ID,
			&template.ProviderName,
			&template.ProviderVersion,
			&template.DisplayName,
			&template.Description,
			&template.IconURL,
			&template.Category,
			pq.Array(&template.Tags),
			&template.IsPublic,
			&template.IsActive,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		templates = append(templates, &template)
	}

	return templates, nil
}

// ListByCategory retrieves templates by category
func (r *toolTemplateRepository) ListByCategory(ctx context.Context, category string) ([]*models.ToolTemplate, error) {
	query := `
		SELECT 
			id, provider_name, provider_version, display_name, description,
			icon_url, category, tags, is_public, is_active,
			created_at, updated_at
		FROM mcp.tool_templates
		WHERE category = $1 AND is_active = true AND is_public = true AND is_deprecated = false
		ORDER BY display_name`

	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var templates []*models.ToolTemplate
	for rows.Next() {
		var template models.ToolTemplate
		err := rows.Scan(
			&template.ID,
			&template.ProviderName,
			&template.ProviderVersion,
			&template.DisplayName,
			&template.Description,
			&template.IconURL,
			&template.Category,
			pq.Array(&template.Tags),
			&template.IsPublic,
			&template.IsActive,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		templates = append(templates, &template)
	}

	return templates, nil
}

// Update modifies an existing template
func (r *toolTemplateRepository) Update(ctx context.Context, template *models.ToolTemplate) error {
	template.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.tool_templates SET
			display_name = $2,
			description = $3,
			default_config = $4,
			operation_groups = $5,
			operation_mappings = $6,
			ai_definitions = $7,
			features = $8,
			updated_at = $9
		WHERE id = $1`

	// Marshal JSON fields
	defaultConfigJSON, err := json.Marshal(template.DefaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	operationGroupsJSON, err := json.Marshal(template.OperationGroups)
	if err != nil {
		return fmt.Errorf("failed to marshal operation groups: %w", err)
	}

	operationMappingsJSON, err := json.Marshal(template.OperationMappings)
	if err != nil {
		return fmt.Errorf("failed to marshal operation mappings: %w", err)
	}

	featuresJSON, err := json.Marshal(template.Features)
	if err != nil {
		return fmt.Errorf("failed to marshal features: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		template.ID,
		template.DisplayName,
		template.Description,
		defaultConfigJSON,
		operationGroupsJSON,
		operationMappingsJSON,
		template.AIDefinitions,
		featuresJSON,
		template.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}

// Delete removes a template (soft delete by marking inactive)
func (r *toolTemplateRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE mcp.tool_templates 
		SET is_active = false, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}
