package worker

import (
	"context"
	"runtime"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// PerformanceMonitor collects and reports performance metrics
type PerformanceMonitor struct {
	metrics  *MetricsCollector
	logger   observability.Logger
	interval time.Duration
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(metrics *MetricsCollector, logger observability.Logger, interval time.Duration) *PerformanceMonitor {
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &PerformanceMonitor{
		metrics:  metrics,
		logger:   logger,
		interval: interval,
	}
}

// Run starts the performance monitoring loop
func (p *PerformanceMonitor) Run(ctx context.Context) error {
	p.logger.Info("Starting performance monitor", map[string]interface{}{
		"interval": p.interval.String(),
	})

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Collect metrics immediately on start
	p.collectMetrics()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Performance monitor stopping", nil)
			return ctx.Err()
		case <-ticker.C:
			p.collectMetrics()
		}
	}
}

// collectMetrics collects various performance metrics
func (p *PerformanceMonitor) collectMetrics() {
	// Memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	p.metrics.RecordMemoryUsage(
		memStats.Alloc,      // bytes allocated and still in use
		memStats.TotalAlloc, // bytes allocated (even if freed)
		memStats.Sys,        // bytes obtained from system
	)

	// Goroutine count
	p.metrics.RecordGoroutineCount(runtime.NumGoroutine())

	// GC statistics
	p.recordGCMetrics(&memStats)

	// CPU statistics
	p.recordCPUMetrics()

	p.logger.Debug("Performance metrics collected", map[string]interface{}{
		"memory_alloc_mb":   memStats.Alloc / 1024 / 1024,
		"memory_sys_mb":     memStats.Sys / 1024 / 1024,
		"goroutines":        runtime.NumGoroutine(),
		"gc_runs":           memStats.NumGC,
		"gc_pause_total_ms": float64(memStats.PauseTotalNs) / 1e6,
	})
}

// recordGCMetrics records garbage collection metrics
func (p *PerformanceMonitor) recordGCMetrics(memStats *runtime.MemStats) {
	p.metrics.metrics.RecordGauge("webhook_gc_runs_total", float64(memStats.NumGC), nil)
	p.metrics.metrics.RecordGauge("webhook_gc_pause_total_seconds", float64(memStats.PauseTotalNs)/1e9, nil)

	if memStats.NumGC > 0 {
		// Record the last GC pause duration
		lastPause := memStats.PauseNs[(memStats.NumGC+255)%256]
		p.metrics.metrics.RecordHistogram("webhook_gc_pause_duration_seconds", float64(lastPause)/1e9, nil)
	}
}

// recordCPUMetrics records CPU-related metrics
func (p *PerformanceMonitor) recordCPUMetrics() {
	p.metrics.metrics.RecordGauge("webhook_cpu_cores", float64(runtime.NumCPU()), nil)
	p.metrics.metrics.RecordGauge("webhook_gomaxprocs", float64(runtime.GOMAXPROCS(0)), nil)
}

// GetRuntimeStats returns current runtime statistics
func (p *PerformanceMonitor) GetRuntimeStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"memory": map[string]interface{}{
			"alloc_mb":       memStats.Alloc / 1024 / 1024,
			"total_alloc_mb": memStats.TotalAlloc / 1024 / 1024,
			"sys_mb":         memStats.Sys / 1024 / 1024,
			"heap_alloc_mb":  memStats.HeapAlloc / 1024 / 1024,
			"heap_sys_mb":    memStats.HeapSys / 1024 / 1024,
			"heap_objects":   memStats.HeapObjects,
		},
		"gc": map[string]interface{}{
			"num_gc":         memStats.NumGC,
			"pause_total_ms": float64(memStats.PauseTotalNs) / 1e6,
			"pause_avg_ms":   float64(memStats.PauseTotalNs) / float64(memStats.NumGC) / 1e6,
			"next_gc_mb":     memStats.NextGC / 1024 / 1024,
		},
		"runtime": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"cpu_cores":  runtime.NumCPU(),
			"gomaxprocs": runtime.GOMAXPROCS(0),
			"version":    runtime.Version(),
		},
	}
}
