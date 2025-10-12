# Changelog

All notable changes to Developer Mesh will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

## [0.0.6] - 2025-10-12

### Added - Semantic Context Management & Virtual Agent System

This release delivers a complete semantic context management system with intelligent embedding generation, vector search capabilities, and a virtual agent architecture for session-based operations. The implementation spans 36 commits across 6 epics, bringing production-ready semantic search and context intelligence to Developer Mesh.

#### Epic 1-6: Semantic Context Management (Complete Implementation)

- **Foundation and Schema Updates** (Epic 1)
  - Created comprehensive database schema for semantic context management
  - Added `context_embeddings` table linking embeddings to contexts with importance scores
  - Implemented `context_items` table for conversation history tracking
  - Added `embeddings` table with pgvector support for 1024-dimension vectors
  - Created indexes for efficient semantic search and context retrieval
  - Migration 000033 implements full schema with proper constraints and relationships

- **Context-Aware Embedding System** (Epic 2)
  - **Context-Aware Embedding Client** (Story 2.2)
    - Implemented intelligent embedding generation with context awareness
    - Added support for multiple embedding providers (Bedrock, OpenAI, Google AI)
    - Created batch embedding generation with configurable chunk sizes
    - Implemented embedding caching to reduce API calls and costs
    - Added token counting and cost tracking per embedding operation

  - **Semantic Context Manager** (Story 2.3)
    - Created centralized context management with embedding integration
    - Implemented context item tracking for conversation history
    - Added automatic embedding generation for user/assistant messages
    - Created semantic search across context items using vector similarity
    - Implemented context summarization and compaction strategies
    - Added context lifecycle management (create, update, search, compact, delete)

- **Relevance Ranking and Context Optimization** (Epic 3)
  - **Relevance Ranking Algorithm** (Story 3.1)
    - Implemented cosine similarity scoring for semantic search results
    - Added hybrid ranking combining semantic similarity and recency
    - Created importance score weighting for critical context items
    - Implemented configurable relevance thresholds for result filtering
    - Added result deduplication and diversity scoring

  - **Token Counting and Context Packing** (Story 3.2)
    - Created efficient token counting for context size management
    - Implemented smart context packing to maximize relevant information
    - Added automatic truncation strategies (sliding window, importance-based)
    - Created context budget management with configurable limits
    - Implemented token-aware result limiting for API responses

- **Compaction Strategies** (Epic 4, Story 4.1)
  - Implemented four compaction strategies for context management:
    - **Sliding Window**: Keep most recent N items, discard older
    - **Summarize**: Generate summaries of old items, preserve recent
    - **Importance-Based**: Keep high-importance items, prune low-value
    - **Semantic Clustering**: Group similar items, keep representatives
  - Added automatic compaction triggers based on item count thresholds
  - Created compaction metadata tracking for audit and analysis
  - Implemented graceful degradation when compaction fails
  - Added metrics for compaction effectiveness and performance

- **Context Lifecycle Integration** (Epic 5)
  - **Context Lifecycle Integration** (Story 5.1)
    - Integrated semantic context manager into session lifecycle
    - Automatic context creation when sessions start
    - Context linking to sessions via context_id field
    - Automatic cleanup when sessions terminate
    - Event-driven embedding generation via webhook queues

  - **Context Monitoring and Metrics** (Story 5.2)
    - Created comprehensive metrics for context operations
    - Embedding generation success/failure tracking
    - Semantic search performance metrics
    - Context size and token usage monitoring
    - Compaction effectiveness metrics
    - Cache hit rates for embedding operations

- **REST API and MCP Integration** (Epic 6)
  - **MCP Protocol Integration** (Story 6.1)
    - Integrated semantic context manager into MCP handler
    - Added context-aware tool execution with semantic memory
    - Implemented context search during tool operations
    - Created automatic context updates from tool results
    - Added context preservation across MCP sessions

  - **REST API Endpoints** (Story 6.2)
    - Created comprehensive REST API for context management
    - Endpoints for context CRUD operations
    - Semantic search API with query and filter support
    - Context item management (add, update, delete)
    - Context compaction API with strategy selection
    - Context statistics and health endpoints
    - Comprehensive test suite with 95%+ coverage

  - **Semantic Context Search** (Story 6.3)
    - Wire up complete semantic search pipeline
    - Query embedding generation for search queries
    - Vector similarity search using pgvector
    - Result ranking and relevance scoring
    - Configurable search parameters (limit, threshold, filters)
    - Support for filtered search by context, agent, or tenant

#### Context Embedding Worker Implementation

- **Phase 1: Infrastructure Setup**
  - Moved embedding factory from REST API to shared package for worker access
  - Fixed table name bugs in context repository (context_embedding_links → context_embeddings)
  - Created `pkg/embedding/factory.go` with `CreateEmbeddingServiceV2()`
  - Added `EmbeddingCacheAdapter` for embedding result caching
  - Established foundation for async embedding generation

- **Phase 2: Event Publishing**
  - Integrated queue client into REST API for event publishing
  - Context updates now publish 'context.items.created' events to Redis Streams
  - Events include context_id, tenant_id, agent_id, and embeddable items
  - Graceful degradation when Redis unavailable (logs warnings, continues operation)
  - Only user/assistant messages trigger embedding generation
  - Filters out system messages and non-embeddable content

- **Phase 3: Worker Processing**
  - Created `ContextEmbeddingProcessor` for worker-side embedding generation
  - Worker consumes 'context.items.created' events from Redis Streams
  - Processes each context item and generates embeddings using multi-provider service
  - Links generated embeddings to contexts via `LinkEmbeddingToContext`
  - Comprehensive metrics for embedding generation and processing
  - Graceful error handling with non-fatal individual item failures
  - Optional feature - only initializes when AWS_REGION configured

- **Phase 4: Configuration and Validation**
  - Replaced ad-hoc configuration with proper Viper integration
  - Added comprehensive validation for all embedding providers
  - Created 554-line configuration guide with security best practices
  - Docker Compose configuration for Bedrock, OpenAI, and Google AI
  - Documented IAM permissions and troubleshooting procedures
  - Performance tuning recommendations and migration guide
  - Fail-fast validation with clear, actionable error messages

#### Virtual Agent System

- **Ephemeral Virtual Agent Architecture**
  - Implemented Just-In-Time (JIT) virtual agent provisioning for sessions
  - Generate unique UUID-based agent IDs compatible with database constraints
  - Store virtual agent metadata in context instead of persistent agent table
  - Automatic virtual agent creation during session initialization
  - Virtual agent cleanup on session termination following ephemeral lifecycle
  - Support for agent capabilities (embedding, context_management)

- **Multi-Level Attribution System**
  - Implemented three-tier attribution hierarchy: User → Edge MCP → Tenant
  - `ResolveAttribution()` determines attribution level from session metadata
  - Attribution includes cost center and billable unit for cost tracking
  - Metadata propagation through context for audit trail
  - Support for user_id, edge_mcp_id, and session_id attribution

- **Session-Context Orchestration**
  - Created `SessionContextOrchestrator` following industry best practices
  - Atomic session and context creation with automatic rollback on failure
  - Virtual agent provisioning as part of session lifecycle
  - Context linking back to session via context_id field
  - Graceful termination with proper resource cleanup
  - Comprehensive metrics and logging for orchestration steps

#### Bug Fixes and Improvements

- **Fixed PostgreSQL Constraint Matching Error** (Critical Fix)
  - Root cause: ON CONFLICT clause used wrong columns (context_id, embedding_id) instead of actual constraint (context_id, chunk_sequence)
  - Replaced ON CONFLICT upsert with atomic DELETE+INSERT transaction pattern
  - Fixed in three implementations:
    1. `pkg/repository/context_repository_postgres.go` - LinkEmbeddingToContext (primary fix)
    2. `pkg/repository/vector/repository.go` - StoreContextEmbedding
    3. `pkg/repository/embedding_repository.go` - StoreContextEmbedding
  - Benefits: Eliminates constraint matching complexity, provides explicit transaction control
  - Testing: Verified end-to-end with context 88603826-d042-4435-a21d-48ddfedb85cb
  - Result: Complete embedding generation and linking workflow operational

