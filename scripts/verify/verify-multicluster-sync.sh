#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
CORE_URL="${CORE_URL:-http://127.0.0.1:8080}"
FORTUNA_JWT="${FORTUNA_JWT:-}"
LOCAL_CONTEXT="${LOCAL_CONTEXT:-}"
REMOTE_KUBECONFIGS="${REMOTE_KUBECONFIGS:-}"

log() { printf '[INFO] %s\n' "$*"; }
ok() { printf '[OK]   %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
fail() { printf '[FAIL] %s\n' "$*" >&2; exit 1; }

kubectl_args=()
if [ -n "$LOCAL_CONTEXT" ]; then
  kubectl_args+=(--context "$LOCAL_CONTEXT")
fi

if [ -n "$FORTUNA_JWT" ] && ! command -v jq >/dev/null 2>&1; then
  fail "jq is required when FORTUNA_JWT is set; install jq or unset FORTUNA_JWT to run DB/Kubernetes checks only"
fi

postgres_pod="$(kubectl "${kubectl_args[@]}" -n "$NAMESPACE" get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [ -z "$postgres_pod" ]; then
  fail "Postgres pod not found in namespace=$NAMESPACE"
fi

log "DB active pod counts by cluster"
kubectl "${kubectl_args[@]}" -n "$NAMESPACE" exec "$postgres_pod" -- psql -U postgres -d fortuna -c \
  "SELECT cluster_id, COUNT(*) FILTER (WHERE deleted_at IS NULL) AS active_pods, MAX(updated_at) AS last_pod_update FROM pods GROUP BY cluster_id ORDER BY cluster_id;"

log "DB active clusters"
kubectl "${kubectl_args[@]}" -n "$NAMESPACE" exec "$postgres_pod" -- psql -U postgres -d fortuna -c \
  "SELECT id, name, source, last_sync, deleted_at FROM clusters ORDER BY id;"

log "Local Kubernetes pod count"
local_count="$(kubectl "${kubectl_args[@]}" get pods -A --no-headers | wc -l | tr -d ' ')"
ok "local pods=$local_count"

if [ -n "$REMOTE_KUBECONFIGS" ]; then
  IFS=',' read -r -a entries <<< "$REMOTE_KUBECONFIGS"
  for entry in "${entries[@]}"; do
    name="${entry%%=*}"
    kubeconfig="${entry#*=}"
    if [ -z "$name" ] || [ -z "$kubeconfig" ] || [ "$name" = "$kubeconfig" ]; then
      warn "Skipping invalid REMOTE_KUBECONFIGS entry: $entry"
      continue
    fi
    if [ ! -f "$kubeconfig" ]; then
      warn "Remote kubeconfig not found for $name: $kubeconfig"
      continue
    fi
    count="$(KUBECONFIG="$kubeconfig" kubectl get pods -A --no-headers | wc -l | tr -d ' ')"
    ok "remote $name pods=$count"
  done
fi

if [ -n "$FORTUNA_JWT" ]; then
  log "API /inventory/clusters/stats"
  curl -fsS -H "Authorization: Bearer $FORTUNA_JWT" "$CORE_URL/api/v1/inventory/clusters/stats" | jq '.clusters[] | {id,name,podCount,riskCount,agentCount,lastSync}'

  log "API /dashboard/stats?byType=all"
  curl -fsS -H "Authorization: Bearer $FORTUNA_JWT" "$CORE_URL/api/v1/dashboard/stats?byType=all" | jq '{totalClusters,activeAgents,runningPods,totalRisks,criticalRisks,affectedPodCount}'
else
  warn "FORTUNA_JWT not set; skipped API checks. Port-forward Core and pass FORTUNA_JWT to include dashboard/API verification."
fi
