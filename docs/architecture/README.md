# Architecture Documentation

This section contains comprehensive architecture documentation for Fortuna K8s Management Platform, including system design, data flow, and component interactions.

---

## ⚠️ Important: Current Architecture is Core-Only

**Fortuna currently operates with a Core-Only architecture.**

- ✅ **Core Pod**: Performs ALL functions (collection, SBOM, CVE, insights, risk scoring)
- ❌ **Agent**: Disabled by design (optional, not currently needed)

See **[Architecture Analysis](../ARCHITECTURE_ANALYSIS.md)** for detailed explanation.

---

## Main Architecture Document

👉 **[ARCHITECTURE.md](../ARCHITECTURE.md)** - Start here for the complete system architecture

This is the primary architecture reference document covering:
- System overview
- Component architecture (Core-centric)
- Data flow diagrams
- Technology stack
- Design decisions
- Scalability patterns

---

## Quick Navigation

### System Overview
- [Component Architecture](#component-architecture)
- [Data Flow](#data-flow)
- [Technology Stack](#technology-stack)

### Deep Dives
- [Agent → Core Communication](#agent-core-communication)
- [Worker Pipeline](#worker-pipeline)
- [Graph Database Design](#graph-database-design)
- [Risk Scoring Algorithm](#risk-scoring-algorithm)

---

## Component Architecture

### Current Architecture: Core-Only (Agent Optional)

Fortuna operates with a streamlined Core-centric architecture:

```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                     │
│                                                         │
│  ┌────────────┐                                        │
│  │ Kubernetes │                                        │
│  │    API     │  List/Watch                           │
│  │  Server    │  (in-cluster)                         │
│  └──────┬─────┘                                        │
│         │                                              │
│         ▼                                              │
│  ┌──────────────────────────────────────────────┐    │
│  │         FORTUNA CORE (Single Pod)            │    │
│  │                                              │    │
│  │  ┌────────────────────────────────────┐    │    │
│  │  │  REST API + gRPC + Webhook         │    │    │
│  │  │  (HTTP: 8080, gRPC: 9090)         │    │    │
│  │  └────────────────────────────────────┘    │    │
│  │                                              │    │
│  │  ┌────────────────────────────────────┐    │    │
│  │  │  Kubernetes API Client             │    │    │
│  │  │  • Collects ServiceAccounts       │    │    │
│  │  │  • Collects Pods                  │    │    │
│  │  │  • Collects RBAC                  │    │    │
│  │  └────────────────────────────────────┘    │    │
│  │                                              │    │
│  │  ┌────────────────────────────────────┐    │    │
│  │  │  Worker Pool (NATS-based)         │    │    │
│  │  │  • Normalizer                     │    │    │
│  │  │  • SBOM Worker                    │    │    │
│  │  │  • CVE Matcher Worker             │    │    │
│  │  │  • Risk Worker (creates insights) │    │    │
│  │  │  • Correlator                     │    │    │
│  │  └────────────────────────────────────┘    │    │
│  │                                              │    │
│  │  ┌────────────────────────────────────┐    │    │
│  │  │  Insight Manager                   │    │    │
│  │  │  • Create/Update Insights          │    │    │
│  │  │  • Deduplication Logic             │    │    │
│  │  └────────────────────────────────────┘    │    │
│  └──────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
                         │
          ┌──────────────┴──────────────┐
          ▼                             ▼
┌──────────────────┐         ┌──────────────────┐
│   PostgreSQL     │         │      NATS        │
│   + Apache AGE   │         │  (JetStream)     │
│   • 74k CVEs     │         │  • Event Bus     │
│   • 18k Insights │         │  • Workers       │
└──────────────────┘         └──────────────────┘
```

### Optional Agent Architecture (Future)

```
┌─────────────────────────────────────────────────────────┐
│  ┌──────────────┐     ┌─────────────────────────┐     │
│  │  Agent Pod   │────▶│      Fortuna Core       │     │
│  │ (DaemonSet)  │gRPC │                         │     │
│  │  - Optional  │     │  All processing here    │     │
│  └──────────────┘     └─────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

**Note**: Agent is currently **disabled** and **not required**. Core handles all collection, processing, and insight generation independently.

See [../03-components/](../03-components/) and [../ARCHITECTURE_ANALYSIS.md](../ARCHITECTURE_ANALYSIS.md) for detailed documentation.

---

## Data Flow

### 1. Resource Collection Flow (Core-Only)

```
Kubernetes API
    ↓ List/Watch (in-cluster ServiceAccount)
Core Kubernetes Client
    ├─▶ Fetch ServiceAccounts, Pods, Roles, RBAC
    ├─▶ Transform to internal models
    └─▶ Store: PostgreSQL (serviceaccounts, pods tables)
        ↓
    Publish: NATS queue for async processing
        ↓
Worker Pool (in Core)
    ├─▶ Normalizer: Clean and enrich data
    ├─▶ Correlator: Build graph relationships
    └─▶ Risk Engine: Calculate risk scores, CREATE INSIGHTS ✅
```

**Note**: All collection happens in Core pod. No Agent involved.

### 2. CVE Scanning Flow (All in Core)

```
Pod Created/Updated
    ↓
Core collects from K8s API → Normalizer publishes event (NATS: ksam.normalized.pods)
    ↓
SBOM Worker (in Core, cache-first)
    ├─▶ Resolve digest → check sboms(image_digest)
    └─▶ If missing: extract layers → parse packages → persist sboms + sbom_components
        ↓
Publish SBOM_CREATED event (NATS: ksam.sbom.created)
    ↓
CVE Matcher Worker (in Core)
    ├─▶ Query PostgreSQL (74,561 CVEs + package_vulnerabilities)
    ├─▶ Persist cve_matches (dedup)
    └─▶ Call Insight Manager → CREATE VULNERABILITY INSIGHTS ✅
        ↓
Risk Engine (in Core)
    └─▶ Auto-calc risk score → Update insights
```

**Database State**:
- CVEs: 74,561 loaded
- Active Insights: 17,766
- All processing in Core pod

### 3. Policy Evaluation Flow

```
Resource Synced
    ↓
Policy Engine
    ├─▶ Load policy templates (CEL expressions)
    ├─▶ Evaluate expressions against resource
    └─▶ Create violations if match
        ↓
    Store violations (policy_violations table)
        ↓
Risk Engine
    └─▶ Create policy insights (if severity ≥ threshold)
```

### 4. Graph Building Flow

```
ServiceAccount/Pod/Role Data
    ↓
Correlator Worker
    ├─▶ Create vertices (ServiceAccount, Pod, Role nodes)
    ├─▶ Find relationships
    │    ├─▶ SA uses Pod (USES edge)
    │    ├─▶ SA has Role (HAS_ROLE edge)
    │    └─▶ Pod in Namespace (IN_NAMESPACE edge)
    └─▶ Store in Apache AGE graph
        ↓
Graph Engine
    └─▶ Query for attack paths, blast radius
```

---

## Technology Stack

### Core Technologies

| Component | Technology | Version | Purpose |
|-----------|------------|---------|----------|
| **Backend** | Go | 1.21+ | Core service, Agent |
| **Database** | PostgreSQL | 14+ | Primary data store |
| **Graph DB** | Apache AGE | 1.5.0 | Relationship graphs |
| **Message Queue** | NATS | 2.10+ | Async processing |
| **Frontend** | React + TypeScript | 18+ | Dashboard UI |
| **ORM** | GORM | 1.25+ | Database access |
| **gRPC** | grpc-go | 1.60+ | Agent-Core communication |

### Supporting Technologies

| Purpose | Technology |
|---------|------------|
| Container Runtime | Docker, containerd |
| Orchestration | Kubernetes 1.24+ |
| Monitoring | Prometheus + Grafana |
| Build Tool | Vite (Frontend), Go toolchain |
| Package Manager | npm (Frontend), go mod |
| UI Components | Tailwind CSS, shadcn/ui |
| Charts | Recharts, D3.js |

---

## Agent-Core Communication (Optional - Currently Not Used)

### Current State: Direct K8s API Access

**Core uses Kubernetes API directly:**
```
Core Pod
  │ in-cluster ServiceAccount
  │ ClusterRole: ksam-core-cluster-reader
  ▼
Kubernetes API Server
  └─▶ List/Watch all resources
```

**No Agent needed** for current architecture.

---

### Future: gRPC with mTLS (If Agent Enabled)

```
Agent                          Core
  │                             │
  │  1. Establish mTLS          │
  ├────────────────────────────▶│
  │     TLS Handshake           │
  │  2. Validate Certificates   │
  │◀────────────────────────────┤
  │                             │
  │  3. SyncServiceAccounts()   │
  ├────────────────────────────▶│
  │     Request: []SA           │
  │  4. Process & Store         │
  │                             │
  │  5. Response: Ack           │
  │◀────────────────────────────┤
  │                             │
```

**Protocol**:
```protobuf
service AgentService {
  rpc SyncServiceAccounts(SyncServiceAccountsRequest) returns (SyncServiceAccountsResponse);
  rpc SyncPods(SyncPodsRequest) returns (SyncPodsResponse);
  rpc GetCertificate(GetCertificateRequest) returns (GetCertificateResponse);
}
```

**Note**: gRPC server in Core is ready (port 9090), but Agent is not deployed.

---

## Worker Pipeline

### NATS-Based Async Processing

```
Event                NATS Queue              Worker
  │                     │                      │
  │  1. Publish         │                      │
  ├────────────────────▶│                      │
  │                     │  2. Subscribe        │
  │                     ├─────────────────────▶│
  │                     │  3. Ack/Nack         │
  │                     │◀─────────────────────┤
```

**Features**:
- Concurrent processing (10-50 workers)
- Dead Letter Queue for failures
- Exponential backoff retry
- Exactly-once processing (idempotent)
- Message durability

**Worker Types**:
1. **Normalizer**: Data cleaning and enrichment
2. **Correlator**: Graph relationship building
3. **Risk Worker**: Risk score calculation
4. **CVE Matcher**: Vulnerability matching

---

## Graph Database Design

### Apache AGE Schema

**Why Apache AGE?**
- PostgreSQL extension (no separate database)
- Cypher query language (Neo4j compatible)
- SQL + Graph queries in same database
- Better integration with existing data

**Graph Structure**:
```
Vertices: ServiceAccount, Pod, Role, Namespace, Secret
Edges: USES, HAS_ROLE, IN_NAMESPACE, MOUNTS_SECRET, CAN_ACCESS
```

**Query Examples**:
```cypher
// Attack path
MATCH path = (sa:ServiceAccount)-[*1..5]->(secret:Secret)
WHERE sa.namespace = 'default'
RETURN path

// Blast radius
MATCH (sa:ServiceAccount)-[:HAS_ROLE]->(role:Role)-[:CAN_ACCESS]->(r)
WHERE sa.name = 'admin-sa'
RETURN count(r)
```

---

## Risk Scoring Algorithm

### Multi-Factor Weighted Scoring

```
Final Risk Score = Base Score × Context Multiplier × Exposure Factor

Base Score:
  - CVE CVSS score (0-10)
  - Policy severity (LOW=2, MEDIUM=5, HIGH=8, CRITICAL=10)
  - RBAC weight (based on permissions)

Context Multiplier:
  - Namespace sensitivity (kube-system=1.5×, default=1.3×)
  - Resource type (DaemonSet=1.2×, Pod=1.0×)

Exposure Factor:
  - Network exposure (LoadBalancer=1.5×, NodePort=1.3×)
  - Privilege level (privileged=1.5×, hostPID=1.4×)
```

See [../03-components/risk-engine/](../03-components/risk-engine/) for details.

---

## Scalability Patterns

### Horizontal Scaling

**Core Service**:
```yaml
replicas: 3  # Run multiple Core instances
```
- Stateless API (can scale horizontally)
- Load balanced via Kubernetes Service
- Shared PostgreSQL and NATS

**Agent** (Optional, Currently Disabled):
```yaml
kind: DaemonSet  # One agent per node
```
- ❌ Not deployed in current architecture
- ✅ Core collects directly from K8s API
- Future: Can be enabled for multi-cluster or very large scale

### Database Optimization

**Connection Pooling**:
```
Max Connections: 100
Pool Size per Core instance: 25
3 Core replicas = 75 concurrent connections
```

**Indexes**:
- Covering indexes for common queries
- Partial indexes for soft-deleted resources
- GIN indexes for JSONB columns

### Worker Scaling

**Dynamic Worker Pool**:
```go
// Adjust based on queue length
if queueLength > 1000 {
    scaleUp(workerPool)
} else if queueLength < 100 {
    scaleDown(workerPool)
}
```

---

## Security Architecture

### Defense in Depth

1. **Network Layer**: mTLS between Agent and Core
2. **API Layer**: JWT authentication, rate limiting
3. **Database Layer**: Encrypted at rest, TLS connections
4. **Admission Layer**: Webhook validation before resource creation

### Secrets Management

- TLS certificates in Kubernetes Secrets
- Database credentials via Secrets
- JWT secret rotation supported
- No secrets in ConfigMaps

---

## Changelog

### Version 2.0 (Current - December 2024)
- ✅ **Core-Only Architecture**: Simplified deployment (Agent optional)
- ✅ **Direct K8s API Access**: Core collects resources directly
- ✅ **CVE Scanning**: 74,561 CVEs loaded from OSV.dev
- ✅ **Custom SBOM Generation**: Zero external dependencies
- ✅ **Insight Manager**: Optimized with JSONB queries & transactions
- ✅ **Policy Engine**: CEL-based evaluation
- ✅ **Attack Path Visualization**: Apache AGE graph
- ✅ **Async Risk Scoring**: Non-blocking pipeline

### Version 2 MVP2 (Previous)
- Added CVE scanning with OSV.dev
- Custom SBOM generation
- Policy engine with CEL
- Attack path visualization
- Dashboard improvements

### Version 1 (MVP1)
- Initial release
- ServiceAccount and Pod collection
- RBAC analysis
- Risk scoring
- Basic dashboard

See [changelog/](changelog/) and [../ARCHITECTURE_ANALYSIS.md](../ARCHITECTURE_ANALYSIS.md) for detailed version history.

---

## Related Documentation

- [Component Details](../03-components/) - Deep dive into each component
- [Deployment Architecture](../04-deployment/) - Production deployment patterns
- [Operations](../05-operations/) - Monitoring and maintenance

---

**Last Updated**: December 16, 2025
