package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/models"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/repository"
	"github.com/developer-mesh/developer-mesh/pkg/rag/security"
)

// SourceHandler handles REST API endpoints for source management
type SourceHandler struct {
	repo      *repository.SourceRepository
	credMgr   *security.CredentialManager
	validator *validator.Validate
}

// NewSourceHandler creates a new source handler instance
func NewSourceHandler(
	repo *repository.SourceRepository,
	credMgr *security.CredentialManager,
) *SourceHandler {
	return &SourceHandler{
		repo:      repo,
		credMgr:   credMgr,
		validator: validator.New(),
	}
}

// Supported source types
var validSourceTypes = map[string]bool{
	"github_org":  true,
	"github_repo": true,
	"gitlab":      true,
	"confluence":  true,
	"slack":       true,
	"s3":          true,
	"jira":        true,
	"notion":      true,
}

// isValidSourceType checks if the source type is supported
func isValidSourceType(sourceType string) bool {
	return validSourceTypes[sourceType]
}

// testCredentials validates credentials by attempting to connect to the service
func (h *SourceHandler) testCredentials(ctx context.Context, sourceType string, config json.RawMessage, credentials map[string]string) error {
	switch sourceType {
	case "github_org", "github_repo":
		return h.testGitHubCredentials(ctx, config, credentials)
	case "confluence":
		return h.testConfluenceCredentials(ctx, config, credentials)
	case "slack":
		return h.testSlackCredentials(ctx, config, credentials)
	case "s3":
		return h.testS3Credentials(ctx, config, credentials)
	default:
		return fmt.Errorf("credential testing not implemented for %s", sourceType)
	}
}

// testGitHubCredentials validates GitHub API token and access
func (h *SourceHandler) testGitHubCredentials(ctx context.Context, config json.RawMessage, credentials map[string]string) error {
	token, ok := credentials["token"]
	if !ok || token == "" {
		return fmt.Errorf("GitHub token is required")
	}

	// Parse config to get org/repo details
	var githubConfig map[string]interface{}
	if err := json.Unmarshal(config, &githubConfig); err != nil {
		return fmt.Errorf("invalid GitHub configuration: %w", err)
	}

	// Test token by making a simple API call
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to GitHub: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid GitHub token")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API error: status %d", resp.StatusCode)
	}

	// If it's an org source, check org access
	if org, ok := githubConfig["org"].(string); ok && org != "" {
		orgReq, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/orgs/%s", org), nil)
		orgReq.Header.Set("Authorization", "token "+token)
		orgReq.Header.Set("Accept", "application/vnd.github.v3+json")

		orgResp, err := client.Do(orgReq)
		if err != nil {
			return fmt.Errorf("failed to access organization: %w", err)
		}
		defer func() {
			if err := orgResp.Body.Close(); err != nil {
				log.Printf("Failed to close org response body: %v", err)
			}
		}()

		if orgResp.StatusCode == 404 {
			return fmt.Errorf("organization '%s' not found or no access", org)
		}
		if orgResp.StatusCode != 200 {
			return fmt.Errorf("cannot access organization '%s': status %d", org, orgResp.StatusCode)
		}
	}

	return nil
}

// testConfluenceCredentials validates Confluence credentials
func (h *SourceHandler) testConfluenceCredentials(ctx context.Context, config json.RawMessage, credentials map[string]string) error {
	// TODO: Implement Confluence credential validation
	return fmt.Errorf("confluence credential testing not yet implemented")
}

// testSlackCredentials validates Slack credentials
func (h *SourceHandler) testSlackCredentials(ctx context.Context, config json.RawMessage, credentials map[string]string) error {
	// TODO: Implement Slack credential validation
	return fmt.Errorf("slack credential testing not yet implemented")
}

// testS3Credentials validates AWS S3 credentials
func (h *SourceHandler) testS3Credentials(ctx context.Context, config json.RawMessage, credentials map[string]string) error {
	// TODO: Implement S3 credential validation
	return fmt.Errorf("S3 credential testing not yet implemented")
}

