# Fortuna - Environment Requirements

**Complete specification of hardware, software, and network requirements for Fortuna deployment**

---

## 📋 Quick Summary

| Component | Minimum | Recommended | Production |
|-----------|---------|-------------|------------|
| **Nodes** | 2 (1 master + 1 worker) | 3 (1 master + 2 workers) | 5+ (1 master + 4+ workers) |
| **CPU** | 4 cores total | 8 cores total | 16+ cores total |
| **RAM** | 8GB total | 16GB total | 32GB+ total |
| **Disk** | 40GB total | 100GB total | 500GB+ total |
| **Network** | 1 Gbps | 1 Gbps | 10 Gbps |

---

## 🖥️ Hardware Requirements

### Master Node (Control Plane)

| Component | Minimum | Recommended | Production |
|-----------|---------|-------------|------------|
| **CPU** | 2 cores | 4 cores | 8+ cores |
| **RAM** | 4GB | 8GB | 16GB+ |
| **Disk** | 20GB SSD | 50GB SSD | 100GB+ SSD |
| **Network** | 1 Gbps | 1 Gbps | 10 Gbps |

**Purpose**:
- Kubernetes API server
- etcd (cluster state)
- kube-scheduler
- kube-controller-manager
- CoreDNS
- Fortuna Core (optional, can run on workers)

### Worker Nodes

| Component | Minimum | Recommended | Production |
|-----------|---------|-------------|------------|
| **CPU** | 2 cores | 4 cores | 8+ cores |
| **RAM** | 4GB | 8GB | 16GB+ |
| **Disk** | 20GB SSD | 50GB SSD | 100GB+ SSD |
| **Network** | 1 Gbps | 1 Gbps | 10 Gbps |

**Purpose**:
- Run application pods
- Fortuna Agent (DaemonSet - one per node)
- Fortuna Core (can run here)
- PostgreSQL (stateful)
- NATS JetStream (stateful)

**Number of Workers**:
- **Minimum**: 1 worker
- **Recommended**: 2-3 workers
- **Production**: 4+ workers (for high availability)

---

## 💾 Storage Requirements

### Persistent Volumes

| Component | Storage | Type | Access Mode |
|-----------|---------|------|-------------|
| **PostgreSQL** | 20GB+ | SSD | ReadWriteOnce |
| **NATS JetStream** | 10GB+ per replica | SSD | ReadWriteOnce |
| **Container Images** | ~2GB | Any | N/A |
| **Logs** | 5GB+ | Any | N/A |

**Total Storage**:
- **Minimum**: 40GB
- **Recommended**: 100GB+
- **Production**: 500GB+

### Storage Classes

Fortuna requires a StorageClass that supports:
- **ReadWriteOnce** (RWO) for PostgreSQL and NATS
- **SSD** recommended for performance
- **Dynamic provisioning** preferred

---

## 🌐 Network Requirements

### Cluster Network

- **Pod Network CIDR**: `10.244.0.0/16` (default for Flannel)
- **Service CIDR**: `10.96.0.0/12` (default)
- **Node Network**: All nodes must be on same network or routable

### Port Requirements

#### Master Node
| Port | Protocol | Purpose |
|------|----------|---------|
| 6443 | TCP | Kubernetes API server |
| 2379-2380 | TCP | etcd server client API |
| 10250 | TCP | kubelet API |
| 10259 | TCP | kube-scheduler |
| 10257 | TCP | kube-controller-manager |

#### Worker Nodes
| Port | TCP | Purpose |
|------|-----|---------|
| 10250 | TCP | kubelet API |
| 30000-32767 | TCP | NodePort services |

#### Fortuna Components
| Port | Protocol | Component | Purpose |
|------|----------|-----------|---------|
| 8080 | TCP | Core | HTTP API |
| 9090 | TCP | Core | gRPC (mTLS) |
| 5432 | TCP | PostgreSQL | Database |
| 4222 | TCP | NATS | Messaging |
| 6222 | TCP | NATS | Clustering |

### Network Policies

Fortuna requires:
- **Pod-to-Pod communication** within cluster
- **Service discovery** via DNS
- **External access** to Core API (optional, via LoadBalancer/Ingress)

---

## 🐧 Software Requirements

### Operating System

| OS | Version | Status |
|----|---------|--------|
| **Ubuntu** | 20.04 LTS | ✅ Recommended |
| **Ubuntu** | 22.04 LTS | ✅ Recommended |
| **Debian** | 11+ | ⚠️ Supported (may need adjustments) |
| **RHEL/CentOS** | 8+ | ⚠️ Supported (may need adjustments) |

### Container Runtime

| Runtime | Version | Status |
|---------|--------|--------|
| **containerd** | 1.7+ | ✅ Recommended |
| **Docker** | 20.10+ | ✅ Supported |
| **CRI-O** | 1.28+ | ⚠️ Supported (may need adjustments) |

