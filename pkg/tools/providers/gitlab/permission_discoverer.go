package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// GitLabPermissionDiscoverer discovers permissions for GitLab API tokens
type GitLabPermissionDiscoverer struct {
	baseDiscoverer *tools.PermissionDiscoverer
	logger         observability.Logger
	httpClient     *http.Client
	baseURL        string
}

// GitLabPermissions represents discovered GitLab permissions
type GitLabPermissions struct {
	*tools.DiscoveredPermissions
	// GitLab-specific fields
	UserID           int                    `json:"user_id,omitempty"`
	Username         string                 `json:"username,omitempty"`
	Email            string                 `json:"email,omitempty"`
	IsAdmin          bool                   `json:"is_admin"`
	CanCreateGroup   bool                   `json:"can_create_group"`
	CanCreateProject bool                   `json:"can_create_project"`
	TokenScopes      []string               `json:"token_scopes"`
	ProjectAccess    map[string]AccessLevel `json:"project_access"` // project path -> access level
	GroupAccess      map[string]AccessLevel `json:"group_access"`   // group path -> access level
	EnabledModules   map[string]bool        `json:"enabled_modules"`
}

// AccessLevel represents GitLab access levels
type AccessLevel struct {
	Level     int    `json:"level"`
	Name      string `json:"name"`
	CanPush   bool   `json:"can_push"`
	CanMerge  bool   `json:"can_merge"`
	CanDelete bool   `json:"can_delete"`
	CanAdmin  bool   `json:"can_admin"`
}

// GitLab access level constants
const (
	NoAccess         = 0
	MinimalAccess    = 5
	GuestAccess      = 10
	ReporterAccess   = 20
	DeveloperAccess  = 30
	MaintainerAccess = 40
	OwnerAccess      = 50
)

// NewGitLabPermissionDiscoverer creates a new GitLab permission discoverer
func NewGitLabPermissionDiscoverer(logger observability.Logger, baseURL string) *GitLabPermissionDiscoverer {
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}
	return &GitLabPermissionDiscoverer{
		baseDiscoverer: tools.NewPermissionDiscoverer(logger),
		logger:         logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
	}
}

