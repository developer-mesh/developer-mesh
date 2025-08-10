package agents

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db := sqlx.NewDb(mockDB, "postgres")
	return db, mock
}

func TestThreeTierRepository_CreateManifest(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()

	manifest := &AgentManifest{
		ID:          uuid.New(),
		AgentID:     "ide-agent",
		AgentType:   "ide",
		Name:        "IDE Agent",
		Description: "IDE integration agent",
		Version:     "1.0.0",
		Capabilities: map[string]interface{}{
			"features": []string{"code_editing", "debugging"},
		},
		Status: "active",
	}

	// Expect INSERT with ON CONFLICT
	mock.ExpectExec("INSERT INTO mcp.agent_manifests").
		WithArgs(
			manifest.ID,
			manifest.AgentID,
			manifest.AgentType,
			manifest.Name,
			manifest.Description,
			manifest.Version,
			sqlmock.AnyArg(), // capabilities JSON
			manifest.Requirements,
			sqlmock.AnyArg(), // metadata JSON
			manifest.Status,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateManifest(ctx, manifest)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_CreateConfiguration(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()

	config := &AgentConfiguration{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		ManifestID: uuid.New(),
		Name:       "Development IDE",
		Enabled:    true,
		Configuration: map[string]interface{}{
			"workspace": "/projects",
		},
		SystemPrompt: "You are a helpful assistant",
		Temperature:  0.7,
		MaxTokens:    4096,
		ModelID:      uuid.New(),
		MaxWorkload:  10,
	}

	// Expect INSERT with RETURNING
	mock.ExpectQuery("INSERT INTO mcp.agent_configurations").
		WithArgs(
			config.ID,
			config.TenantID,
			config.ManifestID,
			config.Name,
			config.Enabled,
			sqlmock.AnyArg(), // configuration JSON
			config.SystemPrompt,
			config.Temperature,
			config.MaxTokens,
			config.ModelID,
			config.MaxWorkload,
			config.CurrentWorkload,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(config.ID))

	err := repo.CreateConfiguration(ctx, config)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_RegisterInstance(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()

	reg := &AgentRegistration{
		TenantID:   uuid.New(),
		AgentID:    "ide-agent",
		InstanceID: "ws_conn_123",
		Name:       "VS Code",
		ConnectionDetails: map[string]interface{}{
			"ip": "192.168.1.100",
		},
	}

	expectedResult := &RegistrationResult{
		RegistrationID: uuid.New(),
		ManifestID:     uuid.New(),
		ConfigID:       uuid.New(),
		IsNew:          true,
		Message:        "New registration created",
	}

	// Expect stored function call
	mock.ExpectQuery("SELECT \\* FROM mcp.register_agent_instance").
		WithArgs(
			reg.TenantID,
			reg.AgentID,
			reg.InstanceID,
			reg.Name,
			sqlmock.AnyArg(), // connection details JSON
			sqlmock.AnyArg(), // runtime config JSON
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"registration_id", "manifest_id", "config_id", "is_new", "message",
		}).AddRow(
			expectedResult.RegistrationID,
			expectedResult.ManifestID,
			expectedResult.ConfigID,
			expectedResult.IsNew,
			expectedResult.Message,
		))

	result, err := repo.RegisterInstance(ctx, reg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResult.RegistrationID, result.RegistrationID)
	assert.True(t, result.IsNew)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_GetAvailableAgents(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()
	tenantID := uuid.New()

	// Mock query for available agents
	mock.ExpectQuery("SELECT .+ FROM mcp.agent_configurations c").
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{
			"config_id", "config_name", "model_id",
			"system_prompt", "temperature", "max_tokens",
			"current_workload", "max_workload",
			"manifest_id", "agent_id", "agent_type",
			"capabilities", "version",
			"active_instances", "healthy_instances",
		}).AddRow(
			uuid.New(), "IDE Agent", uuid.New(),
			"You are helpful", 0.7, 4096,
			2, 10,
			uuid.New(), "ide-agent", "ide",
			[]byte(`{"features": ["code_editing"]}`), "1.0.0",
			3, 2,
		))

	agents, err := repo.GetAvailableAgents(ctx, tenantID)
	assert.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, "IDE Agent", agents[0].ConfigName)
	assert.Equal(t, 2, agents[0].CurrentWorkload)
	assert.Equal(t, 10, agents[0].MaxWorkload)
	assert.Greater(t, agents[0].AvailabilityScore, 0.0)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_UpdateHealth(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()
	instanceID := "ws_conn_123"

	// Expect health update
	mock.ExpectExec("UPDATE mcp.agent_registrations").
		WithArgs(instanceID, HealthStatusHealthy).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateHealth(ctx, instanceID, HealthStatusHealthy)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_Cleanup(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()
	staleThreshold := 5 * time.Minute

	// Expect update for stale registrations
	mock.ExpectExec("UPDATE mcp.agent_registrations").
		WithArgs(sqlmock.AnyArg()). // threshold time
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect deactivation of dead registrations
	mock.ExpectExec("UPDATE mcp.agent_registrations").
		WithArgs(sqlmock.AnyArg()). // deactivate threshold
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Cleanup(ctx, staleThreshold)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_UpdateWorkload(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()
	configID := uuid.New()

	// Test incrementing workload
	mock.ExpectQuery("UPDATE mcp.agent_configurations").
		WithArgs(configID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"current_workload"}).AddRow(5))

	err := repo.UpdateWorkload(ctx, configID, 1)
	assert.NoError(t, err)

	// Test decrementing workload
	mock.ExpectQuery("UPDATE mcp.agent_configurations").
		WithArgs(configID, -1).
		WillReturnRows(sqlmock.NewRows([]string{"current_workload"}).AddRow(4))

	err = repo.UpdateWorkload(ctx, configID, -1)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_GetAgentMetrics(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()
	configID := uuid.New()
	period := 24 * time.Hour

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM mcp.agent_configurations c").
		WithArgs(configID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "current_workload", "max_workload",
			"total_registrations", "healthy_registrations", "active_registrations",
			"avg_failure_count", "last_activity",
		}).AddRow(
			configID, "IDE Agent", 3, 10,
			5, 4, 4,
			1, &now,
		))

	metrics, err := repo.GetAgentMetrics(ctx, configID, period)
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, "IDE Agent", metrics.Name)
	assert.Equal(t, 3, metrics.CurrentWorkload)
	assert.Equal(t, 10, metrics.MaxWorkload)
	assert.Equal(t, 5, metrics.TotalRegistrations)
	assert.Equal(t, 4, metrics.HealthyRegistrations)
	assert.Equal(t, 0.3, metrics.WorkloadUtilization)
	assert.Equal(t, 0.8, metrics.HealthRate)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestThreeTierRepository_DeactivateRegistration(t *testing.T) {
	db, mock := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			// Ignore close error in tests
			_ = err
		}
	}()

	// Create a no-op logger for tests
	logger := observability.NewNoopLogger()
	repo := NewThreeTierRepository(db, "mcp", logger)
	ctx := context.Background()
	instanceID := "ws_conn_123"

	mock.ExpectExec("UPDATE mcp.agent_registrations").
		WithArgs(instanceID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeactivateRegistration(ctx, instanceID)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestAvailabilityScoreCalculation(t *testing.T) {
	repo := &ThreeTierRepository{}

	tests := []struct {
		name     string
		agent    *AvailableAgent
		expected float64
	}{
		{
			name: "Idle agent with healthy instances",
			agent: &AvailableAgent{
				CurrentWorkload:  0,
				MaxWorkload:      10,
				ActiveInstances:  3,
				HealthyInstances: 3,
			},
			expected: 1.0, // Perfect score
		},
		{
			name: "Half loaded agent",
			agent: &AvailableAgent{
				CurrentWorkload:  5,
				MaxWorkload:      10,
				ActiveInstances:  2,
				HealthyInstances: 2,
			},
			expected: 0.6833, // 0.5*0.5 + 1.0*0.3 + 0.667*0.2
		},
		{
			name: "Fully loaded agent",
			agent: &AvailableAgent{
				CurrentWorkload:  10,
				MaxWorkload:      10,
				ActiveInstances:  1,
				HealthyInstances: 1,
			},
			expected: 0.3667, // 0*0.5 + 1.0*0.3 + 0.333*0.2
		},
		{
			name: "Agent with unhealthy instances",
			agent: &AvailableAgent{
				CurrentWorkload:  2,
				MaxWorkload:      10,
				ActiveInstances:  4,
				HealthyInstances: 2,
			},
			expected: 0.75, // 0.8*0.5 + 0.5*0.3 + 1.0*0.2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := repo.calculateAvailabilityScore(tt.agent)
			assert.InDelta(t, tt.expected, score, 0.01)
		})
	}
}