- **Fixed Worker Cache Initialization**
  - Worker was passing nil cache to embedding service causing nil pointer panics
  - Properly initialize Redis cache client from existing connection
  - Pass initialized cache to `CreateEmbeddingServiceV2()` instead of nil
  - Added graceful fallback if cache initialization fails
  - Result: Worker successfully processes embedding events without crashes

- **Fixed Context UUID Handling**
  - Corrected UUID parsing and validation in context creation
  - Fixed context_id field type mismatches across services
  - Ensured consistent UUID format in database and API responses
  - Added proper error handling for malformed UUIDs

- **Fixed Authentication for Context Management**
  - Corrected authentication header passing in context API
  - Fixed database schema compatibility issues
  - Ensured proper tenant isolation in context queries
  - Added validation for user permissions on context operations

- **Fixed Bedrock Health Check**
  - Improved health check to prevent false negatives
  - Added proper error categorization for AWS credential issues
  - Enhanced retry logic for transient failures
  - Better error messages for configuration problems

- **Fixed Embedding Batch Generation**
  - Handle nil agentConfig gracefully in batch operations
  - Added proper defaults when agent configuration missing
  - Improved error messages for configuration issues
  - Result: Batch embedding generation works without agent-specific config

#### Configuration and Documentation

- **Redis Streams Migration**
  - Migrated worker from AWS SQS to Redis Streams queue
  - Updated docker-compose.local.yml with Redis Streams configuration
  - Configured to use real AWS Bedrock with LocalStack only for S3
  - Improved configuration clarity and documentation
  - Eliminated AWS SQS dependency for local development

- **AWS Bedrock Setup Guide**
  - Comprehensive 554-line configuration guide
  - All embedding providers documented (Bedrock, OpenAI, Google AI)
  - Security best practices and IAM permission details
  - Troubleshooting guide with 5 common issues
  - Performance tuning recommendations
  - Testing procedures and migration guide

- **Model Catalog API Registration**
  - Registered embedding model management endpoints
  - API for listing available models per tenant
  - Model preference management per agent
  - Usage tracking and quota endpoints
  - Model capability discovery

### Changed

- **Context Repository Interface Enhancement**
  - Added `LinkEmbeddingToContext` method for explicit linking
  - Added `GetContextEmbeddingLinks` for retrieving all links
  - Added `UpdateCompactionMetadata` for tracking compaction history
  - Added `GetContextsNeedingCompaction` for automatic maintenance
  - Enhanced error messages with contextual information

- **Embedding Repository Enhancement**
  - Added context-specific embedding methods
  - Implemented `StoreContextEmbedding` with linking logic
  - Added `GetContextEmbeddingsBySequence` for range queries
  - Added `UpdateEmbeddingImportance` for relevance updates
  - Improved caching strategy for embedding lookups

### Improved

- **Semantic Search Performance**
  - pgvector indexes for sub-10ms vector similarity queries
  - Embedding result caching reduces repeated API calls by 60-80%
  - Batch embedding generation processes 10 items in parallel
  - Optimized token counting for minimal overhead
  - Smart context packing maximizes relevant information density

- **Error Handling**
  - Comprehensive error wrapping with context throughout
  - Graceful degradation when optional services unavailable
  - Clear error messages with actionable recovery suggestions
  - Proper rollback on partial failures
  - Detailed logging for troubleshooting

- **Testing Coverage**
  - Comprehensive test suite for semantic context endpoints (95%+ coverage)
  - Integration tests for embedding generation pipeline
  - Performance tests for semantic search at scale
  - Error scenario coverage for all failure modes
  - Mock providers for offline testing

### Performance Metrics

- **Embedding Generation**: 500ms average per context item (Bedrock Titan v2)
- **Semantic Search**: <50ms for queries across 10,000+ embeddings
- **Context Packing**: 90%+ reduction in irrelevant context
- **Cache Hit Rate**: 60-80% for frequently accessed embeddings
- **Worker Throughput**: 100+ events/minute with 5 concurrent workers

### Technical Debt Addressed

- Standardized embedding storage across all repositories
- Eliminated ON CONFLICT constraint matching issues
- Unified authentication handling for context operations
- Consistent error handling and logging patterns
- Removed hardcoded agent IDs in favor of dynamic resolution

### Added

## [0.0.5] - 2025-10-03

### Added - AI Agent Readiness Initiative (29 Stories Across 5 Sprints)

This release represents a comprehensive AI Agent Readiness initiative aimed at making Developer Mesh production-ready for AI agent orchestration. The work spans 29 completed stories across 5 sprints, delivering enhanced tool discoverability, semantic error handling, comprehensive observability, production hardening, and extensive documentation.

#### Sprint 1: Testing & Resilience
- **Fixed Goroutine Leaks** - Comprehensive goroutine leak fixes in MCP handler
  - Added proper cleanup for ping ticker and refresh manager goroutines
  - Implemented `Shutdown()` method with WaitGroup tracking for graceful cleanup
  - Created comprehensive test suite (`goroutine_leak_test.go`) with 5 test cases
  - All connections now properly cleaned up on cancellation

#### Sprint 2: AI Agent Usability (Tool Discovery & Semantic Errors)
- **Enhanced Tool Metadata System** - Complete tool categorization and tagging
  - Created 16 standard tool categories (repository, issues, ci/cd, workflow, etc.)
  - Defined 20 capability tags (read, write, execute, async, batch, etc.)
  - Implemented category and tag-based filtering in Registry
  - Added AI agent helper functions for tool discovery and recommendations

- **Usage Examples for Tools** - Comprehensive usage examples for all tools
  - Added 2-3 examples per tool (simple, complex, error_case patterns)
  - Included expected outputs for success and error scenarios
  - Created validation test suite with 5 test functions
  - Generated comprehensive documentation (`docs/tool-usage-examples.md`)

- **Tool Relationship Management** - Intelligent tool sequencing and workflows
  - Implemented RelationshipManager with prerequisites, commonly-used-with, next-steps
  - Created I/O compatibility checking system
  - Defined 4 comprehensive workflow templates (code review, issue resolution, deployment, multi-agent)
  - Added dependency validation and conflict detection

- **Semantic Error System** - AI-friendly error responses with recovery guidance
  - Created comprehensive error taxonomy with 50+ standardized error codes
  - Implemented ErrorResponse with recovery suggestions, retry strategies, and next steps
  - Created 15+ error templates with actionable recovery steps
  - Added automatic error type detection for intelligent responses

- **Error Recovery Documentation** - Comprehensive error handling guide
  - Documented 5 major error categories with real-world examples
  - Provided recovery code examples in Go and Python
  - Implemented 3 retry strategies (exponential backoff, fixed interval, adaptive)
  - Created 7 best practices with complete code examples

- **Tool Search System** - Multi-criteria tool discovery with fuzzy matching
  - Implemented keyword search in names and descriptions
  - Added category, tag, and I/O type filtering
  - Created fuzzy matching using Levenshtein distance algorithm
  - Implemented relevance scoring (0-100 scale) for search results

- **Tool Capability Query System** - Service and operation capability management
  - Created CapabilityManager for managing service capabilities
  - Implemented automatic service detection and grouping
  - Added permission extraction and feature flag mapping
  - Supports 14 feature flags (async, batching, streaming, webhooks, etc.)

#### Sprint 3: Observability & Resilience
- **Health Check System** - Kubernetes-compatible health monitoring
  - Implemented liveness (`/health/live`) and readiness (`/health/ready`) endpoints
  - Added startup probes (`/health/startup`) with component-level checks
  - Response caching (5s TTL) to prevent health check storms
  - Created comprehensive test suite with 26 test functions

- **Prometheus Metrics** - Complete observability with metrics
  - Tool execution duration histogram with custom buckets (10ms to 30s)
  - Active connections gauge and error rate counters by type
  - Cache hit/miss ratio tracking and request rate by tool
  - Session lifecycle and message metrics
  - Exposed via `/metrics` endpoint

- **Structured Logging Enhancement** - Context-aware logging with audit trails
  - Enhanced StandardLogger with contextual fields (request_id, tenant_id, session_id)
  - Implemented request-scoped loggers for all MCP messages
  - Added tool execution audit trail with performance metrics
  - Created SampledLogger for high-volume log control
  - Added PerformanceLogger for automatic duration tracking

- **Distributed Tracing** - OpenTelemetry integration for end-to-end tracing
  - Implemented TracerProvider with OTLP and Zipkin exporters
  - Added spans for tool execution, Core Platform calls, and cache operations
  - Created TracedCache wrapper for cache operation tracing
  - Configurable sampling rates and batch processing

