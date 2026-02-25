#!/usr/bin/env bash
# ============================================================================
# Verify pod count: cluster (kubectl) vs API (dashboard/stats, GET /pods).
# Run with Core port-forward: kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
# Or set CORE_URL (default http://localhost:8080).
# ============================================================================

set -euo pipefail

CORE_URL="${CORE_URL:-http://localhost:8080}"
API_BASE="${CORE_URL}/api/v1"
NAMESPACE="${NAMESPACE:-fortuna}"

# Actual pod count from cluster (all namespaces)
ACTUAL=$(kubectl get pods -A --no-headers 2>/dev/null | wc -l)
ACTUAL=$(echo "$ACTUAL" | tr -d ' ')
echo "=== Pod count verification ==="
echo "1. Cluster (kubectl get pods -A): $ACTUAL pods"

# Login and get token (admin/admin123 from deploy default)
extract_token() {
  python3 -c "
import sys, json
try:
  d = json.loads(sys.stdin.read())
  print(d.get('token', '') or '')
except Exception:
  print('')
"
}

LOGIN=$(curl -s -X POST "$API_BASE/auth/login" -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' 2>/dev/null || echo "{}")
TOKEN=$(echo "$LOGIN" | extract_token 2>/dev/null || echo "")

api_get() {
  local path="$1"
  if [ -n "${TOKEN:-}" ]; then
    curl -s -H "Authorization: Bearer $TOKEN" "$API_BASE/$path" 2>/dev/null || echo "{}"
    return
  fi
  local core_pod
  core_pod=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -z "$core_pod" ]; then
    echo "{}"
    return
  fi
  if [ -z "${TOKEN:-}" ]; then
    TOKEN=$(kubectl -n "$NAMESPACE" exec "$core_pod" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -d '{"username":"admin","password":"admin123"}' 2>/dev/null | extract_token 2>/dev/null || echo "")
  fi
  [ -z "${TOKEN:-}" ] && { echo "{}"; return; }
  kubectl -n "$NAMESPACE" exec "$core_pod" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
}

if [ -z "$TOKEN" ]; then
  echo "2. API: local login failed at $CORE_URL, trying in-cluster fallback..."
  STATS="$(api_get "dashboard/stats")"
  TOKEN_STATUS=$(echo "$STATS" | python3 -c "import sys,json
try:
 d=json.load(sys.stdin)
 print('ok' if ('runningPods' in d) else 'no')
except Exception:
 print('no')
" 2>/dev/null || echo "no")
  if [ "$TOKEN_STATUS" != "ok" ]; then
    echo "   Fallback login also failed. Ensure core is healthy and admin credentials are valid."
    exit 1
  fi
fi

# Dashboard stats runningPods
STATS=$(api_get "dashboard/stats")
RUNNING_PODS=$(echo "$STATS" | python3 -c "
import sys, json
try:
  d = json.load(sys.stdin)
  print(d.get('runningPods', '') or '0')
except Exception:
  print('0')
" 2>/dev/null || echo "0")
echo "2. API GET /api/v1/dashboard/stats (runningPods): $RUNNING_PODS pods"

# GET /pods total
PODS_RESP=$(api_get "pods?pageSize=1")
PODS_TOTAL=$(echo "$PODS_RESP" | python3 -c "
import sys, json
try:
  d = json.load(sys.stdin)
  print(d.get('total', '') or '0')
except Exception:
  print('0')
" 2>/dev/null || echo "0")
echo "3. API GET /api/v1/pods (total): $PODS_TOTAL pods"

echo ""
if [ -n "$ACTUAL" ] && [ -n "$PODS_TOTAL" ] && [ "$ACTUAL" = "$PODS_TOTAL" ]; then
  echo "Result: Cluster and GET /pods match ($ACTUAL pods)."
elif [ -n "$ACTUAL" ] && [ -n "$RUNNING_PODS" ] && [ "$ACTUAL" = "$RUNNING_PODS" ]; then
  echo "Result: Cluster and dashboard/stats match ($ACTUAL pods)."
else
  echo "Result: Cluster=$ACTUAL, dashboard/stats=$RUNNING_PODS, GET /pods total=$PODS_TOTAL"
  if [ -n "$ACTUAL" ] && [ -n "$PODS_TOTAL" ] && [ "$ACTUAL" != "$PODS_TOTAL" ]; then
    echo "Note: Sync may be delayed or agent may not be reporting all namespaces."
  fi
fi
