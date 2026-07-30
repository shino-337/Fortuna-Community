# Fortuna Scripts Guide

Scripts are grouped by intent. Run scripts from the repository root and use full paths such as `./scripts/pipeline/full-clean-database-rebuild-deploy.sh`.

## Public Entrypoints

| Goal | Command |
|------|---------|
| Full local rebuild and deploy | `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full` |
| Full rebuild, deploy, and E2E | `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-e2e` |
| Full DB reset, rebuild, deploy | `./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset --full` |
| Deploy existing images/manifests | `./scripts/deploy/deploy-fortuna-robust.sh` |
| Sync remote Agent-only clusters | `REMOTE_KUBECONFIGS="cluster02=/path/to/kubeconfig" MANAGEMENT_NODE=<ip> ./scripts/deploy/sync-remote-agent.sh` |
| Build local containerd images | `./scripts/build/build-and-load-containerd.sh` |
| Publish release tag | `./scripts/utils/create-github-release.sh vX.Y.Z` |
| Clean local host image/cache pressure | `./scripts/clean/check-and-clean-host-resources.sh --clean -y` |
| Verify deployment health | `./scripts/verify/check-full-deployment.sh` |
| Verify multi-cluster DB/API sync | `./scripts/verify/verify-multicluster-sync.sh` |
| Open dashboard locally | `./scripts/utils/port-forward-dashboard.sh` |

For normal user installs, prefer published images through the docs in `docs/01-getting-started/`. Local build scripts are for development, lab validation, and private forks.

## Directory Contract

| Directory | Purpose | Keep Here |
|-----------|---------|-----------|
| `pipeline/` | End-to-end local workflows | Scripts that orchestrate clean/build/deploy/verify steps |
| `build/` | Image build and load | Containerd/Docker/buildctl build scripts |
| `deploy/` | Kubernetes apply and cluster prerequisites | CNI/storage/secret/deploy helpers |
| `clean/` | Local or cluster cleanup | Host cache, image cleanup, evicted pods, test cleanup |
| `verify/` | Read-only health checks | API, DB, rollout, runtime, SBOM, and dashboard checks |
| `e2e/` | Scenario and integration tests | Workload-creating test suites |
| `monitor/` | Live observability helpers | Log and status watchers |
| `utils/` | Shared operational utilities | CVE load, mTLS, port-forward, image push, release helpers |

Do not add one-off local maintenance scripts to `scripts/`. Keep private experiments outside the repo unless they are promoted into one of the categories above.

## Pipeline

`full-clean-database-rebuild-deploy.sh` is the main local pipeline.

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-rebuild
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-deploy
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-e2e
```

Important environment variables:

| Variable | Meaning |
|----------|---------|
| `VERSION` | Image tag used by local build/deploy scripts |
| `FORTUNA_PACKAGE_SOURCE` | `github` by default; set `local` for registryless source builds |
| `FORTUNA_REGISTRY` | GHCR registry namespace, default `ghcr.io/shino-337/fortuna-community` |
| `FORTUNA_VERSION` | Published package tag, default `v1.0.0` |
| `SYNC_DEPLOY_IMAGE_TAG` | Defaults to `true`; syncs built tag into deploy YAML |
| `BUILD_TOOL` | `nerdctl`, `docker`, or `buildctl` |
| `PUSH_IMAGES_AFTER_REBUILD` | Defaults to `true` only when current cluster has more than one node; controls registryless node image copy/import |
| `WITH_RUNTIME` | Enable runtime sensor install path where supported |
| `REMOTE_KUBECONFIGS` | Optional comma-separated remote Agent clusters: `name=/path/to/kubeconfig` |
| `REMOTE_IMAGE_MODE` | `auto`, `registry`, or `local` for remote Agent image distribution |
| `REMOTE_SYNC_REQUIRED` | Defaults to `true`; set `false` to continue pipeline when remote sync fails |
| `REMOTE_IMAGE_PULL_POLICY` | Defaults to `Always` for `:latest`, otherwise `IfNotPresent` |
| `MANAGEMENT_NODE` | Management Core node/IP used for remote Agent NodePort endpoints |

For long local runs, use the same entrypoint with async mode:

```bash
RUN_ASYNC=1 PIPELINE_LOG_FILE=/tmp/fortuna-pipeline.log ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
./scripts/pipeline/watch-pipeline-log.sh /tmp/fortuna-pipeline.log
```

## Build

```bash
./scripts/build/build-and-load-containerd.sh
BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh
BUILD_CORE_ONLY=true ./scripts/build/build-and-load-containerd.sh
BUILD_AGENT_ONLY=true ./scripts/build/build-and-load-containerd.sh
BUILD_DASHBOARD_ONLY=true ./scripts/build/build-and-load-containerd.sh
NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
```

Use the same entrypoint for registry push. `PUSH_IMAGES=true` requires `REGISTRY`; use Docker or nerdctl for push mode:

```bash
BUILD_TOOL=docker REGISTRY=registry.example.com/fortuna PUSH_IMAGES=true ./scripts/build/build-and-load-containerd.sh
```

## Deploy

Primary deploy path:

```bash
./scripts/deploy/deploy-fortuna-robust.sh
```

Useful prerequisite and repair helpers:

| Script | Purpose |
|--------|---------|
| `sync-remote-agent.sh` | Deploy/patch Agent-only remote clusters from `REMOTE_KUBECONFIGS` |
| `ensure-flannel.sh` | Install or verify Flannel when the cluster uses Flannel |
| `ensure-storage-class.sh` | Ensure a usable default StorageClass |
| `ensure-control-plane-label.sh` | Ensure Core can schedule on the intended control-plane node |
| `install-falco-fortuna.sh` | Install/upgrade Falco with Fortuna values |
| `fix-flannel-vxlan.sh` | Repair Flannel VXLAN issues |

Use the main pipeline for component-specific rebuilds instead of deploy helpers:

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-core --with-clean
```

