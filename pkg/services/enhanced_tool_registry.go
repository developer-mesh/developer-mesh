package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// EnhancedToolRegistry manages both standard tool providers and dynamic tools
type EnhancedToolRegistry struct {
	// Providers
	providerRegistry *providers.Registry

	// Repositories
	db              *sqlx.DB
	templateRepo    repository.ToolTemplateRepository
	orgToolRepo     repository.OrganizationToolRepository
	dynamicToolRepo repository.DynamicToolRepository

	// Services
	encryptionService *security.EncryptionService
	operationCache    *tools.OperationCache

	// Internal state
	mu            sync.RWMutex
	toolNameCache map[string]map[string]string // tenant_id -> tool_name -> tool_id
	logger        observability.Logger
}

// getMapKeys returns the keys of a map as a slice
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// NewEnhancedToolRegistry creates a new enhanced tool registry
func NewEnhancedToolRegistry(
	db *sqlx.DB,
	encryptionService *security.EncryptionService,
	operationCache *tools.OperationCache,
	logger observability.Logger,
) *EnhancedToolRegistry {
	return &EnhancedToolRegistry{
		providerRegistry:  providers.NewRegistry(logger),
		db:                db,
		templateRepo:      repository.NewToolTemplateRepository(db),
		orgToolRepo:       repository.NewOrganizationToolRepository(db),
		dynamicToolRepo:   repository.NewDynamicToolRepository(db),
		encryptionService: encryptionService,
		operationCache:    operationCache,
		toolNameCache:     make(map[string]map[string]string),
		logger:            logger,
	}
}

// RegisterProvider registers a standard tool provider
func (r *EnhancedToolRegistry) RegisterProvider(provider providers.StandardToolProvider) error {
	// Register with provider registry
	if err := r.providerRegistry.RegisterProvider(provider); err != nil {
		return fmt.Errorf("failed to register provider: %w", err)
	}

	// Auto-create or update template from provider
	template := r.createTemplateFromProvider(provider)
	if err := r.templateRepo.Upsert(context.Background(), template); err != nil {
		r.logger.Warn("Failed to create template from provider", map[string]interface{}{
			"provider": provider.GetProviderName(),
			"error":    err.Error(),
		})
		// Don't fail registration if template creation fails
	}

	r.logger.Info("Registered standard tool provider", map[string]interface{}{
		"provider": provider.GetProviderName(),
		"versions": provider.GetSupportedVersions(),
		"tools":    len(provider.GetToolDefinitions()),
	})

	return nil
}

