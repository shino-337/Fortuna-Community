# Fortuna K8s Management Platform - Comprehensive Project Analysis

**Date**: December 22, 2024  
**Purpose**: Complete analysis for next phase planning  
**Status**: ✅ Analysis Complete

---

## 📋 Executive Summary

Fortuna (formerly KSAM) is a **Kubernetes-native security and management platform** that provides:
- **SBOM Generation**: Custom zero-dependency SBOM extractor
- **CVE Scanning**: Vulnerability matching against 74,561+ CVEs
- **Risk Scoring**: V2 algorithm with multi-factor analysis
- **Policy Engine**: CEL-based admission control
- **Insights Management**: Automated security findings
- **Graph Analysis**: Apache AGE for attack path visualization

**Current Architecture**: **Hybrid** - Documentation shows both Agent-Based and Core-Only models
- **Code Implementation**: Agent-Based (Agent + Core)
- **Documentation**: Mixed (some docs mention Core-Only, some mention Agent-Based)

---

## 🏗️ Architecture Analysis

### Current State: Dual Architecture Documentation

#### 1. Agent-Based Architecture (Code Implementation)
**Location**: `docs/02-architecture/README.md`, `agent/`, `core/internal/grpc/`

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
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │              │
│         └─────────────────┼─────────────────┘              │
│                           │ mTLS gRPC                      │
│                           ▼                                │
│                  ┌──────────────────┐                       │
│                  │  Fortuna Core    │                       │
│                  │  (Deployment)    │                       │
│                  │                  │                       │
│                  │ • gRPC Server    │                       │
│                  │ • SBOM Ingestion │                       │
│                  │ • CVE Matching   │                       │
│                  │ • Policy Engine  │                       │
│                  │ • Risk Engine    │                       │
│                  │ • API Server     │                       │
│                  └────────┬─────────┘                       │
│                           │                                 │
│              ┌────────────┼────────────┐                    │
│              ▼            ▼            ▼                    │
│         ┌─────────┐  ┌────────┐  ┌──────────┐              │
│         │PostgreSQL│  │  NATS  │  │Dashboard │              │
│         │  + AGE   │  │JetStream│  │  (Web)   │              │
│         └─────────┘  └────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────────┘
```

**Key Characteristics**:
- **Agent (DaemonSet)**: One per node, watches local pods only
- **SBOM Extraction**: Done locally on each node (docker socket access)
- **Communication**: mTLS gRPC from Agent → Core
- **CVE Matching**: Done in Core (not in Agent)
- **Scalability**: Distributed SBOM extraction, centralized processing

#### 2. Core-Only Architecture (Documentation Mention)
**Location**: Some documentation references

**Key Characteristics**:
- Core handles all processing (collection, SBOM, CVE, insights)
- Agent is disabled/not needed
- Simpler deployment (single pod vs N pods)
- Single-cluster focused

**⚠️ DISCREPANCY**: Code implements Agent-Based, but some docs mention Core-Only

---

## 🔄 Complete Logic Flow

### Phase 1: Pod Discovery & SBOM Generation

```
1. Pod Created in Kubernetes
   │
   ├─> Agent (DaemonSet) detects pod via K8s informer
   │   └─> Field selector: spec.nodeName = <agent's node>
   │
   ├─> Agent verifies pod is on local node
   │   └─> If not, skip (safety check)
   │
   ├─> Agent extracts SBOM for each container
   │   ├─> Uses local docker socket
   │   ├─> Custom extractor (zero external tools)
   │   ├─> Extracts packages from image layers
   │   └─> Generates PURLs for each package
   │
   ├─> Agent converts to proto format
   │   └─> SBOMFinding proto message
   │
   └─> Agent sends to Core via mTLS gRPC
       └─> SendSBOMFinding() RPC call
