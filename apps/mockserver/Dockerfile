# syntax=docker/dockerfile:1
# Simple, reliable Dockerfile for mockserver

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

# Copy go.work files first to enable workspace
COPY go.work go.work.sum ./

# Copy everything - simple and reliable
COPY . .

# Change to the app directory and build
WORKDIR /workspace/apps/mockserver
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
    -o /tmp/mockserver ./cmd/main.go

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates from builder stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary
COPY --from=builder /tmp/mockserver /app/mockserver

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8081

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
ENTRYPOINT ["/app/mockserver"]