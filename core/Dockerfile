# syntax=docker/dockerfile:1
# Multi-stage build for Fortuna Core (Production-Ready)
# Build context: repository root
# Usage: docker build -f core/Dockerfile -t fortuna-core:latest .
# Go deps/build use BuildKit cache mounts (shared across rebuilds; use NO_CACHE on the build tool for cold layers).
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

# Copy core go.mod and go.sum first (for better layer caching)
COPY core/go.mod core/go.sum ./core/
WORKDIR /build/core

# Download modules (cache mounts persist across local/CI builds when BuildKit is enabled)
# Note: We don't copy go.work because it references ./agent which isn't needed for Core build
RUN --mount=type=cache,target=/gomodcache \
    --mount=type=cache,target=/gocache \
    GOMODCACHE=/gomodcache GOCACHE=/gocache \
    go mod download

# Copy core source
COPY core/ ./

# Verify + build Core (no -a: allows incremental compile from gocache between builds)
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
    -o fortuna-core ./cmd/main.go && \
    test -f fortuna-core

# Build CVE loader binaries (optional; empty files if main.go missing for COPY --from=builder)
RUN --mount=type=cache,target=/gomodcache \
    --mount=type=cache,target=/gocache \
    export GOMODCACHE=/gomodcache GOCACHE=/gocache; \
    set -e; \
    if [ -f "./cmd/cve-loader/main.go" ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -trimpath \
        -buildvcs=false \
        -o /cve-loader ./cmd/cve-loader/main.go; \
    else \
      : > /cve-loader; \
    fi; \
    if [ -f "./cmd/cve-loader-optimized/main.go" ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -trimpath \
        -buildvcs=false \
        -o /cve-loader-optimized ./cmd/cve-loader-optimized/main.go; \
    else \
      : > /cve-loader-optimized; \
    fi

# Final stage - minimal runtime image
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown

# Install only runtime dependencies
# Retry apt-get update in case of network issues (up to 5 retries)
RUN for i in 1 2 3 4 5; do \
        echo "Attempt $i: Updating Debian packages..." && \
        apt-get update && break || \
        (echo "Attempt $i failed, retrying in 5 seconds..." && sleep 5); \
    done && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    curl \
    && \
    # Create non-root user (UID/GID 1000 to avoid conflict with system GID 100 in debian)
    groupadd -r -g 1000 fortuna && \
    useradd -r -m -u 1000 -g fortuna fortuna && \
    # Clean up
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /app

# Copy binaries from builder (paths under builder WORKDIR /build/core)
COPY --from=builder --chown=fortuna:fortuna /build/core/fortuna-core /app/fortuna-core
# Copy CVE loaders if they exist and have content
COPY --from=builder /cve-loader /tmp/cve-loader
COPY --from=builder /cve-loader-optimized /tmp/cve-loader-optimized
RUN if [ -s /tmp/cve-loader ]; then \
        mv /tmp/cve-loader /app/cve-loader && \
        chmod +x /app/cve-loader && \
        chown fortuna:fortuna /app/cve-loader; \
    else \
        rm -f /tmp/cve-loader; \
    fi && \
    if [ -s /tmp/cve-loader-optimized ]; then \
        mv /tmp/cve-loader-optimized /app/cve-loader-optimized && \
        chmod +x /app/cve-loader-optimized && \
        chown fortuna:fortuna /app/cve-loader-optimized; \
    else \
        rm -f /tmp/cve-loader-optimized; \
    fi

# Copy migrations and rules with proper ownership
COPY --from=builder --chown=fortuna:fortuna /build/core/migrations /app/migrations/
COPY --from=builder --chown=fortuna:fortuna /build/core/rules /app/rules/

# Ensure all files are owned by fortuna user
RUN chown -R fortuna:fortuna /app

# Switch to non-root user
USER fortuna

# Expose ports
EXPOSE 8080 9090

# Healthcheck (using curl)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Labels for metadata
LABEL org.opencontainers.image.title="Fortuna Core" \
      org.opencontainers.image.description="Fortuna K8s Management Platform - Core Service" \
      org.opencontainers.image.vendor="Fortuna" \
      org.opencontainers.image.version="${FORTUNA_BUILD_VERSION}" \
      org.opencontainers.image.revision="${FORTUNA_BUILD_COMMIT}"

ENTRYPOINT ["/app/fortuna-core"]
