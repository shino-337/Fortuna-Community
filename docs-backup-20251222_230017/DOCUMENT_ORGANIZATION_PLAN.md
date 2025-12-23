# Document Organization Plan - Final Cleanup

**Date**: December 22, 2024  
**Status**: Execution Phase

---

## 📊 Current State Analysis

### Root Level (`KSAM/`)
**Files to organize**:
- `README.md` ✅ Keep (main project README)
- `MIGRATION_COMPLETE.md` → Move to `docs/migration/`
- `test_*.sh` → Move to `tests/e2e/`
- `*.sh` scripts → Move to `scripts/`

### Docs Root (`KSAM/docs/`)
**Files to organize**:
```
ARCHITECTURE.md                   → archive/ (superseded by architecture/README.md)
CVE_OPTIMIZATION_STATUS.md       → development/cve/
CVE_OPTIMIZATION_SUMMARY.md      → development/cve/
DOCUMENTATION_CLEANUP_SUMMARY.md → archive/
EXPECTED_PERFORMANCE_BENCHMARKS.md → operations/performance/
FILE_MIGRATION_MAP.md            → migration/
FOLDER_STRUCTURE_PLAN.md         → archive/
IMPLEMENTATION_COMPLETE.md       → archive/
PROGRESS_AND_ISSUES_SUMMARY.md   → archive/
PROJECT_RESTRUCTURE_PLAN.md      → archive/
README.md                         ✅ Keep (docs index)
RESTRUCTURE_SUMMARY.md           → archive/
SECURITY.md                       ✅ Keep (important reference)
START_HERE.md                     ✅ Keep (entry point)
TEST_READINESS_REPORT.md         → development/testing/
```

---

## 🎯 Organization Plan

### Phase 1: Move Root-Level Files

**Scripts** → `scripts/`:
```
test_e2e_cve_insights_v2.sh          → tests/e2e/
test_e2e_sbom_cache_hit.sh           → tests/e2e/
test_e2e_cve_boundary_versions.sh    → tests/e2e/
test_e2e_multi_container_sbom_cve.sh → tests/e2e/
verify_pipeline_logic.sh             → tests/e2e/
```

**Migration docs** → `docs/migration/`:
```
MIGRATION_COMPLETE.md
```

### Phase 2: Organize Docs Root

**Development docs**:
```
CVE_OPTIMIZATION_STATUS.md       → docs/development/cve-optimization/
CVE_OPTIMIZATION_SUMMARY.md      → docs/development/cve-optimization/
TEST_READINESS_REPORT.md         → docs/development/testing/
```

**Operations docs**:
```
EXPECTED_PERFORMANCE_BENCHMARKS.md → docs/operations/performance/
```

**Migration docs**:
```
FILE_MIGRATION_MAP.md → docs/migration/
```

**Archive** (historical/completed):
```
ARCHITECTURE.md                   → docs/archive/
DOCUMENTATION_CLEANUP_SUMMARY.md  → docs/archive/
FOLDER_STRUCTURE_PLAN.md          → docs/archive/
IMPLEMENTATION_COMPLETE.md        → docs/archive/
PROGRESS_AND_ISSUES_SUMMARY.md    → docs/archive/
PROJECT_RESTRUCTURE_PLAN.md       → docs/archive/
RESTRUCTURE_SUMMARY.md            → docs/archive/
analyze_pipeline_logic.md         → docs/archive/
pipeline_logic_verification_report.md → docs/archive/
```

### Phase 3: Create Missing Directories

**New directories needed**:
```
tests/
  ├── e2e/              ← End-to-end tests
  ├── integration/      ← Integration tests
  └── unit/             ← Unit tests (reference)

docs/
  ├── development/
  │   ├── cve-optimization/
  │   └── testing/
  └── operations/
      └── performance/
```

---

## 📋 Final Directory Structure

