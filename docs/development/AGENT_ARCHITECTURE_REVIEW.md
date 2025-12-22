# Agent Architecture Review

**Date**: December 22, 2024  
**Purpose**: Review agent logic and prepare for mTLS deployment

---

## 🏗️ Agent Architecture

### Purpose
The KSAM Agent is a **DaemonSet** that runs on each Kubernetes node to:
1. Collect Kubernetes resources (Pods, ServiceAccounts, RBAC)
2. Send data to Core via gRPC with mTLS
3. Watch for resource changes in real-time
4. Provide node-level visibility

### Key Components

```
KSAM Agent (DaemonSet)
├── main.go                  ← Entry point
├── pkg/
│   ├── collector/           ← Resource collection
│   │   ├── pod_collector.go
│   │   ├── sa_collector.go
│   │   └── rbac_collector.go
│   ├── client/              ← gRPC client to Core
│   │   └── grpc_client.go
│   └── watcher/             ← Kubernetes watch
│       └── resource_watcher.go
└── config/                  ← Configuration
    └── config.go
```

---

## 🔐 mTLS Configuration

### Required Certificates

**For Agent**:
- `tls.crt` - Agent certificate (client cert)
- `tls.key` - Agent private key
- `ca.crt` - CA certificate (to verify Core)

**For Core**:
- `tls.crt` - Core certificate (server cert)
- `tls.key` - Core private key
- `ca.crt` - CA certificate (to verify Agent)

### Current Issue
❌ **Secret `ksam-agent-tls` not found in `fortuna` namespace**

### Solution
Generate new certificates for Fortuna deployment.

---

## 📊 Agent Decision: Deploy or Not?

### Option 1: Deploy Agent (Distributed Architecture) ✅ RECOMMENDED
**Pros**:
- Real-time resource collection from all nodes
- Distributed workload (each agent handles its node)
- Scalable (automatic per-node deployment)
- Node-level visibility
- Lower latency for resource updates

**Cons**:
- Requires mTLS setup
- More pods to manage (1 per node)
- Additional resource usage

**Use Case**: Production clusters with multiple nodes

### Option 2: Core-Only (Centralized Architecture)
**Pros**:
- Simpler deployment (no mTLS setup)
- Fewer pods (only Core)
- Lower resource usage in small clusters

**Cons**:
- Core must use Kubernetes API directly
- All collection logic in Core
- Single point of load
- No node-level distribution

**Use Case**: Small dev clusters, single-node setups

---

## 🎯 Recommendation for Fortuna

### Deploy Agent: ✅ YES

**Reason**:
1. **Scalability**: Fortuna is designed for production
2. **Best Practice**: Distributed data collection
3. **Performance**: Parallel collection across nodes
4. **Architecture**: Agent-Core is the intended design

### Implementation Plan
1. Generate TLS certificates (CA + Agent + Core)
2. Create Kubernetes secrets
3. Update agent deployment with proper TLS mounts
4. Update Core to accept agent connections
5. Deploy agent as DaemonSet
6. Verify mTLS connection

---

## 🔧 Agent Logic Review

### 1. Resource Collection

**Pods** (`pkg/collector/pod_collector.go`):
```go
// Collects pods from local node
// Filters by node name
// Extracts: name, namespace, uid, labels, annotations, containers, status
```

**ServiceAccounts** (`pkg/collector/sa_collector.go`):
```go
// Collects ServiceAccounts cluster-wide
// No node filtering (global resource)
// Extracts: name, namespace, uid, secrets, automount token
```

**RBAC** (`pkg/collector/rbac_collector.go`):
```go
// Collects Roles, RoleBindings, ClusterRoles, ClusterRoleBindings
// Global resources (no node filtering)
// Extracts: rules, subjects, role refs
```

### 2. gRPC Communication

**Client** (`pkg/client/grpc_client.go`):
```go
// Establishes mTLS connection to Core
// Sends collected resources via gRPC
// Handles retries and reconnection
// Batch sends for efficiency
```

**Protocol**:
- **Transport**: gRPC over TLS 1.3
- **Auth**: Mutual TLS (mTLS)
- **Format**: Protobuf messages
- **Compression**: gzip

### 3. Resource Watching

**Watcher** (`pkg/watcher/resource_watcher.go`):
```go
// Uses Kubernetes Informers
// Watches for: Add, Update, Delete events
// Filters by node (for Pods)
// Queues changes for batch sending
```

**Event Flow**:
```
Kubernetes API → Informer → Event Handler → Queue → Batch Send → gRPC → Core
```

---