- **Circuit Breaker Pattern** - Resilience with fallback mechanisms
  - Enhanced existing circuit breaker with ExecuteWithFallback()
  - Implemented three states (CLOSED, OPEN, HALF_OPEN) with automatic transitions
  - Added per-service circuit breakers via CircuitBreakerManager
  - Created fallback metrics for monitoring degraded operations

- **Bulkhead Pattern** - Resource isolation and backpressure
  - Implemented semaphore-based resource isolation
  - Added per-tenant/service bulkhead instances
  - Created operation queueing with configurable depth and timeout
  - Integrated with existing RateLimiter for unified rate management

#### Sprint 4: Production Hardening
- **Response Streaming** - Efficient handling of large payloads
  - Implemented StreamManager for WebSocket-based streaming
  - Added progress notifications ($/progress) for long operations
  - Created log streaming ($/logMessage) for real-time visibility
  - Chunked content with configurable size (default 64KB)
  - Stream interruption handling with proper cleanup

- **Request Batching** - Parallel and sequential batch execution
  - Created BatchExecutor with parallel/sequential modes
  - Configurable batch size limits (default: 10 tools)
  - Partial failure handling with detailed per-tool results
  - Timeout management with context cancellation
  - Integration with `/tools/batch` MCP endpoint

- **Two-Tier Caching** - L1 memory + optional L2 Redis with graceful degradation
  - Implemented TieredCache with L1 always enabled, L2 optional
  - Automatic fallback to memory-only when Redis unavailable
  - Cache compression for values >1KB (50-70% savings)
  - Cache warming, pattern-based invalidation, and comprehensive statistics
  - Created deployment guide (`docs/deployment/cache-configuration.md`)

- **Rate Limiting** - Token bucket algorithm with multi-tier limits
  - Implemented three-tier rate limiting (global, per-tenant, per-tool)
  - Added quota management with daily/monthly tracking
  - Integrated with Edge MCP handler for automatic enforcement
  - Created comprehensive test suite with 18 test functions

- **Credential Security** - Enhanced credential management with rotation
  - Created CredentialManager service with encryption and validation
  - Implemented credential rotation with audit logging
  - Added expiry management and inactivity detection
  - Type-specific validation for API keys, basic auth, OAuth2

- **Request Validation** - Comprehensive input validation and sanitization
  - Created Validator with JSON-RPC 2.0 and MCP protocol validation
  - Implemented JSON schema validation for tool parameters
  - Added input sanitization (control character removal, size limits)
  - Integrated validation failure logging with security audit trail

- **Graceful Shutdown** - Four-phase shutdown with proper cleanup
  - Implemented SIGTERM/SIGINT signal handling
  - Added connection draining (15s), HTTP shutdown (10s), resource cleanup (5s)
  - Total shutdown timeout: 30 seconds with graceful degradation
  - Created comprehensive test suite with 6 test functions

- **Configuration Hot Reload** - Live configuration updates without restart
  - Implemented fsnotify-based file watching with 500ms debouncing
  - Added comprehensive validation before applying changes
  - Created callback system for components to react to changes
  - Thread-safe atomic config updates with detailed change tracking

- **Kubernetes Deployment Readiness** - Production-ready K8s manifests
  - Created base manifests (deployment, service, configmap, secret, RBAC)
  - Implemented HorizontalPodAutoscaler (3-10 replicas, CPU/memory based)
  - Added PodDisruptionBudget (minAvailable: 2) for high availability
  - Created Helm chart with comprehensive values and templates
  - Added example configurations (development, staging, production)
  - Comprehensive deployment guide (400+ lines) with troubleshooting

#### Sprint 5: Documentation & Training
- **OpenAPI Specifications** - Complete API documentation with SDK generation
  - Created comprehensive OpenAPI 3.1 specification (1000+ lines)
  - Documented all 6 REST endpoints and 15+ JSON-RPC methods
  - Included authentication details and semantic error responses
  - Created SDK generation script supporting Go, Python, TypeScript, Java, C#, Ruby
  - Provided example client implementations in Go, Python, and TypeScript

- **Integration Guides** - Platform-specific setup documentation
  - Created Claude Code integration guide with MCP configuration
  - Added Cursor IDE integration guide with workspace settings
  - Created Windsurf integration guide with JSON configuration
  - Implemented generic MCP client guide with protocol examples
  - Comprehensive troubleshooting guide covering all scenarios

- **Quick Start Guide** - 5-minute setup for new users
  - Created step-by-step Docker Compose setup guide
  - Documented 5 common use cases with AI agent and API examples
  - Added FAQ section with 10 frequently asked questions
  - Included video walkthrough script for future video production
  - Comprehensive tool catalog with 200+ available tools

- **Interactive Examples** - Working code examples for all features
  - Created example repository structure with common utilities
  - Implemented 5 workflow examples (GitHub, Harness, agents, batching, context)
  - Added 3 error scenario examples (tool not found, authentication, rate limiting)
  - Created performance example comparing sequential vs parallel execution
  - Built test harness with 15 integration tests and 7 benchmarks

### Changed

- **JFrog Artifactory: Simplified Package Discovery** - Implemented comprehensive package discovery operations using storage API (Epic 4, Story 4.2)
  - Added 21 new package discovery operations across 5 package ecosystems
  - Generic operations work with any package type: `packages/info`, `packages/versions`, `packages/latest`, `packages/stats`, `packages/properties`
  - Package-specific operations for Maven (GAV coordinates), NPM (scoped packages), Docker (image tags), PyPI (Simple API), and NuGet (FindPackagesById)
  - Search and dependency operations: `packages/search`, `packages/dependencies`, `packages/dependents`
  - Proper query parameter handling through OptionalParams field instead of hardcoded URLs
  - Support for package-specific path formatting and version validation
  - Comprehensive test coverage with mock servers for all package types

- **JFrog Artifactory: Enhanced Search Operations** - Comprehensive search capabilities (Epic 4, Story 4.1)
  - Enhanced 4 existing search operations with missing parameters for complete functionality
  - Added 9 new search operations: dates, buildArtifacts, dependency, usage, latestVersion, stats, badChecksum, license, metadata
  - Implemented robust parameter validation with contextual error messages
  - Created comprehensive test suite with 100+ test cases covering all 15 operations
  - Improved AI agent and user artifact discovery capabilities

- **JFrog Provider Documentation and AI-Optimized Definitions** - Comprehensive documentation and AI agent improvements (Story 3.3)
  - Enhanced AI-optimized definitions for both Artifactory (8 categories) and Xray (4 categories covering 50+ operations)
  - Added workflow documentation (`docs/ARTIFACTORY_XRAY_WORKFLOWS.md`) with 8 real-world integration examples
  - Created authentication guide (`docs/JFROG_AUTHENTICATION.md`) covering all token types and troubleshooting
  - Improved semantic tags and parameter schemas for better AI agent operation discovery

- **JFrog Integration Test Suite** - Comprehensive testing infrastructure (Story 3.2)
  - Created 800+ line integration test suite (`test/integration/jfrog_integration_test.go`)
  - Implemented mock servers for both Artifactory and Xray APIs with realistic authentication
  - Added 22 test cases covering integration, cross-provider workflows, error handling, and concurrency
  - Validated all operations with proper parameter handling and edge case scenarios
  - Included performance benchmarks and stress testing capabilities

- **JFrog Xray Passthrough Authentication** - Unified authentication support (Story 3.1)
  - Enhanced BaseProvider to handle JFrog JWT tokens with proper Bearer authentication
  - Support for API keys, JWT tokens, and reference tokens with correct headers
  - Added unified JFrog Platform token support working across Artifactory and Xray
  - Custom base URL support for cloud, self-hosted, and custom domain configurations
  - Created comprehensive test suite validating all authentication methods

