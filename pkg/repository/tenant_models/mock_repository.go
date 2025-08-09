package tenant_models

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockTenantModelsRepository is a mock implementation of TenantModelsRepository
type MockTenantModelsRepository struct {
	mock.Mock
}

// GetTenantModel mocks the GetTenantModel method
func (m *MockTenantModelsRepository) GetTenantModel(ctx context.Context, tenantID uuid.UUID, modelID uuid.UUID) (*TenantEmbeddingModel, error) {
	args := m.Called(ctx, tenantID, modelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TenantEmbeddingModel), args.Error(1)
}

// ListTenantModels mocks the ListTenantModels method
func (m *MockTenantModelsRepository) ListTenantModels(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]*TenantEmbeddingModel, error) {
	args := m.Called(ctx, tenantID, enabledOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*TenantEmbeddingModel), args.Error(1)
}

// GetDefaultModel mocks the GetDefaultModel method
func (m *MockTenantModelsRepository) GetDefaultModel(ctx context.Context, tenantID uuid.UUID) (*TenantEmbeddingModel, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TenantEmbeddingModel), args.Error(1)
}

// CreateTenantModel mocks the CreateTenantModel method
func (m *MockTenantModelsRepository) CreateTenantModel(ctx context.Context, model *TenantEmbeddingModel) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

// UpdateTenantModel mocks the UpdateTenantModel method
func (m *MockTenantModelsRepository) UpdateTenantModel(ctx context.Context, model *TenantEmbeddingModel) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

// DeleteTenantModel mocks the DeleteTenantModel method
func (m *MockTenantModelsRepository) DeleteTenantModel(ctx context.Context, tenantID uuid.UUID, modelID uuid.UUID) error {
	args := m.Called(ctx, tenantID, modelID)
	return args.Error(0)
}

// SetDefaultModel mocks the SetDefaultModel method
func (m *MockTenantModelsRepository) SetDefaultModel(ctx context.Context, tenantID uuid.UUID, modelID uuid.UUID) error {
	args := m.Called(ctx, tenantID, modelID)
	return args.Error(0)
}

// UpdatePriority mocks the UpdatePriority method
func (m *MockTenantModelsRepository) UpdatePriority(ctx context.Context, tenantID uuid.UUID, modelID uuid.UUID, priority int) error {
	args := m.Called(ctx, tenantID, modelID, priority)
	return args.Error(0)
}

// GetModelForRequest mocks the GetModelForRequest method
func (m *MockTenantModelsRepository) GetModelForRequest(ctx context.Context, tenantID uuid.UUID, agentID *uuid.UUID, taskType, requestedModel *string) (*ModelSelection, error) {
	args := m.Called(ctx, tenantID, agentID, taskType, requestedModel)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ModelSelection), args.Error(1)
}

// CheckUsageLimits mocks the CheckUsageLimits method
func (m *MockTenantModelsRepository) CheckUsageLimits(ctx context.Context, tenantID, modelID uuid.UUID) (*UsageStatus, error) {
	args := m.Called(ctx, tenantID, modelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UsageStatus), args.Error(1)
}

// UpdateUsageLimits mocks the UpdateUsageLimits method
func (m *MockTenantModelsRepository) UpdateUsageLimits(ctx context.Context, tenantID, modelID uuid.UUID, monthlyTokens, dailyTokens *int64, monthlyRequests *int) error {
	args := m.Called(ctx, tenantID, modelID, monthlyTokens, dailyTokens, monthlyRequests)
	return args.Error(0)
}

// BulkEnableModels mocks the BulkEnableModels method
func (m *MockTenantModelsRepository) BulkEnableModels(ctx context.Context, tenantID uuid.UUID, modelIDs []uuid.UUID) error {
	args := m.Called(ctx, tenantID, modelIDs)
	return args.Error(0)
}

// BulkDisableModels mocks the BulkDisableModels method
func (m *MockTenantModelsRepository) BulkDisableModels(ctx context.Context, tenantID uuid.UUID, modelIDs []uuid.UUID) error {
	args := m.Called(ctx, tenantID, modelIDs)
	return args.Error(0)
}
