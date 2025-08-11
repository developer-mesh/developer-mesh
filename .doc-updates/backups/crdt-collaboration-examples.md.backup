# CRDT Collaboration Examples

This guide demonstrates how to implement real-time collaboration using Conflict-free Replicated Data Types (CRDTs) in Developer Mesh.

## Overview

Developer Mesh's CRDT implementation enables:
- **🔄 Real-time collaboration** between multiple AI agents
- **🌍 Distributed consensus** without central coordination
- **⚡ Eventual consistency** with automatic conflict resolution
- **📊 Scalable collaboration** for thousands of agents

## CRDT Types Supported

### 1. G-Counter (Grow-only Counter)
Perfect for metrics aggregation and vote counting.

```typescript
import { GCounter, CRDTClient } from '@developer-mesh/crdt';

class MetricsAggregator {
  private counter: GCounter;
  private client: CRDTClient;
  
  constructor(agentId: string, wsUrl: string) {
    this.client = new CRDTClient({
      url: wsUrl,
      agentId: agentId,
      syncInterval: 1000 // Sync every second
    });
    
    // Initialize G-Counter for metrics
    this.counter = new GCounter(agentId);
    
    // Register with sync client
    this.client.register('metrics:requests', this.counter);
  }
  
  // Increment counter locally
  async trackRequest(endpoint: string) {
    // Increment local counter
    this.counter.increment(1);
    
    // Optionally track by endpoint
    const endpointCounter = this.client.getOrCreate(
      `metrics:${endpoint}`,
      () => new GCounter(this.agentId)
    );
    endpointCounter.increment(1);
    
    // Sync happens automatically based on interval
  }
  
  // Get total across all agents
  getTotalRequests(): number {
    return this.counter.value();
  }
  
  // Monitor real-time updates
  watchMetrics(callback: (total: number) => void) {
    this.counter.on('update', () => {
      callback(this.counter.value());
    });
  }
}

// Usage across multiple agents
async function multiAgentMetrics() {
  // Agent 1
  const agent1 = new MetricsAggregator('agent-1', 'wss://mcp.example.com/ws');
  await agent1.trackRequest('/api/v1/contexts');
  
  // Agent 2
  const agent2 = new MetricsAggregator('agent-2', 'wss://mcp.example.com/ws');
  await agent2.trackRequest('/api/v1/contexts');
  
  // Both agents will eventually see total = 2
  setTimeout(() => {
    console.log('Agent 1 sees:', agent1.getTotalRequests()); // 2
    console.log('Agent 2 sees:', agent2.getTotalRequests()); // 2
  }, 2000);
}
```

### 2. LWW-Register (Last-Write-Wins Register)
For configuration management and state synchronization.

```go
package main

import (
    "github.com/developer-mesh/developer-mesh/pkg/crdt"
    "time"
)

type ConfigManager struct {
    register *crdt.LWWRegister
    client   *crdt.Client
}

func NewConfigManager(agentID string) *ConfigManager {
    client := crdt.NewClient(agentID, "wss://mcp.example.com/ws")
    register := crdt.NewLWWRegister(agentID)
    
    // Register CRDT with client
    client.Register("config:global", register)
    
    return &ConfigManager{
        register: register,
        client:   client,
    }
}

// Update configuration value
func (cm *ConfigManager) UpdateConfig(key string, value interface{}) error {
    // Create config entry
    config := map[string]interface{}{
        key:         value,
        "updatedBy": cm.client.AgentID,
        "timestamp": time.Now().Unix(),
    }
    
    // Set value with timestamp
    cm.register.Set(config, time.Now())
    
    // Sync with other agents
    return cm.client.Sync()
}

// Get current configuration
func (cm *ConfigManager) GetConfig() interface{} {
    return cm.register.Value()
}

// Example: Distributed feature flags
func DistributedFeatureFlags() {
    // Multiple agents managing feature flags
    agent1 := NewConfigManager("agent-1")
    agent2 := NewConfigManager("agent-2")
    agent3 := NewConfigManager("agent-3")
    
    // Concurrent updates
    go agent1.UpdateConfig("feature:ai-review", true)
    go agent2.UpdateConfig("feature:auto-merge", false)
    go agent3.UpdateConfig("feature:ai-review", false) // Conflict!
    
    // LWW ensures consistent resolution
    // The update with the latest timestamp wins
    time.Sleep(2 * time.Second)
    
    // All agents see the same final state
    fmt.Println("Agent 1:", agent1.GetConfig())
    fmt.Println("Agent 2:", agent2.GetConfig())
    fmt.Println("Agent 3:", agent3.GetConfig())
}
```

