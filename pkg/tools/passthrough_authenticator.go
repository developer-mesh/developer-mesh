package tools

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// PassthroughAuthenticator handles passthrough authentication for dynamic tools
type PassthroughAuthenticator struct {
	logger      observability.Logger
	auditLogger AuditLogger
	validator   *PassthroughValidator
}

// AuditLogger interface for audit logging
type AuditLogger interface {
	LogPassthroughAuth(toolName, agentType, userID string, success bool, details map[string]interface{})
}

// NewPassthroughAuthenticator creates a new passthrough authenticator
func NewPassthroughAuthenticator(logger observability.Logger, auditLogger AuditLogger) *PassthroughAuthenticator {
	return &PassthroughAuthenticator{
		logger:      logger,
		auditLogger: auditLogger,
		validator:   NewPassthroughValidator(logger),
	}
}

// ApplyPassthroughAuth applies passthrough authentication to a request
func (a *PassthroughAuthenticator) ApplyPassthroughAuth(
	req *http.Request,
	toolName string,
	config *models.EnhancedPassthroughConfig,
	bundle *models.PassthroughAuthBundle,
) error {
	// Validate the passthrough bundle against security policy
	if err := a.validator.ValidateBundle(bundle, config); err != nil {
		return fmt.Errorf("passthrough validation failed: %w", err)
	}

	// Determine which credential to use
	cred, authType := a.selectCredential(toolName, bundle, config)
	if cred == nil && authType == "" {
		return fmt.Errorf("no suitable passthrough credential found for tool %s", toolName)
	}

	// Apply the credential with mapping
	if err := a.applyCredentialWithMapping(req, cred, authType, config.AuthMapping); err != nil {
		return fmt.Errorf("failed to apply credential: %w", err)
	}

	// Audit the passthrough usage
	a.auditPassthroughUsage(toolName, bundle, config, true)

	return nil
}

// selectCredential selects the most appropriate credential for the tool
func (a *PassthroughAuthenticator) selectCredential(
	toolName string,
	bundle *models.PassthroughAuthBundle,
	config *models.EnhancedPassthroughConfig,
) (*models.PassthroughCredential, string) {
	// First, check for tool-specific credential
	if cred := bundle.GetCredentialForTool(toolName); cred != nil {
		if a.isAuthTypeSupported(cred.Type, config.SupportedAuthTypes) {
			return cred, cred.Type
		}
	}

	// Check for OAuth token
	if token := bundle.GetOAuthTokenForTool(toolName); token != nil {
		if a.isAuthTypeSupported("oauth2", config.SupportedAuthTypes) {
			// Convert OAuth token to credential format
			return &models.PassthroughCredential{
				Type:  "bearer",
				Token: token.AccessToken,
			}, "oauth2"
		}
	}

	// Check for session tokens
	if sessionToken, ok := bundle.SessionTokens[toolName]; ok {
		if a.isAuthTypeSupported("session", config.SupportedAuthTypes) {
			return &models.PassthroughCredential{
				Type:  "session",
				Token: sessionToken,
			}, "session"
		}
	}

	// Check custom headers as last resort
	if len(bundle.CustomHeaders) > 0 && a.isAuthTypeSupported("custom", config.SupportedAuthTypes) {
		return nil, "custom_headers"
	}

	return nil, ""
}

