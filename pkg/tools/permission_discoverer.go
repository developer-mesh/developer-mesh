package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// PermissionDiscoverer discovers what permissions/scopes a token has for ANY API
// This is TOOL-AGNOSTIC and works with any OpenAPI-based API
type PermissionDiscoverer struct {
	logger     observability.Logger
	httpClient *http.Client
}

// DiscoveredPermissions represents the permissions/scopes available to a token
type DiscoveredPermissions struct {
	// Scopes are the actual permissions the token has (if discoverable)
	Scopes []string `json:"scopes,omitempty"`

	// AcceptedScopes are what scopes would be accepted for the current endpoint
	AcceptedScopes []string `json:"accepted_scopes,omitempty"`

	// RawHeaders contains any permission-related headers from the API
	// Different APIs use different headers (X-OAuth-Scopes, X-Accepted-OAuth-Scopes, etc)
	RawHeaders map[string]string `json:"raw_headers,omitempty"`

	// UserInfo contains user/token metadata if available
	UserInfo map[string]interface{} `json:"user_info,omitempty"`

	// Limitations detected from API responses
	Limitations []string `json:"limitations,omitempty"`
}

// NewPermissionDiscoverer creates a new permission discoverer
func NewPermissionDiscoverer(logger observability.Logger) *PermissionDiscoverer {
	return &PermissionDiscoverer{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DiscoverPermissions attempts to discover what permissions a token has
// This is GENERIC and works with ANY API by using multiple strategies
func (d *PermissionDiscoverer) DiscoverPermissions(ctx context.Context, baseURL string, token string, authType string) (*DiscoveredPermissions, error) {
	discovered := &DiscoveredPermissions{
		RawHeaders: make(map[string]string),
		Scopes:     []string{},
	}

	// Strategy 1: Try common user/auth endpoints that might reveal permissions
	strategies := []struct {
		path        string
		description string
	}{
		{"/user", "user profile endpoint"},
		{"/me", "current user endpoint"},
		{"/auth/verify", "auth verification endpoint"},
		{"/api/user", "API user endpoint"},
		{"/api/v1/user", "versioned user endpoint"},
		{"/api/auth/verify", "API auth verify"},
		{"/oauth/tokeninfo", "OAuth token info"},
		{"/token/introspect", "OAuth introspection"},
	}

	for _, strategy := range strategies {
		if perms := d.tryEndpoint(ctx, baseURL+strategy.path, token, authType); perms != nil {
			d.mergePermissions(discovered, perms)
			d.logger.Debug("Discovered permissions from endpoint", map[string]interface{}{
				"endpoint": strategy.path,
				"scopes":   perms.Scopes,
			})
		}
	}

	// Strategy 2: Make a HEAD request to extract permission headers
	// Many APIs return permission info in headers
	if perms := d.extractFromHeaders(ctx, baseURL, token, authType); perms != nil {
		d.mergePermissions(discovered, perms)
	}

	// Strategy 3: Try to parse JWT token locally (if it's a JWT)
	if strings.Count(token, ".") == 2 {
		if perms := d.parseJWTClaims(token); perms != nil {
			d.mergePermissions(discovered, perms)
		}
	}

	return discovered, nil
}

// tryEndpoint attempts to get permission info from a specific endpoint
func (d *PermissionDiscoverer) tryEndpoint(ctx context.Context, url string, token string, authType string) *DiscoveredPermissions {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}

	// Apply authentication based on type
	d.applyAuth(req, token, authType)

	resp, err := d.httpClient.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	perms := &DiscoveredPermissions{
		RawHeaders: make(map[string]string),
	}

	// Extract permission-related headers (tool-agnostic)
	permissionHeaders := []string{
		"X-OAuth-Scopes",
		"X-Accepted-OAuth-Scopes",
		"X-Scopes",
		"X-Permissions",
		"X-User-Permissions",
		"X-Token-Scopes",
		"X-API-Scopes",
		"X-Access-Level",
		"X-Rate-Limit-Scope",
		"Authorization-Scopes",
	}

	for _, header := range permissionHeaders {
		if value := resp.Header.Get(header); value != "" {
			perms.RawHeaders[header] = value
			// Parse comma-separated scopes
			if strings.Contains(strings.ToLower(header), "scope") {
				scopes := strings.Split(value, ",")
				for _, scope := range scopes {
					scope = strings.TrimSpace(scope)
					if scope != "" {
						perms.Scopes = append(perms.Scopes, scope)
					}
				}
			}
		}
	}

	// Try to parse response body for user/permission info
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		// Look for common permission/scope fields in response
		if scopes := d.extractScopesFromBody(body); len(scopes) > 0 {
			perms.Scopes = append(perms.Scopes, scopes...)
		}
		perms.UserInfo = body
	}

	return perms
}

