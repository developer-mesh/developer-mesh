package artifactory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// ArtifactoryPermissionDiscoverer discovers permissions for JFrog Artifactory API keys
type ArtifactoryPermissionDiscoverer struct {
	logger     observability.Logger
	httpClient *http.Client
	baseURL    string
}

// ArtifactoryPermissions represents discovered Artifactory permissions
type ArtifactoryPermissions struct {
	UserInfo        map[string]interface{}
	Repositories    map[string][]string // repo -> permissions (read/write/admin/deploy/delete/annotate)
	EnabledFeatures map[string]bool     // feature -> enabled (e.g., xray, pipelines)
	IsAdmin         bool
	Scopes          []string // For compatibility with base
	RawHeaders      map[string]string
}

// NewArtifactoryPermissionDiscoverer creates a new Artifactory permission discoverer
func NewArtifactoryPermissionDiscoverer(logger observability.Logger, baseURL string) *ArtifactoryPermissionDiscoverer {
	if baseURL == "" {
		baseURL = "https://mycompany.jfrog.io/artifactory"
	}
	return &ArtifactoryPermissionDiscoverer{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
	}
}

// DiscoverPermissions discovers what permissions a JFrog API key has
func (d *ArtifactoryPermissionDiscoverer) DiscoverPermissions(ctx context.Context, apiKey string) (*ArtifactoryPermissions, error) {
	perms := &ArtifactoryPermissions{
		UserInfo:        make(map[string]interface{}),
		Repositories:    make(map[string][]string),
		EnabledFeatures: make(map[string]bool),
		Scopes:          []string{},
		RawHeaders:      make(map[string]string),
	}

	// 1. Get user identity (2-step process as per requirements)
	if err := d.getUserInfo(ctx, apiKey, perms); err != nil {
		d.logger.Debug("Failed to get user info", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 2. Probe repository access
	d.probeRepositoryAccess(ctx, apiKey, perms)

	// 3. Check admin capabilities
	d.checkAdminAccess(ctx, apiKey, perms)

	// 4. Detect available features
	d.detectFeatures(ctx, apiKey, perms)

	return perms, nil
}

// getUserInfo retrieves user information (2-step process: get API key info, then user details)
func (d *ArtifactoryPermissionDiscoverer) getUserInfo(ctx context.Context, apiKey string, perms *ArtifactoryPermissions) error {
	// Step 1: Get API key info to extract username
	apiKeyURL := fmt.Sprintf("%s/api/security/apiKey", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiKeyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create API key request: %w", err)
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get API key info: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API key info request failed with status %d", resp.StatusCode)
	}

	// Parse API key response
	var apiKeyResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiKeyResp); err != nil {
		return fmt.Errorf("failed to decode API key response: %w", err)
	}

	// Extract username from response (might be in different fields)
	username := ""
	if user, ok := apiKeyResp["username"].(string); ok {
		username = user
	} else if user, ok := apiKeyResp["user"].(string); ok {
		username = user
	} else if principal, ok := apiKeyResp["principal"].(string); ok {
		username = principal
	}

	if username == "" {
		return fmt.Errorf("could not extract username from API key response")
	}

	// Step 2: Get detailed user information
	userURL := fmt.Sprintf("%s/api/security/users/%s", d.baseURL, username)

	req, err = http.NewRequestWithContext(ctx, "GET", userURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create user request: %w", err)
	}

	d.applyAuthentication(req, apiKey)

	resp, err = d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get user details: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		// User might not have permission to view their own details
		perms.UserInfo["username"] = username
		return nil
	}

	// Parse user details
	var userResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return fmt.Errorf("failed to decode user response: %w", err)
	}

	// Store user info
	perms.UserInfo = userResp

	// Check if user is admin
	if admin, ok := userResp["admin"].(bool); ok {
		perms.IsAdmin = admin
	}

	// Extract groups if present
	if groups, ok := userResp["groups"].([]interface{}); ok {
		for _, group := range groups {
			if groupStr, ok := group.(string); ok {
				perms.Scopes = append(perms.Scopes, fmt.Sprintf("group:%s", groupStr))
			}
		}
	}

	return nil
}

