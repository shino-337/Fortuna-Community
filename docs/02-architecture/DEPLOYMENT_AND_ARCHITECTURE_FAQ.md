# Deployment & Architecture FAQ

**Purpose:** Answer common deployment-scenario, communication, K8s resources, secrets/certs, and scale/HA questions from a single reference.

---

## 1. Deployment scenario: single-tenant vs shared control plane vs both?

**Current design:** **Shared control plane managing multiple clusters** is supported; single-cluster is a subset.

| Scenario | Supported today | Notes |
|----------|-----------------|--------|
| **Single cluster, one control plane** | ✅ | One Core + one (or more) clusters. Core runs on control-plane node(s); Agent DaemonSet per cluster. |
| **Shared control plane, multiple clusters** | ✅ | One Core instance, one PostgreSQL, one NATS. Each cluster runs its own Agent Daemonset; Agents report with `cluster_id` (auto or env). Core is SSOT: creates/updates cluster by id. Dashboard shows cluster selector when >1 cluster. See [Cluster Identity Flow](../03-components/dashboad/CLUSTER_IDENTITY_FLOW.md). |
| **Multiple control planes (e.g. per-tenant Core)** | ⚠️ Not documented | Would require separate Core/DB/NATS per tenant and possibly routing; not the current deployment model. |

**Recommendation:** Focus on **shared control plane + multiple clusters** if you need multi-cluster; otherwise **single control plane for one cluster** is the default and simplest.

---

## 2. How do Agent / Core / Dashboard communicate? Multi-cluster?

### Existing diagram (from `docs/README.md`)

```
┌─────────────┐     HTTP /api      ┌─────────────┐
│  Dashboard  │ ◄────────────────► │    Core     │ (Deployment, control-plane)
│ (React/Vite)│                    │  - Storage  │
│ Risk Center │                    │  - CVE Match│
│ SBOM, PCE   │                    │  - REST API │
└─────────────┘                    └──────┬──────┘
                                         │
┌─────────────┐     gRPC (mTLS)          ├──► PostgreSQL (Database)
│   Agent     │ ─────────────────────────►│
│ (DaemonSet  │                           └──► NATS JetStream (Events)
│  per node)  │
│ - Pod Watch │
│ - SBOM Ext  │
└─────────────┘
```

### Transport summary

| Channel | Protocol | TLS / mTLS | Purpose |
|---------|----------|------------|---------|
| **Dashboard → Core** | REST (HTTP) | TLS via proxy/Ingress | All API calls; JWT when `AUTH_ENABLED=true`. |
| **Agent → Core** | gRPC | mTLS (TLS 1.3, client cert required) | SBOM ingest, pod sync, pod-detail ingest (HTTP POST to Core also used for some endpoints). |
| **Core → NATS** | Client (JetStream) | Depends on NATS config | Events: SBOM created, insights updated, internal ingest queues. |
| **Core → PostgreSQL** | TCP (DB driver) | Optional (connection string) | Persistence. |
| **Dashboard → Core (real-time)** | WebSocket | Same as HTTP (upgrade) | `/api/v1/ws/risks`, `/api/v1/ws/pod/:uid` for live updates. |

References: `docs/02-architecture/COMPONENTS.md`, `docs/02-architecture/API-ARCHITECT_AND_ROUTE_STANDARD.md`, `docs/03-components/WebSocket-Flows-Sync.md`, `docs/AGENT_CORE_CONNECTIVITY.md`.

### Multi-cluster awareness

- **Agent:** Sends `cluster_id` (and optional `cluster_name`) in every sync payload. Default: auto from `sha256(kube-system namespace UID)`; optional override via `CLUSTER_ID` / `CLUSTER_NAME` (ConfigMap/env).
- **Core:** Single source of truth. On first seen `cluster_id` → insert row in `clusters`; on subsequent syncs → update mutable fields only (name, k8s_version, distribution, last_sync). “Active” cluster = last_sync within 7 days (single constant).
- **Dashboard:** Reads clusters and data only from Core API (`/clusters`, `/clusters/stats`, `/dashboard/stats`); no hardcoded cluster. If multiple clusters, UI shows cluster selector.

No separate “multi-cluster diagram” exists; the same flow applies with multiple Agent DaemonSets (one per cluster) reporting to one Core.

---

