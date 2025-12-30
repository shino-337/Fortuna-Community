# DNS Troubleshooting Guide - Multi-Node K8s Cluster

**Date**: 2025-12-28  
**Version**: 1.0

---

## Overview

This guide addresses DNS resolution issues when deploying Fortuna on multi-node Kubernetes clusters (master + worker nodes).

---

## Common DNS Issues

### Issue 1: Service Name Mismatch

**Symptom**: Agent cannot connect to Core
```
Error: failed to connect to Core: dial tcp: lookup fortuna-core.fortuna.svc.cluster.local: no such host
```

**Root Cause**: Service name doesn't match what Agent is looking for

**Solution**: Ensure Service name matches Agent's `CORE_GRPC_ENDPOINT`

```yaml
# Service MUST be named "fortuna-core"
apiVersion: v1
kind: Service
metadata:
  name: fortuna-core  # ✅ Correct
  namespace: fortuna
```

**Agent Configuration**:
```yaml
env:
  - name: CORE_GRPC_ENDPOINT
    value: "fortuna-core.fortuna.svc.cluster.local:9090"  # ✅ Matches Service name
```

---

### Issue 2: Label Selector Mismatch

**Symptom**: Service has no endpoints (0/1 endpoints)

**Root Cause**: Service selector doesn't match Deployment pod labels

**Solution**: Use consistent labels

```yaml
# Service selector
spec:
  selector:
    app.kubernetes.io/name: fortuna
    app.kubernetes.io/component: core

# Deployment pod labels (MUST match)
template:
  metadata:
    labels:
      app.kubernetes.io/name: fortuna
      app.kubernetes.io/component: core
```

---

### Issue 3: Namespace Mismatch

**Symptom**: DNS resolution fails with "no such host"

**Root Cause**: Service and Agent in different namespaces

**Solution**: Ensure both are in the same namespace

```bash
# Verify namespace
kubectl get svc fortuna-core -n fortuna
kubectl get pods -l app.kubernetes.io/component=agent -n fortuna
```

---

### Issue 4: CoreDNS Not Ready

**Symptom**: DNS queries timeout or fail

**Root Cause**: CoreDNS pods not running or misconfigured

**Solution**: Check CoreDNS status

```bash
# Check CoreDNS pods
kubectl get pods -n kube-system -l k8s-app=kube-dns

# Check CoreDNS logs
kubectl logs -n kube-system -l k8s-app=kube-dns

# Verify CoreDNS service
kubectl get svc kube-dns -n kube-system
```

---

## DNS Resolution in Kubernetes

### Service DNS Format

```
<service-name>.<namespace>.svc.cluster.local
```

**Example**:
```
fortuna-core.fortuna.svc.cluster.local:9090
```

### Short Names

Within the same namespace, you can use short names:
```
fortuna-core:9090  # ✅ Works if Agent is in 'fortuna' namespace
```

**Best Practice**: Always use FQDN for cross-namespace or multi-node reliability

---

## Verification Steps

### Step 1: Verify Service Exists

```bash
# Check Service
kubectl get svc fortuna-core -n fortuna

# Expected output:
# NAME           TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
# fortuna-core   ClusterIP   10.96.123.45    <none>        8080/TCP,9090/TCP 5m
```

### Step 2: Verify Service Has Endpoints

```bash
# Check endpoints
kubectl get endpoints fortuna-core -n fortuna

# Expected output:
# NAME           ENDPOINTS                    AGE
# fortuna-core   10.244.1.5:8080,10.244.1.5:9090   5m
```

**If endpoints are empty**: Label selector mismatch

### Step 3: Verify Pod Labels Match Service Selector

```bash
# Get Core pod labels
kubectl get pods -n fortuna -l app.kubernetes.io/component=core --show-labels

# Expected output:
# NAME                            READY   STATUS    LABELS
# fortuna-core-xxxxx-xxxxx        1/1     Running   app.kubernetes.io/component=core,app.kubernetes.io/name=fortuna
```

### Step 4: Test DNS Resolution from Agent Pod

```bash
# Get Agent pod name
AGENT_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}')

# Test DNS resolution
kubectl exec -n fortuna $AGENT_POD -- nslookup fortuna-core.fortuna.svc.cluster.local

# Expected output:
# Server:         10.96.0.10
# Address:        10.96.0.10:53
# Name:   fortuna-core.fortuna.svc.cluster.local
# Address: 10.96.123.45
```

### Step 5: Test gRPC Connection

```bash
# Test connection from Agent pod
kubectl exec -n fortuna $AGENT_POD -- nc -zv fortuna-core.fortuna.svc.cluster.local 9090

# Expected output:
# fortuna-core.fortuna.svc.cluster.local (10.96.123.45:9090) open
```

---

## Multi-Node Cluster Specific Issues

### Issue: DNS Resolution Fails on Worker Nodes

