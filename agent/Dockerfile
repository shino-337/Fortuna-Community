# syntax=docker/dockerfile:1
# Multi-stage build for Fortuna Agent (Production-Ready)
# Build context: repository root
# Usage: docker build -f agent/Dockerfile -t fortuna-agent:latest .
# Go deps/build use BuildKit cache mounts when BuildKit is enabled.
FROM golang:1.26.8 AS builder
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
    export GOMODCACHE=/gomodcache GOCACHE=/gocache; \
    go mod verify && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -o /fortuna-agent ./cmd/main.go
RUN --mount=type=cache,target=/gomodcache \
    --mount=type=cache,target=/gocache \
    export GOMODCACHE=/gomodcache GOCACHE=/gocache; \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -trimpath -buildvcs=false \
    -o /fortuna-image-export ./cmd/image-export

# Verify Agent binary exists
RUN test -f /fortuna-agent || (echo "ERROR: /fortuna-agent not found after build" && exit 1)

# Build identifier for verifying image in pod (same pattern as dashboard)
RUN echo "build=$(date -Iseconds) utc" > /build-version.txt

# Build the pinned Syft release with the patched Go toolchain. The upstream
# v1.52.0 binary was built with Go 1.26.3, which has known stdlib findings.
FROM golang:1.26.8 AS syft-builder
ARG SYFT_VERSION=v1.52.0
RUN --mount=type=cache,target=/gomodcache \
    --mount=type=cache,target=/gocache \
    export GOMODCACHE=/gomodcache GOCACHE=/gocache GOBIN=/out CGO_ENABLED=0; \
    go install -ldflags="-w -s -X main.version=${SYFT_VERSION}" github.com/anchore/syft/cmd/syft@${SYFT_VERSION} && \
    /out/syft --version

# Final stage - minimal runtime image
FROM debian:trixie-slim
ENV DEBIAN_FRONTEND=noninteractive

ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown

# Install only runtime dependencies
# The Agent uses the containerd Go client and Syft directly; Docker CLI and
# its bundled daemon/runc/containerd packages are not needed in this image.
# Retry apt-get update in case of network issues (up to 5 retries)
RUN for i in 1 2 3 4 5; do \
        echo "Attempt $i: Updating Debian packages..." && \
        apt-get update && break || \
        (echo "Attempt $i failed, retrying in 5 seconds..." && sleep 5); \
    done && \
    apt-get upgrade -y --no-install-recommends && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    tzdata \
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
COPY --from=builder /fortuna-image-export /app/fortuna-image-export
COPY --from=builder /build-version.txt /app/version.txt
COPY --from=syft-builder --chown=root:root /out/syft /usr/local/bin/syft

# Ensure all files are owned by fortuna user
RUN chown -R fortuna:fortuna /app && \
    chmod 0755 /app/fortuna-agent /usr/local/bin/syft && \
    syft --version > /app/syft-version.txt && \
    test -f /app/fortuna-agent && test -f /usr/local/bin/syft

# Runs as root so the image-export container (same image, command
# /app/fortuna-image-export) can open the root-owned containerd socket. The
# DaemonSet drops every capability for both containers; see
# docs/06-reference/SECURITY.md#agent-privileges.
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
