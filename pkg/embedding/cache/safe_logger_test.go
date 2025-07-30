package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func TestSensitiveDataRedactor_Redact(t *testing.T) {
	redactor := NewSensitiveDataRedactor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API key in message",
			input:    "Failed to connect with api_key: sk-1234567890abcdef",
			expected: "Failed to connect with api_key: [REDACTED]",
		},
		{
			name:     "Password in message",
			input:    "Login failed for user admin with password: supersecret123",
			expected: "Login failed for user admin with password: [REDACTED]",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "Credit card number",
			input:    "Payment failed for card 4532-1234-5678-9012",
			expected: "Payment failed for card [REDACTED]",
		},
		{
			name:     "SSN",
			input:    "User SSN is 123-45-6789",
			expected: "User SSN is [REDACTED]",
		},
		{
			name:     "Multiple sensitive values",
			input:    "api_key=secret123 and password=pass456",
			expected: "api_key=[REDACTED] and password=[REDACTED]",
		},
		{
			name:     "No sensitive data",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSensitiveDataRedactor_RedactMap(t *testing.T) {
	redactor := NewSensitiveDataRedactor()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "Sensitive keys",
			input: map[string]interface{}{
				"api_key":  "sk-1234567890",
				"user":     "admin",
				"password": "secret123",
			},
			expected: map[string]interface{}{
				"api_key":  "[REDACTED]",
				"user":     "admin",
				"password": "[REDACTED]",
			},
		},
		{
			name: "Sensitive values in non-sensitive keys",
			input: map[string]interface{}{
				"error":   "Failed with api_key: sk-123",
				"message": "Normal message",
			},
			expected: map[string]interface{}{
				"error":   "Failed with api_key: [REDACTED]",
				"message": "Normal message",
			},
		},
		{
			name: "Mixed case sensitive keys",
			input: map[string]interface{}{
				"API_KEY":       "secret",
				"AccessToken":   "token123",
				"refresh-token": "refresh123",
			},
			expected: map[string]interface{}{
				"API_KEY":       "[REDACTED]",
				"AccessToken":   "[REDACTED]",
				"refresh-token": "[REDACTED]",
			},
		},
		{
			name: "Non-string values",
			input: map[string]interface{}{
				"count":   123,
				"enabled": true,
				"api_key": "secret",
			},
			expected: map[string]interface{}{
				"count":   123,
				"enabled": true,
				"api_key": "[REDACTED]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.RedactMap(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeLogger(t *testing.T) {
	// Create a real logger for testing
	logger := observability.NewLogger("test")
	safeLogger := NewSafeLogger(logger)

	t.Run("Debug with sensitive data", func(t *testing.T) {
		// This test verifies that sensitive data is redacted
		// In a real test environment, we would capture log output
		// For now, we just ensure the method doesn't panic
		safeLogger.Debug(
			"Connection failed with api_key: sk-1234567890",
			map[string]interface{}{
				"password": "secret123",
				"user":     "admin",
			},
		)
	})

	t.Run("Error with credit card", func(t *testing.T) {
		safeLogger.Error(
			"Payment failed for card 4532-1234-5678-9012",
			map[string]interface{}{
				"error": "Invalid card",
			},
		)
	})

	t.Run("Info with no sensitive data", func(t *testing.T) {
		safeLogger.Info(
			"Cache hit for query",
			map[string]interface{}{
				"query": "SELECT * FROM users",
				"hits":  10,
			},
		)
	})

	t.Run("Warn with authorization header", func(t *testing.T) {
		safeLogger.Warn(
			"Request failed with Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			map[string]interface{}{
				"status": 401,
			},
		)
	})

	// Skip Fatal test as it exits the program
	// In production, Fatal should be used sparingly and only for unrecoverable errors
}
