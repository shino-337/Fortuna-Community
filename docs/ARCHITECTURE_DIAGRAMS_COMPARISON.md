# KSAM Architecture Diagrams - Current vs Proposed

**Date:** 2025-12-22
**Purpose:** Visual comparison of current Core-centric vs proposed Agent-based architecture

---

## Table of Contents
1. [High-Level Architecture Comparison](#high-level-architecture-comparison)
2. [Data Flow Comparison](#data-flow-comparison)
3. [Component Interaction Diagrams](#component-interaction-diagrams)
4. [Deployment Architecture](#deployment-architecture)
5. [Scaling Scenarios](#scaling-scenarios)

---

## High-Level Architecture Comparison

### CURRENT: Core-Centric Architecture (PROBLEMATIC)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          KUBERNETES CLUSTER                              │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    Kubernetes API Server                           │ │
│  │                                                                    │ │
│  │  • Pods                    • RBAC Resources                        │ │
│  │  • ServiceAccounts         • Namespaces                            │ │
│  │  • Deployments             • Events                                │ │
│  └───────────────────┬────────────────────────────────────────────────┘ │
│                      │                                                   │
│                      │ ⚠️ SINGLE CLIENT (All reads from one pod)        │
│                      │ ⚠️ HIGH API SERVER LOAD                          │
│                      │ ⚠️ BOTTLENECK: All collection in one place       │
│                      ↓                                                   │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                       KSAM CORE POD                                │ │
│  │  ┌──────────────────────────────────────────────────────────────┐ │ │
│  │  │  🔴 PROBLEM: TOO MANY RESPONSIBILITIES                       │ │ │
│  │  ├──────────────────────────────────────────────────────────────┤ │ │
│  │  │  1. COLLECTION     ← Watch K8s API (list/watch)              │ │ │
│  │  │  2. VALIDATION     ← Validate incoming data                  │ │ │
│  │  │  3. PERSISTENCE    ← Write to PostgreSQL                     │ │ │
│  │  │  4. PUBLISHING     ← Publish NATS events                     │ │ │
│  │  │  5. PROCESSING     ← Run worker pool                         │ │ │
│  │  │  6. RISK ANALYSIS  ← Calculate risk scores                   │ │ │
│  │  │  7. API SERVING    ← REST/gRPC endpoints                     │ │ │
│  │  │  8. SBOM GENERATION← Extract container packages              │ │ │
│  │  │  9. CVE MATCHING   ← Match vulnerabilities                   │ │ │
│  │  └──────────────────────────────────────────────────────────────┘ │ │
│  │                                                                    │ │
│  │  Resource Usage:                                                   │ │
│  │  • CPU: HIGH (doing everything)                                    │ │
│  │  • Memory: HIGH (maintaining all watchers + processing)            │ │
│  │  • Network: HIGH (all K8s API traffic)                             │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │               KSAM AGENT (DaemonSet) - DISABLED                    │ │
│  │  ┌──────────────────────────────────────────────────────────────┐ │ │
│  │  │  ❌ EXISTS BUT NOT USED                                      │ │ │
│  │  │  ❌ WOULD BE REDUNDANT (Core already collects)               │ │ │
│  │  │  ❌ NO CLEAR ROLE IN ARCHITECTURE                            │ │ │
│  │  │  ❌ CODE MAINTAINED BUT NEVER DEPLOYED                       │ │ │
│  │  └──────────────────────────────────────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘

ISSUES:
1. Single point of failure (Core pod crash = no collection)
2. Scalability limited (Core can't scale horizontally for collection)
3. High resource usage in one pod
4. Violates separation of concerns
5. Agent component wasted/unclear purpose
6. API server overload from single client
```

---

### PROPOSED: Agent-Based Distributed Architecture (SOLUTION)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          KUBERNETES CLUSTER                              │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    Kubernetes API Server                           │ │
│  │                                                                    │ │
│  │  • Pods                    • RBAC Resources                        │ │
│  │  • ServiceAccounts         • Namespaces                            │ │
│  │  • Deployments             • Events                                │ │
│  └─┬─────────┬─────────┬─────────┬────────────────────────────────────┘ │
│    │         │         │         │                                      │
│    │ ✅ DISTRIBUTED WATCHERS (Load balanced across nodes)              │
│    │ ✅ LOWER API SERVER LOAD                                           │
│    │ ✅ HORIZONTAL SCALABILITY                                          │
│    │         │         │         │                                      │
│    ↓         ↓         ↓         ↓                                      │
│  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐                                   │
│  │Agent│  │Agent│  │Agent│  │Agent│  ... (1 per node)                 │
│  │ A   │  │ B   │  │ C   │  │ D   │                                   │
│  └──┬──┘  └──┬──┘  └──┬──┘  └──┬──┘                                   │
│     │        │        │        │                                        │
│     │ RESPONSIBILITY: COLLECT ONLY                                      │
│     │ • Watch local node pods (filtered)                                │
│     │ • Watch ServiceAccounts (distributed)                             │
│     │ • Watch RBAC (one designated agent)                               │
│     │ • Convert to proto                                                │
│     │ • Stream to Core                                                  │
│     │                                                                    │
│     │        │        │        │                                        │
│     │        │        │        │  gRPC Stream (TLS/mTLS)                │
│     │        │        │        │  Bidirectional                         │
│     └────────┴────────┴────────┘                                        │
│                      │                                                   │
│                      ↓                                                   │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                       KSAM CORE POD                                │ │
│  │  ┌──────────────────────────────────────────────────────────────┐ │ │
│  │  │  ✅ CLEAR RESPONSIBILITY: PROCESS & ANALYZE                  │ │ │
│  │  ├──────────────────────────────────────────────────────────────┤ │ │
│  │  │  1. RECEIVE         ← gRPC streaming from agents             │ │ │
│  │  │  2. VALIDATE        ← Check data integrity                   │ │ │
│  │  │  3. DEDUPLICATE     ← Cache-based deduplication              │ │ │
│  │  │  4. PERSIST         ← Write to PostgreSQL                    │ │ │
│  │  │  5. PUBLISH         ← Publish NATS events                    │ │ │
│  │  │  6. PROCESS         ← Run worker pool                        │ │ │
│  │  │  7. ANALYZE         ← Risk scoring, insights                 │ │ │
│  │  │  8. SERVE           ← REST/gRPC API for dashboards           │ │ │
│  │  │  9. ENFORCE         ← Admission webhook                      │ │ │
│  │  └──────────────────────────────────────────────────────────────┘ │ │
│  │                                                                    │ │
│  │  Resource Usage:                                                   │ │
│  │  • CPU: MEDIUM (no K8s watchers)                                   │ │
│  │  • Memory: MEDIUM (no K8s informer caches)                         │ │
│  │  • Network: MEDIUM (only gRPC receive)                             │ │
│  │                                                                    │ │
│  │  ✅ CAN SCALE HORIZONTALLY (stateless processing)                  │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘

BENEFITS:
1. Separation of concerns (Collection vs Processing)
2. Horizontal scalability (more nodes = more collectors)
3. Lower resource usage per component
4. Fault tolerance (agent failure = only that node affected)
5. Lower API server load (distributed watchers)
6. Clear architectural boundaries
```

---

## Data Flow Comparison

### CURRENT: Direct Collection Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        CURRENT DATA FLOW                                │
│                        (Core-Centric)                                   │
└─────────────────────────────────────────────────────────────────────────┘

STEP 1: Collection (PROBLEMATIC)
════════════════════════════════════════════════════════════════════════════

    K8s API Server
         │
         │ List/Watch ALL resources
         │ from SINGLE client
         │ ⚠️ Heavy load on API server
         │
         ↓
    ┌────────────┐
    │ Core Pod   │  ← Maintains ALL informers
    │            │  ← Caches ALL resources in memory
    │ K8s Client │  ← Single point of bottleneck
    └─────┬──────┘
          │
          │ Immediate processing
          │ (no buffering)
          ↓

STEP 2: Processing (TIGHTLY COUPLED)
════════════════════════════════════════════════════════════════════════════

    ┌────────────┐
    │ Core Pod   │
    │            │
    │  Direct    │  ← Immediate write to DB
    │  Insert    │  ← No event queue (synchronous)
    └─────┬──────┘
          │
          ↓
    PostgreSQL
          │
          ↓
    NATS Event Published
          │
          ↓
    Workers Process

⚠️ ISSUES:
   • If DB slow, entire collection blocks
   • If NATS down, data lost
   • No buffering or retry
   • Tight coupling between collection and processing
```

---

### PROPOSED: Agent-Based Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     PROPOSED DATA FLOW                                  │
│                     (Agent-Based)                                       │
└─────────────────────────────────────────────────────────────────────────┘

STEP 1: Distributed Collection
════════════════════════════════════════════════════════════════════════════

    K8s API Server
         │
    ┌────┼────┬────┬────┬───── ... ───┐
    │    │    │    │    │              │  ✅ Multiple clients (distributed)
    │    │    │    │    │              │  ✅ Load balanced
    ↓    ↓    ↓    ↓    ↓              ↓
  Agent Agent Agent Agent Agent ... Agent
   A     B     C     D     E          Z
    │    │    │    │    │              │
    │    │    │    │    │              │  Each agent:
    │    │    │    │    │              │  • Watches subset of resources
    │    │    │    │    │              │  • Local node filtering
    │    │    │    │    │              │  • Batches events
    │    │    │    │    │              │
    └────┴────┴────┴────┴──────────────┘
                   │
                   │ Batch events (max 100, max 5s)
                   │ Convert to proto
                   ↓

STEP 2: Streaming (DECOUPLED)
════════════════════════════════════════════════════════════════════════════

         gRPC Bidirectional Stream
              (TLS encrypted)
                   │
                   │  Features:
                   │  • Buffered (10MB in-memory)
                   │  • Disk spillover (if offline)
                   │  • Compression (if >1MB)
                   │  • Retry with backoff
                   │  • ACK mechanism
                   │
                   ↓
         ┌──────────────────┐
         │   Core gRPC      │
         │   Server         │
         │                  │  ✅ Receives from multiple agents
         │  Rate Limiter    │  ✅ Rate limits per agent
         │  (1000/s/agent)  │  ✅ Backpressure signals
         └────────┬─────────┘
                  │
                  ↓

STEP 3: Validation & Enrichment (ROBUST)
════════════════════════════════════════════════════════════════════════════

         ┌──────────────────┐
         │   IngestAPI      │
         │                  │
         │  1. Validate     │  ← JSON schema validation
         │  2. Deduplicate  │  ← Cache-based (UID+version)
         │  3. Enrich       │  ← Add cluster/namespace metadata
         │  4. Persist      │  ← Write to DB
         │  5. Publish      │  ← NATS event
         └────────┬─────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
        ↓                   ↓
   PostgreSQL         NATS JetStream
   (Durable)          (Event Log)
        │                   │
        │                   │
        └─────────┬─────────┘
                  │
                  ↓

STEP 4: Event-Driven Processing (SCALABLE)
════════════════════════════════════════════════════════════════════════════

    NATS Topics: ksam.raw.*
                  │
       ┌──────────┼──────────┐
       │          │          │
       ↓          ↓          ↓
  Normalizer  Correlator  Risk Worker
  (5 workers) (5 workers) (5 workers)
       │          │          │
       └──────────┼──────────┘
                  │
    ksam.normalized.*
                  │
       ┌──────────┼──────────┐
       │          │          │
       ↓          ↓          ↓
  SBOM Worker  CVE Matcher  Insight Mgr
       │          │          │
       │          │          │
       └──────────┼──────────┘
                  │
                  ↓
              Insights
              Risk Scores
              Dashboard

✅ BENEFITS:
   • Async processing (no blocking)
   • Horizontal scaling (add more workers)
   • Fault tolerant (worker restart = resume)
   • Backpressure handling
   • Clear event boundaries
```

---

## Component Interaction Diagrams

### Sequence Diagram: Pod Creation Flow

#### CURRENT (Core-Centric)
```
User          K8s API       Core          PostgreSQL    NATS     Workers
 │              │            │                │          │         │
 │ Create Pod   │            │                │          │         │
 ├─────────────>│            │                │          │         │
 │              │            │                │          │         │
 │              │  Watch     │                │          │         │
 │              │  Event     │                │          │         │
 │              ├───────────>│                │          │         │
 │              │            │                │          │         │
 │              │            │ INSERT Pod     │          │         │
 │              │            ├───────────────>│          │         │
 │              │            │                │          │         │
 │              │            │ ACK            │          │         │
 │              │            │<───────────────┤          │         │
 │              │            │                │          │         │
 │              │            │ PUBLISH        │          │         │
 │              │            │ ksam.raw.pods  │          │         │
 │              │            ├────────────────┼─────────>│         │
 │              │            │                │          │         │
 │              │            │                │          │  RECV   │
 │              │            │                │          ├────────>│
 │              │            │                │          │         │
 │              │            │ ⚠️ If DB slow, entire flow blocks    │
 │              │            │ ⚠️ No retry if NATS publish fails    │
 │              │            │ ⚠️ Tight coupling                    │

Time: ~50ms (p99)
```

#### PROPOSED (Agent-Based)
```
User      K8s API    Agent     Core      IngestAPI    DB      NATS   Workers
 │          │         │         │            │         │       │       │
 │ Create   │         │         │            │         │       │       │
 │ Pod      │         │         │            │         │       │       │
 ├─────────>│         │         │            │         │       │       │
 │          │         │         │            │         │       │       │
 │          │ Watch   │         │            │         │       │       │
 │          │ Event   │         │            │         │       │       │
 │          ├────────>│         │            │         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │ Buffer  │            │         │       │       │
 │          │         │ (local) │            │         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │ Batch   │            │         │       │       │
 │          │         │ Ready   │            │         │       │       │
 │          │         │ (100ms) │            │         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │ gRPC    │            │         │       │       │
 │          │         │ Send    │            │         │       │       │
 │          │         ├────────>│            │         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │         │ Validate   │         │       │       │
 │          │         │         ├───────────>│         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │         │ Enrich     │         │       │       │
 │          │         │         │ Dedup      │         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │         │ Persist    │         │       │       │
 │          │         │         │            ├────────>│       │       │
 │          │         │         │            │         │       │       │
 │          │         │         │            │ Publish │       │       │
 │          │         │         │            ├─────────┼──────>│       │
 │          │         │         │            │         │       │       │
 │          │         │ ACK     │            │         │       │ RECV  │
 │          │         │<────────┤            │         │       ├──────>│
 │          │         │         │            │         │       │       │
 │          │         │         │            │         │       │       │
 │          │         │ ✅ Agent continues collecting while Core processes │
 │          │         │ ✅ Buffered (no blocking)                           │
 │          │         │ ✅ Retry on failure                                 │
 │          │         │ ✅ Disk spillover if offline                        │

Time: ~80ms (p99) - Slightly higher but non-blocking
```

---

## Deployment Architecture

### CURRENT: Core-Only Deployment

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CLUSTER                              │
│                                                                         │
│  Namespace: fortuna                                                     │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                                                                   │ │
│  │  ┌─────────────────────────────────────────────────────────┐     │ │
│  │  │  Deployment: ksam-core                                  │     │ │
│  │  │  Replicas: 1 (⚠️ Cannot scale for collection)           │     │ │
│  │  │                                                          │     │ │
│  │  │  ┌───────────────────────────────────────────────────┐  │     │ │
│  │  │  │  Pod: ksam-core-0                                │  │     │ │
│  │  │  │                                                  │  │     │ │
│  │  │  │  Containers:                                     │  │     │ │
│  │  │  │  ┌──────────────────────────────────────────┐   │  │     │ │
│  │  │  │  │  ksam-core                              │   │  │     │ │
│  │  │  │  │                                          │   │  │     │ │
│  │  │  │  │  Resources:                              │   │  │     │ │
│  │  │  │  │    CPU: 2 cores                          │   │  │     │ │
│  │  │  │  │    Memory: 4Gi                           │   │  │     │ │
│  │  │  │  │                                          │   │  │     │ │
│  │  │  │  │  Responsibilities:                       │   │  │     │ │
│  │  │  │  │  • K8s Client (watchers)                 │   │  │     │ │
│  │  │  │  │  • gRPC Server (9090)                    │   │  │     │ │
│  │  │  │  │  • REST API (8080)                       │   │  │     │ │
│  │  │  │  │  • Worker Pool                           │   │  │     │ │
│  │  │  │  │  • SBOM Generation                       │   │  │     │ │
│  │  │  │  │  • CVE Matching                          │   │  │     │ │
│  │  │  │  │  • Risk Scoring                          │   │  │     │ │
│  │  │  │  └──────────────────────────────────────────┘   │  │     │ │
│  │  │  │                                                  │  │     │ │
│  │  │  │  Volumes:                                        │  │     │ │
│  │  │  │  • /certs (TLS certificates)                     │  │     │ │
│  │  │  │  • /rules (Policy YAML files)                    │  │     │ │
│  │  │  └───────────────────────────────────────────────────┘  │     │ │
│  │  │                                                          │     │ │
│  │  └─────────────────────────────────────────────────────────┘     │ │
│  │                                                                   │ │
│  │  ┌─────────────────────────────────────────────────────────┐     │ │
│  │  │  StatefulSet: postgres                                  │     │ │
│  │  │  Replicas: 1                                            │     │ │
│  │  │                                                          │     │ │
│  │  │  ┌───────────────────────────────────────────────────┐  │     │ │
│  │  │  │  Pod: postgres-0                                 │  │     │ │
│  │  │  │  Resources: CPU 1, Memory 2Gi                    │  │     │ │
│  │  │  │  PVC: 100Gi                                      │  │     │ │
│  │  │  └───────────────────────────────────────────────────┘  │     │ │
│  │  └─────────────────────────────────────────────────────────┘     │ │
│  │                                                                   │ │
│  │  ┌─────────────────────────────────────────────────────────┐     │ │
│  │  │  StatefulSet: nats                                      │     │ │
│  │  │  Replicas: 1                                            │     │ │
│  │  │                                                          │     │ │
│  │  │  ┌───────────────────────────────────────────────────┐  │     │ │
│  │  │  │  Pod: nats-0                                     │  │     │ │
│  │  │  │  Resources: CPU 500m, Memory 1Gi                 │  │     │ │
│  │  │  │  PVC: 10Gi (JetStream)                           │  │     │ │
│  │  │  └───────────────────────────────────────────────────┘  │     │ │
│  │  └─────────────────────────────────────────────────────────┘     │ │
│  │                                                                   │ │
│  │  ⚠️ AGENT DAEMONSET: NOT DEPLOYED                                │ │
│  │                                                                   │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  Total Resource Usage:                                                 │
│  • Pods: 3                                                             │
│  • CPU: 3.5 cores                                                      │
│  • Memory: 7Gi                                                         │
│  • Storage: 110Gi                                                      │
│                                                                         │
│  ⚠️ ISSUES:                                                             │
│  • Core pod is single point of failure for collection                  │
│  • Cannot scale Core horizontally for collection                       │
│  • High resource usage in single pod                                   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

### PROPOSED: Agent-Based Deployment

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CLUSTER                              │
│                   (5 Nodes for this example)                            │
│                                                                         │
│  Namespace: fortuna                                                     │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                                                                   │ │
│  │  ┌─────────────────────────────────────────────────────────────┐ │ │
│  │  │  DaemonSet: ksam-agent                                      │ │ │
│  │  │  Replicas: 5 (1 per node)                                   │ │ │
│  │  │                                                              │ │ │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │ │ │
│  │  │  │ Agent A  │  │ Agent B  │  │ Agent C  │  │ Agent D  │ ...│ │ │
│  │  │  │ Node-1   │  │ Node-2   │  │ Node-3   │  │ Node-4   │   │ │ │
│  │  │  ├──────────┤  ├──────────┤  ├──────────┤  ├──────────┤   │ │ │
│  │  │  │Resources:│  │Resources:│  │Resources:│  │Resources:│   │ │ │
│  │  │  │CPU: 100m │  │CPU: 100m │  │CPU: 100m │  │CPU: 100m │   │ │ │
│  │  │  │Mem: 128Mi│  │Mem: 128Mi│  │Mem: 128Mi│  │Mem: 128Mi│   │ │ │
│  │  │  │          │  │          │  │          │  │          │   │ │ │
│  │  │  │Watches:  │  │Watches:  │  │Watches:  │  │Watches:  │   │ │ │
│  │  │  │• Local   │  │• Local   │  │• Local   │  │• Local   │   │ │ │
│  │  │  │  pods    │  │  pods    │  │  pods    │  │  pods    │   │ │ │
│  │  │  │• SA      │  │• SA      │  │• SA      │  │• RBAC    │   │ │ │
│  │  │  │  (shard) │  │  (shard) │  │  (shard) │  │  (leader)│   │ │ │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │ │ │
│  │  │       │             │             │             │          │ │ │
│  │  │       └─────────────┴─────────────┴─────────────┘          │ │ │
│  │  │                          │                                 │ │ │
│  │  │                          │ gRPC Streams                    │ │ │
│  │  │                          │ (Bidirectional)                 │ │ │
│  │  └──────────────────────────┼─────────────────────────────────┘ │ │
│  │                             ↓                                   │ │
│  │  ┌─────────────────────────────────────────────────────────┐   │ │
│  │  │  Deployment: ksam-core                                  │   │ │
│  │  │  Replicas: 2 (✅ Can scale for processing)              │   │ │
│  │  │                                                          │   │ │
│  │  │  ┌───────────────────────┐  ┌───────────────────────┐   │   │ │
│  │  │  │ Pod: ksam-core-0      │  │ Pod: ksam-core-1      │   │   │ │
│  │  │  │                       │  │                       │   │   │ │
│  │  │  │ Resources:            │  │ Resources:            │   │   │ │
│  │  │  │   CPU: 1 core         │  │   CPU: 1 core         │   │   │ │
│  │  │  │   Memory: 2Gi         │  │   Memory: 2Gi         │   │   │ │
│  │  │  │                       │  │                       │   │   │ │
│  │  │  │ Responsibilities:     │  │ Responsibilities:     │   │   │ │
│  │  │  │ • gRPC Server (9090)  │  │ • gRPC Server (9090)  │   │   │ │
│  │  │  │ • REST API (8080)     │  │ • REST API (8080)     │   │   │ │
│  │  │  │ • IngestAPI           │  │ • IngestAPI           │   │   │ │
│  │  │  │ • Worker Pool         │  │ • Worker Pool         │   │   │ │
│  │  │  │ • SBOM Generation     │  │ • SBOM Generation     │   │   │ │
│  │  │  │ • CVE Matching        │  │ • CVE Matching        │   │   │ │
│  │  │  │ • Risk Scoring        │  │ • Risk Scoring        │   │   │ │
│  │  │  │                       │  │                       │   │   │ │
│  │  │  │ ❌ NO K8s Client      │  │ ❌ NO K8s Client      │   │   │ │
│  │  │  └───────────────────────┘  └───────────────────────┘   │   │ │
│  │  └─────────────────────────────────────────────────────────┘   │ │
│  │                                                                 │ │
│  │  ┌─────────────────────────────────────────────────────────┐   │ │
│  │  │  StatefulSet: postgres                                  │   │ │
│  │  │  (Same as before)                                       │   │ │
│  │  └─────────────────────────────────────────────────────────┘   │ │
│  │                                                                 │ │
│  │  ┌─────────────────────────────────────────────────────────┐   │ │
│  │  │  StatefulSet: nats                                      │   │ │
│  │  │  (Same as before)                                       │   │ │
│  │  └─────────────────────────────────────────────────────────┘   │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  Total Resource Usage:                                             │
│  • Pods: 9 (5 agents + 2 core + 1 postgres + 1 nats)              │
│  • CPU: 3.5 cores (5x100m + 2x1 + 1x1 + 1x500m)                   │
│  • Memory: 5.64Gi (5x128Mi + 2x2Gi + 1x2Gi + 1x1Gi)               │
│  • Storage: 110Gi (same)                                           │
│                                                                     │
│  ✅ BENEFITS:                                                       │
│  • Collection distributed across 5 nodes                            │
│  • Core can scale to 3+ replicas for processing                    │
│  • Lower per-pod resource usage                                    │
│  • Fault tolerant (agent failure = 1 node, not entire cluster)     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Scaling Scenarios

### Scenario 1: Cluster Growth (100 → 1000 pods)

#### CURRENT (Core-Centric)
```
┌────────────────────────────────────────────────────────────────┐
│  Cluster Size: 100 → 1000 pods                                 │
│  Growth: 10x                                                   │
└────────────────────────────────────────────────────────────────┘

Core Pod Resource Usage:

  100 pods                1000 pods
  ┌───────┐               ┌───────┐
  │ CPU:  │               │ CPU:  │
  │ 500m  │    ──────>    │ 4000m │  ❌ 8x increase!
  └───────┘               └───────┘

  ┌───────┐               ┌───────┐
  │Memory:│               │Memory:│
  │ 1Gi   │    ──────>    │ 8Gi   │  ❌ 8x increase!
  └───────┘               └───────┘

K8s API Server Load:

  Requests/sec            Requests/sec
  ┌───────┐               ┌───────┐
  │  10   │    ──────>    │  100  │  ❌ 10x increase from single client!
  └───────┘               └───────┘

⚠️ RESULT: Must vertically scale Core pod (limits at node capacity)
```

#### PROPOSED (Agent-Based)
```
┌────────────────────────────────────────────────────────────────┐
│  Cluster Size: 100 → 1000 pods                                 │
│  Nodes: 10 → 50 (assume 20 pods/node)                         │
│  Growth: 10x pods, 5x nodes                                    │
└────────────────────────────────────────────────────────────────┘

Agent Pod Resource Usage (per agent):

  100 pods / 10 nodes     1000 pods / 50 nodes
  (10 pods/node)          (20 pods/node)
  ┌───────┐               ┌───────┐
  │ CPU:  │               │ CPU:  │
  │ 100m  │    ──────>    │ 150m  │  ✅ Only 1.5x increase per agent!
  └───────┘               └───────┘

  ┌───────┐               ┌───────┐
  │Memory:│               │Memory:│
  │ 128Mi │    ──────>    │ 192Mi │  ✅ Only 1.5x increase per agent!
  └───────┘               └───────┘

Core Pod Resource Usage (no K8s client):

  ┌───────┐               ┌───────┐
  │ CPU:  │               │ CPU:  │
  │ 1 core│    ──────>    │ 2 cores│  ✅ Scale horizontally (2 → 4 pods)
  └───────┘               └───────┘

K8s API Server Load (distributed across agents):

  Requests/sec            Requests/sec
  from single client      from 50 clients
  ┌───────┐               ┌───────┐
  │  10   │    ──────>    │  100  │  ✅ Load distributed!
  └───────┘               └───────┘
                          (2 req/sec per agent)

✅ RESULT: Horizontal scaling (more nodes = more agents automatically)
```

---

### Scenario 2: Core Pod Failure

#### CURRENT (Core-Centric)
```
Time: 0s                 Time: 10s               Time: 60s
┌───────────┐            ┌───────────┐           ┌───────────┐
│ Core Pod  │            │ Core Pod  │           │ Core Pod  │
│ ┌───────┐ │            │ ❌ CRASH  │           │ ┌───────┐ │
│ │Running│ │   ────>    │           │   ────>   │ │Restart│ │
│ └───────┘ │            │           │           │ └───────┘ │
└───────────┘            └───────────┘           └───────────┘
      │                        │                        │
      │                        │                        │
   Collect                  ❌ STOPPED                Resume
   Process                  ❌ DATA LOST           Re-watch K8s
   API Serve                ❌ API DOWN

⚠️ IMPACT:
   • Collection stopped for 50s (pod restart time)
   • Events during restart window LOST
   • Dashboard down
   • No new insights generated
   • Risk scores stale
```

#### PROPOSED (Agent-Based)
```
Time: 0s                 Time: 10s               Time: 20s
┌───────────┐            ┌───────────┐           ┌───────────┐
│ Core Pod  │            │ Core Pod  │           │ Core Pod  │
│ ┌───────┐ │            │ ❌ CRASH  │           │ ┌───────┐ │
│ │Running│ │   ────>    │           │   ────>   │ │Restart│ │
│ └───────┘ │            │           │           │ └───────┘ │
└─────┬─────┘            └─────┬─────┘           └─────┬─────┘
      │                        │                        │
      ↑                        ↑                        ↑
      │                        │                        │
┌─────┴─────┐            ┌─────┴─────┐           ┌─────┴─────┐
│  Agents   │            │  Agents   │           │  Agents   │
│           │            │           │           │           │
│ ✅ STILL  │            │ ✅ BUFFER │           │ ✅ RESUME │
│  RUNNING  │            │  LOCALLY  │           │  SENDING  │
│           │            │  (disk)   │           │           │
└───────────┘            └───────────┘           └───────────┘

✅ IMPACT:
   • Collection continues (agents still watch)
   • Data buffered to disk (no loss)
   • When Core restarts, agents replay buffer
   • Dashboard down for 20s (faster restart without K8s watchers)
   • Minimal data delay
```

---

### Scenario 3: Agent Pod Failure

#### PROPOSED (Agent-Based)
```
Cluster: 50 nodes, 1000 pods
Agent D on Node-4 crashes

Time: 0s                     Time: 10s
┌──────────────────┐         ┌──────────────────┐
│  50 Agents       │         │  50 Agents       │
│                  │         │                  │
│ Agent A ✅       │         │ Agent A ✅       │
│ Agent B ✅       │         │ Agent B ✅       │
│ Agent C ✅       │         │ Agent C ✅       │
│ Agent D ✅       │  ────>  │ Agent D ❌ CRASH │  ← Only 1 node affected
│ Agent E ✅       │         │ Agent E ✅       │
│ ... (46 more)    │         │ ... (46 more)    │
└──────────────────┘         └──────────────────┘

⚠️ IMPACT:
   • Only Node-4 pods not collected (20 pods)
   • Other 980 pods still collected normally
   • 98% collection rate maintained
   • DaemonSet auto-restarts Agent D (30s)
   • When restarted, Agent D catches up via K8s watch

✅ FAULT ISOLATION:
   • Blast radius: 2% of cluster
   • No cascading failure
   • Self-healing (DaemonSet)
```

---

## Network Traffic Comparison

### CURRENT (Core-Centric)
```
┌─────────────────────────────────────────────────────────────┐
│              K8s API Server Network Traffic                 │
└─────────────────────────────────────────────────────────────┘

  All traffic from single IP (Core pod)

  ┌─────────────────────────────────────────┐
  │  Watchers:                              │
  │  • Pods           (list + watch)        │
  │  • ServiceAccounts (list + watch)       │
  │  • Roles          (list + watch)        │
  │  • RoleBindings   (list + watch)        │
  │  • ClusterRoles   (list + watch)        │
  │  • ClusterRoleBindings (list + watch)   │
  │  • Namespaces     (list + watch)        │
  └─────────────────────────────────────────┘
           │
           │  All from 10.244.1.15 (Core pod IP)
           │  Request rate: ~100 req/s
           │
           ↓
  ┌─────────────────────────────────────────┐
  │  K8s API Server                         │
  │                                         │
  │  Single client workload                 │
  │  Potential rate limiting                │
  │  All eggs in one basket                 │
  └─────────────────────────────────────────┘

⚠️ RISK: API server rate limits single client
```

---

### PROPOSED (Agent-Based)
```
┌─────────────────────────────────────────────────────────────┐
│              K8s API Server Network Traffic                 │
└─────────────────────────────────────────────────────────────┘

  Traffic distributed across 50 agent IPs

  Agent A (10.244.1.20)        Agent B (10.244.2.30)
  • Pods on Node-1            • Pods on Node-2
  ~2 req/s                    ~2 req/s
           │                           │
           └───────────┬───────────────┘
                       │
           ... (48 more agents)
                       │
                       ↓
  ┌─────────────────────────────────────────┐
  │  K8s API Server                         │
  │                                         │
  │  Load balanced across clients           │
  │  Total: ~100 req/s (same as before)     │
  │  Per client: ~2 req/s                   │
  └─────────────────────────────────────────┘

✅ BENEFITS:
   • Distributed load
   • No single client rate limiting
   • Graceful degradation
```

---

## Summary Table

| Aspect | Current (Core-Centric) | Proposed (Agent-Based) |
|--------|------------------------|------------------------|
| **Collection** | Single Core pod | Distributed agents (1 per node) |
| **Scalability** | Vertical (limited by node) | Horizontal (scales with nodes) |
| **Fault Tolerance** | Single point of failure | Isolated failures (per node) |
| **Resource Usage** | High (single pod) | Low (distributed) |
| **API Server Load** | High (single client) | Low (distributed clients) |
| **Data Loss Risk** | High (no buffer on crash) | Low (disk buffer) |
| **Deployment Complexity** | Simple (1 deployment) | Moderate (+ DaemonSet) |
| **Core Pod Restart** | 50s downtime, data lost | 20s downtime, buffered |
| **Agent Pod Restart** | N/A | 30s, 2% impact |
| **Processing Scalability** | Coupled with collection | Independent (can scale Core) |
| **Network Traffic** | Centralized | Distributed |
| **Clear Architecture** | ❌ Mixed concerns | ✅ Separated concerns |

---

**Conclusion:** The agent-based architecture provides clear separation of concerns, better scalability, and fault tolerance at the cost of slightly increased deployment complexity. The trade-off is worthwhile for production environments.
