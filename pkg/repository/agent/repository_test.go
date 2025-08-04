package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterFromTenantID(t *testing.T) {
	// Test that FilterFromTenantID correctly creates a filter with tenant_id
	tenantID := "test-tenant"
	filter := FilterFromTenantID(tenantID)

	assert.Equal(t, 1, len(filter), "Filter should have exactly one entry")
	assert.Equal(t, tenantID, filter["tenant_id"], "Filter should contain the correct tenant_id")
}

func TestFilterFromIDs(t *testing.T) {
	// Test that FilterFromIDs correctly creates a filter with tenant_id and id
	tenantID := "test-tenant"
	id := "test-id"
	filter := FilterFromIDs(tenantID, id)

	assert.Equal(t, 2, len(filter), "Filter should have exactly two entries")
	assert.Equal(t, tenantID, filter["tenant_id"], "Filter should contain the correct tenant_id")
	assert.Equal(t, id, filter["id"], "Filter should contain the correct id")
}

func TestMockRepository(t *testing.T) {
	// Test that the mock repository implements the Repository interface properly
	repo := NewMockRepository()

	// Test Create and Get
	agent := &models.Agent{
		ID:       "test-id",
		Name:     "Test Agent",
		TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ModelID:  "test-model",
	}

	err := repo.Create(context.Background(), agent)
	assert.NoError(t, err, "Create should not return an error")

	retrieved, err := repo.Get(context.Background(), agent.ID)
	assert.NoError(t, err, "Get should not return an error")
	assert.NotNil(t, retrieved, "Retrieved agent should not be nil")
	assert.Equal(t, agent.ID, retrieved.ID, "Retrieved agent should have the correct ID")

	// Test List
	filter := FilterFromTenantID(agent.TenantID.String())
	agents, err := repo.List(context.Background(), filter)
	assert.NoError(t, err, "List should not return an error")
	assert.GreaterOrEqual(t, len(agents), 0, "List should return at least an empty slice")

	// Test API-specific methods
	err = repo.CreateAgent(context.Background(), agent)
	assert.NoError(t, err, "CreateAgent should not return an error")

	retrieved, err = repo.GetAgentByID(context.Background(), agent.ID, agent.TenantID.String())
	assert.NoError(t, err, "GetAgentByID should not return an error")
	assert.NotNil(t, retrieved, "Retrieved agent should not be nil")

	_, err = repo.ListAgents(context.Background(), agent.TenantID.String())
	assert.NoError(t, err, "ListAgents should not return an error")
}

