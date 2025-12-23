# ADR-0010: Agent as Data Plane (Mandatory)

**Status**: ✅ ACCEPTED  
**Date**: December 23, 2024  
**Decision Makers**: Architecture Review  
**Supersedes**: Previous Core-Only assumptions

---

## Context

### The Mistake We Almost Made

We initially analyzed the codebase and concluded:
- ❌ "Core-Only is the correct design"
- ❌ "Agent is optional for advanced use cases"
- ❌ "Current implementation works fine"

**This was WRONG.**

### Why It Was Wrong

**Data Gravity Rule**: *Compute should move to data, not data to compute.*

Current Core-Only implementation:
1. Pod created on Node-47
2. Core (running on Node-1) detects pod
3. Core pulls entire image layers over network
4. Core extracts SBOM (CPU/I/O intensive)
5. Core runs CVE scan (CPU intensive)
6. Core stores results

**Problems**:
- 🔴 Massive network transfer (image layers can be GBs)
- 🔴 Core becomes bottleneck (single pod doing all scanning)
- 🔴 Doesn't scale beyond 100 pods
- 🔴 Inefficient resource usage (centralized heavy compute)
- 🔴 High latency (network + queue + compute)

---

## Decision

### ✅ AGENT IS MANDATORY as Data Plane

**Agent (DaemonSet)** is the **PRIMARY** component for:
1. **SBOM Generation**: Extract packages from container filesystems/layers
2. **CVE Scanning**: Run local trivy/grype library
3. **Runtime Metadata**: Collect namespace, pod, image digest
4. **Batch Results**: Send **summarized findings**, not raw data

**Core (Deployment)** is the **BRAIN** for:
1. **Policy Evaluation**: Check findings against policies
2. **Risk Scoring**: Calculate risk scores
3. **Correlation**: Link SBOM + CVE + RBAC + Policy
4. **Graph Building**: Attack path analysis
5. **API/Dashboard**: Serve results

### Architecture Principle

```
╔══════════════════════════════════════════════════════════════╗
║  AGENT = DATA PLANE (Heavy Lifting)                          ║
║  CORE  = CONTROL PLANE (Decision Making)                     ║
╚══════════════════════════════════════════════════════════════╝
```

**NOT**:
```
❌ Agent = optional collector
❌ Core = do everything
```

**BUT**:
```
✅ Agent = sensor + worker (distributed)
✅ Core = brain (centralized)
```

---

## Rationale

### 1. Data Locality

**Bad** (Current Core-Only):
```
Node-47 (4GB image) ──────> Network Transfer ──────> Core Node-1
                      (slow, expensive)              (bottleneck)
```

**Good** (Agent-Based):
```
Node-47 (4GB image) ──> Local Agent (same node)
                         ↓ Extract SBOM + Scan CVE
                         ↓ Send 50KB result
                         ↓
                        Core (fast, cheap)
```

**Savings**: 4GB → 50KB = **99.9% network reduction**

---

### 2. Scalability

**Core-Only Limits**:
- Single Core pod
- 2 CPU, 4GB RAM
- Queue depth: ~100 pods
- **Max cluster size: ~100 nodes**

**Agent-Based Capacity**:
- 1 Agent per node (DaemonSet)
- 100m CPU, 128MB RAM per agent
- Parallel processing
- **Max cluster size: 10,000+ nodes**

**Linear scaling**: More nodes = more agents = more capacity

---

### 3. Resource Efficiency

**Core-Only** (100 nodes):
```
Core: 2 CPU, 4GB RAM (single pod)
Total: 2 CPU, 4GB RAM
Utilization: 80% (bottleneck)
```

**Agent-Based** (100 nodes):
```
Core: 1 CPU, 2GB RAM (decision only)
Agents: 100 × (100m CPU, 128MB RAM) = 10 CPU, 12.8GB RAM (distributed)
Total: 11 CPU, 14.8GB RAM
Utilization: 40% (balanced across nodes)
```

**Better resource distribution**, no single bottleneck.

---

### 4. Industry Best Practices

**This is how security scanning SHOULD work**:

| Tool | Architecture |
|------|--------------|
| Falco | Agent-based (DaemonSet) |
| Trivy Operator | In-cluster scanning (Jobs/DaemonSet) |
| Aqua Security | Agent-based sensors |
| Sysdig | Agent-based collectors |
| Datadog | Agent-based (node-level) |
| Prometheus | Exporters on each node |

**Pattern**: *Heavy data collection happens at the edge, centralized aggregation/analysis.*

---

### 5. CVE Scanning Reality

**SBOM + CVE scanning is**:
- ✅ CPU-bound (parsing, matching)
- ✅ I/O-bound (reading layers, files)
- ✅ Cacheable (same image = same SBOM)
- ✅ Throttleable (per-node rate limits)

