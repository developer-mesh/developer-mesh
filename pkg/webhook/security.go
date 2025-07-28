package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"golang.org/x/time/rate"
)

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	// Webhook signature verification
	EnableSignatureVerification bool              `yaml:"enable_signature_verification" json:"enable_signature_verification"`
	SignatureSecrets            map[string]string `yaml:"signature_secrets" json:"-"` // toolID -> secret
	SignatureHeader             string            `yaml:"signature_header" json:"signature_header"`
	TimestampTolerance          time.Duration     `yaml:"timestamp_tolerance" json:"timestamp_tolerance"`

	// Rate limiting
	EnableRateLimiting bool                     `yaml:"enable_rate_limiting" json:"enable_rate_limiting"`
	GlobalRateLimit    int                      `yaml:"global_rate_limit" json:"global_rate_limit"`       // requests per second
	PerToolRateLimits  map[string]int           `yaml:"per_tool_rate_limits" json:"per_tool_rate_limits"` // toolID -> RPS
	BurstSize          int                      `yaml:"burst_size" json:"burst_size"`
	RateLimiters       map[string]*rate.Limiter `yaml:"-" json:"-"`

	// DDoS protection
	MaxPayloadSize int64         `yaml:"max_payload_size" json:"max_payload_size"`
	MaxHeaderSize  int           `yaml:"max_header_size" json:"max_header_size"`
	IPWhitelist    []string      `yaml:"ip_whitelist" json:"ip_whitelist"`
	IPBlacklist    []string      `yaml:"ip_blacklist" json:"ip_blacklist"`
	BlockDuration  time.Duration `yaml:"block_duration" json:"block_duration"`

	// Audit logging
	EnableAuditLogging bool     `yaml:"enable_audit_logging" json:"enable_audit_logging"`
	AuditLogFields     []string `yaml:"audit_log_fields" json:"audit_log_fields"`
	SensitiveFields    []string `yaml:"sensitive_fields" json:"sensitive_fields"` // Fields to redact

	// Encryption
	EnableEncryption bool   `yaml:"enable_encryption" json:"enable_encryption"`
	EncryptionKey    []byte `yaml:"-" json:"-"` // 32 bytes for AES-256
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		EnableSignatureVerification: true,
		SignatureHeader:             "X-Webhook-Signature",
		TimestampTolerance:          5 * time.Minute,
		EnableRateLimiting:          true,
		GlobalRateLimit:             1000, // 1000 RPS globally
		BurstSize:                   100,
		MaxPayloadSize:              10 * 1024 * 1024, // 10MB
		MaxHeaderSize:               8192,
		BlockDuration:               1 * time.Hour,
		EnableAuditLogging:          true,
		AuditLogFields: []string{
			"event_id", "tenant_id", "tool_id", "event_type", "timestamp",
		},
		SensitiveFields: []string{
			"password", "token", "secret", "key", "authorization",
		},
		EnableEncryption: false, // Requires explicit key configuration
		RateLimiters:     make(map[string]*rate.Limiter),
	}
}

// SecurityService handles webhook security
type SecurityService struct {
	config       *SecurityConfig
	logger       observability.Logger
	rateLimiters map[string]*rate.Limiter
	blockedIPs   map[string]time.Time
}

// NewSecurityService creates a new security service
func NewSecurityService(config *SecurityConfig, logger observability.Logger) *SecurityService {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	// Initialize rate limiters
	if config.EnableRateLimiting {
		// Global rate limiter
		config.RateLimiters["global"] = rate.NewLimiter(rate.Limit(config.GlobalRateLimit), config.BurstSize)

		// Per-tool rate limiters
		for toolID, limit := range config.PerToolRateLimits {
			config.RateLimiters[toolID] = rate.NewLimiter(rate.Limit(limit), config.BurstSize)
		}
	}

	return &SecurityService{
		config:       config,
		logger:       logger,
		rateLimiters: config.RateLimiters,
		blockedIPs:   make(map[string]time.Time),
	}
}

