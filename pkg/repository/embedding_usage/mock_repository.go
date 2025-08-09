package embedding_usage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockEmbeddingUsageRepository is a mock implementation of EmbeddingUsageRepository
type MockEmbeddingUsageRepository struct {
	mock.Mock
}

// TrackUsage mocks the TrackUsage method
func (m *MockEmbeddingUsageRepository) TrackUsage(ctx context.Context, record *UsageRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

// GetUsageSummary mocks the GetUsageSummary method
func (m *MockEmbeddingUsageRepository) GetUsageSummary(ctx context.Context, tenantID uuid.UUID, start, end time.Time) (*UsageSummary, error) {
	args := m.Called(ctx, tenantID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UsageSummary), args.Error(1)
}

// GetUsageByModel mocks the GetUsageByModel method
func (m *MockEmbeddingUsageRepository) GetUsageByModel(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]ModelUsage, error) {
	args := m.Called(ctx, tenantID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ModelUsage), args.Error(1)
}

// GetUsageByAgent mocks the GetUsageByAgent method
func (m *MockEmbeddingUsageRepository) GetUsageByAgent(ctx context.Context, agentID uuid.UUID, start, end time.Time) (*UsageSummary, error) {
	args := m.Called(ctx, agentID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UsageSummary), args.Error(1)
}

// GetCurrentMonthUsage mocks the GetCurrentMonthUsage method
func (m *MockEmbeddingUsageRepository) GetCurrentMonthUsage(ctx context.Context, tenantID uuid.UUID) (*UsageSummary, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UsageSummary), args.Error(1)
}

// GetCurrentDayUsage mocks the GetCurrentDayUsage method
func (m *MockEmbeddingUsageRepository) GetCurrentDayUsage(ctx context.Context, tenantID uuid.UUID) (*UsageSummary, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UsageSummary), args.Error(1)
}

// BulkTrackUsage mocks the BulkTrackUsage method
func (m *MockEmbeddingUsageRepository) BulkTrackUsage(ctx context.Context, records []*UsageRecord) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

// PurgeOldRecords mocks the PurgeOldRecords method
func (m *MockEmbeddingUsageRepository) PurgeOldRecords(ctx context.Context, retentionDays int) error {
	args := m.Called(ctx, retentionDays)
	return args.Error(0)
}
