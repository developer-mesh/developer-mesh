package proxies

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EmbeddingProxy handles MCP embedding requests and forwards them to the REST API
type EmbeddingProxy struct {
	restAPIURL    string
	restAPIHost   string // Store the host for validation
	restAPIScheme string // Store the scheme for validation
	httpClient    *http.Client
	logger        observability.Logger
}

// NewEmbeddingProxy creates a new embedding proxy
func NewEmbeddingProxy(restAPIURL string, logger observability.Logger) *EmbeddingProxy {
	// Validate and normalize the REST API URL
	parsedURL, err := url.Parse(restAPIURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		logger.Error("Invalid REST API URL provided", map[string]any{
			"url":   sanitizeLogValue(restAPIURL),
			"error": err,
		})
		// Fall back to a safe default
		restAPIURL = "http://localhost:8081"
	}

	// Ensure URL ends without trailing slash for consistent path joining
	restAPIURL = strings.TrimRight(restAPIURL, "/")

	// Store the host and scheme for validation
	restAPIHost := "localhost:8081" // default host
	restAPIScheme := "http"         // default scheme
	if parsedURL != nil && parsedURL.Host != "" {
		restAPIHost = parsedURL.Host
		restAPIScheme = parsedURL.Scheme
	} else {
		// Re-parse the fallback URL to get the host and scheme
		if fallbackURL, err := url.Parse(restAPIURL); err == nil && fallbackURL != nil {
			restAPIHost = fallbackURL.Host
			restAPIScheme = fallbackURL.Scheme
		}
	}

	return &EmbeddingProxy{
		restAPIURL:    restAPIURL,
		restAPIHost:   restAPIHost,
		restAPIScheme: restAPIScheme,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// RegisterRoutes registers all embedding proxy routes
func (p *EmbeddingProxy) RegisterRoutes(router *gin.RouterGroup) {
	embeddings := router.Group("/embeddings")
	{
		// Core operations
		embeddings.POST("/generate", p.generateEmbedding)
		embeddings.POST("/batch", p.batchGenerateEmbeddings)
		embeddings.POST("/search", p.searchEmbeddings)
		embeddings.POST("/search/cross-model", p.crossModelSearch)

		// Provider health
		embeddings.GET("/providers/health", p.getProviderHealth)

		// Agent configuration
		agents := embeddings.Group("/agents")
		{
			agents.POST("", p.createAgentConfig)
			agents.GET("/:agentId", p.getAgentConfig)
			agents.PUT("/:agentId", p.updateAgentConfig)
			agents.GET("/:agentId/models", p.getAgentModels)
			agents.GET("/:agentId/costs", p.getAgentCosts)
		}
	}
}

// generateEmbedding proxies POST /embeddings/generate
func (p *EmbeddingProxy) generateEmbedding(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings", "POST")
}

// batchGenerateEmbeddings proxies POST /embeddings/batch
func (p *EmbeddingProxy) batchGenerateEmbeddings(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/batch", "POST")
}

// searchEmbeddings proxies POST /embeddings/search
func (p *EmbeddingProxy) searchEmbeddings(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/search", "POST")
}

// crossModelSearch proxies POST /embeddings/search/cross-model
func (p *EmbeddingProxy) crossModelSearch(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/search/cross-model", "POST")
}

// getProviderHealth proxies GET /embeddings/providers/health
func (p *EmbeddingProxy) getProviderHealth(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/providers/health", "GET")
}

// createAgentConfig proxies POST /embeddings/agents
func (p *EmbeddingProxy) createAgentConfig(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/agents", "POST")
}

// getAgentConfig proxies GET /embeddings/agents/:agentId
func (p *EmbeddingProxy) getAgentConfig(c *gin.Context) {
	agentID := c.Param("agentId")
	// Validate agentID to prevent path injection
	if strings.ContainsAny(agentID, "/.\\") || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s", agentID), "GET")
}

// updateAgentConfig proxies PUT /embeddings/agents/:agentId
func (p *EmbeddingProxy) updateAgentConfig(c *gin.Context) {
	agentID := c.Param("agentId")
	// Validate agentID to prevent path injection
	if strings.ContainsAny(agentID, "/.\\") || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s", agentID), "PUT")
}

// getAgentModels proxies GET /embeddings/agents/:agentId/models
func (p *EmbeddingProxy) getAgentModels(c *gin.Context) {
	agentID := c.Param("agentId")
	// Validate agentID to prevent path injection
	if strings.ContainsAny(agentID, "/.\\") || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s/models", agentID), "GET")
}

// getAgentCosts proxies GET /embeddings/agents/:agentId/costs
func (p *EmbeddingProxy) getAgentCosts(c *gin.Context) {
	agentID := c.Param("agentId")
	// Validate agentID to prevent path injection
	if strings.ContainsAny(agentID, "/.\\") || agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s/costs", agentID), "GET")
}

// proxyRequest forwards requests to the REST API
func (p *EmbeddingProxy) proxyRequest(c *gin.Context, path string, method string) {
	// Validate that path doesn't contain any URL components that could lead to SSRF
	if strings.Contains(path, "://") || strings.Contains(path, "..") || strings.HasPrefix(path, "//") {
		p.logger.Warn("Invalid path detected in proxy request", map[string]any{
			"path":   sanitizeLogValue(path),
			"method": method,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request path"})
		return
	}

	// Parse the base URL to safely construct the target URL
	baseURL, err := url.Parse(p.restAPIURL)
	if err != nil {
		p.logger.Error("Failed to parse base URL", map[string]any{
			"error":    err.Error(),
			"base_url": p.restAPIURL,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Safely construct the path
	baseURL.Path = path

	// Add query parameters if present
	if c.Request.URL.RawQuery != "" {
		baseURL.RawQuery = c.Request.URL.RawQuery
	}

	// Get the final URL string
	targetURL := baseURL.String()

	// Read request body
	var body []byte
	if c.Request.Body != nil {
		var readErr error
		body, readErr = io.ReadAll(c.Request.Body)
		if readErr != nil {
			p.logger.Error("Failed to read request body", map[string]any{
				"error": readErr.Error(),
				"path":  path,
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
	}

	// Validate the final URL to ensure it's pointing to our REST API
	finalURL, err := url.Parse(targetURL)
	if err != nil || finalURL == nil || finalURL.Host != p.restAPIHost {
		actualHost := ""
		if finalURL != nil {
			actualHost = finalURL.Host
		}
		p.logger.Error("Invalid target URL detected", map[string]any{
			"error":         err,
			"target_url":    sanitizeLogValue(targetURL),
			"expected_host": p.restAPIHost,
			"actual_host":   actualHost,
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target URL"})
		return
	}

	// Create a new request with empty URL first
	// Security: This is safe - we construct the URL from validated components below, not from user input
	// nosec G107 -- URL is constructed from pre-validated server configuration, not user input
	req, err := http.NewRequestWithContext(c.Request.Context(), method, "", bytes.NewReader(body))
	if err != nil {
		p.logger.Error("Failed to create proxy request", map[string]any{
			"error": err.Error(),
			"path":  path,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Now set the URL components directly on the request to avoid any string-based URL construction
	req.URL = &url.URL{
		Scheme:   p.restAPIScheme,
		Host:     p.restAPIHost,
		Path:     path,
		RawQuery: c.Request.URL.RawQuery,
	}
	req.Host = p.restAPIHost // Explicitly set the Host header

	// Forward headers
	for key, values := range c.Request.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Inject tenant ID if available
	if tenantID, exists := c.Get("tenant_id"); exists {
		req.Header.Set("X-Tenant-ID", tenantID.(string))
	}

	// Add request ID for tracing
	requestID := uuid.New().String()
	req.Header.Set("X-Request-ID", requestID)

	// Log proxy request
	p.logger.Debug("Proxying embedding request", map[string]any{
		"method":     method,
		"path":       path,
		"target_url": sanitizeLogValue(targetURL),
		"request_id": requestID,
	})

	// Execute request - URL has been validated to only point to our configured REST API
	// nosec G107 -- Request URL constructed from validated components above
	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logger.Error("Proxy request failed", map[string]any{
			"error":      err.Error(),
			"path":       path,
			"request_id": requestID,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service unavailable"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.Error("Failed to read response body", map[string]any{
			"error":      err.Error(),
			"path":       path,
			"request_id": requestID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	// Forward response headers
	for key, values := range resp.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Forward status code and body
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// isHopByHopHeader checks if a header is hop-by-hop
func isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopByHopHeaders {
		if header == h {
			return true
		}
	}
	return false
}

// EmbeddingRequest represents a request to generate embeddings
type EmbeddingRequest struct {
	AgentID   string                 `json:"agent_id" validate:"required"`
	Text      string                 `json:"text" validate:"required,max=50000"`
	TaskType  string                 `json:"task_type,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// EmbeddingResponse represents the response from embedding generation
type EmbeddingResponse struct {
	EmbeddingID          uuid.UUID              `json:"embedding_id"`
	RequestID            string                 `json:"request_id"`
	ModelUsed            string                 `json:"model_used"`
	Provider             string                 `json:"provider"`
	Dimensions           int                    `json:"dimensions"`
	NormalizedDimensions int                    `json:"normalized_dimensions"`
	CostUSD              float64                `json:"cost_usd"`
	TokensUsed           int                    `json:"tokens_used"`
	GenerationTimeMs     int64                  `json:"generation_time_ms"`
	Cached               bool                   `json:"cached"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query          string                 `json:"query,omitempty"`
	QueryEmbedding []float32              `json:"query_embedding,omitempty"`
	AgentID        string                 `json:"agent_id" validate:"required"`
	Limit          int                    `json:"limit,omitempty"`
	MinSimilarity  float64                `json:"min_similarity,omitempty"`
	MetadataFilter map[string]interface{} `json:"metadata_filter,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID         uuid.UUID              `json:"id"`
	Content    string                 `json:"content"`
	Similarity float64                `json:"similarity"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// sanitizeLogValue removes newlines and carriage returns from user input to prevent log injection
func sanitizeLogValue(input string) string {
	// Remove newlines, carriage returns, and other control characters
	sanitized := strings.ReplaceAll(input, "\n", "\\n")
	sanitized = strings.ReplaceAll(sanitized, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\t", "\\t")
	// Limit length to prevent excessive log sizes
	if len(sanitized) > 100 {
		sanitized = sanitized[:100] + "..."
	}
	return sanitized
}
