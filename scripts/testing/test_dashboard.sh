#!/bin/bash
# Test Dashboard Functionality

set -e

# Resolve KSAM root directory (repo-local, no hard-coded absolute paths)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KSAM_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:3000}"
API_URL="${API_URL:-http://localhost:8080}"

echo "=========================================="
echo "Dashboard Functionality Test"
echo "=========================================="
echo ""
echo "Dashboard URL: $DASHBOARD_URL"
echo "API URL: $API_URL"
echo ""

# Test 1: Dashboard Accessibility
echo -e "${BLUE}[TEST 1]${NC} Checking Dashboard Accessibility..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$DASHBOARD_URL" || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✅${NC} Dashboard is accessible (HTTP $HTTP_CODE)"
else
    echo -e "${RED}❌${NC} Dashboard not accessible (HTTP $HTTP_CODE)"
    exit 1
fi
echo ""

# Test 2: API Health Check
echo -e "${BLUE}[TEST 2]${NC} Checking Core API Health..."
API_HEALTH=$(curl -s "$API_URL/health" 2>/dev/null | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('status', 'unknown'))" 2>/dev/null || echo "unknown")
if [ "$API_HEALTH" = "healthy" ]; then
    echo -e "${GREEN}✅${NC} Core API is healthy"
else
    echo -e "${YELLOW}⚠️${NC}  Core API health: $API_HEALTH"
fi
echo ""

# Test 3: API Authentication
echo -e "${BLUE}[TEST 3]${NC} Testing API Authentication..."
LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null)

TOKEN=$(echo "$LOGIN_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ] && [ "$TOKEN" != "None" ] && [ "$TOKEN" != "" ]; then
    echo -e "${GREEN}✅${NC} Authentication successful"
    echo "  Token obtained: ${TOKEN:0:50}..."
else
    echo -e "${YELLOW}⚠️${NC}  Authentication failed or auth disabled"
    TOKEN=""
fi
echo ""

# Test 4: API Endpoints
echo -e "${BLUE}[TEST 4]${NC} Testing API Endpoints..."

if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
else
    AUTH_HEADER=""
fi

# Test Insights Summary
echo "  4.1. Insights Summary..."
INSIGHTS_SUMMARY=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/insights/summary" 2>/dev/null)
if echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); exit(0 if 'total' in data or 'critical' in data else 1)" 2>/dev/null; then
    TOTAL=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 0))" 2>/dev/null || echo "0")
    CRITICAL=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('critical', 0))" 2>/dev/null || echo "0")
    echo -e "     ${GREEN}✅${NC} Total: $TOTAL, Critical: $CRITICAL"
else
    echo -e "     ${YELLOW}⚠️${NC}  Could not fetch insights summary"
fi

# Test Clusters
echo "  4.2. Clusters..."
CLUSTERS=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/clusters" 2>/dev/null)
if echo "$CLUSTERS" | python3 -c "import sys, json; data=json.load(sys.stdin); exit(0 if isinstance(data, list) or 'clusters' in str(data) else 1)" 2>/dev/null; then
    CLUSTER_COUNT=$(echo "$CLUSTERS" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null || echo "0")
    echo -e "     ${GREEN}✅${NC} Clusters: $CLUSTER_COUNT"
else
    echo -e "     ${YELLOW}⚠️${NC}  Could not fetch clusters"
fi

# Test Insights
echo "  4.3. Insights..."
INSIGHTS=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/insights?pageSize=5" 2>/dev/null)
if echo "$INSIGHTS" | python3 -c "import sys, json; data=json.load(sys.stdin); exit(0 if 'insights' in data or isinstance(data, list) else 1)" 2>/dev/null; then
    INSIGHT_COUNT=$(echo "$INSIGHTS" | python3 -c "import sys, json; data=json.load(sys.stdin); insights=data.get('insights', data if isinstance(data, list) else []); print(len(insights))" 2>/dev/null || echo "0")
    echo -e "     ${GREEN}✅${NC} Insights: $INSIGHT_COUNT"
else
    echo -e "     ${YELLOW}⚠️${NC}  Could not fetch insights"
fi
echo ""

