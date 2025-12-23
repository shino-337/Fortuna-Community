# Project Restructure Plan - KSAM → Fortuna K8s Management Platform

**Date**: 2024-12-20  
**Status**: 📋 Planning Phase

---

## Executive Summary

### Objectives
1. ✅ Rename project: KSAM → **Fortuna K8s Management Platform**
2. ✅ Restructure documentation for clarity and maintainability
3. ✅ Remove obsolete/redundant documentation
4. ✅ Sync code comments with reality
5. ✅ Update architecture documentation
6. ✅ Document current progress and roadmap

### Scope Expansion
- **Before**: Kubernetes Service Account Manager (focused on ServiceAccounts)
- **After**: Fortuna K8s Management Platform (comprehensive K8s security & compliance)

---

## Current State Analysis

### Documentation Structure (As-Is)

```
KSAM/docs/
├── 01-getting-started/          (5 files) - User guides
├── 02-architecture/             (4 files) - Architecture docs
├── 03-components/               (Multiple subdirs) - Component docs
│   ├── admission-webhook/
│   ├── cve-scanner/
│   ├── graph-engine/
│   ├── policy-engine/
│   ├── risk-engine/
│   └── sbom/
├── 04-deployment/               (Not exists - scattered)
├── 05-operations/               (Not exists - scattered)
├── 06-development/              (30+ files) - Dev docs & analysis
├── API_DOCS.md
├── ARCHITECTURE.md              (Main architecture doc)
├── CHANGELOG.md
├── CVE_OPTIMIZATION_STATUS.md
├── IMPLEMENTATION_COMPLETE.md
├── POLICY_FRAMEWORK.md
├── ROADMAP.md
├── SECURITY.md
├── START_HERE.md
├── TEST_READINESS_REPORT.md
└── (Many other scattered files)
```

**Issues**:
- ⚠️ Too many files in root `docs/`
- ⚠️ Inconsistent naming (KSAM vs component names)
- ⚠️ Mix of user docs, dev docs, and status reports
- ⚠️ `06-development/` has 30+ files (hard to navigate)
- ⚠️ Missing `04-deployment/` and `05-operations/`
- ⚠️ Duplicate/overlapping docs

---

## Proposed New Structure

### Documentation Structure (To-Be)

```
docs/
├── README.md                    (Project overview, quick links)
├── CHANGELOG.md                 (Release notes)
├── ROADMAP.md                   (Product roadmap)
│
├── 01-overview/
│   ├── README.md                (What is Fortuna?)
│   ├── ARCHITECTURE.md          (High-level architecture)
│   ├── FEATURES.md              (Feature list)
│   └── COMPARISON.md            (vs other solutions)
│
├── 02-getting-started/
│   ├── README.md                (Quick start guide)
│   ├── INSTALLATION.md          (Installation steps)
│   ├── FIRST_STEPS.md           (Tutorial)
│   └── TROUBLESHOOTING.md       (Common issues)
│
├── 03-user-guide/
│   ├── README.md                (User guide overview)
│   ├── DASHBOARD.md             (Using the dashboard)
│   ├── INSIGHTS.md              (Managing insights)
│   ├── POLICIES.md              (Policy management)
│   ├── CVE_SCANNING.md          (Vulnerability scanning)
│   └── RBAC.md                  (RBAC analysis)
│
├── 04-deployment/
│   ├── README.md                (Deployment overview)
│   ├── KUBERNETES.md            (K8s deployment)
│   ├── CONFIGURATION.md         (Config reference)
│   ├── SECRETS.md               (Secrets management)
│   └── UPGRADE.md               (Upgrade guide)
│
├── 05-operations/
│   ├── README.md                (Operations overview)
│   ├── MONITORING.md            (Monitoring & metrics)
│   ├── BACKUP.md                (Backup & restore)
│   ├── SCALING.md               (Scaling guide)
│   └── MAINTENANCE.md           (Maintenance tasks)
│
├── 06-components/
│   ├── README.md                (Component overview)
│   ├── agent/
│   │   └── README.md            (Agent documentation)
│   ├── core/
│   │   └── README.md            (Core service documentation)
│   ├── admission-webhook/
│   │   └── README.md            (Webhook documentation)
│   ├── policy-engine/
│   │   └── README.md            (Policy engine)
│   ├── risk-engine/
│   │   └── README.md            (Risk engine)
│   ├── cve-scanner/
│   │   └── README.md            (CVE scanner)
│   ├── sbom/
│   │   └── README.md            (SBOM generator)
│   └── graph-engine/
│       └── README.md            (Graph engine)
│
├── 07-api-reference/
│   ├── README.md                (API overview)
│   ├── AUTHENTICATION.md        (Auth endpoints)
│   ├── INSIGHTS.md              (Insights API)
│   ├── POLICIES.md              (Policies API)
│   └── WEBHOOKS.md              (Webhook API)
│
├── 08-development/
│   ├── README.md                (Dev guide overview)
│   ├── CONTRIBUTING.md          (How to contribute)
│   ├── ARCHITECTURE_DEEP_DIVE.md (Detailed architecture)
│   ├── CODING_STANDARDS.md      (Code standards)
│   ├── TESTING.md               (Testing guide)
│   └── performance/
│       ├── CVE_OPTIMIZATION.md
│       ├── INSIGHT_OPTIMIZATION.md
│       └── BULK_LOADER_ANALYSIS.md
│
├── 09-security/
│   ├── README.md                (Security overview)
│   ├── SECURITY_MODEL.md        (Security model)
│   ├── THREAT_MODEL.md          (Threat analysis)
│   └── AUDIT.md                 (Security audit)
│
└── archive/
    └── (Old/obsolete docs)
```

