# Multi-stage build for Fortuna Agent
FROM golang:1.24-alpine AS builder

WORKDIR /build/agent

# Build metadata
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git

# Copy proto module first
COPY api/proto/agent /build/api/proto/agent/

# Copy agent go.mod and download deps
COPY agent/go.mod agent/go.sum ./
RUN go mod download

# Copy agent source
COPY agent/ ./

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

RUN apk --no-cache add ca-certificates tzdata docker-cli

WORKDIR /app

COPY --from=builder /fortuna-agent /app/fortuna-agent

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep fortuna-agent || exit 1

USER root

ENTRYPOINT ["/app/fortuna-agent"]
