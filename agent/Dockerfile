# Multi-stage build for Fortuna Agent (Production-Ready)
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Build metadata
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown
ARG BUILDKIT_INLINE_CACHE=1

# Install build dependencies
RUN apk add --no-cache git make

# Copy go.work and go.work.sum for workspace support
COPY go.work go.work.sum ./

# Copy api module first (needed for go.mod replace directive)
COPY api/ ./api/

# Copy agent go.mod and download deps (cache layer)
COPY agent/go.mod agent/go.sum ./agent/
WORKDIR /build/agent
RUN go mod download

# Copy agent source
COPY agent/ ./

# Verify dependencies
RUN go mod verify

# Build Agent binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -o /fortuna-agent ./cmd/main.go

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

# Copy binary from builder
COPY --from=builder --chown=fortuna:fortuna /fortuna-agent /app/fortuna-agent

# Set ownership
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