// extractScopesFromBody looks for scope/permission info in response body
func (d *PermissionDiscoverer) extractScopesFromBody(body map[string]interface{}) []string {
	var scopes []string

	// Common field names for scopes/permissions across different APIs
	scopeFields := []string{
		"scopes", "scope", "permissions", "authorities",
		"roles", "grants", "privileges", "access", "capabilities",
	}

	for _, field := range scopeFields {
		if value, ok := body[field]; ok {
			switch v := value.(type) {
			case []interface{}:
				for _, item := range v {
					if str, ok := item.(string); ok {
						scopes = append(scopes, str)
					}
				}
			case []string:
				scopes = append(scopes, v...)
			case string:
				// Could be comma-separated
				for _, s := range strings.Split(v, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						scopes = append(scopes, s)
					}
				}
			}
		}
	}

	return scopes
}

// extractFromHeaders makes a HEAD request to get permission headers
func (d *PermissionDiscoverer) extractFromHeaders(ctx context.Context, baseURL string, token string, authType string) *DiscoveredPermissions {
	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL, nil)
	if err != nil {
		return nil
	}

	d.applyAuth(req, token, authType)

	resp, err := d.httpClient.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	perms := &DiscoveredPermissions{
		RawHeaders: make(map[string]string),
		Scopes:     []string{},
	}

	// Extract all potentially relevant headers
	for key, values := range resp.Header {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "scope") ||
			strings.Contains(lowerKey, "permission") ||
			strings.Contains(lowerKey, "role") ||
			strings.Contains(lowerKey, "grant") {
			value := strings.Join(values, ", ")
			perms.RawHeaders[key] = value

			// Try to parse as scopes
			if strings.Contains(lowerKey, "scope") {
				for _, scope := range strings.Split(value, ",") {
					scope = strings.TrimSpace(scope)
					if scope != "" {
						perms.Scopes = append(perms.Scopes, scope)
					}
				}
			}
		}
	}

	return perms
}

// parseJWTClaims attempts to extract scopes from a JWT token
func (d *PermissionDiscoverer) parseJWTClaims(token string) *DiscoveredPermissions {
	// Split the JWT
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}

	// Decode the payload (we don't verify signature as we're just extracting claims)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}

	perms := &DiscoveredPermissions{
		Scopes: []string{},
	}

	// Look for scope/permission claims (following OAuth 2.0 and OpenID Connect standards)
	scopeFields := []string{"scope", "scopes", "scp", "permissions", "roles", "authorities"}
	for _, field := range scopeFields {
		if value, ok := claims[field]; ok {
			switch v := value.(type) {
			case string:
				// Space or comma separated
				for _, s := range strings.Fields(strings.ReplaceAll(v, ",", " ")) {
					if s != "" {
						perms.Scopes = append(perms.Scopes, s)
					}
				}
			case []interface{}:
				for _, item := range v {
					if str, ok := item.(string); ok {
						perms.Scopes = append(perms.Scopes, str)
					}
				}
			}
		}
	}

	// Store relevant user info
	if sub, ok := claims["sub"].(string); ok {
		if perms.UserInfo == nil {
			perms.UserInfo = make(map[string]interface{})
		}
		perms.UserInfo["subject"] = sub
	}

	return perms
}

