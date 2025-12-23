# Architecture Analysis - Core vs Agent

**Date**: December 22, 2024  
**Current State**: Core-Only Architecture (Agent Disabled)

---

## 🎯 Key Question

**"Việc monitor và thu thập insight đang thực hiện ở Core hay ở Agent?"**

### Answer: **100% Ở CORE**

Agent **KHÔNG** tham gia vào việc monitor hay thu thập insight.

---

## 📊 Current Architecture (As-Implemented)

```
┌─────────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                         │
│                                                             │
│  ┌──────────────┐                                          │
│  │  Kubernetes  │                                          │
│  │  API Server  │                                          │
│  └───────┬──────┘                                          │
│          │                                                  │
│          │ K8s API (list/watch)                           │
│          │ (in-cluster ServiceAccount)                    │
│          ▼                                                  │
│  ┌──────────────────────────────────────────────────┐     │
│  │          FORTUNA CORE (Single Pod)               │     │
│  │                                                  │     │
│  │  ┌────────────────────────────────────────┐    │     │
│  │  │         HTTP/gRPC Server               │    │     │
│  │  │  • REST API (port 8080)                │    │     │
│  │  │  • gRPC (port 9090 - for future)      │    │     │
│  │  │  • Admission Webhook                   │    │     │
│  │  └────────────────────────────────────────┘    │     │
│  │                                                  │     │
│  │  ┌────────────────────────────────────────┐    │     │
│  │  │      Kubernetes API Client             │    │     │
│  │  │  • List/Watch ServiceAccounts         │    │     │
│  │  │  • List/Watch Pods                    │    │     │
│  │  │  • List/Watch RBAC                    │    │     │
│  │  │  • ClusterRole: read all resources    │    │     │
│  │  └────────────────────────────────────────┘    │     │
│  │                                                  │     │
│  │  ┌────────────────────────────────────────┐    │     │
│  │  │          Worker Pool (NATS)            │    │     │
│  │  │                                         │    │     │
│  │  │  • Normalizer Worker                   │    │     │
│  │  │    └─ Clean & enrich data             │    │     │
│  │  │                                         │    │     │
│  │  │  • Correlator Worker                   │    │     │
│  │  │    └─ Build graph relationships        │    │     │
│  │  │                                         │    │     │
│  │  │  • SBOM Worker                         │    │     │
│  │  │    └─ Extract packages from images    │    │     │
│  │  │                                         │    │     │
│  │  │  • CVE Matcher Worker                  │    │     │
│  │  │    └─ Match CVEs against packages     │    │     │
│  │  │                                         │    │     │
│  │  │  • Risk Worker                         │    │     │
│  │  │    └─ Calculate risk scores           │    │     │
│  │  │    └─ CREATE INSIGHTS ✅              │    │     │
│  │  │                                         │    │     │
│  │  └────────────────────────────────────────┘    │     │
│  │                                                  │     │
│  │  ┌────────────────────────────────────────┐    │     │
│  │  │      Insight Manager                    │    │     │
│  │  │  • Create/Update Insights              │    │     │
│  │  │  • Deduplication logic                 │    │     │
│  │  │  • Persistence (insights table)        │    │     │
│  │  └────────────────────────────────────────┘    │     │
│  └──────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ NATS + PostgreSQL
                          ▼
         ┌────────────────────────────────────┐
         │      PostgreSQL + NATS             │
         │                                    │
         │  • cves (74,561 records)          │
         │  • insights (17,766 active) ✅    │
         │  • sboms, cve_matches, etc.       │
         └────────────────────────────────────┘
```

---

## 🔍 Detailed Analysis

### 1. Resource Collection (Monitor)

**WHO**: **Core Pod**  
**HOW**: Direct Kubernetes API access via in-cluster ServiceAccount

```
Core → K8s API Server
  ├─ List/Watch ServiceAccounts
  ├─ List/Watch Pods  
  ├─ List/Watch Roles/RoleBindings
  └─ List/Watch ClusterRoles/ClusterRoleBindings
```

**Code Location**: `KSAM/core/pkg/ingest/`