// CreateSource handles POST /api/v1/rag/sources
func (h *SourceHandler) CreateSource(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	userID := c.MustGet("user_id").(uuid.UUID)

	var req CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate source type
	if !isValidSourceType(req.SourceType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source type"})
		return
	}

	// Test credentials before saving
	if err := h.testCredentials(c.Request.Context(), req.SourceType, req.Config, req.Credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid credentials",
			"details": err.Error(),
		})
		return
	}

	// Begin transaction
	tx, err := h.repo.BeginTx(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	// Create source
	source := &models.TenantSource{
		ID:         uuid.New(),
		TenantID:   tenantID,
		SourceID:   req.SourceID,
		SourceType: req.SourceType,
		Config:     req.Config,
		Schedule:   req.Schedule,
		Enabled:    true,
		SyncStatus: "pending",
		CreatedBy:  userID,
	}

	if err = h.repo.CreateSource(c.Request.Context(), tx, source); err != nil {
		log.Printf("Failed to create source: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create source",
			"details": err.Error(),
		})
		return
	}

	// Commit transaction BEFORE storing credentials (FK constraint requires source to exist first)
	if err = tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save"})
		return
	}
	log.Printf("Transaction committed successfully, source created: %s", source.SourceID)

	// Store encrypted credentials AFTER transaction commits
	// (foreign key constraint requires source to exist in rag.tenant_sources)
	if req.Credentials != nil {
		for credType, value := range req.Credentials {
			if err = h.credMgr.StoreCredential(
				c.Request.Context(),
				tenantID,
				req.SourceID,
				credType,
				value,
			); err != nil {
				log.Printf("Failed to store credential %s: %v", credType, err)
				// Source is already created, so delete it to maintain consistency
				if delErr := h.repo.DeleteSource(c.Request.Context(), tenantID, req.SourceID); delErr != nil {
					log.Printf("Failed to delete source after credential error: %v", delErr)
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "failed to store credentials",
					"details": err.Error(),
				})
				return
			}
		}
	}

	// TODO: Schedule sync job if schedule provided
	// This will be implemented in the scheduler component

	c.JSON(http.StatusCreated, gin.H{
		"id":        source.ID,
		"source_id": source.SourceID,
		"message":   "Source created successfully",
	})
}

// ListSources handles GET /api/v1/rag/sources
func (h *SourceHandler) ListSources(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	sources, err := h.repo.ListSourcesByTenant(c.Request.Context(), tenantID)
	if err != nil {
		log.Printf("Failed to list sources for tenant %s: %v", tenantID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"count":   len(sources),
	})
}

// GetSource handles GET /api/v1/rag/sources/:id
func (h *SourceHandler) GetSource(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	sourceID := c.Param("id")

	source, err := h.repo.GetSource(c.Request.Context(), tenantID, sourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	c.JSON(http.StatusOK, source)
}

// UpdateSource handles PUT /api/v1/rag/sources/:id
func (h *SourceHandler) UpdateSource(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	userID := c.MustGet("user_id").(uuid.UUID)
	sourceID := c.Param("id")

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing source
	source, err := h.repo.GetSource(c.Request.Context(), tenantID, sourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	// Update fields
	if req.Config != nil {
		source.Config = req.Config
	}
	if req.Schedule != "" {
		source.Schedule = req.Schedule
	}
	if req.Enabled != nil {
		source.Enabled = *req.Enabled
	}
	source.UpdatedBy = &userID

	// Test new credentials if provided
	if req.Credentials != nil {
		if err := h.testCredentials(c.Request.Context(), source.SourceType, source.Config, req.Credentials); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid credentials",
				"details": err.Error(),
			})
			return
		}

		// Store updated credentials
		for credType, value := range req.Credentials {
			if err := h.credMgr.StoreCredential(
				c.Request.Context(),
				tenantID,
				sourceID,
				credType,
				value,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update credentials"})
				return
			}
		}
	}

	// Update source
	if err := h.repo.UpdateSource(c.Request.Context(), source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update source"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Source updated successfully",
		"source":  source,
	})
}

// TriggerSync handles POST /api/v1/rag/sources/:id/sync
func (h *SourceHandler) TriggerSync(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	sourceID := c.Param("id")

	// Verify source belongs to tenant
	source, err := h.repo.GetSource(c.Request.Context(), tenantID, sourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	// Create sync job
	job := &models.TenantSyncJob{
		ID:       uuid.New(),
		TenantID: tenantID,
		SourceID: source.SourceID,
		JobType:  "manual",
		Status:   "queued",
		Priority: 5,
	}

	if err := h.repo.CreateSyncJob(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue sync"})
		return
	}

	// TODO: Trigger immediate processing via scheduler
	// This will be implemented when scheduler is integrated

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  job.ID,
		"message": "Sync job queued successfully",
	})
}

// GetSyncJobs handles GET /api/v1/rag/sources/:id/jobs
func (h *SourceHandler) GetSyncJobs(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	sourceID := c.Param("id")

	// Get limit from query params
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			log.Printf("Invalid limit parameter '%s': %v", limitStr, err)
			limit = 10 // Reset to default on error
		}
	}

	jobs, err := h.repo.ListSyncJobs(c.Request.Context(), tenantID, sourceID, limit)
	if err != nil {
		log.Printf("Failed to list sync jobs for source %s: %v", sourceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list jobs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// DeleteSource handles DELETE /api/v1/rag/sources/:id
func (h *SourceHandler) DeleteSource(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	sourceID := c.Param("id")

	// This will cascade delete all documents and credentials
	err := h.repo.DeleteSource(c.Request.Context(), tenantID, sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete source"})
		return
	}

	// TODO: Remove from scheduler when scheduler is integrated

	c.JSON(http.StatusOK, gin.H{"message": "Source deleted successfully"})
}