// VerifySignature verifies webhook signature
func (s *SecurityService) VerifySignature(toolID string, payload []byte, signature string, timestamp time.Time) error {
	if !s.config.EnableSignatureVerification {
		return nil
	}

	// Check timestamp to prevent replay attacks
	if time.Since(timestamp) > s.config.TimestampTolerance {
		return fmt.Errorf("webhook timestamp too old: %v", timestamp)
	}

	secret, exists := s.config.SignatureSecrets[toolID]
	if !exists {
		return fmt.Errorf("no signature secret configured for tool: %s", toolID)
	}

	// Calculate expected signature
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(h, "%d.", timestamp.Unix())
	h.Write(payload)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		s.logger.Warn("Webhook signature verification failed", map[string]interface{}{
			"tool_id":  toolID,
			"expected": expectedSignature[:8] + "...", // Log partial for debugging
			"actual":   signature[:8] + "...",
		})
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// CheckRateLimit checks if request is within rate limits
func (s *SecurityService) CheckRateLimit(ctx context.Context, toolID string) error {
	if !s.config.EnableRateLimiting {
		return nil
	}

	// Check global rate limit
	globalLimiter, exists := s.rateLimiters["global"]
	if exists && !globalLimiter.Allow() {
		return fmt.Errorf("global rate limit exceeded")
	}

	// Check per-tool rate limit
	toolLimiter, exists := s.rateLimiters[toolID]
	if exists && !toolLimiter.Allow() {
		return fmt.Errorf("tool rate limit exceeded for %s", toolID)
	}

	return nil
}

// CheckIPAccess checks if IP is allowed
func (s *SecurityService) CheckIPAccess(ip string) error {
	// Check if IP is blocked
	if blockTime, blocked := s.blockedIPs[ip]; blocked {
		if time.Since(blockTime) < s.config.BlockDuration {
			return fmt.Errorf("IP %s is blocked until %v", ip, blockTime.Add(s.config.BlockDuration))
		}
		// Unblock if duration expired
		delete(s.blockedIPs, ip)
	}

	// Check whitelist (if configured)
	if len(s.config.IPWhitelist) > 0 {
		allowed := false
		for _, allowedIP := range s.config.IPWhitelist {
			if ip == allowedIP || strings.HasPrefix(ip, allowedIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("IP %s not in whitelist", ip)
		}
	}

	// Check blacklist
	for _, blockedIP := range s.config.IPBlacklist {
		if ip == blockedIP || strings.HasPrefix(ip, blockedIP) {
			s.blockedIPs[ip] = time.Now()
			return fmt.Errorf("IP %s is blacklisted", ip)
		}
	}

	return nil
}

// ValidatePayloadSize validates payload size
func (s *SecurityService) ValidatePayloadSize(size int64) error {
	if size > s.config.MaxPayloadSize {
		return fmt.Errorf("payload size %d exceeds maximum %d", size, s.config.MaxPayloadSize)
	}
	return nil
}

// RedactSensitiveData redacts sensitive fields from data
func (s *SecurityService) RedactSensitiveData(data map[string]interface{}) map[string]interface{} {
	if !s.config.EnableAuditLogging {
		return data
	}

	redacted := make(map[string]interface{})
	for k, v := range data {
		redacted[k] = s.redactValue(k, v)
	}
	return redacted
}

// redactValue redacts sensitive values
func (s *SecurityService) redactValue(key string, value interface{}) interface{} {
	// Check if key is sensitive
	for _, sensitiveField := range s.config.SensitiveFields {
		if strings.Contains(strings.ToLower(key), strings.ToLower(sensitiveField)) {
			return "[REDACTED]"
		}
	}

	// Recursively redact nested structures
	switch v := value.(type) {
	case map[string]interface{}:
		return s.RedactSensitiveData(v)
	case []interface{}:
		redacted := make([]interface{}, len(v))
		for i, item := range v {
			redacted[i] = s.redactValue(fmt.Sprintf("%s[%d]", key, i), item)
		}
		return redacted
	default:
		return value
	}
}

// AuditLog creates an audit log entry
func (s *SecurityService) AuditLog(event *WebhookEvent, action string, result string, metadata map[string]interface{}) {
	if !s.config.EnableAuditLogging {
		return
	}

	auditEntry := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"action":    action,
		"result":    result,
	}

	// Add configured fields from event
	for _, field := range s.config.AuditLogFields {
		switch field {
		case "event_id":
			auditEntry[field] = event.EventId
		case "tenant_id":
			auditEntry[field] = event.TenantId
		case "tool_id":
			auditEntry[field] = event.ToolId
		case "event_type":
			auditEntry[field] = event.EventType
		case "timestamp":
			auditEntry[field] = event.Timestamp
		}
	}

	// Add metadata (redacted)
	if metadata != nil {
		auditEntry["metadata"] = s.RedactSensitiveData(metadata)
	}

	s.logger.Info("Security audit log", auditEntry)
}

// BlockIP blocks an IP address
func (s *SecurityService) BlockIP(ip string, reason string) {
	s.blockedIPs[ip] = time.Now()
	s.logger.Warn("IP blocked", map[string]interface{}{
		"ip":       ip,
		"reason":   reason,
		"duration": s.config.BlockDuration,
	})
}

// GetMetrics returns security metrics
func (s *SecurityService) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"signature_verification_enabled": s.config.EnableSignatureVerification,
		"rate_limiting_enabled":          s.config.EnableRateLimiting,
		"audit_logging_enabled":          s.config.EnableAuditLogging,
		"encryption_enabled":             s.config.EnableEncryption,
		"blocked_ips_count":              len(s.blockedIPs),
		"rate_limiters_count":            len(s.rateLimiters),
	}

	// Clean up expired blocks
	now := time.Now()
	for ip, blockTime := range s.blockedIPs {
		if now.Sub(blockTime) > s.config.BlockDuration {
			delete(s.blockedIPs, ip)
		}
	}

	return metrics
}
