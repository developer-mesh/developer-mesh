package resilience

import (
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CircuitBreakerServiceConfig defines circuit breaker configuration for a specific service
type CircuitBreakerServiceConfig struct {
	Enabled             bool          `mapstructure:"enabled" json:"enabled"`
	MaxRequests         uint32        `mapstructure:"max_requests" json:"max_requests"`
	Interval            time.Duration `mapstructure:"interval" json:"interval"`
	Timeout             time.Duration `mapstructure:"timeout" json:"timeout"`
	FailureThreshold    float64       `mapstructure:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold    uint32        `mapstructure:"success_threshold" json:"success_threshold"`
	MinimumRequestCount uint32        `mapstructure:"minimum_request_count" json:"minimum_request_count"`
	MaxRequestsHalfOpen uint32        `mapstructure:"max_requests_half_open" json:"max_requests_half_open"`
}

// DefaultCircuitBreakerConfigs provides default configurations for all services
var DefaultCircuitBreakerConfigs = map[string]CircuitBreakerServiceConfig{
	"task_service": {
		Enabled:             true,
		MaxRequests:         100,
		Interval:            10 * time.Second,
		Timeout:             30 * time.Second,
		FailureThreshold:    0.5,
		SuccessThreshold:    5,
		MinimumRequestCount: 10,
		MaxRequestsHalfOpen: 10,
	},
	"workflow_service": {
		Enabled:             true,
		MaxRequests:         50,
		Interval:            10 * time.Second,
		Timeout:             45 * time.Second,
		FailureThreshold:    0.6,
		SuccessThreshold:    3,
		MinimumRequestCount: 5,
		MaxRequestsHalfOpen: 5,
	},
	"agent_service": {
		Enabled:             true,
		MaxRequests:         200,
		Interval:            5 * time.Second,
		Timeout:             15 * time.Second,
		FailureThreshold:    0.5,
		SuccessThreshold:    5,
		MinimumRequestCount: 20,
		MaxRequestsHalfOpen: 20,
	},
	"context_service": {
		Enabled:             true,
		MaxRequests:         150,
		Interval:            10 * time.Second,
		Timeout:             20 * time.Second,
		FailureThreshold:    0.4,
		SuccessThreshold:    5,
		MinimumRequestCount: 15,
		MaxRequestsHalfOpen: 15,
	},
	"document_service": {
		Enabled:             true,
		MaxRequests:         100,
		Interval:            10 * time.Second,
		Timeout:             25 * time.Second,
		FailureThreshold:    0.5,
		SuccessThreshold:    5,
		MinimumRequestCount: 10,
		MaxRequestsHalfOpen: 10,
	},
	"workspace_service": {
		Enabled:             true,
		MaxRequests:         100,
		Interval:            10 * time.Second,
		Timeout:             20 * time.Second,
		FailureThreshold:    0.5,
		SuccessThreshold:    5,
		MinimumRequestCount: 10,
		MaxRequestsHalfOpen: 10,
	},
	"embedding_service": {
		Enabled:             true,
		MaxRequests:         30,
		Interval:            30 * time.Second,
		Timeout:             60 * time.Second,
		FailureThreshold:    0.3,
		SuccessThreshold:    2,
		MinimumRequestCount: 3,
		MaxRequestsHalfOpen: 3,
	},
	// Dynamic tools will register their own circuit breaker configs
	// via the dynamic tools API
	"aws_s3": {
		Enabled:             true,
		MaxRequests:         200,
		Interval:            10 * time.Second,
		Timeout:             30 * time.Second,
		FailureThreshold:    0.3,
		SuccessThreshold:    3,
		MinimumRequestCount: 10,
		MaxRequestsHalfOpen: 20,
	},
	"aws_sqs": {
		Enabled:             true,
		MaxRequests:         500,
		Interval:            5 * time.Second,
		Timeout:             10 * time.Second,
		FailureThreshold:    0.3,
		SuccessThreshold:    5,
		MinimumRequestCount: 20,
		MaxRequestsHalfOpen: 50,
	},
	"redis_cache": {
		Enabled:             true,
		MaxRequests:         1000,
		Interval:            5 * time.Second,
		Timeout:             5 * time.Second,
		FailureThreshold:    0.2,
		SuccessThreshold:    10,
		MinimumRequestCount: 50,
		MaxRequestsHalfOpen: 100,
	},
	"postgres_db": {
		Enabled:             true,
		MaxRequests:         200,
		Interval:            10 * time.Second,
		Timeout:             10 * time.Second,
		FailureThreshold:    0.1,
		SuccessThreshold:    5,
		MinimumRequestCount: 20,
		MaxRequestsHalfOpen: 20,
	},
}

// GetCircuitBreakerConfig returns the configuration for a specific service
func GetCircuitBreakerConfig(serviceName string) (CircuitBreakerServiceConfig, bool) {
	config, exists := DefaultCircuitBreakerConfigs[serviceName]
	return config, exists
}

// SetCircuitBreakerConfig sets or updates the configuration for a specific service
func SetCircuitBreakerConfig(serviceName string, config CircuitBreakerServiceConfig) {
	DefaultCircuitBreakerConfigs[serviceName] = config
}

// ToCircuitBreakerConfig converts service config to CircuitBreakerConfig
func (c CircuitBreakerServiceConfig) ToCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    int(c.MinimumRequestCount), // Use minimum count as failure threshold
		FailureRatio:        c.FailureThreshold,
		SuccessThreshold:    int(c.SuccessThreshold),
		ResetTimeout:        c.Timeout,
		TimeoutThreshold:    c.Timeout,
		MinimumRequestCount: int(c.MinimumRequestCount),
		MaxRequestsHalfOpen: int(c.MaxRequestsHalfOpen),
	}
}

// CircuitBreakerRegistry manages circuit breakers for all services
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	configs  map[string]CircuitBreakerServiceConfig
	logger   observability.Logger
	metrics  observability.MetricsClient
}

// NewCircuitBreakerRegistry creates a new circuit breaker registry
func NewCircuitBreakerRegistry(logger observability.Logger, metrics observability.MetricsClient) *CircuitBreakerRegistry {
	registry := &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		configs:  make(map[string]CircuitBreakerServiceConfig),
		logger:   logger,
		metrics:  metrics,
	}

	// Initialize with default configurations
	for service, config := range DefaultCircuitBreakerConfigs {
		registry.configs[service] = config
	}

	return registry
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (r *CircuitBreakerRegistry) GetOrCreate(serviceName string) *CircuitBreaker {
	if breaker, exists := r.breakers[serviceName]; exists {
		return breaker
	}

	config, exists := r.configs[serviceName]
	if !exists {
		// Use a default configuration if service not configured
		config = CircuitBreakerServiceConfig{
			Enabled:             true,
			MaxRequests:         100,
			Interval:            10 * time.Second,
			Timeout:             30 * time.Second,
			FailureThreshold:    0.5,
			SuccessThreshold:    5,
			MinimumRequestCount: 10,
			MaxRequestsHalfOpen: 10,
		}
	}

	breaker := NewCircuitBreaker(serviceName, config.ToCircuitBreakerConfig(), r.logger, r.metrics)
	r.breakers[serviceName] = breaker

	return breaker
}

// UpdateConfig updates the configuration for a service
func (r *CircuitBreakerRegistry) UpdateConfig(serviceName string, config CircuitBreakerServiceConfig) {
	r.configs[serviceName] = config

	// If a breaker exists, recreate it with new config
	if _, exists := r.breakers[serviceName]; exists {
		r.breakers[serviceName] = NewCircuitBreaker(serviceName, config.ToCircuitBreakerConfig(), r.logger, r.metrics)
	}
}

// GetAllBreakers returns all registered circuit breakers
func (r *CircuitBreakerRegistry) GetAllBreakers() map[string]*CircuitBreaker {
	result := make(map[string]*CircuitBreaker)
	for k, v := range r.breakers {
		result[k] = v
	}
	return result
}

// GetHealthStatus returns the health status of all circuit breakers
func (r *CircuitBreakerRegistry) GetHealthStatus() map[string]string {
	status := make(map[string]string)
	for name := range r.breakers {
		// For now, just indicate if breaker exists
		status[name] = "registered"
	}
	return status
}

// GlobalCircuitBreakerRegistry needs to be initialized with logger and metrics
var GlobalCircuitBreakerRegistry *CircuitBreakerRegistry

// InitializeGlobalCircuitBreakerRegistry initializes the global registry
func InitializeGlobalCircuitBreakerRegistry(logger observability.Logger, metrics observability.MetricsClient) {
	GlobalCircuitBreakerRegistry = NewCircuitBreakerRegistry(logger, metrics)
}

// GetGlobalCircuitBreaker gets a circuit breaker from the global registry
func GetGlobalCircuitBreaker(serviceName string) *CircuitBreaker {
	if GlobalCircuitBreakerRegistry == nil {
		return nil
	}
	return GlobalCircuitBreakerRegistry.GetOrCreate(serviceName)
}