Remote Agent sync:

```bash
# Registry mode: remote clusters pull the published Agent image.
REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig" \
MANAGEMENT_NODE=<management-node-ip-or-dns> \
FORTUNA_REGISTRY=ghcr.io/shino-337/fortuna-community \
FORTUNA_VERSION=v1.0.0 \
./scripts/deploy/sync-remote-agent.sh

# Local registryless mode: import the local Agent image into remote nodes through SSH.
REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig" \
MANAGEMENT_NODE=<management-node-ip-or-dns> \
REMOTE_IMAGE_MODE=local \
VERSION="$(git describe --tags --always)" \
./scripts/deploy/sync-remote-agent.sh
```

## Clean

| Script | Use |
|--------|-----|
| `cleanup-environment.sh` | General cleanup of port-forwards, test namespaces, old images, and optional DB test data |
| `check-and-clean-host-resources.sh` | Host disk/image/cache inspection and cleanup |
| `clean-evicted-completed-pods.sh` | Remove Evicted, Completed, and Failed pods; supports `NAMESPACE` and `DRY_RUN=1` |
| `clean-e2e-test-images.sh` | Remove E2E test pods/images |

Examples:

```bash
./scripts/clean/check-and-clean-host-resources.sh
./scripts/clean/check-and-clean-host-resources.sh --clean -y
DRY_RUN=1 ./scripts/clean/clean-evicted-completed-pods.sh
NAMESPACE=fortuna ./scripts/clean/clean-evicted-completed-pods.sh
```

