#!/bin/bash

set -euo pipefail

echo "=========================================="
echo "Test Promotion Flow"
echo "=========================================="

CORE_IP=$(kubectl get svc fortuna-core -n fortuna -o jsonpath='{.spec.clusterIP}')
CORE_PORT=$(kubectl get svc fortuna-core -n fortuna -o jsonpath='{.spec.ports[?(@.name=="http")].port}')
CORE_URL="http://${CORE_IP}:${CORE_PORT}/api/v1"

echo "Core URL: ${CORE_URL}"
echo ""

# Get pod with runtime signal
POD_UID="1e9cfc47-56c2-41e0-83e6-d5052267796d"
SIGNAL_TYPE="PROC_ROOT_PIVOT"

echo "=== 1. Check Pod Capabilities ==="
echo "Pod UID: ${POD_UID}"
curl -s "${CORE_URL}/pods/${POD_UID}/capabilities" | python3 -m json.tool 2>/dev/null | head -40 || curl -s "${CORE_URL}/pods/${POD_UID}/capabilities"
echo ""

echo "=== 2. Check Runtime Signals for Pod ==="
curl -s "${CORE_URL}/runtime-signals/pods/${POD_UID}" | python3 -m json.tool 2>/dev/null | head -40 || curl -s "${CORE_URL}/runtime-signals/pods/${POD_UID}"
echo ""

echo "=== 3. Check Promotion Rules for Signal ==="
curl -s "${CORE_URL}/promotion-rules/signal/${SIGNAL_TYPE}" | python3 -m json.tool 2>/dev/null | head -60 || curl -s "${CORE_URL}/promotion-rules/signal/${SIGNAL_TYPE}"
echo ""

echo "=== 4. Database Check ==="
echo "Capabilities for pod:"
kubectl exec -n fortuna postgres-7858fc8764-kgccd -- psql -U postgres -d fortuna -c "SELECT capability_id, state, confidence FROM pod_capabilities WHERE pod_uid='${POD_UID}' ORDER BY capability_id;" 2>/dev/null || echo "Cannot query database"
echo ""

echo "Runtime signals count for pod:"
kubectl exec -n fortuna postgres-7858fc8764-kgccd -- psql -U postgres -d fortuna -c "SELECT COUNT(*) as signal_count FROM runtime_signals WHERE pod_uid='${POD_UID}' AND signal_type='${SIGNAL_TYPE}';" 2>/dev/null || echo "Cannot query database"
echo ""

echo "=========================================="
echo "Test Complete"
echo "=========================================="
