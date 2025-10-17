package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/crawler/github"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/indexer"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/models"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/processor"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/repository"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
	ragModels "github.com/developer-mesh/developer-mesh/pkg/rag/models"
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
	batchProcessor := indexer.NewBatchProcessor(batchConfig, db, embeddingClient, vectorRepo, logger)

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

	// Optional: base_url (for GitHub Enterprise)
	baseURL := ""
	if url, ok := config["base_url"].(string); ok {
		baseURL = url
		orgConfig.BaseURL = url
	}

	// Create org client
	orgClient, err := github.NewOrgClient(token, baseURL, p.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub org client: %w", err)
	}

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

	// Process documents in pipeline: chunk → embed → store
	var (
		documentsAdded    = 0
		documentsUpdated  = 0
		totalChunks       = 0
		embeddingsCreated = 0
		allChunkRequests  []indexer.ChunkEmbeddingRequest
		documentChunkMap  = make(map[string][]*ragModels.Chunk) // Track chunks per document
	)

	// Phase 1: Chunk all documents and prepare embedding requests
	p.logger.Info("Phase 1: Chunking documents", map[string]interface{}{
		"job_id":    job.ID,
		"doc_count": len(documents),
	})

	for _, doc := range documents {
		// Check if document already exists
		exists, err := p.docRepo.DocumentExists(p.ctx, doc.ContentHash)
		if err != nil {
			p.logger.Warn("Failed to check document existence", map[string]interface{}{
				"doc_url": doc.URL,
				"error":   err.Error(),
			})
			continue
		}

		if exists {
			documentsUpdated++
			p.logger.Debug("Document already exists, skipping", map[string]interface{}{
				"doc_url":      doc.URL,
				"content_hash": doc.ContentHash,
			})
			continue
		}

		// Select appropriate chunker based on file type
		chunker := processor.GetChunkerForDocument(doc)

		// Generate chunks
		chunks, err := chunker.Chunk(doc)
		if err != nil {
			p.logger.Error("Failed to chunk document", map[string]interface{}{
				"doc_url": doc.URL,
				"error":   err.Error(),
			})
			continue
		}

		if len(chunks) == 0 {
			p.logger.Debug("Document produced no chunks, skipping", map[string]interface{}{
				"doc_url": doc.URL,
			})
			continue
		}

		// Store chunks for later storage
		documentChunkMap[doc.ID.String()] = chunks
		totalChunks += len(chunks)

		// Create embedding requests for all chunks
		for _, chunk := range chunks {
			allChunkRequests = append(allChunkRequests, indexer.ChunkEmbeddingRequest{
				DocumentID: doc.ID,
				Chunk:      chunk,
				TenantID:   doc.TenantID,
			})
		}

		documentsAdded++
	}

	p.logger.Info("Chunking complete", map[string]interface{}{
		"job_id":            job.ID,
		"documents_added":   documentsAdded,
		"documents_updated": documentsUpdated,
		"total_chunks":      totalChunks,
	})

	// Phase 2: Batch process all chunks to generate embeddings
	if len(allChunkRequests) > 0 {
		p.logger.Info("Phase 2: Generating embeddings", map[string]interface{}{
			"job_id":      job.ID,
			"chunk_count": len(allChunkRequests),
		})

		results, err := p.batchProcessor.ProcessChunks(p.ctx, allChunkRequests)
		if err != nil {
			// Partial failure - log but continue with successful embeddings
			p.logger.Error("Batch processing encountered errors", map[string]interface{}{
				"job_id": job.ID,
				"error":  err.Error(),
			})
		}

		// Build embedding ID map for successful results
		embeddingMap := make(map[string]string) // chunk_id -> embedding_id
		for _, result := range results {
			if result.Error == nil {
				embeddingMap[result.ChunkID.String()] = result.EmbeddingID
				embeddingsCreated++
			} else {
				p.logger.Warn("Failed to process chunk", map[string]interface{}{
					"chunk_id": result.ChunkID,
					"error":    result.Error.Error(),
				})
			}
		}

		p.logger.Info("Embedding generation complete", map[string]interface{}{
			"job_id":             job.ID,
			"embeddings_created": embeddingsCreated,
			"embeddings_failed":  len(results) - embeddingsCreated,
		})

		// Phase 3: Store documents and chunks with embedding IDs
		p.logger.Info("Phase 3: Storing documents and chunks", map[string]interface{}{
			"job_id":      job.ID,
			"doc_count":   documentsAdded,
			"chunk_count": totalChunks,
		})

		for _, doc := range documents {
			// Skip documents that weren't chunked
			chunks, exists := documentChunkMap[doc.ID.String()]
			if !exists {
				continue
			}

			// Store document
			if err := p.docRepo.CreateDocument(p.ctx, doc); err != nil {
				p.logger.Error("Failed to store document", map[string]interface{}{
					"doc_url": doc.URL,
					"doc_id":  doc.ID,
					"error":   err.Error(),
				})
				continue
			}

			// Store chunks with embedding IDs
			for _, chunk := range chunks {
				// Set embedding ID if available
				if embeddingID, ok := embeddingMap[chunk.ID.String()]; ok {
					embeddingUUID, parseErr := uuid.Parse(embeddingID)
					if parseErr == nil {
						chunk.EmbeddingID = &embeddingUUID
					}
				}

				// Store chunk
				if err := p.docRepo.CreateChunk(p.ctx, chunk); err != nil {
					p.logger.Error("Failed to store chunk", map[string]interface{}{
						"doc_id":   doc.ID,
						"chunk_id": chunk.ID,
						"error":    err.Error(),
					})
				}
			}
		}

		p.logger.Info("Storage complete", map[string]interface{}{
			"job_id":           job.ID,
			"documents_stored": documentsAdded,
			"chunks_stored":    totalChunks,
		})
	}

	// Update job statistics
	job.DocumentsProcessed = len(documents)
	job.DocumentsAdded = documentsAdded
	job.DocumentsUpdated = documentsUpdated
	job.ChunksCreated = totalChunks

	p.logger.Info("Ingestion processing complete", map[string]interface{}{
		"job_id":              job.ID,
		"documents_processed": job.DocumentsProcessed,
		"documents_added":     job.DocumentsAdded,
		"documents_updated":   job.DocumentsUpdated,
		"chunks_created":      job.ChunksCreated,
		"embeddings_created":  embeddingsCreated,
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
