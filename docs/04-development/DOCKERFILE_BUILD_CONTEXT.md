# Dockerfile Build Context Configuration

## Overview

Dockerfiles for Core and Agent have been updated to support building from the repository root with containerd/buildkit. This ensures proper handling of the Go workspace (`go.work`) and module dependencies.

## Changes Made

### 1. Build Context
- **Before**: Build context was the component directory (`core/` or `agent/`)
- **After**: Build context is the repository root (`.`)
- **Dockerfile Location**: Still in `core/Dockerfile` and `agent/Dockerfile`
- **Build Command**: `docker build -f core/Dockerfile -t fortuna-core:latest .`

### 2. Go Workspace Support
- Added `COPY go.work* ./` to support Go workspace files
- Ensures proper module resolution with `replace` directives

### 3. Module Dependencies
- API module is copied first: `COPY api/ ./api/`
- Component go.mod/go.sum copied before source code for better Docker layer caching
- Dependencies downloaded before copying source code

### 4. CVE Loader Handling
- CVE loader binary is built conditionally (if `cmd/cve-loader/main.go` exists)
- Empty file created if CVE loader doesn't exist to ensure COPY succeeds
- Final stage checks if file has content before keeping it

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
- Go workspace file is optional but recommended
- Create it with: `go work init core agent api`

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