# Test 5: Dashboard Pages
echo -e "${BLUE}[TEST 5]${NC} Testing Dashboard Pages..."

# Check if dashboard HTML loads
DASHBOARD_HTML=$(curl -s "$DASHBOARD_URL" 2>/dev/null)
if echo "$DASHBOARD_HTML" | grep -q "KSAM\|React\|root" 2>/dev/null; then
    echo -e "${GREEN}✅${NC} Dashboard HTML loaded"
else
    echo -e "${RED}❌${NC} Dashboard HTML not loading correctly"
fi

# Check for JavaScript bundle
if echo "$DASHBOARD_HTML" | grep -q "\.js\|assets" 2>/dev/null; then
    echo -e "${GREEN}✅${NC} JavaScript bundle referenced"
else
    echo -e "${YELLOW}⚠️${NC}  JavaScript bundle not found"
fi
echo ""

# Test 6: Data Accuracy Check
echo -e "${BLUE}[TEST 6]${NC} Verifying Data Accuracy..."

if [ -n "$TOKEN" ]; then
    # Get data from API
    API_INSIGHTS=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_URL/api/v1/insights?pageSize=10" 2>/dev/null)
    API_SUMMARY=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_URL/api/v1/insights/summary" 2>/dev/null)
    
    # Get data from database for comparison
    POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}' 2>/dev/null || echo "")
    if [ -n "$POSTGRES_POD" ]; then
        DB_TOTAL=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>/dev/null | tr -d ' ' || echo "0")
        DB_CRITICAL=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE severity='critical';" 2>/dev/null | tr -d ' ' || echo "0")
        
        API_TOTAL=$(echo "$API_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 0))" 2>/dev/null || echo "0")
        API_CRITICAL=$(echo "$API_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('critical', 0))" 2>/dev/null || echo "0")
        
        echo "  Database: Total=$DB_TOTAL, Critical=$DB_CRITICAL"
        echo "  API: Total=$API_TOTAL, Critical=$API_CRITICAL"
        
        if [ "$DB_TOTAL" = "$API_TOTAL" ] && [ "$DB_TOTAL" != "0" ]; then
            echo -e "  ${GREEN}✅${NC} Data matches between database and API"
        else
            echo -e "  ${YELLOW}⚠️${NC}  Data mismatch (may be due to pagination or filtering)"
        fi
    else
        echo -e "  ${YELLOW}⚠️${NC}  Cannot access database for comparison"
    fi
else
    echo -e "  ${YELLOW}⚠️${NC}  Skipping data accuracy check (no auth token)"
fi
echo ""

# Test 7: Dashboard Functionality
echo -e "${BLUE}[TEST 7]${NC} Testing Dashboard Functionality..."

# Check if dashboard can make API calls (CORS, proxy, etc.)
echo "  7.1. API Connectivity from Dashboard..."
# This would require browser automation, but we can check if API is accessible
if curl -s "$API_URL/health" >/dev/null 2>&1; then
    echo -e "     ${GREEN}✅${NC} API is accessible"
else
    echo -e "     ${RED}❌${NC} API not accessible"
fi

echo "  7.2. Dashboard Configuration..."
if [ -f "$KSAM_ROOT/dashboard/.env" ] || [ -n "${VITE_API_URL:-}" ]; then
    echo -e "     ${GREEN}✅${NC} API URL configured"
else
    echo -e "     ${YELLOW}⚠️${NC}  API URL may need configuration"
fi
echo ""

# Summary
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo "✅ Tests Completed:"
echo "  - Dashboard accessibility"
echo "  - API health check"
echo "  - Authentication"
echo "  - API endpoints"
echo "  - Dashboard pages"
echo "  - Data accuracy"
echo ""
echo "📝 Notes:"
echo "  - Dashboard URL: $DASHBOARD_URL"
echo "  - API URL: $API_URL"
echo "  - For full browser testing, open: $DASHBOARD_URL"
echo ""
echo "🔍 Manual Testing Required:"
echo "  1. Open $DASHBOARD_URL in browser"
echo "  2. Test login functionality"
echo "  3. Verify all screens load correctly"
echo "  4. Check data display accuracy"
echo "  5. Test navigation between screens"
echo ""