```

**Code Path**:
- `agent/internal/watcher/pod_watcher_local.go` - Watches pods
- `agent/internal/sbom/processor.go` - Processes pods, extracts SBOM
- `agent/internal/client/grpc_client_mtls.go` - Sends to Core
- `core/internal/grpc/handler_sbom.go` - Receives SBOM

---

### Phase 2: SBOM Storage & Event Publishing

```
1. Core receives SBOMFinding via gRPC
   │
   ├─> Core validates SBOM data
   │
   ├─> Core starts database transaction
   │
   ├─> Core inserts SBOM record
   │   └─> Table: sboms
   │       - PodUID, PodName, Namespace
   │       - ContainerName, ImageDigest
   │       - AgentID, NodeID
   │       - GeneratedAt timestamp
   │
   ├─> Core inserts SBOM components
   │   └─> Table: sbom_components
   │       - SBOMID (FK)
   │       - ComponentType, ComponentName, ComponentVersion
   │       - PURL (Package URL)
   │       - Licenses, Source, Description
   │
   ├─> Core commits transaction
   │
   └─> Core publishes SBOM_CREATED event to NATS
       └─> Subject: "ksam.sbom.created"
           - SBOMID
           - Pod context (UID, name, namespace)
           - ImageDigest
```

**Code Path**:
- `core/internal/grpc/handler_sbom.go` - Receives and stores SBOM
- `core/pkg/sbom/events.go` - Event definition
- `core/pkg/messaging/nats.go` - NATS publishing

---

### Phase 3: CVE Matching

```
1. NATS: SBOM_CREATED event published
   │
   ├─> CVEMatcherWorker subscribes to "ksam.sbom.created"
   │   └─> Worker pool with concurrency control
   │
   ├─> Worker loads SBOM from database
   │   └─> Verify SBOM exists (may have been deleted)
   │
   ├─> Worker loads SBOM components
   │   └─> Query: SELECT * FROM sbom_components WHERE sbom_id = ?
   │
   ├─> For each component:
   │   │
   │   ├─> Parse PURL (Package URL)
   │   │   └─> Extract: ecosystem, name, version
   │   │
   │   ├─> Normalize ecosystem
   │   │   └─> Map to DB values (debian/ubuntu/alpine/go)
   │   │
   │   ├─> Query CVE database
   │   │   └─> SELECT * FROM package_vulnerabilities
   │   │       WHERE ecosystem = ? AND package_name = ?
   │   │
   │   ├─> For each CVE found:
   │   │   │
   │   │   ├─> Check version constraint
   │   │   │   └─> Is component version vulnerable?
   │   │   │       - Uses semver comparison
   │   │   │       - Handles ranges (e.g., "< 2.0.0")
   │   │   │
   │   │   └─> If vulnerable:
   │   │       └─> Create CVEMatch record
   │   │           - SBOMID, PodUID, ContainerName
   │   │           - CVEID, PackageName, PackageVersion
   │   │           - Severity, CVSS, FixedVersion
   │   │
   │   └─> Persist matches (with deduplication)
   │
   └─> Create insights (CRITICAL/HIGH only)
       └─> One insight per CVE match
```

**Code Path**:
- `core/pkg/worker/cve_matcher_worker.go` - Worker implementation
- `core/pkg/cve/matcher/matcher.go` - CVE matching logic
- `core/pkg/cve/db_manager.go` - Database queries
- `core/pkg/cve/version/constraint.go` - Version comparison

**CVE Database Structure**:
- `cves` table: CVE metadata (ID, description, severity, CVSS)
- `package_vulnerabilities` table: Package-to-CVE mapping
  - ecosystem, package_name, version_constraint
  - Links to cves table

---

### Phase 4: Insight Creation

```
1. CVEMatch created
   │
   ├─> Filter by severity (CRITICAL/HIGH only)
   │   └─> Configurable: CVEMatcherWorker.onlySeverities
   │
   ├─> Create Insight record
   │   └─> Table: insights
   │       - ResourceType: "Pod"
   │       - ResourceUID: Pod UID
   │       - ResourceName: Pod name
   │       - ResourceNamespace: Pod namespace
   │       - InsightType: "vulnerability"
   │       - Title: CVE ID
   │       - Description: CVE description
   │       - Severity: CRITICAL/HIGH/MEDIUM/LOW
   │       - Status: "active"
   │       - AffectedResources: JSONB array
   │         [{"uid": "pod-uid", "type": "Pod"}]
   │       - Metadata: JSONB with CVE details
   │
   └─> Insight available via API
       └─> GET /api/v1/insights
