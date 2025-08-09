package model_catalog

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

// ModelCatalogRepositoryTestSuite defines the test suite
type ModelCatalogRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	mock sqlmock.Sqlmock
	repo ModelCatalogRepository
}

// SetupTest runs before each test
func (s *ModelCatalogRepositoryTestSuite) SetupTest() {
	db, mock, err := sqlmock.New()
	s.Require().NoError(err)

	s.db = sqlx.NewDb(db, "postgres")
	s.mock = mock
	s.repo = NewModelCatalogRepository(s.db)
}

// TearDownTest runs after each test
func (s *ModelCatalogRepositoryTestSuite) TearDownTest() {
	if err := s.db.Close(); err != nil {
		// Log error but don't fail the test
		s.T().Logf("Failed to close database: %v", err)
	}
}

// TestGetByID tests retrieving a model by ID
func (s *ModelCatalogRepositoryTestSuite) TestGetByID() {
	ctx := context.Background()
	modelID := uuid.New()

	// Setup expectations
	rows := sqlmock.NewRows([]string{
		"id", "provider", "model_name", "model_id", "model_version",
		"dimensions", "max_tokens", "supports_binary",
		"supports_dimensionality_reduction", "min_dimensions", "model_type",
		"cost_per_million_tokens", "cost_per_million_chars",
		"is_available", "is_deprecated", "deprecation_date",
		"minimum_tier", "requires_api_key", "provider_config",
		"capabilities", "performance_metrics", "created_at", "updated_at",
	}).AddRow(
		modelID, "bedrock", "Titan Embed V2", "amazon.titan-embed-text-v2:0", "2.0",
		1024, 8192, false,
		false, nil, "embedding",
		0.02, nil,
		true, false, nil,
		nil, false, nil,
		nil, nil, time.Now(), time.Now(),
	)

	s.mock.ExpectQuery("SELECT .+ FROM mcp.embedding_model_catalog WHERE id = \\$1").
		WithArgs(modelID).
		WillReturnRows(rows)

	// Execute
	model, err := s.repo.GetByID(ctx, modelID)

	// Assert
	s.NoError(err)
	s.NotNil(model)
	s.Equal(modelID, model.ID)
	s.Equal("bedrock", model.Provider)
	s.Equal("Titan Embed V2", model.ModelName)
	s.Equal(1024, model.Dimensions)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestGetByModelID tests retrieving a model by model_id
func (s *ModelCatalogRepositoryTestSuite) TestGetByModelID() {
	ctx := context.Background()
	modelID := "amazon.titan-embed-text-v2:0"

	// Setup expectations
	rows := sqlmock.NewRows([]string{
		"id", "provider", "model_name", "model_id", "model_version",
		"dimensions", "max_tokens", "supports_binary",
		"supports_dimensionality_reduction", "min_dimensions", "model_type",
		"cost_per_million_tokens", "cost_per_million_chars",
		"is_available", "is_deprecated", "deprecation_date",
		"minimum_tier", "requires_api_key", "provider_config",
		"capabilities", "performance_metrics", "created_at", "updated_at",
	}).AddRow(
		uuid.New(), "bedrock", "Titan Embed V2", modelID, "2.0",
		1024, 8192, false,
		false, nil, "embedding",
		0.02, nil,
		true, false, nil,
		nil, false, nil,
		nil, nil, time.Now(), time.Now(),
	)

	s.mock.ExpectQuery("SELECT .+ FROM mcp.embedding_model_catalog WHERE model_id = \\$1").
		WithArgs(modelID).
		WillReturnRows(rows)

	// Execute
	model, err := s.repo.GetByModelID(ctx, modelID)

	// Assert
	s.NoError(err)
	s.NotNil(model)
	s.Equal(modelID, model.ModelID)
	s.Equal("bedrock", model.Provider)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestListAvailable tests listing available models
func (s *ModelCatalogRepositoryTestSuite) TestListAvailable() {
	ctx := context.Background()
	provider := "bedrock"

	// Setup expectations
	rows := sqlmock.NewRows([]string{
		"id", "provider", "model_name", "model_id", "model_version",
		"dimensions", "max_tokens", "supports_binary",
		"supports_dimensionality_reduction", "min_dimensions", "model_type",
		"cost_per_million_tokens", "cost_per_million_chars",
		"is_available", "is_deprecated", "deprecation_date",
		"minimum_tier", "requires_api_key", "provider_config",
		"capabilities", "performance_metrics", "created_at", "updated_at",
	})

	// Add multiple rows
	for i := 0; i < 3; i++ {
		rows.AddRow(
			uuid.New(), provider, "Model"+string(rune(i)), "model-"+string(rune(i)), "1.0",
			1024, 8192, false,
			false, nil, "embedding",
			0.02, nil,
			true, false, nil,
			nil, false, nil,
			nil, nil, time.Now(), time.Now(),
		)
	}

	s.mock.ExpectQuery("SELECT .+ FROM mcp.embedding_model_catalog WHERE is_available = true AND is_deprecated = false AND provider = \\$1").
		WithArgs(provider).
		WillReturnRows(rows)

	// Execute
	filter := &ModelFilter{
		Provider: &provider,
	}
	models, err := s.repo.ListAvailable(ctx, filter)

	// Assert
	s.NoError(err)
	s.Len(models, 3)
	for _, model := range models {
		s.Equal(provider, model.Provider)
		s.True(model.IsAvailable)
		s.False(model.IsDeprecated)
	}

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestCreate tests creating a new model
func (s *ModelCatalogRepositoryTestSuite) TestCreate() {
	ctx := context.Background()

	model := &EmbeddingModel{
		ID:             uuid.New(),
		Provider:       "openai",
		ModelName:      "Text Embedding 3",
		ModelID:        "text-embedding-3-small",
		Dimensions:     1536,
		IsAvailable:    true,
		IsDeprecated:   false,
		RequiresAPIKey: true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Setup expectations
	s.mock.ExpectExec("INSERT INTO mcp.embedding_model_catalog").
		WithArgs(
			model.ID, model.Provider, model.ModelName, model.ModelID,
			sqlmock.AnyArg(), model.Dimensions, sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), model.IsAvailable, model.IsDeprecated,
			sqlmock.AnyArg(), sqlmock.AnyArg(), model.RequiresAPIKey, sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute
	err := s.repo.Create(ctx, model)

	// Assert
	s.NoError(err)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestUpdate tests updating a model
func (s *ModelCatalogRepositoryTestSuite) TestUpdate() {
	ctx := context.Background()

	model := &EmbeddingModel{
		ID:           uuid.New(),
		Provider:     "openai",
		ModelName:    "Updated Model",
		ModelID:      "updated-model",
		Dimensions:   2048,
		IsAvailable:  false,
		IsDeprecated: true,
		UpdatedAt:    time.Now(),
	}

	// Setup expectations
	s.mock.ExpectExec("UPDATE mcp.embedding_model_catalog SET").
		WithArgs(
			model.Provider, model.ModelName, model.ModelID,
			sqlmock.AnyArg(), model.Dimensions, sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), model.IsAvailable, model.IsDeprecated,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			model.ID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err := s.repo.Update(ctx, model)

	// Assert
	s.NoError(err)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestDelete tests deleting a model
func (s *ModelCatalogRepositoryTestSuite) TestDelete() {
	ctx := context.Background()
	modelID := uuid.New()

	// Setup expectations
	s.mock.ExpectExec("DELETE FROM mcp.embedding_model_catalog WHERE id = \\$1").
		WithArgs(modelID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err := s.repo.Delete(ctx, modelID)

	// Assert
	s.NoError(err)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestSetAvailability tests setting model availability
func (s *ModelCatalogRepositoryTestSuite) TestSetAvailability() {
	ctx := context.Background()
	modelID := uuid.New()
	isAvailable := false

	// Setup expectations
	s.mock.ExpectExec("UPDATE mcp.embedding_model_catalog SET is_available = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(isAvailable, sqlmock.AnyArg(), modelID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err := s.repo.SetAvailability(ctx, modelID, isAvailable)

	// Assert
	s.NoError(err)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestGetProviders tests getting unique providers
func (s *ModelCatalogRepositoryTestSuite) TestGetProviders() {
	ctx := context.Background()

	// Setup expectations
	rows := sqlmock.NewRows([]string{"provider"}).
		AddRow("bedrock").
		AddRow("openai").
		AddRow("google")

	s.mock.ExpectQuery("SELECT DISTINCT provider FROM mcp.embedding_model_catalog WHERE is_available = true").
		WillReturnRows(rows)

	// Execute
	providers, err := s.repo.GetProviders(ctx)

	// Assert
	s.NoError(err)
	s.Len(providers, 3)
	s.Contains(providers, "bedrock")
	s.Contains(providers, "openai")
	s.Contains(providers, "google")

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestBulkUpsert tests bulk upserting models
func (s *ModelCatalogRepositoryTestSuite) TestBulkUpsert() {
	ctx := context.Background()

	models := []*EmbeddingModel{
		{
			ID:           uuid.New(),
			Provider:     "bedrock",
			ModelName:    "Model 1",
			ModelID:      "model-1",
			Dimensions:   1024,
			IsAvailable:  true,
			IsDeprecated: false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			Provider:     "openai",
			ModelName:    "Model 2",
			ModelID:      "model-2",
			Dimensions:   1536,
			IsAvailable:  true,
			IsDeprecated: false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	// Setup expectations
	s.mock.ExpectBegin()

	for _, model := range models {
		s.mock.ExpectExec("INSERT INTO mcp.embedding_model_catalog .+ ON CONFLICT").
			WithArgs(
				model.ID, model.Provider, model.ModelName, model.ModelID,
				sqlmock.AnyArg(), model.Dimensions, sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), model.IsAvailable, model.IsDeprecated,
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	s.mock.ExpectCommit()

	// Execute
	err := s.repo.BulkUpsert(ctx, models)

	// Assert
	s.NoError(err)

	// Verify expectations
	s.NoError(s.mock.ExpectationsWereMet())
}

// TestModelCatalogRepositorySuite runs the test suite
func TestModelCatalogRepositorySuite(t *testing.T) {
	suite.Run(t, new(ModelCatalogRepositoryTestSuite))
}
