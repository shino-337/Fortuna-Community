# Database Migrations Guide

**Version**: 1.0  
**Last Updated**: 2026-01-05

---

## Overview

Fortuna uses automatic database migrations that run on Core startup. This document describes the migration system, migration order, and how to handle migration issues.

---

## Migration System

### Automatic Migrations

Core automatically runs all migrations on startup in the correct order. Migrations are tracked in the `schema_migrations` table.

### Migration Execution Order

Migrations are executed in the following order:

1. **001-003**: Initial schema (users, audit logs)
2. **008-010**: Deployments, replica sets, implementation guide schema
3. **011-013**: Insights soft delete, risk scores
4. **014-016**: Policy engine (templates, instances, violations)
5. **018**: Risk scores V2 columns
6. **019**: CVE tables (`cves`, `package_vulnerabilities`)
7. **020**: SBOM tables (`sboms`, `sbom_components`, `cve_matches`)
8. **021-023**: SBOM schema fixes and indexes
9. **024-026**: Performance indexes and constraints
10. **027-029**: CVE file metadata, performance indexes, unique constraints
11. **030-036**: Schema migrations and cleanup

### Current Migrations (36 total)

| Migration | Description | Status |
|-----------|-------------|--------|
| 001 | Initial schema | ✅ Active |
| 002 | Add users | ✅ Active |
| 003 | Add user to audit logs | ✅ Active |
| 008 | Add deployments | ✅ Active |
| 009 | Add replica sets | ✅ Active |
| 010 | Implementation guide schema | ✅ Active |
| 011 | Add insights soft delete | ✅ Active |
| 012 | Add risk scores | ✅ Active |
| 013 | Add risk scores deleted_at | ✅ Active |
| 014 | Add policy templates | ✅ Active |
| 015 | Add policy instances | ✅ Active |
| 016 | Add policy violations | ✅ Active |
| 018 | Add risk scores V2 columns | ✅ Active |
| 019 | Add CVE tables | ✅ Active |
| 020 | Add SBOM tables | ✅ Active |
| 021 | Fix SBOM schema | ✅ Active |
| 022 | Add CVE columns to insights | ✅ Active |
| 023 | Fix SBOM CVE indexes | ✅ Active |
| 024 | Add pod image scans unique index | ✅ Active |
| 025 | Make upsert unique indexes non-partial | ✅ Active |
| 026 | Add insights JSONB indexes | ✅ Active |
| 027 | Add CVE file metadata | ✅ Active |
| 028 | Add performance indexes | ✅ Active |
| 029 | Add insights unique constraint | ✅ Active |
| 030 | Migrate insights schema complete | ✅ Active |
| 031 | Cleanup duplicate indexes | ✅ Active |
| 032 | Migrate CVE matches complete | ✅ Active |
| 033 | Add unique constraints | ✅ Active |
| 034 | Standardize CVSS type | ✅ Active |
| 035 | Evaluate Trivy tables | ✅ Active |
| 036 | Add missing SBOM columns | ✅ Active |

---

## Migration Verification

### Check Migration Status

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT version, name, executed_at FROM schema_migrations ORDER BY version DESC LIMIT 10;"
```

### Verify Tables Exist

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('sboms', 'sbom_components', 'cve_matches', 'insights', 'cves', 'package_vulnerabilities') ORDER BY table_name;"
```

Expected output:
```
     table_name      
-------------------
 cve_matches
 cves
 insights
 package_vulnerabilities
 sbom_components
 sboms
(6 rows)
```

### Verify Schema Columns

Check critical columns exist:

```bash
# Check insights table columns
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "\d insights" | grep -E "(insight_type|resource_type|resource_name|title|recommendation|cvss|detected_at)"

# Check cve_matches table columns
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "\d cve_matches" | grep -E "(package_name|package_version|purl|pod_uid|container_name|matched_by)"
```

---

## Manual Migration Execution

### When to Run Manually

Manual migrations are only needed if:
1. Automatic migrations fail
2. Core cannot start due to migration errors
3. You need to run migrations before deploying Core

### Running Migrations Manually

#### Option 1: Using Core Binary

