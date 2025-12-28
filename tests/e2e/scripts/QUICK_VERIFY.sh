#!/bin/bash

# Quick Verification Script
# Combines database and API queries for quick verification

set -e

NAMESPACE="${NAMESPACE:-fortuna}"
TEST_POD_NAME="${TEST_POD_NAME:-test-pod-e2e-1766897554}"
POD_UID="${POD_UID:-9caa6290-5471-46de-9a0b-a43ba7937d9d}"

echo "=========================================="
echo "Quick E2E Verification"
echo "=========================================="
echo ""
echo "Test Pod: $TEST_POD_NAME"
echo "Pod UID: $POD_UID"
echo ""

POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POSTGRES_POD" ] || [ -z "$CORE_POD" ]; then
    echo "❌ Required pods not found"
    exit 1
fi

echo "1. SBOM Check:"
SBOM_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME';" 2>/dev/null | tr -d ' ')
echo "   SBOMs found: $SBOM_COUNT"

if [ "$SBOM_COUNT" -gt 0 ]; then
    SBOM_ID=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d ' ')
    echo "   SBOM ID: $SBOM_ID"
    
    COMPONENT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;" 2>/dev/null | tr -d ' ')
    echo "   Components: $COMPONENT_COUNT"
    
    CVE_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;" 2>/dev/null | tr -d ' ')
    echo "   CVE Matches: $CVE_COUNT"
fi

echo ""
echo "2. Insights Check:"
INSIGHT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT COUNT(*) FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;" 2>/dev/null | tr -d ' ')
echo "   Insights found: $INSIGHT_COUNT"

echo ""
echo "3. API Check:"
kubectl port-forward -n $NAMESPACE $CORE_POD 8080:8080 > /tmp/port-forward-quick.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 2

API_TOTAL=$(curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" 2>/dev/null | grep -o '"total":[0-9]*' | cut -d: -f2 || echo "0")
echo "   API total insights: $API_TOTAL"

kill $PORT_FORWARD_PID 2>/dev/null || true

echo ""
echo "=========================================="
echo "Verification Complete"
echo "=========================================="

