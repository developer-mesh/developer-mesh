package api

import (
	"errors"
	"net/url"

	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// CreateToolRequest represents a request to create a new tool
type CreateToolRequest struct {
	Name              string                   `json:"name" binding:"required"`
	DisplayName       string                   `json:"display_name"`
	BaseURL           string                   `json:"base_url" binding:"required"`
	DocumentationURL  string                   `json:"documentation_url,omitempty"`
	OpenAPIURL        string                   `json:"openapi_url,omitempty"`
	AuthType          string                   `json:"auth_type" binding:"required"`
	Credentials       *CredentialInput         `json:"credentials,omitempty"`
	Config            map[string]interface{}   `json:"config,omitempty"`
	RetryPolicy       *tools.ToolRetryPolicy   `json:"retry_policy,omitempty"`
	HealthConfig      *tools.HealthCheckConfig `json:"health_config,omitempty"`
	Provider          string                   `json:"provider,omitempty"`
	PassthroughConfig *PassthroughConfig       `json:"passthrough_config,omitempty"`
}

// Validate validates the create tool request
func (r *CreateToolRequest) Validate() error {
	// Validate URL
	if _, err := url.Parse(r.BaseURL); err != nil {
		return errors.New("invalid base URL")
	}

	// Validate auth type
	validAuthTypes := []string{"token", "bearer", "api_key", "basic", "oauth2", "custom"}
	valid := false
	for _, t := range validAuthTypes {
		if r.AuthType == t {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid auth type")
	}

	// Validate credentials if provided
	if r.Credentials != nil {
		if r.AuthType == "basic" && (r.Credentials.Username == "" || r.Credentials.Password == "") {
			return errors.New("username and password required for basic auth")
		}
		if r.AuthType != "basic" && r.Credentials.Token == "" {
			return errors.New("token required for selected auth type")
		}
	}

	return nil
}

// UpdateToolRequest represents a request to update a tool
type UpdateToolRequest struct {
	Name              string                   `json:"name,omitempty"`
	DisplayName       string                   `json:"display_name,omitempty"`
	BaseURL           string                   `json:"base_url,omitempty"`
	DocumentationURL  string                   `json:"documentation_url,omitempty"`
	OpenAPIURL        string                   `json:"openapi_url,omitempty"`
	Config            map[string]interface{}   `json:"config,omitempty"`
	RetryPolicy       *tools.ToolRetryPolicy   `json:"retry_policy,omitempty"`
	HealthConfig      *tools.HealthCheckConfig `json:"health_config,omitempty"`
	PassthroughConfig *PassthroughConfig       `json:"passthrough_config,omitempty"`
}

// CredentialInput represents credential input
type CredentialInput struct {
	Token        string `json:"token,omitempty"`
	HeaderName   string `json:"header_name,omitempty"`
	HeaderPrefix string `json:"header_prefix,omitempty"`
	QueryParam   string `json:"query_param,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
}

// DiscoverToolRequest represents a request to discover a tool
type DiscoverToolRequest struct {
	BaseURL     string                 `json:"base_url" binding:"required"`
	OpenAPIURL  string                 `json:"openapi_url,omitempty"`
	AuthType    string                 `json:"auth_type,omitempty"`
	Credentials *CredentialInput       `json:"credentials,omitempty"`
	Hints       map[string]interface{} `json:"hints,omitempty"`
}

// Validate validates the discover tool request
func (r *DiscoverToolRequest) Validate() error {
	// Validate URL
	if _, err := url.Parse(r.BaseURL); err != nil {
		return errors.New("invalid base URL")
	}

	// If OpenAPI URL provided, validate it
	if r.OpenAPIURL != "" {
		if _, err := url.Parse(r.OpenAPIURL); err != nil {
			return errors.New("invalid OpenAPI URL")
		}
	}

	return nil
}

// ConfirmDiscoveryRequest represents a request to confirm discovery and create a tool
type ConfirmDiscoveryRequest struct {
	Name         string                   `json:"name" binding:"required"`
	DisplayName  string                   `json:"display_name"`
	SelectedURL  string                   `json:"selected_url"`
	AuthType     string                   `json:"auth_type" binding:"required"`
	Credentials  *CredentialInput         `json:"credentials,omitempty"`
	Config       map[string]interface{}   `json:"config,omitempty"`
	RetryPolicy  *tools.ToolRetryPolicy   `json:"retry_policy,omitempty"`
	HealthConfig *tools.HealthCheckConfig `json:"health_config,omitempty"`
}

// UpdateCredentialsRequest represents a request to update tool credentials
type UpdateCredentialsRequest struct {
	AuthType    string           `json:"auth_type" binding:"required"`
	Credentials *CredentialInput `json:"credentials" binding:"required"`
}

// PassthroughConfig defines how user token passthrough should be handled
type PassthroughConfig struct {
	Mode              string `json:"mode"`                // optional, required, disabled
	FallbackToService bool   `json:"fallback_to_service"` // Allow fallback to service account
}

// Tool represents a configured tool with its current state
type Tool struct {
	ID                string                   `json:"id"`
	TenantID          string                   `json:"tenant_id"`
	Name              string                   `json:"name"`
	DisplayName       string                   `json:"display_name"`
	BaseURL           string                   `json:"base_url"`
	DocumentationURL  string                   `json:"documentation_url,omitempty"`
	OpenAPIURL        string                   `json:"openapi_url,omitempty"`
	AuthType          string                   `json:"auth_type"`
	Config            map[string]interface{}   `json:"config"`
	RetryPolicy       *tools.ToolRetryPolicy   `json:"retry_policy,omitempty"`
	HealthConfig      *tools.HealthCheckConfig `json:"health_config,omitempty"`
	Status            string                   `json:"status"`
	HealthStatus      *tools.HealthStatus      `json:"health_status,omitempty"`
	Provider          string                   `json:"provider,omitempty"`
	PassthroughConfig *PassthroughConfig       `json:"passthrough_config,omitempty"`
	CreatedAt         string                   `json:"created_at"`
	UpdatedAt         string                   `json:"updated_at"`

	// Internal fields not exposed in JSON
	InternalConfig tools.ToolConfig `json:"-"`
}

// DiscoverySession represents an active discovery session
type DiscoverySession struct {
	ID             string                 `json:"id"`
	TenantID       string                 `json:"tenant_id"`
	SessionID      string                 `json:"session_id"`
	BaseURL        string                 `json:"base_url"`
	Status         tools.DiscoveryStatus  `json:"status"`
	DiscoveredURLs []string               `json:"discovered_urls,omitempty"`
	SelectedURL    string                 `json:"selected_url,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	CreatedAt      string                 `json:"created_at"`
	ExpiresAt      string                 `json:"expires_at"`
}

// ActionDefinition represents a tool action
type ActionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Parameters  map[string]interface{} `json:"parameters"`
	Returns     map[string]interface{} `json:"returns"`
}

// ExecutionResult represents the result of executing a tool action
type ExecutionResult struct {
	ToolID       string      `json:"tool_id"`
	Action       string      `json:"action"`
	Status       string      `json:"status"`
	Result       interface{} `json:"result,omitempty"`
	Error        string      `json:"error,omitempty"`
	ResponseTime int         `json:"response_time_ms"`
	RetryCount   int         `json:"retry_count"`
	ExecutedAt   string      `json:"executed_at"`
}

// Common errors
var (
	ErrDynamicToolNotFound   = errors.New("tool not found")
	ErrSessionNotFound       = errors.New("discovery session not found")
	ErrDynamicActionNotFound = errors.New("action not found")
)
