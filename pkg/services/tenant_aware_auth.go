package services

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/pkg/errors"
)

// TenantAwareAuthService extends the auth service with tenant configuration support
type TenantAwareAuthService struct {
	*auth.Service
	tenantConfigService TenantConfigService
	logger              observability.Logger
}

// NewTenantAwareAuthService creates a new tenant-aware auth service
func NewTenantAwareAuthService(authService *auth.Service, tenantConfigService TenantConfigService, logger observability.Logger) *TenantAwareAuthService {
	return &TenantAwareAuthService{
		Service:             authService,
		tenantConfigService: tenantConfigService,
		logger:              logger,
	}
}

// ValidateAPIKeyWithTenantConfig validates an API key and returns both user and tenant configuration
func (s *TenantAwareAuthService) ValidateAPIKeyWithTenantConfig(ctx context.Context, apiKey string) (*auth.User, *models.TenantConfig, error) {
	// Validate API key using base service
	user, err := s.ValidateAPIKey(ctx, apiKey)
	if err != nil {
		return nil, nil, err
	}

	// Load tenant configuration
	config, err := s.tenantConfigService.GetConfig(ctx, user.TenantID.String())
	if err != nil {
		// Log but don't fail - use defaults
		s.logger.Warn("Failed to load tenant config", map[string]interface{}{
			"tenant_id": user.TenantID.String(),
			"error":     err.Error(),
		})
		// Return default config
		config = models.DefaultTenantConfig(user.TenantID.String())
	}

	// Apply tenant-specific rate limits if available
	if user.Metadata != nil {
		if keyType, ok := user.Metadata["key_type"].(string); ok {
			rateLimit := config.GetRateLimitForKeyType(keyType)
			user.Metadata["rate_limit_per_minute"] = rateLimit.RequestsPerMinute
			user.Metadata["rate_limit_per_hour"] = rateLimit.RequestsPerHour
			user.Metadata["rate_limit_per_day"] = rateLimit.RequestsPerDay
		}
	}

	return user, config, nil
}

// CheckFeatureEnabled checks if a feature is enabled for a tenant
func (s *TenantAwareAuthService) CheckFeatureEnabled(ctx context.Context, tenantID, feature string) (bool, error) {
	return s.tenantConfigService.IsFeatureEnabled(ctx, tenantID, feature)
}

// GetServiceToken retrieves a decrypted service token for a tenant and provider
func (s *TenantAwareAuthService) GetServiceToken(ctx context.Context, tenantID, provider string) (string, error) {
	config, err := s.tenantConfigService.GetConfig(ctx, tenantID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get tenant config")
	}

	token, exists := config.GetServiceToken(provider)
	if !exists {
		return "", errors.Errorf("no service token found for provider %s", provider)
	}

	return token, nil
}

// ValidateWithEndpointRateLimit validates an API key and checks endpoint-specific rate limits
func (s *TenantAwareAuthService) ValidateWithEndpointRateLimit(ctx context.Context, apiKey, endpoint string) (*auth.User, *models.EndpointRateLimit, error) {
	// Validate API key and get tenant config
	user, config, err := s.ValidateAPIKeyWithTenantConfig(ctx, apiKey)
	if err != nil {
		return nil, nil, err
	}

	// Check for endpoint-specific rate limit
	if limit, exists := config.GetRateLimitForEndpoint(endpoint); exists {
		return user, &limit, nil
	}

	// Return user with no endpoint-specific limit
	return user, nil, nil
}

// GetAllowedOrigins returns the allowed CORS origins for a tenant
func (s *TenantAwareAuthService) GetAllowedOrigins(ctx context.Context, tenantID string) ([]string, error) {
	config, err := s.tenantConfigService.GetConfig(ctx, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant config")
	}

	return config.AllowedOrigins, nil
}