### 3. OR-Set (Observed-Remove Set)
For managing collections with concurrent add/remove operations.

```python
from typing import Set, Any
import asyncio
from devops_mcp.crdt import ORSet, CRDTClient

class TaskQueue:
    """Distributed task queue using OR-Set CRDT"""
    
    def __init__(self, agent_id: str, ws_url: str):
        self.agent_id = agent_id
        self.client = CRDTClient(agent_id, ws_url)
        
        # OR-Set for active tasks
        self.active_tasks = ORSet(agent_id)
        self.completed_tasks = ORSet(agent_id)
        
        # Register CRDTs
        self.client.register("tasks:active", self.active_tasks)
        self.client.register("tasks:completed", self.completed_tasks)
        
        # Start sync loop
        asyncio.create_task(self._sync_loop())
    
    async def add_task(self, task_id: str, task_data: dict):
        """Add a task to the distributed queue"""
        task = {
            "id": task_id,
            "data": task_data,
            "assigned_to": None,
            "created_by": self.agent_id,
            "created_at": time.time()
        }
        
        # Add to active tasks
        self.active_tasks.add(task)
        
        # Notify other agents
        await self.client.broadcast({
            "type": "task_added",
            "task": task
        })
    
    async def claim_task(self, task_id: str) -> bool:
        """Claim a task for processing"""
        # Find task in active set
        task = None
        for t in self.active_tasks.elements():
            if t["id"] == task_id:
                task = t
                break
        
        if not task or task["assigned_to"]:
            return False  # Already claimed
        
        # Update task assignment
        updated_task = {**task, "assigned_to": self.agent_id}
        
        # Remove old, add updated (atomic in OR-Set)
        self.active_tasks.remove(task)
        self.active_tasks.add(updated_task)
        
        await self.client.sync()
        return True
    
    async def complete_task(self, task_id: str, result: Any):
        """Mark task as completed"""
        # Find and remove from active
        task = None
        for t in self.active_tasks.elements():
            if t["id"] == task_id:
                task = t
                break
        
        if task:
            self.active_tasks.remove(task)
            
            # Add to completed with result
            completed = {
                **task,
                "completed_by": self.agent_id,
                "completed_at": time.time(),
                "result": result
            }
            self.completed_tasks.add(completed)
            
            await self.client.sync()
    
    def get_available_tasks(self) -> Set[dict]:
        """Get unclaimed tasks"""
        return {
            task for task in self.active_tasks.elements()
            if not task.get("assigned_to")
        }
    
    async def _sync_loop(self):
        """Periodic sync with other agents"""
        while True:
            await asyncio.sleep(1)
            await self.client.sync()

# Example: Multi-agent task processing
async def collaborative_task_processing():
    # Three agents working on shared task queue
    agents = [
        TaskQueue(f"agent-{i}", "wss://mcp.example.com/ws")
        for i in range(3)
    ]
    
    # Agent 0 adds tasks
    for i in range(10):
        await agents[0].add_task(
            f"task-{i}",
            {"type": "analyze", "target": f"repo-{i}"}
        )
    
    # All agents process tasks concurrently
    async def process_agent(agent: TaskQueue, agent_id: int):
        while True:
            tasks = agent.get_available_tasks()
            if not tasks:
                await asyncio.sleep(0.5)
                continue
            
            # Try to claim a task
            for task in tasks:
                if await agent.claim_task(task["id"]):
                    print(f"Agent {agent_id} claimed {task['id']}")
                    
                    # Simulate processing
                    await asyncio.sleep(random.uniform(1, 3))
                    
                    # Complete task
                    result = f"Processed by agent-{agent_id}"
                    await agent.complete_task(task["id"], result)
                    print(f"Agent {agent_id} completed {task['id']}")
                    break
    
    # Run all agents
    await asyncio.gather(*[
        process_agent(agent, i)
        for i, agent in enumerate(agents)
    ])
```

