# Multi-stage build for Fortuna Agent
# Build context: repository root
# Usage: docker build -f agent/Dockerfile -t fortuna-agent:latest .
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

# Copy agent directory (includes go.mod, go.sum, and source)
COPY agent/ ./agent/
WORKDIR /build/agent

# Download dependencies (before building for better caching)
RUN go mod download

# Tidy dependencies
RUN go mod tidy

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.BuildVersion=${FORTUNA_BUILD_VERSION} \
    -X main.BuildCommit=${FORTUNA_BUILD_COMMIT} \
    -X main.BuildTime=${FORTUNA_BUILD_TIME}" \
    -trimpath \
    -o /fortuna-agent ./cmd/main.go

# Final stage
FROM alpine:3.20

RUN apk update && apk --no-cache add ca-certificates tzdata docker-cli

WORKDIR /app

COPY --from=builder /fortuna-agent /app/fortuna-agent

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep fortuna-agent || exit 1

USER root

ENTRYPOINT ["/app/fortuna-agent"]
