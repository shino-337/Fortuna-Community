# FortunaK8s Deployment

Kubernetes manifests for **FortunaK8s** (Core, Agent, Dashboard, infrastructure). Deploy with **kubectl apply** or **Helm** (see [Helm](#helm)).

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐        ┌──────────────┐                   │
│  │ Fortuna Agent│───────▶│ Fortuna Core │──▶ Dashboard      │
│  │  (DaemonSet) │ gRPC   │ (Deployment) │   (Deployment)    │
│  └──────────────┘ mTLS   └──────┬───────┘                   │
│  Remote clusters run Agent only and connect to Core externally│
│                                  │                           │
│                         ┌────────┴────────┐                  │
│                         │                 │                  │
│                  ┌──────▼─────┐    ┌─────▼─────┐            │
│                  │ PostgreSQL │    │   NATS    │            │
│                  │  + AGE     │    │JetStream  │            │
│                  └────────────┘    └───────────┘            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Component Requirements

| Component | Type | CPU (req/limit) | Memory (req/limit) | Storage |
|-----------|------|-----------------|---------------------|---------|
| **Core** | Deployment | 100m / 1000m | 256Mi / 1Gi | — |
| **Agent** | DaemonSet | 100m / 1000m | 1Gi / 6Gi | — |
| **Dashboard** | Deployment | 10m / 100m | 64Mi / 128Mi | — |
| **PostgreSQL** | StatefulSet | 200m / 1000m | 512Mi / 2Gi | 20Gi+ SSD |
| **NATS** | StatefulSet (3x) | 100m / 500m | 256Mi / 2Gi | 10Gi/replica |
| **Redis** | Deployment | 50m / 200m | 64Mi / 256Mi | — |

### Environment Requirements

- **Kubernetes**: 1.28+
- **Container Runtime**: containerd 1.7+ (recommended) or Docker 20.10+
- **Nodes**: Min 2 (1 master + 1 worker), recommended 3+
- **Network**: Pod-to-pod communication, DNS, ports 8080 (HTTP), 9090 (gRPC), 5432 (PostgreSQL), 4222 (NATS)
- **StorageClass**: `local-path` or equivalent with ReadWriteOnce support

See [Environment Requirements](../docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md) for full details.

### ⚠️ Production Security Notes

- **Credentials**: Create `fortuna-secrets` from environment variables with `./scripts/utils/ensure-fortuna-secrets.sh`; do not commit real values.
- **Database credentials**: Provide `FORTUNA_DATABASE_URL` from your secret manager, CI, or local shell before deployment.
- **Image tags**: Use versioned tags (e.g., `v1.0.0`) instead of `:latest` for reproducibility.
- **mTLS**: Required for Agent↔Core communication. Generate certificates before deployment.

---

## Deploy checklist

### Before deploy

| Item | Notes |
|------|--------|
| **fortuna-secrets** | Create with `./scripts/utils/ensure-fortuna-secrets.sh` (from repo root). Requires `FORTUNA_DATABASE_URL`; `FORTUNA_ADMIN_PASSWORD` is recommended for production. If omitted, the script writes the first-login bootstrap default and Core requires a password change. `FORTUNA_JWT_SECRET` and `FORTUNA_INGEST_TOKEN` are generated when omitted. Optional values include `POD_DETAIL_ENCRYPTION_KEY`. |
| **Database password** | Bundled PostgreSQL uses `postgres-credentials` from `FORTUNA_POSTGRES_PASSWORD`. Set `FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"` so Core and PostgreSQL use the same password. |
| **mTLS** | Required secrets: `fortuna-core-tls`, `fortuna-agent-tls`, `fortuna-ca-cert`, `fortuna-webhook-tls`. Create with: `NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh` (from repo root). Certificates are written to `.certs/` (gitignored), then created in the cluster. Both Helm and kubectl need this step first. The full local pipeline now ensures these secrets before Core/Agent rollout. |
| **Order** | CNI (Flannel) → StorageClass (`local-path`) → mTLS secrets → **fortuna-secrets** / `postgres-credentials` → PostgreSQL + NATS → RBAC → control-plane label → Core → Agent → Dashboard. Wrong order can leave pods Pending or ContainerCreating. |
| **Namespace** | Default `fortuna`. If you use another namespace, update all manifests or use Helm with `-n <ns>` and `namespaceOverride`. |
| **PostgreSQL / NATS** | Must run in the same namespace (or set Core `DATABASE_URL` / `NATS_ENDPOINT` accordingly). The Helm chart does **not** deploy infra; run `kubectl apply -f deploy/infrastructure/...` first. |

### During deploy

| Item | Notes |
|------|--------|
| **Core** | Schedules only on **control-plane** nodes (nodeSelector + tolerations). If no node has `node-role.kubernetes.io/control-plane`, Core stays **Pending**. Run: `./scripts/deploy/ensure-control-plane-label.sh`. |
| **Agent** | DaemonSet on all nodes; needs containerd socket. Manifest uses **6Gi** memory limit + `SBOM_WORKERS=1` when Falco JSONL is enabled (SBOM + tail reader headroom). Build image with `nerdctl -n k8s.io` so kubelet sees the same `fortuna-agent:<VERSION>` tag referenced by the manifest. |
| **Dashboard** | Service type **LoadBalancer**; change to NodePort/ClusterIP or use Ingress as needed. Local registryless deploys schedule Dashboard on the control-plane/master, so its image does not need to be copied to workers. Apply ConfigMap `fortuna-dashboard-nginx` **before** the Dashboard deployment (proxies `/api` to Core). |
| **Images** | Dev/local manifests use versioned tags with `imagePullPolicy: IfNotPresent`; build/load the same tag on the node runtime. Production and multi-node installs should use immutable tags or digests from a registry. Use `./scripts/utils/push-images-to-workers.sh` only for registryless local/air-gapped Core/Agent runtime images. |
| **Auth** | Bootstrap admin is `admin`; password comes from `fortuna-secrets` key `admin-password`. If `FORTUNA_ADMIN_PASSWORD` was omitted on a fresh DB, use `Fortuna_ChangeMe_123!` once and change it when prompted. Existing DBs keep the current admin password; Core will not reset it to the bootstrap default. |

### After deploy

| Item | Notes |
|------|--------|
| **Verify** | Run `./scripts/verify/check-full-deployment.sh` (set `NAMESPACE` if not `fortuna`). |
| **Troubleshooting** | See [Troubleshooting](#troubleshooting). |
| **Access** | Dashboard: `kubectl port-forward --address 0.0.0.0 -n fortuna svc/fortuna-dashboard 8081:80` → http://localhost:8081. Core API: `kubectl port-forward -n fortuna svc/fortuna-core 8080:8080` → http://localhost:8080/healthz. |

---

## Directory structure

```
deploy/
├── fortuna-core-deployment.yaml     # Core: Service + Deployment (mTLS, auth, PCE)
├── fortuna-agent-daemonset.yaml     # Agent DaemonSet (containerd, mTLS)
├── dashboard-deployment.yaml        # Dashboard: Deployment + Service (LoadBalancer)
├── dashboard-nginx-configmap.yaml   # Nginx config (proxy /api → Core)
├── fortuna-rbac.yaml                # RBAC: ServiceAccount, ClusterRole, ClusterRoleBinding (core + agent)
├── core-secrets.example.yaml        # Example only; create fortuna-secrets from environment variables
├── secrets.env.example              # Optional local overrides (copy to secrets.env, gitignored)
├── webhook-config.yaml              # Admission webhook (MutatingWebhookConfiguration)
├── webhook-service.yaml             # Webhook Service (optional)
├── certs/                            # mTLS (Certificate / cert-manager)
│   ├── 01-root-ca.yaml
│   ├── 02-core-server-cert.yaml
│   └── 03-agent-client-cert.yaml
├── infrastructure/                   # Infrastructure
│   ├── postgresql-with-age.yaml     # PostgreSQL + PVC (recommended)
│   ├── postgresql.yaml              # PostgreSQL simple
│   ├── nats.yaml                    # NATS JetStream (StatefulSet 3 replica)
│   └── redis.yaml                   # Redis (optional)
└── e2e/                             # E2E / test
    ├── clear_all_cluster_data.sql
    ├── reset_database_full.sql
    └── *.yaml                       # Pod/test manifests
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

### Legacy (deprecated)

- `core-deployment.yaml`, `agent-daemonset.yaml` are no longer in the repo; use `fortuna-core-deployment.yaml` and `fortuna-agent-daemonset.yaml`.

### Monitoring

No monitoring stack (Grafana, Prometheus) is included. Core exposes `/metrics` for Prometheus scraping.

## Full pipeline (clean + rebuild + redeploy)

Build and deploy use **containerd / nerdctl**. See **`docs/05-operations/DEPLOYMENT_CONTAINERD.md`**. From repo root:

```bash
# Clean all old images + rebuild (nerdctl) + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh

# Also: delete DB data (DELETE, keep schema) then rebuild + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db

# Clean images + DB + rebuild + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db
# Or full reset DB: ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
```

If the script times out during deploy (e.g. Flannel step), ensure Core is deployed:

```bash
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl rollout status deployment/fortuna-core -n fortuna
```

Then run verification: `./scripts/verify/check-full-deployment.sh`, `./scripts/e2e/run-e2e-full.sh`.

## Cluster identity and multi-cluster sync

- **Agent:** Cluster identity (id/name) auto-discovered from K8s API (kube-system UID, `/version`). Optional override via ConfigMap `fortuna-cluster-config` (keys `cluster_id`, `cluster_name`) or env `CLUSTER_ID`/`CLUSTER_NAME`. Do **not** hardcode cluster id/name in deployments.
- **Core:** Single source of truth: creates cluster by id when missing, updates only mutable fields (name, source, k8s_version, distribution, last_sync) when id exists.
- **Migrations:** 054 adds cluster metadata columns; 055 drops UNIQUE on `clusters(name)` so multiple clusters can share a display name. Optional one-time rename: set Core env `CLUSTER_ID_TO_UPDATE` and `CLUSTER_DISPLAY_NAME` before starting (migration 053).
- **Management cluster:** Run Core, Dashboard, PostgreSQL, NATS, local Agent, and optional Falco.
- **Remote clusters:** Run Agent only. Point `CORE_HTTP_ENDPOINT` and `CORE_GRPC_ENDPOINT` to the management Core endpoint. Do not deploy Dashboard on workers or remote clusters in the local registryless flow.
- **Core external service:** Apply `deploy/fortuna-core-external-service.yaml` when remote agents need NodePort access. It exposes HTTP `30080` and gRPC mTLS `30090`.
- **Cleanup safety:** Pod ghost cleanup is scoped to the local cluster observed by Core. Remote cluster pods are removed by that cluster's own Agent sync or stale retention, not by the management cluster Kubernetes API.

Remote Agent example:

```bash
export REMOTE_KUBECONFIG=/path/to/remote.kubeconfig
export REMOTE_KUBECONFIGS="cluster02=${REMOTE_KUBECONFIG}"
export MANAGEMENT_NODE=<management-node-ip-or-dns>
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="latest"

./scripts/deploy/sync-remote-agent.sh
```

`sync-remote-agent.sh` runs Agent-only setup on each `REMOTE_KUBECONFIGS` entry: namespace, mTLS secrets, ingest token, RBAC, Agent DaemonSet, image, Core endpoints, and rollout. It does not set `CLUSTER_ID` by default; Agent auto-discovers a stable cluster ID. Use `REMOTE_IMAGE_MODE=local` when a registry is unavailable and the local Agent image must be imported to remote nodes through SSH. Registry image pull policy defaults to `Always` for `:latest`, otherwise `IfNotPresent`.

Verify DB/API/dashboard alignment:

```bash
FORTUNA_JWT="<admin-or-operator-jwt>" \
CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster101=/path/to/remote.kubeconfig" \
./scripts/verify/verify-multicluster-sync.sh
```

## Prerequisites and auto-handled steps

- **CNI (Flannel):** Pod network required (including for local-path-provisioner). If the cluster uses Flannel but it is not installed (empty `kube-flannel` namespace), pods stay **ContainerCreating** with `open /run/flannel/subnet.env: no such file or directory` and PVCs never bind.
  - **Auto:** `deploy-fortuna-robust.sh` and the full pipeline run `ensure-flannel.sh` **before** StorageClass. They install [Flannel](https://github.com/flannel-io/flannel) if missing (skipped if Calico/Cilium/Weave exist).
  - **Manual:** `./scripts/deploy/ensure-flannel.sh` or `kubectl apply -f https://raw.githubusercontent.com/flannel-io/flannel/v0.26.0/Documentation/kube-flannel.yml`
- **StorageClass `local-path`:** Required for PostgreSQL and NATS PVCs. If missing, PVCs stay **Pending**.
  - **Auto:** Scripts run `ensure-storage-class.sh` **after** Flannel. They install [Rancher local-path-provisioner](https://github.com/rancher/local-path-provisioner) if needed.
  - **Manual:** `./scripts/deploy/ensure-storage-class.sh` or `kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml`
- **Order:** Addons (kube-proxy, CoreDNS) → **Flannel CNI** → StorageClass → Deploy infra. Do not reorder.
- **Cluster addons:** The full pipeline ensures addons before deploy. Standalone deploy works if the cluster already has addons.

## Helm

Fortuna has a Helm chart at **`helm/fortuna/`**. Install or upgrade:

```bash
# Add repo (if published) or use local path
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace

# Override image tag / registry
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace \
  --set image.tag=v1.0.0 \
  --set image.registry=registry.company.com/fortuna
```

**Helm notes:**
- Chart does **not** deploy PostgreSQL/NATS. Deploy infra first: `kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml` and `deploy/infrastructure/nats.yaml` (same namespace).
- Create mTLS secrets first: `./scripts/utils/create_mtls_secret.sh`. Use [docs/05-operations/PRODUCTION_DEPLOYMENT.md](../docs/05-operations/PRODUCTION_DEPLOYMENT.md) for the current production flow.

---

## Quick deploy (kubectl)

**Recommended:** Use the robust deploy entrypoint so CNI, StorageClass, infra, mTLS, RBAC, control-plane label, and **prerequisites check** (secrets + postgres + nats) run in order; the script exits on first failure:

```bash
./scripts/deploy/deploy-fortuna-robust.sh
```

Standalone prerequisites check (before deploying Core/Agent manually): `./scripts/deploy/check-prerequisites-core-agent.sh` — verifies `fortuna-core-tls`, `fortuna-agent-tls`, PostgreSQL and NATS endpoints; fails fast with instructions if something is missing.

Manual (requires StorageClass `local-path`):

```bash
# 1. Create namespace
kubectl create namespace fortuna

# 2. Ensure StorageClass (if missing)
./scripts/deploy/ensure-storage-class.sh

# 3. Deploy certificates (or create mTLS: ./scripts/utils/create_mtls_secret.sh)
kubectl apply -f deploy/certs/

# 4. Deploy secrets
./scripts/utils/ensure-fortuna-secrets.sh fortuna

# 5. Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# 6. Deploy RBAC
kubectl apply -f deploy/fortuna-rbac.yaml

# 7. Deploy Core (file includes Service + Deployment)
kubectl apply -f deploy/fortuna-core-deployment.yaml

# 8. Deploy Agent
kubectl apply -f deploy/fortuna-agent-daemonset.yaml

# 9. Deploy Dashboard (apply nginx ConfigMap first)
kubectl apply -f deploy/dashboard-nginx-configmap.yaml
kubectl apply -f deploy/dashboard-deployment.yaml

# 10. Deploy webhook (optional)
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
- `FORTUNA_ALLOW_AUTH_QUERY_TOKEN`: Enable JWT `?token=` auth for browser WebSocket handshakes only. Required by the local dashboard WebSocket streams because browsers cannot attach an `Authorization` header to `new WebSocket(...)`.
- `FORTUNA_WS_ALLOWED_ORIGINS`: Comma-separated dashboard origins allowed to open WebSocket streams, for example `http://localhost:8081,http://127.0.0.1:8081,http://192.168.56.100:8081,http://192.168.56.100:30956`. Include the actual browser origin, including NodePort, or WebSocket handshakes will be rejected with 403 by Core.
- `PCE_SCHEDULER_ENABLED`: Enable PCE scheduler (default: `true`)
- `PCE_SCHEDULER_INTERVAL`: Scheduler interval (default: `6h`)
- **Per-cluster rate limit:** `RATE_LIMIT_PER_CLUSTER_ENABLED` (default: `true`), `RATE_LIMIT_SYNC_PER_CLUSTER_RPS` (default: `10`), `RATE_LIMIT_SYNC_PER_CLUSTER_BURST` (default: `20`), `RATE_LIMIT_SBOM_PER_CLUSTER_RPS` (default: `50`), `RATE_LIMIT_SBOM_PER_CLUSTER_BURST` (default: `100`). When enabled, sync and SBOM ingest are limited per `cluster_id` to avoid one noisy cluster impacting others.

**Agent**:
- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint
- **Cluster identity**: Auto-discovered from K8s API (kube-system UID hash + kubeconfig name). No hardcoded default. Optional override via ConfigMap `fortuna-cluster-config` keys `cluster_id`, `cluster_name`.
- `CLUSTER_ID` / `CLUSTER_NAME`: Optional. If set (e.g. from ConfigMap), agent uses them and logs `source=env_override`; else auto-discovery and logs `source=auto`.
- `SYNC_INTERVAL`: Sync interval (default: `5m`)
- `TLS_ENABLED`: Enable mTLS (default: `true`)
- **SBOM cache:** `SBOM_CACHE_DIR`: Directory for on-disk SBOM cache by image digest + signature version (default: `/var/lib/fortuna/sbom-cache`). When set and writable, agent reuses cached SBOM for the same digest to avoid re-scanning. Set to `0`, `off`, or `disabled` to disable cache.

## Image Pull Policy

For local development with containerd:
- Keep `imagePullPolicy: IfNotPresent`.
- Build and load the same `fortuna-*:<VERSION>` tags referenced by `deploy/*.yaml`.
- The main pipeline syncs manifest image tags to `VERSION` after a successful build by default.

For production:
- Set `imagePullPolicy: IfNotPresent` or `Always`
- Use immutable registry tags or digests. Published GHCR tags are `latest`/`sha-<12-char-commit>` from `main`, `v*` release tags, or a manual workflow `version`.
- Do not assume local `git describe` tags in `deploy/*.yaml` exist in GHCR unless the image publish workflow has published the same value.

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

## Operations, monitoring & troubleshooting

### Monitor

| Purpose | Command |
|---------|--------|
| Pod status + health + recent logs | `./scripts/monitor/monitor-agent-core.sh` |
| Core/Agent errors and warnings only | `./scripts/monitor/monitor-agent-core-errors.sh` |
| Follow errors continuously | `./scripts/monitor/monitor-agent-core-errors.sh --follow` |
| Core logs | `kubectl logs -n fortuna -l app.kubernetes.io/component=core -f --tail=300` |
| Agent logs (one pod) | `kubectl logs -n fortuna <agent-pod> -f --tail=300` |

### Troubleshooting

| Symptom | Action |
|---------|--------|
| Core/Agent **ContainerCreating** (secret not found) | Create mTLS: `./scripts/utils/create_mtls_secret.sh` (deploy script Step 7c does this) |
| Core pod **Pending** (0 nodes available, node affinity) | Add control-plane label: `./scripts/deploy/ensure-control-plane-label.sh` or `kubectl label node <master> node-role.kubernetes.io/control-plane=` |
| Agent **CrashLoopBackOff**, **OOMKilled** (Exit 137) | DaemonSet has 2Gi limit; if still OOM set env `SBOM_WORKERS=1` (rebuild agent) then `kubectl rollout restart daemonset/fortuna-agent -n fortuna` |
| **ErrImageNeverPull** (Core/Agent) | Preferred: set image to a registry tag reachable by every node. Registryless fallback: push images to all nodes with `./scripts/utils/push-images-to-workers.sh` (use `scripts/utils/push-images.config` or SSH_USER/SSH_PASS). |
| Agent **Sync failed: status=500** / Core log `column "kubeconfig" does not exist` | Run pipeline with `--db-reset` then redeploy (Core runs migrations) |
| Agent **no such host** / cannot reach Core | Check DNS and endpoint: `./scripts/verify/verify-agent-core-connectivity.sh` |

For broader operational checks, see [docs/05-operations/DEPLOYMENT.md](../docs/05-operations/DEPLOYMENT.md).

## mTLS certificate rotation

Certificates from `create_mtls_secret.sh` are valid **365 days**. Rotate **before expiry** (e.g. within 30 days of expiration) to avoid connection failures.

**Steps:**

1. **Generate new certs** (from repo root):  
   `NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh`  
   This writes to `.certs/` and creates/updates K8s secrets.

2. **Update secrets in cluster:**  
   `kubectl apply -f .certs/` (or use the script’s `kubectl create secret ... --dry-run=client -o yaml | kubectl apply -f -` if it outputs to stdout).

3. **Rollout Core** so it loads new server cert (Core uses file-based reload; restart ensures clean load):  
   `kubectl rollout restart deployment/fortuna-core -n fortuna`

4. **Rollout Agent** so it loads new client cert (Agent loads certs at connect time):  
   `kubectl rollout restart daemonset/fortuna-agent -n fortuna`

5. **Verify:** Core and Agent logs should show successful gRPC connection; no TLS handshake or “certificate expired” errors.

**One-liner:** `NAMESPACE=fortuna ./scripts/utils/rotate_mtls_secret.sh` — generates new certs, applies secrets, and rollout restarts Core + Agent.

**Optional:** Use cert-manager (Certificate + Issuer) for automatic renewal if your cluster already standardizes on cert-manager.

---

## Notes

- **Core:** Requires control-plane node (nodeSelector + tolerations). Missing label → pod Pending.
- **Agent:** DaemonSet on all nodes; memory limit 2Gi; use `SBOM_WORKERS=1` if OOM.
- **Dashboard:** Service LoadBalancer (can use NodePort/Ingress). Apply nginx ConfigMap before deploy.
- **mTLS:** All Core–Agent communication uses mTLS; do not skip creating secrets.
- **Order:** Flannel → StorageClass → Infra (Postgres, NATS) → mTLS → RBAC → control-plane label → Core → Agent → Dashboard.
- **Troubleshooting:** see the [Troubleshooting](#troubleshooting) section above.