## 3. Exact Kubernetes resources Fortuna creates or watches

### Created by Fortuna (from `deploy/`)

| Resource | File(s) | Purpose |
|----------|---------|---------|
| **Deployment** | `fortuna-core-deployment.yaml`, `dashboard-deployment.yaml` | Core, Dashboard. |
| **DaemonSet** | `fortuna-agent-daemonset.yaml` | Agent on every node. |
| **Service** | In same YAMLs + `webhook-service.yaml` | Core, Dashboard, Webhook. |
| **ServiceAccount** | `fortuna-rbac.yaml` | `fortuna-core`, `fortuna-agent`. |
| **ClusterRole** | `fortuna-rbac.yaml` | Read-only: pods, services, namespaces, serviceaccounts, roles, rolebindings, clusterroles, clusterrolebindings, networkpolicies; Agent also exec for Pod Detail. |
| **ClusterRoleBinding** | `fortuna-rbac.yaml` | Bind ClusterRoles to ServiceAccounts. |
| **ConfigMap** | `dashboard-nginx-configmap.yaml` | Nginx config (proxy `/api` to Core). |
| **ValidatingWebhookConfiguration** | `webhook-config.yaml` | Policy admission for pods, deployments, statefulsets, daemonsets, replicasets (only in namespaces with `fortuna.io/policy-enabled=true`). |
| **Secrets** | Created by script / external | mTLS and webhook TLS (see below). |

### Watched / read by Core or Agent (no CRDs)

- **Core (RBAC):** pods, services, namespaces, serviceaccounts, roles, rolebindings, clusterroles, clusterrolebindings, networkpolicies (get/list/watch).
- **Agent (RBAC):** pods, nodes, serviceaccounts, roles, rolebindings, clusterroles, clusterrolebindings, etc., plus containerd/runtime and exec for Pod Detail.

### Not used

- **No CRDs** (no CustomResourceDefinitions) are defined or watched.
- **No MutatingWebhookConfiguration** in repo; only **ValidatingWebhookConfiguration** (`fortuna-policy-webhook`).
- **Admission:** One validating webhook, path `/admission/validate`, HTTPS on Core port 8443 (or fallback HTTP for dev). Optional: enabled only when TLS certs are present and optionally when namespace has label `fortuna.io/policy-enabled=true`.

Webhook details: `deploy/webhook-config.yaml`, `core/internal/webhook/admission.go`, `core/cmd/main.go` (Phase 2.7).

---

## 4. Secrets and certificates: provisioning and rotation

### Core–Agent mTLS

| Item | How it works |
|------|-------------------------------|
| **Provisioning** | Script `scripts/utils/create_mtls_secret.sh` (OpenSSL). Generates CA + server cert (Core) + client cert (Agent); creates K8s secrets: `fortuna-ca-cert`, `fortuna-core-tls`, `fortuna-agent-tls`. |
| **Validity** | 365 days (script default). |
| **Rotation** | **Manual:** re-run script, then restart Core and Agent so they load new certs. **Core** uses `pkg/security.CertManager` for dynamic reload (hot reload on file change); **Agent** loads certs at connect time (restart to pick new certs). No cert-manager or automatic rotation in repo. |
| **Paths** | Core: env `TLS_CERT_PATH`, `TLS_KEY_PATH`, `TLS_CA_CERT_PATH`. Agent: mounted from secrets (e.g. `fortuna-agent-tls`, `fortuna-ca-cert`). |

Refs: `scripts/utils/create_mtls_secret.sh`, `core/internal/grpc/server.go`, `agent/internal/client/grpc_client_mtls.go`, `docs/AGENT_CORE_CONNECTIVITY.md`.

### Webhook TLS

- **Secret:** `fortuna-webhook-tls` (script reuses server cert/key).
- **Core:** Serves webhook on 8443 with `WEBHOOK_TLS_CERT_PATH` / `WEBHOOK_TLS_KEY_PATH` (e.g. `/etc/webhook/certs/tls.crt|tls.key`).
- **Rotation:** Same as Core mTLS: regenerate and update secret, restart Core.

### Dashboard / API auth

- **JWT:** Configurable; default admin credentials in manifests. Production: use Secret (e.g. `jwt-secret`, `admin-password`) and `secretKeyRef` in Core deployment.
- **No built-in certificate-based auth for Dashboard;** TLS is typically at proxy/Ingress.