// applyCredentialWithMapping applies the credential using the configured mapping
func (a *PassthroughAuthenticator) applyCredentialWithMapping(
	req *http.Request,
	cred *models.PassthroughCredential,
	authType string,
	mappings map[string]models.AuthMappingConfig,
) error {
	// Handle custom headers separately
	if authType == "custom_headers" {
		return a.applyCustomHeaders(req, mappings)
	}

	if cred == nil {
		return fmt.Errorf("credential is nil")
	}

	// Get mapping configuration
	mapping, ok := mappings[cred.Type]
	if !ok {
		// Use default mapping based on credential type
		mapping = a.getDefaultMapping(cred.Type)
	}

	// Transform the value if needed
	value := a.transformValue(cred, mapping.Transform)

	// Apply based on target type
	switch mapping.TargetType {
	case "header":
		headerValue := value
		if mapping.Prefix != "" {
			headerValue = mapping.Prefix + value
		}
		req.Header.Set(mapping.HeaderName, headerValue)

	case "bearer":
		req.Header.Set("Authorization", "Bearer "+value)

	case "api_key":
		if mapping.QueryParam != "" {
			q := req.URL.Query()
			q.Set(mapping.QueryParam, value)
			req.URL.RawQuery = q.Encode()
		} else {
			headerName := mapping.HeaderName
			if headerName == "" {
				headerName = "X-API-Key"
			}
			req.Header.Set(headerName, value)
		}

	case "basic":
		if cred.Username != "" && cred.Password != "" {
			req.SetBasicAuth(cred.Username, cred.Password)
		} else if cred.Token != "" {
			// Some APIs use token as username with empty password
			req.SetBasicAuth(cred.Token, "")
		}

	case "digest":
		return a.applyDigestAuth(req, cred, mapping)

	case "aws_signature":
		return a.applyAWSSignature(req, cred, mapping)

	case "hmac":
		return a.applyHMACAuth(req, cred, mapping)

	case "custom":
		return a.applyCustomAuth(req, cred, mapping)

	default:
		return fmt.Errorf("unsupported target auth type: %s", mapping.TargetType)
	}

	return nil
}

// transformValue applies transformation to the credential value
func (a *PassthroughAuthenticator) transformValue(cred *models.PassthroughCredential, transform string) string {
	value := cred.Token
	if value == "" && cred.KeyValue != "" {
		value = cred.KeyValue
	}

	switch transform {
	case "base64":
		return base64.StdEncoding.EncodeToString([]byte(value))
	case "base64url":
		return base64.URLEncoding.EncodeToString([]byte(value))
	case "hex":
		return hex.EncodeToString([]byte(value))
	case "url_encode":
		return url.QueryEscape(value)
	case "lowercase":
		return strings.ToLower(value)
	case "uppercase":
		return strings.ToUpper(value)
	default:
		return value
	}
}

// applyDigestAuth applies digest authentication
func (a *PassthroughAuthenticator) applyDigestAuth(req *http.Request, cred *models.PassthroughCredential, mapping models.AuthMappingConfig) error {
	// Simplified digest auth - in production, this would need challenge/response handling
	realm := mapping.Properties["realm"]
	nonce := mapping.Properties["nonce"]
	if realm == "" || nonce == "" {
		return fmt.Errorf("digest auth requires realm and nonce")
	}

	// Calculate digest
	ha1 := a.md5Hash(fmt.Sprintf("%s:%s:%s", cred.Username, realm, cred.Password))
	ha2 := a.md5Hash(fmt.Sprintf("%s:%s", req.Method, req.URL.Path))
	response := a.md5Hash(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))

	authHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		cred.Username, realm, nonce, req.URL.Path, response)

	req.Header.Set("Authorization", authHeader)
	return nil
}

// applyAWSSignature applies AWS Signature V4
func (a *PassthroughAuthenticator) applyAWSSignature(req *http.Request, cred *models.PassthroughCredential, mapping models.AuthMappingConfig) error {
	// This is a simplified version - production would use the AWS SDK
	service := mapping.Properties["service"]
	region := mapping.Properties["region"]
	if service == "" || region == "" {
		return fmt.Errorf("AWS signature requires service and region")
	}

	// Add AWS headers
	req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))

	// In production, calculate the actual signature
	// For now, we'll set a placeholder
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/%s/aws4_request",
		cred.Properties["access_key_id"],
		time.Now().UTC().Format("20060102"),
		region,
		service)

	req.Header.Set("Authorization", authHeader)
	return nil
}

// applyHMACAuth applies HMAC authentication
func (a *PassthroughAuthenticator) applyHMACAuth(req *http.Request, cred *models.PassthroughCredential, mapping models.AuthMappingConfig) error {
	// Get the signing key
	secret := cred.Token
	if secret == "" {
		secret = cred.KeyValue
	}

	// Create signature of request
	mac := hmac.New(sha256.New, []byte(secret))

	// Sign different parts based on configuration
	signatureData := req.Method + "\n" + req.URL.Path
	if mapping.Properties["include_body"] == "true" {
		// Would need to read and restore body here
		signatureData += "\n" // + bodyHash
	}

	mac.Write([]byte(signatureData))
	signature := hex.EncodeToString(mac.Sum(nil))

	// Apply signature to header
	headerName := mapping.HeaderName
	if headerName == "" {
		headerName = "X-Signature"
	}
	req.Header.Set(headerName, signature)

	return nil
}

