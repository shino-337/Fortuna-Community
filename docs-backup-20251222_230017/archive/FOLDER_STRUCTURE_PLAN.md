# KSAM Documentation Folder Structure Plan

**Date**: December 16, 2025
**Purpose**: Reorganize 524+ documentation files into logical structure
**Status**: 🎯 Implementation Plan

---

## Proposed Structure

```
docs/
├── README.md                          # Main navigation and overview
├── ARCHITECTURE.md                    # Current architecture (root level)
├── SECURITY.md                        # Security policies (root level)
│
├── 01-getting-started/                # NEW: Quick start guides
│   ├── README.md
│   ├── installation-guide.md
│   ├── quick-start.md
│   └── prerequisites.md
│
├── 02-architecture/                   # Architecture documentation
│   ├── README.md
│   ├── system-overview.md
│   ├── data-flow.md
│   ├── component-details.md
│   ├── technology-stack.md
│   └── changelog/
│       ├── v2-changelog.md
│       └── update-plan.md
│
├── 03-components/                     # Component-specific docs
│   ├── agent/
│   │   ├── README.md
│   │   ├── configuration.md
│   │   └── troubleshooting.md
│   ├── core/
│   │   ├── README.md
│   │   ├── apis.md
│   │   └── workers.md
│   ├── policy-engine/
│   │   ├── README.md
│   │   ├── cel-syntax.md
│   │   └── yaml-rules.md
│   ├── risk-engine/
│   │   ├── README.md
│   │   ├── scoring.md
│   │   └── insights.md
│   ├── graph-engine/
│   │   ├── README.md
│   │   ├── apache-age.md
│   │   └── queries.md
│   ├── cve-scanner/                   # NEW: CVE documentation
│   │   ├── README.md
│   │   ├── osv-database-design.md
│   │   ├── trivy-comparison.md
│   │   ├── quick-start.md
│   │   └── matcher-engine.md
│   ├── sbom/                          # Existing SBOM docs (move here)
│   │   ├── README.md
│   │   ├── custom-sbom-implementation.md
│   │   ├── zero-dependency-design.md
│   │   └── local-first-extraction.md
│   └── dashboard/
│       ├── README.md
│       ├── ui-specifications.md
│       └── api-integration.md
│
├── 04-deployment/                     # Deployment guides
│   ├── README.md
│   ├── kubernetes/
│   │   ├── helm-charts.md
│   │   ├── manifests.md
│   │   └── scaling.md
│   ├── configuration/
│   │   ├── environment-variables.md
│   │   ├── secrets-management.md
│   │   └── tls-certificates.md
│   └── troubleshooting/
│       ├── common-issues.md
│       ├── debugging-guide.md
│       └── log-analysis.md
│
├── 05-operations/                     # Operational guides
│   ├── README.md
│   ├── monitoring/
│   │   ├── prometheus-metrics.md
│   │   ├── grafana-dashboards.md
│   │   └── alerting.md
│   ├── maintenance/
│   │   ├── database-backups.md
│   │   ├── upgrades.md
│   │   └── disaster-recovery.md
│   └── performance/
│       ├── optimization.md
│       ├── benchmarks.md
│       └── capacity-planning.md
│
├── 06-development/                    # Developer documentation
│   ├── README.md
│   ├── setup/
│   │   ├── local-development.md
│   │   ├── minikube-setup.md
│   │   └── testing-environment.md
│   ├── contributing/
│   │   ├── code-style.md
│   │   ├── pull-request-process.md
│   │   └── review-guidelines.md
│   ├── testing/
│   │   ├── unit-tests.md
│   │   ├── integration-tests.md
│   │   └── e2e-tests.md
│   └── migrations/
│       ├── database-migrations.md
│       ├── migration-021-fix.md
│       └── migration-022-cve.md
│
├── 07-features/                       # Feature documentation
│   ├── README.md
│   ├── policy-enforcement.md
│   ├── risk-detection.md
│   ├── cve-scanning.md
│   ├── sbom-generation.md
│   ├── attack-paths.md
│   └── compliance.md
│
├── 08-api-reference/                  # API documentation
│   ├── README.md
│   ├── rest-api.md
│   ├── grpc-api.md
│   └── webhooks.md
│
├── 09-use-cases/                      # Use case examples
│   ├── README.md
│   ├── vulnerability-detection.md
│   ├── rbac-analysis.md
│   ├── compliance-auditing.md
│   └── incident-response.md
│
├── 10-reference/                      # Reference materials
│   ├── README.md
│   ├── cis-benchmarks.md
│   ├── yaml-rules-catalog.md
│   ├── cel-functions.md
│   └── glossary.md
│
├── archive/                           # Historical documents
│   ├── README.md
│   ├── mvp1/                          # MVP1 artifacts
│   │   ├── test-reports/
│   │   ├── implementation-logs/
│   │   └── release-notes/
│   ├── mvp2/                          # MVP2 artifacts
│   │   ├── test-reports/
│   │   ├── implementation-logs/
│   │   └── phase-documents/
│   ├── deprecated/                    # Deprecated docs
│   │   ├── old-architecture/
│   │   └── obsolete-guides/
│   └── investigations/                # Research & analysis
│       ├── performance-tests/
│       ├── compatibility-tests/
│       └── poc-documents/
│
└── templates/                         # Document templates
    ├── feature-design.md
    ├── test-report.md
    ├── troubleshooting-guide.md
    └── api-endpoint.md
```

