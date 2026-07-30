#!/usr/bin/env bash
# Verify Falco -> JSONL -> fortuna-agent -> Core v2 runtime ingest -> DB.
set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
TEST_NAMESPACE="${TEST_NAMESPACE:-fortuna-falco-e2e}"
TRIGGER_POD="${TRIGGER_POD:-falco-k8s-api-trigger}"
FALCO_EVENTS_PATH="${FALCO_EVENTS_PATH:-/var/log/falco/events.jsonl}"
WAIT_SECONDS="${WAIT_SECONDS:-25}"
CLEANUP="${CLEANUP:-false}"
KEEP_TRIGGER_RUNNING="${KEEP_TRIGGER_RUNNING:-true}"
TRIGGER_HOLD_SECONDS="${TRIGGER_HOLD_SECONDS:-3600}"
INVENTORY_WAIT_SECONDS="${INVENTORY_WAIT_SECONDS:-180}"
INVENTORY_RESTART_AGENT_ON_MISS="${INVENTORY_RESTART_AGENT_ON_MISS:-true}"

ok() { echo "[OK] $*"; }
warn() { echo "[WARN] $*"; }
fail() { echo "[FAIL] $*" >&2; exit 1; }

run_sql() {
  kubectl exec -n "$NAMESPACE" deploy/postgres -- psql -U postgres -d fortuna -Atc "$1"
}

count_sql() {
  local value
  value="$(run_sql "$1" 2>/dev/null || echo "0")"
  echo "$value" | tr -d '[:space:]'
}

falco_pod() {
  kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=falco -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true
}

agent_pod() {
  kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true
}

wait_inventory_pod() {
  local uid="$1" max_seconds="${2:-180}" attempts db_count waited
  attempts=$((max_seconds / 5))
  [ "$attempts" -gt 0 ] || attempts=1
  waited=0
  for _ in $(seq 1 "$attempts"); do
    db_count="$(count_sql "SELECT COUNT(*) FROM pods WHERE uid='$uid' AND deleted_at IS NULL;")"
    if [ "${db_count:-0}" -gt 0 ]; then
      return 0
    fi
    if [ "$waited" -gt 0 ] && [ $((waited % 30)) -eq 0 ]; then
      echo "[INFO] Waiting for inventory pod sync uid=$uid (${waited}/${max_seconds}s)..."
    fi
    sleep 5
    waited=$((waited + 5))
  done
  return 1
}

echo "== Falco runtime E2E =="
kubectl get daemonset/falco -n "$NAMESPACE" >/dev/null 2>&1 || fail "Falco DaemonSet not found in namespace $NAMESPACE"
kubectl rollout status daemonset/falco -n "$NAMESPACE" --timeout=120s >/dev/null || fail "Falco DaemonSet is not ready"
kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s >/dev/null || fail "fortuna-agent DaemonSet is not ready"
ok "Falco and agent DaemonSets are ready"

FALCO_POD="$(falco_pod)"
AGENT_POD="$(agent_pod)"
[ -n "$FALCO_POD" ] || fail "Falco pod not found"
[ -n "$AGENT_POD" ] || fail "Agent pod not found"

FALCO_ENV="$(kubectl get daemonset/fortuna-agent -n "$NAMESPACE" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="FALCO_EVENTS_ENABLED")].value}' 2>/dev/null || true)"
[ "$FALCO_ENV" = "true" ] || fail "Agent FALCO_EVENTS_ENABLED is not true"
ok "Agent Falco reader env is enabled"

BEFORE_FALCO="$(count_sql "SELECT COUNT(*) FROM runtime_events WHERE lower(coalesce(source_kind, runtime, ''))='falco' OR lower(coalesce(runtime, source_kind, ''))='falco';")"
BEFORE_SIGNALS="$(count_sql "SELECT COUNT(*) FROM runtime_signals;")"
ok "DB before: falco_events=$BEFORE_FALCO runtime_signals=$BEFORE_SIGNALS"

echo "[INFO] Truncating Falco JSONL and restarting agent reader to avoid backlog skip..."
kubectl exec -n "$NAMESPACE" "$FALCO_POD" -c falco -- sh -c ": > '$FALCO_EVENTS_PATH'" >/dev/null
kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" >/dev/null
kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s >/dev/null || fail "Agent rollout after Falco JSONL reset failed"
AGENT_POD="$(agent_pod)"
ok "Agent restarted: $AGENT_POD"

kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl delete pod "$TRIGGER_POD" -n "$TEST_NAMESPACE" --ignore-not-found=true --wait=true >/dev/null

