#!/bin/bash
# Check risk status for a specific pod
# Usage: ./scripts/check-pod-risk.sh <pod-name-or-uid>

set -e
POD_ID="${1:-cve-2025-31133-pod}"

echo "=========================================="
echo "Pod Risk Check: $POD_ID"
echo "=========================================="
echo ""

# Get pod UID from Kubernetes
POD_UID=$(kubectl get pod "$POD_ID" -n default -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
  echo "⚠️  Pod not found in Kubernetes, using provided ID as UID"
  POD_UID="$POD_ID"
fi

echo "Pod UID: $POD_UID"
echo ""

# Check if port-forward is running
if ! pgrep -f "port-forward.*postgres" > /dev/null; then
  echo "Starting PostgreSQL port-forward..."
  kubectl port-forward -n fortuna svc/postgres 5432:5432 > /tmp/postgres-pf.log 2>&1 &
  sleep 3
fi

echo "=== 1. Pod in Database ==="
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d fortuna -c "
  SELECT name, uid, namespace, cluster_id, service_account, created_at 
  FROM pods 
  WHERE name LIKE '%$POD_ID%' OR uid = '$POD_UID' 
  LIMIT 5;
" 2>/dev/null || echo "❌ Cannot query pods table"

echo ""
echo "=== 2. Pod Capabilities ==="
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d fortuna -c "
  SELECT pod_name, pod_uid, namespace, capability_id, severity, state, created_at 
  FROM pod_capabilities 
  WHERE pod_name LIKE '%$POD_ID%' OR pod_uid = '$POD_UID' 
  ORDER BY created_at DESC 
  LIMIT 10;
" 2>/dev/null || echo "❌ Cannot query pod_capabilities table"

echo ""
echo "=== 3. Runtime Signals ==="
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d fortuna -c "
  SELECT pod_name, pod_uid, signal_type, severity, description, detected_at 
  FROM runtime_signals 
  WHERE pod_name LIKE '%$POD_ID%' OR pod_uid = '$POD_UID' 
  ORDER BY detected_at DESC 
  LIMIT 10;
" 2>/dev/null || echo "❌ Cannot query runtime_signals table"

echo ""
echo "=== 4. Risk Scores ==="
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d fortuna -c "
  SELECT resource_name, resource_uid, namespace, total_score, priority_level, calculated_at 
  FROM risk_scores 
  WHERE resource_name LIKE '%$POD_ID%' OR resource_uid = '$POD_UID' 
  ORDER BY calculated_at DESC 
  LIMIT 5;
" 2>/dev/null || echo "❌ Cannot query risk_scores table"

echo ""
echo "=== 5. Insights ==="
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d fortuna -c "
  SELECT id, title, resource_name, resource_uid, severity, status, detected_at 
  FROM insights 
  WHERE resource_name LIKE '%$POD_ID%' OR resource_uid = '$POD_UID' 
  ORDER BY detected_at DESC 
  LIMIT 5;
" 2>/dev/null || echo "❌ Cannot query insights table"

echo ""
echo "=== 6. API Check ==="
echo "Pod Capabilities API:"
curl -s "http://localhost:8080/api/v1/pod-capabilities?podUid=$POD_UID" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30 || echo "❌ API error"

echo ""
echo "Runtime Signals API:"
curl -s "http://localhost:8080/api/v1/runtime-signals?podUid=$POD_UID" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30 || echo "❌ API error"

echo ""
echo "=========================================="
echo "Check complete"
echo "=========================================="
