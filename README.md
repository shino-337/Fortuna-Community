# Fortuna — Kubernetes Security Analysis & Management

![Fortuna Platform Integrity](docs/assets/screenshots/platform-integrity.png)

**Enterprise Kubernetes security platform** providing real-time threat detection, vulnerability management, attack path analysis, and risk-based prioritization across multi-node and multi-cluster Kubernetes environments.
Fortuna combines static SBOM/CVE analysis with runtime behavioral signals (Falco, eBPF, process snapshots) to deliver contextual, actionable security insights — from individual pod risk scores to cluster-wide attack path graphs.

The project name is **Fortuna**. The public repository is `shino-337/Fortuna-Community`, and published GHCR images use the lowercase repository namespace `ghcr.io/shino-337/fortuna-community`.

**Project metadata:** [Apache-2.0 license](LICENSE) · [Security policy](SECURITY.md) · [Contributing guide](CONTRIBUTING.md) · [Code of conduct](CODE_OF_CONDUCT.md)

**New to the project?** Open the [documentation hub](docs/README.md), then [Getting started](docs/01-getting-started/README.md) and the [API reference](#api-reference) (REST/gRPC overview + links to the full route standard).

---

## Getting started

| Guide | What it covers |
|-------|----------------|
| [docs/01-getting-started/README.md](docs/01-getting-started/README.md) | Index: quickstart, deployment, environment |
| [docs/01-getting-started/INSTALLATION.md](docs/01-getting-started/INSTALLATION.md) | Maintained install, runtime coverage, verification, and reset workflow |
| [docs/01-getting-started/QUICKSTART.md](docs/01-getting-started/QUICKSTART.md) | Short path to a running stack |
| [docs/04-user-guide/README.md](docs/04-user-guide/README.md) | Dashboard workspaces, roles, empty states, and navigation flow |
| [docs/04-user-guide/USE_CASES.md](docs/04-user-guide/USE_CASES.md) | Main operational use cases from platform health to reports |
| [docs/05-operations/PRODUCTION_DEPLOYMENT.md](docs/05-operations/PRODUCTION_DEPLOYMENT.md) | Production-style deploy and checklist |
| [docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md](docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md) | Hardware, OS, Kubernetes, tooling |

**On this page:** [Installation](#installation) (clone, mTLS, prerequisites) → [Quick Start](#quick-start) (published images first, local build second) → [Testing](#testing). For cluster operations after install, see [docs/05-operations/DEPLOYMENT.md](docs/05-operations/DEPLOYMENT.md).

---

## Key Capabilities

| Domain | Capabilities |
|--------|-------------|
| **Vulnerability Management** | SBOM extraction per pod, PURL-based CVE matching (OSV + Aikido malware feeds), trust-aware scoring, deterministic replay-safe processing |
| **Runtime Security** | Falco event ingestion (kmod/eBPF), custom eBPF syscall sensor, process snapshot diffing, network connection tracking |
| **Attack Path Analysis** | Automated graph construction (pod → SA → RBAC → escape surfaces), chain detection (ESCAPE_TO_PRIV_ESC, SA_TOKEN_REUSE, etc.), MITRE ATT&CK mapping |
| **Risk Scoring** | Multi-factor risk engine (V3): static config + CVE severity + runtime signals + attack path boost + MITRE correlation |
| **Pod Capability Engine (PCE)** | Static privilege analysis, runtime signal promotion (observed → confirmed → exploited), toxic combo detection |
| **Policy & Rules** | Detection rule templates, exception policies, promotion rules (signal → capability state transitions) |
| **Compliance & Audit** | Full audit trail, SBOM inventory per workload, risk reports with explainability factors |
| **Dashboard** | Role-aware React UI for platform integrity, operations dashboard, findings queue, attack paths, runtime network, inventory, policy rules, pipeline health, reports, and settings |

---

## Architecture

```mermaid
flowchart LR
  browser[Browser] --> dashboard[Dashboard nginx]
  dashboard --> core[Core REST API]
  agent[Agent DaemonSet] -->|gRPC mTLS| core
  remoteAgent[Remote cluster Agent] -->|NodePort/Ingress gRPC mTLS| core
  agent -->|HTTP ingest fallback| core
  falco[Optional Falco/eBPF sensors] --> agent
  core --> db[(PostgreSQL)]
  core <--> nats[(NATS JetStream)]
  workers[Core workers] <--> nats
  workers --> db
```

### Data Flow

1. **Agent** (DaemonSet on every node) collects SBOM, process snapshots, network connections, K8s events, and runtime signals.
2. **Core** receives data via gRPC (mTLS) and HTTP, stores in PostgreSQL, publishes events to NATS JetStream.
3. **Workers** consume events: CVE matching, risk scoring, attack path computation, insight correlation.
4. **Dashboard** queries Core REST API for visualization.
5. **Falco** (optional DaemonSet) generates kernel-level security events, written to hostPath JSONL, tailed by Agent's FalcoReader.

### Multi-Cluster Model

Fortuna runs one management stack per environment: Core, Dashboard, PostgreSQL, and NATS live in the management cluster. Each additional Kubernetes cluster runs only `fortuna-agent`, configured with a stable `CLUSTER_ID` / `CLUSTER_NAME` and a reachable Core endpoint.

| Scope | Runs |
|-------|------|
| Management cluster | Core, Dashboard, PostgreSQL, NATS, Agent, optional Falco |
| Remote cluster | Agent DaemonSet, optional Falco |

Core exposes `fortuna-core-external` when remote agents need to connect by NodePort. In local lab deployments this is commonly `http://<management-node>:30080` for HTTP ingest and `<management-node>:30090` for gRPC mTLS. Production should use an internal LoadBalancer or Ingress with stable DNS.

Verify synchronization with:

```bash
FORTUNA_JWT="<admin-or-operator-jwt>" \
CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster101=/path/to/cluster101.kubeconfig" \
./scripts/verify/verify-multicluster-sync.sh
```

The dashboard cluster totals are derived from active pods joined to active clusters, so `/clusters`, `/dashboard/stats`, and DB pod counts should agree after each agent full sync.

---

## Repository Structure

| Path | Purpose |
|------|---------|
| `agent/` | Agent daemon: SBOM extraction, process/network collectors, Falco reader, eBPF sensor, gRPC client |
| `core/` | Core backend: API server, repository, CVE matcher, workers, risk engine, attack path engine, DB migrations (`core/migrations/`) |
| `api/` | Shared protobuf contracts (`github.com/fortuna/api`) |
| `dashboard/` | React/Vite dashboard workspaces and shared UI primitives |
| `deploy/` | Kubernetes manifests: Core, Agent, Dashboard, PostgreSQL, NATS, RBAC, Falco Helm values, mTLS certs |
| `scenarios/` | Attack path E2E workloads (S1–S5): escape, RBAC escalation, noisy discovery, lateral movement, broken chain |
| `scripts/` | Build, deploy, verify, E2E, monitoring, and pipeline automation scripts |
| `docs/` | Public documentation: getting started, architecture, components, operations, user guide, and reference |

**Component catalog (domains and spec links):** [docs/03-components/README.md](docs/03-components/README.md)

---

## Features

### Vulnerability Management
- Automated SBOM extraction per pod image (syft-based, in-agent)
- PURL validation and canonicalization with ecosystem-aware normalization
- Trust-aware CVE matching: high/medium/low trust tiers with explainability
- OSV-backed CVE data in PostgreSQL with optional offline loaders (`scripts/utils/load-cve-data.sh`)
- Aikido malware feed integration (~122K packages, auto-synced every 6h)
- Deterministic resolver logic with stable ordering and replay guard
- SBOM confidence levels and audit trail

### Runtime Security
- **Falco integration**: kernel-level syscall monitoring via kmod driver, JSONL event ingestion
- **eBPF sensor**: lightweight syscall tracing (execve, connect, attach) with configurable flush intervals
- **Process snapshot diffing**: detects new processes in containers, generates `PROCESS_SNAPSHOT_DIFF` events
- **Network connection tracking**: per-pod TCP/UDP connections from `/proc/net`
- **Runtime signal promotion**: observed → confirmed → exploited state machine per capability

### Attack Path Analysis
- Automated graph construction: pod → serviceAccount → RBAC bindings → escape surfaces → cluster objectives
- Chain types: `ESCAPE_TO_PRIV_ESC`, `SA_TOKEN_REUSE`, `HOSTPATH_ESCAPE`, and more
- MITRE ATT&CK technique mapping and correlation precision scoring
- Blast radius analysis per resource
- Risk score boosting based on confirmed attack paths
- 5 built-in validation scenarios (S1–S5)

### Risk Scoring (V3)
- Multi-factor scoring: static config + CVE severity + runtime signals + attack path boost + MITRE correlation
- Per-pod risk profiles with temporal decay
- Historical risk evaluation and trend tracking
- Explainability: every score includes contributing factors breakdown
- Pod Capability Engine (PCE): 40+ capability definitions with severity grades

### Policy & Compliance
- Detection rule engine with templates and custom instances
- Exception policies for accepted risks
- Promotion rules: signal type → capability state transitions
- Full audit logging of all system actions
- Risk reports per pod with exportable data

---

## Dashboard

Role-aware React UI covering security operations:

| Workspace | Route | Purpose |
|-----------|-------|---------|
| **Platform Integrity** | `/#/` | Telemetry reliability, governance, runtime coverage, and platform health |
| **Operations Dashboard** | `/#/dashboard` | Executive risk, exposure, attack-path, and cluster posture summary |
| **Findings Queue** | `/#/risks/findings` | Unified-risk finding triage and workflow actions |
| **Attack Paths** | `/#/attack-paths` | Attack-path graph, RBAC escalation, lateral movement, and runtime attack steps |
| **Runtime Network** | `/#/network-activity` | Pod traffic, external destinations, and runtime network observations |
| **Kubernetes Inventory** | `/#/resources` | Inventory and pod detail entry point for SBOM, CVE, runtime, events, and spec |
| **Policy Rules** | `/#/rules` | Rule catalog metadata and linked findings |
| **Pipeline & Runtime Health** | `/#/monitoring` | Pipeline processing, runtime ingestion, Falco visibility, and freshness |
| **Reports** | `/#/reports` | Time-windowed operational summaries |
| **Settings** | `/#/settings` | Users, roles, sessions, and administrative controls |

More screenshots are in the [user guide](docs/04-user-guide/README.md#workspace-screenshots).

Login with `admin`. If `FORTUNA_ADMIN_PASSWORD` was set when creating `fortuna-secrets`, use that value. If it was omitted for a local bootstrap deploy, use `Fortuna_ChangeMe_123!`; Fortuna will require a password change before the dashboard is usable.

---

## System Requirements

### Cluster Requirements

| | Minimum (dev/test) | Recommended (production) |
|---|---|---|
| **Nodes** | 1 master + 1 worker | 1 master + 2+ workers |
| **CPU per node** | 2 vCPU | 4 vCPU |
| **Memory per node** | 4 GB | 8 GB |
| **Disk per node** | 20 GB | 50 GB |
| **Kubernetes** | v1.28+ | v1.29+ |
| **Container runtime** | containerd 1.7+ | containerd 2.0+ |
| **Kernel** | 5.10+ | 5.15+ (for eBPF/Falco kmod) |

### Component Resource Allocation

| Component | Replicas | CPU Req/Limit | Memory Req/Limit | Notes |
|-----------|----------|---------------|-------------------|-------|
| **fortuna-core** | 1 Deployment | 100m / 1000m | 256Mi / 1Gi | API + workers + migrations |
| **fortuna-agent** | 1 per node (DaemonSet) | 100m / 1000m | 1Gi / 6Gi | SBOM extraction is memory-intensive |
| **fortuna-dashboard** | 1 Deployment | 10m / 100m | 64Mi / 128Mi | Static React app (nginx) |
| **postgres** | 1 Deployment | 10m / 500m | 256Mi / 512Mi | Primary data store (AGE extension) |
| **nats** | 3 StatefulSet | 250m / 500m | 384Mi / 1Gi | JetStream event bus |
| **falco** (optional) | 1 per node (DaemonSet) | 10m / 500m | 64Mi / 512Mi | Runtime security (kmod driver) |

**Cluster totals (2-node, all components):** ~1.1 CPU requests / ~6.3 CPU limits, ~3.9 Gi memory requests / ~17.9 Gi memory limits.

### Storage

| Resource | Size | Notes |
|----------|------|-------|
| PostgreSQL database | ~2.3 GB | Grows with pods × CVEs × time |
| PostgreSQL PVC | 20 Gi | `local-path` StorageClass |
| NATS JetStream PVC (×3) | 10 Gi each | Event replay and stream persistence |
| Falco event log | < 1 MB/day | `/var/log/falco/events.jsonl` per node |
| Container images | agent 609 MB, core 268 MB, dashboard 76 MB | Per-node containerd storage |

### Network Ports

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| fortuna-core | 8080 | HTTP | REST API, health checks |
| fortuna-core | 9090 | gRPC (mTLS) | Agent registration, SBOM ingest |
| fortuna-dashboard | 80 | HTTP | Web UI |
| postgres | 5432 | TCP | Database (ClusterIP) |
| nats | 4222/6222/8222 | TCP | Client / cluster / monitoring |

---

## Installation

### Prerequisites

- Go 1.24+
- Docker or nerdctl/containerd toolchain only when building images locally
- Kubernetes cluster (v1.28+) — see [System Requirements](#system-requirements)
- `kubectl` configured for target cluster
- `helm` 3+ (optional, for Falco deployment — auto-installed by pipeline if missing)

### Clone

```bash
git clone https://github.com/shino-337/Fortuna-Community.git
cd Fortuna
```

### Go Workspace

```bash
go work use ./core ./agent ./api
```

### mTLS Certificates

Generate agent/core mTLS secrets (required for gRPC communication):

```bash
./scripts/utils/create_mtls_secret.sh
```

---

## Quick Start

This quick path deploys the Fortuna management stack into the `fortuna` namespace:

- PostgreSQL with Apache AGE support and NATS JetStream.
- Fortuna Core REST API and gRPC ingest service.
- Fortuna Agent DaemonSet on each node.
- Fortuna Dashboard.
- Core/Agent mTLS secrets and application secrets.

Use **Option 1** when you want to install a released package from GitHub/GHCR. Use **Option 2** or **Option 3** when changing source code or when your cluster cannot pull images from a registry.

### Option 1: Deploy Published Images (recommended for users)

GitHub Actions publishes images to GHCR on `main`, release tags, and manual dispatch:

```bash
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"
export FORTUNA_ADMIN_PASSWORD="<strong-admin-password>" # recommended; omit only for first-login bootstrap default
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
```

Create the namespace first. If GHCR packages are private, create `ghcr-pull` after the namespace exists. `GITHUB_TOKEN` needs `read:packages`; public packages can skip the docker-registry secret:

```bash
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -

kubectl -n fortuna create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USER" \
  --docker-password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Deploy manifests and point workloads at published images:

```bash
./scripts/deploy/ensure-storage-class.sh
NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
./scripts/utils/ensure-fortuna-secrets.sh fortuna

kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml

if kubectl -n fortuna get secret ghcr-pull >/dev/null 2>&1; then
  kubectl -n fortuna patch serviceaccount fortuna-core -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
  kubectl -n fortuna patch serviceaccount fortuna-agent -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
  kubectl -n fortuna patch serviceaccount default -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
fi

kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-deployment.yaml

kubectl -n fortuna set image deployment/fortuna-core core="${FORTUNA_REGISTRY}/fortuna-core:${FORTUNA_VERSION}"
kubectl -n fortuna set image daemonset/fortuna-agent agent="${FORTUNA_REGISTRY}/fortuna-agent:${FORTUNA_VERSION}"
kubectl -n fortuna set image deployment/fortuna-dashboard dashboard="${FORTUNA_REGISTRY}/fortuna-dashboard:${FORTUNA_VERSION}"
```

Anonymous pulls return `401 Unauthorized` when the GHCR package is private. Make the package public or use the pull secret above.

YAML examples for private package pulls and tag overrides are available under `deploy/samples/`.

Expected workloads:

| Workload | Type | Expected state |
|----------|------|----------------|
| `postgres` | Deployment | 1 ready pod |
| `nats` | StatefulSet | 3 ready pods |
| `fortuna-core` | Deployment | 1 ready pod, `/healthz` responds |
| `fortuna-agent` | DaemonSet | 1 ready pod per schedulable node |
| `fortuna-dashboard` | Deployment | 1 ready pod, service port `80` |

Wait and open the dashboard:

```bash
kubectl rollout status -n fortuna deployment/fortuna-core --timeout=180s
kubectl rollout status -n fortuna daemonset/fortuna-agent --timeout=180s
kubectl rollout status -n fortuna deployment/fortuna-dashboard --timeout=180s
kubectl get pods,svc -n fortuna -o wide
kubectl port-forward --address 0.0.0.0 -n fortuna svc/fortuna-dashboard 8081:80
```

Login with `admin`. If `FORTUNA_ADMIN_PASSWORD` was omitted on a fresh deployment, use `Fortuna_ChangeMe_123!`; the dashboard will require an immediate password change. If a previous DB already had an admin user, Core does not overwrite that password with the bootstrap default.

Published image names:

- `ghcr.io/shino-337/fortuna-community/fortuna-core:<tag>`
- `ghcr.io/shino-337/fortuna-community/fortuna-agent:<tag>`
- `ghcr.io/shino-337/fortuna-community/fortuna-dashboard:<tag>`

Valid published tags are created by `.github/workflows/publish-images.yml`:

- `latest` and `sha-<12-char-commit>` after each push to `main`.
- `vX.Y.Z` and `sha-<12-char-commit>` after pushing a `v*` release tag.
- The supplied `version` and `sha-<12-char-commit>` when running the workflow manually.

The checked-in `deploy/*.yaml` files may contain local containerd tags from the developer pipeline. For a registry-based install, always set the workload images to `FORTUNA_REGISTRY/FORTUNA_VERSION` as shown above.

Basic troubleshooting:

```bash
# Image pull or scheduling issues
kubectl describe pod -n fortuna -l app.kubernetes.io/name=fortuna

# Core and Agent logs
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100

# Health checks through port-forward
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080

# In another terminal:
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/ready
```

### Option 2: Developer Local Build

Use this when testing source changes or when you do not want to pull from a registry. For multi-node clusters, a registry is still the cleaner way to avoid copying/importing images on every worker.

Single command to clean, build, deploy, and verify:

```bash
export FORTUNA_PACKAGE_SOURCE=local
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset
```

With runtime security (Falco + eBPF):

```bash
export FORTUNA_PACKAGE_SOURCE=local
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset --with-runtime
```

The pipeline derives `VERSION` from Git, builds `fortuna-core`, `fortuna-agent`, and `fortuna-dashboard`, and syncs the matching `deploy/*.yaml` image tags after a successful build when `FORTUNA_PACKAGE_SOURCE=local`. Without that setting, the pipeline keeps using GitHub/GHCR packages through `FORTUNA_REGISTRY` and `FORTUNA_VERSION`.

For multi-cluster, do not deploy Core or Dashboard on remote clusters. Deploy or patch only the Agent there. The pipeline can sync remote Agent-only clusters when `REMOTE_KUBECONFIGS` is set:

```bash
REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig" \
MANAGEMENT_NODE=<management-node-ip-or-dns> \
FORTUNA_REGISTRY=ghcr.io/shino-337/fortuna-community \
FORTUNA_VERSION=v1.0.0 \
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
```

Use `REMOTE_IMAGE_MODE=local` for registryless labs. That imports only `fortuna-agent:$VERSION` into remote cluster nodes via SSH before rolling out the remote DaemonSet.

### Option 3: Step-by-step

```bash
export FORTUNA_PACKAGE_SOURCE=local

# 1. Build images
./scripts/build/build-and-load-containerd.sh

# 2. Multi-node only without a registry: push Core/Agent runtime images to cluster nodes
./scripts/utils/push-images-to-workers.sh

# 3. Deploy stack
./scripts/deploy/deploy-fortuna-robust.sh

# 4. Verify
./scripts/verify/check-full-deployment.sh

# 5. Access services
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
```

- Dashboard: http://localhost:8081
- Core API: http://localhost:8080/healthz

### First API Login

Most user-facing API routes require a JWT. After forwarding Core to `localhost:8080`, login once and export the token:

```bash
export FORTUNA_ADMIN_PASSWORD="${FORTUNA_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}"
export FORTUNA_JWT="$(
  curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"admin\",\"password\":\"${FORTUNA_ADMIN_PASSWORD}\"}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))'
)"
```

If the deployment used the bootstrap default and the login response indicates `mustChangePassword`, change it before calling data APIs:

```bash
export FORTUNA_NEW_ADMIN_PASSWORD="<new-strong-admin-password>"

curl -s -X POST http://127.0.0.1:8080/api/v1/change-password \
  -H "Authorization: Bearer ${FORTUNA_JWT}" \
  -H 'Content-Type: application/json' \
  -d "{\"oldPassword\":\"${FORTUNA_ADMIN_PASSWORD}\",\"newPassword\":\"${FORTUNA_NEW_ADMIN_PASSWORD}\"}"

export FORTUNA_ADMIN_PASSWORD="${FORTUNA_NEW_ADMIN_PASSWORD}"
export FORTUNA_JWT="$(
  curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"admin\",\"password\":\"${FORTUNA_ADMIN_PASSWORD}\"}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))'
)"
```

Use it with:

```bash
curl -H "Authorization: Bearer ${FORTUNA_JWT}" http://127.0.0.1:8080/api/v1/dashboard/stats
```

### Pipeline Options Reference

| Flag | Description |
|------|-------------|
| `--full` | Full pipeline: clean + rebuild + deploy |
| `--db-reset` | Drop all tables; Core re-runs all DB migrations on startup |
| `--db` | Clear data only (DELETE, keep schema) |
| `--with-falco` | Deploy Falco via Helm for runtime security events |
| `--with-ebpf` | Enable eBPF sensor on Agent |
| `--with-runtime` | Enable both Falco + eBPF |
| `--with-e2e` | Run E2E tests after deploy, generate report |
| `--e2e-suite=<name>` | E2E suite: `full-report` (default), `full`, `risk-center`, `runtime`, `pce`, `sbom`, etc. |
| `--e2e-cleanup` | Remove test namespaces and data after E2E report |
| `--only-e2e` | Skip clean/rebuild/deploy; run E2E only (cluster must already be up). Implies `--with-e2e`. |
| `--e2e-with-scenario` | With `full-report`, deploy `scenarios/` and run S1–S5 checks (default). |
| `--e2e-no-scenario` | With `full-report`, skip scenario deploy (faster API-only path). Env: `E2E_WITH_SCENARIO=false`. |
| `--only-core` | Rebuild + redeploy Core only |
| `--only-agent` | Rebuild + redeploy Agent only |
| `--only-dashboard` | Rebuild + redeploy Dashboard only |
| `--skip-rebuild` | Clean + deploy (use existing images) |
| `--menu` | Interactive menu with all options |

---

## Use Cases

The flows below assume:

- Dashboard is available at `http://127.0.0.1:8081`.
- Core is port-forwarded to `http://127.0.0.1:8080`.
- `FORTUNA_JWT` is exported from [First API Login](#first-api-login).
- Agent has completed at least one full sync. Check `/#/monitoring` or `kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100`.

Use the dashboard for investigation and the API examples for automation or verification.

### 1. Vulnerability Assessment

Detect and prioritize CVEs across all cluster workloads:

```bash
# After deployment, Agent auto-extracts SBOMs for all pods.
# Core matches CVEs using OSV-backed tables in PostgreSQL plus Aikido malware feeds.
# View results:
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/inventory/sbom

curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/risk/insights
```

Dashboard path:

1. Open `/#/risks/findings`.
2. Filter by severity, namespace, cluster, status, or source.
3. Open a finding drawer and review affected pod, CVE/package evidence, risk factors, and workflow status.
4. Jump to pod detail for SBOM components and runtime context.

Expected output: SBOM inventory should contain discovered images/components; Findings Queue should show active CVE, malware, policy, or runtime findings after data has been ingested.

### 2. Runtime Threat Detection

Monitor live container behavior for suspicious activity:

```bash
# Deploy with runtime sensors
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime

# Falco detects: container escapes, privilege escalation, suspicious file access, network anomalies
# Agent process diffing detects: new processes spawned in containers
# eBPF sensor captures: execve, connect, attach syscalls

# View runtime events per pod:
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/runtime/pods/<pod-uid>/events

# Runtime pipeline health:
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/runtime/signals
```

Dashboard path:

1. Open `/#/monitoring` and confirm Agent/Falco/runtime freshness.
2. Open `/#/resources`, select a pod, then inspect Runtime Events, Processes, Network, Events, and Spec tabs.
3. Open `/#/network-activity` for cluster-wide runtime network topology.

Expected output: process snapshots and network connections appear even without Falco; Falco/eBPF events appear only when runtime sensors are enabled and healthy.

### 3. Attack Path Analysis

Identify exploitable paths from compromised pods to cluster-level objectives:

```bash
# Deploy attack path validation scenarios
kubectl apply -f scenarios/

# View attack paths
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/graph/attack-paths/bundle

curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/graph/attack-paths/summary

# Per-pod attack paths
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/graph/attack-paths/<pod-uid>
```

Dashboard path:

1. Open `/#/attack-paths`.
2. Sort by priority or confidence.
3. Inspect graph nodes, edge labels, runtime evidence, RBAC links, and blast radius.
4. Open the source pod or linked finding to validate the underlying evidence.

Expected output: high-risk paths combine escape surfaces, privileged service accounts, RBAC reachability, runtime evidence, or toxic capability combinations. Scenario manifests under `scenarios/` can be used in non-production clusters to validate the graph.

### 4. Compliance & Audit

Track all security-relevant actions and maintain audit trails:

```bash
# Audit logs
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/audit/logs

# Risk reports
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/audit/reports

# Pod-level SBOM inventory
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/inventory/sbom
```

Dashboard path:

1. Open `/#/reports`.
2. Select a time window such as 1 day, 3 days, 7 days, or 30 days.
3. Review findings, workload changes, runtime events, and posture summary.
4. Export only after verifying filters and cluster scope.

Expected output: reports should align with Findings Queue and Inventory for the selected time window; audit logs show user and system actions relevant to security workflows.

### 5. Policy-Based Detection

Create custom detection rules and exception policies:

```bash
# List detection rules
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/policy/rules

# Risk rules with YAML support
curl -H "Authorization: Bearer ${FORTUNA_JWT}" \
  http://127.0.0.1:8080/api/v1/risk/rules
```

Dashboard path:

1. Open `/#/rules`.
2. Search by rule UID, category, severity, source, or name.
3. Open rule detail to review catalog metadata, matching behavior, affected findings, and linked capabilities.
4. Use Findings Queue to acknowledge, dismiss, resolve, or escalate matched findings according to role permissions.

Expected output: catalog-backed rules map to findings and capabilities; custom or YAML-backed rules should remain traceable through rule UID and evidence.

### 6. Multi-Cluster Visibility

Run one management stack and collect telemetry from remote clusters:

```bash
kubectl -n fortuna apply -f deploy/fortuna-core-external-service.yaml

REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig" \
MANAGEMENT_NODE=<management-node-ip-or-dns> \
FORTUNA_REGISTRY=ghcr.io/shino-337/fortuna-community \
FORTUNA_VERSION=v1.0.0 \
./scripts/deploy/sync-remote-agent.sh

FORTUNA_JWT="${FORTUNA_JWT}" \
CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig" \
./scripts/verify/verify-multicluster-sync.sh
```

Dashboard path:

1. Open `/#/` and verify platform integrity across clusters.
2. Open `/#/resources` and filter by cluster, namespace, pod, image, or risk.
3. Open `/#/attack-paths` and validate cluster scope before triage.

Expected output: management cluster runs Core/Dashboard/PostgreSQL/NATS; remote clusters run Agent only. Cluster totals in Platform Integrity, Inventory, and API stats should agree after the remote Agent full sync.

### 7. Operational Health and Upgrade Checks

Confirm a deployment is healthy before investigations, upgrades, or releases:

```bash
./scripts/verify/check-full-deployment.sh
./scripts/verify/verify-core-agent-rebuild-deploy-status.sh

kubectl get pods -n fortuna -o wide
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=100
```

Dashboard path:

1. Open `/#/` for telemetry reliability, governance, runtime coverage, and platform health.
2. Open `/#/monitoring` for pipeline activity, agent freshness, runtime ingestion, and sensor state.
3. Treat missing findings as inconclusive until ingestion and runtime coverage are healthy.

Expected output: Core, Dashboard, PostgreSQL, NATS, and all expected Agent pods are ready; image tags match the intended package version; pipeline freshness timestamps are recent.

---

## Attack Path Scenarios

Example manifests (S1–S5) live under `scenarios/` for exercising path detection in a non-production cluster. Summary, optional lab YAMLs, and teardown: [scenarios/README.md](scenarios/README.md).

```bash
kubectl apply -f scenarios/
kubectl wait --for=condition=Ready pod -l fortuna.io/e2e-scenario -n fortuna-test --timeout=180s
```

| ID | Manifest | Attack surface | Expected behavior |
|----|----------|----------------|---------------------|
| **S1** | `s1-escape-privesc.yaml` | HostPath `/` + cluster-admin SA | Highest risk: ESCAPE_HOSTPATH + PRIV_ESC chain |
| **S2** | `s2-rbac-only.yaml` | cluster-admin SA, no escape surface | RBAC-only escalation, no ESCAPE_PATH class |
| **S3** | `s3-noisy-discovery.yaml` | Network scanning (nmap/ping) | Noisy runtime → lower correlation precision |
| **S4** | `s4-token-lateral.yaml` | SA token reuse against API server | SA_TOKEN_REUSE technique detection |
| **S5** | `s5-broken-chain.yaml` | Default SA, no escalation | Minimal risk, capability gaps expected |

**Expected risk ordering:** S1 > S2 > S3/S4 > S5

---

## Automated checks

Contributors and CI use `go test` under `core/` and `agent/`, plus optional automation in `scripts/e2e/` and `scripts/pipeline/`. See [scripts/README.md](scripts/README.md) for script layout. Full cluster rebuild (optional flags): `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --help`.

---

## API reference

**Bases:** User-facing REST lives under **`/api/v1/`** on Core (default `8080`), authenticated with **JWT** (`Authorization: Bearer …`) except `POST /api/v1/auth/login` and `register`. Agents use **gRPC on `9090` (mTLS)** and selected **HTTP ingest** routes on `8080` (cluster-internal; not for browser use). Some runtime ingest uses **`/api/v2/runtime/events`**.

| Spec / index | Content |
|--------------|---------|
| [docs/02-architecture/API_STANDARD.md](docs/02-architecture/API_STANDARD.md) | Route domains, methods, auth model, versioning conventions |
| [docs/06-reference/README.md](docs/06-reference/README.md) | Reference index and security guidance |
| [docs/06-reference/SECURITY.md](docs/06-reference/SECURITY.md) | Security practices for operating the platform |

The tables below are a **condensed** overview; new endpoints are defined in `core/internal/api/` and documented in the API standard.

### Core REST API (`/api/v1/`)

| Group | Key Endpoints | Auth |
|-------|---------------|------|
| **Auth** | `POST /auth/login`, `POST /auth/register` | None |
| **Dashboard** | `GET /dashboard/stats` | JWT |
| **Inventory** | `GET /inventory/pods`, `/clusters`, `/sbom`, `/pod-capabilities/summary` | JWT |
| **Risk** | `GET /risk/insights`, `/scores/:uid`, `/rules`; `POST /risk/scores/:uid/calculate` | JWT |
| **Runtime** | `GET /runtime/signals`, `/pods/:uid/events`, `/pods/:uid/processes`, `/pods/:uid/network` | JWT |
| **Graph** | `GET /graph/attack-paths/bundle`, `/attack-paths/:uid`, `/blast-radius/:uid`, `/risky-pods` | JWT |
| **Audit** | `GET /audit/logs`, `/audit/reports` | JWT |
| **Policy** | `GET /policy/rules`; CRUD for detection templates and instances | JWT |
| **Malware** | `GET /malware/threats`, `/malware/stats`; `POST /malware/db/upload` | JWT |
| **Agents** | `GET /agents/status` | JWT |
| **Metrics** | `GET /metrics/system` | JWT |

### Agent Ingest (no auth — internal cluster traffic)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/agent/sync` | POST | Full inventory sync |
| `/api/v1/agent/pod-runtime-metrics` | POST | Pod resource metrics |
| `/api/v1/agent/pod-processes` | POST | Process snapshots |
| `/api/v1/agent/pod-network-connections` | POST | Network connections |
| `/api/v1/agent/pod-events` | POST | K8s events |
| `/api/v1/runtime/events` | POST | Runtime events (Falco, eBPF) |
| `/api/v2/runtime/events` | POST | Runtime events (canonical v2 DTO) |

### gRPC (mTLS, port 9090)

- `Register` — Agent registration
- `Ping` / `Heartbeat` — Liveness
- `SubmitSBOM` — SBOM ingestion

---

## Configuration

### Core Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `NATS_URL` | `nats://nats:4222` | NATS server URL |
| `AUTO_SYNC_CVE_SOURCE` | `true` | Auto-sync Aikido malware feeds on startup |

### Agent Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CORE_GRPC_ENDPOINT` | `fortuna-core.fortuna.svc.cluster.local:9090` | Core gRPC endpoint |
| `CORE_HTTP_ENDPOINT` | `http://fortuna-core.fortuna.svc.cluster.local:8080` | Core HTTP endpoint |
| `TLS_ENABLED` | `true` | Enable mTLS for gRPC |
| `FALCO_EVENTS_ENABLED` | `true` | Enable Falco JSONL reader |
| `EBPF_ENABLED` | `false` | Enable eBPF syscall sensor |
| `SYNC_INTERVAL` | `5m` | Full sync interval |
| `HEARTBEAT_INTERVAL` | `15s` | Heartbeat ping interval |
| `SBOM_WORKERS` | `1` | Concurrent SBOM extraction workers |
| `POD_DETAIL_RUNTIME_SOURCE` | `host` | `host` = /proc-based collection (no exec) |

---

## Operations

### Monitoring

```bash
# Real-time agent + core error monitoring
./scripts/monitor/monitor-agent-core-errors.sh --follow

# Check deployment status
./scripts/verify/check-full-deployment.sh

# Verify image versions match deployed pods
./scripts/verify/verify-core-agent-rebuild-deploy-status.sh
```

### Database Management

```bash
# Full DB reset (drop all tables, re-run migrations)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-db-reset

# Clear data only (keep schema)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db
```

### Component-Only Rebuilds

```bash
# Rebuild + redeploy Core only (fast iteration)
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-core

# Agent only
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-agent

# Dashboard only
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-dashboard
```

### Multi-Node Image Distribution

```bash
# Preferred: publish/push to a registry all nodes can pull from.
# Fallback for air-gapped local builds: push Core/Agent runtime images to nodes.
# Full pipeline option 15 passes the rebuilt VERSION tag explicitly, so node images
# match the deployment manifests. Standalone use detects tags from deploy/*.yaml.
# Add --include-dashboard only when dashboard image distribution is also required.
./scripts/utils/push-images-to-workers.sh --build-if-missing
```

### Falco Management

```bash
# Install/upgrade Falco (auto-installs Helm if missing)
./scripts/deploy/install-falco-fortuna.sh

# Helm values: deploy/falco/helm-values-fortuna.yaml
# Default driver: kmod (compatible with kernel 5.10–5.15)
# Override for newer kernels: --set driver.kind=modern_ebpf
```

---

## Database Growth & Retention

| Table | Typical Size | Growth Rate | Retention Strategy |
|-------|-------------|-------------|-------------------|
| `cves` | ~770K rows | OSV / loader pipelines | Stable after initial load |
| `package_vulnerabilities` | ~2.7M rows | Proportional to CVEs × packages | Rebuilt on SBOM re-match |
| `pod_network_connections` | ~618K rows | ~100K/day at 120 pods | Periodic pruning recommended |
| `audit_logs` | ~447K rows | ~50K/day | Retention policy by age |
| `pod_processes` | ~120K rows | ~15K/day at 120 pods | Prune older snapshots |
| `runtime_events` | ~10K rows | Depends on Falco + process churn | Low volume without threats |
| `attack_paths` | Variable | Recomputed on inventory changes | Auto-managed |

---

## Security Considerations

- **Git history & leaked credentials:** If secrets were committed, rotate them immediately, invalidate affected credentials, and document the remediation outside the repository.
- **mTLS**: Agent-Core gRPC communication uses mutual TLS. Generate certs with `scripts/utils/create_mtls_secret.sh`.
- **RBAC**: Agent runs with a dedicated ServiceAccount with read-only cluster access for inventory sync.
- **Agent privileges**: Agent DaemonSet requires `hostPID`, `SYS_BPF`, `SYS_ADMIN` capabilities for process collection and eBPF.
- **Falco**: Runs with kernel module access (`kmod` driver). Ensure kernel headers are available.
- **Database**: PostgreSQL uses internal ClusterIP — not exposed outside the cluster.
- **Authentication**: Core API uses JWT. The local bootstrap default is `admin` / `Fortuna_ChangeMe_123!` and is blocked by a first-login password-change requirement. Production deployments should set `FORTUNA_ADMIN_PASSWORD` from a secret manager.
- **Network**: All internal communication stays within the Kubernetes cluster network.

---

## Documentation

| Directory | Content |
|-----------|---------|
| [`docs/README.md`](docs/README.md) | **Documentation hub** — curated map of all guides |
| `docs/01-getting-started/` | **[Getting started](docs/01-getting-started/README.md)** — install, quickstart, environment requirements |
| `docs/04-user-guide/` | **[User guide](docs/04-user-guide/README.md)** — dashboard workspaces and use cases |
| `docs/02-architecture/` | System architecture and API conventions |
| `docs/03-components/` | Component catalog and product-domain reference |
| `docs/05-operations/` | Deployment, production rollout, local containerd builds, and troubleshooting |
| `docs/06-reference/` | Security and credential handling |

**Quick links by topic**

| Topic | Documentation |
|-------|----------------|
| Documentation hub | [docs/README.md](docs/README.md) |
| Getting started (quickstart, environment) | [docs/01-getting-started/README.md](docs/01-getting-started/README.md) |
| REST conventions / route overview | [docs/02-architecture/API_STANDARD.md](docs/02-architecture/API_STANDARD.md) |
| Domain reference (SBOM, CVE, risk, runtime, network, attack paths) | [docs/03-components/COMPONENTS.md](docs/03-components/COMPONENTS.md) |
| Full component index | [docs/03-components/README.md](docs/03-components/README.md) |
| Attack paths | [docs/03-components/COMPONENTS.md#attack-paths](docs/03-components/COMPONENTS.md#attack-paths) |
| Risk model | [docs/03-components/README.md#risk-model](docs/03-components/README.md#risk-model) |
| Security guidance | [docs/06-reference/SECURITY.md](docs/06-reference/SECURITY.md) |

---

## Roadmap

- [x] Multi-cluster telemetry with centralized dashboard: one management stack receives Agent/Falco data from remote clusters and scopes UI/API data by `cluster_id`.
- [ ] Advanced federation controls: per-cluster enrollment UI, remote agent lifecycle management, and cross-cluster policy rollout.
- [ ] Admission webhook for real-time risk gating (`deploy/webhook-config.yaml`)
- [ ] SIEM integration (Splunk, Elasticsearch export)
- [ ] Database retention policies and automatic pruning
- [ ] Reduce Agent memory footprint (streaming SBOM extraction)
- [ ] Expand MITRE ATT&CK coverage and technique library
- [ ] Prometheus metrics endpoint and Grafana dashboards (`deploy/prometheus/`)
- [ ] OIDC/SAML authentication integration
- [ ] Air-gapped deployment support with offline CVE database

---

## Contributing

Contributions are welcome.

- Open an issue for bugs or feature proposals.
- Keep changes scoped and covered by tests.
- Update related docs/specs when contracts change.
- Run `go test ./...` (and any checks your change touches) before submitting PRs.

---

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.
