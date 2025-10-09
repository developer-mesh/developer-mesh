package mcp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
)

// ServiceCapability represents the capabilities of a service or tool provider
type ServiceCapability struct {
	ServiceName  string                 `json:"service_name"`
	DisplayName  string                 `json:"display_name"`
	Description  string                 `json:"description"`
	Provider     string                 `json:"provider,omitempty"`    // e.g., "github", "harness", "builtin"
	APIVersion   string                 `json:"api_version,omitempty"` // API version supported
	Category     string                 `json:"category,omitempty"`    // Primary category
	Operations   []OperationCapability  `json:"operations"`            // Available operations/tools
	Features     []string               `json:"features,omitempty"`    // Service-level features
	AuthRequired bool                   `json:"auth_required"`         // Whether authentication is required
	AuthTypes    []string               `json:"auth_types,omitempty"`  // Supported authentication types
	Limitations  map[string]interface{} `json:"limitations,omitempty"` // Service limitations (rate limits, etc.)
	Metadata     map[string]interface{} `json:"metadata,omitempty"`    // Additional metadata
}

// OperationCapability represents the capabilities of a specific operation/tool
type OperationCapability struct {
	Name           string                 `json:"name"`
	DisplayName    string                 `json:"display_name,omitempty"`
	Description    string                 `json:"description"`
	Method         string                 `json:"method,omitempty"`          // HTTP method (GET, POST, etc.) if applicable
	Path           string                 `json:"path,omitempty"`            // API path if applicable
	Category       string                 `json:"category,omitempty"`        // Tool category
	Tags           []string               `json:"tags,omitempty"`            // Capability tags (read, write, etc.)
	Permissions    []Permission           `json:"permissions,omitempty"`     // Required permissions
	Features       []string               `json:"features,omitempty"`        // Operation-specific features
	InputSchema    map[string]interface{} `json:"input_schema,omitempty"`    // JSON schema for inputs
	RequiredParams []string               `json:"required_params,omitempty"` // Required parameters
	OptionalParams []string               `json:"optional_params,omitempty"` // Optional parameters
	Relationships  ToolRelationships      `json:"relationships,omitempty"`   // Tool relationships
	Limitations    map[string]interface{} `json:"limitations,omitempty"`     // Operation limitations
	Examples       []OperationExample     `json:"examples,omitempty"`        // Usage examples
}

// Permission represents a required permission for an operation
type Permission struct {
	Type        string   `json:"type"`             // e.g., "oauth_scope", "api_key", "role"
	Name        string   `json:"name"`             // Permission identifier
	Description string   `json:"description"`      // Human-readable description
	Required    bool     `json:"required"`         // Whether this permission is required
	Scopes      []string `json:"scopes,omitempty"` // OAuth scopes if applicable
	Roles       []string `json:"roles,omitempty"`  // Required roles if applicable
}

// ToolRelationships represents relationships between tools
type ToolRelationships struct {
	Prerequisites    []string `json:"prerequisites,omitempty"`      // Tools that must be executed before
	CommonlyUsedWith []string `json:"commonly_used_with,omitempty"` // Tools frequently used together
	NextSteps        []string `json:"next_steps,omitempty"`         // Recommended follow-up tools
	Alternatives     []string `json:"alternatives,omitempty"`       // Alternative tools
	ConflictsWith    []string `json:"conflicts_with,omitempty"`     // Incompatible tools
}

// OperationExample represents a usage example for an operation
type OperationExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output,omitempty"`
}

// CapabilityManager manages service and operation capabilities
type CapabilityManager struct {
	registry            *tools.Registry
	serviceCapabilities map[string]*ServiceCapability
	mu                  sync.RWMutex
}

// NewCapabilityManager creates a new capability manager
func NewCapabilityManager(registry *tools.Registry) *CapabilityManager {
	manager := &CapabilityManager{
		registry:            registry,
		serviceCapabilities: make(map[string]*ServiceCapability),
	}

	// Initialize with built-in services
	manager.initializeBuiltinCapabilities()

	return manager
}

