package auth

import (
	"net/http"
	"strings"
)

// Authenticator handles authentication
type Authenticator interface {
	AuthenticateRequest(r *http.Request) bool
}

// EdgeAuthenticator implements simple API key authentication for Edge MCP
type EdgeAuthenticator struct {
	apiKey string
}

// NewEdgeAuthenticator creates a new Edge authenticator
func NewEdgeAuthenticator(apiKey string) Authenticator {
	return &EdgeAuthenticator{
		apiKey: apiKey,
	}
}

// AuthenticateRequest authenticates an HTTP request
func (a *EdgeAuthenticator) AuthenticateRequest(r *http.Request) bool {
	// If no API key is configured, allow all requests (for local development)
	if a.apiKey == "" {
		return true
	}

	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Check X-API-Key header
		authHeader = r.Header.Get("X-API-Key")
	}

	if authHeader == "" {
		return false
	}

	// Handle Bearer token format
	authHeader = strings.TrimPrefix(authHeader, "Bearer ")

	return authHeader == a.apiKey
}
