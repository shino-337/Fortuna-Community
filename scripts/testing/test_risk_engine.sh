#!/bin/bash

# Test Risk Engine
# Usage: ./scripts/test_risk_engine.sh [namespace]

set -e

NAMESPACE="${1:-ksam}"

echo "🧪 Testing Risk Engine"
echo "Namespace: $NAMESPACE"
echo "================================"
echo ""

# Test 1: Trigger Risk Evaluation
echo "Test 1: Triggering Risk Evaluation..."
CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

# Get auth token (if needed)
TOKEN=$(kubectl exec -n $NAMESPACE $CORE_POD -- cat /tmp/token 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
    echo "  ⚠️  No auth token found, testing without auth..."
    RESPONSE=$(kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- --post-data='{}' --header='Content-Type: application/json' http://localhost:8080/api/v1/insights/evaluate 2>&1 || echo "failed")
else
    RESPONSE=$(kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- --post-data='{}' --header="Content-Type: application/json" --header="Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/insights/evaluate 2>&1 || echo "failed")
fi

if echo "$RESPONSE" | grep -q "success\|message"; then
    echo "  ✅ Risk evaluation triggered"
else
    echo "  ⚠️  Risk evaluation response: $RESPONSE"
fi

# Test 2: Check Insights Created
echo ""
echo "Test 2: Checking Insights..."
sleep 3

POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')

INSIGHT_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>/dev/null | tr -d ' ' || echo "0")
echo "  Total insights: $INSIGHT_COUNT"

if [ "$INSIGHT_COUNT" -gt 0 ]; then
    echo "  ✅ Insights created"
    
    # Show insight types
    echo ""
    echo "  Insight types:"
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT type, COUNT(*) as count FROM insights GROUP BY type ORDER BY count DESC;" 2>&1 | grep -v "count\|row\|type" | head -10
    
    # Show by severity
    echo ""
    echo "  By severity:"
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT severity, COUNT(*) as count FROM insights GROUP BY severity ORDER BY count DESC;" 2>&1 | grep -v "count\|row\|severity" | head -10
else
    echo "  ⚠️  No insights found"
fi

# Test 3: Test API Endpoints
echo ""
echo "Test 3: Testing API Endpoints..."

# Test GET /api/v1/insights
echo "  Testing GET /api/v1/insights..."
if [ -z "$TOKEN" ]; then
    API_RESPONSE=$(kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- http://localhost:8080/api/v1/insights 2>&1 || echo "failed")
else
    API_RESPONSE=$(kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- --header="Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/insights 2>&1 || echo "failed")
fi

if echo "$API_RESPONSE" | grep -q "insights\|total"; then
    echo "  ✅ GET /api/v1/insights working"
else
    echo "  ⚠️  GET /api/v1/insights response: $API_RESPONSE"
fi

# Test GET /api/v1/insights/summary
echo "  Testing GET /api/v1/insights/summary..."
if [ -z "$TOKEN" ]; then
    SUMMARY_RESPONSE=$(kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- http://localhost:8080/api/v1/insights/summary 2>&1 || echo "failed")
else
    SUMMARY_RESPONSE=$(kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- --header="Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/insights/summary 2>&1 || echo "failed")
fi

if echo "$SUMMARY_RESPONSE" | grep -q "total\|critical\|high"; then
    echo "  ✅ GET /api/v1/insights/summary working"
else
    echo "  ⚠️  GET /api/v1/insights/summary response: $SUMMARY_RESPONSE"
fi

# Test 4: Check Risk Engine Logs
echo ""
echo "Test 4: Checking Risk Engine Logs..."
RISK_LOGS=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=100 | grep -E "RiskEngine|RBAC.*risk|insight" | head -10 || echo "")
if [ -n "$RISK_LOGS" ]; then
    echo "  ✅ Risk Engine activity found:"
    echo "$RISK_LOGS" | sed 's/^/    /'
else
    echo "  ⚠️  No Risk Engine logs found"
fi

echo ""
echo "================================"
echo "✅ Risk Engine tests completed"


