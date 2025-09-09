package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// URLValidator provides secure URL validation to prevent SSRF attacks
type URLValidator struct {
	// AllowedDomains is an optional list of allowed domains
	// If empty, all non-private domains are allowed
	AllowedDomains []string

	// AllowLocalhost allows connections to localhost (default: false)
	AllowLocalhost bool

	// AllowPrivateNetworks allows connections to private networks (default: false)
	AllowPrivateNetworks bool
}

// NewURLValidator creates a new URL validator with secure defaults
func NewURLValidator() *URLValidator {
	return &URLValidator{
		AllowLocalhost:       false,
		AllowPrivateNetworks: false,
	}
}

// ValidateURL validates a URL for security issues
func (v *URLValidator) ValidateURL(rawURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme - only allow HTTP/HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: only http and https are allowed, got %s", parsedURL.Scheme)
	}

	// Extract hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Check against allowed domains if configured
	if len(v.AllowedDomains) > 0 {
		allowed := false
		for _, domain := range v.AllowedDomains {
			if strings.HasSuffix(hostname, domain) || hostname == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("domain %s is not in the allowed list", hostname)
		}
	}

	// Check if hostname is an IP address
	ip := net.ParseIP(hostname)
	if ip != nil {
		return v.validateIPAddress(ip)
	}

	// Resolve hostname to check the actual IP
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// If we can't resolve, it might be okay (external domain)
		// But we should be cautious
		return fmt.Errorf("unable to resolve hostname %s: %w", hostname, err)
	}

	// Check all resolved IPs
	for _, ip := range ips {
		if err := v.validateIPAddress(ip); err != nil {
			return fmt.Errorf("hostname %s resolves to blocked IP: %w", hostname, err)
		}
	}

	return nil
}

// validateIPAddress checks if an IP address is safe to connect to
func (v *URLValidator) validateIPAddress(ip net.IP) error {
	// Block IPv4 loopback (127.0.0.0/8)
	if !v.AllowLocalhost && ip.IsLoopback() {
		return fmt.Errorf("localhost/loopback addresses are not allowed")
	}

	// Block private networks if not allowed
	if !v.AllowPrivateNetworks {
		if ip.IsPrivate() {
			return fmt.Errorf("private network addresses are not allowed")
		}

		// Block link-local addresses (169.254.0.0/16)
		if ip.IsLinkLocalUnicast() {
			return fmt.Errorf("link-local addresses are not allowed")
		}

		// Block multicast addresses
		if ip.IsMulticast() {
			return fmt.Errorf("multicast addresses are not allowed")
		}

		// Explicitly block AWS metadata endpoint (169.254.169.254)
		if ip.String() == "169.254.169.254" {
			return fmt.Errorf("metadata endpoints are not allowed")
		}

		// Block common metadata endpoints
		metadataIPs := []string{
			"169.254.169.254",          // AWS
			"metadata.google.internal", // GCP
			"169.254.169.254",          // Azure
			"100.100.100.200",          // Alibaba Cloud
		}

		ipStr := ip.String()
		for _, metaIP := range metadataIPs {
			if ipStr == metaIP {
				return fmt.Errorf("cloud metadata endpoints are not allowed")
			}
		}
	}

	// Block unspecified addresses (0.0.0.0)
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified addresses are not allowed")
	}

	return nil
}

// SafeHTTPClient creates an HTTP client that validates URLs before making requests
// This is a helper function that can be used with existing HTTP clients
func (v *URLValidator) ValidateAndSanitizeURL(rawURL string) (string, error) {
	// First validate the URL
	if err := v.ValidateURL(rawURL); err != nil {
		return "", err
	}

	// Parse and reconstruct to ensure proper encoding
	parsedURL, _ := url.Parse(rawURL) // Already validated above

	// Ensure proper encoding of path and query parameters
	sanitized := &url.URL{
		Scheme:   parsedURL.Scheme,
		Host:     parsedURL.Host,
		Path:     parsedURL.Path,
		RawQuery: parsedURL.RawQuery,
		Fragment: "", // Remove fragments for security
	}

	return sanitized.String(), nil
}
