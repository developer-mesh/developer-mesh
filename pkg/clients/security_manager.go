package clients

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
)

// SecurityManager provides comprehensive security features for the REST client
type SecurityManager struct {
	// mu     sync.RWMutex // TODO: Implement locking when methods are added
	logger observability.Logger

	// Token management
	tokenManager *TokenManager

	// Encryption
	encryptor *DataEncryptor

	// Rate limiting
	rateLimiter *TenantRateLimiter

	// Input validation
	validator *InputValidator

	// Audit logging
	auditLogger *AuditLogger

	// Threat detection
	threatDetector *ThreatDetector

	// Security metrics
	metrics *SecurityMetrics

	// Configuration
	config SecurityConfig
}

// SecurityConfig defines security configuration
type SecurityConfig struct {
	// Token management
	TokenRotationInterval time.Duration `json:"token_rotation_interval"`
	TokenTTL              time.Duration `json:"token_ttl"`
	MaxTokensPerUser      int           `json:"max_tokens_per_user"`

	// Encryption
	EncryptionEnabled   bool          `json:"encryption_enabled"`
	EncryptionAlgorithm string        `json:"encryption_algorithm"`
	KeyRotationInterval time.Duration `json:"key_rotation_interval"`

	// Rate limiting
	RateLimitEnabled  bool `json:"rate_limit_enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	BurstSize         int  `json:"burst_size"`

	// Security headers
	EnableSecurityHeaders bool   `json:"enable_security_headers"`
	CSPPolicy             string `json:"csp_policy"`

	// Audit logging
	AuditEnabled   bool          `json:"audit_enabled"`
	AuditRetention time.Duration `json:"audit_retention"`

	// Threat detection
	ThreatDetectionEnabled bool    `json:"threat_detection_enabled"`
	AnomalyThreshold       float64 `json:"anomaly_threshold"`
}

// TokenManager handles token lifecycle management
type TokenManager struct {
	mu sync.RWMutex

	// Token storage
	tokens     map[string]*Token
	userTokens map[string][]string // user ID -> token IDs

	// Configuration
	rotationInterval time.Duration
	tokenTTL         time.Duration
	maxTokensPerUser int

	// Rotation tracking
	lastRotation    time.Time
	rotationHistory []TokenRotation
}

// Token represents an API token
type Token struct {
	ID          string
	UserID      string
	TenantID    string
	Value       string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	LastUsed    time.Time
	UseCount    int64
	Permissions []string
	Metadata    map[string]string
	IsActive    bool
}

// TokenRotation tracks token rotation events
type TokenRotation struct {
	OldTokenID string
	NewTokenID string
	UserID     string
	Timestamp  time.Time
	Reason     string
}

// DataEncryptor handles data encryption/decryption
type DataEncryptor struct {
	mu sync.RWMutex

	// Encryption keys
	currentKey   []byte
	previousKeys [][]byte
	keyVersion   int

	// Key rotation
	lastKeyRotation  time.Time
	rotationInterval time.Duration

	// Statistics
	encryptionCount int64
	decryptionCount int64
}

// TenantRateLimiter implements per-tenant rate limiting
type TenantRateLimiter struct {
	mu sync.RWMutex

	// Tenant limits
	tenantLimits map[string]*RateLimit
	defaultLimit *RateLimit

	// Configuration
	enabled         bool
	cleanupInterval time.Duration

	// Statistics
	limitExceeded map[string]int64
}

// RateLimit defines rate limiting parameters
type RateLimit struct {
	RequestsPerMinute int
	BurstSize         int
	tokens            float64
	lastRefill        time.Time
	mu                sync.Mutex
}

// InputValidator validates and sanitizes input
type InputValidator struct {
	mu sync.RWMutex

	// Validation rules
	rules map[string]ValidationRule

	// Sanitization patterns
	sanitizers map[string]Sanitizer

	// Statistics
	validationFailures map[string]int64
	injectionAttempts  int64
}

// ValidationRule defines input validation rules
type ValidationRule struct {
	Field       string
	Type        string // string, number, uuid, etc.
	Required    bool
	MinLength   int
	MaxLength   int
	Pattern     string
	CustomCheck func(interface{}) error
}

// Sanitizer cleans potentially malicious input
type Sanitizer struct {
	Type    string // sql, html, script, etc.
	Pattern string
	Replace string
}

// AuditLogger logs security-relevant events
type AuditLogger struct {
	mu sync.RWMutex

	// Event storage
	events    []AuditEvent
	maxEvents int

	// Configuration
	enabled   bool
	retention time.Duration

	// Event handlers
	handlers []AuditEventHandler
}

// AuditEvent represents a security audit event
type AuditEvent struct {
	ID        string
	Timestamp time.Time
	EventType string
	UserID    string
	TenantID  string
	IPAddress string
	UserAgent string
	Action    string
	Resource  string
	Result    string
	Details   map[string]interface{}
	Risk      string // low, medium, high, critical
}

// AuditEventHandler processes audit events
type AuditEventHandler func(event AuditEvent)

// ThreatDetector identifies suspicious activity
type ThreatDetector struct {
	mu sync.RWMutex

	// Activity tracking
	userActivity   map[string]*UserActivity
	tenantActivity map[string]*TenantActivity

	// Threat patterns
	threatPatterns []ThreatPattern

	// Configuration
	enabled          bool
	anomalyThreshold float64

	// Detected threats
	activeThreats map[string]*Threat
	threatHistory []Threat
}

// UserActivity tracks user behavior
type UserActivity struct {
	UserID          string
	RequestCount    int64
	ErrorCount      int64
	UniqueEndpoints map[string]int
	LastActivity    time.Time
	SuspiciousScore float64
	Blocked         bool
}

// TenantActivity tracks tenant-level activity
type TenantActivity struct {
	TenantID     string
	RequestCount int64
	UniqueUsers  int
	DataAccess   int64
	LastActivity time.Time
	RiskScore    float64
}

// ThreatPattern defines suspicious behavior patterns
type ThreatPattern struct {
	Name        string
	Description string
	Detector    func(*UserActivity) bool
	Severity    string
	Action      string // log, alert, block
}

// Threat represents a detected security threat
type Threat struct {
	ID          string
	Type        string
	UserID      string
	TenantID    string
	Severity    string
	DetectedAt  time.Time
	Description string
	Evidence    map[string]interface{}
	Action      string
	Resolved    bool
}

// SecurityMetrics tracks security metrics
type SecurityMetrics struct {
	mu sync.RWMutex

	// Authentication metrics
	AuthAttempts   int64
	AuthSuccesses  int64
	AuthFailures   int64
	TokenRotations int64

	// Encryption metrics
	DataEncrypted    int64
	DataDecrypted    int64
	EncryptionErrors int64

	// Rate limiting metrics
	RateLimitHits     int64
	RateLimitExceeded int64

	// Validation metrics
	ValidationErrors int64
	InjectionBlocked int64

	// Threat metrics
	ThreatsDetected int64
	ThreatsBlocked  int64
	ActiveThreats   int

	// Audit metrics
	AuditEventsLogged int64
	HighRiskEvents    int64
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		TokenRotationInterval:  24 * time.Hour,
		TokenTTL:               7 * 24 * time.Hour,
		MaxTokensPerUser:       5,
		EncryptionEnabled:      true,
		EncryptionAlgorithm:    "AES-256-GCM",
		KeyRotationInterval:    30 * 24 * time.Hour,
		RateLimitEnabled:       true,
		RequestsPerMinute:      1000,
		BurstSize:              100,
		EnableSecurityHeaders:  true,
		CSPPolicy:              "default-src 'self'",
		AuditEnabled:           true,
		AuditRetention:         90 * 24 * time.Hour,
		ThreatDetectionEnabled: true,
		AnomalyThreshold:       0.8,
	}
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config SecurityConfig, logger observability.Logger) (*SecurityManager, error) {
	manager := &SecurityManager{
		logger:  logger,
		config:  config,
		metrics: &SecurityMetrics{},
	}

	// Initialize token manager
	manager.tokenManager = &TokenManager{
		tokens:           make(map[string]*Token),
		userTokens:       make(map[string][]string),
		rotationInterval: config.TokenRotationInterval,
		tokenTTL:         config.TokenTTL,
		maxTokensPerUser: config.MaxTokensPerUser,
		lastRotation:     time.Now(),
		rotationHistory:  make([]TokenRotation, 0, 100),
	}

	// Initialize encryptor
	if config.EncryptionEnabled {
		key := make([]byte, 32) // AES-256
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate encryption key: %w", err)
		}

		manager.encryptor = &DataEncryptor{
			currentKey:       key,
			previousKeys:     make([][]byte, 0, 5),
			keyVersion:       1,
			lastKeyRotation:  time.Now(),
			rotationInterval: config.KeyRotationInterval,
		}
	}

	// Initialize rate limiter
	if config.RateLimitEnabled {
		manager.rateLimiter = &TenantRateLimiter{
			tenantLimits: make(map[string]*RateLimit),
			defaultLimit: &RateLimit{
				RequestsPerMinute: config.RequestsPerMinute,
				BurstSize:         config.BurstSize,
				tokens:            float64(config.BurstSize),
				lastRefill:        time.Now(),
			},
			enabled:         true,
			cleanupInterval: 5 * time.Minute,
			limitExceeded:   make(map[string]int64),
		}
	}

	// Initialize input validator
	manager.validator = &InputValidator{
		rules:              make(map[string]ValidationRule),
		sanitizers:         make(map[string]Sanitizer),
		validationFailures: make(map[string]int64),
	}
	manager.setupDefaultValidationRules()

	// Initialize audit logger
	if config.AuditEnabled {
		manager.auditLogger = &AuditLogger{
			events:    make([]AuditEvent, 0, 10000),
			maxEvents: 10000,
			enabled:   true,
			retention: config.AuditRetention,
			handlers:  make([]AuditEventHandler, 0),
		}
	}

	// Initialize threat detector
	if config.ThreatDetectionEnabled {
		manager.threatDetector = &ThreatDetector{
			userActivity:     make(map[string]*UserActivity),
			tenantActivity:   make(map[string]*TenantActivity),
			threatPatterns:   make([]ThreatPattern, 0),
			enabled:          true,
			anomalyThreshold: config.AnomalyThreshold,
			activeThreats:    make(map[string]*Threat),
			threatHistory:    make([]Threat, 0, 100),
		}
		manager.setupDefaultThreatPatterns()
	}

	// Start background workers
	go manager.tokenRotationWorker()
	go manager.keyRotationWorker()
	go manager.threatMonitoringWorker()

	return manager, nil
}

// Token Management Methods

// CreateToken creates a new API token
func (m *SecurityManager) CreateToken(ctx context.Context, userID, tenantID string, permissions []string) (*Token, error) {
	m.tokenManager.mu.Lock()
	defer m.tokenManager.mu.Unlock()

	// Check token limit for user
	if userTokens := m.tokenManager.userTokens[userID]; len(userTokens) >= m.tokenManager.maxTokensPerUser {
		// Revoke oldest token
		oldestToken := userTokens[0]
		if token, exists := m.tokenManager.tokens[oldestToken]; exists {
			token.IsActive = false
		}
		m.tokenManager.userTokens[userID] = userTokens[1:]
	}

	// Generate secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	token := &Token{
		ID:          uuid.New().String(),
		UserID:      userID,
		TenantID:    tenantID,
		Value:       base64.URLEncoding.EncodeToString(tokenBytes),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.tokenManager.tokenTTL),
		Permissions: permissions,
		Metadata:    make(map[string]string),
		IsActive:    true,
	}

	// Store token
	m.tokenManager.tokens[token.ID] = token
	m.tokenManager.userTokens[userID] = append(m.tokenManager.userTokens[userID], token.ID)

	// Audit log
	m.auditLog("token_created", userID, tenantID, map[string]interface{}{
		"token_id":    token.ID,
		"permissions": permissions,
	})

	m.metrics.mu.Lock()
	m.metrics.AuthAttempts++
	m.metrics.AuthSuccesses++
	m.metrics.mu.Unlock()

	return token, nil
}

// ValidateToken validates an API token
func (m *SecurityManager) ValidateToken(ctx context.Context, tokenValue string) (*Token, error) {
	m.tokenManager.mu.RLock()
	defer m.tokenManager.mu.RUnlock()

	// Find token by value
	for _, token := range m.tokenManager.tokens {
		if token.Value == tokenValue && token.IsActive {
			// Check expiration
			if time.Now().After(token.ExpiresAt) {
				return nil, fmt.Errorf("token expired")
			}

			// Update usage
			token.LastUsed = time.Now()
			token.UseCount++

			return token, nil
		}
	}

	m.metrics.mu.Lock()
	m.metrics.AuthAttempts++
	m.metrics.AuthFailures++
	m.metrics.mu.Unlock()

	return nil, fmt.Errorf("invalid token")
}

// RotateToken rotates an existing token
func (m *SecurityManager) RotateToken(ctx context.Context, oldTokenID string) (*Token, error) {
	m.tokenManager.mu.Lock()
	defer m.tokenManager.mu.Unlock()

	oldToken, exists := m.tokenManager.tokens[oldTokenID]
	if !exists || !oldToken.IsActive {
		return nil, fmt.Errorf("token not found or inactive")
	}

	// Create new token with same permissions
	newToken, err := m.CreateToken(ctx, oldToken.UserID, oldToken.TenantID, oldToken.Permissions)
	if err != nil {
		return nil, err
	}

	// Deactivate old token
	oldToken.IsActive = false

	// Record rotation
	m.tokenManager.rotationHistory = append(m.tokenManager.rotationHistory, TokenRotation{
		OldTokenID: oldTokenID,
		NewTokenID: newToken.ID,
		UserID:     oldToken.UserID,
		Timestamp:  time.Now(),
		Reason:     "manual_rotation",
	})

	m.metrics.mu.Lock()
	m.metrics.TokenRotations++
	m.metrics.mu.Unlock()

	return newToken, nil
}

// Data Encryption Methods

// EncryptData encrypts sensitive data
func (m *SecurityManager) EncryptData(data []byte) ([]byte, error) {
	if m.encryptor == nil {
		return data, nil // Encryption not enabled
	}

	m.encryptor.mu.RLock()
	defer m.encryptor.mu.RUnlock()

	// Create AES cipher
	block, err := aes.NewCipher(m.encryptor.currentKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to create nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Prepend key version
	result := append([]byte{byte(m.encryptor.keyVersion)}, ciphertext...)

	m.encryptor.encryptionCount++
	m.metrics.mu.Lock()
	m.metrics.DataEncrypted++
	m.metrics.mu.Unlock()

	return result, nil
}

// DecryptData decrypts encrypted data
func (m *SecurityManager) DecryptData(encryptedData []byte) ([]byte, error) {
	if m.encryptor == nil || len(encryptedData) < 1 {
		return encryptedData, nil
	}

	m.encryptor.mu.RLock()
	defer m.encryptor.mu.RUnlock()

	// Extract key version
	keyVersion := int(encryptedData[0])
	ciphertext := encryptedData[1:]

	// Select appropriate key
	var key []byte
	if keyVersion == m.encryptor.keyVersion {
		key = m.encryptor.currentKey
	} else {
		// Look for previous key
		if keyVersion > 0 && keyVersion <= len(m.encryptor.previousKeys) {
			key = m.encryptor.previousKeys[keyVersion-1]
		} else {
			return nil, fmt.Errorf("unknown key version: %d", keyVersion)
		}
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		m.metrics.mu.Lock()
		m.metrics.EncryptionErrors++
		m.metrics.mu.Unlock()
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	m.encryptor.decryptionCount++
	m.metrics.mu.Lock()
	m.metrics.DataDecrypted++
	m.metrics.mu.Unlock()

	return plaintext, nil
}

// Rate Limiting Methods

// CheckRateLimit checks if a request is within rate limits
func (m *SecurityManager) CheckRateLimit(ctx context.Context, tenantID string) error {
	if m.rateLimiter == nil || !m.rateLimiter.enabled {
		return nil
	}

	m.rateLimiter.mu.Lock()
	defer m.rateLimiter.mu.Unlock()

	// Get or create tenant limit
	limit, exists := m.rateLimiter.tenantLimits[tenantID]
	if !exists {
		limit = &RateLimit{
			RequestsPerMinute: m.rateLimiter.defaultLimit.RequestsPerMinute,
			BurstSize:         m.rateLimiter.defaultLimit.BurstSize,
			tokens:            float64(m.rateLimiter.defaultLimit.BurstSize),
			lastRefill:        time.Now(),
		}
		m.rateLimiter.tenantLimits[tenantID] = limit
	}

	// Token bucket algorithm
	limit.mu.Lock()
	defer limit.mu.Unlock()

	// Refill tokens
	elapsed := time.Since(limit.lastRefill)
	tokensToAdd := elapsed.Seconds() * float64(limit.RequestsPerMinute) / 60.0
	limit.tokens = min(limit.tokens+tokensToAdd, float64(limit.BurstSize))
	limit.lastRefill = time.Now()

	// Check if request allowed
	if limit.tokens >= 1 {
		limit.tokens--

		m.metrics.mu.Lock()
		m.metrics.RateLimitHits++
		m.metrics.mu.Unlock()

		return nil
	}

	// Rate limit exceeded
	m.rateLimiter.limitExceeded[tenantID]++

	m.metrics.mu.Lock()
	m.metrics.RateLimitExceeded++
	m.metrics.mu.Unlock()

	return fmt.Errorf("rate limit exceeded for tenant %s", tenantID)
}

// Input Validation Methods

// ValidateInput validates and sanitizes input data
func (m *SecurityManager) ValidateInput(field string, value interface{}) error {
	m.validator.mu.RLock()
	defer m.validator.mu.RUnlock()

	rule, exists := m.validator.rules[field]
	if !exists {
		return nil // No validation rule defined
	}

	// Type validation
	switch rule.Type {
	case "string":
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %s must be a string", field)
		}

		// Length validation
		if rule.MinLength > 0 && len(str) < rule.MinLength {
			return fmt.Errorf("field %s must be at least %d characters", field, rule.MinLength)
		}
		if rule.MaxLength > 0 && len(str) > rule.MaxLength {
			return fmt.Errorf("field %s must be at most %d characters", field, rule.MaxLength)
		}

		// SQL injection check
		if m.detectSQLInjection(str) {
			m.validator.injectionAttempts++
			m.metrics.mu.Lock()
			m.metrics.InjectionBlocked++
			m.metrics.mu.Unlock()
			return fmt.Errorf("potential SQL injection detected in field %s", field)
		}

	case "uuid":
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %s must be a string UUID", field)
		}
		if _, err := uuid.Parse(str); err != nil {
			return fmt.Errorf("field %s must be a valid UUID", field)
		}

	case "number":
		switch v := value.(type) {
		case int, int32, int64, float32, float64:
			// Valid number
		default:
			return fmt.Errorf("field %s must be a number, got %T", field, v)
		}
	}

	// Custom validation
	if rule.CustomCheck != nil {
		if err := rule.CustomCheck(value); err != nil {
			m.validator.validationFailures[field]++
			m.metrics.mu.Lock()
			m.metrics.ValidationErrors++
			m.metrics.mu.Unlock()
			return err
		}
	}

	return nil
}

// detectSQLInjection checks for common SQL injection patterns
func (m *SecurityManager) detectSQLInjection(input string) bool {
	sqlPatterns := []string{
		"' OR '",
		"'; DROP TABLE",
		"' OR 1=1",
		"UNION SELECT",
		"'; EXEC",
		"' AND SLEEP",
	}

	for _, pattern := range sqlPatterns {
		if containsIgnoreCase(input, pattern) {
			return true
		}
	}

	return false
}

// Audit Logging Methods

// auditLog logs a security event
func (m *SecurityManager) auditLog(eventType, userID, tenantID string, details map[string]interface{}) {
	if m.auditLogger == nil || !m.auditLogger.enabled {
		return
	}

	event := AuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: eventType,
		UserID:    userID,
		TenantID:  tenantID,
		Details:   details,
		Risk:      m.assessRisk(eventType, details),
	}

	m.auditLogger.mu.Lock()
	m.auditLogger.events = append(m.auditLogger.events, event)

	// Trim old events
	if len(m.auditLogger.events) > m.auditLogger.maxEvents {
		m.auditLogger.events = m.auditLogger.events[len(m.auditLogger.events)-m.auditLogger.maxEvents:]
	}
	m.auditLogger.mu.Unlock()

	// Call handlers
	for _, handler := range m.auditLogger.handlers {
		go handler(event)
	}

	m.metrics.mu.Lock()
	m.metrics.AuditEventsLogged++
	if event.Risk == "high" || event.Risk == "critical" {
		m.metrics.HighRiskEvents++
	}
	m.metrics.mu.Unlock()
}

// assessRisk assesses the risk level of an event
func (m *SecurityManager) assessRisk(eventType string, details map[string]interface{}) string {
	switch eventType {
	case "auth_failure":
		if count, ok := details["failure_count"].(int); ok && count > 5 {
			return "high"
		}
		return "medium"
	case "injection_attempt":
		return "critical"
	case "rate_limit_exceeded":
		return "medium"
	case "token_created", "token_rotated":
		return "low"
	default:
		return "low"
	}
}

// Threat Detection Methods

// DetectThreat analyzes activity for threats
func (m *SecurityManager) DetectThreat(ctx context.Context, userID, tenantID string, activity map[string]interface{}) *Threat {
	if m.threatDetector == nil || !m.threatDetector.enabled {
		return nil
	}

	m.threatDetector.mu.Lock()
	defer m.threatDetector.mu.Unlock()

	// Get or create user activity
	userActivity, exists := m.threatDetector.userActivity[userID]
	if !exists {
		userActivity = &UserActivity{
			UserID:          userID,
			UniqueEndpoints: make(map[string]int),
		}
		m.threatDetector.userActivity[userID] = userActivity
	}

	// Update activity
	userActivity.RequestCount++
	userActivity.LastActivity = time.Now()

	if endpoint, ok := activity["endpoint"].(string); ok {
		userActivity.UniqueEndpoints[endpoint]++
	}

	if isError, ok := activity["error"].(bool); ok && isError {
		userActivity.ErrorCount++
	}

	// Check threat patterns
	for _, pattern := range m.threatDetector.threatPatterns {
		if pattern.Detector(userActivity) {
			threat := &Threat{
				ID:          uuid.New().String(),
				Type:        pattern.Name,
				UserID:      userID,
				TenantID:    tenantID,
				Severity:    pattern.Severity,
				DetectedAt:  time.Now(),
				Description: pattern.Description,
				Evidence:    activity,
				Action:      pattern.Action,
			}

			m.threatDetector.activeThreats[threat.ID] = threat
			m.threatDetector.threatHistory = append(m.threatDetector.threatHistory, *threat)

			m.metrics.mu.Lock()
			m.metrics.ThreatsDetected++
			if pattern.Action == "block" {
				m.metrics.ThreatsBlocked++
				userActivity.Blocked = true
			}
			m.metrics.ActiveThreats = len(m.threatDetector.activeThreats)
			m.metrics.mu.Unlock()

			// Audit log the threat
			m.auditLog("threat_detected", userID, tenantID, map[string]interface{}{
				"threat_type": pattern.Name,
				"severity":    pattern.Severity,
				"action":      pattern.Action,
			})

			return threat
		}
	}

	return nil
}

// Setup Methods

// setupDefaultValidationRules sets up default validation rules
func (m *SecurityManager) setupDefaultValidationRules() {
	m.validator.rules["tenant_id"] = ValidationRule{
		Field:    "tenant_id",
		Type:     "uuid",
		Required: true,
	}

	m.validator.rules["tool_id"] = ValidationRule{
		Field:     "tool_id",
		Type:      "string",
		Required:  true,
		MinLength: 1,
		MaxLength: 100,
		Pattern:   "^[a-zA-Z0-9_-]+$",
	}

	m.validator.rules["api_key"] = ValidationRule{
		Field:     "api_key",
		Type:      "string",
		Required:  true,
		MinLength: 32,
		MaxLength: 256,
	}
}

// setupDefaultThreatPatterns sets up default threat detection patterns
func (m *SecurityManager) setupDefaultThreatPatterns() {
	// Brute force detection
	m.threatDetector.threatPatterns = append(m.threatDetector.threatPatterns, ThreatPattern{
		Name:        "brute_force",
		Description: "Multiple failed authentication attempts",
		Detector: func(activity *UserActivity) bool {
			return activity.ErrorCount > 10 &&
				float64(activity.ErrorCount)/float64(activity.RequestCount) > 0.8
		},
		Severity: "high",
		Action:   "block",
	})

	// Scanning detection
	m.threatDetector.threatPatterns = append(m.threatDetector.threatPatterns, ThreatPattern{
		Name:        "endpoint_scanning",
		Description: "Accessing many different endpoints rapidly",
		Detector: func(activity *UserActivity) bool {
			return len(activity.UniqueEndpoints) > 50 &&
				time.Since(activity.LastActivity) < 5*time.Minute
		},
		Severity: "medium",
		Action:   "alert",
	})

	// Anomaly detection
	m.threatDetector.threatPatterns = append(m.threatDetector.threatPatterns, ThreatPattern{
		Name:        "anomalous_activity",
		Description: "Unusual activity pattern detected",
		Detector: func(activity *UserActivity) bool {
			return activity.SuspiciousScore > m.threatDetector.anomalyThreshold
		},
		Severity: "medium",
		Action:   "log",
	})
}

// Background Workers

// tokenRotationWorker periodically rotates old tokens
func (m *SecurityManager) tokenRotationWorker() {
	ticker := time.NewTicker(m.config.TokenRotationInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.tokenManager.mu.Lock()
		now := time.Now()

		for tokenID, token := range m.tokenManager.tokens {
			if token.IsActive && now.Sub(token.CreatedAt) > m.config.TokenRotationInterval {
				// Mark for rotation
				m.tokenManager.rotationHistory = append(m.tokenManager.rotationHistory, TokenRotation{
					OldTokenID: tokenID,
					UserID:     token.UserID,
					Timestamp:  now,
					Reason:     "automatic_rotation",
				})
				token.IsActive = false
			}
		}

		m.tokenManager.lastRotation = now
		m.tokenManager.mu.Unlock()
	}
}

// keyRotationWorker periodically rotates encryption keys
func (m *SecurityManager) keyRotationWorker() {
	if m.encryptor == nil {
		return
	}

	ticker := time.NewTicker(m.config.KeyRotationInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.encryptor.mu.Lock()

		// Save current key as previous
		m.encryptor.previousKeys = append(m.encryptor.previousKeys, m.encryptor.currentKey)
		if len(m.encryptor.previousKeys) > 5 {
			m.encryptor.previousKeys = m.encryptor.previousKeys[1:]
		}

		// Generate new key
		newKey := make([]byte, 32)
		if _, err := rand.Read(newKey); err != nil {
			m.logger.Error("Failed to generate new encryption key", map[string]interface{}{
				"error": err.Error(),
			})
			m.encryptor.mu.Unlock()
			continue
		}

		m.encryptor.currentKey = newKey
		m.encryptor.keyVersion++
		m.encryptor.lastKeyRotation = time.Now()

		m.encryptor.mu.Unlock()

		m.logger.Info("Encryption key rotated", map[string]interface{}{
			"key_version": m.encryptor.keyVersion,
		})
	}
}

// threatMonitoringWorker monitors for threats
func (m *SecurityManager) threatMonitoringWorker() {
	if m.threatDetector == nil {
		return
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.threatDetector.mu.Lock()

		// Clean up old threats
		for id, threat := range m.threatDetector.activeThreats {
			if time.Since(threat.DetectedAt) > 24*time.Hour {
				threat.Resolved = true
				delete(m.threatDetector.activeThreats, id)
			}
		}

		// Update metrics
		m.metrics.mu.Lock()
		m.metrics.ActiveThreats = len(m.threatDetector.activeThreats)
		m.metrics.mu.Unlock()

		m.threatDetector.mu.Unlock()
	}
}

// Helper functions

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// HashPassword creates a secure hash of a password
func HashPassword(password string, salt []byte) string {
	hash := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)
	return hex.EncodeToString(hash)
}

// GetMetrics returns security metrics
func (m *SecurityManager) GetMetrics() map[string]interface{} {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	return map[string]interface{}{
		"authentication": map[string]interface{}{
			"attempts":  m.metrics.AuthAttempts,
			"successes": m.metrics.AuthSuccesses,
			"failures":  m.metrics.AuthFailures,
			"rotations": m.metrics.TokenRotations,
		},
		"encryption": map[string]interface{}{
			"encrypted": m.metrics.DataEncrypted,
			"decrypted": m.metrics.DataDecrypted,
			"errors":    m.metrics.EncryptionErrors,
		},
		"rate_limiting": map[string]interface{}{
			"hits":     m.metrics.RateLimitHits,
			"exceeded": m.metrics.RateLimitExceeded,
		},
		"validation": map[string]interface{}{
			"errors":            m.metrics.ValidationErrors,
			"injection_blocked": m.metrics.InjectionBlocked,
		},
		"threats": map[string]interface{}{
			"detected": m.metrics.ThreatsDetected,
			"blocked":  m.metrics.ThreatsBlocked,
			"active":   m.metrics.ActiveThreats,
		},
		"audit": map[string]interface{}{
			"events_logged":    m.metrics.AuditEventsLogged,
			"high_risk_events": m.metrics.HighRiskEvents,
		},
	}
}
