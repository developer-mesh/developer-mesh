# Embedding Model Management - Sequence Diagrams

## 1. Model Selection Flow

```mermaid
sequenceDiagram
    participant Client as Client/Agent
    participant WS as WebSocket Server
    participant API as REST API
    participant MS as Model Service
    participant Cache as Redis Cache
    participant DB as PostgreSQL
    participant Metrics as Prometheus

    Client->>WS: embedding.generate request
    WS->>API: GenerateEmbedding(text, model?)
    API->>MS: SelectModelForRequest(tenant, params)
    
    MS->>Cache: Check cached selection
    alt Cache Hit
        Cache-->>MS: Return cached model
    else Cache Miss
        MS->>DB: GetTenantModels(tenant_id)
        DB-->>MS: List of configured models
        MS->>DB: GetUsageStats(tenant_id)
        DB-->>MS: Current usage/quotas
        
        Note over MS: Evaluate models:<br/>1. Check quotas<br/>2. Check availability<br/>3. Calculate costs<br/>4. Apply priorities
        
        MS->>Cache: Store selection (TTL: 5m)
    end
    
    MS-->>API: Selected model details
    API->>API: Generate embedding with model
    API->>DB: RecordUsage(tenant, model, tokens)
    API->>Metrics: Record metrics
    API-->>WS: Embedding response
    WS-->>Client: Result with model metadata
```

## 2. Tenant Model Configuration Flow

```mermaid
sequenceDiagram
    participant Admin as Admin User
    participant API as REST API
    participant Auth as Auth Service
    participant MS as Model Service
    participant DB as PostgreSQL
    participant Cache as Redis Cache
    participant Alert as Alert Manager

    Admin->>API: POST /tenant-models
    API->>Auth: Validate API Key
    Auth-->>API: Tenant context
    
    API->>MS: ConfigureTenantModel(tenant, model, quotas)
    MS->>DB: Check model exists in catalog
    DB-->>MS: Model details
    
    MS->>DB: BEGIN TRANSACTION
    MS->>DB: Insert/Update tenant_embedding_models
    MS->>DB: Update default if needed
    MS->>DB: COMMIT
    
    MS->>Cache: Invalidate tenant cache
    
    MS-->>API: Configuration result
    API-->>Admin: Success response
    
    MS->>Alert: Check quota thresholds
    alt Quota > 90%
        Alert->>Admin: Send warning notification
    end
```

## 3. Automatic Model Failover Flow

```mermaid
sequenceDiagram
    participant Client as Client
    participant API as REST API
    participant MS as Model Service
    participant Primary as Primary Provider
    participant Fallback as Fallback Provider
    participant CB as Circuit Breaker
    participant DB as PostgreSQL
    participant Metrics as Prometheus

    Client->>API: Generate embedding request
    API->>MS: SelectModelForRequest()
    MS-->>API: Primary model selected
    
    API->>CB: Check circuit state
    alt Circuit Open
        CB-->>API: Primary unavailable
        API->>MS: Request fallback model
        MS->>DB: Get next priority model
        DB-->>MS: Fallback model
        MS-->>API: Fallback model details
        API->>Fallback: Generate embedding
        Fallback-->>API: Success
    else Circuit Closed
        CB-->>API: Primary available
        API->>Primary: Generate embedding
        alt Primary Fails
            Primary--X API: Error/Timeout
            API->>CB: Record failure
            CB->>CB: Open circuit if threshold
            API->>MS: Request fallback
            MS-->>API: Fallback model
            API->>Fallback: Generate embedding
            Fallback-->>API: Success
            API->>DB: Record model switch
            API->>Metrics: Record failover metric
        else Primary Success
            Primary-->>API: Embedding result
            API->>CB: Record success
        end
    end
    
    API-->>Client: Embedding with model info
```

## 4. Quota Management and Enforcement Flow

