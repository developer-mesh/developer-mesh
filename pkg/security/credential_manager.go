package security

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// CredentialRepositoryInterface defines the interface for credential storage
type CredentialRepositoryInterface interface {
	Create(ctx context.Context, cred *repository.TenantCredential) error
	Get(ctx context.Context, id uuid.UUID) (*repository.TenantCredential, error)
	Update(ctx context.Context, cred *repository.TenantCredential) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	ListExpiring(ctx context.Context, within time.Duration) ([]*repository.TenantCredential, error)
	ListExpired(ctx context.Context) ([]*repository.TenantCredential, error)
	ListUnusedSince(ctx context.Context, since time.Time) ([]*repository.TenantCredential, error)
}

// CredentialManager manages secure credential storage, rotation, validation, and auditing
type CredentialManager struct {
	encryption *EncryptionService
	repo       CredentialRepositoryInterface
	audit      *auth.AuditLogger
	logger     observability.Logger
}

// CredentialConfig holds configuration for credential management
type CredentialConfig struct {
	// DefaultExpiry is the default TTL for new credentials
	DefaultExpiry time.Duration
	// RotationInterval is how often credentials should be rotated
	RotationInterval time.Duration
	// ExpiryWarningThreshold is how far in advance to warn about expiring credentials
	ExpiryWarningThreshold time.Duration
	// MinPasswordLength for basic auth credentials
	MinPasswordLength int
	// RequireStrongPasswords enforces password complexity
	RequireStrongPasswords bool
	// InactivityThreshold is how long a credential can be unused before flagging
	InactivityThreshold time.Duration
}

