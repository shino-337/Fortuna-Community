#!/bin/bash
# Comprehensive Dashboard Test

set -e

DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:3000}"
API_URL="${API_URL:-http://localhost:8080}"

echo "=========================================="
echo "Comprehensive Dashboard Test"
echo "=========================================="
echo ""

# Test 1: Get API Token
echo "[TEST 1] Getting API Token..."
TOKEN=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null | \
    python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -z "$TOKEN" ] || [ "$TOKEN" == "None" ] || [ "$TOKEN" == "" ]; then
    echo "⚠️  Auth disabled or failed, testing without token"
    AUTH_HEADER=""
else
    echo "✅ Token obtained"
    AUTH_HEADER="Authorization: Bearer $TOKEN"
fi
echo ""

# Test 2: Get Data from API
echo "[TEST 2] Fetching Data from API..."

echo "  2.1. Insights Summary:"
INSIGHTS_SUMMARY=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/insights/summary" 2>/dev/null)
echo "$INSIGHTS_SUMMARY" | python3 -m json.tool 2>/dev/null || echo "$INSIGHTS_SUMMARY"
echo ""

echo "  2.2. Clusters:"
CLUSTERS=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/clusters" 2>/dev/null)
echo "$CLUSTERS" | python3 -m json.tool 2>/dev/null | head -30 || echo "$CLUSTERS"
echo ""

echo "  2.3. Insights (first 5):"
INSIGHTS=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/insights?pageSize=5" 2>/dev/null)
echo "$INSIGHTS" | python3 -m json.tool 2>/dev/null | head -100 || echo "$INSIGHTS"
echo ""

# Test 3: Compare with Database
echo "[TEST 3] Comparing API Data with Database..."
POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}' 2>/dev/null || echo "")

if [ -n "$POSTGRES_POD" ]; then
    echo "  3.1. Database Insights Count:"
    DB_TOTAL=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>/dev/null | tr -d ' ' || echo "0")
    DB_CRITICAL=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE severity='critical';" 2>/dev/null | tr -d ' ' || echo "0")
    DB_HIGH=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE severity='high';" 2>/dev/null | tr -d ' ' || echo "0")
    DB_LOW=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE severity='low';" 2>/dev/null | tr -d ' ' || echo "0")
    
    echo "    Total: $DB_TOTAL"
    echo "    Critical: $DB_CRITICAL"
    echo "    High: $DB_HIGH"
    echo "    Low: $DB_LOW"
    echo ""
    
    echo "  3.2. API Insights Summary:"
    API_TOTAL=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 0))" 2>/dev/null || echo "0")
    API_CRITICAL=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('critical', 0))" 2>/dev/null || echo "0")
    API_HIGH=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('high', 0))" 2>/dev/null || echo "0")
    API_LOW=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('low', 0))" 2>/dev/null || echo "0")
    
    echo "    Total: $API_TOTAL"
    echo "    Critical: $API_CRITICAL"
    echo "    High: $API_HIGH"
    echo "    Low: $API_LOW"
    echo ""
    
    echo "  3.3. Comparison:"
    if [ "$DB_TOTAL" = "$API_TOTAL" ] && [ "$DB_TOTAL" != "0" ]; then
        echo "    ✅ Total matches: $DB_TOTAL"
    else
        echo "    ⚠️  Total mismatch: DB=$DB_TOTAL, API=$API_TOTAL"
    fi
    
    if [ "$DB_CRITICAL" = "$API_CRITICAL" ]; then
        echo "    ✅ Critical matches: $DB_CRITICAL"
    else
        echo "    ⚠️  Critical mismatch: DB=$DB_CRITICAL, API=$API_CRITICAL"
    fi
else
    echo "  ⚠️  Cannot access database"
fi
echo ""

# Test 4: Dashboard Accessibility
echo "[TEST 4] Testing Dashboard Accessibility..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$DASHBOARD_URL" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ Dashboard accessible (HTTP $HTTP_CODE)"
    echo ""
    echo "  4.1. Dashboard Content:"
    DASHBOARD_HTML=$(curl -s "$DASHBOARD_URL" 2>/dev/null)
    if echo "$DASHBOARD_HTML" | grep -q "root\|React\|KSAM" 2>/dev/null; then
        echo "    ✅ React app detected"
    fi
    if echo "$DASHBOARD_HTML" | grep -q "\.js\|assets" 2>/dev/null; then
        echo "    ✅ JavaScript bundle found"
    fi
else
    echo "❌ Dashboard not accessible (HTTP $HTTP_CODE)"
fi
echo ""

# Test 5: Sample Insights for Display Check
echo "[TEST 5] Sample Insights for Display Verification..."
if [ -n "$TOKEN" ]; then
    SAMPLE_INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_URL/api/v1/insights?severity=critical&pageSize=3" 2>/dev/null)
    echo "$SAMPLE_INSIGHTS" | python3 -c "
import sys, json
data = json.load(sys.stdin)
insights = data.get('insights', data if isinstance(data, list) else [])
print(f'Found {len(insights)} critical insights:')
for i, insight in enumerate(insights[:3], 1):
    print(f'  {i}. [{insight.get(\"id\")}] {insight.get(\"severity\", \"N/A\").upper()}')
    print(f'     {insight.get(\"description\", \"N/A\")[:80]}...')
" 2>/dev/null || echo "Could not parse insights"
else
    echo "⚠️  Skipping (no auth token)"
fi
echo ""

echo "=========================================="
echo "Test Complete"
echo "=========================================="
echo ""
echo "📊 Summary:"
echo "  - Dashboard URL: $DASHBOARD_URL"
echo "  - API URL: $API_URL"
echo "  - Authentication: $([ -n "$TOKEN" ] && echo "Working" || echo "Disabled/Failed")"
echo ""
echo "🔍 Next Steps:"
echo "  1. Open $DASHBOARD_URL in browser"
echo "  2. Test login (if auth enabled)"
echo "  3. Verify data display matches API"
echo "  4. Test all screens navigation"
echo ""

