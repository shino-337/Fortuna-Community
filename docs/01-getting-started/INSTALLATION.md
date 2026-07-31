# Installation Guide

This guide installs Fortuna into an existing Kubernetes cluster. Prefer published images for normal installs; use local build scripts only for development or private forks.

The public repository is `shino-337/Fortuna-Community`, and published GHCR images use `ghcr.io/shino-337/fortuna-community`.

## 1. Requirements

Read [ENVIRONMENT_REQUIREMENTS.md](ENVIRONMENT_REQUIREMENTS.md) first. The short version:

- Kubernetes 1.28+ with `kubectl` pointed at the target cluster.
- Linux host with `kubectl`. Go, Node.js, Docker, and `nerdctl` are required only when building images locally.
- Permission to create the `fortuna` namespace, RBAC, Deployments, DaemonSets, Services, PVCs, and optional Falco resources.
- Local access to the node container runtime only when using the developer local build path.

## 2. Verify the cluster

```bash
kubectl get nodes -o wide
kubectl get storageclass
./scripts/verify/check-env-rebuild-deploy.sh
```

If the cluster does not have a default StorageClass, run:

```bash
./scripts/deploy/ensure-storage-class.sh
```

## 3. Deploy from published images

GitHub Actions publishes images to GHCR:

- `ghcr.io/shino-337/fortuna-community/fortuna-core:<tag>`
- `ghcr.io/shino-337/fortuna-community/fortuna-agent:<tag>`
- `ghcr.io/shino-337/fortuna-community/fortuna-dashboard:<tag>`

Set the image tag you want to deploy:

```bash
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"
```

Create namespace, mTLS secrets, application secrets, and infrastructure:

```bash
kubectl create namespace fortuna --dry-run=client -o yaml | kubectl apply -f -
NAMESPACE=fortuna ./scripts/utils/create_mtls_secret.sh
./scripts/utils/ensure-fortuna-secrets.sh fortuna

kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml
```

If GHCR is private, create `ghcr-pull` after the namespace exists and before applying workloads:

```bash
kubectl -n fortuna create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USER" \
  --docker-password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Deploy the workloads and override images to the published registry:

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

kubectl -n fortuna set image deployment/fortuna-core core="${FORTUNA_REGISTRY}/fortuna-core:${FORTUNA_VERSION}"
kubectl -n fortuna set image daemonset/fortuna-agent agent="${FORTUNA_REGISTRY}/fortuna-agent:${FORTUNA_VERSION}"
kubectl -n fortuna set image deployment/fortuna-dashboard dashboard="${FORTUNA_REGISTRY}/fortuna-dashboard:${FORTUNA_VERSION}"
```

Anonymous pulls fail with `401 Unauthorized` when images are private and `ghcr-pull` is missing.

YAML examples for private GHCR pulls and image tag overrides are in `deploy/samples/`.

## 4. Developer local build

Use local build scripts when testing source changes or when you intentionally do not use registry images.

For a normal local rebuild and deploy:

```bash
export FORTUNA_PACKAGE_SOURCE=local
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full
```

For a complete reset where Core reruns migrations on startup:

```bash
export FORTUNA_PACKAGE_SOURCE=local
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --db-reset
```

For runtime coverage with Falco and eBPF enabled:

```bash
export FORTUNA_PACKAGE_SOURCE=local
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --full --with-runtime
```

Important pipeline behavior:

- `VERSION` controls the image tag. By default it is derived from Git.
- `FORTUNA_PACKAGE_SOURCE=local` tells the pipeline to deploy local `fortuna-*:${VERSION}` images instead of GitHub/GHCR packages.
- The default package source is `github`, which deploys `ghcr.io/shino-337/fortuna-community/*:${FORTUNA_VERSION}`.
- `SYNC_DEPLOY_IMAGE_TAG=true` by default, so deploy manifests are updated to the built tag after a successful build.
- Set `SYNC_DEPLOY_IMAGE_TAG=false` only when you intentionally want to keep existing `deploy/*.yaml` image tags.
- `--only-core`, `--only-agent`, and `--only-dashboard` rebuild and roll out a single component.
- `--with-e2e` runs the selected end-to-end suite after deploy.

## 5. Verify the deployment

```bash
kubectl get pods,svc -n fortuna -o wide
kubectl rollout status -n fortuna deployment/fortuna-core --timeout=180s
kubectl rollout status -n fortuna deployment/fortuna-dashboard --timeout=180s
kubectl rollout status -n fortuna daemonset/fortuna-agent --timeout=180s
```

Core health:

```bash
kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
curl -fsS http://127.0.0.1:8080/healthz
```

Dashboard access:

```bash
kubectl port-forward --address 0.0.0.0 -n fortuna svc/fortuna-dashboard 8081:80
```

Open `http://127.0.0.1:8081/`. The development admin account is:

- Username: `admin`
- Password: `<FORTUNA_ADMIN_PASSWORD>`, or `Fortuna_ChangeMe_123!` for a fresh deploy where `FORTUNA_ADMIN_PASSWORD` was omitted. The bootstrap default requires an immediate password change.

Set `FORTUNA_ADMIN_PASSWORD` from a secret manager before using Fortuna outside an isolated development environment.
If the database already has an `admin` user, Core does not overwrite that password with the bootstrap default.

## 6. Load security data

The normal pipeline and Core startup prepare schemas. Load CVE catalog data when the environment needs vulnerability matching:

```bash
./scripts/utils/load-cve-data.sh
```

Runtime events require a sensor path:

```bash
./scripts/deploy/install-falco-fortuna.sh
kubectl rollout restart -n fortuna daemonset/fortuna-agent
```

## 7. Clean up or reset

Targeted old-image and cache cleanup:

```bash
./scripts/clean/check-and-clean-host-resources.sh --clean
```

Full database reset without rebuild/deploy:

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh --only-db-reset
```

Remove Fortuna from the cluster:

```bash
kubectl delete namespace fortuna
```
