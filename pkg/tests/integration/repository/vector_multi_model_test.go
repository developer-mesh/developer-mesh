//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	integration "pkg-integration-tests"

	"github.com/developer-mesh/developer-mesh/pkg/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiModelEmbeddings tests the storage and retrieval of embeddings from multiple models
// with different vector dimensions
//
// This is an integration test that requires a real PostgreSQL database with the pgvector extension.
// Set the environment variable MCP_DATABASE_DSN to the DSN of the test database.
func TestMultiModelEmbeddings(t *testing.T) {
	// Create test helpers
	testHelper := integration.NewTestHelper(t)
	dbHelper := integration.NewDatabaseHelper(t)

	// Setup context with timeout
	ctx, cancel := testHelper.Context()
	defer cancel()

	// Setup database connection using the standard DatabaseHelper
	dbConfig := config.DatabaseConfig{
		Driver:          "postgres",
		DSN:             getTestDatabaseDSN(),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}
	// Convert to database.Config
	config := database.FromDatabaseConfig(dbConfig)

	// Try to connect to the database, skip test if connection fails
	dbInstance, err := database.NewDatabase(ctx, config)
	if err != nil {
		t.Skip("Skipping vector multi-model test due to database connection failure: " + err.Error())
		return
	}
	db := dbInstance.GetDB()
	dbHelper.SetupTestDatabaseWithConnection(ctx, db)
	defer dbHelper.CleanupDatabase()

	// Create embedding repository using the adapter which can handle various database types
	repo := repository.NewEmbeddingAdapter(dbInstance)

	// Create a test context ID
	contextID := fmt.Sprintf("test-context-%d", time.Now().UnixNano())

	// Test with multiple models with different dimensions
	testModels := []struct {
		ID        string
		Dimension int
		Count     int
	}{
		{ID: "test.openai.ada-002", Dimension: 1536, Count: 3},
		{ID: "test.anthropic.claude", Dimension: 768, Count: 3},
		{ID: "test.mcp.small", Dimension: 384, Count: 3},
	}

	// Clean up test data after the test
	defer func() {
		err := repo.DeleteContextEmbeddings(context.Background(), contextID)
		assert.NoError(t, err, "Failed to clean up test data")
	}()

	// Store embeddings for each model
	for _, model := range testModels {
		t.Run(fmt.Sprintf("Model_%s", model.ID), func(t *testing.T) {
			// Generate test embeddings
			embeddings := generateTestEmbeddings(contextID, model.ID, model.Dimension, model.Count)

			// Store embeddings
			for i, emb := range embeddings {
				err := repo.StoreEmbedding(context.Background(), emb)
				require.NoError(t, err, "Failed to store embedding %d for model %s", i, model.ID)
			}

			// Retrieve embeddings for the model
			retrievedEmbeddings, err := repo.GetEmbeddingsByModel(context.Background(), contextID, model.ID)
			require.NoError(t, err, "Failed to retrieve embeddings for model %s", model.ID)
			assert.Equal(t, model.Count, len(retrievedEmbeddings), "Expected %d embeddings for model %s, got %d", model.Count, model.ID, len(retrievedEmbeddings))

			// Test vector search
			if len(retrievedEmbeddings) > 0 {
				// Use the first embedding as the query
				queryEmbedding := retrievedEmbeddings[0].Embedding

				// Search for similar embeddings
				searchResults, err := repo.SearchEmbeddings(
					context.Background(),
					queryEmbedding,
					contextID,
					model.ID,
					model.Count,
					0.5,
				)
				require.NoError(t, err, "Failed to search embeddings for model %s", model.ID)
				assert.GreaterOrEqual(t, len(searchResults), 1, "Expected at least 1 search result for model %s", model.ID)

				// Vector similarity results in current implementation don't include a Similarity field
				// Instead, we just verify that we got results
				if len(searchResults) > 0 {
					// The most similar embedding should be the first result
					// We can't check the actual similarity score as it's not exposed in the current implementation
					assert.Equal(t, embeddings[0].ID, searchResults[0].ID, "Expected first result to be the query embedding")
				}

				// Test model isolation - verify we don't get results from other models
				for _, otherModel := range testModels {
					if otherModel.ID == model.ID {
						continue
					}

					// Try to search with a query vector from one model but specifying a different model
					searchResults, err = repo.SearchEmbeddings(
						context.Background(),
						queryEmbedding,
						contextID,
						otherModel.ID,
						model.Count,
						0.5,
					)

					// This should fail or return no results because the dimensions are different
					if err == nil {
						assert.Empty(t, searchResults, "Expected no results when searching with model %s vector but specifying model %s", model.ID, otherModel.ID)
					}
				}
			}
		})
	}

	// Test getting all supported models
	t.Run("GetSupportedModels", func(t *testing.T) {
		models, err := repo.GetSupportedModels(context.Background())
		require.NoError(t, err, "Failed to get supported models")

		// Check that all test models are in the results
		for _, model := range testModels {
			found := false
			for _, m := range models {
				if m == model.ID {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected model %s to be in supported models", model.ID)
		}
	})

	// Test deleting embeddings for a specific model
	t.Run("DeleteModelEmbeddings", func(t *testing.T) {
		// Pick a model to delete
		modelToDelete := testModels[0]

		// Delete embeddings for the model
		err := repo.DeleteModelEmbeddings(context.Background(), contextID, modelToDelete.ID)
		require.NoError(t, err, "Failed to delete embeddings for model %s", modelToDelete.ID)

		// Verify embeddings were deleted
		embeddings, err := repo.GetEmbeddingsByModel(context.Background(), contextID, modelToDelete.ID)
		require.NoError(t, err, "Failed to retrieve embeddings for model %s", modelToDelete.ID)
		assert.Empty(t, embeddings, "Expected no embeddings for model %s after deletion", modelToDelete.ID)

		// Verify other models were not affected
		for _, otherModel := range testModels {
			if otherModel.ID == modelToDelete.ID {
				continue
			}

			embeddings, err := repo.GetEmbeddingsByModel(context.Background(), contextID, otherModel.ID)
			require.NoError(t, err, "Failed to retrieve embeddings for model %s", otherModel.ID)
			assert.Equal(t, otherModel.Count, len(embeddings), "Expected %d embeddings for model %s, got %d", otherModel.Count, otherModel.ID, len(embeddings))
		}
	})
}

// generateTestEmbeddings generates random embeddings for testing
func generateTestEmbeddings(contextID, modelID string, dimension, count int) []*repository.Embedding {
	embeddings := make([]*repository.Embedding, count)

	for i := 0; i < count; i++ {
		// Generate a simple vector where one position has a value of 1.0
		// This makes it easy to create unique but predictable vectors
		vector := make([]float32, dimension)
		vector[i%dimension] = 1.0

		// Create embedding with a unique ID
		embeddings[i] = &repository.Embedding{
			ID:           fmt.Sprintf("%s-%s-%d", contextID, modelID, i),
			ContextID:    contextID,
			ContentIndex: i,
			Text:         fmt.Sprintf("Test embedding %d for model %s", i, modelID),
			Embedding:    vector,
			ModelID:      modelID,
			CreatedAt:    time.Now(),
		}
	}

	return embeddings
}

// getTestDatabaseDSN returns the database DSN from environment variables
func getTestDatabaseDSN() string {
	// Default test database DSN for development
	dsn := "postgres://postgres:postgres@localhost:5432/mcp_test?sslmode=disable"

	// Check for environment variable override
	if envDSN := os.Getenv("MCP_DATABASE_DSN"); envDSN != "" {
		dsn = envDSN
	}
	return dsn
}