echo "[INFO] Creating pod that contacts Kubernetes API to trigger Falco rule..."
if [ "$KEEP_TRIGGER_RUNNING" = "true" ]; then
  kubectl run "$TRIGGER_POD" -n "$TEST_NAMESPACE" \
    --image=curlimages/curl:8.10.1 \
    --restart=Never \
    --labels=app="$TRIGGER_POD",fortuna.dev/runtime-test=falco \
    --command -- sh -c "for i in \$(seq 1 12); do curl -k -s --connect-timeout 2 https://kubernetes.default.svc >/dev/null || true; sleep 5; done; sleep ${TRIGGER_HOLD_SECONDS}" >/dev/null
else
  kubectl run "$TRIGGER_POD" -n "$TEST_NAMESPACE" \
    --image=curlimages/curl:8.10.1 \
    --restart=Never \
    --labels=app="$TRIGGER_POD",fortuna.dev/runtime-test=falco \
    --command -- sh -c 'for i in $(seq 1 8); do curl -k -s --connect-timeout 2 https://kubernetes.default.svc >/dev/null || true; sleep 2; done' >/dev/null
fi

kubectl wait --for=condition=Ready "pod/$TRIGGER_POD" -n "$TEST_NAMESPACE" --timeout=60s >/dev/null || true
sleep "$WAIT_SECONDS"

JSONL_MATCHES="$(kubectl exec -n "$NAMESPACE" "$FALCO_POD" -c falco -- sh -c "grep -c '$TRIGGER_POD' '$FALCO_EVENTS_PATH' 2>/dev/null || true" | tr -d '[:space:]')"
[ "${JSONL_MATCHES:-0}" -gt 0 ] || fail "Falco JSONL did not record trigger pod $TRIGGER_POD"
ok "Falco JSONL recorded $JSONL_MATCHES event(s) for $TRIGGER_POD"

AFTER_FALCO="$(count_sql "SELECT COUNT(*) FROM runtime_events WHERE (lower(coalesce(source_kind, runtime, ''))='falco' OR lower(coalesce(runtime, source_kind, ''))='falco') AND namespace='$TEST_NAMESPACE' AND pod_name='$TRIGGER_POD';")"
[ "${AFTER_FALCO:-0}" -gt 0 ] || fail "DB has no Falco runtime_events for $TEST_NAMESPACE/$TRIGGER_POD"
ok "DB Falco runtime_events for trigger pod: $AFTER_FALCO"

TRIGGER_UID="$(kubectl get pod "$TRIGGER_POD" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null || true)"
if [ -n "$TRIGGER_UID" ]; then
  if wait_inventory_pod "$TRIGGER_UID" "$INVENTORY_WAIT_SECONDS"; then
    ok "Inventory pod synced for dashboard: $TEST_NAMESPACE/$TRIGGER_POD uid=$TRIGGER_UID"
  else
    if [ "$INVENTORY_RESTART_AGENT_ON_MISS" = "true" ]; then
      warn "Runtime events ingested but inventory pod is not synced yet; restarting agent once to force initial full sync while trigger pod is Running"
      kubectl rollout restart daemonset/fortuna-agent -n "$NAMESPACE" >/dev/null
      kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s >/dev/null || fail "Agent rollout after inventory sync miss failed"
      AGENT_POD="$(agent_pod)"
      if wait_inventory_pod "$TRIGGER_UID" "$INVENTORY_WAIT_SECONDS"; then
        ok "Inventory pod synced for dashboard after agent restart: $TEST_NAMESPACE/$TRIGGER_POD uid=$TRIGGER_UID"
      else
        warn "Runtime events ingested but inventory pod not synced yet; dashboard pod list may lag until next agent full sync"
      fi
    else
      warn "Runtime events ingested but inventory pod not synced yet; dashboard pod list may lag until next agent full sync"
    fi
  fi
fi

AFTER_SIGNALS="$(count_sql "SELECT COUNT(*) FROM runtime_signals;")"
if [ "${AFTER_SIGNALS:-0}" -gt "${BEFORE_SIGNALS:-0}" ]; then
  ok "runtime_signals increased: $BEFORE_SIGNALS -> $AFTER_SIGNALS"
else
  warn "runtime_signals did not increase; event ingest passed, signal classification may not map this rule"
fi

kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail=220 | grep -Ei 'FalcoEvents|IngestQuality|send_ok' || warn "No recent FalcoEvents send_ok line in agent logs"

if [ "$CLEANUP" = "true" ]; then
  kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true >/dev/null
  ok "Cleaned test namespace $TEST_NAMESPACE"
elif [ "$KEEP_TRIGGER_RUNNING" = "true" ]; then
  ok "Trigger pod kept Running for dashboard visibility: $TEST_NAMESPACE/$TRIGGER_POD (hold=${TRIGGER_HOLD_SECONDS}s)"
fi

ok "Falco runtime E2E passed"
