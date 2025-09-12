package models

import (
	"encoding/json"
	"time"
)

// DynamicTool represents a dynamically configured tool
type DynamicTool struct {
	ID                   string                 `json:"id" db:"id"`
	TenantID             string                 `json:"tenant_id" db:"tenant_id"`
	ToolName             string                 `json:"tool_name" db:"tool_name"`
	ToolType             string                 `json:"tool_type" db:"tool_type"`
	DisplayName          string                 `json:"display_name" db:"display_name"`
	BaseURL              string                 `json:"base_url" db:"base_url"`
	InputSchema          map[string]interface{} `json:"input_schema,omitempty" db:"-"`
	DocumentationURL     *string                `json:"documentation_url,omitempty" db:"documentation_url"`
	OpenAPIURL           *string                `json:"openapi_url,omitempty" db:"openapi_url"`
	OpenAPISpecURL       *string                `json:"openapi_spec_url,omitempty" db:"openapi_spec_url"`
	OpenAPISpec          *json.RawMessage       `json:"openapi_spec,omitempty" db:"openapi_spec"`
	Config               map[string]interface{} `json:"config" db:"config"`
	AuthType             string                 `json:"auth_type" db:"auth_type"`
	AuthConfig           map[string]interface{} `json:"auth_config,omitempty" db:"auth_config"`
	CredentialConfig     *json.RawMessage       `json:"credential_config,omitempty" db:"credential_config"`
	CredentialsEncrypted []byte                 `json:"-" db:"credentials_encrypted"`
	EncryptedCredentials string                 `json:"-" db:"encrypted_credentials"`
	Headers              *json.RawMessage       `json:"headers,omitempty" db:"headers"`
	APISpec              *json.RawMessage       `json:"api_spec,omitempty" db:"api_spec"`
	DiscoveredEndpoints  *json.RawMessage       `json:"discovered_endpoints,omitempty" db:"discovered_endpoints"`
	HealthCheckConfig    *json.RawMessage       `json:"health_check_config,omitempty" db:"health_check_config"`
	RetryPolicy          *json.RawMessage       `json:"retry_policy,omitempty" db:"retry_policy"`
	HealthConfig         *json.RawMessage       `json:"health_config,omitempty" db:"health_config"`
	Status               string                 `json:"status" db:"status"`
	IsActive             bool                   `json:"is_active" db:"is_active"`
	LastHealthCheck      *time.Time             `json:"last_health_check,omitempty" db:"last_health_check"`
	HealthStatus         *json.RawMessage       `json:"health_status,omitempty" db:"health_status"`
	HealthMessage        *string                `json:"health_message,omitempty" db:"health_message"`
	Description          *string                `json:"description,omitempty" db:"description"`
	Tags                 []string               `json:"tags,omitempty" db:"tags"`
	Metadata             *json.RawMessage       `json:"metadata,omitempty" db:"metadata"`
	Provider             string                 `json:"provider,omitempty" db:"provider"`
	PassthroughConfig    *json.RawMessage       `json:"passthrough_config,omitempty" db:"passthrough_config"`
	WebhookConfig        *json.RawMessage       `json:"webhook_config,omitempty" db:"webhook_config"`
	CreatedAt            time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at" db:"updated_at"`
	CreatedBy            *string                `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy            *string                `json:"updated_by,omitempty" db:"updated_by"`
}

// DiscoverySession represents an active discovery session
type DiscoverySession struct {
	ID                string                 `json:"id" db:"id"`
	SessionID         string                 `json:"session_id" db:"session_id"`
	TenantID          string                 `json:"tenant_id" db:"tenant_id"`
	BaseURL           string                 `json:"base_url" db:"base_url"`
	Status            string                 `json:"status" db:"status"`
	DiscoveryResult   *DiscoveryResult       `json:"discovery_result,omitempty" db:"discovery_result"`
	DiscoveryMetadata map[string]interface{} `json:"discovery_metadata" db:"discovery_metadata"`
	CreatedAt         time.Time              `json:"created_at" db:"created_at"`
	ExpiresAt         time.Time              `json:"expires_at" db:"expires_at"`
}

// ToolHealthStatus represents the health status of a tool
type ToolHealthStatus struct {
	IsHealthy    bool                   `json:"is_healthy"`
	LastChecked  time.Time              `json:"last_checked"`
	ResponseTime int                    `json:"response_time_ms"`
	Error        string                 `json:"error,omitempty"`
	Version      string                 `json:"version,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// ToolAction represents an available action on a tool
type ToolAction struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Parameters  []ActionParameter      `json:"parameters,omitempty"`
	Responses   map[string]interface{} `json:"responses,omitempty"`
}

// ActionParameter represents a parameter for a tool action
type ActionParameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"` // query, path, header, body
	Required    bool        `json:"required"`
	Description string      `json:"description,omitempty"`
	Type        string      `json:"type"`
	Default     interface{} `json:"default,omitempty"`
}

// ToolExecutionRequest represents a request to execute a tool action
type ToolExecutionRequest struct {
	Action          string                 `json:"action"`
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
	Headers         map[string]string      `json:"headers,omitempty"`
	Timeout         int                    `json:"timeout,omitempty"` // in seconds
	PassthroughAuth *PassthroughAuthBundle `json:"passthrough_auth,omitempty"`
}

