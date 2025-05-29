// Package core provides the central engine for coordinating API subsystems
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// Metric constants for the context manager
const (
	MetricContextCreationLatency = "context_creation_latency_seconds"
	MetricContextOperationsTotal = "context_operations_total"
)

// ContextManagerInterface defines the interface for context management
type ContextManagerInterface interface {
	CreateContext(ctx context.Context, context *models.Context) (*models.Context, error)
	GetContext(ctx context.Context, contextID string) (*models.Context, error)
	UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context, agentID, sessionID string, options map[string]any) ([]*models.Context, error)
	SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error)
	SummarizeContext(ctx context.Context, contextID string) (string, error)
}

// ContextManager provides a production-ready implementation of ContextManagerInterface
// It handles persistence, caching, and error handling for context operations
type ContextManager struct {
	db      *sqlx.DB
	cache   map[string]*models.Context
	mutex   sync.RWMutex
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewContextManager creates a new context manager with database persistence
func NewContextManager(db *sqlx.DB, logger observability.Logger, metrics observability.MetricsClient) ContextManagerInterface {
	if db == nil {
		logger.Warn("Database connection is nil, context manager will operate in memory-only mode", nil)
	}

	return &ContextManager{
		db:      db,
		logger:  logger,
		cache:   make(map[string]*models.Context),
		mutex:   sync.RWMutex{},
		metrics: metrics,
	}
}

// CreateContext creates a new context with database persistence
func (cm *ContextManager) CreateContext(ctx context.Context, context *models.Context) (*models.Context, error) {
	startTime := time.Now()
	defer func() {
		cm.metrics.RecordHistogram(MetricContextCreationLatency, time.Since(startTime).Seconds(), nil)
	}()

	if context == nil {
		cm.logger.Error("Attempted to create nil context", nil)
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "create", "status": "error"})
		return nil, errors.New("cannot create nil context")
	}

	// Generate ID if not provided
	if context.ID == "" {
		context.ID = generateUniqueID()
	}

	// Set creation timestamp if not already set
	if context.CreatedAt.IsZero() {
		context.CreatedAt = time.Now()
	}
	context.UpdatedAt = time.Now()

	// Store in database
	if cm.db != nil {
		// Marshal metadata to JSON
		var metadataJSON []byte
		if context.Metadata != nil && len(context.Metadata) > 0 {
			var err error
			metadataJSON, err = json.Marshal(context.Metadata)
			if err != nil {
				cm.logger.Error("Failed to marshal metadata", map[string]any{
					"error":      err.Error(),
					"context_id": context.ID,
				})
				cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "create", "status": "error"})
				return nil, fmt.Errorf("failed to marshal metadata: %w", err)
			}
		} else {
			// Use empty JSON object for null/empty metadata
			metadataJSON = []byte("{}")
		}

		// Create insert query for context
		q := `INSERT INTO mcp.contexts (id, name, description, agent_id, model_id, session_id, current_tokens, max_tokens, metadata, created_at, updated_at, expires_at) 
		      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

		// Use standard Exec with explicit parameters
		_, err := cm.db.ExecContext(ctx, q, 
			context.ID, 
			context.Name, 
			context.Description, 
			context.AgentID, 
			context.ModelID, 
			context.SessionID, 
			context.CurrentTokens, 
			context.MaxTokens, 
			metadataJSON,
			context.CreatedAt, 
			context.UpdatedAt, 
			context.ExpiresAt)
		if err != nil {
			cm.logger.Error("Failed to store context in database", map[string]any{
				"error":      err.Error(),
				"context_id": context.ID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "create", "status": "error"})
			return nil, fmt.Errorf("failed to store context: %w", err)
		}
	} else {
		cm.logger.Warn("Database not available, storing context in memory only", map[string]any{
			"context_id": context.ID,
		})
	}

	// Update cache
	cm.mutex.Lock()
	cm.cache[context.ID] = context
	cm.mutex.Unlock()

	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "create", "status": "success"})
	return context, nil
}

// GetContext retrieves a context by ID
func (cm *ContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	if contextID == "" {
		cm.logger.Error("Attempted to get context with empty ID", nil)
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "get", "status": "error"})
		return nil, errors.New("context ID cannot be empty")
	}

	// First check cache
	cm.mutex.RLock()
	cachedContext, found := cm.cache[contextID]
	cm.mutex.RUnlock()

	if found {
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "get", "status": "cache_hit"})
		return cachedContext, nil
	}

	// Not in cache, try database
	if cm.db != nil {
		// Create a temporary struct to handle JSON metadata
		var dbContext struct {
			ID            string          `db:"id"`
			Name          string          `db:"name"`
			Description   string          `db:"description"`
			AgentID       string          `db:"agent_id"`
			ModelID       string          `db:"model_id"`
			SessionID     string          `db:"session_id"`
			CurrentTokens int             `db:"current_tokens"`
			MaxTokens     int             `db:"max_tokens"`
			Metadata      json.RawMessage `db:"metadata"`
			CreatedAt     time.Time       `db:"created_at"`
			UpdatedAt     time.Time       `db:"updated_at"`
			ExpiresAt     time.Time       `db:"expires_at"`
		}

		q := `SELECT id, name, description, agent_id, model_id, session_id, current_tokens, max_tokens, metadata, created_at, updated_at, expires_at FROM mcp.contexts WHERE id = $1 LIMIT 1`

		// Use QueryRowxContext to fetch a single row
		err := cm.db.QueryRowxContext(ctx, q, contextID).StructScan(&dbContext)
		if err != nil {
			cm.logger.Error("Failed to retrieve context from database", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "get", "status": "error"})
			return nil, fmt.Errorf("failed to retrieve context: %w", err)
		}

		// Convert to models.Context
		context := models.Context{
			ID:            dbContext.ID,
			Name:          dbContext.Name,
			Description:   dbContext.Description,
			AgentID:       dbContext.AgentID,
			ModelID:       dbContext.ModelID,
			SessionID:     dbContext.SessionID,
			CurrentTokens: dbContext.CurrentTokens,
			MaxTokens:     dbContext.MaxTokens,
			CreatedAt:     dbContext.CreatedAt,
			UpdatedAt:     dbContext.UpdatedAt,
			ExpiresAt:     dbContext.ExpiresAt,
		}

		// Unmarshal metadata if present
		if len(dbContext.Metadata) > 0 && string(dbContext.Metadata) != "null" {
			if err := json.Unmarshal(dbContext.Metadata, &context.Metadata); err != nil {
				cm.logger.Warn("Failed to unmarshal metadata", map[string]any{
					"error":      err.Error(),
					"context_id": contextID,
				})
				// Continue without metadata - context is still valid
			}
		}

		// Load context items
		itemsQuery := `SELECT id, context_id, role, content, tokens, timestamp, metadata FROM mcp.context_items WHERE context_id = $1 ORDER BY timestamp`
		rows, err := cm.db.QueryxContext(ctx, itemsQuery, contextID)
		if err != nil {
			cm.logger.Warn("Failed to load context items", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			// Continue without items - context can still be valid without items
		} else {
			defer rows.Close()
			var items []models.ContextItem
			for rows.Next() {
				// Create a temporary struct to handle JSON metadata
				var dbItem struct {
					ID        string          `db:"id"`
					ContextID string          `db:"context_id"`
					Role      string          `db:"role"`
					Content   string          `db:"content"`
					Tokens    int             `db:"tokens"`
					Timestamp time.Time       `db:"timestamp"`
					Metadata  json.RawMessage `db:"metadata"`
				}
				
				if err := rows.StructScan(&dbItem); err != nil {
					cm.logger.Warn("Failed to scan context item", map[string]any{
						"error": err.Error(),
					})
					continue
				}
				
				// Convert to models.ContextItem
				item := models.ContextItem{
					ID:        dbItem.ID,
					ContextID: dbItem.ContextID,
					Role:      dbItem.Role,
					Content:   dbItem.Content,
					Tokens:    dbItem.Tokens,
					Timestamp: dbItem.Timestamp,
				}
				
				// Unmarshal metadata if present
				if len(dbItem.Metadata) > 0 && string(dbItem.Metadata) != "null" {
					if err := json.Unmarshal(dbItem.Metadata, &item.Metadata); err != nil {
						cm.logger.Warn("Failed to unmarshal item metadata", map[string]any{
							"error": err.Error(),
							"item_id": dbItem.ID,
						})
						// Continue without metadata - item is still valid
					}
				}
				
				items = append(items, item)
			}
			context.Content = items
		}

		// Update cache with retrieved context
		cm.mutex.Lock()
		cm.cache[contextID] = &context
		cm.mutex.Unlock()

		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "get", "status": "db_hit"})
		return &context, nil
	}

	cm.logger.Error("Context not found and database not available", map[string]any{
		"context_id": contextID,
	})
	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "get", "status": "not_found"})
	return nil, fmt.Errorf("context not found: %s", contextID)
}

// UpdateContext updates an existing context
func (cm *ContextManager) UpdateContext(ctx context.Context, contextID string, updatedContext *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	if contextID == "" || updatedContext == nil {
		cm.logger.Error("Invalid parameters for context update", map[string]any{
			"context_id":  contextID,
			"context_nil": updatedContext == nil,
		})
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "update", "status": "error"})
		return nil, errors.New("invalid parameters for context update")
	}

	// First get the existing context
	existingContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "update", "status": "error"})
		return nil, fmt.Errorf("cannot update non-existent context: %w", err)
	}
	
	// Debug log the existing context
	cm.logger.Info("UpdateContext - retrieved existing context", map[string]any{
		"context_id": existingContext.ID,
		"name": existingContext.Name,
		"agent_id": existingContext.AgentID,
		"model_id": existingContext.ModelID,
		"existing_is_nil": existingContext == nil,
	})

	// Create a new context object based on existing context
	// Only update the fields that were provided in the update request
	result := &models.Context{
		ID:          existingContext.ID,
		Name:        existingContext.Name,
		Description: existingContext.Description,
		AgentID:     existingContext.AgentID,
		ModelID:     existingContext.ModelID,
		SessionID:   existingContext.SessionID,
		Content:     updatedContext.Content, // Use the new content from update request
		Metadata:    existingContext.Metadata,
		CreatedAt:   existingContext.CreatedAt,
		UpdatedAt:   time.Now(),
		ExpiresAt:   existingContext.ExpiresAt,
		MaxTokens:   existingContext.MaxTokens,
		CurrentTokens: existingContext.CurrentTokens,
	}
	
	cm.logger.Debug("UpdateContext - after applying updates", map[string]any{
		"context_id": result.ID,
		"name": result.Name,
		"agent_id": result.AgentID,
		"model_id": result.ModelID,
	})

	// Persist to database
	if cm.db != nil {
		// Start transaction
		tx, err := cm.db.BeginTxx(ctx, nil)
		if err != nil {
			cm.logger.Error("Failed to start transaction", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "update", "status": "error"})
			return nil, fmt.Errorf("failed to start transaction: %w", err)
		}

		// Update context metadata
		q := `UPDATE mcp.contexts SET 
		       name = :name,
		       description = :description,
		       updated_at = :updated_at 
		     WHERE id = :id`

		_, err = tx.NamedExecContext(ctx, q, result)
		if err != nil {
			tx.Rollback()
			cm.logger.Error("Failed to update context in database", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "update", "status": "error"})
			return nil, fmt.Errorf("failed to update context: %w", err)
		}

		// Delete existing context items
		_, err = tx.ExecContext(ctx, "DELETE FROM mcp.context_items WHERE context_id = $1", contextID)
		if err != nil {
			tx.Rollback()
			cm.logger.Error("Failed to delete old context items", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			return nil, fmt.Errorf("failed to delete old context items: %w", err)
		}

		// Insert new context items if any
		if len(result.Content) > 0 {
			itemsQuery := `INSERT INTO mcp.context_items (id, context_id, role, content, tokens, timestamp, metadata) 
			              VALUES ($1, $2, $3, $4, $5, $6, $7)`
			
			for i, item := range result.Content {
				// Ensure each item has an ID and context ID
				if item.ID == "" {
					item.ID = generateUniqueID()
				}
				item.ContextID = contextID
				if item.Timestamp.IsZero() {
					item.Timestamp = time.Now()
				}
				result.Content[i] = item

				// Marshal item metadata to JSON
				var itemMetadataJSON []byte
				if item.Metadata != nil && len(item.Metadata) > 0 {
					itemMetadataJSON, err = json.Marshal(item.Metadata)
					if err != nil {
						tx.Rollback()
						cm.logger.Error("Failed to marshal item metadata", map[string]any{
							"error":      err.Error(),
							"context_id": contextID,
							"item_index": i,
						})
						return nil, fmt.Errorf("failed to marshal item metadata: %w", err)
					}
				} else {
					itemMetadataJSON = []byte("{}")
				}

				_, err = tx.ExecContext(ctx, itemsQuery, 
					item.ID, 
					item.ContextID, 
					item.Role, 
					item.Content, 
					item.Tokens, 
					item.Timestamp, 
					itemMetadataJSON)
				if err != nil {
					tx.Rollback()
					cm.logger.Error("Failed to insert context item", map[string]any{
						"error":      err.Error(),
						"context_id": contextID,
						"item_index": i,
					})
					return nil, fmt.Errorf("failed to insert context item: %w", err)
				}
			}
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			cm.logger.Error("Failed to commit transaction", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "update", "status": "error"})
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	} else {
		cm.logger.Warn("Database not available, updating context in memory only", map[string]any{
			"context_id": contextID,
		})
	}

	// Update cache
	cm.mutex.Lock()
	cm.cache[contextID] = result
	cm.mutex.Unlock()

	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "update", "status": "success"})
	
	cm.logger.Debug("UpdateContext - returning context", map[string]any{
		"context_id": result.ID,
		"name": result.Name,
		"agent_id": result.AgentID,
		"model_id": result.ModelID,
	})
	
	return result, nil
}

// DeleteContext removes a context
func (cm *ContextManager) DeleteContext(ctx context.Context, contextID string) error {
	if contextID == "" {
		cm.logger.Error("Attempted to delete context with empty ID", nil)
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "delete", "status": "error"})
		return errors.New("context ID cannot be empty")
	}

	// Remove from database first
	if cm.db != nil {
		// Create delete query
		q := `DELETE FROM mcp.contexts WHERE id = $1`

		// Execute query with parameter
		_, err := cm.db.ExecContext(ctx, q, contextID)
		if err != nil {
			cm.logger.Error("Failed to delete context from database", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "delete", "status": "error"})
			return fmt.Errorf("failed to delete context: %w", err)
		}
	} else {
		cm.logger.Warn("Database not available, deleting context from memory only", map[string]any{
			"context_id": contextID,
		})
	}

	// Remove from cache
	cm.mutex.Lock()
	delete(cm.cache, contextID)
	cm.mutex.Unlock()

	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "delete", "status": "success"})
	return nil
}

// ListContexts lists all contexts matching the filter criteria
func (cm *ContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]any) ([]*models.Context, error) {
	// Create an array to hold the results
	var results []*models.Context

	// Build query conditions
	conditions := make(map[string]any)
	if agentID != "" {
		conditions["agent_id"] = agentID
	}
	if sessionID != "" {
		conditions["session_id"] = sessionID
	}

	// Query database if available
	if cm.db != nil {

		// Build the query based on conditions
		q := `SELECT * FROM contexts WHERE 1=1`
		args := []interface{}{}

		// Add conditions to the query
		if agentID != "" {
			q += " AND agent_id = $" + fmt.Sprintf("%d", len(args)+1)
			args = append(args, agentID)
		}

		if sessionID != "" {
			q += " AND session_id = $" + fmt.Sprintf("%d", len(args)+1)
			args = append(args, sessionID)
		}

		// Apply limit if specified
		if limit, ok := options["limit"].(int); ok {
			q += " LIMIT $" + fmt.Sprintf("%d", len(args)+1)
			args = append(args, limit)
		}

		// Apply offset if specified
		if offset, ok := options["offset"].(int); ok {
			q += " OFFSET $" + fmt.Sprintf("%d", len(args)+1)
			args = append(args, offset)
		}

		// Execute the query
		rows, err := cm.db.QueryxContext(ctx, q, args...)
		if err != nil {
			cm.logger.Error("Failed to list contexts from database", map[string]any{
				"error":      err.Error(),
				"agent_id":   agentID,
				"session_id": sessionID,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "list", "status": "error"})
			return nil, fmt.Errorf("failed to list contexts: %w", err)
		}
		defer rows.Close()

		// Iterate through results
		for rows.Next() {
			var context models.Context
			if err := rows.StructScan(&context); err != nil {
				cm.logger.Error("Failed to scan context from database", map[string]any{
					"error": err.Error(),
				})
				continue
			}
			results = append(results, &context)
		}

		// Check for errors during iteration
		if err := rows.Err(); err != nil {
			cm.logger.Error("Error during context rows iteration", map[string]any{
				"error": err.Error(),
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "list", "status": "error"})
			return results, fmt.Errorf("error during context iteration: %w", err)
		}
	} else {
		// Fallback to memory if database not available
		cm.logger.Warn("Database not available, listing contexts from memory only", nil)

		cm.mutex.RLock()
		defer cm.mutex.RUnlock()

		for _, context := range cm.cache {
			include := true

			// Apply filters
			if agentID != "" && context.AgentID != agentID {
				include = false
			}
			if sessionID != "" && context.SessionID != sessionID {
				include = false
			}

			if include {
				results = append(results, context)
			}
		}
	}

	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "list", "status": "success"})
	return results, nil
}

// SearchInContext searches for items within a context
func (cm *ContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error) {
	if contextID == "" || query == "" {
		cm.logger.Error("Invalid parameters for context search", map[string]any{
			"context_id":  contextID,
			"query_empty": query == "",
		})
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "search", "status": "error"})
		return nil, errors.New("context ID and query cannot be empty")
	}

	// First, ensure the context exists
	_, err := cm.GetContext(ctx, contextID)
	if err != nil {
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "search", "status": "error"})
		return nil, fmt.Errorf("cannot search in non-existent context: %w", err)
	}

	var results []models.ContextItem

	// Perform search in database if available
	if cm.db != nil {
		// Create query to search in context items
		q := `SELECT * FROM context_items WHERE context_id = $1 AND content LIKE $2`

		// Execute query with parameters
		rows, err := cm.db.QueryxContext(ctx, q, contextID, "%"+query+"%")
		if err != nil {
			cm.logger.Error("Failed to search in context", map[string]any{
				"error":      err.Error(),
				"context_id": contextID,
				"query":      query,
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "search", "status": "error"})
			return nil, fmt.Errorf("failed to search in context: %w", err)
		}
		defer rows.Close()

		// Iterate through results
		for rows.Next() {
			var item models.ContextItem
			if err := rows.StructScan(&item); err != nil {
				cm.logger.Error("Failed to scan context item from database", map[string]any{
					"error": err.Error(),
				})
				continue
			}
			results = append(results, item)
		}

		// Check for errors during iteration
		if err := rows.Err(); err != nil {
			cm.logger.Error("Error during context item rows iteration", map[string]any{
				"error": err.Error(),
			})
			cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "search", "status": "error"})
			// Return partial results with error
			return results, fmt.Errorf("error during context item iteration: %w", err)
		}
	} else {
		cm.logger.Warn("Database not available for context search, returning empty results", map[string]any{
			"context_id": contextID,
		})
	}

	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "search", "status": "success"})
	return results, nil
}

// SummarizeContext generates a summary of the context
func (cm *ContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	if contextID == "" {
		cm.logger.Error("Attempted to summarize context with empty ID", nil)
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "summarize", "status": "error"})
		return "", errors.New("context ID cannot be empty")
	}

	// First, ensure the context exists
	context, err := cm.GetContext(ctx, contextID)
	if err != nil {
		cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "summarize", "status": "error"})
		return "", fmt.Errorf("cannot summarize non-existent context: %w", err)
	}

	// In a real implementation, this would use more sophisticated summarization logic
	// For now, we'll create a simple summary
	summary := fmt.Sprintf("Context ID: %s\nName: %s\nCreated At: %s",
		context.ID,
		context.Name,
		context.CreatedAt.Format(time.RFC3339))

	cm.metrics.IncrementCounterWithLabels(MetricContextOperationsTotal, float64(1), map[string]string{"operation": "summarize", "status": "success"})
	return summary, nil
}

// Helper function to generate a unique ID
func generateUniqueID() string {
	return fmt.Sprintf("ctx_%d", time.Now().UnixNano())
}
