package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CreateAPIKeyRequest represents a request to create a new API key
type CreateAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required"`
	TenantID  string     `json:"tenant_id" binding:"required"`
	UserID    string     `json:"user_id"`
	KeyType   KeyType    `json:"key_type" binding:"required"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`

	// Gateway-specific
	AllowedServices []string `json:"allowed_services,omitempty"`
	ParentKeyID     *string  `json:"parent_key_id,omitempty"`

	// Rate limiting
	RateLimit *int `json:"rate_limit,omitempty"`
}

// CreateAPIKeyWithType creates a new API key with the specified type
func (s *Service) CreateAPIKeyWithType(ctx context.Context, req CreateAPIKeyRequest) (*APIKey, error) {
	// Validate key type
	if !req.KeyType.Valid() {
		return nil, fmt.Errorf("invalid key type: %s", req.KeyType)
	}

	// Parse tenant ID
	tenantUUID, err := uuid.Parse(req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Parse or generate user ID
	var userUUID uuid.UUID
	if req.UserID == "" || req.UserID == "system" {
		userUUID = SystemUserID
	} else {
		userUUID, err = uuid.Parse(req.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
	}

	// Generate secure random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Create key string: prefix + base64(random)
	keyString := fmt.Sprintf("%s_%s", generatePrefix(req.KeyType), base64.URLEncoding.EncodeToString(keyBytes))
	keyHash := s.hashAPIKey(keyString)
	keyPrefix := keyString[:8]

	// Set default rate limit based on key type
	rateLimit := req.KeyType.GetRateLimit()
	if req.RateLimit != nil {
		rateLimit = *req.RateLimit
	}

	// Set default scopes if not provided
	if len(req.Scopes) == 0 {
		req.Scopes = req.KeyType.GetScopes()
	}

	// Insert into database if available
	if s.db != nil {
		query := `
			INSERT INTO mcp.api_keys (
				id, key_hash, key_prefix, tenant_id, user_id, name, key_type,
				scopes, is_active, expires_at, rate_limit,
				rate_window, parent_key_id, allowed_services,
				created_at, updated_at
			) VALUES (
				uuid_generate_v4(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14
			) RETURNING id, created_at
		`

		var id string
		var createdAt time.Time

		// Handle nullable user_id
		var userID sql.NullString
		if req.UserID != "" && req.UserID != "system" {
			userID = sql.NullString{String: req.UserID, Valid: true}
		}

		err := s.db.QueryRowContext(ctx, query,
			keyHash, keyPrefix, req.TenantID, userID, req.Name, req.KeyType,
			pq.Array(req.Scopes), true, req.ExpiresAt, rateLimit, 60,
			req.ParentKeyID, pq.Array(req.AllowedServices), time.Now(),
		).Scan(&id, &createdAt)

		if err != nil {
			return nil, fmt.Errorf("failed to create API key in database: %w", err)
		}

		// Log API key creation
		s.logger.Info("API key created", map[string]interface{}{
			"key_id":    id,
			"key_type":  req.KeyType,
			"tenant_id": req.TenantID,
			"key_name":  req.Name,
		})

		return &APIKey{
			Key:                    keyString, // Only returned once
			KeyPrefix:              keyPrefix,
			TenantID:               tenantUUID,
			UserID:                 userUUID,
			Name:                   req.Name,
			KeyType:                req.KeyType,
			Scopes:                 req.Scopes,
			Active:                 true,
			CreatedAt:              createdAt,
			ExpiresAt:              req.ExpiresAt,
			AllowedServices:        req.AllowedServices,
			ParentKeyID:            req.ParentKeyID,
			RateLimitRequests:      rateLimit,
			RateLimitWindowSeconds: 60,
		}, nil
	}

	// If no database, store in memory
	apiKey := &APIKey{
		Key:                    keyString,
		KeyHash:                keyHash,
		KeyPrefix:              keyPrefix,
		TenantID:               tenantUUID,
		UserID:                 userUUID,
		Name:                   req.Name,
		KeyType:                req.KeyType,
		Scopes:                 req.Scopes,
		ExpiresAt:              req.ExpiresAt,
		CreatedAt:              time.Now(),
		Active:                 true,
		AllowedServices:        req.AllowedServices,
		ParentKeyID:            req.ParentKeyID,
		RateLimitRequests:      rateLimit,
		RateLimitWindowSeconds: 60,
	}

	s.mu.Lock()
	s.apiKeys[keyString] = apiKey
	s.mu.Unlock()

	s.logger.Info("API key created in memory", map[string]interface{}{
		"key_prefix": keyPrefix,
		"key_type":   req.KeyType,
		"tenant_id":  req.TenantID,
		"key_name":   req.Name,
	})

	return apiKey, nil
}

// hashAPIKey generates a SHA256 hash of the API key
func (s *Service) hashAPIKey(apiKey string) string {
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	return hex.EncodeToString(hasher.Sum(nil))
}

// updateLastUsed updates the last used timestamp for an API key
func (s *Service) updateLastUsed(ctx context.Context, keyHash string) {
	if s.db == nil {
		return
	}

	query := `UPDATE mcp.api_keys SET last_used_at = $1 WHERE key_hash = $2`
	if _, err := s.db.ExecContext(ctx, query, time.Now(), keyHash); err != nil {
		// Log warning but don't fail - last_used is not critical
		if s.logger != nil {
			s.logger.Warn("Failed to update API key last used timestamp", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

// generatePrefix generates a key prefix based on the key type
func generatePrefix(keyType KeyType) string {
	switch keyType {
	case KeyTypeAdmin:
		return "adm"
	case KeyTypeGateway:
		return "gw"
	case KeyTypeAgent:
		return "agt"
	default:
		return "usr"
	}
}

// getKeyPrefix extracts the key prefix for logging
func getKeyPrefix(apiKey string) string {
	if len(apiKey) > 8 {
		return apiKey[:8]
	}
	return apiKey
}
