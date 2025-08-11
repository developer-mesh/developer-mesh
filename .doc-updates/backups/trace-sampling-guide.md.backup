# Trace Sampling Guide

> **Purpose**: Comprehensive guide for implementing trace sampling strategies that balance performance and visibility
> **Audience**: Platform engineers and SREs managing observability infrastructure
> **Scope**: Sampling algorithms, performance impacts, and adaptive strategies

## Table of Contents

1. [Overview](#overview)
2. [Sampling Fundamentals](#sampling-fundamentals)
3. [Sampling Strategies](#sampling-strategies)
4. [Performance vs Visibility Tradeoffs](#performance-vs-visibility-tradeoffs)
5. [Implementing Sampling](#implementing-sampling)
6. [Adaptive Sampling](#adaptive-sampling)
7. [Head-Based vs Tail-Based Sampling](#head-based-vs-tail-based-sampling)
8. [Sampling for Different Scenarios](#sampling-for-different-scenarios)
9. [Monitoring and Tuning](#monitoring-and-tuning)
10. [Best Practices](#best-practices)

## Overview

Trace sampling is critical for managing the cost and performance overhead of distributed tracing while maintaining sufficient visibility for debugging and monitoring.

### Why Sampling Matters

- **Volume**: Production systems generate millions of traces
- **Cost**: Storage and processing costs scale with trace volume
- **Performance**: Tracing overhead impacts application latency
- **Network**: Trace data transmission affects bandwidth
- **Analysis**: Too much data can overwhelm analysis tools

### Sampling Goals

1. **Capture important traces**: Errors, slow requests, critical paths
2. **Minimize overhead**: Keep performance impact under 1%
3. **Control costs**: Stay within budget for storage and processing
4. **Maintain visibility**: Ensure debugging capabilities
5. **Statistical accuracy**: Preserve system behavior patterns

## Sampling Fundamentals

### Sampling Decision Points

```
Request → [Sampling Decision] → Trace Created → [Propagation] → Downstream Services
                ↓ No
            No Trace
```

### Key Concepts

#### Sampling Rate
The percentage of requests that generate traces.

```go
// 10% sampling rate
sampler := trace.TraceIDRatioBased(0.1)

// 1% sampling rate for high-volume services
sampler := trace.TraceIDRatioBased(0.01)
```

#### Sampling Decision
Made at trace creation and propagated to all spans.

```go
// Decision is encoded in trace flags
type TraceFlags byte

const (
    FlagsSampled = TraceFlags(0x01)
)
```

#### Parent-Based Sampling
Respects upstream sampling decisions.

```go
// Follow parent's sampling decision
sampler := trace.ParentBased(
    trace.TraceIDRatioBased(0.1), // Root sampler
)
```

## Sampling Strategies

### 1. Always On/Off Sampling

```go
// Always sample (development/testing)
sampler := trace.AlwaysSample()

// Never sample (disabled)
sampler := trace.NeverSample()
```

**Use Cases**:
- Development environments (Always On)
- Load testing without tracing overhead (Always Off)
- Debugging specific issues (Always On temporarily)

### 2. Probabilistic Sampling

```go
// TraceID-based ratio sampling
type TraceIDRatioSampler struct {
    ratio float64
}

func (s *TraceIDRatioSampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    // Use trace ID for consistent decision
    traceID := parameters.TraceID
    
    // Convert trace ID to number
    val := binary.BigEndian.Uint64(traceID[8:16])
    
    // Sample if value falls within ratio
    if float64(val)/float64(math.MaxUint64) < s.ratio {
        return trace.SamplingResult{
            Decision: trace.RecordAndSample,
        }
    }
    
    return trace.SamplingResult{
        Decision: trace.Drop,
    }
}
```

**Characteristics**:
- Deterministic based on trace ID
- Consistent across services
- Statistically accurate

### 3. Rate-Limited Sampling

```go
// Rate-limited sampler
type RateLimitedSampler struct {
    limiter *rate.Limiter
    fallback trace.Sampler
}

func NewRateLimitedSampler(tracesPerSecond float64) *RateLimitedSampler {
    return &RateLimitedSampler{
        limiter: rate.NewLimiter(rate.Limit(tracesPerSecond), int(tracesPerSecond)),
        fallback: trace.TraceIDRatioBased(0.001), // 0.1% fallback
    }
}

func (s *RateLimitedSampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    // Try rate limiter first
    if s.limiter.Allow() {
        return trace.SamplingResult{
            Decision: trace.RecordAndSample,
        }
    }
    
    // Fall back to probabilistic sampling
    return s.fallback.ShouldSample(parameters)
}
```

**Benefits**:
- Predictable trace volume
- Cost control
- Protection against traffic spikes

### 4. Priority Sampling

```go
// Priority-based sampler
type PrioritySampler struct {
    rules []SamplingRule
}

type SamplingRule struct {
    Priority  int
    Condition func(trace.SamplingParameters) bool
    Sampler   trace.Sampler
}

func (s *PrioritySampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    // Evaluate rules in priority order
    for _, rule := range s.rules {
        if rule.Condition(parameters) {
            return rule.Sampler.ShouldSample(parameters)
        }
    }
    
    // Default sampling
    return trace.SamplingResult{
        Decision: trace.Drop,
    }
}

// Configuration
sampler := &PrioritySampler{
    rules: []SamplingRule{
        {
            Priority: 1,
            Condition: func(p trace.SamplingParameters) bool {
                // Always sample errors
                return p.Attributes.HasAttribute("error")
            },
            Sampler: trace.AlwaysSample(),
        },
        {
            Priority: 2,
            Condition: func(p trace.SamplingParameters) bool {
                // Sample slow requests
                return p.Attributes.HasAttribute("slow_request")
            },
            Sampler: trace.TraceIDRatioBased(0.5),
        },
        {
            Priority: 3,
            Condition: func(p trace.SamplingParameters) bool {
                // VIP users
                return p.Attributes.HasAttribute("user.vip")
            },
            Sampler: trace.TraceIDRatioBased(0.1),
        },
        {
            Priority: 4,
            Condition: func(p trace.SamplingParameters) bool {
                return true // Default
            },
            Sampler: trace.TraceIDRatioBased(0.001),
        },
    },
}
```

### 5. Adaptive Sampling

```go
// Adaptive sampler that adjusts based on load
type AdaptiveSampler struct {
    mu              sync.RWMutex
    currentRate     float64
    targetTPM       float64 // Traces per minute
    metricsCollector *MetricsCollector
}

func (s *AdaptiveSampler) adjustSamplingRate() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        currentTPM := s.metricsCollector.GetTracesPerMinute()
        
        s.mu.Lock()
        if currentTPM > s.targetTPM {
            // Reduce sampling rate
            s.currentRate *= 0.9
        } else if currentTPM < s.targetTPM*0.8 {
            // Increase sampling rate
            s.currentRate = math.Min(s.currentRate*1.1, 1.0)
        }
        s.mu.Unlock()
    }
}

func (s *AdaptiveSampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    s.mu.RLock()
    rate := s.currentRate
    s.mu.RUnlock()
    
    sampler := trace.TraceIDRatioBased(rate)
    return sampler.ShouldSample(parameters)
}
```

## Performance vs Visibility Tradeoffs

### Performance Impact Analysis

#### Overhead Components

1. **CPU Overhead**
```go
// Benchmark tracing overhead
func BenchmarkWithTracing(b *testing.B) {
    // With tracing
    tracedHandler := otelhttp.NewHandler(handler, "benchmark")
    
    b.Run("with_tracing_1pct", func(b *testing.B) {
        configureSampling(0.01)
        benchmarkRequests(b, tracedHandler)
    })
    
    b.Run("with_tracing_10pct", func(b *testing.B) {
        configureSampling(0.10)
        benchmarkRequests(b, tracedHandler)
    })
    
    b.Run("without_tracing", func(b *testing.B) {
        benchmarkRequests(b, handler)
    })
}

// Results:
// with_tracing_1pct:   120ns/op (0.5% overhead)
// with_tracing_10pct:  125ns/op (0.8% overhead)
// without_tracing:     115ns/op (baseline)
```

2. **Memory Overhead**
```go
// Trace memory usage
type TraceMemoryStats struct {
    SpansPerTrace    int
    BytesPerSpan     int
    ActiveTraces     int
    TotalMemoryMB    float64
}

func calculateMemoryOverhead(samplingRate float64, rps int) TraceMemoryStats {
    avgSpansPerTrace := 10
    avgBytesPerSpan := 2048 // Including attributes
    avgTraceDuration := 500 * time.Millisecond
    
    activeTraces := int(float64(rps) * samplingRate * avgTraceDuration.Seconds())
    totalBytes := activeTraces * avgSpansPerTrace * avgBytesPerSpan
    
    return TraceMemoryStats{
        SpansPerTrace: avgSpansPerTrace,
        BytesPerSpan:  avgBytesPerSpan,
        ActiveTraces:  activeTraces,
        TotalMemoryMB: float64(totalBytes) / 1024 / 1024,
    }
}
```

3. **Network Overhead**
```go
// Calculate network bandwidth for traces
func calculateBandwidth(samplingRate float64, rps int) BandwidthStats {
    avgTraceSize := 10 * 1024 // 10KB per trace
    tracesPerSecond := float64(rps) * samplingRate
    
    return BandwidthStats{
        TracesPerSecond: tracesPerSecond,
        MBitsPerSecond:  tracesPerSecond * float64(avgTraceSize) * 8 / 1_000_000,
        GBPerDay:        tracesPerSecond * float64(avgTraceSize) * 86400 / 1_000_000_000,
    }
}
```

### Visibility Requirements

#### Critical Visibility Needs

```yaml
# Minimum sampling for different scenarios
visibility_requirements:
  error_detection:
    minimum_sampling: 100%  # All errors must be traced
    rationale: "Need complete error visibility"
    
  performance_monitoring:
    minimum_sampling: 1%    # Statistical significance
    rationale: "1% provides accurate P99 latency"
    
  user_journey_tracking:
    minimum_sampling: 10%   # Key user flows
    rationale: "Track conversion funnels"
    
  security_monitoring:
    minimum_sampling: 100%  # Suspicious activities
    rationale: "Security events need full visibility"
```

### Cost Analysis

```go
// Cost calculator for different sampling rates
type TracingCosts struct {
    StoragePerGB     float64
    ProcessingPerGB  float64
    TransferPerGB    float64
}

func calculateMonthlyCost(samplingRate float64, rps int, costs TracingCosts) float64 {
    avgTraceSize := 10 * 1024 // 10KB
    tracesPerMonth := float64(rps) * samplingRate * 86400 * 30
    gbPerMonth := tracesPerMonth * float64(avgTraceSize) / 1_000_000_000
    
    storageCost := gbPerMonth * costs.StoragePerGB
    processingCost := gbPerMonth * costs.ProcessingPerGB
    transferCost := gbPerMonth * costs.TransferPerGB
    
    return storageCost + processingCost + transferCost
}

// Example calculation
// 1000 RPS, 1% sampling = $150/month
// 1000 RPS, 10% sampling = $1500/month
// 1000 RPS, 100% sampling = $15000/month
```

## Implementing Sampling

### Service-Level Configuration

```go
// Per-service sampling configuration
type ServiceSamplingConfig struct {
    ServiceName     string
    BaseSamplingRate float64
    Rules           []SamplingRule
    RateLimit       *int
}

var serviceConfigs = map[string]ServiceSamplingConfig{
    "mcp-server": {
        ServiceName:      "mcp-server",
        BaseSamplingRate: 0.01, // 1% base
        Rules: []SamplingRule{
            {Name: "errors", Rate: 1.0},
            {Name: "slow_requests", Rate: 0.5},
            {Name: "websocket", Rate: 0.001},
        },
        RateLimit: intPtr(100), // Max 100 traces/second
    },
    "rest-api": {
        ServiceName:      "rest-api",
        BaseSamplingRate: 0.1, // 10% for API gateway
        Rules: []SamplingRule{
            {Name: "health_checks", Rate: 0.0001},
            {Name: "admin_endpoints", Rate: 1.0},
        },
    },
    "worker": {
        ServiceName:      "worker",
        BaseSamplingRate: 0.05, // 5% for background jobs
        RateLimit:        intPtr(50),
    },
}
```

### Dynamic Sampling Rules

```go
// Rule-based sampler with hot reload
type DynamicRuleSampler struct {
    mu     sync.RWMutex
    rules  []DynamicRule
    config *SamplingConfig
}

type DynamicRule struct {
    ID         string
    Priority   int
    Conditions map[string]interface{}
    SampleRate float64
    Enabled    bool
}

func (s *DynamicRuleSampler) LoadRules(configPath string) error {
    // Watch config file for changes
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    
    go func() {
        for event := range watcher.Events {
            if event.Op&fsnotify.Write == fsnotify.Write {
                s.reloadRules(configPath)
            }
        }
    }()
    
    return watcher.Add(configPath)
}

func (s *DynamicRuleSampler) ShouldSample(p trace.SamplingParameters) trace.SamplingResult {
    s.mu.RLock()
    rules := s.rules
    s.mu.RUnlock()
    
    // Evaluate rules
    for _, rule := range rules {
        if !rule.Enabled {
            continue
        }
        
        if s.evaluateConditions(rule.Conditions, p) {
            if shouldSample(p.TraceID, rule.SampleRate) {
                return trace.SamplingResult{
                    Decision: trace.RecordAndSample,
                    Attributes: []attribute.KeyValue{
                        attribute.String("sampling.rule", rule.ID),
                    },
                }
            }
        }
    }
    
    return trace.SamplingResult{Decision: trace.Drop}
}
```

### Sampling Configuration Examples

```yaml
# sampling-config.yaml
version: "1.0"
default_rate: 0.001  # 0.1% default

services:
  mcp-server:
    base_rate: 0.01
    rate_limit: 100  # traces per second
    
    rules:
      - name: "error_traces"
        priority: 1
        conditions:
          - field: "error"
            operator: "exists"
        sample_rate: 1.0
        
      - name: "slow_requests"
        priority: 2
        conditions:
          - field: "duration_ms"
            operator: "gt"
            value: 1000
        sample_rate: 0.5
        
      - name: "vip_users"
        priority: 3
        conditions:
          - field: "user.tier"
            operator: "eq"
            value: "premium"
        sample_rate: 0.1
        
      - name: "expensive_operations"
        priority: 4
        conditions:
          - field: "cost.estimated"
            operator: "gt"
            value: 1.0
        sample_rate: 1.0

  worker:
    base_rate: 0.05
    
    rules:
      - name: "long_running_tasks"
        conditions:
          - field: "task.duration"
            operator: "gt"
            value: 300000  # 5 minutes
        sample_rate: 1.0
```

## Adaptive Sampling

### Load-Based Adaptation

```go
// Adaptive sampler that responds to system load
type LoadAdaptiveSampler struct {
    baseRate        float64
    minRate         float64
    maxRate         float64
    metricsProvider MetricsProvider
    
    // Adaptation parameters
    cpuThreshold    float64
    memoryThreshold float64
    latencyTarget   time.Duration
}

func (s *LoadAdaptiveSampler) calculateRate() float64 {
    metrics := s.metricsProvider.GetCurrentMetrics()
    
    // Start with base rate
    rate := s.baseRate
    
    // Reduce if CPU is high
    if metrics.CPUUsage > s.cpuThreshold {
        rate *= (s.cpuThreshold / metrics.CPUUsage)
    }
    
    // Reduce if memory is high
    if metrics.MemoryUsage > s.memoryThreshold {
        rate *= (s.memoryThreshold / metrics.MemoryUsage)
    }
    
    // Reduce if latency is high
    if metrics.P99Latency > s.latencyTarget {
        rate *= float64(s.latencyTarget) / float64(metrics.P99Latency)
    }
    
    // Apply bounds
    return math.Max(s.minRate, math.Min(rate, s.maxRate))
}
```

### Budget-Based Sampling

```go
// Sampler that respects cost budgets
type BudgetSampler struct {
    hourlyBudget    int     // Maximum traces per hour
    currentHour     int
    tracesThisHour  int64
    mu              sync.Mutex
}

func (s *BudgetSampler) ShouldSample(p trace.SamplingParameters) trace.SamplingResult {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Check if new hour
    hour := time.Now().Hour()
    if hour != s.currentHour {
        s.currentHour = hour
        s.tracesThisHour = 0
    }
    
    // Check budget
    if s.tracesThisHour >= int64(s.hourlyBudget) {
        return trace.SamplingResult{Decision: trace.Drop}
    }
    
    // Sample and increment counter
    s.tracesThisHour++
    return trace.SamplingResult{
        Decision: trace.RecordAndSample,
        Attributes: []attribute.KeyValue{
            attribute.Int64("sampling.hour_count", s.tracesThisHour),
        },
    }
}
```

## Head-Based vs Tail-Based Sampling

### Head-Based Sampling

**Decision made at trace start**

```go
// Head-based sampler
func HeadBasedSampling(ctx context.Context) (context.Context, bool) {
    // Make decision before creating trace
    sampler := getSampler()
    result := sampler.ShouldSample(trace.SamplingParameters{
        TraceID: generateTraceID(),
    })
    
    if result.Decision == trace.RecordAndSample {
        ctx, _ = tracer.Start(ctx, "operation")
        return ctx, true
    }
    
    return ctx, false
}
```

**Pros**:
- Low overhead
- Immediate decision
- No buffering required
- Simple implementation

**Cons**:
- Can't sample based on full trace
- Might miss interesting traces
- No latency-based decisions

### Tail-Based Sampling

**Decision made after trace completion**

```go
// Tail-based sampling collector
type TailSampler struct {
    buffer       *TraceBuffer
    policies     []Policy
    decisionWait time.Duration
}

type Policy interface {
    Evaluate(trace *Trace) (bool, float64) // sample, priority
}

// Latency policy
type LatencyPolicy struct {
    Threshold time.Duration
}

func (p *LatencyPolicy) Evaluate(trace *Trace) (bool, float64) {
    if trace.Duration > p.Threshold {
        return true, 1.0 // Always sample slow traces
    }
    return false, 0
}

// Error policy
type ErrorPolicy struct{}

func (p *ErrorPolicy) Evaluate(trace *Trace) (bool, float64) {
    for _, span := range trace.Spans {
        if span.Status.Code == codes.Error {
            return true, 1.0
        }
    }
    return false, 0
}

// Implementation
func (s *TailSampler) ProcessTrace(trace *Trace) {
    // Buffer trace temporarily
    s.buffer.Add(trace)
    
    // Wait for trace to complete
    time.AfterFunc(s.decisionWait, func() {
        completeTrace := s.buffer.Get(trace.ID)
        
        // Evaluate policies
        shouldSample := false
        maxPriority := 0.0
        
        for _, policy := range s.policies {
            sample, priority := policy.Evaluate(completeTrace)
            if sample && priority > maxPriority {
                shouldSample = true
                maxPriority = priority
            }
        }
        
        if shouldSample {
            s.export(completeTrace)
        } else {
            s.buffer.Remove(trace.ID)
        }
    })
}
```

**Pros**:
- Can sample based on complete trace
- Catches all interesting traces
- Supports complex policies

**Cons**:
- Higher memory usage
- Added latency
- Complex implementation
- Requires trace collector

## Sampling for Different Scenarios

### High-Volume Services

```yaml
# API Gateway - High volume, low sampling
sampling:
  service: api-gateway
  expected_rps: 100000
  
  strategy:
    base_rate: 0.0001  # 0.01%
    rate_limit: 100    # Max 100 traces/second
    
  special_cases:
    - path: /health
      rate: 0.00001    # Almost never
    - path: /metrics
      rate: 0.00001
    - errors: true
      rate: 1.0        # Always sample errors
```

### Critical Services

```yaml
# Payment Service - Critical, higher sampling
sampling:
  service: payment-service
  expected_rps: 1000
  
  strategy:
    base_rate: 0.1     # 10%
    min_rate: 0.05     # Never go below 5%
    
  rules:
    - operation: ProcessPayment
      rate: 0.5        # 50% of payments
    - amount_gt: 1000
      rate: 1.0        # All high-value transactions
    - user_type: new
      rate: 1.0        # All new user transactions
```

### Development Environment

```go
// Development sampling - High visibility
func DevelopmentSampler() trace.Sampler {
    if os.Getenv("TRACE_ALL") == "true" {
        return trace.AlwaysSample()
    }
    
    // High sampling in dev
    return trace.ParentBased(
        trace.TraceIDRatioBased(0.5), // 50% sampling
    )
}
```

### Load Testing

```go
// Load test sampling - Minimal overhead
func LoadTestSampler() trace.Sampler {
    return &CompositeSampler{
        samplers: []trace.Sampler{
            // Sample first minute at 1%
            &TimedSampler{
                Duration: 1 * time.Minute,
                Sampler:  trace.TraceIDRatioBased(0.01),
            },
            // Then drop to 0.01%
            trace.TraceIDRatioBased(0.0001),
        },
    }
}
```

## Monitoring and Tuning

### Sampling Metrics

```go
// Track sampling effectiveness
type SamplingMetrics struct {
    TotalRequests   int64
    SampledRequests int64
    DroppedTraces   int64
    
    // By decision reason
    SampledByError    int64
    SampledByLatency  int64
    SampledByUser     int64
    SampledByRandom   int64
}

// Prometheus metrics
var (
    samplingDecisions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tracing_sampling_decisions_total",
            Help: "Sampling decisions by type",
        },
        []string{"decision", "reason"},
    )
    
    effectiveSamplingRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "tracing_effective_sampling_rate",
            Help: "Actual sampling rate",
        },
        []string{"service"},
    )
)
```

### Dashboard for Sampling

```json
{
  "dashboard": {
    "title": "Trace Sampling Monitor",
    "panels": [
      {
        "title": "Effective Sampling Rate",
        "query": "tracing_effective_sampling_rate",
        "type": "graph"
      },
      {
        "title": "Traces Per Second",
        "query": "rate(tracing_sampling_decisions_total[5m])",
        "type": "graph"
      },
      {
        "title": "Sampling Decisions",
        "query": "sum by (reason) (rate(tracing_sampling_decisions_total[5m]))",
        "type": "piechart"
      },
      {
        "title": "Trace Storage Size",
        "query": "jaeger_collector_traces_saved_by_svc_total",
        "type": "graph"
      }
    ]
  }
}
```

### Tuning Guidelines

```yaml
# Sampling tuning checklist
tuning_process:
  1_establish_baseline:
    - measure: Current trace volume
    - measure: Storage costs
    - measure: Query performance
    - measure: Application overhead
    
  2_identify_requirements:
    - define: SLA for trace availability
    - define: Budget constraints
    - define: Performance targets
    - define: Debugging needs
    
  3_implement_changes:
    - start: Conservative sampling (lower rates)
    - monitor: Impact for 24-48 hours
    - adjust: Gradually increase if needed
    
  4_validate_effectiveness:
    - check: Error traces captured (should be 100%)
    - check: P99 latency accuracy (within 5%)
    - check: Cost within budget
    - check: Performance overhead (<1%)
```

## Best Practices

### 1. Start Conservative

```go
// Begin with low sampling rates
var defaultSamplingRates = map[string]float64{
    "production":  0.001,  // 0.1%
    "staging":     0.01,   // 1%
    "development": 0.1,    // 10%
}
```

### 2. Always Sample Critical Operations

```go
// Force sampling for critical paths
func ForceSampling(ctx context.Context, operation string) context.Context {
    return trace.ContextWithSpanOptions(ctx, 
        trace.WithAttributes(attribute.Bool("force.sample", true)),
    )
}
```

### 3. Use Consistent Sampling

```go
// Ensure consistent decisions across services
func ConsistentSampler(traceID trace.TraceID, rate float64) bool {
    // Use trace ID for consistency
    hash := fnv.New64a()
    hash.Write(traceID[:])
    
    threshold := uint64(rate * float64(math.MaxUint64))
    return hash.Sum64() < threshold
}
```

### 4. Monitor Sampling Health

```go
// Health check for sampling
func SamplingHealthCheck() error {
    recentSampleRate := getRecentSampleRate()
    
    if recentSampleRate == 0 {
        return fmt.Errorf("no traces sampled in last hour")
    }
    
    if recentSampleRate > expectedRate * 2 {
        return fmt.Errorf("sampling rate too high: %.2f%%", recentSampleRate*100)
    }
    
    return nil
}
```

### 5. Document Sampling Decisions

```yaml
# Document your sampling strategy
sampling_documentation:
  rationale: "Balance cost and visibility for 10K RPS service"
  
  decisions:
    - service: api-gateway
      rate: 0.001
      reason: "High volume, stateless operations"
      
    - service: payment-service  
      rate: 0.1
      reason: "Critical path, need visibility"
      
    - rule: errors
      rate: 1.0
      reason: "All errors must be captured"
      
  review_schedule: "Monthly"
  owner: "platform-team@example.com"
```

## Sampling Decision Matrix

| Scenario | Sampling Rate | Strategy | Rationale |
|----------|--------------|----------|-----------|
| High-volume API | 0.01% - 0.1% | Rate-limited | Control costs |
| Critical Services | 5% - 10% | Probabilistic | Balance visibility |
| Errors | 100% | Always | Debug all errors |
| New Features | 50% - 100% | Time-limited | Validate behavior |
| Load Tests | 0.01% | Minimal | Reduce overhead |
| Dev Environment | 50% - 100% | High | Easy debugging |
| Health Checks | 0.001% | Very low | Reduce noise |
| VIP Users | 10% - 50% | Priority | Business critical |

## Next Steps

1. Review [Observability Architecture](./observability-architecture.md) for system design
2. See [Trace-Based Debugging](./trace-based-debugging.md) for using sampled traces
3. Check [Cross-Service Tracing](./cross-service-tracing.md) for propagation
4. Read [Performance Tuning Guide](./performance-tuning-guide.md) for optimization