// DiscoverPermissions discovers what permissions a GitLab token has
func (d *GitLabPermissionDiscoverer) DiscoverPermissions(ctx context.Context, token string) (*GitLabPermissions, error) {
	perms := &GitLabPermissions{
		DiscoveredPermissions: &tools.DiscoveredPermissions{
			Scopes:     []string{},
			RawHeaders: make(map[string]string),
			UserInfo:   make(map[string]interface{}),
		},
		ProjectAccess:  make(map[string]AccessLevel),
		GroupAccess:    make(map[string]AccessLevel),
		EnabledModules: make(map[string]bool),
		TokenScopes:    []string{},
	}

	// 1. Get user information and token scopes
	if err := d.getUserInfo(ctx, token, perms); err != nil {
		d.logger.Debug("Failed to get user info", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// 2. Check module access based on token scopes
	d.checkModuleAccess(perms)

	// 3. Discover accessible projects
	d.discoverProjectAccess(ctx, token, perms)

	// 4. Discover accessible groups
	d.discoverGroupAccess(ctx, token, perms)

	// 5. Check specific API permissions
	d.checkAPIPermissions(ctx, token, perms)

	return perms, nil
}

// getUserInfo retrieves user information and token scopes
func (d *GitLabPermissionDiscoverer) getUserInfo(ctx context.Context, token string, perms *GitLabPermissions) error {
	url := fmt.Sprintf("%s/user", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// GitLab supports multiple authentication methods
	if strings.HasPrefix(token, "glpat-") || strings.HasPrefix(token, "gldt-") {
		req.Header.Set("PRIVATE-TOKEN", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
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

	// Extract token scopes from response headers
	if scopes := resp.Header.Get("X-Oauth-Scopes"); scopes != "" {
		perms.TokenScopes = strings.Split(scopes, " ")
		perms.Scopes = perms.TokenScopes
	}

	var userInfo struct {
		ID               int    `json:"id"`
		Username         string `json:"username"`
		Email            string `json:"email"`
		Name             string `json:"name"`
		State            string `json:"state"`
		IsAdmin          bool   `json:"is_admin"`
		CanCreateGroup   bool   `json:"can_create_group"`
		CanCreateProject bool   `json:"can_create_project"`
		TwoFactorEnabled bool   `json:"two_factor_enabled"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return err
	}

	perms.UserID = userInfo.ID
	perms.Username = userInfo.Username
	perms.Email = userInfo.Email
	perms.IsAdmin = userInfo.IsAdmin
	perms.CanCreateGroup = userInfo.CanCreateGroup
	perms.CanCreateProject = userInfo.CanCreateProject

	perms.UserInfo["id"] = userInfo.ID
	perms.UserInfo["username"] = userInfo.Username
	perms.UserInfo["email"] = userInfo.Email
	perms.UserInfo["name"] = userInfo.Name
	perms.UserInfo["is_admin"] = userInfo.IsAdmin

	return nil
}

// checkModuleAccess determines which modules are accessible based on token scopes
func (d *GitLabPermissionDiscoverer) checkModuleAccess(perms *GitLabPermissions) {
	// Map token scopes to modules
	scopeModuleMap := map[string][]string{
		"api":              {"projects", "issues", "merge_requests", "pipelines", "jobs", "repositories", "groups", "users"},
		"read_api":         {"projects", "issues", "merge_requests", "pipelines", "jobs", "repositories", "groups", "users"},
		"read_user":        {"users"},
		"read_repository":  {"repositories"},
		"write_repository": {"repositories"},
		"read_registry":    {"container_registry", "packages"},
		"write_registry":   {"container_registry", "packages"},
		"read_wiki":        {"wikis"},
		"write_wiki":       {"wikis"},
		"create_runner":    {"runners"},
		"manage_runner":    {"runners"},
		"ai_features":      {"ai"},
		"k8s_proxy":        {"kubernetes"},
	}

	// Initialize all modules as disabled
	allModules := []string{
		"projects", "issues", "merge_requests", "pipelines", "jobs",
		"repositories", "groups", "users", "wikis", "snippets",
		"deployments", "environments", "packages", "container_registry",
		"security_reports", "runners",
	}

	for _, module := range allModules {
		perms.EnabledModules[module] = false
	}

	// Enable modules based on scopes
	for _, scope := range perms.TokenScopes {
		if modules, ok := scopeModuleMap[scope]; ok {
			for _, module := range modules {
				perms.EnabledModules[module] = true
				d.logger.Debug("Module enabled by scope", map[string]interface{}{
					"scope":  scope,
					"module": module,
				})
			}
		}
	}

	// If we have 'api' scope, enable all modules
	for _, scope := range perms.TokenScopes {
		if scope == "api" {
			for module := range perms.EnabledModules {
				perms.EnabledModules[module] = true
			}
			break
		}
	}
}

// discoverProjectAccess discovers which projects the token can access
func (d *GitLabPermissionDiscoverer) discoverProjectAccess(ctx context.Context, token string, perms *GitLabPermissions) {
	url := fmt.Sprintf("%s/projects?membership=true&per_page=100", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	if strings.HasPrefix(token, "glpat-") || strings.HasPrefix(token, "gldt-") {
		req.Header.Set("PRIVATE-TOKEN", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var projects []struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		Permissions       struct {
			ProjectAccess struct {
				AccessLevel int `json:"access_level"`
			} `json:"project_access"`
			GroupAccess struct {
				AccessLevel int `json:"access_level"`
			} `json:"group_access"`
		} `json:"permissions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return
	}

	for _, project := range projects {
		accessLevel := project.Permissions.ProjectAccess.AccessLevel
		if accessLevel == 0 {
			accessLevel = project.Permissions.GroupAccess.AccessLevel
		}

		perms.ProjectAccess[project.PathWithNamespace] = AccessLevel{
			Level:     accessLevel,
			Name:      getAccessLevelName(accessLevel),
			CanPush:   accessLevel >= DeveloperAccess,
			CanMerge:  accessLevel >= DeveloperAccess,
			CanDelete: accessLevel >= MaintainerAccess,
			CanAdmin:  accessLevel >= MaintainerAccess,
		}

		// Add to scopes
		perms.Scopes = append(perms.Scopes, fmt.Sprintf("project:%s:%s",
			project.PathWithNamespace, getAccessLevelName(accessLevel)))
	}
}

// discoverGroupAccess discovers which groups the token can access
func (d *GitLabPermissionDiscoverer) discoverGroupAccess(ctx context.Context, token string, perms *GitLabPermissions) {
	url := fmt.Sprintf("%s/groups?per_page=100", d.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	if strings.HasPrefix(token, "glpat-") || strings.HasPrefix(token, "gldt-") {
		req.Header.Set("PRIVATE-TOKEN", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var groups []struct {
		ID       int    `json:"id"`
		FullPath string `json:"full_path"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return
	}

	// For each group, check access level
	for _, group := range groups {
		// Try to get detailed group info to check permissions
		groupURL := fmt.Sprintf("%s/groups/%d", d.baseURL, group.ID)
		req, _ := http.NewRequestWithContext(ctx, "GET", groupURL, nil)

		if strings.HasPrefix(token, "glpat-") || strings.HasPrefix(token, "gldt-") {
			req.Header.Set("PRIVATE-TOKEN", token)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := d.httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		var groupDetail struct {
			ID          int    `json:"id"`
			FullPath    string `json:"full_path"`
			AccessLevel int    `json:"access_level"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&groupDetail); err == nil {
			accessLevel := groupDetail.AccessLevel
			if accessLevel == 0 {
				accessLevel = GuestAccess // Default to guest if we can see it
			}

			perms.GroupAccess[group.FullPath] = AccessLevel{
				Level:     accessLevel,
				Name:      getAccessLevelName(accessLevel),
				CanPush:   accessLevel >= DeveloperAccess,
				CanMerge:  accessLevel >= DeveloperAccess,
				CanDelete: accessLevel >= MaintainerAccess,
				CanAdmin:  accessLevel >= MaintainerAccess,
			}

			// Add to scopes
			perms.Scopes = append(perms.Scopes, fmt.Sprintf("group:%s:%s",
				group.FullPath, getAccessLevelName(accessLevel)))
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				d.logger.Warn("Failed to close response body", map[string]interface{}{"error": err.Error()})
			}
		}()
	}
}

// checkAPIPermissions checks specific API endpoint permissions
func (d *GitLabPermissionDiscoverer) checkAPIPermissions(ctx context.Context, token string, perms *GitLabPermissions) {
	// Check various endpoints to determine permissions
	endpoints := []struct {
		path   string
		module string
		scope  string
	}{
		{"/projects", "projects", "read:projects"},
		{"/issues", "issues", "read:issues"},
		{"/merge_requests", "merge_requests", "read:merge_requests"},
		{"/runners", "runners", "read:runners"},
		{"/jobs", "jobs", "read:jobs"},
	}

	for _, endpoint := range endpoints {
		if d.checkEndpointAccess(ctx, token, endpoint.path) {
			if !contains(perms.Scopes, endpoint.scope) {
				perms.Scopes = append(perms.Scopes, endpoint.scope)
			}
			perms.EnabledModules[endpoint.module] = true
		}
	}
}

// checkEndpointAccess checks if an endpoint is accessible
func (d *GitLabPermissionDiscoverer) checkEndpointAccess(ctx context.Context, token string, path string) bool {
	url := fmt.Sprintf("%s%s?per_page=1", d.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	if strings.HasPrefix(token, "glpat-") || strings.HasPrefix(token, "gldt-") {
		req.Header.Set("PRIVATE-TOKEN", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 200-299 means we have access
	// 403 means no access
	// 401 means authentication failed
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// FilterOperationsByPermissions filters operations based on discovered permissions
func (d *GitLabPermissionDiscoverer) FilterOperationsByPermissions(
	operationMappings map[string]interface{},
	permissions *GitLabPermissions,
) map[string]bool {
	allowedOps := make(map[string]bool)

	for opID := range operationMappings {
		// Check module access first
		module := strings.Split(opID, "/")[0]
		var allowed bool

		// Map operation prefix to module
		switch module {
		case "projects":
			allowed = permissions.EnabledModules["projects"]
		case "issues":
			allowed = permissions.EnabledModules["issues"]
		case "merge_requests":
			allowed = permissions.EnabledModules["merge_requests"]
		case "pipelines":
			allowed = permissions.EnabledModules["pipelines"]
		case "jobs":
			allowed = permissions.EnabledModules["jobs"]
		case "branches", "commits", "tags":
			allowed = permissions.EnabledModules["repositories"]
		case "groups":
			allowed = permissions.EnabledModules["groups"]
		case "users":
			allowed = permissions.EnabledModules["users"]
		case "runners":
			allowed = permissions.EnabledModules["runners"]
		default:
			// Check if we have any related scope
			allowed = d.hasRelatedScope(opID, permissions.Scopes)
		}

		// Check write operations require write scopes
		if allowed && (strings.Contains(opID, "create") || strings.Contains(opID, "update") ||
			strings.Contains(opID, "delete") || strings.Contains(opID, "trigger")) {

			// Check if we have write permissions
			hasWriteScope := false
			for _, scope := range permissions.TokenScopes {
				if scope == "api" || strings.HasPrefix(scope, "write_") {
					hasWriteScope = true
					break
				}
			}

			if !hasWriteScope {
				allowed = false
			}
		}

		// Check project-specific operations
		if allowed && strings.Contains(opID, "projects/") {
			// For project-specific operations, check if user has any project access
			if len(permissions.ProjectAccess) == 0 && !permissions.CanCreateProject {
				allowed = false
			}
		}

		// Check group-specific operations
		if allowed && strings.Contains(opID, "groups/") {
			// For group-specific operations, check if user has any group access
			if len(permissions.GroupAccess) == 0 && !permissions.CanCreateGroup {
				allowed = false
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

	d.logger.Info("GitLab permission filtering complete", map[string]interface{}{
		"total_operations":   len(allowedOps),
		"allowed_operations": countAllowed(allowedOps),
		"enabled_modules":    countEnabled(permissions.EnabledModules),
		"project_access":     len(permissions.ProjectAccess),
		"group_access":       len(permissions.GroupAccess),
	})

	return allowedOps
}

// hasRelatedScope checks if any scope relates to the operation
func (d *GitLabPermissionDiscoverer) hasRelatedScope(operation string, scopes []string) bool {
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

func getAccessLevelName(level int) string {
	switch level {
	case NoAccess:
		return "no_access"
	case MinimalAccess:
		return "minimal"
	case GuestAccess:
		return "guest"
	case ReporterAccess:
		return "reporter"
	case DeveloperAccess:
		return "developer"
	case MaintainerAccess:
		return "maintainer"
	case OwnerAccess:
		return "owner"
	default:
		return "unknown"
	}
}

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
