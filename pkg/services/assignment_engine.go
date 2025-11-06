package services

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rules"
	"github.com/pkg/errors"
)

// AssignmentEngine handles intelligent task assignment
type AssignmentEngine struct {
	ruleEngine   rules.Engine
	agentService AgentService
	strategies   map[string]AssignmentStrategy
	logger       observability.Logger
	metrics      observability.MetricsClient

	// Metrics and caching
	assignments   map[string]int
	workloadCache sync.Map // Cache agent workloads for 30 seconds
	cacheDuration time.Duration
}

// NewAssignmentEngine creates a new assignment engine
func NewAssignmentEngine(ruleEngine rules.Engine, agentService AgentService, logger observability.Logger, metrics observability.MetricsClient) *AssignmentEngine {
	e := &AssignmentEngine{
		ruleEngine:    ruleEngine,
		agentService:  agentService,
		strategies:    make(map[string]AssignmentStrategy),
		logger:        logger,
		metrics:       metrics,
		assignments:   make(map[string]int),
		cacheDuration: 30 * time.Second,
	}

	// Register default strategies
	e.RegisterStrategy("round_robin", &RoundRobinStrategy{})
	e.RegisterStrategy("least_loaded", &LeastLoadedStrategy{
		agentService: agentService,
		logger:       logger,
		metrics:      metrics,
	})
	e.RegisterStrategy("capability_match", &CapabilityMatchStrategy{
		logger:  logger,
		metrics: metrics,
	})
	e.RegisterStrategy("performance_based", &PerformanceBasedStrategy{
		agentService:  agentService,
		logger:        logger,
		metrics:       metrics,
		cacheDuration: 5 * time.Minute,
	})
	e.RegisterStrategy("cost_optimized", &CostOptimizedStrategy{
		logger:  logger,
		metrics: metrics,
	})

	// Register new orchestration strategies (Phase 1.2)
	e.RegisterStrategy("affinity", NewAffinityStrategy(agentService, logger))
	e.RegisterStrategy("hierarchical_cascade", NewHierarchicalCascadeStrategy(logger, metrics))
	e.RegisterStrategy("priority_queue", NewPriorityQueueStrategy(agentService, logger, metrics))
	e.RegisterStrategy("collaborative_team", NewCollaborativeTeamStrategy(logger, metrics))
	e.RegisterStrategy("predictive", NewPredictiveLoadStrategy(agentService, logger, metrics))

	logger.Info("Assignment engine initialized with 10 strategies", map[string]interface{}{
		"legacy_strategies": []string{
			"round_robin",
			"least_loaded",
			"capability_match",
			"performance_based",
			"cost_optimized",
		},
		"orchestration_strategies": []string{
			"affinity",
			"hierarchical_cascade",
			"priority_queue",
			"collaborative_team",
			"predictive",
		},
	})

	return e
}

// RegisterStrategy registers an assignment strategy
func (e *AssignmentEngine) RegisterStrategy(name string, strategy AssignmentStrategy) {
	e.strategies[name] = strategy
}

// FindBestAgent finds the best agent for a task using rules and strategies
func (e *AssignmentEngine) FindBestAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	// Get assignment rules if rule engine is configured
	var assignmentRules []rules.Rule
	if e.ruleEngine != nil {
		var err error
		assignmentRules, err = e.ruleEngine.GetRules(ctx, "task.assignment", map[string]interface{}{
			"task_type": task.Type,
			"priority":  task.Priority,
		})
		if err != nil {
			return nil, err
		}
	}

	// Get eligible agents
	eligibleAgents, err := e.getEligibleAgents(ctx, task, assignmentRules)
	if err != nil {
		return nil, err
	}

	if len(eligibleAgents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Determine strategy
	strategy := e.determineStrategy(ctx, task, assignmentRules)

	// Apply strategy
	return strategy.Assign(ctx, task, eligibleAgents)
}

func (e *AssignmentEngine) getEligibleAgents(ctx context.Context, task *models.Task, rules []rules.Rule) ([]*models.Agent, error) {
	// Get all available agents with caching
	agents, err := e.agentService.GetAvailableAgents(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get available agents")
	}

	// Track metrics
	totalAgents := len(agents)
	filteredOut := 0

	// Filter agents by multiple criteria
	var eligibleAgents []*models.Agent
	for _, agent := range agents {
		// 1. Check tenant match (multi-tenant isolation)
		if agent.TenantID != task.TenantID {
			filteredOut++
			continue
		}

		// 2. Check agent status - must be Active
		if agent.Status != string(models.AgentStatusActive) {
			filteredOut++
			e.logger.Debug("Agent filtered out due to status", map[string]interface{}{
				"agent_id": agent.ID,
				"status":   agent.Status,
			})
			continue
		}

		// 3. Get agent capabilities with error handling
		capabilities, err := e.agentService.GetAgentCapabilities(ctx, agent.ID)
		if err != nil {
			e.logger.Warn("Failed to get agent capabilities", map[string]interface{}{
				"agent_id": agent.ID,
				"error":    err.Error(),
			})
			filteredOut++
			continue
		}

		// 4. Check capability matching
		requiredCaps := e.extractRequiredCapabilities(task)
		if !e.hasRequiredCapabilities(agent, capabilities, requiredCaps) {
			filteredOut++
			continue
		}

		// 5. Check agent workload if not overloaded
		if e.isAgentOverloaded(ctx, agent.ID) {
			filteredOut++
			e.logger.Debug("Agent filtered out due to overload", map[string]interface{}{
				"agent_id": agent.ID,
			})
			continue
		}

		// Agent passed all filters
		eligibleAgents = append(eligibleAgents, agent)
	}

	// 6. Apply rule-based filtering if rule engine is available
	if e.ruleEngine != nil && len(rules) > 0 && len(eligibleAgents) > 0 {
		filtered := make([]*models.Agent, 0, len(eligibleAgents))
		for _, agent := range eligibleAgents {
			eligible := true

			// Get agent capabilities for rule evaluation
			agentCaps, _ := e.agentService.GetAgentCapabilities(ctx, agent.ID)

			for _, rule := range rules {
				// Evaluate rule against agent and task context
				decision, err := e.ruleEngine.Evaluate(ctx, rule.ID.String(), map[string]interface{}{
					"agent":              agent,
					"task":               task,
					"agent_capabilities": agentCaps,
				})
				if err != nil {
					e.logger.Warn("Rule evaluation failed", map[string]interface{}{
						"rule_id": rule.ID,
						"error":   err.Error(),
					})
					eligible = false
					break
				}
				if decision == nil || !decision.Allowed {
					eligible = false
					e.logger.Debug("Agent filtered by rule", map[string]interface{}{
						"agent_id": agent.ID,
						"rule":     rule.Name,
						"reason":   decision.Reason,
					})
					break
				}
			}
			if eligible {
				filtered = append(filtered, agent)
			} else {
				filteredOut++
			}
		}
		eligibleAgents = filtered
	}

	// Track metrics
	e.metrics.RecordGauge("assignment.eligible_agents", float64(len(eligibleAgents)), map[string]string{
		"task_type": string(task.Type),
	})
	e.metrics.RecordGauge("assignment.filtered_agents", float64(filteredOut), map[string]string{
		"task_type": string(task.Type),
	})

	// Log assignment decision
	e.logger.Info("Agent filtering completed", map[string]interface{}{
		"task_id":         task.ID,
		"task_type":       task.Type,
		"total_agents":    totalAgents,
		"eligible_agents": len(eligibleAgents),
		"filtered_out":    filteredOut,
	})

	return eligibleAgents, nil
}

