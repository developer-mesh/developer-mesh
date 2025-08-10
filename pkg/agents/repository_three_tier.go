package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ThreeTierRepository implements the desired state architecture for agent management
// This follows the three-tier model: Manifests -> Configurations -> Registrations
type ThreeTierRepository struct {
	db     *sqlx.DB
	schema string
	logger observability.Logger
}

// NewThreeTierRepository creates a repository for the three-tier agent architecture
func NewThreeTierRepository(db *sqlx.DB, schema string, logger observability.Logger) *ThreeTierRepository {
	if schema == "" {
		schema = "mcp"
	}
	return &ThreeTierRepository{
		db:     db,
		schema: schema,
		logger: logger,
	}
}

// ============================================================================
// Agent Manifest Operations (Blueprint/Type definitions)
// ============================================================================

// CreateManifest creates or updates an agent type definition
func (r *ThreeTierRepository) CreateManifest(ctx context.Context, manifest *AgentManifest) error {
	if manifest.ID == uuid.Nil {
		manifest.ID = uuid.New()
	}

	capabilitiesJSON, err := json.Marshal(manifest.Capabilities)
	if err != nil {
		return fmt.Errorf("failed to marshal capabilities: %w", err)
	}

	metadataJSON, err := json.Marshal(manifest.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.agent_manifests (
			id, agent_id, agent_type, name, description, version,
			capabilities, requirements, metadata, status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (agent_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			version = EXCLUDED.version,
			capabilities = EXCLUDED.capabilities,
			requirements = EXCLUDED.requirements,
			metadata = EXCLUDED.metadata,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, r.schema)

	now := time.Now()
	_, err = r.db.ExecContext(ctx, query,
		manifest.ID, manifest.AgentID, manifest.AgentType, manifest.Name, manifest.Description,
		manifest.Version, capabilitiesJSON, manifest.Requirements, metadataJSON,
		manifest.Status, now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to create/update manifest: %w", err)
	}

	return nil
}

// GetManifest retrieves an agent manifest by ID or agent_id
func (r *ThreeTierRepository) GetManifest(ctx context.Context, identifier string) (*AgentManifest, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, agent_type, name, description, version,
			capabilities, requirements, metadata, status,
			created_at, updated_at
		FROM %s.agent_manifests
		WHERE agent_id = $1 OR id = $2
	`, r.schema)

	var manifest AgentManifest
	var capabilitiesJSON, metadataJSON []byte

	// Try parsing as UUID first
	var queryID uuid.UUID
	if uid, err := uuid.Parse(identifier); err == nil {
		queryID = uid
	}

	err := r.db.QueryRowContext(ctx, query, identifier, queryID).Scan(
		&manifest.ID, &manifest.AgentID, &manifest.AgentType, &manifest.Name,
		&manifest.Description, &manifest.Version, &capabilitiesJSON,
		&manifest.Requirements, &metadataJSON, &manifest.Status,
		&manifest.CreatedAt, &manifest.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("manifest not found: %s", identifier)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	if err := json.Unmarshal(capabilitiesJSON, &manifest.Capabilities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &manifest.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &manifest, nil
}

// ListManifests lists all available agent manifests
func (r *ThreeTierRepository) ListManifests(ctx context.Context, filter ManifestFilter) ([]*AgentManifest, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, agent_type, name, description, version,
			capabilities, requirements, metadata, status,
			created_at, updated_at
		FROM %s.agent_manifests
		WHERE 1=1
	`, r.schema)

	args := []interface{}{}
	argCount := 0

	if filter.AgentType != "" {
		argCount++
		query += fmt.Sprintf(" AND agent_type = $%d", argCount)
		args = append(args, filter.AgentType)
	}

	if filter.Status != "" {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, filter.Status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list manifests: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var manifests []*AgentManifest
	for rows.Next() {
		var manifest AgentManifest
		var capabilitiesJSON, metadataJSON []byte

		err := rows.Scan(
			&manifest.ID, &manifest.AgentID, &manifest.AgentType, &manifest.Name,
			&manifest.Description, &manifest.Version, &capabilitiesJSON,
			&manifest.Requirements, &metadataJSON, &manifest.Status,
			&manifest.CreatedAt, &manifest.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan manifest: %w", err)
		}

		if err := json.Unmarshal(capabilitiesJSON, &manifest.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &manifest.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		manifests = append(manifests, &manifest)
	}

	return manifests, nil
}

// ============================================================================
// Agent Configuration Operations (Tenant-specific configurations)
// ============================================================================

// CreateConfiguration creates a tenant-specific configuration for an agent manifest
func (r *ThreeTierRepository) CreateConfiguration(ctx context.Context, config *AgentConfiguration) error {
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}

	configJSON, err := json.Marshal(config.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.agent_configurations (
			id, tenant_id, manifest_id, name, enabled,
			configuration, system_prompt, temperature, max_tokens,
			model_id, max_workload, current_workload,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (tenant_id, manifest_id) DO UPDATE SET
			name = EXCLUDED.name,
			enabled = EXCLUDED.enabled,
			configuration = EXCLUDED.configuration,
			system_prompt = EXCLUDED.system_prompt,
			temperature = EXCLUDED.temperature,
			max_tokens = EXCLUDED.max_tokens,
			model_id = EXCLUDED.model_id,
			max_workload = EXCLUDED.max_workload,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`, r.schema)

	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		config.ID, config.TenantID, config.ManifestID, config.Name, config.Enabled,
		configJSON, config.SystemPrompt, config.Temperature, config.MaxTokens,
		config.ModelID, config.MaxWorkload, config.CurrentWorkload,
		now, now,
	).Scan(&config.ID)

	if err != nil {
		return fmt.Errorf("failed to create/update configuration: %w", err)
	}

	return nil
}

// GetConfiguration retrieves a specific agent configuration
func (r *ThreeTierRepository) GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*AgentConfiguration, error) {
	query := fmt.Sprintf(`
		SELECT 
			c.id, c.tenant_id, c.manifest_id, c.name, c.enabled,
			c.configuration, c.system_prompt, c.temperature, c.max_tokens,
			c.model_id, c.max_workload, c.current_workload,
			c.created_at, c.updated_at,
			m.agent_id, m.agent_type, m.name as manifest_name
		FROM %s.agent_configurations c
		JOIN %s.agent_manifests m ON m.id = c.manifest_id
		WHERE c.tenant_id = $1 AND c.id = $2
	`, r.schema, r.schema)

	var config AgentConfiguration
	var configJSON []byte
	var manifestAgentID, manifestType, manifestName string

	err := r.db.QueryRowContext(ctx, query, tenantID, configID).Scan(
		&config.ID, &config.TenantID, &config.ManifestID, &config.Name, &config.Enabled,
		&configJSON, &config.SystemPrompt, &config.Temperature, &config.MaxTokens,
		&config.ModelID, &config.MaxWorkload, &config.CurrentWorkload,
		&config.CreatedAt, &config.UpdatedAt,
		&manifestAgentID, &manifestType, &manifestName,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("configuration not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}

	if err := json.Unmarshal(configJSON, &config.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Add manifest info to metadata for context
	if config.Configuration == nil {
		config.Configuration = make(map[string]interface{})
	}
	config.Configuration["_manifest"] = map[string]interface{}{
		"agent_id": manifestAgentID,
		"type":     manifestType,
		"name":     manifestName,
	}

	return &config, nil
}

// ListConfigurations lists all configurations for a tenant
func (r *ThreeTierRepository) ListConfigurations(ctx context.Context, tenantID uuid.UUID, filter ConfigurationFilter) ([]*AgentConfiguration, error) {
	query := fmt.Sprintf(`
		SELECT 
			c.id, c.tenant_id, c.manifest_id, c.name, c.enabled,
			c.configuration, c.system_prompt, c.temperature, c.max_tokens,
			c.model_id, c.max_workload, c.current_workload,
			c.created_at, c.updated_at
		FROM %s.agent_configurations c
		WHERE c.tenant_id = $1
	`, r.schema)

	args := []interface{}{tenantID}
	argCount := 1

	if filter.Enabled != nil {
		argCount++
		query += fmt.Sprintf(" AND c.enabled = $%d", argCount)
		args = append(args, *filter.Enabled)
	}

	if filter.ManifestID != nil {
		argCount++
		query += fmt.Sprintf(" AND c.manifest_id = $%d", argCount)
		args = append(args, *filter.ManifestID)
	}

	query += " ORDER BY c.created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list configurations: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var configs []*AgentConfiguration
	for rows.Next() {
		var config AgentConfiguration
		var configJSON []byte

		err := rows.Scan(
			&config.ID, &config.TenantID, &config.ManifestID, &config.Name, &config.Enabled,
			&configJSON, &config.SystemPrompt, &config.Temperature, &config.MaxTokens,
			&config.ModelID, &config.MaxWorkload, &config.CurrentWorkload,
			&config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration: %w", err)
		}

		if err := json.Unmarshal(configJSON, &config.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}

		configs = append(configs, &config)
	}

	return configs, nil
}

// UpdateWorkload updates the current workload for a configuration
func (r *ThreeTierRepository) UpdateWorkload(ctx context.Context, configID uuid.UUID, delta int) error {
	query := fmt.Sprintf(`
		UPDATE %s.agent_configurations 
		SET current_workload = GREATEST(0, current_workload + $2),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING current_workload
	`, r.schema)

	var newWorkload int
	err := r.db.QueryRowContext(ctx, query, configID, delta).Scan(&newWorkload)
	if err != nil {
		return fmt.Errorf("failed to update workload: %w", err)
	}

	return nil
}

// ============================================================================
// Agent Registration Operations (Active instances)
// ============================================================================

// RegisterInstance registers or updates an agent instance (handles reconnections)
func (r *ThreeTierRepository) RegisterInstance(ctx context.Context, reg *AgentRegistration) (*RegistrationResult, error) {
	// Use the stored function for idempotent registration
	query := fmt.Sprintf(`
		SELECT * FROM %s.register_agent_instance($1, $2, $3, $4, $5, $6)
	`, r.schema)

	connectionDetails, _ := json.Marshal(reg.ConnectionDetails)
	runtimeConfig, _ := json.Marshal(reg.RuntimeConfig)

	var result RegistrationResult
	err := r.db.QueryRowContext(ctx, query,
		reg.TenantID, reg.AgentID, reg.InstanceID, reg.Name,
		connectionDetails, runtimeConfig,
	).Scan(
		&result.RegistrationID,
		&result.ManifestID,
		&result.ConfigID,
		&result.IsNew,
		&result.Message,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to register instance: %w", err)
	}

	return &result, nil
}

// GetActiveRegistrations gets all active registrations for a tenant
func (r *ThreeTierRepository) GetActiveRegistrations(ctx context.Context, tenantID uuid.UUID) ([]*AgentRegistration, error) {
	query := fmt.Sprintf(`
		SELECT 
			r.id, r.manifest_id, r.tenant_id, r.instance_id,
			r.registration_status, r.health_status,
			r.connection_details, r.runtime_config,
			r.activation_date, r.last_health_check,
			r.failure_count, r.created_at, r.updated_at,
			m.agent_id, m.agent_type, m.name
		FROM %s.agent_registrations r
		JOIN %s.agent_manifests m ON m.id = r.manifest_id
		WHERE r.tenant_id = $1 
		  AND r.registration_status = 'active'
		  AND r.health_status IN ('healthy', 'degraded')
		ORDER BY r.last_health_check DESC
	`, r.schema, r.schema)

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active registrations: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var registrations []*AgentRegistration
	for rows.Next() {
		var reg AgentRegistration
		var connectionJSON, runtimeJSON []byte
		var manifestAgentID, manifestType, manifestName string

		err := rows.Scan(
			&reg.ID, &reg.ManifestID, &reg.TenantID, &reg.InstanceID,
			&reg.RegistrationStatus, &reg.HealthStatus,
			&connectionJSON, &runtimeJSON,
			&reg.ActivationDate, &reg.LastHealthCheck,
			&reg.FailureCount, &reg.CreatedAt, &reg.UpdatedAt,
			&manifestAgentID, &manifestType, &manifestName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan registration: %w", err)
		}

		if len(connectionJSON) > 0 {
			_ = json.Unmarshal(connectionJSON, &reg.ConnectionDetails)
		}
		if len(runtimeJSON) > 0 {
			_ = json.Unmarshal(runtimeJSON, &reg.RuntimeConfig)
		}

		// Store manifest info
		reg.AgentID = manifestAgentID
		reg.Name = manifestName

		registrations = append(registrations, &reg)
	}

	return registrations, nil
}

// UpdateHealth updates the health status of a registration
func (r *ThreeTierRepository) UpdateHealth(ctx context.Context, instanceID string, status HealthStatus) error {
	query := fmt.Sprintf(`
		UPDATE %s.agent_registrations
		SET health_status = $2,
		    last_health_check = NOW(),
		    failure_count = CASE WHEN $2 = 'healthy' THEN 0 ELSE failure_count END,
		    updated_at = NOW()
		WHERE instance_id = $1
	`, r.schema)

	_, err := r.db.ExecContext(ctx, query, instanceID, status)
	if err != nil {
		return fmt.Errorf("failed to update health: %w", err)
	}

	return nil
}

// DeactivateRegistration marks a registration as inactive
func (r *ThreeTierRepository) DeactivateRegistration(ctx context.Context, instanceID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.agent_registrations
		SET registration_status = 'inactive',
		    health_status = 'disconnected',
		    deactivation_date = NOW(),
		    updated_at = NOW()
		WHERE instance_id = $1
	`, r.schema)

	_, err := r.db.ExecContext(ctx, query, instanceID)
	if err != nil {
		return fmt.Errorf("failed to deactivate registration: %w", err)
	}

	return nil
}

// ============================================================================
// Combined Operations (Queries across the three tiers)
// ============================================================================

// GetAvailableAgents returns agents that are configured and have healthy registrations
func (r *ThreeTierRepository) GetAvailableAgents(ctx context.Context, tenantID uuid.UUID) ([]*AvailableAgent, error) {
	query := fmt.Sprintf(`
		SELECT 
			c.id as config_id,
			c.name as config_name,
			c.model_id,
			c.system_prompt,
			c.temperature,
			c.max_tokens,
			c.current_workload,
			c.max_workload,
			m.id as manifest_id,
			m.agent_id,
			m.agent_type,
			m.capabilities,
			m.version,
			COALESCE(r.instance_count, 0) as active_instances,
			COALESCE(r.healthy_count, 0) as healthy_instances
		FROM %s.agent_configurations c
		JOIN %s.agent_manifests m ON m.id = c.manifest_id
		LEFT JOIN (
			SELECT 
				manifest_id,
				tenant_id,
				COUNT(*) as instance_count,
				COUNT(*) FILTER (WHERE health_status = 'healthy') as healthy_count
			FROM %s.agent_registrations
			WHERE registration_status = 'active'
			GROUP BY manifest_id, tenant_id
		) r ON r.manifest_id = c.manifest_id AND r.tenant_id = c.tenant_id
		WHERE c.tenant_id = $1
		  AND c.enabled = true
		  AND c.current_workload < c.max_workload
		  AND m.status = 'active'
		ORDER BY c.current_workload ASC, r.healthy_count DESC
	`, r.schema, r.schema, r.schema)

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get available agents: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var agents []*AvailableAgent
	for rows.Next() {
		var agent AvailableAgent
		var capabilitiesJSON []byte

		err := rows.Scan(
			&agent.ConfigID, &agent.ConfigName, &agent.ModelID,
			&agent.SystemPrompt, &agent.Temperature, &agent.MaxTokens,
			&agent.CurrentWorkload, &agent.MaxWorkload,
			&agent.ManifestID, &agent.AgentID, &agent.AgentType,
			&capabilitiesJSON, &agent.Version,
			&agent.ActiveInstances, &agent.HealthyInstances,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan available agent: %w", err)
		}

		if err := json.Unmarshal(capabilitiesJSON, &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}

		// Calculate availability score
		agent.AvailabilityScore = r.calculateAvailabilityScore(&agent)

		agents = append(agents, &agent)
	}

	return agents, nil
}

// GetAgentMetrics retrieves metrics for a specific agent configuration
func (r *ThreeTierRepository) GetAgentMetrics(ctx context.Context, configID uuid.UUID, period time.Duration) (*AgentMetrics, error) {
	since := time.Now().Add(-period)

	query := fmt.Sprintf(`
		SELECT 
			c.id,
			c.name,
			c.current_workload,
			c.max_workload,
			COUNT(DISTINCT r.id) as total_registrations,
			COUNT(DISTINCT r.id) FILTER (WHERE r.health_status = 'healthy') as healthy_registrations,
			COUNT(DISTINCT r.id) FILTER (WHERE r.registration_status = 'active') as active_registrations,
			AVG(r.failure_count)::INTEGER as avg_failure_count,
			MAX(r.last_health_check) as last_activity
		FROM %s.agent_configurations c
		LEFT JOIN %s.agent_registrations r 
			ON r.manifest_id = c.manifest_id 
			AND r.tenant_id = c.tenant_id
			AND r.created_at >= $2
		WHERE c.id = $1
		GROUP BY c.id, c.name, c.current_workload, c.max_workload
	`, r.schema, r.schema)

	var metrics AgentMetrics
	err := r.db.QueryRowContext(ctx, query, configID, since).Scan(
		&metrics.ConfigID,
		&metrics.Name,
		&metrics.CurrentWorkload,
		&metrics.MaxWorkload,
		&metrics.TotalRegistrations,
		&metrics.HealthyRegistrations,
		&metrics.ActiveRegistrations,
		&metrics.AvgFailureCount,
		&metrics.LastActivity,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get agent metrics: %w", err)
	}

	// Calculate derived metrics
	metrics.WorkloadUtilization = float64(metrics.CurrentWorkload) / float64(metrics.MaxWorkload)
	if metrics.TotalRegistrations > 0 {
		metrics.HealthRate = float64(metrics.HealthyRegistrations) / float64(metrics.TotalRegistrations)
	}

	return &metrics, nil
}

// ============================================================================
// Helper Methods
// ============================================================================

// calculateAvailabilityScore calculates a score for agent selection
func (r *ThreeTierRepository) calculateAvailabilityScore(agent *AvailableAgent) float64 {
	// Factors:
	// - Workload utilization (lower is better)
	// - Healthy instances (more is better)
	// - Active instances (more is better for redundancy)

	workloadScore := 1.0 - (float64(agent.CurrentWorkload) / float64(agent.MaxWorkload))
	healthScore := float64(agent.HealthyInstances) / float64(max(agent.ActiveInstances, 1))
	redundancyScore := min(float64(agent.ActiveInstances)/3.0, 1.0) // Ideal is 3+ instances

	// Weighted average
	return (workloadScore * 0.5) + (healthScore * 0.3) + (redundancyScore * 0.2)
}

// RecordEvent records an agent-related event
func (r *ThreeTierRepository) RecordEvent(ctx context.Context, event *ThreeTierAgentEvent) error {
	payloadJSON, _ := json.Marshal(event.Payload)

	query := fmt.Sprintf(`
		INSERT INTO %s.agent_events (
			id, agent_id, tenant_id, event_type, event_version,
			payload, initiated_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, r.schema)

	_, err := r.db.ExecContext(ctx, query,
		event.ID, event.AgentID, event.TenantID, event.EventType, event.EventVersion,
		payloadJSON, event.InitiatedBy, event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record event: %w", err)
	}

	return nil
}

// Cleanup removes stale registrations and updates health status
func (r *ThreeTierRepository) Cleanup(ctx context.Context, staleThreshold time.Duration) error {
	threshold := time.Now().Add(-staleThreshold)

	// Mark stale registrations as unhealthy
	query := fmt.Sprintf(`
		UPDATE %s.agent_registrations
		SET health_status = 'unknown',
		    failure_count = failure_count + 1,
		    updated_at = NOW()
		WHERE last_health_check < $1
		  AND health_status IN ('healthy', 'degraded')
		  AND registration_status = 'active'
	`, r.schema)

	_, err := r.db.ExecContext(ctx, query, threshold)
	if err != nil {
		return fmt.Errorf("failed to update stale registrations: %w", err)
	}

	// Deactivate registrations that have been unhealthy too long
	deactivateThreshold := time.Now().Add(-staleThreshold * 3)
	query = fmt.Sprintf(`
		UPDATE %s.agent_registrations
		SET registration_status = 'inactive',
		    health_status = 'disconnected',
		    deactivation_date = NOW(),
		    updated_at = NOW()
		WHERE last_health_check < $1
		  AND registration_status = 'active'
	`, r.schema)

	_, err = r.db.ExecContext(ctx, query, deactivateThreshold)
	if err != nil {
		return fmt.Errorf("failed to deactivate dead registrations: %w", err)
	}

	return nil
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