// applyAuth applies authentication to a request
func (d *PermissionDiscoverer) applyAuth(req *http.Request, token string, authType string) {
	switch strings.ToLower(authType) {
	case "bearer", "oauth", "oauth2":
		req.Header.Set("Authorization", "Bearer "+token)
	case "token":
		req.Header.Set("Authorization", "token "+token)
	case "apikey", "api_key":
		req.Header.Set("X-API-Key", token)
	default:
		// Try bearer as default
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// mergePermissions merges discovered permissions
func (d *PermissionDiscoverer) mergePermissions(target, source *DiscoveredPermissions) {
	if source == nil {
		return
	}

	// Merge scopes (deduplicate)
	scopeMap := make(map[string]bool)
	for _, s := range target.Scopes {
		scopeMap[s] = true
	}
	for _, s := range source.Scopes {
		if !scopeMap[s] {
			target.Scopes = append(target.Scopes, s)
			scopeMap[s] = true
		}
	}

	// Merge headers
	for k, v := range source.RawHeaders {
		target.RawHeaders[k] = v
	}

	// Merge user info
	if source.UserInfo != nil && target.UserInfo == nil {
		target.UserInfo = source.UserInfo
	}
}

// FilterOperationsByPermissions filters OpenAPI operations based on discovered permissions
// This is the CORE GENERIC FUNCTION that works with ANY OpenAPI spec
func (d *PermissionDiscoverer) FilterOperationsByPermissions(
	spec *openapi3.T,
	permissions *DiscoveredPermissions,
) map[string]bool {
	// Map of operationID -> allowed
	allowedOps := make(map[string]bool)

	if spec.Paths == nil {
		return allowedOps
	}

	// If we have no discovered permissions, we need to be conservative
	hasPermissions := len(permissions.Scopes) > 0

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Default to allowed if no security is defined
			allowed := true

			// Check operation-level security first
			if operation.Security != nil && len(*operation.Security) > 0 {
				allowed = d.checkSecurityRequirements(*operation.Security, permissions)
			} else if len(spec.Security) > 0 {
				// Fall back to global security
				allowed = d.checkSecurityRequirements(spec.Security, permissions)
			}

			// If we have discovered permissions but this operation requires none, allow it
			if !hasPermissions && !d.hasSecurityRequirements(operation, spec) {
				allowed = true
			}

			// Store result
			if operation.OperationID != "" {
				allowedOps[operation.OperationID] = allowed

				if allowed {
					d.logger.Debug("Operation allowed", map[string]interface{}{
						"operation_id": operation.OperationID,
						"path":         path,
						"method":       method,
					})
				}
			}
		}
	}

	d.logger.Info("Permission filtering complete", map[string]interface{}{
		"total_operations": len(allowedOps),
		"allowed_count":    d.countAllowed(allowedOps),
	})

	return allowedOps
}

// countAllowed counts how many operations are allowed
func (d *PermissionDiscoverer) countAllowed(allowedOps map[string]bool) int {
	count := 0
	for _, allowed := range allowedOps {
		if allowed {
			count++
		}
	}
	return count
}

// checkSecurityRequirements checks if permissions satisfy security requirements
func (d *PermissionDiscoverer) checkSecurityRequirements(
	requirements openapi3.SecurityRequirements,
	permissions *DiscoveredPermissions,
) bool {
	// If no requirements, allow
	if len(requirements) == 0 {
		return true
	}

	// OR logic: any requirement can be satisfied
	for _, requirement := range requirements {
		if d.checkSingleRequirement(requirement, permissions) {
			return true
		}
	}

	return false
}

// checkSingleRequirement checks a single security requirement
func (d *PermissionDiscoverer) checkSingleRequirement(
	requirement openapi3.SecurityRequirement,
	permissions *DiscoveredPermissions,
) bool {
	// AND logic: all schemes in the requirement must be satisfied
	for _, requiredScopes := range requirement {
		// Note: schemeName could be used for scheme-specific logic in the future
		if !d.hasRequiredScopes(requiredScopes, permissions.Scopes) {
			return false
		}
	}
	return true
}

// hasRequiredScopes checks if user has required scopes
func (d *PermissionDiscoverer) hasRequiredScopes(required []string, available []string) bool {
	// If no scopes required, allow
	if len(required) == 0 {
		return true
	}

	// Build a map of available scopes for quick lookup
	availableMap := make(map[string]bool)
	for _, scope := range available {
		availableMap[scope] = true
		// Also check for wildcard scopes (e.g., "admin" might cover "admin:org")
		if strings.Contains(scope, ":") {
			parts := strings.Split(scope, ":")
			availableMap[parts[0]] = true
		}
	}

	// Check each required scope
	for _, req := range required {
		found := false

		// Direct match
		if availableMap[req] {
			found = true
		}

		// Check for parent scope (e.g., "admin" covers "admin:org")
		if !found && strings.Contains(req, ":") {
			parts := strings.Split(req, ":")
			if availableMap[parts[0]] {
				found = true
			}
		}

		// Check for wildcard in available (e.g., "repo:*" covers "repo:status")
		if !found {
			for avail := range availableMap {
				if d.matchesWildcard(avail, req) {
					found = true
					break
				}
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// matchesWildcard checks if a scope with wildcard matches a required scope
func (d *PermissionDiscoverer) matchesWildcard(pattern, scope string) bool {
	// Simple wildcard matching (can be enhanced)
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(scope, prefix)
	}
	return false
}

// hasSecurityRequirements checks if an operation has security requirements
func (d *PermissionDiscoverer) hasSecurityRequirements(operation *openapi3.Operation, spec *openapi3.T) bool {
	if operation.Security != nil && len(*operation.Security) > 0 {
		return true
	}
	if len(spec.Security) > 0 {
		return true
	}
	return false
}
