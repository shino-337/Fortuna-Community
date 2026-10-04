#!/usr/bin/env bash
# One-command Fortuna demo on a local kind cluster.
#
#   ./scripts/demo/up.sh        # create cluster, install Fortuna, load the S2 RBAC example
#   ./scripts/demo/down.sh      # delete everything
#
# Requires: docker, kind, kubectl, openssl. Uses published images from GHCR.
# Everything runs inside a disposable kind cluster with its own kubeconfig
# (.fortuna-demo/kubeconfig); your existing kubectl contexts are not touched.
set -euo pipefail

CLUSTER_NAME="${FORTUNA_DEMO_CLUSTER:-fortuna-demo}"
NAMESPACE=fortuna
REGISTRY="${FORTUNA_REGISTRY:-ghcr.io/shino-337/fortuna-community}"
VERSION="${FORTUNA_VERSION:-latest}"
DASHBOARD_PORT="${FORTUNA_DEMO_PORT:-8081}"
ROLLOUT_TIMEOUT="${FORTUNA_DEMO_TIMEOUT:-600s}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE_DIR="${ROOT}/.fortuna-demo"

step() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

for tool in docker kind kubectl openssl; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required (see docs/01-getting-started/DEMO.md)"
done
docker info >/dev/null 2>&1 || die "docker is installed but the daemon is not reachable"

mkdir -p "$STATE_DIR"
chmod 700 "$STATE_DIR"
export KUBECONFIG="$STATE_DIR/kubeconfig"

step "Creating kind cluster '${CLUSTER_NAME}'"
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "Cluster already exists; reusing it."
  kind export kubeconfig --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG"
else
  kind create cluster --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG" --wait 120s
fi
k() { kubectl "$@"; }

step "Preparing storage and namespace"
# The bundled manifests request the 'local-path' StorageClass; kind ships the
# same provisioner under the name 'standard'.
k apply -f - <<'EOF'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-path
provisioner: rancher.io/local-path
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
EOF
k create namespace "$NAMESPACE" --dry-run=client -o yaml | k apply -f -

step "Generating demo secrets"
if [ ! -s "$STATE_DIR/admin-password" ]; then
  # Meets the password policy: length, upper, lower, digit, symbol.
  printf 'Demo-%s-9x!' "$(openssl rand -hex 12)" > "$STATE_DIR/admin-password"
  openssl rand -hex 24 > "$STATE_DIR/postgres-password"
fi
chmod 600 "$STATE_DIR"/*
export FORTUNA_ADMIN_PASSWORD; FORTUNA_ADMIN_PASSWORD="$(cat "$STATE_DIR/admin-password")"
export FORTUNA_POSTGRES_PASSWORD; FORTUNA_POSTGRES_PASSWORD="$(cat "$STATE_DIR/postgres-password")"
export FORTUNA_DATABASE_URL="postgres://postgres:${FORTUNA_POSTGRES_PASSWORD}@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable"
export FORTUNA_BOOTSTRAP_DEFAULT_CREDENTIAL=false
# The helper scripts use plain kubectl, which follows the exported KUBECONFIG.
NAMESPACE="$NAMESPACE" "$ROOT/scripts/utils/create_mtls_secret.sh" >/dev/null
"$ROOT/scripts/utils/ensure-fortuna-secrets.sh" "$NAMESPACE" >/dev/null

step "Installing PostgreSQL and NATS"
k apply -f "$ROOT/deploy/infrastructure/postgresql-with-age.yaml"
k apply -f "$ROOT/deploy/infrastructure/nats.yaml"
k apply -f "$ROOT/deploy/infrastructure/network-policies.yaml"
k apply -f "$ROOT/deploy/fortuna-rbac.yaml"
k apply -f "$ROOT/deploy/dashboard-nginx-configmap.yaml"

step "Installing Fortuna (${REGISTRY}/fortuna-*:${VERSION})"
k apply -f "$ROOT/deploy/fortuna-core-deployment.yaml"
k apply -f "$ROOT/deploy/fortuna-agent-daemonset.yaml"
k apply -f "$ROOT/deploy/dashboard-deployment.yaml"
k -n "$NAMESPACE" set image deployment/fortuna-core core="${REGISTRY}/fortuna-core:${VERSION}" >/dev/null
k -n "$NAMESPACE" set image daemonset/fortuna-agent agent="${REGISTRY}/fortuna-agent:${VERSION}" >/dev/null
k -n "$NAMESPACE" set image deployment/fortuna-dashboard dashboard="${REGISTRY}/fortuna-dashboard:${VERSION}" >/dev/null
if [ "$VERSION" = "latest" ]; then
  # Always pull the current published build rather than a stale cached one.
  for target in deployment/fortuna-core daemonset/fortuna-agent deployment/fortuna-dashboard; do
    k -n "$NAMESPACE" patch "$target" --type=json \
      -p '[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Always"}]' >/dev/null
  done
fi

step "Waiting for Fortuna to become ready (first start runs migrations; this takes a few minutes)"
k -n "$NAMESPACE" rollout status deployment/postgres --timeout="$ROLLOUT_TIMEOUT"
k -n "$NAMESPACE" rollout status statefulset/nats --timeout="$ROLLOUT_TIMEOUT"
k -n "$NAMESPACE" rollout status deployment/fortuna-core --timeout="$ROLLOUT_TIMEOUT"
k -n "$NAMESPACE" rollout status daemonset/fortuna-agent --timeout="$ROLLOUT_TIMEOUT"
k -n "$NAMESPACE" rollout status deployment/fortuna-dashboard --timeout="$ROLLOUT_TIMEOUT"

step "Loading the example: a pod whose ServiceAccount is bound to cluster-admin"
k apply -f "$ROOT/scenarios/00-namespace.yaml"
k apply -f "$ROOT/scenarios/s2-rbac-only.yaml"
k -n fortuna-test wait --for=condition=Ready pod/rbac-pod --timeout=180s

cat <<EOF

Fortuna is running in kind cluster '${CLUSTER_NAME}'.

  1. Open the dashboard:
       export KUBECONFIG=\$PWD/.fortuna-demo/kubeconfig
       kubectl -n ${NAMESPACE} port-forward svc/fortuna-dashboard ${DASHBOARD_PORT}:80
       then browse to http://localhost:${DASHBOARD_PORT}

  2. Log in as:  admin / $(cat "$STATE_DIR/admin-password")
     (also saved in .fortuna-demo/admin-password)

  3. Investigate pod fortuna-test/rbac-pod: its ServiceAccount sa-rbac is bound
     to cluster-admin through ClusterRoleBinding crb-rbac-admin. The agent
     syncs inventory every 5 minutes, so the finding can take a few minutes to
     appear. Walkthrough: docs/01-getting-started/FIRST_FINDING.md

Remove everything with: ./scripts/demo/down.sh
EOF
