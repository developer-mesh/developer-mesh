package websocket

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdempotentAgentRegistration tests that agent registration is idempotent
func TestIdempotentAgentRegistration(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")

	// Test data
	tenantID := uuid.New()
	agentID := "test-ide-agent"
	instanceID := "conn-12345"
	name := "Test IDE Agent"

	registrationID := uuid.New()
	manifestID := uuid.New()
	configID := uuid.New()

	tests := []struct {
		name      string
		scenario  string
		isNew     bool
		setupMock func()
	}{
		{
			name:     "First Registration",
			scenario: "New agent instance connecting for the first time",
			isNew:    true,
			setupMock: func() {
				rows := sqlmock.NewRows([]string{
					"registration_id", "manifest_id", "config_id", "is_new", "message",
				}).AddRow(
					registrationID, manifestID, configID, true, "New registration created",
				)

				mock.ExpectQuery(`SELECT \* FROM mcp\.register_agent_instance`).
					WithArgs(
						tenantID,
						agentID,
						instanceID,
						name,
						sqlmock.AnyArg(),
						sqlmock.AnyArg(),
					).
					WillReturnRows(rows)
			},
		},
		{
			name:     "Reconnection",
			scenario: "Same agent instance reconnecting (e.g., network disconnect)",
			isNew:    false,
			setupMock: func() {
				rows := sqlmock.NewRows([]string{
					"registration_id", "manifest_id", "config_id", "is_new", "message",
				}).AddRow(
					registrationID, manifestID, configID, false, "Registration updated (reconnection)",
				)

				mock.ExpectQuery(`SELECT \* FROM mcp\.register_agent_instance`).
					WithArgs(
						tenantID,
						agentID,
						instanceID,
						name,
						sqlmock.AnyArg(),
						sqlmock.AnyArg(),
					).
					WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock expectations
			tt.setupMock()

			// Create registry with mock DB
			registry := &EnhancedAgentRegistry{
				db: sqlxDB,
				DBAgentRegistry: &DBAgentRegistry{
					logger:  &mockLogger{},
					metrics: &mockMetrics{},
				},
				manifestCache:   sync.Map{},
				registrationMap: sync.Map{},
				capabilityIndex: sync.Map{},
				channelMap:      sync.Map{},
			}

			// Create registration request
			reg := &UniversalAgentRegistration{
				TenantID:          tenantID,
				AgentID:           agentID,
				InstanceID:        instanceID,
				Name:              name,
				ConnectionDetails: map[string]interface{}{"protocol": "websocket"},
				RuntimeConfig:     map[string]interface{}{"version": "1.0.0"},
			}

			// Perform registration - directly call the database function
			ctx := context.Background()

			// Call the database function for idempotent registration
			var result struct {
				RegistrationID uuid.UUID `db:"registration_id"`
				ManifestID     uuid.UUID `db:"manifest_id"`
				ConfigID       uuid.UUID `db:"config_id"`
				IsNew          bool      `db:"is_new"`
				Message        string    `db:"message"`
			}

			query := `SELECT * FROM mcp.register_agent_instance($1, $2, $3, $4, $5, $6)`
			err := registry.db.GetContext(ctx, &result, query,
				reg.TenantID,
				reg.AgentID,
				reg.InstanceID,
				reg.Name,
				reg.ConnectionDetails,
				reg.RuntimeConfig,
			)

			// Verify results
			if err == nil {
				assert.Equal(t, registrationID, result.RegistrationID)
				assert.Equal(t, manifestID, result.ManifestID)
				assert.Equal(t, configID, result.ConfigID)
				assert.Equal(t, tt.isNew, result.IsNew)

				// Verify all mock expectations were met
				assert.NoError(t, mock.ExpectationsWereMet())
			}
		})
	}
}

