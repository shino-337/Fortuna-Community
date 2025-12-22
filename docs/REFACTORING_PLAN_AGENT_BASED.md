# KSAM Refactoring Plan: Agent-Based Architecture

**Date:** 2025-12-22
**Objective:** Transform KSAM from Core-centric to Agent-based distributed collection architecture
**Target Model:** Agent DaemonSet as primary collector, Core as central processing brain
**Estimated Effort:** 4-6 weeks (3 engineers)

---

## Table of Contents
1. [Executive Summary](#executive-summary)
2. [Current vs Proposed Architecture](#current-vs-proposed-architecture)
3. [Target Architecture Diagrams](#target-architecture-diagrams)
4. [Detailed Refactoring Phases](#detailed-refactoring-phases)
5. [File-by-File Changes](#file-by-file-changes)
6. [Migration Strategy](#migration-strategy)
7. [Testing Plan](#testing-plan)
8. [Rollback Strategy](#rollback-strategy)
9. [Success Metrics](#success-metrics)

---

## Executive Summary

### Why Agent-Based Architecture?

**Current Problem:**
- Core does BOTH collection AND processing (violates single responsibility)
- Agent exists but is redundant/disabled
- No clear logical flow through agent
- Limited scalability (single Core pod collects from entire cluster)

**Proposed Solution:**
- **Agent (DaemonSet):** Distributed collectors on each node
- **Core (Deployment):** Central processing, no direct K8s API access
- **Clear data flow:** Agent → gRPC → Core → Processing Pipeline
- **Better scalability:** Collection load distributed across nodes

### Key Benefits
- ✅ Separation of concerns (collect vs process)
- ✅ Horizontal scalability (more nodes = more collectors)
- ✅ Reduced Core complexity
- ✅ Lower API server load (distributed watchers)
- ✅ Clear architectural boundaries
- ✅ Better resource isolation

### Risks
- ⚠️ More complex deployment (DaemonSet required)
- ⚠️ Network dependency (gRPC must be reliable)
- ⚠️ Migration complexity (2 weeks of dual-mode)

---

## Current vs Proposed Architecture

### CURRENT (Core-Centric)
```
┌─────────────────────────────────────────────┐
│         Kubernetes API Server               │
│  (Pods, ServiceAccounts, RBAC, etc.)        │
└──────────────┬──────────────────────────────┘
               │
               │ Direct K8s Client
               │ (Single point of collection)
               ↓
    ┌──────────────────────────┐
    │     CORE POD             │
    │ ┌──────────────────────┐ │
    │ │  K8s Client          │ │ ← PROBLEM: Too many responsibilities
    │ │  (List/Watch)        │ │
    │ └──────────┬───────────┘ │
    │            │              │
    │ ┌──────────▼───────────┐ │
    │ │  Direct DB Insert    │ │
    │ └──────────┬───────────┘ │
    │            │              │
    │ ┌──────────▼───────────┐ │
    │ │  Event Publisher     │ │
    │ └──────────┬───────────┘ │
    │            │              │
    │ ┌──────────▼───────────┐ │
    │ │  Worker Pool         │ │
    │ │  - Normalizer        │ │
    │ │  - Correlator        │ │
    │ │  - Risk              │ │
    │ └──────────────────────┘ │
    └──────────────────────────┘

    AGENT POD (DaemonSet)
    ┌──────────────────────┐
    │  ❌ DISABLED         │  ← Agent exists but not used
    │  ❌ REDUNDANT        │
    └──────────────────────┘
```

### PROPOSED (Agent-Based)
```
┌─────────────────────────────────────────────────────────────┐
│              Kubernetes API Server                          │
│         (Pods, ServiceAccounts, RBAC, etc.)                 │
└──────────────┬──────────────────┬───────────────────────────┘
               │                  │
               │ Watch            │ Watch
               │ (Node A)         │ (Node B, C, D...)
               ↓                  ↓
    ┌──────────────────┐   ┌──────────────────┐
    │  AGENT POD       │   │  AGENT POD       │  ← DaemonSet
    │  (Node A)        │   │  (Node B)        │     (1 per node)
    │ ┌──────────────┐ │   │ ┌──────────────┐ │
    │ │  Collector   │ │   │ │  Collector   │ │
    │ │  + Watchers  │ │   │ │  + Watchers  │ │
    │ └──────┬───────┘ │   │ └──────┬───────┘ │
    │        │          │   │        │          │
    │ ┌──────▼───────┐ │   │ ┌──────▼───────┐ │
    │ │ gRPC Client  │ │   │ │ gRPC Client  │ │
    │ └──────┬───────┘ │   │ └──────┬───────┘ │
    └────────┼─────────┘   └────────┼─────────┘
             │                      │
             │ TLS/mTLS             │ TLS/mTLS
             │                      │
             └──────────┬───────────┘
                        │
                        ↓
             ┌──────────────────────────┐
             │     CORE POD             │
             │ ┌──────────────────────┐ │
             │ │  gRPC Server         │ │ ← ONLY receives from agents
             │ │  (StreamInventory)   │ │
             │ └──────────┬───────────┘ │
             │            │              │
             │ ┌──────────▼───────────┐ │
             │ │  IngestAPI           │ │
             │ │  (Rate Limit/Dedup)  │ │
             │ └──────────┬───────────┘ │
             │            │              │
             │ ┌──────────▼───────────┐ │
             │ │  PostgreSQL Insert   │ │
             │ └──────────┬───────────┘ │
             │            │              │
             │ ┌──────────▼───────────┐ │
             │ │  NATS Publisher      │ │
             │ └──────────┬───────────┘ │
             │            │              │
             │ ┌──────────▼───────────┐ │
             │ │  Worker Pool         │ │
             │ │  - Normalizer        │ │
             │ │  - Correlator        │ │
             │ │  - Risk              │ │
             │ │  - SBOM              │ │
             │ │  - CVE Matcher       │ │
             │ └──────────┬───────────┘ │
             │            │              │
             │ ┌──────────▼───────────┐ │
             │ │  REST/gRPC API       │ │
             │ │  (Dashboard/CLI)     │ │
             │ └──────────────────────┘ │
             └──────────────────────────┘
                        │
                        ↓
                 ┌──────────────┐
                 │  Dashboard   │
                 │  (Frontend)  │
                 └──────────────┘
```

---

## Target Architecture Diagrams

### 1. Component Architecture
```
┌────────────────────────────────────────────────────────────────┐
│                    KUBERNETES CLUSTER                          │
│                                                                │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                 AGENT LAYER (DaemonSet)                 │  │
│  │                                                         │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │  │
│  │  │ Agent A  │  │ Agent B  │  │ Agent C  │  ...        │  │
│  │  │ Node-1   │  │ Node-2   │  │ Node-3   │             │  │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘             │  │
│  │       │             │             │                     │  │
│  │       │ Watch local │ Watch local │ Watch all          │  │
│  │       │ node pods   │ node pods   │ cluster resources  │  │
│  │       │             │             │                     │  │
│  └───────┼─────────────┼─────────────┼─────────────────────┘  │
│          │             │             │                         │
│          └─────────────┴─────────────┘                         │
│                        │                                       │
│                        │ gRPC Stream                           │
│                        │ (Bidirectional)                       │
│                        ↓                                       │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │              CORE LAYER (Deployment)                    │  │
│  │                                                         │  │
│  │  ┌───────────────────────────────────────────────────┐ │  │
│  │  │              gRPC Server                          │ │  │
│  │  │  - AgentService.StreamInventory()                │ │  │
│  │  │  - AgentService.RegisterAgent()                  │ │  │
│  │  │  - AgentService.Heartbeat()                      │ │  │
│  │  └─────────────────┬─────────────────────────────────┘ │  │
│  │                    │                                   │  │
│  │  ┌─────────────────▼─────────────────────────────────┐ │  │
│  │  │           IngestAPI (Rate Limiter)                │ │  │
│  │  │  - Validates inventory items                      │ │  │
│  │  │  - Deduplicates by UID                            │ │  │
│  │  │  - Rate limits per agent                          │ │  │
│  │  │  - Enriches with metadata                         │ │  │
│  │  └─────────────────┬─────────────────────────────────┘ │  │
│  │                    │                                   │  │
│  │  ┌─────────────────▼─────────────────────────────────┐ │  │
│  │  │          Event-Driven Pipeline                    │ │  │
│  │  │                                                   │ │  │
│  │  │  PostgreSQL ←──→ NATS JetStream                  │ │  │
│  │  │                    │                              │ │  │
│  │  │         ┌──────────┼──────────┐                  │ │  │
│  │  │         ↓          ↓          ↓                  │ │  │
│  │  │    Normalizer  Correlator  Risk Worker          │ │  │
│  │  │         │          │          │                  │ │  │
│  │  │         └──────────┼──────────┘                  │ │  │
│  │  │                    │                              │ │  │
│  │  │         ┌──────────┼──────────┐                  │ │  │
│  │  │         ↓          ↓          ↓                  │ │  │
│  │  │    SBOM Worker  CVE Matcher  Insight Manager    │ │  │
│  │  │                                                   │ │  │
│  │  └───────────────────────────────────────────────────┘ │  │
│  │                                                         │  │
│  │  ┌───────────────────────────────────────────────────┐ │  │
│  │  │               API Layer                           │ │  │
│  │  │  - REST API (Dashboard, CLI)                     │ │  │
│  │  │  - gRPC API (Agent communication)                │ │  │
│  │  │  - Admission Webhook (Policy enforcement)        │ │  │
│  │  └───────────────────────────────────────────────────┘ │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### 2. Data Flow with Agent as Primary Source

```
PHASE 1: COLLECTION (Distributed)
═══════════════════════════════════════════════════════════════

┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Agent A   │      │   Agent B   │      │   Agent C   │
│  (Node-1)   │      │  (Node-2)   │      │  (Node-3)   │
└──────┬──────┘      └──────┬──────┘      └──────┬──────┘
       │                    │                    │
       │ Watches:           │ Watches:           │ Watches:
       │ - Local Pods       │ - Local Pods       │ - ServiceAccounts
       │ - Local Events     │ - Local Events     │ - RBAC (all)
       │                    │                    │ - Namespaces
       │                    │                    │
       └────────────────────┴────────────────────┘
                            │
                     Collect & Convert
                     (Proto Messages)
                            │
                            ↓

PHASE 2: STREAMING (gRPC Bidirectional)
═══════════════════════════════════════════════════════════════

               ┌─────────────────────┐
               │    Agent Stream     │
               │  InventoryItem{}    │
               │                     │
               │  {                  │
               │    uid: "abc-123"   │
               │    type: "pod"      │
               │    payload: {...}   │
               │    timestamp: ...   │
               │    nodeID: "node-1" │
               │    agentID: "a-001" │
               │  }                  │
               └──────────┬──────────┘
                          │
                          │ TLS/mTLS (Port 9090)
                          │
                          ↓
               ┌──────────────────────┐
               │   Core gRPC Server   │
               │  StreamInventory()   │
               │                      │
               │  - Receives stream   │
               │  - ACKs receipt      │
               │  - Sends heartbeat   │
               └──────────┬───────────┘
                          │
                          ↓

PHASE 3: INGESTION (Validation & Rate Limiting)
═══════════════════════════════════════════════════════════════

               ┌──────────────────────┐
               │     IngestAPI        │
               │                      │
               │  1. Validate schema  │
               │  2. Check rate limit │
               │     (per agent)      │
               │  3. Deduplicate      │
               │     (by UID)         │
               │  4. Enrich metadata  │
               │  5. Persist to DB    │
               │  6. Publish to NATS  │
               └──────────┬───────────┘
                          │
            ┌─────────────┴─────────────┐
            │                           │
            ↓                           ↓
   ┌─────────────────┐       ┌──────────────────┐
   │   PostgreSQL    │       │  NATS JetStream  │
   │                 │       │                  │
   │  INSERT INTO    │       │  PUBLISH TO:     │
   │  pods, sa, etc. │       │  ksam.raw.pods   │
   │                 │       │  ksam.raw.sa     │
   └─────────────────┘       └────────┬─────────┘
                                      │
                                      ↓

PHASE 4: NORMALIZATION (Event-Driven Workers)
═══════════════════════════════════════════════════════════════

                          Subscribe: ksam.raw.>
                                      │
                          ┌───────────▼──────────┐
                          │  NormalizerWorker    │
                          │  (5 concurrent)      │
                          │                      │
                          │  - Extract labels    │
                          │  - Map types         │
                          │  - Standardize       │
                          └───────────┬──────────┘
                                      │
                          Publish: ksam.normalized.*
                                      │
                                      ↓

PHASE 5: CORRELATION & RISK (Parallel Processing)
═══════════════════════════════════════════════════════════════

          Subscribe: ksam.normalized.>
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ↓                 ↓                 ↓
┌───────────────┐ ┌───────────────┐ ┌──────────────┐
│ Correlator    │ │  Risk Worker  │ │ SBOM Worker  │
│               │ │               │ │              │
│ Build graphs  │ │ Eval policies │ │ Extract pkgs │
│ SA→Pod→NS     │ │ Check RBAC    │ │ from images  │
└───────┬───────┘ └───────┬───────┘ └──────┬───────┘
        │                 │                 │
        │                 │                 │ Publish:
        │                 │                 │ ksam.sbom.created
        │                 │                 │
        │                 │                 ↓
        │                 │         ┌────────────────┐
        │                 │         │ CVE Matcher    │
        │                 │         │                │
        │                 │         │ Match vulns    │
        │                 │         │ to components  │
        │                 │         └────────┬───────┘
        │                 │                  │
        │                 └──────────┬───────┘
        │                            │
        └────────────────────────────┘
                                     │
                                     ↓

PHASE 6: INSIGHT GENERATION (Unified)
═══════════════════════════════════════════════════════════════

                    ┌────────────────────┐
                    │  Insight Manager   │
                    │                    │
                    │  Sources:          │
                    │  - Policy violations│
                    │  - CVE matches     │
                    │  - RBAC risks      │
                    │  - Misconfigs      │
                    │                    │
                    │  Dedup by:         │
                    │  - Type+Severity   │
                    │  - Description     │
                    │  - Resources       │
                    └──────────┬─────────┘
                               │
                    Publish: ksam.insights.created
                               │
                               ↓

PHASE 7: RISK SCORING (Async)
═══════════════════════════════════════════════════════════════

                    ┌────────────────────┐
                    │  Risk Score Worker │
                    │  (Non-blocking)    │
                    │                    │
                    │  Score = (E + B)   │
                    │  * Severity * Decay│
                    │                    │
                    │  E: Exploitability │
                    │  B: Business Impact│
                    └──────────┬─────────┘
                               │
                    Persist to risk_scores
                               │
                               ↓

PHASE 8: API EXPOSURE & ENFORCEMENT
═══════════════════════════════════════════════════════════════

        ┌───────────────┐           ┌────────────────────┐
        │   REST API    │           │  Admission Webhook │
        │               │           │                    │
        │ GET insights  │           │  ValidateCreate    │
        │ GET risks     │           │  - Check policies  │
        │ GET sboms     │           │  - Block/Warn/Audit│
        │ PATCH remediate│          │  - Return response │
        └───────┬───────┘           └──────────┬─────────┘
                │                              │
                └──────────┬───────────────────┘
                           │
                           ↓
                  ┌─────────────────┐
                  │   Dashboard     │
                  │   CLI Tools     │
                  │   Webhooks      │
                  └─────────────────┘

PHASE 9: FEEDBACK LOOP (NEW)
═══════════════════════════════════════════════════════════════

                  ┌─────────────────┐
                  │  Operator/User  │
                  └────────┬────────┘
                           │
              PATCH /insights/:id/remediate
                           │
                           ↓
                  ┌─────────────────┐
                  │  Insight Manager│
                  │                 │
                  │  - Update status│
                  │  - Add ticket ID│
                  │  - Notify agents│
                  └────────┬────────┘
                           │
              Publish: ksam.remediation.action
                           │
                           ↓
                  ┌─────────────────┐
                  │   Agents        │
                  │  (Optional)     │
                  │                 │
                  │  - Apply labels │
                  │  - Trigger scans│
                  └─────────────────┘
```

### 3. Agent Internal Architecture

```
┌────────────────────────────────────────────────────────────┐
│                      AGENT POD                             │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐ │
│  │                  WATCH LAYER                         │ │
│  │                                                      │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌───────────┐ │ │
│  │  │ PodWatcher   │  │  SAWatcher   │  │RBACWatcher│ │ │
│  │  │              │  │              │  │           │ │ │
│  │  │ SharedIndexer│  │ SharedIndexer│  │SharedIndex│ │ │
│  │  │ Informer     │  │ Informer     │  │Informer   │ │ │
│  │  └──────┬───────┘  └──────┬───────┘  └─────┬─────┘ │ │
│  │         │                 │                 │       │ │
│  │         │    OnAdd/Update/Delete Events     │       │ │
│  │         │                 │                 │       │ │
│  └─────────┼─────────────────┼─────────────────┼───────┘ │
│            │                 │                 │         │
│  ┌─────────▼─────────────────▼─────────────────▼───────┐ │
│  │               EVENT QUEUE (Buffered Chan)           │ │
│  │                  (Cap: 1000)                        │ │
│  └─────────┬───────────────────────────────────────────┘ │
│            │                                             │
│  ┌─────────▼───────────────────────────────────────────┐ │
│  │                 COLLECTOR                           │ │
│  │                                                     │ │
│  │  - Batch events (max 100, max 5s)                  │ │
│  │  - Convert to proto (K8s → InventoryItem)          │ │
│  │  - Add agent metadata (nodeID, agentID)            │ │
│  │  - Compress if >1MB                                │ │
│  └─────────┬───────────────────────────────────────────┘ │
│            │                                             │
│  ┌─────────▼───────────────────────────────────────────┐ │
│  │              gRPC CLIENT                            │ │
│  │                                                     │ │
│  │  StreamInventory(ctx) stream                       │ │
│  │  ├─ Send(InventoryItem) -> ACK                     │ │
│  │  ├─ Recv(Heartbeat)                                │ │
│  │  └─ Retry on disconnect (exponential backoff)      │ │
│  │                                                     │ │
│  │  RegisterAgent(ctx) -> AgentInfo                   │ │
│  │  └─ Receive config, rules, rate limits             │ │
│  └─────────┬───────────────────────────────────────────┘ │
│            │                                             │
│  ┌─────────▼───────────────────────────────────────────┐ │
│  │            CONNECTION MANAGER                       │ │
│  │                                                     │ │
│  │  - Maintains TLS connection pool                   │ │
│  │  - Health checks (every 30s)                       │ │
│  │  - Reconnect on failure                            │ │
│  │  - Backpressure handling                           │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐ │
│  │              LOCAL CACHE (Optional)                  │ │
│  │                                                      │ │
│  │  - Buffer when Core unreachable                     │ │
│  │  - Disk-backed queue (max 10GB)                     │ │
│  │  - Replay on reconnect                              │ │
│  └──────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
```

---

## Detailed Refactoring Phases

### PHASE 0: Preparation (Week 1)
**Duration:** 5 days
**Goal:** Set up foundation for refactoring

#### Tasks:
1. **Create feature flag system**
   - Add config: `COLLECTION_MODE=agent|core|hybrid`
   - Add config: `AGENT_ENABLED=true|false`
   - Add config: `CORE_DIRECT_COLLECTION=true|false`

2. **Set up metrics and observability**
   - Add agent connection metrics
   - Add ingestion rate metrics
   - Add gRPC health checks
   - Add dashboards for monitoring

3. **Documentation**
   - Document current state
   - Create migration runbook
   - Create rollback procedures

4. **Testing infrastructure**
   - Set up integration test cluster
   - Create test data generators
   - Set up CI/CD for dual-mode testing

#### Deliverables:
- [ ] Feature flags implemented
- [ ] Metrics exported to Prometheus
- [ ] Grafana dashboards created
- [ ] Migration documentation complete
- [ ] Test cluster provisioned

---

### PHASE 1: Agent Enhancement (Week 2-3)
**Duration:** 10 days
**Goal:** Make agent production-ready for primary collection

#### 1.1: Enhance Agent Watchers
**Files to modify:**
- `/agent/internal/watcher/pod_watcher.go`
- `/agent/internal/watcher/serviceaccount_watcher.go`
- `/agent/internal/watcher/rbac_watcher.go`

**Changes:**
```go
// Add shared informer factory for efficiency
type WatcherManager struct {
    factory        informers.SharedInformerFactory
    podWatcher     *PodWatcher
    saWatcher      *ServiceAccountWatcher
    rbacWatcher    *RBACWatcher
    eventChan      chan *WatchEvent
    stopChan       chan struct{}
}

// Add event batching
type EventBatcher struct {
    events      []*WatchEvent
    maxBatch    int
    maxWait     time.Duration
    flushChan   chan []*WatchEvent
}

// Add local node filtering for pods
func (w *PodWatcher) shouldProcess(pod *corev1.Pod) bool {
    return pod.Spec.NodeName == w.nodeID
}
```

#### 1.2: Improve gRPC Client
**Files to modify:**
- `/agent/internal/client/grpc_client.go`
- `/agent/internal/client/grpc_client_new.go`

**Changes:**
```go
// Add connection pooling
type ConnectionPool struct {
    conns    []*grpc.ClientConn
    current  atomic.Uint32
    size     int
}

// Add exponential backoff retry
type RetryConfig struct {
    MaxRetries     int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    Multiplier     float64
}

// Add backpressure handling
type BackpressureHandler struct {
    maxBufferSize  int
    currentBuffer  int
    diskSpillPath  string
    metrics        *BackpressureMetrics
}

// Implement bidirectional streaming
func (c *Client) StreamInventory(ctx context.Context) error {
    stream, err := c.client.StreamInventory(ctx)

    // Send goroutine
    go func() {
        for item := range c.sendChan {
            if err := stream.Send(item); err != nil {
                c.handleSendError(err, item)
            }
        }
    }()

    // Receive goroutine (heartbeat, backpressure signals)
    for {
        resp, err := stream.Recv()
        if err != nil {
            return err
        }
        c.handleResponse(resp)
    }
}
```

#### 1.3: Add Agent Registration
**New file:** `/agent/internal/registration/registrar.go`

```go
type Registrar struct {
    client     pb.AgentServiceClient
    agentInfo  *pb.AgentInfo
    registered atomic.Bool
}

func (r *Registrar) Register(ctx context.Context) error {
    info := &pb.AgentInfo{
        AgentId:    r.agentInfo.AgentId,
        NodeId:     r.agentInfo.NodeId,
        Version:    VERSION,
        Capabilities: []string{"pods", "serviceaccounts", "rbac"},
        Hostname:   os.Hostname(),
    }

    resp, err := r.client.RegisterAgent(ctx, info)
    if err != nil {
        return err
    }

    // Apply received config
    r.applyConfig(resp.Config)
    r.registered.Store(true)

    // Start heartbeat
    go r.sendHeartbeat(ctx)

    return nil
}
```

#### 1.4: Add Local Cache/Buffer
**New file:** `/agent/internal/buffer/disk_buffer.go`

```go
type DiskBuffer struct {
    path       string
    maxSize    int64
    currentSize atomic.Int64
    queue      *bbolt.DB
}

func (b *DiskBuffer) Enqueue(item *pb.InventoryItem) error {
    data, _ := proto.Marshal(item)

    return b.queue.Update(func(tx *bbolt.Tx) error {
        bucket := tx.Bucket([]byte("pending"))
        id := uuid.New().String()
        return bucket.Put([]byte(id), data)
    })
}

func (b *DiskBuffer) Dequeue() (*pb.InventoryItem, error) {
    // FIFO dequeue logic
}
```

#### Deliverables:
- [ ] Agent watchers use shared informers
- [ ] Event batching implemented
- [ ] gRPC client with retry/backpressure
- [ ] Agent registration working
- [ ] Disk buffer for offline mode
- [ ] Unit tests (80% coverage)

---

### PHASE 2: Core gRPC Enhancement (Week 3-4)
**Duration:** 10 days
**Goal:** Make Core gRPC server robust for production traffic

#### 2.1: Enhance gRPC Server
**Files to modify:**
- `/core/internal/grpc/server.go`
- `/core/internal/grpc/handler_new.go`

**Changes:**
```go
// Add agent session management
type AgentSessionManager struct {
    sessions sync.Map // agentID -> *Session
    metrics  *SessionMetrics
}

type Session struct {
    AgentID      string
    NodeID       string
    ConnectedAt  time.Time
    LastSeen     time.Time
    MessageCount int64
    ErrorCount   int64
    RateLimit    *rate.Limiter
}

// Implement StreamInventory with session tracking
func (s *Server) StreamInventory(stream pb.AgentService_StreamInventoryServer) error {
    // Get agent info from metadata
    md, _ := metadata.FromIncomingContext(stream.Context())
    agentID := md.Get("agent-id")[0]

    // Create/update session
    session := s.sessionMgr.GetOrCreate(agentID)
    defer s.sessionMgr.Remove(agentID)

    // Receive loop
    for {
        item, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }

        // Rate limiting
        if !session.RateLimit.Allow() {
            s.sendBackpressure(stream, "rate_limit")
            continue
        }

        // Process item
        if err := s.ingestAPI.Process(item); err != nil {
            session.ErrorCount++
            continue
        }

        session.MessageCount++
        session.LastSeen = time.Now()

        // Send ACK periodically
        if session.MessageCount%100 == 0 {
            stream.Send(&pb.StreamResponse{
                Type: pb.StreamResponse_ACK,
                MessageCount: session.MessageCount,
            })
        }
    }
}

// Add agent management endpoints
func (s *Server) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
    // Validate agent
    if err := s.validateAgent(req.AgentInfo); err != nil {
        return nil, err
    }

    // Create session
    session := &Session{
        AgentID:     req.AgentInfo.AgentId,
        NodeID:      req.AgentInfo.NodeId,
        ConnectedAt: time.Now(),
        RateLimit:   rate.NewLimiter(1000, 5000), // 1000/s burst 5000
    }

    s.sessionMgr.Add(session)

    // Return config
    return &pb.RegisterAgentResponse{
        Config: &pb.AgentConfig{
            RateLimit:      1000,
            BatchSize:      100,
            BatchTimeout:   5000, // ms
            EnabledWatchers: []string{"pods", "serviceaccounts", "rbac"},
        },
    }, nil
}
```

#### 2.2: Enhance IngestAPI
**Files to modify:**
- `/core/internal/ingest/ingest.go`

**Changes:**
```go
// Add better deduplication
type DeduplicationCache struct {
    cache *ristretto.Cache
    ttl   time.Duration
}

func (d *DeduplicationCache) IsDuplicate(item *pb.InventoryItem) bool {
    key := fmt.Sprintf("%s:%s:%d", item.Type, item.Uid, item.ResourceVersion)
    _, found := d.cache.Get(key)
    if found {
        return true
    }
    d.cache.SetWithTTL(key, true, 1, d.ttl)
    return false
}

// Add validation
type Validator struct {
    schemas map[string]*jsonschema.Schema
}

func (v *Validator) Validate(item *pb.InventoryItem) error {
    schema := v.schemas[item.Type]
    if schema == nil {
        return fmt.Errorf("unknown type: %s", item.Type)
    }
    return schema.Validate(item.Payload)
}

// Add enrichment
type Enricher struct {
    db *gorm.DB
}

func (e *Enricher) Enrich(item *pb.InventoryItem) error {
    // Add cluster info
    item.Metadata["cluster_id"] = e.getClusterID()

    // Add namespace metadata
    if item.Namespace != "" {
        ns := e.getNamespaceMetadata(item.Namespace)
        item.Metadata["namespace_labels"] = ns.Labels
    }

    // Add node metadata (if pod)
    if item.Type == "pod" && item.Metadata["node_id"] != "" {
        node := e.getNodeMetadata(item.Metadata["node_id"])
        item.Metadata["node_labels"] = node.Labels
        item.Metadata["node_zone"] = node.Zone
    }

    return nil
}
```

#### 2.3: Add Health Checks
**New file:** `/core/internal/grpc/health.go`

```go
type HealthServer struct {
    pb.UnimplementedHealthServer
    sessionMgr *AgentSessionManager
    db         *gorm.DB
    nats       *nats.Conn
}

func (h *HealthServer) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
    // Check database
    if err := h.db.Exec("SELECT 1").Error; err != nil {
        return &pb.HealthCheckResponse{
            Status: pb.HealthCheckResponse_NOT_SERVING,
        }, nil
    }

    // Check NATS
    if !h.nats.IsConnected() {
        return &pb.HealthCheckResponse{
            Status: pb.HealthCheckResponse_NOT_SERVING,
        }, nil
    }

    // Check agent connections
    activeAgents := h.sessionMgr.CountActive()
    if activeAgents == 0 {
        return &pb.HealthCheckResponse{
            Status: pb.HealthCheckResponse_NOT_SERVING,
        }, nil
    }

    return &pb.HealthCheckResponse{
        Status: pb.HealthCheckResponse_SERVING,
    }, nil
}
```

#### Deliverables:
- [ ] Agent session management
- [ ] Rate limiting per agent
- [ ] ACK/backpressure signaling
- [ ] Deduplication cache
- [ ] Input validation
- [ ] Metadata enrichment
- [ ] Health check endpoints
- [ ] Integration tests

---

### PHASE 3: Core K8s Client Removal (Week 4-5)
**Duration:** 5 days
**Goal:** Remove direct K8s API access from Core

#### 3.1: Add Feature Flag Gate
**Files to modify:**
- `/core/cmd/main.go`

**Changes:**
```go
func main() {
    cfg := config.Load()

    // FEATURE FLAG: Control collection mode
    if cfg.CollectionMode == "agent" {
        log.Info("Running in agent-based collection mode")
        // Skip K8s client initialization
    } else if cfg.CollectionMode == "core" {
        log.Info("Running in legacy core collection mode")
        initKubernetesClient(cfg)
    } else if cfg.CollectionMode == "hybrid" {
        log.Info("Running in hybrid mode (both agent and core)")
        initKubernetesClient(cfg)
    }

    // Always start gRPC server (for agent communication)
    grpcServer := grpc.NewServer(cfg, db, nats)
    go grpcServer.Start()
}
```

#### 3.2: Conditional K8s Client
**New file:** `/core/internal/collector/k8s_collector.go`

```go
type K8sCollector struct {
    client    kubernetes.Interface
    enabled   bool
    publisher *messaging.Publisher
}

func NewK8sCollector(cfg *config.Config, publisher *messaging.Publisher) *K8sCollector {
    if cfg.CoreDirectCollection {
        client, _ := kubernetes.NewForConfig(cfg.KubeConfig)
        return &K8sCollector{
            client:    client,
            enabled:   true,
            publisher: publisher,
        }
    }
    return &K8sCollector{enabled: false}
}

func (c *K8sCollector) Start(ctx context.Context) error {
    if !c.enabled {
        log.Info("Core direct collection disabled, relying on agents")
        return nil
    }

    // Original collection logic
}
```

#### 3.3: Update Helm Charts
**Files to modify:**
- `/helm/ksam/values.yaml`
- `/helm/ksam/templates/deployment.yaml`

**Changes:**
```yaml
# values.yaml
core:
  collectionMode: "agent"  # agent | core | hybrid
  directCollection: false  # Disable K8s client in core

agent:
  enabled: true  # Enable agent DaemonSet
  daemonset:
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 512Mi

# deployment.yaml
env:
- name: COLLECTION_MODE
  value: {{ .Values.core.collectionMode }}
- name: CORE_DIRECT_COLLECTION
  value: {{ .Values.core.directCollection | quote }}
```

#### 3.4: Update RBAC
**Files to modify:**
- `/helm/ksam/templates/agent-rbac.yaml`

**Changes:**
```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ksam-agent
  namespace: {{ .Values.namespace }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksam-agent-reader
rules:
- apiGroups: [""]
  resources: ["pods", "serviceaccounts", "namespaces", "nodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ksam-agent-reader-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ksam-agent-reader
subjects:
- kind: ServiceAccount
  name: ksam-agent
  namespace: {{ .Values.namespace }}
```

#### Deliverables:
- [ ] Feature flag implemented
- [ ] K8s client conditional
- [ ] Helm charts updated
- [ ] RBAC for agent created
- [ ] Documentation updated
- [ ] Backward compatibility maintained

---

### PHASE 4: Worker Pool Integration (Week 5)
**Duration:** 5 days
**Goal:** Integrate SBOM/CVE workers into pool

#### 4.1: Add Message Deduplication
**Files to modify:**
- `/core/pkg/messaging/nats_client.go`

**Changes:**
```go
// Configure JetStream with message deduplication
func (c *Client) CreateStream(name string, subjects []string) error {
    _, err := c.js.AddStream(&nats.StreamConfig{
        Name:     name,
        Subjects: subjects,

        // Enable deduplication
        Duplicates: 5 * time.Minute,

        // Other configs
        Retention:    nats.InterestPolicy,
        MaxAge:       24 * time.Hour,
        MaxConsumers: 10,
    })
    return err
}

// Add deduplication ID to published messages
func (c *Client) Publish(subject string, data []byte, uid string) error {
    msg := &nats.Msg{
        Subject: subject,
        Data:    data,
        Header:  nats.Header{},
    }

    // Use resource UID as deduplication ID
    msg.Header.Set(nats.MsgIdHdr, uid)

    _, err := c.js.PublishMsg(msg)
    return err
}
```

#### 4.2: Refactor SBOM Worker
**Files to modify:**
- `/core/pkg/worker/sbom_worker.go`

**Changes:**
```go
// Remove manual subscription, use pool interface
type SBOMWorker struct {
    db        *gorm.DB
    nats      jetstream.JetStream
    extractor *sbom.Extractor
}

func (w *SBOMWorker) Subject() string {
    return "ksam.normalized.pods"
}

func (w *SBOMWorker) ConsumerConfig() *jetstream.ConsumerConfig {
    return &jetstream.ConsumerConfig{
        Durable:       "sbom-worker",
        FilterSubject: w.Subject(),
        MaxAckPending: 10,  // Limit concurrency per instance
        AckWait:       10 * time.Minute,
    }
}

func (w *SBOMWorker) Process(ctx context.Context, msg jetstream.Msg) error {
    var event PodEvent
    if err := json.Unmarshal(msg.Data(), &event); err != nil {
        return err
    }

    // NATS will deduplicate based on message ID
    // No need for manual dedup here

    return w.processPod(&event.Pod)
}

// Pool will handle subscription and concurrency
```

#### 4.3: Refactor CVE Matcher Worker
**Files to modify:**
- `/core/pkg/worker/cve_matcher_worker.go`

**Similar changes as SBOM worker**

#### 4.4: Update Pool
**Files to modify:**
- `/core/pkg/worker/pool.go`

**Changes:**
```go
func (p *Pool) Start(ctx context.Context) error {
    for _, worker := range p.workers {
        // Create consumer for each worker
        consumerConfig := worker.ConsumerConfig()
        consumer, err := p.js.CreateOrUpdateConsumer(ctx, STREAM_NAME, *consumerConfig)
        if err != nil {
            return err
        }

        // Start N concurrent processors
        for i := 0; i < p.concurrency; i++ {
            go func(workerInstance int) {
                for {
                    // Fetch next message
                    msgs, err := consumer.Fetch(1)
                    if err != nil {
                        continue
                    }

                    for msg := range msgs.Messages() {
                        // Process with worker
                        if err := worker.Process(ctx, msg); err != nil {
                            // DLQ handling
                            p.dlq.Send(msg, err)
                            msg.Nak()
                        } else {
                            msg.Ack()
                        }
                    }
                }
            }(i)
        }
    }

    return nil
}
```

#### Deliverables:
- [ ] NATS message deduplication configured
- [ ] SBOM worker in pool
- [ ] CVE worker in pool
- [ ] Pool handles all subscriptions
- [ ] Concurrency properly controlled
- [ ] No duplicate SBOM generation

---

### PHASE 5: Feedback Loop Implementation (Week 6)
**Duration:** 5 days
**Goal:** Add closed-loop remediation

#### 5.1: Add Remediation API
**New file:** `/core/internal/api/remediation_handlers.go`

```go
type RemediationRequest struct {
    TicketID  string `json:"ticket_id"`
    Assignee  string `json:"assignee"`
    Notes     string `json:"notes"`
    Status    string `json:"status"` // acknowledged | in_progress | remediated | false_positive
}

func (h *Handler) RemediateInsight(c *gin.Context) {
    id := c.Param("id")
    var req RemediationRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Update insight
    insight := &models.Insight{}
    if err := h.db.First(insight, "id = ?", id).Error; err != nil {
        c.JSON(404, gin.H{"error": "insight not found"})
        return
    }

    insight.RemediationTicketID = req.TicketID
    insight.RemediatedBy = req.Assignee
    insight.RemediationNotes = req.Notes
    insight.Status = req.Status

    if req.Status == "remediated" {
        now := time.Now()
        insight.RemediatedAt = &now
    }

    if err := h.db.Save(insight).Error; err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // Publish remediation event
    h.publisher.Publish("ksam.remediation.updated", insight)

    c.JSON(200, insight)
}
```

#### 5.2: Add Webhook Notifier
**New file:** `/core/pkg/webhook/notifier.go`

```go
type WebhookNotifier struct {
    endpoints []string
    client    *http.Client
}

func (n *WebhookNotifier) NotifyCriticalInsight(insight *models.Insight) error {
    if insight.Severity != "CRITICAL" {
        return nil
    }

    payload := map[string]interface{}{
        "type":      "insight.created",
        "severity":  insight.Severity,
        "title":     insight.Title,
        "resources": insight.AffectedResources,
        "detected":  insight.DetectedAt,
    }

    for _, endpoint := range n.endpoints {
        go n.send(endpoint, payload)
    }

    return nil
}
```

#### 5.3: Update Admission Webhook
**Files to modify:**
- `/core/internal/webhook/admission.go`

**Changes:**
```go
func (w *AdmissionWebhook) ValidateCreate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
    // Parse pod
    pod := &corev1.Pod{}
    json.Unmarshal(ar.Request.Object.Raw, pod)

    // Evaluate policies
    violations := w.policyEngine.Evaluate(pod)

    if len(violations) == 0 {
        return &admissionv1.AdmissionResponse{
            Allowed: true,
        }
    }

    // Check enforcement mode
    for _, v := range violations {
        if v.Enforcement == "deny" {
            return &admissionv1.AdmissionResponse{
                Allowed: false,
                Result: &metav1.Status{
                    Message: fmt.Sprintf("Policy violation: %s", v.Message),
                    Reason:  metav1.StatusReasonForbidden,
                },
            }
        } else if v.Enforcement == "warn" {
            // Add warning annotation
            pod.Annotations["fortuna.io/policy-warning"] = v.Message
        }
    }

    return &admissionv1.AdmissionResponse{
        Allowed: true,
        Warnings: []string{violations[0].Message},
    }
}
```

#### Deliverables:
- [ ] Remediation API endpoints
- [ ] Webhook notifier for critical insights
- [ ] Admission webhook enforcement
- [ ] Remediation fields in DB
- [ ] Frontend integration guide
- [ ] Webhook configuration docs

---

### PHASE 6: Testing & Migration (Week 6-7)
**Duration:** 10 days
**Goal:** Comprehensive testing and production migration

#### 6.1: Integration Tests
**New file:** `/core/test/integration/agent_flow_test.go`

```go
func TestAgentToInsightFlow(t *testing.T) {
    // 1. Start Core
    core := startCore(t)
    defer core.Stop()

    // 2. Start Agent
    agent := startAgent(t, core.GRPCAddr)
    defer agent.Stop()

    // 3. Create pod in cluster
    pod := createTestPod(t)

    // 4. Wait for agent to detect
    time.Sleep(5 * time.Second)

    // 5. Verify pod in DB
    var dbPod models.Pod
    err := core.DB.Where("uid = ?", string(pod.UID)).First(&dbPod).Error
    assert.NoError(t, err)

    // 6. Verify SBOM generated
    var sbom models.SBOM
    err = core.DB.Where("pod_uid = ?", string(pod.UID)).First(&sbom).Error
    assert.NoError(t, err)

    // 7. Verify CVE matches
    var matches []models.CVEMatch
    err = core.DB.Where("sbom_id = ?", sbom.ID).Find(&matches).Error
    assert.NoError(t, err)
    assert.Greater(t, len(matches), 0)

    // 8. Verify insights created
    var insights []models.Insight
    err = core.DB.Where("affected_resources LIKE ?", "%"+string(pod.UID)+"%").Find(&insights).Error
    assert.NoError(t, err)
    assert.Greater(t, len(insights), 0)
}
```

#### 6.2: Load Testing
**New file:** `/core/test/load/agent_load_test.go`

```go
func TestMultipleAgentsLoad(t *testing.T) {
    core := startCore(t)
    defer core.Stop()

    // Simulate 100 agents
    agents := make([]*Agent, 100)
    for i := 0; i < 100; i++ {
        agents[i] = startAgent(t, core.GRPCAddr)
        defer agents[i].Stop()
    }

    // Each agent sends 1000 items
    wg := sync.WaitGroup{}
    for i, agent := range agents {
        wg.Add(1)
        go func(a *Agent, idx int) {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                pod := generateTestPod(idx, j)
                a.Send(pod)
            }
        }(agent, i)
    }

    wg.Wait()

    // Verify ingestion rate
    var count int64
    core.DB.Model(&models.Pod{}).Count(&count)
    assert.Equal(t, int64(100*1000), count)
}
```

#### 6.3: Migration Plan
**Create:** `/docs/MIGRATION_GUIDE.md`

```markdown
# Migration to Agent-Based Architecture

## Pre-Migration Checklist
- [ ] Backup PostgreSQL database
- [ ] Export NATS streams
- [ ] Document current state (pod count, insights, etc.)
- [ ] Test rollback procedure
- [ ] Schedule maintenance window

## Migration Steps

### Step 1: Deploy Agents (Dual Mode)
```bash
# Enable hybrid mode
helm upgrade ksam ./helm/ksam \
  --set core.collectionMode=hybrid \
  --set agent.enabled=true

# Verify agents connected
kubectl logs -n fortuna -l app=ksam-agent | grep "Connected to core"
```

### Step 2: Verify Dual Collection
```bash
# Check metrics
curl http://ksam-core:8080/metrics | grep agent_connected
curl http://ksam-core:8080/metrics | grep ingestion_rate

# Compare data sources
kubectl exec -n fortuna ksam-core-0 -- \
  psql -U ksam -c "SELECT source, COUNT(*) FROM pods GROUP BY source"
```

### Step 3: Switch to Agent-Only
```bash
# Disable core collection
helm upgrade ksam ./helm/ksam \
  --set core.collectionMode=agent \
  --set core.directCollection=false

# Verify core stopped K8s client
kubectl logs -n fortuna ksam-core-0 | grep "agent-based collection mode"
```

### Step 4: Monitor
```bash
# Watch for errors
kubectl logs -n fortuna -l app=ksam-agent -f

# Check ingestion rate
watch -n 5 'curl -s http://ksam-core:8080/metrics | grep ingestion_total'
```

### Step 5: Cleanup (Optional)
```bash
# Remove core RBAC (no longer needed)
kubectl delete clusterrolebinding ksam-core-reader-binding
kubectl delete clusterrole ksam-core-reader
```

## Rollback Procedure
```bash
# Revert to core-only mode
helm rollback ksam

# Verify
kubectl get pods -n fortuna
```
```

#### Deliverables:
- [ ] Integration tests pass
- [ ] Load tests pass (100 agents)
- [ ] Migration guide complete
- [ ] Rollback tested
- [ ] Monitoring dashboards updated
- [ ] Production deployment plan

---

## File-by-File Changes

### Core Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `/core/cmd/main.go` | **MODIFY** | Add feature flags, conditional K8s client |
| `/core/internal/grpc/server.go` | **MODIFY** | Add session management |
| `/core/internal/grpc/handler_new.go` | **MODIFY** | Enhance StreamInventory with ACK/backpressure |
| `/core/internal/grpc/health.go` | **CREATE** | Add health check service |
| `/core/internal/ingest/ingest.go` | **MODIFY** | Add validation, enrichment, dedup cache |
| `/core/internal/api/remediation_handlers.go` | **CREATE** | Add remediation endpoints |
| `/core/internal/api/routes.go` | **MODIFY** | Add remediation routes |
| `/core/internal/webhook/admission.go` | **MODIFY** | Wire to policy engine, add enforcement |
| `/core/internal/collector/k8s_collector.go` | **CREATE** | Conditional K8s collector |
| `/core/pkg/worker/pool.go` | **MODIFY** | Handle all workers uniformly |
| `/core/pkg/worker/sbom_worker.go` | **MODIFY** | Use pool interface, remove manual subscription |
| `/core/pkg/worker/cve_matcher_worker.go` | **MODIFY** | Use pool interface |
| `/core/pkg/messaging/nats_client.go` | **MODIFY** | Add message deduplication |
| `/core/pkg/webhook/notifier.go` | **CREATE** | Add webhook notifications |
| `/core/pkg/models/insight.go` | **MODIFY** | Add remediation fields |
| `/core/internal/config/config.go` | **MODIFY** | Add collection mode config |

### Agent Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `/agent/cmd/main.go` | **MODIFY** | Enhanced startup with registration |
| `/agent/internal/watcher/pod_watcher.go` | **MODIFY** | Shared informer, local node filtering |
| `/agent/internal/watcher/serviceaccount_watcher.go` | **MODIFY** | Shared informer |
| `/agent/internal/watcher/rbac_watcher.go` | **MODIFY** | Shared informer |
| `/agent/internal/watcher/manager.go` | **CREATE** | Watcher manager with event batching |
| `/agent/internal/client/grpc_client_new.go` | **MODIFY** | Connection pooling, retry logic |
| `/agent/internal/client/connection_pool.go` | **CREATE** | gRPC connection pool |
| `/agent/internal/registration/registrar.go` | **CREATE** | Agent registration logic |
| `/agent/internal/buffer/disk_buffer.go` | **CREATE** | Offline buffer for network outages |
| `/agent/internal/collector/collector.go` | **MODIFY** | Event batching, compression |

### Infrastructure Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `/helm/ksam/values.yaml` | **MODIFY** | Add agent config, collection mode |
| `/helm/ksam/templates/agent-daemonset.yaml` | **MODIFY** | Update resources, node selector |
| `/helm/ksam/templates/agent-rbac.yaml` | **CREATE** | Agent RBAC |
| `/helm/ksam/templates/deployment.yaml` | **MODIFY** | Add feature flag env vars |
| `/helm/ksam/templates/configmap.yaml` | **MODIFY** | Add agent config |

### Documentation Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `/docs/REFACTORING_PLAN_AGENT_BASED.md` | **CREATE** | This document |
| `/docs/MIGRATION_GUIDE.md` | **CREATE** | Migration steps |
| `/docs/ARCHITECTURE_ANALYSIS.md` | **MODIFY** | Update with new architecture |
| `/docs/START_HERE.md` | **MODIFY** | Update deployment instructions |
| `/README.md` | **MODIFY** | Update overview |

### Test Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `/core/test/integration/agent_flow_test.go` | **CREATE** | End-to-end flow test |
| `/core/test/load/agent_load_test.go` | **CREATE** | Load testing |
| `/agent/test/unit/watcher_test.go` | **CREATE** | Watcher unit tests |

---

## Migration Strategy

### Timeline

```
Week 1: Preparation
├─ Day 1-2: Feature flags + metrics
├─ Day 3-4: Documentation + test infrastructure
└─ Day 5:   Review + validation

Week 2-3: Agent Enhancement
├─ Day 6-8:   Enhance watchers + gRPC client
├─ Day 9-11:  Agent registration + buffering
├─ Day 12-14: Unit tests + code review
└─ Day 15:    Agent deployment testing

Week 3-4: Core Enhancement
├─ Day 16-18: gRPC server + session management
├─ Day 19-21: IngestAPI + validation
├─ Day 22-24: Health checks + integration tests
└─ Day 25:    Performance testing

Week 4-5: Core Refactoring
├─ Day 26-27: Feature flag implementation
├─ Day 28-29: K8s client conditional
└─ Day 30:    Helm charts + RBAC

Week 5: Worker Pool
├─ Day 31-32: NATS deduplication
├─ Day 33-34: SBOM/CVE worker refactor
└─ Day 35:    Pool integration testing

Week 6: Feedback Loop
├─ Day 36-37: Remediation API
├─ Day 38-39: Webhook notifier + admission
└─ Day 40:    Frontend integration

Week 6-7: Testing & Migration
├─ Day 41-43: Integration tests + load tests
├─ Day 44-46: Staging migration
├─ Day 47-48: Production migration (blue/green)
└─ Day 49-50: Monitoring + cleanup
```

### Deployment Modes

#### Mode 1: Core-Only (Current/Legacy)
```yaml
core:
  collectionMode: "core"
  directCollection: true
agent:
  enabled: false
```

#### Mode 2: Hybrid (Migration)
```yaml
core:
  collectionMode: "hybrid"
  directCollection: true
agent:
  enabled: true
```

#### Mode 3: Agent-Only (Target)
```yaml
core:
  collectionMode: "agent"
  directCollection: false
agent:
  enabled: true
```

### Validation Checkpoints

**After Each Phase:**
1. Run integration tests
2. Check metrics dashboards
3. Verify data consistency
4. Review logs for errors
5. Performance benchmarking

**Before Production:**
1. Full regression test suite
2. Load test with 3x expected traffic
3. Failover testing
4. Security audit
5. Documentation review

---

## Testing Plan

### Unit Tests
- Agent watcher logic
- gRPC client retry/backpressure
- IngestAPI validation
- Deduplication cache
- Worker pool management

### Integration Tests
- Agent → Core flow
- SBOM generation from agent data
- CVE matching from agent-sourced SBOMs
- Insight creation end-to-end
- Remediation API workflow

### Load Tests
- 100 agents, 1000 items each
- Burst traffic (10k items/sec)
- Network interruption simulation
- Core restart during agent streaming
- Agent churn (pods restarting)

### Performance Tests
- Latency: Agent send → Core persist (target: <100ms p99)
- Throughput: Items/sec (target: 10k/sec)
- Memory: Core with 1000 agents (target: <8GB)
- CPU: Core with peak load (target: <4 cores)

### Security Tests
- mTLS certificate validation
- Rate limiting enforcement
- Input validation (malformed payloads)
- RBAC for agent ServiceAccount
- Admission webhook enforcement

---

## Rollback Strategy

### Immediate Rollback (< 5 minutes)
```bash
# Helm rollback to previous release
helm rollback ksam -n fortuna

# Verify
kubectl get pods -n fortuna
kubectl logs -n fortuna ksam-core-0 | grep "collection mode"
```

### Partial Rollback (Hybrid Mode)
```bash
# Switch back to hybrid mode
helm upgrade ksam ./helm/ksam \
  --set core.collectionMode=hybrid

# Keep agents running, re-enable core collection
```

### Data Recovery
```bash
# Restore database from backup
kubectl exec -n fortuna postgres-0 -- \
  psql -U ksam < backup-$(date +%Y%m%d).sql

# Restore NATS streams
nats stream restore ksam-events < nats-backup.tar.gz
```

### Rollback Decision Matrix

| Issue | Severity | Rollback Decision |
|-------|----------|-------------------|
| Agent connection failures | HIGH | Immediate rollback to hybrid |
| Data loss detected | CRITICAL | Full rollback + restore |
| Performance degradation | MEDIUM | Switch to hybrid, investigate |
| Single agent pod crash | LOW | Fix agent, no rollback |
| Core gRPC errors | HIGH | Rollback to core-only |

---

## Success Metrics

### Technical Metrics
- ✅ Agent connection success rate: >99%
- ✅ Data ingestion latency p99: <100ms
- ✅ No data loss (compared to baseline)
- ✅ Core CPU usage: <50% with 100 agents
- ✅ Core memory: <8GB with 1000 agents
- ✅ SBOM generation time: <30s per image

### Business Metrics
- ✅ Zero downtime during migration
- ✅ 100% feature parity with core-only mode
- ✅ Improved scalability (10x pod capacity)
- ✅ Reduced API server load (distributed watchers)

### Operational Metrics
- ✅ Mean time to detect (MTTD): <30s
- ✅ Mean time to insight (MTTI): <2min
- ✅ False positive rate: <5%
- ✅ Policy enforcement: 100% coverage

---

## Risk Mitigation

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Agent pod crashes | HIGH | MEDIUM | Health checks, auto-restart, buffer |
| Network partition | HIGH | LOW | Disk buffer, exponential retry |
| gRPC stream disconnect | MEDIUM | MEDIUM | Reconnect logic, state resumption |
| Core overwhelmed | HIGH | LOW | Rate limiting, backpressure signals |
| Data duplication | MEDIUM | MEDIUM | NATS message deduplication |
| Migration data loss | CRITICAL | LOW | Hybrid mode, validation, rollback |
| Performance regression | MEDIUM | MEDIUM | Load testing, gradual rollout |
| Security vulnerability | HIGH | LOW | mTLS, input validation, audit |

---

## Post-Migration Checklist

- [ ] All agents connected and healthy
- [ ] Core K8s client disabled (verify logs)
- [ ] Data ingestion rate matches baseline
- [ ] No errors in agent/core logs
- [ ] Metrics dashboards updated
- [ ] Documentation updated
- [ ] Team trained on new architecture
- [ ] Monitoring alerts configured
- [ ] Backup/restore tested
- [ ] Rollback procedure validated
- [ ] Performance baseline established
- [ ] Security audit passed

---

## Appendix

### A. Proto Definitions
```protobuf
// /core/api/proto/agent.proto

message InventoryItem {
    string uid = 1;
    string type = 2;
    string namespace = 3;
    bytes payload = 4;
    int64 resource_version = 5;
    google.protobuf.Timestamp timestamp = 6;
    map<string, string> metadata = 7;
    string agent_id = 8;
    string node_id = 9;
}

message StreamResponse {
    enum Type {
        ACK = 0;
        BACKPRESSURE = 1;
        HEARTBEAT = 2;
    }
    Type type = 1;
    int64 message_count = 2;
    string message = 3;
}

service AgentService {
    rpc RegisterAgent(RegisterAgentRequest) returns (RegisterAgentResponse);
    rpc StreamInventory(stream InventoryItem) returns (stream StreamResponse);
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
}
```

### B. Configuration Reference
```yaml
# /core/config/config.yaml

collection:
  mode: "agent"  # agent | core | hybrid

agent:
  enabled: true
  registration:
    required: true
    timeout: 30s
  rate_limit:
    items_per_second: 1000
    burst: 5000
  buffer:
    enabled: true
    max_size_gb: 10

grpc:
  server:
    port: 9090
    max_connections: 1000
    max_concurrent_streams: 100
  tls:
    enabled: true
    cert_path: /certs/server.crt
    key_path: /certs/server.key
    ca_path: /certs/ca.crt

ingest:
  deduplication:
    enabled: true
    ttl: 5m
  validation:
    enabled: true
    strict: false
  enrichment:
    enabled: true
```

### C. Metrics Reference
```
# Agent Metrics
ksam_agent_connected_total
ksam_agent_items_sent_total
ksam_agent_send_errors_total
ksam_agent_buffer_size_bytes
ksam_agent_connection_duration_seconds

# Core Metrics
ksam_ingestion_rate_items_per_second
ksam_ingestion_errors_total
ksam_active_agent_sessions
ksam_grpc_stream_duration_seconds
ksam_deduplication_cache_hits_total
```

---

**END OF REFACTORING PLAN**