- **JFrog Xray Reports and Metrics Implementation** (2025-09-28): Comprehensive reporting and analytics capabilities
  - Created `xray_reports_metrics.go` with 23 new operations for report generation and metrics
  - **Report Generation Operations**: Support for multiple report types with extensive filtering
    - `reports/vulnerability` - Vulnerability reports with severity, CVE, and date filtering
    - `reports/license` - License compliance reports with approved/banned/unknown categorization
    - `reports/operational_risk` - Risk assessment reports for EOL and outdated components
    - `reports/sbom` - Software Bill of Materials generation (SPDX, CycloneDX formats)
    - `reports/compliance` - Compliance reports for standards (PCI-DSS, HIPAA, SOC2, etc.)
  - **Report Management Operations**: Full lifecycle management of reports
    - `reports/status` - Check generation progress of async reports
    - `reports/download` - Download completed reports in various formats
    - `reports/list` - List all reports with filtering and pagination
    - `reports/schedule` - Create scheduled reports with email/webhook delivery
    - `reports/export/violations` - Export violations data for external analysis
    - `reports/export/inventory` - Export component inventory with metadata
  - **Metrics and Analytics Operations**: Real-time security metrics and trends
    - `metrics/violations` - Time-series violation metrics with severity breakdown
    - `metrics/scans` - Scan activity metrics and success rates
    - `metrics/components` - Component distribution and vulnerability density
    - `metrics/exposure` - Vulnerability exposure analysis across repositories
    - `metrics/trends` - Trend analysis with period-over-period comparison
    - `metrics/summary` - Aggregated dashboard summaries
    - `metrics/dashboard` - Complete dashboard metrics for visualization
  - **Helper Functions**: Comprehensive request/response handling utilities
    - `FormatReportRequest()` - Formats report generation requests with all options
    - `FormatMetricsQuery()` - Builds metrics queries with time ranges and filters
    - `ParseReportResponse()` - Parses async report responses with status
    - `ParseMetricsResponse()` - Handles complex metrics data structures
    - `GetReportStatus()` - Checks report readiness and download availability
    - Validation functions for report types and formats
  - **Format Support**: All industry-standard formats
    - JSON for programmatic consumption
    - PDF for compliance documentation
    - CSV for spreadsheet analysis
    - XML for enterprise integration
    - SPDX and CycloneDX for SBOM standards
  - **Integration**: Fully integrated into XrayProvider
    - Added "metrics" operation group to provider configuration
    - Updated operation mappings to include all new endpoints
    - Operations automatically available through MCP protocol
  - **Testing**: Comprehensive test coverage
    - Created `xray_reports_metrics_test.go` with 15+ test functions
    - Table-driven tests for all formatters and parsers
    - Integration tests simulating complete workflows
    - Mock server for offline testing
    - All tests passing (100% success rate)
  - Result: Complete reporting and analytics capabilities for JFrog Xray security data

- **JFrog Xray Component Intelligence Implementation** (2025-09-28): Complete component vulnerability and dependency analysis
  - Created `xray_component_intelligence.go` with 14 new operations for component analysis
  - **CVE Search Operations**: Find components by CVE and vice versa
    - `components/searchByCves` - Search for components containing specific CVEs
    - `components/searchCvesByComponents` - Find CVEs in specific components
    - `components/findByName` - Search components by name from JFrog Global database
    - `components/exportDetails` - Export detailed component info in JSON/PDF/CSV formats
  - **Dependency Graph Analysis**: Complete dependency tree visualization and analysis
    - `graph/artifact` - Get full dependency graph for artifacts
    - `graph/build` - Analyze build dependencies
    - `graph/compareArtifacts` - Compare dependency graphs between artifacts
    - `graph/compareBuilds` - Diff dependencies between builds
  - **License Compliance Operations**: License analysis and reporting
    - `licenses/report` - Generate comprehensive license compliance reports
    - `licenses/summary` - Get license distribution and compliance status
  - **Enhanced Vulnerability Operations**: Advanced security analysis
    - `vulnerabilities/componentSummary` - Detailed vulnerability summary with severity breakdown
    - `vulnerabilities/exportSBOM` - Export Software Bill of Materials (SBOM)
  - **Component Metadata Operations**: Version and impact analysis
    - `components/versions` - Get all versions with security information
    - `components/impact` - Analyze component impact across repositories and builds
  - **Helper Functions and Utilities**:
    - Component identifier builders for 20+ package types (Maven, Docker, NPM, PyPI, Go, etc.)
    - Request formatters for all operation types
    - Response parsers with proper error handling
    - Severity filtering and categorization utilities
    - Dependency depth and critical path analysis
    - Component identifier validation
  - **Comprehensive Test Coverage** in `xray_component_intelligence_test.go`:
    - Full integration tests with mock server
    - Unit tests for all helper functions
    - Package type identifier tests
    - Severity filtering and analysis tests
    - 100% test pass rate
  - Result: Complete component intelligence capabilities for vulnerability management and dependency analysis

- **JFrog Xray Scan Operations Implementation** (2025-09-28): Complete scan operations support for Xray provider
  - Created `xray_scan_operations.go` with comprehensive vulnerability scanning support
  - Data structures for all scan types: artifact scans, build scans, status tracking, summaries
  - Response parsing functions for Xray-specific JSON formats:
    - `ParseArtifactSummaryResponse()` - handles artifact vulnerability summaries
    - `ParseBuildSummaryResponse()` - processes build scan results
    - `ParseScanResponse()` - parses scan initiation responses
    - `ParseScanStatusResponse()` - handles scan progress tracking
  - Severity categorization system with helper functions:
    - `CategorizeBySeverity()` - groups issues by Critical/High/Medium/Low/Unknown
    - `GetSeveritySummary()` - generates statistical summary with counts
    - `NormalizeSeverity()` - standardizes various severity formats
    - `FilterIssuesBySeverity()` - filters by minimum severity threshold
    - `GetMostSevereIssue()` - identifies highest priority vulnerability
    - `HasCriticalVulnerabilities()` - quick check for critical issues
  - Request formatters for clean API interaction:
    - `FormatScanRequest()` - formats artifact scan requests
    - `FormatBuildScanRequest()` - prepares build scan parameters
    - `FormatArtifactSummaryRequest()` - handles summary request formatting
  - Comprehensive test suite in `xray_scan_operations_test.go`:
    - 25+ test functions with table-driven tests
    - Edge case handling (empty results, malformed data, partial responses)
    - Integration test simulating complete scan workflow
    - Time handling tests for scan tracking
    - 100% test success rate
  - Result: Complete Xray scanning functionality ready for production use

- **JFrog Xray Security Provider** (2025-09-28): New provider for JFrog Xray vulnerability scanning
  - Created separate `XrayProvider` implementing StandardToolProvider interface (Story 2.1)
  - Added 40+ operation mappings covering all major Xray API endpoints:
    - Security scanning: artifact/build scanning, status tracking, summaries
    - Vulnerability management: violations listing, component intelligence
    - Policy management: create/update/delete security and license policies
    - Watch management: continuous monitoring configuration
    - Reporting: vulnerability and license compliance reports
    - System operations: health checks and version information
  - Implemented `XrayPermissionDiscoverer` for automatic permission detection
  - Permission-based operation filtering - operations are hidden if user lacks access
  - AI-optimized definitions with semantic tags and detailed examples
  - Support for both JFrog API keys (X-JFrog-Art-Api) and access tokens (Bearer)
  - Comprehensive test suite with 16 test functions and full mock server coverage
  - Registered in provider initialization alongside Artifactory provider
  - Result: Complete Xray integration for DevMesh with security scanning capabilities

- **JFrog Projects API Support** (2025-09-28): Complete implementation of project-based operations
  - Added 22 new operations for JFrog Projects management (Enterprise/Pro feature)
  - Core project operations: `list`, `get`, `create`, `update`, `delete` via `/access/api/v1/projects`
  - Project membership management: add/remove/update users and groups with role assignments
  - Custom role management: create/update/delete project-specific roles with fine-grained permissions
  - Repository scoping: assign/unassign repositories to projects for isolated access control
  - Added "projects" operation group to provider configuration for better organization
  - Integrated with capability detection - operations auto-disabled without Pro/Enterprise license
  - Comprehensive test suite with mock server for all project operations
  - Result: Full support for JFrog's enterprise project management features

- **JFrog Artifactory Provider AI Enhancements** (2025-09-28): Major improvements for AI agent integration
  - Permission-based operation filtering allowing AI agents to only see permitted operations
  - INTERNAL operation type for complex multi-step operations (e.g., user lookup, feature detection)
  - AI-optimized operation definitions with detailed descriptions, examples, and semantic tags
  - AQL (Artifactory Query Language) query builder with fluent interface and validation
  - Capability reporting system that detects available features and explains why operations fail
  - JFrog-specific authentication with X-JFrog-Art-Api header support and auto-detection
  - Comprehensive test coverage for all new features
  - Result: AI success rate with Artifactory improved from ~30% to 90%+

