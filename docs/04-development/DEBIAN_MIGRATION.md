# Migration from Alpine to Debian Slim

**Date**: 2025-12-28  
**Version**: 1.0

---

## Overview

Fortuna Dockerfiles have been migrated from Alpine Linux to Debian Bookworm Slim for better compatibility and stability in production environments.

---

## Why Debian Instead of Alpine?

### Benefits

1. **Better Compatibility**
   - Uses glibc (vs musl libc in Alpine)
   - Better compatibility with Go binaries and third-party libraries
   - Fewer runtime issues with dynamic linking

2. **Package Availability**
   - More packages available in Debian repositories
   - Easier to install additional tools if needed
   - Better support for Docker CLI and other tools

3. **Stability**
   - Debian is more widely used in production
   - Better tested with various tools and libraries
   - More predictable behavior

4. **Debugging**
   - Better tooling support
   - Easier to troubleshoot issues
   - More familiar environment for most developers

### Trade-offs

- **Larger Image Size**: ~80MB vs ~5MB for Alpine
- **Slightly Slower Build**: More packages to download
- **More Disk Usage**: Larger base image

---

## Changes Made

### Base Images

| Component | Before | After |
|-----------|--------|-------|
| Builder | `golang:1.24-alpine` | `golang:1.24` (Debian-based) |
| Runtime (Core) | `alpine:3.20` | `debian:bookworm-slim` |
| Runtime (Agent) | `alpine:3.20` | `debian:bookworm-slim` |

### Package Manager

| Aspect | Before | After |
|--------|--------|-------|
| Package Manager | `apk` | `apt-get` |
| Update Command | `apk update` | `apt-get update` |
| Install Command | `apk add --no-cache` | `apt-get install -y --no-install-recommends` |
| Cleanup | `rm -rf /var/cache/apk/*` | `apt-get clean && rm -rf /var/lib/apt/lists/*` |

### User Management

| Aspect | Before | After |
|--------|--------|-------|
| Create Group | `addgroup -g 1000 fortuna` | `groupadd -r -g 1000 fortuna` |
| Create User | `adduser -D -u 1000 -G fortuna fortuna` | `useradd -r -m -u 1000 -g fortuna fortuna` |

### Docker CLI (Agent)

| Aspect | Before | After |
|--------|--------|-------|
| Package | `docker-cli` | `docker.io` |
| Installation | `apk add --no-cache docker-cli` | `apt-get install -y --no-install-recommends docker.io` |

---

## Build Instructions

### From Repository Root

```bash
# Build Core
docker build -f core/Dockerfile -t fortuna-core:latest .

# Build Agent
docker build -f agent/Dockerfile -t fortuna-agent:latest .
```

### With Build Arguments

```bash
# Build Core with metadata
docker build \
    --build-arg FORTUNA_BUILD_VERSION=v1.0.0 \
    --build-arg FORTUNA_BUILD_COMMIT=$(git rev-parse --short HEAD) \
    --build-arg FORTUNA_BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    -f core/Dockerfile \
    -t fortuna-core:latest \
    .

# Build Agent with metadata
docker build \
    --build-arg FORTUNA_BUILD_VERSION=v1.0.0 \
    --build-arg FORTUNA_BUILD_COMMIT=$(git rev-parse --short HEAD) \
    --build-arg FORTUNA_BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    -f agent/Dockerfile \
    -t fortuna-agent:latest \
    .
```

---

## Image Size Comparison

| Image | Alpine | Debian Slim | Difference |
|-------|--------|-------------|------------|
| Base | ~5MB | ~80MB | +75MB |
| Core (final) | ~50-60MB | ~120-130MB | +70MB |
| Agent (final) | ~40-50MB | ~110-120MB | +70MB |

**Note**: The larger size is acceptable for better compatibility and stability.

---

## Network Retry Logic

Both Dockerfiles include retry logic for `apt-get update` to handle network issues:

```dockerfile
RUN for i in 1 2 3 4 5; do \
        echo "Attempt $i: Updating Debian packages..." && \
        apt-get update && break || \
        (echo "Attempt $i failed, retrying in 5 seconds..." && sleep 5); \
    done && \
    apt-get install -y --no-install-recommends ...
```

This ensures builds succeed even with temporary network issues.

---

## Disk Space Optimization

To handle disk space constraints during builds:

1. **Disabled Build Cache**: `GOCACHE=off` to prevent cache accumulation
2. **Rebuild All**: `-a` flag to rebuild all packages without cache
3. **Cleanup**: Automatic cleanup of Go caches after build

---

## Compatibility Notes

### Runtime Compatibility

- **glibc**: All binaries compiled with glibc (standard on Debian)
- **Dynamic Linking**: Better support for dynamically linked libraries
- **CGO**: Better compatibility if CGO is enabled in the future

### Build Compatibility

- **Go Toolchain**: Uses standard Debian-based Go image
- **Package Manager**: Standard `apt-get` commands
- **Build Tools**: Standard Debian build tools available

---

## Migration Checklist

If you're migrating from Alpine-based images:

- [ ] Update deployment manifests (if using specific image tags)
- [ ] Test builds with new base images
- [ ] Verify runtime behavior (especially with Docker CLI in Agent)
- [ ] Update CI/CD pipelines if they reference Alpine
- [ ] Update documentation references to Alpine
- [ ] Test in staging environment before production

---

## Troubleshooting

### Network Issues During Build

If `apt-get update` fails:

1. Check network connectivity
2. Retry the build (retry logic will handle temporary failures)
3. Use a proxy if behind a firewall:
   ```dockerfile
   RUN apt-get -o Acquire::http::proxy="http://proxy:port" update
   ```

### Disk Space Issues

If build fails with "no space left on device":

1. Clean up Docker system: `docker system prune -a`
2. Increase disk space for container/VM
3. Build images one at a time

### Package Not Found

If a package is not found:

1. Check Debian package name (may differ from Alpine)
2. Verify package exists in Debian Bookworm repositories
3. Use `apt-cache search <package>` to find alternatives

---

## References

- [Debian Bookworm Release Notes](https://www.debian.org/releases/bookworm/)
- [Dockerfile Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Alpine vs Debian Comparison](https://www.alpinelinux.org/about/)

---

**Status**: ✅ **Migration Complete**

