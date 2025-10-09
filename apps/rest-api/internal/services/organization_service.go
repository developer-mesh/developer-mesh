package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// OrganizationService handles organization registration and management
type OrganizationService struct {
	db       *sqlx.DB
	authSvc  *auth.Service
	logger   observability.Logger
	emailSvc EmailService // Interface for sending emails
}

// EmailService interface for sending emails
type EmailService interface {
	SendWelcomeEmail(ctx context.Context, email, name, orgName string) error
	SendInvitationEmail(ctx context.Context, email, inviterName, orgName, token string) error
	SendPasswordResetEmail(ctx context.Context, email, name, token string) error
	SendEmailVerificationEmail(ctx context.Context, email, name, token string) error
}

// NewOrganizationService creates a new organization service
func NewOrganizationService(db *sqlx.DB, authSvc *auth.Service, emailSvc EmailService, logger observability.Logger) *OrganizationService {
	return &OrganizationService{
		db:       db,
		authSvc:  authSvc,
		emailSvc: emailSvc,
		logger:   logger,
	}
}

// OrganizationRegistration represents a new organization registration request
type OrganizationRegistration struct {
	OrganizationName string `json:"organization_name" binding:"required,min=3,max=100"`
	OrganizationSlug string `json:"organization_slug" binding:"required,min=3,max=50"`
	AdminEmail       string `json:"admin_email" binding:"required,email"`
	AdminName        string `json:"admin_name" binding:"required,min=2,max=100"`
	AdminPassword    string `json:"admin_password" binding:"required,min=8"`
	CompanySize      string `json:"company_size,omitempty"`
	Industry         string `json:"industry,omitempty"`
	UseCase          string `json:"use_case,omitempty"`
}

// Organization represents an organization
type Organization struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	Name             string     `db:"name" json:"name"`
	Slug             string     `db:"slug" json:"slug"`
	OwnerUserID      *uuid.UUID `db:"owner_user_id" json:"owner_user_id,omitempty"`
	SubscriptionTier string     `db:"subscription_tier" json:"subscription_tier"`
	MaxUsers         int        `db:"max_users" json:"max_users"`
	MaxAgents        int        `db:"max_agents" json:"max_agents"`
	BillingEmail     string     `db:"billing_email" json:"billing_email,omitempty"`
	Settings         string     `db:"settings" json:"settings"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

// User represents a user in the system
type User struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	OrganizationID      *uuid.UUID `db:"organization_id" json:"organization_id,omitempty"`
	TenantID            uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	Email               string     `db:"email" json:"email"`
	Name                string     `db:"name" json:"name"`
	PasswordHash        string     `db:"password_hash" json:"-"`
	Role                string     `db:"role" json:"role"`
	Status              string     `db:"status" json:"status"`
	EmailVerified       bool       `db:"email_verified" json:"email_verified"`
	EmailVerifiedAt     *time.Time `db:"email_verified_at" json:"email_verified_at,omitempty"`
	LastLoginAt         *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	PasswordChangedAt   *time.Time `db:"password_changed_at" json:"password_changed_at,omitempty"`
	FailedLoginAttempts int        `db:"failed_login_attempts" json:"-"`
	LockedUntil         *time.Time `db:"locked_until" json:"-"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
}

