# syntax=docker/dockerfile:1
# Edge MCP Dockerfile following the workspace pattern

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
RUN cd apps/edge-mcp && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT_SHA}" \
    -o /tmp/edge-mcp ./cmd/server

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy binary
COPY --from=builder /tmp/edge-mcp /app/edge-mcp

# Copy configuration files
COPY --chown=nonroot:nonroot apps/edge-mcp/configs /app/configs

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8082

# Minimal environment variables
ENV PORT=8082

# Metadata labels
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application with Docker config by default
ENTRYPOINT ["/app/edge-mcp"]
CMD ["--config", "/app/configs/config.docker.yaml", "--port", "8082"]