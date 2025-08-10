package intelligence

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DataClassification levels
type DataClassification int

const (
	ClassificationPublic DataClassification = iota
	ClassificationInternal
	ClassificationConfidential
	ClassificationRestricted
)

// SecurityLayer provides security checks and data protection
type SecurityLayer struct {
	piiDetector    *PIIDetector
	secretScanner  *SecretScanner
	dataClassifier *DataClassifier
	encryptor      *Encryptor
	auditor        AuditLogger
	config         SecurityConfig
}

// SecurityConfig contains security configuration
type SecurityConfig struct {
	EnablePIIDetection   bool
	EnableSecretScanning bool
	EnableEncryption     bool
	EncryptionKey        []byte
	RedactPII            bool
	BlockOnSecrets       bool
	AuditEnabled         bool
	SensitivePatterns    []string
}

// NewSecurityLayer creates a new security layer
func NewSecurityLayer(config SecurityConfig) *SecurityLayer {
	return &SecurityLayer{
		piiDetector:    NewPIIDetector(),
		secretScanner:  NewSecretScanner(),
		dataClassifier: NewDataClassifier(),
		encryptor:      NewEncryptor(config.EncryptionKey),
		config:         config,
	}
}

// ValidateContent performs security validation on content
func (s *SecurityLayer) ValidateContent(ctx context.Context, content []byte) (*SecurityValidation, error) {
	validation := &SecurityValidation{
		ID:        uuid.New(),
		Timestamp: timeNow(),
		Passed:    true,
	}

	// Check for PII
	if s.config.EnablePIIDetection {
		piiResults := s.piiDetector.Detect(content)
		validation.PIIDetected = len(piiResults) > 0
		validation.PIITypes = piiResults

		if validation.PIIDetected && s.config.RedactPII {
			content = s.piiDetector.Redact(content, piiResults)
			validation.ContentRedacted = true
		}
	}

	// Check for secrets
	if s.config.EnableSecretScanning {
		secrets := s.secretScanner.Scan(content)
		validation.SecretsDetected = len(secrets) > 0
		validation.SecretTypes = secrets

		if validation.SecretsDetected && s.config.BlockOnSecrets {
			validation.Passed = false
			validation.BlockReason = "Secrets detected in content"
			return validation, fmt.Errorf("content contains secrets")
		}
	}

	// Classify data
	classification := s.dataClassifier.Classify(content)
	validation.Classification = classification

	// Block restricted content
	if classification >= ClassificationRestricted {
		validation.Passed = false
		validation.BlockReason = "Content classified as restricted"
		return validation, fmt.Errorf("content is restricted")
	}

	// Audit the validation
	if s.config.AuditEnabled && s.auditor != nil {
		s.auditor.LogSecurityEvent(ctx, SecurityAuditEvent{
			EventID:         validation.ID,
			EventType:       "content_validation",
			Classification:  classification,
			PIIDetected:     validation.PIIDetected,
			SecretsDetected: validation.SecretsDetected,
			Passed:          validation.Passed,
		})
	}

	validation.ProcessedContent = content
	return validation, nil
}

// PIIDetector detects personally identifiable information
type PIIDetector struct {
	patterns map[string]*regexp.Regexp
}

// NewPIIDetector creates a new PII detector
func NewPIIDetector() *PIIDetector {
	return &PIIDetector{
		patterns: map[string]*regexp.Regexp{
			"ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			"credit_card": regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
			"email":       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			"phone":       regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
			"ip_address":  regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
			"dob":         regexp.MustCompile(`\b(0[1-9]|1[0-2])/(0[1-9]|[12]\d|3[01])/\d{4}\b`),
			"passport":    regexp.MustCompile(`\b[A-Z][0-9]{8}\b`),
			"driver_lic":  regexp.MustCompile(`\b[A-Z]\d{7,12}\b`),
		},
	}
}

// Detect finds PII in content
func (d *PIIDetector) Detect(content []byte) []string {
	var detected []string
	contentStr := string(content)

	for piiType, pattern := range d.patterns {
		if pattern.MatchString(contentStr) {
			detected = append(detected, piiType)
		}
	}

	return detected
}

// Redact removes PII from content
func (d *PIIDetector) Redact(content []byte, piiTypes []string) []byte {
	contentStr := string(content)

	for _, piiType := range piiTypes {
		if pattern, ok := d.patterns[piiType]; ok {
			contentStr = pattern.ReplaceAllString(contentStr, "[REDACTED]")
		}
	}

	return []byte(contentStr)
}

// SecretScanner scans for secrets and credentials
type SecretScanner struct {
	patterns map[string]*regexp.Regexp
}

