# Fortuna Deployment Files

This directory contains Kubernetes deployment manifests for Fortuna components.

## Structure

```
deploy/
├── fortuna-core-deployment.yaml      # Core service deployment (recommended)
├── fortuna-agent-daemonset.yaml     # Agent DaemonSet (recommended)
├── dashboard-deployment.yaml         # Dashboard deployment
├── fortuna-rbac.yaml                 # RBAC for core and agent
├── webhook-config.yaml               # Admission webhook configuration
├── webhook-service.yaml              # Admission webhook service
├── core-service.yaml                 # Core service (HTTP/gRPC)
├── core-secrets.yaml                 # Secrets template
├── certs/                            # mTLS certificates
│   ├── 01-root-ca.yaml
│   ├── 02-core-server-cert.yaml
│   └── 03-agent-client-cert.yaml
├── infrastructure/                   # Infrastructure components
│   ├── postgresql.yaml              # PostgreSQL deployment
│   ├── nats.yaml                    # NATS deployment
│   └── redis.yaml                   # Redis (optional)
└── e2e/                             # E2E test resources
    └── ksam-e2e-vuln-pod.yaml
```

## Deployment Files

### Recommended (Latest)

- **`fortuna-core-deployment.yaml`**: Core service deployment with:
  - mTLS configuration
  - Webhook TLS certificates
  - PCE scheduler enabled
  - Auth enabled
  - Health probes

- **`fortuna-agent-daemonset.yaml`**: Agent DaemonSet with:
  - Containerd socket access
  - mTLS client certificates
  - Resource limits optimized

### Legacy (Deprecated)

- `core-deployment.yaml`: Old core deployment (use `fortuna-core-deployment.yaml`)
- `agent-daemonset.yaml`: Old agent deployment (use `fortuna-agent-daemonset.yaml`)

## Removed Components

The following monitoring components have been removed as they were not in use:

- `grafana/` - Grafana dashboards (not deployed)
- `monitoring/` - Prometheus alerts (not deployed)
- `prometheus/` - Prometheus rules (not deployed)

**Note**: Core service exposes `/metrics` endpoint for Prometheus scraping, but no Prometheus deployment is included.

## Full pipeline (clean + rebuild + redeploy)

From repo root, for a full reset (clean all data/cache/images, rebuild no-cache, redeploy):

```bash
NO_CACHE=true ./scripts/full-clean-rebuild-redeploy.sh --db
```

If the script times out during deploy (e.g. Flannel step), ensure Core is deployed:

```bash
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl rollout status deployment/fortuna-core -n fortuna
```

Then run verification: `./scripts/check-full-deployment.sh`, `./scripts/run-e2e-full.sh`.

## Cluster identity (auto-discovery & SSOT)

- **Agent:** Cluster identity (id/name) auto-discovered from K8s API (kube-system UID, `/version`). Optional override via ConfigMap `fortuna-cluster-config` (keys `cluster_id`, `cluster_name`) or env `CLUSTER_ID`/`CLUSTER_NAME`. Do **not** hardcode cluster id/name in deployments.
- **Core:** Single source of truth: creates cluster by id when missing, updates only mutable fields (name, source, k8s_version, distribution, last_sync) when id exists.
- **Migrations:** 054 adds cluster metadata columns; 055 drops UNIQUE on `clusters(name)` so multiple clusters can share a display name. Optional one-time rename: set Core env `CLUSTER_ID_TO_UPDATE` and `CLUSTER_DISPLAY_NAME` before starting (migration 053).
- Chi tiết: `docs/03-components/dashboad/CLUSTER_IDENTITY_FLOW.md`.

## Quick Deploy

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Deploy infrastructure
kubectl apply -f infrastructure/postgresql.yaml
kubectl apply -f infrastructure/nats.yaml

# 3. Deploy certificates
kubectl apply -f certs/

# 4. Deploy secrets
kubectl apply -f core-secrets.yaml

# 5. Deploy RBAC
kubectl apply -f fortuna-rbac.yaml

