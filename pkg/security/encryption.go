package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptionService provides credential encryption using AES-256-GCM
type EncryptionService struct {
	masterKey []byte
	saltSize  int
	keyIter   int
}

// NewEncryptionService creates a new encryption service
func NewEncryptionService(masterKey string) *EncryptionService {
	// Derive key from master key using SHA-256
	hash := sha256.Sum256([]byte(masterKey))
	return &EncryptionService{
		masterKey: hash[:],
		saltSize:  32,
		keyIter:   10000,
	}
}

// EncryptCredential encrypts sensitive data using AES-256-GCM with per-tenant key derivation
func (e *EncryptionService) EncryptCredential(plaintext string, tenantID string) ([]byte, error) {
	// Generate salt for this encryption
	salt := make([]byte, e.saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive tenant-specific key
	key := e.deriveKey(tenantID, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Combine salt + nonce + ciphertext
	encrypted := make([]byte, len(salt)+len(nonce)+len(ciphertext))
	copy(encrypted, salt)
	copy(encrypted[len(salt):], nonce)
	copy(encrypted[len(salt)+len(nonce):], ciphertext)

	return encrypted, nil
}

// DecryptCredential decrypts data encrypted with EncryptCredential
func (e *EncryptionService) DecryptCredential(encrypted []byte, tenantID string) (string, error) {
	// Validate minimum size
	if len(encrypted) < e.saltSize+12 { // 12 is minimum nonce size for GCM
		return "", fmt.Errorf("invalid encrypted data: too short")
	}

	// Extract components
	salt := encrypted[:e.saltSize]
	encrypted = encrypted[e.saltSize:]

	// Derive tenant-specific key
	key := e.deriveKey(tenantID, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and ciphertext
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return "", fmt.Errorf("invalid encrypted data: missing nonce")
	}

	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// deriveKey derives a tenant-specific encryption key
func (e *EncryptionService) deriveKey(tenantID string, salt []byte) []byte {
	// Combine master key with tenant ID for key derivation
	info := append(e.masterKey, []byte(tenantID)...)
	return pbkdf2.Key(info, salt, e.keyIter, 32, sha256.New)
}

// EncryptJSON encrypts a JSON-serializable object
func (e *EncryptionService) EncryptJSON(data interface{}, tenantID string) (string, error) {
	// Convert to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Encrypt
	encrypted, err := e.EncryptCredential(string(jsonData), tenantID)
	if err != nil {
		return "", err
	}

	// Base64 encode for storage
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptJSON decrypts and unmarshals encrypted JSON data
func (e *EncryptionService) DecryptJSON(encryptedBase64 string, tenantID string, target interface{}) error {
	// Base64 decode
	encrypted, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64: %w", err)
	}

	// Decrypt
	decrypted, err := e.DecryptCredential(encrypted, tenantID)
	if err != nil {
		return err
	}

	// Unmarshal JSON
	if err := json.Unmarshal([]byte(decrypted), target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// RotateKey creates a new encryption with a new key while preserving the data
func (e *EncryptionService) RotateKey(oldEncrypted []byte, tenantID string, newMasterKey string) ([]byte, error) {
	// Decrypt with current key
	plaintext, err := e.DecryptCredential(oldEncrypted, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Create new service with new key
	newService := NewEncryptionService(newMasterKey)

	// Encrypt with new key
	return newService.EncryptCredential(plaintext, tenantID)
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashPassword creates a secure hash of a password
func HashPassword(password string) string {
	hash := pbkdf2.Key([]byte(password), []byte("devops-mcp-salt"), 10000, 32, sha256.New)
	return base64.StdEncoding.EncodeToString(hash)
}

// VerifyPassword checks if a password matches a hash
func VerifyPassword(password, hash string) bool {
	expectedHash := HashPassword(password)
	return expectedHash == hash
}
