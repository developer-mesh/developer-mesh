package artifactory

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// Capability represents the availability status of a feature or operation
type Capability struct {
	Available bool     `json:"available"`
	Reason    string   `json:"reason,omitempty"`
	Required  []string `json:"required,omitempty"` // Required: ["Xray license", "Admin permission"]
}

// CapabilityReport provides a comprehensive report of available operations and features
type CapabilityReport struct {
	Operations map[string]Capability `json:"operations"`
	Features   map[string]Capability `json:"features"`
	Timestamp  time.Time             `json:"timestamp"`
	CacheValid bool                  `json:"cache_valid"`
}

// CapabilityDiscoverer handles capability discovery and caching
type CapabilityDiscoverer struct {
	logger        observability.Logger
	cache         *CapabilityReport
	cacheMutex    sync.RWMutex
	cacheDuration time.Duration
	lastDiscovery time.Time
}

// NewCapabilityDiscoverer creates a new capability discoverer
func NewCapabilityDiscoverer(logger observability.Logger) *CapabilityDiscoverer {
	if logger == nil {
		logger = &observability.NoopLogger{}
	}

	return &CapabilityDiscoverer{
		logger:        logger,
		cacheDuration: 15 * time.Minute, // Cache capabilities for 15 minutes
	}
}

// DiscoverCapabilities performs comprehensive capability discovery
func (cd *CapabilityDiscoverer) DiscoverCapabilities(ctx context.Context, provider *ArtifactoryProvider) (*CapabilityReport, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if provider == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}

	// Check cache validity
	cd.cacheMutex.RLock()
	if cd.cache != nil && time.Since(cd.lastDiscovery) < cd.cacheDuration {
		cached := *cd.cache
		cached.CacheValid = true
		cd.cacheMutex.RUnlock()
		cd.logger.Debug("Returning cached capability report", map[string]interface{}{
			"age_seconds": time.Since(cd.lastDiscovery).Seconds(),
		})
		return &cached, nil
	}
	cd.cacheMutex.RUnlock()

	// Perform fresh discovery
	report := &CapabilityReport{
		Operations: make(map[string]Capability),
		Features:   make(map[string]Capability),
		Timestamp:  time.Now(),
		CacheValid: false,
	}

	// Discover feature availability
	cd.discoverFeatures(ctx, provider, report)

	// Discover operation availability
	cd.discoverOperations(ctx, provider, report)

	// Cache the results
	cd.cacheMutex.Lock()
	cd.cache = report
	cd.lastDiscovery = time.Now()
	cd.cacheMutex.Unlock()

	cd.logger.Info("Capability discovery completed", map[string]interface{}{
		"operations_count":     len(report.Operations),
		"features_count":       len(report.Features),
		"available_operations": cd.countAvailable(report.Operations),
		"available_features":   cd.countAvailable(report.Features),
	})

	return report, nil
}

// discoverFeatures probes various endpoints to determine feature availability
func (cd *CapabilityDiscoverer) discoverFeatures(ctx context.Context, provider *ArtifactoryProvider, report *CapabilityReport) {
	// Core features to check
	features := []struct {
		name     string
		endpoint string
		required []string
	}{
		{
			name:     "artifactory_core",
			endpoint: "/api/system/ping",
			required: []string{"Artifactory license"},
		},
		{
			name:     "xray",
			endpoint: "/xray/api/v1/system/version",
			required: []string{"Xray license", "Xray installation"},
		},
		{
			name:     "pipelines",
			endpoint: "/pipelines/api/v1/system/info",
			required: []string{"Pipelines license", "Cloud or self-hosted Pipelines"},
		},
		{
			name:     "mission_control",
			endpoint: "/mc/api/v1/system/info",
			required: []string{"Mission Control license", "Enterprise license"},
		},
		{
			name:     "distribution",
			endpoint: "/distribution/api/v1/system/info",
			required: []string{"Distribution license", "Edge nodes"},
		},
		{
			name:     "access_service",
			endpoint: "/access/api/v1/system/ping",
			required: []string{"Access service", "Platform installation"},
		},
		{
			name:     "projects",
			endpoint: "/access/api/v1/projects",
			required: []string{"Projects feature", "Platform Pro or Enterprise"},
		},
		{
			name:     "federation",
			endpoint: "/api/federation/status",
			required: []string{"Federation license", "Enterprise Plus"},
		},
	}

	for _, feature := range features {
		capability := cd.probeEndpoint(ctx, provider, feature.endpoint)
		capability.Required = feature.required

		// Add context-specific reasons or override generic messages
		if !capability.Available {
			switch feature.name {
			case "xray":
				if capability.Reason == "" {
					capability.Reason = "Xray is not installed or accessible"
				}
			case "pipelines":
				if capability.Reason == "" {
					capability.Reason = "Pipelines is a cloud-only feature or not configured"
				}
			case "mission_control":
				if capability.Reason == "" {
					capability.Reason = "Mission Control requires Enterprise license"
				}
			case "projects":
				// Always override for projects to provide specific license info
				capability.Reason = "Projects feature requires Platform Pro or Enterprise license"
			case "federation":
				if capability.Reason == "" {
					capability.Reason = "Federation requires Enterprise Plus license"
				}
			default:
				if capability.Reason == "" {
					capability.Reason = fmt.Sprintf("%s is not available", feature.name)
				}
			}
		}

		report.Features[feature.name] = capability
	}

	// Check package type support
	cd.checkPackageTypeSupport(ctx, provider, report)
}

