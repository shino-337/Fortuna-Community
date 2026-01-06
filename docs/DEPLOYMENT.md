# Fortuna Deployment Guide

**Note**: This is the main deployment guide. For detailed production deployment instructions, see [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md).

## Quick Reference

1. **Environment Setup**: [Environment Preparation Guide](ENVIRONMENT_PREPARATION.md)
2. **Build Images**: [Build Guide](BUILD_GUIDE.md)
3. **Full Deployment**: [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
4. **Migrations**: [Migration Guide](MIGRATIONS.md)

## Quick Start

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Generate certificates
bash scripts/create_mtls_secret.sh

# 3. Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# 4. Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/agent-rbac.yaml

# 5. Create secrets
kubectl create secret generic fortuna-secrets \
  --from-literal=database-url="postgres://postgres:postgres@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --namespace=fortuna

# 6. Deploy Core
kubectl apply -f deploy/core-deployment.yaml

# 7. Deploy Agent
kubectl apply -f deploy/agent-daemonset.yaml
```

## Verification

```bash
# Check pods
kubectl get pods -n fortuna

# Check Core health
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &
curl http://localhost:8080/health

# Check Agent logs
kubectl logs -n fortuna -l app=fortuna-agent | grep Heartbeat
```

For detailed instructions, troubleshooting, and production considerations, see [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md).