```mermaid
sequenceDiagram
    participant Client as Client
    participant API as REST API
    participant MS as Model Service
    participant QM as Quota Manager
    participant DB as PostgreSQL
    participant Cache as Redis Cache
    participant Alert as Alert System

    Client->>API: Embedding request
    API->>MS: SelectModelForRequest(tenant, tokens)
    
    MS->>QM: CheckQuota(tenant, model, tokens)
    QM->>Cache: Get current usage
    alt Cache Hit
        Cache-->>QM: Current usage
    else Cache Miss
        QM->>DB: SELECT SUM(tokens) WHERE date=today
        DB-->>QM: Usage statistics
        QM->>Cache: Store usage (TTL: 1h)
    end
    
    QM->>QM: Calculate remaining quota
    
    alt Quota Available
        QM-->>MS: Quota approved
        MS-->>API: Model selected
        API->>API: Generate embedding
        API->>DB: INSERT usage record
        API->>Cache: Increment usage counter
        API-->>Client: Success
        
        par Check thresholds
            QM->>Alert: Check usage > 80%
            alt Usage > 80%
                Alert->>Admin: Send quota warning
            end
        end
    else Quota Exceeded
        QM-->>MS: Quota exceeded
        MS->>DB: Find alternative models
        alt Alternative Available
            DB-->>MS: Lower-tier model
            MS-->>API: Alternative model
            API->>API: Generate with alternative
            API-->>Client: Success with model info
        else No Alternative
            MS-->>API: No models available
            API-->>Client: 402 Quota Exceeded
            API->>Alert: Send quota exceeded alert
        end
    end
```

## 5. Model Discovery and Catalog Update Flow

```mermaid
sequenceDiagram
    participant Scheduler as Redis Scheduler
    participant Worker as Discovery Worker
    participant DS as Discovery Service
    participant Provider as Provider API
    participant DB as PostgreSQL
    participant Cache as Redis Cache
    participant Metrics as Prometheus

    Scheduler->>Worker: Trigger discovery job
    Worker->>DS: DiscoverModels()
    
    loop For each provider
        DS->>Provider: List available models
        Provider-->>DS: Model specifications
        DS->>DS: Validate and normalize
        
        DS->>DB: BEGIN TRANSACTION
        loop For each model
            DS->>DB: UPSERT embedding_model_catalog
            DS->>DB: Check tenant dependencies
            alt Model deprecated
                DS->>DB: Mark as deprecated
                DS->>DB: Get affected tenants
                DS->>Alert: Notify deprecation
            end
        end
        DS->>DB: COMMIT
    end
    
    DS->>Cache: Invalidate catalog cache
    DS->>Metrics: Record discovery metrics
    
    DS-->>Worker: Discovery complete
    Worker->>Scheduler: Schedule next run
```

## 6. Cost Tracking and Optimization Flow

```mermaid
sequenceDiagram
    participant Client as Client
    participant API as REST API
    participant ES as Embedding Service
    participant MS as Model Service
    participant CT as Cost Tracker
    participant DB as PostgreSQL
    participant Metrics as Prometheus
    participant Billing as Billing System

    Client->>API: Generate embedding
    API->>MS: SelectModelForRequest(optimize=cost)
    
    MS->>DB: Get models with costs
    DB-->>MS: Models sorted by cost
    MS->>MS: Select cheapest adequate model
    MS-->>API: Selected model with cost estimate
    
    API->>ES: Generate embedding
    ES-->>API: Result with token count
    
    API->>CT: RecordUsage(tenant, model, tokens)
    CT->>CT: Calculate cost
    CT->>DB: INSERT usage_tracking
    CT->>Metrics: Record cost metric
    
    par Async billing
        CT->>Billing: Send usage event
    and Check budget
        CT->>DB: Get monthly spending
        DB-->>CT: Current total
        alt Budget exceeded
            CT->>Alert: Budget exceeded alert
            CT->>MS: Disable expensive models
        end
    end
    
    API-->>Client: Result with cost info
```

## 7. Multi-Agent Coordination Flow