func (e *AssignmentEngine) determineStrategy(ctx context.Context, task *models.Task, rules []rules.Rule) AssignmentStrategy {
	// 1. Check task priority for urgent tasks
	if task.Priority == models.TaskPriorityHigh || task.Priority == models.TaskPriorityCritical {
		// High priority tasks should use performance-based assignment
		e.logger.Debug("Using performance-based strategy for high priority task", map[string]interface{}{
			"task_id":  task.ID,
			"priority": task.Priority,
		})
		return e.strategies["performance_based"]
	}

	// 2. Check for cost-sensitive tasks
	if task.Parameters != nil {
		if costSensitive, ok := task.Parameters["cost_sensitive"].(bool); ok && costSensitive {
			e.logger.Debug("Using cost-optimized strategy for cost-sensitive task", map[string]interface{}{
				"task_id": task.ID,
			})
			return e.strategies["cost_optimized"]
		}
	}

	// 3. Check rules from rule engine if available
	if e.ruleEngine != nil && len(rules) > 0 {
		for _, rule := range rules {
			// Check if rule relates to assignment strategy
			if rule.Name == "assignment_strategy" || rule.Category == "assignment" {
				// Evaluate rule to get strategy preference
				decision, err := e.ruleEngine.Evaluate(ctx, rule.ID.String(), map[string]interface{}{
					"task":      task,
					"priority":  task.Priority,
					"task_type": task.Type,
				})
				if err == nil && decision != nil && decision.Allowed {
					// Try to extract strategy from decision metadata
					if decision.Metadata != nil {
						if strategyName, ok := decision.Metadata["strategy"].(string); ok {
							if strategy, exists := e.strategies[strategyName]; exists {
								e.logger.Info("Using rule-based strategy selection", map[string]interface{}{
									"task_id":  task.ID,
									"rule":     rule.Name,
									"strategy": strategyName,
								})
								return strategy
							}
						}
					}
				}
			}
		}
	}

	// 4. Default strategy based on task type and characteristics
	switch task.Type {
	case "build", "deploy":
		// For builds and deploys, use least loaded to distribute work evenly
		e.logger.Debug("Using least-loaded strategy for build/deploy task", map[string]interface{}{
			"task_id":   task.ID,
			"task_type": task.Type,
		})
		return e.strategies["least_loaded"]

	case "test":
		// For tests, use capability match to ensure proper environment
		e.logger.Debug("Using capability-match strategy for test task", map[string]interface{}{
			"task_id":   task.ID,
			"task_type": task.Type,
		})
		return e.strategies["capability_match"]

	case "monitor", "health_check":
		// For monitoring, use round robin for even distribution
		e.logger.Debug("Using round-robin strategy for monitoring task", map[string]interface{}{
			"task_id":   task.ID,
			"task_type": task.Type,
		})
		return e.strategies["round_robin"]

	case "backup", "restore":
		// For backups, use performance based to complete quickly
		e.logger.Debug("Using performance-based strategy for backup/restore task", map[string]interface{}{
			"task_id":   task.ID,
			"task_type": task.Type,
		})
		return e.strategies["performance_based"]

	case "data_processing", "analysis":
		// For data tasks, consider cost if large
		if task.Parameters != nil {
			if dataSize, ok := task.Parameters["data_size"].(int); ok && dataSize > 1000000 {
				e.logger.Debug("Using cost-optimized strategy for large data processing task", map[string]interface{}{
					"task_id":   task.ID,
					"data_size": dataSize,
				})
				return e.strategies["cost_optimized"]
			}
		}
		return e.strategies["least_loaded"]

	default:
		// Default to capability match for safety and correctness
		e.logger.Debug("Using default capability-match strategy", map[string]interface{}{
			"task_id":   task.ID,
			"task_type": task.Type,
		})
		return e.strategies["capability_match"]
	}
}

// RoundRobinStrategy assigns tasks in round-robin fashion
type RoundRobinStrategy struct {
	counter uint64
	mu      sync.Mutex
}

func (s *RoundRobinStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	s.mu.Lock()
	index := s.counter % uint64(len(agents))
	s.counter++
	s.mu.Unlock()

	return agents[index], nil
}

func (s *RoundRobinStrategy) GetName() string {
	return "round_robin"
}

// LeastLoadedStrategy assigns tasks to the least loaded agent
type LeastLoadedStrategy struct {
	agentService AgentService
	logger       observability.Logger
	metrics      observability.MetricsClient
}

