FROM golang:1.24-alpine AS builder

WORKDIR /app

# Build metadata (used to verify the running binary is updated)
ARG KSAM_BUILD_VERSION=dev
ARG KSAM_BUILD_COMMIT=none
ARG KSAM_BUILD_TIME=unknown

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
# Use -a flag to force rebuild all packages
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -gcflags="all=-N -l" \
    -ldflags "-X main.BuildVersion=${KSAM_BUILD_VERSION} -X main.BuildCommit=${KSAM_BUILD_COMMIT} -X main.BuildTime=${KSAM_BUILD_TIME}" \
    -trimpath \
    -o /ksam-core ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl wget

WORKDIR /root/

# Custom SBOM pipeline - no external tools needed!
# All SBOM extraction and CVE matching is done in Go code

COPY --from=builder /ksam-core .

# Copy rules directory if it exists (rules are in core/rules from build context)
COPY --from=builder /app/rules /root/rules/

EXPOSE 8080 9090

ENTRYPOINT ["./ksam-core"]

