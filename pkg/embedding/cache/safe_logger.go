package cache

import (
	"regexp"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SafeLogger wraps the standard logger to redact sensitive data
type SafeLogger struct {
	logger   observability.Logger
	redactor *SensitiveDataRedactor
}

// NewSafeLogger creates a new safe logger instance
func NewSafeLogger(logger observability.Logger) *SafeLogger {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.safe")
	}
	return &SafeLogger{
		logger:   logger,
		redactor: NewSensitiveDataRedactor(),
	}
}

// SensitiveDataRedactor handles redaction of sensitive information
type SensitiveDataRedactor struct {
	patterns []*regexp.Regexp
}

// NewSensitiveDataRedactor creates a new redactor with default patterns
func NewSensitiveDataRedactor() *SensitiveDataRedactor {
	patterns := []*regexp.Regexp{
		// API keys
		regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*["']?([^"'\s]+)["']?`),
		// Passwords
		regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["']?([^"'\s]+)["']?`),
		// Tokens
		regexp.MustCompile(`(?i)(token|access[_-]?token|refresh[_-]?token)\s*[:=]\s*["']?([^"'\s]+)["']?`),
		// Secrets
		regexp.MustCompile(`(?i)(secret|secret[_-]?key)\s*[:=]\s*["']?([^"'\s]+)["']?`),
		// Private keys
		regexp.MustCompile(`(?i)(private[_-]?key|privatekey)\s*[:=]\s*["']?([^"'\s]+)["']?`),
		// Bearer tokens in Authorization headers
		regexp.MustCompile(`(?i)Bearer\s+([^\s]+)`),
		// Basic auth
		regexp.MustCompile(`(?i)Basic\s+([^\s]+)`),
		// Credit card patterns
		regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
		// SSN patterns
		regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	}

	return &SensitiveDataRedactor{
		patterns: patterns,
	}
}

// Redact removes sensitive data from a string
func (r *SensitiveDataRedactor) Redact(input string) string {
	result := input

	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// Keep the label but redact the value
			parts := pattern.FindStringSubmatch(match)
			if len(parts) > 2 {
				// For key=value patterns, keep the key and redact the value
				return strings.Replace(match, parts[2], "[REDACTED]", 1)
			}
			// For standalone patterns like credit cards, redact the whole thing
			return "[REDACTED]"
		})
	}

	return result
}

// RedactMap redacts sensitive data from a map
func (r *SensitiveDataRedactor) RedactMap(data map[string]interface{}) map[string]interface{} {
	redacted := make(map[string]interface{})

	sensitiveKeys := []string{
		"api_key", "apikey", "api-key",
		"password", "passwd", "pwd",
		"token", "access_token", "refresh_token",
		"secret", "secret_key", "secret-key",
		"private_key", "private-key", "privatekey",
		"credential", "credentials",
		"authorization", "auth",
		"ssn", "social_security_number",
		"credit_card", "card_number",
		"cvv", "cvc",
	}

	for key, value := range data {
		lowerKey := strings.ToLower(key)
		shouldRedact := false

		// Check if key contains sensitive patterns
		for _, sensitiveKey := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitiveKey) {
				shouldRedact = true
				break
			}
		}

		if shouldRedact {
			redacted[key] = "[REDACTED]"
		} else {
			// Check if value is a string and redact it
			if strValue, ok := value.(string); ok {
				redacted[key] = r.Redact(strValue)
			} else {
				redacted[key] = value
			}
		}
	}

	return redacted
}

// Logger interface implementation

// Debug logs at debug level with sensitive data redaction
func (s *SafeLogger) Debug(message string, fields map[string]interface{}) {
	s.logger.Debug(s.redactor.Redact(message), s.redactor.RedactMap(fields))
}

// Info logs at info level with sensitive data redaction
func (s *SafeLogger) Info(message string, fields map[string]interface{}) {
	s.logger.Info(s.redactor.Redact(message), s.redactor.RedactMap(fields))
}

// Warn logs at warn level with sensitive data redaction
func (s *SafeLogger) Warn(message string, fields map[string]interface{}) {
	s.logger.Warn(s.redactor.Redact(message), s.redactor.RedactMap(fields))
}

// Error logs at error level with sensitive data redaction
func (s *SafeLogger) Error(message string, fields map[string]interface{}) {
	s.logger.Error(s.redactor.Redact(message), s.redactor.RedactMap(fields))
}

// Fatal logs at fatal level with sensitive data redaction
func (s *SafeLogger) Fatal(message string, fields map[string]interface{}) {
	s.logger.Fatal(s.redactor.Redact(message), s.redactor.RedactMap(fields))
}
