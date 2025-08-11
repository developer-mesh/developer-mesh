# Developer Mesh Documentation

Welcome to the Developer Mesh documentation. This guide provides comprehensive information for users, developers, and operators of the Developer Mesh platform.

## 📖 Documentation Overview

### [Getting Started](getting-started/)
Start here if you're new to Developer Mesh
- [Quick Start Guide](getting-started/quick-start-guide.md) - Get up and running in minutes
- Installation & Configuration guides

### [Architecture](architecture/)
Understanding the system design
- [System Overview](architecture/system-overview.md) - High-level architecture
- [Go Workspace Structure](architecture/go-workspace-structure.md) - Multi-module organization
- [Multi-Agent Embedding Architecture](architecture/multi-agent-embedding-architecture.md) - Embedding system design
- [Package Dependencies](architecture/package-dependencies.md) - Module dependencies

### [API Reference](api-reference/)
Complete API documentation
- [REST API Reference](api-reference/rest-api-reference.md) - REST endpoints
- [MCP Server Reference](api-reference/mcp-server-reference.md) - WebSocket protocol
- [Embedding API](api-reference/embedding-api-reference.md) - Multi-agent embedding endpoints
- [Webhook API](api-reference/webhook-api-reference.md) - Webhook processing
- [Authentication API](api-reference/authentication-api-reference.md) - Auth endpoints

### [Developer Guide](developer/)
For contributors and developers
- [Development Environment](developer/development-environment.md) - Setup instructions
- [Debugging Guide](developer/debugging-guide.md) - Troubleshooting tips

### [Examples](examples/)
Real-world integration examples
- [GitHub Integration](examples/github-integration.md) - Using GitHub features
- [AI Agent Integration](examples/ai-agent-integration.md) - Connecting AI assistants
- [Custom Tools](examples/custom-tool-integration.md) - Adding new integrations
- [IDE Integration](examples/ide-integration.md) - Using with Windsurf, Cursor, and other IDEs
- [Binary WebSocket Protocol](examples/binary-websocket-protocol.md) - Binary protocol examples
- [Embedding Examples](examples/embedding-examples.md) - Using embeddings

## 🚀 Quick Links

### For Users
- [Quick Start Guide](getting-started/quick-start-guide.md)
- [Local Development](LOCAL_DEVELOPMENT.md)
- [Examples](examples/README.md)

### For Developers
- [Development Setup](developer/development-environment.md)
- [Contributing Guide](../CONTRIBUTING.md)
- [Architecture Overview](architecture/system-overview.md)

### For Operators
- [Operations Runbook](operations/OPERATIONS_RUNBOOK.md)
- [Monitoring](operations/MONITORING.md)
- [Security](operations/SECURITY.md)

## 📚 Documentation Structure

```
docs/
├── architecture/           # System design and architecture
├── api-reference/         # API documentation
├── developer/             # Developer guides
├── examples/              # Usage examples
├── getting-started/       # Quick start and setup
├── operations/            # Deployment and operations
└── troubleshooting/       # Problem solving guides
```

## 🔍 Finding Information

### By Role

**Application Developer**
- Start with [Examples](examples/README.md)
- Review [API Reference](api-reference/)
- Check [Integration Patterns](examples/)

**Platform Developer**
- Read [Architecture](architecture/system-overview.md)
- Set up [Development Environment](developer/development-environment.md)
- Follow [Contributing Guide](../CONTRIBUTING.md)

**DevOps Engineer**
- Check [Operations Guide](operations/)
- Review [Configuration Guide](operations/configuration-guide.md)
- Understand [Monitoring](operations/MONITORING.md)

### By Topic

**Integration**
- [GitHub Integration](examples/github-integration.md)
- [AI Agent Setup](examples/ai-agent-integration.md)
- [Custom Tools](examples/custom-tool-integration.md)

**Embedding & Search**
- [Embedding Examples](examples/embedding-examples.md)
- [Embedding API Reference](api-reference/embedding-api-reference.md)

**Troubleshooting**
- [Troubleshooting Guide](TROUBLESHOOTING.md)
- [Debugging Guide](developer/debugging-guide.md)

## 📝 Documentation Standards

Our documentation follows these principles:

1. **Clear and Concise**: Easy to understand
2. **Example-Driven**: Real-world code examples
3. **Up-to-Date**: Reflects current implementation
4. **Searchable**: Well-organized and indexed
5. **Accessible**: Written for various skill levels

## 🤝 Contributing to Documentation

Documentation improvements are always welcome! See our [Contributing Guide](../CONTRIBUTING.md) for:

- Documentation style guide
- How to submit documentation PRs
- Building documentation locally

## 📞 Getting Help

Can't find what you need?

1. Search the documentation
2. Check [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
3. Ask in [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)
4. Review [Examples](examples/README.md)

---

*Last updated: January 2025*