func (s *LeastLoadedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	type agentLoad struct {
		agent     *models.Agent
		loadScore float64
		workload  *models.AgentWorkload
	}

	agentLoads := make([]agentLoad, 0, len(agents))

	// Get workload for each agent and calculate load score
	for _, agent := range agents {
		workload, err := s.agentService.GetAgentWorkload(ctx, agent.ID)
		if err != nil {
			// If we can't get workload, assume minimal load but log warning
			workload = &models.AgentWorkload{
				AgentID:     agent.ID,
				ActiveTasks: 0,
				QueuedTasks: 0,
				LoadScore:   0.1, // Small penalty for unknown workload
			}
		}

		// Calculate load score: activeTasks * 0.7 + queuedTasks * 0.3
		loadScore := float64(workload.ActiveTasks)*0.7 + float64(workload.QueuedTasks)*0.3

		// Factor in the existing load score if available
		if workload.LoadScore > 0 {
			loadScore = (loadScore + workload.LoadScore*100) / 2 // Average with scaled LoadScore
		}

		agentLoads = append(agentLoads, agentLoad{
			agent:     agent,
			loadScore: loadScore,
			workload:  workload,
		})
	}

	// Sort by load score (ascending)
	sort.Slice(agentLoads, func(i, j int) bool {
		return agentLoads[i].loadScore < agentLoads[j].loadScore
	})

	// Handle tie-breaking with random selection among equally loaded agents
	minLoad := agentLoads[0].loadScore
	var candidates []agentLoad
	for _, al := range agentLoads {
		if al.loadScore == minLoad {
			candidates = append(candidates, al)
		} else {
			break // Since sorted, no more will have min load
		}
	}

	// Random selection among candidates for tie-breaking
	selectedIndex := 0
	if len(candidates) > 1 {
		selectedIndex = rand.Intn(len(candidates))
	}

	selected := candidates[selectedIndex]

	// Log the selection decision
	s.logger.Info("Least loaded agent selected", map[string]interface{}{
		"task_id":      task.ID,
		"agent_id":     selected.agent.ID,
		"load_score":   selected.loadScore,
		"active_tasks": selected.workload.ActiveTasks,
		"queued_tasks": selected.workload.QueuedTasks,
		"candidates":   len(candidates),
	})

	// Track metrics
	s.metrics.RecordGauge("assignment.load_score", selected.loadScore, map[string]string{
		"agent_id": selected.agent.ID,
	})

	return selected.agent, nil
}

func (s *LeastLoadedStrategy) GetName() string {
	return "least_loaded"
}

// CapabilityMatchStrategy assigns tasks based on agent capabilities
type CapabilityMatchStrategy struct {
	logger  observability.Logger
	metrics observability.MetricsClient
}

func (s *CapabilityMatchStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Extract required capabilities using the method
	requiredCaps := s.extractCapabilities(task)

	// Score agents based on capability match
	type agentScore struct {
		agent       *models.Agent
		score       float64
		matchedCaps []string
		missingCaps []string
	}

	scores := make([]agentScore, 0, len(agents))

	for _, agent := range agents {
		// Calculate detailed capability matching
		matched, missing := s.getCapabilityDetails(agent.Capabilities, requiredCaps)
		score := float64(len(matched)) / float64(len(requiredCaps))

		// Bonus for agents with extra relevant capabilities
		extraScore := 0.0
		for _, cap := range agent.Capabilities {
			if s.isRelevantCapability(cap, task.Type) && !contains(requiredCaps, cap) {
				extraScore += 0.1
			}
		}

		// Cap extra score at 0.3
		if extraScore > 0.3 {
			extraScore = 0.3
		}

		totalScore := score + extraScore

		scores = append(scores, agentScore{
			agent:       agent,
			score:       totalScore,
			matchedCaps: matched,
			missingCaps: missing,
		})
	}

	// Sort by score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Only select agents with 100% required capability match
	var eligibleScores []agentScore
	for _, as := range scores {
		if len(as.missingCaps) == 0 {
			eligibleScores = append(eligibleScores, as)
		}
	}

	if len(eligibleScores) == 0 {
		if s.logger != nil {
			s.logger.Warn("No agent with all required capabilities", map[string]interface{}{
				"task_id":          task.ID,
				"required_caps":    requiredCaps,
				"best_match_score": scores[0].score,
				"best_match_agent": scores[0].agent.ID,
				"missing_caps":     scores[0].missingCaps,
			})
		}
		return nil, ErrNoCapableAgent
	}

	// Select the best match
	selected := eligibleScores[0]

	if s.logger != nil {
		s.logger.Info("Capability-matched agent selected", map[string]interface{}{
			"task_id":       task.ID,
			"agent_id":      selected.agent.ID,
			"score":         selected.score,
			"matched_caps":  selected.matchedCaps,
			"required_caps": requiredCaps,
		})
	}

	if s.metrics != nil {
		s.metrics.RecordGauge("assignment.capability_score", selected.score, map[string]string{
			"agent_id":  selected.agent.ID,
			"task_type": task.Type,
		})
	}

	return selected.agent, nil
}

// extractCapabilities extracts required capabilities from the task
func (s *CapabilityMatchStrategy) extractCapabilities(task *models.Task) []string {
	return extractRequiredCapabilities(task)
}

// getCapabilityDetails returns matched and missing capabilities
func (s *CapabilityMatchStrategy) getCapabilityDetails(agentCaps, requiredCaps []string) (matched, missing []string) {
	capSet := make(map[string]bool)
	for _, cap := range agentCaps {
		capSet[cap] = true
	}

	for _, req := range requiredCaps {
		if capSet[req] || capSet["all"] {
			matched = append(matched, req)
		} else {
			missing = append(missing, req)
		}
	}

	return matched, missing
}

// isRelevantCapability checks if a capability is relevant to the task type
func (s *CapabilityMatchStrategy) isRelevantCapability(capability, taskType string) bool {
	// Task type itself is always relevant
	if capability == taskType {
		return true
	}

	// Check for language/tool relevance
	relevanceMap := map[string][]string{
		"build":   {"docker", "maven", "gradle", "npm", "go", "make"},
		"test":    {"junit", "pytest", "jest", "go-test", "selenium"},
		"deploy":  {"kubernetes", "docker", "terraform", "ansible", "helm"},
		"monitor": {"prometheus", "grafana", "datadog", "newrelic"},
	}

	if relevantCaps, ok := relevanceMap[taskType]; ok {
		for _, rc := range relevantCaps {
			if capability == rc {
				return true
			}
		}
	}

	return false
}