// initializeBuiltinCapabilities initializes capabilities for built-in services
func (cm *CapabilityManager) initializeBuiltinCapabilities() {
	// These capabilities are static and don't change
	builtinServices := []ServiceCapability{
		{
			ServiceName:  "agent",
			DisplayName:  "Agent Management",
			Description:  "AI agent lifecycle management and orchestration",
			Provider:     "builtin",
			Category:     "agent",
			AuthRequired: true,
			AuthTypes:    []string{"api_key"},
			Features: []string{
				"agent_registration",
				"heartbeat_monitoring",
				"status_tracking",
				"agent_listing",
			},
		},
		{
			ServiceName:  "workflow",
			DisplayName:  "Workflow Management",
			Description:  "Workflow creation, execution, and management",
			Provider:     "builtin",
			Category:     "workflow",
			AuthRequired: true,
			AuthTypes:    []string{"api_key"},
			Features: []string{
				"workflow_creation",
				"workflow_execution",
				"workflow_cancellation",
				"execution_tracking",
				"template_support",
			},
		},
		{
			ServiceName:  "task",
			DisplayName:  "Task Management",
			Description:  "Task creation, assignment, and tracking",
			Provider:     "builtin",
			Category:     "task",
			AuthRequired: true,
			AuthTypes:    []string{"api_key"},
			Features: []string{
				"task_creation",
				"task_assignment",
				"task_completion",
				"batch_operations",
				"priority_management",
			},
		},
		{
			ServiceName:  "context",
			DisplayName:  "Context Management",
			Description:  "Session context management and state tracking",
			Provider:     "builtin",
			Category:     "context",
			AuthRequired: true,
			AuthTypes:    []string{"api_key"},
			Features: []string{
				"context_storage",
				"context_retrieval",
				"context_update",
				"context_append",
				"session_management",
			},
		},
	}

	for _, svc := range builtinServices {
		// Make a copy to avoid pointer issues
		service := svc
		cm.serviceCapabilities[service.ServiceName] = &service
	}
}

// RefreshFromRegistry refreshes capabilities from the tool registry
func (cm *CapabilityManager) RefreshFromRegistry() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get all tools from registry
	allTools := cm.registry.ListAll()

	// Group tools by service (extract from tool name prefix)
	serviceTools := make(map[string][]tools.ToolDefinition)

	for _, tool := range allTools {
		serviceName := extractServiceName(tool.Name)
		serviceTools[serviceName] = append(serviceTools[serviceName], tool)
	}

	// Build or update service capabilities
	for serviceName, toolsList := range serviceTools {
		// Skip built-in services as they're already initialized
		if _, exists := cm.serviceCapabilities[serviceName]; exists && isBuiltinService(serviceName) {
			// Update operations for built-in services
			cm.updateBuiltinServiceOperations(serviceName, toolsList)
			continue
		}

		// Create new service capability for dynamic tools
		service := cm.buildServiceCapability(serviceName, toolsList)
		cm.serviceCapabilities[serviceName] = service
	}
}

// extractServiceName extracts the service name from a tool name
// Examples: "github_repos_list" -> "github", "harness_pipelines_get" -> "harness"
func extractServiceName(toolName string) string {
	// Handle built-in tools
	if strings.HasPrefix(toolName, "agent_") {
		return "agent"
	}
	if strings.HasPrefix(toolName, "workflow_") {
		return "workflow"
	}
	if strings.HasPrefix(toolName, "task_") {
		return "task"
	}
	if strings.HasPrefix(toolName, "context_") {
		return "context"
	}
	if strings.HasPrefix(toolName, "template_") {
		return "template"
	}

	// For dynamic tools, extract prefix before first underscore
	parts := strings.Split(toolName, "_")
	if len(parts) > 0 {
		return parts[0]
	}

	return "unknown"
}

// isBuiltinService checks if a service is built-in
func isBuiltinService(serviceName string) bool {
	builtinServices := map[string]bool{
		"agent":    true,
		"workflow": true,
		"task":     true,
		"context":  true,
		"template": true,
	}
	return builtinServices[serviceName]
}

// updateBuiltinServiceOperations updates operations for a built-in service
func (cm *CapabilityManager) updateBuiltinServiceOperations(serviceName string, toolsList []tools.ToolDefinition) {
	service := cm.serviceCapabilities[serviceName]
	if service == nil {
		return
	}

	// Build operations from tools
	operations := make([]OperationCapability, 0, len(toolsList))
	for _, tool := range toolsList {
		op := cm.buildOperationCapability(tool)
		operations = append(operations, op)
	}

	service.Operations = operations
}

