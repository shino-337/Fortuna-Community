# Project Cleanup Analysis

**Date**: 2025-12-29  
**Status**: Analysis Complete

---

## Executive Summary

This document outlines the comprehensive cleanup and reorganization plan for the Fortuna project.

### Issues Identified

1. **Documentation**: 31 markdown files in root `docs/` folder (should be organized)
2. **Scripts**: 42 shell scripts, many duplicates/obsolete
3. **Deployments**: Duplicate deployment files (old vs new naming)
4. **Certificates**: 2 cert folders with unclear usage
5. **Config Files**: Inconsistent naming and organization

---

## 1. Documentation Cleanup

### Current State
- **31 markdown files** in `docs/` root
- Files should be organized into subfolders
- Many duplicate/outdated files

### Files to Organize

#### Move to `docs/01-getting-started/`
- `DEPLOYMENT_QUICK_START.md`
- `DEPLOYMENT_STEP_BY_STEP.md`
- `DEPLOYMENT_COMPLETE_GUIDE.md`
- `DEPLOYMENT_FULL_GUIDE.md`
- `DEPLOYMENT_EXECUTION_LOG.md`
- `START_HERE_NEW.md`
- `START_HERE.md.OLD` (archive or delete)

#### Move to `docs/05-operations/`
- `DNS_TROUBLESHOOTING.md`
- `DEPLOYMENT_NODE_PLACEMENT.md`
- `CONTAINERD_BUILD_GUIDE.md`
- `CONTAINERD_BUILD_TROUBLESHOOTING.md`
- `CONTAINERD_CLEANUP_COMMANDS.md`
- `DOCKERFILE_PRODUCTION_GUIDE.md`

#### Move to `docs/06-reference/`
- `API_REFERENCE_UPDATED.md`
- `ARCHITECTURE_UPDATED.md`
- `DATABASE_SCHEMA_UPDATED.md`
- `DETAILED_LOGIC_FLOW_UPDATED.md`
- `FUNCTIONAL_SPECIFICATION_UPDATED.md`
- `README_UPDATED.md`
- `CHANGELOG_UPDATED.md`

#### Move to `docs/04-development/`
- `VERSION_COMPARISON_IMPLEMENTATION.md`
- `VERSION_COMPARISON_ISSUE_ANALYSIS.md`
- `VERSION_COMPARISON_SOLUTION.md`
- `ASYNC_SBOM_QUEUE_IMPLEMENTATION.md`
- `SBOM_BLOCKING_ISSUE_STATUS.md`

#### Move to `docs/02-architecture/`
- `POLICY_ENGINE_ACTION_PLAN.md`
- `POLICY_ENGINE_IMPLEMENTATION_PLAN.md`
- `policy-engine-analysis.md`
- `migration-executive-summary.md`
- `migration-implementation-checklist.md`

#### Archive/Delete
- `DOCUMENTATION_UPDATE_SUMMARY.md` (outdated)
- `START_HERE.md.OLD` (old version)

---

## 2. Scripts Cleanup

### Current State
- **42 shell scripts** in `scripts/` folder
- Many duplicates and obsolete scripts

### Scripts to Keep (Active)

#### Build Scripts
- ✅ `build-with-containerd.sh` (primary)
- ✅ `build-and-import-containerd.sh` (wrapper)
- ✅ `build-production.sh` (Docker registry)
- ✅ `import-to-containerd.sh` (standalone import)

#### Deployment Scripts
- ✅ `deploy-fortuna-robust.sh` (primary, comprehensive)
- ✅ `quick-deploy-containerd.sh` (quick deploy)
- ✅ `pre-deployment-checks.sh` (validation)

#### Cleanup Scripts
- ✅ `clean-containerd-images.sh` (comprehensive)
- ✅ `clean-all-containerd-images.sh` (quick)

#### Utility Scripts
- ✅ `create-mtls-secrets.sh` (certificate generation)
- ✅ `fix-dns-issues.sh` (DNS troubleshooting)
- ✅ `copy-containerd-images-to-nodes.sh` (multi-node)

### Scripts to Archive/Remove

#### Obsolete Build Scripts
- ❌ `build-and-deploy.sh` (use robust script)
- ❌ `build-and-deploy-multinode.sh` (use copy script)

#### Obsolete Deployment Scripts
- ❌ `deploy-fortuna.sh` (superseded by robust)
- ❌ `deploy-fortuna-complete.sh` (superseded by robust)
- ❌ `complete-deploy.sh` (duplicate)
- ❌ `DEPLOY_ALL.sh` (use robust)
- ❌ `quick-deploy.sh` (use containerd version)

#### Obsolete Fix Scripts
- ❌ `fix-deployment-issues.sh` (superseded by robust)
- ❌ `fix-all-deployment-issues.sh` (superseded by robust)
- ❌ `fix-all-core-issues.sh` (obsolete)
- ❌ `fix-core-connection.sh` (obsolete)
- ❌ `fix-core-database-dns.sh` (use robust)
- ❌ `fix-dns-issue.sh` (use fix-dns-issues.sh)
- ❌ `fix-nats-storage.sh` (obsolete)
- ❌ `fix-pvc-issue.sh` (obsolete)
- ❌ `fix-containerd-images.sh` (use clean script)
- ❌ `fix-agent-core-connection.sh` (obsolete)

