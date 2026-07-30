#!/usr/bin/env bash
# =============================================================================
# Sync Fortuna Agent to remote Kubernetes clusters
# =============================================================================
# Management cluster runs Core/Dashboard/PostgreSQL/NATS. Remote clusters run
# Agent only and send telemetry back to the management Core endpoint.
#
# Required:
#   REMOTE_KUBECONFIGS="cluster02=/path/to/cluster02.kubeconfig[,cluster03=/path]"
#   MANAGEMENT_NODE=<management-node-ip-or-dns>
#
# Image modes:
#   REMOTE_IMAGE_MODE=registry  Remote pulls REMOTE_AGENT_IMAGE from registry.
#   REMOTE_IMAGE_MODE=local     Import local fortuna-agent image into remote nodes via SSH.
#   REMOTE_IMAGE_MODE=auto      registry when image contains '/', otherwise local.
#
# Common image env:
#   REMOTE_AGENT_IMAGE=ghcr.io/org/repo/fortuna-agent:vX.Y.Z
#   FORTUNA_REGISTRY=ghcr.io/org/repo
#   FORTUNA_VERSION=latest
#   VERSION=<local-build-tag>
#   REMOTE_IMAGE_PULL_POLICY=Always|IfNotPresent
#
# Local image mode requires SSH credentials accepted by push-images-to-workers.sh:
#   SSH_USER/SSH_PASS or PUSH_CONFIG_FILE/scripts/utils/push-images.config.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"

NAMESPACE="${NAMESPACE:-fortuna}"
REMOTE_KUBECONFIGS="${REMOTE_KUBECONFIGS:-}"
MANAGEMENT_NODE="${MANAGEMENT_NODE:-}"
CORE_HTTP_ENDPOINT="${CORE_HTTP_ENDPOINT:-}"
CORE_GRPC_ENDPOINT="${CORE_GRPC_ENDPOINT:-}"
REMOTE_IMAGE_MODE="${REMOTE_IMAGE_MODE:-auto}"
REMOTE_IMAGE_PULL_POLICY="${REMOTE_IMAGE_PULL_POLICY:-}"
REMOTE_SET_CLUSTER_NAME="${REMOTE_SET_CLUSTER_NAME:-true}"
REMOTE_ROLLOUT_TIMEOUT="${REMOTE_ROLLOUT_TIMEOUT:-180s}"
SKIP_CORE_EXTERNAL_SERVICE="${SKIP_CORE_EXTERNAL_SERVICE:-false}"

VERSION="${VERSION:-$(cd "$PROJECT_ROOT" && git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo latest)}"
FORTUNA_VERSION="${FORTUNA_VERSION:-$VERSION}"

if [ -n "${REMOTE_AGENT_IMAGE:-}" ]; then
  AGENT_IMAGE="$REMOTE_AGENT_IMAGE"
elif [ -n "${FORTUNA_REGISTRY:-}" ]; then
  AGENT_IMAGE="${FORTUNA_REGISTRY%/}/fortuna-agent:${FORTUNA_VERSION}"
else
  AGENT_IMAGE="fortuna-agent:${VERSION}"
fi

if [ -z "$REMOTE_KUBECONFIGS" ]; then
  echo "[INFO] REMOTE_KUBECONFIGS is empty; skipping remote Agent sync."
  exit 0
fi

if [ -z "$CORE_HTTP_ENDPOINT" ] || [ -z "$CORE_GRPC_ENDPOINT" ]; then
  if [ -z "$MANAGEMENT_NODE" ]; then
    echo "[ERR] MANAGEMENT_NODE is required unless CORE_HTTP_ENDPOINT and CORE_GRPC_ENDPOINT are set." >&2
    exit 1
  fi
  CORE_HTTP_ENDPOINT="${CORE_HTTP_ENDPOINT:-http://${MANAGEMENT_NODE}:30080}"
  CORE_GRPC_ENDPOINT="${CORE_GRPC_ENDPOINT:-${MANAGEMENT_NODE}:30090}"
fi

