# KSAM Architecture

**Version**: 2.0 (MVP2)
**Last Updated**: December 15, 2025
**Status**: MVP1 Released (v4.3.0) | MVP2 In Progress (~60% Complete)
**Old Version**: [Archived](archive/old-architecture/ARCHITECTURE_v1_backup_20251215.md)

---

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Core Components](#core-components)
  - [KSAM Agent](#1-ksam-agent-daemonset)
  - [KSAM Core Controller](#2-ksam-core-controller)
  - [Policy Engine](#3-policy-engine)
  - [Risk Engine](#4-risk-engine)
  - [Graph Engine](#5-graph-engine-apache-age)
  - [CVE & SBOM](#6-cve--sbom-integration)
  - [Dashboard](#7-ksam-dashboard)
- [Data Flow](#data-flow)
- [Storage](#storage)
- [Security](#security)
- [Deployment](#deployment-architecture)
- [Roadmap](#development-roadmap)
- [Technology Stack](#technology-stack)
- [Performance](#performance--scalability)
- [Getting Started](#getting-started)

---

## Overview

**KSAM (Fortuna K8s Management Platform)** is a container security and observability platform that provides centralized inventory management, RBAC analysis, risk detection, policy enforcement, and compliance monitoring for multi-cluster Kubernetes environments.

### Core Philosophy

**Lightweight, graph-based, policy-driven security platform** with built-in CIS Benchmark compliance and automated remediation capabilities.

### Key Capabilities

- **Multi-Cluster Inventory**: Real-time collection of Pods, ServiceAccounts, Roles, RoleBindings across clusters
- **Graph-Based Analysis**: PostgreSQL + Apache AGE for relationship mapping and attack path analysis
- **Policy Enforcement**: CEL-based policy engine with admission webhook integration
- **Risk Detection**: YAML-based rule system with 25+ built-in CIS Benchmark rules
- **Automated Remediation**: Policy violation detection with actionable guidance
- **Compliance Monitoring**: CIS Kubernetes Benchmark v1.8 compliance tracking
- **Interactive Visualization**: D3.js force-directed graph with real-time updates

### Current State (MVP2)

```
✅ COMPLETE (MVP1 - Released v4.3.0)
├─ Agent-based inventory collection (Pods, SA, Roles, RoleBindings)
├─ NATS JetStream event processing
├─ Worker pipeline (Normalizer, Correlator, Risk)
├─ PostgreSQL + Apache AGE graph storage
├─ REST + gRPC APIs
├─ React dashboard with D3 visualization
└─ Risk detection with YAML rules

✅ COMPLETE (MVP2)
├─ Policy Engine with Google CEL evaluator
├─ Admission Webhook (ValidatingWebhookConfiguration)
├─ Risk Scoring V2 with auto-resolution
├─ Insights lifecycle management (soft delete, auto-resolved)
├─ Dashboard pages (all 9 pages implemented)
└─ mTLS for Agent-Core communication

⏳ IN PROGRESS (MVP2)
├─ CVE scanning (75% - extractors ✅, CVE matcher ✅, pipeline ✅, testing ⏳)
├─ SBOM generation (75% - extractors ✅, normalizer ✅, database ✅, testing ⏳)
└─ Attack path algorithms (20% - Apache AGE ready, algorithms needed)

⏳ PLANNED (MVP3+)
├─ eBPF runtime monitoring (deprioritized)
├─ ML-based anomaly detection
├─ Cross-cluster correlation
└─ Advanced compliance frameworks (SOC2, NIST)
```

---

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                       Kubernetes Cluster(s)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │
│  │   Node 1     │  │   Node 2     │  │   Node N     │             │
│  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │             │
│  │  │ Agent  │  │  │  │ Agent  │  │  │  │ Agent  │  │             │
│  │  │  (DS)  │  │  │  │  (DS)  │  │  │  │  (DS)  │  │             │
│  │  │        │  │  │  │        │  │  │  │        │  │             │
│  │  │ K8s API│  │  │  │ K8s API│  │  │  │ K8s API│  │             │
│  │  │ Watcher│  │  │  │ Watcher│  │  │  │ Watcher│  │             │
│  │  └───┬────┘  │  │  └───┬────┘  │  │  └───┬────┘  │             │
│  └──────┼───────┘  └──────┼───────┘  └──────┼───────┘             │
│         │                  │                  │                     │
│         └──────────────────┴──────────────────┘                    │
│                    │ gRPC/mTLS (port 9090)                          │
│                    │                                                │
│  ┌─────────────────▼────────────────────────────────────────┐      │
│  │          Admission Webhook (port 8443)                   │      │
│  │  ValidatingWebhookConfiguration for policy enforcement   │      │
│  └──────────────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────┘
                     │
         ┌───────────▼──────────────┐
         │   KSAM Core Controller   │
         │  ┌────────────────────┐  │
         │  │  Ingest API        │  │
         │  │  (gRPC + HTTP)     │  │
         │  │  Ports: 8080,9090  │  │
         │  └───────┬────────────┘  │
         │          │                │
         │  ┌───────▼────────────┐  │
         │  │  NATS JetStream    │  │
         │  │  (Event Bus)       │  │
         │  └────┬───┬───┬───────┘  │
         │       │   │   │           │
         │   ┌───▼───▼───▼────────┐ │
         │   │  Worker Pool        │ │
         │   ├─────────────────────┤ │
         │   │ • Normalizer        │ │
         │   │ • Correlator        │ │
         │   │ • Risk Worker       │ │
         │   │ • Policy Worker     │ │
         │   └──────────┬──────────┘ │
         │              │             │
         │   ┌──────────▼──────────┐ │
         │   │  Policy Engine      │ │
         │   │  (CEL Evaluator)    │ │
         │   └──────────┬──────────┘ │
         │              │             │
         │   ┌──────────▼──────────┐ │
         │   │  Risk Engine        │ │
         │   │  (YAML Rules)       │ │
         │   └─────────────────────┘ │
         └───────────┬─────────────────┘
                     │
         ┌───────────▼──────────────┐
         │     Storage Layer        │
         │  ┌────────────────────┐  │
         │  │   PostgreSQL 15    │  │
         │  │   + Apache AGE     │  │ ← Graph database
         │  │   (Graph Engine)   │  │ ← Attack paths
         │  └────────────────────┘  │
         │  ┌────────────────────┐  │
         │  │  NATS JetStream    │  │ ← Event persistence
         │  └────────────────────┘  │
         └───────────┬──────────────┘
                     │
         ┌───────────▼──────────────┐
         │      Dashboard           │
         │   (React + TypeScript)   │
         │  ┌────────────────────┐  │
         │  │ • D3 Graph View    │  │
         │  │ • Risk Center      │  │
         │  │ • Insights         │  │
         │  │ • Resources        │  │
         │  │ • Attack Paths     │  │
         │  │ • Rules Mgmt       │  │
         │  │ • Settings         │  │
         │  └────────────────────┘  │
         └──────────────────────────┘
```

### Data Flow Overview

```
1. COLLECTION FLOW
   Agent → K8s API Watch → gRPC Stream → Core Ingest API → NATS

2. PROCESSING FLOW
   NATS → Normalizer → Correlator → PostgreSQL/AGE → Risk Worker → Insights

3. POLICY FLOW
   Policy Worker → CEL Evaluator → Policy Instances → Violations

4. ENFORCEMENT FLOW
   Pod Creation → Admission Webhook → Policy Check → Allow/Deny

5. QUERY FLOW
   Dashboard → REST API → PostgreSQL/AGE → Graph/Tabular Data → UI
```

---

## Core Components

### 1. KSAM Agent (DaemonSet)

**Status**: ✅ Inventory Collection Complete | ⏳ eBPF Runtime Not Started

**Type**: Kubernetes DaemonSet
**Language**: Go
**Resource Footprint**: <100MB memory, <100m CPU
**Deployment**: One agent per node

#### Responsibilities

**Inventory Collection** (✅ Complete):
- Watch Kubernetes resources via Informers
- Collect: Pods, ServiceAccounts, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings
- Real-time synchronization with 30-second reconciliation
- Incremental updates (only changes sent)
- Efficient caching with in-memory state

**Communication** (✅ Complete):
- gRPC streaming to Core Controller (port 9090)
- mTLS authentication with automatic cert rotation
- Heartbeat every 30 seconds
- Automatic reconnection with exponential backoff

**RBAC Requirements**:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksam-agent
rules:
- apiGroups: [""]
  resources: ["pods", "nodes", "namespaces", "serviceaccounts"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
  verbs: ["get", "list", "watch"]
# Note: Read-only permissions, NO write access, NO secret access
```

#### Key Features

**Efficient Collection**:
- Kubernetes Informers for watch-based updates
- Delta compression (only changes sent)
- Batching (100 items per message)
- Deduplication of redundant events

**Resilience**:
- Automatic reconnection
- State persistence across restarts
- Graceful shutdown (30-second drain)
- Health checks (liveness/readiness probes)

**Security**:
- Minimal RBAC permissions
- mTLS for all communication
- No sensitive data collection (secrets, configmaps)
- Certificate auto-rotation

#### Future: eBPF Runtime Monitoring

**Status**: ⏳ Planned for MVP3 (Deprioritized)

Currently **not implemented** and **deprioritized** in favor of policy enforcement and CVE scanning.

---

### 2. KSAM Core Controller

**Status**: ✅ Core Complete | ⏳ Advanced Features Partial

**Type**: Kubernetes Deployment
**Language**: Go
**Resource Footprint**: 512Mi-2Gi memory, 100m-1000m CPU
**Replicas**: 1 (scalable to 3+ for HA)

#### 2.1 Ingest API

**Status**: ✅ Complete

**Protocols**:
- gRPC (port 9090) - Agent communication
- HTTP REST (port 8080) - Dashboard & external tools

**gRPC API**:
```protobuf
service AgentService {
  rpc RegisterAgent(AgentInfo) returns (AgentRegistration);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc StreamInventory(stream InventoryItem) returns (StreamResponse);
  rpc StreamCertificates(stream CertificateInfo) returns (StreamResponse);
}
```

**REST API** (Key Endpoints):
```
Authentication:
  POST /api/v1/auth/login
  POST /api/v1/auth/refresh

Inventory:
  GET  /api/v1/clusters
  GET  /api/v1/serviceaccounts
  GET  /api/v1/pods
  GET  /api/v1/roles

Insights:
  GET  /api/v1/insights
  PUT  /api/v1/insights/:id/resolve

Risk:
  GET  /api/v1/risk/scores
  GET  /api/v1/risk/trends

Graph:
  POST /api/v1/graph/query
  GET  /api/v1/graph/attack-paths

Policy:
  GET  /api/v1/policy-templates
  POST /api/v1/policy-instances
  GET  /api/v1/policy-violations
```

#### 2.2 Message Queue (NATS JetStream)

**Status**: ✅ Complete

**Subject Hierarchy**:
```
ksam.inventory.pods
ksam.inventory.serviceaccounts
ksam.inventory.roles
ksam.normalized.pods
ksam.graph.updated
ksam.insights.created
ksam.policy.violations
```

**Configuration**:
```yaml
jetstream:
  enabled: true
  max_memory: 1GB
  max_storage: 10GB
  retention_policy: limits
  max_age: 7d
  replicas: 1
```

#### 2.3 Worker Pipeline

**Status**: ✅ Complete

1. **Normalizer Worker** - Convert proto/JSON to canonical schema
2. **Correlator Worker** - Build graph relationships in PostgreSQL/AGE
3. **Risk Worker** - Evaluate YAML rules, generate insights
4. **Policy Worker** - Evaluate CEL policies, detect violations

**Concurrency**: 5 workers per type (20 total workers)

---

### 3. Policy Engine

**Status**: ✅ Complete (MVP2)

The policy engine provides CEL-based policy evaluation with admission webhook integration.

#### Policy Template Format (YAML)

```yaml
apiVersion: policy.ksam.io/v1
kind: PolicyTemplate
metadata:
  name: block-privileged-containers
spec:
  category: security
  severity: critical
  expression: |
    object.spec.containers.all(c,
      !has(c.securityContext.privileged) ||
      c.securityContext.privileged == false
    )
  enforcement:
    mode: block  # block | warn | audit
  remediation:
    description: "Remove privileged: true from securityContext"
    autoFixable: false
```

#### CEL Evaluator

**Google CEL (Common Expression Language)** provides:
- Type safety with compile-time checking
- Performance (compiled to bytecode, cached)
- Security (sandboxed execution)
- Rich standard library

#### Admission Webhook

**Status**: ✅ Complete

Integrates with Kubernetes ValidatingWebhookConfiguration:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: ksam-webhook
webhooks:
- name: policy.ksam.io
  clientConfig:
    service:
      name: ksam-core
      namespace: ksam-system
      path: /api/v1/webhook/validate
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
  failurePolicy: Fail
  timeoutSeconds: 5
```

**Enforcement Modes**:
- **Block**: Reject resource creation
- **Warn**: Allow with warning message
- **Audit**: Allow and log violation

---

### 4. Risk Engine

**Status**: ✅ Complete with V2 Algorithm (MVP2)

#### YAML Rule Format

```yaml
apiVersion: risk.ksam.io/v1
kind: RiskRule
metadata:
  name: privileged-container
spec:
  severity: critical  # critical, high, medium, low
  compliance:
    - framework: CIS
      version: "1.8"
      control: "5.2.1"
  condition: |
    resource.kind == "Pod" &&
    resource.spec.containers.exists(c,
      c.securityContext.privileged == true
    )
  scoring:
    baseScore: 90
  insight:
    title: "Privileged container: {{.PodName}}"
    remediation:
      steps:
        - "Remove 'privileged: true' from securityContext"
```

#### Built-in Rules (25+ Rules)

**CIS Kubernetes Benchmark v1.8**:
- 5.1.3: Minimize cluster-admin role bindings
- 5.2.1: Privileged containers
- 5.2.2: Host PID/IPC namespace sharing
- 5.2.3: Host network access
- 5.2.6: NET_RAW capability
- 5.2.9: Dangerous capabilities
- 5.3.1: Network policy enforcement
- 5.7.2: Resource limits

#### Risk Scoring V2 Algorithm

```go
func CalculateRiskScore(insight *Insight, resource *Resource) float64 {
    baseScore := severityScores[insight.Severity]
    exposureMultiplier := getExposureMultiplier(resource)
    ageMultiplier := getAgeMultiplier(insight)
    permMultiplier := getPermissionMultiplier(resource)

    finalScore := baseScore * exposureMultiplier * ageMultiplier * permMultiplier
    return min(finalScore, 100.0)
}
```

#### Insight Lifecycle

**Status Transitions**:
- `active` - Issue exists
- `resolved` - Manually resolved
- `auto_resolved` - System detected fix
- `ignored` - User accepted risk

**Auto-Resolution**: System automatically marks insights as resolved when the underlying issue is fixed

**Soft Delete**: Resolved insights soft-deleted after 30 days, purged after 90 days

---

### 5. Graph Engine (Apache AGE)

**Status**: ✅ Complete (MVP2)

KSAM uses Apache AGE (A Graph Extension) for PostgreSQL.

#### Why Apache AGE?

- ✅ PostgreSQL extension (reuse existing infrastructure)
- ✅ SQL + Cypher queries in same database
- ✅ ACID transactions
- ✅ No additional database cluster
- ✅ Open source, battle-tested

#### Schema

```sql
-- Node types
CREATE VLABEL ServiceAccount;
CREATE VLABEL Pod;
CREATE VLABEL Role;
CREATE VLABEL RoleBinding;

-- Edge types
CREATE ELABEL USES_SERVICE_ACCOUNT;
CREATE ELABEL HAS_ROLE_BINDING;
CREATE ELABEL BINDS_TO_ROLE;
CREATE ELABEL MOUNTS_SECRET;
```

#### Query Examples

**Find Pods using cluster-admin**:
```cypher
MATCH (p:pod)-[:USES_SERVICE_ACCOUNT]->(sa:serviceaccount)
      -[:HAS_ROLE_BINDING]->(rb:rolebinding)
      -[:BINDS_TO_ROLE]->(r:clusterrole)
WHERE r.name = 'cluster-admin'
RETURN p.name, sa.name
```

**Attack path analysis**:
```cypher
MATCH path = (p:pod)-[*1..5]->(s:secret)
WHERE p.namespace <> s.namespace
RETURN path
ORDER BY length(path) ASC
LIMIT 10
```

#### Performance

**Benchmarks** (10K nodes, 50K edges):
- Simple path query (1-hop): ~5ms
- Complex path query (3-hop): ~50ms
- Attack path analysis (5-hop): ~200ms

---

### 6. CVE & SBOM Integration

**Status**: ⏳ Nearly Complete (MVP2 In Progress - ~75% Complete) - Updated Dec 15, 2025

KSAM uses a **zero-dependency, custom SBOM pipeline** that extracts package information from container images and matches against CVE databases **without requiring external tools**.

#### Architecture Philosophy

**Zero External Dependencies**:
- ✅ No Trivy CLI/Server required
- ✅ No Syft/Grype binaries needed
- ✅ Custom extractors for all package formats
- ✅ Direct image layer analysis
- ✅ Uses local OSV JSON dataset (`/cve-data`) loaded into PostgreSQL (no third-party scanner service)

**Benefits**:
- Lightweight deployment (<50MB additional footprint)
- No license concerns (pure data reading)
- Full control over scanning logic
- Fast scanning (no external process overhead)
- Secure (no external binary execution)

#### Current State

**✅ Implementation Complete (~75%)**:

1. **Custom SBOM Extractors** (✅ 100% - 11 files, ~1,500 LOC)
   - DpkgParser (Debian/Ubuntu packages)
   - ApkParser (Alpine packages)
   - NpmParser (Node.js packages from package.json)
   - PipParser (Python packages from requirements.txt)
   - GoModParser (Go modules from go.mod)
   - Filesystem extraction and tar archive handling

2. **SBOM Normalizer** (✅ 100% - ~300 LOC)
   - CycloneDX format conversion
   - PURL (Package URL) generation
   - Component deduplication
   - License detection

3. **CVE Database Manager** (✅ 100% - 3 files, ~300 LOC)
   - PostgreSQL-backed CVE manager (tables: `cves`, `package_vulnerabilities`)
   - Data source: OSV JSON files in `/cve-data/all` loaded via `core/cmd/cve-loader`
   - In-memory caching (1-hour TTL)

4. **Version Comparator** (✅ 90% - 233 LOC)
   - Debian version comparison (with epoch support)
   - RPM version comparison
   - Alpine version comparison
   - Semantic versioning (npm, pypi, go) using hashicorp/go-version
   - Edge case testing in progress

5. **CVE Matcher** (✅ 80% - 3 files, 441 LOC)
   - PURL parser - 67 LOC
   - CVE matching logic - 141 LOC
   - Version comparison integration - 233 LOC
   - Basic enrichment (needs enhancement)

6. **Pipeline Integration** (✅ 70% - 346 LOC)
   - Feature flags (KSAM_SBOM_USE_CUSTOM)
   - Dual-mode support (custom + legacy)
   - Event-driven scanning (SBOM_CREATED → CVE matching)
   - Testing in progress

7. **Database Schema** (✅ 100%)
   - CVE tables (cves, vulnerabilities)
   - SBOM tables (sboms, sbom_components)
   - Image scan results
   - Migrations complete

**⏳ Remaining Work (~25%)**:

1. **Testing & Validation** (⏳ In Progress)
   - Integration testing with real container images (nginx, alpine, ubuntu)
   - CVE matching accuracy validation against known vulnerabilities
   - Performance benchmarks (target: <15s per image scan)
   - Edge case testing (version comparisons, malformed SBOM data)
   - Load testing (1000+ pods)

2. **Monitoring & Metrics** (⏳ Partial)
   - Prometheus metrics for scan duration
   - CVE match rate tracking
   - SBOM extraction success/failure rates
   - Cache hit/miss ratios
   - Alert rules for scan failures

3. **OSV Dataset Refresh Automation** (⏳ Planned)
   - Automate `cve-loader` execution against `/cve-data/all`
   - Track loader version + dataset checksum
   - Re-run CVE matching for affected SBOMs via `needs_recheck`

4. **Documentation** (⏳ Partial)
   - Operator guide for CVE/SBOM features
   - Troubleshooting guide
   - Performance tuning recommendations
   - Architecture decision records (ADRs)

#### CVE Dataset (OSV JSON) Clarification

**IMPORTANT**: KSAM does **not** depend on any third-party scanning tool/server.
We use **OSV JSON files** stored in `/cve-data/all` and load them into PostgreSQL with `cve-loader`.

#### Custom SBOM Pipeline Flow

```
1. POD EVENT (Inventory Pipeline)
   Agent → NATS (ksam.raw.pods) → Normalizer → NATS (ksam.normalized.pods)

2. IMAGE DIGEST RESOLUTION (Cache-First)
   SBOMWorker → Resolve digest (immutable) → Check sboms(image_digest)

3. SBOM GENERATION (Only on cache-miss)
   Extract layers → Parse package files → Normalize to CycloneDX → Persist sboms + sbom_components

4. SBOM_CREATED EVENT
   Publish `ksam.sbom.created` (sbom_id + pod/container context)

5. CVE MATCHING (PostgreSQL / OSV)
   CVEMatcherWorker → Query (cves + package_vulnerabilities) → Persist cve_matches (dedup)

6. INSIGHTS + RISK
   Create vulnerability insights (HIGH/CRITICAL) → Risk score auto-calculated → Dashboard
```

#### Data Models

```go
// CVE (Common Vulnerabilities and Exposures)
type CVE struct {
    ID              string    // CVE-2024-1234
    Description     string
    Severity        string    // critical, high, medium, low
    CVSSScore       float64   // 0.0-10.0
    CVSSVector      string
    Published       time.Time
    Modified        time.Time
    References      []string
    Constraint      string    // Version constraint (e.g., "< 1.2.3")
    FixedVersion    string    // Version with fix
}

// Vulnerability (CVE instance in a specific image)
type Vulnerability struct {
    ID               string
    ImageName        string
    ImageDigest      string
    CVEID            string
    PackageName      string
    PackageVersion   string
    Ecosystem        string    // deb, apk, npm, pip, go
    Severity         string
    CVSSScore        float64
    FixedVersion     string
    Status           string    // detected, patched, ignored
    DetectedAt       time.Time
    ResolvedAt       *time.Time
}

// SBOM (Software Bill of Materials)
type SBOM struct {
    ID              string
    ImageName       string
    ImageDigest     string
    Format          string    // CycloneDX, SPDX
    SpecVersion     string    // 1.4
    Components      []Component
    Dependencies    []Dependency
    GeneratedAt     time.Time
    GeneratorName   string    // KSAM Custom SBOM Generator
}

type Component struct {
    Name            string
    Version         string
    Type            string    // library, framework, application
    PURL            string    // pkg:npm/express@4.18.0
    Ecosystem       string    // npm, pip, deb, apk
    License         string
    Hash            string
    Supplier        string
}
```

#### Performance Characteristics

**SBOM Generation**:
- Small image (Alpine): ~2-5 seconds
- Medium image (Ubuntu): ~5-10 seconds
- Large image (Full stack): ~10-20 seconds

**CVE Matching**:
- PostgreSQL lookup (indexed): ~2-15ms per package (depends on dataset size + cache)
- Caching: ~1ms per package (subsequent lookups)

**Overall Scanning**:
- Complete scan (100 packages): ~30-60 seconds
- Cached scan: ~5-10 seconds
- Parallel processing: 5 images concurrently

#### Roadmap

**Short Term** (Next 4-6 weeks):
- ✅ Complete version comparison logic
- ✅ Implement automatic scanning pipeline
- ✅ NATS integration for scan events
- ⏳ OSV dataset refresh automation (re-run `cve-loader` + recheck affected SBOMs)
- ✅ Vulnerability-to-Insight conversion

**Medium Term** (2-3 months):
- ⏳ Add RPM package support
- ⏳ Add Java/Maven support
- ⏳ Add Ruby Gem support
- ⏳ License compliance checking
- ⏳ SBOM export API (CycloneDX, SPDX)

**Long Term** (3-6 months):
- 📋 Real-time CVE alerting
- 📋 Automated patching recommendations
- 📋 Supply chain risk analysis
- 📋 SBOM diffing (image versions)
- 📋 Integration with CI/CD pipelines

---

### 7. KSAM Dashboard

**Status**: ✅ Complete (MVP2)

React-based dashboard with 9 pages and D3.js graph visualization.

#### Technology Stack

- **Framework**: React 18
- **Language**: TypeScript
- **Build Tool**: Vite
- **State Management**: Zustand
- **Data Fetching**: TanStack React Query
- **Visualization**: D3.js v7, Recharts
- **Styling**: Tailwind CSS

#### Pages

1. **Dashboard** - Overview, metrics, risk trends
2. **Clusters** - Multi-cluster management
3. **Resources** - Inventory table
4. **Insights** - Security insights
5. **Risk Center** - Risk scores, distribution
6. **Risk-Driven Inventory** - Resources by risk
7. **Attack Paths** - Graph visualization
8. **Rules Management** - Policy management
9. **Settings** - User preferences

#### D3 Graph Visualization

**Features**:
- Force-directed graph layout
- Node types: Pod, ServiceAccount, Role, Secret
- Interactive (click, drag, filter)
- Multiple layouts (Force, Radial, Tree, Grid)
- Pink "K8s Fortuna" branding
- Real-time updates

---

## Data Flow

### 1. Inventory Collection Flow

```
Agent → K8s Watch → gRPC Stream → Core API → NATS
→ Normalizer → Correlator → PostgreSQL/AGE → Dashboard
```

### 2. Policy Enforcement Flow

```
kubectl apply → K8s API → Admission Webhook → Policy Engine
→ CEL Evaluation → Allow/Deny → K8s API → User
```

### 3. Risk Evaluation Flow

```
Resource Change → Risk Worker → YAML Rules → CEL Evaluation
→ Insight Generator → Risk Scorer → PostgreSQL → Dashboard
```

### 4. Auto-Resolution Flow

```
Scheduled Job → Insight Status Updater → Re-evaluate Rules
→ Update Status → Soft Delete (30 days) → Purge (90 days)
```

---

## Storage

### PostgreSQL 15 with Apache AGE

**Status**: ✅ Complete

**Tables**:
- Core: clusters, agents, pods, serviceaccounts, roles, rolebindings
- Security: insights, policy_templates, policy_violations, risk_scores
- CVE: cves, vulnerabilities, sboms (partial)
- Graph: ksam_graph (Apache AGE vertices and edges)

**Migrations**: 14 migration files

**Configuration**:
```yaml
postgresql:
  max_connections: 50
  max_idle_connections: 10
  connection_max_lifetime: 1h
```

### NATS JetStream

**Status**: ✅ Complete

**Purpose**: Event bus for async processing

**Configuration**:
```yaml
jetstream:
  max_memory: 1GB
  max_storage: 10GB
  retention: 7d
```

---

## Security

### Authentication & Authorization

**Status**: ✅ Complete

- JWT-based authentication
- Bcrypt password hashing
- Session expiration (24 hours)
- API authorization middleware

### Communication Security

**Status**: ✅ Complete

**mTLS (Agent ↔ Core)**:
- Mutual TLS authentication
- Certificate rotation (90-day validity)
- CA certificate management

**HTTPS (Dashboard ↔ Core)**:
- TLS 1.3
- Strong cipher suites

---

## Deployment Architecture

### Kubernetes Resources

**KSAM Agent** (DaemonSet):
- 1 agent per node
- <100MB memory, <100m CPU

**KSAM Core** (Deployment):
- 1-3 replicas
- 512Mi-2Gi memory, 100m-1000m CPU
- Ports: 8080 (HTTP), 9090 (gRPC), 8443 (Webhook)

**PostgreSQL** (StatefulSet):
- 1 replica
- 20Gi persistent storage

**NATS** (Deployment):
- 1 replica
- JetStream enabled

**Dashboard** (Deployment):
- 2 replicas
- Nginx serving static assets

See [Deployment Guide](guides/setup/) for details.

---

## Development Roadmap

### MVP1 - Foundation ✅ COMPLETE

**Released**: v4.3.0 (December 2025)

**Features**:
- ✅ Agent inventory collection
- ✅ NATS JetStream
- ✅ Worker pipeline
- ✅ PostgreSQL + Apache AGE
- ✅ REST + gRPC APIs
- ✅ Dashboard with D3 visualization
- ✅ Risk detection (25+ YAML rules)
- ✅ mTLS security

**Test Coverage**: 87.5% (28/32 passing)

---

### MVP2 - Enterprise Features ⏳ 65% COMPLETE

**Target**: January 2026 (Updated Dec 15: CVE/SBOM ahead of schedule at 75%)

**Completed** (✅):
- ✅ Policy Engine (CEL evaluator)
- ✅ Admission Webhook
- ✅ Risk Scoring V2
- ✅ Insight lifecycle
- ✅ All dashboard pages
- ✅ Apache AGE graph queries

**In Progress** (⏳):
- ⏳ CVE scanning (75% - extractors ✅, matcher ✅, database ✅, testing in progress)
- ⏳ SBOM generation (75% - extractors ✅, normalizer ✅, pipeline ✅, testing in progress)
- ⏳ Attack path algorithms (20% - graph engine ready, algorithms needed)

**Remaining Work**:
- CVE/SBOM testing and validation (integration tests, performance benchmarks)
- OSV dataset refresh automation (re-run `cve-loader` + controlled recheck of affected SBOMs)
- Attack path detection algorithms (graph traversal, risk scoring)
- Notification integrations (Slack, email, webhook)
- Operator documentation and guides

**Estimated Completion**: 4-6 weeks

---

### MVP3 - Advanced Analytics 📋 PLANNED

**Target**: Q1 2026

**Features**:
- eBPF runtime monitoring
- ML-based anomaly detection
- Automated policy generation
- Cross-cluster correlation
- Advanced compliance (SOC2, NIST)

**Estimated Effort**: 12-14 weeks

---

## Technology Stack

### Backend

- **Language**: Go 1.21+
- **Frameworks**: Gin, gRPC, GORM
- **Queue**: NATS JetStream
- **Policy**: Google CEL
- **Graph**: Apache AGE

### Frontend

- **Language**: TypeScript 5.x
- **Framework**: React 18
- **Build**: Vite
- **State**: Zustand
- **Visualization**: D3.js v7, Recharts

### Infrastructure

- **Orchestration**: Kubernetes 1.28+
- **Database**: PostgreSQL 15 + Apache AGE
- **Monitoring**: Prometheus + Grafana

---

## Performance & Scalability

### Performance Targets

**Agent**:
- Memory: <100MB
- CPU: <100m
- Latency: <100ms

**Core**:
- API Response: <100ms (p95)
- Throughput: 1000 req/s
- Graph Query: <200ms (5-hop)

**Dashboard**:
- Initial Load: <2s
- Graph Render: <1s (500 nodes)

### Scalability

**Horizontal Scaling**:
- Core: 1-5 replicas
- Workers: Auto-scale on queue depth
- Dashboard: 2-10 replicas

**Tested With**:
- 10 clusters, 1000 nodes, 10K pods

**Estimated Max**:
- 50 clusters, 5K nodes, 50K pods

---

## Testing

### Current Coverage

- E2E Tests: 87.5% passing (28/32)
- Unit Tests: ~40% coverage
- Integration Tests: Core systems tested

### Test Scripts

Located in `/scripts`:
- `test_all_apis.sh`
- `test_end_to_end_event_flow.sh`
- `test_mtls_traffic_encryption.sh`
- `run_all_tests.sh`

---

## Getting Started

### Prerequisites

- Kubernetes cluster (minikube, kind, or cloud)
- kubectl configured
- Docker

### Quick Start

See **[Setup Guide](guides/setup/MINIKUBE_SETUP.md)** for detailed instructions.

**1. Deploy Infrastructure**:
```bash
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
```

**2. Deploy KSAM**:
```bash
kubectl apply -f deploy/core-deployment.yaml
kubectl apply -f deploy/agent-daemonset.yaml
kubectl apply -f deploy/dashboard-deployment.yaml
```

**3. Access Dashboard**:
```bash
kubectl port-forward -n ksam-system svc/ksam-dashboard 8080:80
# Open http://localhost:8080
# Login: admin / admin
```

---

## Differentiation from Competitors

| Feature | KSAM | Kubescape | Falco | Aqua |
|---------|------|-----------|-------|------|
| **Policy Engine** | ✅ CEL + Webhook | ✅ Rego | ❌ | ✅ |
| **Graph Analysis** | ✅ Apache AGE | ❌ | ❌ | ⚠️ |
| **Attack Paths** | ✅ Graph-based | ❌ | ❌ | ❌ |
| **Open Source** | ✅ | ✅ | ✅ | ❌ |
| **Self-Hosted** | ✅ | ✅ | ✅ | ⚠️ |
| **Cost** | Free | Free | Free | $$$ |

### Key Differentiators

1. **Graph-Based Analysis**: True graph queries with Apache AGE
2. **Policy-Driven**: CEL policies with admission webhook
3. **Lightweight**: <100MB agent footprint
4. **CIS Benchmark Native**: Built-in compliance
5. **Auto-Resolution**: Intelligent insight lifecycle
6. **Open Source**: Fully self-hosted, no lock-in

---

## Contributing

See contributing guidelines in the main repository.

---

## Support

- **Documentation**: [docs/](.)
- **Issues**: GitHub Issues
- **Email**: support@ksam.io

---

**Last Updated**: December 15, 2025
**Version**: 2.0 (MVP2)
**Maintained By**: KSAM Team
