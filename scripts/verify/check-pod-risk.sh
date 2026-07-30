#!/bin/bash
# Check risk status for a specific pod
# Usage: ./scripts/verify/check-pod-risk.sh [pod-name-or-uid]
# Uses kubectl exec into Postgres pod (no local psql or port-forward required).

set -e
POD_ID="${1:-}"
NAMESPACE="${NAMESPACE:-fortuna}"

# If no argument, try to pick a real pod from the cluster
if [ -z "$POD_ID" ]; then
  POD_ID=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -v "fortuna-agent\|fortuna-core\|fortuna-dashboard" | head -1 | awk '{print $1}')
  [ -z "$POD_ID" ] && POD_ID="cve-2025-31133-pod"
fi

echo "=========================================="
echo "Pod Risk Check: $POD_ID"
echo "=========================================="
echo ""

# Get pod UID from Kubernetes (try fortuna namespace first, then all)
POD_UID=$(kubectl get pod "$POD_ID" -n "$NAMESPACE" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
  POD_UID=$(kubectl get pod "$POD_ID" -A -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
fi
if [ -z "$POD_UID" ]; then
  echo "⚠️  Pod not found in Kubernetes, using provided ID as UID"
  POD_UID="$POD_ID"
fi

echo "Pod UID: $POD_UID"
echo "Namespace: $NAMESPACE"
echo ""

# Find Postgres pod and run psql via kubectl exec (no port-forward or local psql)
PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi
if [ -z "$PG_POD" ]; then
  echo "❌ Postgres pod not found in namespace $NAMESPACE. Cannot query database."
  exit 1
fi

run_sql() {
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "$1" 2>&1
}

echo "=== 1. Pod in Database ==="
OUT=$(run_sql "SELECT name, uid, namespace, cluster_id, service_account, created_at FROM pods WHERE name LIKE '%$POD_ID%' OR uid = '$POD_UID' LIMIT 5;")
if [ $? -eq 0 ] && [ -n "$(echo "$OUT" | tr -d ' \n')" ]; then
  echo "$OUT"
else
  echo "❌ No rows or query failed (table may not exist or no matching pod)"
  [ -n "$OUT" ] && echo "$OUT" | head -3
fi

echo ""
echo "=== 2. Pod Capabilities ==="
OUT=$(run_sql "SELECT pod_uid, namespace, capability_id, severity, state, created_at FROM pod_capabilities WHERE pod_uid = '$POD_UID' ORDER BY created_at DESC LIMIT 10;")
if [ $? -eq 0 ] && [ -n "$(echo "$OUT" | tr -d ' \n')" ]; then
  echo "$OUT"
else
  echo "❌ No rows or query failed"
  [ -n "$OUT" ] && echo "$OUT" | head -3
fi

echo ""
echo "=== 3. Runtime Signals ==="
OUT=$(run_sql "SELECT pod_uid, signal_type, category, confidence, created_at FROM runtime_signals WHERE pod_uid = '$POD_UID' ORDER BY created_at DESC LIMIT 10;")
if [ $? -eq 0 ] && [ -n "$(echo "$OUT" | tr -d ' \n')" ]; then
  echo "$OUT"
else
  echo "❌ No rows or query failed"
  [ -n "$OUT" ] && echo "$OUT" | head -3
fi

echo ""
echo "=== 4. Risk Scores ==="
OUT=$(run_sql "SELECT resource_name, resource_uid, namespace, total_score, priority_level, calculated_at FROM risk_scores WHERE resource_name LIKE '%$POD_ID%' OR resource_uid = '$POD_UID' ORDER BY calculated_at DESC LIMIT 5;")
if [ $? -eq 0 ] && [ -n "$(echo "$OUT" | tr -d ' \n')" ]; then
  echo "$OUT"
else
  echo "❌ No rows or query failed"
  [ -n "$OUT" ] && echo "$OUT" | head -3
fi

echo ""
echo "=== 5. Insights ==="
OUT=$(run_sql "SELECT id, title, resource_name, resource_uid, severity, status, detected_at FROM insights WHERE resource_name LIKE '%$POD_ID%' OR resource_uid = '$POD_UID' ORDER BY detected_at DESC LIMIT 5;")
if [ $? -eq 0 ] && [ -n "$(echo "$OUT" | tr -d ' \n')" ]; then
  echo "$OUT"
else
  echo "❌ No rows or query failed"
  [ -n "$OUT" ] && echo "$OUT" | head -3
fi

echo ""
echo "=== 6. API Check (requires Core port-forward or ClusterIP reachable) ==="
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$CORE_POD" ]; then
  TOKEN=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"username\":\"${FORTUNA_ADMIN_USER:-admin}\",\"password\":\"${FORTUNA_ADMIN_PASSWORD:-${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}}\"}" 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
  if [ -n "$TOKEN" ]; then
    echo "Pod Capabilities API:"
    kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/pod-capabilities?podUid=$POD_UID" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -25 || echo "❌ API error"
    echo ""
    echo "Runtime Signals API:"
    kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/runtime-signals?podUid=$POD_UID" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -25 || echo "❌ API error"
  else
    echo "⚠️  Login failed; skip API check"
  fi
else
  echo "⚠️  Core pod not found; skip API check"
fi

echo ""
echo "=========================================="
echo "Check complete"
echo "=========================================="
