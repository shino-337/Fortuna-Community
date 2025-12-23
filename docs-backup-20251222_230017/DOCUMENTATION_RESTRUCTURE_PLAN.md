# Documentation Restructure Plan - Fortuna K8s Management Platform

**Date**: December 22, 2024  
**Current Files**: 129 markdown files  
**Goal**: Create professional, production-ready documentation structure

---

## 🎯 Objectives

1. **Remove Outdated Content**: Archive/delete old KSAM references
2. **Consolidate Duplicates**: Merge similar documents
3. **Update Architecture**: Reflect Core-Only architecture (Agent disabled)
4. **Professional Structure**: Industry-standard documentation layout
5. **Clear Navigation**: Easy to find what you need

---

## 📊 Current State Analysis

### Total Files: 129

**By Category**:
- Architecture: 13 files
- Components: 26 files (SBOM: 17, CVE: 6, Agent: 1, Others: 2)
- Development: 51 files
- Getting Started: 12 files
- Migration: 6 files
- Operations: 1 file
- Archive: 10 files (already archived)
- Root Level: 10 files

**Issues Identified**:
- ❌ Many duplicate/overlapping documents
- ❌ Outdated Agent-based architecture references
- ❌ Too many granular files (51 in development/)
- ❌ Inconsistent naming conventions
- ❌ No clear document hierarchy
- ❌ KSAM references instead of Fortuna

---

## 🗂️ Proposed New Structure

```
docs/
├── README.md ⭐ (Updated - Main index)
├── START_HERE.md ⭐ (Keep - 5-min intro)
├── ARCHITECTURE.md ⭐ (Keep - High-level overview)
│
├── 01-getting-started/
│   ├── README.md (Navigation index)
│   ├── QUICKSTART.md (10-minute tutorial)
│   ├── INSTALLATION.md (Full installation guide)
│   ├── MINIKUBE_SETUP.md (Local development)
│   ├── CONFIGURATION.md (All config options)
│   └── TROUBLESHOOTING.md (Common issues)
│
├── 02-architecture/
│   ├── README.md (Architecture overview)
│   ├── CORE_ARCHITECTURE.md (Core pod details)
│   ├── DATA_FLOW.md (Pipeline explanation)
│   ├── DATABASE_SCHEMA.md (PostgreSQL + AGE)
│   ├── EVENT_SYSTEM.md (NATS JetStream)
│   ├── SECURITY_ARCHITECTURE.md (mTLS, RBAC)
│   └── DECISIONS.md (ADRs - Architecture Decision Records)
│
├── 03-components/
│   ├── README.md (Component index)
│   ├── core/
│   │   ├── README.md (Core controller)
│   │   ├── API_REFERENCE.md (REST + gRPC)
│   │   ├── WORKERS.md (Worker pipeline)
│   │   └── KUBERNETES_CLIENT.md (K8s integration)
│   ├── sbom/
│   │   ├── README.md (Overview)
│   │   ├── ARCHITECTURE.md (Zero-dependency design)
│   │   ├── EXTRACTORS.md (Package parsers)
│   │   └── PERFORMANCE.md (Benchmarks)
│   ├── cve-scanner/
│   │   ├── README.md (Overview)
│   │   ├── DATABASE.md (CVE database design)
│   │   ├── MATCHING.md (Version comparison)
│   │   └── LOADING_GUIDE.md (Load CVE data)
│   ├── policy-engine/
│   │   ├── README.md (CEL-based policies)
│   │   ├── WRITING_POLICIES.md (How to write)
│   │   ├── ADMISSION_WEBHOOK.md (Integration)
│   │   └── EXAMPLES.md (Policy examples)
│   ├── risk-engine/
│   │   ├── README.md (Risk scoring)
│   │   ├── ALGORITHM.md (Scoring formula)
│   │   ├── INSIGHTS.md (Insight management)
│   │   └── RULES.md (Built-in rules)
│   └── graph-engine/
│       ├── README.md (Apache AGE)
│       ├── SCHEMA.md (Graph schema)
│       ├── QUERIES.md (Cypher examples)
│       └── ATTACK_PATHS.md (Path analysis)
│
├── 04-development/
│   ├── README.md (Developer guide)
│   ├── SETUP.md (Local development)
│   ├── CONTRIBUTING.md (How to contribute)
│   ├── TESTING.md (Test strategy)
│   ├── CVE_LOADING.md (Load CVE data)
│   ├── DATABASE_MIGRATIONS.md (Schema changes)
│   └── API_EXAMPLES.md (API usage examples)
│
├── 05-operations/
│   ├── README.md (Operations guide)
│   ├── DEPLOYMENT.md (Production deployment)
│   ├── MONITORING.md (Prometheus + Grafana)
│   ├── BACKUP_RESTORE.md (Data management)
│   ├── SCALING.md (Horizontal scaling)
│   ├── UPGRADES.md (Version upgrades)
│   ├── SECURITY.md (Security best practices)
│   └── TROUBLESHOOTING.md (Advanced troubleshooting)
│
├── 06-reference/
│   ├── README.md (Reference index)
│   ├── API_REFERENCE.md (Complete API docs)
│   ├── CLI_REFERENCE.md (CLI commands - future)
│   ├── CONFIGURATION_REFERENCE.md (All env vars)
│   ├── DATABASE_SCHEMA_REFERENCE.md (Complete schema)
│   └── GLOSSARY.md (Terms and definitions)
│
├── 07-guides/
│   ├── README.md (Guides index)
│   ├── CVE_SCANNING_GUIDE.md (Complete CVE workflow)
│   ├── SBOM_GENERATION_GUIDE.md (SBOM workflow)
│   ├── POLICY_ENFORCEMENT_GUIDE.md (Policy workflow)
│   ├── INSIGHTS_MANAGEMENT_GUIDE.md (Insight workflow)
│   └── ATTACK_PATH_ANALYSIS_GUIDE.md (Graph workflow)
│
├── 08-tutorials/
│   ├── README.md (Tutorials index)
│   ├── 01-FIRST_DEPLOYMENT.md (Deploy Fortuna)
│   ├── 02-SCAN_YOUR_FIRST_IMAGE.md (CVE scanning)
│   ├── 03-WRITE_YOUR_FIRST_POLICY.md (Policy creation)
│   ├── 04-ANALYZE_ATTACK_PATHS.md (Graph analysis)
│   └── 05-INTEGRATE_WITH_CI_CD.md (CI/CD integration)
│
└── 09-archive/ (Keep for historical reference)
    ├── README.md (Archive index)
    ├── KSAM_TO_FORTUNA_MIGRATION.md
    ├── AGENT_BASED_ARCHITECTURE.md (Old design)
    └── IMPLEMENTATION_HISTORY.md (Development history)
```

