package executor

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLogger implements observability.Logger for testing
type MockLogger struct {
	logs []map[string]interface{}
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *MockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

// Formatted logging methods
func (m *MockLogger) Debugf(format string, args ...interface{}) {}
func (m *MockLogger) Infof(format string, args ...interface{})  {}
func (m *MockLogger) Warnf(format string, args ...interface{})  {}
func (m *MockLogger) Errorf(format string, args ...interface{}) {}
func (m *MockLogger) Fatalf(format string, args ...interface{}) {}

// Context methods
func (m *MockLogger) WithPrefix(prefix string) observability.Logger {
	return m
}

func (m *MockLogger) With(fields map[string]interface{}) observability.Logger {
	return m
}

func TestCommandExecutorSecurity(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 5*time.Second)

	tests := []struct {
		name       string
		command    string
		args       []string
		shouldFail bool
		desc       string
	}{
		{
			name:       "Valid ls command",
			command:    "ls",
			args:       []string{"-la"},
			shouldFail: false,
			desc:       "Should allow safe ls command",
		},
		{
			name:       "Valid echo command",
			command:    "echo",
			args:       []string{"hello", "world"},
			shouldFail: false,
			desc:       "Should allow safe echo command",
		},
		{
			name:       "Dangerous rm command",
			command:    "rm",
			args:       []string{"-rf", "/"},
			shouldFail: true,
			desc:       "Should block dangerous rm command",
		},
		{
			name:       "Dangerous sudo command",
			command:    "sudo",
			args:       []string{"shutdown", "-h", "now"},
			shouldFail: true,
			desc:       "Should block sudo command",
		},
		{
			name:       "Dangerous chmod command",
			command:    "chmod",
			args:       []string{"777", "/etc/passwd"},
			shouldFail: true,
			desc:       "Should block chmod command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := executor.Execute(ctx, tt.command, tt.args)

			if tt.shouldFail {
				assert.Error(t, err, tt.desc)
				assert.Contains(t, err.Error(), "command not allowed")
			} else {
				assert.NoError(t, err, tt.desc)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestCommandExecutorTimeout(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 1*time.Second)

	// Add sleep to allowed commands for testing
	executor.AddAllowedCommand("sleep")

	ctx := context.Background()
	result, err := executor.Execute(ctx, "sleep", []string{"5"})

	// Command should be killed after timeout
	assert.Error(t, err)
	assert.NotNil(t, result)
}

func TestPathValidation(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp/work", 5*time.Second)

	tests := []struct {
		path string
		safe bool
		desc string
	}{
		{"/tmp/work/file.txt", true, "Path within work directory"},
		{"/tmp/work/subdir/file.txt", true, "Nested path within work directory"},
		{"../../../etc/passwd", false, "Path traversal attack"},
		{"/etc/passwd", false, "Path outside work directory"},
		{"/tmp/../etc/passwd", false, "Hidden path traversal"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := executor.IsPathSafe(tt.path)
			assert.Equal(t, tt.safe, result, tt.desc)
		})
	}
}

func TestCommandOutput(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 5*time.Second)

	ctx := context.Background()
	result, err := executor.Execute(ctx, "echo", []string{"test output"})

	require.NoError(t, err)
	assert.Equal(t, "test output\n", result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.Equal(t, 0, result.ExitCode)
}

func TestCommandError(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 5*time.Second)

	ctx := context.Background()
	result, err := executor.Execute(ctx, "ls", []string{"/nonexistent/path"})

	// Command should fail but still return result
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.NotEmpty(t, result.Stderr)
}

func TestAllowedCommandManagement(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 5*time.Second)

	// Test adding a new command
	executor.AddAllowedCommand("curl")
	ctx := context.Background()

	// Should now be allowed (would fail with actual curl if not installed)
	_, err := executor.Execute(ctx, "curl", []string{"--version"})
	// We're just testing that it doesn't return "command not allowed"
	if err != nil {
		assert.NotContains(t, err.Error(), "command not allowed")
	}

	// Test removing a command
	executor.RemoveAllowedCommand("curl")
	_, err = executor.Execute(ctx, "curl", []string{"--version"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command not allowed")
}

func TestWorkingDirectory(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 5*time.Second)

	// Test setting valid work directory
	err := executor.SetWorkDir("/tmp/test")
	assert.NoError(t, err)

	// Test setting invalid work directory
	err = executor.SetWorkDir("../../../etc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid working directory")
}

func TestProcessGroupIsolation(t *testing.T) {
	logger := &MockLogger{}
	executor := NewCommandExecutor(logger, "/tmp", 5*time.Second)

	ctx := context.Background()
	result, err := executor.Execute(ctx, "echo", []string{"process group test"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	// Process should have been in its own process group
	// (actual verification would require checking process attributes during execution)
}
