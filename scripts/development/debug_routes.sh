#!/bin/bash

# Debug script for route 404 issues

set -e

echo "=== Route Debugging Script ==="
echo ""

# 1. Check pod status
echo "1. Pod Status:"
kubectl get pods -n ksam -l app=ksam-core
echo ""

# 2. Check route registration logs
echo "2. Route Registration Logs:"
kubectl logs -n ksam -l app=ksam-core --tail=500 | grep -E "\[API\].*Risk|Registering Risk|risk.GetRiskScores" | head -20
echo ""

# 3. Test routes
echo "3. Testing Routes:"
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n ksam "$CORE_POD" 8080:8080 > /dev/null 2>&1 &
PF_PID=$!
sleep 5

TOKEN=$(curl -s -X POST "http://localhost:8080/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' | grep -o '"token":"[^"]*' | cut -d'"' -f4)

echo "Testing /api/v1/risk/scores:"
curl -s "http://localhost:8080/api/v1/risk/scores?pageSize=1" \
    -H "Authorization: Bearer $TOKEN" \
    -w "\nHTTP Status: %{http_code}\n" | head -5

echo ""
echo "Testing /api/v1/insights (should work):"
curl -s "http://localhost:8080/api/v1/insights?pageSize=1" \
    -H "Authorization: Bearer $TOKEN" \
    -w "\nHTTP Status: %{http_code}\n" | head -3

kill $PF_PID 2>/dev/null || true
echo ""

# 4. Check code structure
echo "4. Code Structure:"
echo "Routes file exists:"
ls -la KSAM/core/internal/api/routes.go
echo ""
echo "Risk handlers file exists:"
ls -la KSAM/core/internal/api/risk/risk_handlers.go
echo ""

# 5. Check imports
echo "5. Import Check:"
grep "api/risk" KSAM/core/internal/api/routes.go
echo ""

echo "=== Debug Complete ==="

