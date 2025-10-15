# Multi-stage Dockerfile for RAG Loader
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates gcc musl-dev curl

# Set working directory
WORKDIR /build

# Copy go workspace files
COPY go.work go.work.sum ./

# Copy all modules for workspace
COPY apps/ apps/
COPY pkg/ pkg/

# Download dependencies
RUN go mod download

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT=unknown

# Build the binary with optimizations
WORKDIR /build/apps/rag-loader
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X 'main.version=${VERSION}' \
    -X 'main.buildTime=${BUILD_TIME}' \
    -X 'main.gitCommit=${GIT_COMMIT}'" \
    -a -installsuffix cgo \
    -trimpath \
    -o /build/rag-loader \
    ./cmd/loader

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 ragloader && \
    adduser -D -u 1000 -G ragloader ragloader

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/rag-loader /app/rag-loader

# Copy configuration files
COPY --from=builder /build/apps/rag-loader/configs/ /app/configs/

# Change ownership
RUN chown -R ragloader:ragloader /app

# Create directories for runtime
RUN mkdir -p /app/data /app/logs && \
    chown -R ragloader:ragloader /app/data /app/logs

# Switch to non-root user
USER ragloader

# Expose ports
# 8084: API server
# 9094: Health and metrics
EXPOSE 8084 9094

# Health check with improved timeout
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:9094/health || exit 1

# Labels for metadata
LABEL maintainer="DevMesh Team"
LABEL description="RAG Loader Service for Developer Mesh"
LABEL version="${VERSION}"

# Run the service
ENTRYPOINT ["/app/rag-loader"]
CMD []