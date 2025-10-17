# Developer Mesh Documentation

Welcome to the Developer Mesh documentation. This guide provides comprehensive information for users, developers, and operators of the Developer Mesh platform - an AI agent orchestration platform for DevOps workflows.

## Quick Links

- **New to Developer Mesh?** Start with the [Quick Start Guide](getting-started/quick-start-guide.md)
- **Setting up your organization?** See [Organization Setup](getting-started/organization-setup.md)
- **Need API docs?** Check the [API Reference](reference/api/)
- **Having issues?** Visit [Troubleshooting](troubleshooting/)

## Documentation Structure

Our documentation is organized following the [Divio documentation system](https://documentation.divio.com/) to help you find exactly what you need:

### 📚 [Getting Started](getting-started/)
**Start here if you're new to Developer Mesh**

Quick tutorials and initial setup guides to get you up and running:
- [Quick Start Guide](getting-started/quick-start-guide.md) - Get running in 5 minutes
- [Organization Setup](getting-started/organization-setup.md) - Create and configure your organization
- [Authentication Quick Start](getting-started/authentication-quick-start.md) - Set up authentication

### 📖 [Guides](guides/)
**Task-oriented how-to guides**

Step-by-step instructions for specific tasks and features:

- **[Agents](guides/agents/)** - Building and managing AI agents
- **[Authentication](guides/authentication/)** - Auth patterns and OAuth integration
- **[Embeddings](guides/embeddings/)** - Working with embeddings and semantic search
- **[Integrations](guides/integrations/)** - Integrating with external tools and platforms
- **[RAG](guides/rag/)** - RAG (Retrieval-Augmented Generation) setup and patterns
- **[Operations](guides/operations/)** - Performance tuning, observability, and cost management

### 📋 [Reference](reference/)
**Technical reference documentation**

Detailed API documentation and configuration reference:

- **[API Reference](reference/api/)** - REST API endpoints and schemas
- **[MCP Protocol](reference/mcp-protocol/)** - Model Context Protocol specification
- **[Configuration](reference/configuration/)** - Environment variables and configuration options
- **[OpenAPI](reference/openapi/)** - OpenAPI specifications and SDKs

### 🏗️ [Architecture](architecture/)
**Understanding the system design**

In-depth explanations of system architecture and concepts:
- [System Overview](architecture/system-overview.md) - High-level architecture
- [Go Workspace Structure](architecture/go-workspace-structure.md) - Project organization
- [Multi-Agent Embedding Architecture](architecture/multi-agent-embedding-architecture.md) - Embedding system design
- [Package Dependencies](architecture/package-dependencies.md) - Module relationships

### 🚀 [Deployment](deployment/)
**Production deployment and operations**

Deployment guides, monitoring, and operational runbooks:
- Production deployment strategies
- Security best practices
- Monitoring and observability
- Infrastructure configuration

### 💡 [Examples](examples/)
**Real-world code examples**

Working code samples organized by category:
- **[Agents](examples/agents/)** - AI agent integration examples
- **[Integrations](examples/integrations/)** - GitHub, IDE, and platform integrations
- **[Tools](examples/tools/)** - Custom tool integration examples

### 🤝 [Contributing](contributing/)
**For developers and contributors**

Development guides and contribution guidelines:
- [Development Environment](contributing/development-environment.md) - Local setup
- [Testing Guide](contributing/testing-guide.md) - Writing and running tests
- [Debugging Guide](contributing/debugging-guide.md) - Troubleshooting development issues
- [Contributing Guidelines](contributing/CONTRIBUTING.md) - How to contribute

### 🔧 [Troubleshooting](troubleshooting/)
**Problem-solving guides**

Common issues and their solutions:
- [Troubleshooting Guide](troubleshooting/README.md) - Common problems and fixes
- Component-specific troubleshooting guides

## Finding Information by Role

### 👨‍💻 Application Developer
You want to integrate Developer Mesh into your application:
1. Start with [Quick Start Guide](getting-started/quick-start-guide.md)
2. Review [API Reference](reference/api/)
3. Check [Integration Examples](examples/integrations/)
4. Follow [Integration Guides](guides/integrations/)

### 🔧 Platform Developer
You want to contribute to or extend Developer Mesh:
1. Set up your [Development Environment](contributing/development-environment.md)
2. Understand the [System Architecture](architecture/system-overview.md)
3. Review [Contributing Guidelines](contributing/CONTRIBUTING.md)
4. Check [Testing Guide](contributing/testing-guide.md)

### ⚙️ DevOps Engineer
You want to deploy and operate Developer Mesh:
1. Review [Deployment Guide](deployment/)
2. Set up [Monitoring](deployment/monitoring.md)
3. Understand [Security Best Practices](deployment/security.md)
4. Follow [Operations Runbook](deployment/operations-runbook.md)

### 🤖 AI Engineer
You want to build and orchestrate AI agents:
1. Read [Agent Guides](guides/agents/)
2. Review [Agent Examples](examples/agents/)
3. Understand [Multi-Agent Architecture](architecture/multi-agent-embedding-architecture.md)
4. Check [RAG Integration Guide](guides/rag/)

## Documentation Principles

Our documentation follows these core principles:

1. **Clear and Concise** - Easy to understand, no unnecessary jargon
2. **Example-Driven** - Every concept illustrated with real code
3. **Up-to-Date** - Reflects the current implementation
4. **Well-Organized** - Follows the Divio documentation system
5. **Accessible** - Written for various skill levels

## Getting Help

Can't find what you need?

1. **Search** the documentation using your browser's search (Cmd/Ctrl + F)
2. **Check** [Troubleshooting](troubleshooting/) for common issues
3. **Browse** [Examples](examples/) for working code samples
4. **Review** [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
5. **Ask** in [GitHub Discussions](https://github.com/developer-mesh/developer-mesh/discussions)

## Contributing to Documentation

Documentation improvements are always welcome! To contribute:

1. Follow our [Contributing Guidelines](contributing/CONTRIBUTING.md)
2. Ensure documentation follows our principles above
3. Include code examples where applicable
4. Test all code examples before submitting
5. Submit a pull request with clear description

## Documentation Standards

When writing documentation:

- Use present tense ("it creates" not "it will create")
- Use active voice ("the server processes" not "the request is processed")
- Keep sentences short and focused
- Include code examples with comments
- Link to related documentation
- Update table of contents when adding sections

## Version Information

- **Documentation Version**: 1.0.0
- **Last Updated**: January 2025
- **Platform Version**: See [main README](../README.md)

---

**Need help?** Check our [Troubleshooting Guide](troubleshooting/) or [open an issue](https://github.com/developer-mesh/developer-mesh/issues).
