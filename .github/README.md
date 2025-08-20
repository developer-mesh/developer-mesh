# Developer Mesh (Model Context Protocol)

![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)
![Go](https://img.shields.io/badge/go-1.24.6+-00ADD8.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/developer-mesh/developer-mesh)](https://goreportcard.com/report/github.com/developer-mesh/developer-mesh)

> A production-ready platform connecting AI agents to DevOps tools through a unified, standardized API

## Overview

Developer Mesh (Model Context Protocol) provides a standardized, secure interface for AI agents to interact with DevOps tools, manage conversation contexts, and perform vector-based semantic search. Built with Go workspaces for modular architecture, it bridges the gap between Large Language Models (LLMs) and external DevOps tools.

### 🚀 Key Capabilities

- **Multi-Tool Integration**: Unified API supporting GitHub, with extensible adapter pattern for additional tools
- **Context Management**: Efficient storage and retrieval of conversation contexts with S3 support
- **Vector Search**: Semantic search using pgvector with support for multiple embedding models
- **Event-Driven Architecture**: Asynchronous processing with SQS integration
- **Production-Ready**: Built-in observability, circuit breakers, and rate limiting

## 🏗️ Architecture

Built using Go workspaces for modularity and clean architecture:

- **Three-Service Architecture**: MCP server, REST API, and Worker services
- **Adapter Pattern**: Clean separation between business logic and external integrations
- **Event-Driven**: Asynchronous processing with AWS SQS support
- **Resilience Patterns**: Circuit breakers, retry logic, and bulkheads
- **Observability**: OpenTelemetry tracing and Prometheus metrics

## 🚀 Quick Start

### Option 1: Using Pre-built Docker Images (Recommended)

The fastest way to get started - no build required!

```bash
# Clone for configuration files
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Copy environment configuration
cp .env.example .env
# Edit .env with your settings

# Pull and run the latest images
GITHUB_USERNAME={github-username} ./scripts/pull-images.sh
docker-compose -f docker-compose.prod.yml up -d

# Access services
# - MCP Server: http://localhost:8080
# - REST API: http://localhost:8081
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000
```

### Option 2: Building from Source

For development or customization:

#### Prerequisites

- **Go 1.24.6+** (required for workspace support)
- Docker & Docker Compose
- PostgreSQL 14+ with pgvector extension
- Redis 6.2+
- Make
- AWS credentials (optional, for S3/SQS integration)

#### Local Development

```bash
# Clone the repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Copy configuration template
cp config.yaml.example config.yaml
# Edit config.yaml with your settings (especially API tokens)

# Start infrastructure services (includes PostgreSQL, Redis, LocalStack)
make dev-setup

# Build all services
make build

# Run database migrations
make migrate-local

# Option 1: Start services (in separate terminals)
make run-mcp-server
make run-rest-api
make run-worker

# Option 2: Run all services with Docker Compose
make local-dev

# Verify health
curl http://localhost:8080/health
curl http://localhost:8081/health
```

## 📚 Documentation

- [Quick Start Guide](docs/getting-started/quick-start-guide.md) - Get up and running quickly
- [Architecture Overview](docs/architecture/system-overview.md) - System design and components
- [API Reference](docs/api-reference/vector-search-api.md) - API endpoints and examples
- [Development Environment](docs/developer/development-environment.md) - Setup for contributors
- [Examples](docs/examples/README.md) - Integration examples and use cases

### Key Documentation

- **Architecture**: [System Overview](docs/architecture/system-overview.md) | [Adapter Pattern](docs/architecture/adapter-pattern.md) | [Go Workspace Structure](docs/architecture/go-workspace-structure.md)
- **Integration Examples**: [GitHub](docs/examples/github-integration.md) | [AI Agent](docs/examples/ai-agent-integration.md) | [Vector Search](docs/examples/vector-search-implementation.md)
- **Developer Resources**: [Development Environment](docs/developer/development-environment.md) | [Debugging Guide](docs/developer/debugging-guide.md)

## 📁 Project Structure

```
developer-mesh/
├── apps/                      # Go workspace applications
│   ├── mcp-server/           # Main MCP protocol server
│   │   ├── cmd/server/       # Server entrypoint
│   │   └── internal/         # Internal packages
│   ├── rest-api/             # REST API service
│   │   ├── cmd/api/          # API entrypoint
│   │   └── internal/         # Internal packages
│   └── worker/               # Event processing worker
│       ├── cmd/worker/       # Worker entrypoint
│       └── internal/         # Internal packages
├── pkg/                      # Shared packages (importable)
│   ├── adapters/            # External service adapters
│   ├── common/              # Common utilities
│   ├── database/            # Database abstractions
│   ├── embedding/           # Vector embedding services
│   ├── models/              # Shared data models
│   ├── observability/       # Logging, metrics, tracing
│   └── repository/          # Data access patterns
├── docs/                    # Documentation
├── scripts/                 # Utility scripts
├── migrations/              # Database migrations
├── configs/                 # Configuration files
├── go.work                  # Go workspace definition
└── Makefile                 # Build automation
```

## 🛠️ Technology Stack

- **Language**: Go 1.24.6+ with workspace support
- **Databases**: PostgreSQL 14+ (with pgvector), Redis 6.2+
- **Message Queue**: AWS SQS
- **Storage**: AWS S3 (optional)
- **Observability**: OpenTelemetry, Prometheus
- **API Framework**: Gin (REST API)
- **Testing**: Go testing package, testify, gomock

## 🐳 Docker Images

Pre-built Docker images are available on GitHub Container Registry:

- `ghcr.io/{github-username}/developer-mesh-mcp-server` - MCP protocol server
- `ghcr.io/{github-username}/developer-mesh-rest-api` - REST API service
- `ghcr.io/{github-username}/developer-mesh-worker` - Event processing worker
- `ghcr.io/{github-username}/developer-mesh-mockserver` - Mock server for testing

All images:
- Support multiple architectures (amd64, arm64)
- Are signed with Sigstore Cosign
- Include SBOMs (Software Bill of Materials)
- Follow semantic versioning

See [Docker Registry Guide](docs/docker-registry.md) for detailed information.

## 🧪 Testing

```bash
# Run unit tests
make test

# Run integration tests (requires Docker)
make test-integration

# Run functional tests (requires full stack)
make test-functional

# Test coverage
make test-coverage

# Generate HTML coverage report
make test-coverage-html
open coverage.html
```

## 🚢 Deployment

The services are containerized and can be deployed using:

- **Docker Compose**: For local development and testing
- **Kubernetes**: Production deployment with Helm charts (coming soon)
- **AWS ECS**: Native AWS deployment option

See [deployment documentation](docs/operations/) for detailed instructions.

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](docs/contributing/CONTRIBUTING.md) for:

- Code of Conduct
- Development workflow
- Coding standards
- Pull request process

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- OpenTelemetry for observability standards
- pgvector for vector similarity search
- The Go community for excellent tooling