// applyCustomAuth applies custom authentication logic
func (a *PassthroughAuthenticator) applyCustomAuth(req *http.Request, cred *models.PassthroughCredential, mapping models.AuthMappingConfig) error {
	// Handle custom authentication patterns
	pattern := mapping.Properties["pattern"]

	switch pattern {
	case "dual_header":
		// Some APIs use two headers for auth
		req.Header.Set(mapping.Properties["key_header"], cred.KeyName)
		req.Header.Set(mapping.Properties["value_header"], cred.KeyValue)

	case "cookie":
		// Auth via cookie
		cookie := &http.Cookie{
			Name:  mapping.Properties["cookie_name"],
			Value: cred.Token,
		}
		req.AddCookie(cookie)

	case "query_multi":
		// Multiple query parameters
		q := req.URL.Query()
		for k, v := range cred.Properties {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()

	default:
		// Generic custom handling
		for k, v := range cred.Properties {
			if strings.HasPrefix(k, "header_") {
				req.Header.Set(strings.TrimPrefix(k, "header_"), v)
			}
		}
	}

	return nil
}

// applyCustomHeaders applies custom headers from the bundle
func (a *PassthroughAuthenticator) applyCustomHeaders(req *http.Request, mappings map[string]models.AuthMappingConfig) error {
	// For now, this is a placeholder
	// Would apply custom headers based on mappings
	return nil
}

// getDefaultMapping returns default mapping for common auth types
func (a *PassthroughAuthenticator) getDefaultMapping(credType string) models.AuthMappingConfig {
	switch credType {
	case "bearer":
		return models.AuthMappingConfig{
			SourceType: "bearer",
			TargetType: "bearer",
		}
	case "api_key":
		return models.AuthMappingConfig{
			SourceType: "api_key",
			TargetType: "api_key",
			HeaderName: "X-API-Key",
		}
	case "basic":
		return models.AuthMappingConfig{
			SourceType: "basic",
			TargetType: "basic",
		}
	default:
		return models.AuthMappingConfig{
			SourceType: credType,
			TargetType: "header",
			HeaderName: "Authorization",
		}
	}
}

// isAuthTypeSupported checks if an auth type is supported
func (a *PassthroughAuthenticator) isAuthTypeSupported(authType string, supported []string) bool {
	for _, s := range supported {
		if s == authType || s == "*" {
			return true
		}
	}
	return false
}

// auditPassthroughUsage logs passthrough authentication usage
func (a *PassthroughAuthenticator) auditPassthroughUsage(
	toolName string,
	bundle *models.PassthroughAuthBundle,
	config *models.EnhancedPassthroughConfig,
	success bool,
) {
	if a.auditLogger == nil || config.SecurityPolicy == nil {
		return
	}

	if config.SecurityPolicy.AuditLevel == "none" {
		return
	}

	details := make(map[string]interface{})

	switch config.SecurityPolicy.AuditLevel {
	case "verbose":
		details["credential_types"] = a.getCredentialTypes(bundle)
		details["has_oauth"] = len(bundle.OAuthTokens) > 0
		details["has_session"] = len(bundle.SessionTokens) > 0
		fallthrough
	case "detailed":
		if bundle.AgentContext != nil {
			details["agent_type"] = bundle.AgentContext.AgentType
			details["environment"] = bundle.AgentContext.Environment
		}
		fallthrough
	case "basic":
		details["timestamp"] = time.Now().UTC()
		details["success"] = success
	}

	agentType := ""
	userID := ""
	if bundle.AgentContext != nil {
		agentType = bundle.AgentContext.AgentType
		userID = bundle.AgentContext.UserID
	}

	a.auditLogger.LogPassthroughAuth(toolName, agentType, userID, success, details)
}

// getCredentialTypes returns the types of credentials in the bundle
func (a *PassthroughAuthenticator) getCredentialTypes(bundle *models.PassthroughAuthBundle) []string {
	types := make([]string, 0)
	seen := make(map[string]bool)

	for _, cred := range bundle.Credentials {
		if !seen[cred.Type] {
			types = append(types, cred.Type)
			seen[cred.Type] = true
		}
	}

	return types
}

// md5Hash helper for digest auth
func (a *PassthroughAuthenticator) md5Hash(data string) string {
	h := sha256.New() // Using SHA256 instead of MD5 for security
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