// RegisterOrganization registers a new organization with an admin user
func (s *OrganizationService) RegisterOrganization(ctx context.Context, req *OrganizationRegistration) (*Organization, *User, string, error) {
	// Validate organization slug format (alphanumeric and hyphens only)
	if !isValidSlug(req.OrganizationSlug) {
		return nil, nil, "", errors.New("invalid organization slug format")
	}

	// Validate password strength
	if err := validatePassword(req.AdminPassword); err != nil {
		return nil, nil, "", fmt.Errorf("password validation failed: %w", err)
	}

	// Start transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			s.logger.Error("Failed to rollback transaction", map[string]interface{}{"error": err.Error()})
		}
	}()

	// Check if organization slug already exists
	var exists bool
	checkOrgQuery := `SELECT EXISTS(SELECT 1 FROM mcp.organizations WHERE slug = $1)`
	if err := tx.Get(&exists, checkOrgQuery, req.OrganizationSlug); err != nil {
		s.logger.Error("Failed to check organization existence", map[string]interface{}{
			"error": err.Error(),
			"slug":  req.OrganizationSlug,
		})
		return nil, nil, "", fmt.Errorf("failed to check organization existence: %w", err)
	}
	if exists {
		return nil, nil, "", errors.New("organization slug already exists")
	}

	// Check if email already exists
	checkEmailQuery := `SELECT EXISTS(SELECT 1 FROM mcp.users WHERE email = $1)`
	if err := tx.Get(&exists, checkEmailQuery, req.AdminEmail); err != nil {
		s.logger.Error("Failed to check email existence", map[string]interface{}{
			"error": err.Error(),
			"email": req.AdminEmail,
		})
		return nil, nil, "", fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return nil, nil, "", errors.New("email already registered")
	}

	// Create organization
	orgID := uuid.New()
	tenantID := uuid.New() // Each org gets its own tenant

	org := &Organization{
		ID:               orgID,
		Name:             req.OrganizationName,
		Slug:             req.OrganizationSlug,
		SubscriptionTier: "free",
		MaxUsers:         5,
		MaxAgents:        10,
		BillingEmail:     req.AdminEmail,
		Settings:         "{}",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	insertOrgQuery := `
		INSERT INTO mcp.organizations (
			id, name, slug, subscription_tier, max_users, max_agents, 
			billing_email, settings, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	if _, err := tx.Exec(insertOrgQuery,
		org.ID, org.Name, org.Slug, org.SubscriptionTier,
		org.MaxUsers, org.MaxAgents, org.BillingEmail,
		org.Settings, org.CreatedAt, org.UpdatedAt); err != nil {
		s.logger.Error("Failed to create organization", map[string]interface{}{
			"error":    err.Error(),
			"org_id":   org.ID,
			"org_name": org.Name,
		})
		return nil, nil, "", fmt.Errorf("failed to create organization: %w", err)
	}

	// Create organization-tenant mapping
	insertTenantQuery := `
		INSERT INTO mcp.organization_tenants (
			organization_id, tenant_id, tenant_name, tenant_type, created_at
		) VALUES ($1, $2, $3, $4, $5)`

	if _, err := tx.Exec(insertTenantQuery,
		orgID, tenantID, org.Name, "standard", time.Now()); err != nil {
		s.logger.Error("Failed to create tenant mapping", map[string]interface{}{
			"error":     err.Error(),
			"org_id":    orgID,
			"tenant_id": tenantID,
		})
		return nil, nil, "", fmt.Errorf("failed to create tenant mapping: %w", err)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	// Create admin user
	userID := uuid.New()
	now := time.Now()

	user := &User{
		ID:                userID,
		OrganizationID:    &orgID,
		TenantID:          tenantID,
		Email:             req.AdminEmail,
		Name:              req.AdminName,
		PasswordHash:      string(passwordHash),
		Role:              "owner",
		Status:            "active",
		EmailVerified:     false,
		PasswordChangedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	insertUserQuery := `
		INSERT INTO mcp.users (
			id, organization_id, tenant_id, email, name, password_hash, 
			role, status, email_verified, password_changed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	if _, err := tx.Exec(insertUserQuery,
		user.ID, user.OrganizationID, user.TenantID, user.Email, user.Name,
		user.PasswordHash, user.Role, user.Status, user.EmailVerified,
		user.PasswordChangedAt, user.CreatedAt, user.UpdatedAt); err != nil {
		s.logger.Error("Failed to create user", map[string]interface{}{
			"error":   err.Error(),
			"user_id": user.ID,
			"email":   user.Email,
			"role":    user.Role,
		})
		return nil, nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Update organization with owner
	updateOrgQuery := `UPDATE mcp.organizations SET owner_user_id = $1 WHERE id = $2`
	if _, err := tx.Exec(updateOrgQuery, userID, orgID); err != nil {
		s.logger.Error("Failed to set organization owner", map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID,
			"org_id":  orgID,
		})
		return nil, nil, "", fmt.Errorf("failed to set organization owner: %w", err)
	}
	org.OwnerUserID = &userID

	// Create email verification token
	verificationToken := generateSecureToken()
	tokenHash := hashToken(verificationToken)
	expiresAt := time.Now().Add(24 * time.Hour)

	// #nosec G101 -- This is a SQL query, not a hardcoded credential
	insertTokenQuery := `
		INSERT INTO mcp.email_verification_tokens (
			user_id, token_hash, expires_at, created_at
		) VALUES ($1, $2, $3, $4)`

	if _, err := tx.Exec(insertTokenQuery, userID, tokenHash, expiresAt, time.Now()); err != nil {
		s.logger.Error("Failed to create verification token", map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID,
		})
		return nil, nil, "", fmt.Errorf("failed to create verification token: %w", err)
	}

	// Create initial API key for the organization
	apiKey, err := s.createInitialAPIKey(ctx, tx, tenantID, userID, org.Name)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create API key: %w", err)
	}

	// Log audit event
	s.logAuditEvent(ctx, tx, &userID, &orgID, "organization_registered", map[string]interface{}{
		"organization_name": org.Name,
		"admin_email":       user.Email,
	}, true)

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, nil, "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Send welcome and verification emails (async)
	go func() {
		if s.emailSvc != nil {
			_ = s.emailSvc.SendWelcomeEmail(context.Background(), user.Email, user.Name, org.Name)
			_ = s.emailSvc.SendEmailVerificationEmail(context.Background(), user.Email, user.Name, verificationToken)
		}
	}()

	s.logger.Info("Organization registered successfully", map[string]interface{}{
		"organization_id": org.ID,
		"organization":    org.Name,
		"admin_email":     user.Email,
	})

	return org, user, apiKey, nil
}