### 4. PN-Counter (Positive-Negative Counter)
For bidirectional counters with increment/decrement.

```typescript
class ResourceAllocation {
  private counter: PNCounter;
  private maxResources: number;
  
  constructor(
    agentId: string,
    resourceType: string,
    maxResources: number
  ) {
    this.counter = new PNCounter(agentId);
    this.maxResources = maxResources;
    
    // Register with CRDT system
    CRDTRegistry.register(`resources:${resourceType}`, this.counter);
  }
  
  async allocate(amount: number): Promise<boolean> {
    const current = this.counter.value();
    
    if (current + amount > this.maxResources) {
      return false; // Would exceed limit
    }
    
    // Increment counter
    this.counter.increment(amount);
    
    // Sync with other agents
    await CRDTRegistry.sync();
    
    return true;
  }
  
  async release(amount: number): Promise<void> {
    // Decrement counter
    this.counter.decrement(amount);
    
    // Sync
    await CRDTRegistry.sync();
  }
  
  getAvailable(): number {
    return this.maxResources - this.counter.value();
  }
}

// Example: Distributed resource management
async function distributedResources() {
  // Multiple agents managing shared GPU resources
  const agent1 = new ResourceAllocation('agent-1', 'gpu', 100);
  const agent2 = new ResourceAllocation('agent-2', 'gpu', 100);
  
  // Concurrent allocations
  const results = await Promise.all([
    agent1.allocate(30),  // true
    agent2.allocate(40),  // true
    agent1.allocate(50),  // false (would exceed 100)
  ]);
  
  console.log('Allocations:', results); // [true, true, false]
  console.log('Available:', agent1.getAvailable()); // 30
}
```

### 5. RGA (Replicated Growable Array)
For collaborative document editing and ordered lists.

```go
type CollaborativeDocument struct {
    rga    *crdt.RGA
    client *crdt.Client
}

func NewCollaborativeDocument(agentID string) *CollaborativeDocument {
    client := crdt.NewClient(agentID, "wss://mcp.example.com/ws")
    rga := crdt.NewRGA(agentID)
    
    client.Register("document:main", rga)
    
    return &CollaborativeDocument{
        rga:    rga,
        client: client,
    }
}

// Insert text at position
func (cd *CollaborativeDocument) Insert(position int, text string) error {
    // Each character is an element
    for i, char := range text {
        cd.rga.InsertAt(position+i, string(char))
    }
    
    return cd.client.Sync()
}

// Delete range
func (cd *CollaborativeDocument) Delete(start, end int) error {
    for i := end - 1; i >= start; i-- {
        cd.rga.RemoveAt(i)
    }
    
    return cd.client.Sync()
}

// Get current document
func (cd *CollaborativeDocument) GetText() string {
    elements := cd.rga.Elements()
    result := make([]byte, len(elements))
    
    for i, elem := range elements {
        result[i] = elem.(string)[0]
    }
    
    return string(result)
}

// Example: Collaborative code editing
func CollaborativeEditing() {
    // Three agents editing the same document
    agent1 := NewCollaborativeDocument("agent-1")
    agent2 := NewCollaborativeDocument("agent-2")
    agent3 := NewCollaborativeDocument("agent-3")
    
    // Concurrent edits
    go agent1.Insert(0, "Hello ")
    go agent2.Insert(6, "World")
    go agent3.Insert(5, ", ")
    
    time.Sleep(2 * time.Second)
    
    // All agents see: "Hello, World"
    fmt.Println("Agent 1:", agent1.GetText())
    fmt.Println("Agent 2:", agent2.GetText())
    fmt.Println("Agent 3:", agent3.GetText())
}
```

## Advanced Collaboration Patterns

