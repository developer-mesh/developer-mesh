package core

import (
	"context"
	"os"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestNewEngine tests the creation of a new engine
func TestNewEngine(t *testing.T) {
	t.Run("Create with logger", func(t *testing.T) {
		// Arrange
		logger := new(MockLogger)

		// Act
		engine := NewEngine(logger)

		// Assert
		assert.NotNil(t, engine)
		assert.Equal(t, logger, engine.logger)
		assert.NotNil(t, engine.adapters)
		assert.Nil(t, engine.contextManager)
	})

	t.Run("Create with nil logger", func(t *testing.T) {
		// Act
		engine := NewEngine(nil)

		// Assert
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.logger)
		assert.NotNil(t, engine.adapters)
		assert.Nil(t, engine.contextManager)
	})
}

// TestRegisterAndGetAdapter tests the adapter registration and retrieval
func TestRegisterAndGetAdapter(t *testing.T) {
	// Setup
	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return()
	engine := NewEngine(logger)
	mockAdapter := &struct{ Name string }{"mock"}

	t.Run("Register and retrieve adapter", func(t *testing.T) {
		// Act - Register
		engine.RegisterAdapter("test", mockAdapter)

		// Act - Retrieve
		adapter, err := engine.GetAdapter("test")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, mockAdapter, adapter)
		logger.AssertCalled(t, "Info", "Registered adapter: test", mock.Anything)
	})

	t.Run("Get non-existent adapter", func(t *testing.T) {
		// Act
		adapter, err := engine.GetAdapter("nonexistent")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, adapter)
		assert.Contains(t, err.Error(), "adapter not found")
	})
}

// TestContextManagerOperations tests the context manager operations
func TestContextManagerOperations(t *testing.T) {
	// Setup
	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	engine := NewEngine(logger)
	mockContextManager := NewMockContextManager()

	t.Run("Set and get context manager", func(t *testing.T) {
		// Act - Set
		engine.SetContextManager(mockContextManager)

		// Act - Get
		cm := engine.GetContextManager()

		// Assert
		assert.Equal(t, mockContextManager, cm)
		logger.AssertCalled(t, "Info", "Set context manager", mock.Anything)
	})

	t.Run("Get non-initialized context manager", func(t *testing.T) {
		// Arrange
		engine := NewEngine(logger)

		// Act
		cm := engine.GetContextManager()

		// Assert
		assert.NotNil(t, cm, "Should return a mock implementation when not initialized")
		logger.AssertCalled(t, "Warn", "Context manager not initialized, returning mock implementation", mock.Anything)
	})

	t.Run("Context operations through context manager", func(t *testing.T) {
		// Arrange
		engine := NewEngine(logger)
		mockContextManager := NewMockContextManager()
		engine.SetContextManager(mockContextManager)
		ctx := context.Background()

		// Create a context through the manager
		testContext := &models.Context{
			ID:   "test-id",
			Name: "Test Context",
		}

		// Act - Create
		createdContext, err := engine.GetContextManager().CreateContext(ctx, testContext)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, testContext.ID, createdContext.ID)

		// Act - Get
		retrievedContext, err := engine.GetContextManager().GetContext(ctx, testContext.ID)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, testContext.ID, retrievedContext.ID)
	})
}