// createInitialAPIKey creates the first API key for an organization
func (s *OrganizationService) createInitialAPIKey(ctx context.Context, tx *sqlx.Tx, tenantID, userID uuid.UUID, orgName string) (string, error) {
	// Generate API key
	apiKeyBytes := make([]byte, 32)
	if _, err := rand.Read(apiKeyBytes); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	apiKey := "devmesh_" + hex.EncodeToString(apiKeyBytes)
	// Use SHA-256 to hash the API key (must match auth/api_keys.go hashAPIKey method)
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	keyHash := hex.EncodeToString(hasher.Sum(nil))
	// key_prefix column is VARCHAR(10) in database, but we use 8 chars for consistency
	keyPrefix := apiKey[:8] // "devmesh_" is exactly 8 characters

	insertKeyQuery := `
		INSERT INTO mcp.api_keys (
			id, key_hash, key_prefix, tenant_id, user_id, name, 
			key_type, role, scopes, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11
		)`

	scopes := []string{"read", "write", "admin"}

	keyID := uuid.New()
	_, err := tx.Exec(insertKeyQuery,
		keyID, keyHash, keyPrefix, tenantID, userID,
		fmt.Sprintf("%s Initial API Key", orgName),
		"admin", "admin", pq.Array(scopes),
		true, time.Now())

	if err != nil {
		s.logger.Error("Failed to create API key", map[string]interface{}{
			"error":      err.Error(),
			"key_id":     keyID,
			"tenant_id":  tenantID,
			"user_id":    userID,
			"key_prefix": keyPrefix,
		})
		return "", fmt.Errorf("failed to insert API key: %w", err)
	}

	return apiKey, nil
}

// logAuditEvent logs an authentication/authorization event
func (s *OrganizationService) logAuditEvent(ctx context.Context, tx *sqlx.Tx, userID, orgID *uuid.UUID, eventType string, details map[string]interface{}, success bool) {
	query := `
		INSERT INTO mcp.auth_audit_log (
			user_id, organization_id, event_type, event_details, success, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)`

	// Convert details to JSON
	var detailsJSON []byte
	var err error
	if details != nil {
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			s.logger.Warn("Failed to marshal audit details to JSON", map[string]interface{}{
				"error": err.Error(),
			})
			detailsJSON = []byte("{}")
		}
	} else {
		detailsJSON = []byte("{}")
	}

	if _, err := tx.Exec(query, userID, orgID, eventType, string(detailsJSON), success, time.Now()); err != nil {
		s.logger.Warn("Failed to log audit event", map[string]interface{}{
			"error":      err.Error(),
			"event_type": eventType,
		})
	}
}

// Helper functions

func isValidSlug(slug string) bool {
	// Slug must be alphanumeric with hyphens, 3-50 characters
	matched, _ := regexp.MatchString(`^[a-z0-9][a-z0-9-]{2,49}$`, slug)
	return matched
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	// Check for at least one uppercase, one lowercase, one number
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)

	if !hasUpper || !hasLower || !hasNumber {
		return errors.New("password must contain uppercase, lowercase, and numbers")
	}

	return nil
}

func generateSecureToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	return hex.EncodeToString(hash)
}
