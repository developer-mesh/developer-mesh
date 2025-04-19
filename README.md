# MCP Server for DevOps

MCP (Model Context Protocol) Server provides AI agents with a unified API for DevOps tool integrations. It serves as a dedicated Model Context Protocol server for:

**DevOps Integration**: A unified API for AI agents to interact with GitHub through the Model Context Protocol.

## Features

### DevOps Integration
- **Unified API**: Interact with GitHub through a consistent API
- **Tool Operations**: Execute GitHub operations through a standardized interface
- **Event Handling**: Process webhooks from GitHub
- **Tool Discovery**: Dynamically discover available tools and their capabilities

### Context Management
- **Conversation History**: Maintain and manage conversation histories for AI agents
- **Multi-tiered Storage**: Store contexts in Redis, PostgreSQL, and S3 for optimal performance and durability
- **Context Window Management**: Handle token counting, truncation, and optimization
- **Vector Search**: Find semantically similar context items using PostgreSQL's pgvector extension
- **Session Management**: Track conversations across multiple interactions

### Platform Capabilities
- **Extensible Design**: Easily add new tool integrations
- **Resilient Processing**: Built-in retry mechanisms, circuit breakers, and error handling
- **Performance Optimized**: Connection pooling, caching, and concurrency management
- **Comprehensive Authentication**: Secure API access and webhook verification
- **AWS Integration**: Seamless integration with AWS services using IAM Roles for Service Accounts (IRSA)

## How It Works with AI Agents

The MCP Server provides a standardized interface for AI agents to interact with DevOps tools following the Model Context Protocol standard. The typical workflow is:

1. **Agent Initialization**: The AI agent connects to the MCP Server
2. **Context Creation**: The agent creates or retrieves a conversation context
3. **Tool Discovery**: The agent discovers available DevOps tools and their capabilities
4. **Tool Interaction**: The agent uses the MCP Server to interact with GitHub
5. **Context Updates**: Tool operations and results are recorded in the context
6. **Event Handling**: The MCP Server processes webhooks from GitHub and notifies the agent

This architecture allows agents to interact with GitHub through a standardized protocol, eliminating the need for custom integrations.

For a comprehensive list of everything an AI Agent can do with the MCP Server, see [AI Agent Capabilities](docs/agent-capabilities.md).

## Supported Models via the Model Context Protocol

The MCP Server follows the Model Context Protocol standard, making it compatible with any AI agent that implements this protocol, including those using:

1. **Anthropic**: Claude models
2. **OpenAI**: GPT models
3. **Custom agents**: Any agent implementing the MCP client specification

## Getting Started

### Prerequisites

- Go 1.24 or higher
- Docker and Docker Compose (for local development)
- AWS credentials for Amazon Bedrock (if using Bedrock)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

2. Copy the configuration template:

```bash
cp configs/config.yaml.template configs/config.yaml
```

3. Edit the configuration file with your credentials and settings.

4. Create an `.env` file with your environment variables:

```bash
# MCP Server configuration
MCP_API_LISTEN_ADDRESS=:8080
MCP_DATABASE_DSN=postgres://user:password@postgres:5432/mcp?sslmode=disable
MCP_AUTH_JWT_SECRET=your-jwt-secret
MCP_AUTH_API_KEYS_ADMIN=your-admin-api-key
```

### Running with Docker Compose

The easiest way to run the MCP Server locally is using Docker Compose:

```bash
docker-compose up -d
```

This will start the MCP Server along with its dependencies (PostgreSQL, Redis, Prometheus, and Grafana).

> **Note for Production Deployments**: The default configuration uses port 8080, which is suitable for development but not recommended for production. For production environments, you should configure HTTPS with TLS certificates on port 443. See the [Production Deployment Security Guide](docs/security/production-deployment-security.md) for details.

### Deploying to EKS

For production deployment on Amazon EKS, MCP Server includes Kubernetes manifests in the `kubernetes/` directory:

```bash
# Create the namespace
kubectl apply -f kubernetes/namespace.yaml

# Create the service account with IRSA annotations
kubectl apply -f kubernetes/serviceaccount.yaml

# Apply other resources
kubectl apply -f kubernetes/deployment.yaml
kubectl apply -f kubernetes/service.yaml
```

The deployment is configured to:
- Use IAM Roles for Service Accounts (IRSA) for secure AWS authentication
- Run on port 443 with TLS in production environments
- Set appropriate resource requests and limits
- Configure health checks and readiness probes
- Specify security contexts for least privilege

See the [AWS IRSA Setup Guide](docs/aws/aws-irsa-setup.md) for detailed configuration instructions for EKS deployments.

### Building and Running Locally

1. Install Go dependencies:

```bash
go mod download
```

2. Build the server:

```bash
go build -o mcp-server ./cmd/server
```

3. Update your `.env` file with real credentials.

4. Run the server:

```bash
./mcp-server
```

## Example Usage with AI Agent following the Model Context Protocol

Here's an example of how an AI agent would use the MCP Server to interact with GitHub:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/S-Corkum/mcp-server/pkg/client"
)

func main() {
	// Initialize MCP client
	mcpClient := client.NewClient(
		"http://localhost:8080",
		client.WithAPIKey("your-api-key"),
	)
	
	ctx := context.Background()
	
	// Create a context for the conversation
	contextData := &mcp.Context{
		AgentID: "test-agent",
		ModelID: "claude-3-haiku",
		MaxTokens: 4000,
		Content: []mcp.ContextItem{
			{
				Role: "system",
				Content: "You are a DevOps assistant that helps with GitHub operations.",
				Tokens: 12,
			},
		},
	}
	
	createdContext, err := mcpClient.CreateContext(ctx, contextData)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}
	
	log.Printf("Created context with ID: %s", createdContext.ID)
	
	// List available tools
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}
	
	log.Printf("Available tools: %v", tools)
	
	// List GitHub actions
	actions, err := mcpClient.ListToolActions(ctx, "github")
	if err != nil {
		log.Fatalf("Failed to list GitHub actions: %v", err)
	}
	
	log.Printf("Available GitHub actions: %v", actions)
	
	// Execute GitHub action to create an issue
	issueParams := map[string]interface{}{
		"owner": "your-organization",
		"repo":  "web-frontend",
		"title": "Login button not working on Safari",
		"body":  "The login button is not working on Safari browsers. This was reported by multiple users.",
		"labels": []string{"bug", "frontend", "priority-high"},
	}
	
	// Execute the action with context
	issueResult, err := mcpClient.ExecuteToolAction(ctx, createdContext.ID, "github", "create_issue", issueParams)
	if err != nil {
		log.Fatalf("Failed to execute GitHub action: %v", err)
	}
	
	log.Printf("Created GitHub issue: %v", issueResult)
	
	// Get the updated context
	updatedContext, err := mcpClient.GetContext(ctx, createdContext.ID)
	if err != nil {
		log.Fatalf("Failed to get updated context: %v", err)
	}
	
	log.Printf("Context now has %d items and %d tokens", len(updatedContext.Content), updatedContext.CurrentTokens)
	
	// Query GitHub repositories
	queryParams := map[string]interface{}{
		"owner": "your-organization",
		"type": "public",
	}
	
	queryResult, err := mcpClient.QueryToolData(ctx, "github", queryParams)
	if err != nil {
		log.Fatalf("Failed to query GitHub data: %v", err)
	}
	
	log.Printf("Query result: %v", queryResult)
}
```

This example demonstrates how an AI agent can:

1. Create and manage conversation contexts
2. Discover available tools and their capabilities
3. Execute tool actions to perform real-world tasks with context tracking
4. Query tool data to get information
5. Handle tool operations using the Model Context Protocol

The MCP server acts as a tool integration hub, giving the agent a standardized interface for interacting with GitHub and managing conversation contexts.

### Running Tests

You can run the Go unit tests with:

```bash
go test ./...
```

For testing AI Agent interactions with the MCP server, we've created a Python-based test suite. See the [Testing Guide](docs/testing-guide.md) for detailed instructions on how to run these tests.

## Configuration

The MCP Server can be configured using a YAML configuration file and/or environment variables. See the `configs/config.yaml.template` file for all available options.

### Environment Variables

All configuration options can be set using environment variables with the `MCP_` prefix. For example:

- `MCP_API_LISTEN_ADDRESS=:8080`
- `MCP_DATABASE_DSN=postgres://user:password@localhost:5432/mcp`
- `MCP_ENGINE_GITHUB_API_TOKEN=your_token`

## API Documentation

### Context API Endpoints

- Create Context: `POST /api/v1/contexts`
- Get Context: `GET /api/v1/contexts/:contextID`
- Update Context: `PUT /api/v1/contexts/:contextID`
- Delete Context: `DELETE /api/v1/contexts/:contextID`
- List Contexts: `GET /api/v1/contexts?agent_id=:agentID&session_id=:sessionID`
- Summarize Context: `GET /api/v1/contexts/:contextID/summary`
- Search in Context: `POST /api/v1/contexts/:contextID/search`

### Tool API Endpoints

- Execute Tool Action: `POST /api/v1/tools/:tool/actions/:action`
  (Note: Safety restrictions prevent dangerous operations like deleting repositories)
- Query Tool Data: `POST /api/v1/tools/:tool/query`
  (Note: Read-only access by default)
