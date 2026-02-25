# Multi-stage build for Fortuna Core (Production-Ready)
# Build context: repository root
# Usage: docker build -f core/Dockerfile -t fortuna-core:latest .
FROM golang:1.24 AS builder

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

# Copy core go.mod and go.sum first (for better layer caching)
COPY core/go.mod core/go.sum ./core/
WORKDIR /build/core

# Download dependencies (before copying source for better caching)
# Note: We don't copy go.work because it references ./agent which isn't needed for Core build
# The replace directive in go.mod handles the api module dependency
# Use temp directory for Go cache to reduce disk usage
ENV GOCACHE=/tmp/go-build-cache
ENV GOMODCACHE=/tmp/go-mod-cache
RUN go mod download && \
    # Clean up any existing build cache before building
    go clean -cache 2>/dev/null || true && \
    rm -rf /tmp/go-build-cache /root/.cache/go-build 2>/dev/null || true

# Copy core source
COPY core/ ./

# Verify dependencies
RUN go mod verify

# Build Core binary
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
    -o /fortuna-core ./cmd/main.go && \
    # Clean build cache immediately after build to save disk space
    go clean -cache && \
    rm -rf /tmp/go-build-cache /root/.cache/go-build 2>/dev/null || true

# Verify Core binary exists
RUN test -f /fortuna-core || (echo "ERROR: /fortuna-core not found after build" && exit 1)

# Build CVE loader binary (if exists) - cleanup cache after
RUN if [ -f "./cmd/cve-loader/main.go" ]; then \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -trimpath \
        -buildvcs=false \
        -a \
        -o /cve-loader ./cmd/cve-loader/main.go && \
        go clean -cache && \
        test -f /cve-loader || touch /cve-loader; \
    else \
        touch /cve-loader; \
    fi

# Clean up Go caches to free disk space (after verifying binaries exist)
RUN go clean -cache -modcache -testcache && \
    rm -rf /tmp/go-build-cache /tmp/go-mod-cache /root/.cache/go-build 2>/dev/null || true

# Final stage - minimal runtime image
FROM debian:bookworm-slim

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

# Copy binaries from builder with proper ownership
COPY --from=builder --chown=fortuna:fortuna /fortuna-core /app/fortuna-core
# Copy CVE loader if it exists and has content
COPY --from=builder /cve-loader /tmp/cve-loader
RUN if [ -s /tmp/cve-loader ]; then \
        mv /tmp/cve-loader /app/cve-loader && \
        chmod +x /app/cve-loader && \
        chown fortuna:fortuna /app/cve-loader; \
    else \
        rm -f /tmp/cve-loader; \
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
