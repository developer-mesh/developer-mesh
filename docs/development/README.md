# Development Documentation

> Developer guide for contributing to Developer Mesh

## 📚 Documentation Structure

### Setup
- **[Development Environment](./setup/environment.md)** - Setting up your dev environment
- **[Local Development](./setup/local.md)** - Local development guide

### Architecture
- **[Go Workspace](./architecture/go-workspace.md)** - Go workspace structure
- **[Dependencies](./architecture/dependencies.md)** - Package dependencies

### Testing
- **[Testing Guide](./testing/guide.md)** - Comprehensive testing guide
- **[UUID Guidelines](./testing/uuid.md)** - UUID usage in tests

### Contributing
- **[Contributing Guide](./contributing/guide.md)** - How to contribute

### Other
- **[Debugging Guide](./debugging.md)** - Debugging tips and tricks

## 🚀 Quick Start

### 1. Prerequisites
```bash
# Required tools
- Go 1.24.6+
- Docker & Docker Compose
- PostgreSQL 14+
- Redis 7+
- Make
```

### 2. Clone and Setup
```bash
# Clone repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Install dependencies
go work sync
go mod download

# Setup environment
cp .env.example .env.development
make setup
```

### 3. Start Development
```bash
# Start all services
make dev

# Or start individually
make dev-mcp      # MCP server only
make dev-api      # REST API only
make dev-worker   # Worker only
```

### 4. Verify Setup
```bash
# Run tests
make test

# Check health
curl http://localhost:8080/health
curl http://localhost:8081/health
```

## 🏗️ Project Structure

```
developer-mesh/
├── apps/               # Application services
│   ├── mcp-server/    # WebSocket MCP server
│   ├── rest-api/      # REST API service
│   └── worker/        # Background worker
├── pkg/               # Shared packages
│   ├── auth/          # Authentication
│   ├── models/        # Data models
│   └── tools/         # Tool integrations
├── migrations/        # Database migrations
├── configs/          # Configuration files
└── scripts/          # Utility scripts
```

## 🛠️ Development Workflow

### 1. Create Feature Branch
```bash
git checkout -b feature/your-feature
```

### 2. Make Changes
- Write code following Go idioms
- Add tests for new functionality
- Update documentation

### 3. Test Changes
```bash
# Run all tests
make test

# Run specific service tests
cd apps/mcp-server && go test ./...

# Run with coverage
make test-coverage
```

### 4. Lint and Format
```bash
# Format code
make fmt

# Run linter
make lint

# Pre-commit checks
make pre-commit
```

### 5. Submit PR
```bash
# Commit changes
git add .
git commit -m "feat: your feature description"

# Push and create PR
git push origin feature/your-feature
gh pr create
```

## 🧪 Testing

### Test Structure
```
service_test.go       # Unit tests
integration_test.go   # Integration tests
e2e_test.go          # End-to-end tests
```

### Running Tests
```bash
# All tests
make test

# Unit tests only
go test -short ./...

# Integration tests
go test -run Integration ./...

# Specific package
go test ./pkg/auth/...
```

### Test Coverage
```bash
# Generate coverage report
make test-coverage

# View in browser
go tool cover -html=coverage.out
```

## 🐛 Debugging

### Debug Mode
```bash
# Enable debug logging
DEBUG=true make dev

# Or set in environment
export DEBUG=true
```

### Debugging Tools
- **Delve**: Go debugger
- **pprof**: Performance profiling
- **trace**: Execution tracing

### Common Issues
- Import errors: Run `go work sync`
- Database issues: Check migrations
- Redis issues: Verify connection

## 📝 Code Standards

### Go Conventions
- Follow `gofmt` formatting
- Use meaningful variable names
- Write clear comments
- Handle errors explicitly

### Git Commit Convention
```
type(scope): description

feat: New feature
fix: Bug fix
docs: Documentation
test: Testing
refactor: Code refactoring
```

### Code Review Checklist
- [ ] Tests pass
- [ ] Code is formatted
- [ ] Documentation updated
- [ ] No security issues
- [ ] Performance considered

## 🔧 Development Tools

### Recommended IDE
- **VS Code** with Go extension
- **GoLand** for full IDE experience
- **Neovim** with gopls

### Useful Commands
```bash
make dev          # Start development
make test         # Run tests
make fmt          # Format code
make lint         # Lint code
make build        # Build binaries
make clean        # Clean build artifacts
```

## 📚 Learning Resources

### Internal Documentation
- [Architecture Overview](../architecture/overview.md)
- [API Reference](../api/)
- [Testing Guide](./testing/guide.md)

### External Resources
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Best Practices](https://go.dev/doc/effective_go)

## 🤝 Contributing

### Ways to Contribute
- Report bugs
- Suggest features
- Submit PRs
- Improve documentation
- Review code

### Contribution Process
1. Check existing issues
2. Discuss major changes
3. Follow coding standards
4. Write tests
5. Update documentation

## 🔗 Related Documentation

- [Architecture](../architecture/) - System design
- [API Reference](../api/) - API documentation
- [Deployment](../deployment/) - Deployment guide

## 🆘 Getting Help

- Check [Debugging Guide](./debugging.md)
- Review [Contributing Guide](./contributing/guide.md)
- Ask in GitHub Discussions

---

*Last updated: August 2025*