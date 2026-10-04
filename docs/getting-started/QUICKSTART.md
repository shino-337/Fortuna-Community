# Install Fortuna on a cluster

This is the single install guide for running Fortuna from published images. To try Fortuna on your laptop first, use the [one-command local demo](DEMO.md). To build images from source, see [local containerd build and deploy](../operations/DEPLOYMENT_CONTAINERD.md). For hardening a long-lived installation, continue with [production deployment](../operations/PRODUCTION_DEPLOYMENT.md).

Start in an isolated cluster and read the [environment requirements](ENVIRONMENT_REQUIREMENTS.md). The Agent runs on every node with read access to host processes and, through a sidecar, the containerd socket; see [Agent privileges](../06-reference/SECURITY.md#agent-privileges).

## Prerequisites

- Kubernetes 1.28+ and `kubectl` pointed at the target cluster (`kubectl config current-context`).
- A `local-path` StorageClass, or run `./scripts/deploy/ensure-storage-class.sh`.
- A CNI that enforces NetworkPolicy (Calico, Cilium) if you want NATS and PostgreSQL restricted to Core.
- This repository checked out at the version you install. For a release, use its tag so the manifests match the images:

```bash
git clone --branch v1.0.0 --depth 1 https://github.com/shino-337/Fortuna-Community.git
cd Fortuna-Community
```

## 1. Choose the image version

```bash
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"
```

GitHub Actions publishes `fortuna-core`, `fortuna-agent` and `fortuna-dashboard` under that registry:

| Tag | Published when |
|---|---|
| `vX.Y.Z` | a release tag is pushed |
| `latest`, `sha-<12-char-commit>` | every push to `main` |
| a custom value | the publish workflow is run manually |

Use a tag that matches your checkout. Tags written into `deploy/*.yaml` by local build scripts are local containerd tags, not registry tags.

## 2. Create secrets

```bash
export FORTUNA_ADMIN_PASSWORD="<strong-admin-password>"
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"

kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
./scripts/deploy/ensure-storage-class.sh
NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
./scripts/utils/ensure-fortuna-secrets.sh fortuna
```

`ensure-fortuna-secrets.sh` creates `fortuna-secrets` (database URL, JWT secret, ingest token, admin password, bootstrap flag) and `postgres-credentials`. It generates any value you did not export and reuses existing ones on later runs. `create_mtls_secret.sh` writes the CA and certificates to `.certs/`; keep that directory, because remote clusters reuse it.

If you omit `FORTUNA_ADMIN_PASSWORD`, the admin starts with the bootstrap password `Fortuna_ChangeMe_123!` and must change it at first login. Only do that in an isolated lab.

**Private registry only.** If the packages are not public, create a pull secret and attach it before deploying workloads (YAML equivalents are in [`deploy/samples/`](../../deploy/samples/README.md)):

```bash
kubectl -n fortuna create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io --docker-username="$GITHUB_USER" --docker-password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
for sa in fortuna-core fortuna-agent default; do
  kubectl -n fortuna patch serviceaccount "$sa" -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
done
```

Run the loop after step 3, once the `fortuna-core` and `fortuna-agent` ServiceAccounts exist.

## 3. Deploy

```bash
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/infrastructure/network-policies.yaml
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml

kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-deployment.yaml

kubectl -n fortuna set image deployment/fortuna-core core="${FORTUNA_REGISTRY}/fortuna-core:${FORTUNA_VERSION}"
kubectl -n fortuna set image daemonset/fortuna-agent agent="${FORTUNA_REGISTRY}/fortuna-agent:${FORTUNA_VERSION}" image-export="${FORTUNA_REGISTRY}/fortuna-agent:${FORTUNA_VERSION}"
kubectl -n fortuna set image deployment/fortuna-dashboard dashboard="${FORTUNA_REGISTRY}/fortuna-dashboard:${FORTUNA_VERSION}"
```

`./scripts/deploy/deploy-fortuna-robust.sh` performs the same steps with additional prerequisite checks.

Wait for the rollout. Core runs database migrations on first start, which takes a few minutes:

```bash
kubectl -n fortuna rollout status deployment/fortuna-core --timeout=600s
kubectl -n fortuna rollout status daemonset/fortuna-agent --timeout=300s
kubectl -n fortuna rollout status deployment/fortuna-dashboard --timeout=300s
kubectl -n fortuna get pods -o wide
```

Expect one `fortuna-core`, one `fortuna-dashboard`, one `postgres`, three `nats` pods and one `fortuna-agent` pod per node, all `Running`.

## 4. Log in

```bash
kubectl -n fortuna port-forward svc/fortuna-dashboard 8081:80
```

Open `http://localhost:8081` and log in as `admin` with `FORTUNA_ADMIN_PASSWORD`. The Agent syncs inventory every 5 minutes, so data appears gradually after the first start. Then follow [Your first RBAC investigation](FIRST_FINDING.md).

To check authentication from inside the cluster:

```bash
kubectl -n fortuna exec deploy/fortuna-core -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"${FORTUNA_ADMIN_PASSWORD}\"}"
```

With the bootstrap password, the response contains `"mustChangePassword": true`, and data APIs return `403 password_change_required` until the password is changed. Core never resets an existing admin back to the bootstrap password.

## 5. Load security data (optional)

Vulnerability matching needs the CVE catalog, and runtime findings need a sensor:

```bash
./scripts/utils/load-cve-data.sh            # large download (about 1.2 GB)
./scripts/deploy/install-falco-fortuna.sh   # Falco with Fortuna settings
kubectl -n fortuna rollout restart daemonset/fortuna-agent
```

Read [runtime sensors](../operations/RUNTIME_SENSORS.md) before relying on runtime evidence, in particular the rules for Falco delivery state.

## 6. Add a remote cluster (optional)

One Fortuna installation can observe several clusters. The management cluster runs Core, Dashboard, PostgreSQL, NATS and its own Agent; each remote cluster runs only the Agent (and optionally Falco).

Expose Core from the management cluster, then deploy the Agent to each remote cluster with the same `.certs` material. Do not set `MTLS_REGEN=1`, or Core will stop trusting remote Agents.

```bash
kubectl -n fortuna apply -f deploy/fortuna-core-external-service.yaml

export MANAGEMENT_NODE=<management-node-ip-or-dns>
export REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig"
./scripts/deploy/sync-remote-agent.sh
```

The script creates the remote namespace, secrets, RBAC and Agent DaemonSet, and points the Agent at Core. Each Agent discovers a stable cluster ID from its API server; set `CLUSTER_ID` only when you need a fixed one. Use `REMOTE_IMAGE_MODE=local` when the remote cluster cannot pull from a registry.

Check that every cluster reports consistent data:

```bash
FORTUNA_JWT="<admin-jwt>" CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig" \
./scripts/verify/verify-multicluster-sync.sh
```

## Troubleshooting

| Symptom | Fix |
|---|---|
| `401 Unauthorized` pulling images | The packages are private: create `ghcr-pull` as in step 2, or make them public |
| Pods stay `Pending` | No usable StorageClass: run `./scripts/deploy/ensure-storage-class.sh` |
| Core restarts during first start | Migrations are still running; check `kubectl -n fortuna logs deploy/fortuna-core` and wait |
| Pods run old code | Compare the workload image (`kubectl -n fortuna get deploy fortuna-core -o jsonpath='{..image}'`) with the pod's `imageID` |
| Bootstrap password rejected | The database already has an admin with another password; use that password |
| Full health check | `./scripts/verify/check-full-deployment.sh` |

## Uninstall or reset

```bash
kubectl delete namespace fortuna          # removes Fortuna and its data
kubectl delete -f deploy/fortuna-rbac.yaml --ignore-not-found   # cluster-wide roles and bindings
```

To reset only the data while keeping the installation, follow the backup-first procedure in [deploy/README.md](../operations/BACKUP_AND_RESET.md).
