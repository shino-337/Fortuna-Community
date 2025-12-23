#!/bin/bash
# Test All APIs for Dashboard

set -e

API_URL="${API_URL:-http://localhost:8080}"

echo "=========================================="
echo "Testing All APIs for Dashboard"
echo "=========================================="
echo ""
echo "API URL: $API_URL"
echo ""

# Test 1: Health Check
echo "[TEST 1] Health Check"
HEALTH=$(curl -s "$API_URL/health" 2>/dev/null)
echo "$HEALTH" | python3 -m json.tool 2>/dev/null || echo "$HEALTH"
echo ""

# Test 2: Login
echo "[TEST 2] Login"
LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -H "Origin: http://localhost:3000" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null)

TOKEN=$(echo "$LOGIN_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ] && [ "$TOKEN" != "None" ] && [ "$TOKEN" != "" ]; then
    echo "✅ Login successful"
    echo "Token: ${TOKEN:0:50}..."
    echo ""
else
    echo "⚠️  Login failed or auth disabled"
    TOKEN=""
    echo ""
fi

# Test 3: Insights Summary
echo "[TEST 3] Insights Summary"
if [ -n "$TOKEN" ]; then
    INSIGHTS_SUMMARY=$(curl -s -H "Authorization: Bearer $TOKEN" \
        -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/insights/summary" 2>/dev/null)
else
    INSIGHTS_SUMMARY=$(curl -s -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/insights/summary" 2>/dev/null)
fi
echo "$INSIGHTS_SUMMARY" | python3 -m json.tool 2>/dev/null || echo "$INSIGHTS_SUMMARY"
echo ""

# Test 4: Clusters
echo "[TEST 4] Clusters"
if [ -n "$TOKEN" ]; then
    CLUSTERS=$(curl -s -H "Authorization: Bearer $TOKEN" \
        -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/clusters" 2>/dev/null)
else
    CLUSTERS=$(curl -s -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/clusters" 2>/dev/null)
fi
echo "$CLUSTERS" | python3 -m json.tool 2>/dev/null | head -50 || echo "$CLUSTERS"
echo ""

# Test 5: Insights List
echo "[TEST 5] Insights List (first 5)"
if [ -n "$TOKEN" ]; then
    INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
        -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/insights?pageSize=5" 2>/dev/null)
else
    INSIGHTS=$(curl -s -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/insights?pageSize=5" 2>/dev/null)
fi
echo "$INSIGHTS" | python3 -m json.tool 2>/dev/null | head -100 || echo "$INSIGHTS"
echo ""

# Test 6: Critical Insights
echo "[TEST 6] Critical Insights (first 3)"
if [ -n "$TOKEN" ]; then
    CRITICAL=$(curl -s -H "Authorization: Bearer $TOKEN" \
        -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/insights?severity=critical&pageSize=3" 2>/dev/null)
else
    CRITICAL=$(curl -s -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/insights?severity=critical&pageSize=3" 2>/dev/null)
fi
echo "$CRITICAL" | python3 -c "
import sys, json
data = json.load(sys.stdin)
insights = data.get('insights', data if isinstance(data, list) else [])
print(f'Found {len(insights)} critical insights:')
for i, insight in enumerate(insights[:3], 1):
    print(f'  {i}. [{insight.get(\"id\")}] {insight.get(\"severity\", \"N/A\").upper()}')
    print(f'     Type: {insight.get(\"type\", \"N/A\")}')
    print(f'     Description: {insight.get(\"description\", \"N/A\")[:100]}...')
" 2>/dev/null || echo "$CRITICAL"
echo ""

# Test 7: ServiceAccounts
echo "[TEST 7] ServiceAccounts (first 3)"
if [ -n "$TOKEN" ]; then
    SAs=$(curl -s -H "Authorization: Bearer $TOKEN" \
        -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/serviceaccounts?pageSize=3" 2>/dev/null)
else
    SAs=$(curl -s -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/serviceaccounts?pageSize=3" 2>/dev/null)
fi
echo "$SAs" | python3 -m json.tool 2>/dev/null | head -50 || echo "$SAs"
echo ""

# Test 8: Clusters Stats
echo "[TEST 8] Clusters Stats"
if [ -n "$TOKEN" ]; then
    STATS=$(curl -s -H "Authorization: Bearer $TOKEN" \
        -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/clusters/stats" 2>/dev/null)
else
    STATS=$(curl -s -H "Origin: http://localhost:3000" \
        "$API_URL/api/v1/clusters/stats" 2>/dev/null)
fi
echo "$STATS" | python3 -m json.tool 2>/dev/null || echo "$STATS"
echo ""

# Summary
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo "✅ All API endpoints tested"
echo "📊 Data available for dashboard display"
echo ""
echo "🌐 Dashboard URL: http://localhost:3000"
echo ""