**Permissions**: ClusterRole `ksam-core-cluster-reader`
```yaml
rules:
  - apiGroups: [""]
    resources: ["serviceaccounts", "pods", "namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
    verbs: ["get", "list", "watch"]
```

**Agent Role**: ❌ **NONE** (Agent không tham gia thu thập)

---

### 2. SBOM Generation

**WHO**: **Core Pod - SBOM Worker**  
**HOW**: Extract packages from container images

```
Pod Event (NATS: ksam.normalized.pods)
  ↓
SBOM Worker (in Core)
  ├─ Resolve image digest
  ├─ Check SBOM cache (sboms table)
  ├─ If missing:
  │   ├─ Pull image (local docker or remote registry)
  │   ├─ Extract layers
  │   ├─ Parse package managers (dpkg, apk, rpm, npm, etc.)
  │   └─ Persist SBOM (sboms + sbom_components tables)
  └─ Publish SBOM_CREATED event
```

**Code Location**: `KSAM/core/pkg/sbom/`

**Agent Role**: ❌ **NONE**

---

### 3. CVE Matching

**WHO**: **Core Pod - CVE Matcher Worker**  
**HOW**: Match SBOM components against CVE database

```
SBOM_CREATED Event (NATS: ksam.sbom.created)
  ↓
CVE Matcher Worker (in Core)
  ├─ Load SBOM components
  ├─ Query CVE database (74,561 CVEs in PostgreSQL)
  ├─ Match packages against package_vulnerabilities
  ├─ Filter by severity (HIGH, CRITICAL)
  ├─ Persist matches (cve_matches table)
  └─ Trigger Insight creation
```

**Code Location**: `KSAM/core/pkg/worker/cve_matcher_worker.go`

**Agent Role**: ❌ **NONE**

---

### 4. Insight Creation (✅ THIS IS THE KEY)

**WHO**: **Core Pod - Risk Worker + Insight Manager**  
**HOW**: Create insights from vulnerabilities, policy violations, and risk analysis

```
CVE Matches / Policy Violations / Risk Analysis
  ↓
Risk Worker (in Core)
  ├─ Load policy rules
  ├─ Evaluate violations
  ├─ Calculate risk scores
  └─ Call InsightManager.CreateOrUpdateInsight()
      ↓
Insight Manager (in Core)
  ├─ Deduplication check (sbom_id + cve_id + pod_uid)
  ├─ Transaction-based insert/update
  ├─ Persist to insights table ✅
  └─ Update risk scores
```

**Code Location**: 
- `KSAM/core/pkg/worker/risk_worker.go`
- `KSAM/core/pkg/riskengine/insight_manager.go`

**Database Table**: `insights` (17,766 active records)

**Agent Role**: ❌ **NONE**

---

### 5. Risk Scoring

**WHO**: **Core Pod - Risk Engine**  
**HOW**: Calculate risk scores based on insights, RBAC, and context

```
Resource with Insights
  ↓
Risk Scorer (in Core)
  ├─ Load all active insights for resource
  ├─ Calculate base score (CVE CVSS, policy severity)
  ├─ Apply context multipliers (namespace, resource type)
  ├─ Apply exposure factors (network, privileges)
  └─ Persist risk_scores table
```

**Code Location**: `KSAM/core/pkg/risk/scorer.go`

**Agent Role**: ❌ **NONE**

---

## 🤔 Where Does Agent Fit? (If Enabled)

### Agent's Original Design (OPTIONAL, Currently Disabled)

```
┌─────────────────────────────────────────────┐
│  Agent Pod (per node) - IF ENABLED          │
│                                             │
│  • Collect local node resources            │
│  • Filter Pods by node name                │
│  • Send to Core via gRPC/mTLS              │
│                                             │
│  Role: DATA COLLECTOR ONLY                 │
│  NOT involved in:                           │
│    ❌ SBOM generation                       │
│    ❌ CVE matching                          │
│    ❌ Insight creation                      │
│    ❌ Risk scoring                          │
└─────────────────────────────────────────────┘
```

**Agent would ONLY**:
- Collect resources from K8s API (same as Core does now)
- Send data to Core via gRPC
- Distribute collection load across nodes

