package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
)

// AgentProvider provides agent management tools
type AgentProvider struct {
	// Store agent status in memory for standalone mode
	agentStatus map[string]*AgentStatus
}

// AgentStatus represents the status of an agent
type AgentStatus struct {
	AgentID       string    `json:"agent_id"`
	Status        string    `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Version       string    `json:"version,omitempty"`
	Capabilities  []string  `json:"capabilities,omitempty"`
}

// NewAgentProvider creates a new agent provider
func NewAgentProvider() *AgentProvider {
	return &AgentProvider{
		agentStatus: make(map[string]*AgentStatus),
	}
}

// GetDefinitions returns the tool definitions for agent management
func (p *AgentProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "agent_list",
			Description: "List all registered agents with their current status",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter agents by status (online, busy, offline)",
						"enum":        []string{"online", "busy", "offline"},
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of agents to return (default: 50, max: 100)",
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Number of agents to skip for pagination (default: 0)",
						"minimum":     0,
					},
					"sort_by": map[string]interface{}{
						"type":        "string",
						"description": "Field to sort by",
						"enum":        []string{"agent_id", "status", "last_heartbeat"},
					},
					"sort_order": map[string]interface{}{
						"type":        "string",
						"description": "Sort order (default: asc)",
						"enum":        []string{"asc", "desc"},
					},
				},
			},
			Handler: p.handleList,
		},
		{
			Name:        "agent_heartbeat",
			Description: "Send agent heartbeat to maintain connection status",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the agent",
					},
					"timestamp": map[string]interface{}{
						"type":        "number",
						"description": "Unix timestamp of the heartbeat",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Current agent status (online, busy, offline)",
						"enum":        []string{"online", "busy", "offline"},
					},
				},
				"required": []string{"agent_id"},
			},
			Handler: p.handleHeartbeat,
		},
		{
			Name:        "agent_status",
			Description: "Get the current status of an agent",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the agent",
					},
				},
				"required": []string{"agent_id"},
			},
			Handler: p.handleStatus,
		},
	}
}

// handleHeartbeat processes agent heartbeat
func (p *AgentProvider) handleHeartbeat(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		AgentID   string  `json:"agent_id"`
		Timestamp float64 `json:"timestamp,omitempty"`
		Status    string  `json:"status,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	// Update or create agent status
	status, exists := p.agentStatus[params.AgentID]
	if !exists {
		status = &AgentStatus{
			AgentID: params.AgentID,
		}
		p.agentStatus[params.AgentID] = status
	}

	// Update heartbeat time
	status.LastHeartbeat = time.Now()

	// Update status if provided
	if params.Status != "" {
		status.Status = params.Status
	} else if status.Status == "" {
		status.Status = "online"
	}

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Heartbeat received for agent %s", params.AgentID),
		"status":  status.Status,
	}, nil
}

// handleStatus returns the status of an agent
func (p *AgentProvider) handleStatus(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		AgentID string `json:"agent_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	status, exists := p.agentStatus[params.AgentID]
	if !exists {
		return map[string]interface{}{
			"agent_id": params.AgentID,
			"status":   "unknown",
			"message":  "Agent not found or no heartbeat received",
		}, nil
	}

	// Check if heartbeat is stale (older than 5 minutes)
	if time.Since(status.LastHeartbeat) > 5*time.Minute {
		status.Status = "offline"
	}

	return map[string]interface{}{
		"agent_id":       status.AgentID,
		"status":         status.Status,
		"last_heartbeat": status.LastHeartbeat.Format(time.RFC3339),
		"version":        status.Version,
		"capabilities":   status.Capabilities,
	}, nil
}

// handleList returns a list of all agents
func (p *AgentProvider) handleList(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Status    string `json:"status,omitempty"`
		Limit     int    `json:"limit,omitempty"`
		Offset    int    `json:"offset,omitempty"`
		SortBy    string `json:"sort_by,omitempty"`
		SortOrder string `json:"sort_order,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 50
	} else if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.SortOrder == "" {
		params.SortOrder = "asc"
	}

	agents := make([]map[string]interface{}, 0)
	now := time.Now()

	// Collect all matching agents
	for _, status := range p.agentStatus {
		// Update status based on heartbeat freshness
		if time.Since(status.LastHeartbeat) > 5*time.Minute {
			status.Status = "offline"
		}

		// Apply filter if specified
		if params.Status != "" && status.Status != params.Status {
			continue
		}

		agents = append(agents, map[string]interface{}{
			"agent_id":       status.AgentID,
			"status":         status.Status,
			"last_heartbeat": status.LastHeartbeat.Format(time.RFC3339),
			"age_seconds":    int(now.Sub(status.LastHeartbeat).Seconds()),
			"version":        status.Version,
			"capabilities":   status.Capabilities,
		})
	}

	// Sort agents if requested
	if params.SortBy != "" {
		sort.Slice(agents, func(i, j int) bool {
			var less bool
			switch params.SortBy {
			case "agent_id":
				less = agents[i]["agent_id"].(string) < agents[j]["agent_id"].(string)
			case "status":
				less = agents[i]["status"].(string) < agents[j]["status"].(string)
			case "last_heartbeat":
				less = agents[i]["last_heartbeat"].(string) < agents[j]["last_heartbeat"].(string)
			default:
				return false
			}
			if params.SortOrder == "desc" {
				return !less
			}
			return less
		})
	}

	// Apply pagination
	totalCount := len(agents)
	start := params.Offset
	end := params.Offset + params.Limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	paginatedAgents := agents[start:end]

	return map[string]interface{}{
		"agents":      paginatedAgents,
		"count":       len(paginatedAgents),
		"total_count": totalCount,
		"offset":      params.Offset,
		"limit":       params.Limit,
		"timestamp":   now.Format(time.RFC3339),
	}, nil
}