---

## Renaming Strategy

### 1. Project Name Changes

| Old | New |
|-----|-----|
| KSAM | Fortuna |
| Kubernetes Service Account Manager | Fortuna K8s Management Platform |
| ksam | fortuna |
| KSAM_ prefix | FORTUNA_ prefix |

### 2. Files to Rename

**Directories**:
- `KSAM/` → `fortuna/`
- `ksam-core` → `fortuna-core`
- `ksam-agent` → `fortuna-agent`

**Go Module Paths**:
- `github.com/ksam/core` → `github.com/fortuna/core`
- `github.com/ksam/agent` → `github.com/fortuna/agent`

**Kubernetes Resources**:
- `namespace: ksam` → `namespace: fortuna`
- `app: ksam-core` → `app: fortuna-core`
- `app: ksam-agent` → `app: fortuna-agent`

**Environment Variables**:
- `KSAM_*` → `FORTUNA_*`

### 3. Documentation Updates

**Files to Update**:
- All `*.md` files in `docs/`
- All `README.md` files
- `ARCHITECTURE.md`
- `API_DOCS.md`
- All component documentation

---

## Migration Plan

### Phase 1: Analysis & Planning (Current)
- ✅ Analyze current structure
- ✅ Identify obsolete docs
- ✅ Design new structure
- ⏳ Create migration checklist

### Phase 2: Documentation Restructure (1-2 days)
1. Create new directory structure
2. Move files to new locations
3. Update internal links
4. Remove obsolete files
5. Create archive for old docs

### Phase 3: Rename Project (1 day)
1. Update Go module paths
2. Update Kubernetes manifests
3. Update environment variables
4. Update documentation
5. Update code comments

### Phase 4: Verification (1 day)
1. Build & test all components
2. Verify documentation links
3. Test deployment
4. Update CI/CD

### Phase 5: Finalization (1 day)
1. Update README
2. Create migration guide
3. Update changelog
4. Tag release

---

## Files to Archive (Obsolete/Redundant)

### Obsolete Documentation
- `docs/IMPLEMENTATION_COMPLETE.md` (outdated status)
- `docs/TEST_READINESS_REPORT.md` (one-time report)
- `docs/CVE_OPTIMIZATION_STATUS.md` (superseded by performance docs)
- `docs/06-development/GO_VERSION_ISSUE.md` (resolved)
- `docs/06-development/GO_VERSION_VERIFICATION.md` (one-time check)
- `docs/06-development/E2E_OPTIMIZATION_VERIFICATION_REPORT.md` (one-time report)
- `docs/06-development/CVE_OPTIMIZATION_ANALYSIS_REPORT.md` (superseded)