**Agent would NOT**:
- Create insights
- Match CVEs
- Calculate risk
- Generate SBOMs

**In current architecture**: Core does collection directly, so Agent is redundant.

---

## 📊 Data Flow: Monitor → Insight

### Complete Flow (All in Core)

```
1. MONITOR (Collection)
   Kubernetes API → Core K8s Client
   ↓
   Store: serviceaccounts, pods, roles tables

2. NORMALIZE
   Core Normalizer Worker
   ↓
   Publish: NATS (ksam.normalized.*)

3. SBOM GENERATION
   Core SBOM Worker
   ↓
   Store: sboms, sbom_components tables
   Publish: NATS (ksam.sbom.created)

4. CVE MATCHING
   Core CVE Matcher Worker
   ↓
   Query: cves, package_vulnerabilities tables
   Store: cve_matches table

5. INSIGHT CREATION ✅
   Core Risk Worker → Insight Manager
   ↓
   Store: insights table (17,766 records)

6. RISK SCORING
   Core Risk Engine
   ↓
   Store: risk_scores table

7. API EXPOSURE
   Core REST API (port 8080)
   ↓
   Dashboard / CLI / External tools
```

**Every step happens IN CORE POD.**

---

## ✅ Current System Capabilities

### What Core Does (Without Agent)

| Capability | Status | Location |
|------------|--------|----------|
| **Resource Collection** | ✅ Active | Core K8s Client |
| **SBOM Generation** | ✅ Active | Core SBOM Worker |
| **CVE Matching** | ✅ Active | Core CVE Matcher |
| **Insight Creation** | ✅ Active | Core Risk Worker |
| **Risk Scoring** | ✅ Active | Core Risk Engine |
| **Policy Evaluation** | ✅ Active | Core Policy Engine |
| **Graph Analysis** | ✅ Active | Core Correlator |
| **API Serving** | ✅ Active | Core HTTP Server |

**Total**: 8/8 capabilities in Core (100%)

---

## 🎯 Architecture Decision

### Current: Core-Only Architecture ✅

**Rationale**:
1. **Simplicity**: Single pod vs N pods (agent per node)
2. **Efficiency**: No gRPC overhead
3. **Maintainability**: One codebase to debug
4. **Sufficient Scale**: Works for <100 node clusters
5. **Feature Complete**: All capabilities present

### When to Add Agent

**Only if**:
- Multi-cluster federation needed
- Very large scale (1000+ nodes)
- Want to distribute collection load
- Need node-level file system access

**Not needed for**:
- Single cluster deployments ✅ (current)
- Resource collection ✅ (Core can do it)
- Insight generation ✅ (always in Core)
- Small-medium clusters ✅ (current)

---

## 📚 Summary

### Question: "Việc monitor và thu thập insight đang thực hiện ở Core hay ở Agent?"

### Answer:

**Monitor (Thu thập resources)**: ✅ **100% Ở CORE**
- Core sử dụng Kubernetes API client
- Core có ClusterRole permissions
- Agent **KHÔNG** tham gia

**Insight Creation**: ✅ **100% Ở CORE**
- Risk Worker tạo insights
- Insight Manager persist vào database
- Agent **KHÔNG** tham gia (và không thể tham gia vì không có logic này)

**Agent Role**: ❌ **DISABLED** (và không cần thiết)
- Agent chỉ có thể thu thập resources (nếu được enable)
- Agent **KHÔNG BAO GIỜ** tạo insights
- Agent **KHÔNG BAO GIỜ** match CVEs
- Agent **KHÔNG BAO GIỜ** tính risk scores

### Current System: **Core-Only Architecture**

```
                  ┌─────────────┐
                  │    CORE     │
                  │             │
                  │  ✅ Monitor │
                  │  ✅ SBOM    │
                  │  ✅ CVE     │
                  │  ✅ Insight │ ← ALL HERE
                  │  ✅ Risk    │
                  │  ✅ API     │
                  └─────────────┘
```

**Agent**: Not deployed, not needed, all work done by Core.

---

*Analysis completed: December 22, 2024*  
*Fortuna K8s Management Platform v2.0*  
*Architecture: Core-Only (Agent Optional)*

