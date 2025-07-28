// Package auth provides centralized authentication and authorization for the Developer Mesh platform
package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// Common errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrNoAPIKey           = errors.New("no API key provided")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrInsufficientScope  = errors.New("insufficient scope")
)

// Type represents the type of authentication
type Type string

const (
	TypeAPIKey Type = "api_key"
	TypeJWT    Type = "jwt"
	TypeNone   Type = "none"
)

// Claims represents JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id"`
	Scopes   []string `json:"scopes,omitempty"`
	Email    string   `json:"email,omitempty"`
}

// APIKey represents an API key
type APIKey struct {
	Key       string     `db:"key"`
	KeyHash   string     `db:"key_hash"`
	KeyPrefix string     `db:"key_prefix"`
	TenantID  string     `db:"tenant_id"`
	UserID    string     `db:"user_id"`
	Name      string     `db:"name"`
	KeyType   KeyType    `db:"key_type"` // NEW
	Scopes    []string   `db:"scopes"`
	ExpiresAt *time.Time `db:"expires_at"`
	CreatedAt time.Time  `db:"created_at"`
	LastUsed  *time.Time `db:"last_used"`
	Active    bool       `db:"is_active"`

	// Gateway-specific fields
	ParentKeyID     *string  `db:"parent_key_id"`    // NEW
	AllowedServices []string `db:"allowed_services"` // NEW

	// Rate limiting
	RateLimitRequests      int `db:"rate_limit_requests"`
	RateLimitWindowSeconds int `db:"rate_limit_window_seconds"`
}

