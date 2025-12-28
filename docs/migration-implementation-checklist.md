# Migration System Overhaul - Implementation Checklist

**Project:** KSAM Database Migration System Improvement
**Start Date:** Week of 2025-12-30
**Target Completion:** 2026-04-15 (16 weeks)
**Owner:** Backend Engineering Team

---

## Executive Summary

### Current State
- **36 active migrations** managing PostgreSQL schema
- **15 migrations** rely on AutoMigrate fallbacks (production risk)
- **No rollback capability** (cannot undo failed migrations)
- **Silent error handling** masks migration failures
- **Grade: C+** (functional but needs hardening)

### Target State
- **100% SQL-based migrations** (no AutoMigrate in production)
- **Full rollback capability** for all migrations
- **Professional migration tool** (golang-migrate)
- **CI/CD validation** prevents migration issues
- **Grade: A** (production-grade reliability)

### Business Impact
- **Reduced deployment risk** (can rollback failed migrations)
- **Faster incident recovery** (no manual database fixes)
- **Increased developer confidence** (migrations are tested and reversible)
- **Better uptime** (fewer schema-related incidents)

---

## Phase 1: Immediate Cleanup (Week 1) 🟢 LOW RISK

**Objective:** Remove dead code and fix critical bugs

### ✅ Task 1.1: File Cleanup (Day 1)
**Owner:** Backend Developer
**Effort:** 4 hours

**Actions:**
- [ ] Determine which AGE trigger variant is used (or if AGE is used at all)
- [ ] Run `scripts/cleanup-orphaned-migrations.sh`
- [ ] Archive MVP3 schema file
- [ ] Delete untracked migration files (038, 039)
- [ ] Commit changes with message: `chore: archive orphaned migration files`

**Deliverable:** Clean migration directory, no orphaned files

**Verification:**
```bash
ls core/migrations/*.sql | wc -l  # Should be reduced
ls core/migrations/archive/       # Should contain archived files
```

---

### ✅ Task 1.2: Add Schema Validation (Days 2-3)
**Owner:** Backend Developer
**Effort:** 12 hours

**Actions:**
- [ ] Add `validateMigrationResult()` helper function
- [ ] Update Migration 001 error handling (remove `continue` on errors)
- [ ] Add explicit table validation after Migration 001
- [ ] Test in local environment
- [ ] Test with intentionally broken migration
- [ ] Commit changes with message: `fix: add explicit schema validation to migrations`

**Code Changes:**
- File: `core/migrations/migrations.go`
- Lines: 98-114, add new validation function

**Deliverable:** Migrations fail loudly instead of silently

**Verification:**
```bash
# Test failure case
# Temporarily remove SQL file, run migration
# Expected: Clear error message, application fails to start

# Test success case
# Restore SQL file, run migration
# Expected: All tables created, validation passes
```

---

### ✅ Task 1.3: Standardize Documentation (Day 4)
**Owner:** Backend Developer
**Effort:** 6 hours

**Actions:**
- [ ] Create migration header template (`MIGRATION_TEMPLATE.go`)
- [ ] Update migrations 030-032 headers (consolidation context)
- [ ] Update migrations 034-036 headers
- [ ] Document rollback plans in comments
- [ ] Update migration README
- [ ] Commit changes with message: `docs: standardize migration headers`

**Deliverable:** All migrations have consistent documentation

**Verification:**
```bash
grep -n "Rollback Plan:" core/migrations/*.go | wc -l
# Should match number of migrations
```

---

### ✅ Task 1.4: Add CI Validation (Day 5)
**Owner:** DevOps Engineer
**Effort:** 4 hours

**Actions:**
- [ ] Create `scripts/validate-migrations.sh`
- [ ] Add GitHub Actions workflow (`.github/workflows/migration-validation.yml`)
- [ ] Test CI pipeline locally
- [ ] Create test PR to verify CI works
- [ ] Commit changes with message: `ci: add migration validation pipeline`

**Deliverable:** CI pipeline validates migrations on every PR

**Verification:**
- Create test PR with intentional migration error
- CI should fail with clear error message

---

## Phase 2: Remove AutoMigrate Fallbacks (Weeks 2-4) 🟡 MEDIUM RISK

