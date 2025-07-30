package encryption

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TenantKeyRepository defines the interface for tenant key storage
// This should be implemented in the pkg/repository package
type TenantKeyRepository interface {
	// CreateKey creates a new encryption key for a tenant
	CreateKey(ctx context.Context, key *TenantKey) error

	// GetKey retrieves a specific key by tenant ID and key ID
	GetKey(ctx context.Context, tenantID uuid.UUID, keyID string) (*TenantKey, error)

	// GetActiveKeys retrieves all active keys for a tenant
	GetActiveKeys(ctx context.Context, tenantID uuid.UUID) ([]TenantKey, error)

	// DeactivateKeys deactivates all keys for a tenant
	DeactivateKeys(ctx context.Context, tenantID uuid.UUID) error

	// GetExpiringKeys retrieves keys expiring within the given duration
	GetExpiringKeys(ctx context.Context, withinDuration time.Duration) ([]TenantKey, error)

	// UpdateKeyStatus updates the active status of a key
	UpdateKeyStatus(ctx context.Context, tenantID uuid.UUID, keyID string, isActive bool) error
}

// MockTenantKeyRepository is a mock implementation for testing
type MockTenantKeyRepository struct {
	keys map[string]*TenantKey // map[tenantID:keyID]*TenantKey
	mu   sync.RWMutex
}

// NewMockTenantKeyRepository creates a new mock repository
func NewMockTenantKeyRepository() *MockTenantKeyRepository {
	return &MockTenantKeyRepository{
		keys: make(map[string]*TenantKey),
	}
}

func (m *MockTenantKeyRepository) CreateKey(ctx context.Context, key *TenantKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := key.TenantID.String() + ":" + key.KeyID
	m.keys[keyStr] = key
	return nil
}

func (m *MockTenantKeyRepository) GetKey(ctx context.Context, tenantID uuid.UUID, keyID string) (*TenantKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyStr := tenantID.String() + ":" + keyID
	if key, exists := m.keys[keyStr]; exists {
		return key, nil
	}
	return nil, fmt.Errorf("key not found")
}

func (m *MockTenantKeyRepository) GetActiveKeys(ctx context.Context, tenantID uuid.UUID) ([]TenantKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var activeKeys []TenantKey
	tenantPrefix := tenantID.String() + ":"

	for k, v := range m.keys {
		if strings.HasPrefix(k, tenantPrefix) && v.IsActive {
			activeKeys = append(activeKeys, *v)
		}
	}

	return activeKeys, nil
}

func (m *MockTenantKeyRepository) DeactivateKeys(ctx context.Context, tenantID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenantPrefix := tenantID.String() + ":"
	for k, v := range m.keys {
		if strings.HasPrefix(k, tenantPrefix) {
			v.IsActive = false
		}
	}

	return nil
}

func (m *MockTenantKeyRepository) GetExpiringKeys(ctx context.Context, withinDuration time.Duration) ([]TenantKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expiringKeys []TenantKey
	expirationThreshold := time.Now().Add(withinDuration)

	for _, v := range m.keys {
		if v.IsActive && v.ExpiresAt.Before(expirationThreshold) {
			expiringKeys = append(expiringKeys, *v)
		}
	}

	return expiringKeys, nil
}

func (m *MockTenantKeyRepository) UpdateKeyStatus(ctx context.Context, tenantID uuid.UUID, keyID string, isActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := tenantID.String() + ":" + keyID
	if key, exists := m.keys[keyStr]; exists {
		key.IsActive = isActive
		return nil
	}
	return fmt.Errorf("key not found")
}
