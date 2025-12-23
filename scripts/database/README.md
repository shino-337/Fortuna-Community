# Database Scripts

**11 scripts** for database setup, migrations, and maintenance.

---

## 📝 Scripts

| Script | Purpose | Destructive |
|--------|---------|-------------|
| **setup_database.sh** | Initialize database | No |
| **clear_database.sh** | Drop all tables | ⚠️ YES |
| **clear_database_k8s.sh** | Clear K8s data | ⚠️ YES |
| **compare_db_k8s.sh** | Compare DB with K8s | No |
| **verify_and_sync_database.sh** | Verify and sync | No |
| **sync_and_verify_all.sh** | Full sync | No |
| **cleanup_insights.sql** | SQL cleanup script | ⚠️ YES |
| **test_database_rename.sh** | Test DB rename | No |
| **manual-run-migration018.sh** | Run migration 018 | No |
| **run-migration-020.sh** | Run migration 020 | No |
| **run-migration018-direct.sh** | Direct migration 018 | No |

---

## 🚀 Quick Start

### Initial Setup
```bash
./setup_database.sh
```

### Verify Database
```bash
./compare_db_k8s.sh
```

### Run Migrations
```bash
./run-migration-020.sh
```

---

## 📚 Detailed Usage

### setup_database.sh
Initialize Fortuna database:
```bash
# Setup with defaults
./setup_database.sh

# Setup with custom namespace
NAMESPACE=production ./setup_database.sh
```

Creates:
- Database schema
- Required tables
- Indexes
- Initial data

---

### clear_database.sh
⚠️ **DESTRUCTIVE** - Drops all database tables:
```bash
# Clear database (prompts for confirmation)
./clear_database.sh

# Force clear (no prompt)
FORCE=yes ./clear_database.sh
```

**WARNING**: This deletes ALL data!

---

### compare_db_k8s.sh
Compare database state with Kubernetes:
```bash
./compare_db_k8s.sh
```

Shows:
- Resources in K8s but not in DB
- Resources in DB but not in K8s
- Mismatches

---

### verify_and_sync_database.sh
Verify database and sync with K8s:
```bash
./verify_and_sync_database.sh
```

Actions:
1. Verifies database schema
2. Syncs missing resources
3. Updates outdated data

---

### sync_and_verify_all.sh
Complete sync and verification:
```bash
./sync_and_verify_all.sh
```

Performs:
- Full database sync
- Data verification
- Consistency checks

---

## 🔄 Migration Scripts

### run-migration-020.sh
Run migration 020:
```bash
./run-migration-020.sh
```

### manual-run-migration018.sh
Manually run migration 018:
```bash
./manual-run-migration018.sh
```

### run-migration018-direct.sh
Direct migration 018 execution:
```bash
./run-migration018-direct.sh
```

---

## 🔧 Common Tasks

### Fresh Database Setup
```bash
# 1. Clear old data (if exists)
./clear_database.sh

# 2. Setup new database
./setup_database.sh

# 3. Run migrations
./run-migration-020.sh

# 4. Verify
./compare_db_k8s.sh
```

### Database Maintenance
```bash
# Sync database with K8s
./sync_and_verify_all.sh

# Verify consistency
./compare_db_k8s.sh
```

### Troubleshooting Database
```bash
# Check database status
kubectl exec -n fortuna postgres-xxx -- \
  psql -U postgres -d fortuna -c "\dt"

# Compare with K8s
./compare_db_k8s.sh

# Sync if needed
./verify_and_sync_database.sh
```

---

## 🗄️ Database Access

### Direct Access
```bash
# Get postgres pod
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')

# Connect to database
kubectl exec -it $POSTGRES_POD -n fortuna -- \
  psql -U postgres -d fortuna
```

### Common Queries
```sql
-- Count CVEs
SELECT COUNT(*) FROM cves;

-- Count insights
SELECT COUNT(*) FROM insights WHERE status='active';

-- List SBOMs
SELECT id, pod_name, namespace, created_at FROM sboms ORDER BY created_at DESC LIMIT 10;

-- View insights
SELECT id, title, severity, resource_name, created_at FROM insights ORDER BY created_at DESC LIMIT 10;
```

---

## ⚠️ Destructive Operations

### Scripts That Delete Data
- `clear_database.sh` - Drops ALL tables
- `clear_database_k8s.sh` - Clears K8s-related data
- `cleanup_insights.sql` - Runs cleanup SQL

### Safety Checklist
Before running destructive scripts:
- [ ] Backup database
- [ ] Verify backup works
- [ ] Understand what will be deleted
- [ ] Have recovery plan
- [ ] Confirm with team (if production)

### Backup Database
```bash
# Backup
kubectl exec -n fortuna $POSTGRES_POD -- \
  pg_dump -U postgres fortuna > backup_$(date +%Y%m%d).sql

# Restore
kubectl exec -i -n fortuna $POSTGRES_POD -- \
  psql -U postgres fortuna < backup_20241222.sql
```

---

## 🚨 Troubleshooting

### Database Connection Failed
```bash
# Check postgres pod
kubectl get pods -n fortuna -l app=postgres

# Check logs
kubectl logs -n fortuna -l app=postgres --tail=50

# Verify service
kubectl get svc -n fortuna postgres
```

### Migration Failed
```bash
# Check migration logs
kubectl logs -n fortuna -l job-name=migration-020

# Check database state
kubectl exec -n fortuna $POSTGRES_POD -- \
  psql -U postgres -d fortuna -c "SELECT * FROM schema_migrations;"

# Rollback if needed
kubectl exec -n fortuna $POSTGRES_POD -- \
  psql -U postgres -d fortuna -c "DELETE FROM schema_migrations WHERE version='020';"
```

### Data Sync Issues
```bash
# Compare DB vs K8s
./compare_db_k8s.sh

# Force sync
./sync_and_verify_all.sh

# Verify
./verify_and_sync_database.sh
```

---

## 📖 Related Documentation

- [Database Schema](../../docs/02-architecture/DATABASE_SCHEMA.md)
- [Migrations Guide](../../docs/04-development/migrations/README.md)
- [Setup Scripts](../setup/README.md)

---

*Back to [Scripts README](../README.md)*

