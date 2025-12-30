# Fortuna Core Node Placement Strategy

**Date**: 2025-12-28  
**Version**: 1.0

---

## Overview

This document describes the node placement strategy for Fortuna Core deployment in multi-node Kubernetes clusters.

---

## Design Decision

### Core Pod Placement: Worker Nodes Only

**Decision**: Fortuna Core should run on **worker nodes only**, not on master/control-plane nodes.

### Rationale

1. **Control Plane Isolation**
   - Master nodes are dedicated to Kubernetes control plane components
   - Running application workloads on master nodes can impact cluster stability
   - Control plane components (API server, etcd, scheduler) need dedicated resources

2. **Resource Availability**
   - Worker nodes typically have more resources allocated for application workloads
   - Master nodes are often smaller and optimized for control plane operations

3. **Best Practices**
   - Kubernetes best practices recommend isolating control plane from application workloads
   - Most production clusters have taints on master nodes to prevent scheduling

4. **Scalability**
   - Worker nodes can be scaled independently
   - Core can leverage worker node resources more effectively

---

## Implementation

### Current Deployment

The `fortuna-core-deployment.yaml` includes a `nodeSelector` to ensure Core runs on worker nodes:

```yaml
spec:
  nodeSelector:
    node-role.kubernetes.io/worker: "true"
```

### Node Labeling

For this to work, worker nodes must be labeled:

```bash
# Label worker nodes
kubectl label nodes <worker-node-name> node-role.kubernetes.io/worker=true

# Verify labels
kubectl get nodes --show-labels
```

### Alternative Configurations

#### Option 1: Explicit Worker Node Label (Recommended)

```yaml
nodeSelector:
  node-role.kubernetes.io/worker: "true"
```

**Requires**: Worker nodes labeled with `node-role.kubernetes.io/worker=true`

#### Option 2: Exclude Control Plane

```yaml
nodeSelector:
  node-role.kubernetes.io/control-plane: "false"
```

**Use when**: Worker nodes don't have explicit labels, but control-plane nodes are labeled

#### Option 3: No Node Selector (Development Only)

Remove `nodeSelector` entirely to allow scheduling on any node.

**Use when**: 
- Single-node cluster (minikube)
- Development/testing environment
- All nodes are schedulable

#### Option 4: Tolerations (Not Recommended)

If you must run Core on master nodes (not recommended):

```yaml
tolerations:
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
```

**Warning**: Only use in special circumstances. Not recommended for production.

---

## Cluster Setup

### Label Worker Nodes

```bash
# Get all nodes
kubectl get nodes

# Label each worker node
kubectl label nodes <node-name> node-role.kubernetes.io/worker=true

# Verify
kubectl get nodes -l node-role.kubernetes.io/worker=true
```

### Verify Master Node Taints

```bash
# Check if master nodes have taints
kubectl describe node <master-node-name> | grep Taints

# Typical output:
# Taints: node-role.kubernetes.io/control-plane:NoSchedule
```

---

## Deployment Scenarios

### Scenario 1: Standard Multi-Node Cluster

**Setup**:
- 1 master node (tainted)
- 2+ worker nodes (labeled)

**Configuration**:
```yaml
nodeSelector:
  node-role.kubernetes.io/worker: "true"
```

**Result**: Core runs on worker nodes only ✅

---

### Scenario 2: Single-Node Cluster (Minikube)

**Setup**:
- 1 node (both master and worker)

**Configuration**:
```yaml
# Remove nodeSelector or use:
nodeSelector: {}
```

**Result**: Core can run on the single node ✅

---

### Scenario 3: Development Cluster

**Setup**:
- Master nodes not tainted
- All nodes schedulable

**Configuration**:
```yaml
# No nodeSelector or explicit exclusion:
nodeSelector:
  node-role.kubernetes.io/control-plane: "false"
```

**Result**: Core avoids control-plane nodes if possible ✅

---

## Verification

### Check Pod Placement

```bash
# Check where Core pod is running
kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o wide

# Verify node type
kubectl get node <node-name> --show-labels
```

### Expected Output

```
NAME                           READY   STATUS    RESTARTS   AGE   IP           NODE
fortuna-core-xxxxx-xxxxx       1/1     Running   0          5m    10.244.1.5   worker-node-1
```

Node should be a worker node, not a master node.

---

## Troubleshooting

### Issue: Core Pod Pending

**Symptom**: Core pod stays in `Pending` state

**Possible Causes**:
1. No worker nodes labeled correctly
2. Worker nodes don't have enough resources
3. NodeSelector too restrictive

**Solutions**:

```bash
# Check pod events
kubectl describe pod -n fortuna -l app.kubernetes.io/component=core

# Check node labels
kubectl get nodes --show-labels

# Check node resources
kubectl describe node <node-name>
```

### Issue: Core Pod on Master Node

**Symptom**: Core pod scheduled on master/control-plane node

**Possible Causes**:
1. NodeSelector not configured
2. Master node not tainted
3. Worker nodes not labeled

**Solutions**:

```bash
# Add nodeSelector to deployment
kubectl patch deployment fortuna-core -n fortuna --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/nodeSelector", "value": {"node-role.kubernetes.io/worker": "true"}}]'

# Or delete and recreate with correct configuration
kubectl delete deployment fortuna-core -n fortuna
kubectl apply -f deploy/fortuna-core-deployment.yaml
```

---

## Agent vs Core Placement

### Agent (DaemonSet)
- **Runs on**: All nodes (master + workers)
- **Reason**: Needs to monitor pods on each node
- **Configuration**: No nodeSelector needed (DaemonSet runs everywhere)

### Core (Deployment)
- **Runs on**: Worker nodes only
- **Reason**: Central service, doesn't need node-specific access
- **Configuration**: Requires nodeSelector

---

## Summary

| Component | Type | Placement | NodeSelector |
|-----------|------|-----------|--------------|
| **Core** | Deployment | Worker nodes only | ✅ Required |
| **Agent** | DaemonSet | All nodes | ❌ Not needed |

---

## References

- [Kubernetes Node Selectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)
- [Control Plane Isolation](https://kubernetes.io/docs/setup/best-practices/multiple-zones/)
- [Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)

---

**Status**: ✅ **Implemented**

