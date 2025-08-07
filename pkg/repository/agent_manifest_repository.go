package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// AgentManifestRepository defines the interface for agent manifest operations
type AgentManifestRepository interface {
	// Manifest operations
	CreateManifest(ctx context.Context, manifest *models.AgentManifest) error
	GetManifestByID(ctx context.Context, id uuid.UUID) (*models.AgentManifest, error)
	GetManifestByAgentID(ctx context.Context, agentID string) (*models.AgentManifest, error)
	ListManifestsByOrg(ctx context.Context, orgID uuid.UUID) ([]models.AgentManifest, error)
	ListManifestsByType(ctx context.Context, agentType string) ([]models.AgentManifest, error)
	UpdateManifest(ctx context.Context, manifest *models.AgentManifest) error
	DeleteManifest(ctx context.Context, id uuid.UUID) error
	UpdateManifestHeartbeat(ctx context.Context, id uuid.UUID) error

	// Registration operations
	CreateRegistration(ctx context.Context, registration *models.AgentRegistration) error
	GetRegistrationByID(ctx context.Context, id uuid.UUID) (*models.AgentRegistration, error)
	GetRegistrationByInstance(ctx context.Context, tenantID uuid.UUID, instanceID string) (*models.AgentRegistration, error)
	ListRegistrationsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.AgentRegistration, error)
	ListRegistrationsByManifest(ctx context.Context, manifestID uuid.UUID) ([]models.AgentRegistration, error)
	UpdateRegistration(ctx context.Context, registration *models.AgentRegistration) error
	UpdateRegistrationHealth(ctx context.Context, id uuid.UUID, status string) error
	DeleteRegistration(ctx context.Context, id uuid.UUID) error

	// Capability operations
	AddCapability(ctx context.Context, capability *models.ManifestCapability) error
	RemoveCapability(ctx context.Context, id uuid.UUID) error
	ListCapabilitiesByManifest(ctx context.Context, manifestID uuid.UUID) ([]models.ManifestCapability, error)
	ListManifestsByCapability(ctx context.Context, capabilityType, capabilityName string) ([]models.AgentManifest, error)

	// Channel operations
	CreateChannel(ctx context.Context, channel *models.AgentChannel) error
	GetChannelByID(ctx context.Context, id uuid.UUID) (*models.AgentChannel, error)
	ListChannelsByRegistration(ctx context.Context, registrationID uuid.UUID) ([]models.AgentChannel, error)
	UpdateChannel(ctx context.Context, channel *models.AgentChannel) error
	DeleteChannel(ctx context.Context, id uuid.UUID) error
}

// agentManifestRepository implements AgentManifestRepository
type agentManifestRepository struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewAgentManifestRepository creates a new agent manifest repository
func NewAgentManifestRepository(db *sqlx.DB, logger observability.Logger) AgentManifestRepository {
	return &agentManifestRepository{
		db:     db,
		logger: logger,
	}
}

