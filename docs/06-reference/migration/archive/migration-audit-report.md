# Database Migration Audit & Implementation Plan
## KSAM (Kubernetes Service Account Management Platform)

**Report Date:** December 27, 2025
**Audit Scope:** Complete database migration system review (Go/GORM/PostgreSQL)
**Project Phase:** MVP2 → MVP3 Transition
**Database:** PostgreSQL 12+ with Apache AGE Extension

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Analysis](#2-current-state-analysis)
3. [Risk Assessment Matrix](#3-risk-assessment-matrix)
4. [Detailed Findings](#4-detailed-findings)
5. [Implementation Plan](#5-implementation-plan)
6. [Code Changes Required](#6-code-changes-required)
7. [Testing Strategy](#7-testing-strategy)
8. [Rollback Procedures](#8-rollback-procedures)
9. [Timeline & Resource Allocation](#9-timeline--resource-allocation)
10. [Long-term Recommendations](#10-long-term-recommendations)

---

## 1. Executive Summary

### 1.1 Overall Assessment

**Current Migration System Grade: C+** (Functional with significant technical debt)

The KSAM project operates a **hybrid migration system** combining SQL migrations with GORM AutoMigrate fallbacks. While the system successfully manages 36 sequential migrations across multiple MVP phases, it suffers from production risks that require immediate attention.

### 1.2 Critical Statistics

| Metric | Value | Status |
|--------|-------|--------|
| Total Migrations | 36 | ✅ |
| AutoMigrate Fallbacks | 15 | ⚠️ High Risk |
| Silent Error Handlers | 3 | 🔴 Critical |
| Orphaned SQL Files | 5 | 🟡 Cleanup Needed |
| Rollback Capability | 0% | 🔴 Critical |
| Production Safety Score | 45/100 | ⚠️ Needs Improvement |

### 1.3 Key Findings Summary

**🔴 CRITICAL (Must Fix Immediately):**
1. Silent AutoMigrate failures masked by error suppression
2. No rollback capability for any migration
3. Production relies on AutoMigrate fallbacks

**🟡 HIGH (Fix Within 1 Sprint):**
4. Multiple AGE trigger file variants causing confusion
5. Orphaned SQL files not being used
6. Deprecated Trivy tables consuming resources

**🟢 MEDIUM (Technical Debt):**
7. Missing migration validation in CI/CD
8. No dry-run testing capability
9. Inconsistent migration documentation

### 1.4 Recommended Action

**Proceed with 3-phase implementation:**
- **Phase 1 (Week 1):** Immediate cleanup and risk mitigation
- **Phase 2 (Weeks 2-4):** Remove AutoMigrate fallbacks
- **Phase 3 (Months 2-3):** Implement proper migration tooling

**Estimated Effort:** 3-4 weeks for critical fixes, 2-3 months for complete overhaul

---

## 2. Current State Analysis

### 2.1 Migration Architecture

```
Current Migration Flow:
┌─────────────────────────────────────────────────────────────┐
│  Application Startup (core/cmd/main.go)                     │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  storage.Initialize() (core/internal/storage/storage.go)    │
│  - Creates database connection pool                          │
│  - Calls migrations.RunMigrations(db)                        │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  migrations.RunMigrations() (core/migrations/migrations.go)  │
│  - Executes 36 migrations sequentially                       │
│  - No version tracking table                                 │
│  - No transaction wrapping                                   │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  Individual Migration Functions                              │
│  ├─ SQL File Execution (primary)                             │
│  ├─ AutoMigrate Fallback (if SQL fails/missing) ⚠️          │
│  └─ Silent Error Suppression (known issues) 🔴              │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Migration Inventory

#### Complete Migration List (36 Migrations)

| # | Name | Type | Tables Affected | Risk Level |
|---|------|------|----------------|------------|
| 001 | Initial Schema | SQL + AutoMigrate | clusters, service_accounts, roles, etc. | 🔴 Critical |
| 002 | Add Users | AutoMigrate | users | 🟡 High |
| 003 | Add User to Audit Logs | AutoMigrate | audit_logs | 🟡 High |
| 008 | Add Deployments | SQL + AutoMigrate | deployments | 🟡 High |
| 009 | Add ReplicaSets | SQL + AutoMigrate | replicasets | 🟡 High |
| 010 | Implementation Guide Schema | SQL + AutoMigrate | nodes, policies, insights, events_index | 🟡 High |
| 011 | Add Insights Soft Delete | SQL + AutoMigrate | insights | 🟡 High |
| 012 | Add Risk Scores | SQL + AutoMigrate | risk_scores | 🟡 High |
| 013 | Add Risk Scores Deleted At | SQL + AutoMigrate | risk_scores | 🟡 High |
| 014 | Add Policy Templates | SQL + AutoMigrate | policy_templates | 🟡 High |
| 015 | Add Policy Instances | Pure AutoMigrate | policy_instances | 🟡 High |
| 016 | Add Policy Violations | Pure AutoMigrate | policy_violations | 🟡 High |
| 018 | Add Risk Scores V2 Columns | SQL + AutoMigrate | risk_scores | 🟡 High |
| 019 | Add CVE Tables | SQL Inline | cves, package_vulnerabilities, etc. | 🟢 Low |
| 020 | Add SBOM Tables | SQL + AutoMigrate | sboms, sbom_components, cve_matches | 🟡 High |
| 021 | Fix SBOM Schema | SQL DDL | sbom_components, insights | 🟢 Low |
| 022 | Add CVE Columns to Insights | SQL DDL | insights | 🟢 Low |
| 023 | Fix SBOM/CVE Indexes | SQL DDL | Multiple indexes | 🟢 Low |
| 024 | Add Pod Image Scans Unique Index | SQL DDL | pod_image_scans | 🟢 Low |
| 025 | Make Upsert Indexes Non-Partial | SQL DDL | Multiple indexes | 🟢 Low |
| 026 | Add Insights JSONB Indexes | SQL DDL | insights | 🟢 Low |
| 027 | Add CVE File Metadata | SQL DDL | cve_file_metadata | 🟢 Low |
| 028 | Add Performance Indexes | SQL DDL | Multiple indexes | 🟢 Low |
| 029 | Add Insights Unique Constraint | SQL DDL | insights | 🟢 Low |
| 030 | Migrate Insights Schema Complete | SQL DDL | insights | 🟢 Low |
| 031 | Cleanup Duplicate Indexes | SQL DDL | Multiple indexes | 🟢 Low |
| 032 | Migrate CVE Matches Complete | SQL DDL | cve_matches | 🟢 Low |
| 033 | Add Unique Constraints | SQL DDL | Multiple tables | 🟢 Low |
| 034 | Standardize CVSS Type | SQL DDL | cve_matches, insights, cves | 🟢 Low |
| 035 | Evaluate Trivy Tables | SQL DDL | Trivy tables (marking deprecated) | 🟢 Low |
| 036 | Add Missing SBOM Columns | SQL DDL | sboms | 🟢 Low |

**Risk Distribution:**
- 🔴 Critical Risk: 1 migration (3%)
- 🟡 High Risk: 15 migrations (42%)
- 🟢 Low Risk: 20 migrations (55%)

### 2.3 SQL File Inventory

#### Main Directory (`core/migrations/`)

```
core/migrations/
├── 001_initial_schema.sql ✅ (Used)
├── 002_install_age.sql ⚠️ (Not integrated in RunMigrations)
├── 003_age_triggers.sql ⚠️ (Duplicate - unclear which is used)
├── 003_age_triggers_conditional.sql ⚠️ (Duplicate - unclear which is used)
├── 003_age_triggers_simple.sql ⚠️ (Duplicate - unclear which is used)
├── 004_age_functions_full.sql ⚠️ (Not integrated in RunMigrations)
├── 008_add_deployments.sql ✅ (Used)
├── 009_add_replicasets.sql ✅ (Used)
├── 010_add_implementation_guide_schema.sql ✅ (Used)
├── 011_add_insights_soft_delete.sql ✅ (Used)
├── mvp3_agent_based_schema.sql 🔴 (Not integrated - orphaned)
└── [*.go files] ✅ (Active migrations)
```

#### MVP2 Directory (`core/migrations/mvp2/`)

```
core/migrations/mvp2/
├── 001_risk_scores.sql ✅ (Used)
├── 002_add_risk_scores_deleted_at.sql ✅ (Used)
├── 003_policy_templates.sql ✅ (Used)
├── 004_policy_instances.sql 🔴 (NOT USED - code uses AutoMigrate)
├── 005_policy_violations.sql 🔴 (NOT USED - code uses AutoMigrate)
├── 004_add_risk_scores_v2_columns.sql ✅ (Used)
├── 005_add_cve_tables.sql 🔴 (NOT USED - code uses inline SQL)
└── 006_add_sbom_tables.sql ✅ (Used)
```

**Legend:**
- ✅ Used and working
- ⚠️ Unclear status / duplicates
- 🔴 Not used / orphaned

### 2.4 GORM Model Inventory

**Location:** `core/pkg/models/`

| File | Models Defined | Tables | AutoMigrate Risk |
|------|----------------|--------|------------------|
| models.go | Cluster, ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding, Pod, AuditLog | 8 tables | 🔴 High (Migration 001) |
| user.go | User | users | 🟡 Medium (Migration 002) |
| deployment.go | Deployment | deployments | 🟡 Medium (Migration 008) |
| replicaset.go | ReplicaSet | replicasets | 🟡 Medium (Migration 009) |
| implementation_guide.go | Node, Policy, Insight, EventIndex | 4 tables | 🟡 Medium (Migration 010) |
| risk_score.go | RiskScore | risk_scores | 🟡 Medium (Migration 012) |
| policy_template.go | PolicyTemplate | policy_templates | 🟡 Medium (Migration 014) |
| policy_instance.go | PolicyInstance | policy_instances | 🟡 Medium (Migration 015) |
| policy_violation.go | PolicyViolation | policy_violations | 🟡 Medium (Migration 016) |
| cve.go | CVE, PackageVulnerability, ImageScanResult, PodImageScan | 4 tables | 🟢 Low (SQL-based) |
| sbom.go | SBOM, SBOMComponent, CVEMatch | 3 tables | 🟡 Medium (Migration 020) |

**Total Models:** 25 models → 25 database tables

---

## 3. Risk Assessment Matrix

### 3.1 Critical Risks (Immediate Action Required)

#### 🔴 RISK-001: Silent AutoMigrate Failures

**Severity:** CRITICAL
**Likelihood:** MEDIUM (happens in edge cases)
**Impact:** HIGH (schema corruption, data loss)

**Location:** `core/migrations/migrations.go:98-114, 189-227`

**Current Code:**
```go
// Line 98-114
if errStr != "" && (strings.Contains(errStr, "insufficient arguments") ||
    strings.Contains(errStr, "migration 1 failed") ||
    strings.Contains(errStr, "Migration 1 failed")) {
    log.Printf("WARNING: Migration %d encountered known issue...", i+1)
    // ... table existence check ...
    if checkErr == nil && tableExists {
        log.Printf("Migration %d: Tables verified to exist, continuing despite error", i+1)
        continue // <-- SILENT FAILURE
    }
}
```

**Problem:**
- Known GORM/PostgreSQL errors are suppressed
- Migration continues even if tables might not be created
- No explicit validation that schema matches expected state
- Production deployments may have incomplete schema

**Business Impact:**
- Application crashes due to missing tables/columns
- Data corruption if writes attempt to use non-existent columns
- Silent schema drift between environments

**CVSS Score:** 7.5 (High)

**Recommended Fix:** See Section 6.1

---

#### 🔴 RISK-002: No Rollback Capability

**Severity:** CRITICAL
**Likelihood:** HIGH (migrations are complex and frequently added)
**Impact:** HIGH (cannot undo failed migrations)

**Current Situation:**
- All migrations are forward-only
- No `Down()` migration functions
- No version tracking table
- If migration fails mid-execution, manual intervention required

**Real-World Scenario:**
```
1. Deploy v2.5.0 with new migration
2. Migration fails at step 15 of 20
3. Database is in inconsistent state
4. Cannot rollback automatically
5. Must manually write rollback SQL
6. Risk of data loss during manual rollback
```

**Business Impact:**
- Extended downtime during failed deployments
- Risk of data loss during manual rollbacks
- Cannot safely test migrations in production
- Deployment rollbacks require database rollback

**CVSS Score:** 8.0 (High)

**Recommended Fix:** See Section 6.2

---

#### 🔴 RISK-003: Production AutoMigrate Fallback

**Severity:** CRITICAL
**Likelihood:** LOW (SQL files are committed to repo)
**Impact:** CRITICAL (uncontrolled schema changes)

**Location:** Migrations 001, 008-016, 020

**Current Code Pattern:**
```go
func Migration001_InitialSchema(db *gorm.DB) error {
    sqlBytes, err := os.ReadFile("migrations/001_initial_schema.sql")
    if err == nil && len(sqlBytes) > 0 {
        if err := db.Exec(string(sqlBytes)).Error; err != nil {
            log.Printf("Warning: SQL migration had errors, attempting AutoMigrate...")
        } else {
            return nil
        }
    }

    // FALLBACK TO AUTOMIGRATE IN PRODUCTION ⚠️
    log.Println("Using AutoMigrate for migration 001")
    return db.AutoMigrate(&models.Cluster{}, &models.ServiceAccount{}, ...)
}
```

**Problem:**
- If SQL file is missing/unreadable, AutoMigrate runs in production
- AutoMigrate may create different schema than SQL
- No control over index creation order
- No control over constraint creation

**Attack Scenarios:**
1. **File System Issues:** SQL file missing due to Docker layer issue → AutoMigrate creates wrong schema
2. **Path Issues:** Application runs from unexpected directory → SQL file not found → AutoMigrate runs
3. **Permission Issues:** SQL file not readable → AutoMigrate runs

**Business Impact:**
- Schema drift between environments
- Missing indexes causing performance issues
- Wrong data types causing data truncation
- Constraint violations not enforced

**CVSS Score:** 8.5 (Critical)

**Recommended Fix:** See Section 6.3

---

### 3.2 High-Priority Risks (Fix Within 1 Sprint)

#### 🟡 RISK-004: Multiple AGE Trigger Variants

**Severity:** MEDIUM
**Likelihood:** MEDIUM
**Impact:** MEDIUM

**Files:**
- `003_age_triggers.sql`
- `003_age_triggers_conditional.sql`
- `003_age_triggers_simple.sql`

**Problem:**
- Three different implementations exist
- Unclear which one is being used (none referenced in `migrations.go`)
- AGE extension triggers may or may not be installed

**Impact:**
- Graph database features may not work
- Inconsistent behavior between environments
- Maintenance confusion

**Recommended Fix:** See Section 6.4

---

#### 🟡 RISK-005: Orphaned SQL Files

**Severity:** MEDIUM
**Likelihood:** HIGH
**Impact:** LOW

**Files to Delete:**
1. `core/migrations/mvp2/004_policy_instances.sql` (code uses AutoMigrate)
2. `core/migrations/mvp2/005_policy_violations.sql` (code uses AutoMigrate)
3. `core/migrations/mvp2/005_add_cve_tables.sql` (code uses inline SQL)
4. `core/migrations/mvp3_agent_based_schema.sql` (not integrated)

**Problem:**
- Dead code in repository
- Source of truth confusion
- Future developers may use wrong file

**Impact:**
- Developer confusion
- Potential bugs if wrong file is used

**Recommended Fix:** See Section 6.5

---

#### 🟡 RISK-006: Deprecated Table Cleanup Missing

**Severity:** MEDIUM
**Likelihood:** LOW
**Impact:** MEDIUM

**Tables Marked Deprecated (Migration 035):**
- `cves` (Trivy-based)
- `package_vulnerabilities` (Trivy-based)
- `image_scan_results` (Trivy-based)

**Problem:**
- Tables still exist in schema
- Consuming storage and backup time
- May contain stale data

**Impact:**
- Unnecessary storage costs
- Slower backups
- Confusion about which tables are active

**Recommended Fix:** See Section 6.6

---

### 3.3 Medium-Priority Risks (Technical Debt)

#### 🟢 INFO-001: No Migration Validation in CI/CD

**Current State:** Migrations are not tested in CI pipeline

**Recommendation:**
- Add migration dry-run tests
- Validate migration file existence
- Check for sequential numbering
- Grep for forbidden patterns (AutoMigrate in production code)

---

#### 🟢 INFO-002: Inconsistent Migration Documentation

**Current State:** Some migrations have detailed comments, others don't

**Recommendation:**
- Standardize migration header format
- Require rollback plan in comments
- Add JIRA ticket references

---

## 4. Detailed Findings

### 4.1 AutoMigrate Usage Analysis

**Total AutoMigrate Calls Found:** 15 locations

#### Production Usage (⚠️ High Risk)

| Migration | Primary Method | Fallback Method | Risk |
|-----------|----------------|----------------|------|
| 001 | SQL File | AutoMigrate | 🔴 Critical |
| 002 | AutoMigrate | None | 🟡 High |
| 003 | AutoMigrate | None | 🟡 High |
| 008 | SQL File | AutoMigrate | 🟡 High |
| 009 | SQL File | AutoMigrate | 🟡 High |
| 010 | SQL File | AutoMigrate | 🟡 High |
| 011 | SQL File | AutoMigrate | 🟡 High |
| 012 | SQL File | AutoMigrate | 🟡 High |
| 013 | SQL File | AutoMigrate | 🟡 High |
| 014 | SQL File | AutoMigrate | 🟡 High |
| 015 | AutoMigrate | None | 🟡 High |
| 016 | AutoMigrate | None | 🟡 High |
| 018 | SQL File | AutoMigrate | 🟡 High |
| 020 | SQL File | AutoMigrate | 🟡 High |

#### Test Usage (✅ Acceptable)

**Location:** `core/pkg/risk/scorer_v2_test.go`, `core/internal/api/risk/risk_handlers_v2_test.go`

**Usage:** AutoMigrate for ephemeral test databases

**Verdict:** ✅ Acceptable (test-only usage)

---

### 4.2 Schema Source of Truth Analysis

**Key Finding:** GORM models and SQL migrations are **mostly consistent** with a few exceptions

#### Resolved Conflicts

| Issue | Resolution | Migration |
|-------|-----------|-----------|
| `sbom_components.p_url` vs `purl` | ✅ Migration 021 renames column | 021 |
| CVSS type mismatch | ✅ Migration 034 standardizes types | 034 |
| `insights` schema drift | ✅ Migration 030 consolidates schema | 030 |
| `cve_matches` schema drift | ✅ Migration 032 consolidates schema | 032 |

#### Active Conflicts (⚠️ Requires Attention)

**CVE.References Column Name Workaround:**

GORM Model uses column override due to PostgreSQL reserved keyword:
```go
// core/pkg/models/cve.go:25
References string `gorm:"type:jsonb;column:cve_references" json:"references"`
```

**Verdict:** ✅ Acceptable (proper workaround for reserved keyword)

---

### 4.3 Migration Consolidation History

**Migrations 030, 031, 032** consolidated previous attempts:

**Migration 030 Consolidates:**
- Old Migration 030: Migrate insights to new schema
- Old Migration 031: Cleanup old insights columns
- Old Migration 038: Cleanup old schema columns (insights portion)

**Migration 031 Consolidates:**
- Old Migration 032: Remove duplicate indexes
- Old Migration 038: Cleanup old schema columns (index portion)

**Migration 032 Consolidates:**
- Old Migration 037: Migrate CVE matches to package_name
- Old Migration 039: Add missing CVE match columns

**Status:** ✅ Consolidation successful, but old migrations never existed as separate files

**Recommendation:** Add historical context in migration headers

---

### 4.4 Index Management Analysis

**Total Indexes Created:** ~50+ indexes across all migrations

**Index Types:**
- B-tree indexes (default): ~40
- GIN indexes (JSONB): ~8
- Unique indexes: ~12
- Partial indexes (with WHERE clause): ~10

**Key Findings:**

✅ **Good Practices:**
- Idempotency: All index creation uses `CREATE INDEX IF NOT EXISTS`
- Performance indexes added in dedicated migration (028)
- JSONB indexes properly use GIN index type (026)

⚠️ **Issues Found:**
- Migration 025 rebuilds indexes to remove partial conditions
- Some duplicate indexes cleaned up in migrations 030-031

**Verdict:** Index management is well-handled in recent migrations

---

### 4.5 Foreign Key Management Analysis

**Foreign Keys Dropped (Migration 030):**
```sql
ALTER TABLE insights DROP CONSTRAINT IF EXISTS insights_sbom_id_fkey CASCADE;
ALTER TABLE insights DROP CONSTRAINT IF EXISTS insights_cve_match_id_fkey CASCADE;
```

**Reason:** Migration from relational to denormalized schema for performance

**Impact:**
- Referential integrity no longer enforced at database level
- Application must ensure data consistency
- Faster inserts/updates (no FK checks)

**Verdict:** ✅ Acceptable trade-off for performance-critical table

---

## 5. Implementation Plan

### Phase 1: Immediate Cleanup & Risk Mitigation (Week 1)

**Objective:** Remove dead code and fix critical bugs without breaking changes

**Duration:** 5 working days
**Risk Level:** 🟢 LOW
**Rollback:** Easy (all changes are additions/deletions)

#### Task 1.1: File Cleanup (Day 1)

**Actions:**
1. Identify which AGE trigger variant is used (if any)
2. Delete unused AGE trigger SQL files
3. Archive `mvp3_agent_based_schema.sql`
4. Delete orphaned MVP2 SQL files
5. Delete untracked migration files (038, 039 if they exist)

**Implementation:** See Section 6.5

---

#### Task 1.2: Add Explicit Schema Validation (Days 2-3)

**Actions:**
1. Create `validateMigrationResult()` helper function
2. Add table validation after Migration 001
3. Add column validation for critical migrations
4. Replace `continue` with explicit failure

**Implementation:** See Section 6.1

---

#### Task 1.3: Add Migration Documentation (Day 4)

**Actions:**
1. Standardize migration header comments
2. Add rollback plans to all migrations
3. Document consolidation in 030-032
4. Update README with migration guidelines

**Implementation:** See Section 6.7

---

#### Task 1.4: Add CI Validation (Day 5)

**Actions:**
1. Create migration validation script
2. Add CI job to check migration integrity
3. Validate no AutoMigrate in production code
4. Check sequential numbering

**Implementation:** See Section 6.8

---

### Phase 2: Remove AutoMigrate Fallbacks (Weeks 2-4)

**Objective:** Eliminate AutoMigrate from production code path

**Duration:** 15 working days
**Risk Level:** 🟡 MEDIUM
**Rollback:** Moderate (requires testing in staging first)

#### Task 2.1: Convert Pure AutoMigrate Migrations to SQL (Week 2)

**Migrations to Convert:**
- Migration 002: Add Users
- Migration 003: Add User to Audit Logs
- Migration 015: Add Policy Instances
- Migration 016: Add Policy Violations

**Actions:**
1. Generate SQL from GORM models
2. Create SQL files: `002_add_users.sql`, `003_add_user_to_audit_logs.sql`, etc.
3. Update migration functions to use SQL files
4. Test in local environment
5. Test in staging environment

**Implementation:** See Section 6.9

---

#### Task 2.2: Remove AutoMigrate Fallbacks (Week 3)

**Migrations to Fix:**
- Migration 001, 008-014, 018, 020

**Actions:**
1. Remove `if err != nil { AutoMigrate... }` blocks
2. Make SQL file path mandatory
3. Fail hard if SQL file not found
4. Add environment check (only fail in production)

**Implementation:** See Section 6.3

---

#### Task 2.3: Add Dry-Run Mode (Week 4)

**Actions:**
1. Add `--dry-run` flag to migration runner
2. Wrap migrations in transaction with rollback
3. Create test harness for migration validation
4. Document dry-run usage

**Implementation:** See Section 6.10

---

### Phase 3: Implement Proper Migration Tooling (Months 2-3)

**Objective:** Add rollback capability and professional migration management

**Duration:** 6-8 weeks
**Risk Level:** 🟡 MEDIUM-HIGH
**Rollback:** Complex (requires careful planning)

#### Task 3.1: Evaluate Migration Tools (Week 1)

**Options to Evaluate:**
1. golang-migrate (recommended)
2. Atlas by Ariga
3. Custom tool enhancement

**Actions:**
1. Prototype with each tool
2. Test with current migrations
3. Document pros/cons
4. Make decision

**Implementation:** See Section 10.2

---

#### Task 3.2: Implement Version Tracking (Week 2)

**Actions:**
1. Create `schema_migrations` table
2. Add version insert/update logic
3. Check applied migrations before running
4. Add migration status tracking

**Implementation:** See Section 6.11

---

#### Task 3.3: Create Down Migrations (Weeks 3-4)

**Actions:**
1. Write `down` function for each migration
2. Test rollback in isolated environment
3. Document rollback procedure
4. Add rollback validation

**Implementation:** See Section 6.12

---

#### Task 3.4: Migrate to golang-migrate (Weeks 5-8)

**Actions:**
1. Convert current migrations to golang-migrate format
2. Create `.up.sql` and `.down.sql` files
3. Update deployment scripts
4. Test end-to-end in staging
5. Deploy to production

**Implementation:** See Section 10.3

---

### Phase 4: Deprecate Old Tables (Month 4+)

**Objective:** Clean up deprecated Trivy tables

**Duration:** 2 weeks
**Risk Level:** 🟢 LOW (data already migrated)

#### Task 4.1: Verify Data Migration (Week 1)

**Actions:**
1. Query deprecated tables for recent data
2. Verify all data exists in new SBOM tables
3. Document findings
4. Get stakeholder approval

**Implementation:** See Section 6.6

---

#### Task 4.2: Archive and Drop Tables (Week 2)

**Actions:**
1. Export table data to CSV backups
2. Create migration 037: Rename tables with `_deprecated` suffix
3. Create migration 038: Drop deprecated tables
4. Test in staging
5. Deploy to production

**Implementation:** See Section 6.6

---

## 6. Code Changes Required

### 6.1 Fix Silent AutoMigrate Failures (RISK-001)

**File:** `core/migrations/migrations.go`

**Current Code (Lines 98-114):**
```go
if errStr != "" && (strings.Contains(errStr, "insufficient arguments") ||
    strings.Contains(errStr, "migration 1 failed") ||
    strings.Contains(errStr, "Migration 1 failed")) {
    log.Printf("WARNING: Migration %d encountered known GORM/PostgreSQL issue...", i+1)
    // Verify tables exist before continuing
    var tableExists bool
    if checkErr := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'clusters')").Scan(&tableExists).Error; checkErr == nil && tableExists {
        log.Printf("Migration %d: Tables verified to exist, continuing despite error", i+1)
        continue // <-- PROBLEM: Silent failure
    }
    // ... more logic ...
}
```

**Proposed Fix:**

```go
// Add helper function
func validateMigrationResult(db *gorm.DB, migrationNum int, requiredTables []string) error {
    for _, tableName := range requiredTables {
        var exists bool
        query := "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = $1)"
        if err := db.Raw(query, tableName).Scan(&exists).Error; err != nil {
            return fmt.Errorf("failed to check table %s: %w", tableName, err)
        }
        if !exists {
            return fmt.Errorf("migration %d: required table %s was not created", migrationNum, tableName)
        }
    }
    return nil
}

// Updated error handling
if errStr != "" && (strings.Contains(errStr, "insufficient arguments") ||
    strings.Contains(errStr, "migration 1 failed") ||
    strings.Contains(errStr, "Migration 1 failed")) {

    log.Printf("WARNING: Migration %d encountered known GORM/PostgreSQL issue (insufficient arguments)", i+1)
    log.Printf("This is a known compatibility issue between GORM and PostgreSQL")

    // For migration 1, validate all core tables exist
    if i == 0 {
        requiredTables := []string{"clusters", "service_accounts", "roles", "cluster_roles",
                                    "role_bindings", "cluster_role_bindings", "pods", "audit_logs"}
        if err := validateMigrationResult(db, i+1, requiredTables); err != nil {
            // FAIL LOUDLY - don't continue with broken schema
            return fmt.Errorf("migration %d schema validation failed: %w", i+1, err)
        }
        log.Printf("Migration %d: All required tables validated successfully", i+1)
        continue
    }

    // For other migrations, still validate but be less strict
    log.Printf("Migration %d: Manual validation recommended", i+1)
}
```

**Testing:**
1. Test with missing tables → should fail loudly
2. Test with all tables present → should pass
3. Test with partial tables → should fail loudly

---

### 6.2 Implement Rollback Capability (RISK-002)

**File:** `core/migrations/migrations.go`

**Step 1: Add Version Tracking Table**

```go
// Add to migrations.go
func ensureVersionTable(db *gorm.DB) error {
    sql := `
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        rolled_back_at TIMESTAMP,
        checksum VARCHAR(64)
    );
    CREATE INDEX IF NOT EXISTS idx_schema_migrations_applied ON schema_migrations(applied_at);
    `
    return db.Exec(sql).Error
}
```

**Step 2: Check Applied Migrations**

```go
func isMigrationApplied(db *gorm.DB, version int) (bool, error) {
    var count int64
    err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ? AND rolled_back_at IS NULL", version).Scan(&count).Error
    return count > 0, err
}

func recordMigration(db *gorm.DB, version int, name string) error {
    sql := "INSERT INTO schema_migrations (version, name) VALUES ($1, $2) ON CONFLICT (version) DO NOTHING"
    return db.Exec(sql, version, name).Error
}
```

**Step 3: Update RunMigrations**

```go
func RunMigrations(db *gorm.DB) error {
    log.Println("Running database migrations...")

    // Ensure version table exists
    if err := ensureVersionTable(db); err != nil {
        return fmt.Errorf("failed to create schema_migrations table: %w", err)
    }

    // Define migrations with metadata
    type migrationDef struct {
        version int
        name    string
        up      func(*gorm.DB) error
        down    func(*gorm.DB) error // For future rollback support
    }

    migrations := []migrationDef{
        {1, "initial_schema", Migration001_InitialSchema, nil},
        {2, "add_users", Migration002_AddUsers, nil},
        // ... etc
    }

    for _, m := range migrations {
        // Check if already applied
        applied, err := isMigrationApplied(db, m.version)
        if err != nil {
            return fmt.Errorf("failed to check migration %d status: %w", m.version, err)
        }
        if applied {
            log.Printf("Migration %d (%s) already applied, skipping", m.version, m.name)
            continue
        }

        log.Printf("Running migration %d: %s", m.version, m.name)
        if err := m.up(db); err != nil {
            return fmt.Errorf("migration %d failed: %w", m.version, err)
        }

        // Record migration
        if err := recordMigration(db, m.version, m.name); err != nil {
            return fmt.Errorf("failed to record migration %d: %w", m.version, err)
        }

        log.Printf("Migration %d completed successfully", m.version)
    }

    log.Println("All migrations completed successfully")
    return nil
}
```

**Step 4: Add Rollback Function (Future)**

```go
func RollbackMigration(db *gorm.DB, version int) error {
    // TODO: Implement when Down functions are created
    return fmt.Errorf("rollback not yet implemented")
}
```

---

### 6.3 Remove Production AutoMigrate Fallbacks (RISK-003)

**File:** `core/migrations/migrations.go`

**Migration 001 (Critical):**

**Current Code:**
```go
func Migration001_InitialSchema(db *gorm.DB) error {
    // Try multiple paths for SQL file
    sqlPaths := []string{
        "migrations/001_initial_schema.sql",
        "/app/migrations/001_initial_schema.sql",
        "./migrations/001_initial_schema.sql",
    }

    var sqlBytes []byte
    var err error
    for _, path := range sqlPaths {
        sqlBytes, err = os.ReadFile(path)
        if err == nil {
            log.Printf("Found SQL migration file at: %s", path)
            break
        }
    }

    if err == nil && len(sqlBytes) > 0 {
        if err := db.Exec(string(sqlBytes)).Error; err != nil {
            log.Printf("Warning: SQL migration had errors: %v. Attempting AutoMigrate fallback.", err)
        } else {
            log.Println("SQL migration 001 executed successfully")
            return nil
        }
    } else {
        log.Printf("SQL migration file not found, using AutoMigrate")
    }

    // FALLBACK TO AUTOMIGRATE (REMOVE THIS)
    log.Println("Using AutoMigrate for migration 001")
    // ... AutoMigrate code ...
}
```

**Proposed Fix:**

```go
func Migration001_InitialSchema(db *gorm.DB) error {
    log.Println("Running migration 001: Initial schema")

    // Determine environment
    env := os.Getenv("ENVIRONMENT")
    if env == "" {
        env = "development"
    }

    // Try multiple paths for SQL file
    sqlPaths := []string{
        "migrations/001_initial_schema.sql",
        "/app/migrations/001_initial_schema.sql",
        "./migrations/001_initial_schema.sql",
    }

    var sqlBytes []byte
    var err error
    var foundPath string

    for _, path := range sqlPaths {
        sqlBytes, err = os.ReadFile(path)
        if err == nil {
            foundPath = path
            log.Printf("Found SQL migration file at: %s", path)
            break
        }
    }

    // In production, SQL file is MANDATORY
    if env == "production" || env == "staging" {
        if err != nil || len(sqlBytes) == 0 {
            return fmt.Errorf("CRITICAL: SQL migration file not found. Production requires explicit SQL migrations. Tried paths: %v", sqlPaths)
        }

        // Execute SQL
        if err := db.Exec(string(sqlBytes)).Error; err != nil {
            return fmt.Errorf("SQL migration failed: %w", err)
        }

        // Validate result
        requiredTables := []string{"clusters", "service_accounts", "roles", "cluster_roles",
                                    "role_bindings", "cluster_role_bindings", "pods", "audit_logs"}
        if err := validateMigrationResult(db, 1, requiredTables); err != nil {
            return err
        }

        log.Println("Migration 001 completed successfully (SQL)")
        return nil
    }

    // In development, allow AutoMigrate fallback
    if err != nil || len(sqlBytes) == 0 {
        log.Println("Development environment: SQL file not found, using AutoMigrate")
        tables := []interface{}{
            &models.Cluster{},
            &models.ServiceAccount{},
            &models.Role{},
            &models.ClusterRole{},
            &models.RoleBinding{},
            &models.ClusterRoleBinding{},
            &models.Pod{},
            &models.AuditLog{},
        }
        return db.AutoMigrate(tables...)
    }

    // Development with SQL file
    if err := db.Exec(string(sqlBytes)).Error; err != nil {
        log.Printf("SQL migration failed, using AutoMigrate fallback: %v", err)
        tables := []interface{}{
            &models.Cluster{},
            &models.ServiceAccount{},
            &models.Role{},
            &models.ClusterRole{},
            &models.RoleBinding{},
            &models.ClusterRoleBinding{},
            &models.Pod{},
            &models.AuditLog{},
        }
        return db.AutoMigrate(tables...)
    }

    log.Println("Migration 001 completed successfully")
    return nil
}
```

**Apply Same Pattern to:**
- Migration 008 (Deployments)
- Migration 009 (ReplicaSets)
- Migration 010 (Implementation Guide)
- Migration 011 (Insights Soft Delete)
- Migration 012-014 (Risk Scores, Policy Templates)
- Migration 018 (Risk Scores V2)
- Migration 020 (SBOM Tables)

**Testing:**
1. Set `ENVIRONMENT=production` and test without SQL file → should fail
2. Set `ENVIRONMENT=production` and test with SQL file → should succeed
3. Set `ENVIRONMENT=development` and test without SQL file → should use AutoMigrate
4. Set `ENVIRONMENT=development` and test with SQL file → should use SQL

---

### 6.4 Resolve AGE Trigger Variants (RISK-004)

**Investigation Steps:**

1. **Check if AGE extension is actually used:**
```sql
-- Connect to database and run:
SELECT * FROM pg_extension WHERE extname = 'age';
```

2. **Check if any AGE triggers exist:**
```sql
SELECT trigger_name, event_object_table, action_statement
FROM information_schema.triggers
WHERE trigger_name LIKE '%age%';
```

3. **Check application code for AGE usage:**
```bash
cd /Users/tuatnh/Desktop/Learn/K8s\ Service\ Account\ Management\ Platform/KSAM
grep -r "ag_catalog" core/
grep -r "cypher" core/
```

**Decision Matrix:**

| Scenario | Action |
|----------|--------|
| AGE is used and triggers are active | Keep one variant, delete others |
| AGE extension exists but no triggers | Delete all trigger variants |
| AGE extension not installed | Delete AGE-related files (002, 003, 004) OR archive for future use |

**Implementation (if keeping AGE):**

Create file: `core/migrations/deprecate_age_files.md`
```markdown
# AGE Graph Extension Files

## Status: DEPRECATED (or ACTIVE - update based on investigation)

## Files:
- 002_install_age.sql - Installs Apache AGE extension
- 003_age_triggers.sql - Full trigger implementation
- 003_age_triggers_conditional.sql - Conditional trigger implementation
- 003_age_triggers_simple.sql - Simplified trigger implementation
- 004_age_functions_full.sql - AGE utility functions

## Current State (as of 2025-12-27):
- AGE extension is NOT integrated into RunMigrations
- Graph database features are NOT currently used
- Files kept for potential MVP3 integration

## Recommendation:
If graph database features are not planned:
- Move files to `core/migrations/archive/age/`
- Document decision in this file

If graph database features are planned:
- Determine which trigger variant to use
- Delete unused variants
- Integrate into RunMigrations as Migration 004-005
```

**Cleanup Script:**

```bash
#!/bin/bash
# File: scripts/cleanup-age-files.sh

cd "$(dirname "$0")/../core/migrations"

# Create archive directory
mkdir -p archive/age

# Move AGE files to archive
mv 002_install_age.sql archive/age/
mv 003_age_triggers.sql archive/age/
mv 003_age_triggers_conditional.sql archive/age/
mv 003_age_triggers_simple.sql archive/age/
mv 004_age_functions_full.sql archive/age/

# Create README
cat > archive/age/README.md << 'EOF'
# Apache AGE Graph Extension Files

These files were archived on 2025-12-27 as they are not currently integrated
into the migration system.

If graph database features are needed in the future, these files can be
re-integrated.

## Files:
- 002_install_age.sql - Installs Apache AGE extension
- 003_age_triggers*.sql - Various trigger implementations
- 004_age_functions_full.sql - AGE utility functions
EOF

echo "AGE files archived successfully"
```

---

### 6.5 Delete Orphaned SQL Files (RISK-005)

**Cleanup Script:**

Create file: `scripts/cleanup-orphaned-migrations.sh`

```bash
#!/bin/bash
# File: scripts/cleanup-orphaned-migrations.sh

set -e

MIGRATIONS_DIR="core/migrations"
MVP2_DIR="$MIGRATIONS_DIR/mvp2"
ARCHIVE_DIR="$MIGRATIONS_DIR/archive"

echo "========================================="
echo "Orphaned Migration Files Cleanup Script"
echo "========================================="

# Create archive directories
mkdir -p "$ARCHIVE_DIR/mvp2"
mkdir -p "$ARCHIVE_DIR/mvp3"

# Function to archive file
archive_file() {
    local file=$1
    local dest=$2
    if [ -f "$file" ]; then
        echo "Archiving: $file -> $dest"
        mv "$file" "$dest"
    else
        echo "File not found (already cleaned?): $file"
    fi
}

# Archive MVP2 orphaned files
echo ""
echo "Archiving MVP2 orphaned SQL files..."
archive_file "$MVP2_DIR/004_policy_instances.sql" "$ARCHIVE_DIR/mvp2/"
archive_file "$MVP2_DIR/005_policy_violations.sql" "$ARCHIVE_DIR/mvp2/"
archive_file "$MVP2_DIR/005_add_cve_tables.sql" "$ARCHIVE_DIR/mvp2/"

# Archive MVP3 schema file
echo ""
echo "Archiving MVP3 schema file..."
archive_file "$MIGRATIONS_DIR/mvp3_agent_based_schema.sql" "$ARCHIVE_DIR/mvp3/"

# Delete untracked migration files if they exist
echo ""
echo "Checking for untracked migration files..."
if [ -f "$MIGRATIONS_DIR/038_cleanup_old_schema_columns.go" ]; then
    echo "Deleting: 038_cleanup_old_schema_columns.go (merged into 030)"
    rm "$MIGRATIONS_DIR/038_cleanup_old_schema_columns.go"
fi

if [ -f "$MIGRATIONS_DIR/039_add_missing_cve_match_columns.go" ]; then
    echo "Deleting: 039_add_missing_cve_match_columns.go (merged into 032)"
    rm "$MIGRATIONS_DIR/039_add_missing_cve_match_columns.go"
fi

# Create archive README
cat > "$ARCHIVE_DIR/README.md" << 'EOF'
# Archived Migration Files

This directory contains migration files that were removed from the active
migration system but are kept for historical reference.

## Directory Structure:

- `mvp2/` - Orphaned MVP2 SQL files that were replaced by Go migrations
- `mvp3/` - MVP3 schema files not yet integrated
- `age/` - Apache AGE graph extension files (if archived)

## Why These Files Were Archived:

### MVP2 Files:
- `004_policy_instances.sql` - Migration 015 uses AutoMigrate instead
- `005_policy_violations.sql` - Migration 016 uses AutoMigrate instead
- `005_add_cve_tables.sql` - Migration 019 uses inline SQL instead

### MVP3 Files:
- `mvp3_agent_based_schema.sql` - Not yet integrated into RunMigrations

## Restoration:

If these files are needed in the future, they can be restored from this archive.

Last updated: 2025-12-27
EOF

echo ""
echo "========================================="
echo "Cleanup completed successfully!"
echo "========================================="
echo ""
echo "Summary:"
echo "  - Archived files are in: $ARCHIVE_DIR"
echo "  - Active migrations remain in: $MIGRATIONS_DIR"
echo ""
echo "Next steps:"
echo "  1. Review archived files in: $ARCHIVE_DIR"
echo "  2. Commit changes: git add core/migrations"
echo "  3. Commit changes: git commit -m 'chore: archive orphaned migration files'"
```

**Usage:**
```bash
chmod +x scripts/cleanup-orphaned-migrations.sh
./scripts/cleanup-orphaned-migrations.sh
```

---

### 6.6 Deprecate Trivy Tables (RISK-006)

**Step 1: Verify Data Migration (SQL Queries)**

Create file: `scripts/verify-trivy-migration.sql`

```sql
-- File: scripts/verify-trivy-migration.sql
-- Purpose: Verify Trivy tables are no longer in use

\echo '========================================='
\echo 'Trivy Table Data Verification'
\echo '========================================='
\echo ''

-- Check for recent data in Trivy tables (last 30 days)
\echo 'Checking for recent data in Trivy tables...'
\echo ''

\echo '1. CVEs table (Trivy-based):'
SELECT
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '30 days') as recent_records,
    MAX(created_at) as last_insert
FROM cves;

\echo ''
\echo '2. Package Vulnerabilities table:'
SELECT
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '30 days') as recent_records,
    MAX(created_at) as last_insert
FROM package_vulnerabilities;

\echo ''
\echo '3. Image Scan Results table:'
SELECT
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '30 days') as recent_records,
    MAX(created_at) as last_insert
FROM image_scan_results;

\echo ''
\echo '========================================='
\echo 'SBOM Table Data Verification'
\echo '========================================='
\echo ''

\echo '4. SBOMs table (current system):'
SELECT
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '30 days') as recent_records,
    MAX(created_at) as last_insert
FROM sboms;

\echo ''
\echo '5. CVE Matches table (current system):'
SELECT
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '30 days') as recent_records,
    MAX(created_at) as last_insert
FROM cve_matches;

\echo ''
\echo '========================================='
\echo 'Decision Matrix'
\echo '========================================='
\echo ''
\echo 'If Trivy tables have recent_records = 0:'
\echo '  -> SAFE to deprecate'
\echo ''
\echo 'If Trivy tables have recent_records > 0:'
\echo '  -> WAIT - system still writing to old tables'
\echo ''
```

**Usage:**
```bash
psql -U postgres -d fortuna -f scripts/verify-trivy-migration.sql
```

**Step 2: Create Deprecation Migration (037)**

Create file: `core/migrations/037_deprecate_trivy_tables.go`

```go
package migrations

import (
    "fmt"
    "log"
    "gorm.io/gorm"
)

// Migration037_DeprecateTrivyTables renames Trivy tables with _deprecated suffix
// This is a non-destructive operation - tables are renamed but not dropped
func Migration037_DeprecateTrivyTables(db *gorm.DB) error {
    log.Println("[Migration 037] Starting: Deprecate Trivy tables")

    // Check if tables exist and have recent data
    var cveCount int64
    if err := db.Raw(`
        SELECT COUNT(*) FROM cves
        WHERE created_at > NOW() - INTERVAL '30 days'
    `).Scan(&cveCount).Error; err == nil && cveCount > 0 {
        log.Printf("[Migration 037] ⚠️  WARNING: cves table has %d recent records", cveCount)
        return fmt.Errorf("cannot deprecate cves table - has recent data (last 30 days)")
    }

    var pkgVulnCount int64
    if err := db.Raw(`
        SELECT COUNT(*) FROM package_vulnerabilities
        WHERE created_at > NOW() - INTERVAL '30 days'
    `).Scan(&pkgVulnCount).Error; err == nil && pkgVulnCount > 0 {
        log.Printf("[Migration 037] ⚠️  WARNING: package_vulnerabilities table has %d recent records", pkgVulnCount)
        return fmt.Errorf("cannot deprecate package_vulnerabilities table - has recent data")
    }

    // Rename tables
    tables := []struct {
        oldName string
        newName string
    }{
        {"cves", "cves_deprecated"},
        {"package_vulnerabilities", "package_vulnerabilities_deprecated"},
        {"image_scan_results", "image_scan_results_deprecated"},
    }

    for _, t := range tables {
        // Check if old table exists
        var exists bool
        if err := db.Raw(`
            SELECT EXISTS (
                SELECT 1 FROM information_schema.tables
                WHERE table_name = $1
            )
        `, t.oldName).Scan(&exists).Error; err != nil {
            log.Printf("[Migration 037] ⚠️  Error checking table %s: %v", t.oldName, err)
            continue
        }

        if !exists {
            log.Printf("[Migration 037] ℹ️  Table %s does not exist, skipping", t.oldName)
            continue
        }

        // Rename table
        log.Printf("[Migration 037] Renaming %s -> %s", t.oldName, t.newName)
        sql := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", t.oldName, t.newName)
        if err := db.Exec(sql).Error; err != nil {
            log.Printf("[Migration 037] ⚠️  Error renaming table %s: %v", t.oldName, err)
            return fmt.Errorf("failed to rename %s: %w", t.oldName, err)
        }

        log.Printf("[Migration 037] ✅ Renamed %s -> %s", t.oldName, t.newName)
    }

    log.Println("[Migration 037] ✅ Completed successfully")
    log.Println("[Migration 037] ℹ️  Deprecated tables can be backed up and dropped in next migration")
    return nil
}
```

**Step 3: Create Table Drop Migration (038)**

Create file: `core/migrations/038_drop_deprecated_tables.go`

```go
package migrations

import (
    "fmt"
    "log"
    "os"
    "time"
    "gorm.io/gorm"
)

// Migration038_DropDeprecatedTables drops Trivy tables after backup
// IMPORTANT: This migration requires manual backup before execution
func Migration038_DropDeprecatedTables(db *gorm.DB) error {
    log.Println("[Migration 038] Starting: Drop deprecated tables")

    // SAFETY CHECK: Require explicit confirmation
    confirmDrop := os.Getenv("CONFIRM_DROP_DEPRECATED_TABLES")
    if confirmDrop != "YES_DROP_TRIVY_TABLES" {
        log.Println("[Migration 038] ⚠️  SKIPPING: Requires explicit confirmation")
        log.Println("[Migration 038] ℹ️  Set CONFIRM_DROP_DEPRECATED_TABLES=YES_DROP_TRIVY_TABLES to proceed")
        log.Println("[Migration 038] ℹ️  Ensure backups are created first!")
        return nil
    }

    log.Println("[Migration 038] ⚠️  CONFIRMATION RECEIVED - proceeding with table drops")

    // Log backup reminder
    log.Println("[Migration 038] ⚠️  ========================================")
    log.Println("[Migration 038] ⚠️  REMINDER: Ensure backups exist before proceeding!")
    log.Println("[Migration 038] ⚠️  Recommended backup commands:")
    log.Println("[Migration 038] ⚠️    pg_dump -t cves_deprecated > cves_backup.sql")
    log.Println("[Migration 038] ⚠️    pg_dump -t package_vulnerabilities_deprecated > pkg_vuln_backup.sql")
    log.Println("[Migration 038] ⚠️    pg_dump -t image_scan_results_deprecated > scan_results_backup.sql")
    log.Println("[Migration 038] ⚠️  ========================================")

    // Wait 5 seconds to give operator chance to cancel
    log.Println("[Migration 038] ⏳ Waiting 5 seconds before proceeding...")
    time.Sleep(5 * time.Second)

    // Drop tables
    tables := []string{
        "cves_deprecated",
        "package_vulnerabilities_deprecated",
        "image_scan_results_deprecated",
    }

    for _, table := range tables {
        log.Printf("[Migration 038] Dropping table: %s", table)
        sql := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)
        if err := db.Exec(sql).Error; err != nil {
            return fmt.Errorf("failed to drop %s: %w", table, err)
        }
        log.Printf("[Migration 038] ✅ Dropped table: %s", table)
    }

    log.Println("[Migration 038] ✅ Completed successfully")
    log.Println("[Migration 038] ℹ️  All deprecated Trivy tables have been dropped")
    return nil
}
```

**Step 4: Add to migrations.go**

```go
// In migrations.go RunMigrations function, add:
migrations := []func(*gorm.DB) error{
    // ... existing migrations ...
    Migration036_AddMissingSBOMColumns,
    Migration037_DeprecateTrivyTables,     // Add this
    Migration038_DropDeprecatedTables,     // Add this
}
```

**Usage:**

```bash
# Step 1: Verify data migration
psql -U postgres -d fortuna -f scripts/verify-trivy-migration.sql

# Step 2: Run deprecation migration (renames tables)
# This runs automatically on next deployment

# Step 3: Create backups (manual)
pg_dump -U postgres -d fortuna -t cves_deprecated > backups/cves_backup_$(date +%Y%m%d).sql
pg_dump -U postgres -d fortuna -t package_vulnerabilities_deprecated > backups/pkg_vuln_backup_$(date +%Y%m%d).sql
pg_dump -U postgres -d fortuna -t image_scan_results_deprecated > backups/scan_results_backup_$(date +%Y%m%d).sql

# Step 4: Drop tables (requires explicit confirmation)
export CONFIRM_DROP_DEPRECATED_TABLES=YES_DROP_TRIVY_TABLES
# Restart application or run migration manually
```

---

### 6.7 Standardize Migration Documentation

**Template for Migration Headers:**

Create file: `core/migrations/MIGRATION_TEMPLATE.go`

```go
package migrations

import (
    "log"
    "gorm.io/gorm"
)

// MigrationXXX_DescriptiveName performs [brief description]
//
// Date: YYYY-MM-DD
// Author: [Your Name]
// Ticket: KSAM-XXX
//
// Description:
//   [Detailed description of what this migration does]
//
// Tables Affected:
//   - table_name: [description of changes]
//   - another_table: [description of changes]
//
// Indexes Created:
//   - idx_table_column: [description]
//
// Dependencies:
//   - Requires Migration XXX to be applied first
//   - Requires [external dependency]
//
// Rollback Plan:
//   [SQL commands to undo this migration]
//   Example:
//     DROP TABLE IF EXISTS new_table;
//     ALTER TABLE existing_table DROP COLUMN new_column;
//
// Testing:
//   - Test in local environment with: [command]
//   - Verify with: SELECT COUNT(*) FROM new_table;
//
// Notes:
//   - [Any special considerations]
//   - [Performance impact]
//   - [Breaking changes]
func MigrationXXX_DescriptiveName(db *gorm.DB) error {
    log.Println("[Migration XXX] Starting: Descriptive Name")

    // Implementation here

    log.Println("[Migration XXX] ✅ Completed successfully")
    return nil
}
```

**Update Existing Migrations:**

Example for Migration 030:

```go
// Migration030_MigrateInsightsSchemaComplete migrates insights table to new schema
//
// Date: 2025-12-XX (original), Updated: 2025-12-27 (documentation)
// Author: KSAM Team
// Ticket: KSAM-XXX
//
// Description:
//   Consolidates insights table schema migration, combining functionality from
//   old migrations 030, 031, and 038. Migrates from old JSONB-based schema to
//   new relational schema with direct resource references.
//
// Schema Changes:
//   OLD Schema: type, affected_resources (JSONB), sbom_id, cve_match_id, etc.
//   NEW Schema: insight_type, resource_type, resource_uid, resource_namespace, etc.
//
// Tables Affected:
//   - insights: Complete schema overhaul (add new columns, migrate data, drop old columns)
//
// Consolidation History:
//   This migration replaces:
//   - Old Migration 030: Initial schema migration
//   - Old Migration 031: Cleanup old columns
//   - Old Migration 038: Remove deprecated fields
//
// Rollback Plan:
//   WARNING: This migration is destructive (drops old columns)
//   Rollback requires restore from backup:
//     pg_restore -t insights insights_backup_before_migration030.sql
//
// Testing:
//   - Verify column migration: SELECT insight_type, resource_type FROM insights LIMIT 10;
//   - Check old columns dropped: \d insights
//   - Verify indexes: \di idx_insights_*
//
// Notes:
//   - Data migration is automatic (copies from old columns to new)
//   - Foreign keys to sbom_id and cve_match_id are dropped
//   - Performance: Migration may take several minutes on large datasets
func Migration030_MigrateInsightsSchemaComplete(db *gorm.DB) error {
    // ... existing code ...
}
```

---

### 6.8 Add CI Migration Validation

**Create validation script:**

File: `scripts/validate-migrations.sh`

```bash
#!/bin/bash
# File: scripts/validate-migrations.sh
# Purpose: Validate migration integrity in CI/CD pipeline

set -e

MIGRATIONS_DIR="core/migrations"
MIGRATIONS_FILE="$MIGRATIONS_DIR/migrations.go"

echo "========================================="
echo "Migration Validation Script"
echo "========================================="

# Test 1: Check migrations.go exists
echo ""
echo "Test 1: Checking migrations.go exists..."
if [ ! -f "$MIGRATIONS_FILE" ]; then
    echo "❌ FAIL: migrations.go not found"
    exit 1
fi
echo "✅ PASS: migrations.go exists"

# Test 2: Check for AutoMigrate in production code
echo ""
echo "Test 2: Checking for forbidden AutoMigrate usage..."
AUTOMIGRATE_COUNT=$(grep -r "db.AutoMigrate" "$MIGRATIONS_DIR"/*.go | grep -v "_test.go" | grep -v "// " | wc -l | tr -d ' ')
if [ "$AUTOMIGRATE_COUNT" -gt 0 ]; then
    echo "⚠️  WARNING: Found $AUTOMIGRATE_COUNT AutoMigrate calls in migrations"
    echo "    This will be enforced as an error after Phase 2 cleanup"
    grep -n "db.AutoMigrate" "$MIGRATIONS_DIR"/*.go | grep -v "_test.go" | grep -v "// "
else
    echo "✅ PASS: No AutoMigrate in production migrations"
fi

# Test 3: Check SQL file references
echo ""
echo "Test 3: Checking SQL file references..."
MISSING_FILES=0
while IFS= read -r line; do
    if [[ $line =~ \"([^\"]+\.sql)\" ]]; then
        SQL_FILE="${BASH_REMATCH[1]}"
        # Check if file exists (try multiple paths)
        if [ ! -f "$MIGRATIONS_DIR/$SQL_FILE" ] && [ ! -f "$SQL_FILE" ]; then
            echo "❌ Missing SQL file: $SQL_FILE"
            MISSING_FILES=$((MISSING_FILES + 1))
        fi
    fi
done < "$MIGRATIONS_FILE"

if [ $MISSING_FILES -gt 0 ]; then
    echo "❌ FAIL: $MISSING_FILES SQL file(s) not found"
    exit 1
else
    echo "✅ PASS: All SQL files exist"
fi

# Test 4: Check migration numbering sequence
echo ""
echo "Test 4: Checking migration numbering sequence..."
MIGRATION_NUMBERS=$(grep -o "Migration[0-9]\{3\}_" "$MIGRATIONS_FILE" | grep -o "[0-9]\{3\}" | sort -n)
EXPECTED=1
GAPS=0
for NUM in $MIGRATION_NUMBERS; do
    NUM_INT=$((10#$NUM))  # Convert to decimal (remove leading zeros)
    if [ $NUM_INT -ne $EXPECTED ]; then
        echo "⚠️  WARNING: Gap in numbering: expected $EXPECTED, found $NUM_INT"
        GAPS=$((GAPS + 1))
    fi
    EXPECTED=$((NUM_INT + 1))
done

if [ $GAPS -gt 0 ]; then
    echo "⚠️  WARNING: Found $GAPS gap(s) in migration numbering"
    echo "    This is acceptable if migrations were consolidated"
else
    echo "✅ PASS: Migration numbering is sequential"
fi

# Test 5: Check for duplicate migration numbers
echo ""
echo "Test 5: Checking for duplicate migration numbers..."
DUPLICATES=$(echo "$MIGRATION_NUMBERS" | sort | uniq -d)
if [ -n "$DUPLICATES" ]; then
    echo "❌ FAIL: Duplicate migration numbers found:"
    echo "$DUPLICATES"
    exit 1
else
    echo "✅ PASS: No duplicate migration numbers"
fi

# Test 6: Validate Go syntax
echo ""
echo "Test 6: Validating Go syntax..."
if ! go vet "$MIGRATIONS_DIR"/*.go 2>&1 | grep -v "no Go files"; then
    echo "❌ FAIL: Go syntax errors found"
    exit 1
fi
echo "✅ PASS: Go syntax is valid"

echo ""
echo "========================================="
echo "Validation completed successfully!"
echo "========================================="
```

**Add to CI/CD Pipeline:**

File: `.github/workflows/migration-validation.yml` (GitHub Actions example)

```yaml
name: Migration Validation

on:
  pull_request:
    paths:
      - 'core/migrations/**'
  push:
    branches:
      - main
      - develop

jobs:
  validate-migrations:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Make validation script executable
        run: chmod +x scripts/validate-migrations.sh

      - name: Run migration validation
        run: ./scripts/validate-migrations.sh

      - name: Check for orphaned SQL files
        run: |
          echo "Checking for SQL files not referenced in migrations.go..."
          cd core/migrations
          for sql_file in *.sql mvp2/*.sql; do
            if ! grep -q "$(basename $sql_file)" migrations.go; then
              echo "⚠️  WARNING: $sql_file not referenced in migrations.go"
            fi
          done
```

---

### 6.9 Convert Pure AutoMigrate Migrations to SQL

**Migration 002: Add Users**

Create file: `core/migrations/002_add_users.sql`

```sql
-- Migration 002: Add Users Table
-- Date: 2025-12-27
-- Description: Creates users table for authentication

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- Add comment
COMMENT ON TABLE users IS 'Application users for authentication and authorization';
```

**Update Migration 002 function:**

```go
func Migration002_AddUsers(db *gorm.DB) error {
    log.Println("Running migration 002: Add users table")

    // Read SQL from file
    sqlPaths := []string{
        "migrations/002_add_users.sql",
        "/app/migrations/002_add_users.sql",
        "./migrations/002_add_users.sql",
    }

    var sqlBytes []byte
    var err error
    for _, path := range sqlPaths {
        sqlBytes, err = os.ReadFile(path)
        if err == nil {
            break
        }
    }

    // Production: SQL file is mandatory
    env := os.Getenv("ENVIRONMENT")
    if env == "production" || env == "staging" {
        if err != nil {
            return fmt.Errorf("SQL migration file required: 002_add_users.sql not found")
        }
        return db.Exec(string(sqlBytes)).Error
    }

    // Development: Allow AutoMigrate fallback
    if err != nil {
        log.Println("Development: SQL file not found, using AutoMigrate")
        return db.AutoMigrate(&models.User{})
    }

    return db.Exec(string(sqlBytes)).Error
}
```

---

**Migration 003: Add User to Audit Logs**

Create file: `core/migrations/003_add_user_to_audit_logs.sql`

```sql
-- Migration 003: Add user_id column to audit_logs
-- Date: 2025-12-27
-- Description: Links audit logs to users table

-- Add user_id column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_logs' AND column_name = 'user_id'
    ) THEN
        ALTER TABLE audit_logs ADD COLUMN user_id INTEGER;

        -- Add foreign key constraint
        ALTER TABLE audit_logs
        ADD CONSTRAINT fk_audit_logs_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

        -- Add index
        CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
    END IF;
END $$;
```

**Update Migration 003 function:**

```go
func Migration003_AddUserToAuditLogs(db *gorm.DB) error {
    log.Println("Running migration 003: Add user_id to audit_logs")

    // Read SQL from file
    sqlPaths := []string{
        "migrations/003_add_user_to_audit_logs.sql",
        "/app/migrations/003_add_user_to_audit_logs.sql",
        "./migrations/003_add_user_to_audit_logs.sql",
    }

    var sqlBytes []byte
    var err error
    for _, path := range sqlPaths {
        sqlBytes, err = os.ReadFile(path)
        if err == nil {
            break
        }
    }

    // Production: SQL file is mandatory
    env := os.Getenv("ENVIRONMENT")
    if env == "production" || env == "staging" {
        if err != nil {
            return fmt.Errorf("SQL migration file required: 003_add_user_to_audit_logs.sql not found")
        }
        return db.Exec(string(sqlBytes)).Error
    }

    // Development: Allow AutoMigrate fallback
    if err != nil {
        log.Println("Development: SQL file not found, using AutoMigrate")
        if !db.Migrator().HasColumn(&models.AuditLog{}, "user_id") {
            if err := db.Migrator().AddColumn(&models.AuditLog{}, "user_id"); err != nil {
                return err
            }
        }
        return nil
    }

    return db.Exec(string(sqlBytes)).Error
}
```

---

**Apply same pattern to:**
- Migration 015: Policy Instances
- Migration 016: Policy Violations

---

### 6.10 Add Dry-Run Mode

**Add dry-run capability to migration runner:**

File: `core/migrations/migrations.go`

```go
// Add new function
func RunMigrationsDryRun(db *gorm.DB) error {
    log.Println("========================================")
    log.Println("DRY RUN MODE: Migrations will be rolled back")
    log.Println("========================================")

    // Start transaction
    tx := db.Begin()
    if tx.Error != nil {
        return fmt.Errorf("failed to start transaction: %w", tx.Error)
    }

    // Run migrations in transaction
    err := runMigrationsInternal(tx)

    // Always rollback
    tx.Rollback()

    if err != nil {
        log.Println("========================================")
        log.Println("DRY RUN FAILED: Migrations have errors")
        log.Println("========================================")
        return fmt.Errorf("dry run failed: %w", err)
    }

    log.Println("========================================")
    log.Println("DRY RUN SUCCEEDED: All migrations valid")
    log.Println("(Changes were rolled back)")
    log.Println("========================================")
    return nil
}

// Refactor existing RunMigrations to use internal function
func RunMigrations(db *gorm.DB) error {
    return runMigrationsInternal(db)
}

// Internal function that can be called with transaction or regular DB
func runMigrationsInternal(db *gorm.DB) error {
    log.Println("Running database migrations...")

    // ... existing migration logic ...

    return nil
}
```

**Add CLI command for dry-run:**

Create file: `core/cmd/migrate-dry-run/main.go`

```go
package main

import (
    "log"
    "os"

    "github.com/fortuna/core/internal/config"
    "github.com/fortuna/core/internal/storage"
    "github.com/fortuna/core/migrations"
)

func main() {
    log.Println("Migration Dry-Run Tool")

    // Load config
    cfg := config.Load()

    // Initialize database connection
    db, err := storage.NewPostgresDB(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }

    // Run dry-run
    if err := migrations.RunMigrationsDryRun(db); err != nil {
        log.Printf("Dry-run failed: %v", err)
        os.Exit(1)
    }

    log.Println("Dry-run completed successfully!")
    os.Exit(0)
}
```

**Usage:**

```bash
# Dry-run migrations before deploying
go run core/cmd/migrate-dry-run/main.go

# Or add to Makefile
make migrate-dry-run
```

---

### 6.11 Implement Version Tracking

**Already covered in Section 6.2** - See the `ensureVersionTable()` and related functions.

---

### 6.12 Create Down Migrations

**Template for Down migrations:**

```go
// Example: Migration 030 with Down function

func Migration030_MigrateInsightsSchemaComplete_Up(db *gorm.DB) error {
    // ... existing up logic ...
}

func Migration030_MigrateInsightsSchemaComplete_Down(db *gorm.DB) error {
    log.Println("[Migration 030] ROLLBACK: Reverting insights schema migration")

    // Step 1: Re-add old columns
    oldColumns := []struct {
        name    string
        sqlType string
    }{
        {"type", "VARCHAR(50)"},
        {"affected_resources", "JSONB"},
        {"recommended_action", "TEXT"},
        {"sbom_id", "INTEGER"},
        {"cve_match_id", "INTEGER"},
        {"source", "VARCHAR(50)"},
        {"cvss_score", "DECIMAL(3,1)"},
        {"cvss_vector", "TEXT"},
        {"exploit_available", "BOOLEAN"},
        {"package_name", "VARCHAR(255)"},
        {"installed_version", "VARCHAR(50)"},
        {"fixed_version", "VARCHAR(50)"},
    }

    for _, col := range oldColumns {
        sql := fmt.Sprintf("ALTER TABLE insights ADD COLUMN IF NOT EXISTS %s %s", col.name, col.sqlType)
        if err := db.Exec(sql).Error; err != nil {
            log.Printf("[Migration 030 Down] ⚠️  Error adding column %s: %v", col.name, err)
        }
    }

    // Step 2: Migrate data back from new columns to old columns
    if err := db.Exec(`
        UPDATE insights
        SET type = insight_type,
            recommended_action = recommendation,
            cvss_score = cvss,
            package_name = affected_component,
            installed_version = affected_version
    `).Error; err != nil {
        return fmt.Errorf("failed to migrate data back: %w", err)
    }

    // Step 3: Drop new columns
    newColumnsToDelete := []string{
        "insight_type", "resource_type", "resource_uid", "resource_namespace",
        "resource_name", "title", "recommendation", "affected_component",
        "affected_version", "cvss", "detected_at",
    }

    for _, col := range newColumnsToDelete {
        sql := fmt.Sprintf("ALTER TABLE insights DROP COLUMN IF EXISTS %s CASCADE", col)
        if err := db.Exec(sql).Error; err != nil {
            log.Printf("[Migration 030 Down] ⚠️  Error dropping column %s: %v", col, err)
        }
    }

    log.Println("[Migration 030] ✅ Rollback completed")
    return nil
}
```

**Note:** Creating Down migrations for all 36 migrations is a significant effort. Prioritize:
1. Recent migrations (030-036)
2. Data-destructive migrations
3. Complex schema changes

---

## 7. Testing Strategy

### 7.1 Local Testing (Development Environment)

**Test Environment Setup:**

```bash
# 1. Create test database
createdb fortuna_test

# 2. Set environment variables
export ENVIRONMENT=development
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/fortuna_test?sslmode=disable

# 3. Run migrations
cd core
go run cmd/main.go
```

**Test Cases:**

#### Test 1: Fresh Installation

```bash
# Ensure clean state
dropdb fortuna_test
createdb fortuna_test

# Run application (migrations run automatically)
go run core/cmd/main.go

# Verify tables created
psql fortuna_test -c "\dt"

# Expected: All 25+ tables should exist
```

---

#### Test 2: Re-run Migrations (Idempotency)

```bash
# Run migrations twice
go run core/cmd/main.go
# Stop and restart
go run core/cmd/main.go

# Verify: No errors, no duplicate tables
psql fortuna_test -c "SELECT COUNT(*) FROM schema_migrations;"
# Expected: 36 rows (one per migration)
```

---

#### Test 3: Dry-Run Mode

```bash
# Run dry-run
go run core/cmd/migrate-dry-run/main.go

# Verify: No actual changes made
psql fortuna_test -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';"
# Expected: Only schema_migrations table exists (if any)
```

---

#### Test 4: Migration Failure Recovery

```bash
# Intentionally break a migration
# Edit core/migrations/030_*.go and add: return fmt.Errorf("test error")

# Run migrations
go run core/cmd/main.go
# Expected: Application fails to start, error logged

# Fix migration and re-run
# Expected: Migration continues from where it failed
```

---

### 7.2 Staging Environment Testing

**Pre-Deployment Checklist:**

```markdown
## Staging Deployment Checklist

### Pre-Deployment
- [ ] Backup production database
- [ ] Copy production data to staging (anonymized)
- [ ] Verify staging database connectivity
- [ ] Set ENVIRONMENT=staging

### Deployment
- [ ] Deploy new code to staging
- [ ] Monitor migration logs
- [ ] Verify application starts successfully
- [ ] Run smoke tests

### Post-Deployment Validation
- [ ] Verify all tables exist: `\dt` in psql
- [ ] Verify schema_migrations table: `SELECT * FROM schema_migrations ORDER BY version;`
- [ ] Check for migration errors in logs
- [ ] Verify indexes created: `\di`
- [ ] Run application health check
- [ ] Test critical API endpoints
- [ ] Verify no performance degradation

### Rollback Plan
- [ ] Document rollback procedure
- [ ] Have database backup ready
- [ ] Test rollback in isolated environment first
```

---

### 7.3 Production Deployment Testing

**Pre-Production Checklist:**

```markdown
## Production Deployment Checklist

### Pre-Deployment (T-24h)
- [ ] Staging deployment successful for 24+ hours
- [ ] No issues found in staging
- [ ] Database backup created and verified
- [ ] Rollback plan documented and tested
- [ ] Deployment window scheduled (low-traffic time)
- [ ] On-call engineer assigned

### Deployment (T-0)
- [ ] Enable read-only mode (if possible)
- [ ] Create final database backup
- [ ] Verify backup can be restored
- [ ] Deploy new version
- [ ] Monitor migration progress (tail logs)
- [ ] Verify migrations complete successfully

### Post-Deployment Validation (T+15min)
- [ ] Application health check passes
- [ ] Database connectivity verified
- [ ] Critical API endpoints respond
- [ ] No error spike in logs
- [ ] Monitor error rates (Sentry/New Relic)
- [ ] Monitor performance metrics
- [ ] Run automated test suite

### Post-Deployment Validation (T+1h)
- [ ] User-facing features working
- [ ] No customer complaints
- [ ] Performance within normal ranges
- [ ] Database query performance normal

### Rollback Triggers
If any of these occur, initiate rollback:
- [ ] Migrations fail with errors
- [ ] Application fails to start
- [ ] Critical API errors > 5%
- [ ] Database connection errors
- [ ] Performance degradation > 50%
- [ ] Data corruption detected
```

---

### 7.4 Performance Testing

**Migration Performance Tests:**

```sql
-- File: scripts/test-migration-performance.sql

-- Test 1: Large table migration performance
-- Create large test table
CREATE TABLE test_large_table AS
SELECT
    generate_series(1, 1000000) as id,
    md5(random()::text) as data
FROM generate_series(1, 10);

-- Test adding index (measure time)
\timing on
CREATE INDEX idx_test_large_table_data ON test_large_table(data);
\timing off

-- Expected: < 10 seconds for 1M rows

-- Test 2: JSONB index performance
CREATE TABLE test_jsonb AS
SELECT
    generate_series(1, 100000) as id,
    ('{"key": "' || md5(random()::text) || '", "value": ' || random() || '}')::jsonb as data
FROM generate_series(1, 10);

\timing on
CREATE INDEX idx_test_jsonb_gin ON test_jsonb USING GIN(data);
\timing off

-- Expected: < 5 seconds for 100k rows

-- Cleanup
DROP TABLE test_large_table;
DROP TABLE test_jsonb;
```

---

## 8. Rollback Procedures

### 8.1 Application Rollback (Without Database Changes)

**Scenario:** New application version has bugs but migrations succeeded

**Procedure:**

```bash
# 1. Rollback application to previous version
kubectl rollout undo deployment/fortuna-core

# 2. Verify previous version running
kubectl get pods -l app=fortuna-core

# 3. Monitor for errors
kubectl logs -f deployment/fortuna-core

# 4. Verify application health
curl http://fortuna-core:8080/health
```

**When to Use:**
- Migrations completed successfully
- Application code has bugs
- Database schema is compatible with old version

---

### 8.2 Database Rollback (Migration Failed Mid-Execution)

**Scenario:** Migration failed, database in inconsistent state

**Procedure:**

```bash
# 1. Immediately stop all application instances
kubectl scale deployment/fortuna-core --replicas=0

# 2. Identify failed migration
psql fortuna_production -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# 3. Restore from backup
pg_restore -U postgres -d fortuna_production \
    --clean --if-exists \
    /backups/fortuna_pre_deployment_$(date +%Y%m%d).backup

# 4. Verify database state
psql fortuna_production -c "\dt"
psql fortuna_production -c "SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# 5. Rollback application
kubectl rollout undo deployment/fortuna-core

# 6. Restart application
kubectl scale deployment/fortuna-core --replicas=3

# 7. Verify health
kubectl logs -f deployment/fortuna-core | grep "migration"
```

**When to Use:**
- Migration failed with errors
- Database in inconsistent state
- Manual fixes would be too risky

---

### 8.3 Partial Migration Rollback (Using Down Functions)

**Scenario:** Need to undo specific migrations without full restore

**Procedure:**

```bash
# 1. Stop application
kubectl scale deployment/fortuna-core --replicas=0

# 2. Connect to database
psql fortuna_production

# 3. Check current migration version
SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 1;

# 4. Manually execute Down migration
-- If using golang-migrate:
migrate -database ${DATABASE_URL} -path core/migrations down 1

-- OR manually:
-- Run SQL commands from migration Down function

# 5. Update schema_migrations table
UPDATE schema_migrations
SET rolled_back_at = CURRENT_TIMESTAMP
WHERE version = 36;

# 6. Restart application with old version
kubectl rollout undo deployment/fortuna-core
kubectl scale deployment/fortuna-core --replicas=3

# 7. Verify
kubectl logs -f deployment/fortuna-core
```

**When to Use:**
- Only recent migrations need rollback
- Down functions are implemented
- Selective rollback is safer than full restore

---

### 8.4 Emergency Rollback Decision Tree

```
Migration failed?
├─ YES
│  ├─ Database corrupted/inconsistent?
│  │  ├─ YES → Full database restore (8.2)
│  │  └─ NO → Partial rollback (8.3)
│  └─ Migration completed but application broken?
│     └─ Application rollback only (8.1)
└─ NO
   └─ No rollback needed
```

---

## 9. Timeline & Resource Allocation

### 9.1 Phase 1: Immediate Cleanup (Week 1)

| Task | Duration | Owner | Dependencies |
|------|----------|-------|--------------|
| 1.1: File Cleanup | 1 day | Backend Dev | None |
| 1.2: Schema Validation | 2 days | Backend Dev | Task 1.1 |
| 1.3: Migration Documentation | 1 day | Backend Dev | None |
| 1.4: CI Validation | 1 day | DevOps | Task 1.3 |

**Total: 5 days**

**Resources:**
- 1x Backend Developer (full-time)
- 0.5x DevOps Engineer

**Deliverables:**
- ✅ Orphaned files removed/archived
- ✅ Schema validation functions added
- ✅ Migration headers standardized
- ✅ CI pipeline validates migrations

---

### 9.2 Phase 2: Remove AutoMigrate Fallbacks (Weeks 2-4)

| Task | Duration | Owner | Dependencies |
|------|----------|-------|--------------|
| 2.1: Convert AutoMigrate to SQL | 1 week | Backend Dev | Phase 1 |
| 2.2: Remove Fallbacks | 1 week | Backend Dev | Task 2.1 |
| 2.3: Add Dry-Run Mode | 1 week | Backend Dev | Task 2.2 |
| Testing in Staging | Ongoing | QA Team | All tasks |

**Total: 3 weeks**

**Resources:**
- 1x Backend Developer (full-time)
- 1x QA Engineer (half-time)
- 0.5x DevOps Engineer

**Deliverables:**
- ✅ All migrations use SQL
- ✅ No AutoMigrate in production code
- ✅ Dry-run mode available
- ✅ Tested in staging

---

### 9.3 Phase 3: Implement Migration Tooling (Months 2-3)

| Task | Duration | Owner | Dependencies |
|------|----------|-------|--------------|
| 3.1: Evaluate Tools | 1 week | Tech Lead | Phase 2 |
| 3.2: Implement Version Tracking | 1 week | Backend Dev | Task 3.1 |
| 3.3: Create Down Migrations | 2 weeks | Backend Dev | Task 3.2 |
| 3.4: Migrate to golang-migrate | 4 weeks | Backend Dev + DevOps | Task 3.3 |
| Testing | Ongoing | QA Team | All tasks |

**Total: 8 weeks**

**Resources:**
- 1x Backend Developer (full-time)
- 1x Tech Lead (25% time)
- 1x DevOps Engineer (50% time)
- 1x QA Engineer (25% time)

**Deliverables:**
- ✅ Version tracking implemented
- ✅ Rollback capability added
- ✅ Migration tool integrated
- ✅ Production deployment successful

---

### 9.4 Phase 4: Deprecate Old Tables (Month 4+)

| Task | Duration | Owner | Dependencies |
|------|----------|-------|--------------|
| 4.1: Verify Data Migration | 1 week | Backend Dev | Phase 3 |
| 4.2: Archive and Drop Tables | 1 week | Backend Dev + DBA | Task 4.1 |

**Total: 2 weeks**

**Resources:**
- 1x Backend Developer (half-time)
- 1x DBA (consultant, if needed)

**Deliverables:**
- ✅ Trivy tables deprecated
- ✅ Data backed up
- ✅ Tables dropped in production

---

### 9.5 Overall Timeline

```
Timeline (16 weeks total):

Week 1:     [Phase 1: Cleanup] ████████
Week 2-4:   [Phase 2: Remove AutoMigrate] ████████████████████
Week 5-12:  [Phase 3: Migration Tooling] ████████████████████████████████
Week 13-14: [Phase 4: Deprecate Tables] ████████
Week 15-16: [Buffer/Testing] ████████

Milestones:
✓ Week 1:  Cleanup complete
✓ Week 4:  No AutoMigrate in production
✓ Week 12: Rollback capability ready
✓ Week 14: Old tables removed
✓ Week 16: Project complete
```

---

## 10. Long-term Recommendations

### 10.1 Migration Governance

**Establish Migration Review Process:**

```markdown
## Migration Review Checklist

Before merging any PR with migrations:

### Code Review
- [ ] Migration follows naming convention (XXX_descriptive_name)
- [ ] Migration header includes all required metadata
- [ ] SQL syntax is correct
- [ ] Idempotency checks present (IF NOT EXISTS, etc.)
- [ ] No AutoMigrate in production code
- [ ] Rollback plan documented

### Testing
- [ ] Tested in local environment
- [ ] Tested in staging environment
- [ ] Performance impact assessed (for large tables)
- [ ] Dry-run executed successfully
- [ ] No data loss verified

### Documentation
- [ ] Migration purpose clearly documented
- [ ] Breaking changes noted
- [ ] Deployment instructions updated

### Approval
- [ ] Reviewed by senior engineer
- [ ] DBA approval (for complex migrations)
- [ ] Security review (if applicable)
```

---

### 10.2 Migration Tool Evaluation

**Detailed Comparison:**

| Feature | golang-migrate | Atlas | Custom (Enhanced) |
|---------|---------------|-------|-------------------|
| **Setup Complexity** | Low | Medium | N/A (existing) |
| **Learning Curve** | Low | Medium | N/A |
| **Rollback Support** | ✅ Built-in | ✅ Built-in | ❌ Need to implement |
| **Version Tracking** | ✅ Built-in | ✅ Built-in | ✅ Can implement |
| **Dry-Run Mode** | ✅ Built-in | ✅ Built-in | ✅ Can implement |
| **GORM Integration** | ⚠️ Manual | ✅ Direct | ✅ Direct |
| **SQL Support** | ✅ Full | ✅ Full | ✅ Full |
| **Go Migrations** | ✅ Supported | ✅ Supported | ✅ Native |
| **CLI Tool** | ✅ Excellent | ✅ Good | ❌ Would need to build |
| **CI/CD Integration** | ✅ Easy | ✅ Easy | ✅ Easy |
| **Cost** | Free | Free (Open Source) | Free |
| **Community** | Large | Growing | N/A |
| **Maturity** | Very Mature | Mature | N/A |
| **PostgreSQL Support** | ✅ Excellent | ✅ Excellent | ✅ Native |
| **Migration Locking** | ✅ Built-in | ✅ Built-in | ❌ Need to implement |

**Recommendation: golang-migrate**

**Reasons:**
1. Industry standard with large community
2. Simple to integrate with existing codebase
3. Excellent documentation and examples
4. Native rollback support
5. CLI tool for manual operations
6. No vendor lock-in

---

### 10.3 golang-migrate Integration Plan

**Step 1: Install golang-migrate**

```bash
# Install CLI tool
brew install golang-migrate  # macOS
# OR
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/

# Install Go library
go get -u github.com/golang-migrate/migrate/v4
go get -u github.com/golang-migrate/migrate/v4/database/postgres
go get -u github.com/golang-migrate/migrate/v4/source/file
```

---

**Step 2: Convert Existing Migrations**

Create new migration structure:

```
core/migrations_v2/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_users.up.sql
├── 000002_add_users.down.sql
├── 000003_add_user_to_audit_logs.up.sql
├── 000003_add_user_to_audit_logs.down.sql
...
└── 000036_add_missing_sbom_columns.up.sql
    000036_add_missing_sbom_columns.down.sql
```

Example conversion:

**From:** `core/migrations/001_initial_schema.sql`

**To:** `core/migrations_v2/000001_initial_schema.up.sql`

```sql
-- Same content as original
CREATE TABLE IF NOT EXISTS clusters (
    ...
);
...
```

**New:** `core/migrations_v2/000001_initial_schema.down.sql`

```sql
-- Rollback migration
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS pods CASCADE;
DROP TABLE IF EXISTS cluster_role_bindings CASCADE;
DROP TABLE IF EXISTS role_bindings CASCADE;
DROP TABLE IF EXISTS cluster_roles CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS service_accounts CASCADE;
DROP TABLE IF EXISTS clusters CASCADE;
```

---

**Step 3: Update Application Code**

File: `core/internal/storage/storage.go`

```go
package storage

import (
    "database/sql"
    "fmt"
    "log"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func Initialize(databaseURL string) (*gorm.DB, error) {
    // Connect with database/sql first (required by golang-migrate)
    sqlDB, err := sql.Open("postgres", databaseURL)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Run migrations using golang-migrate
    if err := runMigrations(sqlDB); err != nil {
        return nil, fmt.Errorf("migrations failed: %w", err)
    }

    // Initialize GORM
    gormDB, err := gorm.Open(postgres.New(postgres.Config{
        Conn: sqlDB,
    }), &gorm.Config{})

    if err != nil {
        return nil, fmt.Errorf("failed to initialize GORM: %w", err)
    }

    return gormDB, nil
}

func runMigrations(db *sql.DB) error {
    log.Println("Running database migrations...")

    // Create migration driver
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        return fmt.Errorf("failed to create migration driver: %w", err)
    }

    // Create migrate instance
    m, err := migrate.NewWithDatabaseInstance(
        "file://migrations_v2",  // Path to migrations
        "postgres",
        driver,
    )
    if err != nil {
        return fmt.Errorf("failed to create migrate instance: %w", err)
    }

    // Run migrations
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migration failed: %w", err)
    }

    log.Println("Migrations completed successfully")
    return nil
}

// New function: Rollback migrations
func RollbackMigration(db *sql.DB, steps int) error {
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        return err
    }

    m, err := migrate.NewWithDatabaseInstance(
        "file://migrations_v2",
        "postgres",
        driver,
    )
    if err != nil {
        return err
    }

    return m.Steps(-steps)  // Negative steps = rollback
}
```

---

**Step 4: Add CLI Commands**

Create file: `core/cmd/migrate/main.go`

```go
package main

import (
    "database/sql"
    "flag"
    "log"
    "os"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    _ "github.com/lib/pq"

    "github.com/fortuna/core/internal/config"
)

func main() {
    var (
        migrationsPath = flag.String("path", "migrations_v2", "Path to migrations directory")
        direction      = flag.String("direction", "up", "Migration direction: up, down, or version")
        steps          = flag.Int("steps", 0, "Number of migrations to apply (for down)")
        version        = flag.Uint("version", 0, "Target version (for goto)")
    )
    flag.Parse()

    // Load config
    cfg := config.Load()

    // Connect to database
    db, err := sql.Open("postgres", cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    // Create migration instance
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        log.Fatalf("Failed to create driver: %v", err)
    }

    m, err := migrate.NewWithDatabaseInstance(
        "file://"+*migrationsPath,
        "postgres",
        driver,
    )
    if err != nil {
        log.Fatalf("Failed to create migrate instance: %v", err)
    }

    // Execute migration
    switch *direction {
    case "up":
        log.Println("Running migrations up...")
        if err := m.Up(); err != nil && err != migrate.ErrNoChange {
            log.Fatalf("Migration failed: %v", err)
        }
        log.Println("Migrations completed successfully")

    case "down":
        if *steps == 0 {
            log.Fatalf("Must specify -steps for down migration")
        }
        log.Printf("Rolling back %d migration(s)...", *steps)
        if err := m.Steps(-*steps); err != nil {
            log.Fatalf("Rollback failed: %v", err)
        }
        log.Println("Rollback completed successfully")

    case "version":
        if *version == 0 {
            log.Fatalf("Must specify -version for goto")
        }
        log.Printf("Migrating to version %d...", *version)
        if err := m.Migrate(*version); err != nil {
            log.Fatalf("Migration failed: %v", err)
        }
        log.Println("Migration completed successfully")

    default:
        log.Fatalf("Invalid direction: %s (use up, down, or version)", *direction)
    }

    // Print current version
    version, dirty, err := m.Version()
    if err != nil && err != migrate.ErrNilVersion {
        log.Fatalf("Failed to get version: %v", err)
    }
    log.Printf("Current version: %d (dirty: %v)", version, dirty)
}
```

**Usage:**

```bash
# Run all pending migrations
go run core/cmd/migrate/main.go -direction=up

# Rollback last migration
go run core/cmd/migrate/main.go -direction=down -steps=1

# Migrate to specific version
go run core/cmd/migrate/main.go -direction=version -version=30

# Or use CLI tool directly
migrate -database postgres://user:pass@localhost/fortuna -path core/migrations_v2 up
migrate -database postgres://user:pass@localhost/fortuna -path core/migrations_v2 down 1
```

---

**Step 5: Update Deployment**

Update Dockerfile:

```dockerfile
# Add golang-migrate CLI to container
FROM golang:1.24-alpine AS builder

# Install migrate CLI
RUN apk add --no-cache curl
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz && \
    mv migrate /usr/local/bin/migrate

# ... rest of Dockerfile ...

# Final stage
FROM alpine:latest

COPY --from=builder /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/migrations_v2 /app/migrations_v2
COPY --from=builder /app/fortuna /app/fortuna

CMD ["/app/fortuna"]
```

Update deployment script:

```bash
#!/bin/bash
# scripts/deploy.sh

# Run migrations before starting application
migrate -database $DATABASE_URL -path /app/migrations_v2 up

# Start application
exec /app/fortuna
```

---

### 10.4 Monitoring & Alerting

**Add Migration Monitoring:**

```go
// File: core/internal/metrics/migrations.go

package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    MigrationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "migration_duration_seconds",
            Help: "Duration of database migrations",
            Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120},
        },
        []string{"version", "status"},
    )

    MigrationStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "migration_status",
            Help: "Status of last migration (1=success, 0=failure)",
        },
        []string{"version"},
    )

    CurrentMigrationVersion = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "current_migration_version",
            Help: "Current database migration version",
        },
    )
)
```

**Add to migration runner:**

```go
import (
    "time"
    "github.com/fortuna/core/internal/metrics"
)

func runMigration(db *gorm.DB, version int, migrationFunc func(*gorm.DB) error) error {
    start := time.Now()

    err := migrationFunc(db)
    duration := time.Since(start).Seconds()

    status := "success"
    if err != nil {
        status = "failure"
    }

    metrics.MigrationDuration.WithLabelValues(
        fmt.Sprintf("%03d", version),
        status,
    ).Observe(duration)

    if err == nil {
        metrics.CurrentMigrationVersion.Set(float64(version))
        metrics.MigrationStatus.WithLabelValues(fmt.Sprintf("%03d", version)).Set(1)
    } else {
        metrics.MigrationStatus.WithLabelValues(fmt.Sprintf("%03d", version)).Set(0)
    }

    return err
}
```

**Add Alerting Rules:**

```yaml
# File: deploy/monitoring/alerts/migrations.yaml

groups:
  - name: database_migrations
    interval: 30s
    rules:
      - alert: MigrationFailed
        expr: migration_status == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Database migration failed"
          description: "Migration {{ $labels.version }} failed. Application may not start."

      - alert: MigrationTooSlow
        expr: migration_duration_seconds > 300
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Database migration taking too long"
          description: "Migration {{ $labels.version }} took {{ $value }}s (>5min threshold)."

      - alert: MigrationVersionMismatch
        expr: |
          current_migration_version != on() group_left
          max(current_migration_version)
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database migration version mismatch between instances"
          description: "Not all application instances are on the same migration version."
```

---

### 10.5 Best Practices Documentation

Create file: `docs/database-migration-best-practices.md`

```markdown
# Database Migration Best Practices

## General Principles

1. **Always backup before migrations**
   - Automated backups before every production deployment
   - Test restore procedure quarterly

2. **Test migrations thoroughly**
   - Local environment (developer machine)
   - Staging environment (production-like data)
   - Dry-run mode before production

3. **Make migrations reversible**
   - Every .up.sql must have corresponding .down.sql
   - Test rollback procedure in staging

4. **Keep migrations small and focused**
   - One logical change per migration
   - Easier to debug and rollback

5. **Never modify old migrations**
   - Once deployed to production, migrations are immutable
   - Create new migration to fix issues

## Forbidden Practices

### ❌ Never Use AutoMigrate in Production

```go
// FORBIDDEN in production code
db.AutoMigrate(&models.User{})
```

### ❌ Never Modify Deployed Migrations

```go
// If Migration030 is already in production, don't change it
// Create Migration037 instead
```

### ❌ Never Use Non-Idempotent DDL

```sql
-- FORBIDDEN
ALTER TABLE users ADD COLUMN email VARCHAR(255);  -- Fails on re-run

-- CORRECT
ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255);
```

### ❌ Never Drop Columns Without Data Migration

```sql
-- FORBIDDEN (data loss!)
ALTER TABLE users DROP COLUMN old_email;

-- CORRECT (multi-step process)
-- Migration 040: Add new column, migrate data
-- Migration 041: (1 week later) Drop old column after verification
```

## Common Patterns

### Pattern 1: Adding a Column

```sql
-- Migration XXX: Add email column to users
ALTER TABLE users
ADD COLUMN IF NOT EXISTS email VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
```

### Pattern 2: Renaming a Column (Multi-Step)

```sql
-- Migration XXX: Add new column
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_address VARCHAR(255);

-- Copy data
UPDATE users SET email_address = email WHERE email_address IS NULL;

-- Migration XXX+1: (Deploy after data migration verified)
-- Drop old column
ALTER TABLE users DROP COLUMN IF EXISTS email CASCADE;
```

### Pattern 3: Adding Foreign Key

```sql
-- Migration XXX: Add foreign key
ALTER TABLE orders
ADD CONSTRAINT fk_orders_user
FOREIGN KEY (user_id) REFERENCES users(id)
ON DELETE CASCADE;
```

### Pattern 4: Creating Index Concurrently (Large Tables)

```sql
-- For large tables, use CONCURRENTLY to avoid blocking
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at
ON users(created_at);
```

## Deployment Checklist

- [ ] Migration tested in local environment
- [ ] Migration tested in staging
- [ ] Rollback tested in staging
- [ ] Performance impact assessed
- [ ] Backup verified
- [ ] Deployment window scheduled
- [ ] Rollback plan documented
- [ ] Monitoring alerts configured
```

---

## Appendix A: Quick Reference

### Migration File Naming Convention

```
Format: NNN_descriptive_name.{up|down}.sql

Examples:
  037_add_user_preferences.up.sql
  037_add_user_preferences.down.sql
  038_drop_deprecated_tables.up.sql
  038_drop_deprecated_tables.down.sql
```

### Migration Function Naming Convention

```go
Format: MigrationNNN_DescriptiveName

Examples:
  func Migration037_AddUserPreferences(db *gorm.DB) error { ... }
  func Migration038_DropDeprecatedTables(db *gorm.DB) error { ... }
```

### Common SQL Patterns

```sql
-- Table Creation
CREATE TABLE IF NOT EXISTS table_name (...);

-- Column Addition
ALTER TABLE table_name ADD COLUMN IF NOT EXISTS column_name TYPE;

-- Index Creation
CREATE INDEX IF NOT EXISTS idx_table_column ON table_name(column);

-- Constraint Addition
ALTER TABLE table_name
ADD CONSTRAINT constraint_name
FOREIGN KEY (column) REFERENCES other_table(id);

-- Idempotent Check
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'table' AND column_name = 'column') THEN
        ALTER TABLE table ADD COLUMN column TYPE;
    END IF;
END $$;
```

---

## Appendix B: Troubleshooting Guide

### Problem: Migration Fails with "insufficient arguments"

**Cause:** Known GORM/PostgreSQL compatibility issue

**Solution:**
- Check if tables were actually created
- Add explicit schema validation
- Use SQL migrations instead of AutoMigrate

---

### Problem: Migration Succeeds but Table Not Created

**Cause:** Silent error handling or AutoMigrate failure

**Solution:**
1. Check migration logs for warnings
2. Query database directly: `\dt table_name`
3. Run migration in transaction to catch all errors

---

### Problem: "relation already exists" Error

**Cause:** Migration not idempotent

**Solution:**
- Add `IF NOT EXISTS` clauses
- Check migration has already run

---

### Problem: Migration Too Slow

**Cause:** Large table index creation or data migration

**Solution:**
- Use `CREATE INDEX CONCURRENTLY` for large tables
- Break data migration into batches
- Schedule during low-traffic window

---

## Appendix C: Useful SQL Queries

```sql
-- Check current migration version
SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 1;

-- List all migrations
SELECT version, name, applied_at FROM schema_migrations ORDER BY version;

-- Check for failed migrations
SELECT * FROM schema_migrations WHERE rolled_back_at IS NOT NULL;

-- Verify table exists
SELECT EXISTS (
    SELECT FROM information_schema.tables
    WHERE table_name = 'users'
);

-- List all indexes on a table
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'users';

-- Check table size
SELECT pg_size_pretty(pg_total_relation_size('users'));

-- Find tables without primary key
SELECT table_name
FROM information_schema.tables t
WHERE table_schema = 'public'
  AND table_type = 'BASE TABLE'
  AND NOT EXISTS (
      SELECT 1 FROM information_schema.table_constraints
      WHERE constraint_type = 'PRIMARY KEY'
        AND table_name = t.table_name
  );
```

---

## Appendix D: Contact Information

**Questions or Issues:**
- Backend Team: #backend-team Slack channel
- Database Team: #database-team Slack channel
- On-call Engineer: Check PagerDuty

**Escalation:**
- L1: Team Lead
- L2: Engineering Manager
- L3: CTO

---

**END OF REPORT**

---

*This report was generated on December 27, 2025 as part of the KSAM database migration audit.*

*For updates or questions, contact the Backend Engineering Team.*