// buildServiceCapability builds a service capability from a list of tools
func (cm *CapabilityManager) buildServiceCapability(serviceName string, toolsList []tools.ToolDefinition) *ServiceCapability {
	if len(toolsList) == 0 {
		return nil
	}

	// Determine service-level properties from tools
	features := cm.extractServiceFeatures(toolsList)
	authTypes := cm.extractAuthTypes(serviceName)
	apiVersion := cm.extractAPIVersion(serviceName)

	// Get primary category from first tool
	category := ""
	if len(toolsList) > 0 {
		category = toolsList[0].Category
	}

	// Build operations
	operations := make([]OperationCapability, 0, len(toolsList))
	for _, tool := range toolsList {
		op := cm.buildOperationCapability(tool)
		operations = append(operations, op)
	}

	return &ServiceCapability{
		ServiceName:  serviceName,
		DisplayName:  formatDisplayName(serviceName),
		Description:  fmt.Sprintf("%s integration via dynamic tools", formatDisplayName(serviceName)),
		Provider:     serviceName,
		APIVersion:   apiVersion,
		Category:     category,
		Operations:   operations,
		Features:     features,
		AuthRequired: true,
		AuthTypes:    authTypes,
		Metadata: map[string]interface{}{
			"tool_count": len(toolsList),
		},
	}
}

// buildOperationCapability builds an operation capability from a tool definition
func (cm *CapabilityManager) buildOperationCapability(tool tools.ToolDefinition) OperationCapability {
	// Extract permissions from tool metadata
	permissions := cm.extractPermissions(tool)

	// Extract features from tags
	features := cm.extractFeatures(tool)

	// Extract parameters from input schema
	required, optional := cm.extractParameters(tool.InputSchema)

	// Build relationships
	relationships := ToolRelationships{
		Prerequisites:    tool.Prerequisites,
		CommonlyUsedWith: tool.CommonlyUsedWith,
		NextSteps:        tool.NextSteps,
		Alternatives:     tool.Alternatives,
		ConflictsWith:    tool.ConflictsWith,
	}

	return OperationCapability{
		Name:           tool.Name,
		Description:    tool.Description,
		Category:       tool.Category,
		Tags:           tool.Tags,
		Permissions:    permissions,
		Features:       features,
		InputSchema:    tool.InputSchema,
		RequiredParams: required,
		OptionalParams: optional,
		Relationships:  relationships,
	}
}

// extractServiceFeatures extracts service-level features from tools
func (cm *CapabilityManager) extractServiceFeatures(toolsList []tools.ToolDefinition) []string {
	featureMap := make(map[string]bool)

	for _, tool := range toolsList {
		for _, tag := range tool.Tags {
			// Convert tags to feature names
			switch tag {
			case "read":
				featureMap["read_operations"] = true
			case "write":
				featureMap["write_operations"] = true
			case "delete":
				featureMap["delete_operations"] = true
			case "batch":
				featureMap["batch_operations"] = true
			case "async":
				featureMap["async_operations"] = true
			case "webhook":
				featureMap["webhook_support"] = true
			case "streaming":
				featureMap["streaming_support"] = true
			}
		}
	}

	features := make([]string, 0, len(featureMap))
	for feature := range featureMap {
		features = append(features, feature)
	}

	return features
}

// extractAuthTypes determines supported authentication types for a service
func (cm *CapabilityManager) extractAuthTypes(serviceName string) []string {
	// This would typically come from the dynamic tool configuration
	// For now, provide defaults based on service
	authTypes := []string{"api_key"}

	switch serviceName {
	case "github":
		authTypes = append(authTypes, "oauth", "personal_access_token")
	case "harness":
		authTypes = append(authTypes, "bearer_token", "service_account")
	}

	return authTypes
}

// extractAPIVersion determines the API version for a service
func (cm *CapabilityManager) extractAPIVersion(serviceName string) string {
	// This would typically come from the OpenAPI spec
	// For now, provide defaults
	versions := map[string]string{
		"github":  "v3/2022-11-28",
		"harness": "v1",
	}

	if version, ok := versions[serviceName]; ok {
		return version
	}

	return "v1"
}

