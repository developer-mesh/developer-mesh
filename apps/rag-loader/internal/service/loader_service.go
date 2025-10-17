// Package service implements the core RAG loader service
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/robfig/cron/v3"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/config"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/crawler/github"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/indexer"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/repository"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
	"github.com/developer-mesh/developer-mesh/pkg/rag/retrieval"
	repoVector "github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// LoaderService orchestrates the RAG loading process
type LoaderService struct {
	config          *config.Config
	db              *sqlx.DB
	docRepo         *repository.DocumentRepository
	embeddingClient *embedding.ContextEmbeddingClient
	vectorRepo      repoVector.Repository
	batchProcessor  *indexer.BatchProcessor
	hybridSearch    *retrieval.HybridSearch
	scheduler       *cron.Cron
	sources         map[string]interfaces.DataSource
	logger          observability.Logger

	// Synchronization
	mu         sync.RWMutex
	activeJobs map[string]*models.IngestionJob
}

// NewLoaderService creates a new loader service instance
func NewLoaderService(
	cfg *config.Config,
	db *sqlx.DB,
	embeddingClient *embedding.ContextEmbeddingClient,
	vectorRepo repoVector.Repository,
	logger observability.Logger,
) *LoaderService {
	// Initialize batch processor with configured settings
	batchConfig := indexer.BatchProcessorConfig{
		BatchSize:      cfg.Processing.Embedding.BatchSize,
		MaxConcurrency: cfg.Scheduler.MaxConcurrentJobs,
		RetryAttempts:  3,
		RetryDelay:     time.Second,
	}
	batchProcessor := indexer.NewBatchProcessor(batchConfig, db, embeddingClient, vectorRepo, logger)

	// Initialize hybrid search
	bm25Search := retrieval.NewBM25Search(db)
	hybridConfig := retrieval.DefaultHybridSearchConfig()
	hybridSearch := retrieval.NewHybridSearch(vectorRepo, bm25Search, embeddingClient, hybridConfig)

	return &LoaderService{
		config:          cfg,
		db:              db,
		docRepo:         repository.NewDocumentRepository(db),
		embeddingClient: embeddingClient,
		vectorRepo:      vectorRepo,
		batchProcessor:  batchProcessor,
		hybridSearch:    hybridSearch,
		scheduler:       cron.New(),
		sources:         make(map[string]interfaces.DataSource),
		logger:          logger,
		activeJobs:      make(map[string]*models.IngestionJob),
	}
}

// Start initializes and starts the loader service
func (s *LoaderService) Start(ctx context.Context) error {
	s.logger.Info("Starting RAG loader service", map[string]interface{}{
		"sources_configured": len(s.config.Sources),
		"scheduler_enabled":  s.config.Scheduler.EnableAPI,
	})

	// Initialize data sources
	if err := s.initializeSources(); err != nil {
		return fmt.Errorf("failed to initialize sources: %w", err)
	}

	// Schedule ingestion jobs
	if err := s.scheduleJobs(); err != nil {
		return fmt.Errorf("failed to schedule jobs: %w", err)
	}

	// Start the scheduler
	s.scheduler.Start()

	s.logger.Info("RAG loader service started successfully", map[string]interface{}{
		"active_sources": len(s.sources),
		"scheduled_jobs": len(s.scheduler.Entries()),
	})

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	s.logger.Info("Shutting down RAG loader service", nil)
	ctx = s.scheduler.Stop()
	<-ctx.Done()

	return nil
}

