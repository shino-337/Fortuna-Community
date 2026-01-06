# Fortuna Build Guide

**Version**: 1.0  
**Last Updated**: 2026-01-05

---

## Overview

This guide covers building Fortuna components (Core and Agent) for production deployment.

---

## Prerequisites

### Required Tools

- **Go**: 1.21+ (tested with 1.21.5)
- **nerdctl**: For building container images with containerd (recommended)
- **ctr** (containerd CLI): For managing images in containerd
- **containerd**: Container runtime (usually pre-installed with Kubernetes)
- **Docker**: Optional, only if not using containerd

### Environment Variables

```bash
export GO_VERSION=1.21
export CORE_IMAGE=fortuna-core:latest
export AGENT_IMAGE=fortuna/agent:latest
export IMAGE_NAMESPACE=k8s.io  # For containerd
```

---

## Building with Containerd (Recommended)

Fortuna is designed to work with containerd, the container runtime used by Kubernetes. Use `nerdctl` to build images.

### Quick Build Script

The easiest way to build and load images:

```bash
cd /path/to/KSAM
bash scripts/build-and-load-containerd.sh
```

This script will:
- Check prerequisites (nerdctl, ctr, go)
- Build Core and Agent images
- Load images into containerd (k8s.io namespace)
- Verify images are available

### Build Core

#### Build Binary

```bash
cd core
go build -o ../bin/fortuna-core ./cmd
```

#### Build Image with nerdctl

```bash
cd /path/to/KSAM
nerdctl build -f core/Dockerfile -t fortuna-core:latest --namespace k8s.io .
```

#### Verify Image

```bash
# List images in containerd
ctr -n k8s.io images list | grep fortuna-core

# Or with nerdctl
nerdctl --namespace k8s.io images list | grep fortuna-core
```

---

## Building Agent

#### Build Binary

```bash
cd agent
go build -o ../bin/fortuna-agent ./cmd
```

#### Build Image with nerdctl

```bash
cd /path/to/KSAM
nerdctl build -f agent/Dockerfile -t fortuna/agent:latest --namespace k8s.io .
```

#### Verify Image

```bash
# List images in containerd
ctr -n k8s.io images list | grep fortuna/agent

# Or with nerdctl
nerdctl --namespace k8s.io images list | grep fortuna/agent
```

---

## Alternative: Using Docker (Not Recommended)

If you must use Docker, build images and then import to containerd:

```bash
# Build with Docker
docker build -f core/Dockerfile -t fortuna-core:latest .
docker build -f agent/Dockerfile -t fortuna/agent:latest .

# Export and import to containerd
docker save fortuna-core:latest -o fortuna-core.tar
docker save fortuna/agent:latest -o fortuna-agent.tar

# Import to containerd
ctr -n k8s.io images import fortuna-core.tar
ctr -n k8s.io images import fortuna-agent.tar
```

---

## Production Build Process

### 1. Version Tagging

For production, use versioned tags:

```bash
VERSION="v1.0.0"
BUILD_DATE=$(date +%Y%m%d-%H%M%S)
GIT_COMMIT=$(git rev-parse --short HEAD)

# Tag Core (in containerd namespace)
nerdctl --namespace k8s.io tag fortuna-core:latest fortuna-core:${VERSION}
nerdctl --namespace k8s.io tag fortuna-core:latest fortuna-core:${VERSION}-${BUILD_DATE}
nerdctl --namespace k8s.io tag fortuna-core:latest fortuna-core:${VERSION}-${GIT_COMMIT}

# Tag Agent
nerdctl --namespace k8s.io tag fortuna/agent:latest fortuna/agent:${VERSION}
nerdctl --namespace k8s.io tag fortuna/agent:latest fortuna/agent:${VERSION}-${BUILD_DATE}
nerdctl --namespace k8s.io tag fortuna/agent:latest fortuna/agent:${VERSION}-${GIT_COMMIT}
```

### 2. Build with Build Args

For production builds with specific configurations:

```bash
VERSION="v1.0.0"
BUILD_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

# Core
nerdctl build \
  --build-arg FORTUNA_BUILD_VERSION=${VERSION} \
  --build-arg FORTUNA_BUILD_COMMIT=${BUILD_COMMIT} \
  --build-arg FORTUNA_BUILD_TIME=${BUILD_TIME} \
  -f core/Dockerfile \
  -t fortuna-core:${VERSION} \
  --namespace k8s.io .

# Agent
nerdctl build \
  --build-arg FORTUNA_BUILD_VERSION=${VERSION} \
  --build-arg FORTUNA_BUILD_COMMIT=${BUILD_COMMIT} \
  --build-arg FORTUNA_BUILD_TIME=${BUILD_TIME} \
  -f agent/Dockerfile \
  -t fortuna/agent:${VERSION} \
  --namespace k8s.io .
```

Or use the provided script:

```bash
VERSION="v1.0.0" bash scripts/build-and-load-containerd.sh
```

### 3. Multi-Architecture Builds

For multi-architecture support (amd64, arm64):

```bash
# Using buildx (Docker)
docker buildx build --platform linux/amd64,linux/arm64 -t fortuna-core:${VERSION} -f core/Dockerfile .

# Note: nerdctl doesn't support buildx directly
# Use Docker buildx, then import to containerd
```