- List Available Tools: `GET /api/v1/tools`
- List Allowed Actions: `GET /api/v1/tools/:tool/actions`

### Webhook Endpoints

- GitHub: `POST /webhook/github`

### Health and Metrics

- Health Check: `GET /health`
- Metrics: `GET /metrics`

## Monitoring

The MCP Server integrates with Prometheus and Grafana for monitoring and observability. The Docker Compose setup includes both services.

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)



## AWS Service Integrations

MCP Server supports integration with AWS services using IAM Roles for Service Accounts (IRSA), providing secure, credential-less authentication for production deployments on EKS.

### IAM Roles for Service Accounts (IRSA)

Instead of managing static AWS credentials, MCP Server can use IRSA to assume IAM roles directly. This provides:

- No hardcoded access keys in configuration files or environment variables
- Temporary, automatically-rotated credentials
- Fine-grained access control using IAM policies
- Simplified security auditing and compliance

For detailed setup instructions, see the [AWS IRSA Setup Guide](docs/aws/aws-irsa-setup.md).

### RDS Aurora PostgreSQL Integration

MCP Server integrates with Amazon RDS Aurora PostgreSQL using IAM authentication:

- Connection pooling optimized for Aurora PostgreSQL
- Automatic token refresh for IAM authentication
- Fallback to standard authentication for local development
- Configurable timeouts and connection parameters

Configuration example:

```yaml
aws:
  rds:
    auth:
      region: "us-west-2"
    host: "your-aurora-cluster.cluster-xxxxxxxxx.us-west-2.rds.amazonaws.com"
    port: 5432
    database: "mcp"
    username: "mcp_admin"
    use_iam_auth: true
    token_expiration: 900 # 15 minutes in seconds
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 5m
    enable_pooling: true
    min_pool_size: 2
    max_pool_size: 10
    connection_timeout: 30
```

### S3 for Context Storage

MCP Server uses Amazon S3 for storing large context data:

- Efficient storage of large conversation histories
- Optimized multipart uploads and downloads
- Server-side encryption for data protection
- Fine-grained access control
- Cost-effective storage for high-volume deployments

Configuration example:

```yaml
aws:
  s3:
    auth:
      region: "us-west-2"
    bucket: "mcp-contexts"
    key_prefix: "contexts/"
    use_iam_auth: true
    encryption:
      enabled: true
      type: "AES256"
    multipart:
      threshold: 8388608 # 8MB
      part_size: 8388608 # 8MB
    retries:
      max_attempts: 3
      min_delay: 1s
      max_delay: 30s
```

### ElastiCache Redis Integration

MCP Server supports Amazon ElastiCache for Redis with advanced features:

- Support for Redis Cluster Mode with node discovery
- IAM-based authentication for enhanced security
- Automatic connection management and failover handling
- TLS encryption for secure communication
- Optimized connection pooling

Configuration example:

```yaml
aws:
  elasticache:
    auth:
      region: "us-west-2"
    cluster_mode: true
    cluster_name: "mcp-cache"
    username: "mcp_cache_user"
    use_iam_auth: true
    cluster_discovery: true
    use_tls: true
    max_retries: 3
    min_idle_connections: 2
    pool_size: 10
    dial_timeout: 5
    read_timeout: 3
    write_timeout: 3
    pool_timeout: 4
    token_expiration: 900 # 15 minutes in seconds
```

### AWS Integration

The MCP Server is designed to work well with AWS services through IAM Roles for Service Accounts (IRSA). This provides secure, credential-less authentication for production deployments.

For detailed setup instructions, see the [AWS IRSA Setup Guide](docs/aws/aws-irsa-setup.md).

## Architecture

The MCP Server is built with a modular architecture following the Model Context Protocol specification:

- **Context Manager**: Handles conversation context storage and management
- **Adapters**: Interface with GitHub API
- **Tool API**: Exposes GitHub capabilities through the MCP protocol
- **Core Engine**: Orchestrates tool operations and manages events
- **API Server**: Provides REST API endpoints following the MCP specification
- **Database**: Persists system state and tool configurations
- **Vector Storage**: Enables semantic search within contexts
- **Multi-tiered Cache**: Optimizes performance for context operations
- **Event System**: Handles events from GitHub
- **Webhook Handlers**: Processes webhook events from GitHub

For detailed information on the context management architecture, see the [Context Management Architecture Diagram](docs/diagrams/context-management-architecture.md).

### Performance Optimizations

- **Concurrency Management**: Worker pools with configurable limits
- **Caching Strategy**: Multi-tiered caching for context data
- **Database Optimizations**: Connection pooling and prepared statements
- **Resilience Patterns**: Circuit breakers and retry mechanisms
- **S3 Optimizations**: Multipart uploads/downloads for large contexts

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.