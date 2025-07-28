package worker

import (
	"encoding/json"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/stretchr/testify/assert"
)

func TestEventTransformer_Transform(t *testing.T) {
	logger := observability.NewNoopLogger()
	transformer := NewEventTransformer(logger)

	tests := []struct {
		name           string
		event          queue.Event
		rules          map[string]interface{}
		expectedFields map[string]interface{}
	}{
		{
			name: "direct field mapping",
			event: queue.Event{
				EventID: "test-123",
				Payload: json.RawMessage(`{"user": "john", "action": "push", "branch": "main"}`),
			},
			rules: map[string]interface{}{
				"username":     "user",
				"event_action": "action",
			},
			expectedFields: map[string]interface{}{
				"username":     "john",
				"event_action": "push",
			},
		},
		{
			name: "rename transformation",
			event: queue.Event{
				EventID: "test-123",
				Payload: json.RawMessage(`{"repository": "test-repo", "ref": "refs/heads/main"}`),
			},
			rules: map[string]interface{}{
				"repo_name": map[string]interface{}{
					"type":   "rename",
					"source": "repository",
				},
			},
			expectedFields: map[string]interface{}{
				"repo_name": "test-repo",
			},
		},
		{
			name: "constant value",
			event: queue.Event{
				EventID: "test-123",
				Payload: json.RawMessage(`{"action": "push"}`),
			},
			rules: map[string]interface{}{
				"provider": map[string]interface{}{
					"type":  "constant",
					"value": "github",
				},
			},
			expectedFields: map[string]interface{}{
				"provider": "github",
			},
		},
		{
			name: "concatenate fields",
			event: queue.Event{
				EventID: "test-123",
				Payload: json.RawMessage(`{"owner": "john", "repo": "test"}`),
			},
			rules: map[string]interface{}{
				"full_name": map[string]interface{}{
					"type":   "concatenate",
					"fields": []interface{}{"owner", "repo"},
				},
			},
			expectedFields: map[string]interface{}{
				"full_name": "johntest",
			},
		},
		{
			name: "no transformation",
			event: queue.Event{
				EventID: "test-123",
				Payload: json.RawMessage(`{"action": "push"}`),
			},
			rules:          map[string]interface{}{},
			expectedFields: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformedEvent, err := transformer.Transform(tt.event, tt.rules)
			assert.NoError(t, err)

			// Parse transformed payload
			var transformedPayload map[string]interface{}
			err = json.Unmarshal(transformedEvent.Payload, &transformedPayload)
			assert.NoError(t, err)

			// Check expected fields
			for key, expectedValue := range tt.expectedFields {
				assert.Equal(t, expectedValue, transformedPayload[key])
			}
		})
	}
}

func TestEventTransformer_Transform_InvalidPayload(t *testing.T) {
	logger := observability.NewNoopLogger()
	transformer := NewEventTransformer(logger)

	event := queue.Event{
		EventID: "test-123",
		Payload: json.RawMessage(`{invalid json`),
	}

	rules := map[string]interface{}{
		"field": "value",
	}

	_, err := transformer.Transform(event, rules)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}
