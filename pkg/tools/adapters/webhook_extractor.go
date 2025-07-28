package adapters

import (
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/getkin/kin-openapi/openapi3"
)

// WebhookExtractor extracts webhook configuration from OpenAPI specifications
type WebhookExtractor struct{}

// NewWebhookExtractor creates a new webhook extractor
func NewWebhookExtractor() *WebhookExtractor {
	return &WebhookExtractor{}
}

// ExtractWebhookConfig extracts webhook configuration from an OpenAPI spec
func (e *WebhookExtractor) ExtractWebhookConfig(spec *openapi3.T, toolID string) *models.ToolWebhookConfig {
	if spec == nil {
		return nil
	}

	config := &models.ToolWebhookConfig{
		Enabled:      false,
		EndpointPath: fmt.Sprintf("/api/webhooks/tools/%s", toolID),
		Events:       []models.WebhookEventConfig{},
	}

	// Check for webhooks extension (OpenAPI 3.1)
	if spec.Extensions != nil {
		if webhooks, ok := spec.Extensions["x-webhooks"]; ok {
			e.parseWebhooksExtension(webhooks, config)
		}
	}

	// Check for callbacks in operations
	if spec.Paths != nil {
		e.parseCallbacks(spec.Paths, config)
	}

	// Check for webhook-related security schemes
	e.parseWebhookSecurity(spec.Components, config)

	// Look for webhook hints in API description
	e.parseWebhookHints(spec.Info, config)

	// Only enable if we found webhook information
	if len(config.Events) > 0 || config.AuthType != "" {
		config.Enabled = true
	}

	return config
}

// parseWebhooksExtension parses the x-webhooks extension
func (e *WebhookExtractor) parseWebhooksExtension(webhooks interface{}, config *models.ToolWebhookConfig) {
	webhooksMap, ok := webhooks.(map[string]interface{})
	if !ok {
		return
	}

	for eventName, eventData := range webhooksMap {
		if eventMap, ok := eventData.(map[string]interface{}); ok {
			event := models.WebhookEventConfig{
				EventType: eventName,
			}

			// Extract schema if available
			if schema, ok := eventMap["schema"].(map[string]interface{}); ok {
				if schemaURL, ok := schema["$ref"].(string); ok {
					event.SchemaURL = schemaURL
				}
			}

			// Extract required fields
			if required, ok := eventMap["required"].([]interface{}); ok {
				for _, field := range required {
					if fieldStr, ok := field.(string); ok {
						event.RequiredFields = append(event.RequiredFields, fieldStr)
					}
				}
			}

			config.Events = append(config.Events, event)
		}
	}
}

// parseCallbacks looks for callback definitions in operations
func (e *WebhookExtractor) parseCallbacks(paths *openapi3.Paths, config *models.ToolWebhookConfig) {
	for path, pathItem := range paths.Map() {
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, operation := range operations {
			if operation == nil || operation.Callbacks == nil {
				continue
			}

			for callbackName, callback := range operation.Callbacks {
				e.parseCallback(callbackName, callback, config, path, method)
			}
		}
	}
}

// parseCallback extracts webhook information from a callback definition
func (e *WebhookExtractor) parseCallback(name string, callback *openapi3.CallbackRef, config *models.ToolWebhookConfig, path, method string) {
	if callback.Value == nil {
		return
	}

	if callback.Value != nil {
		for expression, pathItem := range callback.Value.Map() {
			// The expression often contains the webhook URL pattern
			event := models.WebhookEventConfig{
				EventType: name,
			}

			// Look for the actual webhook payload schema
			if pathItem.Post != nil && pathItem.Post.RequestBody != nil && pathItem.Post.RequestBody.Value != nil {
				if content, ok := pathItem.Post.RequestBody.Value.Content["application/json"]; ok && content.Schema != nil {
					if content.Schema.Ref != "" {
						event.SchemaURL = content.Schema.Ref
					}
				}
			}

			config.Events = append(config.Events, event)
			_ = expression // Will extract more details in the future
		}
	}
}

// parseWebhookSecurity looks for webhook-specific security schemes
func (e *WebhookExtractor) parseWebhookSecurity(components *openapi3.Components, config *models.ToolWebhookConfig) {
	if components == nil || components.SecuritySchemes == nil {
		return
	}

	for name, scheme := range components.SecuritySchemes {
		if scheme.Value == nil {
			continue
		}

		// Look for webhook-related security schemes
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, "webhook") || strings.Contains(nameLower, "signature") {
			switch scheme.Value.Type {
			case "apiKey":
				if scheme.Value.In == "header" {
					config.AuthType = "signature"
					config.SignatureHeader = scheme.Value.Name
					if config.AuthConfig == nil {
						config.AuthConfig = make(map[string]interface{})
					}
					config.AuthConfig["header_name"] = scheme.Value.Name
				}
			case "http":
				switch scheme.Value.Scheme {
				case "bearer":
					config.AuthType = "bearer"
				case "basic":
					config.AuthType = "basic"
				}
			}
		}
	}
}

