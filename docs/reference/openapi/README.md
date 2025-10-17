# Edge MCP OpenAPI Specification

This directory contains the OpenAPI 3.1 specification for Edge MCP and tools for generating client SDKs.

## Files

- `edge-mcp.yaml` - Complete OpenAPI 3.1 specification for Edge MCP
- `generate-sdks.sh` - Script to generate client SDKs in multiple languages
- `examples/` - Example client code demonstrating API usage

## Generating Client SDKs

### Prerequisites

Install OpenAPI Generator:
```bash
# Using npm
npm install -g @openapitools/openapi-generator-cli

# Using Homebrew (macOS)
brew install openapi-generator

# Using Docker (no installation required)
# See Docker examples below
```

### Generate SDKs

```bash
# Make the generation script executable
chmod +x generate-sdks.sh

# Generate all SDKs
./generate-sdks.sh

# Generate specific SDK
./generate-sdks.sh go
./generate-sdks.sh python
./generate-sdks.sh typescript
```

### Using Docker (No Java/npm Required)

```bash
# Generate Go SDK
docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
  -i /local/edge-mcp.yaml \
  -g go \
  -o /local/sdks/go \
  --additional-properties=packageName=edgemcp,packageVersion=1.0.0

# Generate Python SDK
docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
  -i /local/edge-mcp.yaml \
  -g python \
  -o /local/sdks/python \
  --additional-properties=packageName=edgemcp,projectName=edge-mcp-client,packageVersion=1.0.0

# Generate TypeScript SDK
docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli generate \
  -i /local/edge-mcp.yaml \
  -g typescript-fetch \
  -o /local/sdks/typescript \
  --additional-properties=npmName=edge-mcp-client,npmVersion=1.0.0
```

### Manual Generation

```bash
# Go
openapi-generator-cli generate \
  -i docs/openapi/edge-mcp.yaml \
  -g go \
  -o docs/openapi/sdks/go \
  --additional-properties=packageName=edgemcp,packageVersion=1.0.0

# Python
openapi-generator-cli generate \
  -i docs/openapi/edge-mcp.yaml \
  -g python \
  -o docs/openapi/sdks/python \
  --additional-properties=packageName=edgemcp,projectName=edge-mcp-client,packageVersion=1.0.0

# TypeScript (fetch-based)
openapi-generator-cli generate \
  -i docs/openapi/edge-mcp.yaml \
  -g typescript-fetch \
  -o docs/openapi/sdks/typescript \
  --additional-properties=npmName=edge-mcp-client,npmVersion=1.0.0

# Java
openapi-generator-cli generate \
  -i docs/openapi/edge-mcp.yaml \
  -g java \
  -o docs/openapi/sdks/java \
  --additional-properties=artifactId=edge-mcp-client,groupId=com.developer-mesh,artifactVersion=1.0.0

# C#
openapi-generator-cli generate \
  -i docs/openapi/edge-mcp.yaml \
  -g csharp-netcore \
  -o docs/openapi/sdks/csharp \
  --additional-properties=packageName=EdgeMcp.Client,packageVersion=1.0.0

# Ruby
openapi-generator-cli generate \
  -i docs/openapi/edge-mcp.yaml \
  -g ruby \
  -o docs/openapi/sdks/ruby \
  --additional-properties=gemName=edge_mcp_client,gemVersion=1.0.0
```

## Supported Languages

OpenAPI Generator supports 50+ languages and frameworks. Common options:

- `go` - Go client
- `python` - Python client
- `typescript-fetch` - TypeScript with fetch API
- `typescript-axios` - TypeScript with axios
- `javascript` - JavaScript client
- `java` - Java client
- `csharp-netcore` - C# .NET Core
- `ruby` - Ruby client
- `php` - PHP client
- `rust` - Rust client
- `kotlin` - Kotlin client
- `swift5` - Swift 5 client

See full list: https://openapi-generator.tech/docs/generators

## Example Usage

See `examples/` directory for complete working examples in:
- Go (`examples/go/`)
- Python (`examples/python/`)
- TypeScript (`examples/typescript/`)

## Viewing the Specification

### Swagger UI (Docker)

```bash
docker run -p 8080:8080 -e SWAGGER_JSON=/app/edge-mcp.yaml \
  -v ${PWD}/edge-mcp.yaml:/app/edge-mcp.yaml swaggerapi/swagger-ui
```

Then open http://localhost:8080

### Online Viewers

Upload `edge-mcp.yaml` to:
- https://editor.swagger.io
- https://redocly.com/redoc/

## Validating the Specification

```bash
# Using OpenAPI Generator
openapi-generator-cli validate -i edge-mcp.yaml

# Using Spectral
npm install -g @stoplight/spectral-cli
spectral lint edge-mcp.yaml

# Using Docker
docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli validate \
  -i /local/edge-mcp.yaml
```

## Integration with Development Tools

### VSCode

Install extensions:
- Swagger Viewer
- OpenAPI (Swagger) Editor

### IntelliJ IDEA

Built-in OpenAPI support in Ultimate edition.

### Postman

Import `edge-mcp.yaml` directly into Postman to create a collection.

## Contributing

When updating the specification:

1. Edit `edge-mcp.yaml`
2. Validate the spec: `openapi-generator-cli validate -i edge-mcp.yaml`
3. Regenerate SDKs: `./generate-sdks.sh`
4. Update examples if API changes
5. Test generated clients

## References

- [OpenAPI 3.1 Specification](https://spec.openapis.org/oas/v3.1.0)
- [OpenAPI Generator Documentation](https://openapi-generator.tech/docs/usage)
- [MCP Protocol Specification](https://modelcontextprotocol.io/specification/2025-06-18)
- [Edge MCP Documentation](../../README.md)
