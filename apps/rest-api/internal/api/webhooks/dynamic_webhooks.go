package webhooks

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	pkgrepository "github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DynamicWebhookHandler handles webhooks for dynamically registered tools
type DynamicWebhookHandler struct {
	toolRepo  pkgrepository.DynamicToolRepository
	eventRepo pkgrepository.WebhookEventRepository
	logger    observability.Logger
}

// NewDynamicWebhookHandler creates a new dynamic webhook handler
func NewDynamicWebhookHandler(
	toolRepo pkgrepository.DynamicToolRepository,
	eventRepo pkgrepository.WebhookEventRepository,
	logger observability.Logger,
) *DynamicWebhookHandler {
	return &DynamicWebhookHandler{
		toolRepo:  toolRepo,
		eventRepo: eventRepo,
		logger:    logger,
	}
}

// HandleDynamicWebhook handles incoming webhooks for dynamic tools
// @Summary Receive webhook for dynamic tool
// @Description Process incoming webhook events for dynamically registered tools
// @Tags webhooks
// @Accept json
// @Produce json
// @Param toolId path string true "Tool ID"
// @Param body body object true "Webhook payload"
// @Success 200 {object} map[string]interface{} "Webhook processed successfully"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 403 {object} map[string]interface{} "Forbidden"
// @Failure 404 {object} map[string]interface{} "Tool not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/webhooks/tools/{toolId} [post]
func (h *DynamicWebhookHandler) HandleDynamicWebhook(c *gin.Context) {
	toolID := c.Param("toolId")

	// Get tool configuration
	tool, err := h.toolRepo.GetByID(c.Request.Context(), toolID)
	if err != nil {
		h.logger.Error("Tool not found for webhook", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool not found"})
		return
	}

	// Check if webhooks are enabled for this tool
	if tool.WebhookConfig == nil || !tool.WebhookConfig.Enabled {
		h.logger.Warn("Webhook received for tool with webhooks disabled", map[string]interface{}{
			"tool_id": toolID,
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "Webhooks not enabled for this tool"})
		return
	}

	// Read the request body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Validate the webhook based on auth type
	if !h.validateWebhook(c.Request, bodyBytes, tool.WebhookConfig) {
		h.logger.Warn("Webhook validation failed", map[string]interface{}{
			"tool_id":   toolID,
			"source_ip": c.ClientIP(),
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}

	// Extract event type from payload
	eventType, err := h.extractEventType(bodyBytes, tool.WebhookConfig)
	if err != nil {
		h.logger.Error("Failed to extract event type", map[string]interface{}{
			"tool_id": toolID,
			"error":   err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to extract event type"})
		return
	}

	// Store the webhook event
	receivedAt := time.Now()
	if rt, exists := c.Get("request_time"); exists {
		if t, ok := rt.(time.Time); ok {
			receivedAt = t
		}
	}

	event := &models.WebhookEvent{
		ID:         uuid.New().String(),
		ToolID:     toolID,
		TenantID:   tool.TenantID,
		EventType:  eventType,
		Payload:    json.RawMessage(bodyBytes),
		Headers:    c.Request.Header,
		SourceIP:   c.ClientIP(),
		ReceivedAt: receivedAt,
		Status:     "pending",
		Metadata: map[string]interface{}{
			"tool_name": tool.ToolName,
			"provider":  tool.Provider,
		},
	}

	if err := h.eventRepo.Create(c.Request.Context(), event); err != nil {
		h.logger.Error("Failed to store webhook event", map[string]interface{}{
			"tool_id":    toolID,
			"event_type": eventType,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store event"})
		return
	}

	// Queue event for processing
	ctx := c.Request.Context()
	queueClient, err := queue.NewClient(ctx, &queue.Config{
		Logger: h.logger,
	})
	if err != nil {
		h.logger.Error("Failed to create queue client", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue event"})
		return
	}
	defer func() {
		if err := queueClient.Close(); err != nil {
			h.logger.Warn("Failed to close queue client", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Create queue event
	queueEvent := queue.Event{
		EventID:    event.ID,
		EventType:  eventType,
		RepoName:   tool.ToolName, // Using tool name as repo name
		SenderName: tool.Provider, // Using provider as sender
		Payload:    json.RawMessage(bodyBytes),
		Timestamp:  receivedAt,
		Metadata: map[string]interface{}{
			"tool_id":     toolID,
			"tool_name":   tool.ToolName,
			"provider":    tool.Provider,
			"tenant_id":   tool.TenantID,
			"source_ip":   c.ClientIP(),
			"webhook_url": c.Request.URL.String(),
		},
		AuthContext: &queue.EventAuthContext{
			TenantID:      tool.TenantID,
			PrincipalID:   toolID,
			PrincipalType: "tool",
			Metadata: map[string]interface{}{
				"tool_config": tool.WebhookConfig,
			},
		},
	}

	if err := queueClient.EnqueueEvent(ctx, queueEvent); err != nil {
		h.logger.Error("Failed to enqueue webhook event", map[string]interface{}{
			"error":    err.Error(),
			"event_id": event.ID,
		})
		// Update event status to failed
		_ = h.eventRepo.UpdateStatus(ctx, event.ID, "failed", nil, err.Error())

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue event"})
		return
	}

	// Update event status to queued
	now := time.Now()
	_ = h.eventRepo.UpdateStatus(ctx, event.ID, "queued", &now, "")

	h.logger.Info("Webhook event received", map[string]interface{}{
		"tool_id":    toolID,
		"event_id":   event.ID,
		"event_type": eventType,
		"source_ip":  c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{
		"event_id": event.ID,
		"status":   "accepted",
		"message":  "Webhook event queued for processing",
	})
}

// validateWebhook validates the webhook based on the configured auth type
func (h *DynamicWebhookHandler) validateWebhook(r *http.Request, body []byte, config *models.ToolWebhookConfig) bool {
	switch config.AuthType {
	case "hmac":
		return h.validateHMACSignature(r, body, config)
	case "bearer":
		return h.validateBearerToken(r, config)
	case "basic":
		return h.validateBasicAuth(r, config)
	case "signature":
		return h.validateCustomSignature(r, body, config)
	case "none":
		return true
	default:
		h.logger.Warn("Unknown webhook auth type", map[string]interface{}{
			"auth_type": config.AuthType,
		})
		return false
	}
}

// validateHMACSignature validates HMAC-based webhook signatures
func (h *DynamicWebhookHandler) validateHMACSignature(r *http.Request, body []byte, config *models.ToolWebhookConfig) bool {
	signatureHeader := config.SignatureHeader
	if signatureHeader == "" {
		signatureHeader = "X-Webhook-Signature"
	}

	signature := r.Header.Get(signatureHeader)
	if signature == "" {
		return false
	}

	secret, ok := config.AuthConfig["secret"].(string)
	if !ok || secret == "" {
		h.logger.Error("HMAC secret not configured", nil)
		return false
	}

	// Determine the algorithm
	var mac hash.Hash
	switch config.SignatureAlgorithm {
	case "hmac-sha1":
		mac = hmac.New(sha1.New, []byte(secret))
	case "hmac-sha256", "":
		mac = hmac.New(sha256.New, []byte(secret))
	default:
		h.logger.Error("Unsupported signature algorithm", map[string]interface{}{
			"algorithm": config.SignatureAlgorithm,
		})
		return false
	}

	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Some webhooks prefix the signature
	if strings.HasPrefix(signature, "sha256=") {
		signature = strings.TrimPrefix(signature, "sha256=")
	} else if strings.HasPrefix(signature, "sha1=") {
		signature = strings.TrimPrefix(signature, "sha1=")
	}

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// validateBearerToken validates bearer token authentication
func (h *DynamicWebhookHandler) validateBearerToken(r *http.Request, config *models.ToolWebhookConfig) bool {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	expectedToken, ok := config.AuthConfig["token"].(string)
	if !ok || expectedToken == "" {
		h.logger.Error("Bearer token not configured", nil)
		return false
	}

	return token == expectedToken
}

// validateBasicAuth validates basic authentication
func (h *DynamicWebhookHandler) validateBasicAuth(r *http.Request, config *models.ToolWebhookConfig) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	expectedUsername, _ := config.AuthConfig["username"].(string)
	expectedPassword, _ := config.AuthConfig["password"].(string)

	return username == expectedUsername && password == expectedPassword
}

// validateCustomSignature validates custom signature schemes
func (h *DynamicWebhookHandler) validateCustomSignature(r *http.Request, body []byte, config *models.ToolWebhookConfig) bool {
	// This is a placeholder for tool-specific signature validation
	// Each tool might have its own signature scheme

	// Example: Some tools concatenate multiple headers with the body
	signatureHeader := config.SignatureHeader
	if signatureHeader == "" {
		return false
	}

	signature := r.Header.Get(signatureHeader)
	if signature == "" {
		return false
	}

	// Tool-specific validation logic would go here
	// For now, we'll just check if the signature is present
	return true
}

// extractEventType extracts the event type from the webhook payload
func (h *DynamicWebhookHandler) extractEventType(body []byte, config *models.ToolWebhookConfig) (string, error) {
	// First, check if there's a header that contains the event type
	eventHeader, ok := config.Headers["event_type_header"]
	if ok {
		// The actual header value would be passed in the request
		// This is just the configuration
		return eventHeader, nil
	}

	// Try to parse the body as JSON and extract event type
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Look for event type in common locations
	commonPaths := []string{"event_type", "eventType", "type", "action", "event"}
	for _, path := range commonPaths {
		if eventType, ok := payload[path].(string); ok {
			return eventType, nil
		}
	}

	// Check configured event types
	for _, eventConfig := range config.Events {
		if eventConfig.PayloadPath != "" {
			// TODO: Implement JSON path extraction
			// For now, simple dot notation
			parts := strings.Split(eventConfig.PayloadPath, ".")
			current := payload
			for i, part := range parts {
				if i == len(parts)-1 {
					if val, ok := current[part].(string); ok {
						return val, nil
					}
				} else {
					if next, ok := current[part].(map[string]interface{}); ok {
						current = next
					} else {
						break
					}
				}
			}
		}
	}

	return "unknown", nil
}

// GetWebhookURL returns the webhook URL for a tool
func (h *DynamicWebhookHandler) GetWebhookURL(baseURL, toolID string) string {
	return fmt.Sprintf("%s/api/webhooks/tools/%s", strings.TrimSuffix(baseURL, "/"), toolID)
}