# 6. Deploy core
kubectl apply -f fortuna-core-deployment.yaml
kubectl apply -f core-service.yaml

# 7. Deploy agent
kubectl apply -f fortuna-agent-daemonset.yaml

# 8. Deploy dashboard
kubectl apply -f dashboard-deployment.yaml

# 9. Deploy webhook (optional)
kubectl apply -f webhook-service.yaml
kubectl apply -f webhook-config.yaml
```

## Configuration

### Environment Variables

**Core**:
- `DATABASE_URL`: PostgreSQL connection string
- `NATS_ENDPOINT`: NATS server endpoint (default: `nats://nats-client.fortuna.svc.cluster.local:4222`)
- `TLS_ENABLED`: Enable mTLS (default: `true`)
- `AUTH_ENABLED`: Enable JWT auth (default: `true`)
- `PCE_SCHEDULER_ENABLED`: Enable PCE scheduler (default: `true`)
- `PCE_SCHEDULER_INTERVAL`: Scheduler interval (default: `6h`)

**Agent**:
- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint
- **Cluster identity**: Auto-discovered from K8s API (kube-system UID hash + kubeconfig name). No hardcoded default. Optional override via ConfigMap `fortuna-cluster-config` keys `cluster_id`, `cluster_name` (see `CLUSTER_IDENTITY_FLOW.md`).
- `CLUSTER_ID` / `CLUSTER_NAME`: Optional. If set (e.g. from ConfigMap), agent uses them and logs `source=env_override`; else auto-discovery and logs `source=auto`.
- `SYNC_INTERVAL`: Sync interval (default: `5m`)
- `TLS_ENABLED`: Enable mTLS (default: `true`)

## Image Pull Policy

For local development with containerd:
- Set `imagePullPolicy: Never` to use local images

For production:
- Set `imagePullPolicy: IfNotPresent` or `Always`
- Use versioned image tags (e.g., `fortuna-core:v1.0.0`)

## Dashboard data and stale cleanup

- **Cluster / node / IP**: All identity comes from **environment or Kubernetes**; no hardcoded cluster name, node name, or IP.
  - **Agent**: Set `CLUSTER_ID` and `CLUSTER_NAME` from your environment (e.g. ConfigMap populated by `kubectl config get-clusters`), or mount `KUBECONFIG` so agent reads cluster name from kubeconfig. `NODE_NAME` is from Downward API (`spec.nodeName`). If unset, agent uses "unknown".
  - **Core**: Set `DEFAULT_CLUSTER_ID` and `DEFAULT_CLUSTER_NAME` when no cluster row exists (e.g. for GetAgents fallback). For migration 053 (rename cluster display name), set `CLUSTER_ID_TO_UPDATE` and `CLUSTER_DISPLAY_NAME` from your environment.
  - **Deploy YAML**: Do not hardcode cluster values; use envFrom/ConfigMap or document that operator must set env per environment.
- **Active clusters only**: Dashboard and `/clusters` API return only clusters that have synced in the last **7 days**. Older clusters are hidden so the UI reflects the current environment.
- **Stale cleanup**: Core soft-deletes clusters (and their pods) that have not synced in **90 days**, so old env data is removed from the database. To see all clusters (including stale) use `?includeStale=true` on `/clusters` (admin only).

## Cleaning cluster data

To reset cluster-related data so Agent re-syncs with the correct cluster id/name (e.g. after changing CLUSTER_ID/CLUSTER_NAME):

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl cp deploy/e2e/clear_all_cluster_data.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec $PG_POD -- psql -U postgres -d fortuna -f /tmp/clear_all_cluster_data.sql
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

Then wait for Agent sync (or trigger by restarting agent pods). API `/api/v1/clusters` will show the cluster from the new sync (auto-discovered id/name or override from ConfigMap).

## Notes

- Core deployment requires control-plane node (nodeSelector + tolerations)
- Agent DaemonSet runs on all nodes
- Dashboard deployment uses LoadBalancer service (adjust for your environment)
- All components use mTLS for secure communication