func (s *CapabilityMatchStrategy) GetName() string {
	return "capability_match"
}

// PerformanceBasedStrategy assigns tasks based on agent performance metrics
type PerformanceBasedStrategy struct {
	performanceCache sync.Map
	agentService     AgentService
	logger           observability.Logger
	metrics          observability.MetricsClient
	cacheDuration    time.Duration
}

func (s *PerformanceBasedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	type agentPerf struct {
		agent       *models.Agent
		performance *models.AgentPerformance
		score       float64
	}

	performances := make([]agentPerf, 0, len(agents))

	for _, agent := range agents {
		// Get performance metrics with caching
		perf := s.getAgentPerformance(ctx, agent.ID, task.Type)

		// Calculate comprehensive performance score
		score := s.calculatePerformance(perf, task)

		performances = append(performances, agentPerf{
			agent:       agent,
			performance: perf,
			score:       score,
		})
	}

	// Sort by performance score (descending)
	sort.Slice(performances, func(i, j int) bool {
		return performances[i].score > performances[j].score
	})

	// Select the best performer
	selected := performances[0]

	if s.logger != nil {
		s.logger.Info("Performance-based agent selected", map[string]interface{}{
			"task_id":       task.ID,
			"agent_id":      selected.agent.ID,
			"score":         selected.score,
			"success_rate":  selected.performance.SuccessRate,
			"speed_score":   selected.performance.SpeedScore,
			"load_factor":   selected.performance.LoadFactor,
			"avg_comp_time": selected.performance.AverageCompletionTime,
		})
	}

	if s.metrics != nil {
		s.metrics.RecordGauge("assignment.performance_score", selected.score, map[string]string{
			"agent_id":  selected.agent.ID,
			"task_type": task.Type,
		})
	}

	return selected.agent, nil
}

// getAgentPerformance retrieves cached or fresh performance metrics
func (s *PerformanceBasedStrategy) getAgentPerformance(ctx context.Context, agentID string, taskType string) *models.AgentPerformance {
	cacheKey := fmt.Sprintf("perf_%s_%s", agentID, taskType)

	// Check cache with expiry
	if cached, ok := s.performanceCache.Load(cacheKey); ok {
		entry := cached.(*performanceCacheEntry)
		if s.cacheDuration == 0 || time.Since(entry.timestamp) < s.cacheDuration {
			return entry.performance
		}
	}

	// Get fresh performance metrics by analyzing agent workload and metadata
	perf := &models.AgentPerformance{
		AgentID: agentID,
	}

	// Get current workload to determine load factor
	workload, err := s.agentService.GetAgentWorkload(ctx, agentID)
	if err == nil {
		perf.LoadFactor = workload.LoadScore
		// Estimate performance based on current load
		if workload.LoadScore < 0.3 {
			perf.SpeedScore = 0.9 // Low load = high speed
		} else if workload.LoadScore < 0.7 {
			perf.SpeedScore = 0.7 // Medium load
		} else {
			perf.SpeedScore = 0.5 // High load = lower speed
		}
	} else {
		perf.LoadFactor = 0.5 // Default 50% load
		perf.SpeedScore = 0.7 // Default speed
	}

	// Get agent details for metadata-based performance metrics
	agent, err := s.agentService.GetAgent(ctx, agentID)
	if err == nil && agent.Metadata != nil {
		// Extract performance metrics from agent metadata
		if tasksCompleted, ok := agent.Metadata["tasks_completed"].(float64); ok {
			perf.TasksCompleted = int64(tasksCompleted)
		}
		if tasksFailed, ok := agent.Metadata["tasks_failed"].(float64); ok {
			perf.TasksFailed = int64(tasksFailed)
		}

		// Calculate success rate
		if perf.TasksCompleted > 0 {
			perf.SuccessRate = float64(perf.TasksCompleted-perf.TasksFailed) / float64(perf.TasksCompleted)
		} else {
			perf.SuccessRate = 0.8 // Default 80%
		}

		// Get average completion time
		if avgTime, ok := agent.Metadata["avg_completion_time"].(float64); ok {
			perf.AverageCompletionTime = avgTime
		} else {
			perf.AverageCompletionTime = 300 // Default 5 minutes
		}

		// Get task-type specific metrics
		if taskTypeMetrics, ok := agent.Metadata["task_type_metrics"].(map[string]interface{}); ok {
			perf.TaskTypeMetrics = make(map[string]models.TaskMetrics)
			for taskType, metrics := range taskTypeMetrics {
				if metricsMap, ok := metrics.(map[string]interface{}); ok {
					tm := models.TaskMetrics{}
					if count, ok := metricsMap["count"].(float64); ok {
						tm.Count = int64(count)
					}
					if successRate, ok := metricsMap["success_rate"].(float64); ok {
						tm.SuccessRate = successRate
					}
					if avgTime, ok := metricsMap["average_time"].(float64); ok {
						tm.AverageTime = avgTime
					}
					perf.TaskTypeMetrics[taskType] = tm
				}
			}
		}
	} else {
		// Use default performance if unable to fetch
		perf = &models.AgentPerformance{
			AgentID:               agentID,
			TasksCompleted:        0,
			TasksFailed:           0,
			AverageCompletionTime: 300, // 5 minutes default
			SuccessRate:           0.8, // 80% default
			LoadFactor:            0.5, // 50% default load
			SpeedScore:            0.7, // 70% default speed
		}

		if s.logger != nil {
			s.logger.Warn("Failed to get agent performance, using defaults", map[string]interface{}{
				"agent_id": agentID,
				"error":    err.Error(),
			})
		}
	}

	// Adjust for task-type specific metrics if available
	if perf.TaskTypeMetrics != nil {
		if taskMetrics, ok := perf.TaskTypeMetrics[taskType]; ok {
			// Use task-type specific success rate if available
			if taskMetrics.SuccessRate > 0 {
				perf.SuccessRate = taskMetrics.SuccessRate
			}
			// Use task-type specific average time
			if taskMetrics.AverageTime > 0 {
				perf.AverageCompletionTime = taskMetrics.AverageTime
			}
		}
	}

	// Cache the performance
	s.performanceCache.Store(cacheKey, &performanceCacheEntry{
		performance: perf,
		timestamp:   time.Now(),
	})

	return perf
}

