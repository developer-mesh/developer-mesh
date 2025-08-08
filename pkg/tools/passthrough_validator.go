package tools

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// PassthroughValidator validates passthrough authentication
type PassthroughValidator struct {
	logger observability.Logger

	// Token format validators
	bearerTokenRegex *regexp.Regexp
	apiKeyRegex      *regexp.Regexp
	jwtRegex         *regexp.Regexp
}

// NewPassthroughValidator creates a new validator
func NewPassthroughValidator(logger observability.Logger) *PassthroughValidator {
	return &PassthroughValidator{
		logger:           logger,
		bearerTokenRegex: regexp.MustCompile(`^[A-Za-z0-9\-._~+/]+=*$`),
		apiKeyRegex:      regexp.MustCompile(`^[A-Za-z0-9\-_]{20,}$`),
		jwtRegex:         regexp.MustCompile(`^[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+$`),
	}
}

// ValidateBundle validates a passthrough auth bundle against security policy
func (v *PassthroughValidator) ValidateBundle(
	bundle *models.PassthroughAuthBundle,
	config *models.EnhancedPassthroughConfig,
) error {
	if bundle == nil {
		if config.Mode == "required" {
			return fmt.Errorf("passthrough authentication is required but not provided")
		}
		return nil // Passthrough is optional or disabled
	}

	// Validate against security policy
	if config.SecurityPolicy != nil {
		if err := v.validateSecurityPolicy(bundle, config.SecurityPolicy); err != nil {
			return fmt.Errorf("security policy validation failed: %w", err)
		}
	}

	// Validate credentials
	for toolName, cred := range bundle.Credentials {
		if err := v.validateCredential(cred, config); err != nil {
			return fmt.Errorf("credential validation failed for %s: %w", toolName, err)
		}
	}

	// Validate OAuth tokens
	for toolName, token := range bundle.OAuthTokens {
		if err := v.validateOAuthToken(token); err != nil {
			return fmt.Errorf("OAuth token validation failed for %s: %w", toolName, err)
		}
	}

	// Validate against rules
	if err := v.validateAgainstRules(bundle, config); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}

	return nil
}

// validateSecurityPolicy validates against security policy
func (v *PassthroughValidator) validateSecurityPolicy(
	bundle *models.PassthroughAuthBundle,
	policy *models.PassthroughSecurityPolicy,
) error {
	// Check user context requirement
	if policy.RequireUserContext && (bundle.AgentContext == nil || bundle.AgentContext.UserID == "") {
		return fmt.Errorf("user context is required but not provided")
	}

	// Check agent context requirement
	if policy.RequireAgentContext && bundle.AgentContext == nil {
		return fmt.Errorf("agent context is required but not provided")
	}

	// Validate token age for OAuth tokens
	if policy.MaxTokenAge > 0 {
		for _, token := range bundle.OAuthTokens {
			if token.IsExpired() {
				return fmt.Errorf("OAuth token is expired")
			}

			if !token.ExpiresAt.IsZero() {
				age := time.Until(token.ExpiresAt)
				maxAge := time.Duration(policy.MaxTokenAge) * time.Second
				if age > maxAge {
					return fmt.Errorf("token age exceeds maximum allowed: %v > %v", age, maxAge)
				}
			}
		}
	}

	return nil
}

// validateCredential validates a single credential
func (v *PassthroughValidator) validateCredential(
	cred *models.PassthroughCredential,
	config *models.EnhancedPassthroughConfig,
) error {
	// Check if auth type is supported
	if !v.isAuthTypeSupported(cred.Type, config.SupportedAuthTypes) {
		return fmt.Errorf("auth type %s is not supported", cred.Type)
	}

	// Validate based on credential type
	switch cred.Type {
	case "bearer":
		if cred.Token == "" {
			return fmt.Errorf("bearer token cannot be empty")
		}
		if config.SecurityPolicy != nil && config.SecurityPolicy.ValidateTokenFormat {
			if !v.isValidBearerToken(cred.Token) {
				return fmt.Errorf("invalid bearer token format")
			}
		}

	case "api_key":
		if cred.KeyValue == "" && cred.Token == "" {
			return fmt.Errorf("API key cannot be empty")
		}
		if config.SecurityPolicy != nil && config.SecurityPolicy.ValidateTokenFormat {
			key := cred.KeyValue
			if key == "" {
				key = cred.Token
			}
			if !v.isValidAPIKey(key) {
				return fmt.Errorf("invalid API key format")
			}
		}

	case "basic":
		if cred.Username == "" {
			return fmt.Errorf("username cannot be empty for basic auth")
		}
		if cred.Password == "" {
			return fmt.Errorf("password cannot be empty for basic auth")
		}

	case "aws_signature":
		if cred.Properties == nil {
			return fmt.Errorf("AWS signature requires properties")
		}
		if cred.Properties["access_key_id"] == "" {
			return fmt.Errorf("AWS access key ID is required")
		}
		if cred.Properties["secret_access_key"] == "" {
			return fmt.Errorf("AWS secret access key is required")
		}

	case "oauth2":
		// OAuth2 validation is handled separately

	case "custom":
		// Custom validation based on properties
		if len(cred.Properties) == 0 {
			return fmt.Errorf("custom auth requires properties")
		}
	}

	return nil
}

