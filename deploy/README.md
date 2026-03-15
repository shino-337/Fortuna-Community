# FortunaK8s Deployment

Kubernetes manifests for **FortunaK8s** (Core, Agent, Dashboard, infrastructure). Deploy with **kubectl apply** or **Helm** (see [Helm](#helm)).

---

## Deploy checklist

### Before deploy

| Item | Notes |
|------|--------|
| **mTLS** | Required secrets: `fortuna-core-tls`, `fortuna-agent-tls`, `fortuna-ca-cert`, `fortuna-webhook-tls`. Create with: `NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh` (from repo root). Certificates are written to `.certs/` (gitignored), then created in the cluster. Both Helm and kubectl need this step first. |
| **Order** | CNI (Flannel) → StorageClass (`local-path`) → PostgreSQL + NATS → mTLS secrets → RBAC → control-plane label → Core → Agent → Dashboard. Wrong order can leave pods Pending or ContainerCreating. |
| **Namespace** | Default `fortuna`. If you use another namespace, update all manifests or use Helm with `-n <ns>` and `namespaceOverride`. |
| **PostgreSQL / NATS** | Must run in the same namespace (or set Core `DATABASE_URL` / `NATS_ENDPOINT` accordingly). The Helm chart does **not** deploy infra; run `kubectl apply -f deploy/infrastructure/...` first. |

### During deploy

| Item | Notes |
|------|--------|
| **Core** | Schedules only on **control-plane** nodes (nodeSelector + tolerations). If no node has `node-role.kubernetes.io/control-plane`, Core stays **Pending**. Run: `./scripts/deploy/ensure-control-plane-label.sh`. |
| **Agent** | DaemonSet on all nodes; needs containerd socket. Memory limit 2Gi; if OOM, set env `SBOM_WORKERS=1` (requires image rebuild). |
| **Dashboard** | Service type **LoadBalancer**; change to NodePort/ClusterIP or use Ingress as needed. Apply ConfigMap `fortuna-dashboard-nginx` **before** the Dashboard deployment (proxies `/api` to Core). |
| **Images** | Dev/local: `imagePullPolicy: Never` and build on node (nerdctl/containerd). Production: use a versioned tag (e.g. `v1.0.0`), `IfNotPresent` or `Always`, and a registry. Multi-node: use `./scripts/utils/push-images-to-workers.sh` or a registry. |
| **Auth** | Default admin `admin` / `admin123` in manifests. **Production:** change password and/or use a Secret (e.g. `fortuna-secrets` with `jwt-secret`, `admin-password`); Core supports `secretKeyRef` for JWT. |
| **NVD API key** | Optional. Core uses NVD API as CVE fallback; without key, rate limit is low (429 possible). Set in Secret `fortuna-secrets` key `nvd-api-key`, or env `NVD_API_KEY`. Example: `kubectl patch secret fortuna-secrets -n fortuna -p '{"stringData":{"nvd-api-key":"YOUR_NVD_KEY"}}'` then rollout restart Core. |

### After deploy

| Item | Notes |
|------|--------|
| **Verify** | Run `./scripts/verify/check-full-deployment.sh` (set `NAMESPACE` if not `fortuna`). |
| **Troubleshooting** | See [Troubleshooting](#troubleshooting) table and [docs/AGENT_CORE_ERRORS_MONITOR.md](../docs/AGENT_CORE_ERRORS_MONITOR.md). |
| **Access** | Dashboard: `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80` → http://localhost:8081. Core API: `kubectl port-forward -n fortuna svc/fortuna-core 8080:8080` → http://localhost:8080/health. |

---

## Directory structure

```
deploy/
├── fortuna-core-deployment.yaml     # Core: Service + Deployment (mTLS, auth, PCE)
├── fortuna-agent-daemonset.yaml     # Agent DaemonSet (containerd, mTLS)
├── dashboard-deployment.yaml        # Dashboard: Deployment + Service (LoadBalancer)
├── dashboard-nginx-configmap.yaml   # Nginx config (proxy /api → Core)
├── fortuna-rbac.yaml                # RBAC: ServiceAccount, ClusterRole, ClusterRoleBinding (core + agent)
├── core-secrets.yaml                # Secrets template (JWT, DB…)
├── webhook-config.yaml              # Admission webhook (MutatingWebhookConfiguration)
├── webhook-service.yaml             # Webhook Service (optional)
├── certs/                            # mTLS (Certificate / cert-manager)
│   ├── 01-root-ca.yaml
│   ├── 02-core-server-cert.yaml
│   └── 03-agent-client-cert.yaml
├── infrastructure/                   # Hạ tầng
│   ├── postgresql-with-age.yaml     # PostgreSQL + PVC (khuyến nghị)
│   ├── postgresql.yaml              # PostgreSQL đơn giản
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
# Clean toàn bộ image cũ + rebuild (nerdctl) + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh

# Thêm: xóa dữ liệu DB (DELETE, giữ schema) rồi rebuild + deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db

# Clean images + DB + rebuild + deploy (script cũ, tùy chọn)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db
# Hoặc full reset DB: ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
```

If the script times out during deploy (e.g. Flannel step), ensure Core is deployed:

```bash
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl rollout status deployment/fortuna-core -n fortuna
```

Then run verification: `./scripts/verify/check-full-deployment.sh`, `./scripts/e2e/run-e2e-full.sh`.

## Cluster identity (auto-discovery & SSOT)

- **Agent:** Cluster identity (id/name) auto-discovered from K8s API (kube-system UID, `/version`). Optional override via ConfigMap `fortuna-cluster-config` (keys `cluster_id`, `cluster_name`) or env `CLUSTER_ID`/`CLUSTER_NAME`. Do **not** hardcode cluster id/name in deployments.
- **Core:** Single source of truth: creates cluster by id when missing, updates only mutable fields (name, source, k8s_version, distribution, last_sync) when id exists.
- **Migrations:** 054 adds cluster metadata columns; 055 drops UNIQUE on `clusters(name)` so multiple clusters can share a display name. Optional one-time rename: set Core env `CLUSTER_ID_TO_UPDATE` and `CLUSTER_DISPLAY_NAME` before starting (migration 053). See `docs/03-components/dashboad/CLUSTER_IDENTITY_FLOW.md`.

## Prerequisites and auto-handled steps

- **CNI (Flannel):** Pod network required (including for local-path-provisioner). If the cluster uses Flannel but it is not installed (empty `kube-flannel` namespace), pods stay **ContainerCreating** with `open /run/flannel/subnet.env: no such file or directory` and PVCs never bind.
  - **Auto:** `deploy-fortuna-robust.sh` (Step 3a) and pipeline (Phase 2a2) run `ensure-flannel.sh` **before** StorageClass. They install [Flannel](https://github.com/flannel-io/flannel) if missing (skipped if Calico/Cilium/Weave exist).
  - **Manual:** `./scripts/deploy/ensure-flannel.sh` or `kubectl apply -f https://raw.githubusercontent.com/flannel-io/flannel/v0.26.0/Documentation/kube-flannel.yml`
- **StorageClass `local-path`:** Required for PostgreSQL and NATS PVCs. If missing, PVCs stay **Pending**.
  - **Auto:** Scripts run `ensure-storage-class.sh` **after** Flannel (Step 3b / Phase 2c). They install [Rancher local-path-provisioner](https://github.com/rancher/local-path-provisioner) if needed.
  - **Manual:** `./scripts/deploy/ensure-storage-class.sh` or `kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml`
- **Order:** Addons (kube-proxy, CoreDNS) → **Flannel CNI** → StorageClass → Deploy infra. Do not reorder.
- **Cluster addons:** Pipeline ensures addons before deploy (Phase 2a). Standalone deploy works if the cluster already has addons.

## Helm

Fortuna có Helm chart tại **`helm/fortuna/`**. Cài đặt hoặc nâng cấp:

```bash
# Thêm repo (nếu publish) hoặc dùng path local
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace

# Override image tag / registry
helm upgrade --install fortuna ./helm/fortuna -n fortuna --create-namespace \
  --set image.tag=v1.0.0 \
  --set image.registry=registry.company.com/fortuna
```

**Helm notes:**
- Chart does **not** deploy PostgreSQL/NATS. Deploy infra first: `kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml` and `deploy/infrastructure/nats.yaml` (same namespace).
- Create mTLS secrets first: `./scripts/utils/create_mtls_secret.sh`. See [helm/fortuna/README.md](../helm/fortuna/README.md).

---

## Quick deploy (kubectl)

**Recommended:** Use the single entrypoint so CNI, StorageClass, infra, mTLS, RBAC, control-plane label, and **prerequisites check** (secrets + postgres + nats) run in order; the script exits on first failure (Finding #7.1–7.2):

```bash
./scripts/deploy/deploy-full.sh
```

Or run the robust deploy script directly (same sequence, including Step 7d prerequisites check before Core/Agent):

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

# 3. Deploy infrastructure
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

# 4. Deploy certificates (or create mTLS: ./scripts/utils/create_mtls_secret.sh)
kubectl apply -f deploy/certs/

# 5. Deploy secrets (if using core-secrets)
kubectl apply -f deploy/core-secrets.yaml

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
- `PCE_SCHEDULER_ENABLED`: Enable PCE scheduler (default: `true`)
- `PCE_SCHEDULER_INTERVAL`: Scheduler interval (default: `6h`)
- **Per-cluster rate limit (Finding #6):** `RATE_LIMIT_PER_CLUSTER_ENABLED` (default: `true`), `RATE_LIMIT_SYNC_PER_CLUSTER_RPS` (default: `10`), `RATE_LIMIT_SYNC_PER_CLUSTER_BURST` (default: `20`), `RATE_LIMIT_SBOM_PER_CLUSTER_RPS` (default: `50`), `RATE_LIMIT_SBOM_PER_CLUSTER_BURST` (default: `100`). When enabled, sync and SBOM ingest are limited per `cluster_id` to avoid one noisy cluster impacting others. See `docs/02-architecture/DEPLOYMENT_AND_ARCHITECTURE_FAQ.md`.

**Agent**:
- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint
- **Cluster identity**: Auto-discovered from K8s API (kube-system UID hash + kubeconfig name). No hardcoded default. Optional override via ConfigMap `fortuna-cluster-config` keys `cluster_id`, `cluster_name` (see `CLUSTER_IDENTITY_FLOW.md`).
- `CLUSTER_ID` / `CLUSTER_NAME`: Optional. If set (e.g. from ConfigMap), agent uses them and logs `source=env_override`; else auto-discovery and logs `source=auto`.
- `SYNC_INTERVAL`: Sync interval (default: `5m`)
- `TLS_ENABLED`: Enable mTLS (default: `true`)
- **SBOM cache (Finding #8.5):** `SBOM_CACHE_DIR`: Directory for on-disk SBOM cache by image digest + signature version (default: `/var/lib/fortuna/sbom-cache`). When set and writable, agent reuses cached SBOM for the same digest to avoid re-scanning. Set to `0`, `off`, or `disabled` to disable cache.

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
| **ErrImageNeverPull** (Core/Agent) | Push images to all nodes: `./scripts/utils/push-images-to-workers.sh` (use `scripts/utils/push-images.config` or SSH_USER/SSH_PASS) |
| Agent **Sync failed: status=500** / Core log `column "kubeconfig" does not exist` | Run pipeline with `--db-reset` then redeploy (Core runs migrations) |
| Agent **no such host** / cannot reach Core | Check DNS and endpoint: `./scripts/verify/verify-agent-core-connectivity.sh` |

Full details: [docs/AGENT_CORE_ERRORS_MONITOR.md](../docs/AGENT_CORE_ERRORS_MONITOR.md).

## mTLS certificate rotation (Finding #4.1)

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

**One-liner (Finding #4.3):** `NAMESPACE=fortuna ./scripts/utils/rotate_mtls_secret.sh` — generates new certs, applies secrets, and rollout restarts Core + Agent.

**Optional:** Use cert-manager (Certificate + Issuer) for automatic renewal; see `docs/02-architecture/Architecture_Finding_Remediation_Plan.md` Finding #4.2.

---

## Notes

- **Core:** Requires control-plane node (nodeSelector + tolerations). Missing label → pod Pending.
- **Agent:** DaemonSet on all nodes; memory limit 2Gi; use `SBOM_WORKERS=1` if OOM.
- **Dashboard:** Service LoadBalancer (can use NodePort/Ingress). Apply nginx ConfigMap before deploy.
- **mTLS:** All Core–Agent communication uses mTLS; do not skip creating secrets.
- **Order:** Flannel → StorageClass → Infra (Postgres, NATS) → mTLS → RBAC → control-plane label → Core → Agent → Dashboard.
- **Troubleshooting:** [docs/AGENT_CORE_ERRORS_MONITOR.md](../docs/AGENT_CORE_ERRORS_MONITOR.md).
