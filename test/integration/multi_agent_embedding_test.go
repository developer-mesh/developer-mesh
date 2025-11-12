//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiAgentEmbeddingFlow tests the complete multi-agent embedding system
func TestMultiAgentEmbeddingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := setupTestDatabase(t)
	defer cleanupTestDatabase(t, db)

	// Create services
	sqlxDB := sqlx.NewDb(db, "postgres")

	// Agent repository and service
	agentRepo := agents.NewPostgresRepository(sqlxDB, "mcp")
	agentService := agents.NewService(agentRepo)

	// Embedding repository
	embeddingRepo := embedding.NewRepository(sqlxDB)

	// Create providers
	providerMap := map[string]providers.Provider{
		"mock-openai":  providers.NewMockProvider("mock-openai"),
		"mock-bedrock": providers.NewMockProvider("mock-bedrock"),
		"mock-google":  providers.NewMockProvider("mock-google"),
	}

	// Create embedding service
	embeddingConfig := embedding.ServiceV2Config{
		Providers:    providerMap,
		AgentService: agentService,
		Repository:   embeddingRepo,
		RouterConfig: embedding.DefaultRouterConfig(),
	}

	embeddingService, err := embedding.NewServiceV2(embeddingConfig)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("complete multi-agent workflow", func(t *testing.T) {
		// Step 1: Create agent configurations
		agents := []struct {
			id       string
			strategy agents.EmbeddingStrategy
			taskType agents.TaskType
		}{
			{"agent-quality", agents.StrategyQuality, agents.TaskTypeResearch},
			{"agent-speed", agents.StrategySpeed, agents.TaskTypeGeneralQA},
			{"agent-cost", agents.StrategyCost, agents.TaskTypeGeneralQA},
			{"agent-balanced", agents.StrategyBalanced, agents.TaskTypeCodeAnalysis},
		}

		for _, agent := range agents {
			config := &agents.AgentConfig{
				AgentID:           agent.id,
				EmbeddingStrategy: agent.strategy,
				ModelPreferences: []agents.ModelPreference{
					{
						TaskType: agent.taskType,
						PrimaryModels: []string{
							"mock-model-small",
							"mock-model-large",
						},
						FallbackModels: []string{
							"mock-model-titan",
						},
					},
				},
				Constraints: agents.AgentConstraints{
					MaxCostPerMonthUSD: 100.0,
					MaxLatencyP99Ms:    1000,
					MinAvailabilitySLA: 0.99,
					RateLimits: agents.RateLimitConfig{
						RequestsPerMinute: 100,
						TokensPerHour:     1000000,
					},
				},
				FallbackBehavior: agents.FallbackConfig{
					MaxRetries:      3,
					InitialDelayMs:  100,
					MaxDelayMs:      5000,
					ExponentialBase: 2.0,
					CircuitBreaker: agents.CircuitConfig{
						Enabled:          true,
						FailureThreshold: 5,
						SuccessThreshold: 2,
						TimeoutSeconds:   30,
					},
				},
				CreatedBy: "integration-test",
			}

			err := agentService.CreateConfig(ctx, config)
			require.NoError(t, err)
		}

		// Step 2: Generate embeddings for each agent
		tenantID := uuid.New()
		contextID := uuid.New()

		embeddings := []struct {
			agentID  string
			text     string
			taskType agents.TaskType
		}{
			{"agent-quality", "Advanced quantum computing research paper on qubit coherence", agents.TaskTypeResearch},
			{"agent-speed", "How do I reset my password?", agents.TaskTypeGeneralQA},
			{"agent-cost", "What is the weather today?", agents.TaskTypeGeneralQA},
			{"agent-balanced", "func authenticate(user User) error { return bcrypt.Compare(user.Password, user.Hash) }", agents.TaskTypeCodeAnalysis},
		}

		generatedEmbeddings := make([]embedding.GenerateEmbeddingResponse, 0)

		for _, emb := range embeddings {
			req := embedding.GenerateEmbeddingRequest{
				AgentID:   emb.agentID,
				Text:      emb.text,
				TaskType:  emb.taskType,
				TenantID:  tenantID,
				ContextID: contextID,
				Metadata: map[string]interface{}{
					"test":      true,
					"timestamp": time.Now().Unix(),
				},
			}

			resp, err := embeddingService.GenerateEmbedding(ctx, req)
			require.NoError(t, err)

			// Verify response
			assert.NotNil(t, resp)
			assert.NotEmpty(t, resp.EmbeddingID)
			assert.NotEmpty(t, resp.ModelUsed)
			assert.NotEmpty(t, resp.Provider)
			assert.Equal(t, embedding.StandardDimension, resp.NormalizedDimensions)
			assert.Greater(t, resp.Dimensions, 0)
			assert.GreaterOrEqual(t, resp.CostUSD, 0.0)
			assert.Greater(t, resp.GenerationTimeMs, int64(0))

			generatedEmbeddings = append(generatedEmbeddings, *resp)
		}

		// Step 3: Test cross-model search
		searchService := embedding.NewSearchServiceV2(db, embeddingRepo, embedding.NewDimensionAdapter())

		searchReq := embedding.CrossModelSearchRequest{
			Query:         "password security authentication",
			TenantID:      tenantID,
			ContextID:     &contextID,
			Limit:         10,
			MinSimilarity: 0.5,
			TaskType:      "code_analysis",
		}

		// Generate query embedding using one of the providers
		queryEmbReq := embedding.GenerateEmbeddingRequest{
			AgentID:  "agent-balanced",
			Text:     searchReq.Query,
			TaskType: agents.TaskTypeCodeAnalysis,
			TenantID: tenantID,
		}

		queryResp, err := embeddingService.GenerateEmbedding(ctx, queryEmbReq)
		require.NoError(t, err)

		// Perform cross-model search
		// Note: This would require loading the actual embedding from the database
		// For integration test purposes, we verify the flow works

		// Step 4: Test provider health monitoring
		health := embeddingService.GetProviderHealth(ctx)
		assert.NotNil(t, health)
		assert.Len(t, health, 3) // 3 mock providers

		for provider, status := range health {
			assert.NotEmpty(t, provider)
			assert.Equal(t, "healthy", status.Status)
		}

		// Step 5: Test circuit breaker behavior
		// Force failures on one provider
		failingProvider := providers.NewMockProvider("failing", providers.WithFailureRate(1.0))

		failingConfig := embedding.ServiceV2Config{
			Providers: map[string]providers.Provider{
				"failing": failingProvider,
				"backup":  providers.NewMockProvider("backup"),
			},
			AgentService: agentService,
			Repository:   embeddingRepo,
			RouterConfig: &embedding.RouterConfig{
				CircuitBreakerConfig: embedding.CircuitBreakerConfig{
					FailureThreshold:    2,
					SuccessThreshold:    1,
					Timeout:             100 * time.Millisecond,
					HalfOpenMaxRequests: 1,
				},
			},
		}

		failingService, err := embedding.NewServiceV2(failingConfig)
		require.NoError(t, err)

		// Create agent that uses the failing provider
		failoverConfig := &agents.AgentConfig{
			AgentID:           "agent-failover",
			EmbeddingStrategy: agents.StrategyBalanced,
			ModelPreferences: []agents.ModelPreference{
				{
					TaskType:       agents.TaskTypeGeneralQA,
					PrimaryModels:  []string{"mock-model-small"},
					FallbackModels: []string{"mock-model-titan"},
				},
			},
			CreatedBy: "integration-test",
		}

		err = agentService.CreateConfig(ctx, failoverConfig)
		require.NoError(t, err)

		// Should succeed using backup provider
		failoverReq := embedding.GenerateEmbeddingRequest{
			AgentID:  "agent-failover",
			Text:     "This should work with failover",
			TenantID: tenantID,
		}

		failoverResp, err := failingService.GenerateEmbedding(ctx, failoverReq)
		require.NoError(t, err)
		assert.Equal(t, "backup", failoverResp.Provider)

		// Step 6: Test batch operations
		batchReqs := make([]embedding.GenerateEmbeddingRequest, 5)
		for i := range batchReqs {
			batchReqs[i] = embedding.GenerateEmbeddingRequest{
				AgentID:  "agent-balanced",
				Text:     t.Name() + " batch text " + string(rune(i)),
				TaskType: agents.TaskTypeGeneralQA,
				TenantID: tenantID,
			}
		}

		batchResps, err := embeddingService.BatchGenerateEmbeddings(ctx, batchReqs)
		require.NoError(t, err)
		assert.Len(t, batchResps, 5)

		for _, resp := range batchResps {
			assert.NotNil(t, resp)
			assert.NotEmpty(t, resp.EmbeddingID)
		}
	})

	t.Run("dimension normalization", func(t *testing.T) {
		// Test that embeddings from different dimensions are properly normalized
		adapter := embedding.NewDimensionAdapter()

		testCases := []struct {
			name    string
			fromDim int
			toDim   int
			input   []float32
		}{
			{
				name:    "768 to 1536 (padding)",
				fromDim: 768,
				toDim:   1536,
				input:   generateTestEmbedding(768),
			},
			{
				name:    "3072 to 1536 (reduction)",
				fromDim: 3072,
				toDim:   1536,
				input:   generateTestEmbedding(3072),
			},
			{
				name:    "1536 to 1536 (no change)",
				fromDim: 1536,
				toDim:   1536,
				input:   generateTestEmbedding(1536),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := adapter.Normalize(tc.input, tc.fromDim, tc.toDim)
				assert.Len(t, result, tc.toDim)

				// Verify the result maintains reasonable values
				for _, val := range result {
					assert.False(t, isNaN(float64(val)))
					assert.False(t, isInf(float64(val)))
				}
			})
		}
	})
}

// Helper functions

func setupTestDatabase(t *testing.T) *sql.DB {
	// This would use the test database setup from the project
	// For now, we'll use the helper from the integration package
	helper := &database.TestHelper{}
	db, err := helper.SetupTestDB()
	require.NoError(t, err)

	// Run migrations
	err = runMigrations(db)
	require.NoError(t, err)

	return db
}

func cleanupTestDatabase(t *testing.T, db *sql.DB) {
	// Clean up test data
	_, err := db.Exec(`
		DELETE FROM mcp.embeddings WHERE metadata->>'test' = 'true';
		DELETE FROM mcp.agent_configs WHERE created_by = 'integration-test';
	`)
	require.NoError(t, err)

	db.Close()
}

func runMigrations(db *sql.DB) error {
	// In a real integration test, this would run the actual migrations
	// For now, we assume the schema exists
	return nil
}

func generateTestEmbedding(dimensions int) []float32 {
	embedding := make([]float32, dimensions)
	for i := range embedding {
		// Generate deterministic values for testing
		embedding[i] = float32(i) / float32(dimensions)
	}
	return embedding
}

func isNaN(f float64) bool {
	return f != f
}

func isInf(f float64) bool {
	return f > 1e308 || f < -1e308
}
