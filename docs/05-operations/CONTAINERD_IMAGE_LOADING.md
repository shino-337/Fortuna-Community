# Loading Images to Containerd for Kubernetes

## Problem

When using containerd as the container runtime, images built with Docker need to be imported into containerd's `k8s.io` namespace for Kubernetes to use them.

**Error:**
```
Warning  ErrImageNeverPull  Container image "fortuna-core:latest" is not present with pull policy of Never
```

## Solution

### Option 1: Use nerdctl (Recommended)

If `nerdctl` is installed:

```bash
# Build with nerdctl directly
nerdctl build -f core/Dockerfile -t fortuna-core:latest .
nerdctl build -f agent/Dockerfile -t fortuna-agent:latest .

# Images are automatically available to Kubernetes
```

### Option 2: Export from Docker and Import to Containerd

If images were built with Docker:

```bash
# Run the automated script
./scripts/load-images-to-containerd.sh

# Or manually:
# 1. Export from Docker
docker save fortuna-core:latest -o fortuna-core.tar
docker save fortuna-agent:latest -o fortuna-agent.tar

# 2. Import into containerd (k8s.io namespace)
ctr -n k8s.io images import fortuna-core.tar
ctr -n k8s.io images import fortuna-agent.tar

# 3. Verify
ctr -n k8s.io images ls | grep fortuna
```

### Option 3: Use nerdctl load

```bash
# Export from Docker
docker save fortuna-core:latest -o fortuna-core.tar
docker save fortuna-agent:latest -o fortuna-agent.tar

# Import with nerdctl
nerdctl load -i fortuna-core.tar
nerdctl load -i fortuna-agent.tar

# Verify
nerdctl images | grep fortuna
```

## Important: Namespace

Containerd uses namespaces. Kubernetes uses the `k8s.io` namespace. When importing with `ctr`, always use:

```bash
ctr -n k8s.io images import <image.tar>
```

When checking images:

```bash
ctr -n k8s.io images ls
```

## Verify Images are Available

```bash
# Check with ctr
ctr -n k8s.io images ls | grep fortuna

# Check with nerdctl
nerdctl images | grep fortuna

# Check with crictl (if available)
crictl images | grep fortuna
```

## Troubleshooting

### Image not found after import

1. **Check namespace:**
   ```bash
   # List all namespaces
   ctr namespaces ls
   
   # Check k8s.io namespace
   ctr -n k8s.io images ls | grep fortuna
   ```

2. **Re-import with correct namespace:**
   ```bash
   ctr -n k8s.io images import fortuna-core.tar
   ```

3. **Check image name:**
   ```bash
   # Images should be tagged as docker.io/library/fortuna-core:latest
   ctr -n k8s.io images ls
   ```

### Image exists but pod still can't find it

1. **Check imagePullPolicy:**
   ```yaml
   imagePullPolicy: Never  # For local images
   ```

2. **Verify image name matches:**
   ```bash
   # In deployment
   image: fortuna-core:latest
   
   # In containerd (should show)
   docker.io/library/fortuna-core:latest
   ```

3. **Restart kubelet (if needed):**
   ```bash
   sudo systemctl restart kubelet
   ```

## Best Practices

1. **Build directly with nerdctl** for containerd clusters:
   ```bash
   nerdctl build -f core/Dockerfile -t fortuna-core:latest .
   ```

2. **Use imagePullPolicy: Never** for local development

3. **Tag images consistently:**
   - Local: `fortuna-core:latest`
   - Registry: `registry.example.com/fortuna-core:v1.0.0`

4. **For production:** Push to registry and use `imagePullPolicy: IfNotPresent`

## References

- [Containerd Namespaces](https://github.com/containerd/containerd/blob/main/docs/namespaces.md)
- [nerdctl Documentation](https://github.com/containerd/nerdctl)

