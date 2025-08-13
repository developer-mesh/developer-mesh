package websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/collaboration"
	"github.com/developer-mesh/developer-mesh/pkg/collaboration/crdt"
	"github.com/google/uuid"
)

// Conflict resolution handlers

// handleDocumentSync synchronizes document changes between agents
func (s *Server) handleDocumentSync(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var syncParams struct {
		DocumentID string                        `json:"document_id"`
		Operations []collaboration.CRDTOperation `json:"operations"`
		Clock      map[string]uint64             `json:"clock"`
	}

	if err := json.Unmarshal(params, &syncParams); err != nil {
		return nil, err
	}

	documentID, err := uuid.Parse(syncParams.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	if s.conflictService != nil {
		// Apply operations
		for _, op := range syncParams.Operations {
			// Convert clock
			op.NodeID = crdt.NodeID(conn.AgentID)
			op.Clock = convertToCRDTClock(syncParams.Clock)

			if err := s.conflictService.ApplyDocumentOperation(ctx, documentID, &op); err != nil {
				s.logger.Error("Failed to apply document operation", map[string]interface{}{
					"document_id":  documentID,
					"operation_id": op.ID,
					"error":        err.Error(),
				})
				// Continue with other operations
			}
		}

		// Get updated document
		docCRDT, err := s.conflictService.GetDocumentCRDT(ctx, documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document CRDT: %w", err)
		}

		// Notify other agents about the sync
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":             "document.synced",
				"document_id":      documentID.String(),
				"agent_id":         conn.AgentID,
				"operations_count": len(syncParams.Operations),
			}
			s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("document:%s", documentID), "document.synced", notification)
		}

		return map[string]interface{}{
			"document_id":        documentID.String(),
			"content":            docCRDT.GetContent(),
			"synced":             true,
			"operations_applied": len(syncParams.Operations),
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"document_id":        documentID.String(),
		"synced":             true,
		"operations_applied": len(syncParams.Operations),
	}, nil
}

// handleWorkspaceStateSync synchronizes workspace state between agents
func (s *Server) handleWorkspaceStateSync(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var syncParams struct {
		WorkspaceID string                         `json:"workspace_id"`
		Operations  []collaboration.StateOperation `json:"operations"`
		Clock       map[string]uint64              `json:"clock"`
	}

	if err := json.Unmarshal(params, &syncParams); err != nil {
		return nil, err
	}

	workspaceID, err := uuid.Parse(syncParams.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace ID: %w", err)
	}

	if s.conflictService != nil {
		// Apply operations
		for _, op := range syncParams.Operations {
			// Set node ID and clock
			op.NodeID = crdt.NodeID(conn.AgentID)
			op.Clock = convertToCRDTClock(syncParams.Clock)

			if err := s.conflictService.ApplyStateOperation(ctx, workspaceID, &op); err != nil {
				s.logger.Error("Failed to apply state operation", map[string]interface{}{
					"workspace_id":   workspaceID,
					"operation_type": op.Type,
					"path":           op.Path,
					"error":          err.Error(),
				})
			}
		}

		// Get updated state
		stateCRDT, err := s.conflictService.GetWorkspaceStateCRDT(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get state CRDT: %w", err)
		}

		state := stateCRDT.GetState()

		// Notify other members
		if s.notificationManager != nil {
			notification := map[string]interface{}{
				"type":             "workspace.state_synced",
				"workspace_id":     workspaceID.String(),
				"agent_id":         conn.AgentID,
				"operations_count": len(syncParams.Operations),
			}
			s.notificationManager.BroadcastNotification(ctx, fmt.Sprintf("workspace:%s", workspaceID), "workspace.state_synced", notification)
		}

		return map[string]interface{}{
			"workspace_id":       workspaceID.String(),
			"state":              state,
			"synced":             true,
			"operations_applied": len(syncParams.Operations),
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"workspace_id":       workspaceID.String(),
		"synced":             true,
		"operations_applied": len(syncParams.Operations),
	}, nil
}

// handleConflictDetect detects conflicts for an entity
func (s *Server) handleConflictDetect(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var detectParams struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
	}

	if err := json.Unmarshal(params, &detectParams); err != nil {
		return nil, err
	}

	entityID, err := uuid.Parse(detectParams.EntityID)
	if err != nil {
		return nil, fmt.Errorf("invalid entity ID: %w", err)
	}

	if s.conflictService != nil {
		conflicts, err := s.conflictService.DetectConflicts(ctx, detectParams.EntityType, entityID)
		if err != nil {
			return nil, fmt.Errorf("failed to detect conflicts: %w", err)
		}

		// Convert conflicts to response format
		var conflictList []map[string]interface{}
		for _, conflict := range conflicts {
			conflictList = append(conflictList, map[string]interface{}{
				"id":             conflict.ID.String(),
				"type":           conflict.Type,
				"document_id":    conflict.DocumentID.String(),
				"local_version":  conflict.LocalVersion,
				"remote_version": conflict.RemoteVersion,
				"affected_path":  conflict.AffectedPath,
				"detected_at":    conflict.DetectedAt,
				"metadata":       conflict.Metadata,
			})
		}

		return map[string]interface{}{
			"entity_type":    detectParams.EntityType,
			"entity_id":      entityID.String(),
			"conflicts":      conflictList,
			"conflict_count": len(conflicts),
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"entity_type":    detectParams.EntityType,
		"entity_id":      entityID.String(),
		"conflicts":      []map[string]interface{}{},
		"conflict_count": 0,
	}, nil
}

// handleVectorClockGet retrieves the vector clock for a node
func (s *Server) handleVectorClockGet(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	if s.conflictService != nil {
		clock, err := s.conflictService.GetVectorClock(ctx, conn.AgentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get vector clock: %w", err)
		}

		// Convert to response format
		clockMap := make(map[string]uint64)
		for nodeID, value := range clock {
			clockMap[string(nodeID)] = value
		}

		return map[string]interface{}{
			"node_id": conn.AgentID,
			"clock":   clockMap,
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"node_id": conn.AgentID,
		"clock": map[string]uint64{
			conn.AgentID: 1,
		},
	}, nil
}

// handleVectorClockUpdate updates the vector clock for a node
func (s *Server) handleVectorClockUpdate(ctx context.Context, conn *Connection, params json.RawMessage) (interface{}, error) {
	var updateParams struct {
		Clock map[string]uint64 `json:"clock"`
	}

	if err := json.Unmarshal(params, &updateParams); err != nil {
		return nil, err
	}

	if s.conflictService != nil {
		clock := convertToCRDTClock(updateParams.Clock)
		if err := s.conflictService.UpdateVectorClock(ctx, conn.AgentID, clock); err != nil {
			return nil, fmt.Errorf("failed to update vector clock: %w", err)
		}

		return map[string]interface{}{
			"node_id": conn.AgentID,
			"clock":   updateParams.Clock,
			"updated": true,
		}, nil
	}

	// Mock response
	return map[string]interface{}{
		"node_id": conn.AgentID,
		"clock":   updateParams.Clock,
		"updated": true,
	}, nil
}

// Helper function to convert map to CRDT vector clock
func convertToCRDTClock(clockMap map[string]uint64) crdt.VectorClock {
	clock := crdt.NewVectorClock()
	for nodeID, value := range clockMap {
		clock[crdt.NodeID(nodeID)] = value
	}
	return clock
}