// calculatePerformance calculates a comprehensive performance score
func (s *PerformanceBasedStrategy) calculatePerformance(perf *models.AgentPerformance, task *models.Task) float64 {
	// Base score components with weights
	successWeight := 0.5      // 50% - Success rate is most important
	speedWeight := 0.3        // 30% - Speed efficiency
	availabilityWeight := 0.2 // 20% - Current availability

	// Calculate success score (0-1)
	successScore := perf.SuccessRate

	// Calculate speed score (0-1)
	// Normalize based on expected completion time
	expectedTime := 300.0 // 5 minutes default
	if task.TimeoutSeconds > 0 {
		expectedTime = float64(task.TimeoutSeconds)
	}

	speedScore := perf.SpeedScore
	if perf.AverageCompletionTime > 0 {
		// Calculate speed score based on how fast agent completes vs expected
		ratio := expectedTime / perf.AverageCompletionTime
		if ratio > 2.0 {
			ratio = 2.0 // Cap at 2x faster
		}
		speedScore = ratio / 2.0 // Normalize to 0-1
	}

	// Calculate availability score (inverse of load factor)
	availabilityScore := 1.0 - perf.LoadFactor

	// Apply task priority modifiers
	if task.Priority == models.TaskPriorityHigh || task.Priority == models.TaskPriorityCritical {
		// For high priority tasks, weight speed more heavily
		speedWeight = 0.4
		successWeight = 0.45
		availabilityWeight = 0.15
	}

	// Calculate weighted score
	score := (successScore * successWeight) +
		(speedScore * speedWeight) +
		(availabilityScore * availabilityWeight)

	// Apply penalties
	if perf.TasksFailed > 10 && perf.SuccessRate < 0.7 {
		// Penalty for agents with many recent failures
		score *= 0.8
	}

	// Apply bonuses
	if perf.TasksCompleted > 100 && perf.SuccessRate > 0.95 {
		// Bonus for highly reliable agents
		score *= 1.1
	}

	// Cap score at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

func (s *PerformanceBasedStrategy) GetName() string {
	return "performance_based"
}

// CostOptimizedStrategy assigns tasks based on cost optimization
type CostOptimizedStrategy struct {
	logger  observability.Logger
	metrics observability.MetricsClient
}

func (s *CostOptimizedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Calculate cost for each agent based on their rate and estimated task duration
	type agentCost struct {
		agent          *models.Agent
		cost           float64
		hourlyRate     float64
		estimatedHours float64
	}

	costs := make([]agentCost, 0, len(agents))

	// Get estimated task duration from task parameters if available
	defaultEstimatedHours := 1.0 // Default 1 hour
	if task.Parameters != nil {
		if duration, ok := task.Parameters["estimated_duration"].(float64); ok {
			defaultEstimatedHours = duration / 3600 // Convert seconds to hours
		}
	}

	for _, agent := range agents {
		// Get agent cost per hour from metadata
		hourlyRate := 10.0 // Default rate in dollars
		if agent.Metadata != nil {
			// Check different possible rate fields
			if rate, ok := agent.Metadata["hourly_rate"].(float64); ok {
				hourlyRate = rate
			} else if rate, ok := agent.Metadata["cost_per_hour"].(float64); ok {
				hourlyRate = rate
			} else if rate, ok := agent.Metadata["rate"].(float64); ok {
				hourlyRate = rate
			}
		}

		// Get agent-specific performance metrics for this task type
		estimatedHours := defaultEstimatedHours
		if agent.Metadata != nil {
			// Check task-type specific metrics first
			taskTypeKey := fmt.Sprintf("avg_%s_time", task.Type)
			if avgTime, ok := agent.Metadata[taskTypeKey].(float64); ok {
				estimatedHours = avgTime / 3600 // Convert seconds to hours
			} else if avgTime, ok := agent.Metadata["avg_task_time"].(float64); ok {
				// Fall back to general average
				estimatedHours = avgTime / 3600 // Convert seconds to hours
			}

			// Apply performance factor if available
			if perfFactor, ok := agent.Metadata["performance_factor"].(float64); ok {
				estimatedHours = estimatedHours / perfFactor // Higher performance = less time
			}
		}

		// Calculate base cost
		baseCost := hourlyRate * estimatedHours

		// Add additional cost factors
		totalCost := baseCost

		// Add overhead costs if any
		if agent.Metadata != nil {
			if overhead, ok := agent.Metadata["overhead_cost"].(float64); ok {
				totalCost += overhead
			}

			// Add setup cost for first-time task types
			if setupCost, ok := agent.Metadata["setup_cost"].(float64); ok {
				if isFirstTimeTaskType(agent, task.Type) {
					totalCost += setupCost
				}
			}
		}

		// Factor in task complexity if specified
		if task.Parameters != nil {
			if complexity, ok := task.Parameters["complexity"].(float64); ok {
				totalCost *= (1 + complexity/10) // 10% increase per complexity point
			}
		}

		costs = append(costs, agentCost{
			agent:          agent,
			cost:           totalCost,
			hourlyRate:     hourlyRate,
			estimatedHours: estimatedHours,
		})
	}

	// Sort by cost (ascending)
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].cost < costs[j].cost
	})

	// Check if we need to consider capabilities as well
	// The cheapest agent must still have required capabilities
	selected := costs[0]

	if s.logger != nil {
		s.logger.Info("Cost-optimized agent selected", map[string]interface{}{
			"task_id":         task.ID,
			"agent_id":        selected.agent.ID,
			"total_cost":      selected.cost,
			"hourly_rate":     selected.hourlyRate,
			"estimated_hours": selected.estimatedHours,
		})
	}

	if s.metrics != nil {
		s.metrics.RecordGauge("assignment.estimated_cost", selected.cost, map[string]string{
			"agent_id":  selected.agent.ID,
			"task_type": task.Type,
		})
	}

	return selected.agent, nil
}

func (s *CostOptimizedStrategy) GetName() string {
	return "cost_optimized"
}

