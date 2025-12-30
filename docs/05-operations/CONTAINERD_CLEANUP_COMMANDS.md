# Containerd Image Cleanup Commands

**Date**: 2025-12-29  
**Quick Reference Guide**

---

## Quick Cleanup

### Clean All Fortuna Images

```bash
# Using script (recommended)
./scripts/clean-containerd-images.sh

# Or quick script
./scripts/clean-all-containerd-images.sh
```

### Manual Commands

```bash
# List all Fortuna images
ctr -n k8s.io images ls | grep -E "fortuna|ksam"

# Delete all Fortuna images (one command)
ctr -n k8s.io images ls -q | grep -E "fortuna|ksam" | xargs -r ctr -n k8s.io images rm

# Delete specific image
ctr -n k8s.io images rm fortuna-core:latest
ctr -n k8s.io images rm fortuna-agent:latest

# Delete by pattern
ctr -n k8s.io images ls -q | grep "fortuna-core" | xargs -r ctr -n k8s.io images rm
ctr -n k8s.io images ls -q | grep "fortuna-agent" | xargs -r ctr -n k8s.io images rm
```

---

## Detailed Cleanup Script

### Usage

```bash
# Dry run (show what would be deleted)
./scripts/clean-containerd-images.sh --dry-run

# Clean all fortuna images
./scripts/clean-containerd-images.sh

# Clean with custom prefix
./scripts/clean-containerd-images.sh --prefix ksam

# Clean all images (dangerous!)
./scripts/clean-containerd-images.sh --all
```

### Options

- `--dry-run`: Show what would be deleted without deleting
- `--prefix PREFIX`: Image name prefix (default: fortuna)
- `--namespace NAMESPACE`: Containerd namespace (default: k8s.io)
- `--all`: Delete ALL images (not just fortuna)

---

## Step-by-Step Cleanup

### Step 1: List Images

```bash
# List all images
ctr -n k8s.io images ls

# List only Fortuna images
ctr -n k8s.io images ls | grep -E "fortuna|ksam"
```

### Step 2: Delete Images

```bash
# Delete by name
ctr -n k8s.io images rm fortuna-core:latest
ctr -n k8s.io images rm fortuna-core:v1.0.0
ctr -n k8s.io images rm fortuna-agent:latest

# Delete all versions of an image
ctr -n k8s.io images ls -q | grep "fortuna-core" | xargs -r ctr -n k8s.io images rm
```

### Step 3: Verify

```bash
# Check remaining images
ctr -n k8s.io images ls | grep -E "fortuna|ksam"
```

---

## Complete Cleanup Before Deploy

### Option 1: Using Script

```bash
# Clean all old images
./scripts/clean-containerd-images.sh

# Build new images
./scripts/build-with-containerd.sh

# Deploy
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

### Option 2: Manual Commands

```bash
# 1. Delete all Fortuna images
ctr -n k8s.io images ls -q | grep -E "fortuna|ksam" | xargs -r ctr -n k8s.io images rm

# 2. Build new images
./scripts/build-with-containerd.sh

# 3. Verify new images
ctr -n k8s.io images ls | grep fortuna

# 4. Deploy
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

---

## Cleanup Specific Versions

```bash
# Delete specific version
ctr -n k8s.io images rm fortuna-core:v1.0.0
ctr -n k8s.io images rm fortuna-agent:v1.0.0

# Delete all except latest
ctr -n k8s.io images ls -q | grep "fortuna-core" | grep -v "latest" | xargs -r ctr -n k8s.io images rm
```

---

## Cleanup All Images (Nuclear Option)

```bash
# ⚠️ WARNING: This deletes ALL images in k8s.io namespace
ctr -n k8s.io images ls -q | xargs -r ctr -n k8s.io images rm

# Or using script with confirmation
./scripts/clean-containerd-images.sh --all
```

---

## Troubleshooting

### Issue: Image still in use

**Error:** `image is in use`

**Solution:**
```bash
# Stop pods using the image first
kubectl delete deployment fortuna-core -n fortuna
kubectl delete daemonset fortuna-agent -n fortuna

# Wait for pods to terminate
kubectl wait --for=delete pod -n fortuna -l app.kubernetes.io/component=core --timeout=60s

# Then delete images
ctr -n k8s.io images rm fortuna-core:latest
```

### Issue: Permission denied

**Solution:**
```bash
# Use sudo if needed
sudo ctr -n k8s.io images rm fortuna-core:latest
```

### Issue: Image not found

**Solution:**
```bash
# Check if image exists
ctr -n k8s.io images ls | grep fortuna

# Check namespace
ctr -n k8s.io images ls
ctr images ls  # default namespace
```

---

## Quick Reference

| Command | Purpose |
|---------|---------|
| `ctr -n k8s.io images ls` | List all images |
| `ctr -n k8s.io images ls \| grep fortuna` | List Fortuna images |
| `ctr -n k8s.io images rm <image>` | Delete specific image |
| `ctr -n k8s.io images ls -q \| grep fortuna \| xargs -r ctr -n k8s.io images rm` | Delete all Fortuna images |

---

**Last Updated**: 2025-12-29

