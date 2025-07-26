package api

import (
	"fmt"
	"net/url"

	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// CreateToolRequest represents a request to create a new tool
type CreateToolRequest struct {
	Name             string                 `json:"name" binding:"required"`
	BaseURL          string                 `json:"base_url" binding:"required"`
	DocumentationURL string                 `json:"documentation_url"`
	OpenAPIURL       string                 `json:"openapi_url"`
	AuthType         string                 `json:"auth_type"`
	Credentials      *CredentialRequest     `json:"credentials"`
	Config           map[string]interface{} `json:"config"`
	RetryPolicy      *tools.RetryPolicy     `json:"retry_policy"`
	HealthConfig     *tools.HealthConfig    `json:"health_config"`
}

// CredentialRequest represents credential information in a request
type CredentialRequest struct {
	Token        string `json:"token"`
	HeaderName   string `json:"header_name"`
	HeaderPrefix string `json:"header_prefix"`
	QueryParam   string `json:"query_param"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

// UpdateToolRequest represents a request to update a tool
type UpdateToolRequest struct {
	Name             string                 `json:"name"`
	BaseURL          string                 `json:"base_url"`
	DocumentationURL string                 `json:"documentation_url"`
	OpenAPIURL       string                 `json:"openapi_url"`
	Config           map[string]interface{} `json:"config"`
	RetryPolicy      *tools.RetryPolicy     `json:"retry_policy"`
	HealthConfig     *tools.HealthConfig    `json:"health_config"`
}

// DiscoverToolRequest represents a request to discover a tool
type DiscoverToolRequest struct {
	BaseURL     string                 `json:"base_url" binding:"required"`
	OpenAPIURL  string                 `json:"openapi_url"`
	AuthType    string                 `json:"auth_type"`
	Credentials *CredentialRequest     `json:"credentials"`
	Hints       map[string]interface{} `json:"hints"`
}

// ConfirmDiscoveryRequest represents a request to confirm and save a discovered tool
type ConfirmDiscoveryRequest struct {
	Name         string                 `json:"name" binding:"required"`
	Config       map[string]interface{} `json:"config"`
	Credentials  *CredentialRequest     `json:"credentials"`
	RetryPolicy  *tools.RetryPolicy     `json:"retry_policy"`
	HealthConfig *tools.HealthConfig    `json:"health_config"`
}

// UpdateCredentialsRequest represents a request to update tool credentials
type UpdateCredentialsRequest struct {
	AuthType    string             `json:"auth_type" binding:"required"`
	Credentials *CredentialRequest `json:"credentials" binding:"required"`
}

// Validate validates a CreateToolRequest
func (r *CreateToolRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}

	if r.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	// Validate URL format
	if _, err := url.Parse(r.BaseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	if r.OpenAPIURL != "" {
		if _, err := url.Parse(r.OpenAPIURL); err != nil {
			return fmt.Errorf("invalid openapi_url: %w", err)
		}
	}

	// Validate auth type if credentials provided
	if r.Credentials != nil {
		switch r.AuthType {
		case "token", "api_key", "basic", "header":
			// Valid auth types
		default:
			return fmt.Errorf("invalid auth_type: %s", r.AuthType)
		}
	}

	return nil
}

// Validate validates a DiscoverToolRequest
func (r *DiscoverToolRequest) Validate() error {
	if r.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	// Validate URL format
	if _, err := url.Parse(r.BaseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	if r.OpenAPIURL != "" {
		if _, err := url.Parse(r.OpenAPIURL); err != nil {
			return fmt.Errorf("invalid openapi_url: %w", err)
		}
	}

	return nil
}