**Symptom**: Agent on worker node cannot resolve Core service

**Possible Causes**:
1. CoreDNS pods only on master node
2. Network policies blocking DNS
3. CNI plugin issues

**Solutions**:

#### 1. Ensure CoreDNS Runs on All Nodes

```bash
# Check CoreDNS pod distribution
kubectl get pods -n kube-system -l k8s-app=kube-dns -o wide

# If CoreDNS only on master, add tolerations to DaemonSet
kubectl edit daemonset coredns -n kube-system
```

#### 2. Check Network Policies

```bash
# Check for network policies blocking DNS
kubectl get networkpolicies -n fortuna
kubectl get networkpolicies -n kube-system

# If policies exist, ensure they allow DNS (port 53 UDP)
```

#### 3. Verify CNI Plugin

```bash
# Check CNI pods
kubectl get pods -n kube-system | grep -E 'calico|flannel|weave|cilium'

# Check CNI logs
kubectl logs -n kube-system <cni-pod-name>
```

---

## Debugging Commands

### Check Service Details

```bash
kubectl describe svc fortuna-core -n fortuna
```

**Look for**:
- Endpoints: Should show Core pod IP
- Selector: Should match pod labels

### Check Pod Details

```bash
kubectl describe pod -n fortuna -l app.kubernetes.io/component=core
```

**Look for**:
- Labels: Should match Service selector
- Status: Should be Running
- IP: Should appear in Service endpoints

### Check DNS from Pod

```bash
# From Agent pod
kubectl exec -n fortuna <agent-pod> -- dig fortuna-core.fortuna.svc.cluster.local

# Or using nslookup
kubectl exec -n fortuna <agent-pod> -- nslookup fortuna-core.fortuna.svc.cluster.local
```

### Check CoreDNS Configuration

```bash
# Get CoreDNS config
kubectl get configmap coredns -n kube-system -o yaml

# Check CoreDNS logs
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=100
```

---

## Quick Fix Checklist

- [ ] Service name is `fortuna-core` (not `ksam-core`)
- [ ] Service namespace is `fortuna`
- [ ] Service selector matches Deployment pod labels
- [ ] Deployment pod labels are correct
- [ ] Agent `CORE_GRPC_ENDPOINT` uses FQDN: `fortuna-core.fortuna.svc.cluster.local:9090`
- [ ] Core pod is Running
- [ ] Service has endpoints (not 0/1)
- [ ] CoreDNS is running and healthy
- [ ] No network policies blocking DNS
- [ ] CNI plugin is working

---

## Common Fixes

### Fix 1: Update Service Name

```bash
# Delete old service
kubectl delete svc ksam-core -n fortuna

# Apply new service
kubectl apply -f deploy/fortuna-core-deployment.yaml
```

### Fix 2: Fix Label Selectors

```bash
# Patch Service selector
kubectl patch svc fortuna-core -n fortuna --type='json' \
  -p='[{"op": "replace", "path": "/spec/selector", "value": {"app.kubernetes.io/name": "fortuna", "app.kubernetes.io/component": "core"}}]'

# Restart Deployment to apply new labels
kubectl rollout restart deployment fortuna-core -n fortuna
```

### Fix 3: Restart CoreDNS

```bash
# Restart CoreDNS
kubectl rollout restart deployment coredns -n kube-system

# Or delete pods to force recreation
kubectl delete pods -n kube-system -l k8s-app=kube-dns
```

---

## Testing DNS Resolution

### From Agent Pod

```bash
# Get Agent pod
AGENT_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}')

# Test DNS
kubectl exec -n fortuna $AGENT_POD -- nslookup fortuna-core.fortuna.svc.cluster.local

# Test connection
kubectl exec -n fortuna $AGENT_POD -- nc -zv fortuna-core.fortuna.svc.cluster.local 9090
```

### From Core Pod

```bash
# Get Core pod
CORE_POD=$(kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')

# Test DNS
kubectl exec -n fortuna $CORE_POD -- nslookup nats.fortuna.svc.cluster.local
kubectl exec -n fortuna $CORE_POD -- nslookup postgres.fortuna.svc.cluster.local
```

---

## Summary

| Issue | Symptom | Fix |
|-------|---------|-----|
| **Service name mismatch** | "no such host" | Rename service to `fortuna-core` |
| **Label mismatch** | 0 endpoints | Match Service selector with pod labels |
| **Namespace mismatch** | DNS fails | Ensure same namespace |
| **CoreDNS not ready** | DNS timeout | Check CoreDNS pods |
| **Network policies** | Connection blocked | Allow DNS (port 53 UDP) |

---

## References

- [Kubernetes DNS](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/)
- [Service DNS](https://kubernetes.io/docs/concepts/services-networking/service/#dns)
- [CoreDNS Troubleshooting](https://coredns.io/plugins/kubernetes/)

---

**Status**: ✅ **Documented**

