package providers

import (
	"context"
	"time"
)

// ProviderRegistry manages all registered standard tool providers
type ProviderRegistry struct {
	providers map[string]StandardToolProvider
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]StandardToolProvider),
	}
}

// Register adds a provider to the registry
func (r *ProviderRegistry) Register(provider StandardToolProvider) error {
	name := provider.GetProviderName()
	if _, exists := r.providers[name]; exists {
		return &ProviderError{
			Provider: name,
			Code:     "PROVIDER_ALREADY_REGISTERED",
			Message:  "Provider " + name + " is already registered",
		}
	}
	r.providers[name] = provider
	return nil
}

// Get retrieves a provider by name
func (r *ProviderRegistry) Get(name string) (StandardToolProvider, bool) {
	provider, exists := r.providers[name]
	return provider, exists
}

// List returns all registered providers
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ProviderCredentials contains credentials for a provider
type ProviderCredentials struct {
	// Common credential types
	APIKey   string `json:"apiKey,omitempty"`
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`

	// OAuth credentials
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`

	// Cloud provider credentials
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	Region          string `json:"region,omitempty"`

	// Custom fields for specific providers
	Custom map[string]string `json:"custom,omitempty"`
}

// ProviderContext contains context for provider operations
type ProviderContext struct {
	TenantID       string                 `json:"tenantId"`
	OrganizationID string                 `json:"organizationId"`
	UserID         string                 `json:"userId,omitempty"`
	SessionID      string                 `json:"sessionId,omitempty"`
	Credentials    *ProviderCredentials   `json:"credentials"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Timeout        time.Duration          `json:"timeout,omitempty"`
}

// WithContext creates a new context with provider information
func WithContext(ctx context.Context, pctx *ProviderContext) context.Context {
	return context.WithValue(ctx, providerContextKey{}, pctx)
}

// FromContext retrieves provider context from context
func FromContext(ctx context.Context) (*ProviderContext, bool) {
	pctx, ok := ctx.Value(providerContextKey{}).(*ProviderContext)
	return pctx, ok
}

type providerContextKey struct{}

// ProviderMetrics tracks provider usage metrics
type ProviderMetrics struct {
	Provider           string    `json:"provider"`
	TotalRequests      int64     `json:"totalRequests"`
	SuccessfulRequests int64     `json:"successfulRequests"`
	FailedRequests     int64     `json:"failedRequests"`
	AverageLatencyMs   float64   `json:"averageLatencyMs"`
	LastUsed           time.Time `json:"lastUsed"`
	ErrorRate          float64   `json:"errorRate"`
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	Provider     string    `json:"provider"`
	Status       string    `json:"status"` // healthy, degraded, unhealthy
	LastChecked  time.Time `json:"lastChecked"`
	ResponseTime int       `json:"responseTimeMs"`
	Error        string    `json:"error,omitempty"`
	Version      string    `json:"version,omitempty"`
}

// ProviderFeatures defines feature flags for a provider
type ProviderFeatures struct {
	SupportsWebhooks   bool `json:"supportsWebhooks"`
	SupportsPagination bool `json:"supportsPagination"`
	SupportsRateLimit  bool `json:"supportsRateLimit"`
	SupportsBatchOps   bool `json:"supportsBatchOps"`
	SupportsAsync      bool `json:"supportsAsync"`
	SupportsSearch     bool `json:"supportsSearch"`
	SupportsFiltering  bool `json:"supportsFiltering"`
	SupportsSorting    bool `json:"supportsSorting"`
}
