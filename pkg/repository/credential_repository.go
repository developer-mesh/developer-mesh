package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// TenantCredential represents a stored credential in the database
type TenantCredential struct {
	ID                         uuid.UUID  `db:"id"`
	TenantID                   uuid.UUID  `db:"tenant_id"`
	ToolID                     *uuid.UUID `db:"tool_id"`
	CredentialName             string     `db:"credential_name"`
	CredentialType             string     `db:"credential_type"`
	EncryptedValue             string     `db:"encrypted_value"`
	OAuthClientID              *string    `db:"oauth_client_id"`
	OAuthClientSecretEncrypted *string    `db:"oauth_client_secret_encrypted"`
	OAuthRefreshTokenEncrypted *string    `db:"oauth_refresh_token_encrypted"`
	OAuthTokenExpiry           *time.Time `db:"oauth_token_expiry"`
	Description                *string    `db:"description"`
	Tags                       []string   `db:"tags"`
	IsActive                   bool       `db:"is_active"`
	LastUsedAt                 *time.Time `db:"last_used_at"`
	EdgeMcpID                  *string    `db:"edge_mcp_id"`
	AllowedEdgeMcps            []string   `db:"allowed_edge_mcps"`
	CreatedAt                  time.Time  `db:"created_at"`
	UpdatedAt                  time.Time  `db:"updated_at"`
	ExpiresAt                  *time.Time `db:"expires_at"`
}

// CredentialRepository handles database operations for credentials
type CredentialRepository struct {
	db *sqlx.DB
}

// NewCredentialRepository creates a new credential repository
func NewCredentialRepository(db *sqlx.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

// Create creates a new credential
func (r *CredentialRepository) Create(ctx context.Context, cred *TenantCredential) error {
	if cred.ID == uuid.Nil {
		cred.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.tenant_tool_credentials (
			id, tenant_id, tool_id, credential_name, credential_type, encrypted_value,
			oauth_client_id, oauth_client_secret_encrypted, oauth_refresh_token_encrypted,
			oauth_token_expiry, description, tags, is_active, edge_mcp_id, allowed_edge_mcps,
			expires_at, created_at, updated_at
		) VALUES (
			:id, :tenant_id, :tool_id, :credential_name, :credential_type, :encrypted_value,
			:oauth_client_id, :oauth_client_secret_encrypted, :oauth_refresh_token_encrypted,
			:oauth_token_expiry, :description, :tags, :is_active, :edge_mcp_id, :allowed_edge_mcps,
			:expires_at, :created_at, :updated_at
		)
	`

	cred.CreatedAt = time.Now()
	cred.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, cred)
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}

	return nil
}

// Get retrieves a credential by ID
func (r *CredentialRepository) Get(ctx context.Context, id uuid.UUID) (*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE id = $1
	`

	var cred TenantCredential
	err := r.db.GetContext(ctx, &cred, query, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("credential not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	return &cred, nil
}

// GetByTenantAndName retrieves a credential by tenant ID and name
func (r *CredentialRepository) GetByTenantAndName(ctx context.Context, tenantID uuid.UUID, name string) (*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE tenant_id = $1 AND credential_name = $2 AND is_active = true
	`

	var cred TenantCredential
	err := r.db.GetContext(ctx, &cred, query, tenantID, name)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("credential not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	return &cred, nil
}

// ListByTenant retrieves all credentials for a tenant
func (r *CredentialRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, includeInactive bool) ([]*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE tenant_id = $1
	`
	if !includeInactive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY created_at DESC`

	var creds []*TenantCredential
	err := r.db.SelectContext(ctx, &creds, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	return creds, nil
}

// ListByTool retrieves all credentials for a specific tool
func (r *CredentialRepository) ListByTool(ctx context.Context, tenantID uuid.UUID, toolID uuid.UUID) ([]*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE tenant_id = $1 AND tool_id = $2 AND is_active = true
		ORDER BY created_at DESC
	`

	var creds []*TenantCredential
	err := r.db.SelectContext(ctx, &creds, query, tenantID, toolID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials by tool: %w", err)
	}

	return creds, nil
}

// Update updates a credential
func (r *CredentialRepository) Update(ctx context.Context, cred *TenantCredential) error {
	query := `
		UPDATE mcp.tenant_tool_credentials
		SET encrypted_value = :encrypted_value,
		    oauth_client_id = :oauth_client_id,
		    oauth_client_secret_encrypted = :oauth_client_secret_encrypted,
		    oauth_refresh_token_encrypted = :oauth_refresh_token_encrypted,
		    oauth_token_expiry = :oauth_token_expiry,
		    description = :description,
		    tags = :tags,
		    is_active = :is_active,
		    edge_mcp_id = :edge_mcp_id,
		    allowed_edge_mcps = :allowed_edge_mcps,
		    expires_at = :expires_at,
		    updated_at = :updated_at
		WHERE id = :id
	`

	cred.UpdatedAt = time.Now()

	result, err := r.db.NamedExecContext(ctx, query, cred)
	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("credential not found")
	}

	return nil
}

// UpdateLastUsed updates the last used timestamp
func (r *CredentialRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE mcp.tenant_tool_credentials
		SET last_used_at = $1, updated_at = $1
		WHERE id = $2
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("credential not found")
	}

	return nil
}

// Deactivate deactivates a credential (soft delete)
func (r *CredentialRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE mcp.tenant_tool_credentials
		SET is_active = false, updated_at = $1
		WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to deactivate credential: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("credential not found")
	}

	return nil
}

// Delete permanently deletes a credential
func (r *CredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM mcp.tenant_tool_credentials
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("credential not found")
	}

	return nil
}

// ListExpiring retrieves credentials that will expire within the given duration
func (r *CredentialRepository) ListExpiring(ctx context.Context, within time.Duration) ([]*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE is_active = true
		  AND expires_at IS NOT NULL
		  AND expires_at <= $1
		  AND expires_at > NOW()
		ORDER BY expires_at ASC
	`

	expiryThreshold := time.Now().Add(within)
	var creds []*TenantCredential
	err := r.db.SelectContext(ctx, &creds, query, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to list expiring credentials: %w", err)
	}

	return creds, nil
}

// ListExpired retrieves credentials that have already expired
func (r *CredentialRepository) ListExpired(ctx context.Context) ([]*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE is_active = true
		  AND expires_at IS NOT NULL
		  AND expires_at <= NOW()
		ORDER BY expires_at ASC
	`

	var creds []*TenantCredential
	err := r.db.SelectContext(ctx, &creds, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list expired credentials: %w", err)
	}

	return creds, nil
}

// ListUnusedSince retrieves credentials that haven't been used since the given time
func (r *CredentialRepository) ListUnusedSince(ctx context.Context, since time.Time) ([]*TenantCredential, error) {
	query := `
		SELECT * FROM mcp.tenant_tool_credentials
		WHERE is_active = true
		  AND (last_used_at IS NULL OR last_used_at < $1)
		ORDER BY last_used_at ASC NULLS FIRST
	`

	var creds []*TenantCredential
	err := r.db.SelectContext(ctx, &creds, query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to list unused credentials: %w", err)
	}

	return creds, nil
}
