# Multi-Node Image Deployment Guide

## Problem

In a multi-node Kubernetes cluster, images built on the master node are not automatically available on worker nodes. Pods on worker nodes will fail with:

```
Warning  ErrImageNeverPull  Container image "fortuna-core:latest" is not present with pull policy of Never
```

## Solution Options

### Option 1: Copy Images to Each Node (Quick Fix)

**For single worker node:**

```bash
# Copy images to worker node
./scripts/manual-copy-images-to-worker.sh k8s-worker01

# Or with IP
./scripts/manual-copy-images-to-worker.sh 192.168.1.101
```

**For multiple worker nodes:**

```bash
# Copy to all nodes (requires SSH access)
./scripts/copy-images-to-all-nodes.sh
```

**Manual steps:**

```bash
# On master node: Export images
ctr -n k8s.io images export /tmp/fortuna-core.tar docker.io/library/fortuna-core:latest
ctr -n k8s.io images export /tmp/fortuna-agent.tar docker.io/library/fortuna-agent:latest

# Copy to worker node
scp /tmp/fortuna-*.tar root@k8s-worker01:/tmp/

# On worker node: Import images
ssh root@k8s-worker01
ctr -n k8s.io images import /tmp/fortuna-core.tar
ctr -n k8s.io images import /tmp/fortuna-agent.tar
ctr -n k8s.io images ls | grep fortuna  # Verify
rm -f /tmp/fortuna-*.tar
```

### Option 2: Use Container Registry (Recommended for Production)

**Push images to registry:**

```bash
# Tag images
docker tag fortuna-core:latest <registry>/fortuna-core:latest
docker tag fortuna-agent:latest <registry>/fortuna-agent:latest

# Push to registry
docker push <registry>/fortuna-core:latest
docker push <registry>/fortuna-agent:latest
```

**Update deployments to use registry:**

```yaml
# In fortuna-core-deployment.yaml
image: <registry>/fortuna-core:latest
imagePullPolicy: IfNotPresent

# In fortuna-agent-daemonset.yaml
image: <registry>/fortuna-agent:latest
imagePullPolicy: IfNotPresent
```

### Option 3: Build on Each Node

Build images directly on each node:

```bash
# On each node
git clone <repository>
cd fortuna
docker build -f core/Dockerfile -t fortuna-core:latest .
docker build -f agent/Dockerfile -t fortuna-agent:latest .

# Import to containerd
docker save fortuna-core:latest -o fortuna-core.tar
docker save fortuna-agent:latest -o fortuna-agent.tar
ctr -n k8s.io images import fortuna-core.tar
ctr -n k8s.io images import fortuna-agent.tar
```

## Verification

After copying images, verify on each node:

```bash
# On each node
ctr -n k8s.io images ls | grep fortuna

# Should show:
# docker.io/library/fortuna-core:latest
# docker.io/library/fortuna-agent:latest
```

Then restart pods:

```bash
# Delete pods to force recreation
kubectl delete pod -n fortuna -l app.kubernetes.io/component=core
kubectl delete pod -n fortuna -l app.kubernetes.io/component=agent

# Check status
kubectl get pods -n fortuna -o wide
```

## Troubleshooting

### SSH Access Issues

If SSH is not configured:

1. **Setup SSH keys:**
   ```bash
   # On master node
   ssh-keygen -t rsa
   ssh-copy-id root@k8s-worker01
   ```

2. **Or use password:**
   ```bash
   # Will prompt for password
   scp /tmp/fortuna-*.tar root@k8s-worker01:/tmp/
   ```

### Images Still Not Found

1. **Check namespace:**
   ```bash
   # On worker node
   ctr -n k8s.io images ls | grep fortuna
   ```

2. **Check image name:**
   ```bash
   # Should match deployment
   ctr -n k8s.io images ls | grep "fortuna-core:latest"
   ```

3. **Restart kubelet:**
   ```bash
   # On worker node
   sudo systemctl restart kubelet
   ```

## Best Practices

1. **For Development:** Copy images manually to each node
2. **For Production:** Use container registry
3. **For CI/CD:** Build and push to registry automatically

## Quick Reference

```bash
# Export on master
ctr -n k8s.io images export /tmp/fortuna-core.tar docker.io/library/fortuna-core:latest
ctr -n k8s.io images export /tmp/fortuna-agent.tar docker.io/library/fortuna-agent:latest

# Copy to worker
scp /tmp/fortuna-*.tar root@<worker-node>:/tmp/

# Import on worker (SSH to worker first)
ctr -n k8s.io images import /tmp/fortuna-core.tar
ctr -n k8s.io images import /tmp/fortuna-agent.tar

# Verify
ctr -n k8s.io images ls | grep fortuna
```