---

## Migration Plan

### Phase 1: Create New Structure (5 minutes)
Create all new directories

### Phase 2: Move Current Files (30 minutes)
Categorize and move 524 files to appropriate folders

### Phase 3: Create Navigation (15 minutes)
Create README.md files for each directory

### Phase 4: Update References (10 minutes)
Update ARCHITECTURE.md and other key docs

### Phase 5: Verification (10 minutes)
Ensure no broken links

---

## File Categorization Rules

### Getting Started (01-getting-started/)
- Installation guides
- Quick starts
- Prerequisites
- First-time setup

### Architecture (02-architecture/)
- System design
- Component architecture
- Data flow diagrams
- Technology decisions

### Components (03-components/)
- Component-specific documentation
- Configuration guides
- Internal architecture
- **CVE scanner** (new)
- **SBOM generator** (move from root)

### Deployment (04-deployment/)
- Kubernetes manifests
- Helm charts
- Configuration
- Troubleshooting

### Operations (05-operations/)
- Monitoring & alerting
- Maintenance procedures
- Performance tuning
- Disaster recovery

### Development (06-development/)
- Local setup
- Testing guides
- Migration scripts
- Contributing guidelines

### Features (07-features/)
- Feature specifications
- User guides
- Configuration examples

### API Reference (08-api-reference/)
- REST API docs
- gRPC endpoints
- Webhook specs

### Use Cases (09-use-cases/)
- Real-world examples
- Step-by-step tutorials
- Best practices

### Reference (10-reference/)
- CIS Benchmarks
- Rule catalogs
- Function references
- Glossary

### Archive
- Old test reports
- Deprecated features
- Historical decisions
- Investigation notes

---

## Key Improvements

1. **Number Prefixes**: Logical reading order (01, 02, 03...)
2. **Clear Categories**: Easy to find relevant docs
3. **Component Isolation**: Each component has own folder
4. **Archive Separation**: Historical docs don't clutter main structure
5. **Navigation**: README.md in each folder guides users
6. **Scalability**: Easy to add new components/features

---

## Root Level Files (Keep Simple)

Only these files at docs/ root:
- README.md (main navigation)
- ARCHITECTURE.md (primary reference)
- SECURITY.md (security policies)

Everything else organized in numbered folders.

---

## Next Steps

1. Execute migration script
2. Create README.md for each folder
3. Update ARCHITECTURE.md references
4. Create master navigation (docs/README.md)
5. Verify all internal links
