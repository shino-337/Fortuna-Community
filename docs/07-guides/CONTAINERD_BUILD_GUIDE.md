# Fortuna Build and Deployment Guide

**Last Updated**: 2026-04-13
**Status**: ✅ Production-Ready

---

## Overview

This guide covers building Fortuna images and loading them into containerd for Kubernetes clusters. The build system auto-detects the available tool: **nerdctl**, **docker**, or **buildctl**. Override with `BUILD_TOOL=docker|nerdctl|buildctl`.

---

## Prerequisites

### Required Tools (one of)

| Tool | Notes |
|------|-------|
| **Docker Engine** | Most common. Images built with `docker build`, imported to containerd via `ctr`. |
| **nerdctl + buildkitd** | Builds directly into containerd. Requires running buildkitd daemon. |
| **buildctl + buildkitd** | CLI-only BuildKit. Exports OCI tarballs, imports via `ctr`. |

### Always Required

- **ctr** (containerd CLI) — needed to import Docker-built images into containerd `k8s.io` namespace
- **kubectl** — Kubernetes CLI
- **containerd** — container runtime (running, socket at `/run/containerd/containerd.sock`)
- **Go 1.24+** — only if building outside Docker (not needed for container builds)

### Installation

#### Docker Engine

```bash
# See https://docs.docker.com/engine/install/
curl -fsSL https://get.docker.com | sh
docker info
```

#### nerdctl + buildkitd

```bash
wget https://github.com/containerd/nerdctl/releases/download/v1.7.0/nerdctl-1.7.0-linux-amd64.tar.gz
tar -xzf nerdctl-1.7.0-linux-amd64.tar.gz
sudo mv nerdctl /usr/local/bin/
nerdctl version

# buildkitd must be running for nerdctl build
sudo systemctl start buildkit  # or: sudo buildkitd &
```

#### Verify containerd

```bash
sudo systemctl status containerd
ls -l /run/containerd/containerd.sock
ctr version
```

---

## Quick Start

### Build All Images (auto-detect tool)

```bash
./scripts/build/build-and-load-containerd.sh
```

### Build with Docker (explicit)

```bash
BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh
```

### Build Individual Components

```bash
BUILD_CORE_ONLY=true ./scripts/build/build-and-load-containerd.sh
BUILD_AGENT_ONLY=true ./scripts/build/build-and-load-containerd.sh
./scripts/build/build-dashboard-containerd.sh    # dashboard only
```

### Build without Cache

```bash
NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
```

### Build for Production Registry

```bash
REGISTRY=registry.company.com/fortuna PUSH_IMAGES=true ./scripts/build/build-production.sh
```

---

## Build Tool Detection

The `build-and-load-containerd.sh` script auto-detects in order:

1. **nerdctl** — checks if `nerdctl` exists AND buildkitd is responding
2. **docker** — checks if `docker` exists AND daemon is running
3. **buildctl** — checks if `buildctl` exists AND a buildkitd socket is available

If you see the error `buildctl needs to be installed and buildkitd needs to be running`, it means nerdctl was found but buildkitd is not running. Solutions:

```bash
# Option 1: Start buildkitd
sudo systemctl start buildkit
# or: sudo buildkitd &

# Option 2: Use Docker instead
BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh

# Option 3: Use the pipeline script with Docker
BUILD_TOOL=docker ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
```

---

## How Docker → Containerd Import Works

When using Docker as the build tool:

1. `docker build` creates the image in Docker's local store
2. `docker save` exports the image as a tarball (piped)
3. `ctr -n k8s.io images import` imports it into containerd's `k8s.io` namespace
4. kubelet (using containerd CRI) can now see the image

This is equivalent to what nerdctl does internally when building with `--namespace k8s.io`.

---

## Multi-Node Distribution

For multi-node clusters, images must exist on every node that runs the workload.

### Using push-images-to-workers.sh (recommended)

```bash
# Configure (first time)
cp scripts/utils/push-images.config.example scripts/utils/push-images.config
# Edit push-images.config with node IPs and SSH credentials

# Push to all nodes
./scripts/utils/push-images-to-workers.sh

# Clean old images on remote nodes first
./scripts/utils/push-images-to-workers.sh --clean-remote
```

### Manual Distribution

```bash
# Export
EXPORT_IMAGES=true ./scripts/build/build-and-load-containerd.sh

# Copy and import on each node
for node in node1 node2; do
    scp /tmp/fortuna-images/fortuna-core-*.tar "$node:/tmp/"
    scp /tmp/fortuna-images/fortuna-agent-*.tar "$node:/tmp/"
    ssh "$node" "ctr -n k8s.io images import /tmp/fortuna-core-*.tar"
    ssh "$node" "ctr -n k8s.io images import /tmp/fortuna-agent-*.tar"
done
```

---

## Full Pipeline

### Single Command (Clean + Build + Deploy)

```bash
# Full pipeline with auto-detected build tool
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full

# Force Docker backend
BUILD_TOOL=docker ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full

# Interactive menu
./scripts/pipeline/full-clean-database-rebuild-deploy.sh
```

### Component-Only Rebuild

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-core
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-agent
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-dashboard
```

---

## Verification

### Check Images in Containerd

```bash
# Using ctr
ctr -n k8s.io images list | grep fortuna

# Using nerdctl
nerdctl --namespace k8s.io images | grep fortuna

# Check Docker (if used for build)
docker images | grep fortuna
```

### Verify Pods Use Correct Image

```bash
kubectl get pods -n fortuna -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `buildctl needs to be installed` | buildkitd not running. Start it or use `BUILD_TOOL=docker` |
| `ErrImageNeverPull` | Image not on node. Run `push-images-to-workers.sh` or set `imagePullPolicy: IfNotPresent` |
| `ctr not found` | Install containerd tools: `apt install containerd` |
| `permission denied` on containerd socket | Run with `sudo` or add user to containerd group |
| Images not visible to kubelet | Ensure images are in `k8s.io` namespace: `ctr -n k8s.io images list` |

---

## Comparison: Docker vs nerdctl vs buildctl

| Operation | Docker | nerdctl | buildctl |
|-----------|--------|---------|----------|
| **Build** | `docker build` | `nerdctl build` | `buildctl build` |
| **Daemon** | Docker Engine | buildkitd | buildkitd |
| **Output** | Docker store → ctr import | Direct to containerd | OCI tarball → ctr import |
| **List** | `docker images` | `nerdctl images` | N/A (use ctr) |
| **Export** | `docker save` | `nerdctl save` | `--output type=oci` |

---

## Scripts Reference

| Script | Purpose |
|--------|---------|
| `scripts/build/build-and-load-containerd.sh` | Main build script (auto-detects nerdctl/docker/buildctl) |
| `scripts/build/build-dashboard-containerd.sh` | Dashboard-only build |
| `scripts/build/build-production.sh` | Production builds with registry push |
| `scripts/pipeline/full-clean-database-rebuild-deploy.sh` | Full pipeline: clean → build → deploy |
| `scripts/utils/push-images-to-workers.sh` | Distribute images to cluster nodes |
