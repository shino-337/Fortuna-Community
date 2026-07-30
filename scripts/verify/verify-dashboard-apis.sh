#!/usr/bin/env bash
# ============================================================================
# Verify all APIs used by the Dashboard return data (run from cluster).
# Use after port-forward or when debugging "data not showing on Dashboard".
# ============================================================================

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
  echo "ERROR: Core pod not found in namespace $NAMESPACE"
  exit 1
fi

echo "Using Core pod: $CORE_POD"
echo "Getting JWT..."
TOKEN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${FORTUNA_ADMIN_USER:-admin}\",\"password\":\"${FORTUNA_ADMIN_PASSWORD:-${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}}\"}" 2>/dev/null | python3 -c "
import sys, json
try:
  d = json.load(sys.stdin)
  print(d.get('token', '') or '')
except Exception:
  print('')
" 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
  echo "ERROR: Could not get JWT (login failed). Check admin credentials/bootstrap default or Core auth."
  exit 1
fi

echo "JWT obtained."
echo ""

api_get() {
  kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/$1" 2>/dev/null || echo '{"error":"request failed"}'
}

check_key() {
  local body="$1"
  local key="$2"
  if echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('$key','MISSING'))" 2>/dev/null | grep -qv MISSING; then
    echo "  OK (has $key)"
  else
    echo "  WARN (missing or invalid $key)"
  fi
}

echo "=== 1. GET /api/v1/dashboard/stats (Dashboard numbers) ==="
BODY=$(api_get "dashboard/stats")
echo "$BODY" | python3 -m json.tool 2>/dev/null || echo "$BODY"
check_key "$BODY" "totalClusters"
check_key "$BODY" "totalRisks"
check_key "$BODY" "runningPods"
check_key "$BODY" "activeAgents"
echo ""

echo "=== 2. GET /api/v1/inventory/clusters (Cluster Health list) ==="
BODY=$(api_get "inventory/clusters")
COUNT=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('clusters',[])))" 2>/dev/null || echo "0")
echo "  clusters count: $COUNT"
echo ""

echo "=== 3. GET /api/v1/risk/insights (Critical Risks / Risk Center) ==="
BODY=$(api_get "risk/insights")
COUNT=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('insights',[])))" 2>/dev/null || echo "0")
echo "  insights count: $COUNT"
echo ""

echo "=== 4. GET /api/v1/notifications (Recent Activity) ==="
BODY=$(api_get "notifications")
COUNT=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('notifications',[])))" 2>/dev/null || echo "0")
echo "  notifications count: $COUNT"
echo ""

echo "=== 5. GET /api/v1/dashboard/metrics/threat-velocity?days=7 (Threat Velocity chart) ==="
BODY=$(api_get "dashboard/metrics/threat-velocity?days=7")
COUNT=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('trend',[])))" 2>/dev/null || echo "0")
echo "  trend points: $COUNT"
echo ""

echo "=== 6. GET /api/v1/inventory/pod-capabilities/summary/capability (Pod Capabilities list) ==="
BODY=$(api_get "inventory/pod-capabilities/summary/capability")
COUNT=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('summary',[])))" 2>/dev/null || echo "0")
echo "  summary count: $COUNT"
echo ""

echo "=== 7. GET /api/v1/inventory/pod-capabilities/trends?days=7 (PCE Trend chart) ==="
BODY=$(api_get "inventory/pod-capabilities/trends?days=7")
COUNT=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('points',[])))" 2>/dev/null || echo "0")
echo "  points count: $COUNT"
echo ""

echo "Done. If all show OK/counts, Dashboard should display data when logged in with configured admin credentials."
echo "If Dashboard still empty: 1) Log in first. 2) Check browser Network tab for 401/404. 3) Ensure port-forward to dashboard (8081:80) and dashboard proxies /api to Core."