// parseWebhookHints looks for webhook information in the API description
func (e *WebhookExtractor) parseWebhookHints(info *openapi3.Info, config *models.ToolWebhookConfig) {
	if info == nil {
		return
	}

	// Check description for webhook-related keywords
	description := strings.ToLower(info.Description)

	// Look for signature algorithms
	if strings.Contains(description, "hmac-sha256") {
		config.SignatureAlgorithm = "hmac-sha256"
		if config.AuthType == "" {
			config.AuthType = "hmac"
		}
	} else if strings.Contains(description, "hmac-sha1") {
		config.SignatureAlgorithm = "hmac-sha1"
		if config.AuthType == "" {
			config.AuthType = "hmac"
		}
	}

	// Look for common webhook headers
	headerPatterns := map[string]string{
		"x-hub-signature":     "X-Hub-Signature",
		"x-webhook-signature": "X-Webhook-Signature",
		"x-signature":         "X-Signature",
		"x-event-type":        "X-Event-Type",
		"x-event":             "X-Event",
	}

	for pattern, header := range headerPatterns {
		if strings.Contains(description, pattern) {
			if strings.Contains(pattern, "signature") && config.SignatureHeader == "" {
				config.SignatureHeader = header
			}
			if config.Headers == nil {
				config.Headers = make(map[string]string)
			}
			if strings.Contains(pattern, "event") {
				config.Headers["event_type_header"] = header
			}
		}
	}

	// Check for webhook documentation in extensions
	if info.Extensions != nil {
		e.parseInfoExtensions(info.Extensions, config)
	}
}

// parseInfoExtensions looks for webhook info in OpenAPI extensions
func (e *WebhookExtractor) parseInfoExtensions(extensions map[string]interface{}, config *models.ToolWebhookConfig) {
	// Check for common webhook extension patterns
	webhookExtensions := []string{
		"x-webhook-config",
		"x-webhooks-config",
		"x-webhook-auth",
		"x-webhook-security",
	}

	for _, ext := range webhookExtensions {
		if val, ok := extensions[ext]; ok {
			if configMap, ok := val.(map[string]interface{}); ok {
				// Extract auth type
				if authType, ok := configMap["auth_type"].(string); ok {
					config.AuthType = authType
				}

				// Extract signature header
				if sigHeader, ok := configMap["signature_header"].(string); ok {
					config.SignatureHeader = sigHeader
				}

				// Extract signature algorithm
				if sigAlg, ok := configMap["signature_algorithm"].(string); ok {
					config.SignatureAlgorithm = sigAlg
				}

				// Extract events
				if events, ok := configMap["events"].([]interface{}); ok {
					for _, event := range events {
						if eventStr, ok := event.(string); ok {
							config.Events = append(config.Events, models.WebhookEventConfig{
								EventType: eventStr,
							})
						}
					}
				}
			}
		}
	}
}

// ExtractWebhookEvents extracts webhook event types from common patterns in the spec
func (e *WebhookExtractor) ExtractWebhookEvents(spec *openapi3.T) []string {
	events := make(map[string]bool)

	// Common webhook event patterns
	commonEvents := []string{
		"push", "pull_request", "issue", "comment", "release",
		"create", "update", "delete", "merge", "deploy",
		"build", "test", "status", "workflow", "pipeline",
	}

	// Search through paths for webhook-related endpoints
	if spec.Paths != nil {
		for path := range spec.Paths.Map() {
			pathLower := strings.ToLower(path)
			for _, event := range commonEvents {
				if strings.Contains(pathLower, event) {
					events[event] = true
				}
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(events))
	for event := range events {
		result = append(result, event)
	}

	return result
}

// GenerateWebhookDocumentation generates webhook documentation for a tool
func (e *WebhookExtractor) GenerateWebhookDocumentation(config *models.ToolWebhookConfig, baseURL, toolName string) map[string]interface{} {
	if config == nil || !config.Enabled {
		return nil
	}

	webhookURL := baseURL + config.EndpointPath

	doc := map[string]interface{}{
		"webhook_url": webhookURL,
		"enabled":     config.Enabled,
		"auth_type":   config.AuthType,
	}

	if config.SignatureHeader != "" {
		doc["signature_header"] = config.SignatureHeader
	}

	if config.SignatureAlgorithm != "" {
		doc["signature_algorithm"] = config.SignatureAlgorithm
	}

	if len(config.Events) > 0 {
		eventTypes := make([]string, 0, len(config.Events))
		for _, event := range config.Events {
			eventTypes = append(eventTypes, event.EventType)
		}
		doc["supported_events"] = eventTypes
	}

	// Add setup instructions based on auth type
	instructions := []string{
		fmt.Sprintf("1. Configure your %s to send webhooks to: %s", toolName, webhookURL),
	}

	switch config.AuthType {
	case "hmac":
		instructions = append(instructions, "2. Set up HMAC signing with the provided secret")
		instructions = append(instructions, fmt.Sprintf("3. Include the signature in the '%s' header", config.SignatureHeader))
	case "bearer":
		instructions = append(instructions, "2. Include the authentication token in the Authorization header as 'Bearer <token>'")
	case "basic":
		instructions = append(instructions, "2. Use HTTP Basic Authentication with the provided credentials")
	}

	doc["setup_instructions"] = instructions

	return doc
}