Refs: `deploy/README.md`, `core/internal/config/config.go`, `deploy/fortuna-core-deployment.yaml`.

---

## 5. Scale expectations and HA/DR

### Scale (from docs)

| Dimension | Stated target / note |
|-----------|------------------------|
| **Nodes** | 20–200 Kubernetes nodes (API/route standard). Some specs mention 50–200 nodes. |
| **Pods** | 10k+ pods per cluster (pod sync spec). |
| **SBOM / events** | Current sync is polling (e.g. 30s). Specs note that ingest at “10k events/min” / 1000 nodes would need async worker pool and NATS offload; dedup today is per-Core in-memory (multi-replica can duplicate). |

**Per-cluster rate limit (Finding #6.2):** When enabled (default), sync and SBOM ingest are limited per `cluster_id` (env: `RATE_LIMIT_SYNC_PER_CLUSTER_RPS`, `RATE_LIMIT_SBOM_PER_CLUSTER_RPS`, etc.). See deploy/README.md Configuration.

Refs: `docs/02-architecture/API-ARCHITECT_AND_ROUTE_STANDARD.md`, `docs/03-components/podDetail/POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC.md`, `docs/03-components/podDetail/Pod_Details_Processing_Flow_Technical_Specifications.md`.

### HA (High Availability)

| Component | Current expectation |
|-----------|--------------------|
| **Core** | Deployment (replicas ≥1). Shared PostgreSQL and NATS. Stateless API with shared DB; in-memory dedup and WebSocket hubs are not shared across replicas (documented as future Redis/cache scale-out). |
| **NATS** | 3-replica JetStream cluster (from `deploy/infrastructure/nats.yaml` and docs). |
| **PostgreSQL** | Single primary in deploy examples; no built-in failover or read replicas in repo. |
| **Agent** | DaemonSet; no HA beyond “one pod per node.” |

**Core replicas: WebSocket, dedup, and sticky session (Finding #1.1)**  
WebSocket hubs (`/api/v1/ws/risks`, `/api/v1/ws/pod/:uid`) and in-memory deduplication (e.g. pod-detail ingest) are **per Core process**, not shared across replicas. If a replica fails or the load balancer routes a client to a different replica, WebSocket connections can drop (client must reconnect) and dedup may allow duplicate events across replicas. **Sticky session** (load balancer affinity or cookie) is **optional for correctness** but **recommended for UX**: it keeps a given client on the same Core replica so WebSocket and in-memory state stay consistent. Future work may add shared coordination (e.g. Redis or NATS pub/sub) for WebSocket and dedup.

Refs: `docs/02-architecture/COMPONENTS.md`, `docs/README.md`, `docs/03-components/risk-center/Risk-Center-Implementation-Plan-Detailed.md`, `docs/Fortuna–Principles_for_Non-Disruptive_Architecture_Improvements.md`.

### DR (Disaster recovery) / Geo

- **No geo-replication or formal DR procedure** is documented in the repo.
- **Data durability:** PostgreSQL + NATS JetStream with 3 replicas.
- **Backup/restore:** Not specified; would be standard DB and (if needed) NATS backup.
- **Insights/risk:** Described as “eventual consistency” (async workers); no multi-region replication configured.

Refs: `docs/03-components/risk-center/Risk-Center_Review_Answers.md`, `docs/03-components/podDetail/podDetail_QA.md`.

---

## Quick reference: where to find more

| Topic | Location |
|-------|----------|
| Cluster identity & multi-cluster | `docs/03-components/dashboad/CLUSTER_IDENTITY_FLOW.md` |
| Components & data flow | `docs/02-architecture/COMPONENTS.md`, `docs/README.md` |
| Agent–Core connectivity & mTLS | `docs/AGENT_CORE_CONNECTIVITY.md` |
| WebSocket flows | `docs/03-components/WebSocket-Flows-Sync.md` |
| API architecture | `docs/02-architecture/API-ARCHITECT_AND_ROUTE_STANDARD.md` |
| Deploy order & mTLS | `deploy/README.md`, `scripts/utils/create_mtls_secret.sh` |
| Webhook | `deploy/webhook-config.yaml`, `core/internal/webhook/admission.go` |
| Open QA (latency, NATS, failure tests) | `docs/02-architecture/QA.md` |
