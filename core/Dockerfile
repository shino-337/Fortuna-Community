# Multi-stage build for Fortuna Core (Production-Ready)
# Build context: repository root
# Usage: docker build -f core/Dockerfile -t fortuna-core:latest .
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

# Copy core go.mod and go.sum first (for better layer caching)
COPY core/go.mod core/go.sum ./core/
WORKDIR /build/core

# Download dependencies (before copying source for better caching)
# Note: We don't copy go.work because it references ./agent which isn't needed for Core build
# The replace directive in go.mod handles the api module dependency
# Use temp directory for Go cache to reduce disk usage
ENV GOCACHE=/tmp/go-build-cache
ENV GOMODCACHE=/tmp/go-mod-cache
RUN go mod download

# Copy core source
COPY core/ ./

# Verify dependencies
RUN go mod verify

# Build Core binary and cleanup cache immediately
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -o /fortuna-core ./cmd/main.go && \
    # Clean build cache immediately to free space
    go clean -cache && \
    rm -rf /tmp/go-build-cache /root/.cache/go-build 2>/dev/null || true

# Build CVE loader binary (if exists) and cleanup
RUN if [ -f "./cmd/cve-loader/main.go" ]; then \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -trimpath \
        -buildvcs=false \
        -o /cve-loader ./cmd/cve-loader/main.go && \
        go clean -cache; \
    else \
        touch /cve-loader; \
    fi

# Final cleanup of all Go caches
RUN go clean -modcache -testcache && \
    rm -rf /tmp/go-build-cache /tmp/go-mod-cache /root/.cache/go-build 2>/dev/null || true

# Final stage - minimal runtime image
FROM alpine:3.20

# Install only runtime dependencies
RUN apk update && \
    apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && \
    # Create non-root user
    addgroup -g 1000 fortuna && \
    adduser -D -u 1000 -G fortuna fortuna && \
    # Clean up
    rm -rf /var/cache/apk/*

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