---

## Image Management

### Export Images

For transferring images between nodes:

```bash
VERSION="v1.0.0"

# Export Core image
nerdctl --namespace k8s.io save -o fortuna-core-${VERSION}.tar fortuna-core:${VERSION}

# Export Agent image
nerdctl --namespace k8s.io save -o fortuna-agent-${VERSION}.tar fortuna/agent:${VERSION}
```

Or use the script with export flag:

```bash
EXPORT_IMAGES=true bash scripts/build-and-load-containerd.sh
```

### Import Images

On target node:

```bash
# Import Core image
ctr -n k8s.io images import fortuna-core-${VERSION}.tar

# Import Agent image
ctr -n k8s.io images import fortuna-agent-${VERSION}.tar

# Verify import
ctr -n k8s.io images list | grep fortuna
```

### Verify Imported Images

```bash
ctr -n k8s.io images list | grep fortuna
```

---

## Build Optimization

### 1. Multi-Stage Builds

Both Core and Agent Dockerfiles use multi-stage builds to minimize image size:

- **Build stage**: Compiles Go binary
- **Runtime stage**: Minimal base image with only the binary

### 2. Build Cache

Leverage build cache for faster rebuilds:

```bash
# Build with cache
nerdctl build --cache-from fortuna-core:latest -f core/Dockerfile -t fortuna-core:latest --namespace k8s.io .
```

### 3. Parallel Builds

Build Core and Agent in parallel:

```bash
# Build both in parallel
(nerdctl build -f core/Dockerfile -t fortuna-core:latest --namespace k8s.io . &)
(nerdctl build -f agent/Dockerfile -t fortuna/agent:latest --namespace k8s.io . &)
wait
```

---

## Build Verification

### 1. Image Size Check

```bash
ctr -n k8s.io images list | grep fortuna
```

Expected sizes:
- Core: ~50-100MB
- Agent: ~50-100MB

### 2. Image Contents

```bash
# Inspect Core image
nerdctl inspect fortuna-core:latest --namespace k8s.io

# Inspect Agent image
nerdctl inspect fortuna/agent:latest --namespace k8s.io
```

### 3. Binary Verification

```bash
# Extract and verify binary
nerdctl run --rm -it --entrypoint /bin/sh fortuna-core:latest --namespace k8s.io
# Inside container:
ls -lh /usr/local/bin/fortuna-core
file /usr/local/bin/fortuna-core
```

---

## Troubleshooting

### Build Fails: Go Module Errors

**Error**: `go: cannot find module providing package`

**Solution**:
```bash
cd core  # or agent
go mod download
go mod tidy
```

### Build Fails: Docker/nerdctl Errors

**Error**: `failed to solve: failed to fetch`

**Solution**: Check network connectivity and Docker/containerd daemon status:

```bash
# Docker
docker info

# containerd
ctr version
```

### Image Too Large

**Solution**: 
1. Use multi-stage builds (already implemented)
2. Use distroless or alpine base images
3. Remove unnecessary files from final image

### Import Fails: Invalid Tar

**Error**: `ctr: invalid tar header`

**Solution**: Ensure tar file is not corrupted:

```bash
tar -tzf fortuna-core-${VERSION}.tar | head -5
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build and Push

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build Core
        run: |
          cd core
          go build -o ../bin/fortuna-core ./cmd
      
      - name: Build Agent
        run: |
          cd agent
          go build -o ../bin/fortuna-agent ./cmd
      
      - name: Build and push images
        run: |
          docker build -t fortuna-core:${{ github.ref_name }} -f core/Dockerfile .
          docker build -t fortuna/agent:${{ github.ref_name }} -f agent/Dockerfile .
```

---

## Next Steps

- [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
- [Architecture Documentation](ARCHITECTURE.md)
- [Environment Preparation](ENVIRONMENT_PREPARATION.md)


---

## Quick Reference

### Build Everything

```bash
# Using the provided script (recommended)
bash scripts/build-and-load-containerd.sh

# Or manually
nerdctl build -f core/Dockerfile -t fortuna-core:latest --namespace k8s.io .
nerdctl build -f agent/Dockerfile -t fortuna/agent:latest --namespace k8s.io .
```

### Verify Images

```bash
# List all Fortuna images
ctr -n k8s.io images list | grep fortuna

# Check specific image
ctr -n k8s.io images ls fortuna-core:latest
```

### Export for Multi-Node

```bash
# Export images
EXPORT_IMAGES=true bash scripts/build-and-load-containerd.sh

# Or manually
nerdctl --namespace k8s.io save -o fortuna-core.tar fortuna-core:latest
nerdctl --namespace k8s.io save -o fortuna-agent.tar fortuna/agent:latest

# Copy to other nodes and import
scp fortuna-*.tar user@node:/tmp/
ssh user@node "ctr -n k8s.io images import /tmp/fortuna-core.tar"
ssh user@node "ctr -n k8s.io images import /tmp/fortuna-agent.tar"
```

### Load Exported Images

```bash
# On target node
ctr -n k8s.io images import fortuna-core.tar
ctr -n k8s.io images import fortuna-agent.tar

# Verify
ctr -n k8s.io images list | grep fortuna
```

---