## 🚀 Agent vs Core-Only Comparison

| Aspect | Agent (Distributed) | Core-Only (Centralized) |
|--------|---------------------|-------------------------|
| **Collection** | Parallel per-node | Sequential from Core |
| **Scalability** | High (per-node) | Limited (single point) |
| **Latency** | Low (local watch) | Higher (remote API) |
| **Resource Usage** | Distributed | Concentrated in Core |
| **Complexity** | Higher (mTLS) | Lower (no agent) |
| **Production Ready** | ✅ Yes | ⚠️  Dev/small clusters |

---

## 📋 Agent Deployment Checklist

### Prerequisites
- [x] Agent code reviewed
- [ ] TLS certificates generated
- [ ] Secrets created in `fortuna` namespace
- [ ] Agent deployment manifest updated
- [ ] Core configured to accept agent connections
- [ ] RBAC configured

### Deployment Steps
1. Generate certificates
2. Create secrets
3. Build agent image
4. Push to registry (or use Minikube's docker)
5. Apply RBAC
6. Deploy DaemonSet
7. Verify pods running
8. Check mTLS connection
9. Verify data flow to Core

---

## 🔍 Key Configuration

### Agent Environment Variables
```yaml
KSAM_CORE_ENDPOINT: "ksam-core.fortuna.svc.cluster.local:9090"
KSAM_CLUSTER_ID: "fortuna-cluster"
KSAM_NODE_NAME: (from downward API)
KSAM_SYNC_INTERVAL: "30s"
KSAM_TLS_CERT: "/etc/tls/tls.crt"
KSAM_TLS_KEY: "/etc/tls/tls.key"
KSAM_TLS_CA: "/etc/tls/ca.crt"
```

### Core gRPC Server
```yaml
KSAM_GRPC_PORT: "9090"
KSAM_GRPC_TLS_ENABLED: "true"
KSAM_GRPC_TLS_CERT: "/etc/tls/tls.crt"
KSAM_GRPC_TLS_KEY: "/etc/tls/tls.key"
KSAM_GRPC_TLS_CA: "/etc/tls/ca.crt"
KSAM_GRPC_REQUIRE_CLIENT_CERT: "true"
```

---

## 🎯 Next Actions

### Immediate
1. **Generate TLS Certificates**
   ```bash
   ./scripts/generate-certs.sh fortuna
   ```

2. **Create Secrets**
   ```bash
   kubectl -n fortuna create secret generic ksam-agent-tls \
     --from-file=tls.crt=./certs/agent.crt \
     --from-file=tls.key=./certs/agent.key \
     --from-file=ca.crt=./certs/ca.crt
   
   kubectl -n fortuna create secret generic ksam-core-tls \
     --from-file=tls.crt=./certs/core.crt \
     --from-file=tls.key=./certs/core.key \
     --from-file=ca.crt=./certs/ca.crt
   ```

3. **Build & Deploy Agent**
   ```bash
   eval $(minikube docker-env)
   docker build -t fortuna/agent:latest -f agent/Dockerfile .
   kubectl apply -f deploy/agent-rbac.yaml
   kubectl apply -f deploy/agent-daemonset.yaml
   ```

4. **Verify**
   ```bash
   kubectl -n fortuna get pods -l app=fortuna-agent
   kubectl -n fortuna logs -l app=fortuna-agent
   ```

---

## 🔒 Security Considerations

### mTLS Benefits
- ✅ **Mutual Authentication**: Both Agent and Core verify each other
- ✅ **Encrypted Transport**: All data encrypted in transit
- ✅ **Certificate-Based**: No passwords or tokens
- ✅ **Tamper-Proof**: Cannot be intercepted or modified

### Certificate Management
- **Validity**: 365 days (renew annually)
- **Rotation**: Update secrets without pod restart
- **Storage**: Kubernetes secrets (base64 encoded)
- **Access**: Only Agent and Core pods can read

---

## 📚 Related Documentation

- **[Agent Code](../../agent/)**
- **[Deployment Guide](../getting-started/README.md)**
- **[mTLS Setup](../operations/SECURITY.md)**
- **[Core gRPC Server](../components/core/GRPC.md)**

---

## ✅ Conclusion

**Agent should be deployed** for Fortuna production architecture.

**Key Points**:
1. Agent provides distributed, scalable data collection
2. mTLS ensures secure communication
3. Certificates need to be generated and mounted
4. Agent is the intended architecture (not optional)

**Next Step**: Generate certificates and deploy agent.

---

*Last Updated: December 22, 2024*  
*Fortuna K8s Management Platform v2.0*