---

## 📋 File Classification

### ✅ KEEP & UPDATE (Core Documents)

**Root Level**:
- `README.md` ✅ Update with new structure
- `START_HERE.md` ✅ Update architecture references
- `ARCHITECTURE.md` ✅ Update to Core-Only architecture
- `ARCHITECTURE_ANALYSIS.md` ✅ Keep (explains Core vs Agent)
- `SECURITY.md` ✅ Keep & update

**Getting Started**:
- `getting-started/README.md` ✅ Update
- `getting-started/QUICKSTART.md` ✅ Update
- `getting-started/MINIKUBE_SETUP.md` ✅ Update
- `getting-started/CVE_MASTER_IMPLEMENTATION_GUIDE.md` ✅ Keep
- `getting-started/INSIGHTS_MANAGEMENT_GUIDE.md` ✅ Keep
- `getting-started/SECURITY_GUIDE.md` ✅ Keep

**Components**:
- `components/sbom/README.md` ✅ Update
- `components/sbom/CUSTOM_SBOM_EXECUTIVE_SUMMARY.md` ✅ Keep
- `components/cve-scanner/README.md` ✅ Update
- `components/cve-scanner/osv-database-design.md` ✅ Keep
- `components/agent/README.md` ✅ Update (mark as optional/disabled)

**Architecture**:
- `architecture/README.md` ✅ Already good, minor updates
- `architecture/KSAM_ADR_FULL.md` ✅ Rename to DECISIONS.md

**Development**:
- `development/CVE_LOADING_GUIDE.md` ✅ Keep
- `development/E2E_CVE_INSIGHTS_TEST.md` ✅ Keep
- `development/INSIGHT_MANAGER_OPTIMIZATION.md` ✅ Keep
- `development/testing/README.md` ✅ Keep

**Operations**:
- `operations/performance/BENCHMARKS.md` ✅ Keep

---

### 🗄️ ARCHIVE (Move to archive/)

