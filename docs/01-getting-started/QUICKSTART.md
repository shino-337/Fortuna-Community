# Quickstart

Use this guide when you want a working Fortuna deployment. Start in an isolated lab and review the [environment requirements](ENVIRONMENT_REQUIREMENTS.md).

For a released version, use the documentation and manifests in the matching checkout (`git clone --branch v1.0.0 --depth 1 https://github.com/shino-337/Fortuna-Community.git`). This file on `main` may describe changes newer than that release.

After installation, follow [Your first RBAC investigation](FIRST_FINDING.md) for one concrete result and a remediation check. A hosted demo and one-command lab are not available yet.

The public repository is `shino-337/Fortuna-Community`, and published GHCR images use `ghcr.io/shino-337/fortuna-community`.

There are two paths:

- **Public install:** deploy from images already published by GitHub Actions. This is the recommended path for users.
- **Developer local build:** build images on your own node and deploy with local pipeline scripts. Use this for development.

## 1. Public Install From Published Images

Prerequisites:

- Kubernetes 1.28+ with `kubectl` configured.
- A working StorageClass for PostgreSQL/NATS PVCs.
- Internet access from nodes to pull images from GHCR, or an imagePullSecret if the package is private.
- The repository checked out locally for manifests and helper scripts.

Set the image registry and version:

```bash
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"
```

Set deployment secrets:

```bash
export FORTUNA_ADMIN_PASSWORD="<strong-admin-password>" # recommended; omit only for first-login bootstrap default
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"

# Bundled PostgreSQL path. Use your external DB URL instead if you do not deploy deploy/infrastructure/postgresql-with-age.yaml.
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
```

Create the namespace first. If GHCR packages are private, create and attach an image pull secret before applying workloads. `GITHUB_TOKEN` must have `read:packages`:

```bash
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
```

Skip the following block for public packages. Run it only when your registry requires authentication and `GITHUB_USER` / `GITHUB_TOKEN` have been set:

```bash
kubectl -n fortuna create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USER" \
  --docker-password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Create namespace, secrets, and infrastructure:

```bash
./scripts/deploy/ensure-storage-class.sh
NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
./scripts/utils/ensure-fortuna-secrets.sh fortuna

kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml
```

Attach the optional GHCR pull secret, deploy Fortuna, and point workloads at the published images:

```bash
if kubectl -n fortuna get secret ghcr-pull >/dev/null 2>&1; then
  kubectl -n fortuna patch serviceaccount fortuna-core \
    -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
  kubectl -n fortuna patch serviceaccount fortuna-agent \
    -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
  kubectl -n fortuna patch serviceaccount default \
    -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
fi

kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-deployment.yaml

kubectl -n fortuna set image deployment/fortuna-core \
  core="${FORTUNA_REGISTRY}/fortuna-core:${FORTUNA_VERSION}"
kubectl -n fortuna set image daemonset/fortuna-agent \
  agent="${FORTUNA_REGISTRY}/fortuna-agent:${FORTUNA_VERSION}"
kubectl -n fortuna set image deployment/fortuna-dashboard \
  dashboard="${FORTUNA_REGISTRY}/fortuna-dashboard:${FORTUNA_VERSION}"
```

Anonymous pulls fail with `401 Unauthorized` when the package is private. Either make the GHCR package public or use the pull secret above.

YAML examples are available for private GHCR package pulls:

- `deploy/samples/ghcr-pull-secret.example.yaml`
- `deploy/samples/ghcr-imagepullsecrets.example.yaml`
- `deploy/samples/github-packages-kustomization.example.yaml`

Wait for workloads:

```bash
kubectl rollout status -n fortuna deployment/fortuna-core --timeout=180s
kubectl rollout status -n fortuna daemonset/fortuna-agent --timeout=180s
kubectl rollout status -n fortuna deployment/fortuna-dashboard --timeout=180s
kubectl get pods,svc -n fortuna -o wide
```

<details>
<summary>Developer/operations only: reset an existing lab database</summary>

This resets database state. It is not a first-install or upgrade command. Use it only when you intend to discard the existing lab data.

For a scripted deploy from published images with a database reset and no local rebuild:

```bash
export FORTUNA_PACKAGE_SOURCE="github"
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"
export FORTUNA_ADMIN_PASSWORD="<strong-admin-password>"
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"

CVE_CATALOG_POST_DEPLOY_CHECK=skip \
  ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset --skip-rebuild
```

Use `PG_RECREATE_PVC=1` with that command only when you intentionally want to delete and recreate the PostgreSQL PVC files.

</details>

## 2. Developer Local Build

Use this only when you want to build images on the cluster node or test local changes.

```bash
export FORTUNA_ADMIN_PASSWORD="<strong-admin-password>" # recommended; omit only for first-login bootstrap default
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
export FORTUNA_PACKAGE_SOURCE="local"

./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime
```

Use `--db-reset` when you want a clean schema and empty data:

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset --with-runtime
```

The local pipeline:

- Builds `fortuna-core`, `fortuna-agent`, and `fortuna-dashboard`.
- Loads images into local containerd.
- Syncs the built `VERSION` into `deploy/*.yaml`.
- Creates `fortuna-secrets`, `postgres-credentials`, and Core/Agent mTLS secrets before rollout.
- Deploys and verifies local workloads.

Notes:

- First local build needs internet access for Go modules, npm packages, base images, Syft, Helm charts, and Falco images.
- Multi-node clusters need Core/Agent images on the nodes where those workloads run. Prefer a registry accessible to every node. If you are air-gapped or intentionally registryless, configure `scripts/utils/push-images.config` or SSH env variables and run `./scripts/utils/push-images-to-workers.sh`. The full pipeline pushes the same rebuilt `VERSION` tag only when `FORTUNA_PACKAGE_SOURCE=local` and the current cluster has more than one node, or when `PUSH_IMAGES_AFTER_REBUILD=true` is set.
- After `--db-reset`, the pipeline verifies the CVE catalog. If the catalog is empty, `AUTO_LOAD_CVE_CATALOG=true` loads the public OSV/package catalog, currently a large download of about 1.2GB plus extract/import time. For a fast deployment smoke test, set `CVE_CATALOG_POST_DEPLOY_CHECK=skip`.

## 3. Add A Remote Cluster

Use this when one Fortuna management cluster should receive telemetry from another Kubernetes cluster.

Management cluster runs:

- Core, Dashboard, PostgreSQL, NATS.
- Local Agent and optional Falco.
- `fortuna-core-external` Service when remote agents connect through NodePort.

Remote cluster runs:

- `fortuna-agent` only.
- Optional Falco if runtime events are required from that cluster.

Expose Core from the management cluster:

```bash
kubectl -n fortuna apply -f deploy/fortuna-core-external-service.yaml
kubectl -n fortuna get svc fortuna-core-external
```

Deploy the Agent prerequisites to the remote cluster. Reuse the existing `.certs` generated for the management cluster; do not regenerate mTLS with `MTLS_REGEN=1`, because Core must trust the remote Agent client certificate.

```bash
export REMOTE_KUBECONFIG=/path/to/remote.kubeconfig
export MANAGEMENT_NODE=<management-node-ip-or-dns>
export REMOTE_KUBECONFIGS="cluster02=${REMOTE_KUBECONFIG}"
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"

./scripts/deploy/sync-remote-agent.sh
```

The script creates/updates the remote namespace, mTLS secrets, ingest secret, RBAC, Agent DaemonSet, image, Core endpoints, and rollout. It does not set `CLUSTER_ID` by default; Agent auto-discovers a stable ID from the cluster API. Use `REMOTE_IMAGE_MODE=local` when the remote cluster cannot pull from a registry and SSH image import is required. Registry image pull policy defaults to `Always` for `:latest`, otherwise `IfNotPresent`.

Verify sync:

```bash
FORTUNA_JWT="<admin-or-operator-jwt>" \
CORE_URL="http://127.0.0.1:8080" \
REMOTE_KUBECONFIGS="cluster101=${REMOTE_KUBECONFIG}" \
./scripts/verify/verify-multicluster-sync.sh
```

## 4. Open the Dashboard

```bash
kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80
```

Open `http://127.0.0.1:8081/`.

Development login:

- Username: `admin`
- Password: value provided in `FORTUNA_ADMIN_PASSWORD`; if omitted on a fresh deployment, use `Fortuna_ChangeMe_123!` and change it when prompted.

The bootstrap default is only for first deployment. If the database already contains an `admin` user, Core will not reset that password back to the default on restart.

Verify auth/bootstrap behavior from Core:

```bash
CORE_POD="$(kubectl -n fortuna get pod -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}')"
kubectl -n fortuna exec "$CORE_POD" -- curl -s \
  -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Fortuna_ChangeMe_123!"}' | python3 -m json.tool
```

When the bootstrap default is active, the response includes `"mustChangePassword": true`, and normal data APIs return `403` with `password_change_required` until the password is changed.

## 5. Verify Data

```bash
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=50
kubectl logs -n fortuna -l app.kubernetes.io/component=agent --tail=50
```

After login, the dashboard should show:

- Platform Integrity: telemetry and pipeline health.
- Dashboard: high-level security posture.
- Kubernetes Inventory: discovered Kubernetes pods and workload context.
- Findings Queue: active findings and unified risk scores.
- Attack Paths: attack paths and runtime-supported graph signals.
- Runtime Network: runtime topology when agent telemetry is available.
- Pipeline & Runtime Health: pipeline, runtime, and sensor visibility.

## 6. Image Publishing

GitHub Actions builds and publishes images through `.github/workflows/publish-images.yml`:

- Push to `main`: publishes `latest` and `sha-<short-sha>`.
- Push tag `v*`: publishes the tag and `sha-<short-sha>`.
- Manual dispatch: publishes the supplied `version`.
- Pull request: builds images without pushing.

Use `latest`, `sha-<12-char-commit>`, a pushed release tag such as `v1.0.0`, or a manual workflow `version` value as `FORTUNA_VERSION`. Tags generated by local scripts, for example `git describe` values written into `deploy/*.yaml`, are local containerd tags unless the publish workflow was run with the same value.

Published image names:

- `ghcr.io/shino-337/fortuna-community/fortuna-core:<tag>`
- `ghcr.io/shino-337/fortuna-community/fortuna-agent:<tag>`
- `ghcr.io/shino-337/fortuna-community/fortuna-dashboard:<tag>`

## 7. Next Steps

- Installation details: [INSTALLATION.md](INSTALLATION.md)
- User workflows: [../04-user-guide/README.md](../04-user-guide/README.md)
- Main use cases: [../04-user-guide/USE_CASES.md](../04-user-guide/USE_CASES.md)
- Operations guide: [../05-operations/DEPLOYMENT.md](../05-operations/DEPLOYMENT.md)
