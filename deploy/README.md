# Deployment manifests

Kubernetes manifests for Fortuna. To install, follow [Install on a cluster](../docs/01-getting-started/QUICKSTART.md); to try it locally, use the [demo](../docs/01-getting-started/DEMO.md).

## Manifests

| File | Purpose |
|------|---------|
| `fortuna-core-deployment.yaml` | Core: REST API, gRPC ingest, auth, risk processing, admission webhook backend |
| `fortuna-agent-daemonset.yaml` | Agent on every node: Kubernetes inventory, SBOM and runtime data sent to Core |
| `dashboard-deployment.yaml`, `dashboard-nginx-configmap.yaml` | Web UI and the nginx proxy to Core |
| `fortuna-rbac.yaml` | ServiceAccounts, ClusterRoles and bindings for Core and Agent |
| `fortuna-core-external-service.yaml` | NodePort for Agents in remote clusters |
| `infrastructure/postgresql-with-age.yaml` | Bundled PostgreSQL (default); `postgresql.yaml` is a simpler fallback |
| `infrastructure/nats.yaml` | NATS JetStream event bus |
| `infrastructure/network-policies.yaml` | Limits NATS and PostgreSQL ingress to Core |
| `webhook-service.yaml`, `webhook-config.yaml` | Optional admission webhook; enable with `./scripts/deploy/enable-webhook.sh` ([guide](../docs/05-operations/WEBHOOK.md)) |
| `risk-evaluation-cronjob.yaml` | Optional CronJob that triggers historical risk evaluation every 6 hours |
| `falco/helm-values-fortuna.yaml` | Falco values used by `./scripts/deploy/install-falco-fortuna.sh` |
| `prometheus/risk-center.alerts.yaml` | Prometheus alert rules for active findings; usable only once Core exposes a `/metrics` endpoint (not yet available) |
| `certs/` | cert-manager alternative to `create_mtls_secret.sh` |
| `scoped-agent-credentials/` | Overlays for per-Agent tokens and mTLS ([guide](scoped-agent-credentials/README.md)) |
| `samples/` | Private registry pull secrets and image tag overlays ([guide](samples/README.md)) |
| `sql/` | Maintenance SQL used by the procedures below and by helper scripts |

The image fields in the workload manifests are local build tags. Installs from the registry override them with `kubectl set image`, as the Quickstart shows.

## Configuration

Core settings (environment variables in `fortuna-core-deployment.yaml`):

| Variable | Purpose |
|---|---|
| `DATABASE_URL`, `NATS_ENDPOINT` | Data stores |
| `JWT_SECRET`, `FORTUNA_ADMIN_USERNAME`, `FORTUNA_ADMIN_PASSWORD` | Authentication and bootstrap admin |
| `FORTUNA_INGEST_TOKEN` | Shared Agent ingest token (legacy; see scoped credentials) |
| `FORTUNA_AGENT_CREDENTIAL_REGISTRY`, `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` | Per-Agent HTTP and gRPC credentials |
| `TLS_ENABLED`, `TLS_*_PATH`, `WEBHOOK_TLS_*_PATH` | mTLS for gRPC and the webhook server |
| `FORTUNA_TRUSTED_PROXIES` | Proxies allowed to set `X-Forwarded-For`; unset trusts none |
| `FORTUNA_WS_ALLOWED_ORIGINS` | Dashboard origins allowed to open WebSockets |
| `FORTUNA_MAX_REQUEST_BODY_BYTES` | Request body limit (default 64 MiB) |
| `AUTH_ENABLED` | Must stay `true`; `false` is accepted only with `FORTUNA_DEV_MODE=1` |

Agent settings (in `fortuna-agent-daemonset.yaml`): `CORE_GRPC_ENDPOINT`, `CORE_HTTP_ENDPOINT`, `FORTUNA_INGEST_TOKEN`, `SYNC_INTERVAL`, `HEARTBEAT_INTERVAL`, `TLS_ENABLED`, `CONTAINERD_SOCKET`, `FALCO_EVENTS_ENABLED`, `EBPF_ENABLED`. Cluster identity is discovered automatically; set `CLUSTER_ID` or `CLUSTER_NAME` only when you need fixed values.

## Maintenance

Reset cluster data while keeping schema:

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl cp deploy/sql/clear_all_cluster_data.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec $PG_POD -- psql -U postgres -d fortuna -f /tmp/clear_all_cluster_data.sql
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

Full database reset helper (destructive: removes all application data, users,
catalogs, and migration history in the dedicated `fortuna` database):

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
set -e
umask 077
BACKUP_DIR=$(mktemp -d /tmp/fortuna-backup.XXXXXX)
kubectl -n fortuna exec "$PG_POD" -- pg_dump -U postgres -d fortuna -Fc -Z 6 > "$BACKUP_DIR/fortuna.dump"
kubectl -n fortuna cp "$BACKUP_DIR/fortuna.dump" "$PG_POD":/tmp/fortuna-check.dump
kubectl -n fortuna exec "$PG_POD" -- pg_restore -l /tmp/fortuna-check.dump >/dev/null
LOCAL_HASH=$(sha256sum "$BACKUP_DIR/fortuna.dump" | cut -d ' ' -f 1)
POD_HASH=$(kubectl -n fortuna exec "$PG_POD" -- sha256sum /tmp/fortuna-check.dump | cut -d ' ' -f 1)
test "$LOCAL_HASH" = "$POD_HASH"
kubectl -n fortuna exec "$PG_POD" -- rm /tmp/fortuna-check.dump
kubectl -n fortuna scale deployment/fortuna-core --replicas=0
kubectl -n fortuna wait --for=delete pod -l app.kubernetes.io/component=core --timeout=120s
kubectl cp deploy/sql/reset_database_full.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec "$PG_POD" -- psql -v ON_ERROR_STOP=1 -U postgres -d fortuna -f /tmp/reset_database_full.sql
kubectl -n fortuna scale deployment/fortuna-core --replicas=1
```

Keep the dump securely until the new rollout is verified. The reset keeps the
PostgreSQL PVC and Kubernetes Secrets; Core recreates the schema and bootstrap
admin on startup. A fresh vulnerability catalog must be loaded separately if
Core has no configured OSV source directory. New Core builds quarantine Agent
gRPC writes until the [per-Agent mTLS registry and client certificate overlay](scoped-agent-credentials/MTLS.md)
is provisioned; the old shared Agent certificate is not a fallback. If the
original Fortuna CA private key is unavailable, use the dedicated Agent-client
CA procedure in that guide rather than rotating Core and webhook certificates.

On a subsequent `deploy-fortuna-robust.sh` run, existing scoped Agent registry
secrets are detected before the old Core Deployment is deleted. The script
requires the HTTP registry, mTLS registry and Agent-client CA secret together,
then reapplies the HTTP/mTLS Core and Agent patches before rollout. Keep the
per-node token and client-certificate files provisioned at the host paths in
those patches. Set `APPLY_SCOPED_AGENT_CREDENTIALS=false` only if another
deployment controller manages these overlays; a partial secret set otherwise
stops deployment before replacing Core. Scoped mTLS also requires cluster DNS;
the script refuses the legacy TLS-disabled IP fallback.

## Security notes

- Never commit secrets, kubeconfigs, certificates, database dumps or local environment files.
- Keep mTLS enabled between Core and Agents, and keep the CA private key offline.
- Restrict `FORTUNA_WS_ALLOWED_ORIGINS` and `FORTUNA_TRUSTED_PROXIES` to your real dashboard origin and proxy.
- See [production deployment](../docs/05-operations/PRODUCTION_DEPLOYMENT.md) for the full hardening list.
