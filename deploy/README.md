# Deployment manifests

Kubernetes manifests for Fortuna. To install, follow [Install on a cluster](../docs/getting-started/QUICKSTART.md); to try it locally, use the [demo](../docs/getting-started/DEMO.md).

## Helm chart

[`helm/fortuna`](helm/fortuna) installs everything below, generates the secrets and mTLS certificates, and works in any namespace:

```bash
helm install fortuna deploy/helm/fortuna --namespace fortuna --create-namespace
```

Settings are documented in [`values.yaml`](helm/fortuna/values.yaml) and the [Quickstart](../docs/getting-started/QUICKSTART.md#option-a-helm).

## Manifests

The plain manifests are rendered from the chart (`scripts/build/render-manifests.sh`, values in [`helm/fortuna/ci/raw-manifests-values.yaml`](helm/fortuna/ci/raw-manifests-values.yaml)) and CI fails when they differ. Change the chart, then re-render; do not edit these files directly. They install into the `fortuna` namespace and expect the secrets from `create_mtls_secret.sh` and `ensure-fortuna-secrets.sh`.

| File | Purpose |
|------|---------|
| `fortuna-core-deployment.yaml` | Core: REST API, gRPC ingest, auth, risk processing, admission webhook backend |
| `fortuna-agent-daemonset.yaml` | Agent on every node: Kubernetes inventory, SBOM and runtime data sent to Core |
| `dashboard-deployment.yaml` | Web UI and its nginx proxy to Core (ClusterIP Service) |
| `fortuna-rbac.yaml` | ServiceAccounts, ClusterRoles and bindings for Core and Agent |
| `fortuna-core-external-service.yaml` | NodePort for Agents in remote clusters |
| `infrastructure/postgresql.yaml` | Bundled single-instance PostgreSQL (`postgres:15-alpine`, no Apache AGE: graph queries use the relational fallback) |
| `infrastructure/nats.yaml` | NATS JetStream event bus |
| `infrastructure/network-policies.yaml` | Limits NATS and PostgreSQL ingress to Core, denies ingress to Agents |
| `webhook-service.yaml`, `webhook-config.yaml` | Optional admission webhook; enable with `./scripts/deploy/enable-webhook.sh` ([guide](../docs/operations/WEBHOOK.md)) |
| `risk-evaluation-cronjob.yaml` | Optional CronJob that triggers historical risk evaluation every 6 hours |
| `vulndb-update-cronjob.yaml` | CronJob and PVC that download OSV advisories every 6 hours and load what changed ([configuration](../docs/reference/CONFIGURATION.md#scheduled-vulnerability-database-update)) |

Other files, maintained by hand:

| Path | Purpose |
|------|---------|
| `falco/helm-values-fortuna.yaml` | Falco values used by `./scripts/deploy/install-falco-fortuna.sh` |
| `prometheus/risk-center.alerts.yaml` | Prometheus alert rules for active findings (scrape Core's `metrics` port) |
| `certs/` | cert-manager alternative to `create_mtls_secret.sh` |
| `scoped-agent-credentials/` | Overlays for per-Agent tokens and mTLS ([guide](scoped-agent-credentials/README.md)) |
| `samples/` | Private registry pull secrets and image tag overlays ([guide](samples/README.md)) |
| `sql/` | Maintenance SQL used by the procedures below and by helper scripts |
| `infrastructure/postgres-with-age/` | Dockerfiles for a PostgreSQL image with Apache AGE; not built by CI |

Workload images are `ghcr.io/shino-337/fortuna-community/fortuna-*:latest`. Pin a release with the Helm value `image.tag` or with `kubectl set image`, as the Quickstart shows.

## Configuration

Every environment variable that Core and the Agent read, with its default, is listed in the [configuration reference](../docs/reference/CONFIGURATION.md). CI (`scripts/verify/check-docs-sync.py`) fails when a manifest sets a variable that is missing there or that the code never reads.

## Backup, reset and data maintenance

See [backup and reset](../docs/operations/BACKUP_AND_RESET.md). Both resets there delete data; take a backup first.

## Security notes

- Never commit secrets, kubeconfigs, certificates, database dumps or local environment files.
- Keep mTLS enabled between Core and Agents, and keep the CA private key offline.
- Restrict `FORTUNA_WS_ALLOWED_ORIGINS` and `FORTUNA_TRUSTED_PROXIES` to your real dashboard origin and proxy.
- See [production deployment](../docs/operations/PRODUCTION_DEPLOYMENT.md) for the full hardening list.
