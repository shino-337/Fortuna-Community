# Migration Guide: KSAM → Fortuna K8s Management Platform

**Date**: 2024-12-20  
**Version**: 1.0  
**Estimated Time**: 2-3 hours

---

## Overview

This guide walks you through migrating from **KSAM (Kubernetes Service Account Manager)** to **Fortuna K8s Management Platform**.

### What's Changing

| Aspect | Before | After |
|--------|--------|-------|
| **Project Name** | KSAM | Fortuna |
| **Full Name** | Kubernetes Service Account Manager | Fortuna K8s Management Platform |
| **Scope** | ServiceAccount management | Comprehensive K8s security & compliance |
| **Namespace** | `ksam` | `fortuna` |
| **Module Path** | `github.com/ksam/*` | `github.com/fortuna/*` |
| **Environment Variables** | `KSAM_*` | `FORTUNA_*` |

### Why This Change?

1. **Scope Expansion**: Project now covers full K8s security, not just ServiceAccounts
2. **Better Branding**: "Fortuna" represents fortune/prosperity in security
3. **Clarity**: Avoids confusion with narrow focus on ServiceAccounts

---

## Prerequisites

Before starting the migration:

- [ ] Backup current deployment
- [ ] Backup database
- [ ] Document custom configurations
- [ ] Review active policies
- [ ] Export insights data
- [ ] Stop all running instances

---

## Migration Steps

### Step 1: Backup Current State (15 min)

```bash
# Backup Kubernetes resources
kubectl get all -n ksam -o yaml > backup/ksam-resources.yaml
kubectl get secrets -n ksam -o yaml > backup/ksam-secrets.yaml
kubectl get configmaps -n ksam -o yaml > backup/ksam-configmaps.yaml
kubectl get pvc -n ksam -o yaml > backup/ksam-pvc.yaml

# Backup database
kubectl -n ksam exec $(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  pg_dump -U postgres ksam > backup/ksam-db-$(date +%Y%m%d).sql

# Backup custom configs
cp -r config/ backup/config/
```

### Step 2: Stop Current Deployment (5 min)

```bash
# Scale down deployments
kubectl -n ksam scale deployment ksam-core --replicas=0
kubectl -n ksam scale daemonset ksam-agent --replicas=0

# Wait for graceful shutdown
sleep 30
```

### Step 3: Create New Namespace (5 min)

```bash
# Create fortuna namespace
kubectl create namespace fortuna

# Copy secrets to new namespace
kubectl get secret -n ksam -o yaml | \
  sed 's/namespace: ksam/namespace: fortuna/g' | \
  kubectl apply -f -

# Copy configmaps (if any)
kubectl get configmap -n ksam -o yaml | \
  sed 's/namespace: ksam/namespace: fortuna/g' | \
  kubectl apply -f -
```

### Step 4: Update Configuration (15 min)

**Update environment variables:**

```yaml
# Old (KSAM)
env:
- name: KSAM_SBOM_ENABLED
  value: "true"
- name: KSAM_CVE_SOURCE
  value: "postgres"
- name: KSAM_ADMIN_USERNAME
  value: "admin"

# New (Fortuna)
env:
- name: FORTUNA_SBOM_ENABLED
  value: "true"
- name: FORTUNA_CVE_SOURCE
  value: "postgres"
- name: FORTUNA_ADMIN_USERNAME
  value: "admin"
```

**Update labels:**

```yaml
# Old
labels:
  app: ksam-core
  component: ksam-agent

# New
labels:
  app: fortuna-core
  component: fortuna-agent
```

### Step 5: Migrate Database (20 min)

```bash
# Option A: Rename database (recommended)
kubectl -n fortuna exec -it $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -c "ALTER DATABASE ksam RENAME TO fortuna;"

# Option B: Create new database and restore
kubectl -n fortuna exec -it $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -c "CREATE DATABASE fortuna;"

kubectl -n fortuna exec -i $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres fortuna < backup/ksam-db-$(date +%Y%m%d).sql
```

### Step 6: Update Deployment Manifests (30 min)

**Update all YAML files:**