// validateOAuthToken validates an OAuth token
func (v *PassthroughValidator) validateOAuthToken(token *models.OAuthToken) error {
	if token.AccessToken == "" {
		return fmt.Errorf("access token cannot be empty")
	}

	if token.IsExpired() {
		return fmt.Errorf("OAuth token is expired")
	}

	// Validate token type
	if token.TokenType == "" {
		token.TokenType = "Bearer" // Default
	}

	// Check if token looks like a JWT
	if v.jwtRegex.MatchString(token.AccessToken) {
		// Could do additional JWT validation here
		v.logger.Debug("OAuth token appears to be a JWT", map[string]interface{}{
			"has_refresh": token.RefreshToken != "",
			"expires_at":  token.ExpiresAt,
		})
	}

	return nil
}

// validateAgainstRules validates the bundle against configured rules
func (v *PassthroughValidator) validateAgainstRules(
	bundle *models.PassthroughAuthBundle,
	config *models.EnhancedPassthroughConfig,
) error {
	if len(config.Rules) == 0 {
		return nil // No rules to validate against
	}

	// Get context for rule matching
	agentType := ""
	environment := ""
	if bundle.AgentContext != nil {
		agentType = bundle.AgentContext.AgentType
		environment = bundle.AgentContext.Environment
	}

	// Find applicable rule (highest priority first)
	var applicableRule *models.PassthroughRule
	for _, rule := range config.Rules {
		if v.ruleMatches(rule, agentType, environment) {
			if applicableRule == nil || rule.Priority > applicableRule.Priority {
				applicableRule = &rule
			}
		}
	}

	if applicableRule == nil {
		// No specific rule found, check if there's a default
		for _, rule := range config.Rules {
			if len(rule.AgentTypes) == 0 && len(rule.Environments) == 0 {
				applicableRule = &rule
				break
			}
		}
	}

	// Apply rule validation
	if applicableRule != nil {
		if applicableRule.AuthRequired {
			hasAuth := len(bundle.Credentials) > 0 ||
				len(bundle.OAuthTokens) > 0 ||
				len(bundle.SessionTokens) > 0

			if !hasAuth {
				return fmt.Errorf("authentication required by rule %s", applicableRule.RuleID)
			}
		}
	}

	return nil
}

// ValidateDomain validates if a URL is allowed by security policy
func (v *PassthroughValidator) ValidateDomain(targetURL string, policy *models.PassthroughSecurityPolicy) error {
	if policy == nil {
		return nil
	}

	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	domain := u.Hostname()

	// Check blocked domains first
	for _, blocked := range policy.BlockedDomains {
		if v.domainMatches(domain, blocked) {
			return fmt.Errorf("domain %s is blocked", domain)
		}
	}

	// If allowed domains are specified, check if domain is in the list
	if len(policy.AllowedDomains) > 0 {
		allowed := false
		for _, allowedDomain := range policy.AllowedDomains {
			if v.domainMatches(domain, allowedDomain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("domain %s is not in allowed list", domain)
		}
	}

	// Check for encryption requirement
	if policy.RequireEncryption && u.Scheme != "https" {
		return fmt.Errorf("HTTPS is required but URL uses %s", u.Scheme)
	}

	return nil
}

// Helper functions

func (v *PassthroughValidator) isAuthTypeSupported(authType string, supported []string) bool {
	if len(supported) == 0 {
		return true // No restrictions
	}

	for _, s := range supported {
		if s == authType || s == "*" {
			return true
		}
	}
	return false
}

func (v *PassthroughValidator) isValidBearerToken(token string) bool {
	// Check if it's a JWT
	if v.jwtRegex.MatchString(token) {
		return true
	}
	// Check general bearer token format
	return v.bearerTokenRegex.MatchString(token)
}

func (v *PassthroughValidator) isValidAPIKey(key string) bool {
	return v.apiKeyRegex.MatchString(key)
}

func (v *PassthroughValidator) ruleMatches(rule models.PassthroughRule, agentType, environment string) bool {
	// Check agent type match
	if len(rule.AgentTypes) > 0 {
		matched := false
		for _, t := range rule.AgentTypes {
			if t == agentType || t == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check environment match
	if len(rule.Environments) > 0 {
		matched := false
		for _, e := range rule.Environments {
			if e == environment || e == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func (v *PassthroughValidator) domainMatches(domain, pattern string) bool {
	// Support wildcard patterns like *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:]
		return strings.HasSuffix(domain, suffix)
	}
	return domain == pattern
}
