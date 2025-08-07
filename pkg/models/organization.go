package models

import (
	"time"

	"github.com/google/uuid"
)

// Organization represents a top-level organizational unit
type Organization struct {
	ID            uuid.UUID `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Slug          string    `json:"slug" db:"slug"`
	IsolationMode string    `json:"isolation_mode" db:"isolation_mode"`
	Settings      JSONMap   `json:"settings" db:"settings"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// OrganizationTenant represents the mapping between organizations and tenants
type OrganizationTenant struct {
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	TenantID       uuid.UUID `json:"tenant_id" db:"tenant_id"`
	TenantName     string    `json:"tenant_name" db:"tenant_name"`
	TenantType     string    `json:"tenant_type" db:"tenant_type"`
	IsolationLevel string    `json:"isolation_level" db:"isolation_level"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// TenantAccessMatrix represents the access permissions between tenants
type TenantAccessMatrix struct {
	ID             uuid.UUID `json:"id" db:"id"`
	SourceTenantID uuid.UUID `json:"source_tenant_id" db:"source_tenant_id"`
	TargetTenantID uuid.UUID `json:"target_tenant_id" db:"target_tenant_id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	AccessType     string    `json:"access_type" db:"access_type"`
	Permissions    JSONMap   `json:"permissions" db:"permissions"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// IsolationMode constants
const (
	IsolationModeStrict  = "strict"  // No cross-tenant access
	IsolationModeRelaxed = "relaxed" // Allow configured cross-tenant access
	IsolationModeOpen    = "open"    // Allow all cross-tenant access within org
)

// TenantType constants
const (
	TenantTypeStandard    = "standard"
	TenantTypeAdmin       = "admin"
	TenantTypeIntegration = "integration"
)

// IsolationLevel constants
const (
	IsolationLevelNormal     = "normal"
	IsolationLevelRestricted = "restricted"
	IsolationLevelElevated   = "elevated"
)

// AccessType constants
const (
	AccessTypeRead     = "read"
	AccessTypeWrite    = "write"
	AccessTypeAdmin    = "admin"
	AccessTypeDelegate = "delegate"
)

// IsStrictlyIsolated checks if the organization enforces strict tenant isolation
func (o *Organization) IsStrictlyIsolated() bool {
	return o.IsolationMode == IsolationModeStrict
}

// AllowsCrossTenantAccess checks if the organization allows any cross-tenant access
func (o *Organization) AllowsCrossTenantAccess() bool {
	return o.IsolationMode != IsolationModeStrict
}

// HasPermission checks if a specific permission exists in the permissions map
func (tam *TenantAccessMatrix) HasPermission(permission string) bool {
	if tam.Permissions == nil {
		return false
	}

	if val, exists := tam.Permissions[permission]; exists {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "enabled" || v == "allow"
		default:
			return false
		}
	}

	return false
}

// CanAccess checks if source tenant can access target tenant with specified access type
func (tam *TenantAccessMatrix) CanAccess(accessType string) bool {
	return tam.AccessType == accessType || tam.AccessType == AccessTypeAdmin
}

// IsAdmin checks if the tenant has admin privileges
func (ot *OrganizationTenant) IsAdmin() bool {
	return ot.TenantType == TenantTypeAdmin
}

// IsRestricted checks if the tenant has restricted access
func (ot *OrganizationTenant) IsRestricted() bool {
	return ot.IsolationLevel == IsolationLevelRestricted
}
