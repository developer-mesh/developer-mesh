package models

import (
	"encoding/json"
	"strings"
	"time"
)

// PassthroughAuthBundle contains all authentication credentials for a request
type PassthroughAuthBundle struct {
	// Support multiple credentials for different services
	Credentials map[string]*PassthroughCredential `json:"credentials,omitempty"`

	// Session tokens for stateful APIs
	SessionTokens map[string]string `json:"session_tokens,omitempty"`

	// OAuth tokens with refresh capability
	OAuthTokens map[string]*OAuthToken `json:"oauth_tokens,omitempty"`

	// Raw headers for custom auth
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`

	// Agent metadata for audit/security
	AgentContext *AgentContext `json:"agent_context,omitempty"`
}

// PassthroughCredential represents a single authentication credential
type PassthroughCredential struct {
	Type       string            `json:"type"` // bearer, api_key, basic, custom, aws_signature, digest
	Token      string            `json:"token,omitempty"`
	Username   string            `json:"username,omitempty"`
	Password   string            `json:"password,omitempty"`
	KeyName    string            `json:"key_name,omitempty"`
	KeyValue   string            `json:"key_value,omitempty"`
	Properties map[string]string `json:"properties,omitempty"` // Flexible fields for tool-specific needs
}

// OAuthToken represents an OAuth2 token with metadata
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Scope        string    `json:"scope,omitempty"`
}

// AgentContext provides context about the agent making the request
type AgentContext struct {
	AgentType   string `json:"agent_type"` // ide, cli, ci, browser, slack, etc.
	AgentID     string `json:"agent_id"`
	UserID      string `json:"user_id"`
	SessionID   string `json:"session_id"`
	Environment string `json:"environment"` // development, staging, production
}

// EnhancedPassthroughConfig defines comprehensive passthrough configuration for a tool
type EnhancedPassthroughConfig struct {
	Mode               string                       `json:"mode"` // required, optional, disabled, hybrid
	FallbackToService  bool                         `json:"fallback_to_service"`
	SupportedAuthTypes []string                     `json:"supported_auth_types"`
	Rules              []PassthroughRule            `json:"rules"`
	AuthMapping        map[string]AuthMappingConfig `json:"auth_mapping"`
	SecurityPolicy     *PassthroughSecurityPolicy   `json:"security_policy"`
}

// PassthroughRule defines when passthrough auth is allowed
type PassthroughRule struct {
	RuleID        string   `json:"rule_id"`
	AgentTypes    []string `json:"agent_types"`
	Actions       []string `json:"actions"` // Specific actions that allow passthrough
	Environments  []string `json:"environments"`
	AuthRequired  bool     `json:"auth_required"`
	AllowFallback bool     `json:"allow_fallback"`
	Priority      int      `json:"priority"` // Higher priority rules are evaluated first
}

// AuthMappingConfig defines how to map incoming auth to tool-specific auth
type AuthMappingConfig struct {
	SourceType string            `json:"source_type"` // What we receive
	TargetType string            `json:"target_type"` // What tool expects
	HeaderName string            `json:"header_name,omitempty"`
	QueryParam string            `json:"query_param,omitempty"`
	Transform  string            `json:"transform,omitempty"` // base64, hex, url_encode, etc.
	Prefix     string            `json:"prefix,omitempty"`    // e.g., "Bearer ", "token "
	Properties map[string]string `json:"properties,omitempty"`
}

// PassthroughSecurityPolicy defines security requirements for passthrough auth
type PassthroughSecurityPolicy struct {
	RequireEncryption   bool     `json:"require_encryption"`
	AllowedDomains      []string `json:"allowed_domains"`
	BlockedDomains      []string `json:"blocked_domains"`
	MaxTokenAge         int      `json:"max_token_age_seconds"`
	RequireUserContext  bool     `json:"require_user_context"`
	RequireAgentContext bool     `json:"require_agent_context"`
	AuditLevel          string   `json:"audit_level"` // none, basic, detailed, verbose
	ValidateTokenFormat bool     `json:"validate_token_format"`
}

// Validate performs validation on the PassthroughAuthBundle
func (p *PassthroughAuthBundle) Validate() error {
	// Basic validation logic
	if p == nil {
		return nil // Passthrough is optional
	}

	// Validate OAuth tokens expiry
	for name, token := range p.OAuthTokens {
		if token.AccessToken == "" {
			return &ValidationError{
				Field:   "oauth_tokens." + name + ".access_token",
				Message: "access token cannot be empty",
			}
		}
	}

	return nil
}

// GetCredentialForTool returns the most appropriate credential for a tool
func (p *PassthroughAuthBundle) GetCredentialForTool(toolName string) *PassthroughCredential {
	if p == nil || p.Credentials == nil {
		return nil
	}

	// First, try exact match
	if cred, ok := p.Credentials[toolName]; ok {
		return cred
	}

	// Then try wildcard
	if cred, ok := p.Credentials["*"]; ok {
		return cred
	}

	// Try lowercase match
	if cred, ok := p.Credentials[toLowercase(toolName)]; ok {
		return cred
	}

	return nil
}

// GetOAuthTokenForTool returns the OAuth token for a specific tool
func (p *PassthroughAuthBundle) GetOAuthTokenForTool(toolName string) *OAuthToken {
	if p == nil || p.OAuthTokens == nil {
		return nil
	}

	if token, ok := p.OAuthTokens[toolName]; ok {
		return token
	}

	// Try wildcard
	if token, ok := p.OAuthTokens["*"]; ok {
		return token
	}

	return nil
}

// IsExpired checks if the OAuth token is expired
func (o *OAuthToken) IsExpired() bool {
	if o.ExpiresAt.IsZero() {
		return false // No expiry set
	}
	return time.Now().After(o.ExpiresAt)
}

// ShouldRefresh checks if the token should be refreshed (80% of lifetime)
func (o *OAuthToken) ShouldRefresh() bool {
	if o.ExpiresAt.IsZero() || o.RefreshToken == "" {
		return false
	}

	lifetime := time.Until(o.ExpiresAt)
	return lifetime < (lifetime * 20 / 100) // Less than 20% remaining
}

// MarshalJSON implements custom JSON marshaling to handle sensitive data
func (p *PassthroughCredential) MarshalJSON() ([]byte, error) {
	type Alias PassthroughCredential
	return json.Marshal(&struct {
		*Alias
		Token    string `json:"token,omitempty"`
		Password string `json:"password,omitempty"`
	}{
		Alias:    (*Alias)(p),
		Token:    maskSensitive(p.Token),
		Password: maskSensitive(p.Password),
	})
}

// Helper functions
func toLowercase(s string) string {
	return strings.ToLower(s)
}

func maskSensitive(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
