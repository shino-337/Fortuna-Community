# Multi-stage build for Fortuna Core
FROM golang:1.24-alpine AS builder

WORKDIR /build/core

# Build metadata
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git

# Copy api module first (needed for go.mod replace directive)
COPY api/ /build/api/

# Copy core go.mod and download deps
COPY core/go.mod core/go.sum ./
RUN go mod download

# Copy core source
COPY core/ ./

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

# Build CVE loader binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /cve-loader ./cmd/cve-loader/main.go

# Final stage
FROM alpine:3.20

RUN apk update && apk --no-cache add ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /fortuna-core /app/fortuna-core
COPY --from=builder /cve-loader /app/cve-loader
COPY --from=builder /build/core/rules /app/rules/
COPY --from=builder /build/core/migrations /app/migrations/

EXPOSE 8080 9090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/fortuna-core"]
