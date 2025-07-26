package tools

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// URLValidator validates URLs for security
type URLValidator struct {
	allowedSchemes []string
	blockedPorts   []int
	maxRedirects   int
	dnsTimeout     time.Duration
	tlsSkipVerify  bool
}

// NewURLValidator creates a new URL validator
func NewURLValidator() *URLValidator {
	return &URLValidator{
		allowedSchemes: []string{"http", "https"},
		blockedPorts: []int{
			22,    // SSH
			23,    // Telnet
			25,    // SMTP
			110,   // POP3
			135,   // RPC
			139,   // NetBIOS
			445,   // SMB
			1433,  // MSSQL
			3306,  // MySQL
			3389,  // RDP
			5432,  // PostgreSQL
			5900,  // VNC
			6379,  // Redis
			7001,  // Cassandra
			8020,  // Hadoop
			9200,  // Elasticsearch
			11211, // Memcached
			27017, // MongoDB
		},
		maxRedirects:  5,
		dnsTimeout:    5 * time.Second,
		tlsSkipVerify: false,
	}
}

// ValidateURL validates a URL for security issues
func (v *URLValidator) ValidateURL(ctx context.Context, rawURL string) error {
	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check scheme
	if !v.isAllowedScheme(u.Scheme) {
		return fmt.Errorf("scheme '%s' not allowed", u.Scheme)
	}

	// Extract host and port
	host, port, err := v.extractHostPort(u)
	if err != nil {
		return err
	}

	// Check port
	if err := v.validatePort(port); err != nil {
		return err
	}

	// Resolve hostname
	ips, err := v.resolveHost(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}

	// Check for internal IPs
	for _, ip := range ips {
		if err := v.validateIP(ip); err != nil {
			return err
		}
	}

	// Validate TLS if HTTPS
	if u.Scheme == "https" && !v.tlsSkipVerify {
		if err := v.validateTLS(ctx, host, port); err != nil {
			return fmt.Errorf("TLS validation failed: %w", err)
		}
	}

	return nil
}

// isAllowedScheme checks if the scheme is allowed
func (v *URLValidator) isAllowedScheme(scheme string) bool {
	scheme = strings.ToLower(scheme)
	for _, allowed := range v.allowedSchemes {
		if scheme == allowed {
			return true
		}
	}
	return false
}

// extractHostPort extracts host and port from URL
func (v *URLValidator) extractHostPort(u *url.URL) (string, string, error) {
	host := u.Hostname()
	port := u.Port()

	if host == "" {
		return "", "", fmt.Errorf("missing hostname")
	}

	// Default ports
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", "", fmt.Errorf("cannot determine port for scheme '%s'", u.Scheme)
		}
	}

	return host, port, nil
}

// validatePort checks if the port is allowed
func (v *URLValidator) validatePort(port string) error {
	portNum := 0
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}

	// Check blocked ports
	for _, blocked := range v.blockedPorts {
		if portNum == blocked {
			return fmt.Errorf("port %d is blocked for security reasons", portNum)
		}
	}

	// Check valid range
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port %d is out of valid range", portNum)
	}

	return nil
}

// resolveHost resolves hostname to IP addresses
func (v *URLValidator) resolveHost(ctx context.Context, host string) ([]net.IP, error) {
	// Create resolver with timeout
	resolver := &net.Resolver{}

	// Create timeout context
	resolveCtx, cancel := context.WithTimeout(ctx, v.dnsTimeout)
	defer cancel()

	// Check if already an IP
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	// Resolve hostname
	addrs, err := resolver.LookupIPAddr(resolveCtx, host)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no IP addresses found for host %s", host)
	}

	// Convert to IPs
	ips := make([]net.IP, len(addrs))
	for i, addr := range addrs {
		ips[i] = addr.IP
	}

	return ips, nil
}