```

**Code Path**:
- `core/pkg/worker/cve_matcher_worker.go` - Creates insights
- `core/pkg/models/insight.go` - Insight model
- `core/internal/api/handlers.go` - API endpoints

---

### Phase 5: Risk Scoring

```
1. Insight created or updated
   │
   ├─> Risk Worker processes resource
   │   └─> Subject: "ksam.normalized.>"
   │
   ├─> Risk Scorer calculates score
   │   └─> Formula V2:
   │       TotalScore = (BaseScore + ExploitabilityScore + BusinessImpactScore) × TimeDecay
   │
   ├─> Base Score (0-40)
   │   ├─> CVE insights: CVSS-based
   │   │   - CRITICAL: 30-40
   │   │   - HIGH: 20-30
   │   │   - MEDIUM: 10-20
   │   │   - LOW: 0-10
   │   │
   │   └─> Policy insights: Severity-based
   │       - CRITICAL: 30-40
   │       - HIGH: 20-30
   │       - etc.
   │
   ├─> Exploitability Score (0-30)
   │   ├─> Public exploit available: +15
   │   ├─> Network accessible: +10
   │   ├─> Privilege escalation: +5
   │   └─> Recent CVE (< 30 days): +5
   │
   ├─> Business Impact Score (0-30)
   │   ├─> Production namespace: +15
   │   ├─> Critical workload: +10
   │   ├─> High resource count: +5
   │   └─> External exposure: +10
   │
   ├─> Time Decay (0.7-1.0)
   │   └─> Older insights get lower scores
   │       - < 7 days: 1.0
   │       - 7-30 days: 0.9
   │       - 30-90 days: 0.8
   │       - > 90 days: 0.7
   │
   ├─> Priority Level (P0-P4)
   │   └─> Based on TotalScore:
   │       - P0: 90-100 (Critical)
   │       - P1: 70-89 (High)
   │       - P2: 50-69 (Medium)
   │       - P3: 30-49 (Low)
   │       - P4: 0-29 (Minimal)
   │
   └─> Store RiskScore
       └─> Table: risk_scores
           - ResourceUID, ResourceType
           - TotalScore, BaseScore, ExploitabilityScore, BusinessImpactScore
           - TimeDecay, PriorityLevel
           - ScorerVersion: "v2"
```

**Code Path**:
- `core/pkg/risk/scorer.go` - V2 risk scoring
- `core/pkg/worker/risk_worker.go` - Risk evaluation worker
- `core/pkg/models/risk_score.go` - Risk score model

**Scheduling**:
- Historical risk evaluator runs every 6 hours
- Updates risk scores for all resources with insights

---

### Phase 6: Policy Evaluation

```
1. Resource created/updated in Kubernetes
   │
   ├─> NormalizerWorker normalizes resource
   │   └─> Subject: "ksam.raw.>"
   │
   ├─> Normalized resource published
   │   └─> Subject: "ksam.normalized.>"
   │
   ├─> Policy Worker subscribes
   │   └─> Subject: "ksam.normalized.>"
   │
   ├─> Policy Engine evaluates resource
   │   ├─> Load applicable policy instances
   │   │   └─> Query: SELECT * FROM policy_instances
   │   │       WHERE enabled = true
   │   │       AND (namespace = ? OR namespace = '*')
   │   │
   │   ├─> For each policy instance:
   │   │   │
   │   │   ├─> Load policy template
   │   │   │   └─> CEL expression
   │   │   │
   │   │   ├─> Evaluate CEL expression
   │   │   │   └─> Input: Resource spec, labels, annotations
   │   │   │
   │   │   └─> If violation:
   │   │       ├─> Determine action (alert/deny/warn)
   │   │       │
   │   │       ├─> If admission webhook:
   │   │       │   └─> Deny resource creation/update
   │   │       │
   │   │       └─> Create insight
   │   │           - InsightType: "policy_violation"
   │   │           - Severity: From policy
   │   │           - Recommendation: Policy remediation
   │   │
   │   └─> Store insights
   │
   └─> Risk score updated (if insights created)