// InstantiateToolForOrg creates a tool instance for an organization
func (r *EnhancedToolRegistry) InstantiateToolForOrg(
	ctx context.Context,
	orgID string,
	tenantID string,
	templateName string,
	instanceName string,
	credentials map[string]string,
	config map[string]interface{},
) (*models.OrganizationTool, error) {
	// Get the template
	template, err := r.templateRepo.GetByProviderName(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("template %s not found: %w", templateName, err)
	}

	// Get the provider if available
	provider, err := r.providerRegistry.GetProvider(templateName)
	if err == nil {
		// Validate credentials with provider
		if err := provider.ValidateCredentials(ctx, credentials); err != nil {
			return nil, fmt.Errorf("invalid credentials: %w", err)
		}
	}

	// Encrypt credentials with the tenant ID
	encryptedCreds, err := r.encryptCredentials(credentials, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Create organization tool instance
	// Ensure config includes provider information
	if config == nil {
		config = make(map[string]interface{})
	}
	config["provider"] = templateName // Store the provider name for later expansion
	if _, exists := config["base_url"]; !exists {
		config["base_url"] = "https://api.github.com" // Default base URL
	}

	// Use display_name from config if provided, otherwise generate one
	displayName := template.DisplayName
	if configDisplayName, ok := config["display_name"].(string); ok && configDisplayName != "" {
		displayName = configDisplayName
		// Remove from config since we're storing it as a field
		delete(config, "display_name")
	}

	orgTool := &models.OrganizationTool{
		ID:                   uuid.New().String(),
		OrganizationID:       orgID,
		TenantID:             tenantID,
		TemplateID:           template.ID,
		InstanceName:         instanceName,
		DisplayName:          displayName,
		InstanceConfig:       config,
		CredentialsEncrypted: encryptedCreds,
		Status:               "provisioning",
		IsActive:             true, // Set to active by default
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Save to database
	if err := r.orgToolRepo.Create(ctx, orgTool); err != nil {
		return nil, fmt.Errorf("failed to create organization tool: %w", err)
	}

	// Update status to active after successful creation
	orgTool.Status = "active"
	orgTool.IsActive = true // Ensure IsActive is set in returned object
	if err := r.orgToolRepo.UpdateStatus(ctx, orgTool.ID, "active"); err != nil {
		r.logger.Warn("Failed to update tool status", map[string]interface{}{
			"tool_id": orgTool.ID,
			"error":   err.Error(),
		})
	}

	// Clear cache for this tenant
	r.clearTenantCache(tenantID)

	return orgTool, nil
}

// GetToolsForTenant returns all tools available for a tenant (both dynamic and templated)
func (r *EnhancedToolRegistry) GetToolsForTenant(ctx context.Context, tenantID string) ([]interface{}, error) {
	var tools []interface{}

	// Get organization tools (from templates)
	orgTools, err := r.orgToolRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		r.logger.Warn("Failed to get organization tools", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
	} else {
		for _, ot := range orgTools {
			tools = append(tools, ot)
		}
	}

	// Get dynamic tools
	dynamicTools, err := r.dynamicToolRepo.List(ctx, tenantID, "active")
	if err != nil {
		r.logger.Warn("Failed to get dynamic tools", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
	} else {
		for _, dt := range dynamicTools {
			tools = append(tools, dt)
		}
	}

	return tools, nil
}

// ExecuteTool executes a tool operation
func (r *EnhancedToolRegistry) ExecuteTool(
	ctx context.Context,
	tenantID string,
	toolID string,
	operation string,
	params map[string]interface{},
) (interface{}, error) {
	// Call the passthrough version with nil auth
	return r.ExecuteToolWithPassthrough(ctx, tenantID, toolID, operation, params, nil)
}

// ExecuteToolWithPassthrough executes a tool operation with optional passthrough authentication
func (r *EnhancedToolRegistry) ExecuteToolWithPassthrough(
	ctx context.Context,
	tenantID string,
	toolID string,
	operation string,
	params map[string]interface{},
	passthroughAuth *models.PassthroughAuthBundle,
) (interface{}, error) {
	actualToolID := toolID

	// First, check if this is an expanded organization tool
	// These are dynamic tools with a parent_tool_id in their config
	dynamicTool, err := r.dynamicToolRepo.GetByID(ctx, toolID)
	if err == nil && dynamicTool != nil {
		// Check if this is an expanded organization tool
		if dynamicTool.Config != nil {
			if parentID, ok := dynamicTool.Config["parent_tool_id"].(string); ok && parentID != "" {
				// This is an expanded organization tool
				actualToolID = parentID

				// Extract the operation from config if not already provided
				if operation == "" || operation == "execute" {
					if op, ok := dynamicTool.Config["operation"].(string); ok && op != "" {
						operation = op
					}
				}

				r.logger.Debug("Expanded organization tool detected", map[string]interface{}{
					"tool_id":        toolID,
					"parent_tool_id": actualToolID,
					"operation":      operation,
					"type":           dynamicTool.Config["type"],
				})

				// Get the parent organization tool
				orgTool, err := r.orgToolRepo.GetByID(ctx, actualToolID)
				if err == nil && orgTool != nil {
					// Pass along the expanded tool's credentials if the org tool doesn't have any
					if len(orgTool.CredentialsEncrypted) == 0 && len(dynamicTool.CredentialsEncrypted) > 0 {
						orgTool.CredentialsEncrypted = dynamicTool.CredentialsEncrypted
					}
					return r.executeOrganizationTool(ctx, orgTool, operation, params, passthroughAuth)
				}
			}
		}

		// Not an expanded tool, execute as regular dynamic tool
		return r.executeDynamicTool(ctx, dynamicTool, operation, params, passthroughAuth)
	}

	// Fallback to the old logic for backward compatibility
	// Only process if we haven't already extracted the operation
	if operation == "" || operation == "execute" {
		// Check if this is an old-style expanded tool ID (format: parent_id_operation)
		if strings.Contains(toolID, "_") {
			parts := strings.SplitN(toolID, "_", 2)
			if len(parts) == 2 {
				// This is an expanded tool, use the parent ID for lookup
				actualToolID = parts[0]
				operation = parts[1]
				r.logger.Debug("Extracted operation from tool ID (legacy)", map[string]interface{}{
					"original_tool_id": toolID,
					"parent_tool_id":   actualToolID,
					"operation":        operation,
				})
			}
		}
	}

	// Check if it's an organization tool
	orgTool, err := r.orgToolRepo.GetByID(ctx, actualToolID)
	if err == nil && orgTool != nil {
		return r.executeOrganizationTool(ctx, orgTool, operation, params, passthroughAuth)
	}

	// Check if it's a dynamic tool (that wasn't found earlier)
	if actualToolID != toolID {
		dynamicTool, err = r.dynamicToolRepo.GetByID(ctx, actualToolID)
		if err == nil && dynamicTool != nil {
			return r.executeDynamicTool(ctx, dynamicTool, operation, params, passthroughAuth)
		}
	}

	return nil, fmt.Errorf("tool %s not found", actualToolID)
}

// executeOrganizationTool executes an operation on an organization tool
func (r *EnhancedToolRegistry) executeOrganizationTool(
	ctx context.Context,
	orgTool *models.OrganizationTool,
	operation string,
	params map[string]interface{},
	passthroughAuth *models.PassthroughAuthBundle,
) (interface{}, error) {
	// Get the template
	template, err := r.templateRepo.GetByID(ctx, orgTool.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Get the provider
	provider, err := r.providerRegistry.GetProvider(template.ProviderName)
	if err != nil {
		return nil, fmt.Errorf("provider %s not found: %w", template.ProviderName, err)
	}

	// Determine credentials to use
	var credentials map[string]string
	var usingPassthrough bool

	// Check if we have passthrough auth for this provider
	if passthroughAuth != nil && passthroughAuth.Credentials != nil {
		// Check if we have credentials for this provider
		providerCred, hasProviderCred := passthroughAuth.Credentials[template.ProviderName]

		// If not found by provider name, check for wildcard/default key
		if !hasProviderCred || providerCred == nil {
			// Check for wildcard key "*"
			providerCred, hasProviderCred = passthroughAuth.Credentials["*"]
			if hasProviderCred && providerCred != nil {
				r.logger.Debug("Using wildcard passthrough credentials", map[string]interface{}{
					"provider": template.ProviderName,
				})
			}
		}

		// Also check for the specific provider key variants
		if !hasProviderCred || providerCred == nil {
			// Try lowercase variant
			providerCred, hasProviderCred = passthroughAuth.Credentials[strings.ToLower(template.ProviderName)]
		}

		if hasProviderCred && providerCred != nil {
			// Use passthrough credentials
			// Map the token to the appropriate credential key based on auth type
			credentials = map[string]string{}
			switch providerCred.Type {
			case "api_key":
				credentials["api_key"] = providerCred.Token
			case "bearer":
				credentials["token"] = providerCred.Token
			default:
				// Default to token for compatibility
				credentials["token"] = providerCred.Token
			}
			usingPassthrough = true
			r.logger.Info("Using passthrough authentication", map[string]interface{}{
				"tool_id":   orgTool.ID,
				"provider":  template.ProviderName,
				"has_token": providerCred.Token != "",
				"token_len": len(providerCred.Token),
				"auth_type": providerCred.Type,
			})
		} else {
			// Log what keys are available for debugging
			var availableKeys []string
			for k := range passthroughAuth.Credentials {
				availableKeys = append(availableKeys, k)
			}
			r.logger.Warn("No passthrough credentials found for provider", map[string]interface{}{
				"provider":       template.ProviderName,
				"available_keys": availableKeys,
			})
		}
	}

	// If no passthrough auth or not applicable, decrypt stored credentials
	if !usingPassthrough {
		// Check if we have stored credentials
		if len(orgTool.CredentialsEncrypted) == 0 {
			// Check template's default_config metadata for passthrough mode
			var passthroughMode string
			if template.DefaultConfig.Metadata != nil {
				if pc, ok := template.DefaultConfig.Metadata["passthrough"].(map[string]interface{}); ok {
					passthroughMode, _ = pc["mode"].(string)
				}
			}

			// If passthrough is optional or required, and we don't have creds, that's an error
			if passthroughMode == "required" || (passthroughMode == "optional" && passthroughAuth == nil) {
				return nil, fmt.Errorf("no credentials available for tool %s (passthrough mode: %s)", orgTool.ID, passthroughMode)
			}
			return nil, fmt.Errorf("no credentials available for tool %s", orgTool.ID)
		}

		credentials, err = r.decryptCredentials(orgTool.CredentialsEncrypted, orgTool.TenantID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}

		// Log decrypted credentials for debugging (without exposing the actual values)
		r.logger.Info("Using stored credentials", map[string]interface{}{
			"tool_id":    orgTool.ID,
			"has_token":  credentials["token"] != "",
			"has_apikey": credentials["api_key"] != "",
			"token_len":  len(credentials["token"]),
			"apikey_len": len(credentials["api_key"]),
			"cred_keys":  getMapKeys(credentials),
			"tenant_id":  orgTool.TenantID,
		})
	}

	// Create provider context
	pctx := &providers.ProviderContext{
		TenantID:       orgTool.TenantID,
		OrganizationID: orgTool.OrganizationID,
		Credentials: &providers.ProviderCredentials{
			Token:  credentials["token"],
			APIKey: credentials["api_key"],
			Email:  credentials["email"],
			Custom: credentials,
		},
	}
	ctx = providers.WithContext(ctx, pctx)

	// Execute operation
	startTime := time.Now()
	result, err := provider.ExecuteOperation(ctx, operation, params)
	duration := time.Since(startTime)

	// Track usage
	go r.trackToolUsage(orgTool.ID, operation, err == nil, int(duration.Milliseconds()))

	if err != nil {
		return nil, fmt.Errorf("failed to execute operation: %w", err)
	}

	return result, nil
}

// executeDynamicTool executes an operation on a dynamic tool
func (r *EnhancedToolRegistry) executeDynamicTool(
	ctx context.Context,
	tool *models.DynamicTool,
	operation string,
	params map[string]interface{},
	passthroughAuth *models.PassthroughAuthBundle,
) (interface{}, error) {
	// This would use the existing dynamic tool execution logic
	// For now, return a placeholder
	return nil, fmt.Errorf("dynamic tool execution not yet implemented in enhanced registry")
}

// createTemplateFromProvider creates a tool template from a provider
func (r *EnhancedToolRegistry) createTemplateFromProvider(provider providers.StandardToolProvider) *models.ToolTemplate {
	config := provider.GetDefaultConfiguration()

	// Convert provider definitions to template format
	operationGroups := []models.OperationGroup{}
	for _, group := range config.OperationGroups {
		operationGroups = append(operationGroups, models.OperationGroup{
			Name:        group.Name,
			DisplayName: group.DisplayName,
			Description: group.Description,
			Operations:  group.Operations,
		})
	}

	// Get AI-optimized definitions
	var aiDefsPtr *json.RawMessage
	aiDefs := provider.GetAIOptimizedDefinitions()
	if len(aiDefs) > 0 {
		aiDefsJSON, _ := json.Marshal(aiDefs)
		aiDefsPtr = (*json.RawMessage)(&aiDefsJSON)
	}

	// Get first tool definition for description and category
	var description, category string
	toolDefs := provider.GetToolDefinitions()
	if len(toolDefs) > 0 {
		description = toolDefs[0].Description
		category = toolDefs[0].Category
	}

	return &models.ToolTemplate{
		ID:                  uuid.New().String(),
		ProviderName:        provider.GetProviderName(),
		ProviderVersion:     provider.GetSupportedVersions()[0],
		DisplayName:         provider.GetProviderName(),
		Description:         description,
		Category:            category,
		DefaultConfig:       config,
		OperationGroups:     operationGroups,
		OperationMappings:   provider.GetOperationMappings(),
		AIDefinitions:       aiDefsPtr,
		RequiredCredentials: []string{"token"},            // Most providers need at least a token
		Features:            providers.ProviderFeatures{}, // Empty features for now
		IsPublic:            true,
		IsActive:            true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// encryptCredentials encrypts credentials
func (r *EnhancedToolRegistry) encryptCredentials(credentials map[string]string, tenantID string) ([]byte, error) {
	// Serialize credentials to JSON for encryption
	credJSON, err := json.Marshal(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize credentials: %w", err)
	}

	return r.encryptionService.EncryptCredential(string(credJSON), tenantID)
}

// decryptCredentials decrypts credentials
func (r *EnhancedToolRegistry) decryptCredentials(encrypted []byte, tenantID string) (map[string]string, error) {
	decrypted, err := r.encryptionService.DecryptCredential(encrypted, tenantID)
	if err != nil {
		return nil, err
	}

	// Deserialize credentials from JSON
	var credentials map[string]string
	if err := json.Unmarshal([]byte(decrypted), &credentials); err != nil {
		// Fallback: if it's not JSON, assume it's a single token value (for backwards compatibility)
		credentials = map[string]string{
			"token":   decrypted,
			"api_key": decrypted, // Also set as api_key for compatibility
		}
	}

	return credentials, nil
}

// clearTenantCache clears the tool name cache for a tenant
func (r *EnhancedToolRegistry) clearTenantCache(tenantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.toolNameCache, tenantID)
}

// RefreshOrganizationTool refreshes an organization tool with the latest provider capabilities
func (r *EnhancedToolRegistry) RefreshOrganizationTool(ctx context.Context, orgID, toolID string) error {
	// Get the organization tool
	orgTool, err := r.orgToolRepo.GetByID(ctx, toolID)
	if err != nil {
		return fmt.Errorf("failed to get organization tool: %w", err)
	}

	// Verify it belongs to the organization
	if orgTool.OrganizationID != orgID {
		return fmt.Errorf("tool does not belong to organization")
	}

	// Get the tool template
	template, err := r.templateRepo.GetByID(ctx, orgTool.TemplateID)
	if err != nil {
		return fmt.Errorf("failed to get tool template: %w", err)
	}

	// Get the provider from registry
	provider, err := r.providerRegistry.GetProvider(template.ProviderName)
	if err != nil || provider == nil {
		return fmt.Errorf("provider %s not found: %w", template.ProviderName, err)
	}

	// Create an updated template from the current provider state
	updatedTemplate := r.createTemplateFromProvider(provider)

	// Preserve the template ID and other metadata
	updatedTemplate.ID = template.ID
	updatedTemplate.CreatedAt = template.CreatedAt
	updatedTemplate.CreatedBy = template.CreatedBy

	// Update the template in the database
	if err := r.templateRepo.Update(ctx, updatedTemplate); err != nil {
		return fmt.Errorf("failed to update tool template: %w", err)
	}

	// Update the organization tool's updated timestamp
	orgTool.UpdatedAt = time.Now()
	if err := r.orgToolRepo.Update(ctx, orgTool); err != nil {
		return fmt.Errorf("failed to update organization tool: %w", err)
	}

	// Clear cache for the tenant
	r.clearTenantCache(orgTool.TenantID)

	r.logger.Info("Organization tool refreshed", map[string]interface{}{
		"org_id":          orgID,
		"tool_id":         toolID,
		"provider":        template.ProviderName,
		"operation_count": len(provider.GetAIOptimizedDefinitions()),
	})

	return nil
}

// trackToolUsage tracks tool usage for analytics
func (r *EnhancedToolRegistry) trackToolUsage(toolID string, operation string, success bool, responseTimeMs int) {
	ctx := context.Background()

	query := `SELECT mcp.track_tool_usage($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query, toolID, operation, success, responseTimeMs)
	if err != nil {
		r.logger.Warn("Failed to track tool usage", map[string]interface{}{
			"tool_id":   toolID,
			"operation": operation,
			"error":     err.Error(),
		})
	}
}

// GetProviderRegistry returns the provider registry for direct access
func (r *EnhancedToolRegistry) GetProviderRegistry() *providers.Registry {
	return r.providerRegistry
}

// MigrateFromDynamic migrates a dynamic tool to a template-based tool if possible
func (r *EnhancedToolRegistry) MigrateFromDynamic(
	ctx context.Context,
	dynamicToolID string,
) (*models.OrganizationTool, error) {
	// Get the dynamic tool
	dynamicTool, err := r.dynamicToolRepo.GetByID(ctx, dynamicToolID)
	if err != nil {
		return nil, fmt.Errorf("dynamic tool not found: %w", err)
	}

	// Check if we have a provider for this URL
	provider, found := r.providerRegistry.GetProviderForURL(dynamicTool.BaseURL)
	if !found {
		return nil, fmt.Errorf("no provider found for URL %s", dynamicTool.BaseURL)
	}

	// Create organization tool from dynamic tool
	orgTool := &models.OrganizationTool{
		ID:             uuid.New().String(),
		OrganizationID: dynamicTool.TenantID, // Use tenant as org for now
		TenantID:       dynamicTool.TenantID,
		InstanceName:   dynamicTool.ToolName,
		DisplayName:    dynamicTool.DisplayName,
		InstanceConfig: dynamicTool.Config,
		Status:         "active",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Get template for provider
	template, err := r.templateRepo.GetByProviderName(ctx, provider.GetProviderName())
	if err != nil {
		return nil, fmt.Errorf("template not found for provider: %w", err)
	}
	orgTool.TemplateID = template.ID

	// Migrate credentials
	if dynamicTool.CredentialsEncrypted != nil {
		orgTool.CredentialsEncrypted = dynamicTool.CredentialsEncrypted
	}

	// Save the organization tool
	if err := r.orgToolRepo.Create(ctx, orgTool); err != nil {
		return nil, fmt.Errorf("failed to create organization tool: %w", err)
	}

	// Optionally mark the dynamic tool as migrated
	if err := r.dynamicToolRepo.UpdateStatus(ctx, dynamicToolID, "migrated"); err != nil {
		r.logger.Warn("Failed to update dynamic tool status", map[string]interface{}{
			"tool_id": dynamicToolID,
			"error":   err.Error(),
		})
	}

	return orgTool, nil
}