// validateIP checks if IP is allowed (not internal)
func (v *URLValidator) validateIP(ip net.IP) error {
	// Block loopback
	if ip.IsLoopback() {
		return fmt.Errorf("loopback addresses are not allowed")
	}

	// Block private networks
	if ip.IsPrivate() {
		return fmt.Errorf("private network addresses are not allowed")
	}

	// Block link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local addresses are not allowed")
	}

	// Block multicast
	if ip.IsMulticast() {
		return fmt.Errorf("multicast addresses are not allowed")
	}

	// Block unspecified
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified addresses are not allowed")
	}

	// Additional checks for specific ranges

	// IPv4 checks
	if ip4 := ip.To4(); ip4 != nil {
		// 0.0.0.0/8
		if ip4[0] == 0 {
			return fmt.Errorf("0.0.0.0/8 addresses are not allowed")
		}

		// 100.64.0.0/10 - Carrier Grade NAT
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return fmt.Errorf("carrier grade NAT addresses are not allowed")
		}

		// 169.254.0.0/16 - Link Local
		if ip4[0] == 169 && ip4[1] == 254 {
			return fmt.Errorf("link-local addresses are not allowed")
		}

		// 192.0.0.0/24 - IETF Protocol Assignments
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0 {
			return fmt.Errorf("IETF protocol assignment addresses are not allowed")
		}

		// 192.0.2.0/24 - TEST-NET-1
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
			return fmt.Errorf("TEST-NET addresses are not allowed")
		}

		// 198.18.0.0/15 - Benchmark tests
		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return fmt.Errorf("benchmark test addresses are not allowed")
		}

		// 198.51.100.0/24 - TEST-NET-2
		if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
			return fmt.Errorf("TEST-NET addresses are not allowed")
		}

		// 203.0.113.0/24 - TEST-NET-3
		if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
			return fmt.Errorf("TEST-NET addresses are not allowed")
		}

		// 224.0.0.0/4 - Multicast
		if ip4[0] >= 224 && ip4[0] <= 239 {
			return fmt.Errorf("multicast addresses are not allowed")
		}

		// 240.0.0.0/4 - Reserved
		if ip4[0] >= 240 {
			return fmt.Errorf("reserved addresses are not allowed")
		}
	}

	// IPv6 checks
	if ip6 := ip.To16(); ip6 != nil && ip.To4() == nil {
		// fc00::/7 - Unique Local
		if ip6[0] >= 0xfc && ip6[0] <= 0xfd {
			return fmt.Errorf("unique local addresses are not allowed")
		}

		// fe80::/10 - Link Local
		if ip6[0] == 0xfe && (ip6[1]&0xc0) == 0x80 {
			return fmt.Errorf("link-local addresses are not allowed")
		}
	}

	return nil
}

// validateTLS validates TLS certificate
func (v *URLValidator) validateTLS(ctx context.Context, host, port string) error {
	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	// Connect
	address := net.JoinHostPort(host, port)
	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS connection failed: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			// Connection close error is not critical
			_ = err
		}
	}()

	// Get certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return fmt.Errorf("no certificates presented")
	}

	// Verify certificate
	cert := certs[0]

	// Check expiry
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}

	// Verify hostname
	if err := cert.VerifyHostname(host); err != nil {
		return fmt.Errorf("certificate hostname verification failed: %w", err)
	}

	return nil
}

// SetTLSSkipVerify sets whether to skip TLS verification (for testing only)
func (v *URLValidator) SetTLSSkipVerify(skip bool) {
	v.tlsSkipVerify = skip
}

// AddBlockedPort adds a port to the blocked list
func (v *URLValidator) AddBlockedPort(port int) {
	v.blockedPorts = append(v.blockedPorts, port)
}

// IsBlockedPort checks if a port is blocked
func (v *URLValidator) IsBlockedPort(port int) bool {
	for _, blocked := range v.blockedPorts {
		if port == blocked {
			return true
		}
	}
	return false
}