#### Obsolete Diagnostic Scripts
- ❌ `diagnose-deployment.sh` (use pre-deployment-checks)
- ❌ `check-core-connection.sh` (obsolete)
- ❌ `check-agent-core-connection.sh` (obsolete)

#### Obsolete Copy Scripts
- ❌ `copy-images-to-all-nodes.sh` (use containerd version)
- ❌ `copy-images-to-k8s-namespace.sh` (obsolete)
- ❌ `manual-copy-images-to-worker.sh` (obsolete)
- ❌ `load-images-to-containerd.sh` (use import script)

#### Migration Scripts (Archive)
- ⚠️ `migrate-imports.sh` (one-time migration, archive)
- ⚠️ `cleanup-orphaned-migrations.sh` (one-time, archive)
- ⚠️ `validate-migrations.sh` (keep for reference)

#### Special Purpose (Keep)
- ✅ `load-cve-database.sh` (CVE loading)
- ✅ `apply-core-master-only.sh` (specific use case)
- ✅ `apply-nats-single-replica.sh` (specific use case)

---

## 3. Deployment Files Cleanup

### Current State
- Duplicate files with old and new naming
- Inconsistent labels and configurations

### Files to Keep (Standardized)

#### Core Components
- ✅ `fortuna-core-deployment.yaml` (primary, updated)
- ✅ `fortuna-agent-daemonset.yaml` (primary, updated)
- ✅ `fortuna-rbac.yaml` (primary)
- ✅ `core-service.yaml` (keep, verify labels)

#### Remove Duplicates
- ❌ `core-deployment.yaml` (old, use fortuna-core-deployment.yaml)
- ❌ `agent-daemonset.yaml` (old, use fortuna-agent-daemonset.yaml)
- ❌ `agent-rbac.yaml` (old, use fortuna-rbac.yaml)

#### Verify and Update
- ⚠️ `core-secrets.yaml` (verify usage)
- ⚠️ `webhook-config.yaml` (verify)
- ⚠️ `webhook-service.yaml` (verify)

---

## 4. Certificate Folders

### Current State
- `KSAM/certs/` - Contains only `ca.srl` (serial file)
- `KSAM/deploy/certs/` - Contains YAML manifests for cert generation

### Analysis

#### `KSAM/certs/`
- **Content**: `ca.srl` (CA serial number file)
- **Usage**: Generated by `create-mtls-secrets.sh`
- **Status**: ✅ Keep (used by cert generation)

#### `KSAM/deploy/certs/`
- **Content**: Certificate YAML manifests
  - `01-root-ca.yaml`
  - `02-core-server-cert.yaml`
  - `03-agent-client-cert.yaml`
- **Usage**: Certificate generation manifests
- **Status**: ✅ Keep (used for cert generation)

### Recommendation
- **Keep both folders** - they serve different purposes
- `certs/` = Generated certificate files
- `deploy/certs/` = Certificate generation manifests

---

## 5. Configuration Files

### Files to Verify
- `docker-compose.yml` (verify if still used)
- `Makefile` (verify targets)
- `go.work` (verify workspace config)

---

## Cleanup Plan

### Phase 1: Documentation Organization
1. Create subfolder structure
2. Move files to appropriate folders
3. Update cross-references
4. Archive outdated files

### Phase 2: Scripts Cleanup
1. Archive obsolete scripts to `scripts/archive/`
2. Update main scripts to reference new locations
3. Create `scripts/README.md` with usage guide

### Phase 3: Deployment Files
1. Remove duplicate files
2. Verify and update remaining files
3. Ensure consistent labeling

### Phase 4: Final Verification
1. Test build scripts
2. Test deployment scripts
3. Verify documentation links
4. Update main README

---

## Files to Delete

### Scripts (Move to archive first)
- `build-and-deploy.sh`
- `build-and-deploy-multinode.sh`
- `deploy-fortuna.sh`
- `deploy-fortuna-complete.sh`
- `complete-deploy.sh`
- `DEPLOY_ALL.sh`
- `quick-deploy.sh`
- `fix-deployment-issues.sh`
- `fix-all-deployment-issues.sh`
- `fix-all-core-issues.sh`
- `fix-core-connection.sh`
- `fix-core-database-dns.sh`
- `fix-dns-issue.sh`
- `fix-nats-storage.sh`
- `fix-pvc-issue.sh`
- `fix-containerd-images.sh`
- `fix-agent-core-connection.sh`
- `diagnose-deployment.sh`
- `check-core-connection.sh`
- `check-agent-core-connection.sh`
- `copy-images-to-all-nodes.sh`
- `copy-images-to-k8s-namespace.sh`
- `manual-copy-images-to-worker.sh`
- `load-images-to-containerd.sh`

### Deployment Files
- `core-deployment.yaml`
- `agent-daemonset.yaml`
- `agent-rbac.yaml`

### Documentation
- `START_HERE.md.OLD`
- `DOCUMENTATION_UPDATE_SUMMARY.md` (if outdated)

---

**Next Steps**: Execute cleanup plan