Use the main pipeline for component rebuilds instead of clean scripts:

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-dashboard
```

## Verify

Start with:

```bash
./scripts/verify/check-full-deployment.sh
./scripts/verify/deploy-status.sh
./scripts/verify/verify-agent-core-connectivity.sh
./scripts/verify/verify-dashboard-apis.sh
./scripts/verify/verify-database-schema.sh
```

Focused checks:

| Area | Scripts |
|------|---------|
| Dashboard/API | `verify-dashboard-api.sh`, `verify-dashboard-apis.sh` |
| Agent/runtime | `verify-agent-core-connectivity.sh`, `verify-risk-runtime-unit.sh` |
| SBOM/CVE | `ensure-cve-catalog-ready.sh`, `verify-sbom-pod.sh`, `verify-sbom-flow-cluster.sh` |
| Pod detail | `check-pod-detail-api.sh`, `verify-pod-detail-api-and-db.sh`, `verify-pod-detail-empty-response.sh` |
| Risk/rules | `check-pod-risk.sh`, `verify-risk-rules-api.sh`, `verify-threat-velocity-pipeline.sh` |
| Multi-cluster | `verify-multicluster-sync.sh` |

Generated reports belong under ignored local output paths such as `test-results/`.

Multi-cluster verification compares Kubernetes pod counts, DB active pod counts, `/inventory/clusters/stats`, and `/dashboard/stats`:

```bash
FORTUNA_JWT="<admin-or-operator-jwt>" \
CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster101=/path/to/cluster101.kubeconfig" \
./scripts/verify/verify-multicluster-sync.sh
```

## E2E

E2E suites are development and maintainer validation tools. They create test workloads and may write ignored local outputs such as `test-results/`.

Main entrypoint:

```bash
./scripts/e2e/run-e2e.sh --list
./scripts/e2e/run-e2e.sh --suite=full
./scripts/e2e/run-e2e.sh --suite=risk-center
./scripts/e2e/run-e2e.sh --suite=runtime
./scripts/e2e/run-e2e.sh --suite=sbom
```

Common suites:

| Suite | Purpose |
|-------|---------|
| `full` | Deployment sanity, full API report, Risk Center, and priority API checks |
| `full-report` | Long-form capability and scenario report |
| `dashboard` | Populate and verify dashboard data APIs |
| `consistency` | Compare Kubernetes, database, dashboard APIs, and pod-delete cleanup behavior |
| `pod-detail` | Unit/DB/API pod-detail verification report |
| `pod-detail-live` | Live process/network workload checks for pod detail |
| `runtime` | Runtime signal API flow |
| `runtime-gap` | Runtime gap matrix and toxic-combo checks |
| `falco-runtime` | Falco ingestion and runtime event visibility |
| `pce` | Pod Capability Engine and promotion flow |
| `sbom` | Default SBOM API verification |
| `sbom-full` | Busybox, distroless, and CoreDNS SBOM extraction flow |
| `sbom-quality` | SBOM confidence, component quality, and audit trail checks |

Keep direct calls to individual `scripts/e2e/*.sh` for local debugging only. Public validation should go through `run-e2e.sh` so suite names stay stable.

Set `E2E_NODE_SELECTOR_HOST=<node-name>` only when a scenario must run on a specific node.

Live-cluster attack-path validation remains a root-level exception:

```bash
./scripts/verify-k8s-e2e.sh
```

## Monitor

```bash
./scripts/monitor/monitor-agent-core.sh
./scripts/monitor/monitor-agent-core-errors.sh
./scripts/monitor/monitor-agent-core-errors.sh --follow
./scripts/monitor/monitor-runtime-signals.sh
./scripts/monitor/monitor-testcases.sh
```

## Utilities

| Script | Purpose |
|--------|---------|
| `load-cve-data.sh` | Load CVE/package vulnerability data into PostgreSQL |
| `ensure-cve-tables.sh` | Ensure CVE-related tables are available |
| `ensure-fortuna-secrets.sh` | Create/update required app secrets |
| `create_mtls_secret.sh` | Create Core/Agent mTLS secrets |
| `rotate_mtls_secret.sh` | Rotate mTLS material |
| `manage-port-forwards.sh` | Start/stop/list local Core/Dashboard port-forwards |
| `port-forward-dashboard.sh` | Dashboard-only port-forward helper |
| `push-images-to-workers.sh` | Copy local Core/Agent runtime images to nodes for registryless clusters |
| `force-fortuna-image-refresh.sh` | Purge/verify/restart local Fortuna images and pods |
| `rebuild-fortuna-workloads-safe.sh` | Safe rebuild helper for local images |
| `ensure-buildkit-running.sh` | Best-effort BuildKit startup for local builds |
| `sync-package-vulnerability-source.sh` | Sync OSV bulk vulnerability source into local CVE data |
| `create-github-release.sh` | Create and push a `v*` release tag |

For multi-node clusters, prefer a registry. Use `push-images-to-workers.sh` only for local registryless environments; it pushes Core and Agent images by default. Add `--include-dashboard` or `PUSH_DASHBOARD=true` when the Dashboard image also needs to be imported onto worker nodes. The full pipeline passes the rebuilt `VERSION` explicitly to this script when the current cluster has more than one node. Single-node clusters skip this step unless `PUSH_IMAGES_AFTER_REBUILD=true` is set. Standalone use detects image tags from `deploy/*.yaml` unless `CORE_IMAGE`, `AGENT_IMAGE`, or `DASHBOARD_IMAGE` are provided. Configure private worker credentials in `scripts/utils/push-images.config`, which is ignored by Git.

## Adding Or Changing Scripts

- Put the script in the directory that matches its operational intent.
- Add `set -euo pipefail` and a usage header.
- Make destructive behavior opt-in with flags such as `--clean`, `--db-reset`, `--all`, or `DRY_RUN=1`.
- Avoid hardcoded credentials, IPs, kubeconfig paths, or local usernames.
- Update this guide and any docs that reference the script.
- Run syntax validation before committing:

```bash
find scripts -type f -name '*.sh' -print0 | xargs -0 -n1 bash -n
```
