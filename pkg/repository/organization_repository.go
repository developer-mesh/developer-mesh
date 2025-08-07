package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// OrganizationRepository defines the interface for organization operations
type OrganizationRepository interface {
	Create(ctx context.Context, org *models.Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*models.Organization, error)
	GetByTenant(ctx context.Context, tenantID uuid.UUID) (*models.Organization, error)
	Update(ctx context.Context, org *models.Organization) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Tenant management
	AddTenant(ctx context.Context, orgID, tenantID uuid.UUID, name, tenantType string) error
	RemoveTenant(ctx context.Context, orgID, tenantID uuid.UUID) error
	ListTenants(ctx context.Context, orgID uuid.UUID) ([]models.OrganizationTenant, error)
	GetTenantInfo(ctx context.Context, tenantID uuid.UUID) (*models.OrganizationTenant, error)

	// Access matrix management
	GrantAccess(ctx context.Context, matrix *models.TenantAccessMatrix) error
	RevokeAccess(ctx context.Context, sourceID, targetID uuid.UUID) error
	GetAccessMatrix(ctx context.Context, sourceID, targetID uuid.UUID) (*models.TenantAccessMatrix, error)
	ListAccessForTenant(ctx context.Context, tenantID uuid.UUID) ([]models.TenantAccessMatrix, error)
}

// organizationRepository implements OrganizationRepository
type organizationRepository struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewOrganizationRepository creates a new organization repository
func NewOrganizationRepository(db *sqlx.DB, logger observability.Logger) OrganizationRepository {
	return &organizationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new organization
func (r *organizationRepository) Create(ctx context.Context, org *models.Organization) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.Create")
	defer span.End()

	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.organizations (id, name, slug, isolation_mode, settings)
		VALUES (:id, :name, :slug, :isolation_mode, :settings)
	`

	_, err := r.db.NamedExecContext(ctx, query, org)
	if err != nil {
		r.logger.Error("Failed to create organization", map[string]interface{}{
			"org_id": org.ID,
			"error":  err.Error(),
		})
		return fmt.Errorf("failed to create organization: %w", err)
	}

	return nil
}

// GetByID retrieves an organization by ID
func (r *organizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.GetByID")
	defer span.End()

	query := `
		SELECT id, name, slug, isolation_mode, settings, created_at, updated_at
		FROM mcp.organizations
		WHERE id = $1
	`

	var org models.Organization
	err := r.db.GetContext(ctx, &org, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get organization", map[string]interface{}{
			"org_id": id,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return &org, nil
}

// GetBySlug retrieves an organization by slug
func (r *organizationRepository) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.GetBySlug")
	defer span.End()

	query := `
		SELECT id, name, slug, isolation_mode, settings, created_at, updated_at
		FROM mcp.organizations
		WHERE slug = $1
	`

	var org models.Organization
	err := r.db.GetContext(ctx, &org, query, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get organization by slug", map[string]interface{}{
			"slug":  slug,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to get organization by slug: %w", err)
	}

	return &org, nil
}

// GetByTenant retrieves the organization for a specific tenant
func (r *organizationRepository) GetByTenant(ctx context.Context, tenantID uuid.UUID) (*models.Organization, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.GetByTenant")
	defer span.End()

	query := `
		SELECT o.id, o.name, o.slug, o.isolation_mode, o.settings, o.created_at, o.updated_at
		FROM mcp.organizations o
		JOIN mcp.organization_tenants ot ON o.id = ot.organization_id
		WHERE ot.tenant_id = $1
	`

	var org models.Organization
	err := r.db.GetContext(ctx, &org, query, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get organization by tenant", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to get organization by tenant: %w", err)
	}

	return &org, nil
}

// Update updates an organization
func (r *organizationRepository) Update(ctx context.Context, org *models.Organization) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.Update")
	defer span.End()

	query := `
		UPDATE mcp.organizations
		SET name = :name, slug = :slug, isolation_mode = :isolation_mode, 
		    settings = :settings, updated_at = CURRENT_TIMESTAMP
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, org)
	if err != nil {
		r.logger.Error("Failed to update organization", map[string]interface{}{
			"org_id": org.ID,
			"error":  err.Error(),
		})
		return fmt.Errorf("failed to update organization: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("organization not found: %s", org.ID)
	}

	return nil
}

