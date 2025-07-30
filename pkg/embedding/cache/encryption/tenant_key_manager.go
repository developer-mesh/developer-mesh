package encryption

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/google/uuid"
)

// TenantKey represents an encryption key for a tenant
type TenantKey struct {
	TenantID     uuid.UUID `db:"tenant_id"`
	KeyID        string    `db:"key_id"`
	EncryptedKey string    `db:"encrypted_key"`
	CreatedAt    time.Time `db:"created_at"`
	ExpiresAt    time.Time `db:"expires_at"`
	IsActive     bool      `db:"is_active"`
}

// TenantKeyManager manages per-tenant encryption keys
type TenantKeyManager struct {
	repo           TenantKeyRepository
	masterKeyID    string
	encryptionSvc  *security.EncryptionService
	keyCache       sync.Map // map[tenantID]map[keyID]*decryptedKey
	rotationPeriod time.Duration
	logger         observability.Logger
	mu             sync.RWMutex
}

type decryptedKey struct {
	keyID     string
	key       []byte
	expiresAt time.Time
	lastUsed  time.Time
}

// NewTenantKeyManager creates a new tenant key manager
func NewTenantKeyManager(
	repo TenantKeyRepository,
	masterKeyID string,
	encryptionSvc *security.EncryptionService,
	rotationPeriod time.Duration,
	logger observability.Logger,
) *TenantKeyManager {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.encryption")
	}

	if rotationPeriod <= 0 {
		rotationPeriod = 30 * 24 * time.Hour // 30 days default
	}

	return &TenantKeyManager{
		repo:           repo,
		masterKeyID:    masterKeyID,
		encryptionSvc:  encryptionSvc,
		rotationPeriod: rotationPeriod,
		logger:         logger,
	}
}

// GetOrCreateKey gets or creates an encryption key for a tenant
func (tkm *TenantKeyManager) GetOrCreateKey(ctx context.Context, tenantID uuid.UUID) ([]byte, string, error) {
	// Check cache first
	if cachedKeys, ok := tkm.keyCache.Load(tenantID); ok {
		if keyMap, ok := cachedKeys.(map[string]*decryptedKey); ok {
			// Find active key
			for _, dk := range keyMap {
				if time.Now().Before(dk.expiresAt) {
					dk.lastUsed = time.Now()
					return dk.key, dk.keyID, nil
				}
			}
		}
	}

	// Get from database
	keys, err := tkm.repo.GetActiveKeys(ctx, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get tenant keys: %w", err)
	}

	// Find or create active key
	var activeKey *TenantKey
	now := time.Now()

	for _, key := range keys {
		if key.IsActive && key.ExpiresAt.After(now) {
			activeKey = &key
			break
		}
	}

	if activeKey == nil {
		// Create new key
		activeKey, err = tkm.createNewKey(ctx, tenantID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create new key: %w", err)
		}
	}

	// Decrypt the key
	decryptedKeyBytes, err := tkm.decryptKey(activeKey.EncryptedKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt key: %w", err)
	}

	// Cache the key
	tkm.cacheKey(tenantID, activeKey.KeyID, decryptedKeyBytes, activeKey.ExpiresAt)

	return decryptedKeyBytes, activeKey.KeyID, nil
}

// RotateKey rotates the encryption key for a tenant
func (tkm *TenantKeyManager) RotateKey(ctx context.Context, tenantID uuid.UUID) (*TenantKey, error) {
	tkm.mu.Lock()
	defer tkm.mu.Unlock()

	// Deactivate old keys
	if err := tkm.repo.DeactivateKeys(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to deactivate old keys: %w", err)
	}

	// Create new key
	newKey, err := tkm.createNewKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to create new key: %w", err)
	}

	// Clear cache for this tenant
	tkm.keyCache.Delete(tenantID)

	tkm.logger.Info("Rotated encryption key for tenant", map[string]interface{}{
		"tenant_id":  tenantID.String(),
		"new_key_id": newKey.KeyID,
	})

	return newKey, nil
}

