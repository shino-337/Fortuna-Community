# Environment Preparation Guide

**Version**: 1.0  
**Last Updated**: 2026-01-05

---

## Overview

This guide covers preparing your Kubernetes environment for Fortuna deployment, including prerequisites, cluster setup, and required configurations.

---

## Prerequisites

### Kubernetes Cluster

- **Version**: v1.25+ (tested with v1.28)
- **Nodes**: Minimum 2 nodes (1 master, 1 worker)
- **CNI**: Flannel, Calico, or equivalent
- **Storage**: Local storage provisioner (local-path-provisioner) or equivalent

### Node Requirements

#### Master Node
- **CPU**: 2 cores minimum, 4 cores recommended
- **Memory**: 4GB minimum, 8GB recommended
- **Storage**: 50GB+ available
- **OS**: Linux (Ubuntu 20.04+, RHEL 8+, etc.)

#### Worker Nodes
- **CPU**: 2 cores minimum per node
- **Memory**: 4GB minimum per node
- **Storage**: 50GB+ available per node
- **OS**: Linux (Ubuntu 20.04+, RHEL 8+, etc.)

### Required Tools

Install the following tools on your deployment machine:

```bash
# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# nerdctl (if using containerd)
# Download from: https://github.com/containerd/nerdctl/releases

# ctr (containerd CLI, usually included with containerd)
# Verify: ctr version

# openssl (for certificate generation)
# Usually pre-installed on Linux
```

---

## Cluster Setup

### 1. Verify Cluster Access

```bash
kubectl cluster-info
kubectl get nodes
```

All nodes should be in `Ready` state.

### 2. Check Storage Class

Fortuna requires persistent storage. Verify storage class exists:

```bash
kubectl get storageclass
```

If using `local-path-provisioner`, verify it's installed:

```bash
kubectl get pods -n local-path-storage
```

If not installed, install local-path-provisioner:

```bash
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml
```

### 3. Verify Container Runtime

#### containerd

```bash
# Check containerd version
ctr version

# Check containerd service
sudo systemctl status containerd
```

#### Docker

```bash
# Check Docker version
docker version

# Check Docker service
sudo systemctl status docker
```

---

## Network Configuration

### 1. Verify CNI Plugin

```bash
# Check CNI pods
kubectl get pods -n kube-system | grep -E "(flannel|calico|weave)"

# Check network policies (if using)
kubectl get networkpolicies --all-namespaces
```

### 2. Test Inter-Node Communication

```bash
# Create test pod on master
kubectl run test-master --image=busybox --restart=Never --overrides='{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""}}}' -- sleep 3600

# Create test pod on worker
kubectl run test-worker --image=busybox --restart=Never --overrides='{"spec":{"nodeSelector":{"node-role.kubernetes.io/control-plane":""},"tolerations":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists","effect":"NoSchedule"}]}}' -- sleep 3600

# Test connectivity
kubectl exec test-master -- ping -c 3 test-worker
```

### 3. DNS Resolution

Verify CoreDNS is working:

```bash
kubectl get pods -n kube-system | grep coredns
kubectl run test-dns --image=busybox --restart=Never -- nslookup kubernetes.default
kubectl delete pod test-dns
```

---

## Security Configuration

### 1. RBAC Verification

Ensure RBAC is enabled:

```bash
kubectl api-resources | grep rbac
```

### 2. Service Account Setup

Fortuna will create its own service accounts. Verify you have permissions:

```bash
kubectl auth can-i create serviceaccounts --all-namespaces
kubectl auth can-i create clusterroles --all-namespaces
```

### 3. Network Policies (Optional)

If using network policies, ensure they allow:
- Core → PostgreSQL (port 5432)
- Core → NATS (port 4222)
- Agent → Core (port 9090)

---

## Storage Preparation

### 1. Check Available Storage

```bash
# Check node disk space
kubectl get nodes -o json | jq '.items[] | {name: .metadata.name, storage: .status.capacity."ephemeral-storage"}'
```

### 2. Verify PVC Support

Test PVC creation:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: local-path
  resources:
    requests:
      storage: 1Gi
EOF

# Wait for binding
kubectl wait --for=condition=Bound pvc/test-pvc --timeout=60s

# Cleanup
kubectl delete pvc test-pvc
```

---

## Namespace Preparation

### 1. Create Namespace

```bash
kubectl create namespace fortuna
```

### 2. Verify Namespace

```bash
kubectl get namespace fortuna
```

---

## Image Registry Configuration

### Option 1: Local Images (containerd)

If using local images with containerd:

```bash
# Verify containerd namespace
ctr -n k8s.io images list

# Set imagePullPolicy: Never in deployment files
```

### Option 2: Container Registry

If using a container registry:

```bash
# Create registry secret
kubectl create secret docker-registry regcred \
  --docker-server=<registry-url> \
  --docker-username=<username> \
  --docker-password=<password> \
  --docker-email=<email> \
  --namespace=fortuna

# Update deployment files to:
# - Use registry URL in image names
# - Set imagePullPolicy: IfNotPresent or Always
# - Add imagePullSecrets
```

---

## Pre-Deployment Checklist

- [ ] Kubernetes cluster v1.25+ running
- [ ] All nodes in Ready state
- [ ] Storage class available (local-path or equivalent)
- [ ] Container runtime (containerd/Docker) working
- [ ] CNI plugin installed and working
- [ ] DNS resolution working (CoreDNS)
- [ ] Inter-node communication verified
- [ ] RBAC enabled
- [ ] Sufficient resources (CPU, memory, storage)
- [ ] Namespace `fortuna` created
- [ ] kubectl configured and working
- [ ] Required tools installed (kubectl, nerdctl, openssl)
- [ ] Image registry configured (if using remote registry)

---

## Troubleshooting

### Nodes Not Ready

**Check**:
```bash
kubectl describe node <node-name>
kubectl get events --field-selector involvedObject.name=<node-name>
```

**Common Issues**:
- Container runtime not running
- CNI plugin not installed
- Insufficient resources

### Storage Class Not Found

**Solution**: Install local-path-provisioner or configure another storage class.

### DNS Not Working

**Check CoreDNS**:
```bash
kubectl get pods -n kube-system | grep coredns
kubectl logs -n kube-system <coredns-pod-name>
```

### Network Issues

**Test connectivity**:
```bash
# Test pod-to-pod
kubectl run test1 --image=busybox --restart=Never -- sleep 3600
kubectl run test2 --image=busybox --restart=Never -- sleep 3600
kubectl exec test1 -- ping -c 3 test2
```

---

## Next Steps

After completing environment preparation:

1. [Generate mTLS Certificates](../scripts/create_mtls_secret.sh)
2. [Build Images](BUILD_GUIDE.md)
3. [Deploy Infrastructure](PRODUCTION_DEPLOYMENT.md#infrastructure-deployment)
4. [Deploy Application](PRODUCTION_DEPLOYMENT.md#application-deployment)

---

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [containerd Documentation](https://containerd.io/docs/)
- [local-path-provisioner](https://github.com/rancher/local-path-provisioner)

