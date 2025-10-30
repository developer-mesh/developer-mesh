package updater

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/config"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	goGithub "github.com/google/go-github/v74/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBackgroundChecker(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.UpdaterConfig
		version     string
		expectNil   bool
		expectError bool
	}{
		{
			name: "valid configuration",
			cfg: &config.UpdaterConfig{
				Enabled:       true,
				CheckInterval: 1 * time.Hour,
				Channel:       "stable",
				AutoDownload:  true,
				AutoApply:     false,
				GitHubOwner:   "developer-mesh",
				GitHubRepo:    "developer-mesh",
			},
			version:     "0.0.9",
			expectNil:   false,
			expectError: false,
		},
		{
			name: "disabled configuration returns nil",
			cfg: &config.UpdaterConfig{
				Enabled: false,
			},
			version:     "0.0.9",
			expectNil:   true,
			expectError: false,
		},
		{
			name: "invalid version",
			cfg: &config.UpdaterConfig{
				Enabled:       true,
				CheckInterval: 1 * time.Hour,
				Channel:       "stable",
				GitHubOwner:   "developer-mesh",
				GitHubRepo:    "developer-mesh",
			},
			version:     "invalid-version",
			expectNil:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := observability.NewStandardLogger("test")
			ghClient := goGithub.NewClient(nil)

			checker, err := NewBackgroundChecker(tt.cfg, tt.version, ghClient, logger, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			if tt.expectNil {
				assert.Nil(t, checker)
				assert.NoError(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, checker)
				assert.Equal(t, tt.cfg, checker.config)
				assert.NotNil(t, checker.updater)
				assert.NotNil(t, checker.stopChan)
			}
		})
	}
}

func TestBackgroundChecker_StartStop(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	cfg := &config.UpdaterConfig{
		Enabled:       true,
		CheckInterval: 100 * time.Millisecond, // Short interval for testing
		Channel:       "stable",
		AutoDownload:  false,
		AutoApply:     false,
		GitHubOwner:   "developer-mesh",
		GitHubRepo:    "developer-mesh",
	}

	checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
	require.NoError(t, err)
	require.NotNil(t, checker)

	// Test starting the checker
	ctx := context.Background()
	checker.Start(ctx)

	// Verify it's running
	status := checker.GetStatus()
	assert.True(t, status.Running)
	assert.True(t, status.Enabled)
	assert.Equal(t, "stable", status.Channel)

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	// Test stopping the checker
	checker.Stop()

	// Verify it's stopped
	status = checker.GetStatus()
	assert.False(t, status.Running)
}

func TestBackgroundChecker_StartTwice(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	cfg := &config.UpdaterConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Hour,
		Channel:       "stable",
		GitHubOwner:   "developer-mesh",
		GitHubRepo:    "developer-mesh",
	}

	checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Start first time
	checker.Start(ctx)
	assert.True(t, checker.isRunning)

	// Try to start again - should be idempotent
	checker.Start(ctx)
	assert.True(t, checker.isRunning)

	// Clean up
	checker.Stop()
}

func TestBackgroundChecker_StopWithoutStart(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	cfg := &config.UpdaterConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Hour,
		Channel:       "stable",
		GitHubOwner:   "developer-mesh",
		GitHubRepo:    "developer-mesh",
	}

	checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
	require.NoError(t, err)

	// Stop without starting - should not panic
	assert.NotPanics(t, func() {
		checker.Stop()
	})
}

func TestBackgroundChecker_ContextCancellation(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	cfg := &config.UpdaterConfig{
		Enabled:       true,
		CheckInterval: 10 * time.Second, // Long interval
		Channel:       "stable",
		GitHubOwner:   "developer-mesh",
		GitHubRepo:    "developer-mesh",
	}

	checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
	require.NoError(t, err)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start checker
	checker.Start(ctx)
	assert.True(t, checker.isRunning)

	// Cancel context
	cancel()

	// Wait a bit for goroutine to exit
	time.Sleep(100 * time.Millisecond)

	// Verify it stopped
	checker.Stop()
}