### To Consolidate
- Multiple CVE optimization docs → Single performance guide
- Multiple insight optimization docs → Single performance guide
- Multiple E2E test docs → Single testing guide

---

## Content to Update

### 1. Architecture Documentation

**Current Issues**:
- Focuses on ServiceAccount management
- Missing CVE/SBOM details
- Outdated component diagrams

**Updates Needed**:
- Expand scope to full K8s management
- Add CVE/SBOM architecture
- Update component interactions
- Add performance characteristics

### 2. Getting Started Guide

**Current Issues**:
- Missing prerequisites
- Complex setup steps
- No quick start

**Updates Needed**:
- Add prerequisites section
- Simplify quick start
- Add video tutorials
- Add troubleshooting

### 3. API Documentation

**Current Issues**:
- Incomplete API reference
- Missing examples
- No authentication guide

**Updates Needed**:
- Complete API reference
- Add cURL examples
- Add authentication guide
- Add SDKs/libraries

### 4. Deployment Guide

**Current Issues**:
- Scattered across multiple docs
- Missing production checklist
- No high availability guide

**Updates Needed**:
- Consolidated deployment guide
- Production checklist
- HA/DR guide
- Monitoring setup

---

## Implementation Checklist

### Phase 1: Preparation
- [ ] Backup current documentation
- [ ] Create new directory structure
- [ ] Create file mapping spreadsheet
- [ ] Review all documentation for accuracy

### Phase 2: File Operations
- [ ] Create new directories
- [ ] Move files to new locations
- [ ] Update internal links
- [ ] Archive obsolete files
- [ ] Create redirects for old links

### Phase 3: Content Updates
- [ ] Update project name (KSAM → Fortuna)
- [ ] Update scope description
- [ ] Update architecture diagrams
- [ ] Update API documentation
- [ ] Update deployment guides

### Phase 4: Code Updates
- [ ] Update Go module paths
- [ ] Update package imports
- [ ] Update environment variables
- [ ] Update Kubernetes manifests
- [ ] Update Docker images

### Phase 5: Testing
- [ ] Build all components
- [ ] Run tests
- [ ] Verify deployment
- [ ] Check documentation links
- [ ] Verify API endpoints

### Phase 6: Release
- [ ] Update CHANGELOG
- [ ] Create migration guide
- [ ] Tag release
- [ ] Update website
- [ ] Announce changes

---

## Risk Assessment

### High Risk
- ❌ **Go module path changes**: Breaks imports
  - Mitigation: Use go mod edit, test thoroughly
  
- ❌ **Kubernetes namespace changes**: Breaks deployments
  - Mitigation: Create migration script, document process

### Medium Risk
- ⚠️ **Documentation link changes**: Broken links
  - Mitigation: Use link checker, create redirects
  
- ⚠️ **Environment variable changes**: Config issues
  - Mitigation: Support both old/new vars during transition

### Low Risk
- ✅ **File moves**: Easy to fix
- ✅ **Documentation updates**: Non-breaking

---

## Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Analysis & Planning | 1 day | None |
| Documentation Restructure | 2 days | Analysis complete |
| Rename Project | 1 day | Docs restructure |
| Code Updates | 2 days | Rename complete |
| Testing | 1 day | Code updates |
| Release | 1 day | Testing complete |
| **Total** | **8 days** | |

---

## Next Steps

### Immediate (Today)
1. ✅ Complete analysis
2. ✅ Create restructure plan
3. ⏳ Get stakeholder approval
4. ⏳ Create backup

### Tomorrow
1. Start Phase 2: Documentation Restructure
2. Create new directory structure
3. Begin file migration
4. Update internal links

### This Week
1. Complete documentation restructure
2. Start project rename
3. Update code
4. Begin testing

---

**Last Updated**: 2024-12-20  
**Status**: 📋 Planning Complete, Awaiting Approval