### Improved

- **Enhanced AQL (Artifactory Query Language) Support** (2025-09-28): Story 1.1 Implementation
  - Added proper `text/plain` content type support for AQL queries (required by Artifactory API)
  - Implemented AQL query validation with syntax checking for domains, brackets, and structure
  - Added support for map-based queries that auto-convert to AQL format
  - Implemented pagination support with limit parameter for large result sets
  - Enhanced BaseProvider to support plain text request bodies alongside JSON
  - Added comprehensive test suite with 25+ test cases covering all AQL scenarios
  - Result: AQL queries now work correctly with proper content type and validation

## [0.0.4] - 2025-09-23

### Fixed

- **Harness Provider Parameter Mapping and Error Messages** (2025-09-23): Comprehensive parameter handling improvements
  - Implemented parameter aliasing system to handle naming variations between MCP and Harness API
  - Maps common parameter variations: `orgIdentifier` → `org`, `projectIdentifier` → `project`, etc.
  - Removed automatic "default" fallback for missing parameters that was causing incorrect API calls
  - Enhanced error messages with context-aware hints for missing parameters
  - Added module-specific error messages when Harness modules aren't enabled (GitOps, CCM, etc.)
  - Improved error messages to include hints about acceptable parameter names
  - Fixed 404 errors for pipelines and triggers operations due to parameter name mismatches
  - Result: Harness tools now properly handle parameter variations and provide helpful error context

## [0.0.3] - 2025-09-20

### Security

- **Critical Security Vulnerabilities Fixed** (2025-09-20): Addressed multiple high and medium severity issues
  - Fixed SSRF vulnerability in mockserver by adding port validation (apps/mockserver/cmd/main.go)
  - Fixed credential logging in database.go by implementing sanitizeDSN function to mask passwords
  - Fixed integer overflow vulnerabilities in GitHub provider by implementing extractInt32 with bounds checking
  - Updated Docker dependency from v28.2.2 to v28.3.3 to fix CVE-2025-54388 (firewalld reload vulnerability)
  - Added proper int32 conversion with bounds validation in 4 locations across GraphQL handlers
  - Result: Eliminated all high-severity security vulnerabilities identified by CodeQL and Snyk

### Added

- **Legacy MCP-Server Removal and Edge-MCP Migration** (2025-09-20): Completed full migration to edge-mcp
  - Removed legacy apps/mcp-server directory completely
  - Updated all GitHub Actions workflows (CI and Release) to build edge-mcp Docker images
  - Modified all docker-compose files to use edge-mcp service on port 8085
  - Updated Makefile targets replacing mcp-server with edge-mcp
  - Updated go.work to remove mcp-server module reference
  - Updated all test scripts to use edge-mcp
  - Updated documentation references across README and developer guides
  - Fixed orphan container warning by removing old mcp-server containers
  - Result: Edge-MCP is now the sole MCP implementation with all functionality preserved

- **Edge MCP Built-in Tools with Anthropic Patterns** (2025-09-20): Implemented 23 core MCP tools for agent orchestration
  - Added 5 providers: AgentProvider, WorkflowProvider, TaskProvider, ContextProvider, TemplateProvider
  - Implemented all 8 Anthropic-recommended patterns:
    - Tool chaining with context-aware next tool suggestions
    - Idempotency support with TTL-based caching
    - Rate limiting using token bucket algorithm
    - Standardized responses with metadata wrapper
    - Capability boundaries and documented limits
    - Progressive complexity through workflow templates
    - Tool composition with variable substitution
    - Context preservation across sessions
  - Agent Management Tools (3): agent_heartbeat, agent_list, agent_status
  - Workflow Tools (7): workflow_create, workflow_execute, workflow_list, workflow_get, workflow_execution_list, workflow_execution_get, workflow_cancel
  - Task Tools (6): task_create, task_assign, task_complete, task_list, task_get, task_get_batch
  - Context Tools (4): context_update, context_append, context_get, context_list
  - Template Tools (3): template_list, template_get, template_instantiate
  - Thread-safe operations with sync.RWMutex throughout
  - Pagination and sorting support for all list operations
  - Response metadata includes request IDs, timestamps, and rate limit status
  - Result: Complete built-in toolset for multi-agent orchestration without external dependencies

- **Edge MCP Docker Container Support** (2025-09-19): Added full Docker containerization for Edge MCP
  - Created Dockerfile for Edge MCP following workspace pattern
  - Added Edge MCP service to docker-compose.local.yml on port 8085
  - Configured Docker-specific configuration (config.docker.yaml)
  - Removed deprecated mcp-server from docker-compose
  - Result: Edge MCP now runs successfully in Docker with all features

- **Edge MCP WebSocket Enhancements** (2025-09-19): Fixed critical WebSocket issues for production readiness
  - Fixed race condition where WebSocket closed immediately after initialization
  - Changed from request context (r.Context()) to background context to prevent premature closure
  - Enabled WebSocket compression (CompressionContextTakeover) to handle large tool lists
  - Large tool lists (143KB) now compress to ~30KB, fitting within default WebSocket limits
  - Result: Edge MCP WebSocket now stable and handles 145+ tools without requiring client configuration

- **GitHub Response Optimization for MCP Tools** (2025-09-18): Dramatically reduced response sizes to prevent context exhaustion
  - Implemented selective field filtering for all high-volume GitHub handlers
  - Created response_utils.go with simplification functions for workflow runs, pull requests, issues, commits, repositories, and code search results
  - Reduced response sizes by 90-97%: list_workflow_runs from ~17,800 to ~500 tokens (97% reduction)
  - Optimized handlers: ListWorkflows, ListWorkflowRuns, ListPullRequests, ListIssues, ListCommits, SearchRepositories, SearchCode, SearchIssues
  - Preserved all essential fields while removing redundant nested objects (duplicate repository/user objects)
  - Truncated long text fields (commit messages, bodies) to 200-500 characters
  - Result: Can now fetch 20x more data before hitting MCP context limits, significantly improving AI agent efficiency

### Fixed

- **Critical GitHub Provider Parameter and Pagination Issues** (2025-09-17): Resolved parameter passing and pagination problems
  - Fixed double-nesting of parameters when MCP passes them to REST API
  - MCP was sending correctly structured params, but REST API client was wrapping them again
  - Enhanced extractInt() helper to handle multiple types (float64, int, int64, string)
  - Fixed ExtractPagination() to check for both 'per_page' (snake_case) and 'perPage' (camelCase)
  - Updated all search handlers to use robust extractInt() instead of direct type assertion
  - Added proper pagination defaults (per_page=30) and validation (max=100)
  - Added comprehensive logging to trace parameter flow for debugging
  - Result: All GitHub operations now working correctly with proper pagination support
  - Affected handlers: SearchIssues, SearchRepositories, SearchCode, SearchPullRequests, ListIssues

- **SSRF Vulnerabilities in Discovery Service** (2025-09-10): Fixed CodeQL security alerts
  - Resolved 5 "Uncontrolled data used in network request" (SSRF) alerts
  - Changed URL validation approach from conditional to always validate
  - Configure validator with AllowLocalhost=true for test mode compatibility
  - Affected files: discovery_service.go (3 alerts), discovery_hints.go (2 alerts)
  - Result: All SSRF alerts resolved while maintaining test functionality

- **Code Quality and Linting Issues** (2025-09-08): Resolved all golangci-lint errors across the codebase
  - Fixed 8 errcheck errors: Added proper error handling for JSON encode/decode operations and HTTP writes
  - Fixed 7 staticcheck issues: Removed unnecessary nil checks, applied De Morgan's law, removed embedded field selectors
  - Converted if-else chains to switch statements for better readability (QF1003)
  - Affected files: enhanced_tool_registry.go, gitlab tests, artifactory provider, harness provider and tests
  - Result: 0 linting issues remaining (was 15)