### 1. Consensus Building
```python
class ConsensusBuilder:
    """Multi-agent consensus using CRDTs"""
    
    def __init__(self, agent_id: str, min_votes: int):
        self.agent_id = agent_id
        self.min_votes = min_votes
        
        # CRDTs for voting
        self.proposals = ORSet(agent_id)
        self.votes = {}  # proposal_id -> GCounter
        
        CRDTRegistry.register("consensus:proposals", self.proposals)
    
    async def propose(self, proposal_id: str, data: dict):
        """Submit a proposal for consensus"""
        proposal = {
            "id": proposal_id,
            "data": data,
            "proposed_by": self.agent_id,
            "timestamp": time.time()
        }
        
        # Add proposal
        self.proposals.add(proposal)
        
        # Initialize vote counter
        vote_counter = GCounter(self.agent_id)
        self.votes[proposal_id] = vote_counter
        CRDTRegistry.register(f"votes:{proposal_id}", vote_counter)
        
        # Cast initial vote
        await self.vote(proposal_id)
    
    async def vote(self, proposal_id: str):
        """Vote for a proposal"""
        if proposal_id in self.votes:
            self.votes[proposal_id].increment(1)
            await CRDTRegistry.sync()
    
    def check_consensus(self, proposal_id: str) -> bool:
        """Check if consensus reached"""
        if proposal_id not in self.votes:
            return False
        
        votes = self.votes[proposal_id].value()
        return votes >= self.min_votes
    
    async def execute_with_consensus(
        self,
        proposal_id: str,
        action: Callable
    ):
        """Execute action only after consensus"""
        # Wait for consensus
        while not self.check_consensus(proposal_id):
            await asyncio.sleep(0.5)
        
        # Execute once
        if not hasattr(self, '_executed'):
            self._executed = set()
        
        if proposal_id not in self._executed:
            self._executed.add(proposal_id)
            return await action()
```

### 2. Distributed Locks with CRDTs
```typescript
class DistributedLock {
  private ownership: LWWRegister;
  private heartbeat: GCounter;
  private lockId: string;
  
  constructor(resourceId: string, agentId: string) {
    this.lockId = `lock:${resourceId}`;
    this.ownership = new LWWRegister(agentId);
    this.heartbeat = new GCounter(agentId);
    
    CRDTRegistry.register(`${this.lockId}:owner`, this.ownership);
    CRDTRegistry.register(`${this.lockId}:heartbeat`, this.heartbeat);
  }
  
  async acquire(timeout: number = 5000): Promise<boolean> {
    const startTime = Date.now();
    
    while (Date.now() - startTime < timeout) {
      const currentOwner = this.ownership.value();
      
      // Check if lock is free or expired
      if (!currentOwner || this.isExpired(currentOwner)) {
        // Try to acquire
        this.ownership.set({
          agent: this.agentId,
          timestamp: Date.now(),
          ttl: 10000 // 10 second TTL
        });
        
        await CRDTRegistry.sync();
        
        // Verify we got it
        const newOwner = this.ownership.value();
        if (newOwner.agent === this.agentId) {
          this.startHeartbeat();
          return true;
        }
      }
      
      await new Promise(r => setTimeout(r, 100));
    }
    
    return false;
  }
  
  async release(): Promise<void> {
    this.stopHeartbeat();
    this.ownership.set(null);
    await CRDTRegistry.sync();
  }
  
  private isExpired(owner: any): boolean {
    if (!owner) return true;
    return Date.now() - owner.timestamp > owner.ttl;
  }
  
  private startHeartbeat(): void {
    this.heartbeatInterval = setInterval(() => {
      this.heartbeat.increment(1);
      CRDTRegistry.sync();
    }, 1000);
  }
  
  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
    }
  }
}
```

### 3. Collaborative State Machine
```go
type CollaborativeStateMachine struct {
    states      *crdt.ORSet
    transitions *crdt.ORSet
    currentState *crdt.LWWRegister
}

func (csm *CollaborativeStateMachine) Transition(
    from, to string,
    condition func() bool,
) error {
    // Check current state
    current := csm.currentState.Value().(string)
    if current != from {
        return fmt.Errorf("invalid state transition")
    }
    
    // Verify condition
    if !condition() {
        return fmt.Errorf("transition condition not met")
    }
    
    // Record transition
    transition := map[string]interface{}{
        "from":      from,
        "to":        to,
        "timestamp": time.Now().Unix(),
        "agent":     csm.agentID,
    }
    csm.transitions.Add(transition)
    
    // Update current state
    csm.currentState.Set(to, time.Now())
    
    return csm.client.Sync()
}
```

