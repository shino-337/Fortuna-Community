# Multi-stage build for Fortuna Agent (Production-Ready)
# Build context: repository root
# Usage: docker build -f agent/Dockerfile -t fortuna-agent:latest .
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Build metadata
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown
ARG BUILDKIT_INLINE_CACHE=1

# Install build dependencies
RUN apk add --no-cache git make

# Copy api module first (needed for go.mod replace directive)
COPY api/ ./api/

# Copy agent go.mod and go.sum first (for better layer caching)
COPY agent/go.mod agent/go.sum ./agent/
WORKDIR /build/agent

# Download dependencies (before copying source for better caching)
# Note: We don't copy go.work because it references ./core which isn't needed for Agent build
# The replace directive in go.mod handles the api module dependency
# Use temp directory for Go cache to reduce disk usage
ENV GOCACHE=/tmp/go-build-cache
ENV GOMODCACHE=/tmp/go-mod-cache
RUN go mod download

# Copy agent source
COPY agent/ ./

# Verify dependencies
RUN go mod verify

# Build Agent binary and cleanup cache immediately
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -o /fortuna-agent ./cmd/main.go && \
    # Clean build cache immediately to free space
    go clean -cache && \
    rm -rf /tmp/go-build-cache /root/.cache/go-build 2>/dev/null || true

# Final cleanup of all Go caches
RUN go clean -modcache -testcache && \
    rm -rf /tmp/go-build-cache /tmp/go-mod-cache /root/.cache/go-build 2>/dev/null || true

# Final stage - minimal runtime image
FROM alpine:3.20

# Install only runtime dependencies
# Note: docker-cli is needed for SBOM extraction
RUN apk update && \
    apk --no-cache add \
    ca-certificates \
    tzdata \
    docker-cli \
    && \
    # Create non-root user (but we'll run as root for docker socket access)
    addgroup -g 1000 fortuna && \
    adduser -D -u 1000 -G fortuna fortuna && \
    # Clean up
    rm -rf /var/cache/apk/*

WORKDIR /app

# Copy binary from builder with proper ownership
COPY --from=builder --chown=fortuna:fortuna /fortuna-agent /app/fortuna-agent

# Ensure all files are owned by fortuna user
RUN chown -R fortuna:fortuna /app

# Note: Agent runs as root (USER root) because it needs access to:
# - Docker socket (/var/run/docker.sock)
# - Container runtime for SBOM extraction
# This is a known security trade-off for DaemonSet workloads that need host access.
# In production, consider:
# 1. Using a dedicated service account with minimal privileges
# 2. Running in a security context with only necessary capabilities
# 3. Using rootless Docker or containerd if possible
USER root

# Healthcheck (simple process check)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep -f fortuna-agent > /dev/null || exit 1

# Labels for metadata
LABEL org.opencontainers.image.title="Fortuna Agent" \
      org.opencontainers.image.description="Fortuna K8s Management Platform - Agent DaemonSet" \
      org.opencontainers.image.vendor="Fortuna" \
      org.opencontainers.image.version="${FORTUNA_BUILD_VERSION}" \
      org.opencontainers.image.revision="${FORTUNA_BUILD_COMMIT}"

ENTRYPOINT ["/app/fortuna-agent"]