```bash
# Find and replace in deployment files
find deploy/ -name "*.yaml" -type f -exec sed -i '' 's/ksam/fortuna/g' {} \;
find deploy/ -name "*.yaml" -type f -exec sed -i '' 's/KSAM/FORTUNA/g' {} \;

# Verify changes
grep -r "ksam\|KSAM" deploy/ || echo "All references updated"
```

**Key files to update:**
- `deploy/core-deployment.yaml`
- `deploy/agent-daemonset.yaml`
- `deploy/core-service.yaml`
- `deploy/webhook-config.yaml`
- `deploy/infrastructure/*.yaml`

### Step 7: Build New Images (30 min)

```bash
# Update Docker image tags
cd core/
docker build -t fortuna/core:latest .

cd ../agent/
docker build -t fortuna/agent:latest .

# If using registry, push images
docker push fortuna/core:latest
docker push fortuna/agent:latest
```

### Step 8: Deploy Fortuna (20 min)

```bash
# Apply infrastructure
kubectl apply -f deploy/infrastructure/postgresql.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/infrastructure/redis.yaml

# Wait for infrastructure
kubectl wait --for=condition=ready pod -n fortuna -l app=postgres --timeout=300s
kubectl wait --for=condition=ready pod -n fortuna -l app=nats --timeout=300s

# Deploy core
kubectl apply -f deploy/core-secrets.yaml
kubectl apply -f deploy/core-deployment.yaml
kubectl apply -f deploy/core-service.yaml

# Deploy agent
kubectl apply -f deploy/agent-rbac.yaml
kubectl apply -f deploy/agent-daemonset.yaml

# Deploy webhook
kubectl apply -f deploy/webhook-config.yaml
kubectl apply -f deploy/webhook-service.yaml
```

### Step 9: Verify Deployment (15 min)

```bash
# Check pods
kubectl get pods -n fortuna

# Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=50

# Check database connection
kubectl -n fortuna exec -it $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d fortuna -c "\dt"

# Test API
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/api/v1/health
```

### Step 10: Update Documentation (30 min)

```bash
# Update local documentation
cd docs/
find . -name "*.md" -type f -exec sed -i '' 's/KSAM/Fortuna/g' {} \;
find . -name "*.md" -type f -exec sed -i '' 's/ksam/fortuna/g' {} \;
find . -name "*.md" -type f -exec sed -i '' 's/Kubernetes Service Account Manager/Fortuna K8s Management Platform/g' {} \;

# Verify changes
grep -r "KSAM\|ksam" . || echo "All documentation updated"
```

### Step 11: Clean Up Old Resources (10 min)

```bash
# After verifying new deployment works

# Delete old namespace (careful!)
kubectl delete namespace ksam

# Remove old images
docker rmi ksam/core:latest
docker rmi ksam/agent:latest

# Clean up backup (optional, after verification period)
# rm -rf backup/
```

---

## Environment Variable Migration

### Complete Mapping

| Old (KSAM) | New (Fortuna) | Notes |
|------------|---------------|-------|
| `KSAM_SBOM_ENABLED` | `FORTUNA_SBOM_ENABLED` | SBOM feature toggle |
| `KSAM_SBOM_USE_CUSTOM` | `FORTUNA_SBOM_USE_CUSTOM` | Custom SBOM extractor |
| `KSAM_SBOM_MODE` | `FORTUNA_SBOM_MODE` | async/sync mode |
| `KSAM_CVE_SOURCE` | `FORTUNA_CVE_SOURCE` | postgres/trivy |
| `KSAM_CVE_AUTO_SYNC` | `FORTUNA_CVE_AUTO_SYNC` | Auto-sync CVEs |
| `KSAM_ADMIN_USERNAME` | `FORTUNA_ADMIN_USERNAME` | Admin username |
| `KSAM_ADMIN_PASSWORD` | `FORTUNA_ADMIN_PASSWORD` | Admin password |
| `KSAM_ADMIN_EMAIL` | `FORTUNA_ADMIN_EMAIL` | Admin email |
| `KSAM_RULES_DIR` | `FORTUNA_RULES_DIR` | Rules directory |

### Backward Compatibility (Optional)

For gradual migration, you can support both old and new variable names:

```go
// In code
func getEnv(key string, defaultValue string) string {
    // Try new name first
    if value := os.Getenv("FORTUNA_" + key); value != "" {
        return value
    }
    // Fall back to old name (deprecated)
    if value := os.Getenv("KSAM_" + key); value != "" {
        log.Printf("WARNING: KSAM_%s is deprecated, use FORTUNA_%s", key, key)
        return value
    }
    return defaultValue
}
```

---

## Troubleshooting

### Issue 1: Database Connection Failed

**Symptoms**: `ERROR: database "ksam" does not exist`

**Solution**:
```bash
# Check database name
kubectl -n fortuna exec -it $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -c "\l"

# If database is still named "ksam", update connection string
kubectl -n fortuna set env deployment/fortuna-core DATABASE_URL="postgresql://postgres:postgres@postgres:5432/ksam"
```

### Issue 2: NATS Connection Failed

**Symptoms**: `ERROR: nats: no servers available for connection`

**Solution**:
```bash
# Check NATS endpoint
kubectl -n fortuna set env deployment/fortuna-core NATS_ENDPOINT="nats://nats.fortuna.svc.cluster.local:4222"
```

### Issue 3: Missing Secrets

**Symptoms**: `ERROR: secret "ksam-secrets" not found`

**Solution**:
```bash
# Copy secrets with new name
kubectl get secret ksam-secrets -n ksam -o yaml | \
  sed 's/name: ksam-secrets/name: fortuna-secrets/g' | \
  sed 's/namespace: ksam/namespace: fortuna/g' | \
  kubectl apply -f -
```

### Issue 4: Old Insights Not Showing

**Symptoms**: Dashboard shows no insights after migration

**Solution**:
```bash
# Check database data
kubectl -n fortuna exec -it $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d fortuna -c "SELECT COUNT(*) FROM insights;"

# If count is 0, restore from backup
kubectl -n fortuna exec -i $(kubectl get pod -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres fortuna < backup/ksam-db-*.sql
```

---

## Rollback Plan

If migration fails, rollback to KSAM:

```bash
# Stop Fortuna
kubectl delete namespace fortuna

# Restore KSAM
kubectl apply -f backup/ksam-resources.yaml

# Restore database (if changed)
kubectl -n ksam exec -i $(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres ksam < backup/ksam-db-*.sql

# Scale up KSAM
kubectl -n ksam scale deployment ksam-core --replicas=1
kubectl -n ksam scale daemonset ksam-agent --replicas=1
```

---

## Verification Checklist

After migration, verify:

- [ ] All pods are running in `fortuna` namespace
- [ ] Database connection works
- [ ] NATS connection works
- [ ] API is accessible
- [ ] Dashboard is accessible
- [ ] Insights are visible
- [ ] Policies are loaded
- [ ] CVE scanning works
- [ ] SBOM generation works
- [ ] Webhook is working
- [ ] Agent is collecting data
- [ ] No errors in logs

---

## Post-Migration Tasks

### Update External Integrations

- Update monitoring dashboards (Grafana, Prometheus)
- Update alert rules
- Update external API clients
- Update CI/CD pipelines
- Update documentation links
- Notify team members

### Update Repository

```bash
# Update repository name (if applicable)
# GitHub → Settings → Repository name → fortuna-k8s-platform

# Update README
# Update LICENSE (if needed)
# Update CONTRIBUTING guide
```

---

## FAQ

**Q: Can I run KSAM and Fortuna side by side?**  
A: Yes, but not recommended for production. Use different namespaces and database names.

**Q: Will my old insights data be preserved?**  
A: Yes, if you use Option A (rename database) or Option B (restore to new database).

**Q: Do I need to rebuild all images?**  
A: Yes, with new tags. But functionality is the same.

**Q: How long does migration take?**  
A: 2-3 hours for full migration with testing.

**Q: Can I rollback after migration?**  
A: Yes, if you keep the backup. See "Rollback Plan" section.

**Q: Will API endpoints change?**  
A: No, API endpoints remain the same (e.g., `/api/v1/insights`).

---

## Support

If you encounter issues during migration:

1. Check logs: `kubectl logs -n fortuna -l app=fortuna-core --tail=100`
2. Review troubleshooting section above
3. Restore from backup if needed
4. File an issue with migration logs

---

**Last Updated**: 2024-12-20  
**Status**: Ready for use