// discoverOperations checks which operations are available based on permissions and features
func (cd *CapabilityDiscoverer) discoverOperations(ctx context.Context, provider *ArtifactoryProvider, report *CapabilityReport) {
	// Get all operations
	allOps := provider.getAllOperationMappings()

	// Check each operation category
	for opID, opMapping := range allOps {
		capability := Capability{Available: true}

		// Categorize operations and check availability
		switch {
		case strings.HasPrefix(opID, "xray/"):
			// Xray operations require Xray to be installed
			if xrayFeature, exists := report.Features["xray"]; exists && !xrayFeature.Available {
				capability.Available = false
				capability.Reason = "Xray is not installed or accessible"
				capability.Required = []string{"Xray license", "Xray installation"}
			}

		case strings.Contains(opID, "admin") || strings.Contains(opID, "system/configuration"):
			// Admin operations require admin permissions
			capability = cd.checkAdminOperation(ctx, provider, opID)

		case strings.Contains(opID, "federation"):
			// Federation operations require federation feature
			if fedFeature, exists := report.Features["federation"]; exists && !fedFeature.Available {
				capability.Available = false
				capability.Reason = "Federation is not available (requires Enterprise Plus)"
				capability.Required = []string{"Federation license", "Enterprise Plus"}
			}

		case strings.Contains(opID, "project"):
			// Project operations require projects feature
			if projFeature, exists := report.Features["projects"]; exists && !projFeature.Available {
				capability.Available = false
				capability.Reason = "Projects feature is not available"
				capability.Required = []string{"Projects feature", "Platform Pro or Enterprise"}
			}

		case strings.Contains(opID, "cloud") || strings.Contains(opID, "runtime"):
			// Cloud-only operations
			capability.Available = false
			capability.Reason = "This is a cloud-only feature"
			capability.Required = []string{"JFrog Cloud subscription"}

		case strings.HasPrefix(opID, "internal/"):
			// Internal operations are always available
			capability.Available = true

		default:
			// Check if operation requires specific permissions
			capability = cd.checkOperationPermission(ctx, provider, opID, opMapping)
		}

		report.Operations[opID] = capability
	}
}

// probeEndpoint checks if an endpoint is accessible
func (cd *CapabilityDiscoverer) probeEndpoint(ctx context.Context, provider *ArtifactoryProvider, endpoint string) Capability {
	// Try to make the request
	resp, err := provider.ExecuteHTTPRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		// Check if this is an authentication error
		errStr := err.Error()
		if strings.Contains(errStr, "no credentials") || strings.Contains(errStr, "authentication") {
			// If no auth is configured, we can't determine availability accurately
			// Assume feature is potentially available but requires auth
			return Capability{
				Available: true,
				Reason:    "Feature availability requires authentication",
			}
		}

		// Check if this is a network/connection error that indicates the feature doesn't exist
		if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") {
			return Capability{
				Available: false,
				Reason:    "Feature not installed or endpoint not available",
			}
		}

		// Other errors
		return Capability{
			Available: false,
			Reason:    fmt.Sprintf("Failed to probe endpoint: %v", err),
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return Capability{
			Available: true,
		}
	case http.StatusUnauthorized:
		return Capability{
			Available: false,
			Reason:    "Authentication required or invalid credentials",
		}
	case http.StatusForbidden:
		return Capability{
			Available: false,
			Reason:    "No permission to access this feature",
		}
	case http.StatusNotFound:
		return Capability{
			Available: false,
			Reason:    "Feature not installed or endpoint not available",
		}
	case http.StatusServiceUnavailable:
		return Capability{
			Available: false,
			Reason:    "Service is currently unavailable",
		}
	default:
		return Capability{
			Available: false,
			Reason:    fmt.Sprintf("Unexpected response: HTTP %d", resp.StatusCode),
		}
	}
}

