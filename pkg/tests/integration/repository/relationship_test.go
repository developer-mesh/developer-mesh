//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/relationship"
	"github.com/developer-mesh/developer-mesh/pkg/tests/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelationshipServiceIntegration tests the relationship service against a real database
func TestRelationshipServiceIntegration(t *testing.T) {
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
	dbConn, err := database.NewDatabase(ctx, config)
	if err != nil {
		t.Skip("Skipping relationship test due to database connection failure: " + err.Error())
		return
	}
	db := dbConn.GetDB()
	dbHelper.SetupTestDatabaseWithConnection(ctx, db)
	defer dbHelper.CleanupDatabase()

	// Create the database instance using pkg/database
	dbInstance := database.NewDatabaseWithConnection(db)
	require.NotNil(t, dbInstance, "Failed to create database instance")

	// Skip table initialization as the Database struct no longer implements InitializeTables directly

	// Clean up any existing test data
	cleanupTestData(t, dbInstance)

	// Create a repository using pkg/database
	repo := database.NewRelationshipRepository(dbInstance)
	require.NotNil(t, repo, "Failed to create relationship repository")

	// Create a service
	service := relationship.NewService(repo)
	require.NotNil(t, service, "Failed to create relationship service")

	// Create test entities
	issueEntity := models.NewEntityID(models.EntityTypeIssue, "test-owner", "test-repo", "1")
	prEntity := models.NewEntityID(models.EntityTypePullRequest, "test-owner", "test-repo", "2")
	commitEntity := models.NewEntityID(models.EntityTypeCommit, "test-owner", "test-repo", "abc123")
	fileEntity := models.NewEntityID(models.EntityTypeFile, "test-owner", "test-repo", "path/to/file.go")

	// Test: Create relationships
	t.Run("CreateRelationship", func(t *testing.T) {
		// Create a relationship between an issue and a PR
		rel := models.NewEntityRelationship(
			models.RelationshipTypeReferences,
			issueEntity,
			prEntity,
			models.DirectionOutgoing,
			0.8,
		).WithContext("Issue references PR in its description")

		err := service.CreateRelationship(ctx, rel)
		assert.NoError(t, err, "Failed to create relationship")

		// Create another relationship
		rel2 := models.NewEntityRelationship(
			models.RelationshipTypeModifies,
			prEntity,
			fileEntity,
			models.DirectionOutgoing,
			1.0,
		).WithContext("PR modifies this file")

		err = service.CreateRelationship(ctx, rel2)
		assert.NoError(t, err, "Failed to create second relationship")

		// Create a bidirectional relationship manually (without using CreateBidirectionalRelationship)
		biRel := models.NewEntityRelationship(
			models.RelationshipTypeAssociates,
			prEntity,
			commitEntity,
			models.DirectionBidirectional,
			0.9,
		).WithMetadata(map[string]interface{}{"commit_sha": "abc123"})

		err = service.CreateRelationship(ctx, biRel)
		assert.NoError(t, err, "Failed to create bidirectional relationship")
	})

	// Test: Get relationship by ID
	t.Run("GetRelationship", func(t *testing.T) {
		// Generate the relationship ID
		relID := models.GenerateRelationshipID(
			models.RelationshipTypeReferences,
			issueEntity,
			prEntity,
			models.DirectionOutgoing,
		)

		// Retrieve the relationship
		rel, err := service.GetRelationship(ctx, relID)
		assert.NoError(t, err, "Failed to get relationship")
		assert.Equal(t, issueEntity.String(), rel.Source.String(), "Source entity should match")
		assert.Equal(t, prEntity.String(), rel.Target.String(), "Target entity should match")
		assert.Equal(t, models.RelationshipTypeReferences, rel.Type, "Relationship type should match")
		assert.Equal(t, 0.8, rel.Strength, "Strength should match")
		assert.Equal(t, "Issue references PR in its description", rel.Context, "Context should match")
	})

	// Test: Get relationships by entity
	t.Run("GetRelationships", func(t *testing.T) {
		// Get outgoing relationships for the PR
		rels, err := service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionOutgoing,
			nil,
		)
		assert.NoError(t, err, "Failed to get outgoing relationships")
		assert.Len(t, rels, 2, "PR should have 2 outgoing relationships")

		// Check relationship types
		hasModifies := false
		hasAssociates := false
		for _, rel := range rels {
			if rel.Type == models.RelationshipTypeModifies {
				hasModifies = true
			}
			if rel.Type == models.RelationshipTypeAssociates {
				hasAssociates = true
			}
		}
		assert.True(t, hasModifies, "Should have modifies relationship")
		assert.True(t, hasAssociates, "Should have associates relationship")

		// Get incoming relationships for the PR
		rels, err = service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionIncoming,
			nil,
		)
		assert.NoError(t, err, "Failed to get incoming relationships")
		assert.Len(t, rels, 1, "PR should have 1 incoming relationship")
		assert.Equal(t, models.RelationshipTypeReferences, rels[0].Type)

		// Get bidirectional relationships for the PR
		rels, err = service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionBidirectional,
			nil,
		)
		assert.NoError(t, err, "Failed to get bidirectional relationships")
		assert.Len(t, rels, 3, "PR should have 3 total relationships")
	})

	// Test: Get related entities using relationship graph instead
	t.Run("GetRelationshipGraph", func(t *testing.T) {
		// Get relationship graph for the PR
		relationships, err := service.GetRelationshipGraph(
			ctx,
			prEntity,
			1,
		)
		assert.NoError(t, err, "Failed to get relationship graph")

		// Extract related entities from relationships
		entitiesMap := make(map[string]models.EntityID)

		// Add entities from relationships
		for _, rel := range relationships {
			entitiesMap[rel.Source.String()] = rel.Source
			entitiesMap[rel.Target.String()] = rel.Target
		}

		// Remove the PR entity itself
		delete(entitiesMap, prEntity.String())

		// Convert to slice
		var entities []models.EntityID
		for _, entity := range entitiesMap {
			entities = append(entities, entity)
		}

		assert.Len(t, entities, 3, "PR should be related to 3 entities")

		// Check entity types
		hasIssue := false
		hasCommit := false
		hasFile := false
		for _, entity := range entities {
			if entity.Type == models.EntityTypeIssue {
				hasIssue = true
			}
			if entity.Type == models.EntityTypeCommit {
				hasCommit = true
			}
			if entity.Type == models.EntityTypeFile {
				hasFile = true
			}
		}
		assert.True(t, hasIssue, "Should be related to issue")
		assert.True(t, hasCommit, "Should be related to commit")
		assert.True(t, hasFile, "Should be related to file")
	})

	// Test: Get relationship graph
	t.Run("GetRelationshipGraph", func(t *testing.T) {
		// Create additional relationships to test graph traversal
		fileToCommitRel := models.NewEntityRelationship(
			models.RelationshipTypeContains,
			commitEntity,
			fileEntity,
			models.DirectionOutgoing,
			0.7,
		)
		err := service.CreateRelationship(ctx, fileToCommitRel)
		assert.NoError(t, err, "Failed to create file-to-commit relationship")

		// Get the relationship graph starting from issue with depth 2
		graph, err := service.GetRelationshipGraph(
			ctx,
			issueEntity,
			2,
		)
		assert.NoError(t, err, "Failed to get relationship graph")
		assert.GreaterOrEqual(t, len(graph), 4, "Should have at least 4 relationships in the graph")
	})

	// Test: Delete relationships
	t.Run("DeleteRelationship", func(t *testing.T) {
		// Delete the issue-to-PR relationship
		relID := models.GenerateRelationshipID(
			models.RelationshipTypeReferences,
			issueEntity,
			prEntity,
			models.DirectionOutgoing,
		)
		err := service.DeleteRelationship(ctx, relID)
		assert.NoError(t, err, "Failed to delete relationship")

		// Verify it's deleted
		_, err = service.GetRelationship(ctx, relID)
		assert.Error(t, err, "Relationship should be deleted")

		// Delete all relationships between PR and commit
		err = service.DeleteRelationshipsBetween(
			ctx,
			prEntity,
			commitEntity,
		)
		assert.NoError(t, err, "Failed to delete relationships between PR and commit")

		// Verify deletion
		rels, err := service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionBidirectional,
			nil,
		)
		assert.NoError(t, err, "Failed to get relationships")
		assert.Len(t, rels, 1, "Should only have 1 relationship left")
	})
}

// cleanupTestData removes any test data from previous test runs
func cleanupTestData(t *testing.T, database *database.Database) {
	ctx := context.Background()

	// Access the underlying database through the standard pkg/database interface
	dbConn := database.GetDB()

	// Execute cleanup queries
	_, err := dbConn.ExecContext(ctx, `
		DELETE FROM mcp.entity_relationships 
		WHERE source_owner = 'test-owner' AND source_repo = 'test-repo'
	`)
	assert.NoError(t, err, "Failed to cleanup test data")

	_, err = dbConn.ExecContext(ctx, `
		DELETE FROM mcp.entity_relationships 
		WHERE target_owner = 'test-owner' AND target_repo = 'test-repo'
	`)
	assert.NoError(t, err, "Failed to cleanup test data")
}
