package jira

import (
	"context"
	"crypto/tls"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/intelligence"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// JiraSecurityConfig holds Jira-specific security settings that extend the base security configuration
type JiraSecurityConfig struct {
	// Inherit base security configuration
	clients.SecurityConfig

	// Jira-specific PII patterns
	JiraCustomPIIPatterns []string `yaml:"jira_custom_pii_patterns" json:"jira_custom_pii_patterns"`

	// Jira-specific sanitization settings
	SanitizeJiraHeaders []string `yaml:"sanitize_jira_headers" json:"sanitize_jira_headers"`
	SanitizeJiraFields  []string `yaml:"sanitize_jira_fields" json:"sanitize_jira_fields"`
}

// JiraSecurityManager integrates existing security packages for Jira-specific needs
type JiraSecurityManager struct {
	logger          observability.Logger
	securityManager *clients.SecurityManager
	securityLayer   *intelligence.SecurityLayer
	config          JiraSecurityConfig
	jiraPatterns    map[string]*regexp.Regexp
}

// NewJiraSecurityManager creates a new Jira security manager using existing security infrastructure
func NewJiraSecurityManager(logger observability.Logger, config JiraSecurityConfig) (*JiraSecurityManager, error) {
	// Create base security manager
	securityManager, err := clients.NewSecurityManager(config.SecurityConfig, logger)
	if err != nil {
		return nil, err
	}

	// Create intelligence security layer
	securityLayer := intelligence.NewSecurityLayer(intelligence.SecurityConfig{})

	jsm := &JiraSecurityManager{
		logger:          logger,
		securityManager: securityManager,
		securityLayer:   securityLayer,
		config:          config,
		jiraPatterns:    make(map[string]*regexp.Regexp),
	}

	// Initialize Jira-specific PII patterns
	jsm.initializeJiraPatterns()

	return jsm, nil
}

// initializeJiraPatterns sets up Jira-specific PII detection patterns
func (jsm *JiraSecurityManager) initializeJiraPatterns() {
	// Jira-specific patterns that extend the base PII detection
	jsm.jiraPatterns["jira_api_token"] = regexp.MustCompile(`ATATT[A-Za-z0-9+/-]{20,}`)
	jsm.jiraPatterns["atlassian_account_id"] = regexp.MustCompile(`[0-9a-f]{6}:[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
	jsm.jiraPatterns["jira_session_id"] = regexp.MustCompile(`JSESSIONID=[A-Za-z0-9]{10,}`)
	jsm.jiraPatterns["jira_webhook_secret"] = regexp.MustCompile(`whsec_[A-Za-z0-9+/=]{32,}`)

	// Add custom patterns from configuration
	for i, pattern := range jsm.config.JiraCustomPIIPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			jsm.jiraPatterns["jira_custom_"+string(rune(i))] = compiled
		} else {
			jsm.logger.Warn("Invalid Jira custom PII pattern", map[string]interface{}{
				"pattern": pattern,
				"error":   err.Error(),
			})
		}
	}
}

// SecureHTTPClient returns an HTTP client with security configurations applied
func (jsm *JiraSecurityManager) SecureHTTPClient() *http.Client {
	// Create a secure HTTP client with Jira-specific settings
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Apply Jira-specific SSL configurations if needed
	if transport, ok := client.Transport.(*http.Transport); ok {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}

		// Set minimum TLS version for Jira (Atlassian requires TLS 1.2+)
		if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
			transport.TLSClientConfig.MinVersion = tls.VersionTLS12
		}
	}

	return client
}

// SanitizeRequest sanitizes outgoing request data using Jira-specific rules
func (jsm *JiraSecurityManager) SanitizeRequest(req *http.Request) error {

	// Apply Jira-specific header sanitization
	for _, header := range jsm.config.SanitizeJiraHeaders {
		if req.Header.Get(header) != "" {
			req.Header.Set(header, "[REDACTED]")
			jsm.LogSecurityEvent("jira_header_sanitized", map[string]interface{}{
				"header": header,
				"url":    req.URL.String(),
			})
		}
	}

	return nil
}

// SanitizeResponse sanitizes incoming response data using Jira-specific sanitization
func (jsm *JiraSecurityManager) SanitizeResponse(data []byte) ([]byte, error) {
	// Apply Jira-specific sanitization
	return jsm.sanitizeJiraSpecificData(data), nil
}

// sanitizeJiraSpecificData applies Jira-specific sanitization rules
func (jsm *JiraSecurityManager) sanitizeJiraSpecificData(data []byte) []byte {
	text := string(data)

	// Sanitize Jira-specific PII patterns
	for patternName, pattern := range jsm.jiraPatterns {
		if pattern.Match(data) {
			text = pattern.ReplaceAllString(text, "[REDACTED]")
			jsm.LogSecurityEvent("jira_pii_sanitized", map[string]interface{}{
				"pattern_type": patternName,
			})
		}
	}

	return []byte(text)
}

// DetectPII detects PII using Jira-specific patterns
func (jsm *JiraSecurityManager) DetectPII(data []byte) ([]string, error) {
	// Create a new PII detector for base patterns
	basePII := intelligence.NewPIIDetector()
	baseMatches := basePII.Detect(data)

	// Then add Jira-specific PII detection
	jiraMatches := jsm.detectJiraSpecificPII(data)

	// Combine results
	allMatches := append(baseMatches, jiraMatches...)

	if len(allMatches) > 0 {
		jsm.LogSecurityEvent("pii_detected", map[string]interface{}{
			"total_matches": len(allMatches),
			"base_matches":  len(baseMatches),
			"jira_matches":  len(jiraMatches),
			"data_length":   len(data),
		})
	}

	return allMatches, nil
}

// detectJiraSpecificPII detects Jira-specific PII patterns
func (jsm *JiraSecurityManager) detectJiraSpecificPII(data []byte) []string {
	text := string(data)
	var matches []string

	for piiType, pattern := range jsm.jiraPatterns {
		if pattern.MatchString(text) {
			matches = append(matches, piiType)
		}
	}

	return matches
}

// calculateJiraConfidence calculates confidence for Jira-specific PII types
func (jsm *JiraSecurityManager) calculateJiraConfidence(piiType string) float64 {
	switch {
	case strings.Contains(piiType, "jira_api_token"):
		return 0.95
	case strings.Contains(piiType, "atlassian_account_id"):
		return 0.90
	case strings.Contains(piiType, "jira_session_id"):
		return 0.85
	case strings.Contains(piiType, "jira_webhook_secret"):
		return 0.90
	default:
		return 0.75
	}
}

// LogSecurityEvent logs a security event using structured logging with Jira context
func (jsm *JiraSecurityManager) LogSecurityEvent(eventType string, details map[string]interface{}) {
	// Add Jira context to the details
	enhancedDetails := make(map[string]interface{})
	for k, v := range details {
		enhancedDetails[k] = v
	}
	enhancedDetails["provider"] = "jira"
	enhancedDetails["component"] = "jira_security_manager"
	enhancedDetails["event_type"] = eventType

	// Log security event
	jsm.logger.Info("Jira security event", enhancedDetails)
}

// ValidateCredentials validates Jira credentials using Jira-specific checks
func (jsm *JiraSecurityManager) ValidateCredentials(ctx context.Context, credentials map[string]string) error {
	// Perform Jira-specific credential validation
	return jsm.validateJiraSpecificCredentials(credentials)
}

// validateJiraSpecificCredentials performs Jira-specific credential validation
func (jsm *JiraSecurityManager) validateJiraSpecificCredentials(credentials map[string]string) error {
	// Check for Jira API token format
	if apiToken, exists := credentials["api_token"]; exists {
		if !jsm.jiraPatterns["jira_api_token"].MatchString(apiToken) {
			// Log but don't fail for non-standard token formats
			jsm.LogSecurityEvent("jira_non_standard_token_format", map[string]interface{}{
				"token_length": len(apiToken),
			})
		}
	}

	// Validate email format for basic auth
	if email, exists := credentials["email"]; exists {
		if !strings.Contains(email, "@") {
			jsm.LogSecurityEvent("jira_invalid_email_format", map[string]interface{}{
				"email_provided": len(email) > 0,
			})
		}
	}

	return nil
}

// GetSecurityMetrics returns Jira-specific security metrics
func (jsm *JiraSecurityManager) GetSecurityMetrics() map[string]interface{} {
	// Return Jira-specific metrics
	return map[string]interface{}{
		"jira_patterns_loaded": len(jsm.jiraPatterns),
		"jira_custom_patterns": len(jsm.config.JiraCustomPIIPatterns),
	}
}

// GetDefaultJiraSecurityConfig returns default Jira security configuration
func GetDefaultJiraSecurityConfig() JiraSecurityConfig {
	return JiraSecurityConfig{
		SecurityConfig: clients.SecurityConfig{
			TokenRotationInterval:  24 * time.Hour,
			TokenTTL:               7 * 24 * time.Hour,
			MaxTokensPerUser:       10,
			EncryptionEnabled:      true,
			EncryptionAlgorithm:    "AES-256-GCM",
			KeyRotationInterval:    7 * 24 * time.Hour,
			RateLimitEnabled:       true,
			RequestsPerMinute:      60,
			BurstSize:              10,
			EnableSecurityHeaders:  true,
			CSPPolicy:              "default-src 'self'",
			AuditEnabled:           true,
			AuditRetention:         30 * 24 * time.Hour,
			ThreatDetectionEnabled: true,
			AnomalyThreshold:       0.8,
		},
		JiraCustomPIIPatterns: []string{
			// Add any additional Jira-specific patterns here
		},
		SanitizeJiraHeaders: []string{
			"X-Atlassian-Token",
			"X-ExperimentalApi",
			"X-Force-Accept-Language",
		},
		SanitizeJiraFields: []string{
			"accountId",
			"emailAddress",
			"displayName",
		},
	}
}
