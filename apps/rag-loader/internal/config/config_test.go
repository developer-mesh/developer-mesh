package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	// Clear any environment variables that might interfere
	clearEnvVars()

	// Set minimal required environment variables
	_ = os.Setenv("DATABASE_HOST", "localhost")
	_ = os.Setenv("DATABASE_NAME", "test_db")
	_ = os.Setenv("REDIS_ADDR", "localhost:6379")
	defer clearEnvVars()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check service defaults
	assert.Equal(t, 8084, cfg.Service.Port)
	assert.Equal(t, 9094, cfg.Service.MetricsPort)
	assert.Equal(t, 30*time.Second, cfg.Service.HealthCheckInterval)
	assert.Equal(t, 30*time.Second, cfg.Service.ShutdownTimeout)
	assert.Equal(t, "info", cfg.Service.LogLevel)

	// Check database defaults
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, 10, cfg.Database.MaxConns)
	assert.Equal(t, 5, cfg.Database.MaxIdleConns)

	// Check Redis defaults
	assert.Equal(t, "localhost:6379", cfg.Redis.Address)
	assert.Equal(t, 0, cfg.Redis.Database)
	assert.Equal(t, 3, cfg.Redis.MaxRetries)
	assert.Equal(t, 5*time.Second, cfg.Redis.DialTimeout)
	assert.Equal(t, 10, cfg.Redis.PoolSize)

	// Check processing defaults
	assert.Equal(t, "fixed", cfg.Processing.ChunkingStrategy)
	assert.Equal(t, 500, cfg.Processing.ChunkSize)
	assert.Equal(t, 50, cfg.Processing.ChunkOverlap)
	assert.Equal(t, 10, cfg.Processing.Embedding.BatchSize)
	assert.Equal(t, 3, cfg.Scheduler.MaxConcurrentJobs)

	// Check scheduler defaults
	assert.Equal(t, "*/30 * * * *", cfg.Scheduler.DefaultSchedule)
	assert.Equal(t, 10*time.Minute, cfg.Scheduler.JobTimeout)
	assert.True(t, cfg.Scheduler.EnableAPI)
	assert.True(t, cfg.Scheduler.EnableEvents)
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	// Clear environment
	clearEnvVars()

	// Set environment variables
	_ = os.Setenv("RAG_LOADER_PORT", "9090")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("DATABASE_HOST", "db.example.com")
	_ = os.Setenv("DATABASE_PORT", "5433")
	_ = os.Setenv("DATABASE_NAME", "prod_db")
	_ = os.Setenv("DATABASE_USER", "admin")
	_ = os.Setenv("DATABASE_PASSWORD", "secret")
	_ = os.Setenv("REDIS_ADDR", "redis.example.com:6380")
	_ = os.Setenv("DEFAULT_SCHEDULE", "0 * * * *")
	defer clearEnvVars()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check overridden values
	assert.Equal(t, 9090, cfg.Service.Port)
	assert.Equal(t, "debug", cfg.Service.LogLevel)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "prod_db", cfg.Database.Database)
	assert.Equal(t, "admin", cfg.Database.Username)
	assert.Equal(t, "secret", cfg.Database.Password)
	assert.Equal(t, "redis.example.com:6380", cfg.Redis.Address)
	assert.Equal(t, "0 * * * *", cfg.Scheduler.DefaultSchedule)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			setup: func() {
				_ = os.Setenv("DATABASE_HOST", "localhost")
				_ = os.Setenv("DATABASE_NAME", "test_db")
				_ = os.Setenv("REDIS_ADDR", "localhost:6379")
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			setup: func() {
				_ = os.Setenv("RAG_LOADER_PORT", "99999")
				_ = os.Setenv("DATABASE_HOST", "localhost")
				_ = os.Setenv("DATABASE_NAME", "test_db")
				_ = os.Setenv("REDIS_ADDR", "localhost:6379")
			},
			wantErr: true,
			errMsg:  "invalid service port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnvVars()
			defer clearEnvVars()

			// Setup test environment
			tt.setup()

			// Load configuration
			cfg, err := Load()

			if tt.wantErr {
				assert.Error(t, err)
				if err != nil && tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

// clearEnvVars clears relevant environment variables
func clearEnvVars() {
	envVars := []string{
		"RAG_LOADER_PORT",
		"LOG_LEVEL",
		"DATABASE_HOST",
		"DATABASE_PORT",
		"DATABASE_NAME",
		"DATABASE_USER",
		"DATABASE_PASSWORD",
		"DATABASE_SSL_MODE",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"DEFAULT_SCHEDULE",
	}

	for _, v := range envVars {
		_ = os.Unsetenv(v)
	}
}