```mermaid
sequenceDiagram
    participant Agent1 as Agent 1
    participant Agent2 as Agent 2
    participant WS as WebSocket Server
    participant API as REST API
    participant MS as Model Service
    participant Pool as Connection Pool
    participant DB as PostgreSQL

    Agent1->>WS: Register with preferences
    WS->>DB: Store agent preferences
    Agent2->>WS: Register with preferences
    WS->>DB: Store agent preferences
    
    par Parallel requests
        Agent1->>WS: Embedding request
        WS->>Pool: Get connection
        Pool->>API: Forward request
        API->>MS: Select model for Agent1
        MS->>DB: Get agent preferences
        DB-->>MS: Preferred models
        MS-->>API: Agent-specific model
    and
        Agent2->>WS: Embedding request
        WS->>Pool: Get connection
        Pool->>API: Forward request
        API->>MS: Select model for Agent2
        MS->>DB: Get agent preferences
        DB-->>MS: Different preferences
        MS-->>API: Different model
    end
    
    Note over MS: Different agents may use<br/>different models based on:<br/>- Task type<br/>- Quality requirements<br/>- Cost constraints
    
    API-->>WS: Agent1 result
    WS-->>Agent1: Model A used
    API-->>WS: Agent2 result
    WS-->>Agent2: Model B used
```

## State Diagrams

### Model Lifecycle States

```mermaid
stateDiagram-v2
    [*] --> Draft: Create model
    Draft --> Testing: Enable for testing
    Testing --> Available: Validation passed
    Testing --> Draft: Validation failed
    Available --> Deprecated: Mark deprecated
    Available --> Unavailable: Provider issue
    Unavailable --> Available: Issue resolved
    Deprecated --> Removed: Grace period ended
    Removed --> [*]
    
    Available --> Available: Update config
    Testing --> Testing: Adjust settings
```

### Tenant Model Configuration States

```mermaid
stateDiagram-v2
    [*] --> Configured: Add model
    Configured --> Enabled: Enable model
    Enabled --> Default: Set as default
    Enabled --> Disabled: Disable temporarily
    Disabled --> Enabled: Re-enable
    Default --> Enabled: Change default
    Enabled --> QuotaExceeded: Hit quota
    QuotaExceeded --> Enabled: Quota reset
    Enabled --> Removed: Remove model
    Removed --> [*]
```

## Data Flow Diagram

```mermaid
graph TB
    subgraph "Client Layer"
        A[AI Agents]
        B[WebSocket Clients]
        C[REST Clients]
    end
    
    subgraph "API Layer"
        D[WebSocket Server]
        E[REST API]
        F[Auth Service]
    end
    
    subgraph "Service Layer"
        G[Model Service]
        H[Embedding Service]
        I[Cost Tracker]
        J[Quota Manager]
    end
    
    subgraph "Data Layer"
        K[(PostgreSQL)]
        L[(Redis Cache)]
        M[Circuit Breaker]
    end
    
    subgraph "External"
        N[OpenAI]
        O[Bedrock]
        P[Google]
    end
    
    subgraph "Monitoring"
        Q[Prometheus]
        R[Grafana]
        S[Alert Manager]
    end
    
    A --> D
    B --> D
    C --> E
    D --> E
    E --> F
    F --> G
    G --> H
    H --> N
    H --> O
    H --> P
    G --> I
    G --> J
    G --> K
    G --> L
    H --> M
    I --> K
    J --> K
    J --> L
    G --> Q
    I --> Q
    Q --> R
    Q --> S
```

## Performance Optimization Flow

```mermaid
sequenceDiagram
    participant Client as Client
    participant API as REST API
    participant Cache as Redis Cache
    participant Batch as Batch Processor
    participant ES as Embedding Service
    participant DB as PostgreSQL

    Note over Client,API: Multiple requests arrive
    
    Client->>API: Request 1
    Client->>API: Request 2
    Client->>API: Request 3
    
    API->>Cache: Check for cached embeddings
    alt Some cached
        Cache-->>API: Return cached
    end
    
    API->>Batch: Queue uncached requests
    Batch->>Batch: Wait for batch window (100ms)
    
    Note over Batch: Batch similar requests
    
    Batch->>ES: Batch embedding request
    ES-->>Batch: Batch results
    
    Batch->>Cache: Store results (TTL: 1h)
    Batch->>DB: Bulk insert usage records
    
    Batch-->>API: Individual results
    API-->>Client: Response 1
    API-->>Client: Response 2
    API-->>Client: Response 3
    
    Note over Client,DB: Reduced API calls<br/>Better throughput<br/>Lower costs
```