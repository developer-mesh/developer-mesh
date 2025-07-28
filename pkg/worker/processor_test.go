package worker

import (
	"encoding/json"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/stretchr/testify/assert"
)

func TestProcessEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   queue.Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful_pull_request_event",
			event: queue.Event{
				EventID:    "test-1",
				EventType:  "pull_request",
				RepoName:   "test-repo",
				SenderName: "test-user",
				Payload:    json.RawMessage(`{"action": "opened", "number": 123}`),
			},
			wantErr: false,
		},
		{
			name: "successful_issue_event",
			event: queue.Event{
				EventID:    "test-2",
				EventType:  "issues",
				RepoName:   "test-repo",
				SenderName: "test-user",
				Payload:    json.RawMessage(`{"action": "opened", "issue": {"number": 456}}`),
			},
			wantErr: false,
		},
		{
			name: "push_event_simulated_failure",
			event: queue.Event{
				EventID:    "test-3",
				EventType:  "push",
				RepoName:   "test-repo",
				SenderName: "test-user",
				Payload:    json.RawMessage(`{"commits": [{"id": "abc123"}]}`),
			},
			wantErr: true,
			errMsg:  "simulated failure for push event",
		},
		{
			name: "invalid_json_payload",
			event: queue.Event{
				EventID:    "test-4",
				EventType:  "pull_request",
				RepoName:   "test-repo",
				SenderName: "test-user",
				Payload:    json.RawMessage(`{invalid json}`),
			},
			wantErr: true,
			errMsg:  "failed to unmarshal payload",
		},
		{
			name: "event_with_auth_context",
			event: queue.Event{
				EventID:    "test-5",
				EventType:  "release",
				RepoName:   "test-repo",
				SenderName: "test-user",
				Payload:    json.RawMessage(`{"tag_name": "v1.0.0"}`),
				AuthContext: &queue.EventAuthContext{
					TenantID:      "tenant-123",
					PrincipalID:   "user-456",
					PrincipalType: "user",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProcessEvent(tt.event)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessSQSEvent_LegacyCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		sqsEvent queue.SQSEvent
		wantErr  bool
		errMsg   string
	}{
		{
			name: "legacy_pull_request",
			sqsEvent: queue.SQSEvent{
				DeliveryID: "legacy-1",
				EventType:  "pull_request",
				RepoName:   "legacy-repo",
				SenderName: "legacy-user",
				Payload:    json.RawMessage(`{"action": "closed"}`),
			},
			wantErr: false,
		},
		{
			name: "legacy_push_failure",
			sqsEvent: queue.SQSEvent{
				DeliveryID: "legacy-2",
				EventType:  "push",
				RepoName:   "legacy-repo",
				SenderName: "legacy-user",
				Payload:    json.RawMessage(`{"commits": 5}`),
			},
			wantErr: true,
			errMsg:  "simulated failure",
		},
		{
			name: "legacy_with_auth",
			sqsEvent: queue.SQSEvent{
				DeliveryID: "legacy-3",
				EventType:  "issues",
				RepoName:   "legacy-repo",
				SenderName: "legacy-user",
				Payload:    json.RawMessage(`{"action": "labeled"}`),
				AuthContext: &queue.EventAuthContext{
					TenantID:    "legacy-tenant",
					PrincipalID: "legacy-principal",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProcessSQSEvent(tt.sqsEvent)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Benchmark tests to ensure performance
func BenchmarkProcessEvent(b *testing.B) {
	event := queue.Event{
		EventID:    "bench-1",
		EventType:  "pull_request",
		RepoName:   "bench-repo",
		SenderName: "bench-user",
		Payload:    json.RawMessage(`{"action": "synchronize", "number": 999}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ProcessEvent(event)
	}
}

func BenchmarkProcessSQSEvent_Legacy(b *testing.B) {
	sqsEvent := queue.SQSEvent{
		DeliveryID: "bench-legacy-1",
		EventType:  "pull_request",
		RepoName:   "bench-repo",
		SenderName: "bench-user",
		Payload:    json.RawMessage(`{"action": "synchronize", "number": 999}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ProcessSQSEvent(sqsEvent)
	}
}