// CreateManifest creates a new agent manifest
func (r *agentManifestRepository) CreateManifest(ctx context.Context, manifest *models.AgentManifest) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.CreateManifest")
	defer span.End()

	if manifest.ID == uuid.Nil {
		manifest.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.agent_manifests (
			id, organization_id, agent_id, agent_type, name, version, description,
			capabilities, requirements, connection_config, auth_config, metadata, 
			status, created_by, updated_by
		) VALUES (
			:id, :organization_id, :agent_id, :agent_type, :name, :version, :description,
			:capabilities, :requirements, :connection_config, :auth_config, :metadata,
			:status, :created_by, :updated_by
		)`

	_, err := r.db.NamedExecContext(ctx, query, manifest)
	if err != nil {
		r.logger.Error("Failed to create agent manifest", map[string]interface{}{
			"agent_id": manifest.AgentID,
			"error":    err.Error(),
		})
		return fmt.Errorf("failed to create agent manifest: %w", err)
	}

	return nil
}

// GetManifestByID retrieves an agent manifest by ID
func (r *agentManifestRepository) GetManifestByID(ctx context.Context, id uuid.UUID) (*models.AgentManifest, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.GetManifestByID")
	defer span.End()

	query := `
		SELECT id, organization_id, agent_id, agent_type, name, version, description,
			capabilities, requirements, connection_config, auth_config, metadata,
			status, last_heartbeat, created_by, updated_by, created_at, updated_at
		FROM mcp.agent_manifests
		WHERE id = $1`

	var manifest models.AgentManifest
	err := r.db.GetContext(ctx, &manifest, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get agent manifest", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to get agent manifest: %w", err)
	}

	return &manifest, nil
}

// GetManifestByAgentID retrieves an agent manifest by agent ID
func (r *agentManifestRepository) GetManifestByAgentID(ctx context.Context, agentID string) (*models.AgentManifest, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.GetManifestByAgentID")
	defer span.End()

	query := `
		SELECT id, organization_id, agent_id, agent_type, name, version, description,
			capabilities, requirements, connection_config, auth_config, metadata,
			status, last_heartbeat, created_by, updated_by, created_at, updated_at
		FROM mcp.agent_manifests
		WHERE agent_id = $1`

	var manifest models.AgentManifest
	err := r.db.GetContext(ctx, &manifest, query, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get agent manifest by agent ID", map[string]interface{}{
			"agent_id": agentID,
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("failed to get agent manifest by agent ID: %w", err)
	}

	return &manifest, nil
}

// ListManifestsByOrg lists all agent manifests for an organization
func (r *agentManifestRepository) ListManifestsByOrg(ctx context.Context, orgID uuid.UUID) ([]models.AgentManifest, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListManifestsByOrg")
	defer span.End()

	query := `
		SELECT id, organization_id, agent_id, agent_type, name, version, description,
			capabilities, requirements, connection_config, auth_config, metadata,
			status, last_heartbeat, created_by, updated_by, created_at, updated_at
		FROM mcp.agent_manifests
		WHERE organization_id = $1
		ORDER BY created_at DESC`

	var manifests []models.AgentManifest
	err := r.db.SelectContext(ctx, &manifests, query, orgID)
	if err != nil {
		r.logger.Error("Failed to list agent manifests by org", map[string]interface{}{
			"org_id": orgID,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to list agent manifests by org: %w", err)
	}

	return manifests, nil
}

// ListManifestsByType lists all agent manifests by type
func (r *agentManifestRepository) ListManifestsByType(ctx context.Context, agentType string) ([]models.AgentManifest, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListManifestsByType")
	defer span.End()

	query := `
		SELECT id, organization_id, agent_id, agent_type, name, version, description,
			capabilities, requirements, connection_config, auth_config, metadata,
			status, last_heartbeat, created_by, updated_by, created_at, updated_at
		FROM mcp.agent_manifests
		WHERE agent_type = $1
		ORDER BY created_at DESC`

	var manifests []models.AgentManifest
	err := r.db.SelectContext(ctx, &manifests, query, agentType)
	if err != nil {
		r.logger.Error("Failed to list agent manifests by type", map[string]interface{}{
			"agent_type": agentType,
			"error":      err.Error(),
		})
		return nil, fmt.Errorf("failed to list agent manifests by type: %w", err)
	}

	return manifests, nil
}

// UpdateManifest updates an agent manifest
func (r *agentManifestRepository) UpdateManifest(ctx context.Context, manifest *models.AgentManifest) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.UpdateManifest")
	defer span.End()

	query := `
		UPDATE mcp.agent_manifests
		SET name = :name, version = :version, description = :description,
			capabilities = :capabilities, requirements = :requirements,
			connection_config = :connection_config, auth_config = :auth_config,
			metadata = :metadata, status = :status, updated_by = :updated_by,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, manifest)
	if err != nil {
		r.logger.Error("Failed to update agent manifest", map[string]interface{}{
			"id":    manifest.ID,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to update agent manifest: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent manifest not found: %s", manifest.ID)
	}

	return nil
}

// DeleteManifest deletes an agent manifest
func (r *agentManifestRepository) DeleteManifest(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.DeleteManifest")
	defer span.End()

	query := `DELETE FROM mcp.agent_manifests WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete agent manifest", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to delete agent manifest: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent manifest not found: %s", id)
	}

	return nil
}

// UpdateManifestHeartbeat updates the last heartbeat timestamp
func (r *agentManifestRepository) UpdateManifestHeartbeat(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.UpdateManifestHeartbeat")
	defer span.End()

	query := `
		UPDATE mcp.agent_manifests
		SET last_heartbeat = CURRENT_TIMESTAMP
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to update manifest heartbeat", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to update manifest heartbeat: %w", err)
	}

	return nil
}

