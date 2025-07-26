package models

import (
	"time"
)

// ToolCredentials represents user-provided credentials for backend tools
type ToolCredentials struct {
	GitHub      *TokenCredential `json:"github,omitempty"`
	Jira        *TokenCredential `json:"jira,omitempty"`
	SonarQube   *TokenCredential `json:"sonarqube,omitempty"`
	Artifactory *TokenCredential `json:"artifactory,omitempty"`
	Jenkins     *TokenCredential `json:"jenkins,omitempty"`
	GitLab      *TokenCredential `json:"gitlab,omitempty"`
	Bitbucket   *TokenCredential `json:"bitbucket,omitempty"`
	// Extensible for more tools
	Custom map[string]*TokenCredential `json:"custom,omitempty"`
}

// TokenCredential represents a single credential
type TokenCredential struct {
	Token        string    `json:"token"`
	Type         string    `json:"type,omitempty"`     // "pat", "oauth", "basic", "bearer", "api_key"
	Username     string    `json:"username,omitempty"` // For basic auth
	Password     string    `json:"password,omitempty"` // For basic auth (encrypted in storage)
	BaseURL      string    `json:"base_url,omitempty"` // For self-hosted instances
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	HeaderName   string    `json:"header_name,omitempty"`   // For API key header name
	HeaderPrefix string    `json:"header_prefix,omitempty"` // For custom header prefix (e.g., "Bearer", "Token")
	QueryParam   string    `json:"query_param,omitempty"`   // For API key as query parameter
}

// CredentialRequest wraps tool requests with optional credentials
type CredentialRequest struct {
	Action      string           `json:"action"`
	Parameters  interface{}      `json:"parameters"`
	Credentials *ToolCredentials `json:"credentials,omitempty"`
}

// SanitizedForLogging returns a version safe for logging
func (tc *ToolCredentials) SanitizedForLogging() map[string]interface{} {
	if tc == nil {
		return nil
	}

	result := make(map[string]interface{})

	if tc.GitHub != nil {
		result["github"] = tc.GitHub.SanitizedForLogging()
	}
	if tc.Jira != nil {
		result["jira"] = tc.Jira.SanitizedForLogging()
	}
	if tc.SonarQube != nil {
		result["sonarqube"] = tc.SonarQube.SanitizedForLogging()
	}
	if tc.Artifactory != nil {
		result["artifactory"] = tc.Artifactory.SanitizedForLogging()
	}
	if tc.Jenkins != nil {
		result["jenkins"] = tc.Jenkins.SanitizedForLogging()
	}
	if tc.GitLab != nil {
		result["gitlab"] = tc.GitLab.SanitizedForLogging()
	}
	if tc.Bitbucket != nil {
		result["bitbucket"] = tc.Bitbucket.SanitizedForLogging()
	}

	if len(tc.Custom) > 0 {
		custom := make(map[string]interface{})
		for k, v := range tc.Custom {
			custom[k] = v.SanitizedForLogging()
		}
		result["custom"] = custom
	}

	return result
}

// SanitizedForLogging returns credential info without sensitive data
func (tc *TokenCredential) SanitizedForLogging() map[string]interface{} {
	if tc == nil {
		return nil
	}

	result := map[string]interface{}{
		"type":         tc.Type,
		"has_token":    tc.Token != "",
		"has_username": tc.Username != "",
		"base_url":     tc.BaseURL,
	}

	// Show last 4 characters of token for identification
	if len(tc.Token) >= 4 {
		result["token_hint"] = "..." + tc.Token[len(tc.Token)-4:]
	}

	if !tc.ExpiresAt.IsZero() {
		result["expires_at"] = tc.ExpiresAt.Format(time.RFC3339)
		result["is_expired"] = tc.ExpiresAt.Before(time.Now())
	}

	return result
}

// IsExpired checks if the credential has expired
func (tc *TokenCredential) IsExpired() bool {
	if tc == nil || tc.ExpiresAt.IsZero() {
		return false
	}
	return tc.ExpiresAt.Before(time.Now())
}

// GetToken returns the token if valid, empty string if expired
func (tc *TokenCredential) GetToken() string {
	if tc == nil || tc.IsExpired() {
		return ""
	}
	return tc.Token
}

// HasCredentialFor checks if credentials exist for a specific tool
func (tc *ToolCredentials) HasCredentialFor(tool string) bool {
	if tc == nil {
		return false
	}

	switch tool {
	case "github":
		return tc.GitHub != nil && tc.GitHub.Token != ""
	case "jira":
		return tc.Jira != nil && tc.Jira.Token != ""
	case "sonarqube":
		return tc.SonarQube != nil && tc.SonarQube.Token != ""
	case "artifactory":
		return tc.Artifactory != nil && tc.Artifactory.Token != ""
	case "jenkins":
		return tc.Jenkins != nil && tc.Jenkins.Token != ""
	case "gitlab":
		return tc.GitLab != nil && tc.GitLab.Token != ""
	case "bitbucket":
		return tc.Bitbucket != nil && tc.Bitbucket.Token != ""
	default:
		if tc.Custom != nil {
			cred, exists := tc.Custom[tool]
			return exists && cred != nil && cred.Token != ""
		}
		return false
	}
}

// GetCredentialFor retrieves the credential for a specific tool
func (tc *ToolCredentials) GetCredentialFor(tool string) *TokenCredential {
	if tc == nil {
		return nil
	}

	switch tool {
	case "github":
		return tc.GitHub
	case "jira":
		return tc.Jira
	case "sonarqube":
		return tc.SonarQube
	case "artifactory":
		return tc.Artifactory
	case "jenkins":
		return tc.Jenkins
	case "gitlab":
		return tc.GitLab
	case "bitbucket":
		return tc.Bitbucket
	default:
		if tc.Custom != nil {
			return tc.Custom[tool]
		}
		return nil
	}
}
