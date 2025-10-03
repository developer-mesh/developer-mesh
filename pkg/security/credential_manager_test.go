package security

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockCredentialRepository is a mock implementation of CredentialRepository
type MockCredentialRepository struct {
	mock.Mock
}

func (m *MockCredentialRepository) Create(ctx context.Context, cred *repository.TenantCredential) error {
	args := m.Called(ctx, cred)
	return args.Error(0)
}

func (m *MockCredentialRepository) Get(ctx context.Context, id uuid.UUID) (*repository.TenantCredential, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.TenantCredential), args.Error(1)
}

func (m *MockCredentialRepository) GetByTenantAndName(ctx context.Context, tenantID uuid.UUID, name string) (*repository.TenantCredential, error) {
	args := m.Called(ctx, tenantID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.TenantCredential), args.Error(1)
}

func (m *MockCredentialRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, includeInactive bool) ([]*repository.TenantCredential, error) {
	args := m.Called(ctx, tenantID, includeInactive)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.TenantCredential), args.Error(1)
}

func (m *MockCredentialRepository) ListByTool(ctx context.Context, tenantID uuid.UUID, toolID uuid.UUID) ([]*repository.TenantCredential, error) {
	args := m.Called(ctx, tenantID, toolID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.TenantCredential), args.Error(1)
}

func (m *MockCredentialRepository) Update(ctx context.Context, cred *repository.TenantCredential) error {
	args := m.Called(ctx, cred)
	return args.Error(0)
}

func (m *MockCredentialRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCredentialRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCredentialRepository) ListExpiring(ctx context.Context, within time.Duration) ([]*repository.TenantCredential, error) {
	args := m.Called(ctx, within)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.TenantCredential), args.Error(1)
}

func (m *MockCredentialRepository) ListExpired(ctx context.Context) ([]*repository.TenantCredential, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.TenantCredential), args.Error(1)
}

func (m *MockCredentialRepository) ListUnusedSince(ctx context.Context, since time.Time) ([]*repository.TenantCredential, error) {
	args := m.Called(ctx, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.TenantCredential), args.Error(1)
}

func setupCredentialManager(t *testing.T) (*CredentialManager, *MockCredentialRepository, *EncryptionService) {
	encryption := NewEncryptionService("test-master-key-for-testing")
	mockRepo := new(MockCredentialRepository)
	audit := auth.NewAuditLogger(observability.NewStandardLogger("test"))
	logger := observability.NewStandardLogger("test")

	manager := NewCredentialManager(encryption, mockRepo, audit, logger)

	return manager, mockRepo, encryption
}

func TestDefaultCredentialConfig(t *testing.T) {
	config := DefaultCredentialConfig()

	assert.Equal(t, 90*24*time.Hour, config.DefaultExpiry)
	assert.Equal(t, 30*24*time.Hour, config.RotationInterval)
	assert.Equal(t, 7*24*time.Hour, config.ExpiryWarningThreshold)
	assert.Equal(t, 12, config.MinPasswordLength)
	assert.True(t, config.RequireStrongPasswords)
	assert.Equal(t, 30*24*time.Hour, config.InactivityThreshold)
}

func TestCreateCredential(t *testing.T) {
	manager, mockRepo, _ := setupCredentialManager(t)
	ctx := context.Background()

	tenantID := uuid.New()
	toolID := uuid.New()

	tests := []struct {
		name      string
		credType  string
		value     string
		expectErr bool
	}{
		{
			name:      "valid API key",
			credType:  "api_key",
			value:     "test-api-key-1234567890",
			expectErr: false,
		},
		{
			name:      "valid basic auth",
			credType:  "basic",
			value:     "username:StrongP@ssw0rd123!",
			expectErr: false,
		},
		{
			name:      "valid OAuth2 token",
			credType:  "oauth2",
			value:     "ya29.a0AfH6SMBxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			expectErr: false,
		},
		{
			name:      "invalid - empty value",
			credType:  "api_key",
			value:     "",
			expectErr: true,
		},
		{
			name:      "invalid - short API key",
			credType:  "api_key",
			value:     "short",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expectErr {
				mockRepo.On("Create", ctx, mock.AnythingOfType("*repository.TenantCredential")).
					Return(nil).Once()
			}

			expiresAt := time.Now().Add(90 * 24 * time.Hour)
			cred, err := manager.CreateCredential(ctx, tenantID, &toolID, "test-cred", tt.credType, tt.value, &expiresAt)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, cred)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cred)
				assert.Equal(t, tenantID, cred.TenantID)
				assert.Equal(t, tt.credType, cred.CredentialType)
				assert.NotEmpty(t, cred.EncryptedValue)
			}
		})
	}
}