// CreateRegistration creates a new agent registration
func (r *agentManifestRepository) CreateRegistration(ctx context.Context, registration *models.AgentRegistration) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.CreateRegistration")
	defer span.End()

	if registration.ID == uuid.Nil {
		registration.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.agent_registrations (
			id, manifest_id, tenant_id, instance_id, registration_token,
			registration_status, activation_date, expiration_date,
			runtime_config, connection_details, health_status,
			health_check_url, metrics
		) VALUES (
			:id, :manifest_id, :tenant_id, :instance_id, :registration_token,
			:registration_status, :activation_date, :expiration_date,
			:runtime_config, :connection_details, :health_status,
			:health_check_url, :metrics
		)`

	_, err := r.db.NamedExecContext(ctx, query, registration)
	if err != nil {
		r.logger.Error("Failed to create agent registration", map[string]interface{}{
			"instance_id": registration.InstanceID,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to create agent registration: %w", err)
	}

	return nil
}

// GetRegistrationByID retrieves an agent registration by ID
func (r *agentManifestRepository) GetRegistrationByID(ctx context.Context, id uuid.UUID) (*models.AgentRegistration, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.GetRegistrationByID")
	defer span.End()

	query := `
		SELECT id, manifest_id, tenant_id, instance_id, registration_token,
			registration_status, activation_date, expiration_date,
			runtime_config, connection_details, health_status,
			health_check_url, last_health_check, metrics,
			created_at, updated_at
		FROM mcp.agent_registrations
		WHERE id = $1`

	var registration models.AgentRegistration
	err := r.db.GetContext(ctx, &registration, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get agent registration", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to get agent registration: %w", err)
	}

	return &registration, nil
}

// GetRegistrationByInstance retrieves an agent registration by tenant and instance ID
func (r *agentManifestRepository) GetRegistrationByInstance(ctx context.Context, tenantID uuid.UUID, instanceID string) (*models.AgentRegistration, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.GetRegistrationByInstance")
	defer span.End()

	query := `
		SELECT id, manifest_id, tenant_id, instance_id, registration_token,
			registration_status, activation_date, expiration_date,
			runtime_config, connection_details, health_status,
			health_check_url, last_health_check, metrics,
			created_at, updated_at
		FROM mcp.agent_registrations
		WHERE tenant_id = $1 AND instance_id = $2`

	var registration models.AgentRegistration
	err := r.db.GetContext(ctx, &registration, query, tenantID, instanceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get agent registration by instance", map[string]interface{}{
			"tenant_id":   tenantID,
			"instance_id": instanceID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to get agent registration by instance: %w", err)
	}

	return &registration, nil
}

// ListRegistrationsByTenant lists all agent registrations for a tenant
func (r *agentManifestRepository) ListRegistrationsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.AgentRegistration, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListRegistrationsByTenant")
	defer span.End()

	query := `
		SELECT id, manifest_id, tenant_id, instance_id, registration_token,
			registration_status, activation_date, expiration_date,
			runtime_config, connection_details, health_status,
			health_check_url, last_health_check, metrics,
			created_at, updated_at
		FROM mcp.agent_registrations
		WHERE tenant_id = $1
		ORDER BY created_at DESC`

	var registrations []models.AgentRegistration
	err := r.db.SelectContext(ctx, &registrations, query, tenantID)
	if err != nil {
		r.logger.Error("Failed to list agent registrations by tenant", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to list agent registrations by tenant: %w", err)
	}

	return registrations, nil
}

// ListRegistrationsByManifest lists all agent registrations for a manifest
func (r *agentManifestRepository) ListRegistrationsByManifest(ctx context.Context, manifestID uuid.UUID) ([]models.AgentRegistration, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListRegistrationsByManifest")
	defer span.End()

	query := `
		SELECT id, manifest_id, tenant_id, instance_id, registration_token,
			registration_status, activation_date, expiration_date,
			runtime_config, connection_details, health_status,
			health_check_url, last_health_check, metrics,
			created_at, updated_at
		FROM mcp.agent_registrations
		WHERE manifest_id = $1
		ORDER BY created_at DESC`

	var registrations []models.AgentRegistration
	err := r.db.SelectContext(ctx, &registrations, query, manifestID)
	if err != nil {
		r.logger.Error("Failed to list agent registrations by manifest", map[string]interface{}{
			"manifest_id": manifestID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to list agent registrations by manifest: %w", err)
	}

	return registrations, nil
}

// UpdateRegistration updates an agent registration
func (r *agentManifestRepository) UpdateRegistration(ctx context.Context, registration *models.AgentRegistration) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.UpdateRegistration")
	defer span.End()

	query := `
		UPDATE mcp.agent_registrations
		SET registration_status = :registration_status,
			activation_date = :activation_date,
			expiration_date = :expiration_date,
			runtime_config = :runtime_config,
			connection_details = :connection_details,
			health_status = :health_status,
			health_check_url = :health_check_url,
			metrics = :metrics,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, registration)
	if err != nil {
		r.logger.Error("Failed to update agent registration", map[string]interface{}{
			"id":    registration.ID,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to update agent registration: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent registration not found: %s", registration.ID)
	}

	return nil
}

// UpdateRegistrationHealth updates the health status of a registration
func (r *agentManifestRepository) UpdateRegistrationHealth(ctx context.Context, id uuid.UUID, status string) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.UpdateRegistrationHealth")
	defer span.End()

	query := `
		UPDATE mcp.agent_registrations
		SET health_status = $2,
			last_health_check = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		r.logger.Error("Failed to update registration health", map[string]interface{}{
			"id":     id,
			"status": status,
			"error":  err.Error(),
		})
		return fmt.Errorf("failed to update registration health: %w", err)
	}

	return nil
}

// DeleteRegistration deletes an agent registration
func (r *agentManifestRepository) DeleteRegistration(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.DeleteRegistration")
	defer span.End()

	query := `DELETE FROM mcp.agent_registrations WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete agent registration", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to delete agent registration: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent registration not found: %s", id)
	}

	return nil
}