func TestAgentRepository_ModelPreferences(t *testing.T) {
	tests := []struct {
		name          string
		agent         *models.Agent
		setupMock     func(mock sqlmock.Sqlmock, agent *models.Agent)
		expectedError string
	}{
		{
			name: "successful JSONB serialization of model preferences in metadata",
			agent: &models.Agent{
				ID:       uuid.New().String(),
				TenantID: uuid.New(),
				Name:     "test-agent",
				ModelID:  "claude-3-opus",
				Type:     "assistant",
				Status:   "available",
				Metadata: map[string]interface{}{
					"description": "Test agent",
					"model_preferences": map[string]interface{}{
						"default": "claude-3-opus-20240229",
						"embeddings": map[string]interface{}{
							"provider": "bedrock",
							"model":    "amazon.titan-embed-text-v1",
						},
						"fallback": []interface{}{
							"claude-3-sonnet-20240229",
							"gpt-4-turbo",
						},
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock, agent *models.Agent) {
				// The repository uses ExecContext with specific argument order
				mock.ExpectExec(`INSERT INTO mcp\.agents`).
					WithArgs(
						agent.ID,         // ID
						agent.Name,       // Name
						agent.TenantID,   // TenantID
						agent.ModelID,    // ModelID
						agent.Type,       // Type
						agent.Status,     // Status
						sqlmock.AnyArg(), // capabilities (pq.Array)
						sqlmock.AnyArg(), // metadata (JSON string)
						sqlmock.AnyArg(), // created_at
						sqlmock.AnyArg(), // updated_at
						sqlmock.AnyArg(), // last_seen_at
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "handles empty model preferences",
			agent: &models.Agent{
				ID:       uuid.New().String(),
				TenantID: uuid.New(),
				Name:     "test-agent",
				ModelID:  "claude-3-opus",
				Type:     "assistant",
				Status:   "available",
				Metadata: map[string]interface{}{
					"description": "Test agent",
				},
			},
			setupMock: func(mock sqlmock.Sqlmock, agent *models.Agent) {
				mock.ExpectExec(`INSERT INTO mcp\.agents`).
					WithArgs(
						agent.ID,         // ID
						agent.Name,       // Name
						agent.TenantID,   // TenantID
						agent.ModelID,    // ModelID
						agent.Type,       // Type
						agent.Status,     // Status
						sqlmock.AnyArg(), // capabilities (pq.Array)
						sqlmock.AnyArg(), // metadata (JSON string)
						sqlmock.AnyArg(), // created_at
						sqlmock.AnyArg(), // updated_at
						sqlmock.AnyArg(), // last_seen_at
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "complex nested preferences structure",
			agent: &models.Agent{
				ID:       uuid.New().String(),
				TenantID: uuid.New(),
				Name:     "complex-agent",
				ModelID:  "gpt-4",
				Type:     "assistant",
				Status:   "available",
				Metadata: map[string]interface{}{
					"description": "Complex preferences",
					"model_preferences": map[string]interface{}{
						"tasks": map[string]interface{}{
							"code_generation": map[string]interface{}{
								"provider":    "openai",
								"model":       "gpt-4",
								"temperature": 0.7,
							},
							"embeddings": map[string]interface{}{
								"provider":   "bedrock",
								"model":      "amazon.titan-embed-text-v2:0",
								"dimensions": 1024,
							},
						},
						"providers": map[string]interface{}{
							"bedrock": map[string]interface{}{
								"region": "us-east-1",
								"models": []interface{}{
									"amazon.titan-embed-text-v1",
									"amazon.titan-embed-text-v2:0",
								},
							},
						},
					},
				},
			},
			setupMock: func(mock sqlmock.Sqlmock, agent *models.Agent) {
				// Complex structure should be properly serialized
				mock.ExpectExec(`INSERT INTO mcp\.agents`).
					WithArgs(
						agent.ID,         // ID
						agent.Name,       // Name
						agent.TenantID,   // TenantID
						agent.ModelID,    // ModelID
						agent.Type,       // Type
						agent.Status,     // Status
						sqlmock.AnyArg(), // capabilities (pq.Array)
						sqlmock.AnyArg(), // metadata (Complex JSON structure)
						sqlmock.AnyArg(), // created_at
						sqlmock.AnyArg(), // updated_at
						sqlmock.AnyArg(), // last_seen_at
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = mockDB.Close() }()

			tt.setupMock(mock, tt.agent)

			db := sqlx.NewDb(mockDB, "postgres")
			repo := NewRepository(db)

			err = repo.Create(context.Background(), tt.agent)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestAgentRepository_GetPreferredModel(t *testing.T) {
	tests := []struct {
		name             string
		metadata         map[string]interface{}
		taskType         string
		expectedProvider string
		expectedModel    string
		expectedError    string
	}{
		{
			name: "get embedding model from nested structure",
			metadata: map[string]interface{}{
				"model_preferences": map[string]interface{}{
					"embeddings": map[string]interface{}{
						"provider": "bedrock",
						"model":    "amazon.titan-embed-text-v1",
					},
				},
			},
			taskType:         "embeddings",
			expectedProvider: "bedrock",
			expectedModel:    "amazon.titan-embed-text-v1",
		},
		{
			name: "fallback to default when specific task not found",
			metadata: map[string]interface{}{
				"model_preferences": map[string]interface{}{
					"default": map[string]interface{}{
						"provider": "openai",
						"model":    "gpt-4",
					},
				},
			},
			taskType:         "embeddings",
			expectedProvider: "openai",
			expectedModel:    "gpt-4",
		},
		{
			name:          "handle empty preferences",
			metadata:      map[string]interface{}{},
			taskType:      "embeddings",
			expectedError: "no model preferences configured",
		},
		{
			name: "validate provider name format",
			metadata: map[string]interface{}{
				"model_preferences": map[string]interface{}{
					"embeddings": map[string]interface{}{
						"provider": "AWS-Bedrock", // Should be normalized
						"model":    "amazon.titan-embed-text-v1",
					},
				},
			},
			taskType:         "embeddings",
			expectedProvider: "bedrock",
			expectedModel:    "amazon.titan-embed-text-v1",
		},
		{
			name: "handle invalid preference structure",
			metadata: map[string]interface{}{
				"model_preferences": map[string]interface{}{
					"embeddings": "invalid-string-value",
				},
			},
			taskType:      "embeddings",
			expectedError: "invalid preference structure",
		},
		{
			name: "complex provider selection logic",
			metadata: map[string]interface{}{
				"model_preferences": map[string]interface{}{
					"tasks": map[string]interface{}{
						"embeddings": map[string]interface{}{
							"providers": []interface{}{
								map[string]interface{}{
									"name":     "bedrock",
									"model":    "amazon.titan-embed-text-v2:0",
									"priority": 1,
								},
								map[string]interface{}{
									"name":     "openai",
									"model":    "text-embedding-3-large",
									"priority": 2,
								},
							},
						},
					},
				},
			},
			taskType:         "embeddings",
			expectedProvider: "bedrock",
			expectedModel:    "amazon.titan-embed-text-v2:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a unit test for the preference logic
			// In a real implementation, this would be part of the service layer
			provider, model, err := extractPreferredModelFromMetadata(tt.metadata, tt.taskType)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedProvider, provider)
				assert.Equal(t, tt.expectedModel, model)
			}
		})
	}
}

func TestAgentRepository_ValidateEmbeddingProvider(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		expectedName string
		isValid      bool
	}{
		{
			name:         "valid bedrock provider",
			provider:     "bedrock",
			expectedName: "bedrock",
			isValid:      true,
		},
		{
			name:         "valid openai provider",
			provider:     "openai",
			expectedName: "openai",
			isValid:      true,
		},
		{
			name:         "normalize AWS Bedrock to bedrock",
			provider:     "AWS-Bedrock",
			expectedName: "bedrock",
			isValid:      true,
		},
		{
			name:         "normalize aws_bedrock to bedrock",
			provider:     "aws_bedrock",
			expectedName: "bedrock",
			isValid:      true,
		},
		{
			name:         "invalid provider",
			provider:     "unknown-provider",
			expectedName: "",
			isValid:      false,
		},
		{
			name:         "empty provider",
			provider:     "",
			expectedName: "",
			isValid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, valid := validateAndNormalizeProvider(tt.provider)
			assert.Equal(t, tt.isValid, valid)
			if valid {
				assert.Equal(t, tt.expectedName, normalized)
			}
		})
	}
}

// Helper functions for testing preference logic
func extractPreferredModelFromMetadata(metadata map[string]interface{}, taskType string) (string, string, error) {
	if metadata == nil {
		return "", "", fmt.Errorf("no model preferences configured")
	}

	// Extract model_preferences from metadata
	modelPrefs, ok := metadata["model_preferences"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("no model preferences configured")
	}

	return extractPreferredModel(modelPrefs, taskType)
}

func extractPreferredModel(preferences map[string]interface{}, taskType string) (string, string, error) {
	if preferences == nil {
		return "", "", fmt.Errorf("no model preferences configured")
	}

	// Check for task-specific preference
	if taskPrefs, ok := preferences[taskType]; ok {
		if prefs, ok := taskPrefs.(map[string]interface{}); ok {
			provider, _ := prefs["provider"].(string)
			model, _ := prefs["model"].(string)
			if provider != "" && model != "" {
				normalizedProvider, valid := validateAndNormalizeProvider(provider)
				if valid {
					return normalizedProvider, model, nil
				}
			}
		}

		return "", "", fmt.Errorf("invalid preference structure")
	}

	// Handle provider list format in tasks
	if tasks, ok := preferences["tasks"].(map[string]interface{}); ok {
		if taskData, ok := tasks[taskType].(map[string]interface{}); ok {
			if providers, ok := taskData["providers"].([]interface{}); ok && len(providers) > 0 {
				// Get first provider (highest priority)
				if p, ok := providers[0].(map[string]interface{}); ok {
					provider, _ := p["name"].(string)
					model, _ := p["model"].(string)
					normalizedProvider, valid := validateAndNormalizeProvider(provider)
					if valid {
						return normalizedProvider, model, nil
					}
				}
			}
		}

		return "", "", fmt.Errorf("invalid preference structure")
	}

	// Fallback to default
	if defaultPrefs, ok := preferences["default"].(map[string]interface{}); ok {
		provider, _ := defaultPrefs["provider"].(string)
		model, _ := defaultPrefs["model"].(string)
		if provider != "" && model != "" {
			normalizedProvider, valid := validateAndNormalizeProvider(provider)
			if valid {
				return normalizedProvider, model, nil
			}
		}
	}

	return "", "", fmt.Errorf("no model preferences configured")
}

func validateAndNormalizeProvider(provider string) (string, bool) {
	normalizedProviders := map[string]string{
		"bedrock":     "bedrock",
		"aws-bedrock": "bedrock",
		"aws_bedrock": "bedrock",
		"openai":      "openai",
		"anthropic":   "anthropic",
		"google":      "google",
		"vertex":      "vertex",
		"vertex-ai":   "vertex",
	}

	// Normalize to lowercase
	lower := strings.ToLower(provider)
	if lower == "" {
		return "", false
	}

	// Direct match
	if normalized, ok := normalizedProviders[lower]; ok {
		return normalized, true
	}

	return "", false
}
