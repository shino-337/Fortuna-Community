# Multi-stage build for Fortuna Agent
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy agent module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary with version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X main.version=${VERSION} \
    -X main.buildTime=${BUILD_TIME} \
    -X main.gitCommit=${GIT_COMMIT}" \
    -trimpath \
    -o /agent \
    ./cmd/main.go

# Final stage - minimal runtime image
FROM alpine:3.20

# Install CA certificates for TLS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 fortuna && \
    adduser -D -u 1000 -G fortuna fortuna

# Copy binary from builder
COPY --from=builder /agent /usr/local/bin/agent

# Set ownership
RUN chown fortuna:fortuna /usr/local/bin/agent

# Switch to non-root user
USER fortuna

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD pgrep agent || exit 1

ENTRYPOINT ["/usr/local/bin/agent"]