**Objective:** Eliminate AutoMigrate from production code

### ✅ Task 2.1: Convert AutoMigrate to SQL (Week 2)
**Owner:** Backend Developer
**Effort:** 40 hours

**Actions:**
- [ ] Create `002_add_users.sql` from User model
- [ ] Create `003_add_user_to_audit_logs.sql`
- [ ] Create `015_add_policy_instances.sql`
- [ ] Create `016_add_policy_violations.sql`
- [ ] Update migration functions to use SQL files
- [ ] Add environment checks (production vs development)
- [ ] Test all converted migrations in local
- [ ] Test in staging environment
- [ ] Commit changes with message: `refactor: convert AutoMigrate migrations to SQL`

**Deliverable:** 4 new SQL files, updated migration functions

**Verification:**
```bash
# In staging
dropdb fortuna_staging_test
createdb fortuna_staging_test
# Deploy and verify all tables created
psql fortuna_staging_test -c "\dt"
```

---

### ✅ Task 2.2: Remove AutoMigrate Fallbacks (Week 3)
**Owner:** Backend Developer
**Effort:** 30 hours

**Actions:**
- [ ] Update Migration 001 (remove AutoMigrate fallback)
- [ ] Update Migrations 008-011 (remove fallbacks)
- [ ] Update Migrations 012-014 (remove fallbacks)
- [ ] Update Migration 018 (remove fallback)
- [ ] Update Migration 020 (remove fallback)
- [ ] Add `ENVIRONMENT` variable checks
- [ ] Update deployment docs
- [ ] Test in local (both production and development mode)
- [ ] Test in staging
- [ ] Commit changes with message: `refactor: remove AutoMigrate fallbacks from production`

**Deliverable:** Production code has no AutoMigrate fallbacks

**Verification:**
```bash
# Should return 0 (no AutoMigrate in production code)
grep -r "db.AutoMigrate" core/migrations/*.go | grep -v "_test.go" | grep -v "ENVIRONMENT.*development" | wc -l

# Test production mode
ENVIRONMENT=production go run core/cmd/main.go
# Should require SQL files, fail if missing

# Test development mode
ENVIRONMENT=development go run core/cmd/main.go
# Can use AutoMigrate if SQL files missing
```

---

### ✅ Task 2.3: Add Dry-Run Mode (Week 4)
**Owner:** Backend Developer
**Effort:** 20 hours

**Actions:**
- [ ] Add `RunMigrationsDryRun()` function
- [ ] Refactor `RunMigrations()` to use internal function
- [ ] Create `core/cmd/migrate-dry-run/main.go`
- [ ] Add dry-run test to CI pipeline
- [ ] Test dry-run in staging
- [ ] Document dry-run usage
- [ ] Commit changes with message: `feat: add migration dry-run mode`

**Deliverable:** Dry-run command available for testing

**Verification:**
```bash
go run core/cmd/migrate-dry-run/main.go
# Should run migrations in transaction, then rollback
# No actual changes to database
```

---

### ✅ Task 2.4: Staging Validation (Ongoing)
**Owner:** QA Engineer
**Effort:** 10 hours/week

**Actions:**
- [ ] Deploy Phase 2 changes to staging
- [ ] Run smoke tests
- [ ] Verify all features work
- [ ] Load test (if applicable)
- [ ] Document any issues found
- [ ] Verify rollback procedure works

**Deliverable:** Staging sign-off for Phase 2

---

## Phase 3: Implement Migration Tooling (Weeks 5-12) 🟡 MEDIUM-HIGH RISK

**Objective:** Add rollback capability and professional tooling

### ✅ Task 3.1: Evaluate Migration Tools (Week 5)
**Owner:** Tech Lead + Backend Developer
**Effort:** 30 hours

**Actions:**
- [ ] Prototype with golang-migrate
- [ ] Prototype with Atlas
- [ ] Document pros/cons
- [ ] Make recommendation
- [ ] Get stakeholder approval
- [ ] Document decision

**Deliverable:** Tool selection decision document

---

### ✅ Task 3.2: Implement Version Tracking (Week 6)
**Owner:** Backend Developer
**Effort:** 30 hours

