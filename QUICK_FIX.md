# Quick Fix Guide - Image Pull Errors

## Problem

```
Warning  ErrImageNeverPull  Container image "fortuna-core:latest" is not present with pull policy of Never
Warning  ErrImageNeverPull  Container image "fortuna-agent:latest" is not present with pull policy of Never
```

## Root Cause

Images exist in containerd but are in the **wrong namespace**. Kubernetes looks for images in the `k8s.io` namespace, but images might be in the default namespace.

## Quick Fix (One Command)

```bash
./scripts/copy-images-to-k8s-namespace.sh
```

This script will:
1. Check if images exist in default namespace
2. Export them
3. Import into `k8s.io` namespace
4. Verify they're available

## Manual Fix

If the script doesn't work, do it manually:

```bash
# 1. Check images in default namespace
ctr images ls | grep fortuna

# 2. Export images
ctr images export /tmp/fortuna-core.tar docker.io/library/fortuna-core:latest
ctr images export /tmp/fortuna-agent.tar docker.io/library/fortuna-agent:latest

# 3. Import into k8s.io namespace (IMPORTANT!)
ctr -n k8s.io images import /tmp/fortuna-core.tar
ctr -n k8s.io images import /tmp/fortuna-agent.tar

# 4. Verify
ctr -n k8s.io images ls | grep fortuna

# 5. Cleanup
rm -f /tmp/fortuna-*.tar
```

## Verify Fix

```bash
# Check images in k8s.io namespace
ctr -n k8s.io images ls | grep fortuna

# Should show:
# docker.io/library/fortuna-core:latest
# docker.io/library/fortuna-agent:latest

# Check pods (should start now)
kubectl get pods -n fortuna

# If still pending, restart pods
kubectl delete pod -n fortuna -l app.kubernetes.io/component=core
kubectl delete pod -n fortuna -l app.kubernetes.io/component=agent
```

## Complete Fix (All Issues)

If you have multiple issues, run the comprehensive fix:

```bash
./scripts/fix-all-deployment-issues.sh
```

This will fix:
- Missing secrets
- Missing images
- Deployment issues

## Troubleshooting

### Images still not found after import

1. **Check image name format:**
   ```bash
   ctr -n k8s.io images ls | grep fortuna
   ```
   Should show: `docker.io/library/fortuna-core:latest`

2. **Restart kubelet (if needed):**
   ```bash
   sudo systemctl restart kubelet
   ```

3. **Delete and recreate pods:**
   ```bash
   kubectl delete deployment fortuna-core -n fortuna
   kubectl delete daemonset fortuna-agent -n fortuna
   kubectl apply -f deploy/fortuna-core-deployment.yaml
   kubectl apply -f deploy/fortuna-agent-daemonset.yaml
   ```

### Multi-node cluster

If you have multiple nodes, you need to copy images to **each node**:

```bash
# On each node, run:
./scripts/copy-images-to-k8s-namespace.sh

# Or manually on each node:
ctr images export /tmp/fortuna-core.tar docker.io/library/fortuna-core:latest
ctr -n k8s.io images import /tmp/fortuna-core.tar
```

## Prevention

For future builds, build directly into k8s.io namespace:

```bash
# Build with nerdctl (automatically goes to k8s.io)
nerdctl build -f core/Dockerfile -t fortuna-core:latest .
nerdctl build -f agent/Dockerfile -t fortuna-agent:latest .

# Or import directly to k8s.io when using Docker
docker save fortuna-core:latest -o fortuna-core.tar
ctr -n k8s.io images import fortuna-core.tar
```

