# Fortuna Deployment Guide

**Note**: This is the main deployment guide. For detailed production deployment instructions, see [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md). For containerd/nerdctl build and full pipeline, see [DEPLOYMENT_CONTAINERD](DEPLOYMENT_CONTAINERD.md).

## Quick Reference

1. **Environment**: [Requirements](../01-getting-started/ENVIRONMENT_REQUIREMENTS.md)
2. **Build & quick deploy**: [Quickstart](../01-getting-started/QUICKSTART.md) · [Containerd pipeline](DEPLOYMENT_CONTAINERD.md)
3. **Full Deployment**: [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
4. **Migrations**: [Core migrations](../../core/migrations/README.md)
5. **Clean / rebuild / deploy:** `scripts/pipeline/full-clean-database-rebuild-deploy.sh` — see [DEPLOYMENT_CONTAINERD](DEPLOYMENT_CONTAINERD.md)

## Quick Start

For users, use the published-image path in [Quickstart](../01-getting-started/QUICKSTART.md). That path deploys images from a registry so workers pull the same image without manual copy/import.

For local development, use the deploy script:

```bash
export FORTUNA_PACKAGE_SOURCE=local

# 1. Create namespace
kubectl create namespace fortuna

# 2. Generate certificates and secrets
NAMESPACE=fortuna bash scripts/utils/create_mtls_secret.sh
./scripts/utils/ensure-fortuna-secrets.sh fortuna

# 3. Deploy infrastructure, RBAC, Core, Agent, and Dashboard
bash scripts/deploy/deploy-fortuna-robust.sh
```

Leave `FORTUNA_PACKAGE_SOURCE` unset for the default GitHub/GHCR package path.

**Capability Catalog:** Core deployment sets `FORTUNA_ENABLE_SEED_DATA=true` so migrations 050/051/061 seed `capability_metadata` and `promotion_rules`. If the catalog is empty after deploy, ensure this environment variable is set on Core and consider a database reset so seed migrations run.

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

Secrets: ensure `fortuna-secrets` exists (`database-url`, `jwt-secret`, `admin-password`, `bootstrap-default-credential`). Use `./scripts/utils/ensure-fortuna-secrets.sh`; see `deploy/README.md` or production guide.

## Verification

```bash
# Check pods
kubectl get pods -n fortuna

# Check Core health (from host or exec into pod)
kubectl exec -n fortuna deploy/fortuna-core -- curl -s http://localhost:8080/healthz

# Or port-forward then: curl http://localhost:8080/healthz
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &

# Check Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=agent | grep -E 'Heartbeat|Sync'
```

## Multi-Cluster Deployment

Use one Fortuna management stack and one Agent DaemonSet per observed cluster.

| Cluster type | Deploy |
|--------------|--------|
| Management cluster | PostgreSQL, NATS, Core, Dashboard, Agent, optional Falco |
| Remote cluster | Agent, optional Falco |

For local/lab remote clusters, expose Core on the management cluster:

```bash
kubectl apply -f deploy/fortuna-core-external-service.yaml
kubectl -n fortuna get svc fortuna-core-external
```

Then configure each remote Agent:

```bash
export REMOTE_KUBECONFIG=/path/to/remote.kubeconfig
export MANAGEMENT_NODE=<management-node-ip-or-dns>
export REMOTE_KUBECONFIGS="cluster02=${REMOTE_KUBECONFIG}"
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"

./scripts/deploy/sync-remote-agent.sh
```

`sync-remote-agent.sh` reuses the same repo checkout and `.certs` material, creates the remote secrets/RBAC, applies Agent only, sets image/endpoints, and waits for rollout. Do not set `MTLS_REGEN=1` for the remote cluster. Use `REMOTE_IMAGE_MODE=local` for registryless labs that need SSH image import instead of registry pull. When used from the full pipeline, remote sync failures fail the pipeline by default; set `REMOTE_SYNC_REQUIRED=false` only for non-blocking lab runs.

Use unique `CLUSTER_ID` values. `CLUSTER_NAME` is display-only and can be changed later without changing resource ownership. Remote clusters should not run Core or Dashboard unless they are intentionally separate Fortuna installations.

Verify end to end:

```bash
FORTUNA_JWT="<admin-or-operator-jwt>" \
CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster101=/path/to/remote.kubeconfig" \
./scripts/verify/verify-multicluster-sync.sh
```

Expected alignment:

- Kubernetes pod count per cluster matches DB active pods for that cluster.
- `/api/v1/inventory/clusters/stats` shows every active cluster.
- `/api/v1/dashboard/stats?byType=all` equals the sum of active cluster-scoped data.

For detailed instructions, troubleshooting, and production considerations, see [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md) and [deploy/README.md](../../deploy/README.md).
