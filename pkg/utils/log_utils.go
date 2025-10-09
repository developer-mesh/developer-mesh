package utils

import (
	"fmt"
	"strings"
)

// SensitiveKeys defines keys that should be redacted in logs
var SensitiveKeys = []string{
	"password", "passwd", "pwd",
	"secret", "api_key", "apikey", "api-key",
	"token", "access_token", "refresh_token", "bearer",
	"authorization", "auth", "credentials", "creds",
	"private_key", "private-key", "privatekey",
	"client_secret", "client-secret", "clientsecret",
	"encrypted_token", "github_token", "personal_access_token",
	"__passthrough_auth", "pat",
}

// RedactSensitiveData creates a safe copy of data with sensitive values redacted
func RedactSensitiveData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := make(map[string]interface{})
	for key, value := range data {
		if isSensitiveKey(key) {
			result[key] = "[REDACTED]"
		} else {
			// Recursively handle nested structures
			switch v := value.(type) {
			case map[string]interface{}:
				result[key] = RedactSensitiveData(v)
			case []interface{}:
				result[key] = redactSlice(v)
			default:
				result[key] = value
			}
		}
	}
	return result
}

func redactSlice(slice []interface{}) []interface{} {
	result := make([]interface{}, len(slice))
	for i, item := range slice {
		switch v := item.(type) {
		case map[string]interface{}:
			result[i] = RedactSensitiveData(v)
		case []interface{}:
			result[i] = redactSlice(v)
		default:
			result[i] = v
		}
	}
	return result
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitive := range SensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// RedactString partially redacts a string value for logging
func RedactString(value string) string {
	if value == "" {
		return value
	}
	if len(value) > 8 {
		return fmt.Sprintf("%s...[REDACTED]", value[:3])
	}
	return "[REDACTED]"
}

// SanitizeLogValue sanitizes a single value based on its key for safe logging
func SanitizeLogValue(key string, value interface{}) interface{} {
	if isSensitiveKey(key) {
		return "[REDACTED]"
	}

	// Recursively handle nested structures
	switch v := value.(type) {
	case map[string]interface{}:
		return RedactSensitiveData(v)
	case []interface{}:
		return redactSlice(v)
	default:
		return value
	}
}
