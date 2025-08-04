package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
)

// MockAgentRepository provides an in-memory implementation of agent repository for testing
type MockAgentRepository struct {
	mu     sync.RWMutex
	agents map[string]*models.Agent
	err    error // For simulating errors
}

// NewMockAgentRepository creates a new mock agent repository
func NewMockAgentRepository() *MockAgentRepository {
	return &MockAgentRepository{
		agents: make(map[string]*models.Agent),
	}
}

// SetError sets an error to be returned by all methods
func (m *MockAgentRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// Create creates a new agent
func (m *MockAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}
	now := time.Now()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	m.agents[agent.ID] = agent
	return nil
}

// Get retrieves an agent by ID
func (m *MockAgentRepository) Get(ctx context.Context, id string) (*models.Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	agent, exists := m.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent not found")
	}

	return agent, nil
}

// Update updates an existing agent
func (m *MockAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.agents[agent.ID]; !exists {
		return fmt.Errorf("agent not found")
	}

	agent.UpdatedAt = time.Now()
	m.agents[agent.ID] = agent
	return nil
}

// Delete removes an agent
func (m *MockAgentRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.agents[id]; !exists {
		return fmt.Errorf("agent not found")
	}

	delete(m.agents, id)
	return nil
}

// List returns all agents for a tenant
func (m *MockAgentRepository) List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.Agent, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, 0, m.err
	}

	var agents []*models.Agent
	for _, agent := range m.agents {
		if agent.TenantID == tenantID {
			agents = append(agents, agent)
		}
	}

	total := len(agents)

	// Apply pagination
	start := offset
	if start > len(agents) {
		start = len(agents)
	}

	end := start + limit
	if end > len(agents) {
		end = len(agents)
	}

	return agents[start:end], total, nil
}

// MockModelRepository provides an in-memory implementation of model repository for testing
type MockModelRepository struct {
	mu     sync.RWMutex
	models map[string]*models.Model
	err    error
}

// NewMockModelRepository creates a new mock model repository
func NewMockModelRepository() *MockModelRepository {
	return &MockModelRepository{
		models: make(map[string]*models.Model),
	}
}

// SetError sets an error to be returned by all methods
func (m *MockModelRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// Create creates a new model
func (m *MockModelRepository) Create(ctx context.Context, model *models.Model) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if model.ID == "" {
		model.ID = uuid.New().String()
	}
	now := time.Now()
	model.CreatedAt = &now
	model.UpdatedAt = &now

	m.models[model.ID] = model
	return nil
}

// Get retrieves a model by ID
func (m *MockModelRepository) Get(ctx context.Context, id string) (*models.Model, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	model, exists := m.models[id]
	if !exists {
		return nil, fmt.Errorf("model not found")
	}

	return model, nil
}

// Update updates an existing model
func (m *MockModelRepository) Update(ctx context.Context, model *models.Model) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.models[model.ID]; !exists {
		return fmt.Errorf("model not found")
	}

	now := time.Now()
	model.UpdatedAt = &now
	m.models[model.ID] = model
	return nil
}

// Delete removes a model
func (m *MockModelRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.models[id]; !exists {
		return fmt.Errorf("model not found")
	}

	delete(m.models, id)
	return nil
}

// List returns all models
func (m *MockModelRepository) List(ctx context.Context, limit, offset int) ([]*models.Model, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, 0, m.err
	}

	var models []*models.Model
	for _, model := range m.models {
		models = append(models, model)
	}

	total := len(models)

	// Apply pagination
	start := offset
	if start > len(models) {
		start = len(models)
	}

	end := start + limit
	if end > len(models) {
		end = len(models)
	}

	return models[start:end], total, nil
}

// GetByName retrieves a model by name
func (m *MockModelRepository) GetByName(ctx context.Context, name string) (*models.Model, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	for _, model := range m.models {
		if model.Name == name {
			return model, nil
		}
	}

	return nil, fmt.Errorf("model not found")
}

// MockDynamicToolRepository provides an in-memory implementation of dynamic tool repository for testing
type MockDynamicToolRepository struct {
	mu    sync.RWMutex
	tools map[string]*models.DynamicTool
	err   error
}

// NewMockDynamicToolRepository creates a new mock dynamic tool repository
func NewMockDynamicToolRepository() *MockDynamicToolRepository {
	return &MockDynamicToolRepository{
		tools: make(map[string]*models.DynamicTool),
	}
}

// SetError sets an error to be returned by all methods
func (m *MockDynamicToolRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// Create creates a new dynamic tool
func (m *MockDynamicToolRepository) Create(ctx context.Context, tool *models.DynamicTool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if tool.ID == "" {
		tool.ID = uuid.New().String()
	}
	tool.CreatedAt = time.Now()
	tool.UpdatedAt = time.Now()

	m.tools[tool.ID] = tool
	return nil
}

// Get retrieves a dynamic tool by ID
func (m *MockDynamicToolRepository) Get(ctx context.Context, id string) (*models.DynamicTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	tool, exists := m.tools[id]
	if !exists {
		return nil, fmt.Errorf("tool not found")
	}

	return tool, nil
}

// Update updates an existing dynamic tool
func (m *MockDynamicToolRepository) Update(ctx context.Context, tool *models.DynamicTool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.tools[tool.ID]; !exists {
		return fmt.Errorf("tool not found")
	}

	tool.UpdatedAt = time.Now()
	m.tools[tool.ID] = tool
	return nil
}

// Delete removes a dynamic tool
func (m *MockDynamicToolRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	if _, exists := m.tools[id]; !exists {
		return fmt.Errorf("tool not found")
	}

	delete(m.tools, id)
	return nil
}

// List returns all dynamic tools for a tenant
func (m *MockDynamicToolRepository) List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.DynamicTool, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, 0, m.err
	}

	var tools []*models.DynamicTool
	for _, tool := range m.tools {
		if tool.TenantID == tenantID.String() {
			tools = append(tools, tool)
		}
	}

	total := len(tools)

	// Apply pagination
	start := offset
	if start > len(tools) {
		start = len(tools)
	}

	end := start + limit
	if end > len(tools) {
		end = len(tools)
	}

	return tools[start:end], total, nil
}

// GetByName retrieves a dynamic tool by name
func (m *MockDynamicToolRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.DynamicTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return nil, m.err
	}

	for _, tool := range m.tools {
		if tool.TenantID == tenantID.String() && tool.ToolName == name {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("tool not found")
}
