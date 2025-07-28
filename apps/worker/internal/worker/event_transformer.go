package worker

import (
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

// EventTransformerImpl transforms events based on rules
type EventTransformerImpl struct {
	logger observability.Logger
}

// NewEventTransformer creates a new event transformer
func NewEventTransformer(logger observability.Logger) EventTransformer {
	return &EventTransformerImpl{
		logger: logger,
	}
}

// Transform applies transformation rules to an event
func (t *EventTransformerImpl) Transform(event queue.Event, rules map[string]interface{}) (queue.Event, error) {
	if len(rules) == 0 {
		// No transformation needed
		return event, nil
	}

	// Parse the event payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return event, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Apply transformation rules
	transformedPayload := make(map[string]interface{})

	// Simple transformation logic - can be enhanced later
	for key, rule := range rules {
		switch r := rule.(type) {
		case string:
			// Direct field mapping
			if val, ok := payload[r]; ok {
				transformedPayload[key] = val
			}
		case map[string]interface{}:
			// Complex transformation rule
			if ruleType, ok := r["type"].(string); ok {
				switch ruleType {
				case "rename":
					if source, ok := r["source"].(string); ok {
						if val, ok := payload[source]; ok {
							transformedPayload[key] = val
						}
					}
				case "constant":
					if value, ok := r["value"]; ok {
						transformedPayload[key] = value
					}
				case "concatenate":
					if fields, ok := r["fields"].([]interface{}); ok {
						var result string
						for _, field := range fields {
							if fieldName, ok := field.(string); ok {
								if val, ok := payload[fieldName].(string); ok {
									result += val
								}
							}
						}
						transformedPayload[key] = result
					}
				default:
					t.logger.Warn("Unknown transformation rule type", map[string]interface{}{
						"type": ruleType,
						"key":  key,
					})
				}
			}
		default:
			// Copy as-is
			transformedPayload[key] = rule
		}
	}

	// Marshal the transformed payload
	newPayload, err := json.Marshal(transformedPayload)
	if err != nil {
		return event, fmt.Errorf("failed to marshal transformed payload: %w", err)
	}

	// Create a new event with the transformed payload
	transformedEvent := event
	transformedEvent.Payload = newPayload

	t.logger.Debug("Event transformed", map[string]interface{}{
		"event_id":         event.EventID,
		"original_size":    len(event.Payload),
		"transformed_size": len(newPayload),
		"rules_applied":    len(rules),
	})

	return transformedEvent, nil
}