```bash
# Build Core binary
cd core
go build -o ../bin/fortuna-core ./cmd

# Run migrations (requires DATABASE_URL)
export DATABASE_URL="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
./bin/fortuna-core migrate
```

#### Option 2: Direct SQL Execution

For critical migrations, you can run SQL directly:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna <<'EOF'
-- Example: Add missing column
ALTER TABLE insights ADD COLUMN IF NOT EXISTS detected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
EOF
```

---

## Migration Troubleshooting

### Migration Fails on Startup

**Symptoms**: Core pod in CrashLoopBackOff, logs show migration errors

**Solution**:

1. Check Core logs:
```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100 | grep -i migration
```

2. Check database connection:
```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT 1;"
```

3. Check migration status:
```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"
```

4. If migration partially completed, you may need to:
   - Fix the migration error
   - Manually complete the migration
   - Restart Core

### Missing Tables After Migration

**Symptoms**: `ERROR: relation "sboms" does not exist`

**Solution**: Run migration 020 manually:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -f /path/to/core/migrations/mvp2/006_add_sbom_tables.sql
```

Or check if migration 020 was executed:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT * FROM schema_migrations WHERE name LIKE '%SBOM%';"
```

### Missing Columns

**Symptoms**: `ERROR: column "insight_type" does not exist`

**Solution**: Run migration 030 manually or add column directly:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna <<'EOF'
ALTER TABLE insights ADD COLUMN IF NOT EXISTS insight_type VARCHAR(50);
ALTER TABLE insights ADD COLUMN IF NOT EXISTS resource_type VARCHAR(50);
ALTER TABLE insights ADD COLUMN IF NOT EXISTS resource_name VARCHAR(255);
ALTER TABLE insights ADD COLUMN IF NOT EXISTS title VARCHAR(500);
ALTER TABLE insights ADD COLUMN IF NOT EXISTS recommendation TEXT;
ALTER TABLE insights ADD COLUMN IF NOT EXISTS cvss REAL;
ALTER TABLE insights ADD COLUMN IF NOT EXISTS detected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
EOF
```

### Migration Order Issues

**Symptoms**: Migration fails because prerequisite migration not run

**Solution**: Ensure migrations run in order. Check `schema_migrations` table:

```bash
kubectl exec -n fortuna deployment/postgres -- psql -U postgres -d fortuna -c "SELECT version, name FROM schema_migrations ORDER BY version;"
```

If migrations are out of order, you may need to:
1. Reset migration tracking (careful - only if no data)
2. Manually run missing migrations
3. Update `schema_migrations` table

---

## Migration Best Practices

1. **Backup Before Migrations**: Always backup database before running migrations in production
2. **Test Migrations**: Test migrations in staging environment first
3. **Monitor Migration Logs**: Watch Core logs during startup for migration errors
4. **Verify Schema**: After migrations, verify all required tables and columns exist
5. **Document Custom Migrations**: If you add custom migrations, document them

---

## Schema Reference

### Key Tables

- **sboms**: SBOM records
- **sbom_components**: Components within SBOMs
- **cve_matches**: CVE matches for SBOM components
- **cves**: CVE database
- **package_vulnerabilities**: Package-to-CVE mappings
- **insights**: Security insights and recommendations

### Key Columns

**insights table**:
- `insight_type`: Type of insight (VULNERABLE_WORKLOAD, etc.)
- `resource_type`: Kubernetes resource type (Pod, Deployment, etc.)
- `resource_name`: Resource name
- `resource_uid`: Resource UID
- `title`: Insight title
- `recommendation`: Remediation recommendation
- `cvss`: CVSS score
- `detected_at`: Detection timestamp

**cve_matches table**:
- `package_name`: Package name
- `package_version`: Package version
- `purl`: Package URL
- `pod_uid`: Pod UID
- `container_name`: Container name
- `matched_by`: Matcher used (grype, etc.)

See [Database Schema Documentation](DATABASE_SCHEMA_UPDATED.md) for complete schema reference.

---

## Next Steps

- [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
- [Database Schema Documentation](DATABASE_SCHEMA_UPDATED.md)
- [Architecture Documentation](ARCHITECTURE.md)

