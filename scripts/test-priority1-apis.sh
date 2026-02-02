#!/bin/bash
set -e

echo "=========================================="
echo "Priority 1 API Testing (Phase 2.3)"
echo "=========================================="

CORE_POD=$(kubectl -n fortuna get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$CORE_POD" ]; then
    echo "❌ Core pod not found"
    exit 1
fi

# JWT: login and get Bearer token (env FORTUNA_E2E_USER / FORTUNA_E2E_PASSWORD, default admin/admin123). Core image has curl, not wget.
E2E_USER="${FORTUNA_E2E_USER:-admin}"
E2E_PASS="${FORTUNA_E2E_PASSWORD:-admin123}"
LOGIN_RESP=$(kubectl -n fortuna exec $CORE_POD -- curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"username\":\"$E2E_USER\",\"password\":\"$E2E_PASS\"}" 2>/dev/null || echo "{}")
TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
if [ -z "$TOKEN" ]; then
    echo "⚠️  Login failed (user=$E2E_USER). APIs may return 401. Set FORTUNA_E2E_USER / FORTUNA_E2E_PASSWORD if admin exists."
    AUTH_HEADER=""
else
    echo "✅ JWT obtained for $E2E_USER"
    AUTH_HEADER="Authorization: Bearer $TOKEN"
fi
echo "Core pod: $CORE_POD"
echo ""

# Helper: exec curl with optional Bearer header (Core image has curl)
api_get() {
    local url="$1"
    if [ -n "$AUTH_HEADER" ]; then
        kubectl -n fortuna exec $CORE_POD -- curl -s -H "$AUTH_HEADER" "$url" 2>/dev/null || echo "ERROR"
    else
        kubectl -n fortuna exec $CORE_POD -- curl -s "$url" 2>/dev/null || echo "ERROR"
    fi
}

# Test Promotion Rules API
echo "Test 1: GET /api/v1/promotion-rules"
echo "----------------------------------------"
RESPONSE=$(api_get "http://localhost:8080/api/v1/promotion-rules")
if echo "$RESPONSE" | grep -q "rules"; then
    COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' || echo "0")
    echo "✅ Status: 200"
    echo "✅ Count: $COUNT"
else
    echo "❌ Failed: $RESPONSE"
fi
echo ""

# Test Promotion Rules by Capability
echo "Test 2: GET /api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE"
echo "----------------------------------------"
RESPONSE=$(api_get "http://localhost:8080/api/v1/promotion-rules/capability/ESC_HOSTPATH_NODE")
if echo "$RESPONSE" | grep -q "rules"; then
    COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' || echo "0")
    echo "✅ Status: 200"
    echo "✅ Count: $COUNT"
else
    echo "❌ Failed: $RESPONSE"
fi
echo ""

# Test Promotion Rules by Signal Type
echo "Test 3: GET /api/v1/promotion-rules/signal/PROC_ROOT_PIVOT"
echo "----------------------------------------"
RESPONSE=$(api_get "http://localhost:8080/api/v1/promotion-rules/signal/PROC_ROOT_PIVOT")
if echo "$RESPONSE" | grep -q "rules"; then
    COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' || echo "0")
    echo "✅ Status: 200"
    echo "✅ Count: $COUNT"
else
    echo "❌ Failed: $RESPONSE"
fi
echo ""

# Test Runtime Signals API
echo "Test 4: GET /api/v1/runtime-signals"
echo "----------------------------------------"
RESPONSE=$(api_get "http://localhost:8080/api/v1/runtime-signals?limit=10")
if echo "$RESPONSE" | grep -q "signals"; then
    COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' || echo "0")
    TOTAL=$(echo "$RESPONSE" | grep -o '"total":[0-9]*' | grep -o '[0-9]*' || echo "0")
    echo "✅ Status: 200"
    echo "✅ Count: $COUNT"
    echo "✅ Total: $TOTAL"
else
    echo "❌ Failed: $RESPONSE"
fi
echo ""

# Test Runtime Signals by Pod
echo "Test 5: Finding pod with runtime signals..."
POD_UID=$(kubectl -n fortuna exec postgres-7858fc8764-kgccd -- psql -U postgres -d fortuna -t -c "SELECT pod_uid FROM runtime_signals LIMIT 1;" 2>/dev/null | tr -d ' ' || echo "")
if [ -n "$POD_UID" ]; then
    echo "Found pod UID: $POD_UID"
    echo ""
    echo "Test 5: GET /api/v1/runtime-signals/pods/$POD_UID"
    echo "----------------------------------------"
    RESPONSE=$(api_get "http://localhost:8080/api/v1/runtime-signals/pods/$POD_UID")
    if echo "$RESPONSE" | grep -q "signals"; then
        COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' || echo "0")
        echo "✅ Status: 200"
        echo "✅ Count: $COUNT"
    else
        echo "❌ Failed: $RESPONSE"
    fi
else
    echo "⚠️  No pods with runtime signals found"
fi
echo ""

echo "=========================================="
echo "API Testing Complete"
echo "=========================================="
