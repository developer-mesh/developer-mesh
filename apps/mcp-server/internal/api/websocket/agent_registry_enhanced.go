package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	agentRepo "github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// EnhancedAgentRegistry extends DBAgentRegistry with universal agent support
type EnhancedAgentRegistry struct {
	*DBAgentRegistry // Embed existing registry to reuse all its methods

	// Additional repositories for enhanced functionality
	manifestRepo repository.AgentManifestRepository
	orgRepo      repository.OrganizationRepository
	db           *sqlx.DB // Database connection for direct queries

	// Enhanced tracking
	manifestCache   sync.Map // manifest_id -> AgentManifest
	registrationMap sync.Map // instance_id -> registration_id
	capabilityIndex sync.Map // capability -> []agent_id
	channelMap      sync.Map // agent_id -> []channel_id
}

// NewEnhancedAgentRegistry creates an enhanced registry that extends DBAgentRegistry
func NewEnhancedAgentRegistry(
	repo agentRepo.Repository,
	cache cache.Cache,
	logger observability.Logger,
	metrics observability.MetricsClient,
	manifestRepo repository.AgentManifestRepository,
	orgRepo repository.OrganizationRepository,
) *EnhancedAgentRegistry {
	// Create base registry
	baseRegistry := NewDBAgentRegistry(repo, cache, logger, metrics)

	return &EnhancedAgentRegistry{
		DBAgentRegistry: baseRegistry,
		manifestRepo:    manifestRepo,
		orgRepo:         orgRepo,
	}
}