// GetDecryptionKey gets a specific decryption key by ID
func (tkm *TenantKeyManager) GetDecryptionKey(ctx context.Context, tenantID uuid.UUID, keyID string) ([]byte, error) {
	// Check cache first
	if cachedKeys, ok := tkm.keyCache.Load(tenantID); ok {
		if keyMap, ok := cachedKeys.(map[string]*decryptedKey); ok {
			if dk, exists := keyMap[keyID]; exists {
				dk.lastUsed = time.Now()
				return dk.key, nil
			}
		}
	}

	// Get from database
	key, err := tkm.repo.GetKey(ctx, tenantID, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Decrypt the key
	decryptedKeyBytes, err := tkm.decryptKey(key.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt key: %w", err)
	}

	// Cache the key
	tkm.cacheKey(tenantID, key.KeyID, decryptedKeyBytes, key.ExpiresAt)

	return decryptedKeyBytes, nil
}

// StartRotationScheduler starts automatic key rotation
func (tkm *TenantKeyManager) StartRotationScheduler(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tkm.rotateExpiredKeys(ctx)
		}
	}
}

// Private methods

func (tkm *TenantKeyManager) createNewKey(ctx context.Context, tenantID uuid.UUID) (*TenantKey, error) {
	// Generate new key
	keyBytes := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Encrypt with master key
	encryptedKey, err := tkm.encryptionSvc.EncryptCredential(string(keyBytes), tkm.masterKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt key: %w", err)
	}

	// Create key record
	key := &TenantKey{
		TenantID:     tenantID,
		KeyID:        uuid.New().String(),
		EncryptedKey: base64.StdEncoding.EncodeToString([]byte(encryptedKey)),
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(tkm.rotationPeriod),
		IsActive:     true,
	}

	// Store in database
	if err := tkm.repo.CreateKey(ctx, key); err != nil {
		return nil, fmt.Errorf("failed to store key: %w", err)
	}

	return key, nil
}

func (tkm *TenantKeyManager) decryptKey(encryptedKey string) ([]byte, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	decrypted, err := tkm.encryptionSvc.DecryptCredential(encryptedBytes, tkm.masterKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt key: %w", err)
	}

	return []byte(decrypted), nil
}

func (tkm *TenantKeyManager) cacheKey(tenantID uuid.UUID, keyID string, key []byte, expiresAt time.Time) {
	dk := &decryptedKey{
		keyID:     keyID,
		key:       key,
		expiresAt: expiresAt,
		lastUsed:  time.Now(),
	}

	// Get or create tenant key map
	var keyMap map[string]*decryptedKey
	if cached, ok := tkm.keyCache.Load(tenantID); ok {
		keyMap = cached.(map[string]*decryptedKey)
	} else {
		keyMap = make(map[string]*decryptedKey)
	}

	keyMap[keyID] = dk
	tkm.keyCache.Store(tenantID, keyMap)
}

func (tkm *TenantKeyManager) rotateExpiredKeys(ctx context.Context) {
	expiredKeys, err := tkm.repo.GetExpiringKeys(ctx, 7*24*time.Hour) // Keys expiring in 7 days
	if err != nil {
		tkm.logger.Error("Failed to get expiring keys", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	for _, key := range expiredKeys {
		if _, err := tkm.RotateKey(ctx, key.TenantID); err != nil {
			tkm.logger.Error("Failed to rotate key", map[string]interface{}{
				"tenant_id": key.TenantID.String(),
				"error":     err.Error(),
			})
		}
	}
}

// CleanupCache removes expired keys from cache
func (tkm *TenantKeyManager) CleanupCache() {
	now := time.Now()
	tkm.keyCache.Range(func(tenantID, value interface{}) bool {
		if keyMap, ok := value.(map[string]*decryptedKey); ok {
			for keyID, dk := range keyMap {
				if dk.expiresAt.Before(now) || now.Sub(dk.lastUsed) > 1*time.Hour {
					delete(keyMap, keyID)
				}
			}
			if len(keyMap) == 0 {
				tkm.keyCache.Delete(tenantID)
			} else {
				tkm.keyCache.Store(tenantID, keyMap)
			}
		}
		return true
	})
}
