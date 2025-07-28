package worker

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// DLQWorker periodically processes the dead letter queue
type DLQWorker struct {
	dlqHandler DLQHandler
	logger     observability.Logger
	interval   time.Duration
}

// NewDLQWorker creates a new DLQ worker
func NewDLQWorker(dlqHandler DLQHandler, logger observability.Logger, interval time.Duration) *DLQWorker {
	if interval == 0 {
		interval = 5 * time.Minute
	}
	return &DLQWorker{
		dlqHandler: dlqHandler,
		logger:     logger,
		interval:   interval,
	}
}

// Run starts the DLQ worker
func (w *DLQWorker) Run(ctx context.Context) error {
	w.logger.Info("Starting DLQ worker", map[string]interface{}{
		"interval": w.interval.String(),
	})

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Process immediately on start
	if err := w.processDLQ(ctx); err != nil {
		w.logger.Error("Failed to process DLQ on startup", map[string]interface{}{
			"error": err.Error(),
		})
	}

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("DLQ worker stopping due to context cancellation", nil)
			return ctx.Err()
		case <-ticker.C:
			if err := w.processDLQ(ctx); err != nil {
				w.logger.Error("Failed to process DLQ", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// processDLQ processes entries in the dead letter queue
func (w *DLQWorker) processDLQ(ctx context.Context) error {
	start := time.Now()

	w.logger.Debug("Processing DLQ entries", nil)

	err := w.dlqHandler.ProcessDLQ(ctx)

	duration := time.Since(start)

	if err != nil {
		w.logger.Error("DLQ processing failed", map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		})
		return err
	}

	w.logger.Debug("DLQ processing completed", map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	})

	return nil
}
