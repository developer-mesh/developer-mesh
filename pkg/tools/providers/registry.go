package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Registry manages all standard tool providers in the system
type Registry struct {
	providers map[string]StandardToolProvider
	mu        sync.RWMutex
	logger    observability.Logger
}

// NewRegistry creates a new provider registry
func NewRegistry(logger observability.Logger) *Registry {
	return &Registry{
		providers: make(map[string]StandardToolProvider),
		logger:    logger,
	}
}

// RegisterProvider registers a new provider
func (r *Registry) RegisterProvider(provider StandardToolProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.GetProviderName()
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	r.providers[name] = provider
	r.logger.Info("Registered standard tool provider", map[string]interface{}{
		"provider": name,
		"versions": provider.GetSupportedVersions(),
		"tools":    len(provider.GetToolDefinitions()),
	})

	return nil
}

// GetProvider retrieves a provider by name
func (r *Registry) GetProvider(name string) (StandardToolProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// ListProviders returns a list of all registered provider names
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// GetAllProviders returns all registered providers
func (r *Registry) GetAllProviders() map[string]StandardToolProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid concurrent modification
	providers := make(map[string]StandardToolProvider, len(r.providers))
	for k, v := range r.providers {
		providers[k] = v
	}

	return providers
}

// UnregisterProvider removes a provider from the registry
func (r *Registry) UnregisterProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// Close the provider if it has resources to clean up
	if provider, ok := r.providers[name]; ok {
		if err := provider.Close(); err != nil {
			r.logger.Warn("Failed to close provider", map[string]interface{}{
				"provider": name,
				"error":    err.Error(),
			})
		}
	}

	delete(r.providers, name)
	r.logger.Info("Unregistered standard tool provider", map[string]interface{}{
		"provider": name,
	})

	return nil
}

// HealthCheck performs health checks on all providers
func (r *Registry) HealthCheck(ctx context.Context) map[string]ProviderHealth {
	r.mu.RLock()
	providers := make(map[string]StandardToolProvider, len(r.providers))
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	results := make(map[string]ProviderHealth)
	var wg sync.WaitGroup

	for name, provider := range providers {
		wg.Add(1)
		go func(n string, p StandardToolProvider) {
			defer wg.Done()

			start := time.Now()
			err := p.HealthCheck(ctx)
			duration := time.Since(start)

			health := ProviderHealth{
				Provider:     n,
				LastChecked:  time.Now(),
				ResponseTime: int(duration.Milliseconds()),
			}

			if err != nil {
				health.Status = "unhealthy"
				health.Error = err.Error()
			} else {
				health.Status = "healthy"
			}

			results[n] = health
		}(name, provider)
	}

	wg.Wait()
	return results
}

// GetProviderForURL attempts to find a provider that can handle the given URL
func (r *Registry) GetProviderForURL(url string) (StandardToolProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check each provider's configuration to see if it matches the URL
	for _, provider := range r.providers {
		config := provider.GetDefaultConfiguration()
		if config.BaseURL != "" && strings.Contains(url, config.BaseURL) {
			return provider, true
		}
	}

	// Check common patterns
	if strings.Contains(url, "github.com") || strings.Contains(url, "api.github.com") {
		if provider, exists := r.providers["github"]; exists {
			return provider, true
		}
	}

	if strings.Contains(url, "gitlab.com") {
		if provider, exists := r.providers["gitlab"]; exists {
			return provider, true
		}
	}

	if strings.Contains(url, "atlassian.net") {
		// Check if it's Confluence or Jira based on the URL
		if strings.Contains(url, "/wiki/") || strings.Contains(url, "confluence") {
			if provider, exists := r.providers["confluence"]; exists {
				return provider, true
			}
		} else if strings.Contains(url, "/jira/") || strings.Contains(url, "jira") {
			if provider, exists := r.providers["jira"]; exists {
				return provider, true
			}
		}
		// Default to Jira if can't determine
		if provider, exists := r.providers["jira"]; exists {
			return provider, true
		}
	}

	if strings.Contains(url, "harness.io") {
		if provider, exists := r.providers["harness"]; exists {
			return provider, true
		}
	}

	if strings.Contains(url, "nexus") || strings.Contains(url, ":8081") {
		if provider, exists := r.providers["nexus"]; exists {
			return provider, true
		}
	}

	return nil, false
}

// Close closes all providers in the registry
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for name, provider := range r.providers {
		if err := provider.Close(); err != nil {
			r.logger.Error("Failed to close provider", map[string]interface{}{
				"provider": name,
				"error":    err.Error(),
			})
			lastErr = err
		}
	}

	r.providers = make(map[string]StandardToolProvider)
	return lastErr
}
