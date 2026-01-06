# Dockerfile Production Guide

**Date**: 2025-12-28  
**Version**: 3.0

---

## Overview

This document describes the production-ready Dockerfiles and deployment configurations for the Fortuna K8s Management Platform.

**Base Image**: Debian Bookworm Slim (changed from Alpine for better compatibility)

---

## Dockerfile Improvements

### Core Dockerfile (`core/Dockerfile`)

#### Security Enhancements
- ✅ **Non-root user**: Runs as `fortuna` user (UID 1000)
- ✅ **Minimal base image**: Debian Bookworm Slim
- ✅ **Build optimizations**: Multi-stage build, layer caching
- ✅ **Dependency verification**: `go mod verify`
- ✅ **Build flags**: `-trimpath`, `-buildvcs=false` for reproducible builds
- ✅ **Disk space optimization**: Disabled build cache (`GOCACHE=off`) to reduce disk usage

#### Features
- ✅ **CVE loader binary**: Included for database population
- ✅ **Migrations**: SQL migration files included
- ✅ **Healthcheck**: HTTP health check endpoint
- ✅ **Labels**: OCI labels for metadata

#### Build Arguments
```dockerfile
ARG FORTUNA_BUILD_VERSION=dev
ARG FORTUNA_BUILD_COMMIT=none
ARG FORTUNA_BUILD_TIME=unknown
```

---

### Agent Dockerfile (`agent/Dockerfile`)

#### Security Considerations
- ⚠️ **Root user**: Required for Docker socket access
  - **Mitigation**: Minimal capabilities, read-only root filesystem
  - **Note**: This is a known security trade-off for DaemonSet workloads
- ✅ **Minimal base image**: Debian Bookworm Slim
- ✅ **Build optimizations**: Multi-stage build, layer caching
- ✅ **Disk space optimization**: Disabled build cache (`GOCACHE=off`) to reduce disk usage

#### Features
- ✅ **Docker CLI**: Included via `docker.io` package for SBOM extraction
- ✅ **Healthcheck**: Process check
- ✅ **Labels**: OCI labels for metadata

---

## Deployment Manifests

### Core Deployment (`deploy/fortuna-core-deployment.yaml`)

#### Production-Ready Features
- ✅ **Image versioning**: Support for versioned tags
- ✅ **Image pull policy**: Configurable (`IfNotPresent` for production)
- ✅ **Security context**: Non-root user (UID 1000)
- ✅ **Resource limits**: Production-appropriate (1 CPU, 1Gi memory)
- ✅ **Health probes**: Liveness and readiness checks
- ✅ **Capabilities**: Drop all, minimal privileges

#### Configuration
```yaml
image: fortuna-core:latest  # Use versioned tag in production
imagePullPolicy: IfNotPresent  # or Always for production
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
```

---

### Agent DaemonSet (`deploy/fortuna-agent-daemonset.yaml`)

#### Production-Ready Features
- ✅ **Image versioning**: Support for versioned tags
- ✅ **Image pull policy**: Configurable (`IfNotPresent` for production)
- ✅ **Security context**: Root required (documented trade-off)
- ✅ **Resource limits**: Production-appropriate (500m CPU, 2Gi memory)
- ✅ **Minimal capabilities**: Only CHOWN, SETGID, SETUID

#### Configuration
```yaml
image: fortuna-agent:latest  # Use versioned tag in production
imagePullPolicy: IfNotPresent  # or Always for production
securityContext:
  runAsNonRoot: false  # Required for docker socket
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
    add:
      - CHOWN
      - SETGID
      - SETUID
```

---

## Building Images

### Local Development (Minikube)

```bash
# Set Docker environment for minikube
eval $(minikube docker-env)

# Build images from repository root
cd /path/to/fortuna
docker build -f core/Dockerfile -t fortuna-core:latest .
docker build -f agent/Dockerfile -t fortuna-agent:latest .

# Use in deployment with:
# imagePullPolicy: Never
```

**Note**: Build context must be the repository root (`.`), not the component directory.

### Production Build

```bash
# Using production build script
./scripts/build-production.sh

# With custom registry and version
DOCKER_REGISTRY=registry.example.com \
VERSION=v1.0.0 \
./scripts/build-production.sh

# Push to registry
PUSH_IMAGES=true ./scripts/build-production.sh
```

