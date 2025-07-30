package encryption

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
)

func TestTenantKeyManager_GetOrCreateKey(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	masterKeyID := "test-master-key"

	// Create mock repository
	mockRepo := NewMockTenantKeyRepository()

	// Create encryption service
	encryptionSvc := security.NewEncryptionService("test-encryption-key")

	// Create key manager
	keyManager := NewTenantKeyManager(
		mockRepo,
		masterKeyID,
		encryptionSvc,
		30*24*time.Hour,
		observability.NewLogger("test"),
	)

	t.Run("Create new key for tenant", func(t *testing.T) {
		// Get or create key
		key, keyID, err := keyManager.GetOrCreateKey(ctx, tenantID)
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.NotEmpty(t, keyID)
		assert.Len(t, key, 32) // 256-bit key

		// Verify key was stored
		storedKey, err := mockRepo.GetKey(ctx, tenantID, keyID)
		require.NoError(t, err)
		assert.Equal(t, tenantID, storedKey.TenantID)
		assert.Equal(t, keyID, storedKey.KeyID)
		assert.True(t, storedKey.IsActive)
	})

	t.Run("Get existing key from cache", func(t *testing.T) {
		// First call to populate cache
		key1, keyID1, err := keyManager.GetOrCreateKey(ctx, tenantID)
		require.NoError(t, err)

		// Second call should get from cache
		key2, keyID2, err := keyManager.GetOrCreateKey(ctx, tenantID)
		require.NoError(t, err)

		assert.Equal(t, key1, key2)
		assert.Equal(t, keyID1, keyID2)
	})
}

func TestTenantKeyManager_RotateKey(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	masterKeyID := "test-master-key"

	// Create mock repository
	mockRepo := NewMockTenantKeyRepository()

	// Create encryption service
	encryptionSvc := security.NewEncryptionService("test-encryption-key")

	// Create key manager
	keyManager := NewTenantKeyManager(
		mockRepo,
		masterKeyID,
		encryptionSvc,
		30*24*time.Hour,
		observability.NewLogger("test"),
	)

	// Create initial key
	oldKey, oldKeyID, err := keyManager.GetOrCreateKey(ctx, tenantID)
	require.NoError(t, err)

	// Rotate key
	newKey, err := keyManager.RotateKey(ctx, tenantID)
	require.NoError(t, err)
	assert.NotEqual(t, oldKeyID, newKey.KeyID)

	// Verify old key is deactivated
	oldKeyRecord, err := mockRepo.GetKey(ctx, tenantID, oldKeyID)
	require.NoError(t, err)
	assert.False(t, oldKeyRecord.IsActive)

	// Verify new key is active
	assert.True(t, newKey.IsActive)

	// Get key should return new key
	currentKey, currentKeyID, err := keyManager.GetOrCreateKey(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, newKey.KeyID, currentKeyID)
	assert.NotEqual(t, oldKey, currentKey)
}

func TestTenantKeyManager_GetDecryptionKey(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	masterKeyID := "test-master-key"

	// Create mock repository
	mockRepo := NewMockTenantKeyRepository()

	// Create encryption service
	encryptionSvc := security.NewEncryptionService("test-encryption-key")

	// Create key manager
	keyManager := NewTenantKeyManager(
		mockRepo,
		masterKeyID,
		encryptionSvc,
		30*24*time.Hour,
		observability.NewLogger("test"),
	)

	// Create a key
	_, keyID, err := keyManager.GetOrCreateKey(ctx, tenantID)
	require.NoError(t, err)

	// Get decryption key
	decryptionKey, err := keyManager.GetDecryptionKey(ctx, tenantID, keyID)
	require.NoError(t, err)
	assert.NotNil(t, decryptionKey)
	assert.Len(t, decryptionKey, 32)

	// Try to get non-existent key
	_, err = keyManager.GetDecryptionKey(ctx, tenantID, "non-existent")
	assert.Error(t, err)
}

func TestTenantKeyManager_CleanupCache(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	masterKeyID := "test-master-key"

	// Create mock repository
	mockRepo := NewMockTenantKeyRepository()

	// Create encryption service
	encryptionSvc := security.NewEncryptionService("test-encryption-key")

	// Create key manager
	keyManager := NewTenantKeyManager(
		mockRepo,
		masterKeyID,
		encryptionSvc,
		1*time.Second, // Short rotation period for testing
		observability.NewLogger("test"),
	)

	// Create a key
	_, keyID, err := keyManager.GetOrCreateKey(ctx, tenantID)
	require.NoError(t, err)

	// Manually set expiration in the past
	if key, err := mockRepo.GetKey(ctx, tenantID, keyID); err == nil {
		key.ExpiresAt = time.Now().Add(-1 * time.Hour)
		err = mockRepo.CreateKey(ctx, key)
		assert.NoError(t, err)
	}

	// Run cleanup
	keyManager.CleanupCache()

	// Try to get key - should fetch from DB since cache was cleaned
	_, err = keyManager.GetDecryptionKey(ctx, tenantID, keyID)
	require.NoError(t, err)
}
