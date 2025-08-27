package models

import (
	"encoding/json"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// ToolTemplate represents a pre-defined tool configuration for standard providers
type ToolTemplate struct {
	ID              string  `json:"id" db:"id"`
	ProviderName    string  `json:"provider_name" db:"provider_name"`
	ProviderVersion string  `json:"provider_version" db:"provider_version"`
	DisplayName     string  `json:"display_name" db:"display_name"`
	Description     string  `json:"description" db:"description"`
	IconURL         *string `json:"icon_url,omitempty" db:"icon_url"`
	Category        string  `json:"category,omitempty" db:"category"`

	// Configurations
	DefaultConfig     providers.ProviderConfig              `json:"default_config" db:"default_config"`
	OperationGroups   []OperationGroup                      `json:"operation_groups" db:"operation_groups"`
	OperationMappings map[string]providers.OperationMapping `json:"operation_mappings" db:"operation_mappings"`
	AIDefinitions     *json.RawMessage                      `json:"ai_definitions,omitempty" db:"ai_definitions"`

	// Customization
	CustomizationSchema *json.RawMessage `json:"customization_schema,omitempty" db:"customization_schema"`
	RequiredCredentials []string         `json:"required_credentials" db:"required_credentials"`
	OptionalCredentials []string         `json:"optional_credentials,omitempty" db:"optional_credentials"`
	OptionalFeatures    *json.RawMessage `json:"optional_features,omitempty" db:"optional_features"`

	// Features
	Features providers.ProviderFeatures `json:"features" db:"features"`

	// Metadata
	Tags                  []string         `json:"tags,omitempty" db:"tags"`
	DocumentationURL      string           `json:"documentation_url,omitempty" db:"documentation_url"`
	APIDocumentationURL   *string          `json:"api_documentation_url,omitempty" db:"api_documentation_url"`
	ExampleConfigurations *json.RawMessage `json:"example_configurations,omitempty" db:"example_configurations"`

	// Visibility
	IsPublic          bool       `json:"is_public" db:"is_public"`
	IsActive          bool       `json:"is_active" db:"is_active"`
	IsDeprecated      bool       `json:"is_deprecated" db:"is_deprecated"`
	DeprecatedAt      *time.Time `json:"deprecated_at,omitempty" db:"deprecated_at"`
	DeprecatedMessage *string    `json:"deprecated_message,omitempty" db:"deprecated_message"`

	// Audit
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy *string   `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy *string   `json:"updated_by,omitempty" db:"updated_by"`
}

// OrganizationTool represents an organization-specific tool instance based on a template
type OrganizationTool struct {
	ID             string `json:"id" db:"id"`
	OrganizationID string `json:"organization_id" db:"organization_id"`
	TenantID       string `json:"tenant_id" db:"tenant_id"`
	TemplateID     string `json:"template_id" db:"template_id"`

	// Instance configuration
	InstanceName string `json:"instance_name" db:"instance_name"`
	DisplayName  string `json:"display_name,omitempty" db:"display_name"`
	Description  string `json:"description,omitempty" db:"description"`

	// Configuration and credentials
	InstanceConfig       map[string]interface{} `json:"instance_config" db:"instance_config"`
	CredentialsEncrypted []byte                 `json:"-" db:"credentials_encrypted"`
	EncryptionKeyID      *string                `json:"-" db:"encryption_key_id"`

	// Customizations
	CustomMappings     *json.RawMessage `json:"custom_mappings,omitempty" db:"custom_mappings"`
	EnabledFeatures    *json.RawMessage `json:"enabled_features,omitempty" db:"enabled_features"`
	DisabledOperations []string         `json:"disabled_operations,omitempty" db:"disabled_operations"`
	RateLimitOverrides *json.RawMessage `json:"rate_limit_overrides,omitempty" db:"rate_limit_overrides"`
	CustomHeaders      *json.RawMessage `json:"custom_headers,omitempty" db:"custom_headers"`

	// Health and status
	Status          string           `json:"status" db:"status"`
	IsActive        bool             `json:"is_active" db:"is_active"`
	LastHealthCheck *time.Time       `json:"last_health_check,omitempty" db:"last_health_check"`
	HealthStatus    *json.RawMessage `json:"health_status,omitempty" db:"health_status"`
	HealthMessage   string           `json:"health_message,omitempty" db:"health_message"`

	// Usage tracking
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	UsageCount int        `json:"usage_count" db:"usage_count"`
	ErrorCount int        `json:"error_count" db:"error_count"`

	// Metadata
	Tags     []string         `json:"tags,omitempty" db:"tags"`
	Metadata *json.RawMessage `json:"metadata,omitempty" db:"metadata"`

	// Audit
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy *string   `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy *string   `json:"updated_by,omitempty" db:"updated_by"`
}

// OperationGroup groups related operations together
type OperationGroup struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Operations  []string `json:"operations"`
}

// ToolTemplateVersion tracks version history of tool templates
type ToolTemplateVersion struct {
	ID               string          `json:"id" db:"id"`
	TemplateID       string          `json:"template_id" db:"template_id"`
	VersionNumber    string          `json:"version_number" db:"version_number"`
	TemplateSnapshot json.RawMessage `json:"template_snapshot" db:"template_snapshot"`
	ChangeSummary    string          `json:"change_summary,omitempty" db:"change_summary"`
	BreakingChanges  bool            `json:"breaking_changes" db:"breaking_changes"`
	MigrationGuide   string          `json:"migration_guide,omitempty" db:"migration_guide"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	CreatedBy        *string         `json:"created_by,omitempty" db:"created_by"`
}

// OrganizationToolUsage tracks usage of organization tools for analytics
type OrganizationToolUsage struct {
	ID                 string          `json:"id" db:"id"`
	OrganizationToolID string          `json:"organization_tool_id" db:"organization_tool_id"`
	OperationName      string          `json:"operation_name" db:"operation_name"`
	ExecutionCount     int             `json:"execution_count" db:"execution_count"`
	SuccessCount       int             `json:"success_count" db:"success_count"`
	ErrorCount         int             `json:"error_count" db:"error_count"`
	AvgResponseTimeMs  int             `json:"avg_response_time_ms,omitempty" db:"avg_response_time_ms"`
	MinResponseTimeMs  int             `json:"min_response_time_ms,omitempty" db:"min_response_time_ms"`
	MaxResponseTimeMs  int             `json:"max_response_time_ms,omitempty" db:"max_response_time_ms"`
	PeriodStart        time.Time       `json:"period_start" db:"period_start"`
	PeriodEnd          time.Time       `json:"period_end" db:"period_end"`
	ErrorTypes         json.RawMessage `json:"error_types,omitempty" db:"error_types"`
}