// Delete deletes an organization
func (r *organizationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.Delete")
	defer span.End()

	query := `DELETE FROM mcp.organizations WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete organization", map[string]interface{}{
			"org_id": id,
			"error":  err.Error(),
		})
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("organization not found: %s", id)
	}

	return nil
}

// AddTenant adds a tenant to an organization
func (r *organizationRepository) AddTenant(ctx context.Context, orgID, tenantID uuid.UUID, name, tenantType string) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.AddTenant")
	defer span.End()

	if tenantType == "" {
		tenantType = models.TenantTypeStandard
	}

	query := `
		INSERT INTO mcp.organization_tenants (organization_id, tenant_id, tenant_name, tenant_type, isolation_level)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (organization_id, tenant_id) DO UPDATE
		SET tenant_name = EXCLUDED.tenant_name,
		    tenant_type = EXCLUDED.tenant_type
	`

	_, err := r.db.ExecContext(ctx, query, orgID, tenantID, name, tenantType, models.IsolationLevelNormal)
	if err != nil {
		r.logger.Error("Failed to add tenant to organization", map[string]interface{}{
			"org_id":    orgID,
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return fmt.Errorf("failed to add tenant to organization: %w", err)
	}

	return nil
}

// RemoveTenant removes a tenant from an organization
func (r *organizationRepository) RemoveTenant(ctx context.Context, orgID, tenantID uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.RemoveTenant")
	defer span.End()

	query := `
		DELETE FROM mcp.organization_tenants 
		WHERE organization_id = $1 AND tenant_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, orgID, tenantID)
	if err != nil {
		r.logger.Error("Failed to remove tenant from organization", map[string]interface{}{
			"org_id":    orgID,
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return fmt.Errorf("failed to remove tenant from organization: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tenant not found in organization")
	}

	return nil
}

// ListTenants lists all tenants in an organization
func (r *organizationRepository) ListTenants(ctx context.Context, orgID uuid.UUID) ([]models.OrganizationTenant, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.ListTenants")
	defer span.End()

	query := `
		SELECT organization_id, tenant_id, tenant_name, tenant_type, isolation_level, created_at
		FROM mcp.organization_tenants
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`

	var tenants []models.OrganizationTenant
	err := r.db.SelectContext(ctx, &tenants, query, orgID)
	if err != nil {
		r.logger.Error("Failed to list organization tenants", map[string]interface{}{
			"org_id": orgID,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to list organization tenants: %w", err)
	}

	return tenants, nil
}

// GetTenantInfo retrieves information about a specific tenant
func (r *organizationRepository) GetTenantInfo(ctx context.Context, tenantID uuid.UUID) (*models.OrganizationTenant, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.GetTenantInfo")
	defer span.End()

	query := `
		SELECT organization_id, tenant_id, tenant_name, tenant_type, isolation_level, created_at
		FROM mcp.organization_tenants
		WHERE tenant_id = $1
	`

	var tenant models.OrganizationTenant
	err := r.db.GetContext(ctx, &tenant, query, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get tenant info", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to get tenant info: %w", err)
	}

	return &tenant, nil
}

// GrantAccess grants access between tenants
func (r *organizationRepository) GrantAccess(ctx context.Context, matrix *models.TenantAccessMatrix) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.GrantAccess")
	defer span.End()

	if matrix.ID == uuid.Nil {
		matrix.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.tenant_access_matrix 
		(id, source_tenant_id, target_tenant_id, organization_id, access_type, permissions)
		VALUES (:id, :source_tenant_id, :target_tenant_id, :organization_id, :access_type, :permissions)
		ON CONFLICT (source_tenant_id, target_tenant_id, access_type) 
		DO UPDATE SET permissions = EXCLUDED.permissions
	`

	_, err := r.db.NamedExecContext(ctx, query, matrix)
	if err != nil {
		r.logger.Error("Failed to grant access", map[string]interface{}{
			"source_tenant": matrix.SourceTenantID,
			"target_tenant": matrix.TargetTenantID,
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to grant access: %w", err)
	}

	return nil
}

// RevokeAccess revokes access between tenants
func (r *organizationRepository) RevokeAccess(ctx context.Context, sourceID, targetID uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.organization.RevokeAccess")
	defer span.End()

	query := `
		DELETE FROM mcp.tenant_access_matrix
		WHERE source_tenant_id = $1 AND target_tenant_id = $2
	`

	_, err := r.db.ExecContext(ctx, query, sourceID, targetID)
	if err != nil {
		r.logger.Error("Failed to revoke access", map[string]interface{}{
			"source_tenant": sourceID,
			"target_tenant": targetID,
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to revoke access: %w", err)
	}

	return nil
}

// GetAccessMatrix retrieves the access matrix between two tenants
func (r *organizationRepository) GetAccessMatrix(ctx context.Context, sourceID, targetID uuid.UUID) (*models.TenantAccessMatrix, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.GetAccessMatrix")
	defer span.End()

	query := `
		SELECT id, source_tenant_id, target_tenant_id, organization_id, access_type, permissions, created_at
		FROM mcp.tenant_access_matrix
		WHERE source_tenant_id = $1 AND target_tenant_id = $2
	`

	var matrix models.TenantAccessMatrix
	err := r.db.GetContext(ctx, &matrix, query, sourceID, targetID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get access matrix", map[string]interface{}{
			"source_tenant": sourceID,
			"target_tenant": targetID,
			"error":         err.Error(),
		})
		return nil, fmt.Errorf("failed to get access matrix: %w", err)
	}

	return &matrix, nil
}

// ListAccessForTenant lists all access permissions for a tenant
func (r *organizationRepository) ListAccessForTenant(ctx context.Context, tenantID uuid.UUID) ([]models.TenantAccessMatrix, error) {
	ctx, span := observability.StartSpan(ctx, "repository.organization.ListAccessForTenant")
	defer span.End()

	query := `
		SELECT id, source_tenant_id, target_tenant_id, organization_id, access_type, permissions, created_at
		FROM mcp.tenant_access_matrix
		WHERE source_tenant_id = $1 OR target_tenant_id = $1
		ORDER BY created_at DESC
	`

	var matrices []models.TenantAccessMatrix
	err := r.db.SelectContext(ctx, &matrices, query, tenantID)
	if err != nil {
		r.logger.Error("Failed to list access for tenant", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to list access for tenant: %w", err)
	}

	return matrices, nil
}
