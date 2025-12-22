#!/bin/bash
# Complete CORS Test

set -e

API_URL="${API_URL:-http://localhost:8080}"

echo "=========================================="
echo "Complete CORS Test"
echo "=========================================="
echo ""
echo "API URL: $API_URL"
echo ""

# Test 1: Preflight Request
echo "[TEST 1] Preflight Request (OPTIONS)"
echo "-----------------------------------"
PREFLIGHT_RESPONSE=$(curl -s -i -X OPTIONS "$API_URL/api/v1/auth/login" \
    -H "Origin: http://localhost:3000" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: Content-Type" 2>/dev/null)

HTTP_CODE=$(echo "$PREFLIGHT_RESPONSE" | head -1 | grep -oE "HTTP/[0-9.]+ [0-9]+" | awk '{print $2}')

if [ "$HTTP_CODE" = "204" ] || [ "$HTTP_CODE" = "200" ]; then
    echo "✅ Preflight successful (HTTP $HTTP_CODE)"
else
    echo "❌ Preflight failed (HTTP $HTTP_CODE)"
fi

# Check CORS headers
echo ""
echo "CORS Headers in Preflight Response:"
echo "$PREFLIGHT_RESPONSE" | grep -i "access-control" | sed 's/^/  /'

# Check if Access-Control-Allow-Origin is set
if echo "$PREFLIGHT_RESPONSE" | grep -qi "access-control-allow-origin"; then
    ORIGIN_HEADER=$(echo "$PREFLIGHT_RESPONSE" | grep -i "access-control-allow-origin" | head -1)
    echo "✅ Access-Control-Allow-Origin: $ORIGIN_HEADER"
else
    echo "❌ Missing Access-Control-Allow-Origin header"
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
else
    echo "❌ Login failed (HTTP $HTTP_CODE)"
    echo "Response:"
    echo "$LOGIN_RESPONSE" | tail -5
fi

# Check CORS headers in response
echo ""
echo "CORS Headers in Login Response:"
CORS_HEADERS=$(echo "$LOGIN_RESPONSE" | grep -i "access-control" || echo "")
if [ -n "$CORS_HEADERS" ]; then
    echo "$CORS_HEADERS" | sed 's/^/  /'
    echo "✅ CORS headers present"
else
    echo "❌ No CORS headers found"
fi

# Check if Access-Control-Allow-Origin is set
if echo "$LOGIN_RESPONSE" | grep -qi "access-control-allow-origin"; then
    ORIGIN_HEADER=$(echo "$LOGIN_RESPONSE" | grep -i "access-control-allow-origin" | head -1)
    echo "✅ Access-Control-Allow-Origin: $ORIGIN_HEADER"
else
    echo "❌ Missing Access-Control-Allow-Origin header"
fi
echo ""

# Test 3: Browser Simulation with all headers
echo "[TEST 3] Browser-like Request (Full Headers)"
echo "-----------------------------------"
BROWSER_RESPONSE=$(curl -s -i -X POST "$API_URL/api/v1/auth/login" \
    -H "Origin: http://localhost:3000" \
    -H "Referer: http://localhost:3000/login" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36" \
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
    echo ""
    echo "📝 If you still see CORS errors in browser:"
    echo "  1. Clear browser cache"
    echo "  2. Hard refresh (Cmd+Shift+R or Ctrl+Shift+R)"
    echo "  3. Check browser console for exact error"
    echo "  4. Verify port-forward is active"
else
    echo "❌ CORS issues detected"
    echo ""
    echo "🔧 Check:"
    echo "  - Core service is running"
    echo "  - CORS middleware is applied"
    echo "  - Port-forward is active"
    echo "  - No firewall blocking"
fi
echo ""