// probeRepositoryAccess checks which repositories the user can access
func (d *ArtifactoryPermissionDiscoverer) probeRepositoryAccess(ctx context.Context, apiKey string, perms *ArtifactoryPermissions) {
	// List repositories
	reposURL := fmt.Sprintf("%s/api/repositories", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reposURL, nil)
	if err != nil {
		d.logger.Debug("Failed to create repositories request", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Debug("Failed to list repositories", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return
	}

	// Parse repository list
	var repos []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return
	}

	// For each repository, probe permissions
	for _, repo := range repos {
		if key, ok := repo["key"].(string); ok {
			// Try to get detailed repository permissions
			d.probeRepositoryPermissions(ctx, apiKey, key, perms)
		}
	}
}

// probeRepositoryPermissions checks specific permissions for a repository
func (d *ArtifactoryPermissionDiscoverer) probeRepositoryPermissions(ctx context.Context, apiKey string, repoKey string, perms *ArtifactoryPermissions) {
	permissions := []string{}

	// Check read permission by trying to list files
	if d.probeEndpoint(ctx, fmt.Sprintf("/api/storage/%s", repoKey), "GET", apiKey) {
		permissions = append(permissions, "read")
		perms.Scopes = append(perms.Scopes, fmt.Sprintf("repo:%s:read", repoKey))
	}

	// Check write/deploy permission (we can't actually write, but we can check the endpoint)
	if d.probeEndpoint(ctx, fmt.Sprintf("/%s/test-write-probe", repoKey), "PUT", apiKey) {
		permissions = append(permissions, "write", "deploy")
		perms.Scopes = append(perms.Scopes, fmt.Sprintf("repo:%s:write", repoKey))
	}

	// Check delete permission
	if d.probeEndpoint(ctx, fmt.Sprintf("/%s/test-delete-probe", repoKey), "DELETE", apiKey) {
		permissions = append(permissions, "delete")
		perms.Scopes = append(perms.Scopes, fmt.Sprintf("repo:%s:delete", repoKey))
	}

	// Check admin permission (ability to modify repo config)
	if d.probeEndpoint(ctx, fmt.Sprintf("/api/repositories/%s", repoKey), "POST", apiKey) {
		permissions = append(permissions, "admin")
		perms.Scopes = append(perms.Scopes, fmt.Sprintf("repo:%s:admin", repoKey))
	}

	if len(permissions) > 0 {
		perms.Repositories[repoKey] = permissions
		d.logger.Debug("Repository permissions discovered", map[string]interface{}{
			"repository":  repoKey,
			"permissions": permissions,
		})
	}
}

// checkAdminAccess checks if the user has admin access
func (d *ArtifactoryPermissionDiscoverer) checkAdminAccess(ctx context.Context, apiKey string, perms *ArtifactoryPermissions) {
	// Try to access system configuration (admin-only endpoint)
	if d.probeEndpoint(ctx, "/api/system/configuration", "GET", apiKey) {
		perms.IsAdmin = true
		perms.Scopes = append(perms.Scopes, "admin")
		d.logger.Info("Admin access detected", nil)
	}
}

// detectFeatures checks which JFrog features are available
func (d *ArtifactoryPermissionDiscoverer) detectFeatures(ctx context.Context, apiKey string, perms *ArtifactoryPermissions) {
	features := map[string]string{
		"xray":            "/xray/api/v1/system/version",
		"pipelines":       "/pipelines/api/v1/system/info",
		"mission-control": "/mc/api/v1/system/info",
		"distribution":    "/distribution/api/v1/system/info",
	}

	for feature, endpoint := range features {
		// Probe feature endpoint
		fullURL := strings.Replace(d.baseURL, "/artifactory", "", 1) + endpoint
		if d.probeFullURL(ctx, fullURL, "GET", apiKey) {
			perms.EnabledFeatures[feature] = true
			perms.Scopes = append(perms.Scopes, fmt.Sprintf("feature:%s", feature))
			d.logger.Debug("Feature detected", map[string]interface{}{
				"feature": feature,
			})
		} else {
			perms.EnabledFeatures[feature] = false
		}
	}
}

// probeEndpoint checks if an endpoint is accessible (relative to baseURL)
func (d *ArtifactoryPermissionDiscoverer) probeEndpoint(ctx context.Context, endpoint, method, apiKey string) bool {
	url := d.baseURL + endpoint
	return d.probeFullURL(ctx, url, method, apiKey)
}

// probeFullURL checks if a full URL is accessible
func (d *ArtifactoryPermissionDiscoverer) probeFullURL(ctx context.Context, url, method, apiKey string) bool {
	var body io.Reader
	if method == "PUT" || method == "POST" {
		// Minimal valid JSON body for probe requests
		body = strings.NewReader("{}")
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return false
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 200-299 means we have access
	// 400-402, 404-499 might mean bad request but we have access to try
	// 403 specifically means forbidden (no access)
	return (resp.StatusCode >= 200 && resp.StatusCode < 300) ||
		(resp.StatusCode >= 400 && resp.StatusCode < 403) ||
		(resp.StatusCode == 404) || // 404 might just mean the probe path doesn't exist, not lack of permission
		(resp.StatusCode > 404 && resp.StatusCode < 500)
}

// applyAuthentication applies JFrog-specific authentication headers
func (d *ArtifactoryPermissionDiscoverer) applyAuthentication(req *http.Request, apiKey string) {
	// Use X-JFrog-Art-Api header for API key authentication (JFrog standard)
	if apiKey != "" {
		req.Header.Set("X-JFrog-Art-Api", apiKey)
		// Also set as Bearer token for compatibility
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// FilterOperationsByPermissions filters operations based on discovered permissions
func (d *ArtifactoryPermissionDiscoverer) FilterOperationsByPermissions(
	operations map[string]providers.OperationMapping,
	permissions *ArtifactoryPermissions,
) map[string]providers.OperationMapping {
	filtered := make(map[string]providers.OperationMapping)

	for opID, op := range operations {
		allowed := false

		// Parse operation ID to determine required permissions
		parts := strings.Split(opID, "/")
		if len(parts) == 0 {
			continue
		}

		resource := parts[0]
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}

		switch resource {
		case "system":
			// System operations - check admin or allow read operations
			if permissions.IsAdmin {
				allowed = true
			} else if action == "info" || action == "version" || action == "ping" || action == "storage" {
				// Allow read-only system operations
				allowed = true
			}

		case "repos", "repositories":
			// Repository management - check admin for create/update/delete
			switch action {
			case "list", "get":
				// Allow listing/getting if user has access to any repo
				allowed = len(permissions.Repositories) > 0
			case "create", "update", "delete":
				allowed = permissions.IsAdmin
			default:
				allowed = true // Default allow for other repo operations
			}

		case "artifacts":
			// Artifact operations - check repository permissions
			if action == "upload" || action == "delete" || strings.Contains(opID, "move") || strings.Contains(opID, "copy") {
				// Need write permission on at least one repo
				allowed = d.hasWritePermission(permissions)
			} else {
				// Read operations allowed if user has access to any repo
				allowed = len(permissions.Repositories) > 0
			}

		case "search":
			// Search operations - allow if user has any repository access
			allowed = len(permissions.Repositories) > 0

		case "builds":
			// Build operations - check for write permissions for upload/promote/delete
			if action == "upload" || action == "promote" || action == "delete" {
				allowed = d.hasWritePermission(permissions)
			} else {
				// Read operations allowed with any repo access
				allowed = len(permissions.Repositories) > 0
			}

		case "users", "groups", "permissions", "tokens":
			// Security operations - mostly admin only
			switch action {
			case "list", "get":
				// May be allowed for regular users to see their own info
				allowed = true
			default:
				// Create/update/delete requires admin
				allowed = permissions.IsAdmin
			}

		case "docker":
			// Docker operations - check repository permissions
			allowed = len(permissions.Repositories) > 0

		default:
			// Unknown resource - be permissive for backward compatibility
			allowed = true
		}

		// Additional check for Xray-related operations
		if strings.Contains(opID, "xray") {
			allowed = allowed && permissions.EnabledFeatures["xray"]
		}

		if allowed {
			filtered[opID] = op
		}
	}

	d.logger.Info("Filtered operations by permissions", map[string]interface{}{
		"total":    len(operations),
		"allowed":  len(filtered),
		"is_admin": permissions.IsAdmin,
	})

	return filtered
}

// hasWritePermission checks if user has write permission on any repository
func (d *ArtifactoryPermissionDiscoverer) hasWritePermission(permissions *ArtifactoryPermissions) bool {
	for _, repoPerms := range permissions.Repositories {
		for _, perm := range repoPerms {
			if perm == "write" || perm == "deploy" || perm == "admin" {
				return true
			}
		}
	}
	return permissions.IsAdmin
}
