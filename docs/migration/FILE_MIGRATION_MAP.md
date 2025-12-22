# File Migration Map - Documentation Restructure

**Date**: 2024-12-20  
**Purpose**: Map old file locations to new locations

---

## Files to Archive (Obsolete)

### Status Reports (One-time, now obsolete)
| File | Reason | Action |
|------|--------|--------|
| `TEST_READINESS_REPORT.md` | One-time report | → archive/ |
| `IMPLEMENTATION_COMPLETE.md` | Outdated status | → archive/ |
| `CVE_OPTIMIZATION_STATUS.md` | Superseded | → archive/ |
| `PROGRESS_AND_ISSUES_SUMMARY.md` | Outdated | → archive/ |
| `06-development/GO_VERSION_VERIFICATION.md` | Resolved | → archive/ |
| `06-development/E2E_OPTIMIZATION_VERIFICATION_REPORT.md` | One-time | → archive/ |
| `06-development/CVE_OPTIMIZATION_ANALYSIS_REPORT.md` | Superseded | → archive/ |

---

## Files to Keep (Active)

### Root Level Documentation
| Old Path | New Path | Notes |
|----------|----------|-------|
| `README.md` | `README.md` | Update content |
| `ARCHITECTURE.md` | `01-overview/ARCHITECTURE.md` | Move to overview |
| `SECURITY.md` | `09-security/README.md` | Move to security |
| `START_HERE.md` | `README.md` | Merge with main README |
| `CVE_OPTIMIZATION_SUMMARY.md` | `08-development/performance/CVE_OPTIMIZATION.md` | Consolidate |
| `EXPECTED_PERFORMANCE_BENCHMARKS.md` | `08-development/performance/BENCHMARKS.md` | Move |
| `DOCUMENTATION_CLEANUP_SUMMARY.md` | Delete | No longer needed |
| `FOLDER_STRUCTURE_PLAN.md` | Delete | Superseded by this plan |

### 01-getting-started/
| Old Path | New Path | Notes |
|----------|----------|-------|
| `01-getting-started/README.md` | `02-getting-started/README.md` | Keep |
| `01-getting-started/MINIKUBE_SETUP.md` | `02-getting-started/INSTALLATION.md` | Merge |
| `01-getting-started/DATABASE_SETUP_RESULTS.md` | archive/ | One-time |
| `01-getting-started/MIGRATION_ERROR_WORKAROUND.md` | `04-deployment/TROUBLESHOOTING.md` | Move |
| `01-getting-started/DASHBOARD_ACCESS_GUIDE.md` | `03-user-guide/DASHBOARD.md` | Move |
| `01-getting-started/INSIGHTS_MANAGEMENT_GUIDE.md` | `03-user-guide/INSIGHTS.md` | Move |
| `01-getting-started/SECURITY_GUIDE.md` | `09-security/README.md` | Merge |
| `01-getting-started/WEBHOOK_DEPLOYMENT_INSTRUCTIONS.md` | `04-deployment/WEBHOOK.md` | Move |
| `01-getting-started/SBOM_DEPLOYMENT_GUIDE.md` | `04-deployment/SBOM.md` | Move |
| `01-getting-started/CVE_MASTER_IMPLEMENTATION_GUIDE.md` | `04-deployment/CVE_SCANNER.md` | Move |
| `01-getting-started/ADMISSION_METRICS_SETUP_GUIDE.md` | `05-operations/MONITORING.md` | Merge |

### 02-architecture/
| Old Path | New Path | Notes |
|----------|----------|-------|
| `02-architecture/README.md` | `01-overview/ARCHITECTURE.md` | Merge with main arch |
| `02-architecture/KSAM_ADR_FULL.md` | `08-development/ADR.md` | Move to dev |
| `02-architecture/changelog/` | `CHANGELOG.md` | Consolidate |

### 03-components/
| Old Path | New Path | Notes |
|----------|----------|-------|
| `03-components/agent/README.md` | `06-components/agent/README.md` | Move |
| `03-components/core/README.md` | `06-components/core/README.md` | Move |
| `03-components/admission-webhook/` | `06-components/admission-webhook/` | Move |
| `03-components/policy-engine/README.md` | `06-components/policy-engine/README.md` | Move |
| `03-components/risk-engine/README.md` | `06-components/risk-engine/README.md` | Move |
| `03-components/cve-scanner/README.md` | `06-components/cve-scanner/README.md` | Move |
| `03-components/sbom/README.md` | `06-components/sbom/README.md` | Move |
| `03-components/graph-engine/README.md` | `06-components/graph-engine/README.md` | Move |
| `03-components/dashboard/README.md` | `06-components/dashboard/README.md` | Move |

