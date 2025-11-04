# syntax=docker/dockerfile:1
# Simple, reliable Dockerfile for rest-api

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
RUN cd apps/rest-api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
    -o /tmp/rest-api ./cmd/api

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates from builder stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary
COPY --from=builder /tmp/rest-api /app/rest-api

# Copy configuration files
COPY --chown=nonroot:nonroot apps/rest-api/configs /app/configs

# Copy migrations
COPY --chown=nonroot:nonroot apps/rest-api/migrations/sql /app/migrations/sql

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8081

# Health check - removed since we'll use docker-compose healthcheck

# Minimal environment variables
ENV PORT=8081 \
    GIN_MODE=release \
    SSL_CERT_DIR=/etc/ssl/certs \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

# Metadata labels
ARG VERSION
ARG COMMIT_SHA  
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application
ENTRYPOINT ["/app/rest-api"]