FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./

# Download dependencies with timeout and retry
RUN go mod download || (sleep 5 && go mod download) || (sleep 10 && go mod download)

# Copy source code
COPY . .

# Update go.mod if needed
RUN go mod tidy

# Build with optimizations (removed -w to keep strings for debugging)
# Disable optimization to ensure code is not optimized away
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -gcflags="all=-N -l" \
    -trimpath \
    -o /ksam-core ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /ksam-core .

EXPOSE 8080 9090

ENTRYPOINT ["./ksam-core"]