**Actions:**
- [ ] Create `schema_migrations` table
- [ ] Add `ensureVersionTable()` function
- [ ] Add `isMigrationApplied()` function
- [ ] Add `recordMigration()` function
- [ ] Update `RunMigrations()` to use version tracking
- [ ] Test in local environment
- [ ] Test in staging
- [ ] Commit changes with message: `feat: add migration version tracking`

**Deliverable:** Migration versions tracked in database

**Verification:**
```sql
SELECT * FROM schema_migrations ORDER BY version;
-- Should show all applied migrations
```

---

### ✅ Task 3.3: Create Down Migrations (Weeks 7-8)
**Owner:** Backend Developer
**Effort:** 60 hours

**Actions:**
- [ ] Create Down functions for migrations 030-036 (recent)
- [ ] Create Down functions for migrations 020-029
- [ ] Create Down functions for migrations 010-019
- [ ] Create Down functions for migrations 001-009
- [ ] Test each Down migration in isolated environment
- [ ] Document rollback procedure
- [ ] Commit changes with message: `feat: add rollback support for all migrations`

**Deliverable:** All migrations have Down functions

**Verification:**
```bash
# Test rollback of last migration
# Should successfully revert changes
```

---

### ✅ Task 3.4: Migrate to golang-migrate (Weeks 9-12)
**Owner:** Backend Developer + DevOps
**Effort:** 120 hours

**Actions:**
- [ ] Install golang-migrate CLI
- [ ] Create `migrations_v2/` directory structure
- [ ] Convert all migrations to `.up.sql` and `.down.sql` format
- [ ] Update `core/internal/storage/storage.go`
- [ ] Create `core/cmd/migrate/main.go` CLI tool
- [ ] Update Dockerfile to include migrate CLI
- [ ] Update deployment scripts
- [ ] Test end-to-end in local
- [ ] Deploy to staging
- [ ] Soak test in staging (1 week)
- [ ] Deploy to production
- [ ] Commit changes with message: `feat: migrate to golang-migrate tool`

**Deliverable:** Production using golang-migrate

**Verification:**
```bash
migrate -version
migrate -database $DATABASE_URL -path migrations_v2 version
# Should show current migration version
```

---

## Phase 4: Deprecate Old Tables (Weeks 13-14) 🟢 LOW RISK

**Objective:** Clean up deprecated Trivy tables

### ✅ Task 4.1: Verify Data Migration (Week 13)
**Owner:** Backend Developer
**Effort:** 20 hours

**Actions:**
- [ ] Run `scripts/verify-trivy-migration.sql` in production
- [ ] Analyze results
- [ ] Confirm no recent writes to Trivy tables
- [ ] Get stakeholder approval to deprecate
- [ ] Document findings

**Deliverable:** Data migration verification report

---

### ✅ Task 4.2: Archive and Drop Tables (Week 14)
**Owner:** Backend Developer + DBA
**Effort:** 15 hours

**Actions:**
- [ ] Create Migration 037 (deprecate tables - rename with `_deprecated`)
- [ ] Create Migration 038 (drop deprecated tables)
- [ ] Create backup of deprecated tables
- [ ] Deploy Migration 037 to production
- [ ] Wait 1 week, monitor for issues
- [ ] Export table data to CSV
- [ ] Deploy Migration 038 to production (with confirmation flag)
- [ ] Verify tables dropped
- [ ] Archive backups to S3/GCS
- [ ] Commit changes with message: `feat: deprecate and drop Trivy tables`

**Deliverable:** Old tables removed, data archived

**Verification:**
```sql
\dt *deprecated*
-- Should return "no relations found" after Migration 038
```

---

## Buffer & Final Testing (Weeks 15-16)

### ✅ Final Validation
**Owner:** QA Team + Backend Team
**Effort:** 40 hours

**Actions:**
- [ ] Full regression testing in staging
- [ ] Performance testing (migration speed, query performance)
- [ ] Security audit of migration code
- [ ] Documentation review
- [ ] Runbook creation for on-call engineers
- [ ] Post-mortem of migration project
- [ ] Celebrate! 🎉

**Deliverable:** Production-ready migration system

---

## Success Criteria