// Helper functions

func extractRequiredCapabilities(task *models.Task) []string {
	caps := []string{}

	// Always require the task type capability
	if task.Type != "" {
		caps = append(caps, task.Type)
	}

	// Extract capabilities from task parameters
	if task.Parameters != nil {
		// Check for required_capabilities parameter
		if reqCaps, ok := task.Parameters["required_capabilities"].([]interface{}); ok {
			for _, cap := range reqCaps {
				if capStr, ok := cap.(string); ok {
					caps = append(caps, capStr)
				}
			}
		}

		// Check for specific capability requirements
		if lang, ok := task.Parameters["language"].(string); ok {
			caps = append(caps, "lang:"+lang)
		}
		if env, ok := task.Parameters["environment"].(string); ok {
			caps = append(caps, "env:"+env)
		}
		if tool, ok := task.Parameters["tool"].(string); ok {
			caps = append(caps, "tool:"+tool)
		}
	}

	return caps
}

// extractRequiredCapabilities extracts required capabilities from the task
func (e *AssignmentEngine) extractRequiredCapabilities(task *models.Task) []string {
	return extractRequiredCapabilities(task)
}

// hasRequiredCapabilities checks if agent has all required capabilities
func (e *AssignmentEngine) hasRequiredCapabilities(agent *models.Agent, agentCaps []string, requiredCaps []string) bool {
	// If no specific capabilities required, any agent can handle
	if len(requiredCaps) == 0 {
		return true
	}

	// Create capability set for fast lookup
	capSet := make(map[string]bool)
	for _, cap := range agentCaps {
		capSet[cap] = true
	}

	// Also include agent's built-in capabilities
	for _, cap := range agent.Capabilities {
		capSet[cap] = true
	}

	// Check if agent has all required capabilities
	for _, req := range requiredCaps {
		if !capSet[req] && !capSet["all"] { // "all" is a wildcard capability
			return false
		}
	}

	return true
}

// isAgentOverloaded checks if agent is overloaded based on cached workload
func (e *AssignmentEngine) isAgentOverloaded(ctx context.Context, agentID string) bool {
	// Check cache first
	cacheKey := "workload_" + agentID
	if cached, ok := e.workloadCache.Load(cacheKey); ok {
		cachedWorkload := cached.(*workloadCacheEntry)
		if time.Since(cachedWorkload.timestamp) < e.cacheDuration {
			return cachedWorkload.workload.LoadScore > 0.8 // 80% load is considered overloaded
		}
	}

	// Get fresh workload data
	workload, err := e.agentService.GetAgentWorkload(ctx, agentID)
	if err != nil {
		e.logger.Warn("Failed to get agent workload, assuming not overloaded", map[string]interface{}{
			"agent_id": agentID,
			"error":    err.Error(),
		})
		return false // Assume not overloaded if we can't check
	}

	// Cache the workload
	e.workloadCache.Store(cacheKey, &workloadCacheEntry{
		workload:  workload,
		timestamp: time.Now(),
	})

	// Check if overloaded (>80% load or >10 active tasks)
	return workload.LoadScore > 0.8 || workload.ActiveTasks > 10
}

// workloadCacheEntry represents a cached workload entry
type workloadCacheEntry struct {
	workload  *models.AgentWorkload
	timestamp time.Time
}

// performanceCacheEntry represents a cached performance entry
type performanceCacheEntry struct {
	performance *models.AgentPerformance
	timestamp   time.Time
}

