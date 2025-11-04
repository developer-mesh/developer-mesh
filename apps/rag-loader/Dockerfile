# syntax=docker/dockerfile:1
# RAG Loader Dockerfile following the workspace pattern

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
RUN cd apps/rag-loader && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT_SHA}" \
    -o /tmp/rag-loader ./cmd/loader

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates from builder stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary
COPY --from=builder /tmp/rag-loader /app/rag-loader

# Copy configuration files
COPY --chown=nonroot:nonroot apps/rag-loader/configs /app/configs

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose ports
# 8084: API server
# 9094: Health and metrics
EXPOSE 8084 9094

# Minimal environment variables
ENV PORT=8084 \
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
ENTRYPOINT ["/app/rag-loader"]
CMD []