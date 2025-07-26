package api

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersConfig defines configuration for security headers
type SecurityHeadersConfig struct {
	Enabled              bool
	EnableHSTS           bool
	HSTSMaxAge           int
	EnableXFrameOptions  bool
	XFrameOptions        string
	EnableXContentType   bool
	EnableXSSProtection  bool
	EnableReferrerPolicy bool
	ReferrerPolicy       string
}

// DefaultSecurityHeadersConfig returns default security headers configuration
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled:              true,
		EnableHSTS:           true,
		HSTSMaxAge:           31536000, // 1 year
		EnableXFrameOptions:  true,
		XFrameOptions:        "DENY",
		EnableXContentType:   true,
		EnableXSSProtection:  true,
		EnableReferrerPolicy: true,
		ReferrerPolicy:       "strict-origin-when-cross-origin",
	}
}

// SecurityHeaders adds security headers to responses
// Note: In production, these headers are typically set by the reverse proxy (nginx, etc.)
// This middleware is useful for development or when not behind a reverse proxy
func SecurityHeaders(config SecurityHeadersConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// HSTS - only on HTTPS
		if config.EnableHSTS && c.Request.TLS != nil {
			c.Header("Strict-Transport-Security",
				"max-age="+string(rune(config.HSTSMaxAge))+"; includeSubDomains")
		}

		// X-Frame-Options
		if config.EnableXFrameOptions {
			c.Header("X-Frame-Options", config.XFrameOptions)
		}

		// X-Content-Type-Options
		if config.EnableXContentType {
			c.Header("X-Content-Type-Options", "nosniff")
		}

		// X-XSS-Protection
		if config.EnableXSSProtection {
			c.Header("X-XSS-Protection", "1; mode=block")
		}

		// Referrer-Policy
		if config.EnableReferrerPolicy {
			c.Header("Referrer-Policy", config.ReferrerPolicy)
		}

		// Additional security headers
		c.Header("X-DNS-Prefetch-Control", "off")
		c.Header("X-Download-Options", "noopen")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")

		c.Next()
	}
}

// APISecurityHeaders adds API-specific security headers
func APISecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent caching of sensitive API responses
		if c.Request.Method != "GET" || c.Request.URL.Path == "/api/v1/auth/me" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		// Add request ID to response if present
		if requestID := c.GetString("request_id"); requestID != "" {
			c.Header("X-Request-ID", requestID)
		}

		c.Next()
	}
}
