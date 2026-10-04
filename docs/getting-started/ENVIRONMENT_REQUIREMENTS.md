# Environment Requirements

This document summarizes the hardware, software, storage, network, and security requirements for deploying Fortuna.

## Quick Summary

| Component | Minimum | Recommended | Production |
|-----------|---------|-------------|------------|
| **Nodes** | 1 (lab; the [kind demo](DEMO.md) uses one node) | 3 (1 master + 2 workers) | 5+ (1 master + 4+ workers) |
| **CPU** | 4 cores total | 8 cores total | 16+ cores total |
| **RAM** | 8GB total | 16GB total | 32GB+ total |
| **Disk** | 40GB total | 100GB total | 500GB+ total |
| **Network** | 1 Gbps | 1 Gbps | 10 Gbps |

---

## Hardware Requirements

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

## Storage Requirements

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

## Network Requirements

### Cluster Network

- **Pod and Service CIDRs**: any; Fortuna has no CIDR requirement of its own
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

## Software Requirements

### Operating System

| OS | Version | Status |
|----|---------|--------|
| **Ubuntu** | 20.04 LTS | Recommended |
| **Ubuntu** | 22.04 LTS | Recommended |
| **Debian** | 11+ | Supported; may need adjustments |
| **RHEL/CentOS** | 8+ | Supported; may need adjustments |

### Container Runtime

| Runtime | Version | Status |
|---------|--------|--------|
| **containerd** | 1.7+ | Required for SBOM extraction |
| **CRI-O**, Docker (cri-dockerd) | — | Inventory, RBAC paths and Pod Detail work; SBOM extraction does not |

**Note**: the Agent reads images from the node's containerd socket (`/run/containerd/containerd.sock`) through its `image-export` sidecar, falling back to anonymous registry pulls.

### Kubernetes

| Component | Version | Status |
|-----------|--------|--------|
| **Kubernetes** | 1.28+ | Required; any distribution (kubeadm, kind, managed) |
| **kubectl** | 1.28+ | Required |
| **metrics-server** | any | Optional; Pod Detail shows CPU and memory usage only when it is installed |

**CNI**: any CNI works for Fortuna itself. Use one that **enforces NetworkPolicy** (Calico, Cilium) for anything beyond a lab: NATS has no client authentication, and `deploy/infrastructure/network-policies.yaml` is what limits it and PostgreSQL to Core. Flannel does not enforce NetworkPolicy.

### Build Tools (Optional - for building images)

| Tool | Version | Purpose |
|------|--------|---------|
| **Go** | 1.26.8 (the version CI pins) | Build and test Fortuna components |
| **nerdctl**, Docker or buildctl | — | Build container images |
| **Node.js** | 24 | Dashboard development only |
| **Git** | Latest | Clone repository |

---

## Security Requirements

### Certificates

- **mTLS Certificates**: Required for Agent↔Core communication
  - CA certificate
  - Server certificate (Core)
  - Client certificate (Agent)

### RBAC

`deploy/fortuna-rbac.yaml` creates:

- **Core**: read-only, cluster-wide (pods, services, namespaces, ServiceAccounts, RBAC objects, NetworkPolicies). ServiceAccount revocation does not use this identity; it uses a per-cluster kubeconfig you provide ([details](../operations/SERVICEACCOUNT_MUTATIONS.md)).
- **Agent**: cluster-wide reads of pods, nodes, namespaces, ServiceAccounts, workloads, events and RBAC objects, plus read access to pod metrics (`metrics.k8s.io`). It cannot exec into pods. On the node it reads host `/proc` and the Falco log, with no host namespaces and no added capabilities; only its credential-less `image-export` sidecar mounts the containerd socket, which is root-equivalent on that node. See [Agent privileges](../reference/SECURITY.md#agent-privileges).

### Network Security

- **Pod-to-Pod encryption**: Optional (via CNI plugin)
- **Service mesh**: Optional (Istio, Linkerd)
- **Network policies**: required outside a lab (see CNI above)

---

## Resource Limits

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

**Note**: Agent runs as DaemonSet (one per node). Most of the Agent's memory goes to SBOM extraction and container image analysis. The bundled manifest sets a 6Gi limit and `SBOM_WORKERS=1`; keep both when the Falco reader is enabled. The built-in eBPF sensor is an experimental scaffold and is disabled by default.

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

## Pre-Deployment Checklist

### Hardware
- [ ] Master node meets minimum requirements
- [ ] Worker nodes meet minimum requirements
- [ ] Sufficient storage available
- [ ] Network connectivity between nodes

### Software
- [ ] OS installed and updated
- [ ] containerd installed and configured
- [ ] Kubernetes components installed
- [ ] CNI plugin ready and enforcing NetworkPolicy

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

## Deployment Scenarios

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

## Related Documentation

- [Install on a cluster](QUICKSTART.md)
- [Production deployment](../operations/PRODUCTION_DEPLOYMENT.md)
- [Architecture](../architecture/ARCHITECTURE.md)
