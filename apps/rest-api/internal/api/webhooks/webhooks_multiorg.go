// Package webhooks provides GitHub webhook handlers with multi-organization support
package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/developer-mesh/developer-mesh/pkg/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// GitHubWebhookHandlerMultiOrg creates a webhook handler with multi-organization support
func GitHubWebhookHandlerMultiOrg(
	config interfaces.WebhookConfigInterface,
	webhookRepo repository.WebhookConfigRepository,
	logger observability.Logger,
) http.HandlerFunc {
	if logger == nil {
		logger = observability.NewLogger("webhooks")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract and validate GitHub event type
		eventType := r.Header.Get("X-GitHub-Event")
		if eventType == "" {
			http.Error(w, "Missing X-GitHub-Event header", http.StatusBadRequest)
			logger.Warn("GitHub webhook missing event header", nil)
			return
		}

		// Verify HTTP method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			logger.Warn("GitHub webhook received non-POST request", map[string]any{"method": r.Method})
			return
		}

		// Read request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			logger.Error("GitHub webhook failed to read request body", map[string]any{"error": err.Error()})
			return
		}
		// Restore the body for later use
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Parse payload to extract organization
		var payload map[string]any
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Failed to parse payload", http.StatusBadRequest)
			logger.Error("GitHub webhook failed to parse payload", map[string]any{"error": err.Error()})
			return
		}

		// Extract organization name from payload
		orgName := extractOrganizationName(payload)
		if orgName == "" {
			http.Error(w, "Could not determine organization from payload", http.StatusBadRequest)
			logger.Warn("GitHub webhook missing organization info", nil)
			return
		}

		// Look up webhook configuration for this organization
		webhookConfig, err := webhookRepo.GetByOrganization(r.Context(), orgName)
		if err != nil {
			http.Error(w, "Unknown organization", http.StatusUnauthorized)
			logger.Warn("GitHub webhook from unknown organization", map[string]any{
				"organization": orgName,
				"error":        err.Error(),
			})
			return
		}

		// Check if webhook is enabled for this organization
		if !webhookConfig.Enabled {
			http.Error(w, "Webhook disabled for this organization", http.StatusForbidden)
			logger.Warn("GitHub webhook received for disabled organization", map[string]any{
				"organization": orgName,
			})
			return
		}

		// Check if event type is allowed for this organization
		if !webhookConfig.IsEventAllowed(eventType) {
			http.Error(w, "Event type not allowed for this organization", http.StatusForbidden)
			logger.Warn("GitHub webhook event type not allowed", map[string]any{
				"eventType":    eventType,
				"organization": orgName,
			})
			return
		}

		// Verify signature with organization-specific secret
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			http.Error(w, "Missing X-Hub-Signature-256 header", http.StatusUnauthorized)
			logger.Warn("GitHub webhook missing signature header", map[string]any{
				"organization": orgName,
			})
			return
		}

		// Verify signature with org-specific secret
		if !verifySignature(bodyBytes, signature, webhookConfig.WebhookSecret) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			logger.Warn("GitHub webhook invalid signature", map[string]any{
				"organization": orgName,
			})
			return
		}

		// Validate required payload structure
		repoName := extractRepoName(payload)
		senderName := extractSenderName(payload)
		if repoName == "unknown" || senderName == "unknown" {
			http.Error(w, "Missing required fields in payload", http.StatusBadRequest)
			logger.Warn("Webhook payload missing repository or sender info", map[string]any{
				"eventType":    eventType,
				"organization": orgName,
			})
			return
		}

		// Log the event
		logger.Info("GitHub webhook received", map[string]any{
			"eventType":    eventType,
			"repository":   repoName,
			"sender":       senderName,
			"organization": orgName,
		})

		// Enqueue event to SQS for asynchronous processing
		sqsURL := os.Getenv("SQS_QUEUE_URL")
		if sqsURL == "" {
			// In test environment, SQS may not be configured - don't fail
			logger.Warn("SQS_QUEUE_URL not configured - skipping SQS enqueue", nil)
			// Return success
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprintf(w, `{"status":"success","message":"Webhook received","organization":"%s"}`, orgName); err != nil {
				logger.Error("Failed to write response", map[string]interface{}{"error": err})
			}
			return
		}

		// Extract auth context from webhook payload
		authContext := extractAuthContext(payload, eventType)

		// Add organization to auth context metadata
		if authContext == nil {
			authContext = &queue.EventAuthContext{
				Metadata: make(map[string]interface{}),
			}
		}
		if authContext.Metadata == nil {
			authContext.Metadata = make(map[string]interface{})
		}
		authContext.Metadata["organization"] = orgName

		event := queue.SQSEvent{
			DeliveryID:  r.Header.Get("X-GitHub-Delivery"),
			EventType:   eventType,
			RepoName:    repoName,
			SenderName:  senderName,
			Payload:     json.RawMessage(bodyBytes),
			AuthContext: authContext,
		}

		ctx := r.Context()
		// Create Redis queue client
		queueClient, err := queue.NewClient(ctx, &queue.Config{
			Logger: logger,
		})
		if err != nil {
			http.Error(w, "Failed to create queue client", http.StatusInternalServerError)
			logger.Error("Failed to create queue client", map[string]any{"error": err.Error()})
			return
		}
		defer queueClient.Close()

		err = queueClient.EnqueueEvent(ctx, event)
		if err != nil {
			http.Error(w, "Failed to enqueue event", http.StatusInternalServerError)
			logger.Error("Failed to enqueue event to Redis", map[string]any{"error": err.Error()})
			return
		}

		// Return success response immediately
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"status":"success","message":"Webhook received","organization":"%s"}`, orgName)
		_, _ = w.Write([]byte(response))
	}
}

// extractOrganizationName extracts the organization name from the webhook payload
func extractOrganizationName(payload map[string]any) string {
	// Try to get organization from repository.owner
	if repo, ok := payload["repository"].(map[string]any); ok {
		if owner, ok := repo["owner"].(map[string]any); ok {
			if login, ok := owner["login"].(string); ok {
				return login
			}
		}
	}

	// For organization events, check organization field directly
	if org, ok := payload["organization"].(map[string]any); ok {
		if login, ok := org["login"].(string); ok {
			return login
		}
	}

	// For some events like team events, check team.organization
	if team, ok := payload["team"].(map[string]any); ok {
		if org, ok := team["organization"].(map[string]any); ok {
			if login, ok := org["login"].(string); ok {
				return login
			}
		}
	}

	return ""
}

// CreateMultiOrgHandler creates a webhook handler that automatically uses multi-org support if repository is provided
func CreateMultiOrgHandler(
	config interfaces.WebhookConfigInterface,
	webhookRepo repository.WebhookConfigRepository,
	logger observability.Logger,
) http.HandlerFunc {
	// If webhook repository is provided, use multi-org handler
	if webhookRepo != nil {
		return GitHubWebhookHandlerMultiOrg(config, webhookRepo, logger)
	}

	// Otherwise, fall back to single-org handler
	return GitHubWebhookHandler(config, logger)
}
