// Package api implements the REST API for the RAG loader
package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/service"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Handler handles API requests for the RAG loader
type Handler struct {
	service *service.LoaderService
	logger  observability.Logger
}

// NewHandler creates a new API handler
func NewHandler(service *service.LoaderService, logger observability.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// API prefix
	api := router.PathPrefix("/api/v1/rag").Subrouter()

	// Ingestion endpoints
	api.HandleFunc("/ingest", h.triggerIngestion).Methods("POST")
	api.HandleFunc("/jobs", h.listJobs).Methods("GET")
	api.HandleFunc("/jobs/{id}", h.getJob).Methods("GET")

	// Source management (Phase 1 - minimal)
	api.HandleFunc("/sources", h.listSources).Methods("GET")

	// Search endpoint (Phase 1 - placeholder)
	api.HandleFunc("/search", h.search).Methods("POST")
}

// triggerIngestion manually triggers an ingestion job
func (h *Handler) triggerIngestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID string `json:"source_id"`
		Mode     string `json:"mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SourceID == "" {
		h.respondError(w, "source_id is required", http.StatusBadRequest)
		return
	}

	// Trigger ingestion
	if err := h.service.RunIngestion(r.Context(), req.SourceID); err != nil {
		h.logger.Error("Failed to trigger ingestion", map[string]interface{}{
			"source_id": req.SourceID,
			"error":     err.Error(),
		})
		h.respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, map[string]interface{}{
		"status":    "accepted",
		"source_id": req.SourceID,
		"message":   "Ingestion job started",
	}, http.StatusAccepted)
}

// listJobs returns active ingestion jobs
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.service.GetActiveJobs()

	h.respondJSON(w, map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	}, http.StatusOK)
}

// getJob retrieves a specific job (Phase 1 - placeholder)
func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	// For Phase 1, just return a placeholder response
	h.respondJSON(w, map[string]interface{}{
		"id":      jobID,
		"status":  "completed",
		"message": "Job details not implemented in Phase 1",
	}, http.StatusOK)
}

// listSources returns configured data sources
func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	// For Phase 1, return a static list
	sources := []map[string]interface{}{
		{
			"id":      "github_main",
			"type":    "github",
			"enabled": true,
			"status":  "configured",
		},
		{
			"id":      "web_docs",
			"type":    "web",
			"enabled": false,
			"status":  "disabled",
		},
	}

	h.respondJSON(w, map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	}, http.StatusOK)
}

// search performs a search (Phase 1 - placeholder)
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		h.respondError(w, "query is required", http.StatusBadRequest)
		return
	}

	// For Phase 1, return a placeholder response
	h.respondJSON(w, map[string]interface{}{
		"query":   req.Query,
		"results": []interface{}{},
		"count":   0,
		"message": "Search functionality will be implemented in Phase 3",
	}, http.StatusOK)
}

// respondJSON sends a JSON response
func (h *Handler) respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// respondError sends an error response
func (h *Handler) respondError(w http.ResponseWriter, message string, statusCode int) {
	h.respondJSON(w, map[string]interface{}{
		"error": message,
	}, statusCode)
}