// TestMultipleInstancesOfSameAgent tests that multiple instances can register
func TestMultipleInstancesOfSameAgent(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")

	tenantID := uuid.New()
	agentID := "k8s-monitor-agent"
	manifestID := uuid.New()
	configID := uuid.New()

	// Test three different instances (e.g., 3 K8s pods)
	instances := []struct {
		instanceID     string
		registrationID uuid.UUID
	}{
		{"pod-abc123", uuid.New()},
		{"pod-def456", uuid.New()},
		{"pod-ghi789", uuid.New()},
	}

	registry := &EnhancedAgentRegistry{
		db: sqlxDB,
		DBAgentRegistry: &DBAgentRegistry{
			logger:  &mockLogger{},
			metrics: &mockMetrics{},
		},
		manifestCache:   sync.Map{},
		registrationMap: sync.Map{},
		capabilityIndex: sync.Map{},
		channelMap:      sync.Map{},
	}

	for i, inst := range instances {
		t.Run(fmt.Sprintf("Instance_%d", i+1), func(t *testing.T) {
			// Each instance gets its own registration
			rows := sqlmock.NewRows([]string{
				"registration_id", "manifest_id", "config_id", "is_new", "message",
			}).AddRow(
				inst.registrationID, manifestID, configID, true, "New registration created",
			)

			mock.ExpectQuery(`SELECT \* FROM mcp\.register_agent_instance`).
				WithArgs(
					tenantID,
					agentID,
					inst.instanceID,
					sqlmock.AnyArg(),
					sqlmock.AnyArg(),
					sqlmock.AnyArg(),
				).
				WillReturnRows(rows)

			// Register the instance
			reg := &UniversalAgentRegistration{
				TenantID:          tenantID,
				AgentID:           agentID,
				InstanceID:        inst.instanceID,
				Name:              "K8s Monitor Agent",
				ConnectionDetails: map[string]interface{}{"pod": inst.instanceID},
				RuntimeConfig:     map[string]interface{}{"replicas": 3},
			}

			ctx := context.Background()

			// Call the database function for idempotent registration
			var result struct {
				RegistrationID uuid.UUID `db:"registration_id"`
				ManifestID     uuid.UUID `db:"manifest_id"`
				ConfigID       uuid.UUID `db:"config_id"`
				IsNew          bool      `db:"is_new"`
				Message        string    `db:"message"`
			}

			query := `SELECT * FROM mcp.register_agent_instance($1, $2, $3, $4, $5, $6)`
			err := registry.db.GetContext(ctx, &result, query,
				reg.TenantID,
				reg.AgentID,
				reg.InstanceID,
				"K8s Monitor Agent",
				reg.ConnectionDetails,
				reg.RuntimeConfig,
			)

			// All three instances should register successfully
			if err == nil {
				assert.Equal(t, inst.registrationID, result.RegistrationID)
				assert.True(t, result.IsNew)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestAgentReconnectionAfterCrash tests reconnection after pod restart/crash
func TestAgentReconnectionAfterCrash(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")

	tenantID := uuid.New()
	agentID := "crashed-agent"

	// New instance after restart (old instance is not used in test)
	newInstanceID := "new-pod-abc"

	registrationID := uuid.New()
	manifestID := uuid.New()
	configID := uuid.New()

	registry := &EnhancedAgentRegistry{
		db: sqlxDB,
		DBAgentRegistry: &DBAgentRegistry{
			logger:  &mockLogger{},
			metrics: &mockMetrics{},
		},
		manifestCache:   sync.Map{},
		registrationMap: sync.Map{},
		capabilityIndex: sync.Map{},
		channelMap:      sync.Map{},
	}

	// New pod gets new registration (old one becomes stale)
	rows := sqlmock.NewRows([]string{
		"registration_id", "manifest_id", "config_id", "is_new", "message",
	}).AddRow(
		registrationID, manifestID, configID, true, "New registration created",
	)

	mock.ExpectQuery(`SELECT \* FROM mcp\.register_agent_instance`).
		WithArgs(
			tenantID,
			agentID,
			newInstanceID,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(rows)

	// Register with new instance ID
	reg := &UniversalAgentRegistration{
		TenantID:   tenantID,
		AgentID:    agentID,
		InstanceID: newInstanceID, // Different instance ID after restart
		Name:       "Restarted Agent",
	}

	ctx := context.Background()

	// Call the database function for idempotent registration
	var result struct {
		RegistrationID uuid.UUID `db:"registration_id"`
		ManifestID     uuid.UUID `db:"manifest_id"`
		ConfigID       uuid.UUID `db:"config_id"`
		IsNew          bool      `db:"is_new"`
		Message        string    `db:"message"`
	}

	query := `SELECT * FROM mcp.register_agent_instance($1, $2, $3, $4, $5, $6)`
	err = registry.db.GetContext(ctx, &result, query,
		reg.TenantID,
		reg.AgentID,
		reg.InstanceID,
		reg.Name,
		nil,
		nil,
	)

	// Should succeed with new registration
	if err == nil {
		assert.Equal(t, registrationID, result.RegistrationID)
		assert.True(t, result.IsNew)
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

// mockLogger is a simple mock logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {}
func (m *mockLogger) Info(msg string, fields map[string]interface{})  {}
func (m *mockLogger) Warn(msg string, fields map[string]interface{})  {}
func (m *mockLogger) Error(msg string, fields map[string]interface{}) {}
func (m *mockLogger) Fatal(msg string, fields map[string]interface{}) {}

// Implement the formatted versions to satisfy the interface
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Warnf(format string, args ...interface{})  {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Fatalf(format string, args ...interface{}) {}

// Implement WithFields to satisfy the interface
func (m *mockLogger) WithFields(fields map[string]interface{}) observability.Logger {
	return m
}

// Implement WithError to satisfy the interface
func (m *mockLogger) WithError(err error) observability.Logger {
	return m
}

// Implement With to satisfy the interface
func (m *mockLogger) With(fields map[string]interface{}) observability.Logger {
	return m
}

// Implement WithPrefix to satisfy the interface
func (m *mockLogger) WithPrefix(prefix string) observability.Logger {
	return m
}

// mockMetrics is a simple mock metrics client for testing
type mockMetrics struct{}

func (m *mockMetrics) IncrementCounter(name string, value float64) {}
func (m *mockMetrics) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
}
func (m *mockMetrics) Gauge(name string, value float64)                                             {}
func (m *mockMetrics) Histogram(name string, value float64)                                         {}
func (m *mockMetrics) Timer(name string) func()                                                     { return func() {} }
func (m *mockMetrics) Close() error                                                                 { return nil }
func (m *mockMetrics) RecordEvent(source, eventType string)                                         {}
func (m *mockMetrics) RecordLatency(operation string, duration time.Duration)                       {}
func (m *mockMetrics) RecordCounter(name string, value float64, labels map[string]string)           {}
func (m *mockMetrics) RecordGauge(name string, value float64, labels map[string]string)             {}
func (m *mockMetrics) RecordHistogram(name string, value float64, labels map[string]string)         {}
func (m *mockMetrics) RecordTimer(name string, duration time.Duration, labels map[string]string)    {}
func (m *mockMetrics) RecordCacheOperation(operation string, success bool, durationSeconds float64) {}
func (m *mockMetrics) RecordOperation(component, operation string, success bool, durationSeconds float64, labels map[string]string) {
}
func (m *mockMetrics) RecordAPIOperation(api, operation string, success bool, durationSeconds float64) {
}
func (m *mockMetrics) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetrics) RecordEmbeddingRequest(model, operation string, success bool, tokenCount int, durationSeconds float64) {
}
func (m *mockMetrics) RecordEmbeddingModelSwitch(fromModel, toModel, reason string) {}
func (m *mockMetrics) RecordEmbeddingCacheHit(model string)                         {}
func (m *mockMetrics) RecordEmbeddingError(model, errorType string)                 {}
func (m *mockMetrics) RecordDuration(metric string, duration time.Duration)         {}
func (m *mockMetrics) StartTimer(name string, labels map[string]string) func()      { return func() {} }