```
KSAM/
├── README.md                  ← Main project README
├── MIGRATION_COMPLETE.md      ← TO MOVE
│
├── scripts/                   ← Utility scripts
│   ├── build.sh
│   ├── deploy.sh
│   ├── monitor_fortuna.sh
│   └── cleanup_insights.sql
│
├── tests/                     ← All test files
│   ├── e2e/                   ← E2E tests
│   │   ├── test_e2e_cve_insights_v2.sh
│   │   ├── test_e2e_sbom_cache_hit.sh
│   │   ├── test_e2e_cve_boundary_versions.sh
│   │   ├── test_e2e_multi_container_sbom_cve.sh
│   │   └── verify_pipeline_logic.sh
│   ├── integration/
│   └── unit/
│
├── docs/
│   ├── README.md              ← Docs index ✅
│   ├── START_HERE.md          ← Quick start ✅
│   ├── SECURITY.md            ← Security policy ✅
│   │
│   ├── getting-started/       ← Installation & setup
│   │   ├── README.md
│   │   ├── QUICKSTART.md
│   │   ├── CONFIGURATION.md
│   │   └── TROUBLESHOOTING.md
│   │
│   ├── architecture/          ← System design
│   │   ├── README.md
│   │   ├── INDEX.md
│   │   ├── DATA_FLOWS.md
│   │   └── DATABASE_SCHEMA.md
│   │
│   ├── components/            ← Component docs
│   │   ├── INDEX.md
│   │   ├── agent/
│   │   ├── core/
│   │   ├── sbom/
│   │   ├── cve-scanner/
│   │   ├── policy-engine/
│   │   └── risk-engine/
│   │
│   ├── development/           ← Developer docs
│   │   ├── README.md
│   │   ├── API_REFERENCE.md
│   │   ├── CONTRIBUTING.md
│   │   ├── cve-optimization/
│   │   │   ├── CVE_OPTIMIZATION_STATUS.md
│   │   │   └── CVE_OPTIMIZATION_SUMMARY.md
│   │   └── testing/
│   │       ├── README.md
│   │       └── TEST_READINESS_REPORT.md
│   │
│   ├── operations/            ← Operations docs
│   │   ├── README.md
│   │   ├── MONITORING.md
│   │   ├── SCALING.md
│   │   ├── BACKUP_RESTORE.md
│   │   └── performance/
│   │       └── BENCHMARKS.md
│   │
│   ├── migration/             ← Migration docs
│   │   ├── FROM_KSAM.md
│   │   ├── FILE_MIGRATION_MAP.md
│   │   ├── MIGRATION_COMPLETE.md
│   │   ├── MIGRATION_EXECUTION_REPORT.md
│   │   └── DATABASE_DEEP_ANALYSIS_REPORT.md
│   │
│   └── archive/               ← Historical docs
│       ├── old-structure/
│       ├── ARCHITECTURE.md
│       ├── DOCUMENTATION_CLEANUP_SUMMARY.md
│       ├── PROJECT_RESTRUCTURE_PLAN.md
│       └── ...
```

---

## ✅ Execution Checklist

### Phase 1: Create Directories
- [ ] Create `tests/e2e/`
- [ ] Create `tests/integration/`
- [ ] Create `tests/unit/`
- [ ] Create `docs/development/cve-optimization/`
- [ ] Create `docs/development/testing/`
- [ ] Create `docs/operations/performance/`

### Phase 2: Move Root Files
- [ ] Move `MIGRATION_COMPLETE.md` → `docs/migration/`
- [ ] Move `test_e2e_*.sh` → `tests/e2e/`
- [ ] Move `verify_pipeline_logic.sh` → `tests/e2e/`

### Phase 3: Move Docs Root Files
- [ ] Move `CVE_OPTIMIZATION_*.md` → `docs/development/cve-optimization/`
- [ ] Move `TEST_READINESS_REPORT.md` → `docs/development/testing/`
- [ ] Move `EXPECTED_PERFORMANCE_BENCHMARKS.md` → `docs/operations/performance/BENCHMARKS.md`
- [ ] Move `FILE_MIGRATION_MAP.md` → `docs/migration/`

### Phase 4: Archive Old Files
- [ ] Move `ARCHITECTURE.md` → `docs/archive/`
- [ ] Move `DOCUMENTATION_CLEANUP_SUMMARY.md` → `docs/archive/`
- [ ] Move `FOLDER_STRUCTURE_PLAN.md` → `docs/archive/`
- [ ] Move `IMPLEMENTATION_COMPLETE.md` → `docs/archive/`
- [ ] Move `PROGRESS_AND_ISSUES_SUMMARY.md` → `docs/archive/`
- [ ] Move `PROJECT_RESTRUCTURE_PLAN.md` → `docs/archive/`
- [ ] Move `RESTRUCTURE_SUMMARY.md` → `docs/archive/`
- [ ] Move `analyze_pipeline_logic.md` → `docs/archive/`
- [ ] Move `pipeline_logic_verification_report.md` → `docs/archive/`

### Phase 5: Create Index Files
- [ ] Create `tests/README.md`
- [ ] Create `docs/development/cve-optimization/README.md`
- [ ] Create `docs/development/testing/README.md`
- [ ] Create `docs/operations/performance/README.md`
- [ ] Update `docs/README.md` with final structure

### Phase 6: Verify & Cleanup
- [ ] Verify all files in correct locations
- [ ] Update all internal links
- [ ] Remove empty directories
- [ ] Create final structure diagram
- [ ] Update main README.md

---

## 🎯 Success Criteria

✅ **No loose files**: All `.md` and `.sh` files in appropriate directories  
✅ **Clear hierarchy**: Easy to find any document  
✅ **Proper indexing**: Each directory has README/INDEX  
✅ **Working links**: All internal links functional  
✅ **Clean root**: Only essential files in root  

---

## 📝 Notes

### Files to Keep in Root
- `README.md` - Project main page
- `LICENSE` - License file
- `go.mod`, `go.sum` - Go dependencies
- `Dockerfile` - Container build
- `.gitignore` - Git config

### Files to Keep in Docs Root
- `README.md` - Documentation index
- `START_HERE.md` - Quick entry point
- `SECURITY.md` - Security policy (GitHub special file)

---

**Last Updated**: December 22, 2024  
**Next Action**: Execute Phase 1

