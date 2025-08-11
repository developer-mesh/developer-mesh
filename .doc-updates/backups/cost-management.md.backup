# Cost Management Guide

> **Purpose**: Comprehensive guide for managing and controlling costs in the Developer Mesh platform
> **Audience**: Platform operators, finance teams, and developers concerned with cost optimization
> **Scope**: Cost tracking, limits, monitoring, and optimization strategies

## Table of Contents

1. [Overview](#overview)
2. [Cost Control Architecture](#cost-control-architecture)
3. [Configuration](#configuration)
4. [Cost Tracking Implementation](#cost-tracking-implementation)
5. [AWS Service Costs](#aws-service-costs)
6. [AI Model Costs](#ai-model-costs)
7. [Monitoring and Alerts](#monitoring-and-alerts)
8. [Cost Optimization Strategies](#cost-optimization-strategies)
9. [Budget Management](#budget-management)
10. [Best Practices](#best-practices)

## Overview

The Developer Mesh platform implements comprehensive cost controls to prevent unexpected charges and optimize resource usage. Key features include:

- **Real-time cost tracking** for AI model usage
- **Session and global cost limits** with automatic cutoffs
- **Per-service cost attribution**
- **Budget alerts and notifications**
- **Cost optimization recommendations**

### Cost Control Philosophy

1. **Predictability**: No surprise bills
2. **Visibility**: Know where every dollar goes
3. **Control**: Hard limits that prevent overruns
4. **Optimization**: Continuous improvement of cost efficiency

## Cost Control Architecture

### Platform Cost Controls

```go
// pkg/common/cost_tracking.go
type CostTracker struct {
    sessionCosts  map[string]float64
    globalCost    float64
    sessionLimit  float64
    globalLimit   float64
    mu            sync.RWMutex
}

// Key cost control points:
// 1. Pre-execution validation
// 2. Real-time tracking
// 3. Post-execution enforcement
// 4. Session and global limits
```

### Cost Flow Diagram

```
┌─────────────────┐
│ Request Arrives │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Check   │
    │ Limits  │
    └────┬────┘
         │
    ┌────▼────┐     ┌───────────┐
    │ Under   ├─No──► Reject    │
    │ Limit?  │     │ Request   │
    └────┬────┘     └───────────┘
         │Yes
    ┌────▼────┐
    │ Process │
    │ Request │
    └────┬────┘
         │
    ┌────▼────┐
    │ Track   │
    │ Costs   │
    └────┬────┘
         │
    ┌────▼────┐
    │ Update  │
    │ Totals  │
    └─────────┘
```

## Configuration

### Environment Variables

```bash
# Cost Control Settings (.env)
# ===========================

# Session-level cost limit (per user session)
BEDROCK_SESSION_LIMIT=0.10      # $0.10 per session

# Global cost limit (across all sessions)
GLOBAL_COST_LIMIT=10.0          # $10.00 total

# Cost tracking interval
COST_TRACKING_INTERVAL=60s      # Update costs every minute

# Budget alert thresholds
BUDGET_WARNING_THRESHOLD=0.80   # Alert at 80% of limit
BUDGET_CRITICAL_THRESHOLD=0.95  # Critical alert at 95%

# AWS Cost Explorer Integration
ENABLE_COST_EXPLORER=true
COST_EXPLORER_GRANULARITY=DAILY

# Cost allocation tags
COST_ALLOCATION_TAGS=Environment,Project,Team,Service
```

### Cost Configuration by Service

```yaml
# configs/cost-config.yaml
services:
  bedrock:
    models:
      - name: "amazon.titan-embed-text-v1"
        input_cost: 0.0001      # per 1K tokens
        output_cost: 0.0000     # embeddings have no output
      - name: "amazon.titan-embed-text-v2"
        input_cost: 0.00002     # per 1K tokens
        output_cost: 0.0000
      - name: "cohere.embed-english-v3"
        input_cost: 0.0001
        output_cost: 0.0000
      - name: "cohere.embed-multilingual-v3"
        input_cost: 0.0001
        output_cost: 0.0000
      - name: "anthropic.claude-3-sonnet"
        input_cost: 0.003       # per 1K tokens
        output_cost: 0.015      # per 1K tokens
        
  storage:
    s3:
      storage_cost: 0.023       # per GB/month
      request_cost: 0.0004      # per 1K requests
      transfer_cost: 0.09       # per GB (out)
      
  compute:
    ecs_fargate:
      vcpu_hour: 0.04048
      gb_hour: 0.004445
      
  database:
    rds_postgres:
      instance_hour: 0.096      # db.t3.medium
      storage_gb_month: 0.115
      iops: 0.10               # per million
    elasticache:
      node_hour: 0.034         # cache.t3.micro
```

## Cost Tracking Implementation

### Session Cost Tracking

```go
// Track costs per user session
func (ct *CostTracker) TrackSessionCost(sessionID string, cost float64) error {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    
    // Update session cost
    ct.sessionCosts[sessionID] += cost
    
    // Check session limit
    if ct.sessionCosts[sessionID] > ct.sessionLimit {
        // Log limit exceeded
        ct.logger.Warn("Session cost limit exceeded", 
            "session_id", sessionID,
            "cost", ct.sessionCosts[sessionID],
            "limit", ct.sessionLimit)
        
        // Send notification
        ct.notifyLimitExceeded(sessionID, "session")
        
        return ErrSessionLimitExceeded
    }
    
    // Update global cost
    ct.globalCost += cost
    
    // Check global limit
    if ct.globalCost > ct.globalLimit {
        ct.logger.Error("Global cost limit exceeded",
            "total_cost", ct.globalCost,
            "limit", ct.globalLimit)
        
        return ErrGlobalLimitExceeded
    }
    
    // Record metric
    ct.metrics.RecordCost("session", sessionID, cost)
    
    return nil
}
```

### Model Usage Cost Calculation

```go
// Calculate AI model costs
func CalculateModelCost(model string, inputTokens, outputTokens int) float64 {
    rates := getModelRates(model)
    
    inputCost := float64(inputTokens) / 1000.0 * rates.InputCost
    outputCost := float64(outputTokens) / 1000.0 * rates.OutputCost
    
    return inputCost + outputCost
}

// Example usage in embedding service
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    // Check cost before processing
    estimatedCost := EstimateEmbeddingCost(s.model, len(text))
    if err := s.costTracker.CheckBudget(ctx, estimatedCost); err != nil {
        return nil, fmt.Errorf("budget check failed: %w", err)
    }
    
    // Generate embedding
    embedding, tokens, err := s.provider.Embed(ctx, text)
    if err != nil {
        return nil, err
    }
    
    // Track actual cost
    actualCost := CalculateModelCost(s.model, tokens, 0)
    s.costTracker.TrackCost(ctx, "embedding", actualCost)
    
    return embedding, nil
}
```

### Cost Attribution

```go
// Tag costs by service, team, and purpose
type CostAttribution struct {
    Service     string
    Team        string
    Purpose     string
    Environment string
    Timestamp   time.Time
    Amount      float64
    Details     map[string]interface{}
}

func (ct *CostTracker) AttributeCost(attr CostAttribution) {
    // Store in time-series database
    ct.metrics.RecordAttributedCost(attr)
    
    // Update aggregates
    ct.updateDailyCosts(attr)
    ct.updateMonthlyCosts(attr)
    
    // Check team budgets
    if teamBudget := ct.getTeamBudget(attr.Team); teamBudget != nil {
        teamBudget.Track(attr.Amount)
    }
}
```

## AWS Service Costs

### Cost Monitoring by Service

#### 1. Compute Costs (ECS Fargate)

```bash
# Monitor ECS costs
aws ce get-cost-and-usage \
  --time-period Start=2024-12-01,End=2024-12-31 \
  --granularity DAILY \
  --metrics "UnblendedCost" \
  --group-by Type=DIMENSION,Key=SERVICE \
  --filter '{
    "Dimensions": {
      "Key": "SERVICE",
      "Values": ["Amazon Elastic Container Service"]
    }
  }'
```

#### 2. Database Costs (RDS + ElastiCache)

```sql
-- Track database operations cost
CREATE TABLE cost_tracking (
    id SERIAL PRIMARY KEY,
    service VARCHAR(50),
    operation VARCHAR(100),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    duration_ms INTEGER,
    estimated_cost DECIMAL(10, 6)
);

-- Index for cost analysis
CREATE INDEX idx_cost_by_service_date ON cost_tracking(service, timestamp);
```

#### 3. Storage Costs (S3)

```go
// Track S3 operations
func TrackS3Cost(bucket string, operation string, bytes int64) {
    cost := calculateS3Cost(operation, bytes)
    
    costTracker.Track(CostItem{
        Service:   "s3",
        Resource:  bucket,
        Operation: operation,
        Units:     bytes,
        Cost:      cost,
    })
}

func calculateS3Cost(operation string, bytes int64) float64 {
    switch operation {
    case "PUT", "POST":
        return 0.005 / 1000  // $0.005 per 1K requests
    case "GET":
        return 0.0004 / 1000 // $0.0004 per 1K requests
    case "TRANSFER":
        return float64(bytes) / (1024*1024*1024) * 0.09 // $0.09 per GB
    default:
        return 0
    }
}
```

### 4. Message Queue Costs (SQS)

```go
// SQS cost tracking
type SQSCostTracker struct {
    requestCount int64
    mu          sync.Mutex
}

func (t *SQSCostTracker) TrackRequest() {
    t.mu.Lock()
    t.requestCount++
    t.mu.Unlock()
    
    // SQS: First 1M requests free, then $0.40 per million
    if t.requestCount > 1_000_000 {
        cost := 0.40 / 1_000_000
        globalCostTracker.Track("sqs", cost)
    }
}
```

## AI Model Costs

### Bedrock Cost Management

```go
// Comprehensive Bedrock cost tracking
type BedrockCostManager struct {
    tracker     *CostTracker
    limits      map[string]float64
    usage       map[string]*ModelUsage
    mu          sync.RWMutex
}

type ModelUsage struct {
    InputTokens  int64
    OutputTokens int64
    Invocations  int64
    TotalCost    float64
    LastReset    time.Time
}

func (m *BedrockCostManager) PreCheckCost(model string, estimatedTokens int) error {
    // Get current usage
    usage := m.getUsage(model)
    
    // Estimate cost
    estimatedCost := m.estimateCost(model, estimatedTokens)
    
    // Check against limits
    if usage.TotalCost + estimatedCost > m.limits[model] {
        return fmt.Errorf("model %s would exceed cost limit: current=%.4f, estimated=%.4f, limit=%.4f",
            model, usage.TotalCost, estimatedCost, m.limits[model])
    }
    
    return nil
}

// Model-specific rate cards
var modelRates = map[string]ModelRate{
    "claude-3-opus": {
        InputCost:  0.015,   // per 1K tokens
        OutputCost: 0.075,   // per 1K tokens
    },
    "claude-3-sonnet": {
        InputCost:  0.003,
        OutputCost: 0.015,
    },
    "titan-embed-v1": {
        InputCost:  0.0001,
        OutputCost: 0.0,     // Embeddings have no output
    },
}
```

### Cost-Aware Model Selection

```go
// Select most cost-effective model for the task
func SelectOptimalModel(task Task, budget float64) string {
    candidates := getCapableModels(task.Type)
    
    var bestModel string
    lowestCost := math.MaxFloat64
    
    for _, model := range candidates {
        estimatedCost := estimateTaskCost(model, task)
        
        if estimatedCost < lowestCost && estimatedCost <= budget {
            lowestCost = estimatedCost
            bestModel = model
        }
    }
    
    if bestModel == "" {
        // Fall back to cheapest model
        return getCheapestModel(candidates)
    }
    
    return bestModel
}
```

## Monitoring and Alerts

### CloudWatch Cost Alarms

```hcl
# Terraform configuration for cost alarms
resource "aws_cloudwatch_metric_alarm" "daily_cost_alarm" {
  alarm_name          = "mcp-daily-cost-exceeded"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name        = "EstimatedCharges"
  namespace          = "AWS/Billing"
  period             = 86400  # 24 hours
  statistic          = "Maximum"
  threshold          = 50.0   # $50 daily limit
  alarm_description  = "Triggers when daily AWS costs exceed $50"
  
  dimensions = {
    Currency = "USD"
  }
  
  alarm_actions = [aws_sns_topic.cost_alerts.arn]
}

resource "aws_budgets_budget" "monthly_budget" {
  budget_type  = "COST"
  limit_amount = "500"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"
  
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type            = "PERCENTAGE"
    notification_type         = "ACTUAL"
    subscriber_email_addresses = ["devops@example.com"]
  }
}
```

### Custom Cost Metrics

```go
// Prometheus metrics for cost tracking
var (
    modelCostTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_model_cost_dollars_total",
            Help: "Total cost of AI model usage in dollars",
        },
        []string{"model", "operation"},
    )
    
    serviceCostRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_service_cost_rate_dollars_per_hour",
            Help: "Current cost rate per service in dollars per hour",
        },
        []string{"service"},
    )
    
    budgetUtilization = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_budget_utilization_ratio",
            Help: "Budget utilization as a ratio (0-1)",
        },
        []string{"budget_type"},
    )
)
```

### Cost Dashboard

```json
{
  "dashboard": {
    "title": "MCP Cost Management",
    "panels": [
      {
        "title": "Daily Cost Trend",
        "targets": [{
          "expr": "sum(rate(mcp_model_cost_dollars_total[24h])) * 86400"
        }]
      },
      {
        "title": "Cost by Service",
        "targets": [{
          "expr": "sum by (service) (mcp_service_cost_rate_dollars_per_hour)"
        }]
      },
      {
        "title": "Budget Utilization",
        "targets": [{
          "expr": "mcp_budget_utilization_ratio"
        }]
      },
      {
        "title": "Top Cost Drivers",
        "targets": [{
          "expr": "topk(5, sum by (model) (rate(mcp_model_cost_dollars_total[1h]) * 3600))"
        }]
      }
    ]
  }
}
```

## Cost Optimization Strategies

### 1. Model Optimization

```go
// Cache embeddings to reduce API calls
type EmbeddingCache struct {
    cache *lru.Cache
    hits  int64
    miss  int64
}

func (c *EmbeddingCache) GetOrGenerate(text string, generator func() ([]float32, error)) ([]float32, error) {
    // Check cache first
    if embedding, ok := c.cache.Get(text); ok {
        atomic.AddInt64(&c.hits, 1)
        costTracker.SavedCost("embedding_cache_hit", 0.0001) // Saved $0.0001
        return embedding.([]float32), nil
    }
    
    // Generate and cache
    atomic.AddInt64(&c.miss, 1)
    embedding, err := generator()
    if err != nil {
        return nil, err
    }
    
    c.cache.Add(text, embedding)
    return embedding, nil
}
```

### 2. Request Batching

```go
// Batch requests to reduce API call overhead
type BatchProcessor struct {
    items    []BatchItem
    mu       sync.Mutex
    interval time.Duration
}

func (b *BatchProcessor) Add(item BatchItem) {
    b.mu.Lock()
    b.items = append(b.items, item)
    b.mu.Unlock()
}

func (b *BatchProcessor) processBatch() {
    b.mu.Lock()
    if len(b.items) == 0 {
        b.mu.Unlock()
        return
    }
    
    batch := b.items
    b.items = nil
    b.mu.Unlock()
    
    // Process as single API call
    result, cost := b.processItems(batch)
    
    // Cost per item is lower in batch
    costPerItem := cost / float64(len(batch))
    savings := (0.0001 * float64(len(batch))) - cost
    
    costTracker.SavedCost("batch_processing", savings)
}
```

### 3. Resource Rightsizing

```yaml
# Cost-optimized resource allocation
production:
  services:
    mcp-server:
      cpu: 1024      # 1 vCPU (was 2)
      memory: 2048   # 2 GB (was 4)
      scaling:
        min: 2       # Minimum instances
        max: 10      # Scale up as needed
        target_cpu: 70
        
    worker:
      cpu: 512       # 0.5 vCPU
      memory: 1024   # 1 GB
      scaling:
        min: 1
        max: 20
        target_queue_depth: 10
```

### 4. Scheduled Scaling

```go
// Scale down during off-hours
func ScheduleScaling() {
    // Scale down at night (EST)
    cron.Schedule("0 22 * * *", func() {
        scaleService("mcp-server", 1)    // Minimum instances
        scaleService("worker", 0)         // No workers at night
        costTracker.Log("Scaled down for night")
    })
    
    // Scale up in morning
    cron.Schedule("0 6 * * *", func() {
        scaleService("mcp-server", 3)    // Normal capacity
        scaleService("worker", 2)         // Normal workers
        costTracker.Log("Scaled up for day")
    })
}
```

## Budget Management

### Team Budget Allocation

```go
type BudgetManager struct {
    budgets map[string]*TeamBudget
    mu      sync.RWMutex
}

type TeamBudget struct {
    Team         string
    Monthly      float64
    Daily        float64
    Used         float64
    LastReset    time.Time
    Alerts       []BudgetAlert
}

func (b *BudgetManager) AllocateBudget(team string, monthly float64) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    b.budgets[team] = &TeamBudget{
        Team:    team,
        Monthly: monthly,
        Daily:   monthly / 30,
        Used:    0,
        LastReset: time.Now(),
    }
}

func (b *BudgetManager) CheckBudget(team string, amount float64) error {
    b.mu.RLock()
    budget := b.budgets[team]
    b.mu.RUnlock()
    
    if budget.Used + amount > budget.Daily {
        return fmt.Errorf("would exceed daily budget: used=%.2f, requested=%.2f, limit=%.2f",
            budget.Used, amount, budget.Daily)
    }
    
    return nil
}
```

### Cost Allocation Tags

```bash
# Tag all resources for cost tracking
aws ec2 create-tags --resources $INSTANCE_ID --tags \
  Key=Project,Value=MCP \
  Key=Environment,Value=Production \
  Key=Team,Value=Platform \
  Key=CostCenter,Value=Engineering

# Enable cost allocation tags
aws ce create-cost-category-definition \
  --name "MCP-Cost-Categories" \
  --rules file://cost-categories.json
```

## Best Practices

### 1. Implement Cost Gates

```go
// Middleware to check costs before expensive operations
func CostGateMiddleware(costLimit float64) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Estimate request cost
        estimatedCost := estimateRequestCost(c.Request)
        
        // Check against limit
        if estimatedCost > costLimit {
            c.JSON(http.StatusPaymentRequired, gin.H{
                "error": "Request would exceed cost limit",
                "estimated_cost": estimatedCost,
                "limit": costLimit,
            })
            c.Abort()
            return
        }
        
        // Track actual cost after processing
        c.Next()
        
        actualCost := c.GetFloat64("request_cost")
        costTracker.Track(c.GetString("user_id"), actualCost)
    }
}
```

### 2. Cost-Aware Caching

```go
// Cache expensive operations based on cost-benefit
func ShouldCache(operationCost float64, cacheHitRate float64) bool {
    // Cache if the saved cost over time exceeds cache storage cost
    monthlySavings := operationCost * cacheHitRate * 1000 // Assume 1000 requests/month
    monthlyCacheCost := 0.023 * 0.001 // 1MB in S3
    
    return monthlySavings > monthlyCacheCost
}
```

### 3. Regular Cost Reviews

```sql
-- Weekly cost analysis query
WITH weekly_costs AS (
  SELECT 
    DATE_TRUNC('week', timestamp) as week,
    service,
    SUM(cost) as total_cost,
    COUNT(*) as request_count,
    AVG(cost) as avg_cost_per_request
  FROM cost_tracking
  WHERE timestamp > NOW() - INTERVAL '4 weeks'
  GROUP BY DATE_TRUNC('week', timestamp), service
)
SELECT 
  week,
  service,
  total_cost,
  request_count,
  avg_cost_per_request,
  LAG(total_cost) OVER (PARTITION BY service ORDER BY week) as prev_week_cost,
  (total_cost - LAG(total_cost) OVER (PARTITION BY service ORDER BY week)) / 
    LAG(total_cost) OVER (PARTITION BY service ORDER BY week) * 100 as week_over_week_change
FROM weekly_costs
ORDER BY week DESC, total_cost DESC;
```

### 4. Automated Cost Optimization

```go
// Automatically switch to cheaper alternatives when possible
type CostOptimizer struct {
    rules []OptimizationRule
}

type OptimizationRule struct {
    Condition func(ctx context.Context) bool
    Action    func(ctx context.Context) error
    Savings   float64
}

func (o *CostOptimizer) Run(ctx context.Context) {
    for _, rule := range o.rules {
        if rule.Condition(ctx) {
            if err := rule.Action(ctx); err != nil {
                log.Printf("Optimization failed: %v", err)
                continue
            }
            costTracker.SavedCost("auto_optimization", rule.Savings)
        }
    }
}

// Example rules
var optimizationRules = []OptimizationRule{
    {
        // Use smaller model for simple tasks
        Condition: func(ctx context.Context) bool {
            return getCurrentLoad() < 0.3 // Low load
        },
        Action: func(ctx context.Context) error {
            return switchModel("claude-3-sonnet", "claude-3-haiku")
        },
        Savings: 0.002, // per request
    },
    {
        // Batch process during off-hours
        Condition: func(ctx context.Context) bool {
            hour := time.Now().Hour()
            return hour >= 22 || hour <= 6
        },
        Action: func(ctx context.Context) error {
            return enableBatchMode()
        },
        Savings: 0.05, // per hour
    },
}
```

## Cost Reporting

### Monthly Cost Report Template

```markdown
# MCP Platform Cost Report - December 2024

## Executive Summary
- **Total Cost**: $487.23 (↓ 12% from last month)
- **Cost per Request**: $0.0023 (↓ 8%)
- **Budget Utilization**: 81%

## Cost Breakdown by Service
| Service | Cost | % of Total | Change |
|---------|------|------------|--------|
| AI Models (Bedrock) | $234.56 | 48% | ↓ 15% |
| Compute (ECS) | $123.45 | 25% | ↑ 5% |
| Database (RDS) | $67.89 | 14% | → 0% |
| Storage (S3) | $34.56 | 7% | ↑ 2% |
| Cache (ElastiCache) | $23.45 | 5% | → 0% |
| Other | $3.32 | 1% | ↓ 20% |

## Cost Optimization Achievements
1. **Embedding Cache**: Saved $45.67 (87% hit rate)
2. **Request Batching**: Saved $23.45 
3. **Off-hours Scaling**: Saved $34.56
4. **Model Optimization**: Saved $56.78

## Recommendations
1. Increase embedding cache size (potential savings: $20/month)
2. Migrate to Graviton instances (potential savings: $40/month)
3. Enable S3 Intelligent-Tiering (potential savings: $15/month)
```

## Troubleshooting

### Common Cost Issues

1. **Unexpected Cost Spike**
   ```bash
   # Check for anomalous usage
   aws ce get-anomalies \
     --date-interval StartDate=2024-12-01,EndDate=2024-12-31 \
     --metric UNBLENDED_COST
   ```

2. **Budget Exceeded**
   ```go
   // Emergency cost reduction
   func EmergencyCostReduction() {
       // Disable non-essential services
       disableService("embedding-cache-warmer")
       
       // Reduce model usage
       setModelLimit("claude-3-opus", 0) // Use cheaper models only
       
       // Scale down infrastructure
       scaleAllServices(0.5) // 50% capacity
   }
   ```

3. **Cost Attribution Errors**
   ```sql
   -- Find untagged resources
   SELECT resource_id, resource_type, cost
   FROM aws_cost_explorer
   WHERE tags IS NULL OR tags = '{}'
   ORDER BY cost DESC;
   ```

## Next Steps

1. Review [Cost Optimization Guide](./cost-optimization-guide.md) for detailed strategies
2. Set up [Budget Alerts](./monitoring/budget-alerts.md)
3. Configure [Cost Allocation Tags](./configuration/cost-tags.md)
4. Enable [AWS Cost Anomaly Detection](https://aws.amazon.com/aws-cost-management/aws-cost-anomaly-detection/)

## Resources

- [AWS Pricing Calculator](https://calculator.aws/)
- [Bedrock Pricing](https://aws.amazon.com/bedrock/pricing/)
- [ECS Fargate Pricing](https://aws.amazon.com/fargate/pricing/)
- [Cost Optimization Whitepaper](https://d1.awsstatic.com/whitepapers/aws-cost-optimization.pdf)