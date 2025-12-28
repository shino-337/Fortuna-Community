# Dockerfile Build Context Configuration

## Overview

Dockerfiles for Core and Agent have been updated to support building from the repository root with containerd/buildkit. This ensures proper handling of the Go workspace (`go.work`) and module dependencies.

**Base Images**: 
- Builder: `golang:1.24` (Debian-based)
- Runtime: `debian:bookworm-slim`

## Changes Made

### 1. Build Context
- **Before**: Build context was the component directory (`core/` or `agent/`)
- **After**: Build context is the repository root (`.`)
- **Dockerfile Location**: Still in `core/Dockerfile` and `agent/Dockerfile`
- **Build Command**: `docker build -f core/Dockerfile -t fortuna-core:latest .`

### 2. Go Workspace Support
- Note: `go.work` is NOT copied to avoid conflicts (references both core and agent)
- Module resolution uses `replace` directives in `go.mod` instead
- This ensures builds work correctly when only one component is needed

### 3. Module Dependencies
- API module is copied first: `COPY api/ ./api/`
- Component go.mod/go.sum copied before source code for better Docker layer caching
- Dependencies downloaded before copying source code

### 4. CVE Loader Handling
- CVE loader binary is built conditionally (if `cmd/cve-loader/main.go` exists)
- Empty file created if CVE loader doesn't exist to ensure COPY succeeds
- Final stage checks if file has content before keeping it

### 5. Base Image Changes
- **Builder**: `golang:1.24` (Debian-based, not Alpine)
- **Runtime**: `debian:bookworm-slim` (changed from Alpine 3.20)
- **Package Manager**: `apt-get` (instead of `apk`)
- **Benefits**: Better compatibility (glibc), more packages available
- **Trade-off**: Larger image size (~80MB vs ~5MB for Alpine)

## Build Instructions

### Using Docker

```bash
# Build Core from repo root
docker build -f core/Dockerfile -t fortuna-core:latest .

# Build Agent from repo root
docker build -f agent/Dockerfile -t fortuna-agent:latest .
```

### Using containerd/buildkit

```bash
# Build Core
buildctl build \
  --frontend dockerfile.v0 \
  --local context=. \
  --local dockerfile=core \
  --output type=image,name=fortuna-core:latest

# Build Agent
buildctl build \
  --frontend dockerfile.v0 \
  --local context=. \
  --local dockerfile=agent \
  --output type=image,name=fortuna-agent:latest
```

### Using Deployment Script

The `scripts/deploy-fortuna.sh` script automatically builds from the repository root:

```bash
# Build and deploy
./scripts/deploy-fortuna.sh

# Skip build (use existing images)
./scripts/deploy-fortuna.sh --skip-build

# Build with custom registry
REGISTRY=docker.io/username ./scripts/deploy-fortuna.sh
```

## File Structure

```
.
├── go.work              # Go workspace file
├── api/                 # Shared API module
├── core/
│   ├── Dockerfile       # Builds from repo root
│   ├── go.mod          # Has: replace github.com/fortuna/api => ../api
│   └── ...
├── agent/
│   ├── Dockerfile      # Builds from repo root
│   ├── go.mod          # Has: replace github.com/fortuna/api => ../api
│   └── ...
└── scripts/
    └── deploy-fortuna.sh  # Builds from repo root
```

## Key Points

1. **Build Context**: Always the repository root (`.`)
2. **Dockerfile Path**: Specified with `-f` flag
3. **Module Resolution**: Uses `go.work` and `replace` directives
4. **Layer Caching**: Dependencies downloaded before source code copy
5. **Conditional Builds**: CVE loader built only if source exists

## Troubleshooting

### Error: "api/ directory not found"
- Ensure you're building from the repository root
- Check that `api/` directory exists

### Error: "go.work not found"
- Go workspace file is NOT used in Docker builds (intentionally)
- Module resolution uses `replace` directives in `go.mod` instead
- This avoids build errors when only one component directory is present

### Error: "module github.com/fortuna/api not found"
- Ensure `replace` directive exists in `go.mod`
- Check that `api/` is copied before building

### Build fails with containerd
- Ensure build context is repository root
- Use `-f` flag to specify Dockerfile path
- Check that all required files are in the build context

## Verification

After building, verify the images:

```bash
# Check image size
docker images fortuna-core:latest fortuna-agent:latest

# Inspect image layers
docker history fortuna-core:latest

# Test image
docker run --rm fortuna-core:latest --version
```