// initializeSources initializes configured data sources
func (s *LoaderService) initializeSources() error {
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // TODO: Get from config

	for _, sourceCfg := range s.config.Sources {
		if !sourceCfg.Enabled {
			s.logger.Debug("Skipping disabled source", map[string]interface{}{
				"source_id": sourceCfg.ID,
			})
			continue
		}

		s.logger.Info("Initializing data source", map[string]interface{}{
			"source_id":   sourceCfg.ID,
			"source_type": sourceCfg.Type,
			"schedule":    sourceCfg.Schedule,
		})

		switch sourceCfg.Type {
		case "github":
			// Single repository source
			if err := s.initializeGitHubSource(tenantID, sourceCfg); err != nil {
				s.logger.Error("Failed to initialize GitHub source", map[string]interface{}{
					"source_id": sourceCfg.ID,
					"error":     err.Error(),
				})
				return fmt.Errorf("failed to initialize GitHub source %s: %w", sourceCfg.ID, err)
			}

		case "github_org":
			// Organization-wide source (discovers all repos)
			if err := s.initializeGitHubOrgSource(ctx, tenantID, sourceCfg); err != nil {
				s.logger.Error("Failed to initialize GitHub org source", map[string]interface{}{
					"source_id": sourceCfg.ID,
					"error":     err.Error(),
				})
				return fmt.Errorf("failed to initialize GitHub org source %s: %w", sourceCfg.ID, err)
			}

		default:
			s.logger.Warn("Unknown source type", map[string]interface{}{
				"source_id":   sourceCfg.ID,
				"source_type": sourceCfg.Type,
			})
		}
	}

	s.logger.Info("All data sources initialized", map[string]interface{}{
		"total_sources": len(s.sources),
	})

	return nil
}

// initializeGitHubSource creates a crawler for a single GitHub repository
func (s *LoaderService) initializeGitHubSource(tenantID uuid.UUID, sourceCfg config.SourceConfig) error {
	// Parse GitHub config
	githubConfig, err := github.ParseRepoConfig(sourceCfg.Config)
	if err != nil {
		return fmt.Errorf("failed to parse GitHub config: %w", err)
	}

	// Create crawler
	crawler, err := github.NewCrawler(tenantID, githubConfig)
	if err != nil {
		return fmt.Errorf("failed to create GitHub crawler: %w", err)
	}

	// Validate crawler
	if err := crawler.Validate(); err != nil {
		return fmt.Errorf("GitHub crawler validation failed: %w", err)
	}

	// Store crawler
	s.sources[sourceCfg.ID] = crawler

	s.logger.Info("GitHub source initialized", map[string]interface{}{
		"source_id": sourceCfg.ID,
		"owner":     githubConfig.Owner,
		"repo":      githubConfig.Repo,
		"branch":    githubConfig.Branch,
	})

	return nil
}

// initializeGitHubOrgSource creates crawlers for all repositories in a GitHub organization
func (s *LoaderService) initializeGitHubOrgSource(ctx context.Context, tenantID uuid.UUID, sourceCfg config.SourceConfig) error {
	// Parse org config
	orgConfig, err := github.ParseOrgConfig(sourceCfg.Config)
	if err != nil {
		return fmt.Errorf("failed to parse GitHub org config: %w", err)
	}

	// Create org client
	orgClient, err := github.NewOrgClient(orgConfig.Token, orgConfig.BaseURL, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create GitHub org client: %w", err)
	}

	// Validate org access
	if err := orgClient.ValidateOrgAccess(ctx, orgConfig.Org); err != nil {
		return fmt.Errorf("failed to validate org access: %w", err)
	}

	// Create crawlers for all repos in the org
	crawlers, err := orgClient.CreateCrawlers(ctx, tenantID, orgConfig)
	if err != nil {
		return fmt.Errorf("failed to create crawlers for org: %w", err)
	}

	if len(crawlers) == 0 {
		s.logger.Warn("No repositories found for organization", map[string]interface{}{
			"source_id": sourceCfg.ID,
			"org":       orgConfig.Org,
		})
		return nil
	}

	// Create org source that wraps all crawlers
	orgSource := github.NewOrgSource(orgConfig.Org, crawlers, s.logger)

	// Validate org source
	if err := orgSource.Validate(); err != nil {
		return fmt.Errorf("GitHub org source validation failed: %w", err)
	}

	// Store org source
	s.sources[sourceCfg.ID] = orgSource

	s.logger.Info("GitHub org source initialized", map[string]interface{}{
		"source_id":        sourceCfg.ID,
		"org":              orgConfig.Org,
		"repo_count":       len(crawlers),
		"include_archived": orgConfig.IncludeArchived,
		"include_forks":    orgConfig.IncludeForks,
	})

	return nil
}