// ToolExecutionResponse represents the response from executing a tool action
type ToolExecutionResponse struct {
	Success    bool                `json:"success"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       interface{}         `json:"body,omitempty"`
	Error      string              `json:"error,omitempty"`
	Duration   int64               `json:"duration_ms"`
	ExecutedAt time.Time           `json:"executed_at"`

	// Cache metadata fields
	FromCache  bool   `json:"from_cache,omitempty"`
	CacheHit   bool   `json:"cache_hit,omitempty"`
	CacheLevel string `json:"cache_level,omitempty"`
	HitCount   int    `json:"hit_count,omitempty"`
}

// DiscoveryHint provides user-supplied hints for API discovery
type DiscoveryHint struct {
	OpenAPIURL       string            `json:"openapi_url,omitempty"`
	AuthHeaders      map[string]string `json:"auth_headers,omitempty"`
	CustomPaths      []string          `json:"custom_paths,omitempty"`
	APIFormat        string            `json:"api_format,omitempty"`
	ExampleEndpoint  string            `json:"example_endpoint,omitempty"`
	DocumentationURL string            `json:"documentation_url,omitempty"`
}

// ToolRetryPolicy defines retry configuration for a tool
type ToolRetryPolicy struct {
	MaxRetries       int      `json:"max_retries"`
	InitialDelay     int      `json:"initial_delay_ms"`
	MaxDelay         int      `json:"max_delay_ms"`
	Multiplier       float64  `json:"multiplier"`
	RetryableErrors  []string `json:"retryable_errors"`
	RetryOnTimeout   bool     `json:"retry_on_timeout"`
	RetryOnRateLimit bool     `json:"retry_on_rate_limit"`
}

// PassthroughConfig defines how user token passthrough should be handled
type PassthroughConfig struct {
	Mode              string `json:"mode"`                // optional, required, disabled
	FallbackToService bool   `json:"fallback_to_service"` // Allow fallback to service account
}

// DiscoveryResult contains the results of API discovery
type DiscoveryResult struct {
	Status           string                 `json:"status"`
	OpenAPISpec      interface{}            `json:"openapi_spec,omitempty"`
	SpecURL          string                 `json:"spec_url,omitempty"`
	DiscoveredURLs   []string               `json:"discovered_urls"`
	RequiresManual   bool                   `json:"requires_manual"`
	SuggestedActions []string               `json:"suggested_actions,omitempty"`
	Capabilities     []string               `json:"capabilities,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ToolWebhookConfig defines webhook configuration for a dynamic tool
type ToolWebhookConfig struct {
	Enabled               bool                   `json:"enabled"`
	EndpointPath          string                 `json:"endpoint_path,omitempty"`           // e.g., /api/webhooks/tools/{toolId}
	AuthType              string                 `json:"auth_type"`                         // hmac, bearer, basic, signature, none
	AuthConfig            map[string]interface{} `json:"auth_config,omitempty"`             // Auth-specific config (e.g., secret, header names)
	Events                []WebhookEventConfig   `json:"events,omitempty"`                  // Supported events
	Headers               map[string]string      `json:"headers,omitempty"`                 // Expected headers
	PayloadFormat         string                 `json:"payload_format,omitempty"`          // json, form, xml
	SignatureHeader       string                 `json:"signature_header,omitempty"`        // Header containing signature
	SignatureAlgorithm    string                 `json:"signature_algorithm,omitempty"`     // hmac-sha256, hmac-sha1, etc.
	IPWhitelist           []string               `json:"ip_whitelist,omitempty"`            // Allowed source IPs
	DefaultProcessingMode string                 `json:"default_processing_mode,omitempty"` // Default processing mode for events
}

// WebhookEventConfig defines configuration for a specific webhook event type
type WebhookEventConfig struct {
	EventType      string                 `json:"event_type"`   // e.g., "push", "pull_request", "issue"
	PayloadPath    string                 `json:"payload_path"` // JSON path to event type in payload
	SchemaURL      string                 `json:"schema_url,omitempty"`
	TransformRules map[string]interface{} `json:"transform_rules,omitempty"` // Rules to transform payload
	RequiredFields []string               `json:"required_fields,omitempty"` // Required fields in payload
	ProcessingMode string                 `json:"processing_mode,omitempty"` // Processing mode for this event type
}

// WebhookEvent represents a received webhook event
type WebhookEvent struct {
	ID          string                 `json:"id" db:"id"`
	ToolID      string                 `json:"tool_id" db:"tool_id"`
	TenantID    string                 `json:"tenant_id" db:"tenant_id"`
	EventType   string                 `json:"event_type" db:"event_type"`
	Payload     json.RawMessage        `json:"payload" db:"payload"`
	Headers     map[string][]string    `json:"headers" db:"headers"`
	SourceIP    string                 `json:"source_ip" db:"source_ip"`
	ReceivedAt  time.Time              `json:"received_at" db:"received_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty" db:"processed_at"`
	Status      string                 `json:"status" db:"status"` // pending, processed, failed
	Error       string                 `json:"error,omitempty" db:"error"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}
