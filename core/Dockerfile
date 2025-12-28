# Multi-stage build for Fortuna Core
# Build context: repository root
# Usage: docker build -f core/Dockerfile -t fortuna-core:latest .
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Build metadata
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git

# Copy go.work and go.work.sum (if exists) for workspace support
COPY go.work* ./

# Copy api module first (needed for go.mod replace directive)
COPY api/ ./api/

# Copy core directory (includes go.mod, go.sum, and source)
COPY core/ ./core/
WORKDIR /build/core

# Download dependencies (before building for better caching)
RUN go mod download

# Tidy dependencies
RUN go mod tidy

# Build Core binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -o /fortuna-core ./cmd/main.go

# Build CVE loader binary (if exists)
RUN if [ -f "./cmd/cve-loader/main.go" ]; then \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s" \
        -trimpath \
        -o /cve-loader ./cmd/cve-loader/main.go; \
    else \
        touch /cve-loader; \
    fi

# Final stage
FROM alpine:3.20

RUN apk update && apk --no-cache add ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /fortuna-core /app/fortuna-core
# Copy CVE loader (always create the file, even if empty)
COPY --from=builder /cve-loader /tmp/cve-loader
RUN if [ -s /tmp/cve-loader ]; then \
        mv /tmp/cve-loader /app/cve-loader && chmod +x /app/cve-loader; \
    else \
        rm -f /tmp/cve-loader; \
    fi
COPY --from=builder /build/core/rules /app/rules/
COPY --from=builder /build/core/migrations /app/migrations/

EXPOSE 8080 9090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/fortuna-core"]
