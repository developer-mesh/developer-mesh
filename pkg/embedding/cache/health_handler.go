package cache

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// HealthHandler provides HTTP endpoints for cache health checks
type HealthHandler struct {
	checker *HealthChecker
	logger  observability.Logger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(cache *SemanticCache, tenantCache *TenantAwareCache) *HealthHandler {
	return &HealthHandler{
		checker: NewHealthChecker(cache, tenantCache),
		logger:  observability.NewLogger("cache.health.handler"),
	}
}

// HandleHealth is the main health check endpoint
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	health := h.checker.CheckHealth(ctx)

	// Set status code based on health
	statusCode := http.StatusOK
	switch health.Status {
	case HealthStatusDegraded:
		statusCode = http.StatusOK // Still return 200 for degraded
	case HealthStatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(health); err != nil {
		h.logger.Warn("Failed to encode health response", map[string]interface{}{"error": err.Error()})
	}
}

// HandleHealthLiveness is a simple liveness check
func (h *HealthHandler) HandleHealthLiveness(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Warn("Failed to encode liveness response", map[string]interface{}{"error": err.Error()})
	}
}

// HandleHealthReadiness checks if the cache is ready to serve requests
func (h *HealthHandler) HandleHealthReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Quick check of critical components
	health := h.checker.CheckHealth(ctx)

	ready := true
	message := "ready"

	// Check critical components
	for _, check := range health.Checks {
		if check.Component == "redis" && check.Status == HealthStatusUnhealthy {
			ready = false
			message = "Redis not available"
			break
		}
		if check.Component == "encryption" && check.Status == HealthStatusUnhealthy {
			ready = false
			message = "Encryption service not available"
			break
		}
	}

	response := map[string]interface{}{
		"ready":     ready,
		"message":   message,
		"timestamp": time.Now().UTC(),
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Warn("Failed to encode readiness response", map[string]interface{}{"error": err.Error()})
	}
}

// HandleHealthStats returns cache statistics
func (h *HealthHandler) HandleHealthStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stats := h.checker.GetStats(ctx)
	stats["timestamp"] = time.Now().UTC()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Warn("Failed to encode stats response", map[string]interface{}{"error": err.Error()})
	}
}

// RegisterRoutes registers health check routes on a mux
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	if prefix == "" {
		prefix = "/health"
	}

	mux.HandleFunc(prefix, h.HandleHealth)
	mux.HandleFunc(prefix+"/live", h.HandleHealthLiveness)
	mux.HandleFunc(prefix+"/ready", h.HandleHealthReadiness)
	mux.HandleFunc(prefix+"/stats", h.HandleHealthStats)
}

// Middleware adds cache health information to response headers
func (h *HealthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add cache health header
		ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
		defer cancel()

		health := h.checker.CheckHealth(ctx)
		w.Header().Set("X-Cache-Health", string(health.Status))

		next.ServeHTTP(w, r)
	})
}