// checkAdminOperation verifies if an admin operation is available
func (cd *CapabilityDiscoverer) checkAdminOperation(ctx context.Context, provider *ArtifactoryProvider, opID string) Capability {
	// Try to access system configuration to check admin rights
	resp, err := provider.ExecuteHTTPRequest(ctx, "GET", "/api/system/configuration", nil, nil)
	if err != nil {
		// Check if this is an authentication error
		errStr := err.Error()
		if strings.Contains(errStr, "no credentials") || strings.Contains(errStr, "authentication") {
			// Can't determine without auth, assume potentially available
			return Capability{
				Available: true,
				Reason:    "Admin permission check requires authentication",
				Required:  []string{"Admin permission", "Authentication"},
			}
		}
		return Capability{
			Available: false,
			Reason:    "Failed to verify admin permissions",
			Required:  []string{"Admin permission"},
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusForbidden {
		return Capability{
			Available: false,
			Reason:    "Admin permissions required",
			Required:  []string{"Admin permission"},
		}
	}

	if resp.StatusCode == http.StatusOK {
		return Capability{Available: true}
	}

	return Capability{
		Available: false,
		Reason:    "Unable to verify admin permissions",
		Required:  []string{"Admin permission"},
	}
}

// checkOperationPermission checks if a specific operation is permitted
func (cd *CapabilityDiscoverer) checkOperationPermission(ctx context.Context, provider *ArtifactoryProvider, opID string, opMapping providers.OperationMapping) Capability {
	// For write operations, check if user has write permissions
	if strings.Contains(opID, "create") || strings.Contains(opID, "update") || strings.Contains(opID, "delete") {
		// Check if we can access permissions endpoint
		resp, err := provider.ExecuteHTTPRequest(ctx, "GET", "/api/v2/security/permissions", nil, nil)
		if err != nil {
			// If we can't check permissions, assume limited access
			return Capability{
				Available: true, // Optimistically available
				Reason:    "Permission check unavailable, operation may fail if insufficient permissions",
			}
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusForbidden {
			return Capability{
				Available: false,
				Reason:    "Write permissions may be required",
				Required:  []string{"Write permission for target resource"},
			}
		}
	}

	// Default: assume available for read operations
	return Capability{Available: true}
}

// checkPackageTypeSupport determines which package types are supported
func (cd *CapabilityDiscoverer) checkPackageTypeSupport(ctx context.Context, provider *ArtifactoryProvider, report *CapabilityReport) {
	packageTypes := []string{
		"maven", "gradle", "ivy", "sbt",
		"npm", "bower", "yarn",
		"nuget",
		"gems",
		"pypi", "conda",
		"docker", "helm",
		"go", "cargo",
		"conan",
		"debian", "rpm",
		"vagrant", "gitlfs",
		"generic",
	}

	// Try to list repositories to see what package types are in use
	resp, err := provider.Execute(ctx, "repos/list", map[string]interface{}{})
	if err != nil {
		// Check if this is an authentication issue
		errStr := err.Error()
		defaultReason := "Package type availability could not be verified"
		if strings.Contains(errStr, "no credentials") || strings.Contains(errStr, "authentication") {
			defaultReason = "Package type not currently in use but may be available"
		}

		// If we can't list repos, assume all package types are potentially available
		for _, pt := range packageTypes {
			report.Features[fmt.Sprintf("package_%s", pt)] = Capability{
				Available: true,
				Reason:    defaultReason,
			}
		}
		return
	}

	// Track which package types we've seen
	seenTypes := make(map[string]bool)
	if repos, ok := resp.([]interface{}); ok {
		for _, repo := range repos {
			if repoMap, ok := repo.(map[string]interface{}); ok {
				if packageType, ok := repoMap["packageType"].(string); ok {
					seenTypes[strings.ToLower(packageType)] = true
				}
			}
		}
	}

	// Mark package types based on what we've seen
	for _, pt := range packageTypes {
		if seenTypes[pt] {
			report.Features[fmt.Sprintf("package_%s", pt)] = Capability{
				Available: true,
			}
		} else {
			report.Features[fmt.Sprintf("package_%s", pt)] = Capability{
				Available: true, // Still potentially available, just not in use
				Reason:    "Package type not currently in use but may be available",
			}
		}
	}
}

// countAvailable counts how many capabilities are available
func (cd *CapabilityDiscoverer) countAvailable(caps map[string]Capability) int {
	count := 0
	for _, cap := range caps {
		if cap.Available {
			count++
		}
	}
	return count
}

// GetCachedReport returns the cached capability report if valid
func (cd *CapabilityDiscoverer) GetCachedReport() *CapabilityReport {
	cd.cacheMutex.RLock()
	defer cd.cacheMutex.RUnlock()

	if cd.cache != nil && time.Since(cd.lastDiscovery) < cd.cacheDuration {
		cached := *cd.cache
		cached.CacheValid = true
		return &cached
	}
	return nil
}

// InvalidateCache forces the next discovery to refresh
func (cd *CapabilityDiscoverer) InvalidateCache() {
	cd.cacheMutex.Lock()
	defer cd.cacheMutex.Unlock()
	cd.cache = nil
	cd.lastDiscovery = time.Time{}
}

// FormatCapabilityError returns a structured error for unavailable operations
func FormatCapabilityError(operation string, capability Capability) map[string]interface{} {
	resolution := "Contact your administrator to enable this feature"

	// Provide specific resolution based on the reason
	if strings.Contains(capability.Reason, "license") {
		resolution = "Upgrade your JFrog license to access this feature"
	} else if strings.Contains(capability.Reason, "permission") {
		resolution = "Request appropriate permissions from your administrator"
	} else if strings.Contains(capability.Reason, "not installed") {
		resolution = "Install and configure the required JFrog component"
	} else if strings.Contains(capability.Reason, "cloud-only") {
		resolution = "This feature is only available in JFrog Cloud"
	}

	return map[string]interface{}{
		"error":      "operation_unavailable",
		"operation":  operation,
		"reason":     capability.Reason,
		"required":   capability.Required,
		"resolution": resolution,
	}
}
