package clients

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityManager provides comprehensive observability capabilities
type ObservabilityManager struct {
	mu            sync.RWMutex
	logger        observability.Logger
	metricsClient observability.MetricsClient
	tracer        trace.Tracer
	config        ObservabilityConfig

	// Distributed tracing
	tracerProvider *sdktrace.TracerProvider
	propagator     propagation.TextMapPropagator

	// Metrics collection
	metricsCollector *EnhancedMetricsCollector
	customMetrics    map[string]*MetricDefinition

	// Structured logging
	logEnricher *LogEnricher
	logBuffer   *CircularLogBuffer

	// Health monitoring
	healthMonitor *HealthMonitor
	healthChecks  map[string]HealthCheck

	// SLA tracking
	slaTracker     *SLATracker
	slaDefinitions map[string]*SLADefinition

	// Business metrics
	businessMetrics *BusinessMetricsCollector

	// Performance profiling
	profiler *PerformanceProfiler

	// Alerting
	alertManager *ObservabilityAlertManager

	// Dashboards
	dashboardManager *DashboardManager

	// Shutdown management
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// ObservabilityConfig defines observability configuration
type ObservabilityConfig struct {
	// Tracing configuration
	TracingEnabled      bool          `json:"tracing_enabled"`
	TracingEndpoint     string        `json:"tracing_endpoint"`
	TracingSampleRate   float64       `json:"tracing_sample_rate"`
	TracingBatchTimeout time.Duration `json:"tracing_batch_timeout"`

	// Metrics configuration
	MetricsEnabled       bool          `json:"metrics_enabled"`
	MetricsEndpoint      string        `json:"metrics_endpoint"`
	MetricsInterval      time.Duration `json:"metrics_interval"`
	CustomMetricsEnabled bool          `json:"custom_metrics_enabled"`

	// Logging configuration
	StructuredLogging bool   `json:"structured_logging"`
	LogLevel          string `json:"log_level"`
	LogBufferSize     int    `json:"log_buffer_size"`
	LogSampling       bool   `json:"log_sampling"`

	// Health monitoring
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`

	// SLA configuration
	SLAEnabled        bool          `json:"sla_enabled"`
	SLAReportInterval time.Duration `json:"sla_report_interval"`

	// Profiling configuration
	ProfilingEnabled  bool          `json:"profiling_enabled"`
	ProfilingInterval time.Duration `json:"profiling_interval"`

	// Dashboard configuration
	DashboardEnabled bool `json:"dashboard_enabled"`
	DashboardPort    int  `json:"dashboard_port"`
}

// EnhancedMetricsCollector provides advanced metrics collection
type EnhancedMetricsCollector struct {
	mu sync.RWMutex

	// Counters
	counters map[string]*atomic.Int64

	// Gauges
	gauges map[string]*atomic.Value

	// Histograms
	histograms map[string]*Histogram

	// Summaries
	summaries map[string]*Summary

	// Rate metrics
	rates map[string]*RateMetric

	// Labels
	metricLabels map[string]map[string]string
}

// MetricDefinition defines a custom metric
type MetricDefinition struct {
	Name        string
	Type        string // counter, gauge, histogram, summary
	Description string
	Unit        string
	Labels      []string
}

// LogEnricher adds context to log entries
type LogEnricher struct {
	mu            sync.RWMutex
	globalFields  map[string]interface{}
	contextFields map[string]map[string]interface{}
}

// CircularLogBuffer stores recent logs for analysis
type CircularLogBuffer struct {
	mu       sync.RWMutex
	buffer   []LogEntry
	size     int
	position int
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp     time.Time              `json:"timestamp"`
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	Fields        map[string]interface{} `json:"fields"`
	TraceID       string                 `json:"trace_id,omitempty"`
	SpanID        string                 `json:"span_id,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
}

// HealthMonitor monitors system health
type HealthMonitor struct {
	mu         sync.RWMutex
	checks     map[string]*HealthCheckResult
	history    *HealthHistory
	aggregator *HealthAggregator
}

// HealthCheck defines a health check function
type HealthCheck func(ctx context.Context) error

// HealthCheckResult stores health check results
type HealthCheckResult struct {
	Name             string        `json:"name"`
	Status           string        `json:"status"` // healthy, degraded, unhealthy
	LastCheck        time.Time     `json:"last_check"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	Message          string        `json:"message,omitempty"`
	ResponseTime     time.Duration `json:"response_time"`
}

// HealthHistory stores historical health data
type HealthHistory struct {
	mu         sync.RWMutex
	entries    []HealthSnapshot
	maxEntries int
}

// HealthSnapshot represents a point-in-time health status
type HealthSnapshot struct {
	Timestamp     time.Time                     `json:"timestamp"`
	OverallStatus string                        `json:"overall_status"`
	Checks        map[string]*HealthCheckResult `json:"checks"`
}

// HealthAggregator aggregates health metrics
type HealthAggregator struct {
	mu            sync.RWMutex
	availability  float64
	lastDowntime  time.Time
	totalDowntime time.Duration
	startTime     time.Time
}

// SLATracker tracks SLA compliance
type SLATracker struct {
	mu         sync.RWMutex
	objectives map[string]*SLAObjective
	violations []SLAViolation
	reports    []SLAReport
}

// SLADefinition defines an SLA
type SLADefinition struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Objectives      []SLAObjective `json:"objectives"`
	ReportingPeriod time.Duration  `json:"reporting_period"`
}

// SLAObjective defines an SLA objective
type SLAObjective struct {
	Name      string        `json:"name"`
	Metric    string        `json:"metric"`
	Target    float64       `json:"target"`
	Window    time.Duration `json:"window"`
	Current   float64       `json:"current"`
	Compliant bool          `json:"compliant"`
}

// SLAViolation represents an SLA violation
type SLAViolation struct {
	Timestamp time.Time     `json:"timestamp"`
	SLA       string        `json:"sla"`
	Objective string        `json:"objective"`
	Target    float64       `json:"target"`
	Actual    float64       `json:"actual"`
	Duration  time.Duration `json:"duration"`
}

// SLAReport represents an SLA compliance report
type SLAReport struct {
	Period     time.Duration            `json:"period"`
	StartTime  time.Time                `json:"start_time"`
	EndTime    time.Time                `json:"end_time"`
	Objectives map[string]*SLAObjective `json:"objectives"`
	Violations []SLAViolation           `json:"violations"`
	Compliance float64                  `json:"compliance"`
}

// BusinessMetricsCollector collects business-level metrics
type BusinessMetricsCollector struct {
	mu               sync.RWMutex
	requestsByTenant map[string]int64
	requestsByAPI    map[string]int64
	costByTenant     map[string]float64
	revenueByTenant  map[string]float64
	customMetrics    map[string]float64
}

// PerformanceProfiler provides performance profiling
type PerformanceProfiler struct {
	mu                sync.RWMutex
	cpuProfiles       []CPUProfile
	memProfiles       []MemProfile
	goroutineProfiles []GoroutineProfile
	blockProfiles     []BlockProfile
	mutexProfiles     []MutexProfile
}

// CPUProfile stores CPU profiling data
type CPUProfile struct {
	Timestamp time.Time
	Duration  time.Duration
	Samples   []ProfileSample
}

// MemProfile stores memory profiling data
type MemProfile struct {
	Timestamp  time.Time
	HeapAlloc  uint64
	HeapInUse  uint64
	StackInUse uint64
	NumGC      uint32
	GCPauseNs  []uint64
}

// GoroutineProfile stores goroutine profiling data
type GoroutineProfile struct {
	Timestamp time.Time
	Count     int
	Stacks    []string
}

// BlockProfile stores blocking profiling data
type BlockProfile struct {
	Timestamp time.Time
	Events    []BlockEvent
}

// MutexProfile stores mutex profiling data
type MutexProfile struct {
	Timestamp time.Time
	Events    []MutexEvent
}

// ProfileSample represents a profiling sample
type ProfileSample struct {
	Function string
	Line     int
	Value    int64
}

// BlockEvent represents a blocking event
type BlockEvent struct {
	Duration time.Duration
	Stack    string
}

// MutexEvent represents a mutex event
type MutexEvent struct {
	Duration time.Duration
	Stack    string
}

// ObservabilityAlertManager manages observability alerts
type ObservabilityAlertManager struct {
	mu           sync.RWMutex
	rules        map[string]*AlertRule
	activeAlerts map[string]*ObservabilityAlert
	history      []ObservabilityAlert
	channels     []AlertChannel
}

// AlertRule defines an alert rule
type AlertRule struct {
	Name      string        `json:"name"`
	Condition string        `json:"condition"`
	Threshold float64       `json:"threshold"`
	Duration  time.Duration `json:"duration"`
	Severity  string        `json:"severity"`
	Actions   []string      `json:"actions"`
}

// ObservabilityAlert represents an active alert
type ObservabilityAlert struct {
	ID          string     `json:"id"`
	Rule        string     `json:"rule"`
	Severity    string     `json:"severity"`
	Value       float64    `json:"value"`
	Threshold   float64    `json:"threshold"`
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Message     string     `json:"message"`
}

// AlertChannel represents an alert notification channel
type AlertChannel interface {
	Send(alert ObservabilityAlert) error
}

// DashboardManager manages observability dashboards
type DashboardManager struct {
	mu         sync.RWMutex
	dashboards map[string]*Dashboard
	widgets    map[string]*Widget
}

// Dashboard represents an observability dashboard
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Widgets     []*Widget `json:"widgets"`
	Layout      string    `json:"layout"`
}

// Widget represents a dashboard widget
type Widget struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Query       string                 `json:"query"`
	Config      map[string]interface{} `json:"config"`
	RefreshRate time.Duration          `json:"refresh_rate"`
}

// Histogram tracks value distributions
type Histogram struct {
	mu      sync.RWMutex
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
}

// Summary provides statistical summaries
type Summary struct {
	// Reserved for future implementation
}

// RateMetric tracks rate over time
type RateMetric struct {
	// Reserved for future implementation
}

// DefaultObservabilityConfig returns default configuration
func DefaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		TracingEnabled:       true,
		TracingEndpoint:      "localhost:4317",
		TracingSampleRate:    0.1,
		TracingBatchTimeout:  5 * time.Second,
		MetricsEnabled:       true,
		MetricsEndpoint:      "localhost:9090",
		MetricsInterval:      30 * time.Second,
		CustomMetricsEnabled: true,
		StructuredLogging:    true,
		LogLevel:             "info",
		LogBufferSize:        10000,
		LogSampling:          true,
		HealthCheckInterval:  30 * time.Second,
		HealthCheckTimeout:   5 * time.Second,
		SLAEnabled:           true,
		SLAReportInterval:    1 * time.Hour,
		ProfilingEnabled:     false,
		ProfilingInterval:    5 * time.Minute,
		DashboardEnabled:     true,
		DashboardPort:        8090,
	}
}

// NewObservabilityManager creates a new observability manager
func NewObservabilityManager(
	config ObservabilityConfig,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
) (*ObservabilityManager, error) {
	manager := &ObservabilityManager{
		logger:         logger,
		metricsClient:  metricsClient,
		config:         config,
		customMetrics:  make(map[string]*MetricDefinition),
		healthChecks:   make(map[string]HealthCheck),
		slaDefinitions: make(map[string]*SLADefinition),
		shutdown:       make(chan struct{}),
	}

	// Initialize distributed tracing
	if config.TracingEnabled {
		if err := manager.initializeTracing(); err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
	}

	// Initialize metrics collector
	manager.metricsCollector = &EnhancedMetricsCollector{
		counters:     make(map[string]*atomic.Int64),
		gauges:       make(map[string]*atomic.Value),
		histograms:   make(map[string]*Histogram),
		summaries:    make(map[string]*Summary),
		rates:        make(map[string]*RateMetric),
		metricLabels: make(map[string]map[string]string),
	}

	// Initialize log enricher
	manager.logEnricher = &LogEnricher{
		globalFields:  make(map[string]interface{}),
		contextFields: make(map[string]map[string]interface{}),
	}

	// Initialize log buffer
	manager.logBuffer = &CircularLogBuffer{
		buffer: make([]LogEntry, config.LogBufferSize),
		size:   config.LogBufferSize,
	}

	// Initialize health monitor
	manager.healthMonitor = &HealthMonitor{
		checks: make(map[string]*HealthCheckResult),
		history: &HealthHistory{
			entries:    make([]HealthSnapshot, 0, 1000),
			maxEntries: 1000,
		},
		aggregator: &HealthAggregator{
			startTime:    time.Now(),
			availability: 100.0,
		},
	}

	// Initialize SLA tracker
	if config.SLAEnabled {
		manager.slaTracker = &SLATracker{
			objectives: make(map[string]*SLAObjective),
			violations: make([]SLAViolation, 0, 100),
			reports:    make([]SLAReport, 0, 10),
		}
	}

	// Initialize business metrics
	manager.businessMetrics = &BusinessMetricsCollector{
		requestsByTenant: make(map[string]int64),
		requestsByAPI:    make(map[string]int64),
		costByTenant:     make(map[string]float64),
		revenueByTenant:  make(map[string]float64),
		customMetrics:    make(map[string]float64),
	}

	// Initialize profiler
	if config.ProfilingEnabled {
		manager.profiler = &PerformanceProfiler{
			cpuProfiles:       make([]CPUProfile, 0, 10),
			memProfiles:       make([]MemProfile, 0, 10),
			goroutineProfiles: make([]GoroutineProfile, 0, 10),
			blockProfiles:     make([]BlockProfile, 0, 10),
			mutexProfiles:     make([]MutexProfile, 0, 10),
		}
	}

	// Initialize alert manager
	manager.alertManager = &ObservabilityAlertManager{
		rules:        make(map[string]*AlertRule),
		activeAlerts: make(map[string]*ObservabilityAlert),
		history:      make([]ObservabilityAlert, 0, 100),
		channels:     make([]AlertChannel, 0),
	}

	// Initialize dashboard manager
	if config.DashboardEnabled {
		manager.dashboardManager = &DashboardManager{
			dashboards: make(map[string]*Dashboard),
			widgets:    make(map[string]*Widget),
		}
		manager.setupDefaultDashboards()
	}

	// Register default health checks
	manager.registerDefaultHealthChecks()

	// Define default SLAs
	if config.SLAEnabled {
		manager.defineDefaultSLAs()
	}

	// Start background workers
	manager.startWorkers()

	return manager, nil
}

// initializeTracing sets up distributed tracing
func (m *ObservabilityManager) initializeTracing() error {
	ctx := context.Background()

	// Create OTLP exporter
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(m.config.TracingEndpoint),
		otlptracegrpc.WithInsecure(),
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	m.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(m.config.TracingBatchTimeout),
		),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("mcp-server"),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", "production"),
		)),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(m.config.TracingSampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(m.tracerProvider)

	// Set global propagator
	m.propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(m.propagator)

	// Create tracer
	m.tracer = m.tracerProvider.Tracer("mcp-server")

	return nil
}

// StartSpan starts a new span with enhanced context
func (m *ObservabilityManager) StartSpan(
	ctx context.Context,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	// Extract correlation ID
	correlationID := extractCorrelationID(ctx)
	if correlationID == "" {
		correlationID = uuid.New().String()
		ctx = context.WithValue(ctx, ContextKeyCorrelationID, correlationID)
	}

	// Add default attributes
	defaultAttrs := []attribute.KeyValue{
		attribute.String("correlation_id", correlationID),
		attribute.String("service.name", "mcp-server"),
	}

	// Extract tenant and user IDs if available
	if tenantID := extractTenantID(ctx); tenantID != "" {
		defaultAttrs = append(defaultAttrs, attribute.String("tenant_id", tenantID))
	}
	if userID := extractUserID(ctx); userID != "" {
		defaultAttrs = append(defaultAttrs, attribute.String("user_id", userID))
	}

	// Start span with attributes
	ctx, span := m.tracer.Start(ctx, name,
		append(opts, trace.WithAttributes(defaultAttrs...))...,
	)

	// Log span start
	m.logWithContext(ctx, "info", fmt.Sprintf("Started span: %s", name), map[string]interface{}{
		"span_id":  span.SpanContext().SpanID().String(),
		"trace_id": span.SpanContext().TraceID().String(),
	})

	// Record metric
	m.metricsClient.IncrementCounter("spans_started", 1.0)

	return ctx, span
}

// RecordError records an error with context
func (m *ObservabilityManager) RecordError(ctx context.Context, err error, message string) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, message)
	}

	// Log error with context
	m.logWithContext(ctx, "error", message, map[string]interface{}{
		"error": err.Error(),
	})

	// Record error metric
	m.metricsClient.IncrementCounter("errors", 1.0)
}

// logWithContext logs with trace and correlation context
func (m *ObservabilityManager) logWithContext(
	ctx context.Context,
	level string,
	message string,
	fields map[string]interface{},
) {
	// Add trace context
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		fields["trace_id"] = span.SpanContext().TraceID().String()
		fields["span_id"] = span.SpanContext().SpanID().String()
	}

	// Add correlation ID
	if correlationID := extractCorrelationID(ctx); correlationID != "" {
		fields["correlation_id"] = correlationID
	}

	// Add enriched fields
	enrichedFields := m.logEnricher.enrich(ctx, fields)

	// Log based on level
	switch level {
	case "debug":
		m.logger.Debug(message, enrichedFields)
	case "info":
		m.logger.Info(message, enrichedFields)
	case "warn":
		m.logger.Warn(message, enrichedFields)
	case "error":
		m.logger.Error(message, enrichedFields)
	default:
		m.logger.Info(message, enrichedFields)
	}

	// Store in buffer
	m.logBuffer.add(LogEntry{
		Timestamp:     time.Now(),
		Level:         level,
		Message:       message,
		Fields:        enrichedFields,
		TraceID:       fields["trace_id"].(string),
		SpanID:        fields["span_id"].(string),
		CorrelationID: fields["correlation_id"].(string),
	})
}

// enrich adds context to log fields
func (e *LogEnricher) enrich(ctx context.Context, fields map[string]interface{}) map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Start with provided fields
	enriched := make(map[string]interface{})
	for k, v := range fields {
		enriched[k] = v
	}

	// Add global fields
	for k, v := range e.globalFields {
		if _, exists := enriched[k]; !exists {
			enriched[k] = v
		}
	}

	// Add context-specific fields
	if contextID := fmt.Sprintf("%p", ctx); contextID != "" {
		if contextFields, exists := e.contextFields[contextID]; exists {
			for k, v := range contextFields {
				if _, exists := enriched[k]; !exists {
					enriched[k] = v
				}
			}
		}
	}

	// Add system context
	enriched["hostname"] = getHostname()
	enriched["timestamp"] = time.Now().Format(time.RFC3339Nano)

	return enriched
}

// add adds a log entry to the circular buffer
func (b *CircularLogBuffer) add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer[b.position] = entry
	b.position = (b.position + 1) % b.size
}

// RegisterHealthCheck registers a health check
func (m *ObservabilityManager) RegisterHealthCheck(name string, check HealthCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.healthChecks[name] = check
	m.logger.Info("Registered health check", map[string]interface{}{
		"name": name,
	})
}

// registerDefaultHealthChecks registers default health checks
func (m *ObservabilityManager) registerDefaultHealthChecks() {
	// Database health check
	m.RegisterHealthCheck("database", func(ctx context.Context) error {
		// Implementation would check database connectivity
		return nil
	})

	// Redis health check
	m.RegisterHealthCheck("redis", func(ctx context.Context) error {
		// Implementation would check Redis connectivity
		return nil
	})

	// Memory health check
	m.RegisterHealthCheck("memory", func(ctx context.Context) error {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// Check if memory usage is too high
		if memStats.Alloc > 1024*1024*1024 { // 1GB
			return fmt.Errorf("high memory usage: %d MB", memStats.Alloc/1024/1024)
		}
		return nil
	})
}

// DefineSLA defines an SLA
func (m *ObservabilityManager) DefineSLA(definition *SLADefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.slaDefinitions[definition.Name] = definition

	// Initialize objectives
	for _, objective := range definition.Objectives {
		m.slaTracker.objectives[objective.Name] = &objective
	}

	m.logger.Info("Defined SLA", map[string]interface{}{
		"name":       definition.Name,
		"objectives": len(definition.Objectives),
	})
}

// defineDefaultSLAs defines default SLAs
func (m *ObservabilityManager) defineDefaultSLAs() {
	// API availability SLA
	m.DefineSLA(&SLADefinition{
		Name:        "api_availability",
		Description: "API availability SLA",
		Objectives: []SLAObjective{
			{
				Name:   "uptime",
				Metric: "availability",
				Target: 99.9,
				Window: 24 * time.Hour,
			},
		},
		ReportingPeriod: 24 * time.Hour,
	})

	// Response time SLA
	m.DefineSLA(&SLADefinition{
		Name:        "response_time",
		Description: "Response time SLA",
		Objectives: []SLAObjective{
			{
				Name:   "p95_latency",
				Metric: "p95_response_time",
				Target: 500, // 500ms
				Window: 1 * time.Hour,
			},
			{
				Name:   "p99_latency",
				Metric: "p99_response_time",
				Target: 1000, // 1s
				Window: 1 * time.Hour,
			},
		},
		ReportingPeriod: 1 * time.Hour,
	})

	// Error rate SLA
	m.DefineSLA(&SLADefinition{
		Name:        "error_rate",
		Description: "Error rate SLA",
		Objectives: []SLAObjective{
			{
				Name:   "error_percentage",
				Metric: "error_rate",
				Target: 1.0, // 1%
				Window: 1 * time.Hour,
			},
		},
		ReportingPeriod: 1 * time.Hour,
	})
}

// RecordBusinessMetric records a business metric
func (m *ObservabilityManager) RecordBusinessMetric(
	ctx context.Context,
	metric string,
	value float64,
	labels map[string]string,
) {
	m.businessMetrics.mu.Lock()
	defer m.businessMetrics.mu.Unlock()

	// Extract tenant ID
	tenantID := extractTenantID(ctx)

	switch metric {
	case "api_request":
		m.businessMetrics.requestsByTenant[tenantID]++
		if api, ok := labels["api"]; ok {
			m.businessMetrics.requestsByAPI[api]++
		}
	case "cost":
		m.businessMetrics.costByTenant[tenantID] += value
	case "revenue":
		m.businessMetrics.revenueByTenant[tenantID] += value
	default:
		m.businessMetrics.customMetrics[metric] = value
	}

	// Record in metrics system
	m.metricsClient.RecordGauge(metric, value, labels)
}

// setupDefaultDashboards sets up default dashboards
func (m *ObservabilityManager) setupDefaultDashboards() {
	// System overview dashboard
	systemDashboard := &Dashboard{
		ID:          "system-overview",
		Name:        "System Overview",
		Description: "Overall system health and performance",
		Layout:      "grid",
		Widgets: []*Widget{
			{
				ID:          "health-status",
				Type:        "health",
				Title:       "Health Status",
				Query:       "health.overall",
				RefreshRate: 30 * time.Second,
			},
			{
				ID:          "request-rate",
				Type:        "line",
				Title:       "Request Rate",
				Query:       "rate(requests_total[5m])",
				RefreshRate: 10 * time.Second,
			},
			{
				ID:          "error-rate",
				Type:        "line",
				Title:       "Error Rate",
				Query:       "rate(errors_total[5m])",
				RefreshRate: 10 * time.Second,
			},
			{
				ID:          "response-time",
				Type:        "histogram",
				Title:       "Response Time Distribution",
				Query:       "histogram_quantile(0.95, response_time_bucket)",
				RefreshRate: 30 * time.Second,
			},
		},
	}

	m.dashboardManager.dashboards[systemDashboard.ID] = systemDashboard

	// SLA compliance dashboard
	slaDashboard := &Dashboard{
		ID:          "sla-compliance",
		Name:        "SLA Compliance",
		Description: "SLA objectives and compliance tracking",
		Layout:      "grid",
		Widgets: []*Widget{
			{
				ID:          "sla-status",
				Type:        "table",
				Title:       "SLA Status",
				Query:       "sla.objectives",
				RefreshRate: 1 * time.Minute,
			},
			{
				ID:    "availability",
				Type:  "gauge",
				Title: "Availability",
				Query: "availability",
				Config: map[string]interface{}{
					"target": 99.9,
					"unit":   "%",
				},
				RefreshRate: 1 * time.Minute,
			},
		},
	}

	m.dashboardManager.dashboards[slaDashboard.ID] = slaDashboard

	// Business metrics dashboard
	businessDashboard := &Dashboard{
		ID:          "business-metrics",
		Name:        "Business Metrics",
		Description: "Business KPIs and usage metrics",
		Layout:      "grid",
		Widgets: []*Widget{
			{
				ID:          "requests-by-tenant",
				Type:        "bar",
				Title:       "Requests by Tenant",
				Query:       "sum by (tenant_id) (requests_total)",
				RefreshRate: 5 * time.Minute,
			},
			{
				ID:          "api-usage",
				Type:        "pie",
				Title:       "API Usage Distribution",
				Query:       "sum by (api) (api_requests_total)",
				RefreshRate: 5 * time.Minute,
			},
		},
	}

	m.dashboardManager.dashboards[businessDashboard.ID] = businessDashboard
}

// startWorkers starts background workers
func (m *ObservabilityManager) startWorkers() {
	// Health check worker
	m.wg.Add(1)
	go m.healthCheckWorker()

	// Metrics collection worker
	if m.config.MetricsEnabled {
		m.wg.Add(1)
		go m.metricsCollectionWorker()
	}

	// SLA tracking worker
	if m.config.SLAEnabled {
		m.wg.Add(1)
		go m.slaTrackingWorker()
	}

	// Profiling worker
	if m.config.ProfilingEnabled {
		m.wg.Add(1)
		go m.profilingWorker()
	}
}

// healthCheckWorker performs periodic health checks
func (m *ObservabilityManager) healthCheckWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performHealthChecks()
		case <-m.shutdown:
			return
		}
	}
}

// performHealthChecks executes all health checks
func (m *ObservabilityManager) performHealthChecks() {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.HealthCheckTimeout)
	defer cancel()

	m.mu.RLock()
	checks := make(map[string]HealthCheck)
	for name, check := range m.healthChecks {
		checks[name] = check
	}
	m.mu.RUnlock()

	results := make(map[string]*HealthCheckResult)
	overallStatus := "healthy"

	for name, check := range checks {
		start := time.Now()
		err := check(ctx)
		responseTime := time.Since(start)

		result := &HealthCheckResult{
			Name:         name,
			LastCheck:    time.Now(),
			ResponseTime: responseTime,
		}

		if err != nil {
			result.Status = "unhealthy"
			result.Message = err.Error()
			overallStatus = "unhealthy"

			// Update consecutive fails
			if prev, exists := m.healthMonitor.checks[name]; exists {
				result.ConsecutiveFails = prev.ConsecutiveFails + 1
			} else {
				result.ConsecutiveFails = 1
			}
		} else {
			result.Status = "healthy"
			result.ConsecutiveFails = 0
		}

		results[name] = result
		m.healthMonitor.checks[name] = result

		// Record metrics
		statusValue := 1.0
		if result.Status != "healthy" {
			statusValue = 0.0
		}
		m.metricsClient.RecordGauge(
			fmt.Sprintf("health.%s", name),
			statusValue,
			map[string]string{"status": result.Status},
		)
	}

	// Store snapshot
	snapshot := HealthSnapshot{
		Timestamp:     time.Now(),
		OverallStatus: overallStatus,
		Checks:        results,
	}

	m.healthMonitor.history.mu.Lock()
	m.healthMonitor.history.entries = append(m.healthMonitor.history.entries, snapshot)
	if len(m.healthMonitor.history.entries) > m.healthMonitor.history.maxEntries {
		m.healthMonitor.history.entries = m.healthMonitor.history.entries[1:]
	}
	m.healthMonitor.history.mu.Unlock()

	// Update availability
	m.updateAvailability(overallStatus)
}

// updateAvailability updates system availability metrics
func (m *ObservabilityManager) updateAvailability(status string) {
	m.healthMonitor.aggregator.mu.Lock()
	defer m.healthMonitor.aggregator.mu.Unlock()

	now := time.Now()
	uptime := now.Sub(m.healthMonitor.aggregator.startTime)

	if status != "healthy" {
		// System is down
		if m.healthMonitor.aggregator.lastDowntime.IsZero() {
			m.healthMonitor.aggregator.lastDowntime = now
		}
	} else {
		// System is up
		if !m.healthMonitor.aggregator.lastDowntime.IsZero() {
			downtime := now.Sub(m.healthMonitor.aggregator.lastDowntime)
			m.healthMonitor.aggregator.totalDowntime += downtime
			m.healthMonitor.aggregator.lastDowntime = time.Time{}
		}
	}

	// Calculate availability
	if uptime > 0 {
		availableTime := uptime - m.healthMonitor.aggregator.totalDowntime
		m.healthMonitor.aggregator.availability = (float64(availableTime) / float64(uptime)) * 100
	}

	// Record metric
	m.metricsClient.RecordGauge("availability", m.healthMonitor.aggregator.availability, nil)
}

// metricsCollectionWorker collects and exports metrics
func (m *ObservabilityManager) metricsCollectionWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectAndExportMetrics()
		case <-m.shutdown:
			return
		}
	}
}

// collectAndExportMetrics collects and exports all metrics
func (m *ObservabilityManager) collectAndExportMetrics() {
	// Collect system metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.metricsClient.RecordGauge("memory.alloc", float64(memStats.Alloc), nil)
	m.metricsClient.RecordGauge("memory.total_alloc", float64(memStats.TotalAlloc), nil)
	m.metricsClient.RecordGauge("memory.sys", float64(memStats.Sys), nil)
	m.metricsClient.RecordGauge("memory.num_gc", float64(memStats.NumGC), nil)
	m.metricsClient.RecordGauge("goroutines", float64(runtime.NumGoroutine()), nil)

	// Export custom metrics
	m.metricsCollector.mu.RLock()
	defer m.metricsCollector.mu.RUnlock()

	// Export counters
	for name, counter := range m.metricsCollector.counters {
		value := counter.Load()
		labels := m.metricsCollector.metricLabels[name]
		m.metricsClient.RecordCounter(name, float64(value), labels)
	}

	// Export gauges
	for name, gauge := range m.metricsCollector.gauges {
		if value := gauge.Load(); value != nil {
			if v, ok := value.(float64); ok {
				labels := m.metricsCollector.metricLabels[name]
				m.metricsClient.RecordGauge(name, v, labels)
			}
		}
	}
}

// slaTrackingWorker tracks SLA compliance
func (m *ObservabilityManager) slaTrackingWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.SLAReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.evaluateSLAs()
		case <-m.shutdown:
			return
		}
	}
}

// evaluateSLAs evaluates SLA compliance
func (m *ObservabilityManager) evaluateSLAs() {
	m.mu.RLock()
	definitions := make([]*SLADefinition, 0, len(m.slaDefinitions))
	for _, def := range m.slaDefinitions {
		definitions = append(definitions, def)
	}
	m.mu.RUnlock()

	for _, definition := range definitions {
		report := m.evaluateSLA(definition)

		m.slaTracker.mu.Lock()
		m.slaTracker.reports = append(m.slaTracker.reports, report)
		if len(m.slaTracker.reports) > 100 {
			m.slaTracker.reports = m.slaTracker.reports[1:]
		}
		m.slaTracker.mu.Unlock()

		// Check for violations
		for _, objective := range report.Objectives {
			if !objective.Compliant {
				violation := SLAViolation{
					Timestamp: time.Now(),
					SLA:       definition.Name,
					Objective: objective.Name,
					Target:    objective.Target,
					Actual:    objective.Current,
				}

				m.slaTracker.mu.Lock()
				m.slaTracker.violations = append(m.slaTracker.violations, violation)
				m.slaTracker.mu.Unlock()

				// Trigger alert
				m.triggerSLAAlert(definition, objective, violation)
			}
		}
	}
}

// evaluateSLA evaluates a single SLA
func (m *ObservabilityManager) evaluateSLA(definition *SLADefinition) SLAReport {
	report := SLAReport{
		Period:     definition.ReportingPeriod,
		StartTime:  time.Now().Add(-definition.ReportingPeriod),
		EndTime:    time.Now(),
		Objectives: make(map[string]*SLAObjective),
	}

	totalObjectives := 0
	compliantObjectives := 0

	for _, objective := range definition.Objectives {
		// Get metric value (would query from metrics system)
		value := m.getSLAMetricValue(objective.Metric, objective.Window)

		compliant := false
		switch objective.Metric {
		case "availability", "uptime":
			compliant = value >= objective.Target
		case "error_rate", "p95_response_time", "p99_response_time":
			compliant = value <= objective.Target
		}

		objCopy := objective
		objCopy.Current = value
		objCopy.Compliant = compliant

		report.Objectives[objective.Name] = &objCopy

		totalObjectives++
		if compliant {
			compliantObjectives++
		}
	}

	// Calculate overall compliance
	if totalObjectives > 0 {
		report.Compliance = (float64(compliantObjectives) / float64(totalObjectives)) * 100
	}

	return report
}

// getSLAMetricValue gets the value of an SLA metric
func (m *ObservabilityManager) getSLAMetricValue(metric string, window time.Duration) float64 {
	// This would query from the actual metrics system
	// For now, return mock values
	switch metric {
	case "availability":
		m.healthMonitor.aggregator.mu.RLock()
		defer m.healthMonitor.aggregator.mu.RUnlock()
		return m.healthMonitor.aggregator.availability
	case "error_rate":
		return 0.5 // Mock 0.5% error rate
	case "p95_response_time":
		return 450 // Mock 450ms
	case "p99_response_time":
		return 800 // Mock 800ms
	default:
		return 0
	}
}

// triggerSLAAlert triggers an alert for SLA violation
func (m *ObservabilityManager) triggerSLAAlert(
	definition *SLADefinition,
	objective *SLAObjective,
	violation SLAViolation,
) {
	alert := ObservabilityAlert{
		ID:          uuid.New().String(),
		Rule:        fmt.Sprintf("sla_%s_%s", definition.Name, objective.Name),
		Severity:    "warning",
		Value:       objective.Current,
		Threshold:   objective.Target,
		TriggeredAt: time.Now(),
		Message: fmt.Sprintf(
			"SLA violation: %s/%s - Current: %.2f, Target: %.2f",
			definition.Name, objective.Name, objective.Current, objective.Target,
		),
	}

	m.alertManager.mu.Lock()
	m.alertManager.activeAlerts[alert.Rule] = &alert
	m.alertManager.history = append(m.alertManager.history, alert)
	m.alertManager.mu.Unlock()

	// Send to alert channels
	for _, channel := range m.alertManager.channels {
		go func(ch AlertChannel) {
			if err := ch.Send(alert); err != nil {
				m.logger.Warn("Failed to send alert", map[string]interface{}{
					"error": err.Error(),
					"alert": alert.Rule,
				})
			}
		}(channel)
	}

	m.logger.Warn("SLA violation detected", map[string]interface{}{
		"sla":       definition.Name,
		"objective": objective.Name,
		"target":    objective.Target,
		"actual":    objective.Current,
	})
}

// profilingWorker performs periodic profiling
func (m *ObservabilityManager) profilingWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.ProfilingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectProfiles()
		case <-m.shutdown:
			return
		}
	}
}

// collectProfiles collects performance profiles
func (m *ObservabilityManager) collectProfiles() {
	// Collect memory profile
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	memProfile := MemProfile{
		Timestamp:  time.Now(),
		HeapAlloc:  memStats.HeapAlloc,
		HeapInUse:  memStats.HeapInuse,
		StackInUse: memStats.StackInuse,
		NumGC:      memStats.NumGC,
		GCPauseNs:  make([]uint64, len(memStats.PauseNs)),
	}
	copy(memProfile.GCPauseNs, memStats.PauseNs[:])

	m.profiler.mu.Lock()
	m.profiler.memProfiles = append(m.profiler.memProfiles, memProfile)
	if len(m.profiler.memProfiles) > 10 {
		m.profiler.memProfiles = m.profiler.memProfiles[1:]
	}

	// Collect goroutine profile
	goroutineProfile := GoroutineProfile{
		Timestamp: time.Now(),
		Count:     runtime.NumGoroutine(),
	}
	m.profiler.goroutineProfiles = append(m.profiler.goroutineProfiles, goroutineProfile)
	if len(m.profiler.goroutineProfiles) > 10 {
		m.profiler.goroutineProfiles = m.profiler.goroutineProfiles[1:]
	}
	m.profiler.mu.Unlock()
}

// GetObservabilityMetrics returns comprehensive observability metrics
func (m *ObservabilityManager) GetObservabilityMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	// Health status
	m.healthMonitor.mu.RLock()
	healthStatus := make(map[string]interface{})
	for name, check := range m.healthMonitor.checks {
		healthStatus[name] = map[string]interface{}{
			"status":            check.Status,
			"last_check":        check.LastCheck,
			"consecutive_fails": check.ConsecutiveFails,
			"response_time":     check.ResponseTime.String(),
		}
	}
	m.healthMonitor.mu.RUnlock()
	metrics["health"] = healthStatus

	// Availability
	m.healthMonitor.aggregator.mu.RLock()
	metrics["availability"] = m.healthMonitor.aggregator.availability
	metrics["total_downtime"] = m.healthMonitor.aggregator.totalDowntime.String()
	m.healthMonitor.aggregator.mu.RUnlock()

	// SLA compliance
	if m.slaTracker != nil {
		m.slaTracker.mu.RLock()
		slaMetrics := make(map[string]interface{})
		for name, objective := range m.slaTracker.objectives {
			slaMetrics[name] = map[string]interface{}{
				"target":    objective.Target,
				"current":   objective.Current,
				"compliant": objective.Compliant,
			}
		}
		metrics["sla_compliance"] = slaMetrics
		metrics["sla_violations"] = len(m.slaTracker.violations)
		m.slaTracker.mu.RUnlock()
	}

	// Business metrics
	m.businessMetrics.mu.RLock()
	metrics["business"] = map[string]interface{}{
		"requests_by_tenant": m.businessMetrics.requestsByTenant,
		"requests_by_api":    m.businessMetrics.requestsByAPI,
		"cost_by_tenant":     m.businessMetrics.costByTenant,
		"revenue_by_tenant":  m.businessMetrics.revenueByTenant,
	}
	m.businessMetrics.mu.RUnlock()

	// Active alerts
	m.alertManager.mu.RLock()
	activeAlerts := make([]map[string]interface{}, 0)
	for _, alert := range m.alertManager.activeAlerts {
		activeAlerts = append(activeAlerts, map[string]interface{}{
			"id":        alert.ID,
			"rule":      alert.Rule,
			"severity":  alert.Severity,
			"value":     alert.Value,
			"threshold": alert.Threshold,
			"triggered": alert.TriggeredAt,
			"message":   alert.Message,
		})
	}
	metrics["active_alerts"] = activeAlerts
	m.alertManager.mu.RUnlock()

	// Profiling data
	if m.profiler != nil {
		m.profiler.mu.RLock()
		if len(m.profiler.memProfiles) > 0 {
			latest := m.profiler.memProfiles[len(m.profiler.memProfiles)-1]
			metrics["memory_profile"] = map[string]interface{}{
				"heap_alloc":   latest.HeapAlloc,
				"heap_in_use":  latest.HeapInUse,
				"stack_in_use": latest.StackInUse,
				"num_gc":       latest.NumGC,
			}
		}
		if len(m.profiler.goroutineProfiles) > 0 {
			latest := m.profiler.goroutineProfiles[len(m.profiler.goroutineProfiles)-1]
			metrics["goroutines"] = latest.Count
		}
		m.profiler.mu.RUnlock()
	}

	return metrics
}

// GetDashboard returns a dashboard by ID
func (m *ObservabilityManager) GetDashboard(id string) (*Dashboard, error) {
	if m.dashboardManager == nil {
		return nil, fmt.Errorf("dashboards not enabled")
	}

	m.dashboardManager.mu.RLock()
	defer m.dashboardManager.mu.RUnlock()

	dashboard, exists := m.dashboardManager.dashboards[id]
	if !exists {
		return nil, fmt.Errorf("dashboard not found: %s", id)
	}

	return dashboard, nil
}

// ExportMetrics exports metrics in Prometheus format
func (m *ObservabilityManager) ExportMetrics() string {
	// This would export metrics in Prometheus format
	// Implementation would format all metrics appropriately
	return ""
}

// Close shuts down the observability manager
func (m *ObservabilityManager) Close() error {
	close(m.shutdown)
	m.wg.Wait()

	// Shutdown tracer provider
	if m.tracerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown tracer provider: %w", err)
		}
	}

	return nil
}

// Helper function to get hostname
func getHostname() string {
	hostname, err := Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// Hostname returns the system hostname
func Hostname() (string, error) {
	// This would be implemented to get actual hostname
	return "mcp-server-001", nil
}

// RecordSpanMetric records metrics for a span
func (m *ObservabilityManager) RecordSpanMetric(
	ctx context.Context,
	metricName string,
	value float64,
	unit string,
) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(
			attribute.Float64(metricName, value),
			attribute.String(fmt.Sprintf("%s.unit", metricName), unit),
		)
	}

	// Also record in metrics system
	labels := map[string]string{"unit": unit}
	m.metricsClient.RecordGauge(metricName, value, labels)
}

// IncrementCounter increments a counter metric
func (m *ObservabilityManager) IncrementCounter(name string, value int64, labels map[string]string) {
	m.metricsCollector.mu.Lock()
	defer m.metricsCollector.mu.Unlock()

	counter, exists := m.metricsCollector.counters[name]
	if !exists {
		counter = &atomic.Int64{}
		m.metricsCollector.counters[name] = counter
	}

	counter.Add(value)
	m.metricsCollector.metricLabels[name] = labels
}

// SetGauge sets a gauge metric
func (m *ObservabilityManager) SetGauge(name string, value float64, labels map[string]string) {
	m.metricsCollector.mu.Lock()
	defer m.metricsCollector.mu.Unlock()

	gauge, exists := m.metricsCollector.gauges[name]
	if !exists {
		gauge = &atomic.Value{}
		m.metricsCollector.gauges[name] = gauge
	}

	gauge.Store(value)
	m.metricsCollector.metricLabels[name] = labels
}

// RecordHistogram records a histogram value
func (m *ObservabilityManager) RecordHistogram(name string, value float64, labels map[string]string) {
	m.metricsCollector.mu.Lock()
	defer m.metricsCollector.mu.Unlock()

	histogram, exists := m.metricsCollector.histograms[name]
	if !exists {
		histogram = &Histogram{
			buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10},
			counts:  make([]int64, 8), // 7 buckets + infinity
		}
		m.metricsCollector.histograms[name] = histogram
	}

	histogram.observe(value)
	m.metricsCollector.metricLabels[name] = labels
}

// observe adds a value to the histogram
func (h *Histogram) observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++

	// Find the right bucket
	for i, boundary := range h.buckets {
		if value <= boundary {
			h.counts[i]++
			return
		}
	}

	// Value is greater than all buckets (infinity bucket)
	h.counts[len(h.counts)-1]++
}

// GetPercentile calculates a percentile from the histogram
func (h *Histogram) GetPercentile(percentile float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return 0
	}

	targetCount := int64(math.Ceil(float64(h.count) * percentile / 100))
	cumulative := int64(0)

	for i, count := range h.counts {
		cumulative += count
		if cumulative >= targetCount {
			if i < len(h.buckets) {
				return h.buckets[i]
			}
			return math.Inf(1)
		}
	}

	return 0
}
