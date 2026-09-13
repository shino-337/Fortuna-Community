# Fortuna Deployment

Kubernetes manifests for running Fortuna Core, Agent, Dashboard, and the bundled infrastructure services.

## Components

| Component | File | Notes |
|-----------|------|-------|
| Core | `fortuna-core-deployment.yaml` | API, gRPC ingest, auth, risk processing, admission webhook backend |
| Agent | `fortuna-agent-daemonset.yaml` | Runs on cluster nodes, reads Kubernetes/runtime data, sends data to Core over mTLS |
| Dashboard | `dashboard-deployment.yaml` + `dashboard-nginx-configmap.yaml` | Web UI and nginx proxy to Core |
| RBAC | `fortuna-rbac.yaml` | ServiceAccounts, ClusterRoles, ClusterRoleBindings |
| PostgreSQL | `infrastructure/postgresql-with-age.yaml` | Recommended bundled database manifest |
| NATS | `infrastructure/nats.yaml` | JetStream event bus |
| Redis | `infrastructure/redis.yaml` | Optional cache |
| Certificates | `certs/` | cert-manager Certificate/Issuer manifests |
| Webhook | `webhook-service.yaml` + `webhook-config.yaml` | Optional admission webhook wiring |
| Operations SQL | `sql/` | Maintenance helpers only; not required for first install |
| Samples | `samples/` | Optional YAML examples for GHCR pull secrets and package tag overlays |

## Required Setup

- Kubernetes 1.28+
- A working CNI
- A `local-path` StorageClass or equivalent `ReadWriteOnce` storage
- `kubectl` configured for the target cluster
- Secrets created before Core and Agent start

Create secrets from environment variables instead of committing real values:

```bash
export FORTUNA_POSTGRES_PASSWORD="<replace-me>"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
export FORTUNA_ADMIN_PASSWORD="<replace-me>"

./scripts/utils/ensure-fortuna-secrets.sh fortuna
NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
```

`FORTUNA_JWT_SECRET`, `FORTUNA_INGEST_TOKEN`, and `POD_DETAIL_ENCRYPTION_KEY` can also be supplied. The helper script generates missing JWT and ingest values.

## Images

The public release manifests use GHCR images:

- `ghcr.io/shino-337/fortuna-community/fortuna-core:v1.0.0`
- `ghcr.io/shino-337/fortuna-community/fortuna-agent:v1.0.0`
- `ghcr.io/shino-337/fortuna-community/fortuna-dashboard:v1.0.0`

For production, prefer immutable version tags or digests. For local registryless testing, build and load matching `fortuna-*:<tag>` images into the node runtime and update the image fields.

If GHCR packages are private, use `deploy/samples/ghcr-pull-secret.example.yaml` as a template and attach it with `deploy/samples/ghcr-imagepullsecrets.example.yaml`. For Kustomize-based installs, `deploy/samples/github-packages-kustomization.example.yaml` shows the package image tag overlay.

## Deploy Order

```bash
kubectl create namespace fortuna

./scripts/deploy/ensure-storage-class.sh

kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml

./scripts/utils/ensure-fortuna-secrets.sh fortuna
NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh

kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml
kubectl apply -f deploy/dashboard-deployment.yaml
```

Optional:

```bash
kubectl apply -f deploy/webhook-service.yaml
kubectl apply -f deploy/webhook-config.yaml
kubectl apply -f deploy/infrastructure/redis.yaml
```

The robust deploy script performs the ordered setup checks automatically:

```bash
./scripts/deploy/deploy-fortuna-robust.sh
```

## Access

```bash
kubectl port-forward --address 0.0.0.0 -n fortuna svc/fortuna-dashboard 8081:80
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
```

Dashboard: `http://localhost:8081`

Core health: `http://localhost:8080/healthz`

## Verification

```bash
./scripts/deploy/check-prerequisites-core-agent.sh
./scripts/verify/check-full-deployment.sh
./scripts/verify/verify-agent-core-connectivity.sh
```

## Configuration

Core reads the following important settings:

- `DATABASE_URL`
- `NATS_ENDPOINT`
- `TLS_ENABLED`
- `AUTH_ENABLED`
- `JWT_SECRET`
- `FORTUNA_INGEST_TOKEN`
- `FORTUNA_ADMIN_USERNAME`
- `FORTUNA_ADMIN_PASSWORD`
- `FORTUNA_WS_ALLOWED_ORIGINS`

Agent reads the following important settings:

- `CORE_GRPC_ENDPOINT`
- `CORE_HTTP_ENDPOINT`
- `FORTUNA_INGEST_TOKEN`
- `CLUSTER_ID` and `CLUSTER_NAME` when explicit cluster identity is required
- `SYNC_INTERVAL`
- `TLS_ENABLED`
- `CONTAINERD_SOCKET`
- `SBOM_CACHE_DIR`

Cluster identity is auto-discovered by default. Only set `CLUSTER_ID` or `CLUSTER_NAME` when the operator needs a fixed identity.

## Maintenance

Reset cluster data while keeping schema:

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl cp deploy/sql/clear_all_cluster_data.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec $PG_POD -- psql -U postgres -d fortuna -f /tmp/clear_all_cluster_data.sql
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

Full database reset helper:

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl cp deploy/sql/reset_database_full.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec $PG_POD -- psql -U postgres -d fortuna -f /tmp/reset_database_full.sql
kubectl rollout restart deployment/fortuna-core -n fortuna
```

## Security Notes

- Do not commit real secrets, kubeconfigs, certificates, database dumps, or local environment files.
- Rotate credentials if they were ever committed before history cleanup.
- Keep mTLS enabled for Core and Agent traffic.
- Restrict `FORTUNA_WS_ALLOWED_ORIGINS` to the real Dashboard origins used by your environment.


## Optional admission webhook

Use `./scripts/deploy/enable-webhook.sh` to validate the serving certificate and inject its CA. Do not apply the webhook configuration alone. See the [webhook guide](../docs/05-operations/WEBHOOK.md) for existing certificates, opt-in namespaces, verification, and disabling.