## Performance Optimization

### 1. Delta Synchronization
```python
class DeltaCRDT:
    """Efficient CRDT sync using deltas"""
    
    def __init__(self):
        self.version = 0
        self.deltas = []
        self.state = {}
    
    def update(self, operation):
        # Record delta
        delta = {
            "version": self.version,
            "op": operation,
            "timestamp": time.time()
        }
        self.deltas.append(delta)
        self.version += 1
        
        # Apply to local state
        self._apply_operation(operation)
    
    def get_deltas_since(self, version: int):
        """Get only changes since version"""
        return [d for d in self.deltas if d["version"] > version]
    
    def merge_deltas(self, remote_deltas):
        """Efficiently merge remote changes"""
        for delta in remote_deltas:
            if delta["version"] > self.version:
                self._apply_operation(delta["op"])
                self.deltas.append(delta)
        
        self.version = max(
            self.version,
            max(d["version"] for d in remote_deltas)
        )
```

### 2. Hierarchical CRDTs
```typescript
class HierarchicalCRDT {
  private root: ORSet;
  private children: Map<string, CRDT>;
  
  constructor(agentId: string) {
    this.root = new ORSet(agentId);
    this.children = new Map();
  }
  
  createNamespace(path: string, crdtType: CRDTType): CRDT {
    const crdt = CRDTFactory.create(crdtType, this.agentId);
    this.children.set(path, crdt);
    
    // Register hierarchically
    CRDTRegistry.register(path, crdt);
    
    // Update root
    this.root.add({ path, type: crdtType, created: Date.now() });
    
    return crdt;
  }
  
  // Efficient partial sync
  async syncNamespace(path: string): Promise<void> {
    const crdt = this.children.get(path);
    if (crdt) {
      await CRDTRegistry.syncSingle(path, crdt);
    }
  }
}
```

## Monitoring & Debugging

```typescript
// CRDT metrics
const crdtMetrics = {
  syncLatency: new Histogram({
    name: 'crdt_sync_latency_ms',
    help: 'CRDT synchronization latency',
    labelNames: ['crdt_type', 'operation']
  }),
  
  conflictResolutions: new Counter({
    name: 'crdt_conflicts_resolved_total',
    help: 'Number of CRDT conflicts resolved',
    labelNames: ['crdt_type', 'resolution_type']
  }),
  
  stateSize: new Gauge({
    name: 'crdt_state_size_bytes',
    help: 'Size of CRDT state in bytes',
    labelNames: ['crdt_type', 'namespace']
  })
};

// Debug viewer
class CRDTDebugger {
  static visualize(crdt: CRDT): string {
    return JSON.stringify({
      type: crdt.constructor.name,
      state: crdt.getState(),
      version: crdt.getVersion(),
      peers: crdt.getPeers()
    }, null, 2);
  }
  
  static compareStates(crdt1: CRDT, crdt2: CRDT): Diff {
    return diff(crdt1.getState(), crdt2.getState());
  }
}
```

## Best Practices

1. **Choose the Right CRDT**: Match CRDT type to use case
2. **Minimize State Size**: Use deltas and garbage collection
3. **Handle Network Partitions**: Design for eventual consistency
4. **Monitor Convergence**: Track sync metrics and conflicts
5. **Test Concurrent Operations**: Simulate real-world scenarios

## Next Steps

1. **Binary Protocol Integration**: See [binary-websocket-protocol.md](binary-websocket-protocol.md)
2. **Multi-Agent Patterns**: Check [multi-agent-collaboration.md](../guides/multi-agent-collaboration.md)
3. **Production Deployment**: Read [production-deployment.md](../deployment/production-deployment.md)

---

*For more advanced CRDT patterns and support, visit our [GitHub repository](https://github.com/developer-mesh/developer-mesh)*