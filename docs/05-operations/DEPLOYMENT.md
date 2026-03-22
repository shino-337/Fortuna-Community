# Fortuna Deployment Guide

**Note**: This is the main deployment guide. For detailed production deployment instructions, see [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md). For containerd/nerdctl build and full pipeline, see [DEPLOYMENT_CONTAINERD](DEPLOYMENT_CONTAINERD.md).

## Quick Reference

1. **Environment Setup**: [Environment Preparation Guide](../01-getting-started/ENVIRONMENT_PREPARATION.md)
2. **Build Images**: [Build Guide](../01-getting-started/BUILD_GUIDE.md)
3. **Full Deployment**: [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
4. **Migrations**: [Migration Guide](../04-development/MIGRATIONS.md)
5. **Containerd pipeline**: [DEPLOYMENT_CONTAINERD](DEPLOYMENT_CONTAINERD.md) — clean, rebuild, deploy với `scripts/pipeline/full-clean-database-rebuild-deploy.sh`

## Quick Start (recommended: use deploy script)

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Generate certificates
bash scripts/utils/create_mtls_secret.sh

# 3. Deploy infrastructure, RBAC, Core, Agent, Dashboard (single script)
bash scripts/deploy/deploy-fortuna-robust.sh
```

**Capability Catalog:** Core deployment sets `FORTUNA_ENABLE_SEED_DATA=true` so migrations 050/051/061 seed `capability_metadata` and `promotion_rules`. If Catalog is empty after deploy, ensure this env is set and consider a DB reset so seed migrations run. See [FORTUNA_ENABLE_SEED_DATA.md](../04-development/FORTUNA_ENABLE_SEED_DATA.md).

Or apply manually (use **fortuna-*** YAMLs; deprecated: core-deployment.yaml / agent-daemonset.yaml):

```bash
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml   # or postgresql.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml
kubectl apply -f deploy/dashboard-deployment.yaml
```

Secrets: ensure `fortuna-secrets` exists (database-url, jwt-secret). See `deploy/README.md` or production guide.

## Verification

```bash
# Check pods
kubectl get pods -n fortuna

# Check Core health (from host or exec into pod)
kubectl exec -n fortuna deploy/fortuna-core -- curl -s http://localhost:8080/health

# Or port-forward then: curl http://localhost:8080/health
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &

# Check Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent | grep -E 'Heartbeat|Sync'
```

For detailed instructions, troubleshooting, and production considerations, see [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md) and [deploy/README.md](../../deploy/README.md).