**Already Archived**: Keep in `archive/` folder

**To Archive**:
- `AGENT_DEPLOYMENT_COMPLETE.md` → archive/agent/
- `AGENT_STATUS_FINAL.md` → archive/agent/
- `REFACTORING_PLAN_AGENT_BASED.md` → archive/agent/
- `CLEANUP_COMPLETE.md` → archive/cleanup/
- `DOCUMENT_ORGANIZATION_PLAN.md` → archive/organization/
- `FINAL_STRUCTURE_COMPLETE.md` → archive/organization/
- `ARCHITECTURE_DIAGRAMS_COMPARISON.md` → archive/organization/
- All `development/setup/*_IMPLEMENTATION_PLAN.md` → archive/implementation/
- `migration/*` → Keep some, archive others

**SBOM Development History** (Archive but keep reference):
- `components/sbom/CUSTOM_SBOM_ANALYSIS_*.md` → archive/sbom/
- `components/sbom/SBOM_BASED_SCANNING_PART*.md` → archive/sbom/
- `components/sbom/SBOM_FIX_*.md` → archive/sbom/

**CVE Development History** (Archive but keep reference):
- `development/CVE_BULK_LOADING_STRATEGY.md` → archive/cve/
- `development/CVE_DATA_OPTIMIZATION.md` → archive/cve/
- `development/CVE_OPTIMIZATION_*.md` → archive/cve/
- `development/BULK_LOADER_*.md` → archive/cve/

---

### ❌ DELETE (Obsolete/Duplicate)

**Duplicates**:
- `02-architecture/KSAM_ADR_FULL.md` (duplicate of architecture/KSAM_ADR_FULL.md)
- `06-development/*.md` (duplicate of development/)
- Multiple CVE optimization reports (consolidate into one)

**Obsolete**:
- `getting-started/DASHBOARD_ACCESS_GUIDE.md` (outdated)
- `getting-started/DATABASE_SETUP_RESULTS.md` (one-time result)
- `getting-started/MIGRATION_ERROR_WORKAROUND.md` (fixed)
- `getting-started/ADMISSION_METRICS_SETUP_GUIDE.md` (too specific)
- `getting-started/WEBHOOK_DEPLOYMENT_INSTRUCTIONS.md` (merge into deployment)

**Migration** (Move to archive, not main docs):
- `migration/*` - All migration docs are historical

---

## 🔄 Consolidation Plan

### Consolidate into Single Documents

**1. CVE Loading Guide** (Consolidate 10+ files):
- `development/CVE_LOADING_GUIDE.md` ✅ Main
- `development/CVE_LOADER_USAGE.md` → merge
- `development/CVE_LOADER_TESTING_GUIDE.md` → merge
- `development/CVE_DATABASE_STATUS.md` → merge
- `development/CVE_UPDATE_MECHANISM.md` → merge
- **Result**: `04-development/CVE_LOADING.md`

**2. SBOM Guide** (Consolidate 17 files):
- `components/sbom/README.md` ✅ Main overview
- `components/sbom/CUSTOM_SBOM_EXECUTIVE_SUMMARY.md` ✅ Architecture
- Archive rest to `archive/sbom/`
- **Result**: `03-components/sbom/README.md` + `ARCHITECTURE.md`

**3. Testing Guide** (Consolidate):
- `development/testing/README.md` ✅ Main
- `development/E2E_CVE_INSIGHTS_TEST.md` → examples section
- `development/testing/TEST_READINESS_REPORT.md` → archive
- **Result**: `04-development/TESTING.md`

**4. Implementation Plans** (Archive all):
- `development/setup/*` (22 files) → archive/implementation/
- Keep reference link in CONTRIBUTING.md

---

## 📝 Content Updates Required

### Global Search & Replace

**1. Project Name**:
- KSAM → Fortuna (except in historical context)
- "K8s Service Account Management" → "Fortuna K8s Management Platform"

**2. Architecture References**:
- "Agent collects..." → "Core collects..."
- "Agent-Core communication" → "Optional (disabled by default)"
- Add note: "⚠️ Agent is optional and currently disabled"

**3. Version References**:
- Update to v2.0 (MVP2)
- Remove MVP1 outdated references

**4. Component Status**:
- SBOM: ~75% complete
- CVE Scanning: ~75% complete
- Attack Paths: ~20% complete
- Agent: Optional (disabled)