**Perfect for distributed agents.**

---

## Architectural Correction (Not Rewrite)

### What Needs to Move

```
FROM: Core (all-in-one)
  ├─ K8s API Client ✅ (stay in Core for metadata)
  ├─ SBOM Generator ❌ (move to Agent)
  ├─ CVE Scanner ❌ (move to Agent)
  ├─ Risk Worker ✅ (stay in Core)
  ├─ Policy Engine ✅ (stay in Core)
  └─ API/Dashboard ✅ (stay in Core)

TO: Agent + Core (distributed)

Agent (DaemonSet):
  ├─ K8s Watcher (local node pods only)
  ├─ SBOM Generator (from local container runtime)
  ├─ CVE Scanner (trivy/grype library)
  └─ gRPC Client (send findings to Core)

Core (Deployment):
  ├─ K8s API Client (metadata, RBAC)
  ├─ gRPC Server (receive findings from agents)
  ├─ Risk Worker (policy evaluation)
  ├─ Correlation Engine (SBOM+CVE+RBAC)
  ├─ Graph Builder (attack paths)
  └─ API/Dashboard (serve results)
```

### Work Estimate: **70% Move, 30% New Code**

**Move** (existing code):
- `pkg/sbom/extractor/` → agent
- `pkg/cve/scanner/` → agent (if exists)
- `pkg/sbom/parsers/` → agent

**New** (integration):
- Agent gRPC client
- Core gRPC server
- Contract schema (proto)
- Agent registration
- Result batching

**Time**: 3-4 weeks (not 6 weeks)  
**Team**: 2 engineers (not 3)  
**Cost**: $40k (not $72k)

---

## Flow (Correct Industry Standard)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Pod Created on Node-47                                   │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Agent-47 detects workload (local K8s watch)              │
│    - Pod UID: abc-123                                       │
│    - Image: nginx:1.21                                      │
│    - Digest: sha256:aabbcc...                               │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Agent-47 generates SBOM (LOCAL)                          │
│    - containerd API (local socket)                          │
│    - Extract layers from local cache                        │
│    - Parse packages (dpkg, rpm, apk, npm, etc.)             │
│    - Result: SBOM JSON (~50KB)                              │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Agent-47 runs CVE scan (LOCAL)                           │
│    - trivy/grype library (embedded)                         │
│    - Match SBOM packages against local CVE DB               │
│    - Result: Vulnerability findings (~20KB)                 │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. Agent-47 sends SUMMARIZED findings (gRPC)                │
│    - NOT raw image layers                                   │
│    - NOT full SBOM                                          │
│    - ONLY: Pod UID + CVE list + severity + affected pkg     │
│    - Size: ~20KB                                            │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓ Network (cheap)
┌─────────────────────────────────────────────────────────────┐
│ 6. Core receives findings                                   │
│    - Validate schema                                        │
│    - Deduplicate                                            │
│    - Store in DB                                            │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│ 7. Core evaluates risk & policy                             │
│    - Check against policy rules                             │
│    - Calculate risk score (CVSS + context)                  │
│    - Create insights                                        │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────────────────────┐
│ 8. Core updates graph + dashboard                           │
│    - Build attack paths (AGE graph)                         │
│    - Expose via API                                         │
│    - Display in dashboard                                   │
└─────────────────────────────────────────────────────────────┘
```

**Key**: Heavy lifting (SBOM + CVE) happens on Node-47, light coordination happens in Core.

---

## Contract: Agent → Core

### Proto Schema (Required)

```protobuf
// agent-finding.proto

message ScanFinding {
  // Identity
  string pod_uid = 1;
  string pod_name = 2;
  string namespace = 3;
  string container_name = 4;
  string image_digest = 5;
  string node_id = 6;
  
  // SBOM Summary (not full SBOM)
  repeated Package packages = 7;
  
  // CVE Findings
  repeated Vulnerability vulnerabilities = 8;
  
  // Metadata
  google.protobuf.Timestamp scanned_at = 9;
  string agent_version = 10;
}

message Package {
  string name = 1;
  string version = 2;
  string type = 3; // deb, rpm, npm, etc.
}

message Vulnerability {
  string cve_id = 1;
  string severity = 2; // CRITICAL, HIGH, MEDIUM, LOW
  string affected_package = 3;
  string fixed_version = 4;
  float cvss_score = 5;
}
```

**Size**: Typical finding = 10-50KB (not GB)

---

## Why "Agent Is Heavy" Is CORRECT

### Agent Resource Usage (Per Node)

```yaml
Agent Pod:
  CPU: 200m (request), 1000m (limit)
  Memory: 256MB (request), 1GB (limit)
  