// AddCapability adds a capability to a manifest
func (r *agentManifestRepository) AddCapability(ctx context.Context, capability *models.ManifestCapability) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.AddCapability")
	defer span.End()

	if capability.ID == uuid.Nil {
		capability.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.agent_capabilities (
			id, manifest_id, capability_type, capability_name,
			capability_config, required
		) VALUES (
			:id, :manifest_id, :capability_type, :capability_name,
			:capability_config, :required
		)`

	_, err := r.db.NamedExecContext(ctx, query, capability)
	if err != nil {
		r.logger.Error("Failed to add capability", map[string]interface{}{
			"manifest_id": capability.ManifestID,
			"type":        capability.CapabilityType,
			"name":        capability.CapabilityName,
			"error":       err.Error(),
		})
		return fmt.Errorf("failed to add capability: %w", err)
	}

	return nil
}

// RemoveCapability removes a capability
func (r *agentManifestRepository) RemoveCapability(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.RemoveCapability")
	defer span.End()

	query := `DELETE FROM mcp.agent_capabilities WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to remove capability", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to remove capability: %w", err)
	}

	return nil
}

// ListCapabilitiesByManifest lists all capabilities for a manifest
func (r *agentManifestRepository) ListCapabilitiesByManifest(ctx context.Context, manifestID uuid.UUID) ([]models.ManifestCapability, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListCapabilitiesByManifest")
	defer span.End()

	query := `
		SELECT id, manifest_id, capability_type, capability_name,
			capability_config, required, created_at
		FROM mcp.agent_capabilities
		WHERE manifest_id = $1
		ORDER BY capability_type, capability_name`

	var capabilities []models.ManifestCapability
	err := r.db.SelectContext(ctx, &capabilities, query, manifestID)
	if err != nil {
		r.logger.Error("Failed to list capabilities by manifest", map[string]interface{}{
			"manifest_id": manifestID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to list capabilities by manifest: %w", err)
	}

	return capabilities, nil
}

// ListManifestsByCapability lists all manifests with a specific capability
func (r *agentManifestRepository) ListManifestsByCapability(ctx context.Context, capabilityType, capabilityName string) ([]models.AgentManifest, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListManifestsByCapability")
	defer span.End()

	query := `
		SELECT DISTINCT m.id, m.organization_id, m.agent_id, m.agent_type, 
			m.name, m.version, m.description, m.capabilities, m.requirements,
			m.connection_config, m.auth_config, m.metadata, m.status,
			m.last_heartbeat, m.created_by, m.updated_by, m.created_at, m.updated_at
		FROM mcp.agent_manifests m
		JOIN mcp.agent_capabilities c ON m.id = c.manifest_id
		WHERE c.capability_type = $1 AND c.capability_name = $2
		ORDER BY m.created_at DESC`

	var manifests []models.AgentManifest
	err := r.db.SelectContext(ctx, &manifests, query, capabilityType, capabilityName)
	if err != nil {
		r.logger.Error("Failed to list manifests by capability", map[string]interface{}{
			"capability_type": capabilityType,
			"capability_name": capabilityName,
			"error":           err.Error(),
		})
		return nil, fmt.Errorf("failed to list manifests by capability: %w", err)
	}

	return manifests, nil
}

// CreateChannel creates a new agent channel
func (r *agentManifestRepository) CreateChannel(ctx context.Context, channel *models.AgentChannel) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.CreateChannel")
	defer span.End()

	if channel.ID == uuid.Nil {
		channel.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.agent_channels (
			id, registration_id, channel_type, channel_config,
			priority, active
		) VALUES (
			:id, :registration_id, :channel_type, :channel_config,
			:priority, :active
		)`

	_, err := r.db.NamedExecContext(ctx, query, channel)
	if err != nil {
		r.logger.Error("Failed to create agent channel", map[string]interface{}{
			"registration_id": channel.RegistrationID,
			"channel_type":    channel.ChannelType,
			"error":           err.Error(),
		})
		return fmt.Errorf("failed to create agent channel: %w", err)
	}

	return nil
}

// GetChannelByID retrieves an agent channel by ID
func (r *agentManifestRepository) GetChannelByID(ctx context.Context, id uuid.UUID) (*models.AgentChannel, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.GetChannelByID")
	defer span.End()

	query := `
		SELECT id, registration_id, channel_type, channel_config,
			priority, active, last_message_at, created_at
		FROM mcp.agent_channels
		WHERE id = $1`

	var channel models.AgentChannel
	err := r.db.GetContext(ctx, &channel, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get agent channel", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to get agent channel: %w", err)
	}

	return &channel, nil
}

// ListChannelsByRegistration lists all channels for a registration
func (r *agentManifestRepository) ListChannelsByRegistration(ctx context.Context, registrationID uuid.UUID) ([]models.AgentChannel, error) {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.ListChannelsByRegistration")
	defer span.End()

	query := `
		SELECT id, registration_id, channel_type, channel_config,
			priority, active, last_message_at, created_at
		FROM mcp.agent_channels
		WHERE registration_id = $1
		ORDER BY priority DESC, created_at DESC`

	var channels []models.AgentChannel
	err := r.db.SelectContext(ctx, &channels, query, registrationID)
	if err != nil {
		r.logger.Error("Failed to list channels by registration", map[string]interface{}{
			"registration_id": registrationID,
			"error":           err.Error(),
		})
		return nil, fmt.Errorf("failed to list channels by registration: %w", err)
	}

	return channels, nil
}

// UpdateChannel updates an agent channel
func (r *agentManifestRepository) UpdateChannel(ctx context.Context, channel *models.AgentChannel) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.UpdateChannel")
	defer span.End()

	query := `
		UPDATE mcp.agent_channels
		SET channel_config = :channel_config,
			priority = :priority,
			active = :active,
			last_message_at = :last_message_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, channel)
	if err != nil {
		r.logger.Error("Failed to update agent channel", map[string]interface{}{
			"id":    channel.ID,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to update agent channel: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent channel not found: %s", channel.ID)
	}

	return nil
}

// DeleteChannel deletes an agent channel
func (r *agentManifestRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.StartSpan(ctx, "repository.agent_manifest.DeleteChannel")
	defer span.End()

	query := `DELETE FROM mcp.agent_channels WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete agent channel", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
		return fmt.Errorf("failed to delete agent channel: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent channel not found: %s", id)
	}

	return nil
}
