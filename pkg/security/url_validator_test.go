package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLValidator_ValidateURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		validator     *URLValidator
		expectedError bool
		errorContains string
	}{
		// Valid URLs
		{
			name:          "valid https URL",
			url:           "https://api.github.com/repos",
			validator:     NewURLValidator(),
			expectedError: false,
		},
		{
			name:          "valid http URL",
			url:           "http://example.com/api",
			validator:     NewURLValidator(),
			expectedError: false,
		},

		// Invalid schemes
		{
			name:          "file scheme blocked",
			url:           "file:///etc/passwd",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "invalid URL scheme",
		},
		{
			name:          "ftp scheme blocked",
			url:           "ftp://example.com/file",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "invalid URL scheme",
		},
		{
			name:          "gopher scheme blocked",
			url:           "gopher://example.com",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "invalid URL scheme",
		},

		// Localhost/loopback
		{
			name:          "localhost blocked by default",
			url:           "http://localhost/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "localhost",
		},
		{
			name:          "127.0.0.1 blocked by default",
			url:           "http://127.0.0.1/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "localhost/loopback",
		},
		{
			name: "localhost allowed when configured",
			url:  "http://localhost/api",
			validator: &URLValidator{
				AllowLocalhost: true,
			},
			expectedError: false,
		},

		// Private networks
		{
			name:          "10.x.x.x blocked",
			url:           "http://10.0.0.1/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "private network",
		},
		{
			name:          "192.168.x.x blocked",
			url:           "http://192.168.1.1/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "private network",
		},
		{
			name:          "172.16.x.x blocked",
			url:           "http://172.16.0.1/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "private network",
		},
		{
			name: "private networks allowed when configured",
			url:  "http://192.168.1.1/api",
			validator: &URLValidator{
				AllowPrivateNetworks: true,
			},
			expectedError: false,
		},

		// Metadata endpoints
		{
			name:          "AWS metadata endpoint blocked",
			url:           "http://169.254.169.254/latest/meta-data/",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "link-local",
		},
		{
			name:          "Link-local addresses blocked",
			url:           "http://169.254.1.1/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "link-local",
		},

		// Domain allowlist
		{
			name: "allowed domain passes",
			url:  "https://api.github.com/repos",
			validator: &URLValidator{
				AllowedDomains: []string{"github.com", "api.github.com"},
			},
			expectedError: false,
		},
		{
			name: "subdomain of allowed domain passes",
			url:  "https://api.github.com/repos",
			validator: &URLValidator{
				AllowedDomains: []string{"github.com"},
			},
			expectedError: false,
		},
		{
			name: "non-allowed domain blocked",
			url:  "https://evil.com/api",
			validator: &URLValidator{
				AllowedDomains: []string{"github.com", "gitlab.com"},
			},
			expectedError: true,
			errorContains: "not in the allowed list",
		},

		// Invalid URLs
		{
			name:          "empty URL",
			url:           "",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "invalid URL scheme",
		},
		{
			name:          "malformed URL",
			url:           "not-a-url",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "invalid URL scheme",
		},
		{
			name:          "URL without hostname",
			url:           "http:///path",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "valid hostname",
		},

		// Special addresses
		{
			name:          "unspecified address blocked",
			url:           "http://0.0.0.0/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "unspecified",
		},
		{
			name:          "multicast address blocked",
			url:           "http://224.0.0.1/api",
			validator:     NewURLValidator(),
			expectedError: true,
			errorContains: "multicast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.ValidateURL(tt.url)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestURLValidator_ValidateAndSanitizeURL(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name          string
		input         string
		expected      string
		expectedError bool
	}{
		{
			name:          "valid URL unchanged",
			input:         "https://api.github.com/repos",
			expected:      "https://api.github.com/repos",
			expectedError: false,
		},
		{
			name:          "fragment removed",
			input:         "https://api.github.com/repos#section",
			expected:      "https://api.github.com/repos",
			expectedError: false,
		},
		{
			name:          "invalid URL returns error",
			input:         "http://localhost/api",
			expected:      "",
			expectedError: true,
		},
		{
			name:          "preserves query parameters",
			input:         "https://api.github.com/repos?page=1&limit=10",
			expected:      "https://api.github.com/repos?page=1&limit=10",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ValidateAndSanitizeURL(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
