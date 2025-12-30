# Containerd Build and Deployment Guide

**Date**: 2025-12-29  
**Version**: 1.0

---

## Overview

This guide covers building Fortuna images using `nerdctl` and importing them into `containerd` for Kubernetes clusters that use containerd (not Docker) as the container runtime.

---

## Prerequisites

### Required Tools

- **nerdctl**: Container runtime CLI for containerd
- **ctr**: Containerd CLI (usually included with containerd)
- **containerd**: Container runtime (should be running)

### Installation

#### Install nerdctl

```bash
# On Linux
wget https://github.com/containerd/nerdctl/releases/download/v1.7.0/nerdctl-1.7.0-linux-amd64.tar.gz
tar -xzf nerdctl-1.7.0-linux-amd64.tar.gz
sudo mv nerdctl /usr/local/bin/

# Verify
nerdctl version
```

#### Verify containerd

```bash
# Check containerd is running
sudo systemctl status containerd

# Check containerd socket
ls -l /run/containerd/containerd.sock

# Verify ctr
ctr version
```

---

## Building Images with nerdctl

### Quick Build

```bash
# Build both Core and Agent
./scripts/build-with-containerd.sh
```

### Build with Versioning

```bash
# Set version
VERSION=v1.0.0 ./scripts/build-with-containerd.sh

# Or with all build args
VERSION=v1.0.0 \
BUILD_COMMIT=$(git rev-parse --short HEAD) \
BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
./scripts/build-with-containerd.sh
```

### Export Images for Distribution

```bash
# Build and export images
EXPORT_IMAGES=true ./scripts/build-with-containerd.sh
```

Images will be exported to `/tmp/fortuna-*-${VERSION}.tar`

---

## Importing Images to Containerd

### Import from Tar File

```bash
# Using ctr (recommended)
./scripts/import-to-containerd.sh /path/to/fortuna-core-v1.0.0.tar
./scripts/import-to-containerd.sh /path/to/fortuna-agent-v1.0.0.tar

# Using nerdctl
./scripts/import-to-containerd.sh -m nerdctl /path/to/fortuna-core-v1.0.0.tar
```

### Import from Registry

```bash
# Pull and import using nerdctl
./scripts/import-to-containerd.sh -m nerdctl docker.io/fortuna/core:v1.0.0
```

### Import Multiple Images

```bash
# Import all tar files in directory
for file in /tmp/fortuna-*.tar; do
    ./scripts/import-to-containerd.sh "$file"
done
```

---

## Complete Workflow

### Build and Import (Single Node)

```bash
# Build and import in one step
./scripts/build-and-import-containerd.sh
```

### Build and Distribute (Multi-Node)

```bash
# Step 1: Build images
./scripts/build-with-containerd.sh

# Step 2: Export images
EXPORT_IMAGES=true ./scripts/build-with-containerd.sh

# Step 3: Copy to all nodes
./scripts/copy-containerd-images-to-nodes.sh
```

---

## Manual Commands

### Build with nerdctl

```bash
# Build Core
nerdctl build \
    -f core/Dockerfile \
    -t fortuna-core:latest \
    --namespace k8s.io \
    --build-arg FORTUNA_BUILD_VERSION=v1.0.0 \
    --build-arg FORTUNA_BUILD_COMMIT=$(git rev-parse --short HEAD) \
    --build-arg FORTUNA_BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
    .

# Build Agent
nerdctl build \
    -f agent/Dockerfile \
    -t fortuna-agent:latest \
    --namespace k8s.io \
    --build-arg FORTUNA_BUILD_VERSION=v1.0.0 \
    --build-arg FORTUNA_BUILD_COMMIT=$(git rev-parse --short HEAD) \
    --build-arg FORTUNA_BUILD_TIME=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
    .
```

### Export Images

```bash
# Export Core
nerdctl --namespace k8s.io save -o fortuna-core-v1.0.0.tar fortuna-core:v1.0.0

# Export Agent
nerdctl --namespace k8s.io save -o fortuna-agent-v1.0.0.tar fortuna-agent:v1.0.0
```

### Import with ctr

```bash
# Import Core
ctr -n k8s.io images import fortuna-core-v1.0.0.tar

# Import Agent
ctr -n k8s.io images import fortuna-agent-v1.0.0.tar
```

### Import with nerdctl

```bash
# Import Core
nerdctl --namespace k8s.io load -i fortuna-core-v1.0.0.tar

# Import Agent
nerdctl --namespace k8s.io load -i fortuna-agent-v1.0.0.tar
```

### List Images

```bash
# Using ctr
ctr -n k8s.io images ls | grep fortuna

# Using nerdctl
nerdctl --namespace k8s.io images ls | grep fortuna
```

---

## Multi-Node Cluster Distribution

### Method 1: Copy Files and Import

```bash
# On build node: Export images
EXPORT_IMAGES=true ./scripts/build-with-containerd.sh

# Copy to each node
for node in node1 node2 node3; do
    scp /tmp/fortuna-*.tar "$node:/tmp/"
    ssh "$node" "ctr -n k8s.io images import /tmp/fortuna-core-*.tar"
    ssh "$node" "ctr -n k8s.io images import /tmp/fortuna-agent-*.tar"
done
```

