# KSAM Implementation Plan

**Document Version**: 1.0  
**Last Updated**: 2025-11-28  
**Planning Horizon**: Q1 2025 - Q4 2025

---

## Executive Summary

This document provides a detailed, sprint-by-sprint implementation plan for KSAM (Kubernetes Service Account Manager). The plan is divided into 4 major phases (MVP-1 through MVP-4) spanning 12 months, with each sprint lasting 2 weeks.

**Current Status**: MVP-1 at 80% completion  
**Next Milestone**: Complete MVP-1 (3 weeks)  
**Team Size**: Assumed 3-4 engineers

---

## Table of Contents

- [Sprint Calendar](#sprint-calendar)
- [MVP-1: Foundation](#mvp-1-foundation)
- [MVP-2: Observability & Graph Engine](#mvp-2-observability--graph-engine)
- [MVP-3: Policy Automation & Attack Simulation](#mvp-3-policy-automation--attack-simulation)
- [MVP-4: Enterprise Features](#mvp-4-enterprise-features)
- [Dependencies & Critical Path](#dependencies--critical-path)
- [Risk Mitigation](#risk-mitigation)
- [Success Metrics](#success-metrics)

---

## Sprint Calendar

| Sprint | Dates | Phase | Focus |
|--------|-------|-------|-------|
| S0 | Week 1-2 | MVP-1 | Complete current work, NATS setup |
| S1 | Week 3-4 | MVP-1 | mTLS, Metrics, Risk Engine fixes |
| S2 | Week 5-6 | MVP-2 | Apache AGE integration |
| S3 | Week 7-8 | MVP-2 | Graph Query API |
| S4 | Week 9-10 | MVP-2 | Adaptive Sampling (Agent) |
| S5 | Week 11-12 | MVP-2 | eBPF integration Phase 1 |
| S6 | Week 13-14 | MVP-2 | eBPF integration Phase 2 |
| S7 | Week 15-16 | MVP-2 | TimescaleDB + Baseline Learning |
| S8 | Week 17-18 | MVP-3 | Attack Simulation Engine |
| S9 | Week 19-20 | MVP-3 | Blast Radius & Lateral Movement |
| S10 | Week 21-22 | MVP-3 | Policy Engine Phase 1 |
| S11 | Week 23-24 | MVP-3 | Policy Engine Phase 2 |
| S12 | Week 25-26 | MVP-3 | KubeArmor Integration |
| S13 | Week 27-28 | MVP-4 | Multi-Cluster Hub |
| S14 | Week 29-30 | MVP-4 | Cross-Cluster Correlation |
| S15 | Week 31-32 | MVP-4 | Advanced ML Anomalies |
| S16 | Week 33-34 | MVP-4 | Compliance Reporting |
| S17 | Week 35-36 | MVP-4 | SIEM Integrations |
| S18 | Week 37-38 | Polish | Performance tuning, bug fixes |
| S19 | Week 39-40 | Polish | Documentation, testing |
| S20 | Week 41-42 | Launch | Beta release preparation |

---

## MVP-1: Foundation

**Goal**: Complete core platform with event-driven architecture, security, and observability  
**Duration**: 3 weeks (1.5 sprints)  
**Status**: 80% Complete

---

### Sprint S0: Complete Current Work & Event-Driven Architecture (Week 1-2)

**Objective**: Transition from monolithic to event-driven architecture

**Stories**:

#### S0.1: NATS JetStream Setup ⭐ CRITICAL
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Deploy NATS JetStream StatefulSet in `ksam` namespace
  - 3 replicas for HA
  - 10GB persistent storage per replica
  - Monitoring endpoint exposed
- [ ] Create stream definitions:
  - `ksam.inventory.pods`
  - `ksam.inventory.serviceaccounts`
  - `ksam.inventory.roles`
  - `ksam.events.runtime`
  - `ksam.insights.created`
- [ ] Configure retention policies (7 days)
- [ ] Set up monitoring (Prometheus scraping)

**Acceptance Criteria**:
- NATS cluster operational with 3 replicas
- All streams created and tested
- Can publish/subscribe messages
- Metrics visible in Grafana

**Code Changes**:
```
pkg/messaging/
├── nats_client.go       # NATS connection management
├── publisher.go         # Generic publisher
├── subscriber.go        # Generic subscriber
└── streams.go           # Stream definitions
```

---

#### S0.2: Refactor Ingest API to Publish to NATS
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Update `IngestAPI` to publish inventory items to NATS
- [ ] Remove direct processing from Ingest API
- [ ] Add message validation before publishing
- [ ] Implement message deduplication
- [ ] Add error handling and retry logic
- [ ] Update gRPC endpoints to return immediately after publishing

**Acceptance Criteria**:
- Inventory items published to correct NATS streams
- Ingest API response time < 10ms (just publish, no processing)
- No message loss (confirm with NATS acks)

**Code Changes**:
```diff
// pkg/api/ingest.go

func (api *IngestAPI) HandleInventory(item *pb.InventoryItem) error {
-   // Old: direct processing
-   normalized := api.normalizer.Normalize(item)
-   api.correlator.Process(normalized)
-   api.riskEngine.Evaluate(normalized)

+   // New: publish to NATS
+   return api.publisher.Publish("ksam.inventory.pods", item)
}
```

---

#### S0.3: Create Worker Pool for Message Processing
- **Estimate**: 4 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement generic worker pool
- [ ] Create worker types:
  - `NormalizerWorker`
  - `CorrelatorWorker`
  - `RiskEngineWorker`
- [ ] Subscribe workers to NATS topics
- [ ] Implement graceful shutdown
- [ ] Add worker health checks
- [ ] Configure worker count via env vars

**Acceptance Criteria**:
- Workers process messages from NATS
- Horizontal scaling works (can increase worker count)
- Graceful shutdown (finish in-flight messages)
- Worker metrics exposed (processed, failed, latency)

**Code Changes**:
```
pkg/worker/
├── pool.go              # Worker pool management
├── normalizer.go        # Normalizer worker
├── correlator.go        # Correlator worker
├── riskengine.go        # Risk engine worker
└── health.go            # Health check endpoint
```

**Configuration**:
```yaml
# configmap/worker-config.yaml
workers:
  normalizer:
    count: 3
    concurrency: 10
  correlator:
    count: 2
    concurrency: 5
  riskengine:
    count: 2
    concurrency: 5
```

---

#### S0.4: Update Deployment Architecture
- **Estimate**: 2 days
- **Owner**: DevOps Engineer
- **Priority**: P1

**Tasks**:
- [ ] Create NATS StatefulSet manifest
- [ ] Update Core deployment to include worker containers
- [ ] Add NATS Service
- [ ] Update ConfigMaps for NATS connection strings
- [ ] Test deployment in minikube
- [ ] Document deployment process

**Acceptance Criteria**:
- `kubectl apply -f deploy/` successfully deploys entire system
- All components healthy
- Can send test inventory items through the pipeline

**Files**:
```
deploy/
├── nats/
│   ├── statefulset.yaml
│   ├── service.yaml
│   └── pvc.yaml
├── core/
│   └── deployment.yaml  # Updated with worker containers
└── configmap/
    └── nats-config.yaml
```

---

### Sprint S1: Security & Observability (Week 3-4)

**Objective**: Add mTLS, Prometheus metrics, fix Risk Engine API routes

---

#### S1.1: Implement mTLS for Agent-Core Communication ⭐ CRITICAL
- **Estimate**: 5 days
- **Owner**: Backend Engineer + Security Engineer
- **Priority**: P0

**Tasks**:
- [ ] Install cert-manager in cluster
- [ ] Create Certificate resources for:
  - CA certificate
  - Server certificate (Core)
  - Client certificate (Agent)
- [ ] Update gRPC server to require client certs
- [ ] Update gRPC client (Agent) to present client cert
- [ ] Implement certificate rotation (30-day renewal)
- [ ] Add certificate validation
- [ ] Document certificate management process

**Acceptance Criteria**:
- Agent cannot connect without valid client certificate
- Certificates auto-renew before expiration
- gRPC connection encrypted and mutually authenticated
- Certificate metrics exposed (expiry time)

**Code Changes**:
```go
// pkg/grpc/server.go

func NewSecureGRPCServer(tlsConfig *tls.Config) (*grpc.Server, error) {
    creds := credentials.NewTLS(tlsConfig)
    return grpc.NewServer(grpc.Creds(creds)), nil
}

// TLS config with client cert verification
tlsConfig := &tls.Config{
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    certPool,
    Certificates: []tls.Certificate{serverCert},
}
```

**Kubernetes Resources**:
```yaml
# Certificate for Core gRPC server
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ksam-core-server
  namespace: ksam
spec:
  secretName: ksam-core-tls
  issuerRef:
    name: ksam-ca-issuer
    kind: Issuer
  dnsNames:
  - ksam-core.ksam.svc
  - ksam-core.ksam.svc.cluster.local
```

---

#### S1.2: Add Prometheus Metrics
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Add Prometheus client library
- [ ] Define metrics for:
  - Agent: memory, CPU, events collected/filtered
  - Core: events processed, processing latency, queue depth
  - Risk Engine: rules evaluated, insights generated
- [ ] Expose `/metrics` endpoint
- [ ] Configure Prometheus scraping
- [ ] Create basic Grafana dashboards

**Metrics to Add**:
```go
// Agent metrics
ksam_agent_memory_bytes
ksam_agent_cpu_percent
ksam_agent_events_collected_total{event_type}
ksam_agent_events_filtered_total{reason}

// Core metrics
ksam_events_processed_total{cluster,type,status}
ksam_processing_duration_seconds{component}
ksam_insights_generated_total{severity,category}
ksam_rules_evaluated_total{rule_id,matched}
ksam_queue_depth{topic}
```

**Acceptance Criteria**:
- All metrics visible in Prometheus
- Basic Grafana dashboard functional
- Alerts configured for critical metrics (queue depth > 5000)

---

#### S1.3: Fix Risk Engine API Routes ⭐ CRITICAL
- **Estimate**: 1 day
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Debug API route registration issue
- [ ] Ensure `/api/v1/insights` endpoints are registered
- [ ] Add integration tests for API endpoints
- [ ] Verify dashboard can fetch insights

**Acceptance Criteria**:
- `GET /api/v1/insights` returns insights
- `GET /api/v1/insights/:id` returns specific insight
- Dashboard displays insights correctly
- All API tests pass

---

#### S1.4: Add Distributed Tracing
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P2

**Tasks**:
- [ ] Add OpenTelemetry SDK
- [ ] Instrument key functions with spans
- [ ] Deploy Jaeger or use existing tracing backend
- [ ] Configure trace sampling (10% default)
- [ ] Add trace IDs to logs

**Acceptance Criteria**:
- Can view traces in Jaeger UI
- Traces show flow: Agent → Ingest → NATS → Worker → DB
- Latency breakdown visible

---

#### S1.5: Testing & Documentation
- **Estimate**: 2 days
- **Owner**: All Engineers
- **Priority**: P1

**Tasks**:
- [ ] Integration tests for event-driven flow
- [ ] Load test with 1000 pods
- [ ] Performance benchmarks
- [ ] Update README with new architecture
- [ ] Create runbook for NATS operations

**Acceptance Criteria**:
- All tests pass
- System handles 1000 pods/cluster
- Documentation up-to-date

---

**MVP-1 Deliverables**:
- ✅ Event-driven architecture with NATS JetStream
- ✅ mTLS for agent-core communication
- ✅ Prometheus metrics + Grafana dashboards
- ✅ Distributed tracing
- ✅ Risk Engine API fully functional

**MVP-1 Exit Criteria**:
- [ ] All components deployed successfully
- [ ] System handles 1000 pods without issues
- [ ] Agent footprint < 50MB
- [ ] API latency p95 < 100ms
- [ ] Zero critical bugs
- [ ] Documentation complete

---

## MVP-2: Observability & Graph Engine

**Goal**: Add graph database, adaptive sampling, eBPF, and baseline learning  
**Duration**: 12 weeks (6 sprints)  
**Sprints**: S2-S7

---

### Sprint S2: Apache AGE Integration (Week 5-6)

**Objective**: Add graph database layer for efficient graph queries

---

#### S2.1: Apache AGE Setup & Schema ⭐ CRITICAL
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Install AGE extension in PostgreSQL
- [ ] Create graph schema (`ksam_graph`)
- [ ] Create vertex labels (ServiceAccount, Pod, Role, etc.)
- [ ] Create edge labels (USES, MOUNTS, HAS_PERMISSION, etc.)
- [ ] Write migration script from PostgreSQL tables to graph
- [ ] Test basic graph operations

**SQL Scripts**:
```sql
-- 01_install_age.sql
CREATE EXTENSION IF NOT EXISTS age;
LOAD 'age';
SET search_path = ag_catalog, "$user", public;

-- 02_create_graph.sql
SELECT create_graph('ksam_graph');

-- 03_create_labels.sql
SELECT create_vlabel('ksam_graph', 'ServiceAccount');
SELECT create_vlabel('ksam_graph', 'Pod');
-- ... more labels

SELECT create_elabel('ksam_graph', 'USES');
SELECT create_elabel('ksam_graph', 'MOUNTS');
-- ... more edges
```

**Acceptance Criteria**:
- AGE extension installed and functional
- Can create vertices and edges via Cypher
- Basic queries work (MATCH, CREATE, etc.)

---

#### S2.2: Implement AgeGraphEngine
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create `AgeGraphEngine` struct
- [ ] Implement vertex creation methods
- [ ] Implement edge creation methods
- [ ] Implement query methods:
  - `GetAccessibleSecrets(saID)`
  - `ShortestPath(fromID, toID)`
  - `GetBlastRadius(resourceID, maxDepth)`
  - `GetNeighborhood(resourceID, depth)`
- [ ] Add connection pooling
- [ ] Add error handling and retries
- [ ] Write unit tests

**Code Structure**:
```
pkg/graph/
├── age_engine.go        # Main engine
├── vertices.go          # Vertex operations
├── edges.go             # Edge operations
├── queries.go           # Query operations
└── age_engine_test.go   # Tests
```

**Acceptance Criteria**:
- All CRUD operations work
- Query methods return correct results
- Performance: simple queries < 10ms
- Unit test coverage > 80%

---

#### S2.3: Dual-Write Integration with Correlator
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Update Correlator to write to both PostgreSQL and AGE
- [ ] Implement eventual consistency strategy
- [ ] Add fallback if AGE write fails (log, don't fail)
- [ ] Create sync verification job (runs nightly)
- [ ] Add metrics for sync status

**Code Changes**:
```go
func (c *Correlator) CorrelatePod(pod *Pod) error {
    // 1. Write to PostgreSQL (source of truth)
    if err := c.db.SavePod(pod); err != nil {
        return err
    }
    
    // 2. Write to AGE graph (async, best-effort)
    go func() {
        if err := c.graphEngine.CreatePodVertex(pod); err != nil {
            log.Error().Err(err).Msg("Failed to sync to graph")
            c.metrics.GraphSyncErrors.Inc()
        }
    }()
    
    return nil
}
```

**Acceptance Criteria**:
- New resources automatically added to graph
- PostgreSQL remains source of truth
- Graph sync errors logged but don't fail requests
- Sync verification job reports discrepancies

---

### Sprint S3: Graph Query API (Week 7-8)

**Objective**: Expose graph queries via REST API and integrate with dashboard

---

#### S3.1: Graph Query Service API
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create GraphQueryService
- [ ] Implement REST endpoints:
  - `GET /api/v1/graph/blast-radius/:id`
  - `GET /api/v1/graph/shortest-path`
  - `GET /api/v1/graph/accessible/:id`
  - `GET /api/v1/graph/neighborhood/:id`
  - `GET /api/v1/graph/criticality-scores` (PageRank)
- [ ] Add pagination for large result sets
- [ ] Add caching (Redis) for expensive queries
- [ ] Write integration tests

**API Examples**:
```
GET /api/v1/graph/blast-radius/sa-12345?max_depth=3
Response:
{
  "source_id": "sa-12345",
  "resources": [
    {"id": "pod-1", "type": "Pod", "distance": 1, "risk_score": 5.0},
    {"id": "secret-1", "type": "Secret", "distance": 2, "risk_score": 8.0}
  ],
  "summary": {
    "total": 15,
    "secrets": 3,
    "pods": 10,
    "high_risk_count": 2
  }
}
```

**Acceptance Criteria**:
- All endpoints functional
- Response time < 100ms for typical queries
- Pagination works correctly
- API documentation generated (Swagger)

---

#### S3.2: Dashboard Integration
- **Estimate**: 4 days
- **Owner**: Frontend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create Graph Explorer page
- [ ] Implement Blast Radius visualization (D3.js)
- [ ] Add interactive graph controls (zoom, pan, filter)
- [ ] Create Shortest Path finder UI
- [ ] Add Neighborhood explorer
- [ ] Display PageRank scores (criticality)

**UI Components**:
```
dashboard/src/components/graph/
├── GraphExplorer.tsx
├── BlastRadiusView.tsx
├── ShortestPathFinder.tsx
├── NeighborhoodView.tsx
└── CriticalityScores.tsx
```

**Acceptance Criteria**:
- Graph visualization renders correctly
- Interactive controls work (zoom, pan, click nodes)
- Blast radius highlights correctly
- Performance: renders 1000 nodes without lag

---

#### S3.3: Performance Benchmarking
- **Estimate**: 2 days
- **Owner**: Backend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Create benchmark suite
- [ ] Test query performance vs PostgreSQL
- [ ] Optimize slow queries
- [ ] Add query result caching
- [ ] Document performance characteristics

**Benchmark Targets**:
- Simple neighbor query (1 hop): < 10ms
- Multi-hop query (3 hops): < 50ms
- Shortest path: < 100ms
- Blast radius (depth 5): < 200ms

**Acceptance Criteria**:
- All queries meet performance targets
- Benchmarks automated in CI
- Performance regression alerts configured

---

### Sprint S4: Adaptive Sampling (Agent) (Week 9-10)

**Objective**: Implement intelligent event sampling in agent

---

#### S4.1: Adaptive Sampler Implementation
- **Estimate**: 5 days
- **Owner**: Backend Engineer (Agent team)
- **Priority**: P0

**Tasks**:
- [ ] Implement `AdaptiveSampler` struct
- [ ] Implement sampling decision algorithm:
  - Critical events: always sample
  - Risk-based: high-risk pods sampled more
  - Event type priority: execve > connect > open
  - Noise detection: reduce sampling for repetitive events
- [ ] Implement backpressure detection
- [ ] Add per-pod sampling state tracking
- [ ] Implement noise scoring (Shannon entropy)
- [ ] Add sampling metrics

**Code Structure**:
```
pkg/agent/sampling/
├── adaptive.go          # Main sampler
├── noise_detector.go    # Noise detection logic
├── backpressure.go      # Backpressure handling
└── metrics.go           # Sampling metrics
```

**Acceptance Criteria**:
- Sampling reduces events by 50-80%
- Critical events always sampled
- High-risk pods get higher sampling rates
- Noise detection works (identifies repetitive events)
- Metrics show sampling rates per pod

---

#### S4.2: Controller Feedback Loop
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement feedback mechanism in Core
- [ ] Add load metrics to heartbeat responses
- [ ] Agent adjusts sampling based on feedback
- [ ] Test backpressure scenario (overload core)
- [ ] Document feedback protocol

**Protocol**:
```protobuf
message HeartbeatResponse {
  bool ok = 1;
  float load = 2;  // 0.0-1.0
  float recommended_sampling_rate = 3;  // 0.0-1.0
  bool overloaded = 4;
}
```

**Acceptance Criteria**:
- Agent receives feedback from core
- Agent adjusts sampling rate dynamically
- Under load, sampling rate decreases
- System self-stabilizes under pressure

---

#### S4.3: Configuration & Testing
- **Estimate**: 2 days
- **Owner**: Backend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Add ConfigMap for sampling configuration
- [ ] Add API endpoint to update sampling config at runtime
- [ ] Load test with sampling enabled
- [ ] Compare event volumes (with/without sampling)
- [ ] Document sampling behavior

**Configuration**:
```yaml
adaptive_sampling:
  enabled: true
  base_rate: 0.8
  risk_multipliers:
    critical: 1.0
    high: 0.8
    medium: 0.5
    low: 0.1
  event_priorities:
    execve: 100
    connect: 80
    open: 50
    read: 20
```

**Acceptance Criteria**:
- Configuration loads correctly
- Can update config without restart
- Load test shows 50-80% event reduction
- No critical events dropped

---

### Sprint S5: eBPF Integration Phase 1 (Week 11-12)

**Objective**: Collect execve events using eBPF

---

#### S5.1: eBPF Setup & Infrastructure
- **Estimate**: 4 days
- **Owner**: Backend Engineer (Systems)
- **Priority**: P0

**Tasks**:
- [ ] Add cilium/ebpf library to agent
- [ ] Set up eBPF program loading infrastructure
- [ ] Implement perf buffer for event collection
- [ ] Add BPF capability requirements to agent pod
- [ ] Create helper functions for eBPF map operations
- [ ] Test eBPF program loading on various kernels

**Agent Deployment Updates**:
```yaml
# Update DaemonSet
securityContext:
  privileged: false
  capabilities:
    add:
    - BPF
    - PERFMON
    - SYS_RESOURCE
volumeMounts:
- name: sys
  mountPath: /sys
  readOnly: true
volumes:
- name: sys
  hostPath:
    path: /sys
```

**Acceptance Criteria**:
- eBPF programs load successfully
- Can read from perf buffer
- No kernel panics or errors
- Works on kernel 5.4+

---

#### S5.2: Execve Tracepoint Implementation
- **Estimate**: 5 days
- **Owner**: Backend Engineer (Systems)
- **Priority**: P0

**Tasks**:
- [ ] Write eBPF program for execve tracepoint
- [ ] Collect process info (PID, PPID, comm, args)
- [ ] Implement filtering (skip system processes)
- [ ] Parse execve events in userspace
- [ ] Implement event buffering (100 events/sec)
- [ ] Add execve metrics

**eBPF Program** (C):
```c
SEC("tracepoint/syscalls/sys_enter_execve")
int trace_execve(struct trace_event_raw_sys_enter *ctx) {
    struct event_t event = {};
    event.pid = bpf_get_current_pid_tgid() >> 32;
    event.type = EVENT_EXECVE;
    
    // Read filename
    bpf_probe_read_user_str(&event.filename, sizeof(event.filename),
                            (void *)ctx->args[0]);
    
    // Filter: skip system processes
    if (skip_process(event.filename)) {
        return 0;
    }
    
    // Send to userspace
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU,
                          &event, sizeof(event));
    return 0;
}
```

**Acceptance Criteria**:
- Captures execve events
- Process info correctly extracted
- Filtering works (system processes skipped)
- Event rate < 1000/sec per node

---

#### S5.3: Container Context Resolution
- **Estimate**: 4 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement PID → Container ID resolver
  - Read from `/proc/<pid>/cgroup`
  - Parse container runtime format
- [ ] Implement Container ID → Pod UID resolver
  - Query container runtime (containerd API)
  - Cache results (TTL: 5 minutes)
- [ ] Enrich events with pod context
- [ ] Handle edge cases (pid reuse, container exits)

**Code Structure**:
```
pkg/agent/resolver/
├── container_resolver.go
├── cgroup_parser.go
├── runtime_client.go
└── cache.go
```

**Acceptance Criteria**:
- Can resolve PID to Pod UID with >95% accuracy
- Cache hit rate > 80%
- Lookup latency < 1ms (cached), < 10ms (uncached)
- Handles container restarts correctly

---

### Sprint S6: eBPF Integration Phase 2 (Week 13-14)

**Objective**: Add connect and open syscall tracing

---

#### S6.1: Network Connect Tracing
- **Estimate**: 5 days
- **Owner**: Backend Engineer (Systems)
- **Priority**: P0

**Tasks**:
- [ ] Write eBPF program for connect syscall
- [ ] Capture destination IP and port
- [ ] Filter out localhost connections
- [ ] Filter known service IPs
- [ ] Enrich with DNS data (if available)
- [ ] Add network metrics

**Acceptance Criteria**:
- Captures outbound connections
- Destination IP/port extracted
- Filters reduce noise by >90%
- Can detect unusual connections

---

#### S6.2: File Access Tracing
- **Estimate**: 4 days
- **Owner**: Backend Engineer (Systems)
- **Priority**: P0

**Tasks**:
- [ ] Write eBPF program for open syscall
- [ ] Capture file path
- [ ] Filter to sensitive paths only:
  - `/etc/shadow`, `/etc/passwd`
  - `/var/run/secrets/*`
  - `/root/.ssh/*`
- [ ] Track read vs write operations
- [ ] Add file access metrics

**Acceptance Criteria**:
- Captures file access to sensitive paths
- Read/write operations differentiated
- Filtering reduces events by >95%
- Can detect suspicious file access

---

#### S6.3: Event Forwarding Integration
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Integrate eBPF events with existing forwarding pipeline
- [ ] Apply adaptive sampling to eBPF events
- [ ] Serialize events to protobuf
- [ ] Send to core via gRPC
- [ ] Add eBPF event metrics

**Acceptance Criteria**:
- eBPF events flow to core controller
- Adaptive sampling applied
- No event loss
- Dashboard shows runtime events

---

### Sprint S7: TimescaleDB & Baseline Learning (Week 15-16)

**Objective**: Migrate to TimescaleDB and implement behavior baseline learning

---

#### S7.1: TimescaleDB Migration
- **Estimate**: 4 days
- **Owner**: Backend Engineer + DBA
- **Priority**: P0

**Tasks**:
- [ ] Enable TimescaleDB extension
- [ ] Convert `events` table to hypertable
- [ ] Create continuous aggregates (hourly, daily)
- [ ] Set up compression (compress after 1 day)
- [ ] Configure retention policy (7 days raw, 90 days aggregated)
- [ ] Migrate existing event data
- [ ] Update queries to use hypertable

**SQL Scripts**:
```sql
-- Enable extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Convert to hypertable
SELECT create_hypertable('events', 'time');

-- Compression policy
ALTER TABLE events SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'cluster_id, pod_id'
);

SELECT add_compression_policy('events', INTERVAL '1 day');

-- Retention policy
SELECT add_retention_policy('events', INTERVAL '7 days');

-- Continuous aggregate
CREATE MATERIALIZED VIEW events_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket('1 hour', time) AS bucket,
  cluster_id,
  pod_id,
  event_type,
  COUNT(*) as count,
  AVG(processing_time) as avg_processing_time
FROM events
GROUP BY bucket, cluster_id, pod_id, event_type;
```

**Acceptance Criteria**:
- Events table is hypertable
- Compression works (10-20x reduction)
- Retention policy runs automatically
- Query performance maintained or improved

---

#### S7.2: Baseline Learner Implementation
- **Estimate**: 5 days
- **Owner**: Backend Engineer (ML)
- **Priority**: P0

**Tasks**:
- [ ] Implement `BaselineLearner` component
- [ ] Create baseline data structures:
  - Process baseline (allowed processes)
  - Network baseline (allowed IPs/ports)
  - File access baseline (accessed paths)
  - API access baseline (for ServiceAccounts)
- [ ] Implement learning algorithm (sliding window)
- [ ] Calculate confidence score (increases over time)
- [ ] Store baselines in Redis (hot cache) + PostgreSQL (persistent)
- [ ] Create baseline API endpoints

**Code Structure**:
```
pkg/baseline/
├── learner.go           # Main learning logic
├── process_baseline.go  # Process behavior
├── network_baseline.go  # Network behavior
├── file_baseline.go     # File access behavior
├── api_baseline.go      # API access (ServiceAccounts)
└── store.go             # Storage interface
```

**Learning Strategy**:
```
Phase 1 (Days 1-7): Collect all behaviors, no anomaly detection
Phase 2 (Days 8-14): Build statistical models, log potential anomalies
Phase 3 (Day 15+): Full anomaly detection enabled
```

**Acceptance Criteria**:
- Baselines created for all pods
- Confidence score increases over time
- Can detect deviations from baseline
- Baseline API returns correct data

---

#### S7.3: Anomaly Detection Integration
- **Estimate**: 3 days
- **Owner**: Backend Engineer (ML)
- **Priority**: P0

**Tasks**:
- [ ] Integrate baseline learner with event processing
- [ ] Add anomaly detection to runtime events
- [ ] Create insights for anomalies
- [ ] Add anomaly metrics
- [ ] Tune detection thresholds (minimize false positives)

**Acceptance Criteria**:
- Anomalies detected and logged
- Insights created for significant anomalies
- False positive rate < 10%
- Dashboard shows anomaly alerts

---

**MVP-2 Deliverables**:
- ✅ Apache AGE graph database integrated
- ✅ Graph query API functional
- ✅ Adaptive sampling reducing events by 50-80%
- ✅ eBPF collecting execve, connect, open events
- ✅ TimescaleDB for time-series events
- ✅ Baseline learning and anomaly detection

**MVP-2 Exit Criteria**:
- [ ] Graph queries <100ms for typical use cases
- [ ] Agent footprint still <50MB with eBPF
- [ ] Event throughput 10K events/sec/node
- [ ] Baseline learning works for 1000+ pods
- [ ] Anomaly false positive rate <10%
- [ ] All integration tests pass

---

## MVP-3: Policy Automation & Attack Simulation

**Goal**: Generate policies, simulate attacks, create playbooks  
**Duration**: 10 weeks (5 sprints)  
**Sprints**: S8-S12

---

### Sprint S8: Attack Simulation Engine (Week 17-18)

**Objective**: Build foundation for attack path simulation

---

#### S8.1: Attack Simulator Core ⭐ CRITICAL
- **Estimate**: 5 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P0

**Tasks**:
- [ ] Create `AttackSimulator` struct
- [ ] Implement `SimulateCompromise()` function
- [ ] Define attack objectives (secrets, escalation, network, etc.)
- [ ] Implement scenario storage (PostgreSQL)
- [ ] Add simulation metrics
- [ ] Write unit tests

**Code Structure**:
```
pkg/attack/
├── simulator.go         # Main simulator
├── objectives.go        # Attack objectives
├── scenario.go          # Scenario data structures
└── store.go             # Scenario persistence
```

**Acceptance Criteria**:
- Can create attack scenarios
- Scenarios stored in database
- Basic simulation logic works
- Unit tests pass

---

#### S8.2: Blast Radius Calculator
- **Estimate**: 4 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P0

**Tasks**:
- [ ] Implement `calculateBlastRadius()` using AGE
- [ ] Traverse graph with max depth (default: 5 hops)
- [ ] Categorize reachable resources (secrets, pods, services)
- [ ] Calculate risk metrics (high-risk count, critical count)
- [ ] Generate blast radius report
- [ ] Add visualization data

**Algorithm**:
```sql
-- Cypher query for blast radius
MATCH (start {id: $resource_id})
MATCH path = (start)-[*1..$max_depth]->(target)
RETURN target.id, target.type, target.risk_score, length(path) as distance
ORDER BY distance
```

**Acceptance Criteria**:
- Blast radius calculated correctly
- Performance: <200ms for typical scenarios
- Reports include resource counts and risk metrics
- Visualization data ready for dashboard

---

#### S8.3: Attack Simulation API
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create REST endpoints:
  - `POST /api/v1/attack-simulation/simulate`
  - `GET /api/v1/attack-simulation/blast-radius/:id`
  - `GET /api/v1/attack-simulation/scenarios`
  - `GET /api/v1/attack-simulation/scenarios/:id`
- [ ] Add input validation
- [ ] Add pagination for scenario list
- [ ] Write API tests
- [ ] Generate API documentation

**Acceptance Criteria**:
- All endpoints functional
- API tests pass
- Documentation generated (Swagger)
- Can trigger simulations via API

---

### Sprint S9: Attack Paths & Lateral Movement (Week 19-20)

**Objective**: Find attack paths and lateral movement opportunities

---

#### S9.1: Attack Path Finder
- **Estimate**: 5 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P0

**Tasks**:
- [ ] Implement `findAttackPaths()` function
- [ ] Define target patterns for each objective
- [ ] Use AGE to find paths from source to targets
- [ ] Analyze each path:
  - Calculate difficulty (based on controls, requirements)
  - Calculate impact (based on target risk)
  - Generate human-readable description
- [ ] Rank paths by risk score (impact/difficulty)
- [ ] Limit to top 10 paths per simulation

**Cypher Query Example**:
```sql
-- Find paths to secrets
MATCH (source {id: $source_id})
MATCH (target:Secret)
WHERE target.risk_score >= $min_risk_score
MATCH path = (source)-[*1..6]->(target)
RETURN path
LIMIT 10
```

**Acceptance Criteria**:
- Finds relevant attack paths
- Risk scoring works correctly
- Descriptions are human-readable
- Performance: <500ms per simulation

---

#### S9.2: Lateral Movement Analyzer
- **Estimate**: 4 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P0

**Tasks**:
- [ ] Implement `findLateralMoves()` function
- [ ] Find pods/resources accessible from compromised resource
- [ ] Determine lateral movement method:
  - RBAC privilege escalation
  - Network access via Service
  - Same namespace access
  - Multi-hop exploitation
- [ ] Calculate difficulty for each lateral move
- [ ] Identify prerequisites
- [ ] Generate descriptions

**Acceptance Criteria**:
- Identifies lateral move opportunities
- Methods correctly determined
- Difficulty scores realistic
- Can explain how to perform lateral move

---

#### S9.3: Dashboard Integration
- **Estimate**: 4 days
- **Owner**: Frontend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create Attack Simulation page
- [ ] Implement Blast Radius visualization
- [ ] Create Attack Path Explorer
- [ ] Display lateral movement opportunities
- [ ] Add "Simulate Attack" button to resource pages
- [ ] Show risk scores and descriptions

**UI Components**:
```
dashboard/src/components/attack-simulation/
├── SimulationPage.tsx
├── BlastRadiusView.tsx
├── AttackPathExplorer.tsx
├── AttackPathCard.tsx
├── LateralMoveList.tsx
└── SimulationReport.tsx
```

**Acceptance Criteria**:
- Can trigger simulations from dashboard
- Blast radius visualized correctly
- Attack paths displayed with details
- UI responsive and intuitive

---

### Sprint S10: Policy Engine Phase 1 (Week 21-22)

**Objective**: Generate KubeArmor policies from baselines

---

#### S10.1: Policy Generator Core
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create `PolicyGenerator` struct
- [ ] Implement policy generation from baseline:
  - Process whitelist
  - Network egress rules
  - File access restrictions
- [ ] Generate KubeArmor CRD YAML
- [ ] Add policy preview mode
- [ ] Store policies in database
- [ ] Add policy metrics

**Code Structure**:
```
pkg/policy/
├── generator.go         # Main generator
├── kubearmor.go         # KubeArmor adapter
├── preview.go           # Preview mode
└── templates.go         # Policy templates
```

**Example Generated Policy**:
```yaml
apiVersion: security.kubearmor.com/v1
kind: KubeArmorPolicy
metadata:
  name: nginx-pod-policy
  namespace: default
spec:
  selector:
    matchLabels:
      app: nginx
  process:
    matchPaths:
    - path: /usr/sbin/nginx
    - path: /bin/sh
  file:
    matchPaths:
    - path: /etc/nginx/
      readOnly: true
  network:
    matchProtocols:
    - protocol: tcp
      fromSource:
      - port: 80
  action: Block
```

**Acceptance Criteria**:
- Policies generated from baselines
- YAML syntax valid
- Preview mode works
- Policies stored in database

---

#### S10.2: Confidence Scoring
- **Estimate**: 3 days
- **Owner**: Backend Engineer (ML)
- **Priority**: P0

**Tasks**:
- [ ] Implement confidence score calculation
- [ ] Consider factors:
  - Observation days (more days = higher confidence)
  - Event count (more events = higher confidence)
  - Behavior stability (low variance = higher confidence)
- [ ] Set confidence thresholds:
  - Low (<0.5): Don't generate policy
  - Medium (0.5-0.8): Generate in preview mode
  - High (>0.8): Safe to apply
- [ ] Add confidence to policy metadata

**Formula**:
```
Confidence = f(observation_days, event_count, stability)

Where:
- observation_days: normalized to [0,1], capped at 14 days
- event_count: log-scaled, normalized
- stability: 1 - coefficient_of_variation
```

**Acceptance Criteria**:
- Confidence scores calculated correctly
- Thresholds prevent premature policy generation
- Confidence increases over time

---

#### S10.3: Policy API
- **Estimate**: 2 days
- **Owner**: Backend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Create REST endpoints:
  - `GET /api/v1/policies`
  - `GET /api/v1/policies/:id`
  - `POST /api/v1/policies/generate/:pod_id`
  - `POST /api/v1/policies/:id/preview`
  - `POST /api/v1/policies/:id/apply`
  - `DELETE /api/v1/policies/:id`
- [ ] Add policy CRUD operations
- [ ] Write API tests

**Acceptance Criteria**:
- All endpoints functional
- Can generate, preview, apply policies via API
- API tests pass

---

### Sprint S11: Policy Engine Phase 2 (Week 23-24)

**Objective**: Add NetworkPolicy generation and policy preview

---

#### S11.1: NetworkPolicy Generator
- **Estimate**: 4 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement NetworkPolicy generation
- [ ] Generate ingress rules from observed traffic
- [ ] Generate egress rules from observed connections
- [ ] Handle DNS (allow CoreDNS)
- [ ] Generate namespace isolation policies
- [ ] Add NetworkPolicy templates

**Example Generated NetworkPolicy**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: nginx-netpol
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: nginx
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: frontend
    ports:
    - protocol: TCP
      port: 80
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: backend
    ports:
    - protocol: TCP
      port: 8080
  - to:  # Allow DNS
    - namespaceSelector:
        matchLabels:
          name: kube-system
    - podSelector:
        matchLabels:
          k8s-app: kube-dns
    ports:
    - protocol: UDP
      port: 53
```

**Acceptance Criteria**:
- NetworkPolicy generated correctly
- Ingress/egress rules accurate
- DNS allowed by default
- YAML syntax valid

---

#### S11.2: Policy Preview Simulator
- **Estimate**: 4 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement policy preview mode
- [ ] Simulate policy application (dry-run)
- [ ] Predict impacts:
  - Blocked processes
  - Blocked network connections
  - Blocked file accesses
- [ ] Calculate false positive risk
- [ ] Generate preview report
- [ ] Show diff (current vs. with policy)

**Preview Report Structure**:
```json
{
  "policy_id": "pol-123",
  "status": "preview",
  "confidence": 0.85,
  "predicted_impacts": {
    "blocked_processes": [
      {"process": "/usr/bin/curl", "frequency": 5}
    ],
    "blocked_connections": [
      {"dest_ip": "1.2.3.4", "port": 443, "frequency": 10}
    ],
    "blocked_file_access": []
  },
  "false_positive_risk": "low",
  "recommendation": "safe_to_apply"
}
```

**Acceptance Criteria**:
- Preview reports generated
- Impacts predicted accurately
- False positive risk assessment works
- Can compare current vs. policy state

---

#### S11.3: Dashboard Policy Management
- **Estimate**: 4 days
- **Owner**: Frontend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Create Policy Management page
- [ ] Display generated policies
- [ ] Show confidence scores
- [ ] Implement policy preview UI
- [ ] Add "Apply Policy" button with confirmation
- [ ] Show policy status (draft, preview, active)
- [ ] Allow policy editing (YAML editor)

**UI Components**:
```
dashboard/src/components/policies/
├── PolicyList.tsx
├── PolicyCard.tsx
├── PolicyPreview.tsx
├── PolicyEditor.tsx
└── PolicyApply.tsx
```

**Acceptance Criteria**:
- Can view all policies
- Preview shows predicted impacts
- Can apply policies from UI
- YAML editor works for custom edits

---

### Sprint S12: KubeArmor Integration (Week 25-26)

**Objective**: Apply policies to cluster via KubeArmor

---

#### S12.1: KubeArmor Adapter
- **Estimate**: 4 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement `KubeArmorAdapter`
- [ ] Apply KubeArmor CRDs to cluster
- [ ] Monitor KubeArmor logs for violations
- [ ] Sync policy status (active, violated)
- [ ] Handle policy conflicts
- [ ] Add rollback capability

**Code Structure**:
```
pkg/controller/
├── kubearmor_adapter.go
├── policy_applier.go
└── rollback.go
```

**Acceptance Criteria**:
- Policies applied to cluster
- KubeArmor enforces policies
- Violations detected and logged
- Can rollback policies if issues

---

#### S12.2: Policy Monitoring & Alerts
- **Estimate**: 3 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Monitor KubeArmor violation logs
- [ ] Create insights for policy violations
- [ ] Alert on unexpected violations (potential attack)
- [ ] Track policy effectiveness (violations prevented)
- [ ] Add policy metrics

**Metrics**:
```
ksam_policies_applied_total{type}
ksam_policy_violations_total{policy_id,severity}
ksam_policies_rolled_back_total{reason}
```

**Acceptance Criteria**:
- Violations create insights
- Alerts sent for critical violations
- Metrics visible in dashboard
- Can track policy effectiveness

---

#### S12.3: SOC Playbook Generator
- **Estimate**: 3 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P1

**Tasks**:
- [ ] Implement playbook generation from attack scenarios
- [ ] Generate playbook sections:
  - Detection steps
  - Containment steps
  - Eradication steps
  - Recovery steps
  - Lessons learned
- [ ] Export playbooks (Markdown, PDF)
- [ ] Store playbooks in database

**Playbook Template**:
```markdown
# Incident Response Playbook: ServiceAccount Compromise

## Detection
1. Monitor for unusual API calls from SA 'default'
   Tools: KSAM Audit Logs, K8s Audit Logs
2. Check for lateral movement
   Tools: KSAM Runtime Events, Falco

## Containment
1. Quarantine compromised pods
   Command: kubectl label pod <n> security.ksam.io/quarantine=true
...
```

**Acceptance Criteria**:
- Playbooks generated for scenarios
- Playbooks include actionable steps
- Can export to Markdown/PDF
- Playbooks stored and retrievable

---

**MVP-3 Deliverables**:
- ✅ Attack simulation engine
- ✅ Blast radius analysis
- ✅ Attack path discovery
- ✅ Lateral movement analysis
- ✅ Policy generator (KubeArmor, NetworkPolicy)
- ✅ Policy preview mode
- ✅ KubeArmor integration
- ✅ SOC playbook generator

**MVP-3 Exit Criteria**:
- [ ] Attack simulations accurate and fast (<1s)
- [ ] Policies generated with >80% confidence
- [ ] False positive rate <10%
- [ ] KubeArmor policies applied successfully
- [ ] Playbooks useful for SOC teams
- [ ] All integration tests pass

---

## MVP-4: Enterprise Features

**Goal**: Multi-cluster, advanced ML, compliance, SIEM  
**Duration**: 10 weeks (5 sprints)  
**Sprints**: S13-S17

---

### Sprint S13: Multi-Cluster Hub (Week 27-28)

**Objective**: Aggregate data from multiple clusters

---

#### S13.1: Hub Architecture Setup
- **Estimate**: 5 days
- **Owner**: Backend Engineer + DevOps
- **Priority**: P0

**Tasks**:
- [ ] Deploy management cluster (hub)
- [ ] Deploy KSAM aggregator in hub
- [ ] Configure spoke clusters to report to hub
- [ ] Implement cluster registration
- [ ] Add cluster health monitoring
- [ ] Set up hub-spoke networking

**Architecture**:
```
Hub (Management Cluster)
  ├─ KSAM Aggregator
  ├─ Unified PostgreSQL
  └─ Unified Dashboard

Spoke Cluster A
  ├─ Local KSAM Core
  ├─ Local PostgreSQL
  └─ Agents

Spoke Cluster B
  ├─ Local KSAM Core
  ├─ Local PostgreSQL
  └─ Agents
```

**Acceptance Criteria**:
- Hub deployed successfully
- Spokes register with hub
- Hub can query spoke data
- Networking secure (mTLS)

---

#### S13.2: Cross-Cluster Aggregation
- **Estimate**: 4 days
- **Owner**: Backend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Implement data aggregation from spokes
- [ ] Sync cluster metadata
- [ ] Aggregate inventory (pods, SAs, etc.)
- [ ] Aggregate insights
- [ ] Handle cluster-specific data
- [ ] Add aggregation metrics

**Acceptance Criteria**:
- Hub shows data from all clusters
- Aggregation happens periodically (every 5 min)
- Cluster-specific context maintained
- Dashboard shows multi-cluster view

---

#### S13.3: Unified Dashboard
- **Estimate**: 3 days
- **Owner**: Frontend Engineer
- **Priority**: P0

**Tasks**:
- [ ] Add cluster selector to dashboard
- [ ] Show aggregated metrics
- [ ] Multi-cluster graph view
- [ ] Cross-cluster search
- [ ] Cluster health dashboard

**Acceptance Criteria**:
- Can switch between clusters
- Aggregated view shows all clusters
- Performance acceptable with 10 clusters
- Clear cluster identification in UI

---

### Sprint S14: Cross-Cluster Correlation (Week 29-30)

**Objective**: Detect attacks spanning multiple clusters

---

#### S14.1: Cross-Cluster Attack Detection
- **Estimate**: 5 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P0

**Tasks**:
- [ ] Implement cross-cluster correlation
- [ ] Detect patterns:
  - Same ServiceAccount in multiple clusters
  - Synchronized attacks
  - Data exfiltration across clusters
- [ ] Create cross-cluster insights
- [ ] Add correlation metrics

**Acceptance Criteria**:
- Can detect cross-cluster attacks
- Insights link resources across clusters
- Alert on suspicious patterns
- Metrics show correlation effectiveness

---

#### S14.2: Global Blast Radius
- **Estimate**: 3 days
- **Owner**: Backend Engineer (Security)
- **Priority**: P1

**Tasks**:
- [ ] Extend blast radius to span clusters
- [ ] Calculate global impact
- [ ] Show cross-cluster attack paths
- [ ] Generate global risk reports

**Acceptance Criteria**:
- Blast radius spans clusters
- Global impact calculated
- Reports show cross-cluster risks

---

### Sprint S15: Advanced ML Anomaly Detection (Week 31-32)

**Objective**: Improve anomaly detection with ML

---

#### S15.1: ML Model Development
- **Estimate**: 5 days
- **Owner**: ML Engineer
- **Priority**: P1

**Tasks**:
- [ ] Collect training data from baselines
- [ ] Train ML models:
  - Process anomaly detection (Random Forest)
  - Network anomaly detection (Isolation Forest)
  - Time-series anomaly (LSTM/Autoencoder)
- [ ] Evaluate models (precision, recall, F1)
- [ ] Export models for inference

**Acceptance Criteria**:
- Models trained on real data
- Precision >90%, Recall >80%
- Models exported (ONNX format)
- Inference latency <10ms

---

#### S15.2: ML Integration
- **Estimate**: 4 days
- **Owner**: Backend Engineer (ML)
- **Priority**: P1

**Tasks**:
- [ ] Integrate ML models into baseline learner
- [ ] Implement model serving (ONNX runtime)
- [ ] A/B test: rule-based vs. ML
- [ ] Tune ML thresholds
- [ ] Add ML metrics

**Acceptance Criteria**:
- ML models used for anomaly detection
- Better accuracy than rule-based
- False positive rate <5%
- Inference performs well at scale

---

### Sprint S16: Compliance Reporting (Week 33-34)

**Objective**: Generate compliance reports (CIS, SOC2, etc.)

---

#### S16.1: Compliance Framework
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Implement compliance checker
- [ ] Add compliance frameworks:
  - CIS Kubernetes Benchmark v1.8
  - SOC2 controls
  - PCI-DSS requirements
- [ ] Map insights to compliance controls
- [ ] Calculate compliance scores
- [ ] Generate compliance reports

**Acceptance Criteria**:
- Compliance checks run automatically
- Reports show compliance status
- Can track compliance over time
- Exportable (PDF, CSV)

---

#### S16.2: Dashboard Integration
- **Estimate**: 3 days
- **Owner**: Frontend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Create Compliance page
- [ ] Show compliance scores (charts)
- [ ] List failed checks
- [ ] Show remediation status
- [ ] Export reports

**Acceptance Criteria**:
- Compliance dashboard functional
- Charts show trends over time
- Can drill down into failed checks
- Reports exportable

---

### Sprint S17: SIEM Integrations (Week 35-36)

**Objective**: Integrate with external SIEM systems

---

#### S17.1: SIEM Adapters
- **Estimate**: 5 days
- **Owner**: Backend Engineer
- **Priority**: P1

**Tasks**:
- [ ] Implement webhook adapter
- [ ] Add SIEM-specific adapters:
  - Splunk HEC
  - Datadog Events API
  - Elastic SIEM
- [ ] Format events for each SIEM
- [ ] Add retry and error handling
- [ ] Document integration setup

**Acceptance Criteria**:
- Events sent to SIEM
- Formats correct for each SIEM
- Reliable delivery (retry on failure)
- Integration docs complete

---

#### S17.2: Syslog Output
- **Estimate**: 2 days
- **Owner**: Backend Engineer
- **Priority**: P2

**Tasks**:
- [ ] Implement syslog output
- [ ] Support RFC5424 format
- [ ] Configure syslog destination
- [ ] Test with various syslog servers

**Acceptance Criteria**:
- Insights sent via syslog
- Format compliant with RFC5424
- Works with popular syslog servers

---

**MVP-4 Deliverables**:
- ✅ Multi-cluster hub architecture
- ✅ Cross-cluster correlation
- ✅ Advanced ML anomaly detection
- ✅ Compliance reporting (CIS, SOC2, PCI-DSS)
- ✅ SIEM integrations (Splunk, Datadog, Elastic)

**MVP-4 Exit Criteria**:
- [ ] Hub manages 10+ clusters
- [ ] Cross-cluster attacks detected
- [ ] ML models improve detection accuracy
- [ ] Compliance reports generated
- [ ] SIEM integrations functional
- [ ] All tests pass
- [ ] Documentation complete

---

## Dependencies & Critical Path

### Critical Path (Must complete in order):

```
MVP-1
└─ S0: NATS Setup ⭐ BLOCKS → S0.2, S0.3
   └─ S0.2: Ingest API Refactor
      └─ S0.3: Worker Pool ⭐ BLOCKS → S1, S2

MVP-2
└─ S2: Apache AGE ⭐ BLOCKS → S3, S8
   └─ S3: Graph Query API ⭐ BLOCKS → S8
      └─ S4: Adaptive Sampling
         └─ S5: eBPF Phase 1 ⭐ BLOCKS → S6
            └─ S6: eBPF Phase 2
               └─ S7: Baseline Learning ⭐ BLOCKS → S10

MVP-3
└─ S8: Attack Simulation ⭐ BLOCKS → S9, S12
   └─ S9: Attack Paths
      └─ S10: Policy Generator ⭐ BLOCKS → S11, S12
         └─ S11: Policy Preview
            └─ S12: KubeArmor Integration

MVP-4
└─ S13: Multi-Cluster Hub ⭐ BLOCKS → S14
   └─ S14: Cross-Cluster Correlation
      └─ Parallel:
         ├─ S15: ML Models
         ├─ S16: Compliance
         └─ S17: SIEM
```

### Parallel Tracks:

**Track 1 (Backend Core)**:
- S0 → S1 → S2 → S3 → S8 → S9 → S10 → S11 → S12 → S13 → S14

**Track 2 (Agent/eBPF)**:
- S4 → S5 → S6 → S7

**Track 3 (Dashboard)**:
- S1.5 → S3.2 → S9.3 → S11.3 → S13.3 → S14.2

**Track 4 (ML/Advanced)**:
- S7.2 → S15

---

## Risk Mitigation

### High-Risk Items:

#### 1. eBPF Kernel Compatibility ⚠️ HIGH RISK
- **Risk**: eBPF programs may not work on older kernels
- **Impact**: Runtime events unavailable
- **Mitigation**:
  - Test on multiple kernel versions (5.4, 5.10, 5.15, 6.1)
  - Graceful degradation if eBPF unavailable
  - Fallback to Falco integration
- **Contingency**: Ship without eBPF for MVP-2, add in MVP-2.5

#### 2. Apache AGE Maturity ⚠️ MEDIUM RISK
- **Risk**: AGE is less mature than Neo4j, may have bugs
- **Impact**: Graph queries unreliable or slow
- **Mitigation**:
  - Thorough testing with large datasets
  - Keep PostgreSQL as source of truth
  - Monitor AGE project health
- **Contingency**: Migrate to Neo4j if issues (estimated 2 weeks)

#### 3. Attack Simulation Accuracy ⚠️ MEDIUM RISK
- **Risk**: Simulated attack paths may not match real attacks
- **Impact**: False confidence, missed risks
- **Mitigation**:
  - Validate simulations with red team exercises
  - Continuous tuning based on real incidents
  - Clear disclaimers about simulation limitations
- **Contingency**: Market as "guidance" not "guarantee"

#### 4. Baseline False Positives ⚠️ MEDIUM RISK
- **Risk**: Baseline learning generates too many false positives
- **Impact**: Alert fatigue, reduced trust
- **Mitigation**:
  - Conservative thresholds initially
  - Tune based on user feedback
  - Allow manual baseline adjustments
- **Contingency**: Add "training mode" where users confirm anomalies

#### 5. Multi-Cluster Complexity ⚠️ LOW RISK
- **Risk**: Hub-spoke architecture adds operational complexity
- **Impact**: Harder to deploy and maintain
- **Mitigation**:
  - Excellent documentation
  - Helm chart for easy deployment
  - Optional feature (can run single-cluster)
- **Contingency**: Focus on single-cluster for MVP, defer multi-cluster

---

## Success Metrics

### MVP-1 Success Metrics:
- [ ] Agent memory footprint: <50MB ✅ Target
- [ ] API latency (p95): <100ms ✅ Target
- [ ] Event throughput: 10K events/sec/node
- [ ] Zero critical bugs
- [ ] NATS queue depth never exceeds 10K

### MVP-2 Success Metrics:
- [ ] Graph query latency (p95): <100ms
- [ ] Adaptive sampling reduces events by >50%
- [ ] eBPF overhead: <2% CPU
- [ ] Baseline false positive rate: <10%
- [ ] TimescaleDB compression: >10x

### MVP-3 Success Metrics:
- [ ] Attack simulation time: <1s
- [ ] Policy confidence accuracy: >85%
- [ ] Policy false positive rate: <10%
- [ ] KubeArmor policies applied successfully: >95%
- [ ] Playbooks rated useful by SOC teams: >4/5

### MVP-4 Success Metrics:
- [ ] Hub manages 10+ clusters without degradation
- [ ] Cross-cluster correlation detects attacks: >90% accuracy
- [ ] ML models improve accuracy by >10% vs. rule-based
- [ ] Compliance reports complete and accurate
- [ ] SIEM integrations deliver events reliably: >99.9%

---

## Resource Planning

### Team Composition (Assumed):

**Backend Engineers**: 2-3
- Core platform, API, workers, integrations

**Systems Engineer**: 1
- eBPF, agent, performance optimization

**Security Engineer**: 0.5 (part-time)
- Risk rules, attack simulation, security reviews

**ML Engineer**: 0.5 (part-time, MVP-2+)
- Baseline learning, anomaly detection, ML models

**Frontend Engineer**: 1
- Dashboard, UI components

**DevOps Engineer**: 0.5 (part-time)
- Deployment, monitoring, infrastructure

**QA Engineer**: 0.5 (part-time)
- Test plans, automation, regression testing

**Total**: ~5-6 FTE

---

## Appendix

### Sprint Velocity:
- Assumed velocity: 40 story points/sprint (2 weeks)
- Assumes 3-4 engineers
- Buffer: 20% for unknowns and bug fixes

### Definition of Done:
- [ ] Code complete and reviewed
- [ ] Unit tests written (>80% coverage)
- [ ] Integration tests written
- [ ] API documentation updated
- [ ] User documentation updated
- [ ] Deployment manifest updated
- [ ] Smoke tested in dev environment
- [ ] Passed code review
- [ ] No critical bugs
- [ ] Metrics/observability added

### Sprint Ceremony Schedule:
- **Sprint Planning**: Monday Week 1 (2 hours)
- **Daily Standups**: Every day (15 min)
- **Sprint Demo**: Friday Week 2 (1 hour)
- **Retrospective**: Friday Week 2 (1 hour)
- **Backlog Grooming**: Thursday Week 2 (1 hour)

---

**End of Implementation Plan**

**Next Steps**:
1. Review and approve plan with stakeholders
2. Set up project management tool (Jira, GitHub Projects)
3. Create detailed tickets for Sprint S0
4. Kick off Sprint S0 on Monday Week 1
5. Begin parallel work streams where possible

**Questions?** Contact: [Your Name/Team]