// extractPermissions extracts required permissions from a tool
func (cm *CapabilityManager) extractPermissions(tool tools.ToolDefinition) []Permission {
	permissions := []Permission{}

	// Determine permissions based on tags and category
	isWrite := contains(tool.Tags, "write") || contains(tool.Tags, "delete")
	isRead := contains(tool.Tags, "read")

	// Add base permission based on operation type
	if isWrite {
		permissions = append(permissions, Permission{
			Type:        "api_scope",
			Name:        fmt.Sprintf("%s:write", tool.Category),
			Description: fmt.Sprintf("Write access to %s resources", tool.Category),
			Required:    true,
		})
	} else if isRead {
		permissions = append(permissions, Permission{
			Type:        "api_scope",
			Name:        fmt.Sprintf("%s:read", tool.Category),
			Description: fmt.Sprintf("Read access to %s resources", tool.Category),
			Required:    true,
		})
	}

	// Add admin permission for delete operations
	if contains(tool.Tags, "delete") {
		permissions = append(permissions, Permission{
			Type:        "api_scope",
			Name:        fmt.Sprintf("%s:admin", tool.Category),
			Description: fmt.Sprintf("Admin access to %s resources", tool.Category),
			Required:    true,
		})
	}

	return permissions
}

// extractFeatures extracts feature flags from a tool
func (cm *CapabilityManager) extractFeatures(tool tools.ToolDefinition) []string {
	features := []string{}

	// Map tags to features
	tagFeatureMap := map[string]string{
		"async":      "supports_async",
		"batch":      "supports_batching",
		"streaming":  "supports_streaming",
		"webhook":    "supports_webhooks",
		"cache":      "supports_caching",
		"retry":      "supports_retry",
		"idempotent": "is_idempotent",
	}

	for _, tag := range tool.Tags {
		if feature, ok := tagFeatureMap[tag]; ok {
			features = append(features, feature)
		}
	}

	// Add passthrough auth support as a feature
	features = append(features, "supports_passthrough_auth")

	return features
}

// extractParameters extracts required and optional parameters from input schema
func (cm *CapabilityManager) extractParameters(schema map[string]interface{}) ([]string, []string) {
	required := []string{}
	optional := []string{}

	if schema == nil {
		return required, optional
	}

	// Extract from JSON schema
	if requiredList, ok := schema["required"].([]interface{}); ok {
		for _, r := range requiredList {
			if str, ok := r.(string); ok {
				required = append(required, str)
			}
		}
	}

	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for propName := range properties {
			if !contains(required, propName) {
				optional = append(optional, propName)
			}
		}
	}

	return required, optional
}

// GetAllServiceCapabilities returns all service capabilities
func (cm *CapabilityManager) GetAllServiceCapabilities() []ServiceCapability {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Refresh before returning
	cm.mu.RUnlock()
	cm.RefreshFromRegistry()
	cm.mu.RLock()

	capabilities := make([]ServiceCapability, 0, len(cm.serviceCapabilities))
	for _, svc := range cm.serviceCapabilities {
		if svc != nil {
			capabilities = append(capabilities, *svc)
		}
	}

	return capabilities
}

// GetServiceCapability returns capability for a specific service
func (cm *CapabilityManager) GetServiceCapability(serviceName string) (*ServiceCapability, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Refresh before returning
	cm.mu.RUnlock()
	cm.RefreshFromRegistry()
	cm.mu.RLock()

	svc, exists := cm.serviceCapabilities[serviceName]
	if !exists || svc == nil {
		return nil, false
	}

	return svc, true
}

// GetToolCapability returns capability for a specific tool
func (cm *CapabilityManager) GetToolCapability(toolName string) (*OperationCapability, bool) {
	// Get the tool from registry
	tool, exists := cm.registry.Get(toolName)
	if !exists {
		return nil, false
	}

	// Build operation capability
	op := cm.buildOperationCapability(tool)
	return &op, true
}

// GetRequiredPermissions returns required permissions for a tool
func (cm *CapabilityManager) GetRequiredPermissions(toolName string) ([]Permission, error) {
	op, exists := cm.GetToolCapability(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	return op.Permissions, nil
}

// GetToolFeatures returns feature flags for a tool
func (cm *CapabilityManager) GetToolFeatures(toolName string) ([]string, error) {
	op, exists := cm.GetToolCapability(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	return op.Features, nil
}

// formatDisplayName formats a service name for display
func formatDisplayName(serviceName string) string {
	// Capitalize first letter
	if len(serviceName) == 0 {
		return serviceName
	}

	return strings.ToUpper(serviceName[:1]) + serviceName[1:]
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