```

**Code Path**:
- `core/pkg/policy/evaluator.go` - CEL evaluation
- `core/pkg/policy/enforcement.go` - Policy enforcement
- `core/internal/webhook/` - Admission webhook
- `core/pkg/worker/risk_worker.go` - Policy-based insights

---

## 🧩 Component Deep Dive

### 1. Agent Component

**Location**: `agent/`

**Responsibilities**:
- ✅ Watch pods on local node (K8s informer with field selector)
- ✅ Extract SBOM from container images (custom extractor)
- ✅ Send SBOM to Core via mTLS gRPC
- ❌ **NOT responsible for CVE matching** (done in Core)

**Key Files**:
- `agent/cmd/main.go` - Entry point
- `agent/internal/watcher/pod_watcher_local.go` - Pod watcher
- `agent/internal/sbom/processor.go` - SBOM processing
- `agent/internal/client/grpc_client_mtls.go` - gRPC client
- `agent/pkg/sbom/extractor/` - Custom SBOM extractor

**Configuration**:
```yaml
NODE_NAME: <from K8s downward API>
CORE_GRPC_ENDPOINT: fortuna-core.fortuna.svc.cluster.local:9090
TLS_ENABLED: "true"
TLS_CERT_PATH: /etc/fortuna/tls/client/tls.crt
TLS_KEY_PATH: /etc/fortuna/tls/client/tls.key
TLS_CA_CERT_PATH: /etc/fortuna/tls/client/ca.crt
```

**Deployment**: DaemonSet (one per node)

---

### 2. Core Component

**Location**: `core/`

**Responsibilities**:
- ✅ gRPC server (receives SBOM from Agents)
- ✅ SBOM storage (PostgreSQL)
- ✅ CVE matching (via workers)
- ✅ Policy evaluation (CEL-based)
- ✅ Risk scoring (V2 algorithm)
- ✅ REST API (Gin framework)
- ✅ Dashboard backend
- ✅ Admission webhook

**Key Files**:
- `core/cmd/main.go` - Entry point, worker pool initialization
- `core/internal/grpc/server.go` - gRPC server with mTLS
- `core/internal/grpc/handler_sbom.go` - SBOM ingestion
- `core/pkg/worker/` - NATS workers
- `core/pkg/cve/matcher/` - CVE matching logic
- `core/pkg/risk/scorer.go` - Risk scoring V2
- `core/pkg/policy/` - Policy engine
- `core/internal/api/` - REST API handlers

**Worker Pool**:
- SBOMWorker: Processes normalized pods → generates SBOM
- CVEMatcherWorker: Processes SBOM_CREATED → matches CVEs
- RiskWorker: Processes normalized resources → policy evaluation
- HistoricalRiskEvaluator: Scheduled risk score updates

**Ports**:
- `8080`: HTTP API
- `9090`: gRPC (Agent communication)

---

### 3. Database (PostgreSQL + Apache AGE)

**Location**: `core/migrations/`

**Key Tables**:

**SBOM Tables**:
- `sboms`: SBOM records
  - PodUID, PodName, Namespace
  - ContainerName, ImageDigest
  - AgentID, NodeID
- `sbom_components`: Package inventory
  - SBOMID (FK), ComponentType, ComponentName, ComponentVersion
  - PURL, Licenses, Source

**CVE Tables**:
- `cves`: CVE metadata
  - CVEID (PK), Description, Severity, CVSS
- `package_vulnerabilities`: Package-to-CVE mapping
  - Ecosystem, PackageName, VersionConstraint
  - CVEID (FK)
- `cve_matches`: Matched CVEs for SBOMs
  - SBOMID (FK), PodUID, ContainerName
  - CVEID, PackageName, PackageVersion
  - Severity, CVSS, FixedVersion

**Insight Tables**:
- `insights`: Security findings
  - ResourceType, ResourceUID, ResourceName, ResourceNamespace
  - InsightType, Title, Description, Severity
  - Status (active/acknowledged/resolved)
  - AffectedResources (JSONB)
  - Metadata (JSONB)

**Risk Tables**:
- `risk_scores`: Calculated risk scores
  - ResourceUID, ResourceType
  - TotalScore, BaseScore, ExploitabilityScore, BusinessImpactScore
  - TimeDecay, PriorityLevel (P0-P4)
  - ScorerVersion (v1/v2)

**Policy Tables**:
- `policy_templates`: Policy definitions
  - TemplateID, Version, CELExpression
  - DefaultAction (alert/deny/warn)
- `policy_instances`: Policy instances
  - TemplateID, TemplateVersion
  - Namespace, Enabled, Action (override)

**Graph Database (Apache AGE)**:
- Stores RBAC relationships
- Enables attack path analysis
- Cypher queries for graph traversal

---

### 4. NATS JetStream (Event Bus)

**Subjects**:
- `ksam.raw.>` - Raw K8s resources (from collector)
- `ksam.normalized.>` - Normalized resources
- `ksam.sbom.created` - SBOM created events
- `ksam.cve.matched` - CVE matched events (future)
- `ksam.policy.violated` - Policy violations (future)

**Workers**:
- Subscribe to subjects
- Process messages asynchronously
- Retry on failure (with DLQ)
- Backpressure handling

**Configuration**:
- Concurrency: 5 workers per type
- Retry: Exponential backoff
- DLQ: Dead letter queue for failed messages

---

### 5. Dashboard (Web UI)

**Location**: `dashboard/`

**Technology**: React + TypeScript + D3.js

**Features**:
- Risk score visualization
- Insight management
- Graph visualization (attack paths)
- Policy management
- CVE details

**API**: REST API (`/api/v1/*`)

---

## 🔐 Security Architecture

### mTLS Communication

**Agent ↔ Core**:
- Protocol: gRPC with mutual TLS
- Port: 9090
- Certificate hierarchy:
  ```
  Root CA (self-signed)
    └─> fortuna-core-server (CN: fortuna-core.fortuna.svc.cluster.local)
    └─> fortuna-agent-client (CN: fortuna-agent)
  ```

**Security Features**:
- TLS 1.3 minimum
- Client authentication required
- Server name verification
- Certificate rotation supported

### RBAC

**Agent**:
- ClusterRole: Read-only access to pods on local node
- Field selector: `spec.nodeName = <agent's node>`

**Core**:
- ClusterRole: Read-only access to K8s metadata
- ServiceAccount: `fortuna-core`

### Admission Webhook

**Location**: `core/internal/webhook/`

**Features**:
- Validates resources before creation/update
- Policy enforcement (deny/allow)
- CEL-based evaluation
- Metrics collection

---

## 📊 Data Flow Summary

### Complete Pipeline (Pod → Insight)

```
Pod Created
  │
  ├─> Agent detects (local node only)
  │   └─> Extract SBOM (docker socket)
  │       └─> Send to Core (mTLS gRPC)
  │
  ├─> Core stores SBOM (PostgreSQL)
  │   └─> Publish SBOM_CREATED (NATS)
  │
  ├─> CVEMatcherWorker processes
  │   └─> Match CVEs (database query)
  │       └─> Create CVEMatches
  │           └─> Create Insights (CRITICAL/HIGH)
  │
  ├─> RiskWorker processes
  │   └─> Policy evaluation
  │       └─> Create policy insights
  │
  ├─> Risk Scorer calculates
  │   └─> V2 formula
  │       └─> Store RiskScore
  │
  └─> Dashboard displays
      └─> API: GET /api/v1/insights
          └─> GET /api/v1/risk/scores/:uid
```

**Timing**:
- Pod creation → SBOM extraction: ~5-10s
- SBOM extraction → CVE matching: ~5-15s
- CVE matching → Insight creation: ~1-3s
- **Total**: ~15-30s end-to-end

---

## 🛠️ Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| **Language** | Go | 1.24+ |
| **Database** | PostgreSQL | 15+ |
| **Graph DB** | Apache AGE | Extension |
| **Message Bus** | NATS JetStream | Latest |
| **API Framework** | Gin | Latest |
| **gRPC** | google.golang.org/grpc | Latest |
| **ORM** | GORM | Latest |
| **Policy** | CEL (Common Expression Language) | Latest |
| **Frontend** | React + TypeScript | Latest |
| **Visualization** | D3.js | Latest |
| **Container** | Docker | Latest |
| **Orchestration** | Kubernetes | 1.28+ |

---

## 📈 Performance Metrics

### Current Performance
- **CVE Database**: 74,561 CVEs loaded and indexed
- **SBOM Generation**: ~2-5 seconds per image
- **CVE Matching**: <1 second per SBOM
- **Risk Scoring**: <100ms per resource
- **API Response**: <50ms (p99)

### Scalability
- Tested with 1000+ pods
- 10,000+ insights managed
- Horizontal scaling supported (Core can scale)

---

## 🔍 Key Code Locations

### Entry Points
- `agent/cmd/main.go` - Agent entry point
- `core/cmd/main.go` - Core entry point

### SBOM Processing
- `agent/internal/sbom/processor.go` - Agent SBOM processing
- `agent/pkg/sbom/extractor/` - Custom SBOM extractor
- `core/internal/grpc/handler_sbom.go` - Core SBOM ingestion
- `core/pkg/worker/sbom_worker.go` - SBOM worker (legacy?)

### CVE Matching
- `core/pkg/cve/matcher/matcher.go` - CVE matching logic
- `core/pkg/cve/db_manager.go` - Database queries
- `core/pkg/worker/cve_matcher_worker.go` - CVE worker

### Risk Scoring
- `core/pkg/risk/scorer.go` - V2 risk scoring
- `core/pkg/worker/risk_worker.go` - Risk worker
- `core/pkg/worker/historical_risk_evaluator.go` - Scheduled scoring

### Policy Engine
- `core/pkg/policy/evaluator.go` - CEL evaluation
- `core/pkg/policy/enforcement.go` - Policy enforcement
- `core/internal/webhook/` - Admission webhook

### API
- `core/internal/api/routes.go` - Route definitions
- `core/internal/api/handlers.go` - Request handlers
- `core/internal/api/risk/` - Risk API handlers
- `core/internal/api/policy/` - Policy API handlers

---

## ⚠️ Architecture Discrepancies

### Issue 1: Agent-Based vs Core-Only

**Problem**: Documentation mentions both architectures
- `docs/02-architecture/README.md` describes Agent-Based
- Some docs mention Core-Only (Agent disabled)

**Reality**: Code implements Agent-Based architecture
- Agent code exists and is functional
- Core expects Agent communication via gRPC

**Recommendation**: 
- Clarify architecture decision
- Update all documentation to reflect actual implementation
- If Core-Only is desired, document migration path

---

## 🎯 Next Phase Recommendations

### 1. Architecture Clarification
- [ ] Decide: Agent-Based or Core-Only?
- [ ] Update all documentation to match decision
- [ ] Remove conflicting documentation

### 2. Code Quality
- [ ] Resolve architecture discrepancies
- [ ] Improve error handling
- [ ] Add comprehensive logging
- [ ] Performance optimization

### 3. Testing
- [ ] E2E test coverage
- [ ] Unit test coverage
- [ ] Integration test coverage
- [ ] Performance benchmarks

### 4. Documentation
- [ ] Complete API documentation
- [ ] Architecture decision records (ADRs)
- [ ] Deployment guides
- [ ] Troubleshooting guides

### 5. Features
- [ ] Multi-cluster support
- [ ] Enhanced reporting
- [ ] CIS Kubernetes Benchmark integration
- [ ] Machine learning for risk prediction

---

## 📚 Key Documents Reference

### Architecture
- `docs/02-architecture/README.md` - Main architecture doc
- `docs/02-architecture/KSAM_ADR_FULL.md` - Architecture decisions

### Components
- `docs/03-components/sbom/README.md` - SBOM component
- `docs/03-components/cve-scanner/README.md` - CVE scanner
- `docs/03-components/policy-engine/` - Policy engine
- `docs/03-components/risk-engine/` - Risk engine

### Development
- `docs/04-development/` - Developer guides
- `core/README.md` - Core component docs
- `agent/README.md` - Agent component docs

---

## ✅ Analysis Complete

This analysis provides a comprehensive overview of:
- ✅ Architecture (current state and discrepancies)
- ✅ Complete logic flow (Pod → Insight)
- ✅ Component responsibilities
- ✅ Data flow and storage
- ✅ Technology stack
- ✅ Performance metrics
- ✅ Key code locations
- ✅ Next phase recommendations

**Ready for next phase planning!** 🚀

---

*Fortuna K8s Management Platform - Comprehensive Analysis v1.0*

