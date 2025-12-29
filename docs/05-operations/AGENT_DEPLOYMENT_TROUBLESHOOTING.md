# Agent Deployment Troubleshooting

## Common Issues

### Issue 1: Secret "fortuna-agent-client-tls" not found

**Error:**
```
Warning  FailedMount  MountVolume.SetUp failed for volume "fortuna-agent-client-tls" : 
secret "fortuna-agent-client-tls" not found
```

**Solution:**

Generate mTLS certificates and create secrets:

```bash
# Run the certificate generation script
./scripts/create-mtls-secrets.sh

# Or manually:
# 1. Generate certificates (see scripts/create-mtls-secrets.sh)
# 2. Create secret:
kubectl create secret generic fortuna-agent-client-tls \
    --from-file=tls.crt=.certs/client.crt \
    --from-file=tls.key=.certs/client.key \
    --from-file=ca.crt=.certs/ca.crt \
    -n fortuna
```

**Verify:**
```bash
kubectl get secrets -n fortuna | grep fortuna-agent-client-tls
```

---

### Issue 2: Docker socket not found

**Error:**
```
Warning  FailedMount  MountVolume.SetUp failed for volume "docker-socket" : 
hostPath type check failed: /var/run/docker.sock is not a socket file
```

**Cause:** Cluster is using containerd, not Docker.

**Solution:**

The Agent DaemonSet has been updated to only mount containerd socket. If you see this error, it means the old version is still deployed.

**Fix:**
```bash
# Delete old DaemonSet
kubectl delete daemonset fortuna-agent -n fortuna

# Apply updated DaemonSet
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

**Note:** The updated DaemonSet only mounts containerd socket (`/run/containerd/containerd.sock`), which is the standard for Kubernetes clusters using containerd runtime.

---

### Issue 3: Agent cannot connect to Core

**Symptoms:**
- Agent pods are running but not connecting to Core
- Logs show connection errors

**Check:**

1. **Core service is running:**
   ```bash
   kubectl get pods -n fortuna -l app.kubernetes.io/component=core
   kubectl get svc -n fortuna fortuna-core
   ```

2. **mTLS certificates are correct:**
   ```bash
   kubectl get secrets -n fortuna fortuna-agent-client-tls
   kubectl get secrets -n fortuna fortuna-core-server-tls
   ```

3. **Network connectivity:**
   ```bash
   # From agent pod
   kubectl exec -n fortuna -it <agent-pod-name> -- \
     nc -zv fortuna-core.fortuna.svc.cluster.local 9090
   ```

4. **Check Agent logs:**
   ```bash
   kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50
   ```

---

### Issue 4: Agent cannot access container runtime

**Symptoms:**
- Agent cannot extract SBOM from containers
- Permission denied errors

**Check:**

1. **Containerd socket exists on node:**
   ```bash
   # On the node
   ls -la /run/containerd/containerd.sock
   ```

2. **Agent has correct permissions:**
   - Agent runs as root (required for container runtime access)
   - Check security context in DaemonSet

3. **Service account has correct RBAC:**
   ```bash
   kubectl get clusterrole fortuna-agent -o yaml
   kubectl get serviceaccount fortuna-agent -n fortuna -o yaml
   ```

---

## Quick Fix Script

Run this to fix common issues:

```bash
#!/bin/bash
# Quick fix for Agent deployment issues

NAMESPACE="fortuna"

# 1. Create namespace if not exists
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# 2. Generate and create mTLS secrets
./scripts/create-mtls-secrets.sh

# 3. Delete and recreate Agent DaemonSet
kubectl delete daemonset fortuna-agent -n "$NAMESPACE" 2>/dev/null || true
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# 4. Wait for pods
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=agent -n "$NAMESPACE" --timeout=120s

# 5. Check status
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent
```

---

## Verification Checklist

After deployment, verify:

- [ ] Agent pods are running: `kubectl get pods -n fortuna -l app.kubernetes.io/component=agent`
- [ ] No mount errors: `kubectl describe pod -n fortuna -l app.kubernetes.io/component=agent`
- [ ] Secrets exist: `kubectl get secrets -n fortuna | grep fortuna`
- [ ] Agent can connect to Core: Check logs for connection success
- [ ] Agent can access containerd: Check logs for SBOM extraction

---

## Container Runtime Support

### Containerd (Default)

The Agent DaemonSet is configured for containerd runtime:
- Mounts: `/run/containerd/containerd.sock`
- Used by: Most Kubernetes clusters (kubeadm, etc.)

### Docker (Legacy)

If your cluster uses Docker:
- The Agent will attempt to use Docker CLI if available
- Containerd socket is still mounted (for compatibility)

---

## References

- [Agent Configuration](../03-components/AGENT.md)
- [mTLS Setup](../02-architecture/SECURITY.md)
- [Container Runtime Access](../02-architecture/AGENT_ARCHITECTURE.md)