### Document-Specific Updates

**README.md**:
```markdown
# Fortuna K8s Management Platform - Documentation

**Current State**: Core-Only Architecture (Agent Optional)
**Version**: v2.0 (MVP2) - 65% Complete
**CVEs Loaded**: 74,561
**Insights**: 17,766 active

...
```

**ARCHITECTURE.md**:
- Add "⚠️ Core-Only Architecture" section at top
- Update all diagrams
- Remove Agent from main flow
- Add section "When to Enable Agent"

**START_HERE.md**:
- Update architecture diagram (Core-only)
- Update quick start (no Agent steps)
- Update performance numbers (current stats)

---

## 🎯 New Documents to Create

### Essential Guides

1. **`03-components/core/README.md`** ⭐
   - Core controller overview
   - All responsibilities
   - Configuration options

2. **`02-architecture/CORE_ARCHITECTURE.md`** ⭐
   - Detailed Core internal architecture
   - Worker pipeline
   - Kubernetes client

3. **`02-architecture/DATA_FLOW.md`** ⭐
   - Complete data flow diagrams
   - Collection → SBOM → CVE → Insights

4. **`04-development/API_EXAMPLES.md`** ⭐
   - Curl examples for all endpoints
   - Authentication flow
   - Common queries

5. **`05-operations/DEPLOYMENT.md`** ⭐
   - Production deployment guide
   - Helm charts
   - Resource requirements

6. **`06-reference/GLOSSARY.md`** ⭐
   - SBOM, CVE, PURL, AGE, CEL, etc.
   - Clear definitions

7. **`07-guides/CVE_SCANNING_GUIDE.md`** ⭐
   - End-to-end workflow
   - Load CVEs → Scan images → Review insights

8. **`08-tutorials/01-FIRST_DEPLOYMENT.md`** ⭐
   - Step-by-step first deployment
   - Screenshots
   - Expected outputs

### Reference Documents

9. **`06-reference/CONFIGURATION_REFERENCE.md`**
   - All environment variables
   - Default values
   - Examples

10. **`06-reference/DATABASE_SCHEMA_REFERENCE.md`**
    - Complete schema documentation
    - Table relationships
    - Indexes

---

## 🚀 Implementation Steps

### Phase 1: Cleanup & Archive (Week 1)

1. ✅ Create `docs/09-archive/` subfolders
2. ✅ Move obsolete documents to archive
3. ✅ Delete duplicate files
4. ✅ Update archive/README.md with index

### Phase 2: Restructure (Week 1-2)

1. ✅ Create new folder structure
2. ✅ Move documents to new locations
3. ✅ Update file references/links
4. ✅ Rename files for consistency

### Phase 3: Update Content (Week 2-3)

1. ✅ Global search/replace (KSAM → Fortuna)
2. ✅ Update architecture references (Core-Only)
3. ✅ Update README.md
4. ✅ Update ARCHITECTURE.md
5. ✅ Update START_HERE.md
6. ✅ Update component READMEs

### Phase 4: Consolidate (Week 3)

1. ✅ Consolidate CVE docs
2. ✅ Consolidate SBOM docs
3. ✅ Consolidate testing docs
4. ✅ Create comprehensive guides

### Phase 5: Create New Docs (Week 4)

1. ✅ Write missing essential guides
2. ✅ Create tutorials
3. ✅ Create reference docs
4. ✅ Create glossary

### Phase 6: Final Review (Week 4)

1. ✅ Review all navigation links
2. ✅ Test all code examples
3. ✅ Check for broken links
4. ✅ Peer review
5. ✅ Final polish

---

## 📊 Success Metrics

**Before** (Current State):
- 129 files
- 51 files in development/
- Confusing navigation
- Outdated references
- Duplicate content

**After** (Target State):
- ~60 files (50% reduction)
- Clear hierarchy (9 sections)
- Updated architecture
- No duplicates
- Professional structure

---

## 🔗 Next Steps

1. **Review this plan** with team
2. **Get approval** for structure
3. **Start Phase 1** (Cleanup)
4. **Create migration script** (automated where possible)
5. **Update incrementally** (don't break existing links)

---

**Document Owner**: DevOps Team  
**Review Date**: December 22, 2024  
**Target Completion**: January 15, 2025

---

*This plan will transform Fortuna documentation from development notes into production-ready, professional documentation.*