// DefaultCredentialConfig returns sensible defaults
func DefaultCredentialConfig() *CredentialConfig {
	return &CredentialConfig{
		DefaultExpiry:          90 * 24 * time.Hour, // 90 days
		RotationInterval:       30 * 24 * time.Hour, // 30 days
		ExpiryWarningThreshold: 7 * 24 * time.Hour,  // 7 days
		MinPasswordLength:      12,
		RequireStrongPasswords: true,
		InactivityThreshold:    30 * 24 * time.Hour, // 30 days
	}
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(
	encryption *EncryptionService,
	repo CredentialRepositoryInterface,
	audit *auth.AuditLogger,
	logger observability.Logger,
) *CredentialManager {
	return &CredentialManager{
		encryption: encryption,
		repo:       repo,
		audit:      audit,
		logger:     logger,
	}
}

// ValidationError represents a credential validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// CreateCredential creates a new encrypted credential
func (cm *CredentialManager) CreateCredential(
	ctx context.Context,
	tenantID uuid.UUID,
	toolID *uuid.UUID,
	name string,
	credType string,
	value string,
	expiresAt *time.Time,
) (*repository.TenantCredential, error) {
	// Validate credential
	if err := cm.ValidateCredential(credType, value); err != nil {
		cm.logger.Warn("Credential validation failed", map[string]interface{}{
			"tenant_id": tenantID.String(),
			"name":      name,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Encrypt the credential value
	encryptedValue, err := cm.encryption.EncryptCredential(value, tenantID.String())
	if err != nil {
		cm.logger.Error("Failed to encrypt credential", map[string]interface{}{
			"tenant_id": tenantID.String(),
			"name":      name,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Create credential record
	cred := &repository.TenantCredential{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ToolID:         toolID,
		CredentialName: name,
		CredentialType: credType,
		EncryptedValue: string(encryptedValue),
		IsActive:       true,
		ExpiresAt:      expiresAt,
	}

	// Save to database
	if err := cm.repo.Create(ctx, cred); err != nil {
		cm.logger.Error("Failed to create credential in database", map[string]interface{}{
			"tenant_id": tenantID.String(),
			"name":      name,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// Audit log
	cm.audit.LogAuthAttempt(ctx, auth.AuditEvent{
		EventType: "credential_created",
		TenantID:  tenantID.String(),
		Success:   true,
		Metadata: map[string]interface{}{
			"credential_id":   cred.ID.String(),
			"credential_name": name,
			"credential_type": credType,
			"has_expiry":      expiresAt != nil,
		},
	})

	cm.logger.Info("Credential created successfully", map[string]interface{}{
		"tenant_id":     tenantID.String(),
		"credential_id": cred.ID.String(),
		"name":          name,
	})

	return cred, nil
}

// GetCredential retrieves and decrypts a credential
func (cm *CredentialManager) GetCredential(ctx context.Context, id uuid.UUID) (string, error) {
	// Retrieve from database
	cred, err := cm.repo.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get credential: %w", err)
	}

	// Check if expired
	if cred.ExpiresAt != nil && cred.ExpiresAt.Before(time.Now()) {
		cm.logger.Warn("Attempted to use expired credential", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
			"expired_at":    cred.ExpiresAt.Format(time.RFC3339),
		})
		return "", fmt.Errorf("credential has expired")
	}

	// Check if active
	if !cred.IsActive {
		cm.logger.Warn("Attempted to use inactive credential", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
		})
		return "", fmt.Errorf("credential is inactive")
	}

	// Decrypt
	decrypted, err := cm.encryption.DecryptCredential([]byte(cred.EncryptedValue), cred.TenantID.String())
	if err != nil {
		cm.logger.Error("Failed to decrypt credential", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
			"error":         err.Error(),
		})
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	// Update last used
	if err := cm.repo.UpdateLastUsed(ctx, id); err != nil {
		// Log but don't fail
		cm.logger.Warn("Failed to update last used timestamp", map[string]interface{}{
			"credential_id": id.String(),
			"error":         err.Error(),
		})
	}

	// Audit log credential access
	cm.audit.LogAuthAttempt(ctx, auth.AuditEvent{
		EventType: "credential_accessed",
		TenantID:  cred.TenantID.String(),
		Success:   true,
		Metadata: map[string]interface{}{
			"credential_id":   id.String(),
			"credential_name": cred.CredentialName,
		},
	})

	return decrypted, nil
}

// RotateCredential rotates a credential to a new value
func (cm *CredentialManager) RotateCredential(
	ctx context.Context,
	id uuid.UUID,
	newValue string,
	newExpiresAt *time.Time,
) error {
	// Get existing credential
	cred, err := cm.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get credential: %w", err)
	}

	// Validate new value
	if err := cm.ValidateCredential(cred.CredentialType, newValue); err != nil {
		cm.logger.Warn("New credential value validation failed", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("validation failed: %w", err)
	}

	// Encrypt new value
	encryptedValue, err := cm.encryption.EncryptCredential(newValue, cred.TenantID.String())
	if err != nil {
		cm.logger.Error("Failed to encrypt new credential value", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Update credential
	cred.EncryptedValue = string(encryptedValue)
	if newExpiresAt != nil {
		cred.ExpiresAt = newExpiresAt
	}

	if err := cm.repo.Update(ctx, cred); err != nil {
		cm.logger.Error("Failed to update credential in database", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to update credential: %w", err)
	}

	// Audit log
	cm.audit.LogAuthAttempt(ctx, auth.AuditEvent{
		EventType: "credential_rotated",
		TenantID:  cred.TenantID.String(),
		Success:   true,
		Metadata: map[string]interface{}{
			"credential_id":   id.String(),
			"credential_name": cred.CredentialName,
			"new_expiry":      newExpiresAt,
		},
	})

	cm.logger.Info("Credential rotated successfully", map[string]interface{}{
		"credential_id": id.String(),
		"tenant_id":     cred.TenantID.String(),
	})

	return nil
}

// ValidateCredential validates a credential based on its type
func (cm *CredentialManager) ValidateCredential(credType, value string) error {
	if value == "" {
		return ValidationError{Field: "value", Message: "credential value cannot be empty"}
	}

	switch credType {
	case "api_key":
		return cm.validateAPIKey(value)
	case "basic":
		return cm.validateBasicAuth(value)
	case "oauth2":
		return cm.validateOAuth2Token(value)
	case "custom":
		// Custom credentials have minimal validation
		if len(value) < 8 {
			return ValidationError{Field: "value", Message: "custom credential must be at least 8 characters"}
		}
	default:
		return ValidationError{Field: "type", Message: fmt.Sprintf("unsupported credential type: %s", credType)}
	}

	return nil
}

// validateAPIKey validates an API key format
func (cm *CredentialManager) validateAPIKey(apiKey string) error {
	// Check minimum length
	if len(apiKey) < 16 {
		return ValidationError{Field: "api_key", Message: "API key must be at least 16 characters"}
	}

	// Check maximum length (reasonable upper bound)
	if len(apiKey) > 512 {
		return ValidationError{Field: "api_key", Message: "API key too long (max 512 characters)"}
	}

	// Check for valid characters (alphanumeric, hyphen, underscore)
	validChars := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validChars.MatchString(apiKey) {
		return ValidationError{Field: "api_key", Message: "API key contains invalid characters"}
	}

	return nil
}

// validateBasicAuth validates basic authentication credentials
func (cm *CredentialManager) validateBasicAuth(value string) error {
	// Basic auth is typically "username:password"
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return ValidationError{Field: "basic_auth", Message: "basic auth must be in format username:password"}
	}

	username := parts[0]
	password := parts[1]

	if username == "" {
		return ValidationError{Field: "username", Message: "username cannot be empty"}
	}

	if len(password) < 8 {
		return ValidationError{Field: "password", Message: "password must be at least 8 characters"}
	}

	// Optional: Check password strength
	if err := cm.validatePasswordStrength(password); err != nil {
		return err
	}

	return nil
}

// validateOAuth2Token validates an OAuth2 token
func (cm *CredentialManager) validateOAuth2Token(token string) error {
	// OAuth2 tokens are typically JWT or opaque tokens
	if len(token) < 20 {
		return ValidationError{Field: "oauth_token", Message: "OAuth2 token too short"}
	}

	if len(token) > 4096 {
		return ValidationError{Field: "oauth_token", Message: "OAuth2 token too long"}
	}

	return nil
}

// validatePasswordStrength checks password complexity
func (cm *CredentialManager) validatePasswordStrength(password string) error {
	if len(password) < 12 {
		return ValidationError{Field: "password", Message: "password must be at least 12 characters"}
	}

	// Check for at least one uppercase, one lowercase, one digit, one special char
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ValidationError{
			Field:   "password",
			Message: "password must contain uppercase, lowercase, digit, and special character",
		}
	}

	return nil
}

// DeactivateCredential deactivates a credential
func (cm *CredentialManager) DeactivateCredential(ctx context.Context, id uuid.UUID) error {
	cred, err := cm.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get credential: %w", err)
	}

	if err := cm.repo.Deactivate(ctx, id); err != nil {
		cm.logger.Error("Failed to deactivate credential", map[string]interface{}{
			"credential_id": id.String(),
			"tenant_id":     cred.TenantID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to deactivate credential: %w", err)
	}

	// Audit log
	cm.audit.LogAuthAttempt(ctx, auth.AuditEvent{
		EventType: "credential_deactivated",
		TenantID:  cred.TenantID.String(),
		Success:   true,
		Metadata: map[string]interface{}{
			"credential_id":   id.String(),
			"credential_name": cred.CredentialName,
		},
	})

	cm.logger.Info("Credential deactivated", map[string]interface{}{
		"credential_id": id.String(),
		"tenant_id":     cred.TenantID.String(),
	})

	return nil
}

// CheckExpiring retrieves credentials expiring within the given duration
func (cm *CredentialManager) CheckExpiring(ctx context.Context, within time.Duration) ([]*repository.TenantCredential, error) {
	creds, err := cm.repo.ListExpiring(ctx, within)
	if err != nil {
		cm.logger.Error("Failed to check expiring credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to check expiring credentials: %w", err)
	}

	// Log for monitoring
	if len(creds) > 0 {
		cm.logger.Warn("Found expiring credentials", map[string]interface{}{
			"count":  len(creds),
			"within": within.String(),
		})
	}

	return creds, nil
}

// EnforceExpiry deactivates expired credentials
func (cm *CredentialManager) EnforceExpiry(ctx context.Context) (int, error) {
	expired, err := cm.repo.ListExpired(ctx)
	if err != nil {
		cm.logger.Error("Failed to list expired credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return 0, fmt.Errorf("failed to list expired credentials: %w", err)
	}

	deactivatedCount := 0
	for _, cred := range expired {
		if err := cm.repo.Deactivate(ctx, cred.ID); err != nil {
			cm.logger.Error("Failed to deactivate expired credential", map[string]interface{}{
				"credential_id": cred.ID.String(),
				"tenant_id":     cred.TenantID.String(),
				"error":         err.Error(),
			})
			continue
		}

		// Audit log
		cm.audit.LogAuthAttempt(ctx, auth.AuditEvent{
			EventType: "credential_expired_deactivated",
			TenantID:  cred.TenantID.String(),
			Success:   true,
			Metadata: map[string]interface{}{
				"credential_id":   cred.ID.String(),
				"credential_name": cred.CredentialName,
				"expired_at":      cred.ExpiresAt.Format(time.RFC3339),
			},
		})

		deactivatedCount++
	}

	if deactivatedCount > 0 {
		cm.logger.Info("Deactivated expired credentials", map[string]interface{}{
			"count": deactivatedCount,
		})
	}

	return deactivatedCount, nil
}

// CheckInactive retrieves credentials that haven't been used recently
func (cm *CredentialManager) CheckInactive(ctx context.Context, threshold time.Duration) ([]*repository.TenantCredential, error) {
	since := time.Now().Add(-threshold)
	creds, err := cm.repo.ListUnusedSince(ctx, since)
	if err != nil {
		cm.logger.Error("Failed to check inactive credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to check inactive credentials: %w", err)
	}

	if len(creds) > 0 {
		cm.logger.Info("Found inactive credentials", map[string]interface{}{
			"count":     len(creds),
			"threshold": threshold.String(),
		})
	}

	return creds, nil
}
