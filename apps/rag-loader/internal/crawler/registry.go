// Package crawler provides the registry and factory for data source crawlers
package crawler

import (
	"fmt"
	"sync"

	"github.com/google/uuid"

	githubcrawler "github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/crawler/github"
	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
)

// Registry manages available data source factories
type Registry struct {
	factories map[string]interfaces.SourceFactory
	mu        sync.RWMutex
}

// NewRegistry creates a new crawler registry with built-in sources
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]interfaces.SourceFactory),
	}

	// Register built-in sources
	r.Register("github", createGitHubSource)

	return r
}

// Register adds a new source factory to the registry
func (r *Registry) Register(sourceType string, factory interfaces.SourceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[sourceType] = factory
}

// CreateSource creates a new data source instance from configuration
func (r *Registry) CreateSource(tenantID uuid.UUID, sourceType string, config map[string]interface{}) (interfaces.DataSource, error) {
	r.mu.RLock()
	factory, exists := r.factories[sourceType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	// Add tenant ID to config
	config["tenant_id"] = tenantID.String()

	return factory(config)
}

// ListSources returns all registered source types
func (r *Registry) ListSources() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make([]string, 0, len(r.factories))
	for sourceType := range r.factories {
		sources = append(sources, sourceType)
	}
	return sources
}

// createGitHubSource is a factory function for GitHub sources
func createGitHubSource(config map[string]interface{}) (interfaces.DataSource, error) {
	// Parse tenant ID
	tenantIDStr, ok := config["tenant_id"].(string)
	if !ok {
		return nil, fmt.Errorf("tenant_id is required")
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}

	// Parse GitHub config
	ghConfig := githubcrawler.Config{}

	if owner, ok := config["owner"].(string); ok {
		ghConfig.Owner = owner
	}
	if repo, ok := config["repo"].(string); ok {
		ghConfig.Repo = repo
	}
	if branch, ok := config["branch"].(string); ok {
		ghConfig.Branch = branch
	}
	if token, ok := config["token"].(string); ok {
		ghConfig.Token = token
	}

	// Parse patterns
	if includePatterns, ok := config["include_patterns"].([]interface{}); ok {
		for _, pattern := range includePatterns {
			if p, ok := pattern.(string); ok {
				ghConfig.IncludePatterns = append(ghConfig.IncludePatterns, p)
			}
		}
	}
	if excludePatterns, ok := config["exclude_patterns"].([]interface{}); ok {
		for _, pattern := range excludePatterns {
			if p, ok := pattern.(string); ok {
				ghConfig.ExcludePatterns = append(ghConfig.ExcludePatterns, p)
			}
		}
	}

	// Create crawler
	crawler, err := githubcrawler.NewCrawler(tenantID, ghConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub crawler: %w", err)
	}

	// Validate configuration
	if err := crawler.Validate(); err != nil {
		return nil, fmt.Errorf("invalid GitHub configuration: %w", err)
	}

	return crawler, nil
}
