package webhook

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
)

// EncryptionService handles data encryption/decryption for webhook data
type EncryptionService struct {
	encryptor *security.EncryptionService
	logger    observability.Logger
}

// NewEncryptionService creates a new encryption service using the existing security package
func NewEncryptionService(masterKey string, logger observability.Logger) (*EncryptionService, error) {
	if masterKey == "" {
		return nil, fmt.Errorf("master key is required for encryption")
	}

	return &EncryptionService{
		encryptor: security.NewEncryptionService(masterKey),
		logger:    logger,
	}, nil
}

// EncryptForTenant encrypts data using tenant-specific key derivation
func (e *EncryptionService) EncryptForTenant(plaintext []byte, tenantID string) ([]byte, error) {
	return e.encryptor.EncryptCredential(string(plaintext), tenantID)
}

// DecryptForTenant decrypts data encrypted with EncryptForTenant
func (e *EncryptionService) DecryptForTenant(ciphertext []byte, tenantID string) ([]byte, error) {
	plaintext, err := e.encryptor.DecryptCredential(ciphertext, tenantID)
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}

// EncryptContextData encrypts context data for storage
func (e *EncryptionService) EncryptContextData(data map[string]interface{}, tenantID string) ([]byte, error) {
	// Serialize to JSON first
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal context data: %w", err)
	}

	// Encrypt using tenant-specific key
	return e.EncryptForTenant(jsonData, tenantID)
}

// DecryptContextData decrypts context data from storage
func (e *EncryptionService) DecryptContextData(encryptedData []byte, tenantID string) (map[string]interface{}, error) {
	// Decrypt using tenant-specific key
	jsonData, err := e.DecryptForTenant(encryptedData, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt context data: %w", err)
	}

	// Deserialize from JSON
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context data: %w", err)
	}

	return data, nil
}

// EncryptWebhookPayload encrypts sensitive webhook payload fields
func (e *EncryptionService) EncryptWebhookPayload(event *WebhookEvent, sensitiveFields []string) error {
	if event.Payload == nil || len(sensitiveFields) == 0 {
		return nil
	}

	for _, field := range sensitiveFields {
		if value, exists := event.Payload[field]; exists {
			if strValue, ok := value.(string); ok {
				encrypted, err := e.EncryptForTenant([]byte(strValue), event.TenantId)
				if err != nil {
					return fmt.Errorf("failed to encrypt field %s: %w", field, err)
				}
				event.Payload[field] = fmt.Sprintf("encrypted:%s",
					base64.StdEncoding.EncodeToString(encrypted))
			}
		}
	}

	return nil
}
