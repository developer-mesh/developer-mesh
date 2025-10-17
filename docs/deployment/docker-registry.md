<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:11
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Docker Registry Publishing Guide

This document describes how Docker images are built and published to GitHub Container Registry (ghcr.io) for the Developer Mesh project.

## Overview

The project automatically builds and publishes Docker images to GitHub Container Registry using GitHub Actions. Images are built for multiple architectures (amd64 and arm64) and tagged following industry best practices.

## Registry Information

- **Registry**: `ghcr.io` (GitHub Container Registry)
- **Image Namespace**: `developer-mesh-{service}` (local build)
- **Services**: `edge-mcp`, `rest-api`, `worker`, `mockserver`

## Image Naming Convention

Images follow this naming pattern:
```
developer-mesh-{service}:{tag}
```

Example:
```
ghcr.io/s-corkum/developer-mesh-edge-mcp:latest
ghcr.io/s-corkum/developer-mesh-rest-api:v1.2.3
```

## Tagging Strategy

The CI/CD pipeline automatically generates the following tags:

### Branch-based Tags
- `latest` - Updated on every push to the main branch
- `{branch-name}` - For feature branches (e.g., `feat/new-feature`)
- `pr-{number}` - For pull requests (e.g., `pr-123`)

### Version Tags (for releases)
- `v1.2.3` - Full semantic version
- `1.2.3` - Version without 'v' prefix
- `1.2` - Major.minor version
- `1` - Major version only

### Commit-based Tags
- `{short-sha}` - Short commit SHA (e.g., `abc1234`)

## Pulling Images

### Latest Stable Version
```bash
# Note: Images are built locally in development
# For production, replace 'developer-mesh' with your registry
docker pull developer-mesh-edge-mcp:latest
docker pull developer-mesh-rest-api:latest
docker pull developer-mesh-worker:latest
docker pull developer-mesh-mockserver:latest
```

### Specific Version
```bash
docker pull developer-mesh-edge-mcp:v1.2.3
docker pull developer-mesh-rest-api:v1.2.3
docker pull developer-mesh-worker:v1.2.3
```

### For ARM64 Architecture
All images support multi-architecture (amd64 and arm64). Docker will automatically pull the correct architecture for your system.

## Authentication

Public images can be pulled without authentication. For private repositories:

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

## Using Images in Docker Compose

Update your `docker-compose.yml` to use the published images:

```yaml
services:
  edge-mcp:
    image: developer-mesh-edge-mcp:latest
    # ... rest of configuration

  rest-api:
    image: developer-mesh-rest-api:latest
    # ... rest of configuration

  worker:
    image: developer-mesh-worker:latest
    # ... rest of configuration
```

## Image Metadata

All images include the following metadata:

- **Version**: The git tag or branch name
- **Commit SHA**: The git commit hash
- **Build Date**: When the image was built
- **Labels**: OCI standard labels for description, vendor, licenses, etc.

To inspect image metadata:
```bash
docker inspect developer-mesh-edge-mcp:latest
```

## Security Features

### Image Signing
Image signing with Sigstore Cosign is temporarily disabled while we resolve GitHub Actions OIDC integration issues. This feature will be re-enabled in a future update.

<!-- When re-enabled, verify signatures with:
```bash
cosign verify developer-mesh-edge-mcp:latest
```
-->

### Vulnerability Scanning
Images are automatically scanned for vulnerabilities using Trivy. Scan results are uploaded to GitHub Security tab.

### SBOM (Software Bill of Materials)
SBOMs are generated for each image and attached to releases. They provide a complete inventory of all packages and dependencies in the image.

## CI/CD Workflow

The Docker publishing workflow (`docker-publish.yml`) runs on:

1. **Push to main branch**: Builds and tags images as `latest`
2. **Push to release branches**: Builds and tags with branch name
3. **Git tags**: Builds and creates semantic version tags
4. **Pull requests**: Builds images but doesn't push (dry-run)
5. **Manual trigger**: Allows custom tag specification

## Build Arguments

The Dockerfiles accept the following build arguments:

- `VERSION`: The version string (default: "dev")
- `COMMIT_SHA`: Git commit hash (default: "unknown")
- `BUILD_DATE`: ISO 8601 build timestamp

These are automatically set by the CI/CD pipeline and embedded in the binary.

## Troubleshooting

### Permission Denied
If you get permission denied when pulling images, ensure:
1. The repository is public, or
2. You're authenticated with a valid GitHub token that has `read:packages` permission

### Wrong Architecture
Docker should automatically select the correct architecture. To force a specific architecture:
```bash
docker pull --platform linux/amd64 developer-mesh-edge-mcp:latest
```

### Old Image Versions
To ensure you have the latest version:
```bash
docker pull developer-mesh-edge-mcp:latest
docker images | grep developer-mesh
```

## Best Practices

1. **Use Specific Tags in Production**: Avoid using `latest` in production. Use specific version tags instead.
2. **Regular Updates**: Set up automated dependency updates to keep base images current.
3. **Image Scanning**: Regularly scan images for vulnerabilities, especially before production deployments.
4. **Size Optimization**: Images use multi-stage builds and distroless base images for minimal size.
5. **Cache Efficiency**: Build cache is optimized using GitHub Actions cache.

## Release Process

1. Create a git tag following semantic versioning: `v1.2.3`
2. Push the tag: `git push origin v1.2.3`
3. The CI/CD pipeline will:
   - Build multi-architecture images
   - Push images with appropriate tags
   - Create/update GitHub release
   - Attach SBOMs to the release
   - Sign the images

## Local Development

To build images locally with proper metadata:

```bash
# Build with version info
docker build \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t developer-mesh-edge-mcp:local \
  -f apps/edge-mcp/Dockerfile .
```

## References

- [GitHub Container Registry Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [OCI Image Specification](https://github.com/opencontainers/image-spec)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