Why?
  - SBOM extraction: CPU + I/O intensive
  - CVE scanning: CPU intensive
  - Local caching: Memory for CVE DB
  
This is GOOD, not BAD.
```

**Rationale**:
- Work happens where data lives (node)
- Node has spare capacity (not all pods scan at once)
- Can throttle/rate-limit per node
- Cache results locally

**Anti-pattern**: Lightweight agent that sends raw data to overloaded Core.

---

## Consequences

### Positive

✅ **Scalability**: Linear scaling with cluster size  
✅ **Network efficiency**: 99.9% reduction in data transfer  
✅ **Resource distribution**: No single bottleneck  
✅ **Industry alignment**: Standard pattern for security tools  
✅ **Fault isolation**: One agent crash ≠ full outage  
✅ **Cache locality**: SBOM cache per node  

### Negative

⚠️ **Complexity**: More components to deploy/manage  
⚠️ **Resource usage**: Higher total resource usage (distributed)  
⚠️ **Debugging**: Need to check agent logs on nodes  
⚠️ **Initial effort**: 3-4 weeks migration work  

### Mitigation

- Use DaemonSet for automatic agent deployment
- Centralized logging (fluent-bit)
- Metrics per agent (Prometheus)
- Health checks + auto-restart

---

## Implementation Priority

### Phase 1: Move SBOM to Agent (Week 1-2)

**Goal**: Agent generates SBOM, sends to Core

**Tasks**:
1. Move `pkg/sbom/extractor/` to `agent/`
2. Agent watches local node pods
3. Agent generates SBOM on pod create
4. Define proto schema
5. Agent sends SBOM via gRPC
6. Core receives + stores

**Deliverable**: Agent-generated SBOMs in Core DB

---

### Phase 2: Move CVE Scan to Agent (Week 2-3)

**Goal**: Agent runs CVE scan, sends findings

**Tasks**:
1. Embed trivy/grype library in agent
2. Agent matches SBOM against CVE DB
3. Agent sends findings via gRPC
4. Core receives + creates insights
5. Remove CVE scan from Core

**Deliverable**: Agent-scanned CVEs in insights

---

### Phase 3: Optimize & Scale (Week 3-4)

**Goal**: Production-ready agent

**Tasks**:
1. Add agent-side caching (SBOM per digest)
2. Rate limiting (scans per minute)
3. Metrics + monitoring
4. Health checks
5. Resource tuning
6. Load testing (1000 pods)

**Deliverable**: Production-ready Agent-Based architecture

---

## Rollback Plan

**Hybrid Mode** (migration period):
- Core keeps SBOM/CVE logic (fallback)
- Agent sends findings (primary)
- Validate both match
- After 2 weeks, disable Core scanning

**Rollback** (if issues):
- Disable agent findings ingestion
- Re-enable Core scanning
- Revert deployment

---

## Monitoring Success

### Metrics

**Agent**:
- `agent_sbom_generation_duration_seconds`
- `agent_cve_scan_duration_seconds`
- `agent_findings_sent_total`
- `agent_cache_hit_rate`

**Core**:
- `core_findings_received_total`
- `core_findings_processing_duration_seconds`
- `core_insights_created_total`

**Network**:
- `network_bytes_transferred` (should drop 99%)

### SLOs

- **SBOM generation**: < 30s per image (p99)
- **CVE scan**: < 60s per SBOM (p99)
- **End-to-end**: Pod create → Insight < 2 minutes (p99)
- **Network**: < 100KB per finding

---

## Final Statement

### The Truth

**Current State**:
- Core-Only architecture exists
- Works for small scale (<50 nodes)
- **But**: Not production-ready for security platform

**Correct State**:
- Agent-Based architecture **IS REQUIRED**
- Agent = data plane (mandatory)
- Core = control plane (brain)
- **This is NOT optional**

### ADR Decision

```
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║  AGENT IS MANDATORY AS DATA PLANE                            ║
║                                                              ║
║  Core MUST NOT perform workload-level scanning               ║
║                                                              ║
║  This is an architectural correction, not a feature request  ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
```

**Enforcement**:
- All future code reviews check this
- No new scanning logic in Core
- Agent is deployed by default
- Documentation reflects Agent-Based

**Rationale**: Data gravity, industry best practices, scalability, efficiency.

---

**Status**: ✅ ACCEPTED  
**Next Action**: Implement Phase 1 (Move SBOM to Agent)  
**Timeline**: 3-4 weeks  
**Team**: 2 engineers  

**This ADR is final and binding.**

---

*Date: December 23, 2024*  
*Fortuna K8s Management Platform*  
*Agent-Based Architecture - Mandatory Decision*