// isFirstTimeTaskType checks if agent has handled this task type before
func isFirstTimeTaskType(agent *models.Agent, taskType string) bool {
	if agent.Metadata == nil {
		return true
	}

	// Check if agent has task history
	if history, ok := agent.Metadata["task_history"].(map[string]interface{}); ok {
		if _, hasType := history[taskType]; hasType {
			return false
		}
	}

	// Check if agent has completed tasks of this type
	taskCountKey := fmt.Sprintf("%s_completed", taskType)
	if count, ok := agent.Metadata[taskCountKey].(int); ok && count > 0 {
		return false
	}

	return true
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ==================== NEW STRATEGIES FOR MULTI-AGENT ORCHESTRATION (Phase 1.2) ====================

// AffinityStrategy maintains context by keeping related tasks with same agent/team
type AffinityStrategy struct {
	sessionAffinity map[string]string // session -> preferred agent
	taskAffinity    map[string]string // task type -> specialist agent
	mu              sync.RWMutex
	agentService    AgentService
	logger          observability.Logger
}

func NewAffinityStrategy(agentService AgentService, logger observability.Logger) *AffinityStrategy {
	return &AffinityStrategy{
		sessionAffinity: make(map[string]string),
		taskAffinity:    make(map[string]string),
		agentService:    agentService,
		logger:          logger,
	}
}

func (s *AffinityStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Extract session from task parameters
	var sessionID string
	if task.Parameters != nil {
		if sid, ok := task.Parameters["session_id"].(string); ok {
			sessionID = sid
		}
	}

	// Check session affinity first (highest priority)
	if sessionID != "" {
		if agentID, exists := s.sessionAffinity[sessionID]; exists {
			for _, agent := range agents {
				if agent.ID == agentID && agent.Status == string(models.AgentStatusActive) {
					s.logger.Debug("Using session affinity", map[string]interface{}{
						"session_id": sessionID,
						"agent_id":   agentID,
						"task_id":    task.ID,
					})
					return agent, nil
				}
			}
		}
	}

	// Check task type affinity (secondary priority)
	if specialist, exists := s.taskAffinity[task.Type]; exists {
		for _, agent := range agents {
			if agent.ID == specialist && agent.Status == string(models.AgentStatusActive) {
				// Update session affinity for future tasks
				if sessionID != "" {
					s.mu.RUnlock()
					s.mu.Lock()
					s.sessionAffinity[sessionID] = agent.ID
					s.mu.Unlock()
					s.mu.RLock()
				}
				return agent, nil
			}
		}
	}

	// Fallback to capability match
	capStrategy := &CapabilityMatchStrategy{logger: s.logger}
	selected, err := capStrategy.Assign(ctx, task, agents)
	if err == nil && selected != nil {
		// Establish affinity for future tasks
		if sessionID != "" {
			s.mu.RUnlock()
			s.mu.Lock()
			s.sessionAffinity[sessionID] = selected.ID
			s.mu.Unlock()
			s.mu.RLock()
		}
		s.mu.RUnlock()
		s.mu.Lock()
		s.taskAffinity[task.Type] = selected.ID
		s.mu.Unlock()
		s.mu.RLock()
	}

	return selected, err
}

func (s *AffinityStrategy) GetName() string {
	return "affinity"
}

// HierarchicalCascadeStrategy routes through domain coordinators before individual agents
type HierarchicalCascadeStrategy struct {
	domainMap map[string]string // task type -> domain
	logger    observability.Logger
	metrics   observability.MetricsClient
}

func NewHierarchicalCascadeStrategy(logger observability.Logger, metrics observability.MetricsClient) *HierarchicalCascadeStrategy {
	return &HierarchicalCascadeStrategy{
		domainMap: map[string]string{
			"lint":     "code",
			"format":   "code",
			"refactor": "code",
			"test":     "testing",
			"unittest": "testing",
			"e2e":      "testing",
			"deploy":   "deployment",
			"release":  "deployment",
			"scan":     "security",
			"audit":    "security",
			"document": "documentation",
			"monitor":  "monitoring",
			"alert":    "monitoring",
			"build":    "build",
			"compile":  "build",
		},
		logger:  logger,
		metrics: metrics,
	}
}

func (s *HierarchicalCascadeStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Determine domain from task type
	domain := s.categorizeToDomain(task)

	// Filter agents by domain
	domainAgents := s.filterByDomain(agents, domain)

	if len(domainAgents) == 0 {
		s.logger.Warn("No agents found for domain, using all agents", map[string]interface{}{
			"task_type": task.Type,
			"domain":    domain,
		})
		domainAgents = agents
	}

	// Use capability matching within domain
	capStrategy := &CapabilityMatchStrategy{logger: s.logger, metrics: s.metrics}
	selected, err := capStrategy.Assign(ctx, task, domainAgents)

	if err == nil && selected != nil {
		s.metrics.RecordGauge("assignment.hierarchical_cascade", 1.0, map[string]string{
			"domain":    domain,
			"task_type": task.Type,
		})
	}

	return selected, err
}

func (s *HierarchicalCascadeStrategy) categorizeToDomain(task *models.Task) string {
	if domain, exists := s.domainMap[task.Type]; exists {
		return domain
	}
	return "general"
}

func (s *HierarchicalCascadeStrategy) filterByDomain(agents []*models.Agent, domain string) []*models.Agent {
	filtered := make([]*models.Agent, 0)
	for _, agent := range agents {
		// Check if agent has domain in metadata
		if agent.Metadata != nil {
			if agentDomain, ok := agent.Metadata["domain"].(string); ok && agentDomain == domain {
				filtered = append(filtered, agent)
				continue
			}
		}
		// Also check if domain is in Type field (e.g., "code_reviewer", "deployment_agent")
		if agent.Type != "" {
			// Map agent types to domains
			var agentDomain string
			switch {
			case contains([]string{"code_reviewer", "refactoring"}, agent.Type):
				agentDomain = "code"
			case contains([]string{"test_writer", "qa_specialist"}, agent.Type):
				agentDomain = "testing"
			case contains([]string{"deployment_agent", "release_manager"}, agent.Type):
				agentDomain = "deployment"
			case contains([]string{"security_scanner", "vulnerability_checker"}, agent.Type):
				agentDomain = "security"
			case contains([]string{"documentation_writer"}, agent.Type):
				agentDomain = "docs"
			default:
				agentDomain = "general"
			}
			if agentDomain == domain {
				filtered = append(filtered, agent)
			}
		}
	}
	return filtered
}

func (s *HierarchicalCascadeStrategy) GetName() string {
	return "hierarchical_cascade"
}

// PriorityQueueStrategy handles tasks based on priority and SLA
type PriorityQueueStrategy struct {
	agentService AgentService
	logger       observability.Logger
	metrics      observability.MetricsClient
}

func NewPriorityQueueStrategy(agentService AgentService, logger observability.Logger, metrics observability.MetricsClient) *PriorityQueueStrategy {
	return &PriorityQueueStrategy{
		agentService: agentService,
		logger:       logger,
		metrics:      metrics,
	}
}

func (s *PriorityQueueStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Critical tasks get dedicated agents with lowest workload
	if task.Priority == models.TaskPriorityCritical {
		return s.assignDedicatedAgent(ctx, task, agents)
	}

	// High priority gets first available capable agent with low workload
	if task.Priority == models.TaskPriorityHigh {
		for _, agent := range agents {
			workload, err := s.agentService.GetAgentWorkload(ctx, agent.ID)
			if err == nil && workload.LoadScore < 0.5 { // Less than 50% loaded
				s.logger.Info("High priority task assigned to low-load agent", map[string]interface{}{
					"task_id":    task.ID,
					"agent_id":   agent.ID,
					"load_score": workload.LoadScore,
				})
				return agent, nil
			}
		}
	}

	// Normal/Low priority uses least loaded strategy
	leastLoaded := &LeastLoadedStrategy{
		agentService: s.agentService,
		logger:       s.logger,
		metrics:      s.metrics,
	}
	return leastLoaded.Assign(ctx, task, agents)
}

func (s *PriorityQueueStrategy) assignDedicatedAgent(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	// Find agent with absolute lowest load
	var bestAgent *models.Agent
	lowestLoad := 1.0

	for _, agent := range agents {
		workload, err := s.agentService.GetAgentWorkload(ctx, agent.ID)
		if err != nil {
			continue
		}

		if workload.LoadScore < lowestLoad {
			lowestLoad = workload.LoadScore
			bestAgent = agent
		}
	}

	if bestAgent != nil {
		s.logger.Info("Critical task assigned to dedicated agent", map[string]interface{}{
			"task_id":    task.ID,
			"agent_id":   bestAgent.ID,
			"load_score": lowestLoad,
			"priority":   "critical",
		})
		return bestAgent, nil
	}

	return nil, ErrNoEligibleAgents
}

func (s *PriorityQueueStrategy) GetName() string {
	return "priority_queue"
}

// CollaborativeTeamStrategy assigns multiple specialized agents as a coordinated team
type CollaborativeTeamStrategy struct {
	teamTemplates map[string][]string // task type -> required agent types
	logger        observability.Logger
	metrics       observability.MetricsClient
}

func NewCollaborativeTeamStrategy(logger observability.Logger, metrics observability.MetricsClient) *CollaborativeTeamStrategy {
	return &CollaborativeTeamStrategy{
		teamTemplates: map[string][]string{
			"full_review": {"linter", "security_scanner", "test_runner", "documentation"},
			"deployment":  {"builder", "tester", "deployer", "monitor"},
			"migration":   {"analyzer", "migrator", "tester", "validator"},
		},
		logger:  logger,
		metrics: metrics,
	}
}

func (s *CollaborativeTeamStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	// For now, return the primary agent (team lead)
	// Full team assignment will be implemented in Phase 3
	template, exists := s.teamTemplates[task.Type]
	if !exists || len(template) == 0 {
		// Fall back to capability match for non-team tasks
		capStrategy := &CapabilityMatchStrategy{logger: s.logger, metrics: s.metrics}
		return capStrategy.Assign(ctx, task, agents)
	}

	// Find team lead (first role in template)
	leadRole := template[0]
	for _, agent := range agents {
		if s.hasRole(agent, leadRole) {
			s.logger.Info("Team lead assigned for collaborative task", map[string]interface{}{
				"task_id":      task.ID,
				"agent_id":     agent.ID,
				"role":         leadRole,
				"team_size":    len(template),
				"team_members": template,
			})
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no suitable team lead found for role: %s", leadRole)
}

func (s *CollaborativeTeamStrategy) hasRole(agent *models.Agent, role string) bool {
	// Check agent type
	if agent.Type == role {
		return true
	}

	// Check capabilities
	for _, cap := range agent.Capabilities {
		if cap == role {
			return true
		}
	}

	// Check metadata
	if agent.Metadata != nil {
		if roles, ok := agent.Metadata["roles"].([]interface{}); ok {
			for _, r := range roles {
				if rStr, ok := r.(string); ok && rStr == role {
					return true
				}
			}
		}
	}

	return false
}

func (s *CollaborativeTeamStrategy) GetName() string {
	return "collaborative_team"
}

// PredictiveLoadStrategy predicts future load and pre-assigns agents
type PredictiveLoadStrategy struct {
	agentService AgentService
	logger       observability.Logger
	metrics      observability.MetricsClient
}

func NewPredictiveLoadStrategy(agentService AgentService, logger observability.Logger, metrics observability.MetricsClient) *PredictiveLoadStrategy {
	return &PredictiveLoadStrategy{
		agentService: agentService,
		logger:       logger,
		metrics:      metrics,
	}
}

func (s *PredictiveLoadStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Predict task duration based on type
	estimatedDuration := s.estimateDuration(task)

	// Calculate future load for each agent
	type agentFutureLoad struct {
		agent       *models.Agent
		currentLoad float64
		futureLoad  float64
	}

	agentLoads := make([]agentFutureLoad, 0, len(agents))

	for _, agent := range agents {
		workload, err := s.agentService.GetAgentWorkload(ctx, agent.ID)
		if err != nil {
			continue
		}

		currentLoad := workload.LoadScore
		// Simple prediction: current load + estimated impact of this task
		taskImpact := s.estimateTaskImpact(task, estimatedDuration)
		futureLoad := currentLoad + taskImpact

		agentLoads = append(agentLoads, agentFutureLoad{
			agent:       agent,
			currentLoad: currentLoad,
			futureLoad:  futureLoad,
		})
	}

	if len(agentLoads) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Sort by future load (ascending)
	sort.Slice(agentLoads, func(i, j int) bool {
		return agentLoads[i].futureLoad < agentLoads[j].futureLoad
	})

	selected := agentLoads[0]

	s.logger.Info("Predictive load assignment", map[string]interface{}{
		"task_id":            task.ID,
		"agent_id":           selected.agent.ID,
		"current_load":       selected.currentLoad,
		"predicted_load":     selected.futureLoad,
		"estimated_duration": estimatedDuration,
	})

	s.metrics.RecordGauge("assignment.predicted_load", selected.futureLoad, map[string]string{
		"agent_id":  selected.agent.ID,
		"task_type": task.Type,
	})

	return selected.agent, nil
}

func (s *PredictiveLoadStrategy) estimateDuration(task *models.Task) time.Duration {
	// Check task parameters for explicit duration
	if task.Parameters != nil {
		if duration, ok := task.Parameters["estimated_duration"].(float64); ok {
			return time.Duration(duration) * time.Second
		}
	}

	// Default estimates by task type
	durationMap := map[string]time.Duration{
		"test":     5 * time.Minute,
		"build":    10 * time.Minute,
		"deploy":   15 * time.Minute,
		"scan":     3 * time.Minute,
		"lint":     1 * time.Minute,
		"format":   30 * time.Second,
		"document": 5 * time.Minute,
	}

	if duration, exists := durationMap[task.Type]; exists {
		return duration
	}

	return 5 * time.Minute // Default 5 minutes
}

func (s *PredictiveLoadStrategy) estimateTaskImpact(task *models.Task, duration time.Duration) float64 {
	// Calculate relative impact (0.0 to 1.0)
	// Longer tasks have more impact, priority affects impact
	baseImpact := float64(duration) / float64(30*time.Minute) // Normalize to 30-minute baseline

	// Priority multiplier
	priorityMultiplier := 1.0
	switch task.Priority {
	case models.TaskPriorityCritical:
		priorityMultiplier = 1.5
	case models.TaskPriorityHigh:
		priorityMultiplier = 1.2
	case models.TaskPriorityLow:
		priorityMultiplier = 0.8
	}

	impact := baseImpact * priorityMultiplier

	// Cap at 1.0
	if impact > 1.0 {
		impact = 1.0
	}

	return impact
}

func (s *PredictiveLoadStrategy) GetName() string {
	return "predictive"
}
