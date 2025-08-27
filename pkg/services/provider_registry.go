package services

import (
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// ProviderRegistry manages standard tool providers
type ProviderRegistry struct {
	providers map[string]providers.StandardToolProvider
	mu        sync.RWMutex
	logger    observability.Logger
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry(logger observability.Logger) *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]providers.StandardToolProvider),
		logger:    logger,
	}
}

// RegisterProvider registers a new provider
func (r *ProviderRegistry) RegisterProvider(name string, provider providers.StandardToolProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = provider
	r.logger.Info("Provider registered", map[string]interface{}{
		"provider": name,
	})
}

// GetProvider retrieves a provider by name
func (r *ProviderRegistry) GetProvider(name string) providers.StandardToolProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.providers[name]
}

// ListProviders returns all registered provider names
func (r *ProviderRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// RemoveProvider removes a provider from the registry
func (r *ProviderRegistry) RemoveProvider(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.providers, name)
	r.logger.Info("Provider removed", map[string]interface{}{
		"provider": name,
	})
}
