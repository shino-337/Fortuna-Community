# Repository Structure

**Last Updated**: 2026-01-06

---

## Overview

This repository contains only **production-essential** files needed for:
- Fresh deployment setup
- Automated build and test
- Production operations

Development, debug, and temporary files are kept **local only** and excluded from git.

---

## Directory Structure

### `/deploy/` - Deployment Manifests
**Status**: ✅ All files in git

Kubernetes deployment manifests:
- `core-deployment.yaml` - Core service deployment
- `agent-daemonset.yaml` - Agent DaemonSet
- `infrastructure/` - PostgreSQL, NATS, Redis
- `*.yaml` - All deployment configurations

### `/scripts/` - Production Scripts
**Status**: ✅ Only production scripts in git

**In Git:**
- `build-and-load-containerd.sh` - Build and load images
- `create_mtls_secret.sh` - Generate mTLS certificates
- `load-cve-data.sh` - Load CVE database
- `push-images-to-workers.sh` - Multi-node image distribution
- `verify-database-schema.sh` - Schema verification
- `pre-deployment-checks.sh` - Pre-deployment validation
- `run-e2e.sh` - E2E entry point (--suite=full|risk-center|…)
- `build-production.sh` - Production builds
- `create-github-release.sh` - Release management

**Local Only (excluded):**
- `fix-*.sh` - Debug/fix scripts
- `apply-*.sh` - Temporary deployment scripts
- `clean-*.sh` - Cleanup scripts
- `archive/` - Archived scripts

### `/docs/` - Documentation
**Status**: ✅ Only essential docs in git

**In Git:**
- `README.md`, `DOCS_STRUCTURE.md` - Main entry and structure
- `01-getting-started/`, `02-architecture/`, `03-components/`, `04-development/`, `05-operations/`, `06-reference/`, `07-guides/`, `08-tutorials/` - Current docs
- `test-results/README.md` - Test results index; older reports in `archive/test-results/`
- `archive/` - Outdated / one-off docs (fixes, task-lists, analysis)

**Local Only (excluded):**
- Generated test reports (older ones in `archive/test-results/`)

### `/tests/` - Test Files
**Status**: ⚠️ Structure only, results excluded

**In Git:**
- Test specifications and test cases
- `run-e2e.sh` script

**Local Only (excluded):**
- `results/` - All test results
- `e2e/results/` - E2E test results
- Generated reports

### `/helm/` - Helm Charts
**Status**: ✅ All files in git

Helm charts for Fortuna deployment.

---

## Files Excluded from Git

The following are kept **local only** and excluded via `.gitignore`:

### Scripts
- Debug/fix scripts (`fix-*.sh`)
- Temporary deployment scripts (`apply-*.sh`)
- Cleanup scripts (`clean-*.sh`)
- One-time migration scripts
- Archive directory

### Test Results
- All generated test reports
- Performance test results
- E2E test execution logs

### Documentation
- Development guides (`04-development/`)
- Reference materials (`06-reference/`)
- Detailed architecture docs (except main)
- Test result reports

### Temporary Files
- `CLEANUP_SUMMARY.md`
- `HELM_CHART_UPDATE.md`
- `PROJECT_CLEANUP_ANALYSIS.md`
- `docker-compose.prod.yml`

---

## Why This Structure?

### Benefits
1. **Clean Repository**: Only production-essential files
2. **Fast Clone**: Smaller repository size
3. **Clear Purpose**: Easy to identify what's needed for deployment
4. **No Confusion**: No mixing of production and development files

### Local Development
- All development/debug files remain on your local machine
- Use `.gitignore` to exclude them from commits
- Keep local history and test results

---

## Adding New Files

### Production Files
If a file is needed for fresh deployment:
1. Add to appropriate directory
2. Ensure it's not in `.gitignore`
3. Commit normally

### Development Files
If a file is for development/debug only:
1. Add to `.gitignore` if not already covered
2. Keep local only
3. Do not commit

---

## Maintenance

- Regularly review `.gitignore` to ensure proper exclusions
- Archive old scripts to `scripts/archive/` (local only)
- Keep test results local, commit only test specifications
- Update this document when structure changes

---

**Note**: This structure ensures the repository remains clean and focused on production deployment needs.