func TestGetCredential(t *testing.T) {
	manager, mockRepo, encryption := setupCredentialManager(t)
	ctx := context.Background()

	tenantID := uuid.New()
	credID := uuid.New()
	originalValue := "test-api-key-1234567890"

	// Encrypt the value
	encryptedValue, err := encryption.EncryptCredential(originalValue, tenantID.String())
	require.NoError(t, err)

	tests := []struct {
		name      string
		setupMock func()
		expectErr bool
		errMsg    string
	}{
		{
			name: "success - valid active credential",
			setupMock: func() {
				mockRepo.On("Get", ctx, credID).Return(&repository.TenantCredential{
					ID:             credID,
					TenantID:       tenantID,
					CredentialType: "api_key",
					EncryptedValue: string(encryptedValue),
					IsActive:       true,
					ExpiresAt:      nil,
				}, nil).Once()
				mockRepo.On("UpdateLastUsed", ctx, credID).Return(nil).Once()
			},
			expectErr: false,
		},
		{
			name: "error - expired credential",
			setupMock: func() {
				expired := time.Now().Add(-24 * time.Hour)
				mockRepo.On("Get", ctx, credID).Return(&repository.TenantCredential{
					ID:             credID,
					TenantID:       tenantID,
					CredentialType: "api_key",
					EncryptedValue: string(encryptedValue),
					IsActive:       true,
					ExpiresAt:      &expired,
				}, nil).Once()
			},
			expectErr: true,
			errMsg:    "expired",
		},
		{
			name: "error - inactive credential",
			setupMock: func() {
				mockRepo.On("Get", ctx, credID).Return(&repository.TenantCredential{
					ID:             credID,
					TenantID:       tenantID,
					CredentialType: "api_key",
					EncryptedValue: string(encryptedValue),
					IsActive:       false,
					ExpiresAt:      nil,
				}, nil).Once()
			},
			expectErr: true,
			errMsg:    "inactive",
		},
		{
			name: "error - not found",
			setupMock: func() {
				mockRepo.On("Get", ctx, credID).Return(nil, sql.ErrNoRows).Once()
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			value, err := manager.GetCredential(ctx, credID)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, originalValue, value)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestRotateCredential(t *testing.T) {
	manager, mockRepo, encryption := setupCredentialManager(t)
	ctx := context.Background()

	tenantID := uuid.New()
	credID := uuid.New()
	oldValue := "old-api-key-1234567890"
	newValue := "new-api-key-0987654321"

	oldEncrypted, err := encryption.EncryptCredential(oldValue, tenantID.String())
	require.NoError(t, err)

	mockRepo.On("Get", ctx, credID).Return(&repository.TenantCredential{
		ID:             credID,
		TenantID:       tenantID,
		CredentialType: "api_key",
		EncryptedValue: string(oldEncrypted),
		IsActive:       true,
	}, nil)

	mockRepo.On("Update", ctx, mock.AnythingOfType("*repository.TenantCredential")).
		Return(nil)

	newExpiresAt := time.Now().Add(90 * 24 * time.Hour)
	err = manager.RotateCredential(ctx, credID, newValue, &newExpiresAt)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestValidateCredential_APIKey(t *testing.T) {
	manager, _, _ := setupCredentialManager(t)

	tests := []struct {
		name      string
		apiKey    string
		expectErr bool
	}{
		{
			name:      "valid API key",
			apiKey:    "test-api-key-1234567890",
			expectErr: false,
		},
		{
			name:      "valid with underscores and hyphens",
			apiKey:    "test_api-key_1234-5678",
			expectErr: false,
		},
		{
			name:      "too short",
			apiKey:    "short",
			expectErr: true,
		},
		{
			name:      "invalid characters",
			apiKey:    "test-api-key-with-invalid-chars!@#",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateCredential("api_key", tt.apiKey)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCredential_BasicAuth(t *testing.T) {
	manager, _, _ := setupCredentialManager(t)

	tests := []struct {
		name      string
		value     string
		expectErr bool
	}{
		{
			name:      "valid basic auth",
			value:     "username:StrongP@ssw0rd123!",
			expectErr: false,
		},
		{
			name:      "weak password",
			value:     "username:weak",
			expectErr: true,
		},
		{
			name:      "no colon separator",
			value:     "usernamepassword",
			expectErr: true,
		},
		{
			name:      "empty username",
			value:     ":password123",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateCredential("basic", tt.value)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	manager, _, _ := setupCredentialManager(t)

	tests := []struct {
		name      string
		password  string
		expectErr bool
	}{
		{
			name:      "strong password",
			password:  "StrongP@ssw0rd123!",
			expectErr: false,
		},
		{
			name:      "too short",
			password:  "Short1!",
			expectErr: true,
		},
		{
			name:      "no uppercase",
			password:  "weakp@ssw0rd123!",
			expectErr: true,
		},
		{
			name:      "no lowercase",
			password:  "WEAKP@SSW0RD123!",
			expectErr: true,
		},
		{
			name:      "no digit",
			password:  "WeakP@ssword!",
			expectErr: true,
		},
		{
			name:      "no special char",
			password:  "WeakPassword123",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validatePasswordStrength(tt.password)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeactivateCredential(t *testing.T) {
	manager, mockRepo, _ := setupCredentialManager(t)
	ctx := context.Background()

	credID := uuid.New()
	tenantID := uuid.New()

	mockRepo.On("Get", ctx, credID).Return(&repository.TenantCredential{
		ID:       credID,
		TenantID: tenantID,
		IsActive: true,
	}, nil)

	mockRepo.On("Deactivate", ctx, credID).Return(nil)

	err := manager.DeactivateCredential(ctx, credID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCheckExpiring(t *testing.T) {
	manager, mockRepo, _ := setupCredentialManager(t)
	ctx := context.Background()

	within := 7 * 24 * time.Hour
	expiringCreds := []*repository.TenantCredential{
		{
			ID:        uuid.New(),
			TenantID:  uuid.New(),
			ExpiresAt: ptrTime(time.Now().Add(3 * 24 * time.Hour)),
		},
		{
			ID:        uuid.New(),
			TenantID:  uuid.New(),
			ExpiresAt: ptrTime(time.Now().Add(5 * 24 * time.Hour)),
		},
	}

	mockRepo.On("ListExpiring", ctx, within).Return(expiringCreds, nil)

	creds, err := manager.CheckExpiring(ctx, within)

	assert.NoError(t, err)
	assert.Len(t, creds, 2)
	mockRepo.AssertExpectations(t)
}

func TestEnforceExpiry(t *testing.T) {
	manager, mockRepo, _ := setupCredentialManager(t)
	ctx := context.Background()

	expiredCreds := []*repository.TenantCredential{
		{
			ID:        uuid.New(),
			TenantID:  uuid.New(),
			ExpiresAt: ptrTime(time.Now().Add(-24 * time.Hour)),
			IsActive:  true,
		},
		{
			ID:        uuid.New(),
			TenantID:  uuid.New(),
			ExpiresAt: ptrTime(time.Now().Add(-48 * time.Hour)),
			IsActive:  true,
		},
	}

	mockRepo.On("ListExpired", ctx).Return(expiredCreds, nil)
	for _, cred := range expiredCreds {
		mockRepo.On("Deactivate", ctx, cred.ID).Return(nil)
	}

	count, err := manager.EnforceExpiry(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	mockRepo.AssertExpectations(t)
}

func TestCheckInactive(t *testing.T) {
	manager, mockRepo, _ := setupCredentialManager(t)
	ctx := context.Background()

	threshold := 30 * 24 * time.Hour
	since := time.Now().Add(-threshold)

	inactiveCreds := []*repository.TenantCredential{
		{
			ID:         uuid.New(),
			TenantID:   uuid.New(),
			LastUsedAt: ptrTime(time.Now().Add(-45 * 24 * time.Hour)),
		},
		{
			ID:         uuid.New(),
			TenantID:   uuid.New(),
			LastUsedAt: nil, // Never used
		},
	}

	mockRepo.On("ListUnusedSince", ctx, mock.MatchedBy(func(t time.Time) bool {
		return t.After(since.Add(-time.Minute)) && t.Before(since.Add(time.Minute))
	})).Return(inactiveCreds, nil)

	creds, err := manager.CheckInactive(ctx, threshold)

	assert.NoError(t, err)
	assert.Len(t, creds, 2)
	mockRepo.AssertExpectations(t)
}

func TestValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "password",
		Message: "too weak",
	}

	assert.Equal(t, "password: too weak", err.Error())
}

// Helper function to create time pointers
func ptrTime(t time.Time) *time.Time {
	return &t
}
