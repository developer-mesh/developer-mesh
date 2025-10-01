package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEdgeAuthenticator_NoAPIKey(t *testing.T) {
	// When no API key is configured, all requests should pass (development mode)
	auth := NewEdgeAuthenticator("")

	req, _ := http.NewRequest("GET", "/test", nil)

	result := auth.AuthenticateRequest(req)
	assert.True(t, result, "Should allow request when no API key is configured")
}

func TestEdgeAuthenticator_BearerToken(t *testing.T) {
	// Setup authenticator with API key
	expectedKey := "test-api-key-12345"
	auth := NewEdgeAuthenticator(expectedKey)

	tests := []struct {
		name        string
		authHeader  string
		shouldPass  bool
		description string
	}{
		{
			name:        "Valid Bearer Token",
			authHeader:  "Bearer test-api-key-12345",
			shouldPass:  true,
			description: "Should authenticate with valid Bearer token",
		},
		{
			name:        "Valid Bearer Token No Space",
			authHeader:  "test-api-key-12345",
			shouldPass:  true,
			description: "Should authenticate without Bearer prefix",
		},
		{
			name:        "Invalid Bearer Token",
			authHeader:  "Bearer wrong-key",
			shouldPass:  false,
			description: "Should reject invalid Bearer token",
		},
		{
			name:        "Empty Bearer",
			authHeader:  "Bearer ",
			shouldPass:  false,
			description: "Should reject empty Bearer token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)

			result := auth.AuthenticateRequest(req)
			assert.Equal(t, tt.shouldPass, result, tt.description)
		})
	}
}

func TestEdgeAuthenticator_XAPIKey(t *testing.T) {
	// Setup authenticator with API key
	expectedKey := "test-api-key-67890"
	auth := NewEdgeAuthenticator(expectedKey)

	tests := []struct {
		name       string
		apiKey     string
		shouldPass bool
	}{
		{
			name:       "Valid X-API-Key",
			apiKey:     "test-api-key-67890",
			shouldPass: true,
		},
		{
			name:       "Invalid X-API-Key",
			apiKey:     "wrong-key",
			shouldPass: false,
		},
		{
			name:       "Empty X-API-Key",
			apiKey:     "",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			result := auth.AuthenticateRequest(req)
			assert.Equal(t, tt.shouldPass, result)
		})
	}
}

func TestEdgeAuthenticator_PreferAuthorizationHeader(t *testing.T) {
	// When both headers are present, Authorization should be preferred
	auth := NewEdgeAuthenticator("correct-key")

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer correct-key")
	req.Header.Set("X-API-Key", "wrong-key")

	result := auth.AuthenticateRequest(req)
	assert.True(t, result, "Should use Authorization header when both are present")
}

func TestEdgeAuthenticator_NoCredentials(t *testing.T) {
	// Setup authenticator with API key
	auth := NewEdgeAuthenticator("required-key")

	req, _ := http.NewRequest("GET", "/test", nil)
	// No headers set

	result := auth.AuthenticateRequest(req)
	assert.False(t, result, "Should reject request with no credentials")
}
