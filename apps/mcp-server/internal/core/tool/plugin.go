package tool

import (
	"context"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// ToolPlugin defines the interface for all tool implementations
type ToolPlugin interface {
	// Discovery
	DiscoverAPIs(ctx context.Context, config ToolConfig) (*DiscoveryResult, error)

	// Tool generation from ANY OpenAPI spec
	GenerateTools(config ToolConfig, spec *openapi3.T) ([]*DynamicTool, error)

	// Dynamic authentication based on security schemes
	AuthenticateRequest(req *http.Request, creds *TokenCredential) error

	// Connection testing
	TestConnection(ctx context.Context, config ToolConfig) error

	// Extract auth requirements from spec
	ExtractAuthSchemes(spec *openapi3.T) []SecurityScheme
}

// ToolConfig represents configuration for a tool
type ToolConfig struct {
	ID               string                 `json:"id"`
	TenantID         string                 `json:"tenant_id"`
	Type             string                 `json:"type"`
	Name             string                 `json:"name"`
	DisplayName      string                 `json:"display_name"`
	BaseURL          string                 `json:"base_url"`
	DocumentationURL string                 `json:"documentation_url,omitempty"`
	OpenAPIURL       string                 `json:"openapi_url,omitempty"`
	Config           map[string]interface{} `json:"config"`
	Credential       *TokenCredential       `json:"-"` // Never serialize
	RetryPolicy      *ToolRetryPolicy       `json:"retry_policy,omitempty"`
	Status           string                 `json:"status"`
	HealthStatus     string                 `json:"health_status"`
	LastHealthCheck  *time.Time             `json:"last_health_check,omitempty"`
}

// TokenCredential represents authentication credentials
type TokenCredential struct {
	Type         string `json:"type"` // bearer, api_key, basic, oauth2
	Token        string `json:"token,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
	HeaderName   string `json:"header_name,omitempty"`
	HeaderPrefix string `json:"header_prefix,omitempty"`
}

// ToolRetryPolicy extends base retry policy with tool-specific settings
type ToolRetryPolicy struct {
	MaxAttempts      int           `json:"max_attempts"`
	InitialDelay     time.Duration `json:"initial_delay"`
	MaxDelay         time.Duration `json:"max_delay"`
	Multiplier       float64       `json:"multiplier"`
	Jitter           float64       `json:"jitter"`
	RetryableErrors  []string      `json:"retryable_errors,omitempty"`
	RetryOnTimeout   bool          `json:"retry_on_timeout"`
	RetryOnRateLimit bool          `json:"retry_on_rate_limit"`
}

// HealthCheckConfig defines health check settings
type HealthCheckConfig struct {
	Mode           string        `json:"mode"` // "on-demand", "active", "hybrid"
	CacheDuration  time.Duration `json:"cache_duration"`
	StaleThreshold time.Duration `json:"stale_threshold"`
	CheckTimeout   time.Duration `json:"check_timeout"`
}

// HealthStatus represents the health of a tool
type HealthStatus struct {
	IsHealthy    bool      `json:"is_healthy"`
	LastChecked  time.Time `json:"last_checked"`
	ResponseTime int       `json:"response_time_ms"`
	Error        string    `json:"error,omitempty"`
	Version      string    `json:"version,omitempty"`
	WasCached    bool      `json:"was_cached"`
}

// DiscoveryResult contains the results of API discovery
type DiscoveryResult struct {
	Status           string       `json:"status"`
	OpenAPISpec      *openapi3.T  `json:"-"`
	DiscoveredURLs   []string     `json:"discovered_urls"`
	Capabilities     []Capability `json:"capabilities"`
	RequiresManual   bool         `json:"requires_manual"`
	SuggestedActions []string     `json:"suggested_actions"`
	Error            string       `json:"error,omitempty"`
}

// Capability represents a discovered tool capability
type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Actions     []string `json:"actions"`
}

// SecurityScheme represents an authentication scheme from OpenAPI
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
	Description  string `json:"description,omitempty"`
	BearerFormat string `json:"bearer_format,omitempty"`
}

// CredentialTemplate provides dynamic credential configuration
type CredentialTemplate struct {
	SupportedTypes []string          `json:"supported_types"`
	RequiredFields []CredentialField `json:"required_fields"`
	AuthSchemes    []SecurityScheme  `json:"auth_schemes"`
}

// CredentialField represents a required credential field
type CredentialField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Example     string `json:"example,omitempty"`
}

// DynamicTool represents a dynamically generated tool from OpenAPI
type DynamicTool struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Parameters  *ParameterSchema `json:"parameters"`
	Returns     *ReturnSchema    `json:"returns,omitempty"`
	OperationID string           `json:"operation_id,omitempty"`
	Method      string           `json:"method,omitempty"`
	Path        string           `json:"path,omitempty"`
}
