# Fortuna Production Deployment Guide

Production deployments should use registry-published images, explicit secrets, and repeatable rollout verification. For a shorter command checklist, see [DEPLOYMENT_CHECKLIST.md](DEPLOYMENT_CHECKLIST.md).

## Production Defaults

- Use immutable image tags or digests from a registry reachable by every node.
- Do not rely on local containerd images for multi-node production clusters.
- Set `FORTUNA_ADMIN_PASSWORD` from a secret manager.
- Keep the first-login bootstrap default only for isolated local installs.
- Generate and mount mTLS secrets before Core/Agent rollout.
- Keep PostgreSQL and NATS in the same namespace unless you explicitly override endpoints.

## Required Inputs

```bash
export NAMESPACE="fortuna"
export FORTUNA_REGISTRY="ghcr.io/shino-337/fortuna-community"
export FORTUNA_VERSION="v1.0.0"
export FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
export FORTUNA_ADMIN_PASSWORD="<strong-admin-password>"
export FORTUNA_POSTGRES_PASSWORD="$(openssl rand -base64 24 | tr -d '=+/ ' | cut -c1-24)"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
```

`FORTUNA_VERSION` must be a tag that exists in the registry. The GitHub image workflow publishes `latest` and `sha-<12-char-commit>` on every `main` push, `v*` release tags when those tags are pushed, and custom tags from manual workflow dispatch. Local `git describe` tags written into `deploy/*.yaml` are not registry tags unless the workflow has published that exact value.

If your registry is private, create an image pull secret:

```bash
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "$NAMESPACE" create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username="$GITHUB_USER" \
  --docker-password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

YAML templates for private GHCR pulls are in `deploy/samples/`.

## Secrets

Preferred path:

```bash
NAMESPACE="$NAMESPACE" ./scripts/utils/create_mtls_secret.sh
./scripts/utils/ensure-fortuna-secrets.sh "$NAMESPACE"
```

`ensure-fortuna-secrets.sh` creates:

- `fortuna-secrets/database-url`
- `fortuna-secrets/jwt-secret`
- `fortuna-secrets/admin-password`
- `fortuna-secrets/bootstrap-default-credential`
- optional ingest and pod-detail encryption keys
- `postgres-credentials` when `FORTUNA_POSTGRES_PASSWORD` is set

Production should set `FORTUNA_ADMIN_PASSWORD`. If it is omitted, the script writes `Fortuna_ChangeMe_123!` and `bootstrap-default-credential=true`, and Core requires a first-login password change.

## Infrastructure

```bash
kubectl apply -f deploy/infrastructure/postgresql-with-age.yaml
kubectl apply -f deploy/infrastructure/nats.yaml
kubectl -n "$NAMESPACE" wait --for=condition=ready pod -l app=postgres --timeout=300s
kubectl -n "$NAMESPACE" wait --for=condition=ready pod -l app=nats --timeout=300s
```

Use `deploy/infrastructure/postgresql.yaml` only when the simpler PostgreSQL fallback is intentional.

## RBAC And Image Pull

```bash
kubectl apply -f deploy/fortuna-rbac.yaml
kubectl apply -f deploy/dashboard-nginx-configmap.yaml

if kubectl -n "$NAMESPACE" get secret ghcr-pull >/dev/null 2>&1; then
  kubectl -n "$NAMESPACE" patch serviceaccount fortuna-core \
    -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
  kubectl -n "$NAMESPACE" patch serviceaccount fortuna-agent \
    -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
  kubectl -n "$NAMESPACE" patch serviceaccount default \
    -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
fi
```

## Workload Rollout

```bash
kubectl apply -f deploy/fortuna-core-deployment.yaml
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
kubectl apply -f deploy/dashboard-deployment.yaml

kubectl -n "$NAMESPACE" set image deployment/fortuna-core \
  core="${FORTUNA_REGISTRY}/fortuna-core:${FORTUNA_VERSION}"
kubectl -n "$NAMESPACE" set image daemonset/fortuna-agent \
  agent="${FORTUNA_REGISTRY}/fortuna-agent:${FORTUNA_VERSION}"
kubectl -n "$NAMESPACE" set image deployment/fortuna-dashboard \
  dashboard="${FORTUNA_REGISTRY}/fortuna-dashboard:${FORTUNA_VERSION}"

kubectl -n "$NAMESPACE" rollout status deployment/fortuna-core --timeout=300s
kubectl -n "$NAMESPACE" rollout status daemonset/fortuna-agent --timeout=300s
kubectl -n "$NAMESPACE" rollout status deployment/fortuna-dashboard --timeout=300s
```

## Verification

```bash
kubectl get pods,svc -n "$NAMESPACE" -o wide
kubectl -n "$NAMESPACE" exec deploy/fortuna-core -- curl -fsS http://localhost:8080/healthz
kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=agent --tail=80
```

Verify migrations and auth state:

```bash
kubectl exec -n "$NAMESPACE" deploy/postgres -- psql -U postgres -d fortuna -c \
  "select max(version), count(*) from schema_migrations;"
kubectl exec -n "$NAMESPACE" deploy/postgres -- psql -U postgres -d fortuna -c \
  "select username,role,must_change_password,bootstrap_credential,password_changed_at from users order by id;"
```

Dashboard:

```bash
kubectl port-forward --address 0.0.0.0 -n "$NAMESPACE" svc/fortuna-dashboard 8081:80
```

Open `http://127.0.0.1:8081/` and log in as `admin` with `FORTUNA_ADMIN_PASSWORD`.

## CVE And Runtime Data

Load CVE data when vulnerability matching is required:

```bash
./scripts/utils/load-cve-data.sh
```

Runtime/Falco path:

```bash
./scripts/deploy/install-falco-fortuna.sh
kubectl rollout restart -n "$NAMESPACE" daemonset/fortuna-agent
kubectl -n "$NAMESPACE" rollout status daemonset/fortuna-agent --timeout=180s
```

## Operational Notes

- Existing databases keep the current `admin` password. Core will not overwrite an existing admin with `Fortuna_ChangeMe_123!`.
- If a rebuild appears deployed but behavior is old, compare `kubectl get deploy -o jsonpath='{.spec.template.spec.containers[0].image}'` with the expected tag and check pod `imageID`.
- For local development only, `./scripts/utils/push-images-to-workers.sh` can copy runtime images to nodes. Production should use a registry reachable by every node.
- Use [Deployment](DEPLOYMENT.md) for image pull, migrations, DNS, and bootstrap auth checks.
