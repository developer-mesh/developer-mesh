# syntax=docker/dockerfile:1
# Simple, reliable Dockerfile for mcp-server

# Build arguments
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /workspace

# Copy everything
COPY . .

# Explicitly set GOWORK to use workspace mode
ENV GOWORK=/workspace/go.work

# Build the binary
RUN cd apps/mcp-server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
    -o /tmp/mcp-server ./cmd/server

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy binary
COPY --from=builder /tmp/mcp-server /app/mcp-server

# Copy configuration files
COPY --chown=nonroot:nonroot apps/mcp-server/configs /app/configs

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/app/mcp-server", "-health-check"]

# Minimal environment variables
ENV PORT=8080 \
    GIN_MODE=release

# Metadata labels
ARG VERSION
ARG COMMIT_SHA  
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application
# MCP server never runs migrations - only REST API handles them
ENTRYPOINT ["/app/mcp-server"]