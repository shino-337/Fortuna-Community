#!/bin/bash

# API Query Commands for E2E Verification
# These commands can be run directly to verify the API responses

set -e

NAMESPACE="${NAMESPACE:-fortuna}"
POD_UID="${POD_UID:-9caa6290-5471-46de-9a0b-a43ba7937d9d}"

CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

echo "=========================================="
echo "API Query Commands"
echo "=========================================="
echo ""
echo "Core Pod: $CORE_POD"
echo "Pod UID: $POD_UID"
echo ""

# Setup port-forward
echo "Setting up port-forward..."
kubectl port-forward -n $NAMESPACE $CORE_POD 8080:8080 > /tmp/port-forward-api.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

# Function to cleanup
cleanup() {
    kill $PORT_FORWARD_PID 2>/dev/null || true
    sleep 1
}
trap cleanup EXIT

echo "=========================================="
echo "1. Health Check"
echo "=========================================="
echo ""
echo "curl -s http://localhost:8080/health"
echo ""
curl -s http://localhost:8080/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8080/health
echo ""
echo ""

echo "=========================================="
echo "2. Get Insights by Resource UID"
echo "=========================================="
echo ""
echo "curl -s \"http://localhost:8080/api/v1/insights?resource_uid=$POD_UID\""
echo ""
API_RESPONSE=$(curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID" 2>&1)
echo "$API_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$API_RESPONSE"
echo ""

# Extract total
TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d: -f2 || echo "0")
echo "Total insights: $TOTAL"
echo ""

if [ "$TOTAL" -gt 0 ]; then
    echo "Sample insight details:"
    echo "$API_RESPONSE" | python3 -m json.tool 2>/dev/null | grep -A 30 '"insights"' | head -40 || echo "$API_RESPONSE" | head -100
    echo ""
fi

echo "=========================================="
echo "3. Get Insights with Filters"
echo "=========================================="
echo ""
echo "curl -s \"http://localhost:8080/api/v1/insights?resource_uid=$POD_UID&severity=high\""
echo ""
curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID&severity=high" | python3 -m json.tool 2>/dev/null || curl -s "http://localhost:8080/api/v1/insights?resource_uid=$POD_UID&severity=high"
echo ""
echo ""

echo "=========================================="
echo "4. Get All Insights (without filter)"
echo "=========================================="
echo ""
echo "curl -s \"http://localhost:8080/api/v1/insights?page=1&pageSize=10\""
echo ""
curl -s "http://localhost:8080/api/v1/insights?page=1&pageSize=10" | python3 -m json.tool 2>/dev/null | head -50 || curl -s "http://localhost:8080/api/v1/insights?page=1&pageSize=10" | head -50
echo ""
echo ""

echo "=========================================="
echo "API Verification Complete"
echo "=========================================="

