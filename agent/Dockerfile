# Multi-stage build for Fortuna Agent (Production-Ready)
# Build context: repository root
# Usage: docker build -f agent/Dockerfile -t fortuna-agent:latest .
FROM golang:1.24 AS builder
ENV DEBIAN_FRONTEND=noninteractive
WORKDIR /build

# Build metadata
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown
ARG BUILDKIT_INLINE_CACHE=1

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    git \
    make \
    && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

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
RUN go mod download && \
    # Clean up any existing build cache before building
    go clean -cache 2>/dev/null || true && \
    rm -rf /tmp/go-build-cache /root/.cache/go-build 2>/dev/null || true

# Copy agent source
COPY agent/ ./

# Verify dependencies
RUN go mod verify

# Build Agent binary
# Note: Go 1.24 requires build cache to be enabled (cannot use GOCACHE=off)
# Use -a flag to rebuild all packages, then cleanup cache immediately
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -a \
    -o /fortuna-agent ./cmd/main.go && \
    # Clean build cache immediately after build to save disk space
    go clean -cache && \
    rm -rf /tmp/go-build-cache /root/.cache/go-build 2>/dev/null || true

# Verify Agent binary exists
RUN test -f /fortuna-agent || (echo "ERROR: /fortuna-agent not found after build" && exit 1)

# Build identifier for verifying image in pod (same pattern as dashboard)
RUN echo "build=$(date -Iseconds) utc" > /build-version.txt

# Clean up Go caches to free disk space (after verifying binary exists)
RUN go clean -cache -modcache -testcache && \
    rm -rf /tmp/go-build-cache /tmp/go-mod-cache /root/.cache/go-build 2>/dev/null || true

# Final stage - minimal runtime image
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

# Install only runtime dependencies
# Note: docker-cli is needed for SBOM extraction
# Retry apt-get update in case of network issues (up to 5 retries)
RUN for i in 1 2 3 4 5; do \
        echo "Attempt $i: Updating Debian packages..." && \
        apt-get update && break || \
        (echo "Attempt $i failed, retrying in 5 seconds..." && sleep 5); \
    done && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    docker.io \
    && \
    # Create non-root user (UID/GID 500 to avoid SYS_UID_MAX warning and conflict with existing 100)
    groupadd -r -g 500 fortuna && \
    useradd -r -m -u 500 -g fortuna fortuna && \
    # Clean up
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /app

# Copy binary and version file from builder
COPY --from=builder --chown=fortuna:fortuna /fortuna-agent /app/fortuna-agent
COPY --from=builder /build-version.txt /app/version.txt

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