### 06-development/
**Performance Docs** → `08-development/performance/`:
- `CVE_DATA_OPTIMIZATION.md` → `CVE_OPTIMIZATION.md`
- `CVE_BULK_LOADING_STRATEGY.md` → merge into above
- `CVE_LOADING_GUIDE.md` → merge into above
- `INSIGHT_MANAGER_OPTIMIZATION.md` → `INSIGHT_OPTIMIZATION.md`
- `BULK_LOADER_DETAILED_ANALYSIS.md` → merge into CVE doc
- `BULK_LOADER_ISSUES_FIXED.md` → archive/

**API Docs** → `07-api-reference/`:
- `INSIGHTS_API_CURL_EXAMPLES.md` → `INSIGHTS_API.md`
- `API_VERIFICATION_RESULTS.md` → archive/

**Testing Docs** → `08-development/testing/`:
- `E2E_CVE_INSIGHTS_TEST.md` → `E2E_TESTING.md`
- `E2E_TEST_ANALYSIS_20251217.md` → archive/
- `CVE_LOADER_TESTING_GUIDE.md` → merge into E2E doc

**Database Docs** → `08-development/database/`:
- `CVE_DATABASE_STATUS.md` → `CVE_DATABASE.md`
- `CVE_DATA_SOURCE_STRATEGY.md` → `CVE_DATA_SOURCE.md`
- `CVE_UPDATE_MECHANISM.md` → merge into above

**Setup Docs** → `04-deployment/`:
- `setup/IMPLEMENTATION_PLAN.md` → archive/ (historical)
- `setup/IMPLEMENTATION_ROADMAP.md` → archive/ (historical)
- `setup/MVP2_PHASE*` → archive/ (historical)
- `setup/MTLS_IMPLEMENTATION.md` → `04-deployment/MTLS.md`

**Migration Docs** → `04-deployment/MIGRATIONS.md`:
- `migrations/MIGRATION_021_EXECUTION_ISSUE_FIX.md` → consolidate
- `migrations/MIGRATION_021_FIX_SUMMARY.md` → consolidate
- `migrations/MIGRATION_022_CVE_COLUMNS_FIX.md` → consolidate

**Parser Analysis** → `08-development/internals/`:
- `OSV_PARSER_ANALYSIS.md` → `PARSERS.md`

**CVE Loader Docs** → `08-development/tools/`:
- `CVE_LOADER_USAGE.md` → `CVE_LOADER.md`

### 07-features/
| Old Path | New Path | Notes |
|----------|----------|-------|
| `07-features/*` | archive/ | UI specs (historical) |
| `07-features/NATS_SUBJECT_HIERARCHY.md` | `08-development/NATS.md` | Move to dev |
| `07-features/CIS_BENCHMARK_RULES.md` | `03-user-guide/CIS_BENCHMARKS.md` | Move |
| `07-features/COMPATIBILITY_*.md` | `10-reference/COMPATIBILITY.md` | Consolidate |

### references/
| Old Path | New Path | Notes |
|----------|----------|-------|
| `references/api/*` | archive/ | Historical analysis |
| `references/policy/*` | archive/ | Historical analysis |
| `references/mvp/*` | archive/ | Historical MVP docs |
| `references/Architecture_Review_and_Critical_Recommendations.md` | archive/ | Historical |

---

## New Directories to Create