// NewSecretScanner creates a new secret scanner
func NewSecretScanner() *SecretScanner {
	return &SecretScanner{
		patterns: map[string]*regexp.Regexp{
			"aws_access_key": regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			"aws_secret_key": regexp.MustCompile(`[0-9a-zA-Z/+=]{40}`),
			"github_token":   regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),
			"api_key":        regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*['"]?([a-zA-Z0-9_-]{20,})['"]?`),
			"private_key":    regexp.MustCompile(`-----BEGIN (RSA |EC )?PRIVATE KEY-----`),
			"jwt_token":      regexp.MustCompile(`eyJ[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*`),
			"basic_auth":     regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/]+=*`),
			"bearer_token":   regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_-]+`),
			"slack_token":    regexp.MustCompile(`xox[baprs]-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,34}`),
			"google_api":     regexp.MustCompile(`AIza[0-9A-Za-z-_]{35}`),
		},
	}
}

// Scan finds secrets in content
func (s *SecretScanner) Scan(content []byte) []string {
	var detected []string
	contentStr := string(content)

	// Convert to lowercase for case-insensitive matching
	lowerContent := strings.ToLower(contentStr)

	// Check for common secret indicators
	secretIndicators := []string{
		"password", "passwd", "pwd",
		"secret", "token", "key",
		"credential", "auth",
	}

	for _, indicator := range secretIndicators {
		if strings.Contains(lowerContent, indicator) {
			// Check patterns more carefully
			for secretType, pattern := range s.patterns {
				if pattern.MatchString(contentStr) {
					detected = append(detected, secretType)
				}
			}
		}
	}

	return detected
}

// DataClassifier classifies data sensitivity
type DataClassifier struct {
	rules []ClassificationRule
}

// ClassificationRule defines a classification rule
type ClassificationRule struct {
	Name           string
	Pattern        *regexp.Regexp
	Keywords       []string
	Classification DataClassification
}

// NewDataClassifier creates a new data classifier
func NewDataClassifier() *DataClassifier {
	return &DataClassifier{
		rules: []ClassificationRule{
			{
				Name:           "financial",
				Keywords:       []string{"bank", "account", "routing", "swift", "iban"},
				Classification: ClassificationRestricted,
			},
			{
				Name:           "health",
				Keywords:       []string{"medical", "health", "diagnosis", "prescription", "patient"},
				Classification: ClassificationRestricted,
			},
			{
				Name:           "legal",
				Keywords:       []string{"confidential", "privileged", "attorney", "legal"},
				Classification: ClassificationConfidential,
			},
			{
				Name:           "personal",
				Keywords:       []string{"personal", "private", "individual"},
				Classification: ClassificationConfidential,
			},
			{
				Name:           "business",
				Keywords:       []string{"proprietary", "trade secret", "internal only"},
				Classification: ClassificationInternal,
			},
		},
	}
}

// Classify determines data classification
func (c *DataClassifier) Classify(content []byte) DataClassification {
	contentStr := strings.ToLower(string(content))
	highestClass := ClassificationPublic

	for _, rule := range c.rules {
		for _, keyword := range rule.Keywords {
			if strings.Contains(contentStr, keyword) {
				if rule.Classification > highestClass {
					highestClass = rule.Classification
				}
			}
		}

		if rule.Pattern != nil && rule.Pattern.MatchString(contentStr) {
			if rule.Classification > highestClass {
				highestClass = rule.Classification
			}
		}
	}

	return highestClass
}

// Encryptor provides encryption services
type Encryptor struct {
	key    []byte
	cipher cipher.AEAD
}

// NewEncryptor creates a new encryptor
func NewEncryptor(key []byte) *Encryptor {
	// If no key provided, generate one
	if len(key) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic(fmt.Sprintf("failed to generate encryption key: %v", err))
		}
	}

	// Ensure key is 32 bytes
	if len(key) != 32 {
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("failed to create cipher: %v", err))
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(fmt.Sprintf("failed to create GCM: %v", err))
	}

	return &Encryptor{
		key:    key,
		cipher: aead,
	}
}

// Encrypt encrypts data
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.cipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := e.cipher.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts data
func (e *Encryptor) Decrypt(ciphertext string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonceSize := e.cipher.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.cipher.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// SecurityValidation contains security validation results
type SecurityValidation struct {
	ID               uuid.UUID
	Timestamp        time.Time
	Passed           bool
	PIIDetected      bool
	PIITypes         []string
	SecretsDetected  bool
	SecretTypes      []string
	Classification   DataClassification
	ContentRedacted  bool
	BlockReason      string
	ProcessedContent []byte
}

// SecurityAuditEvent represents a security audit event
type SecurityAuditEvent struct {
	EventID         uuid.UUID
	EventType       string
	Timestamp       time.Time
	TenantID        uuid.UUID
	AgentID         uuid.UUID
	Classification  DataClassification
	PIIDetected     bool
	SecretsDetected bool
	Passed          bool
	Details         map[string]interface{}
}
