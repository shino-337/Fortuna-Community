#!/usr/bin/env bash
# Verify Dashboard API connectivity: login + call key endpoints used by Dashboard.
# Run with port-forward to Core (8080) or set CORE_URL.
# Usage: ./scripts/verify-dashboard-api.sh

set -euo pipefail

CORE_URL="${CORE_URL:-http://localhost:8080}"
API="${CORE_URL}/api/v1"

echo "=========================================="
echo "Dashboard API verification"
echo "=========================================="
echo "API base: $API"
echo ""

# Login
echo "[1] Login..."
LOGIN=$(curl -s -X POST "$API/auth/login" -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null) || true
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token','') or '')" 2>/dev/null) || true

if [ -z "$TOKEN" ]; then
  echo "  FAIL: Could not get token. Check Core is reachable and admin user exists."
  echo "  Response: $(echo "$LOGIN" | head -c 200)"
  exit 1
fi
echo "  OK: Token obtained"

AUTH="Authorization: Bearer $TOKEN"

# Dashboard stats
echo "[2] GET /dashboard/stats..."
STATS=$(curl -s -H "$AUTH" "$API/dashboard/stats" 2>/dev/null) || true
if echo "$STATS" | grep -q totalClusters; then
  echo "  OK: $(echo "$STATS" | python3 -c "import sys,json; d=json.load(sys.stdin); print('clusters=%s pods=%s risks=%s agents=%s' % (d.get('totalClusters'), d.get('runningPods'), d.get('totalRisks'), d.get('activeAgents')))" 2>/dev/null)"
else
  echo "  FAIL: $(echo "$STATS" | head -c 150)"
fi

# Clusters
echo "[3] GET /clusters..."
CL=$(curl -s -H "$AUTH" "$API/clusters" 2>/dev/null) || true
if echo "$CL" | grep -q '"clusters"'; then
  N=$(echo "$CL" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('clusters',[])))" 2>/dev/null)
  echo "  OK: $N clusters"
else
  echo "  FAIL: $(echo "$CL" | head -c 120)"
fi

# Risks
echo "[4] GET /risks..."
RISKS=$(curl -s -H "$AUTH" "$API/risks" 2>/dev/null) || true
if echo "$RISKS" | grep -q '"insights"'; then
  N=$(echo "$RISKS" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('insights',[])))" 2>/dev/null)
  echo "  OK: $N risks"
else
  echo "  FAIL: $(echo "$RISKS" | head -c 120)"
fi

echo ""
echo "If all OK above, Dashboard should show data when:"
echo "  1. You are logged in (admin / admin123)"
echo "  2. You open Dashboard via port-forward: kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80"
echo "  3. Browser uses http://localhost:8081 (so /api/ is proxied to Core by dashboard nginx)"
echo "=========================================="