### Method 2: Using Script

```bash
# Automatically copy to all K8s nodes
./scripts/copy-containerd-images-to-nodes.sh
```

This script:
1. Exports images from current node
2. Copies to all cluster nodes via `kubectl cp`
3. Imports on each node using `ctr`
4. Verifies images on all nodes

---

## Verification

### Check Images in Containerd

```bash
# List all Fortuna images
ctr -n k8s.io images ls | grep fortuna

# Show image details
ctr -n k8s.io images ls fortuna-core:latest

# Inspect image
ctr -n k8s.io images inspect fortuna-core:latest
```

### Verify on All Nodes

```bash
# Check images on each node
for node in $(kubectl get nodes -o name); do
    echo "Node: $node"
    kubectl exec "$node" -- ctr -n k8s.io images ls | grep fortuna
done
```

---

## Kubernetes Deployment

### Update Deployment Manifests

After images are imported into containerd, update deployment manifests:

```yaml
# deploy/fortuna-core-deployment.yaml
spec:
  template:
    spec:
      containers:
      - name: core
        image: fortuna-core:latest  # or fortuna-core:v1.0.0
        imagePullPolicy: IfNotPresent  # or Never for local images
```

### Deploy

```bash
# Deploy with containerd images
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

---

## Troubleshooting

### Issue: nerdctl not found

**Solution:**
```bash
# Install nerdctl (see Prerequisites section)
# Or use ctr directly for import operations
```

### Issue: containerd.sock not found

**Solution:**
```bash
# Check containerd socket location
ls -l /run/containerd/containerd.sock
ls -l /var/run/containerd/containerd.sock

# Set CONTAINERD_ADDRESS if different location
export CONTAINERD_ADDRESS=/var/run/containerd/containerd.sock
```

### Issue: Permission denied

**Solution:**
```bash
# nerdctl/ctr may need sudo for some operations
sudo nerdctl --namespace k8s.io images ls

# Or add user to containerd group (if configured)
sudo usermod -aG containerd $USER
```

### Issue: Images not visible to Kubernetes

**Solution:**
```bash
# Ensure using correct namespace (k8s.io for Kubernetes)
ctr -n k8s.io images ls

# Verify image name matches deployment manifest
kubectl get deployment fortuna-core -o yaml | grep image:
```

### Issue: ImagePullBackOff in Kubernetes

**Solution:**
```bash
# Check image exists in containerd
ctr -n k8s.io images ls | grep fortuna-core

# Verify imagePullPolicy
kubectl get deployment fortuna-core -o yaml | grep imagePullPolicy

# Set to IfNotPresent or Never for local images
kubectl patch deployment fortuna-core -p '{"spec":{"template":{"spec":{"containers":[{"name":"core","imagePullPolicy":"IfNotPresent"}]}}}}'
```

---

## Comparison: Docker vs Containerd

| Operation | Docker | Containerd (nerdctl) |
|-----------|--------|---------------------|
| **Build** | `docker build` | `nerdctl build` |
| **List** | `docker images` | `nerdctl images ls` |
| **Export** | `docker save` | `nerdctl save` |
| **Import** | `docker load` | `nerdctl load` or `ctr images import` |
| **Namespace** | N/A | `--namespace k8s.io` |
| **Socket** | `/var/run/docker.sock` | `/run/containerd/containerd.sock` |

---

## Best Practices

### 1. Use Consistent Namespace

Always use `k8s.io` namespace for Kubernetes images:

```bash
nerdctl --namespace k8s.io build ...
ctr -n k8s.io images import ...
```

### 2. Version Your Images

```bash
VERSION=v1.0.0 ./scripts/build-with-containerd.sh
```

### 3. Export Before Distribution

```bash
EXPORT_IMAGES=true ./scripts/build-with-containerd.sh
```

### 4. Verify Before Deployment

```bash
ctr -n k8s.io images ls | grep fortuna
```

### 5. Use Appropriate imagePullPolicy

For local containerd images:
- `IfNotPresent`: Try to pull, use local if not found
- `Never`: Never pull, use local only

---

## Scripts Reference

| Script | Purpose |
|--------|---------|
| `build-with-containerd.sh` | Build images with nerdctl |
| `import-to-containerd.sh` | Import images to containerd |
| `build-and-import-containerd.sh` | Complete build + import workflow |
| `copy-containerd-images-to-nodes.sh` | Distribute images to all nodes |

---

## Quick Reference

### Build and Deploy (Single Node)

```bash
# Build
./scripts/build-with-containerd.sh

# Deploy
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

### Build and Deploy (Multi-Node)

```bash
# Build
./scripts/build-with-containerd.sh

# Distribute
./scripts/copy-containerd-images-to-nodes.sh

# Deploy
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

---

**Last Updated**: 2025-12-29  
**Status**: ✅ **Production-Ready**