// RegisterUniversalAgent registers any type of agent (IDE, Slack, monitoring, etc.)
func (ear *EnhancedAgentRegistry) RegisterUniversalAgent(ctx context.Context, reg *UniversalAgentRegistration) (*UniversalAgentInfo, error) {
	// First check if manifest exists
	manifest, err := ear.manifestRepo.GetManifestByAgentID(ctx, reg.AgentID)
	if err != nil {
		// Create new manifest if it doesn't exist
		manifest = &models.AgentManifest{
			ID:             uuid.New(),
			OrganizationID: reg.OrganizationID,
			AgentID:        reg.AgentID,
			AgentType:      reg.AgentType,
			Name:           reg.Name,
			Version:        reg.Version,
			Description:    reg.Description,
			Capabilities:   reg.Capabilities,
			Requirements:   reg.Requirements,
			Status:         models.ManifestStatusActive,
		}

		if err := ear.manifestRepo.CreateManifest(ctx, manifest); err != nil {
			return nil, fmt.Errorf("failed to create agent manifest: %w", err)
		}

		ear.logger.Info("Created new agent manifest", map[string]interface{}{
			"agent_id":   reg.AgentID,
			"agent_type": reg.AgentType,
			"org_id":     reg.OrganizationID,
		})
	}

	// Cache the manifest
	ear.manifestCache.Store(manifest.ID, manifest)

	// Declare registration variable at the beginning
	var registration *models.AgentRegistration

	// Use the new idempotent registration function if database is available
	if ear.db != nil {
		// Call the database function for idempotent registration
		var result struct {
			RegistrationID uuid.UUID `db:"registration_id"`
			ManifestID     uuid.UUID `db:"manifest_id"`
			ConfigID       uuid.UUID `db:"config_id"`
			IsNew          bool      `db:"is_new"`
			Message        string    `db:"message"`
		}

		query := `SELECT * FROM mcp.register_agent_instance($1, $2, $3, $4, $5, $6)`
		err := ear.db.GetContext(ctx, &result, query,
			reg.TenantID,
			reg.AgentID,
			reg.InstanceID,
			reg.Name,
			reg.ConnectionDetails,
			reg.RuntimeConfig,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to register agent instance: %w", err)
		}

		ear.logger.Info("Agent registered via database function", map[string]interface{}{
			"registration_id": result.RegistrationID,
			"config_id":       result.ConfigID,
			"manifest_id":     result.ManifestID,
			"instance_id":     reg.InstanceID,
			"is_new":          result.IsNew,
			"message":         result.Message,
		})

		// Create a registration object for compatibility
		registration = &models.AgentRegistration{
			ID:                 result.RegistrationID,
			ManifestID:         result.ManifestID,
			TenantID:           reg.TenantID,
			InstanceID:         reg.InstanceID,
			RegistrationToken:  reg.Token,
			RegistrationStatus: models.RegistrationStatusActive,
			RuntimeConfig:      reg.RuntimeConfig,
			ConnectionDetails:  reg.ConnectionDetails,
			HealthStatus:       models.RegistrationHealthHealthy,
			HealthCheckURL:     reg.HealthCheckURL,
		}
		now := time.Now()
		registration.ActivationDate = &now

		// Store in cache
		ear.registrationMap.Store(reg.InstanceID, registration.ID)
	} else {
		// Fallback: create registration without database
		registration = &models.AgentRegistration{
			ID:                 uuid.New(),
			ManifestID:         manifest.ID,
			TenantID:           reg.TenantID,
			InstanceID:         reg.InstanceID,
			RegistrationToken:  reg.Token,
			RegistrationStatus: models.RegistrationStatusActive,
			RuntimeConfig:      reg.RuntimeConfig,
			ConnectionDetails:  reg.ConnectionDetails,
			HealthStatus:       models.RegistrationHealthHealthy,
			HealthCheckURL:     reg.HealthCheckURL,
		}
		now := time.Now()
		registration.ActivationDate = &now

		// Try to create via repository if available
		if ear.manifestRepo != nil {
			if err := ear.manifestRepo.CreateRegistration(ctx, registration); err != nil {
				// Log but continue - the registration exists in memory
				ear.logger.Warn("Failed to persist registration", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}

		// Store in cache
		ear.registrationMap.Store(reg.InstanceID, registration.ID)
	}

	// Track registration
	ear.registrationMap.Store(reg.InstanceID, registration.ID)

	// Index capabilities for fast lookup
	if caps, ok := manifest.Capabilities["capabilities"].([]interface{}); ok {
		for _, cap := range caps {
			if capStr, ok := cap.(string); ok {
				ear.indexCapability(capStr, manifest.AgentID)
			}
		}
	}

	// Create channels if specified
	for _, channelConfig := range reg.Channels {
		channel := &models.AgentChannel{
			ID:             uuid.New(),
			RegistrationID: registration.ID,
			ChannelType:    channelConfig.Type,
			ChannelConfig:  channelConfig.Config,
			Priority:       channelConfig.Priority,
			Active:         true,
		}

		if err := ear.manifestRepo.CreateChannel(ctx, channel); err != nil {
			ear.logger.Warn("Failed to create channel", map[string]interface{}{
				"channel_type": channelConfig.Type,
				"error":        err.Error(),
			})
		} else {
			ear.trackChannel(manifest.AgentID, channel.ID)
		}
	}

	// No need for legacy registration - the view handles backward compatibility
	baseInfo := &AgentInfo{
		ID:           manifest.AgentID,
		Name:         reg.Name,
		TenantID:     reg.TenantID.String(),
		Capabilities: convertCapabilities(manifest.Capabilities),
		Status:       "active",
	}

	// Build enhanced response
	info := &UniversalAgentInfo{
		ManifestID:     manifest.ID,
		RegistrationID: registration.ID,
		AgentID:        manifest.AgentID,
		AgentType:      manifest.AgentType,
		Name:           manifest.Name,
		Version:        manifest.Version,
		Status:         registration.RegistrationStatus,
		HealthStatus:   registration.HealthStatus,
		Capabilities:   manifest.Capabilities,
		Channels:       ear.getAgentChannels(manifest.AgentID),
		RegisteredAt:   registration.CreatedAt,
		LastSeen:       time.Now(),
	}

	// Copy base info (always exists since we just created it)
	info.ActiveTasks = baseInfo.ActiveTasks
	info.ConnectionID = baseInfo.ConnectionID

	ear.metrics.IncrementCounter("universal_agents_registered", 1)
	ear.metrics.IncrementCounter(fmt.Sprintf("agents_type_%s", reg.AgentType), 1)

	return info, nil
}

// DiscoverUniversalAgents finds agents by capability across all types
func (ear *EnhancedAgentRegistry) DiscoverUniversalAgents(ctx context.Context, filter *UniversalAgentFilter) ([]*UniversalAgentInfo, error) {
	var results []*UniversalAgentInfo

	// If specific capabilities requested, use capability index
	if len(filter.RequiredCapabilities) > 0 {
		agentIDs := ear.findAgentsByCapabilities(filter.RequiredCapabilities)
		for _, agentID := range agentIDs {
			info, err := ear.GetUniversalAgentInfo(ctx, agentID)
			if err != nil {
				continue
			}

			// Apply additional filters
			if filter.AgentType != "" && info.AgentType != filter.AgentType {
				continue
			}

			if filter.OrganizationID != uuid.Nil {
				manifest, _ := ear.getManifestByAgentID(ctx, agentID)
				if manifest == nil || manifest.OrganizationID != filter.OrganizationID {
					continue
				}
			}

			if filter.OnlyHealthy && info.HealthStatus != models.RegistrationHealthHealthy {
				continue
			}

			results = append(results, info)
		}
	} else {
		// Get all agents and filter
		var manifests []models.AgentManifest

		if filter.OrganizationID != uuid.Nil {
			manifests, _ = ear.manifestRepo.ListManifestsByOrg(ctx, filter.OrganizationID)
		} else if filter.AgentType != "" {
			manifests, _ = ear.manifestRepo.ListManifestsByType(ctx, filter.AgentType)
		}

		for _, manifest := range manifests {
			info, err := ear.GetUniversalAgentInfo(ctx, manifest.AgentID)
			if err != nil {
				continue
			}

			if filter.OnlyHealthy && info.HealthStatus != models.RegistrationHealthHealthy {
				continue
			}

			results = append(results, info)
		}
	}

	// Also check base registry for backward compatibility
	if filter.TenantID != uuid.Nil {
		baseAgents, err := ear.DBAgentRegistry.DiscoverAgents( //nolint:staticcheck // Explicit call to embedded registry
			ctx,
			filter.TenantID.String(),
			filter.RequiredCapabilities,
			filter.ExcludeSelf,
			filter.SelfID,
		)

		if err == nil {
			for _, baseAgent := range baseAgents {
				// Convert and add if not already in results
				if !ear.containsAgent(results, baseAgent["id"].(string)) {
					info := ear.convertBaseToUniversal(baseAgent)
					results = append(results, info)
				}
			}
		}
	}

	return results, nil
}

// GetUniversalAgentInfo retrieves comprehensive agent information
func (ear *EnhancedAgentRegistry) GetUniversalAgentInfo(ctx context.Context, agentID string) (*UniversalAgentInfo, error) {
	manifest, err := ear.getManifestByAgentID(ctx, agentID)
	if err != nil {
		// Try base registry
		baseInfo, baseErr := ear.DBAgentRegistry.GetAgentStatus(ctx, agentID) //nolint:staticcheck // Explicit call to embedded registry
		if baseErr != nil {
			return nil, fmt.Errorf("agent not found in either registry: %w", baseErr)
		}

		// Convert base info to universal format
		return ear.convertBaseInfoToUniversal(baseInfo), nil
	}

	// Get registration info
	registrations, err := ear.manifestRepo.ListRegistrationsByManifest(ctx, manifest.ID)
	if err != nil || len(registrations) == 0 {
		return nil, fmt.Errorf("no active registrations for agent %s", agentID)
	}

	// Use most recent registration
	registration := registrations[0]

	// Get channels
	channels, _ := ear.manifestRepo.ListChannelsByRegistration(ctx, registration.ID)

	info := &UniversalAgentInfo{
		ManifestID:     manifest.ID,
		RegistrationID: registration.ID,
		AgentID:        manifest.AgentID,
		AgentType:      manifest.AgentType,
		Name:           manifest.Name,
		Version:        manifest.Version,
		Status:         registration.RegistrationStatus,
		HealthStatus:   registration.HealthStatus,
		Capabilities:   manifest.Capabilities,
		Channels:       ear.convertChannels(channels),
		RegisteredAt:   registration.CreatedAt,
		LastSeen:       time.Now(),
	}

	// Get task count from base registry if available
	if baseInfo, err := ear.DBAgentRegistry.GetAgentStatus(ctx, agentID); err == nil { //nolint:staticcheck // Explicit call to embedded registry
		info.ActiveTasks = baseInfo.ActiveTasks
		info.ConnectionID = baseInfo.ConnectionID
	}

	return info, nil
}

// UpdateAgentHealth updates health status for a universal agent
func (ear *EnhancedAgentRegistry) UpdateAgentHealth(ctx context.Context, agentID string, health *AgentHealthUpdate) error {
	// Update in manifest repository
	registrations, err := ear.getRegistrationsByAgentID(ctx, agentID)
	if err == nil && len(registrations) > 0 {
		for _, reg := range registrations {
			if err := ear.manifestRepo.UpdateRegistrationHealth(ctx, reg.ID, health.Status); err != nil {
				ear.logger.Warn("Failed to update registration health", map[string]interface{}{
					"registration_id": reg.ID,
					"error":           err.Error(),
				})
			}
		}
	}

	// Also update in base registry
	metadata := map[string]interface{}{
		"health_status":     health.Status,
		"health_message":    health.Message,
		"health_updated_at": time.Now(),
	}

	if health.Metrics != nil {
		metadata["health_metrics"] = health.Metrics
	}

	return ear.DBAgentRegistry.UpdateAgentStatus(ctx, agentID, health.Status, metadata) //nolint:staticcheck // Explicit call to embedded registry
}

// Helper methods

func (ear *EnhancedAgentRegistry) indexCapability(capability, agentID string) {
	val, _ := ear.capabilityIndex.LoadOrStore(capability, []string{})
	agents := val.([]string)

	// Check if already indexed
	for _, id := range agents {
		if id == agentID {
			return
		}
	}

	agents = append(agents, agentID)
	ear.capabilityIndex.Store(capability, agents)
}

func (ear *EnhancedAgentRegistry) trackChannel(agentID string, channelID uuid.UUID) {
	val, _ := ear.channelMap.LoadOrStore(agentID, []uuid.UUID{})
	channels := val.([]uuid.UUID)
	channels = append(channels, channelID)
	ear.channelMap.Store(agentID, channels)
}

func (ear *EnhancedAgentRegistry) getAgentChannels(agentID string) []ChannelInfo {
	val, ok := ear.channelMap.Load(agentID)
	if !ok {
		return nil
	}

	channelIDs := val.([]uuid.UUID)
	var channels []ChannelInfo

	for _, channelID := range channelIDs {
		// Get channel details from manifest repo
		ctx := context.Background()
		channel, err := ear.manifestRepo.GetChannelByID(ctx, channelID)
		if err != nil {
			continue
		}

		channels = append(channels, ChannelInfo{
			ID:       channel.ID,
			Type:     channel.ChannelType,
			Priority: channel.Priority,
			Active:   channel.Active,
		})
	}

	return channels
}

func (ear *EnhancedAgentRegistry) findAgentsByCapabilities(capabilities []string) []string {
	agentMap := make(map[string]int)

	for _, cap := range capabilities {
		val, ok := ear.capabilityIndex.Load(cap)
		if !ok {
			continue
		}

		agents := val.([]string)
		for _, agentID := range agents {
			agentMap[agentID]++
		}
	}

	// Return agents that have all required capabilities
	var results []string
	for agentID, count := range agentMap {
		if count == len(capabilities) {
			results = append(results, agentID)
		}
	}

	return results
}

func (ear *EnhancedAgentRegistry) getManifestByAgentID(ctx context.Context, agentID string) (*models.AgentManifest, error) {
	// Check cache first
	var cachedManifest *models.AgentManifest
	ear.manifestCache.Range(func(key, value interface{}) bool {
		manifest := value.(*models.AgentManifest)
		if manifest.AgentID == agentID {
			cachedManifest = manifest
			return false
		}
		return true
	})

	if cachedManifest != nil {
		return cachedManifest, nil
	}

	// Get from repository
	return ear.manifestRepo.GetManifestByAgentID(ctx, agentID)
}

func (ear *EnhancedAgentRegistry) getRegistrationsByAgentID(ctx context.Context, agentID string) ([]models.AgentRegistration, error) {
	manifest, err := ear.getManifestByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return ear.manifestRepo.ListRegistrationsByManifest(ctx, manifest.ID)
}

func (ear *EnhancedAgentRegistry) containsAgent(agents []*UniversalAgentInfo, agentID string) bool {
	for _, agent := range agents {
		if agent.AgentID == agentID {
			return true
		}
	}
	return false
}

func (ear *EnhancedAgentRegistry) convertBaseToUniversal(baseAgent map[string]interface{}) *UniversalAgentInfo {
	info := &UniversalAgentInfo{
		AgentID:      baseAgent["id"].(string),
		Name:         baseAgent["name"].(string),
		AgentType:    AgentTypeStandard,
		Status:       baseAgent["status"].(string),
		HealthStatus: baseAgent["health"].(string),
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	if caps, ok := baseAgent["capabilities"].([]string); ok {
		info.Capabilities = map[string]interface{}{
			"capabilities": caps,
		}
	}

	if tasks, ok := baseAgent["active_tasks"].(int); ok {
		info.ActiveTasks = tasks
	}

	return info
}

func (ear *EnhancedAgentRegistry) convertBaseInfoToUniversal(baseInfo *AgentInfo) *UniversalAgentInfo {
	return &UniversalAgentInfo{
		AgentID:      baseInfo.ID,
		Name:         baseInfo.Name,
		AgentType:    AgentTypeStandard,
		Status:       baseInfo.Status,
		HealthStatus: baseInfo.Health,
		Capabilities: map[string]interface{}{
			"capabilities": baseInfo.Capabilities,
		},
		ConnectionID: baseInfo.ConnectionID,
		ActiveTasks:  baseInfo.ActiveTasks,
		RegisteredAt: baseInfo.RegisteredAt,
		LastSeen:     baseInfo.LastSeen,
	}
}

func (ear *EnhancedAgentRegistry) convertChannels(channels []models.AgentChannel) []ChannelInfo {
	var result []ChannelInfo
	for _, channel := range channels {
		result = append(result, ChannelInfo{
			ID:       channel.ID,
			Type:     channel.ChannelType,
			Priority: channel.Priority,
			Active:   channel.Active,
		})
	}
	return result
}

func convertCapabilities(capsMap models.JSONMap) []string {
	if capsMap == nil {
		return nil
	}

	if caps, ok := capsMap["capabilities"].([]interface{}); ok {
		result := make([]string, 0, len(caps))
		for _, cap := range caps {
			if capStr, ok := cap.(string); ok {
				result = append(result, capStr)
			}
		}
		return result
	}

	return nil
}

// UniversalAgentRegistration contains registration data for any agent type
type UniversalAgentRegistration struct {
	OrganizationID    uuid.UUID
	TenantID          uuid.UUID
	AgentID           string
	AgentType         string
	InstanceID        string
	Name              string
	Version           string
	Description       string
	Token             string
	Capabilities      models.JSONMap
	Requirements      models.JSONMap
	RuntimeConfig     models.JSONMap
	ConnectionDetails models.JSONMap
	Metadata          map[string]interface{}
	ConnectionID      string
	HealthCheckURL    string
	Channels          []ChannelConfig
}

// UniversalAgentInfo contains comprehensive agent information
type UniversalAgentInfo struct {
	ManifestID     uuid.UUID
	RegistrationID uuid.UUID
	AgentID        string
	AgentType      string
	Name           string
	Version        string
	Status         string
	HealthStatus   string
	Capabilities   models.JSONMap
	Channels       []ChannelInfo
	ConnectionID   string
	ActiveTasks    int
	RegisteredAt   time.Time
	LastSeen       time.Time
}

// UniversalAgentFilter for discovering agents
type UniversalAgentFilter struct {
	OrganizationID       uuid.UUID
	TenantID             uuid.UUID
	AgentType            string
	RequiredCapabilities []string
	OnlyHealthy          bool
	ExcludeSelf          bool
	SelfID               string
}

// ChannelConfig for agent communication channels
type ChannelConfig struct {
	Type     string
	Config   models.JSONMap
	Priority int
}

// ChannelInfo contains channel information
type ChannelInfo struct {
	ID       uuid.UUID
	Type     string
	Priority int
	Active   bool
}

// AgentHealthUpdate contains health update information
type AgentHealthUpdate struct {
	Status  string
	Message string
	Metrics map[string]interface{}
}
