// Package github implements the GitHub data source crawler for the RAG loader
package github

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// OrgSource implements DataSource interface for a GitHub organization
// It manages multiple repository crawlers internally
type OrgSource struct {
	orgName  string
	crawlers []*Crawler
	logger   observability.Logger
	mu       sync.RWMutex
}

// NewOrgSource creates a new organization source
func NewOrgSource(orgName string, crawlers []*Crawler, logger observability.Logger) *OrgSource {
	return &OrgSource{
		orgName:  orgName,
		crawlers: crawlers,
		logger:   logger,
	}
}

// ID returns a unique identifier for this org source
func (o *OrgSource) ID() string {
	return fmt.Sprintf("github_org_%s", o.orgName)
}

// Type returns the type of data source
func (o *OrgSource) Type() string {
	return "github_org"
}

// Fetch retrieves documents from all repositories in the organization
func (o *OrgSource) Fetch(ctx context.Context, since *time.Time) ([]*models.Document, error) {
	o.mu.RLock()
	crawlers := o.crawlers
	o.mu.RUnlock()

	o.logger.Info("Fetching documents from organization", map[string]interface{}{
		"org":        o.orgName,
		"repo_count": len(crawlers),
	})

	var allDocs []*models.Document
	var mu sync.Mutex
	var wg sync.WaitGroup
	errors := make([]error, 0)

	// Fetch from all repos concurrently
	for _, crawler := range crawlers {
		wg.Add(1)
		go func(c *Crawler) {
			defer wg.Done()

			docs, err := c.Fetch(ctx, since)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to fetch from %s: %w", c.ID(), err))
				mu.Unlock()
				o.logger.Error("Failed to fetch from repository", map[string]interface{}{
					"org":   o.orgName,
					"repo":  c.ID(),
					"error": err.Error(),
				})
				return
			}

			mu.Lock()
			allDocs = append(allDocs, docs...)
			mu.Unlock()

			o.logger.Debug("Fetched documents from repository", map[string]interface{}{
				"org":       o.orgName,
				"repo":      c.ID(),
				"doc_count": len(docs),
			})
		}(crawler)
	}

	wg.Wait()

	// Log summary
	o.logger.Info("Organization fetch complete", map[string]interface{}{
		"org":         o.orgName,
		"total_docs":  len(allDocs),
		"error_count": len(errors),
	})

	// Return error if all repos failed
	if len(errors) > 0 && len(allDocs) == 0 {
		return nil, fmt.Errorf("all repositories failed to fetch: %v", errors)
	}

	return allDocs, nil
}

// GetMetadata returns organization-specific metadata
func (o *OrgSource) GetMetadata() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	repoNames := make([]string, len(o.crawlers))
	for i, c := range o.crawlers {
		repoNames[i] = c.config.Repo
	}

	return map[string]interface{}{
		"org":        o.orgName,
		"repo_count": len(o.crawlers),
		"repos":      repoNames,
	}
}

// Validate checks if the organization source is valid
func (o *OrgSource) Validate() error {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.orgName == "" {
		return fmt.Errorf("organization name is required")
	}

	if len(o.crawlers) == 0 {
		return fmt.Errorf("no repositories configured for organization %s", o.orgName)
	}

	// Validate all crawlers
	for _, crawler := range o.crawlers {
		if err := crawler.Validate(); err != nil {
			return fmt.Errorf("invalid crawler for repo %s: %w", crawler.config.Repo, err)
		}
	}

	return nil
}

// HealthCheck verifies all repositories are accessible
func (o *OrgSource) HealthCheck(ctx context.Context) error {
	o.mu.RLock()
	crawlers := o.crawlers
	o.mu.RUnlock()

	// Check a sample of repositories (up to 5)
	sampleSize := len(crawlers)
	if sampleSize > 5 {
		sampleSize = 5
	}

	for i := 0; i < sampleSize; i++ {
		if err := crawlers[i].HealthCheck(ctx); err != nil {
			return fmt.Errorf("health check failed for %s: %w", crawlers[i].ID(), err)
		}
	}

	return nil
}

// AddCrawler adds a new repository crawler to the organization
func (o *OrgSource) AddCrawler(crawler *Crawler) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.crawlers = append(o.crawlers, crawler)
}

// RemoveCrawler removes a repository crawler from the organization
func (o *OrgSource) RemoveCrawler(repoName string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	filtered := make([]*Crawler, 0, len(o.crawlers))
	for _, c := range o.crawlers {
		if c.config.Repo != repoName {
			filtered = append(filtered, c)
		}
	}
	o.crawlers = filtered
}

// GetCrawlerCount returns the number of repository crawlers
func (o *OrgSource) GetCrawlerCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.crawlers)
}

// Ensure OrgSource implements interfaces.DataSource
var _ interfaces.DataSource = (*OrgSource)(nil)
