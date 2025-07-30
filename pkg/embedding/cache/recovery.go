package cache

import (
	"fmt"
	"runtime/debug"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RecoverMiddleware wraps functions with panic recovery
func RecoverMiddleware(logger observability.Logger, operation string) func(func()) {
	return func(fn func()) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered", map[string]interface{}{
					"operation": operation,
					"panic":     r,
					"stack":     string(debug.Stack()),
				})

				// Record metric if metrics client is available
				// Note: We don't have direct access to metrics here,
				// so callers should handle metrics recording
			}
		}()

		fn()
	}
}

// RecoverWithMetrics wraps functions with panic recovery and metrics recording
func RecoverWithMetrics(logger observability.Logger, metrics observability.MetricsClient, operation string) func(func()) {
	return func(fn func()) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered", map[string]interface{}{
					"operation": operation,
					"panic":     r,
					"stack":     string(debug.Stack()),
				})

				// Record metric
				if metrics != nil {
					metrics.IncrementCounterWithLabels("cache.panic_recovered", 1, map[string]string{
						"operation": operation,
					})
				}
			}
		}()

		fn()
	}
}

// SafeExecute executes a function with panic recovery and returns error
func SafeExecute(logger observability.Logger, operation string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic recovered", map[string]interface{}{
				"operation": operation,
				"panic":     r,
				"stack":     string(debug.Stack()),
			})

			// Convert panic to error
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic in %s: %v", operation, r)
			}
		}
	}()

	return fn()
}

// SafeGo runs a goroutine with panic recovery
func SafeGo(logger observability.Logger, operation string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered in goroutine", map[string]interface{}{
					"operation": operation,
					"panic":     r,
					"stack":     string(debug.Stack()),
				})
			}
		}()

		fn()
	}()
}