```
docs/
├── 01-overview/
│   ├── README.md (new)
│   ├── ARCHITECTURE.md (from ARCHITECTURE.md + 02-architecture/README.md)
│   ├── FEATURES.md (new)
│   └── COMPARISON.md (new)
│
├── 02-getting-started/
│   ├── README.md (from 01-getting-started/README.md)
│   ├── INSTALLATION.md (from MINIKUBE_SETUP.md + new content)
│   ├── FIRST_STEPS.md (new)
│   └── TROUBLESHOOTING.md (from MIGRATION_ERROR_WORKAROUND.md + new)
│
├── 03-user-guide/
│   ├── README.md (new)
│   ├── DASHBOARD.md (from DASHBOARD_ACCESS_GUIDE.md)
│   ├── INSIGHTS.md (from INSIGHTS_MANAGEMENT_GUIDE.md)
│   ├── POLICIES.md (new)
│   ├── CVE_SCANNING.md (new)
│   ├── RBAC.md (new)
│   └── CIS_BENCHMARKS.md (from 07-features/CIS_BENCHMARK_RULES.md)
│
├── 04-deployment/
│   ├── README.md (new)
│   ├── KUBERNETES.md (new)
│   ├── CONFIGURATION.md (new)
│   ├── SECRETS.md (new)
│   ├── MTLS.md (from 06-development/setup/MTLS_IMPLEMENTATION.md)
│   ├── WEBHOOK.md (from WEBHOOK_DEPLOYMENT_INSTRUCTIONS.md)
│   ├── SBOM.md (from SBOM_DEPLOYMENT_GUIDE.md)
│   ├── CVE_SCANNER.md (from CVE_MASTER_IMPLEMENTATION_GUIDE.md)
│   ├── TROUBLESHOOTING.md (new)
│   ├── UPGRADE.md (new)
│   └── MIGRATIONS.md (consolidate from 06-development/migrations/)
│
├── 05-operations/
│   ├── README.md (new)
│   ├── MONITORING.md (from ADMISSION_METRICS_SETUP_GUIDE.md + new)
│   ├── BACKUP.md (new)
│   ├── SCALING.md (new)
│   └── MAINTENANCE.md (new)
│
├── 06-components/
│   ├── README.md (new)
│   ├── agent/ (from 03-components/agent/)
│   ├── core/ (from 03-components/core/)
│   ├── admission-webhook/ (from 03-components/admission-webhook/)
│   ├── policy-engine/ (from 03-components/policy-engine/)
│   ├── risk-engine/ (from 03-components/risk-engine/)
│   ├── cve-scanner/ (from 03-components/cve-scanner/)
│   ├── sbom/ (from 03-components/sbom/)
│   ├── graph-engine/ (from 03-components/graph-engine/)
│   └── dashboard/ (from 03-components/dashboard/)
│
├── 07-api-reference/
│   ├── README.md (new)
│   ├── AUTHENTICATION.md (new)
│   ├── INSIGHTS_API.md (from INSIGHTS_API_CURL_EXAMPLES.md)
│   ├── POLICIES_API.md (new)
│   ├── CVE_API.md (new)
│   └── WEBHOOKS_API.md (new)
│
├── 08-development/
│   ├── README.md (new)
│   ├── CONTRIBUTING.md (new)
│   ├── ARCHITECTURE_DEEP_DIVE.md (new)
│   ├── CODING_STANDARDS.md (new)
│   ├── ADR.md (from 02-architecture/KSAM_ADR_FULL.md)
│   ├── NATS.md (from 07-features/NATS_SUBJECT_HIERARCHY.md)
│   │
│   ├── performance/
│   │   ├── CVE_OPTIMIZATION.md (consolidate from multiple)
│   │   ├── INSIGHT_OPTIMIZATION.md (from INSIGHT_MANAGER_OPTIMIZATION.md)
│   │   └── BENCHMARKS.md (from EXPECTED_PERFORMANCE_BENCHMARKS.md)
│   │
│   ├── testing/
│   │   ├── E2E_TESTING.md (from E2E_CVE_INSIGHTS_TEST.md)
│   │   └── TESTING_GUIDE.md (new)
│   │
│   ├── database/
│   │   ├── CVE_DATABASE.md (from CVE_DATABASE_STATUS.md)
│   │   └── CVE_DATA_SOURCE.md (consolidate)
│   │
│   ├── tools/
│   │   └── CVE_LOADER.md (from CVE_LOADER_USAGE.md)
│   │
│   └── internals/
│       └── PARSERS.md (from OSV_PARSER_ANALYSIS.md)
│
├── 09-security/
│   ├── README.md (from SECURITY.md + SECURITY_GUIDE.md)
│   ├── SECURITY_MODEL.md (new)
│   ├── THREAT_MODEL.md (new)
│   └── AUDIT.md (new)
│
├── 10-reference/
│   ├── README.md (new)
│   ├── COMPATIBILITY.md (from 07-features/COMPATIBILITY_*.md)
│   └── GLOSSARY.md (new)
│
└── archive/
    └── (All obsolete docs)
```

---

## Migration Steps

### Phase 1: Create New Structure
```bash
# Create new directories
mkdir -p docs/01-overview
mkdir -p docs/02-getting-started
mkdir -p docs/03-user-guide
mkdir -p docs/04-deployment
mkdir -p docs/05-operations
mkdir -p docs/07-api-reference
mkdir -p docs/08-development/{performance,testing,database,tools,internals}
mkdir -p docs/09-security
mkdir -p docs/10-reference
mkdir -p docs/archive
```

### Phase 2: Move & Consolidate Files
```bash
# Move component docs
mv docs/03-components/ docs/06-components/

# Move to archive
mv docs/references/ docs/archive/
mv docs/07-features/ docs/archive/
# ... (see full list above)
```

### Phase 3: Update Links
- Run link checker
- Update internal references
- Create redirects

### Phase 4: Rename Project
- Update all "KSAM" → "Fortuna"
- Update "Service Account Manager" → "K8s Management Platform"

---

## Verification Checklist

- [ ] All active files moved to new locations
- [ ] Obsolete files archived
- [ ] Internal links updated
- [ ] No broken links
- [ ] All README.md files created
- [ ] Project renamed (KSAM → Fortuna)
- [ ] Build & test pass
- [ ] Documentation site works

---

**Last Updated**: 2024-12-20  
**Status**: Ready for execution