// User represents an authenticated user
type User struct {
	ID       string                 `json:"id"`
	TenantID string                 `json:"tenant_id"`
	Email    string                 `json:"email,omitempty"`
	Scopes   []string               `json:"scopes,omitempty"`
	AuthType Type                   `json:"auth_type"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceConfig represents auth configuration
type ServiceConfig struct {
	JWTSecret         string
	JWTExpiration     time.Duration
	APIKeyHeader      string
	EnableAPIKeys     bool
	EnableJWT         bool
	CacheEnabled      bool
	CacheTTL          time.Duration
	MaxFailedAttempts int
	LockoutDuration   time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *ServiceConfig {
	return &ServiceConfig{
		JWTExpiration:     24 * time.Hour,
		APIKeyHeader:      "X-API-Key",
		EnableAPIKeys:     true,
		EnableJWT:         true,
		CacheEnabled:      true,
		CacheTTL:          5 * time.Minute,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
	}
}

// Service provides authentication services
type Service struct {
	config *ServiceConfig
	db     *sqlx.DB
	cache  cache.Cache
	logger observability.Logger

	// In-memory storage for development/testing
	apiKeys map[string]*APIKey
	mu      sync.RWMutex
}

// NewService creates a new auth service
func NewService(config *ServiceConfig, db *sqlx.DB, cache cache.Cache, logger observability.Logger) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	return &Service{
		config:  config,
		db:      db,
		cache:   cache,
		logger:  logger,
		apiKeys: make(map[string]*APIKey),
	}
}

// logDebug logs a debug message if logger is available
func (s *Service) logDebug(msg string, fields map[string]interface{}) {
	if s.logger != nil {
		s.logger.Debug(msg, fields)
	}
}

// logInfo logs an info message if logger is available
func (s *Service) logInfo(msg string, fields map[string]interface{}) {
	if s.logger != nil {
		s.logger.Info(msg, fields)
	}
}

// logWarn logs a warning message if logger is available
func (s *Service) logWarn(msg string, fields map[string]interface{}) {
	if s.logger != nil {
		s.logger.Warn(msg, fields)
	}
}

// logError logs an error message if logger is available
func (s *Service) logError(msg string, fields map[string]interface{}) {
	if s.logger != nil {
		s.logger.Error(msg, fields)
	}
}

// ValidateAPIKey validates an API key and returns the associated user
func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (*User, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}

	// Validate API key format - only allow alphanumeric, dash, and underscore
	// This prevents any potential injection attacks even though we use parameterized queries
	if !isValidAPIKeyFormat(apiKey) {
		return nil, ErrInvalidAPIKey
	}

	s.logDebug("ValidateAPIKey called", map[string]interface{}{
		"key_prefix":         getKeyPrefix(apiKey),
		"has_db":             s.db != nil,
		"cache_enabled":      s.config != nil && s.config.CacheEnabled,
		"api_keys_in_memory": len(s.apiKeys),
		"db_type":            fmt.Sprintf("%T", s.db),
	})

	// Check cache first if enabled
	if s.config != nil && s.config.CacheEnabled && s.cache != nil {
		cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
		var cachedUser User
		if err := s.cache.Get(ctx, cacheKey, &cachedUser); err == nil {
			// Return the properly deserialized user from cache
			cachedUser.AuthType = TypeAPIKey // Ensure auth type is set
			return &cachedUser, nil
		}
	}

	// Check in-memory storage (for development)
	s.mu.RLock()
	key, exists := s.apiKeys[apiKey]
	// Always log for debugging auth issues
	if s.logger != nil {
		s.logInfo("Checking API key", map[string]interface{}{
			"provided_key_suffix": getKeyPrefix(apiKey),
			"exists":              exists,
			"total_keys_loaded":   len(s.apiKeys),
		})
	}
	s.mu.RUnlock()

	// If not found in memory and we have a database connection (production mode)
	if !exists && s.db != nil {
		s.logDebug("API key not in memory, checking database", map[string]interface{}{
			"key_prefix": getKeyPrefix(apiKey),
		})

		// Hash the API key to match stored hash
		keyHash := s.hashAPIKey(apiKey)

		// Query database for the API key
		query := `
			SELECT tenant_id, user_id, name, key_type, scopes, is_active, 
			       expires_at, rate_limit_requests, allowed_services
			FROM mcp.api_keys 
			WHERE key_hash = $1 AND is_active = true
		`

		var dbKey struct {
			TenantID        string         `db:"tenant_id"`
			UserID          sql.NullString `db:"user_id"`
			Name            string         `db:"name"`
			KeyType         string         `db:"key_type"`
			Scopes          pq.StringArray `db:"scopes"`
			Active          bool           `db:"is_active"`
			ExpiresAt       *time.Time     `db:"expires_at"`
			RateLimit       *int           `db:"rate_limit_requests"`
			AllowedServices pq.StringArray `db:"allowed_services"`
		}

		err := s.db.Get(&dbKey, query, keyHash)
		if err != nil {
			if err == sql.ErrNoRows {
				s.logInfo("API key not found in database", map[string]interface{}{
					"key_prefix": getKeyPrefix(apiKey),
				})
				return nil, ErrInvalidAPIKey
			}
			s.logError("Failed to query API key from database", map[string]interface{}{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("database error: %w", err)
		}

		// Check expiration
		if dbKey.ExpiresAt != nil && time.Now().After(*dbKey.ExpiresAt) {
			s.logInfo("API key expired", map[string]interface{}{
				"key_prefix": getKeyPrefix(apiKey),
				"expired_at": dbKey.ExpiresAt.Format(time.RFC3339),
			})
			return nil, ErrInvalidAPIKey
		}

		// Build user object
		userID := ""
		if dbKey.UserID.Valid {
			userID = dbKey.UserID.String
		}

		user := &User{
			ID:       userID,
			TenantID: dbKey.TenantID,
			Scopes:   []string(dbKey.Scopes),
			AuthType: TypeAPIKey,
			Metadata: map[string]interface{}{
				"key_type":         dbKey.KeyType,
				"key_name":         dbKey.Name,
				"allowed_services": []string(dbKey.AllowedServices),
			},
		}

		// Update last used timestamp asynchronously
		go s.updateLastUsed(ctx, keyHash)

		// Cache the result
		if s.config.CacheEnabled && s.cache != nil {
			cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
			if err := s.cache.Set(ctx, cacheKey, user, s.config.CacheTTL); err != nil {
				s.logWarn("Failed to cache API key validation", map[string]interface{}{"error": err})
			}
		}

		s.logInfo("API key validated from database", map[string]interface{}{
			"key_prefix": getKeyPrefix(apiKey),
			"tenant_id":  user.TenantID,
			"key_type":   dbKey.KeyType,
		})

		return user, nil
	}

	if exists && key.Active {
		// Check expiration
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			return nil, ErrInvalidAPIKey
		}

		user := &User{
			ID:       key.UserID,
			TenantID: key.TenantID,
			Scopes:   key.Scopes,
			AuthType: TypeAPIKey,
			Metadata: map[string]interface{}{
				"key_type":         string(key.KeyType),
				"key_name":         key.Name,
				"allowed_services": key.AllowedServices,
			},
		}

		// Update last used timestamp asynchronously
		go func() {
			now := time.Now()
			s.mu.Lock()
			if k, ok := s.apiKeys[apiKey]; ok {
				k.LastUsed = &now
			}
			s.mu.Unlock()
		}()

		// Cache the result
		if s.config.CacheEnabled && s.cache != nil {
			cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
			// Cache the entire user object for proper retrieval
			if err := s.cache.Set(ctx, cacheKey, user, s.config.CacheTTL); err != nil {
				s.logWarn("Failed to cache API key validation", map[string]interface{}{"error": err})
			}
		}

		return user, nil
	}

	// Check database if available
	if s.db != nil {
		// Hash the API key for database lookup
		keyHash := s.hashAPIKey(apiKey)

		// Extract key prefix for additional validation
		keyPrefix := getKeyPrefix(apiKey)

		s.logDebug("Querying database for API key", map[string]interface{}{
			"key_prefix":   keyPrefix,
			"key_hash_len": len(keyHash),
			"db_connected": s.db != nil,
			"db_type":      fmt.Sprintf("%T", s.db),
			"key_hash":     keyHash,
		})

		var dbKey struct {
			ID              string         `db:"id"`
			KeyPrefix       string         `db:"key_prefix"`
			TenantID        string         `db:"tenant_id"`
			UserID          *string        `db:"user_id"`
			Name            string         `db:"name"`
			KeyType         KeyType        `db:"key_type"`
			Scopes          pq.StringArray `db:"scopes"`
			ExpiresAt       *time.Time     `db:"expires_at"`
			IsActive        bool           `db:"is_active"`
			ParentKeyID     *string        `db:"parent_key_id"`
			AllowedServices pq.StringArray `db:"allowed_services"`
		}

		query := `
			SELECT id, key_prefix, tenant_id, user_id, name, key_type, scopes, 
			       expires_at, is_active, parent_key_id, allowed_services
			FROM mcp.api_keys
			WHERE key_hash = $1 AND key_prefix = $2 AND is_active = true
		`
		err := s.db.GetContext(ctx, &dbKey, query, keyHash, keyPrefix)
		if err != nil {
			if err == sql.ErrNoRows {
				s.logDebug("API key not found in database", map[string]interface{}{
					"key_prefix": keyPrefix,
					"error":      "no rows",
				})
				return nil, ErrInvalidAPIKey
			}
			s.logError("Database query error", map[string]interface{}{
				"error": err.Error(),
				"query": query,
			})
			return nil, fmt.Errorf("database error: %w", err)
		}

		s.logDebug("API key found in database", map[string]interface{}{
			"key_prefix": keyPrefix,
			"tenant_id":  dbKey.TenantID,
			"is_active":  dbKey.IsActive,
		})

		// Check expiration
		if dbKey.ExpiresAt != nil && time.Now().After(*dbKey.ExpiresAt) {
			return nil, ErrInvalidAPIKey
		}

		// Default user ID if not set - use a system user UUID
		userID := "00000000-0000-0000-0000-000000000000" // System user UUID
		if dbKey.UserID != nil {
			userID = *dbKey.UserID
		}

		user := &User{
			ID:       userID,
			TenantID: dbKey.TenantID,
			Scopes:   dbKey.Scopes,
			AuthType: TypeAPIKey,
			Metadata: map[string]interface{}{
				"key_type":         dbKey.KeyType,
				"key_name":         dbKey.Name,
				"allowed_services": dbKey.AllowedServices,
			},
		}

		// Update last used timestamp asynchronously
		go s.updateLastUsed(context.Background(), keyHash)

		// Cache the result
		if s.config.CacheEnabled && s.cache != nil {
			cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
			// Cache the entire user object for proper retrieval
			if err := s.cache.Set(ctx, cacheKey, user, s.config.CacheTTL); err != nil {
				s.logWarn("Failed to cache API key from database", map[string]interface{}{"error": err})
			}
		}

		return user, nil
	}

	s.logWarn("API key validation failed - no database connection", map[string]interface{}{
		"key_prefix": getKeyPrefix(apiKey),
		"has_db":     s.db != nil,
	})

	return nil, ErrInvalidAPIKey
}

// storeAPIKeyInDB stores an API key in the database
func (s *Service) storeAPIKeyInDB(rawKey string, apiKey *APIKey) error {
	// Hash the API key
	hasher := sha256.New()
	hasher.Write([]byte(rawKey))
	keyHash := hex.EncodeToString(hasher.Sum(nil))

	// Extract key prefix
	keyPrefix := rawKey
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}

	// Check if key already exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM mcp.api_keys WHERE key_hash = $1)`
	if err := s.db.Get(&exists, checkQuery, keyHash); err != nil {
		return fmt.Errorf("failed to check existing key: %w", err)
	}

	if exists {
		// Update the existing key
		updateQuery := `
			UPDATE mcp.api_keys 
			SET name = $2, scopes = $3, is_active = $4, key_type = $5, updated_at = CURRENT_TIMESTAMP
			WHERE key_hash = $1
		`
		_, err := s.db.Exec(updateQuery, keyHash, apiKey.Name, pq.Array(apiKey.Scopes), apiKey.Active, apiKey.KeyType)
		return err
	}

	// Insert new key
	insertQuery := `
		INSERT INTO mcp.api_keys (
			id, key_hash, key_prefix, tenant_id, user_id, name, key_type, scopes, 
			is_active, created_at, updated_at
		) VALUES (
			uuid_generate_v4(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $9
		)
	`

	// Handle user_id - use NULL if it's "system" or empty
	var userID sql.NullString
	if apiKey.UserID != "" && apiKey.UserID != "system" {
		userID = sql.NullString{String: apiKey.UserID, Valid: true}
	}

	_, err := s.db.Exec(insertQuery,
		keyHash, keyPrefix, apiKey.TenantID, userID,
		apiKey.Name, apiKey.KeyType, pq.Array(apiKey.Scopes), apiKey.Active, time.Now())

	return err
}

