package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// WebhookProvider represents a webhook provider's registration logic
// Extend this struct for additional providers (e.g., GitLab, Bitbucket)
type WebhookProvider struct {
	Name       string
	Enabled    func() bool
	Endpoint   func() string
	Handler    func() http.HandlerFunc
	Middleware func() mux.MiddlewareFunc // can be nil
}

// RegisterWebhookRoutes is deprecated - webhook routes are now handled by the dynamic webhook handler
// This function is kept for backwards compatibility but does nothing
func (s *Server) RegisterWebhookRoutes(router *mux.Router) {
	s.logger.Info("RegisterWebhookRoutes called but is deprecated - using dynamic webhook handler instead", nil)
}