// TestHealth tests the Health method
func TestHealth(t *testing.T) {
	// Test health check functionality

	t.Run("Health with initialized context manager", func(t *testing.T) {
		// Arrange
		logger := new(MockLogger)
		logger.On("Info", mock.Anything, mock.Anything).Return()
		engine := NewEngine(logger)
		mockContextManager := NewMockContextManager()
		engine.SetContextManager(mockContextManager)

		// Act
		health := engine.Health()

		// Assert
		assert.Equal(t, "healthy", health["core_engine"])
		assert.Equal(t, "healthy", health["context_manager"])
	})

	t.Run("Health with nil context manager in mock mode", func(t *testing.T) {
		// Arrange
		logger := new(MockLogger)
		logger.On("Info", mock.Anything, mock.Anything).Return()
		engine := NewEngine(logger)
		engine.contextManager = nil

		// Override os.Getenv
		if err := os.Setenv("USE_MOCK_CONTEXT_MANAGER", "true"); err != nil {
			t.Fatalf("Failed to set env var: %v", err)
		}
		defer func() {
			if err := os.Setenv("USE_MOCK_CONTEXT_MANAGER", ""); err != nil {
				t.Errorf("Failed to reset env var: %v", err)
			}
		}()

		// Act
		health := engine.Health()

		// Assert
		assert.Equal(t, "healthy", health["core_engine"])
		assert.Equal(t, "healthy", health["context_manager"])
		assert.NotNil(t, engine.contextManager)
	})

	t.Run("Health with nil context manager not in mock mode", func(t *testing.T) {
		// Arrange
		logger := new(MockLogger)
		logger.On("Info", mock.Anything, mock.Anything).Return()
		engine := NewEngine(logger)
		engine.contextManager = nil

		// Override os.Getenv
		if err := os.Setenv("USE_MOCK_CONTEXT_MANAGER", ""); err != nil {
			t.Errorf("Failed to reset env var: %v", err)
		}

		// Act
		health := engine.Health()

		// Assert
		assert.Equal(t, "healthy", health["core_engine"])
		assert.Equal(t, "not_initialized", health["context_manager"])
		assert.Nil(t, engine.contextManager)
	})

	t.Run("Health with adapters", func(t *testing.T) {
		// Arrange
		logger := new(MockLogger)
		logger.On("Info", mock.Anything, mock.Anything).Return()
		engine := NewEngine(logger)
		mockContextManager := NewMockContextManager()
		engine.SetContextManager(mockContextManager)
		engine.RegisterAdapter("test", &struct{}{})

		// Act
		health := engine.Health()

		// Assert
		assert.Equal(t, "healthy", health["core_engine"])
		assert.Equal(t, "healthy", health["context_manager"])
		assert.Equal(t, "healthy", health["adapter_test"])
	})
}

// TestShutdown tests the Shutdown method
func TestShutdown(t *testing.T) {
	t.Run("Shutdown with adapters", func(t *testing.T) {
		// Setup
		logger := new(MockLogger)
		logger.On("Info", mock.Anything, mock.Anything).Return()
		logger.On("Warn", mock.Anything, mock.Anything).Return()

		engine := NewEngine(logger)

		// Create a mock adapter that implements Close
		mockCloser := &MockCloser{}
		mockCloser.On("Close").Return(nil)
		engine.RegisterAdapter("closer", mockCloser)

		// Create a mock adapter that implements Shutdown
		mockShutdowner := &MockShutdowner{}
		mockShutdowner.On("Shutdown", mock.Anything).Return(nil)
		engine.RegisterAdapter("shutdowner", mockShutdowner)

		// Act
		err := engine.Shutdown(context.Background())

		// Assert
		assert.NoError(t, err)
		mockCloser.AssertCalled(t, "Close")
		mockShutdowner.AssertCalled(t, "Shutdown", mock.Anything)
		logger.AssertCalled(t, "Info", "Shutting down engine", mock.Anything)
	})
}

// MockCloser mocks an adapter that implements Close
type MockCloser struct {
	mock.Mock
}

func (m *MockCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockShutdowner mocks an adapter that implements Shutdown
type MockShutdowner struct {
	mock.Mock
}

func (m *MockShutdowner) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// TestEngineIntegration tests the integration of engine with a real context manager
func TestEngineIntegration(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Error", mock.Anything, mock.Anything).Return()
	logger.On("WithPrefix", mock.Anything).Return(logger)

	metrics := new(MockMetricsClient)
	metrics.On("IncrementCounter", mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("IncrementCounterWithLabels", mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("RecordOperation", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	metrics.On("RecordOperationWithContext", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create an engine
	engine := NewEngine(logger)

	// Create a real context manager (with nil DB so it works in-memory)
	contextManager := NewContextManager(nil, logger, metrics, nil)

	// Set the context manager on the engine
	engine.SetContextManager(contextManager)

	// Test the context lifecycle through the engine's context manager
	ctx := context.Background()
	testContext := &models.Context{
		ID:   "integration-test-id",
		Name: "Integration Test Context",
	}

	// Act - Create
	createdContext, err := engine.GetContextManager().CreateContext(ctx, testContext)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, testContext.ID, createdContext.ID)

	// Act - Get
	retrievedContext, err := engine.GetContextManager().GetContext(ctx, testContext.ID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, testContext.ID, retrievedContext.ID)

	// Act - Update
	testContext.Name = "Updated Integration Test Context"
	updatedContext, err := engine.GetContextManager().UpdateContext(ctx, testContext.ID, testContext, nil)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Updated Integration Test Context", updatedContext.Name)

	// Act - Delete
	err = engine.GetContextManager().DeleteContext(ctx, testContext.ID)

	// Assert
	assert.NoError(t, err)
	_, err = engine.GetContextManager().GetContext(ctx, testContext.ID)
	assert.Error(t, err, "Context should not exist after deletion")
}