if [ "$REMOTE_IMAGE_MODE" = "auto" ]; then
  case "$AGENT_IMAGE" in
    */*) REMOTE_IMAGE_MODE="registry" ;;
    *)   REMOTE_IMAGE_MODE="local" ;;
  esac
fi

if [ -z "$REMOTE_IMAGE_PULL_POLICY" ]; then
  case "$AGENT_IMAGE" in
    *:latest) REMOTE_IMAGE_PULL_POLICY="Always" ;;
    *)        REMOTE_IMAGE_PULL_POLICY="IfNotPresent" ;;
  esac
fi

log() { printf '[INFO] %s\n' "$*"; }
ok() { printf '[OK]   %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
fail() { printf '[ERR]  %s\n' "$*" >&2; exit 1; }

require_file() {
  [ -f "$1" ] || fail "Required file not found: $1"
}

require_file "$PROJECT_ROOT/deploy/fortuna-rbac.yaml"
require_file "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml"

if [ "$REMOTE_IMAGE_MODE" = "local" ]; then
  [ -x "$SCRIPTS/utils/push-images-to-workers.sh" ] || fail "Missing executable: $SCRIPTS/utils/push-images-to-workers.sh"
fi

if [ "$SKIP_CORE_EXTERNAL_SERVICE" != "true" ] && [ -f "$PROJECT_ROOT/deploy/fortuna-core-external-service.yaml" ]; then
  log "Ensuring management Core external service ($NAMESPACE/fortuna-core-external)..."
  kubectl -n "$NAMESPACE" apply -f "$PROJECT_ROOT/deploy/fortuna-core-external-service.yaml" >/dev/null
fi

if [ -n "${FORTUNA_INGEST_TOKEN:-}" ]; then
  ingest_token="$FORTUNA_INGEST_TOKEN"
else
  ingest_token="$(kubectl -n "$NAMESPACE" get secret fortuna-secrets -o jsonpath='{.data.ingest-token}' 2>/dev/null | base64 -d 2>/dev/null || true)"
fi
[ -n "$ingest_token" ] || fail "Could not resolve ingest token. Set FORTUNA_INGEST_TOKEN or create management fortuna-secrets first."

sync_one_remote() {
  local name="$1"
  local kubeconfig="$2"

  [ -f "$kubeconfig" ] || { warn "Skipping $name: kubeconfig not found: $kubeconfig"; return 1; }

  log "Syncing remote Agent: name=$name kubeconfig=$kubeconfig image=$AGENT_IMAGE mode=$REMOTE_IMAGE_MODE"
  KUBECONFIG="$kubeconfig" kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | \
    KUBECONFIG="$kubeconfig" kubectl apply -f - >/dev/null

  KUBECONFIG="$kubeconfig" NAMESPACE="$NAMESPACE" "$SCRIPTS/utils/create_mtls_secret.sh" >/dev/null

  KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" create secret generic fortuna-secrets \
    --from-literal=ingest-token="$ingest_token" \
    --dry-run=client -o yaml | KUBECONFIG="$kubeconfig" kubectl apply -f - >/dev/null

  KUBECONFIG="$kubeconfig" kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-rbac.yaml" >/dev/null
  KUBECONFIG="$kubeconfig" kubectl apply -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" >/dev/null

  if [ "$REMOTE_IMAGE_MODE" = "local" ]; then
    log "Importing local Agent image into remote nodes for $name..."
    KUBECONFIG="$kubeconfig" AGENT_IMAGE="$AGENT_IMAGE" \
      "$SCRIPTS/utils/push-images-to-workers.sh" --agent-only --build-if-missing --no-dashboard
  fi

  KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" set image daemonset/fortuna-agent "agent=$AGENT_IMAGE" >/dev/null
  KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" patch daemonset/fortuna-agent \
    --type=strategic \
    -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"agent\",\"imagePullPolicy\":\"$REMOTE_IMAGE_PULL_POLICY\"}]}}}}" >/dev/null
  KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" set env daemonset/fortuna-agent \
    CORE_HTTP_ENDPOINT="$CORE_HTTP_ENDPOINT" \
    CORE_GRPC_ENDPOINT="$CORE_GRPC_ENDPOINT" \
    TLS_ENABLED=true >/dev/null

  if [ "$REMOTE_SET_CLUSTER_NAME" = "true" ]; then
    KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" set env daemonset/fortuna-agent "CLUSTER_NAME=$name" >/dev/null
  fi

  KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" rollout restart daemonset/fortuna-agent >/dev/null
  KUBECONFIG="$kubeconfig" kubectl -n "$NAMESPACE" rollout status daemonset/fortuna-agent --timeout="$REMOTE_ROLLOUT_TIMEOUT"
  ok "Remote Agent synced: $name"
}

failed=0
IFS=',' read -r -a entries <<< "$REMOTE_KUBECONFIGS"
for raw_entry in "${entries[@]}"; do
  entry="$(printf '%s' "$raw_entry" | xargs)"
  [ -n "$entry" ] || continue
  if [[ "$entry" == *"="* ]]; then
    name="${entry%%=*}"
    kubeconfig="${entry#*=}"
  else
    kubeconfig="$entry"
    name="$(basename "$kubeconfig" .kubeconfig)"
  fi
  if ! sync_one_remote "$name" "$kubeconfig"; then
    failed=$((failed + 1))
  fi
done

if [ "$failed" -gt 0 ]; then
  fail "$failed remote cluster(s) failed to sync"
fi

ok "Remote cluster Agent sync complete"
