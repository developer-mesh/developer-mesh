package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// CredentialManager handles encryption and storage of tenant credentials
type CredentialManager struct {
	db        *sqlx.DB
	masterKey []byte
}

// NewCredentialManager creates a new credential manager
// masterKey must be exactly 32 bytes for AES-256
func NewCredentialManager(db *sqlx.DB, masterKey []byte) *CredentialManager {
	if len(masterKey) != 32 {
		panic("master key must be 32 bytes for AES-256")
	}
	return &CredentialManager{
		db:        db,
		masterKey: masterKey,
	}
}

// StoreCredential encrypts and stores a credential for a tenant/source
func (cm *CredentialManager) StoreCredential(ctx context.Context, tenantID uuid.UUID, sourceID string, credType string, value string) error {
	encrypted, err := cm.encryptForTenant(tenantID, value)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	query := `
		INSERT INTO rag.tenant_source_credentials
		(tenant_id, source_id, credential_type, encrypted_value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, source_id, credential_type)
		DO UPDATE SET
			encrypted_value = EXCLUDED.encrypted_value,
			last_rotated_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = cm.db.ExecContext(ctx, query, tenantID, sourceID, credType, encrypted)
	if err != nil {
		return fmt.Errorf("failed to store credential: %w", err)
	}

	return nil
}

// GetCredential retrieves and decrypts a specific credential
func (cm *CredentialManager) GetCredential(ctx context.Context, tenantID uuid.UUID, sourceID string, credType string) (string, error) {
	var encrypted string
	query := `
		SELECT encrypted_value
		FROM rag.tenant_source_credentials
		WHERE tenant_id = $1 AND source_id = $2 AND credential_type = $3
	`

	err := cm.db.GetContext(ctx, &encrypted, query, tenantID, sourceID, credType)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve credential: %w", err)
	}

	decrypted, err := cm.decryptForTenant(tenantID, encrypted)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return decrypted, nil
}

// GetAllCredentials retrieves and decrypts all credentials for a source
func (cm *CredentialManager) GetAllCredentials(ctx context.Context, tenantID uuid.UUID, sourceID string) (map[string]string, error) {
	type credRow struct {
		CredentialType string `db:"credential_type"`
		EncryptedValue string `db:"encrypted_value"`
	}

	var rows []credRow
	query := `
		SELECT credential_type, encrypted_value
		FROM rag.tenant_source_credentials
		WHERE tenant_id = $1 AND source_id = $2
	`

	err := cm.db.SelectContext(ctx, &rows, query, tenantID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	credentials := make(map[string]string)
	for _, row := range rows {
		decrypted, err := cm.decryptForTenant(tenantID, row.EncryptedValue)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credential %s: %w", row.CredentialType, err)
		}
		credentials[row.CredentialType] = decrypted
	}

	return credentials, nil
}

// DeleteCredentials removes all credentials for a source
func (cm *CredentialManager) DeleteCredentials(ctx context.Context, tenantID uuid.UUID, sourceID string) error {
	query := `
		DELETE FROM rag.tenant_source_credentials
		WHERE tenant_id = $1 AND source_id = $2
	`

	_, err := cm.db.ExecContext(ctx, query, tenantID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	return nil
}

// encryptForTenant encrypts plaintext using a tenant-specific derived key
// Uses AES-256-GCM with the tenant ID as additional authenticated data
func (cm *CredentialManager) encryptForTenant(tenantID uuid.UUID, plaintext string) (string, error) {
	// Derive tenant-specific key
	tenantKey := cm.deriveTenantKey(tenantID)

	// Create cipher block
	block, err := aes.NewCipher(tenantKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt with tenant ID as additional authenticated data (AAD)
	// This ensures the ciphertext can only be decrypted for the correct tenant
	aad := []byte(tenantID.String())
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), aad)

	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptForTenant decrypts ciphertext using a tenant-specific derived key
func (cm *CredentialManager) decryptForTenant(tenantID uuid.UUID, ciphertext string) (string, error) {
	// Decode from base64
	encrypted, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Derive tenant key
	tenantKey := cm.deriveTenantKey(tenantID)

	// Create cipher block
	block, err := aes.NewCipher(tenantKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce from ciphertext
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encryptedData := encrypted[:nonceSize], encrypted[nonceSize:]

	// Decrypt with tenant ID as AAD
	aad := []byte(tenantID.String())
	plaintext, err := gcm.Open(nil, nonce, encryptedData, aad)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// deriveTenantKey derives a unique encryption key for each tenant
// This ensures that credentials encrypted for one tenant cannot be decrypted by another
func (cm *CredentialManager) deriveTenantKey(tenantID uuid.UUID) []byte {
	h := sha256.New()
	h.Write(cm.masterKey)
	h.Write([]byte(tenantID.String()))
	h.Write([]byte("RAG_TENANT_KEY_V1")) // Version tag for future key rotation
	return h.Sum(nil)[:32]               // Use first 32 bytes for AES-256
}