### Build Script Features

The `scripts/build-production.sh` script provides:
- ✅ **Version tagging**: Git tags or custom version
- ✅ **Build metadata**: Version, commit, build time
- ✅ **Multi-tag support**: Version tag + `latest`
- ✅ **Optional push**: Push to registry if `PUSH_IMAGES=true`
- ✅ **Build cache**: Uses BuildKit inline cache

---

## Image Versioning Strategy

### Recommended Approach

1. **Development**: Use `latest` tag with `imagePullPolicy: Never` (minikube)
2. **Staging**: Use versioned tags (e.g., `v1.0.0-rc1`) with `imagePullPolicy: IfNotPresent`
3. **Production**: Use semantic version tags (e.g., `v1.0.0`) with `imagePullPolicy: Always`

### Version Format

- **Semantic versioning**: `v1.0.0`, `v1.0.1`, `v2.0.0`
- **Pre-release**: `v1.0.0-rc1`, `v1.0.0-beta1`
- **Development**: `dev`, `latest`

---

## Security Best Practices

### Core Service
- ✅ Runs as non-root user (UID 1000)
- ✅ Minimal capabilities (drop ALL)
- ✅ Read-write root filesystem (for logs/temp files)
- ✅ No privilege escalation

### Agent Service
- ⚠️ Runs as root (required for Docker socket)
- ✅ Read-only root filesystem
- ✅ Minimal capabilities (CHOWN, SETGID, SETUID only)
- ✅ No privilege escalation
- ✅ Dedicated service account with minimal privileges

### Image Security
- ✅ Minimal base images (Debian Bookworm Slim)
- ✅ Multi-stage builds (smaller final images)
- ✅ No unnecessary packages
- ✅ Regular base image updates
- ✅ Better compatibility (glibc vs musl libc)

---

## Production Deployment Checklist

### Pre-Deployment
- [ ] Build images with version tags
- [ ] Push images to registry
- [ ] Update deployment manifests with versioned image tags
- [ ] Set `imagePullPolicy: IfNotPresent` or `Always`
- [ ] Verify security contexts
- [ ] Test resource limits
- [ ] Verify health checks

### Deployment
- [ ] Create namespace
- [ ] Create secrets (TLS certificates)
- [ ] Apply RBAC
- [ ] Deploy infrastructure (PostgreSQL, NATS)
- [ ] Deploy Core
- [ ] Deploy Agent
- [ ] Verify pods are running
- [ ] Check logs for errors

### Post-Deployment
- [ ] Verify health endpoints
- [ ] Test SBOM extraction
- [ ] Test CVE matching
- [ ] Monitor resource usage
- [ ] Check security contexts
- [ ] Verify non-root execution (Core)

---

## Troubleshooting

### Image Pull Errors

**Problem**: `ErrImagePull` or `ImagePullBackOff`

**Solutions**:
1. Check image exists in registry
2. Verify image pull secrets (if using private registry)
3. Check network connectivity to registry
4. Verify `imagePullPolicy` setting

### Permission Errors

**Problem**: Permission denied errors in Core

**Solutions**:
1. Verify security context (runAsUser: 1000)
2. Check volume mount permissions
3. Verify file ownership in image

### Docker Socket Access (Agent)

**Problem**: Cannot access Docker socket

**Solutions**:
1. Verify Docker socket volume mount
2. Check security context (runAsNonRoot: false)
3. Verify socket path on host
4. Check SELinux/AppArmor policies

---

## Migration from Development to Production

### Step 1: Update Image Tags

```yaml
# Before (development)
image: fortuna-core:latest
imagePullPolicy: Never

# After (production)
image: registry.example.com/fortuna-core:v1.0.0
imagePullPolicy: IfNotPresent
```

### Step 2: Update Security Contexts

```yaml
# Core - already production-ready
securityContext:
  runAsNonRoot: true
  runAsUser: 1000

# Agent - already production-ready
securityContext:
  runAsNonRoot: false  # Required
  readOnlyRootFilesystem: true
```

### Step 3: Update Resource Limits

```yaml
# Adjust based on actual usage
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

---

## References

- [Dockerfile Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Kubernetes Security Contexts](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
- [Container Image Security](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy)

---

**Status**: ✅ **Production-Ready**

