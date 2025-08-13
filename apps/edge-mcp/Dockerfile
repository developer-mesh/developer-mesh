# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o edge-mcp \
    cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates git docker-cli

# Create non-root user
RUN addgroup -g 1000 edge && \
    adduser -D -u 1000 -G edge edge

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/edge-mcp /app/edge-mcp

# Copy configuration
COPY configs/config.yaml /app/configs/config.yaml

# Change ownership
RUN chown -R edge:edge /app

# Switch to non-root user
USER edge

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the binary
ENTRYPOINT ["/app/edge-mcp"]
CMD ["--config", "/app/configs/config.yaml"]