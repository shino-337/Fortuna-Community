# syntax=docker/dockerfile:1
# Multi-stage build for Fortuna Agent (Production-Ready)
# Build context: repository root
# Usage: docker build -f agent/Dockerfile -t fortuna-agent:latest .
# Go deps/build use BuildKit cache mounts when BuildKit is enabled.
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
    && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy api module first (needed for go.mod replace directive)
COPY api/ ./api/

# Copy agent go.mod and go.sum first (for better layer caching)
COPY agent/go.mod agent/go.sum ./agent/
WORKDIR /build/agent

RUN --mount=type=cache,target=/gomodcache \
    --mount=type=cache,target=/gocache \
    GOMODCACHE=/gomodcache GOCACHE=/gocache \
    go mod download

# Copy agent source
COPY agent/ ./

RUN --mount=type=cache,target=/gomodcache \
    --mount=type=cache,target=/gocache \
    GOMODCACHE=/gomodcache GOCACHE=/gocache \
    go mod verify && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -o /fortuna-agent ./cmd/main.go

# Verify Agent binary exists
RUN test -f /fortuna-agent || (echo "ERROR: /fortuna-agent not found after build" && exit 1)

# Build identifier for verifying image in pod (same pattern as dashboard)
RUN echo "build=$(date -Iseconds) utc" > /build-version.txt

# Syft acquirer stage (optional build arg cache URL, release fallback)
FROM debian:bookworm-slim AS syft-acquirer
ENV DEBIAN_FRONTEND=noninteractive
WORKDIR /tmp

ARG SYFT_VERSION=v0.98.0
ARG SYFT_BIN_CACHE_URL=""

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl tar && \
    rm -rf /var/lib/apt/lists/*

# 1) Try internal binary cache first (fast path) if configured.
# 2) Fall back to official GitHub release artifact with retries (transient resets/timeouts are common).
RUN set -eux; \
    if [ -n "${SYFT_BIN_CACHE_URL}" ]; then \
      echo "Trying Syft binary cache: ${SYFT_BIN_CACHE_URL}"; \
      if curl -fsSL --max-time 30 --retry 3 --retry-delay 3 "${SYFT_BIN_CACHE_URL}/syft-${SYFT_VERSION}-linux-amd64.tar.gz" -o /tmp/syft.tgz; then \
        tar -xzf /tmp/syft.tgz -C /tmp; \
      fi; \
    fi; \
    if [ ! -f /tmp/syft ]; then \
      url="https://github.com/anchore/syft/releases/download/${SYFT_VERSION}/syft_${SYFT_VERSION#v}_linux_amd64.tar.gz"; \
      echo "Downloading Syft from ${url}"; \
      ok=0; \
      for i in 1 2 3 4 5 6; do \
        if curl -fsSL --max-time 180 --retry 5 --retry-delay 8 --retry-all-errors "$url" -o /tmp/syft.tgz; then \
          ok=1; \
          break; \
        fi; \
        echo "Syft download attempt $i failed, retrying in 20s..."; \
        rm -f /tmp/syft.tgz; \
        sleep 20; \
      done; \
      test "$ok" = 1; \
      tar -xzf /tmp/syft.tgz -C /tmp; \
    fi; \
    test -f /tmp/syft; \
    install -m 0755 /tmp/syft /syft-bin; \
    /syft-bin --version; \
    rm -rf /tmp/*

# Final stage - minimal runtime image
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown

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
    curl \
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
COPY --from=syft-acquirer --chown=root:root /syft-bin /usr/local/bin/syft

# Ensure all files are owned by fortuna user
RUN chown -R fortuna:fortuna /app && \
    chmod 0755 /app/fortuna-agent /usr/local/bin/syft && \
    syft --version > /app/syft-version.txt && \
    test -f /app/fortuna-agent && test -f /usr/local/bin/syft

# Note: Agent runs as root (USER root) because it needs access to:
# - Docker socket (/var/run/docker.sock)
# - Container runtime for SBOM extraction
# This is a known security trade-off for DaemonSet workloads that need host access.
# In production, consider:
# 1. Using a dedicated service account with minimal privileges
# 2. Running in a security context with only necessary capabilities
# 3. Using rootless Docker or containerd if possible
USER root

# Healthcheck: ensure agent process and syft binary are available
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD sh -c 'syft --version >/dev/null 2>&1 && pgrep -f fortuna-agent >/dev/null'

# Labels for metadata
LABEL org.opencontainers.image.title="Fortuna Agent" \
      org.opencontainers.image.description="Fortuna K8s Management Platform - Agent DaemonSet" \
      org.opencontainers.image.vendor="Fortuna" \
      org.opencontainers.image.version="${FORTUNA_BUILD_VERSION}" \
      org.opencontainers.image.revision="${FORTUNA_BUILD_COMMIT}"

ENTRYPOINT ["/app/fortuna-agent"]
