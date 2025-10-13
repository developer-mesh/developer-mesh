package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	webhookcontext "github.com/developer-mesh/developer-mesh/pkg/webhook/context"
	"github.com/google/uuid"
)

// PackageEnrichmentProcessor processes package release events to generate enriched context and embeddings
type PackageEnrichmentProcessor struct {
	releaseRepo      repository.PackageReleaseRepository
	contextRepo      repository.ContextRepository
	contextBuilder   *webhookcontext.PackageContextBuilder
	embeddingService *embedding.ServiceV2
	logger           observability.Logger
	metrics          observability.MetricsClient
}

// NewPackageEnrichmentProcessor creates a new package enrichment processor
func NewPackageEnrichmentProcessor(
	releaseRepo repository.PackageReleaseRepository,
	contextRepo repository.ContextRepository,
	embeddingService *embedding.ServiceV2,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *PackageEnrichmentProcessor {
	if logger == nil {
		logger = observability.NewLogger("package-enrichment-processor")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &PackageEnrichmentProcessor{
		releaseRepo:      releaseRepo,
		contextRepo:      contextRepo,
		contextBuilder:   webhookcontext.NewPackageContextBuilder(embeddingService, logger),
		embeddingService: embeddingService,
		logger:           logger,
		metrics:          metrics,
	}
}

// PackageEnrichmentEvent represents the event payload for package enrichment
type PackageEnrichmentEvent struct {
	ReleaseID uuid.UUID `json:"release_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	AgentID   string    `json:"agent_id,omitempty"`
}

// ProcessEvent processes a package enrichment event
func (p *PackageEnrichmentProcessor) ProcessEvent(ctx context.Context, event queue.Event) error {
	start := time.Now()
	defer func() {
		p.metrics.RecordHistogram("package_enrichment_duration_seconds", time.Since(start).Seconds(), nil)
	}()

	// Parse event payload
	var payload PackageEnrichmentEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		p.logger.Error("Failed to parse package enrichment event", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		p.metrics.IncrementCounter("package_enrichment_parse_errors_total", 1)
		return fmt.Errorf("failed to parse package enrichment event: %w", err)
	}

	p.logger.Info("Processing package enrichment", map[string]interface{}{
		"event_id":   event.EventID,
		"release_id": payload.ReleaseID,
		"tenant_id":  payload.TenantID,
	})

	// Fetch release with all details
	releaseDetails, err := p.releaseRepo.GetWithDetails(ctx, payload.ReleaseID)
	if err != nil {
		p.logger.Error("Failed to fetch release details", map[string]interface{}{
			"release_id": payload.ReleaseID,
			"error":      err.Error(),
		})
		p.metrics.IncrementCounter("package_enrichment_fetch_errors_total", 1)
		return fmt.Errorf("failed to fetch release details: %w", err)
	}

	// Parse release notes if available
	var parsedNotes *models.ParsedReleaseNotes
	if releaseDetails.Release.ReleaseNotes != nil {
		parsedNotes = p.parseReleaseNotes(*releaseDetails.Release.ReleaseNotes)
	}

	// Build enriched context
	enrichedCtx, err := p.contextBuilder.BuildReleaseContext(
		ctx,
		&releaseDetails.Release,
		releaseDetails.Assets,
		releaseDetails.APIChanges,
		releaseDetails.Dependencies,
		parsedNotes,
	)
	if err != nil {
		p.logger.Error("Failed to build enriched context", map[string]interface{}{
			"release_id": payload.ReleaseID,
			"error":      err.Error(),
		})
		p.metrics.IncrementCounter("package_enrichment_build_errors_total", 1)
		return fmt.Errorf("failed to build enriched context: %w", err)
	}

	// Use agent ID from payload or default
	agentID := payload.AgentID
	if agentID == "" {
		agentID = "package-enrichment-worker"
	}

	// Generate embedding
	if err := p.contextBuilder.GenerateEmbedding(ctx, enrichedCtx, payload.TenantID, agentID); err != nil {
		p.logger.Error("Failed to generate embedding", map[string]interface{}{
			"release_id": payload.ReleaseID,
			"error":      err.Error(),
		})
		p.metrics.IncrementCounter("package_enrichment_embedding_errors_total", 1)
		// Continue even if embedding fails - we still want to store the context
	}

	// Store enriched context
	if err := p.storeEnrichedContext(ctx, enrichedCtx, payload.TenantID); err != nil {
		p.logger.Error("Failed to store enriched context", map[string]interface{}{
			"release_id": payload.ReleaseID,
			"error":      err.Error(),
		})
		p.metrics.IncrementCounter("package_enrichment_storage_errors_total", 1)
		return fmt.Errorf("failed to store enriched context: %w", err)
	}

	// Record success metrics
	p.metrics.IncrementCounterWithLabels("package_enrichment_processed_total", 1, map[string]string{
		"tenant_id":    payload.TenantID.String(),
		"package_type": enrichedCtx.PackageType,
	})

	p.logger.Info("Successfully enriched package release", map[string]interface{}{
		"release_id":       payload.ReleaseID,
		"package":          enrichedCtx.PackageName,
		"version":          enrichedCtx.Version,
		"keywords_count":   len(enrichedCtx.Keywords),
		"categories_count": len(enrichedCtx.Categories),
		"has_embedding":    len(enrichedCtx.Embedding) > 0,
		"duration_ms":      time.Since(start).Milliseconds(),
	})

	return nil
}

// storeEnrichedContext stores the enriched context in the context repository
func (p *PackageEnrichmentProcessor) storeEnrichedContext(
	ctx context.Context,
	enrichedCtx *webhookcontext.EnrichedPackageContext,
	tenantID uuid.UUID,
) error {
	// Store context using context repository
	// The context ID will be the release ID for easy lookup
	contextID := enrichedCtx.ReleaseID.String()

	// Create context
	contextObj := &repository.Context{
		ID:       contextID,
		TenantID: tenantID.String(),
		Name:     fmt.Sprintf("%s@%s", enrichedCtx.PackageName, enrichedCtx.Version),
		Status:   "active",
		Properties: map[string]interface{}{
			"source":          "package_enrichment",
			"release_id":      enrichedCtx.ReleaseID.String(),
			"package_name":    enrichedCtx.PackageName,
			"package_version": enrichedCtx.Version,
			"package_type":    enrichedCtx.PackageType,
			"repository":      enrichedCtx.Repository,
			"enriched_at":     time.Now().UTC().Format(time.RFC3339),
			"keywords":        enrichedCtx.Keywords,
			"categories":      enrichedCtx.Categories,
		},
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	err := p.contextRepo.Create(ctx, contextObj)
	if err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}

	// Add context item with the searchable text
	contextItem := &repository.ContextItem{
		ID:        uuid.New().String(),
		ContextID: contextID,
		Content:   enrichedCtx.SearchableText,
		Type:      "package_release",
		Score:     1.0, // Full importance
		Metadata: map[string]interface{}{
			"package_name":        enrichedCtx.PackageName,
			"version":             enrichedCtx.Version,
			"package_type":        enrichedCtx.PackageType,
			"repository":          enrichedCtx.Repository,
			"has_breaking_change": len(enrichedCtx.BreakingChanges) > 0,
			"dependency_count":    len(enrichedCtx.Dependencies),
			"api_change_count":    len(enrichedCtx.APIChanges),
			"asset_count":         len(enrichedCtx.Assets),
		},
	}

	err = p.contextRepo.AddContextItem(ctx, contextID, contextItem)
	if err != nil {
		return fmt.Errorf("failed to add context item: %w", err)
	}

	// If we have an embedding ID, link it to the context
	if embeddingID, ok := enrichedCtx.Metadata["embedding_id"].(string); ok && embeddingID != "" {
		// Link the embedding to the context
		err = p.contextRepo.LinkEmbeddingToContext(ctx, contextID, embeddingID, 0, 1.0)
		if err != nil {
			p.logger.Warn("Failed to link embedding to context", map[string]interface{}{
				"context_id":   contextID,
				"embedding_id": embeddingID,
				"error":        err.Error(),
			})
			// Don't fail the whole operation if linking fails
		} else {
			p.logger.Debug("Embedding linked to package context", map[string]interface{}{
				"context_id":   contextID,
				"embedding_id": embeddingID,
				"model":        enrichedCtx.EmbeddingModel,
			})
		}
	}

	return nil
}

// parseReleaseNotes parses release notes into structured format
func (p *PackageEnrichmentProcessor) parseReleaseNotes(notes string) *models.ParsedReleaseNotes {
	// This is a simple implementation - could be enhanced with more sophisticated parsing
	parsed := &models.ParsedReleaseNotes{
		RawNotes:          notes,
		HasBreakingChange: false,
		BreakingChanges:   []string{},
		NewFeatures:       []string{},
		BugFixes:          []string{},
		Sections:          []models.ReleaseNotesSection{},
	}

	// Simple keyword-based detection
	lowerNotes := strings.ToLower(notes)
	if strings.Contains(lowerNotes, "breaking") || strings.Contains(lowerNotes, "breaking change") {
		parsed.HasBreakingChange = true
	}

	// Extract sections by headers (simplified)
	lines := strings.Split(notes, "\n")
	var currentSection *models.ReleaseNotesSection

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check if line is a header
		if strings.HasPrefix(trimmed, "#") {
			if currentSection != nil && len(currentSection.Content) > 0 {
				parsed.Sections = append(parsed.Sections, *currentSection)
			}
			currentSection = &models.ReleaseNotesSection{
				Title:   strings.TrimLeft(trimmed, "# "),
				Content: []string{},
			}
		} else if currentSection != nil {
			// Add to current section
			currentSection.Content = append(currentSection.Content, trimmed)
		}
	}

	// Add last section
	if currentSection != nil && len(currentSection.Content) > 0 {
		parsed.Sections = append(parsed.Sections, *currentSection)
	}

	return parsed
}

// ValidateEvent validates a package enrichment event
func (p *PackageEnrichmentProcessor) ValidateEvent(event queue.Event) error {
	var payload PackageEnrichmentEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("invalid event payload: %w", err)
	}

	if payload.ReleaseID == uuid.Nil {
		return fmt.Errorf("missing or invalid release_id in event payload")
	}

	if payload.TenantID == uuid.Nil {
		return fmt.Errorf("missing or invalid tenant_id in event payload")
	}

	return nil
}

// GetProcessingMode returns the processing mode for package enrichment events
func (p *PackageEnrichmentProcessor) GetProcessingMode() string {
	return "async"
}