**Note**: Fortuna Agent requires access to container runtime socket for SBOM extraction:
- **containerd**: `/run/containerd/containerd.sock`
- **Docker**: `/var/run/docker.sock`

### Kubernetes

| Component | Version | Status |
|-----------|--------|--------|
| **kubeadm** | 1.28+ | ✅ Required |
| **kubelet** | 1.28+ | ✅ Required |
| **kubectl** | 1.28+ | ✅ Required |

**CNI Plugins**:
- **Flannel**: ✅ Recommended (default)
- **Calico**: ✅ Supported
- **Cilium**: ✅ Supported
- **Weave**: ✅ Supported

### Build Tools (Optional - for building images)

| Tool | Version | Purpose |
|------|--------|---------|
| **Go** | 1.24+ | Build Fortuna components |
| **Docker** | 20.10+ | Build container images |
| **Git** | Latest | Clone repository |
| **Make** | Latest | Build automation |

---

## 🔐 Security Requirements

### Certificates

- **mTLS Certificates**: Required for Agent↔Core communication
  - CA certificate
  - Server certificate (Core)
  - Client certificate (Agent)
- **Kubernetes Certificates**: Managed by kubeadm

### RBAC

Fortuna requires:
- **ServiceAccount** for Core (read-only cluster access)
- **ServiceAccount** for Agent (read-only pod access on local node)
- **ClusterRole** and **ClusterRoleBinding** for Core
- **ClusterRole** and **ClusterRoleBinding** for Agent

### Network Security

- **Pod-to-Pod encryption**: Optional (via CNI plugin)
- **Service mesh**: Optional (Istio, Linkerd)
- **Network policies**: Optional (for pod isolation)

---

## 📊 Resource Limits

### Fortuna Core

| Resource | Request | Limit |
|----------|---------|-------|
| **CPU** | 100m | 1000m |
| **Memory** | 256Mi | 1Gi |

### Fortuna Agent

| Resource | Request | Limit |
|----------|---------|-------|
| **CPU** | 100m | 1000m |
| **Memory** | 1Gi | 6Gi |

**Note**: Agent runs as DaemonSet (one per node). The Agent requires significant memory for eBPF-based runtime monitoring, SBOM extraction, and container image analysis. When Falco JSONL tail reader is enabled, ensure 6Gi limit and set `SBOM_WORKERS=1`.

### PostgreSQL

| Resource | Request | Limit |
|----------|---------|-------|
| **CPU** | 200m | 1000m |
| **Memory** | 512Mi | 2Gi |
| **Storage** | 20Gi | - |

### NATS JetStream

| Resource | Request | Limit |
|----------|---------|-------|
| **CPU** | 100m | 500m |
| **Memory** | 256Mi | 2Gi |
| **Storage** | 10Gi per replica | - |

---

## ✅ Pre-Deployment Checklist

### Hardware
- [ ] Master node meets minimum requirements
- [ ] Worker nodes meet minimum requirements
- [ ] Sufficient storage available
- [ ] Network connectivity between nodes

### Software
- [ ] OS installed and updated
- [ ] containerd installed and configured
- [ ] Kubernetes components installed
- [ ] CNI plugin ready

### Network
- [ ] All required ports open
- [ ] DNS resolution working
- [ ] Nodes can communicate
- [ ] Internet access (for pulling images)

### Security
- [ ] SSH access configured
- [ ] Firewall rules configured
- [ ] Certificate generation tools available
- [ ] RBAC ready

---

## 🚀 Deployment Scenarios

### Scenario 1: Development (Single Node)

- **Nodes**: 1 (master + worker combined)
- **Resources**: 4 cores, 8GB RAM, 40GB disk
- **Use Case**: Local development, testing
- **Limitations**: No high availability, limited scalability

### Scenario 2: Testing (Small Cluster)

- **Nodes**: 2 (1 master + 1 worker)
- **Resources**: 4 cores total, 8GB RAM total, 80GB disk total
- **Use Case**: Integration testing, staging
- **Limitations**: Single worker, no redundancy

### Scenario 3: Production (Standard)

- **Nodes**: 3 (1 master + 2 workers)
- **Resources**: 8 cores total, 16GB RAM total, 150GB disk total
- **Use Case**: Small to medium production workloads
- **Features**: Worker redundancy, basic HA

### Scenario 4: Production (High Availability)

- **Nodes**: 5+ (1 master + 4+ workers)
- **Resources**: 16+ cores total, 32GB+ RAM total, 500GB+ disk total
- **Use Case**: Large production workloads, high availability
- **Features**: Full redundancy, horizontal scaling

---

## 📖 Related Documentation

- [Multi-Node K8s Deployment Guide](./MULTI_NODE_K8S_DEPLOYMENT.md)
- [Complete Setup Guide](./COMPLETE_SETUP_GUIDE.md)
- [Architecture Documentation](../02-architecture/README.md)

---

**Environment Requirements v1.0.0**


