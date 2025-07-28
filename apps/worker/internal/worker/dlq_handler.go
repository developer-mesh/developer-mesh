package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/jmoiron/sqlx"
)

// EventEnqueuer interface for enqueuing events
type EventEnqueuer interface {
	EnqueueEvent(ctx context.Context, event queue.Event) error
}

// DLQHandler handles dead letter queue operations
type DLQHandler interface {
	// SendToDLQ sends a failed event to the dead letter queue
	SendToDLQ(ctx context.Context, event queue.Event, err error) error

	// ProcessDLQ processes events in the dead letter queue
	ProcessDLQ(ctx context.Context) error

	// RetryFromDLQ retries a specific event from the DLQ
	RetryFromDLQ(ctx context.Context, eventID string) error
}

// DLQEntry represents an entry in the dead letter queue
type DLQEntry struct {
	ID           string          `json:"id" db:"id"`
	EventID      string          `json:"event_id" db:"event_id"`
	EventType    string          `json:"event_type" db:"event_type"`
	Payload      json.RawMessage `json:"payload" db:"payload"`
	ErrorMessage string          `json:"error_message" db:"error_message"`
	RetryCount   int             `json:"retry_count" db:"retry_count"`
	LastRetryAt  *time.Time      `json:"last_retry_at,omitempty" db:"last_retry_at"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	Status       string          `json:"status" db:"status"` // pending, retrying, failed, resolved
	Metadata     json.RawMessage `json:"metadata,omitempty" db:"metadata"`
}

// DLQHandlerImpl implements DLQHandler
type DLQHandlerImpl struct {
	db          *sqlx.DB
	logger      observability.Logger
	metrics     observability.MetricsClient
	queueClient EventEnqueuer
	eventRepo   repository.WebhookEventRepository
}

// NewDLQHandler creates a new DLQ handler
func NewDLQHandler(
	db *sqlx.DB,
	logger observability.Logger,
	metrics observability.MetricsClient,
	queueClient EventEnqueuer,
) DLQHandler {
	return &DLQHandlerImpl{
		db:          db,
		logger:      logger,
		metrics:     metrics,
		queueClient: queueClient,
		eventRepo:   repository.NewWebhookEventRepository(db),
	}
}

// SendToDLQ sends a failed event to the dead letter queue
func (d *DLQHandlerImpl) SendToDLQ(ctx context.Context, event queue.Event, err error) error {
	d.logger.Info("Sending event to DLQ", map[string]interface{}{
		"event_id":   event.EventID,
		"event_type": event.EventType,
		"error":      err.Error(),
	})

	// Store in DLQ table
	var metadataJSON json.RawMessage
	if event.Metadata != nil {
		metadataBytes, marshalErr := json.Marshal(event.Metadata)
		if marshalErr != nil {
			d.logger.Warn("Failed to marshal metadata", map[string]interface{}{
				"event_id": event.EventID,
				"error":    marshalErr.Error(),
			})
			metadataJSON = json.RawMessage("{}")
		} else {
			metadataJSON = metadataBytes
		}
	} else {
		metadataJSON = json.RawMessage("{}")
	}

	dlqEntry := &DLQEntry{
		EventID:      event.EventID,
		EventType:    event.EventType,
		Payload:      event.Payload,
		ErrorMessage: err.Error(),
		RetryCount:   0,
		CreatedAt:    time.Now(),
		Status:       "pending",
		Metadata:     metadataJSON,
	}

	query := `
		INSERT INTO webhook_dlq (
			event_id, event_type, payload, error_message, 
			retry_count, created_at, status, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		RETURNING id
	`

	var id string
	err = d.db.QueryRowContext(
		ctx,
		query,
		dlqEntry.EventID,
		dlqEntry.EventType,
		dlqEntry.Payload,
		dlqEntry.ErrorMessage,
		dlqEntry.RetryCount,
		dlqEntry.CreatedAt,
		dlqEntry.Status,
		dlqEntry.Metadata,
	).Scan(&id)

	if err != nil {
		d.logger.Error("Failed to insert DLQ entry", map[string]interface{}{
			"event_id": event.EventID,
			"error":    err.Error(),
		})
		return fmt.Errorf("failed to insert DLQ entry: %w", err)
	}

	// Update webhook event status to 'dlq'
	if d.eventRepo != nil {
		processedAt := time.Now()
		if err := d.eventRepo.UpdateStatus(ctx, event.EventID, "dlq", &processedAt, dlqEntry.ErrorMessage); err != nil {
			d.logger.Warn("Failed to update event status to DLQ", map[string]interface{}{
				"event_id": event.EventID,
				"error":    err.Error(),
			})
		}
	}

	// Record metrics
	d.metrics.IncrementCounterWithLabels("webhook_dlq_entries_total", 1, map[string]string{
		"event_type": event.EventType,
		"status":     "new",
	})

	d.logger.Info("Event sent to DLQ successfully", map[string]interface{}{
		"event_id": event.EventID,
		"dlq_id":   id,
	})

	return nil
}

// ProcessDLQ processes events in the dead letter queue
func (d *DLQHandlerImpl) ProcessDLQ(ctx context.Context) error {
	// Get pending DLQ entries older than 5 minutes
	query := `
		SELECT id, event_id, event_type, payload, error_message, 
		       retry_count, last_retry_at, created_at, status, metadata
		FROM webhook_dlq
		WHERE status = 'pending' 
		  AND created_at < NOW() - INTERVAL '5 minutes'
		  AND retry_count < 3
		ORDER BY created_at ASC
		LIMIT 10
	`

	var entries []DLQEntry
	err := d.db.SelectContext(ctx, &entries, query)
	if err != nil {
		return fmt.Errorf("failed to fetch DLQ entries: %w", err)
	}

	d.logger.Info("Processing DLQ entries", map[string]interface{}{
		"count": len(entries),
	})

	for _, entry := range entries {
		if err := d.retryDLQEntry(ctx, &entry); err != nil {
			d.logger.Error("Failed to retry DLQ entry", map[string]interface{}{
				"dlq_id":   entry.ID,
				"event_id": entry.EventID,
				"error":    err.Error(),
			})
		}
	}

	return nil
}

// RetryFromDLQ retries a specific event from the DLQ
func (d *DLQHandlerImpl) RetryFromDLQ(ctx context.Context, eventID string) error {
	query := `
		SELECT id, event_id, event_type, payload, error_message, 
		       retry_count, last_retry_at, created_at, status, metadata
		FROM webhook_dlq
		WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var entry DLQEntry
	err := d.db.GetContext(ctx, &entry, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to fetch DLQ entry: %w", err)
	}

	return d.retryDLQEntry(ctx, &entry)
}

// retryDLQEntry attempts to retry a DLQ entry
func (d *DLQHandlerImpl) retryDLQEntry(ctx context.Context, entry *DLQEntry) error {
	// Update status to retrying
	updateQuery := `
		UPDATE webhook_dlq 
		SET status = 'retrying', 
		    last_retry_at = NOW(),
		    retry_count = retry_count + 1
		WHERE id = $1
	`

	if _, err := d.db.ExecContext(ctx, updateQuery, entry.ID); err != nil {
		return fmt.Errorf("failed to update DLQ entry status: %w", err)
	}

	// Recreate the event
	var metadata map[string]interface{}
	if len(entry.Metadata) > 0 {
		if err := json.Unmarshal(entry.Metadata, &metadata); err != nil {
			d.logger.Warn("Failed to unmarshal metadata", map[string]interface{}{
				"dlq_id":   entry.ID,
				"event_id": entry.EventID,
				"error":    err.Error(),
			})
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	event := queue.Event{
		EventID:   entry.EventID,
		EventType: entry.EventType,
		Payload:   entry.Payload,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	// Send back to main queue
	if err := d.queueClient.EnqueueEvent(ctx, event); err != nil {
		// Update status back to pending
		d.updateDLQStatus(ctx, entry.ID, "pending")
		return fmt.Errorf("failed to resend event to queue: %w", err)
	}

	// Update status to resolved
	d.updateDLQStatus(ctx, entry.ID, "resolved")

	d.logger.Info("DLQ entry retried successfully", map[string]interface{}{
		"dlq_id":   entry.ID,
		"event_id": entry.EventID,
	})

	d.metrics.IncrementCounterWithLabels("webhook_dlq_retries_total", 1, map[string]string{
		"event_type": entry.EventType,
		"status":     "success",
	})

	return nil
}

// updateDLQStatus updates the status of a DLQ entry
func (d *DLQHandlerImpl) updateDLQStatus(ctx context.Context, id string, status string) {
	query := `UPDATE webhook_dlq SET status = $2 WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, query, id, status); err != nil {
		d.logger.Error("Failed to update DLQ status", map[string]interface{}{
			"dlq_id": id,
			"status": status,
			"error":  err.Error(),
		})
	}
}
