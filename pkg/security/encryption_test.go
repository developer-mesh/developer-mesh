package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptionService(t *testing.T) {
	masterKey := "test-master-key-for-testing"
	service := NewEncryptionService(masterKey)

	t.Run("EncryptDecrypt", func(t *testing.T) {
		plaintext := "my-secret-token"
		tenantID := "tenant-123"

		// Encrypt
		encrypted, err := service.EncryptCredential(plaintext, tenantID)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.NotEqual(t, plaintext, string(encrypted))

		// Decrypt
		decrypted, err := service.DecryptCredential(encrypted, tenantID)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("DifferentTenantsDifferentEncryption", func(t *testing.T) {
		plaintext := "shared-secret"
		tenant1 := "tenant-1"
		tenant2 := "tenant-2"

		// Encrypt with different tenants
		encrypted1, err := service.EncryptCredential(plaintext, tenant1)
		require.NoError(t, err)

		encrypted2, err := service.EncryptCredential(plaintext, tenant2)
		require.NoError(t, err)

		// Encrypted values should be different
		assert.NotEqual(t, encrypted1, encrypted2)

		// But both should decrypt to the same plaintext
		decrypted1, err := service.DecryptCredential(encrypted1, tenant1)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted1)

		decrypted2, err := service.DecryptCredential(encrypted2, tenant2)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted2)
	})

	t.Run("WrongTenantCannotDecrypt", func(t *testing.T) {
		plaintext := "secret-data"
		rightTenant := "tenant-right"
		wrongTenant := "tenant-wrong"

		// Encrypt with right tenant
		encrypted, err := service.EncryptCredential(plaintext, rightTenant)
		require.NoError(t, err)

		// Try to decrypt with wrong tenant
		_, err = service.DecryptCredential(encrypted, wrongTenant)
		assert.Error(t, err)
	})

	t.Run("EmptyPlaintext", func(t *testing.T) {
		plaintext := ""
		tenantID := "tenant-123"

		encrypted, err := service.EncryptCredential(plaintext, tenantID)
		require.NoError(t, err)

		decrypted, err := service.DecryptCredential(encrypted, tenantID)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("InvalidCiphertext", func(t *testing.T) {
		tenantID := "tenant-123"

		// Too short
		_, err := service.DecryptCredential([]byte("short"), tenantID)
		assert.Error(t, err)

		// Invalid data
		invalidData := make([]byte, 100)
		_, err = service.DecryptCredential(invalidData, tenantID)
		assert.Error(t, err)
	})
}

func TestDeriveKey(t *testing.T) {
	masterKey := "test-master-key"
	service := NewEncryptionService(masterKey)

	t.Run("ConsistentKeyDerivation", func(t *testing.T) {
		tenantID := "tenant-123"
		salt := []byte("test-salt-12345678901234567890123") // 32 bytes

		key1 := service.deriveKey(tenantID, salt)
		key2 := service.deriveKey(tenantID, salt)

		assert.Equal(t, key1, key2)
	})

	t.Run("DifferentTenantsGetDifferentKeys", func(t *testing.T) {
		salt := []byte("test-salt-12345678901234567890123") // 32 bytes

		key1 := service.deriveKey("tenant-1", salt)
		key2 := service.deriveKey("tenant-2", salt)

		assert.NotEqual(t, key1, key2)
	})

	t.Run("KeyLengthIs32Bytes", func(t *testing.T) {
		salt := []byte("test-salt-12345678901234567890123") // 32 bytes
		key := service.deriveKey("tenant-123", salt)
		assert.Len(t, key, 32)
	})
}
