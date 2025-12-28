# Multi-stage build for Fortuna Core (Production-Ready)
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

# Copy core go.mod and download deps (cache layer)
COPY core/go.mod core/go.sum ./core/
WORKDIR /build/core
RUN go mod download

# Copy core source
COPY core/ ./

# Verify dependencies
RUN go mod verify

# Build Core binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -buildvcs=false \
    -o /fortuna-core ./cmd/main.go

# Build CVE loader binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -buildvcs=false \
    -o /cve-loader ./cmd/cve-loader/main.go

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

# Copy binaries from builder
COPY --from=builder --chown=fortuna:fortuna /fortuna-core /app/fortuna-core
COPY --from=builder --chown=fortuna:fortuna /cve-loader /app/cve-loader

# Copy migrations and rules
COPY --from=builder --chown=fortuna:fortuna /build/core/migrations /app/migrations/
COPY --from=builder --chown=fortuna:fortuna /build/core/rules /app/rules/ 2>/dev/null || true

# Set ownership
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