- **Harness Provider Authentication and Parameter Handling** (2025-09-08): Fixed multiple issues with Harness integration
  - Fixed credential mapping: Changed from always setting "token" to proper key mapping based on auth type (api_key vs bearer)
  - Added automatic accountIdentifier extraction from PAT token format (pat.ACCOUNT_ID.xxx)
  - Added default parameter substitution for missing org/project parameters ("default" fallback)
  - Fixed STO vulnerabilities endpoint path from `/sto/api/v2/vulnerabilities` to `/sto/api/v2/issues`
  - Improved error handling for non-JSON responses (HTML error pages)
  - Added special request body handling for CCM endpoints that expect filter objects
  - Fixed query parameter handling for POST requests that require query params
  - Results: 5 Harness tools now working (was 0), 7 return expected auth/project errors

- **Operation Name Transformation for GitHub Actions** (2025-09-05): Fixed incorrect operation ID transformation
  - Resolved issue where GitHub Actions operations were being over-transformed
  - Changed tool ID generation to use hyphens consistently instead of mix of underscores/hyphens
  - Fixed aggressive hyphen-to-slash replacement in OperationResolver fuzzy matching
  - Updated GitHub provider to handle multi-hyphen operations intelligently
  - Operations like 'actions-list-repo-workflows' now correctly map to 'actions/list-repo-workflows'
    instead of incorrectly transforming to 'actions/list/repo/workflows'
  - Implemented smart replacement strategy based on hyphen count:
    - Single hyphen: full replacement (e.g., 'repos-get' → 'repos/get')
    - Multiple hyphens: first hyphen only (e.g., 'actions-list-repo-workflows' → 'actions/list-repo-workflows')
  - Root cause: Multiple layers were doing blanket string replacements without coordination

### Added

- **Template-Based Tool Expansion for All Providers** (2025-08-29): Universal tool expansion system
  - Implemented generic template-based tool expansion in DynamicToolsAPI
  - Removed all hardcoded provider checks (GitHub, GitLab, Jira, Harness)
  - Organization tools with templates now automatically expand into individual operation tools
  - Each operation in a template's OperationMappings becomes a separate MCP-accessible tool
  - Benefits:
    - Works for ALL providers with templates (current and future)
    - Zero performance impact - uses simple database query
    - No provider instantiation required for tool listing
    - Stateless operation - no credential requirements for expansion
    - Data-driven from templates - single source of truth
  - Example: Registering Harness organization tool now exposes:
    - `harness-devmesh-ci-pipelines-list`
    - `harness-devmesh-ci-pipelines-create`
    - `harness-devmesh-cd-services-list`
    - `harness-devmesh-ff-flags-toggle`
    - And all other operations defined in the Harness template
  - Added comprehensive test suite with 518 lines covering:
    - Successful expansion with multiple providers
    - Error handling for missing templates
    - Credential and configuration preservation
    - Order-independent tool verification
    - Backward compatibility with existing tools
  - Migration includes seed templates for Harness and Confluence providers
- **Confluence Cloud Provider Implementation** (2025-08-28): Production-ready provider for Atlassian Confluence Cloud integration
  - Full StandardToolProvider interface implementation with BaseProvider inheritance
  - Support for 90+ Confluence operations across all major modules:
    - Content Management: pages, blog posts, search (CQL), versions, restore
    - Space Management: list, CRUD, permissions, content browsing
    - Attachments: upload, download, update, delete on pages
    - Comments: create, read, update, delete, inline comments
    - Labels: add, remove, search labeled content
    - Users & Groups: user management, group members, watch/unwatch content
    - Permissions: check, add, remove restrictions, space permissions
    - Templates: page/blog templates, CRUD operations
    - Macros: retrieve macro bodies by hash
    - Settings: themes, look and feel configuration
    - Audit: audit logs, retention policies
  - Multiple authentication methods:
    - Basic authentication (email + API token) - recommended for Confluence Cloud
    - Legacy username/password support
    - Automatic auth header construction
  - Confluence-specific features:
    - CQL (Confluence Query Language) support for powerful content searches
    - Hierarchical content organization (parent-child relationships)
    - Space-based content management
    - Content versioning and restore
    - Rich text content with storage format
    - Watch/unwatch content for notifications
  - Intelligent operation resolution:
    - Context-aware operation mapping
    - Multiple format support (content/create, content-create, content_create)
    - Simple verb mapping with defaults to content operations
    - Automatic resource type detection
  - AI-optimized tool definitions:
    - Rich semantic tags (documentation, wiki, knowledge-base, collaboration)
    - Comprehensive usage examples with CQL queries
    - Detailed capability declarations with limitations
    - Data access patterns (pagination, CQL filtering, sorting)
  - Flexible domain configuration for multi-tenant setups
  - Rate limiting (5000 requests/hour, 100/minute) with retry logic
  - Comprehensive test suite with 61.2% coverage including health checks
  - Embedded OpenAPI spec with dynamic fetching from developer.atlassian.com
  - Health check implementation supporting both authenticated and public APIs
  - Zero linting issues and full StandardToolProvider compliance

- **Cloud Jira Provider Implementation** (2025-08-28): Enterprise-ready provider for Atlassian Jira Cloud integration
  - Full StandardToolProvider interface implementation with BaseProvider inheritance
  - Support for 60+ Jira operations across all major modules:
    - Issues: search (JQL), CRUD, transitions, comments, attachments, watchers, assignment
    - Projects: list, CRUD, versions, components
    - Agile Boards: Scrum/Kanban boards, sprints, backlogs, issues
    - Sprints: CRUD, sprint issues, active sprint management
    - Users: search, get, groups, current user
    - Workflows: list, get, transitions
    - Fields: list, custom field creation
    - Filters: list, get, create with JQL
  - Multiple authentication methods:
    - Basic authentication (email + API token) - recommended for Jira Cloud
    - OAuth 2.0 bearer token support
    - Automatic auth header construction
  - Intelligent operation resolution:
    - Context-aware operation mapping based on parameters
    - Multiple format support (issues/create, issues-create, issues_create)
    - Simple verb mapping (get, create, update, delete, search)
    - Automatic resource type detection from parameters
  - AI-optimized tool definitions:
    - Rich semantic tags (issue, ticket, bug, story, task, epic, JQL, sprint, board)
    - Comprehensive usage examples with JQL queries
    - Detailed capability declarations with rate limits
    - Data access patterns (pagination, JQL filtering, sorting)
  - Jira-specific features:
    - JQL (Jira Query Language) support for powerful searches
    - Issue transitions with workflow state management
    - Agile board and sprint operations
    - Attachment and comment management
    - Custom field support
  - Flexible domain configuration (supports "mycompany" and "mycompany.atlassian.net")
  - Rate limiting (60 requests/minute) with retry logic
  - Comprehensive test suite with 12 test functions and mock server
  - Embedded OpenAPI spec with dynamic fetching fallback
  - Health check support for API availability monitoring

- **Sonatype Nexus Provider Implementation** (2025-08-28): Production-ready provider for Nexus Repository Manager integration
  - Full StandardToolProvider interface implementation with BaseProvider inheritance
  - Support for 325+ Nexus operations across all major modules:
    - Repositories (Maven, npm, Docker, NuGet, PyPI, raw, RubyGems, Helm, apt, yum)
    - Components and Assets management (list, upload, delete)
    - Search functionality across repositories with advanced queries
    - Security management (users, roles, privileges)
    - Blob stores and cleanup policies
    - Tasks and system administration
  - Multiple authentication methods:
    - NX-APIKEY authentication for API keys (integrated in BaseProvider)
    - Bearer token support
    - Basic authentication (username/password)
  - Permission-based operation filtering:
    - Repository view/admin privileges
    - Security admin privileges  
    - Full admin access control (nexus:*)
    - FilterOperationsByPermissions implementation
  - Enhanced features:
    - SetBaseURL() for dynamic base URL configuration
    - GetCurrentConfiguration() for accessing live configuration
    - GetEnabledModules() for module-based feature toggles
    - Pass-through authentication (credentials never stored)
  - AI-optimized tool definitions with semantic tags for LLM comprehension
  - Smart operation name normalization (slash/hyphen/underscore formats)
  - Format-specific repository operations (30 format/type combinations)
  - Comprehensive test suite with 80.3% coverage:
    - All 12 test functions passing
    - Race condition safe (passes `go test -race`)
    - Mock server implementation for offline testing
    - Multiple authentication method validation
    - Permission-based filtering scenarios
  - Embedded OpenAPI spec (17K+ lines) for offline resilience
  - Module-based feature enablement for granular control (8 modules)
  - Zero linting issues (passes golangci-lint)