// scheduleJobs schedules ingestion jobs for all enabled sources
func (s *LoaderService) scheduleJobs() error {
	for _, sourceCfg := range s.config.Sources {
		if !sourceCfg.Enabled {
			continue
		}

		schedule := sourceCfg.Schedule
		if schedule == "" {
			schedule = s.config.Scheduler.DefaultSchedule
		}

		// Add cron job for this source
		entryID, err := s.scheduler.AddFunc(schedule, func() {
			ctx := context.Background()
			if s.config.Scheduler.JobTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, s.config.Scheduler.JobTimeout)
				defer cancel()
			}

			if err := s.RunIngestion(ctx, sourceCfg.ID); err != nil {
				s.logger.Error("Ingestion job failed", map[string]interface{}{
					"source_id": sourceCfg.ID,
					"error":     err.Error(),
				})
			}
		})

		if err != nil {
			return fmt.Errorf("failed to schedule job for source %s: %w", sourceCfg.ID, err)
		}

		s.logger.Info("Scheduled ingestion job", map[string]interface{}{
			"source_id": sourceCfg.ID,
			"schedule":  schedule,
			"entry_id":  entryID,
		})
	}

	return nil
}

// RunIngestion manually triggers an ingestion for a specific source
func (s *LoaderService) RunIngestion(ctx context.Context, sourceID string) error {
	s.logger.Info("Starting ingestion", map[string]interface{}{
		"source_id": sourceID,
	})

	// Get source
	s.mu.RLock()
	source, exists := s.sources[sourceID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("source %s not found", sourceID)
	}

	// Check if job is already running
	s.mu.RLock()
	if activeJob, exists := s.activeJobs[sourceID]; exists {
		s.mu.RUnlock()
		return fmt.Errorf("ingestion already running for source %s (job: %s)", sourceID, activeJob.ID)
	}
	s.mu.RUnlock()

	// Create new ingestion job
	job := &models.IngestionJob{
		ID:        uuid.New(),
		TenantID:  uuid.New(), // TODO: Get from context or config
		SourceID:  sourceID,
		Status:    models.StatusRunning,
		StartedAt: &[]time.Time{time.Now()}[0],
		Metadata: map[string]interface{}{
			"trigger":     "manual",
			"source_type": source.Type(),
		},
	}

	// Store job in database
	if err := s.docRepo.CreateIngestionJob(ctx, job); err != nil {
		return fmt.Errorf("failed to create ingestion job: %w", err)
	}

	// Track active job
	s.mu.Lock()
	s.activeJobs[sourceID] = job
	s.mu.Unlock()

	// Run ingestion in background
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.activeJobs, sourceID)
			s.mu.Unlock()
		}()

		// Perform actual ingestion
		if err := s.performIngestion(ctx, job, source); err != nil {
			job.Status = models.StatusFailed
			job.ErrorMessage = err.Error()
			s.logger.Error("Ingestion failed", map[string]interface{}{
				"job_id":    job.ID,
				"source_id": sourceID,
				"error":     err.Error(),
			})
		} else {
			job.Status = models.StatusCompleted
		}

		// Update job status
		completedAt := time.Now()
		job.CompletedAt = &completedAt

		if err := s.docRepo.UpdateIngestionJob(ctx, job); err != nil {
			s.logger.Error("Failed to update ingestion job", map[string]interface{}{
				"job_id": job.ID,
				"error":  err.Error(),
			})
		}

		s.logger.Info("Ingestion completed", map[string]interface{}{
			"job_id":              job.ID,
			"source_id":           sourceID,
			"status":              job.Status,
			"documents_processed": job.DocumentsProcessed,
			"chunks_created":      job.ChunksCreated,
			"embeddings_created":  job.EmbeddingsCreated,
			"duration":            completedAt.Sub(*job.StartedAt).String(),
		})
	}()

	return nil
}

