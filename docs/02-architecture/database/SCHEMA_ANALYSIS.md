# Database Schema Analysis

**Last Validated Against Version:** v2.0.0  
**Last Updated:** 2025-12-27  
**Status:** Current

---

## Overview

This document describes the current database schema state for Fortuna K8s Management Platform. The schema uses PostgreSQL 15+ with Apache AGE extension for graph capabilities.

---

## Schema Design Principles

1. **PostgreSQL as Source of Truth**: All state data (workloads, identities, permissions, risks, insights) stored in PostgreSQL
2. **Graph Layer as Derived View**: Apache AGE builds graph from relational state
3. **ACID Transactions**: All critical operations use transactions
4. **Referential Integrity**: Foreign keys ensure data consistency
5. **Audit-Friendly**: Soft deletes (`deleted_at`) for all entities

---

## Core Tables

### Workloads & Resources

- `pods` - Kubernetes Pods
- `service_accounts` - ServiceAccounts
- `roles` - RBAC Roles
- `role_bindings` - RoleBindings
- `deployments` - Deployments
- `replicasets` - ReplicaSets

### Security & Risk

- `insights` - Security insights and risk findings
- `risks` - Risk assessments
- `cve_matches` - CVE vulnerability matches
- `cves` - CVE database entries
- `package_vulnerabilities` - Package-to-CVE mappings

### SBOM & Components

- `sboms` - Software Bill of Materials
- `sbom_components` - Components within SBOMs
- `pod_image_scans` - Links pods to SBOMs

### Graph Data

- Graph nodes and edges are managed by Apache AGE extension
- Graph is built from relational state (not source of truth)

---

## Key Indexes

### Insights Table
- `idx_insights_created_at` - Created timestamp
- `idx_insights_detected_at` - Detection timestamp
- `idx_insights_insight_type` - Insight type
- `idx_insights_severity` - Severity level
- `idx_insights_resource_uid` - Resource UID

### SBOM Table
- `idx_sboms_image_digest` - Image digest (unique)
- `idx_sboms_created_at` - Creation timestamp

### CVE Matches
- `idx_cve_matches_sbom_id` - SBOM reference
- `idx_cve_matches_cve_id` - CVE reference

---

## Unique Constraints

### Data Integrity Constraints

- `sboms.image_digest` - Unique (one SBOM per image digest)
- `sbom_components(sbom_id, purl)` - Unique (one component per SBOM/PURL)
- `cve_matches(sbom_id, package_name, cve_id)` - Unique (one match per combination)
- `insights(resource_uid, cve_id, insight_type)` - Unique (one insight per resource/CVE/type)

---

## Migration History

Total migrations: 34+ (as of v2.0.0)

Key migrations:
- **Migration 032**: Removed duplicate indexes
- **Migration 033**: Added unique constraints
- **Migration 034**: Standardized CVSS column types

For detailed migration history, see `docs/06-reference/migration/SCHEMA_MIGRATION_HISTORY.md`.

---

## Schema Evolution

The schema has evolved through multiple iterations:

1. **Initial Schema** (Migrations 001-010): Core tables for workloads, RBAC, insights
2. **SBOM Integration** (Migrations 011-020): SBOM and component tracking
3. **CVE Integration** (Migrations 021-030): CVE matching and vulnerability tracking
4. **Optimization** (Migrations 031-034): Index optimization, constraint enforcement

---

## Current Schema Status

✅ **All Critical Issues Resolved**
- No duplicate indexes
- Unique constraints enforced
- CVSS types standardized
- Foreign key relationships intact

⚠️ **Known Optimizations**
- Some historical columns may be unused (audit for cleanup)
- Index usage monitoring recommended for large deployments

---

## Graph Integration

Apache AGE extension provides graph capabilities:

**Graph Nodes:**
- Pod
- ServiceAccount
- Role
- Node
- Image
- CVE

**Graph Edges:**
- RUNS_AS (Pod → ServiceAccount)
- CAN_ACCESS (ServiceAccount → Resource)
- BINDS_TO (RoleBinding → Role)
- EXPOSES (Pod → Image)
- AFFECTED_BY (Image → CVE)

Graph is built from relational state and updated asynchronously.

---

## Performance Considerations

- Indexes optimized for common query patterns
- Soft deletes use `WHERE deleted_at IS NULL` in indexes
- Partitioning not currently used (consider for large-scale deployments)
- Graph queries may be slower for large datasets (consider caching)

---

## Backup & Recovery

- All tables support soft deletes via `deleted_at` timestamp
- Foreign key constraints ensure referential integrity
- Regular backups recommended for production deployments
- Migration rollback supported via migration system

---

## References

- [Migration History](../06-reference/migration/SCHEMA_MIGRATION_HISTORY.md)
- [Database Setup Guide](../../01-getting-started/DATABASE_SETUP.md)
- [Apache AGE Documentation](https://age.apache.org/)

