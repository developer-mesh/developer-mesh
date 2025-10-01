package tools

// ToolCategory represents the primary category for a tool
type ToolCategory string

// Standard tool categories for AI agent discovery
const (
	CategoryRepository    ToolCategory = "repository"      // Repository management tools
	CategoryIssues        ToolCategory = "issues"          // Issue and bug tracking tools
	CategoryPullRequests  ToolCategory = "pull_requests"   // Pull request and code review tools
	CategoryCICD          ToolCategory = "ci_cd"           // CI/CD and deployment tools
	CategoryWorkflow      ToolCategory = "workflow"        // Workflow orchestration tools
	CategoryTask          ToolCategory = "task_management" // Task management tools
	CategoryAgent         ToolCategory = "agent"           // Agent coordination tools
	CategoryContext       ToolCategory = "context"         // Context and session management
	CategoryMonitoring    ToolCategory = "monitoring"      // Monitoring and observability tools
	CategorySecurity      ToolCategory = "security"        // Security scanning and compliance
	CategoryCollaboration ToolCategory = "collaboration"   // Team collaboration tools
	CategoryDocumentation ToolCategory = "documentation"   // Documentation tools
	CategoryAnalytics     ToolCategory = "analytics"       // Analytics and reporting tools
	CategoryTemplate      ToolCategory = "template"        // Template and boilerplate tools
	CategoryConfiguration ToolCategory = "configuration"   // Configuration management
	CategoryGeneral       ToolCategory = "general"         // General purpose tools
)

// ToolCapability represents a capability tag for a tool
type ToolCapability string

// Standard capability tags for tools
const (
	// Access level capabilities
	CapabilityRead    ToolCapability = "read"    // Can read/retrieve data
	CapabilityWrite   ToolCapability = "write"   // Can create/write data
	CapabilityUpdate  ToolCapability = "update"  // Can update/modify data
	CapabilityDelete  ToolCapability = "delete"  // Can delete data
	CapabilityExecute ToolCapability = "execute" // Can execute actions

	// Operation type capabilities
	CapabilityList     ToolCapability = "list"     // Can list/enumerate items
	CapabilitySearch   ToolCapability = "search"   // Can search/query data
	CapabilityFilter   ToolCapability = "filter"   // Can filter results
	CapabilitySort     ToolCapability = "sort"     // Can sort results
	CapabilityPaginate ToolCapability = "paginate" // Supports pagination

	// Data handling capabilities
	CapabilityAsync    ToolCapability = "async"    // Supports async operations
	CapabilityBatch    ToolCapability = "batch"    // Supports batch operations
	CapabilityStream   ToolCapability = "stream"   // Supports streaming
	CapabilityCache    ToolCapability = "cache"    // Supports caching
	CapabilityRealtime ToolCapability = "realtime" // Real-time data

	// Special capabilities
	CapabilityIdempotent ToolCapability = "idempotent" // Idempotent operations
	CapabilityDryRun     ToolCapability = "dry_run"    // Supports dry run mode
	CapabilityValidation ToolCapability = "validation" // Input validation
	CapabilityRollback   ToolCapability = "rollback"   // Can rollback changes
	CapabilityPreview    ToolCapability = "preview"    // Can preview changes
)