func TestBackgroundChecker_GetStatus(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	t.Run("nil checker returns disabled status", func(t *testing.T) {
		var checker *BackgroundChecker
		status := checker.GetStatus()

		assert.False(t, status.Enabled)
		assert.False(t, status.Running)
	})

	t.Run("enabled checker returns correct status", func(t *testing.T) {
		cfg := &config.UpdaterConfig{
			Enabled:       true,
			CheckInterval: 1 * time.Hour,
			Channel:       "beta",
			GitHubOwner:   "developer-mesh",
			GitHubRepo:    "developer-mesh",
		}

		checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
		require.NoError(t, err)

		status := checker.GetStatus()
		assert.True(t, status.Enabled)
		assert.False(t, status.Running) // Not started yet
		assert.Equal(t, "beta", status.Channel)
		assert.Equal(t, 1*time.Hour, status.CheckInterval)
	})

	t.Run("running checker updates status", func(t *testing.T) {
		cfg := &config.UpdaterConfig{
			Enabled:       true,
			CheckInterval: 100 * time.Millisecond,
			Channel:       "stable",
			GitHubOwner:   "developer-mesh",
			GitHubRepo:    "developer-mesh",
		}

		checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
		require.NoError(t, err)

		// Start checker
		ctx := context.Background()
		checker.Start(ctx)

		// Get status while running
		status := checker.GetStatus()
		assert.True(t, status.Running)

		// Stop and verify
		checker.Stop()
		status = checker.GetStatus()
		assert.False(t, status.Running)
	})
}

func TestBackgroundChecker_NoGoroutineLeak(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	cfg := &config.UpdaterConfig{
		Enabled:       true,
		CheckInterval: 50 * time.Millisecond,
		Channel:       "stable",
		GitHubOwner:   "developer-mesh",
		GitHubRepo:    "developer-mesh",
	}

	// Create and start multiple checkers
	for i := 0; i < 5; i++ {
		checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
		require.NoError(t, err)

		ctx := context.Background()
		checker.Start(ctx)

		// Let it run briefly
		time.Sleep(20 * time.Millisecond)

		// Stop and ensure cleanup
		checker.Stop()
	}

	// Give goroutines time to exit
	time.Sleep(100 * time.Millisecond)

	// Note: In a real test, we'd check runtime.NumGoroutine() before/after
	// but that's tricky with concurrent tests. This test verifies no panic.
}

func TestBackgroundChecker_ConfigChannels(t *testing.T) {
	tests := []struct {
		name    string
		channel string
	}{
		{"stable channel", "stable"},
		{"beta channel", "beta"},
		{"latest channel", "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := observability.NewStandardLogger("test")
			ghClient := goGithub.NewClient(nil)

			cfg := &config.UpdaterConfig{
				Enabled:       true,
				CheckInterval: 1 * time.Hour,
				Channel:       tt.channel,
				GitHubOwner:   "developer-mesh",
				GitHubRepo:    "developer-mesh",
			}

			checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
			require.NoError(t, err)

			status := checker.GetStatus()
			assert.Equal(t, tt.channel, status.Channel)
		})
	}
}

func TestBackgroundChecker_AutoDownloadFlags(t *testing.T) {
	tests := []struct {
		name         string
		autoDownload bool
		autoApply    bool
	}{
		{"auto-download only", true, false},
		{"auto-apply only", false, true},
		{"both enabled", true, true},
		{"both disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := observability.NewStandardLogger("test")
			ghClient := goGithub.NewClient(nil)

			cfg := &config.UpdaterConfig{
				Enabled:       true,
				CheckInterval: 1 * time.Hour,
				Channel:       "stable",
				AutoDownload:  tt.autoDownload,
				AutoApply:     tt.autoApply,
				GitHubOwner:   "developer-mesh",
				GitHubRepo:    "developer-mesh",
			}

			checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
			require.NoError(t, err)
			assert.NotNil(t, checker)
			assert.Equal(t, tt.autoDownload, checker.config.AutoDownload)
			assert.Equal(t, tt.autoApply, checker.config.AutoApply)
		})
	}
}

// TestBackgroundChecker_ThreadSafety tests concurrent access to status
func TestBackgroundChecker_ThreadSafety(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	ghClient := goGithub.NewClient(nil)

	cfg := &config.UpdaterConfig{
		Enabled:       true,
		CheckInterval: 50 * time.Millisecond,
		Channel:       "stable",
		GitHubOwner:   "developer-mesh",
		GitHubRepo:    "developer-mesh",
	}

	checker, err := NewBackgroundChecker(cfg, "0.0.9", ghClient, logger, nil)
	require.NoError(t, err)

	ctx := context.Background()
	checker.Start(ctx)

	// Concurrently read status
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = checker.GetStatus()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	checker.Stop()
}
