#!/bin/bash
# Test Login CORS

set -e

API_URL="${API_URL:-http://localhost:8080}"

echo "=========================================="
echo "Login CORS Test"
echo "=========================================="
echo ""
echo "API URL: $API_URL"
echo ""

# Test 1: Preflight Request
echo "[TEST 1] Preflight Request (OPTIONS)"
echo "-----------------------------------"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X OPTIONS "$API_URL/api/v1/auth/login" \
    -H "Origin: http://localhost:3000" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: Content-Type" 2>/dev/null || echo "000")

if [ "$HTTP_CODE" = "204" ] || [ "$HTTP_CODE" = "200" ]; then
    echo "✅ Preflight successful (HTTP $HTTP_CODE)"
else
    echo "❌ Preflight failed (HTTP $HTTP_CODE)"
fi

# Check CORS headers
CORS_HEADERS=$(curl -s -i -X OPTIONS "$API_URL/api/v1/auth/login" \
    -H "Origin: http://localhost:3000" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: Content-Type" 2>/dev/null | grep -i "access-control" || echo "")

if [ -n "$CORS_HEADERS" ]; then
    echo "✅ CORS headers present:"
    echo "$CORS_HEADERS" | sed 's/^/  /'
else
    echo "❌ No CORS headers found"
fi
echo ""

# Test 2: Actual Login Request
echo "[TEST 2] Login Request (POST)"
echo "-----------------------------------"
LOGIN_RESPONSE=$(curl -s -i -X POST "$API_URL/api/v1/auth/login" \
    -H "Origin: http://localhost:3000" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null)

HTTP_CODE=$(echo "$LOGIN_RESPONSE" | head -1 | grep -oE "HTTP/[0-9.]+ [0-9]+" | awk '{print $2}')

if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ Login successful (HTTP $HTTP_CODE)"
    
    # Check CORS headers in response
    CORS_HEADERS=$(echo "$LOGIN_RESPONSE" | grep -i "access-control" || echo "")
    if [ -n "$CORS_HEADERS" ]; then
        echo "✅ CORS headers in response:"
        echo "$CORS_HEADERS" | sed 's/^/  /'
    else
        echo "⚠️  No CORS headers in response"
    fi
    
    # Check token
    TOKEN=$(echo "$LOGIN_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin.split('\\r\\n\\r\\n')[1] if '\\r\\n\\r\\n' in sys.stdin.read() else sys.stdin.read()); print(data.get('token', ''))" 2>/dev/null || echo "")
    if [ -n "$TOKEN" ] && [ "$TOKEN" != "None" ]; then
        echo "✅ Token received: ${TOKEN:0:50}..."
    else
        echo "⚠️  Could not extract token"
    fi
else
    echo "❌ Login failed (HTTP $HTTP_CODE)"
    echo "Response:"
    echo "$LOGIN_RESPONSE" | tail -5
fi
echo ""

# Test 3: Browser Simulation
echo "[TEST 3] Browser-like Request"
echo "-----------------------------------"
echo "Simulating browser request with all headers..."
BROWSER_RESPONSE=$(curl -s -i -X POST "$API_URL/api/v1/auth/login" \
    -H "Origin: http://localhost:3000" \
    -H "Referer: http://localhost:3000/login" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -H "User-Agent: Mozilla/5.0" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null)

HTTP_CODE=$(echo "$BROWSER_RESPONSE" | head -1 | grep -oE "HTTP/[0-9.]+ [0-9]+" | awk '{print $2}')
CORS_ORIGIN=$(echo "$BROWSER_RESPONSE" | grep -i "access-control-allow-origin" | head -1)

if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ Request successful (HTTP $HTTP_CODE)"
    if [ -n "$CORS_ORIGIN" ]; then
        echo "✅ CORS origin header: $CORS_ORIGIN"
    else
        echo "❌ Missing CORS origin header"
    fi
else
    echo "❌ Request failed (HTTP $HTTP_CODE)"
fi
echo ""

# Summary
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
if [ "$HTTP_CODE" = "200" ] && [ -n "$CORS_ORIGIN" ]; then
    echo "✅ CORS is working correctly"
    echo ""
    echo "🌐 Dashboard can now:"
    echo "  - Make login requests from http://localhost:3000"
    echo "  - Receive CORS headers"
    echo "  - Get authentication token"
else
    echo "❌ CORS issues detected"
    echo ""
    echo "🔧 Check:"
    echo "  - Core service is running"
    echo "  - CORS middleware is applied"
    echo "  - Port-forward is active"
fi
echo ""