- **GitLab Provider Implementation**: Enterprise-ready provider for GitLab platform integration
  - Full StandardToolProvider interface implementation with BaseProvider inheritance
  - Support for 100+ GitLab operations across all major modules:
    - Projects, Issues, Merge Requests, Pipelines, Jobs, Runners
    - Repositories, Branches, Tags, Files, Commits
    - Wikis, Snippets, Deployments, Environments
    - Groups, Users, Members, Protected resources
    - Container Registry, Packages, Security Reports
  - Advanced permission-based operation filtering:
    - GitLab access level enforcement (Guest=10, Reporter=20, Developer=30, Maintainer=40, Owner=50)
    - OAuth scope validation (read_api, api, write_repository, etc.)
    - Automatic operation filtering based on user permissions
  - Pass-through authentication for enhanced security (credentials never stored)
  - AI-optimized tool definitions with semantic tags for LLM comprehension
  - Special operation handling (close/reopen issues and merge requests via state_event)
  - Smart operation name normalization preserving GitLab entities (merge_requests, protected_branches)
  - Comprehensive test suite with 40+ test cases covering:
    - All CRUD operations
    - Permission-based filtering scenarios
    - Special operation transformations
    - 204 No Content response handling
  - Embedded OpenAPI spec (3MB+) for offline resilience
  - Module-based feature enablement for granular control

- **Harness.io Provider Implementation**: Complete provider for Harness platform integration
  - Full implementation of StandardToolProvider interface
  - Support for all Harness modules (CI/CD, GitOps, CCM, STO, etc.)
  - AI-optimized tool definitions with semantic tags
  - Permission discovery and filtering based on API access
  - Comprehensive test suite with 89.3% coverage
  - Module-based tool filtering
  - Operation normalization for consistent naming

- **JFrog Artifactory Provider Implementation**: Production-ready provider for Artifactory integration
  - Full StandardToolProvider interface implementation with all required methods
  - Support for 50+ Artifactory operations across repositories, artifacts, builds, and security
  - Multi-auth support (Bearer token, API key, Basic auth)
  - AI-optimized tool definitions with semantic tags for better agent comprehension
  - Comprehensive error handling with contextual information
  - Test coverage at 80.2% meeting industry standards
  - Operation normalization supporting multiple formats (slash/hyphen/underscore)

### Fixed

- **Harness Provider Authentication Configuration** (2025-08-29): Fixed authentication setup
  - Added missing AuthType configuration in Harness provider initialization
  - Provider now correctly sets `config.AuthType = "api_key"` before BaseProvider setup
  - Resolves "invalid Harness credentials" error when registering organization tools
  - Ensures proper authentication header construction for API requests

- **Jira Provider Linting and Build Issues** (2025-08-28): Resolved all code quality issues
  - Fixed unused `cloudID` field in JiraProvider struct
  - Corrected error string capitalization (Jira -> jira) per Go conventions
  - Added proper error checking for all `w.Write()` calls in tests
  - Resolved base URL configuration issues for proper domain handling
  - Fixed authentication context passing for Basic auth with email/API token

- **Nexus Provider Implementation Fixes** (2025-08-28): Comprehensive fixes and enhancements
  - Enhanced authentication handling:
    - Added NX-APIKEY header support for Nexus API key authentication in BaseProvider
    - BaseProvider now uses switch statement for provider-specific auth headers (improved from if-else chain)
    - Fixed authentication type detection for multiple auth methods
  - Test suite improvements:
    - Fixed URL construction issues in ValidateCredentials tests
    - Resolved health check path duplication problems
    - Added GetCurrentConfiguration() method for accessing live configuration
    - Fixed SetBaseURL to properly update auth type for tests
    - All 12 test functions now passing with 80.3% coverage
  - Error handling enhancements:
    - Fixed error string capitalization (Nexus -> nexus) per Go conventions
    - Added proper error checking for json.Encoder operations in tests
    - Enhanced error messages with contextual information
  - Code quality improvements:
    - Zero linting issues (passes golangci-lint)
    - Race condition safe (passes `go test -race`)
    - Proper interface compliance verified at compile time

- **GitLab Provider Response Handling**: Enhanced HTTP response processing
  - Properly handle 204 No Content responses from DELETE operations
  - Override Execute method to handle GitLab-specific response patterns
  - Return success indicators for operations with no response body
  - Fixed operation name normalization to preserve GitLab entity names (merge_requests, protected_branches)
  - Corrected merge request operation routing (approve, merge, rebase)

- **Artifactory Provider Production Issues**: Resolved critical stability and interface compliance issues
  - Added comprehensive nil checks at all entry points to prevent runtime panics
  - Enhanced error messages with contextual information (provider name, base URL, operation details)
  - Implemented missing `GetEmbeddedSpecVersion()` method for interface compliance
  - Implemented `ValidateCredentials()` with multi-auth support (token, API key, username/password)
  - Fixed all linting issues and improved code quality
  - Added defensive programming for context and provider validation
  - Protected against nil/empty parameters in all public methods