### Phase 1 Success
- ✅ All orphaned files archived
- ✅ No silent error handling in migrations
- ✅ CI validates migrations on every PR
- ✅ All migrations have documentation headers

### Phase 2 Success
- ✅ Zero AutoMigrate calls in production code
- ✅ All migrations use SQL files
- ✅ Dry-run mode works correctly
- ✅ Staging runs successfully for 1 week

### Phase 3 Success
- ✅ All migrations have rollback capability
- ✅ golang-migrate integrated
- ✅ Version tracking implemented
- ✅ Can rollback failed migrations in < 5 minutes

### Phase 4 Success
- ✅ Deprecated tables removed
- ✅ Data archived securely
- ✅ Storage savings achieved

### Overall Success
- ✅ Zero migration-related incidents in first 3 months
- ✅ Deployment rollback time reduced by 50%
- ✅ Developer confidence score > 8/10
- ✅ Migration system grade: A

---

## Risk Mitigation

### High Risk Items
1. **Removing AutoMigrate fallbacks** (Phase 2)
   - Mitigation: Extensive staging testing, feature flag for rollback
   - Contingency: Keep old code in separate branch for quick revert

2. **Production deployment of golang-migrate** (Phase 3)
   - Mitigation: Soak test in staging for 1+ week
   - Contingency: Blue/green deployment with instant rollback

### Medium Risk Items
1. **Converting migrations to SQL** (Phase 2)
   - Mitigation: Generate SQL from GORM models, compare schemas
   - Contingency: AutoMigrate fallback available in development

2. **Creating Down migrations** (Phase 3)
   - Mitigation: Test every Down migration in isolated environment
   - Contingency: Database backup available for full restore

---

## Communication Plan

### Stakeholders
- **Engineering Team**: Weekly updates in team meeting
- **Product Team**: Bi-weekly status email
- **SRE Team**: Coordinate deployment windows
- **Leadership**: Monthly executive summary

### Status Updates
- **Daily**: Standup updates (if blockers)
- **Weekly**: Progress report in Slack #backend-team
- **Bi-weekly**: Demo in team showcase
- **Monthly**: Written status report to leadership

### Incident Response
- **Issue Found**: Report in #backend-team immediately
- **Blocker**: Escalate to Tech Lead within 1 hour
- **Production Incident**: Follow standard incident response procedure

---

## Resources Required

### Personnel
- **1x Backend Developer** (full-time, 16 weeks)
- **1x DevOps Engineer** (25% time, 16 weeks)
- **1x Tech Lead** (10% time, weeks 5-12 for tool evaluation)
- **1x QA Engineer** (25% time, weeks 2-16 for testing)
- **1x DBA** (consultant, 1 week for Phase 4)

### Infrastructure
- **Staging Environment**: Full production replica
- **Test Database**: For migration testing
- **CI/CD Pipeline**: GitHub Actions credits (minimal cost)

### Budget
- **Personnel**: ~$40,000 (4 months @ $10k/month)
- **Infrastructure**: ~$500/month staging costs
- **Tooling**: $0 (all open-source)
- **Total**: ~$42,000

---

## Quick Start

**Ready to begin? Start here:**

1. **Read the full audit report:**
   ```bash
   open docs/migration-audit-report.md
   ```

2. **Run Phase 1, Task 1.1 (file cleanup):**
   ```bash
   ./scripts/cleanup-orphaned-migrations.sh
   ```

3. **Implement Phase 1, Task 1.2 (schema validation):**
   - Edit `core/migrations/migrations.go`
   - Add validation function from Section 6.1

4. **Track progress:**
   - Use this checklist
   - Update status in weekly team meeting
   - Mark tasks complete as you go

---

## Questions?

**Contact:**
- Backend Team: #backend-team Slack
- Project Owner: [Your Name]
- Escalation: Tech Lead / Engineering Manager

**Resources:**
- Full Report: `docs/migration-audit-report.md`
- Best Practices: `docs/database-migration-best-practices.md`
- Migration Template: `core/migrations/MIGRATION_TEMPLATE.go`

---

**Last Updated:** 2025-12-27
**Next Review:** 2026-01-03 (end of Week 1)