// CategoryInfo provides detailed information about a category
type CategoryInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`   // Display priority (lower = higher priority)
	Icon        string   `json:"icon"`       // Optional emoji or icon
	RelatedTo   []string `json:"related_to"` // Related categories
}

// CategoryDescriptions provides detailed descriptions for each category
var CategoryDescriptions = map[ToolCategory]CategoryInfo{
	CategoryRepository: {
		Name:        "Repository Management",
		Description: "Tools for managing source code repositories, branches, and version control",
		Priority:    1,
		Icon:        "📦",
		RelatedTo:   []string{string(CategoryPullRequests), string(CategoryIssues)},
	},
	CategoryIssues: {
		Name:        "Issue Tracking",
		Description: "Tools for creating, managing, and tracking issues and bugs",
		Priority:    2,
		Icon:        "🐛",
		RelatedTo:   []string{string(CategoryPullRequests), string(CategoryTask)},
	},
	CategoryPullRequests: {
		Name:        "Pull Requests",
		Description: "Tools for code reviews, pull requests, and merge management",
		Priority:    3,
		Icon:        "🔄",
		RelatedTo:   []string{string(CategoryRepository), string(CategoryCICD)},
	},
	CategoryCICD: {
		Name:        "CI/CD & Deployment",
		Description: "Tools for continuous integration, deployment, and pipeline management",
		Priority:    4,
		Icon:        "🚀",
		RelatedTo:   []string{string(CategoryWorkflow), string(CategoryMonitoring)},
	},
	CategoryWorkflow: {
		Name:        "Workflow Orchestration",
		Description: "Tools for defining and executing complex workflows and automations",
		Priority:    5,
		Icon:        "⚡",
		RelatedTo:   []string{string(CategoryTask), string(CategoryCICD)},
	},
	CategoryTask: {
		Name:        "Task Management",
		Description: "Tools for creating, assigning, and tracking tasks across teams",
		Priority:    6,
		Icon:        "✅",
		RelatedTo:   []string{string(CategoryWorkflow), string(CategoryAgent)},
	},
	CategoryAgent: {
		Name:        "Agent Coordination",
		Description: "Tools for managing and coordinating AI agents and their capabilities",
		Priority:    7,
		Icon:        "🤖",
		RelatedTo:   []string{string(CategoryTask), string(CategoryContext)},
	},
	CategoryContext: {
		Name:        "Context Management",
		Description: "Tools for managing session context and state across operations",
		Priority:    8,
		Icon:        "🧠",
		RelatedTo:   []string{string(CategoryAgent), string(CategoryWorkflow)},
	},
	CategoryMonitoring: {
		Name:        "Monitoring & Observability",
		Description: "Tools for monitoring, logging, metrics, and system observability",
		Priority:    9,
		Icon:        "📊",
		RelatedTo:   []string{string(CategoryCICD), string(CategorySecurity)},
	},
	CategorySecurity: {
		Name:        "Security & Compliance",
		Description: "Tools for security scanning, vulnerability detection, and compliance",
		Priority:    10,
		Icon:        "🔐",
		RelatedTo:   []string{string(CategoryMonitoring), string(CategoryRepository)},
	},
	CategoryCollaboration: {
		Name:        "Team Collaboration",
		Description: "Tools for team communication, notifications, and collaboration",
		Priority:    11,
		Icon:        "👥",
		RelatedTo:   []string{string(CategoryTask), string(CategoryDocumentation)},
	},
	CategoryDocumentation: {
		Name:        "Documentation",
		Description: "Tools for generating, managing, and publishing documentation",
		Priority:    12,
		Icon:        "📚",
		RelatedTo:   []string{string(CategoryRepository), string(CategoryCollaboration)},
	},
	CategoryAnalytics: {
		Name:        "Analytics & Reporting",
		Description: "Tools for data analysis, metrics, and report generation",
		Priority:    13,
		Icon:        "📈",
		RelatedTo:   []string{string(CategoryMonitoring), string(CategoryWorkflow)},
	},
	CategoryTemplate: {
		Name:        "Templates & Boilerplate",
		Description: "Tools for managing templates, boilerplate code, and scaffolding",
		Priority:    14,
		Icon:        "🎨",
		RelatedTo:   []string{string(CategoryWorkflow), string(CategoryDocumentation)},
	},
	CategoryConfiguration: {
		Name:        "Configuration Management",
		Description: "Tools for managing application and infrastructure configuration",
		Priority:    15,
		Icon:        "⚙️",
		RelatedTo:   []string{string(CategoryCICD), string(CategorySecurity)},
	},
	CategoryGeneral: {
		Name:        "General Purpose",
		Description: "General utility tools that don't fit a specific category",
		Priority:    99,
		Icon:        "🔧",
		RelatedTo:   []string{},
	},
}

// GetCategoriesForAgent returns recommended categories for a specific agent type
func GetCategoriesForAgent(agentType string) []ToolCategory {
	switch agentType {
	case "code_reviewer":
		return []ToolCategory{CategoryPullRequests, CategoryRepository, CategoryIssues, CategorySecurity}
	case "deployer":
		return []ToolCategory{CategoryCICD, CategoryWorkflow, CategoryMonitoring, CategoryConfiguration}
	case "tester":
		return []ToolCategory{CategoryCICD, CategoryMonitoring, CategoryAnalytics, CategorySecurity}
	case "architect":
		return []ToolCategory{CategoryRepository, CategoryDocumentation, CategoryTemplate, CategoryConfiguration}
	case "project_manager":
		return []ToolCategory{CategoryTask, CategoryIssues, CategoryCollaboration, CategoryAnalytics}
	default:
		return []ToolCategory{CategoryGeneral}
	}
}

// GetCapabilitiesForOperation returns standard capabilities for common operations
func GetCapabilitiesForOperation(operation string) []ToolCapability {
	switch operation {
	case "get", "fetch", "retrieve":
		return []ToolCapability{CapabilityRead}
	case "list", "enumerate":
		return []ToolCapability{CapabilityRead, CapabilityList, CapabilityFilter, CapabilitySort, CapabilityPaginate}
	case "create", "add", "new":
		return []ToolCapability{CapabilityWrite, CapabilityValidation}
	case "update", "modify", "edit", "patch":
		return []ToolCapability{CapabilityUpdate, CapabilityValidation, CapabilityPreview}
	case "delete", "remove", "destroy":
		return []ToolCapability{CapabilityDelete, CapabilityDryRun}
	case "execute", "run", "trigger":
		return []ToolCapability{CapabilityExecute, CapabilityAsync}
	case "search", "query", "find":
		return []ToolCapability{CapabilityRead, CapabilitySearch, CapabilityFilter}
	case "batch", "bulk":
		return []ToolCapability{CapabilityBatch, CapabilityAsync}
	default:
		return []ToolCapability{CapabilityRead}
	}
}