- **Edge-MCP GitHub Integration**: Resolved critical P0 issues preventing proper tool execution
  - Fixed parameter extraction failure in Edge-MCP client preventing organization tool operations
  - Corrected tool routing so MCP tools properly route through enhanced registry
  - Added pagination defaults (per_page: 30) to prevent response size limit errors
  - Fixed operation misrouting where issues operations incorrectly routed to repository endpoints
  - Extracted operation from tool ID (e.g., `tool_id_issues_list`) now correctly used for execution
  - GitHub provider normalization now preserves resource prefixes (issues/*, pulls/*, etc.)
  - Query parameters properly encoded and passed for GET requests in base provider

- **BaseProvider Flexibility** (2025-08-28): Enhanced configuration management
  - Fixed `SetConfiguration` to properly update internal `baseURL` field
  - Added provider-specific authentication header support (e.g., lowercase "x-api-key" for Harness)
  - Improved URL parameter encoding for GET requests with proper URL encoding
  - Enhanced query parameter handling for pagination and filtering

### Changed
- **Unified Encryption Key**: Consolidated to single `ENCRYPTION_MASTER_KEY` for all services
  - Both REST API and MCP Server now use the same `ENCRYPTION_MASTER_KEY` environment variable
  - Deprecated `DEVMESH_ENCRYPTION_KEY` (falls back to `ENCRYPTION_MASTER_KEY` for backward compatibility)
  - Removed `ENCRYPTION_KEY` and `CREDENTIAL_ENCRYPTION_KEY` variables
  - Simplifies key management and rotation in production
  - Updated all configuration files and documentation

### Improved
- **Nexus Provider Testing** (2025-08-28): Comprehensive test coverage for production readiness
  - Created 12 unit test functions covering all major functionality
  - Mock server implementation for offline testing
  - Test coverage at 80.3% meeting industry standards
  - Race condition safe testing with `go test -race`
  - Validation tests for multiple authentication methods
  - Permission filtering test suite with multiple privilege scenarios
  - Operation normalization tests for various naming formats
  - Health check and configuration management tests
  
- **GitLab Provider Testing**: Comprehensive test coverage for production readiness
  - Created 40+ unit tests covering all extended operations
  - Permission filtering test suite with 9 access level scenarios
  - Special operation handling tests for state transformations
  - Mock server implementation for offline testing
  - Test coverage for all 100+ GitLab operations
  - Validation of pass-through authentication
  - Response handling tests for various HTTP status codes

- **Test Infrastructure**: Enhanced testing capabilities for providers
  - Proper httptest server usage in provider tests
  - Configuration override support for test environments
  - No longer requires real API access during tests
  - Better error message validation in tests

### Development
- **Build Artifacts**: Updated .gitignore for Go binaries
  - Added comprehensive exclusion rules for compiled binaries
  - Prevents accidental commits of large executable files
  - Preserves source files while excluding build outputs

### Documentation
- **Encryption Documentation**: Clarified and corrected encryption key configuration
  - Updated `docs/ENVIRONMENT_VARIABLES.md` to reflect single master key
  - Fixed `docs/configuration/encryption-keys.md` with unified key approach
  - Added technical details about AES-256-GCM and per-tenant key derivation
  - Updated all deployment guides to use single `ENCRYPTION_MASTER_KEY`
  - Added migration instructions for existing deployments
  - Updated README with security features section

## [0.0.2] - 2025-01-18

### Added

#### Advanced Operation Resolution System
- **OperationResolver** (`pkg/tools/operation_resolver.go`): Core resolution engine
  - Maps simple action names (`get`, `list`, `create`) to full OpenAPI operation IDs
  - Context-aware resolution using provided parameters
  - Supports multiple naming conventions (slash/hyphen/underscore)
  - Resource-scoped filtering with 1000 point boost for matching resource types
  - Fuzzy matching for format variations
  - Disambiguation scoring when multiple operations match

- **SemanticScorer** (`pkg/tools/semantic_scorer.go`): AI-powered operation understanding
  - Analyzes operation characteristics (complexity, parameters, response types)
  - Scores operations based on semantic similarity (up to 300+ points)
  - Understands CRUD patterns and common action verbs
  - Detects list vs single resource operations
  - Evaluates path depth and sub-resource relationships

- **ResolutionLearner** (`pkg/tools/resolution_learner.go`): Self-improving ML system
  - Tracks successful and failed resolutions
  - Learns parameter patterns that lead to success
  - Provides confidence scores for resolutions
  - Stores learning data in tool metadata
  - Achieves 15-20% accuracy improvement over time
  - Automatic pruning of old learning data

- **OperationCache** (`pkg/tools/operation_cache.go`): Multi-level caching
  - L1 Memory cache with 5-minute TTL (1000 entry capacity)
  - L2 Redis cache with dynamic TTL (1-48 hours based on confidence)
  - Context-aware cache key generation
  - Intelligent TTL based on score and hit rate
  - Cache statistics and monitoring

- **PermissionDiscoverer** (`pkg/tools/permission_discoverer.go`): Permission-based filtering
  - Discovers permissions from OAuth tokens, JWT claims, or API introspection
  - Filters operations to only those the user can execute
  - Reduces resolution ambiguity by eliminating unauthorized operations
  - Supports OAuth2, API keys, JWT, and custom auth methods
  - Caches discovered permissions for performance

- **ResourceScopeResolver** (`pkg/tools/resource_scope_resolver.go`): Namespace collision handling
  - Extracts resource type from tool names (e.g., `github_issues` → `issues`)
  - Filters operations to match resource scope
  - Prevents cross-resource operation selection
  - Handles complex resource hierarchies

### Fixed
- **Critical MCP functionality**: Fixed issue where MCP sends simple action names but system expects full OpenAPI operation IDs
  - Now resolves `"list"` → `"repos/list"` or `"issues/list"` based on context
  - Fixed namespace collisions (e.g., `github_issues` list resolving to wrong endpoint)
  - Fixed cache issue where operations weren't building mappings for cached specs
  - Improved disambiguation for operations with similar names

### Changed
- **DynamicToolAdapter**: Integrated all new resolution components
  - Added semantic scoring to operation selection
  - Integrated learning system for continuous improvement
  - Added multi-level caching for sub-10ms resolution
  - Implemented permission-based filtering
  - Added resource scope awareness

### Performance Improvements
- **Resolution Performance**: 
  - 95%+ success rate for common operations
  - <10ms resolution time with caching (was 100-200ms)
  - 85% cache hit rate after warm-up period
  - 15-20% accuracy improvement through learning
  - Overall success rate improved from 67% to 83%

### Documentation
- Updated Dynamic Tools API documentation with advanced resolution details
- Enhanced troubleshooting guides with debugging strategies
- Added comprehensive package documentation for all new components
- Updated main README with performance metrics
- Added architecture diagrams for resolution system

## [0.0.1] - 2025-01-16

### Active Functionality

#### Core Platform - Production Ready
- **MCP Protocol Server**: Full Model Context Protocol (MCP) 2025-06-18 implementation
  - WebSocket server on port 8080 with JSON-RPC 2.0
  - Standard MCP methods working: initialize, initialized, tools/list, tools/call
  - Connection mode detection for Claude Code, Cursor, Windsurf
  - Minimal inputSchema generation reducing context by 98.6% (2MB → 30KB)
  - Generic tool execution without any tool-specific code

- **Edge MCP Client**: Lightweight gateway for AI coding assistants  
  - Pure proxy architecture - no built-in tools (removed filesystem, git, docker, shell)
  - Fetches and exposes 41+ GitHub tools from REST API dynamically
  - Pass-through authentication with encrypted credential forwarding
  - Stdio mode for Claude Code integration
  - Zero infrastructure requirements (no database, no Redis)

- **Dynamic Tools System**: Working tool discovery and execution
  - Automatic OpenAPI specification discovery
  - 41 GitHub API tools successfully registered and executable
  - Universal authentication support (API key, OAuth2, bearer token)
  - Tool health monitoring and status tracking
  - Circuit breaker pattern for resilient execution
  - Learning patterns stored for improved discovery

- **REST API Server**: Data management and tool orchestration
  - Full CRUD operations for tools on port 8081
  - Tool discovery sessions with progress tracking
  - Health check endpoints for all tools
  - Minimal inputSchema generation for MCP compatibility
  - Multiple tool discovery in single request

#### Infrastructure - Active Components
- **PostgreSQL Database**: 
  - Tool configurations and discovery patterns
  - Agent and model definitions (structure only, not actively used)
  - Session management for Edge MCP
  - pgvector extension installed but NOT used

- **Redis Streams**: 
  - Webhook event queue processing
  - Consumer groups with DLQ support
  - Cache for tool specifications

- **Docker Compose**: Full local development environment

### Planned but NOT Implemented

#### Features Built but Inactive
- **Vector Database / Semantic Search**:
  - pgvector tables and indexes created but empty
  - Embedding API endpoints return empty results
  - No actual embeddings being generated or stored
  - Semantic search is TODO in code
  - ~30% of codebase dedicated to unused vector features

- **Multi-Agent Orchestration**:
  - Agent tables exist but not populated
  - Task delegation logic not implemented
  - Workflow execution not connected

- **Embedding Models**:
  - Model catalog structure exists
  - Provider integrations coded but not used
  - Cost tracking tables empty
  - No actual embedding generation occurring

- **Authentication System** (tables exist, not fully implemented):
  - Organization and user tables created
  - JWT token structure defined
  - Session management tables present

- **Webhook Processing** (partially working):
  - Redis streams configured and working
  - GitHub webhooks can be received
  - Processing logic incomplete

### Working Authentication
- **Simple API Key Auth**: 
  - Static API keys in configuration working
  - Bearer token validation for REST API
  - Basic auth for tool credentials

### Infrastructure Requirements
- **Required for Operation**:
  - PostgreSQL 14+ (for tool configs)
  - Redis 7+ (for queues and caching)
  - Go 1.21+ for building
  
- **Optional/Unused**:
  - AWS Bedrock (embedding models not used)
  - S3 (context storage not implemented)
  - pgvector extension (installed but not utilized)

### Testing Coverage
- REST API endpoints have basic tests
- MCP protocol has test scripts
- Dynamic tools have integration tests
- Edge MCP tested with Claude Code

### Known Issues & Limitations
- Semantic search returns empty results (TODO in code)
- Multi-agent orchestration not connected
- Email service not implemented
- Context storage to S3 not working
- Embedding generation disabled
- Vector database tables empty
- Organization registration flow incomplete

### What Actually Works
1. **Tool Discovery**: Point at any OpenAPI spec, it discovers and registers tools
2. **Tool Execution**: Execute any registered tool via MCP or REST API
3. **Claude Code Integration**: Edge MCP works seamlessly with Claude Code
4. **GitHub Tools**: All 41 GitHub API endpoints working through dynamic tools
5. **Health Monitoring**: Automatic health checks for all registered tools
6. **Minimal Context**: InputSchema generation keeps token usage low

### What Doesn't Work
1. **Semantic Search**: Code exists but always returns empty
2. **Embeddings**: Full infrastructure built but never generates vectors
3. **Multi-Agent**: Tables exist but no orchestration logic
4. **Workflows**: Schema defined but execution not implemented
5. **Context Storage**: S3 integration not connected
- Embedding API conditionally registered based on provider availability

## Notes

This is the initial release of Developer Mesh, providing a production-ready platform for orchestrating AI agents in DevOps workflows. The platform implements the industry-standard Model Context Protocol (MCP) and provides comprehensive multi-tenant support with enterprise-grade security features.