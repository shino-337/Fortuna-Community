FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./

# Download dependencies with timeout and retry
RUN go mod download || (sleep 5 && go mod download) || (sleep 10 && go mod download)

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /ksam-agent ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /ksam-agent /usr/local/bin/ksam-agent

RUN chmod +x /usr/local/bin/ksam-agent

USER 1000

WORKDIR /tmp

ENTRYPOINT ["/usr/local/bin/ksam-agent"]

