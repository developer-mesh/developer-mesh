package mocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockCache is a mock implementation of the Cache interface for testing
type MockCache struct {
	mock.Mock
}

// Get mocks the Get method
func (m *MockCache) Get(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

// Set mocks the Set method
func (m *MockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

// Delete mocks the Delete method
func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// Exists mocks the Exists method
func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

// Flush mocks the Flush method
func (m *MockCache) Flush(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Close mocks the Close method
func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Size mocks the Size method
func (m *MockCache) Size() int {
	args := m.Called()
	return args.Int(0)
}
