package tools

import (
	"context"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPIHandler defines the interface for the generic OpenAPI handler
type OpenAPIHandler interface {
	// DiscoverAPIs discovers available APIs and OpenAPI specifications
	DiscoverAPIs(ctx context.Context, config ToolConfig) (*DiscoveryResult, error)

	// GenerateTools generates tool definitions from OpenAPI specification
	GenerateTools(config ToolConfig, spec *openapi3.T) ([]*Tool, error)

	// AuthenticateRequest adds authentication to HTTP requests based on OpenAPI security schemes
	AuthenticateRequest(req *http.Request, creds *models.TokenCredential, securitySchemes map[string]SecurityScheme) error

	// TestConnection tests the connection to the tool
	TestConnection(ctx context.Context, config ToolConfig) error

	// ExtractSecuritySchemes extracts security schemes from OpenAPI spec
	ExtractSecuritySchemes(spec *openapi3.T) map[string]SecurityScheme
}

// OpenAPIMetadata contains metadata about the OpenAPI handler
type OpenAPIMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// ConfigField defines a configuration field schema
type ConfigField struct {
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description"`
	Validation  string      `json:"validation,omitempty"`
	Options     []string    `json:"options,omitempty"`
}

// ToolConfig represents a tool configuration
type ToolConfig struct {
	ID                string                  `json:"id"`
	TenantID          string                  `json:"tenant_id"`
	Name              string                  `json:"name"`
	BaseURL           string                  `json:"base_url"`
	DocumentationURL  string                  `json:"documentation_url,omitempty"`
	OpenAPIURL        string                  `json:"openapi_url,omitempty"`
	Config            map[string]interface{}  `json:"config"`
	Credential        *models.TokenCredential `json:"-"` // Never serialize credentials
	RetryPolicy       *ToolRetryPolicy        `json:"retry_policy,omitempty"`
	HealthConfig      *HealthCheckConfig      `json:"health_config,omitempty"`
	Provider          string                  `json:"provider,omitempty"`
	PassthroughConfig *PassthroughConfig      `json:"passthrough_config,omitempty"`
}

// PassthroughConfig defines how user token passthrough should be handled
type PassthroughConfig struct {
	Mode              string `json:"mode"`                // optional, required, disabled
	FallbackToService bool   `json:"fallback_to_service"` // Allow fallback to service account
}

// ToolRetryPolicy extends the base retry policy with tool-specific settings
type ToolRetryPolicy struct {
	resilience.RetryPolicy
	RetryableErrors  []string `json:"retryable_errors"`
	RetryOnTimeout   bool     `json:"retry_on_timeout"`
	RetryOnRateLimit bool     `json:"retry_on_rate_limit"`
}

// HealthCheckConfig defines health check configuration
type HealthCheckConfig struct {
	Mode           string        `json:"mode"` // "on_demand", "periodic", "disabled"
	CacheDuration  time.Duration `json:"cache_duration"`
	StaleThreshold time.Duration `json:"stale_threshold"`
	CheckTimeout   time.Duration `json:"check_timeout"`
	HealthEndpoint string        `json:"health_endpoint,omitempty"`
}

// HealthStatus represents the health status of a tool
type HealthStatus struct {
	IsHealthy    bool                   `json:"is_healthy"`
	LastChecked  time.Time              `json:"last_checked"`
	ResponseTime int                    `json:"response_time_ms"`
	Error        string                 `json:"error,omitempty"`
	Version      string                 `json:"version,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// DiscoveryResult contains the results of API discovery
type DiscoveryResult struct {
	Status           DiscoveryStatus        `json:"status"`
	OpenAPISpec      *openapi3.T            `json:"-"` // Don't serialize the full spec
	SpecURL          string                 `json:"spec_url,omitempty"`
	DiscoveredURLs   []string               `json:"discovered_urls"`
	Capabilities     []Capability           `json:"capabilities"`
	RequiresManual   bool                   `json:"requires_manual"`
	SuggestedActions []string               `json:"suggested_actions,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// DiscoveryStatus represents the status of API discovery
type DiscoveryStatus string

const (
	DiscoveryStatusSuccess      DiscoveryStatus = "success"
	DiscoveryStatusPartial      DiscoveryStatus = "partial"
	DiscoveryStatusFailed       DiscoveryStatus = "failed"
	DiscoveryStatusManualNeeded DiscoveryStatus = "manual_needed"
)

// Capability represents a tool capability
type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
	Required    bool     `json:"required"`
}

// Common capabilities
var (
	CapabilityIssueManagement = Capability{
		Name:        "issue_management",
		Description: "Create, update, and manage issues",
		Actions:     []string{"create_issue", "update_issue", "close_issue", "list_issues"},
	}

	CapabilityPullRequests = Capability{
		Name:        "pull_requests",
		Description: "Manage pull/merge requests",
		Actions:     []string{"create_pr", "update_pr", "merge_pr", "list_prs"},
	}

	CapabilityPipelines = Capability{
		Name:        "pipelines",
		Description: "Manage CI/CD pipelines",
		Actions:     []string{"trigger_pipeline", "stop_pipeline", "get_pipeline_status"},
	}

	CapabilityCodeQuality = Capability{
		Name:        "code_quality",
		Description: "Code quality analysis and reporting",
		Actions:     []string{"trigger_analysis", "get_quality_metrics", "get_issues"},
	}

	CapabilityArtifacts = Capability{
		Name:        "artifacts",
		Description: "Artifact management",
		Actions:     []string{"upload_artifact", "download_artifact", "search_artifacts"},
	}

	CapabilitySecurityScanning = Capability{
		Name:        "security_scanning",
		Description: "Security vulnerability scanning",
		Actions:     []string{"scan_artifact", "get_vulnerabilities", "get_licenses"},
	}

	CapabilityMonitoring = Capability{
		Name:        "monitoring",
		Description: "Application and infrastructure monitoring",
		Actions:     []string{"get_metrics", "create_event", "get_problems"},
	}
)

// ToolExecution represents a tool action execution
type ToolExecution struct {
	ID            string                 `json:"id"`
	ToolConfigID  string                 `json:"tool_config_id"`
	TenantID      string                 `json:"tenant_id"`
	Action        string                 `json:"action"`
	Parameters    map[string]interface{} `json:"parameters"`
	Status        ExecutionStatus        `json:"status"`
	Result        interface{}            `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	RetryCount    int                    `json:"retry_count"`
	ResponseTime  int                    `json:"response_time_ms"`
	ExecutedAt    time.Time              `json:"executed_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	ExecutedBy    string                 `json:"executed_by"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
}

// ExecutionStatus represents the status of a tool execution
type ExecutionStatus string

const (
	ExecutionStatusPending  ExecutionStatus = "pending"
	ExecutionStatusRunning  ExecutionStatus = "running"
	ExecutionStatusSuccess  ExecutionStatus = "success"
	ExecutionStatusFailed   ExecutionStatus = "failed"
	ExecutionStatusTimeout  ExecutionStatus = "timeout"
	ExecutionStatusRetrying ExecutionStatus = "retrying"
)
