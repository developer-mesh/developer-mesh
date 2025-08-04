package testutil

import (
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
)

// TestDataFactory provides helper functions to create test data with consistent defaults
type TestDataFactory struct{}

// NewTestDataFactory creates a new instance of TestDataFactory
func NewTestDataFactory() *TestDataFactory {
	return &TestDataFactory{}
}

// CreateTestAgent creates a test agent with default values
func (f *TestDataFactory) CreateTestAgent(opts ...AgentOption) *models.Agent {
	agent := &models.Agent{
		ID:        uuid.New().String(),
		TenantID:  TestTenantID,
		Name:      "Test Agent",
		Type:      "standard",
		Status:    "active",
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		opt(agent)
	}

	return agent
}

// CreateTestModel creates a test model with default values
func (f *TestDataFactory) CreateTestModel(opts ...ModelOption) *models.Model {
	now := time.Now()
	model := &models.Model{
		ID:        uuid.New().String(),
		TenantID:  TestTenantIDString(),
		Name:      "Test Model",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Apply options
	for _, opt := range opts {
		opt(model)
	}

	return model
}

// CreateTestDynamicTool creates a test dynamic tool with default values
func (f *TestDataFactory) CreateTestDynamicTool(opts ...DynamicToolOption) *models.DynamicTool {
	tool := &models.DynamicTool{
		ID:          uuid.New().String(),
		TenantID:    TestTenantIDString(),
		ToolName:    "test-tool",
		DisplayName: "Test Tool",
		BaseURL:     "https://api.github.com",
		Config: map[string]interface{}{
			"timeout": 30,
		},
		AuthType:  "bearer",
		Status:    "active",
		Provider:  "github",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// CreateTestWebhookConfig creates a test webhook configuration with default values
func (f *TestDataFactory) CreateTestWebhookConfig(opts ...WebhookConfigOption) *models.WebhookConfig {
	webhookConfig := &models.WebhookConfig{
		ID:               uuid.New(),
		OrganizationName: "test-org",
		WebhookSecret:    "test-secret-32-characters-minimum",
		Enabled:          true,
		AllowedEvents:    []string{"*"},
		Metadata:         make(map[string]interface{}),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		opt(webhookConfig)
	}

	return webhookConfig
}

// Option types for customizing test data

// AgentOption is a function that modifies an Agent
type AgentOption func(*models.Agent)

// WithAgentID sets a specific ID for the agent
func WithAgentID(id string) AgentOption {
	return func(a *models.Agent) {
		a.ID = id
	}
}

// WithAgentName sets a specific name for the agent
func WithAgentName(name string) AgentOption {
	return func(a *models.Agent) {
		a.Name = name
	}
}

// WithAgentTenantID sets a specific tenant ID for the agent
func WithAgentTenantID(tenantID uuid.UUID) AgentOption {
	return func(a *models.Agent) {
		a.TenantID = tenantID
	}
}

// WithAgentMetadata sets metadata for the agent
func WithAgentMetadata(metadata map[string]interface{}) AgentOption {
	return func(a *models.Agent) {
		a.Metadata = metadata
	}
}

// ModelOption is a function that modifies a Model
type ModelOption func(*models.Model)

// WithModelID sets a specific ID for the model
func WithModelID(id string) ModelOption {
	return func(m *models.Model) {
		m.ID = id
	}
}

// WithModelName sets a specific name for the model
func WithModelName(name string) ModelOption {
	return func(m *models.Model) {
		m.Name = name
	}
}

// WithModelTenantID sets a specific tenant ID for the model
func WithModelTenantID(tenantID string) ModelOption {
	return func(m *models.Model) {
		m.TenantID = tenantID
	}
}

// DynamicToolOption is a function that modifies a DynamicTool
type DynamicToolOption func(*models.DynamicTool)

// WithDynamicToolID sets a specific ID for the dynamic tool
func WithDynamicToolID(id string) DynamicToolOption {
	return func(t *models.DynamicTool) {
		t.ID = id
	}
}

// WithDynamicToolName sets a specific name for the dynamic tool
func WithDynamicToolName(name string) DynamicToolOption {
	return func(t *models.DynamicTool) {
		t.ToolName = name
		t.DisplayName = name
	}
}

// WithDynamicToolTenantID sets a specific tenant ID for the dynamic tool
func WithDynamicToolTenantID(tenantID string) DynamicToolOption {
	return func(t *models.DynamicTool) {
		t.TenantID = tenantID
	}
}

// WithDynamicToolStatus sets the status for the dynamic tool
func WithDynamicToolStatus(status string) DynamicToolOption {
	return func(t *models.DynamicTool) {
		t.Status = status
	}
}

// WebhookConfigOption is a function that modifies a WebhookConfig
type WebhookConfigOption func(*models.WebhookConfig)

// WithWebhookConfigID sets a specific ID for the webhook configuration
func WithWebhookConfigID(id uuid.UUID) WebhookConfigOption {
	return func(w *models.WebhookConfig) {
		w.ID = id
	}
}

// WithWebhookConfigOrg sets a specific organization name
func WithWebhookConfigOrg(orgName string) WebhookConfigOption {
	return func(w *models.WebhookConfig) {
		w.OrganizationName = orgName
	}
}

// WithWebhookConfigSecret sets a specific webhook secret
func WithWebhookConfigSecret(secret string) WebhookConfigOption {
	return func(w *models.WebhookConfig) {
		w.WebhookSecret = secret
	}
}

// Global instance for convenience
var Factory = NewTestDataFactory()
