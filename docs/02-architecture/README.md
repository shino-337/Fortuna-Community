# Fortuna K8s Management Platform - Architecture Documentation

**Project:** Fortuna K8s Management Platform (formerly KSAM)  
**Version:** 2.0.0 (Agent-Based Architecture)  
**Last Updated:** 2025-12-23  
**Status:** ✅ Phase 1 Complete (SBOM Pipeline)

---

## 📋 Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Components](#core-components)
3. [Data Flow](#data-flow)
4. [Communication](#communication)
5. [Security](#security)
6. [Deployment](#deployment)
7. [References](#references)

---

## 🏗️ Architecture Overview

Fortuna follows a **mandatory Agent-Based architecture** where:

- **Agent (Data Plane)**: DaemonSet running on each node, responsible for data collection, SBOM extraction, and CVE scanning
- **Core (Control Plane)**: Centralized service for policy evaluation, risk scoring, correlation, API, and dashboard

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Node 1     │  │   Node 2     │  │   Node N     │      │
│  │              │  │              │  │              │      │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │      │
│  │ │  Agent   │ │  │ │  Agent   │ │  │ │  Agent   │ │      │
│  │ │(DaemonSet)│◄┼──┼▶│(DaemonSet)│◄┼──┼▶│(DaemonSet)│ │      │
│  │ └────┬─────┘ │  │ └────┬─────┘ │  │ └────┬─────┘ │      │
│  │      │       │  │      │       │  │      │       │      │
│  │      │ Watch │  │      │ Watch │  │      │ Watch │      │
│  │      ▼ Pods  │  │      ▼ Pods  │  │      ▼ Pods  │      │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │      │
│  │ │Pod       │ │  │ │Pod       │ │  │ │Pod       │ │      │
│  │ │(nginx)   │ │  │ │(app)     │ │  │ │(db)      │ │      │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                  │                  │             │
│         └──────────────────┼──────────────────┘             │
│                            │ mTLS gRPC                      │
│                            ▼                                │
│                  ┌──────────────────┐                       │
│                  │  Fortuna Core    │                       │
│                  │  (Deployment)    │                       │
│                  │                  │                       │
│                  │ • gRPC Server    │                       │
│                  │ • SBOM Ingestion │                       │
│                  │ • Policy Engine  │                       │
│                  │ • Risk Engine    │                       │
│                  │ • API Server     │                       │
│                  └────────┬─────────┘                       │
│                           │                                 │
│              ┌────────────┼────────────┐                    │
│              ▼            ▼            ▼                    │
│         ┌─────────┐  ┌────────┐  ┌──────────┐              │
│         │PostgreSQL│  │  NATS  │  │Dashboard │              │
│         │(+AGE)    │  │JetStream│  │  (Web)   │              │
│         └─────────┘  └────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────────┘
```

---

## 🧩 Core Components

### 1. Fortuna Agent (Data Plane)

**Deployment:** DaemonSet (one per node)  
**Responsibilities:**
- Watch pods on **local node only** (field selector: `spec.nodeName`)
- Extract SBOM from container images (using local docker socket)
- Send summarized SBOM findings to Core via mTLS gRPC
- Local caching to reduce redundant scanning

**Technology Stack:**
- Go 1.24
- Kubernetes client-go (informers with field selectors)
- Docker API (for image layer extraction)
- gRPC + mTLS (for Core communication)

**Key Files:**
- `agent/cmd/main.go`: Agent entrypoint
- `agent/internal/sbom/processor.go`: SBOM extraction & processing
- `agent/internal/watcher/pod_watcher_local.go`: Local pod watcher
- `agent/internal/client/grpc_client_mtls.go`: mTLS gRPC client

**Environment Variables:**
```yaml
NODE_NAME: <node-name>           # From Kubernetes downward API
CORE_GRPC_ENDPOINT: fortuna-core.fortuna.svc.cluster.local:9090
TLS_ENABLED: "true"
TLS_CERT_PATH: /etc/fortuna/tls/client/tls.crt
TLS_KEY_PATH: /etc/fortuna/tls/client/tls.key
TLS_CA_CERT_PATH: /etc/fortuna/tls/client/ca.crt
```

---

### 2. Fortuna Core (Control Plane)

**Deployment:** Deployment (1-3 replicas)  
**Responsibilities:**
- Receive SBOM findings from Agents via gRPC
- Store SBOM in PostgreSQL
- Publish `SBOM_CREATED` events to NATS
- CVE matching (via workers listening to NATS)
- Policy evaluation
- Risk scoring
- API server (REST)
- Dashboard backend

**Technology Stack:**
- Go 1.24
- gRPC server + mTLS
- PostgreSQL + Apache AGE (graph database)
- NATS JetStream (event bus)
- Gin (HTTP framework)

**Key Files:**
- `core/cmd/main.go`: Core entrypoint
- `core/internal/grpc/server.go`: gRPC server with mTLS
- `core/internal/grpc/handler_sbom.go`: SBOM ingestion handler
- `core/pkg/worker/`: NATS workers (CVE matcher, policy, risk)

**Exposed Ports:**
- `8080`: HTTP API
- `9090`: gRPC (Agent communication)

**Environment Variables:**
```yaml
DATABASE_URL: postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable
NATS_ENDPOINT: nats://nats.fortuna.svc.cluster.local:4222
GRPC_PORT: "9090"
HTTP_PORT: "8080"
TLS_ENABLED: "true"
TLS_CERT_PATH: /etc/fortuna/tls/server/tls.crt
TLS_KEY_PATH: /etc/fortuna/tls/server/tls.key
TLS_CA_CERT_PATH: /etc/fortuna/tls/server/ca.crt
```

---

### 3. Supporting Services

#### PostgreSQL + Apache AGE
- **Purpose:** Primary data store + graph database
- **Tables:** `sboms`, `sbom_components`, `cves`, `cve_matches`, `insights`, `pod_image_scans`
- **Deployment:** StatefulSet

#### NATS JetStream
- **Purpose:** Event bus for asynchronous processing
- **Subjects:** `ksam.sbom.created`, `ksam.cve.matched`, `ksam.policy.violated`
- **Deployment:** StatefulSet

---

## 🔄 Data Flow

### Phase 1: SBOM Pipeline (Current)

```
1. Pod Created
   └─> Agent detects pod (via informer on local node)
       └─> Agent extracts SBOM (docker API)
           └─> Agent converts to proto format
               └─> Agent sends SBOM to Core (mTLS gRPC)
                   └─> Core receives SBOM
                       └─> Core stores in PostgreSQL (sboms, sbom_components)
                           └─> Core publishes SBOM_CREATED event (NATS)
                               └─> CVE Matcher Worker picks up event
                                   └─> Matches CVEs from database
                                       └─> Creates cve_matches
                                           └─> Creates insights
                                               └─> Risk Engine scores
                                                   └─> Dashboard displays
```

**Key Points:**
- Agent watches **only local node** (reduces load on API server)
- Agent extracts SBOM **locally** (no central bottleneck)
- Agent sends **summarized findings** (not raw image layers)
- Core stores, correlates, and scores (centralized intelligence)

---

## 🔐 Communication

### Agent ↔ Core: mTLS gRPC

**Protocol:** gRPC with mutual TLS (mTLS)  
**Port:** 9090  
**Certificate Hierarchy:**
```
Root CA (self-signed)
  └─> fortuna-core-server (CN: fortuna-core.fortuna.svc.cluster.local)
  └─> fortuna-agent-client (CN: fortuna-agent)
```

**Proto Contract:** `api/proto/agent/service.proto`

**RPCs:**
- `RegisterAgent`: Agent registration
- `SendSBOMFinding`: Single SBOM submission
- `BatchSendSBOMFindings`: Batch SBOM submission
- `Ping`: Health check

**Security:**
- TLS 1.3 minimum
- Client authentication required
- Server name verification
- Certificate rotation supported

---

## 🛡️ Security

### Network Security
- All Agent↔Core communication encrypted with mTLS
- NATS uses internal cluster network (no external exposure)
- PostgreSQL accessible only from Core (no external access)

### RBAC
- **fortuna-core**: ClusterRole with read-only access to K8s metadata
- **fortuna-agent**: ClusterRole with read-only access to pods on local node

### Container Security
- Agent runs as **root** (required for docker socket access)
  - Mitigation: DaemonSet with limited blast radius
- Core runs as **non-root**
- All images use minimal Alpine base

---

## 🚀 Deployment

### Prerequisites
- Kubernetes 1.28+
- Docker runtime (for SBOM extraction)
- PostgreSQL 15+
- NATS JetStream

### Deployment Steps

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Deploy certificates (if using cert-manager)
kubectl apply -f deploy/certs/

# 3. Create self-signed certs (alternative)
# Already created in .certs/ directory

# 4. Create K8s secrets
kubectl create secret generic fortuna-core-server-tls \
  --from-file=tls.crt=.certs/server.crt \
  --from-file=tls.key=.certs/server.key \
  --from-file=ca.crt=.certs/ca.crt \
  -n fortuna

kubectl create secret generic fortuna-agent-client-tls \
  --from-file=tls.crt=.certs/client.crt \
  --from-file=tls.key=.certs/client.key \
  --from-file=ca.crt=.certs/ca.crt \
  -n fortuna

# 5. Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml

# 6. Deploy supporting services (PostgreSQL, NATS)
kubectl apply -f deploy/postgres.yaml
kubectl apply -f deploy/nats.yaml

# 7. Deploy Core
kubectl apply -f deploy/fortuna-core-deployment.yaml

# 8. Deploy Agent
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# 9. Verify
kubectl get pods -n fortuna
kubectl logs -n fortuna -l app.kubernetes.io/component=core
kubectl logs -n fortuna -l app.kubernetes.io/component=agent
```

---

## 📚 References

### Architecture Decision Records (ADRs)
- [ADR-0010: Agent as Data Plane (Mandatory)](./ADR-0010-AGENT-AS-DATA-PLANE-MANDATORY.md)

### Implementation Guides
- [Phase 1 Implementation Plan](./PHASE1_IMPLEMENTATION_PLAN.md)
- [Phase 1 Implementation Status](./PHASE1_IMPLEMENTATION_STATUS.md)
- [Agent Implementation Complete](./AGENT_IMPLEMENTATION_COMPLETE.md)
- [mTLS Setup Guide](./MTLS_SETUP_GUIDE.md)
- [Proto Contract Specification](../api/proto/agent/PROTO_CONTRACT_SPEC.md)
- [Code Review Checklist](./CODE_REVIEW_CHECKLIST.md)

### Component Documentation
- [SBOM Pipeline](../03-components/sbom/)
- [CVE Matching](../03-components/cve/)
- [Risk Engine](../03-components/risk/)
- [Policy Engine](../03-components/policy/)

---

## 🎯 Current Status

| Component | Status | Version |
|-----------|--------|---------|
| Agent (Data Plane) | ✅ Complete | 1.0.0 |
| Core (Control Plane) | ✅ Complete | 1.0.0 |
| SBOM Pipeline | ✅ Complete | 1.0.0 |
| CVE Matching | ✅ Implemented | 1.0.0 |
| Policy Engine | 🔄 Legacy (to be updated) | 0.9.0 |
| Risk Engine | 🔄 Legacy (to be updated) | 0.9.0 |
| Dashboard | 🔄 Legacy (to be updated) | 0.9.0 |

---

## 🔮 Roadmap

### Phase 2: Move CVE Scanning to Agent (Planned)
- Agent performs local CVE matching
- Agent sends CVE findings to Core
- Reduce Core's CVE database load

### Phase 3: Runtime Security (Planned)
- Agent monitors runtime behavior
- Syscall monitoring
- Network policy enforcement

### Phase 4: Multi-Cluster Support (Planned)
- Federation of multiple clusters
- Cross-cluster correlation
- Centralized dashboard

---

**For detailed technical specifications, see the individual component documentation in `docs/03-components/`.**