// performIngestion performs the actual ingestion process
func (s *LoaderService) performIngestion(ctx context.Context, job *models.IngestionJob, source interfaces.DataSource) error {
	s.logger.Info("Performing ingestion", map[string]interface{}{
		"job_id":      job.ID,
		"source_id":   job.SourceID,
		"source_type": source.Type(),
	})

	// Fetch documents from source
	documents, err := source.Fetch(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch documents: %w", err)
	}

	s.logger.Info("Documents fetched", map[string]interface{}{
		"job_id":    job.ID,
		"doc_count": len(documents),
	})

	if len(documents) == 0 {
		s.logger.Warn("No documents to process", map[string]interface{}{
			"job_id":    job.ID,
			"source_id": job.SourceID,
		})
		return nil
	}

	// TODO: Implement full document processing pipeline:
	// 1. Chunk documents using configured chunking strategy
	// 2. Generate embeddings using batch processor
	// 3. Store document metadata in rag.documents
	// 4. Track actual statistics
	//
	// For now, we just log the documents fetched
	s.logger.Info("Documents ready for processing", map[string]interface{}{
		"job_id":    job.ID,
		"doc_count": len(documents),
	})

	// Update job statistics
	job.DocumentsProcessed = len(documents)
	job.ChunksCreated = 0     // TODO: Implement chunking
	job.EmbeddingsCreated = 0 // TODO: Implement embedding generation

	s.logger.Info("Ingestion processing complete", map[string]interface{}{
		"job_id":              job.ID,
		"documents_processed": job.DocumentsProcessed,
		"chunks_created":      job.ChunksCreated,
	})

	return nil
}

// GetActiveJobs returns currently active ingestion jobs
func (s *LoaderService) GetActiveJobs() map[string]*models.IngestionJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent race conditions
	jobs := make(map[string]*models.IngestionJob, len(s.activeJobs))
	for k, v := range s.activeJobs {
		jobs[k] = v
	}

	return jobs
}

// Health performs a health check on the service
func (s *LoaderService) Health(ctx context.Context) error {
	// Check database connection
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Check if scheduler is running
	if s.scheduler.Location() == nil {
		return fmt.Errorf("scheduler not running")
	}

	return nil
}

// Stop gracefully stops the loader service
func (s *LoaderService) Stop() error {
	s.logger.Info("Stopping RAG loader service", nil)

	// Stop the scheduler
	ctx := s.scheduler.Stop()

	// Wait for active jobs to complete (with timeout)
	timeout := time.After(30 * time.Second)
	ticker := time.Tick(1 * time.Second)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped", nil)
			return nil
		case <-timeout:
			s.logger.Warn("Timeout waiting for scheduler to stop", nil)
			return fmt.Errorf("timeout waiting for jobs to complete")
		case <-ticker:
			s.mu.RLock()
			activeCount := len(s.activeJobs)
			s.mu.RUnlock()

			if activeCount == 0 {
				s.logger.Info("All jobs completed", nil)
				return nil
			}

			s.logger.Debug("Waiting for jobs to complete", map[string]interface{}{
				"active_jobs": activeCount,
			})
		}
	}
}

// Search performs hybrid search across RAG documents
func (s *LoaderService) Search(ctx context.Context, query string, limit int) ([]retrieval.SearchResult, error) {
	s.logger.Debug("Performing hybrid search", map[string]interface{}{
		"query": query,
		"limit": limit,
	})

	results, err := s.hybridSearch.Search(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

// SearchWithOptions performs hybrid search with custom options
func (s *LoaderService) SearchWithOptions(ctx context.Context, query string, opts retrieval.SearchOptions) ([]retrieval.SearchResult, error) {
	s.logger.Debug("Performing hybrid search with options", map[string]interface{}{
		"query":       query,
		"limit":       opts.Limit,
		"min_score":   opts.MinScore,
		"apply_mmr":   opts.ApplyMMR,
		"tenant_id":   opts.TenantID,
		"source_type": opts.SourceType,
	})

	results, err := s.hybridSearch.SearchWithOptions(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("search with options failed: %w", err)
	}

	return results, nil
}
