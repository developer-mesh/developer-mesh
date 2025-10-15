package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/crawler/github"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/indexer"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/models"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/repository"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/rag/security"
	repoVector "github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// JobProcessor polls the database for queued sync jobs and executes them
type JobProcessor struct {
	db              *sqlx.DB
	sourceRepo      *repository.SourceRepository
	docRepo         *repository.DocumentRepository
	credMgr         *security.CredentialManager
	embeddingClient *embedding.ContextEmbeddingClient
	vectorRepo      repoVector.Repository
	batchProcessor  *indexer.BatchProcessor
	logger          observability.Logger
	pollInterval    time.Duration
	maxConcurrent   int

	ctx    context.Context
	cancel context.CancelFunc
}

// JobProcessorConfig holds configuration for the job processor
type JobProcessorConfig struct {
	PollInterval  time.Duration
	MaxConcurrent int
	BatchSize     int
	RetryAttempts int
	RetryDelay    time.Duration
}

// NewJobProcessor creates a new job processor instance
func NewJobProcessor(
	db *sqlx.DB,
	credMgr *security.CredentialManager,
	embeddingClient *embedding.ContextEmbeddingClient,
	vectorRepo repoVector.Repository,
	logger observability.Logger,
	config JobProcessorConfig,
) *JobProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize batch processor
	batchConfig := indexer.BatchProcessorConfig{
		BatchSize:      config.BatchSize,
		MaxConcurrency: config.MaxConcurrent,
		RetryAttempts:  config.RetryAttempts,
		RetryDelay:     config.RetryDelay,
	}
	batchProcessor := indexer.NewBatchProcessor(batchConfig, embeddingClient, vectorRepo, logger)

	return &JobProcessor{
		db:              db,
		sourceRepo:      repository.NewSourceRepository(db),
		docRepo:         repository.NewDocumentRepository(db),
		credMgr:         credMgr,
		embeddingClient: embeddingClient,
		vectorRepo:      vectorRepo,
		batchProcessor:  batchProcessor,
		logger:          logger,
		pollInterval:    config.PollInterval,
		maxConcurrent:   config.MaxConcurrent,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start begins polling for queued jobs
func (p *JobProcessor) Start() error {
	p.logger.Info("Starting job processor", map[string]interface{}{
		"poll_interval":  p.pollInterval.String(),
		"max_concurrent": p.maxConcurrent,
	})

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Job processor stopped", nil)
			return nil
		case <-ticker.C:
			if err := p.processQueuedJobs(); err != nil {
				p.logger.Error("Error processing queued jobs", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// Stop gracefully stops the job processor
func (p *JobProcessor) Stop() {
	p.logger.Info("Stopping job processor", nil)
	p.cancel()
}

// processQueuedJobs fetches and processes all queued jobs
func (p *JobProcessor) processQueuedJobs() error {
	// Fetch queued jobs
	jobs, err := p.sourceRepo.GetQueuedJobs(p.ctx, p.maxConcurrent)
	if err != nil {
		return fmt.Errorf("failed to get queued jobs: %w", err)
	}

	if len(jobs) == 0 {
		return nil
	}

	p.logger.Info("Processing queued jobs", map[string]interface{}{
		"job_count": len(jobs),
	})

	// Process each job
	for _, job := range jobs {
		if err := p.processJob(job); err != nil {
			p.logger.Error("Failed to process job", map[string]interface{}{
				"job_id":    job.ID,
				"source_id": job.SourceID,
				"error":     err.Error(),
			})
		}
	}

	return nil
}

// processJob processes a single sync job
func (p *JobProcessor) processJob(job *models.TenantSyncJob) error {
	p.logger.Info("Starting job processing", map[string]interface{}{
		"job_id":    job.ID,
		"tenant_id": job.TenantID,
		"source_id": job.SourceID,
	})

	// Update job status to running
	startTime := time.Now()
	job.Status = "running"
	job.StartedAt = &startTime
	if err := p.sourceRepo.UpdateSyncJob(p.ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Get source configuration
	source, err := p.sourceRepo.GetSource(p.ctx, job.TenantID, job.SourceID)
	if err != nil {
		p.markJobFailed(job, fmt.Errorf("failed to get source: %w", err))
		return err
	}

	// Get credentials
	credentials, err := p.sourceRepo.GetSourceCredentials(p.ctx, job.TenantID, job.SourceID)
	if err != nil {
		p.markJobFailed(job, fmt.Errorf("failed to get credentials: %w", err))
		return err
	}

	// Decrypt credentials
	credMap := make(map[string]string)
	for _, cred := range credentials {
		decrypted, err := p.credMgr.GetCredential(
			p.ctx,
			job.TenantID,
			job.SourceID,
			cred.CredentialType,
		)
		if err != nil {
			p.markJobFailed(job, fmt.Errorf("failed to decrypt credential %s: %w", cred.CredentialType, err))
			return err
		}
		credMap[cred.CredentialType] = decrypted
	}

	// Create data source based on type
	dataSource, err := p.createDataSource(source, credMap)
	if err != nil {
		p.markJobFailed(job, fmt.Errorf("failed to create data source: %w", err))
		return err
	}

	// Validate data source
	if err := dataSource.Validate(); err != nil {
		p.markJobFailed(job, fmt.Errorf("data source validation failed: %w", err))
		return err
	}

	// Execute ingestion
	if err := p.executeIngestion(job, source, dataSource); err != nil {
		p.markJobFailed(job, fmt.Errorf("ingestion failed: %w", err))
		return err
	}

	// Mark job as completed
	completedAt := time.Now()
	job.Status = "completed"
	job.CompletedAt = &completedAt
	durationMs := int(completedAt.Sub(startTime).Milliseconds())
	job.DurationMs = &durationMs

	if err := p.sourceRepo.UpdateSyncJob(p.ctx, job); err != nil {
		p.logger.Error("Failed to update completed job", map[string]interface{}{
			"job_id": job.ID,
			"error":  err.Error(),
		})
		return err
	}

	// Update source last sync time
	now := time.Now()
	source.LastSyncAt = &now
	source.SyncStatus = "success"
	source.SyncErrorCount = 0
	if err := p.sourceRepo.UpdateSource(p.ctx, source); err != nil {
		p.logger.Error("Failed to update source sync status", map[string]interface{}{
			"source_id": source.SourceID,
			"error":     err.Error(),
		})
	}

	p.logger.Info("Job completed successfully", map[string]interface{}{
		"job_id":              job.ID,
		"source_id":           job.SourceID,
		"documents_processed": job.DocumentsProcessed,
		"duration_ms":         durationMs,
	})

	return nil
}

// createDataSource creates a data source based on the source type and config
func (p *JobProcessor) createDataSource(source *models.TenantSource, credentials map[string]string) (interfaces.DataSource, error) {
	switch source.SourceType {
	case "github_org":
		return p.createGitHubOrgSource(source, credentials)
	case "github_repo":
		return p.createGitHubRepoSource(source, credentials)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.SourceType)
	}
}

// createGitHubOrgSource creates a GitHub organization data source
func (p *JobProcessor) createGitHubOrgSource(source *models.TenantSource, credentials map[string]string) (interfaces.DataSource, error) {
	// Parse config
	var config map[string]interface{}
	if err := json.Unmarshal(source.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Extract org and other settings
	org, ok := config["org"].(string)
	if !ok || org == "" {
		return nil, fmt.Errorf("org not specified in config")
	}

	token, ok := credentials["token"]
	if !ok {
		return nil, fmt.Errorf("GitHub token not found in credentials")
	}

	// Build GitHub org config
	orgConfig := github.OrgConfig{
		Org:   org,
		Token: token,
	}

	// Optional settings
	if includeArchived, ok := config["include_archived"].(bool); ok {
		orgConfig.IncludeArchived = includeArchived
	}
	if includeForks, ok := config["include_forks"].(bool); ok {
		orgConfig.IncludeForks = includeForks
	}
	if patterns, ok := config["include_patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if pattern, ok := p.(string); ok {
				orgConfig.IncludePatterns = append(orgConfig.IncludePatterns, pattern)
			}
		}
	}
	if patterns, ok := config["exclude_patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if pattern, ok := p.(string); ok {
				orgConfig.ExcludePatterns = append(orgConfig.ExcludePatterns, pattern)
			}
		}
	}

	// Create org client
	orgClient := github.NewOrgClient(token, p.logger)

	// Create crawlers for all repos
	crawlers, err := orgClient.CreateCrawlers(p.ctx, source.TenantID, orgConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create crawlers: %w", err)
	}

	if len(crawlers) == 0 {
		p.logger.Warn("No repositories found for organization", map[string]interface{}{
			"org":       org,
			"source_id": source.SourceID,
		})
	}

	// Create org source
	orgSource := github.NewOrgSource(org, crawlers, p.logger)

	return orgSource, nil
}

// createGitHubRepoSource creates a GitHub repository data source
func (p *JobProcessor) createGitHubRepoSource(source *models.TenantSource, credentials map[string]string) (interfaces.DataSource, error) {
	// Parse config
	var repoConfig github.Config
	if err := json.Unmarshal(source.Config, &repoConfig); err != nil {
		return nil, fmt.Errorf("failed to parse repo config: %w", err)
	}

	// Set token from credentials
	token, ok := credentials["token"]
	if !ok {
		return nil, fmt.Errorf("GitHub token not found in credentials")
	}
	repoConfig.Token = token

	// Create crawler
	crawler, err := github.NewCrawler(source.TenantID, repoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create crawler: %w", err)
	}

	return crawler, nil
}

// executeIngestion performs the actual ingestion
func (p *JobProcessor) executeIngestion(job *models.TenantSyncJob, source *models.TenantSource, dataSource interfaces.DataSource) error {
	p.logger.Info("Executing ingestion", map[string]interface{}{
		"job_id":      job.ID,
		"source_id":   job.SourceID,
		"source_type": source.SourceType,
	})

	// Fetch documents from source
	documents, err := dataSource.Fetch(p.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch documents: %w", err)
	}

	p.logger.Info("Documents fetched from source", map[string]interface{}{
		"job_id":    job.ID,
		"doc_count": len(documents),
	})

	if len(documents) == 0 {
		p.logger.Warn("No documents to process", map[string]interface{}{
			"job_id":    job.ID,
			"source_id": job.SourceID,
		})
		job.DocumentsProcessed = 0
		return nil
	}

	// TODO: Implement full document processing pipeline:
	// 1. Chunk documents using configured chunking strategy
	// 2. Generate embeddings using batch processor
	// 3. Store document metadata in rag.tenant_documents
	// 4. Track actual statistics (chunks_created, documents_added/updated)
	//
	// For now, we just log the documents fetched and mark as processed

	job.DocumentsProcessed = len(documents)
	job.DocumentsAdded = len(documents)
	job.ChunksCreated = 0 // TODO: Implement chunking

	p.logger.Info("Ingestion processing complete", map[string]interface{}{
		"job_id":              job.ID,
		"documents_processed": job.DocumentsProcessed,
		"documents_added":     job.DocumentsAdded,
	})

	return nil
}

// markJobFailed marks a job as failed with an error message
func (p *JobProcessor) markJobFailed(job *models.TenantSyncJob, err error) {
	completedAt := time.Now()
	job.Status = "failed"
	job.CompletedAt = &completedAt
	job.ErrorsCount++
	errMsg := err.Error()
	job.ErrorMessage = &errMsg

	if job.StartedAt != nil {
		durationMs := int(completedAt.Sub(*job.StartedAt).Milliseconds())
		job.DurationMs = &durationMs
	}

	if updateErr := p.sourceRepo.UpdateSyncJob(p.ctx, job); updateErr != nil {
		p.logger.Error("Failed to update failed job", map[string]interface{}{
			"job_id": job.ID,
			"error":  updateErr.Error(),
		})
	}

	// Update source error count
	source, getErr := p.sourceRepo.GetSource(p.ctx, job.TenantID, job.SourceID)
	if getErr == nil {
		source.SyncStatus = "error"
		source.SyncErrorCount++
		if updateErr := p.sourceRepo.UpdateSource(p.ctx, source); updateErr != nil {
			p.logger.Error("Failed to update source error count", map[string]interface{}{
				"source_id": source.SourceID,
				"error":     updateErr.Error(),
			})
		}
	}

	p.logger.Error("Job marked as failed", map[string]interface{}{
		"job_id":    job.ID,
		"source_id": job.SourceID,
		"error":     err.Error(),
	})
}
