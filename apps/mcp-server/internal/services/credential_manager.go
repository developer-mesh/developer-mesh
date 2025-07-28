package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
)

// CredentialManager handles encryption and decryption of tool credentials
type CredentialManager struct {
	masterKey []byte
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(masterKey string) (*CredentialManager, error) {
	if len(masterKey) < 32 {
		return nil, fmt.Errorf("master key must be at least 32 characters")
	}

	// Hash the master key to ensure consistent length
	hash := sha256.Sum256([]byte(masterKey))

	return &CredentialManager{
		masterKey: hash[:],
	}, nil
}

// EncryptCredential encrypts tool credentials for storage
func (c *CredentialManager) EncryptCredential(tenantID string, credential *tool.TokenCredential) ([]byte, error) {
	if credential == nil {
		return nil, nil
	}

	// Serialize credential to JSON
	data, err := json.Marshal(credential)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credential: %w", err)
	}

	// Get tenant-specific key by combining master key with tenant ID
	tenantKey := c.getTenantKey(tenantID)

	// Encrypt the data
	encrypted, err := c.encrypt(tenantKey, data)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	return encrypted, nil
}

// DecryptCredential decrypts stored credentials
func (c *CredentialManager) DecryptCredential(tenantID string, encryptedData []byte) (*tool.TokenCredential, error) {
	if len(encryptedData) == 0 {
		return nil, nil
	}

	// Get tenant-specific key
	tenantKey := c.getTenantKey(tenantID)

	// Decrypt the data
	decrypted, err := c.decrypt(tenantKey, encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Deserialize credential
	var credential tool.TokenCredential
	if err := json.Unmarshal(decrypted, &credential); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credential: %w", err)
	}

	return &credential, nil
}

// getTenantKey derives a tenant-specific key from the master key
func (c *CredentialManager) getTenantKey(tenantID string) []byte {
	// Combine master key with tenant ID and hash
	combined := append(c.masterKey, []byte(tenantID)...)
	hash := sha256.Sum256(combined)
	return hash[:]
}

// encrypt performs AES-256-GCM encryption
func (c *CredentialManager) encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt and append nonce to the beginning
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	// Encode to base64 for storage
	return []byte(base64.StdEncoding.EncodeToString(ciphertext)), nil
}

// decrypt performs AES-256-GCM decryption
func (c *CredentialManager) decrypt(key, ciphertext []byte) ([]byte, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	// Decrypt
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// RotateCredential re-encrypts a credential with a new key
func (c *CredentialManager) RotateCredential(tenantID string, oldManager *CredentialManager, encryptedData []byte) ([]byte, error) {
	// Decrypt with old manager
	credential, err := oldManager.DecryptCredential(tenantID, encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Encrypt with new manager
	return c.EncryptCredential(tenantID, credential)
}
