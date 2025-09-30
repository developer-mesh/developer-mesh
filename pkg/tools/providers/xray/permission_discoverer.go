package xray

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

// XrayPermissionDiscoverer discovers permissions for JFrog Xray API keys
type XrayPermissionDiscoverer struct {
	logger     observability.Logger
	httpClient *http.Client
	baseURL    string
}

// XrayPermissions represents discovered Xray permissions
type XrayPermissions struct {
	UserInfo        map[string]interface{}
	EnabledFeatures map[string]bool // feature -> enabled (e.g., watches, policies, reports)
	IsAdmin         bool
	CanScan         bool
	CanCreatePolicy bool
	CanCreateWatch  bool
	CanViewReports  bool
	Scopes          []string // For compatibility
	RawHeaders      map[string]string
}

// NewXrayPermissionDiscoverer creates a new Xray permission discoverer
func NewXrayPermissionDiscoverer(logger observability.Logger, baseURL string) *XrayPermissionDiscoverer {
	if baseURL == "" {
		baseURL = "https://mycompany.jfrog.io/xray"
	}
	return &XrayPermissionDiscoverer{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
	}
}

// DiscoverPermissions discovers what permissions an Xray API key has
func (d *XrayPermissionDiscoverer) DiscoverPermissions(ctx context.Context, apiKey string) (*XrayPermissions, error) {
	perms := &XrayPermissions{
		UserInfo:        make(map[string]interface{}),
		EnabledFeatures: make(map[string]bool),
		Scopes:          []string{},
		RawHeaders:      make(map[string]string),
	}

	// 1. Check basic connectivity and get version info
	if err := d.getSystemInfo(ctx, apiKey, perms); err != nil {
		d.logger.Debug("Failed to get system info", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 2. Check scan permissions
	d.checkScanPermissions(ctx, apiKey, perms)

	// 3. Check policy management permissions
	d.checkPolicyPermissions(ctx, apiKey, perms)

	// 4. Check watch management permissions
	d.checkWatchPermissions(ctx, apiKey, perms)

	// 5. Check reporting permissions
	d.checkReportingPermissions(ctx, apiKey, perms)

	// 6. Check admin capabilities
	d.checkAdminAccess(ctx, apiKey, perms)

	// 7. Detect available features based on probes
	d.detectFeatures(perms)

	return perms, nil
}

// getSystemInfo retrieves system information and version
func (d *XrayPermissionDiscoverer) getSystemInfo(ctx context.Context, apiKey string, perms *XrayPermissions) error {
	versionURL := fmt.Sprintf("%s/api/v1/system/version", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create version request: %w", err)
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get version info: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("version request failed with status %d", resp.StatusCode)
	}

	// Parse version response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read version response: %w", err)
	}

	var versionInfo map[string]interface{}
	if err := json.Unmarshal(body, &versionInfo); err != nil {
		return fmt.Errorf("failed to parse version response: %w", err)
	}

	perms.UserInfo["xray_version"] = versionInfo
	perms.EnabledFeatures["xray"] = true

	return nil
}

// checkScanPermissions checks if the user can perform scans
func (d *XrayPermissionDiscoverer) checkScanPermissions(ctx context.Context, apiKey string, perms *XrayPermissions) {
	// Try to list recent scans (usually requires scan permissions)
	scanURL := fmt.Sprintf("%s/api/v1/scan/status/test", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", scanURL, nil)
	if err != nil {
		d.logger.Debug("Failed to create scan check request", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Debug("Failed to check scan permissions", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 404 is expected (scan doesn't exist), but means we can access scan endpoints
	// 403 means no permission
	if resp.StatusCode != http.StatusForbidden {
		perms.CanScan = true
		perms.EnabledFeatures["scanning"] = true
	}
}

// checkPolicyPermissions checks if the user can manage policies
func (d *XrayPermissionDiscoverer) checkPolicyPermissions(ctx context.Context, apiKey string, perms *XrayPermissions) {
	// Try to list policies
	policyURL := fmt.Sprintf("%s/api/v2/policies", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", policyURL, nil)
	if err != nil {
		d.logger.Debug("Failed to create policy check request", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Debug("Failed to check policy permissions", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		perms.CanCreatePolicy = true
		perms.EnabledFeatures["policies"] = true

		// Parse response to see if any policies exist
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			var policies []interface{}
			if err := json.Unmarshal(body, &policies); err == nil {
				perms.UserInfo["policy_count"] = len(policies)
			}
		}
	}
}

// checkWatchPermissions checks if the user can manage watches
func (d *XrayPermissionDiscoverer) checkWatchPermissions(ctx context.Context, apiKey string, perms *XrayPermissions) {
	// Try to list watches
	watchURL := fmt.Sprintf("%s/api/v2/watches", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", watchURL, nil)
	if err != nil {
		d.logger.Debug("Failed to create watch check request", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Debug("Failed to check watch permissions", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		perms.CanCreateWatch = true
		perms.EnabledFeatures["watches"] = true

		// Parse response to see if any watches exist
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			var watches []interface{}
			if err := json.Unmarshal(body, &watches); err == nil {
				perms.UserInfo["watch_count"] = len(watches)
			}
		}
	}
}

// checkReportingPermissions checks if the user can generate reports
func (d *XrayPermissionDiscoverer) checkReportingPermissions(ctx context.Context, apiKey string, perms *XrayPermissions) {
	// Try to get a non-existent report (to check access to reports API)
	reportURL := fmt.Sprintf("%s/api/v1/reports/test", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reportURL, nil)
	if err != nil {
		d.logger.Debug("Failed to create report check request", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	d.applyAuthentication(req, apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Debug("Failed to check report permissions", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 404 is expected (report doesn't exist), but means we can access report endpoints
	// 403 means no permission
	if resp.StatusCode != http.StatusForbidden {
		perms.CanViewReports = true
		perms.EnabledFeatures["reports"] = true
	}
}

// checkAdminAccess checks if the user has admin permissions
func (d *XrayPermissionDiscoverer) checkAdminAccess(ctx context.Context, apiKey string, perms *XrayPermissions) {
	// Admin users can typically access system configuration endpoints
	// This is a heuristic - actual admin detection may vary

	// Check if user can create policies (often admin-only)
	if perms.CanCreatePolicy {
		// Try to access a more privileged endpoint
		configURL := fmt.Sprintf("%s/api/v1/configuration/systemParameters", d.baseURL)

		req, err := http.NewRequestWithContext(ctx, "GET", configURL, nil)
		if err != nil {
			d.logger.Debug("Failed to create admin check request", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		d.applyAuthentication(req, apiKey)

		resp, err := d.httpClient.Do(req)
		if err != nil {
			d.logger.Debug("Failed to check admin access", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusOK {
			perms.IsAdmin = true
			perms.EnabledFeatures["admin"] = true
		}
	}
}

// detectFeatures sets feature flags based on discovered permissions
func (d *XrayPermissionDiscoverer) detectFeatures(perms *XrayPermissions) {
	// Aggregate features based on permissions
	if perms.CanScan || perms.UserInfo["xray_version"] != nil {
		perms.EnabledFeatures["vulnerability_scanning"] = true
	}

	if perms.CanCreatePolicy || perms.CanCreateWatch {
		perms.EnabledFeatures["compliance"] = true
	}

	if perms.CanViewReports {
		perms.EnabledFeatures["reporting"] = true
	}

	// Set scopes based on permissions for compatibility
	scopes := []string{}
	if perms.CanScan {
		scopes = append(scopes, "xray:scan:read", "xray:scan:write")
	}
	if perms.CanCreatePolicy {
		scopes = append(scopes, "xray:policy:read", "xray:policy:write")
	}
	if perms.CanCreateWatch {
		scopes = append(scopes, "xray:watch:read", "xray:watch:write")
	}
	if perms.CanViewReports {
		scopes = append(scopes, "xray:report:read", "xray:report:write")
	}
	if perms.IsAdmin {
		scopes = append(scopes, "xray:admin")
	}
	perms.Scopes = scopes
}

// FilterOperationsByPermissions filters operations based on discovered permissions
func (d *XrayPermissionDiscoverer) FilterOperationsByPermissions(
	operations map[string]providers.OperationMapping,
	permissions *XrayPermissions,
) map[string]providers.OperationMapping {
	filtered := make(map[string]providers.OperationMapping)

	for opID, op := range operations {
		allowed := false

		// Always allow system operations
		if strings.HasPrefix(opID, "system/") {
			allowed = true
		} else if strings.Contains(opID, "scan") {
			allowed = permissions.CanScan
		} else if strings.Contains(opID, "summary") || strings.Contains(opID, "component") {
			// These are read operations, generally allowed if Xray is available
			allowed = permissions.EnabledFeatures["xray"]
		} else if strings.Contains(opID, "violation") {
			// Violations are viewable with basic access
			allowed = permissions.EnabledFeatures["xray"]
		} else if strings.Contains(opID, "watch") {
			if strings.Contains(opID, "create") || strings.Contains(opID, "update") || strings.Contains(opID, "delete") {
				allowed = permissions.CanCreateWatch
			} else {
				// List/get operations
				allowed = permissions.EnabledFeatures["watches"]
			}
		} else if strings.Contains(opID, "polic") {
			if strings.Contains(opID, "create") || strings.Contains(opID, "update") || strings.Contains(opID, "delete") {
				allowed = permissions.CanCreatePolicy
			} else {
				// List/get operations
				allowed = permissions.EnabledFeatures["policies"]
			}
		} else if strings.Contains(opID, "report") {
			allowed = permissions.CanViewReports
		} else if strings.Contains(opID, "ignore-rule") {
			// Ignore rules typically require admin or special permissions
			allowed = permissions.IsAdmin || permissions.CanCreatePolicy
		} else {
			// Default to allowed for unknown operations if Xray is available
			allowed = permissions.EnabledFeatures["xray"]
		}

		if allowed {
			filtered[opID] = op
		}
	}

	d.logger.Info("Filtered Xray operations", map[string]interface{}{
		"total":    len(operations),
		"allowed":  len(filtered),
		"is_admin": permissions.IsAdmin,
		"can_scan": permissions.CanScan,
	})

	return filtered
}

// applyAuthentication applies the appropriate authentication header
func (d *XrayPermissionDiscoverer) applyAuthentication(req *http.Request, apiKey string) {
	// Detect auth type and apply appropriate header
	if d.isJFrogAPIKey(apiKey) {
		req.Header.Set("X-JFrog-Art-Api", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
}

// isJFrogAPIKey detects if the provided credential is a JFrog API key vs access token
func (d *XrayPermissionDiscoverer) isJFrogAPIKey(apiKey string) bool {
	// JFrog API keys are typically 64-73 character base64 strings
	// Access tokens are JWTs starting with "ey"
	if len(apiKey) >= 64 && len(apiKey) <= 73 && !strings.HasPrefix(apiKey, "ey") {
		return true
	}
	return false
}
