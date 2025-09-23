package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// HarnessPermissionDiscoverer discovers permissions for Harness API keys
type HarnessPermissionDiscoverer struct {
	baseDiscoverer *tools.PermissionDiscoverer
	logger         observability.Logger
	httpClient     *http.Client
}

// HarnessPermissions represents discovered Harness permissions
type HarnessPermissions struct {
	*tools.DiscoveredPermissions
	// Harness-specific fields
	AccountID      string              `json:"account_id,omitempty"`
	UserEmail      string              `json:"user_email,omitempty"`
	UserName       string              `json:"user_name,omitempty"`
	EnabledModules map[string]bool     `json:"enabled_modules"`
	ResourceAccess map[string][]string `json:"resource_access"` // resource type -> permissions
	ProjectAccess  map[string]bool     `json:"project_access"`  // project ID -> has access
	OrgAccess      map[string]bool     `json:"org_access"`      // org ID -> has access
}

// NewHarnessPermissionDiscoverer creates a new Harness permission discoverer
func NewHarnessPermissionDiscoverer(logger observability.Logger) *HarnessPermissionDiscoverer {
	return &HarnessPermissionDiscoverer{
		baseDiscoverer: tools.NewPermissionDiscoverer(logger),
		logger:         logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DiscoverPermissions discovers what permissions a Harness API key has
func (d *HarnessPermissionDiscoverer) DiscoverPermissions(ctx context.Context, apiKey string) (*HarnessPermissions, error) {
	perms := &HarnessPermissions{
		DiscoveredPermissions: &tools.DiscoveredPermissions{
			Scopes:     []string{},
			RawHeaders: make(map[string]string),
			UserInfo:   make(map[string]interface{}),
		},
		EnabledModules: make(map[string]bool),
		ResourceAccess: make(map[string][]string),
		ProjectAccess:  make(map[string]bool),
		OrgAccess:      make(map[string]bool),
	}

	// Extract account ID from API key if possible
	if strings.HasPrefix(apiKey, "pat.") {
		parts := strings.Split(apiKey, ".")
		if len(parts) >= 3 {
			perms.AccountID = parts[1]
		}
	}

	// 1. Get user information
	if err := d.getUserInfo(ctx, apiKey, perms); err != nil {
		d.logger.Debug("Failed to get user info", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 2. Probe module access
	d.probeModuleAccess(ctx, apiKey, perms)

	// 3. Check resource permissions
	d.checkResourcePermissions(ctx, apiKey, perms)

	// 4. List accessible projects and orgs
	d.discoverAccessibleResources(ctx, apiKey, perms)

	return perms, nil
}

// getUserInfo retrieves user information from the API key
func (d *HarnessPermissionDiscoverer) getUserInfo(ctx context.Context, apiKey string, perms *HarnessPermissions) error {
	url := "https://app.harness.io/gateway/ng/api/user/currentUser"
	if perms.AccountID != "" {
		url = fmt.Sprintf("%s?accountIdentifier=%s", url, perms.AccountID)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get user info: status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Email          string   `json:"email"`
			Name           string   `json:"name"`
			UUID           string   `json:"uuid"`
			Accounts       []string `json:"accounts"`
			DefaultAccount string   `json:"defaultAccountId"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	perms.UserEmail = result.Data.Email
	perms.UserName = result.Data.Name
	perms.UserInfo["uuid"] = result.Data.UUID
	perms.UserInfo["email"] = result.Data.Email
	perms.UserInfo["name"] = result.Data.Name

	if perms.AccountID == "" && result.Data.DefaultAccount != "" {
		perms.AccountID = result.Data.DefaultAccount
	}

	return nil
}

// probeModuleAccess checks which Harness modules the API key can access
func (d *HarnessPermissionDiscoverer) probeModuleAccess(ctx context.Context, apiKey string, perms *HarnessPermissions) {
	modules := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"pipeline", "/pipeline/api/pipelines/list", "POST"},
		{"project", "/v1/orgs", "GET"},
		{"connector", "/ng/api/connectors/listV2", "POST"},
		{"ci", "/ci/api/builds", "GET"},
		{"cd", "/ng/api/services", "GET"},
		{"ccm", "/ccm/api/graphql", "POST"},
		{"gitops", "/gitops/api/v1/agents", "GET"},
		{"cv", "/cv/api/monitored-service/list", "POST"},
		{"sto", "/sto/api/v2/scans", "GET"},
		{"cf", "/cf/admin/features", "GET"},
		{"iacm", "/iacm/api/workspaces", "GET"},
		{"code", "/code/api/v1/repos", "GET"},
	}

	for _, module := range modules {
		hasAccess := d.probeEndpoint(ctx, module.endpoint, module.method, apiKey, perms.AccountID)
		perms.EnabledModules[module.name] = hasAccess

		if hasAccess {
			d.logger.Debug("Module access granted", map[string]interface{}{
				"module": module.name,
			})
			perms.Scopes = append(perms.Scopes, fmt.Sprintf("module:%s", module.name))
		}
	}
}

// probeEndpoint checks if an endpoint is accessible
func (d *HarnessPermissionDiscoverer) probeEndpoint(ctx context.Context, endpoint, method, apiKey, accountID string) bool {
	url := "https://app.harness.io" + endpoint

	// Add account ID to query params if needed
	if accountID != "" && !strings.Contains(url, "accountIdentifier") {
		if strings.Contains(url, "?") {
			url += "&accountIdentifier=" + accountID
		} else {
			url += "?accountIdentifier=" + accountID
		}
	}

	var body io.Reader
	if method == "POST" {
		// Minimal valid body for POST requests
		bodyStr := `{"filterType":"All"}`
		if strings.Contains(endpoint, "graphql") {
			bodyStr = `{"query":"{ __typename }"}`
		}
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return false
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 200-299 means we have access
	// 403 means no access
	// 400-402, 404-499 might mean bad request but we have access to try
	return resp.StatusCode >= 200 && resp.StatusCode < 300 ||
		(resp.StatusCode >= 400 && resp.StatusCode < 403) ||
		(resp.StatusCode > 403 && resp.StatusCode < 500)
}

// checkResourcePermissions checks specific resource permissions
func (d *HarnessPermissionDiscoverer) checkResourcePermissions(ctx context.Context, apiKey string, perms *HarnessPermissions) {
	// Check pipeline permissions
	if perms.EnabledModules["pipeline"] {
		perms.ResourceAccess["pipeline"] = []string{}

		// Check various pipeline operations
		operations := []struct {
			name     string
			endpoint string
			method   string
		}{
			{"create", "/v1/orgs/default/projects/default/pipelines", "POST"},
			{"execute", "/pipeline/api/pipeline/execute/test", "POST"},
			{"approve", "/pipeline/api/approvals/test", "POST"},
		}

		for _, op := range operations {
			if d.probeEndpoint(ctx, op.endpoint, op.method, apiKey, perms.AccountID) {
				perms.ResourceAccess["pipeline"] = append(perms.ResourceAccess["pipeline"], op.name)
			}
		}
	}

	// Check project permissions
	if perms.EnabledModules["project"] {
		perms.ResourceAccess["project"] = []string{}

		// Try to create a project (will fail but we can check the error)
		if d.probeEndpoint(ctx, "/v1/orgs/default/projects", "POST", apiKey, perms.AccountID) {
			perms.ResourceAccess["project"] = append(perms.ResourceAccess["project"], "create")
		}

		// List is already checked via module probe
		perms.ResourceAccess["project"] = append(perms.ResourceAccess["project"], "list", "read")
	}

	// Check connector permissions
	if perms.EnabledModules["connector"] {
		perms.ResourceAccess["connector"] = []string{"list", "read"}

		// Check if we can validate connectors
		if d.probeEndpoint(ctx, "/ng/api/connectors/testConnection", "POST", apiKey, perms.AccountID) {
			perms.ResourceAccess["connector"] = append(perms.ResourceAccess["connector"], "validate")
		}
	}
}

// discoverAccessibleResources lists accessible projects and organizations
func (d *HarnessPermissionDiscoverer) discoverAccessibleResources(ctx context.Context, apiKey string, perms *HarnessPermissions) {
	// Try to list organizations
	orgsURL := "https://app.harness.io/v1/orgs"
	if perms.AccountID != "" {
		orgsURL += "?account=" + perms.AccountID
	}

	req, err := http.NewRequestWithContext(ctx, "GET", orgsURL, nil)
	if err == nil {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := d.httpClient.Do(req)
		if err == nil {
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode == http.StatusOK {
				var result struct {
					Orgs []struct {
						Identifier string `json:"identifier"`
						Name       string `json:"name"`
					} `json:"orgs"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					for _, org := range result.Orgs {
						perms.OrgAccess[org.Identifier] = true
						perms.Scopes = append(perms.Scopes, fmt.Sprintf("org:%s", org.Identifier))

						// Try to list projects in this org
						d.discoverProjectsInOrg(ctx, apiKey, org.Identifier, perms)
					}
				}
			}
		}
	}
}

// discoverProjectsInOrg lists accessible projects in an organization
func (d *HarnessPermissionDiscoverer) discoverProjectsInOrg(ctx context.Context, apiKey, orgID string, perms *HarnessPermissions) {
	projectsURL := fmt.Sprintf("https://app.harness.io/v1/orgs/%s/projects", orgID)
	if perms.AccountID != "" {
		projectsURL += "?account=" + perms.AccountID
	}

	req, err := http.NewRequestWithContext(ctx, "GET", projectsURL, nil)
	if err != nil {
		return
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		var result struct {
			Projects []struct {
				Identifier string   `json:"identifier"`
				Name       string   `json:"name"`
				Modules    []string `json:"modules"`
			} `json:"projects"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			for _, project := range result.Projects {
				projectKey := fmt.Sprintf("%s/%s", orgID, project.Identifier)
				perms.ProjectAccess[projectKey] = true
				perms.Scopes = append(perms.Scopes, fmt.Sprintf("project:%s", projectKey))
			}
		}
	}
}

// FilterOperationsByPermissions filters operations based on discovered permissions
func (d *HarnessPermissionDiscoverer) FilterOperationsByPermissions(
	operationMappings map[string]interface{},
	permissions *HarnessPermissions,
) map[string]bool {
	allowedOps := make(map[string]bool)

	for opID := range operationMappings {
		// Check module access first
		module := strings.Split(opID, "/")[0]
		var allowed bool
		switch module {
		case "pipelines":
			allowed = permissions.EnabledModules["pipeline"]
		case "projects", "orgs":
			allowed = permissions.EnabledModules["project"]
		case "connectors":
			allowed = permissions.EnabledModules["connector"]
		case "gitops":
			allowed = permissions.EnabledModules["gitops"]
		case "sto":
			allowed = permissions.EnabledModules["sto"]
		case "ccm":
			allowed = permissions.EnabledModules["ccm"]
		default:
			// Unknown module, check if we have any related scope
			allowed = d.hasRelatedScope(opID, permissions.Scopes)
		}

		// Check specific resource permissions
		if allowed && len(permissions.ResourceAccess) > 0 {
			// Map the module name to resource key (e.g., "pipelines" -> "pipeline")
			resourceKey := module
			switch module {
			case "pipelines":
				resourceKey = "pipeline"
			case "projects", "orgs":
				resourceKey = "project"
			case "connectors":
				resourceKey = "connector"
			}
			resourcePerms, hasPerms := permissions.ResourceAccess[resourceKey]
			if hasPerms {
				// Check if the operation requires specific permission
				if strings.Contains(opID, "create") && !contains(resourcePerms, "create") {
					allowed = false
				} else if strings.Contains(opID, "delete") && !contains(resourcePerms, "delete") {
					allowed = false
				} else if strings.Contains(opID, "update") && !contains(resourcePerms, "update") {
					allowed = false
				} else if strings.Contains(opID, "execute") && !contains(resourcePerms, "execute") {
					allowed = false
				}
			}
		}

		allowedOps[opID] = allowed

		if allowed {
			d.logger.Debug("Operation allowed", map[string]interface{}{
				"operation": opID,
				"module":    module,
			})
		}
	}

	d.logger.Info("Harness permission filtering complete", map[string]interface{}{
		"total_operations":   len(allowedOps),
		"allowed_operations": countAllowed(allowedOps),
		"enabled_modules":    countEnabled(permissions.EnabledModules),
	})

	return allowedOps
}

// hasRelatedScope checks if any scope relates to the operation
func (d *HarnessPermissionDiscoverer) hasRelatedScope(operation string, scopes []string) bool {
	opLower := strings.ToLower(operation)
	for _, scope := range scopes {
		scopeLower := strings.ToLower(scope)
		// Check if scope contains any part of the operation
		if strings.Contains(opLower, scopeLower) || strings.Contains(scopeLower, opLower) {
			return true
		}
	}
	return false
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func countAllowed(ops map[string]bool) int {
	count := 0
	for _, allowed := range ops {
		if allowed {
			count++
		}
	}
	return count
}

func countEnabled(modules map[string]bool) int {
	count := 0
	for _, enabled := range modules {
		if enabled {
			count++
		}
	}
	return count
}