// ValidateJWT validates a JWT token and returns the associated user
func (s *Service) ValidateJWT(ctx context.Context, tokenString string) (*User, error) {
	if tokenString == "" || s.config == nil || s.config.JWTSecret == "" {
		return nil, ErrInvalidToken
	}

	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	// Validate claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check expiration (jwt library handles this, but we can add custom logic)
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrTokenExpired
	}

	// Create user from claims
	user := &User{
		ID:       claims.UserID,
		TenantID: claims.TenantID,
		Email:    claims.Email,
		Scopes:   claims.Scopes,
		AuthType: TypeJWT,
	}

	return user, nil
}

// GenerateJWT generates a new JWT token for a user
func (s *Service) GenerateJWT(ctx context.Context, user *User) (string, error) {
	if s.config.JWTSecret == "" {
		return "", errors.New("JWT secret not configured")
	}

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWTExpiration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        generateID(), // You would implement this
		},
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Scopes:   user.Scopes,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// CreateAPIKey creates a new API key
func (s *Service) CreateAPIKey(ctx context.Context, tenantID, userID, name string, scopes []string, expiresAt *time.Time) (*APIKey, error) {
	// Generate a secure random key
	keyStr := generateAPIKey() // You would implement this

	apiKey := &APIKey{
		Key:       keyStr,
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		Active:    true,
	}

	// Store in memory (for development)
	s.mu.Lock()
	s.apiKeys[keyStr] = apiKey
	s.mu.Unlock()

	// Store in database if available
	if s.db != nil {
		query := `
			INSERT INTO api_keys (key, tenant_id, user_id, name, scopes, expires_at, created_at, active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		_, err := s.db.ExecContext(ctx, query,
			apiKey.Key, apiKey.TenantID, apiKey.UserID, apiKey.Name,
			apiKey.Scopes, apiKey.ExpiresAt, apiKey.CreatedAt, apiKey.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create API key: %w", err)
		}
	}

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (s *Service) RevokeAPIKey(ctx context.Context, apiKey string) error {
	// Remove from memory
	s.mu.Lock()
	delete(s.apiKeys, apiKey)
	s.mu.Unlock()

	// Update in database if available
	if s.db != nil {
		query := `UPDATE api_keys SET active = false WHERE key = $1`
		_, err := s.db.ExecContext(ctx, query, apiKey)
		if err != nil {
			return fmt.Errorf("failed to revoke API key: %w", err)
		}
	}

	// Remove from cache
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := fmt.Sprintf("auth:apikey:%s", apiKey)
		if err := s.cache.Delete(ctx, cacheKey); err != nil {
			s.logWarn("Failed to delete API key from cache", map[string]interface{}{"error": err})
		}
	}

	return nil
}

// AuthorizeScopes checks if a user has the required scopes
func (s *Service) AuthorizeScopes(user *User, requiredScopes []string) error {
	if len(requiredScopes) == 0 {
		return nil // No scopes required
	}

	userScopeMap := make(map[string]bool)
	for _, scope := range user.Scopes {
		userScopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if !userScopeMap[required] {
			return ErrInsufficientScope
		}
	}

	return nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a password with its hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Helper functions (implement these based on your needs)
func generateID() string {
	// Implement UUID generation
	return fmt.Sprintf("id_%d", time.Now().UnixNano())
}

func generateAPIKey() string {
	// Implement secure random key generation
	// In production, use crypto/rand
	return fmt.Sprintf("mcp_%s_%d", strings.ReplaceAll(time.Now().Format("20060102"), "-", ""), time.Now().UnixNano())
}

// GetConfig returns the service configuration
func (s *Service) GetConfig() *ServiceConfig {
	return s.config
}

// InitializeDefaultAPIKeys initializes default API keys from configuration
func (s *Service) InitializeDefaultAPIKeys(keys map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, role := range keys {
		// Parse role to determine scopes
		var scopes []string
		switch role {
		case "admin":
			scopes = []string{"read", "write", "admin"}
		case "write":
			scopes = []string{"read", "write"}
		case "read":
			scopes = []string{"read"}
		default:
			scopes = []string{"read"}
		}

		s.apiKeys[key] = &APIKey{
			Key:       key,
			TenantID:  "default",
			UserID:    "system",
			Name:      fmt.Sprintf("Default %s key", role),
			Scopes:    scopes,
			CreatedAt: time.Now(),
			Active:    true,
		}

		s.logInfo("Initialized default API key", map[string]interface{}{
			"key_suffix": key[len(key)-4:], // Log only last 4 chars for security
			"role":       role,
			"scopes":     scopes,
		})
	}
}

// InitializeAPIKeysWithConfig initializes API keys with full configuration including tenant IDs
func (s *Service) InitializeAPIKeysWithConfig(keysConfig map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, config := range keysConfig {
		var apiKey *APIKey

		switch v := config.(type) {
		case string:
			// Simple role string - use defaults
			var scopes []string
			switch v {
			case "admin":
				scopes = []string{"read", "write", "admin"}
			case "write":
				scopes = []string{"read", "write"}
			case "read", "reader":
				scopes = []string{"read"}
			default:
				scopes = []string{"read"}
			}

			apiKey = &APIKey{
				Key:       key,
				TenantID:  "default",
				UserID:    "system",
				Name:      fmt.Sprintf("Default %s key", v),
				Scopes:    scopes,
				CreatedAt: time.Now(),
				Active:    true,
			}

		case map[string]interface{}:
			// Full configuration with tenant_id, scopes, etc.
			role, _ := v["role"].(string)
			tenantID, _ := v["tenant_id"].(string)
			if tenantID == "" {
				tenantID = "default"
			}

			// Get scopes from config or derive from role
			var scopes []string
			if scopesInterface, ok := v["scopes"].([]interface{}); ok {
				for _, s := range scopesInterface {
					if scope, ok := s.(string); ok {
						scopes = append(scopes, scope)
					}
				}
			} else {
				// Derive from role
				switch role {
				case "admin":
					scopes = []string{"read", "write", "admin"}
				case "write":
					scopes = []string{"read", "write"}
				case "read", "reader":
					scopes = []string{"read"}
				default:
					scopes = []string{"read"}
				}
			}

			// Check if user_id is provided in config
			userID, _ := v["user_id"].(string)
			if userID == "" {
				// Generate a unique user ID based on the key
				// This ensures each API key has a unique agent ID
				userID = fmt.Sprintf("user-%s", key)
			}

			apiKey = &APIKey{
				Key:       key,
				TenantID:  tenantID,
				UserID:    userID,
				Name:      fmt.Sprintf("%s key for %s", role, tenantID),
				Scopes:    scopes,
				CreatedAt: time.Now(),
				Active:    true,
			}
		}

		if apiKey != nil {
			s.apiKeys[key] = apiKey
			s.logInfo("Initialized API key with config", map[string]interface{}{
				"key_suffix": key[len(key)-4:], // Log only last 4 chars for security
				"tenant_id":  apiKey.TenantID,
				"scopes":     apiKey.Scopes,
			})
		}
	}
}

// AddAPIKey adds an API key to the service at runtime (thread-safe)
func (s *Service) AddAPIKey(key string, settings APIKeySettings) error {
	// Validation
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if len(key) < 16 {
		return fmt.Errorf("API key too short (minimum 16 characters)")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create API key object
	apiKey := &APIKey{
		Key:       key,
		TenantID:  settings.TenantID,
		UserID:    "system",
		Name:      fmt.Sprintf("%s API key", settings.Role),
		Scopes:    settings.Scopes,
		Active:    true,
		CreatedAt: time.Now(),
	}

	// Apply defaults
	if apiKey.TenantID == "" {
		apiKey.TenantID = "default"
	}
	if len(apiKey.Scopes) == 0 {
		apiKey.Scopes = []string{"read"} // Minimum scope
	}

	// Handle expiration
	if settings.ExpiresIn != "" {
		duration, err := time.ParseDuration(settings.ExpiresIn)
		if err != nil {
			return fmt.Errorf("invalid expiration duration %q: %w", settings.ExpiresIn, err)
		}
		if duration < 0 {
			return fmt.Errorf("expiration duration cannot be negative")
		}
		expiresAt := time.Now().Add(duration)
		apiKey.ExpiresAt = &expiresAt
	}

	// Store in memory
	s.apiKeys[key] = apiKey

	// Persist to database if available
	if s.db != nil {
		if err := s.persistAPIKey(context.Background(), apiKey); err != nil {
			// Log but don't fail - memory storage sufficient for operation
			s.logWarn("Failed to persist API key", map[string]interface{}{
				"key_suffix": lastN(key, 4),
				"error":      err.Error(),
			})
		}
	}

	s.logInfo("API key added", map[string]interface{}{
		"key_suffix": lastN(key, 4),
		"role":       settings.Role,
		"scopes":     settings.Scopes,
		"tenant_id":  apiKey.TenantID,
	})

	return nil
}

// persistAPIKey saves to database with upsert semantics
func (s *Service) persistAPIKey(ctx context.Context, apiKey *APIKey) error {
	query := `
        INSERT INTO api_keys (
            key, tenant_id, user_id, name, scopes, 
            expires_at, created_at, active
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (key) DO UPDATE SET
            scopes = EXCLUDED.scopes,
            expires_at = EXCLUDED.expires_at,
            active = EXCLUDED.active,
            updated_at = NOW()
    `

	_, err := s.db.ExecContext(ctx, query,
		apiKey.Key,
		apiKey.TenantID,
		apiKey.UserID,
		apiKey.Name,
		apiKey.Scopes,
		apiKey.ExpiresAt,
		apiKey.CreatedAt,
		apiKey.Active,
	)

	return err
}

// isValidAPIKeyFormat validates that the API key contains only safe characters
// This prevents any potential injection attacks even though we use parameterized queries
var apiKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isValidAPIKeyFormat(apiKey string) bool {
	// Allow alphanumeric characters, underscore, and dash
	// Most API keys use base64 or similar encoding
	return len(apiKey) > 0 && len(apiKey) <= 256 && apiKeyRegex.MatchString(apiKey)
}
