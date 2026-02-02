#!/bin/bash
# Verify pod data in database and API
# Usage: ./scripts/verify-pod-data.sh <pod-name-or-uid>

set -e

POD_ID="${1:-cve-2025-31133-pod}"

echo "=========================================="
echo "Pod Data Verification: $POD_ID"
echo "=========================================="
echo ""

# Get pod UID from Kubernetes if pod exists
POD_UID=$(kubectl get pod "$POD_ID" -n default -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
  echo "⚠️  Pod not found in Kubernetes, using provided ID as UID"
  POD_UID="$POD_ID"
fi

echo "Pod UID: $POD_UID"
echo ""

# Setup port-forwards
if ! pgrep -f "port-forward.*postgres" > /dev/null; then
  echo "Starting PostgreSQL port-forward..."
  kubectl port-forward -n fortuna svc/postgres 5432:5432 > /tmp/postgres-pf.log 2>&1 &
  sleep 3
fi

if ! pgrep -f "port-forward.*fortuna-core.*8080" > /dev/null; then
  echo "Starting Core API port-forward..."
  kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 > /tmp/core-pf.log 2>&1 &
  sleep 2
fi

CORE_API="http://localhost:8080"

# Query database via postgres pod
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$POSTGRES_POD" ]; then
  echo "❌ PostgreSQL pod not found"
  exit 1
fi

echo "=== 1. Pod in Database ==="
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT 
    name, 
    uid, 
    namespace, 
    cluster_id, 
    service_account,
    created_at 
  FROM pods 
  WHERE name LIKE '%$POD_ID%' OR uid = '$POD_UID' 
  ORDER BY created_at DESC 
  LIMIT 5;
" 2>/dev/null || echo "❌ Query failed"

echo ""
echo "=== 2. Pod Capabilities (with pod_name) ==="
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT 
    pc.pod_name,
    pc.pod_uid, 
    pc.namespace, 
    pc.capability_id, 
    pc.severity, 
    pc.state,
    p.name as pod_name_from_join,
    pc.created_at 
  FROM pod_capabilities pc
  LEFT JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL
  WHERE pc.pod_name LIKE '%$POD_ID%' OR pc.pod_uid = '$POD_UID' 
  ORDER BY pc.created_at DESC 
  LIMIT 10;
" 2>/dev/null || echo "❌ Query failed"

echo ""
echo "=== 3. Runtime Signals ==="
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT 
    pod_name, 
    pod_uid, 
    signal_type, 
    severity, 
    description,
    detected_at 
  FROM runtime_signals 
  WHERE pod_name LIKE '%$POD_ID%' OR pod_uid = '$POD_UID' 
  ORDER BY detected_at DESC 
  LIMIT 10;
" 2>/dev/null || echo "❌ Query failed"

echo ""
echo "=== 4. Risk Scores ==="
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT 
    resource_name, 
    resource_uid, 
    namespace, 
    total_score, 
    priority_level,
    calculated_at 
  FROM risk_scores 
  WHERE resource_name LIKE '%$POD_ID%' OR resource_uid = '$POD_UID' 
  ORDER BY calculated_at DESC 
  LIMIT 5;
" 2>/dev/null || echo "❌ Query failed"

echo ""
echo "=== 5. Insights ==="
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT 
    id, 
    title, 
    resource_name, 
    resource_uid, 
    severity, 
    status, 
    detected_at 
  FROM insights 
  WHERE resource_name LIKE '%$POD_ID%' OR resource_uid = '$POD_UID' 
  ORDER BY detected_at DESC 
  LIMIT 5;
" 2>/dev/null || echo "❌ Query failed"

echo ""
echo "=== 6. API: Pod Capabilities (by podUid) ==="
curl -s "$CORE_API/api/v1/pod-capabilities?podUid=$POD_UID" 2>/dev/null | \
  python3 -c "import sys, json; d=json.load(sys.stdin); caps=d.get('capabilities',[]); print(f'Total: {len(caps)}'); [print(f\"  - {c.get('podName','N/A')} | {c.get('podUid','')[:8]}... | {c.get('capabilityId','')} | {c.get('severity','')}\") for c in caps[:5]]" 2>/dev/null || \
  echo "❌ API error"

echo ""
echo "=== 7. API: Pod Capabilities (by podName) ==="
curl -s "$CORE_API/api/v1/pod-capabilities?podName=$POD_ID" 2>/dev/null | \
  python3 -c "import sys, json; d=json.load(sys.stdin); caps=d.get('capabilities',[]); print(f'Total: {len(caps)}'); [print(f\"  - {c.get('podName','N/A')} | {c.get('podUid','')[:8]}... | {c.get('capabilityId','')} | {c.get('severity','')}\") for c in caps[:5]]" 2>/dev/null || \
  echo "❌ API error"

echo ""
echo "=== 8. API: Runtime Signals ==="
curl -s "$CORE_API/api/v1/runtime-signals?podUid=$POD_UID" 2>/dev/null | \
  python3 -c "import sys, json; d=json.load(sys.stdin); sigs=d.get('signals',[]); print(f'Total: {len(sigs)}'); [print(f\"  - {s.get('podName','N/A')} | {s.get('signalType','')} | {s.get('severity','')} | {s.get('createdAt','')[:19]}\") for s in sigs[:5]]" 2>/dev/null || \
  echo "❌ API error"

echo ""
echo "=========================================="
echo "Verification Complete"
echo "=========================================="
