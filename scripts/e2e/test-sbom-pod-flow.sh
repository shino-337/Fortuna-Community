#!/usr/bin/env bash
# ============================================================================
# Test SBOM flow: create a new pod, wait for agent to send SBOM, verify API
# returns the pod in /sbom list and /sbom/:podId detail.
# ============================================================================
# Prerequisites: Core running, Agent running on node where pod is scheduled,
# CORE_API_URL or port-forward to Core (e.g. 8080).
# If Core has auth enabled: set API_USER and API_PASS (default admin/admin).
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CORE_URL="${CORE_API_URL:-http://localhost:8080}"
TEST_NS="${TEST_NS:-fortuna}"
API_USER="${API_USER:-admin}"
API_PASS="${API_PASS:-admin123}"
POD_NAME="sbom-test-pod-$(date +%s)"
AUTH_HEADER=""

# Obtain JWT token if Core requires auth (401 on first request)
get_token() {
  local resp
  resp=$(curl -s -w "\n%{http_code}" -X POST "${CORE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${API_USER}\",\"password\":\"${API_PASS}\"}" 2>/dev/null) || true
  local code
  code=$(echo "$resp" | tail -n1)
  resp=$(echo "$resp" | sed '$d')
  if [ -z "$code" ] || [ "$code" != "200" ]; then
    if [ -z "$resp" ]; then
      echo "Login failed: Core unreachable at ${CORE_URL}/api/v1/auth/login" >&2
    else
      echo "Login failed (HTTP ${code:-none}): $(echo "$resp" | head -c 200)" >&2
    fi
    return 1
  fi
  if command -v jq >/dev/null 2>&1; then
    echo "$resp" | jq -r '.token // empty'
  else
    # Sed: allow "token":"v" or "token": "v"
    echo "$resp" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
  fi
}

echo "=========================================="
echo "SBOM Pod Flow Test"
echo "=========================================="
echo "Namespace: $TEST_NS"
echo "Test pod:  $POD_NAME"
echo "Core URL:  $CORE_URL/api/v1"
echo ""

# 1. Create a simple test pod (single container, small image)
echo "[1/4] Creating test pod $POD_NAME..."
kubectl run "$POD_NAME" \
  --image=busybox:1.36 \
  --restart=Never \
  -n "$TEST_NS" \
  -- sleep 3600
echo ""

# 2. Wait for pod to be Running and get UID
echo "[2/4] Waiting for pod to be Running..."
kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$TEST_NS" --timeout=120s || true
POD_UID=$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
  echo "Could not get pod UID. Pod may not be ready."
  kubectl get pod "$POD_NAME" -n "$TEST_NS"
  exit 1
fi
echo "  Pod UID: $POD_UID"
echo ""

# 2b. Get auth token if Core requires it (avoids 401 on /sbom)
TOKEN=$(get_token) || true
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer $TOKEN"
  echo "  API auth: using token (user: $API_USER)"
else
  AUTH_HEADER=""
  echo "  API auth: no token — list/detail may return 401. Set API_USER/API_PASS if Core has auth enabled."
fi
CURL_AUTH=()
[ -n "$AUTH_HEADER" ] && CURL_AUTH=(-H "$AUTH_HEADER")
echo ""

# 3. Agent will queue the pod for SBOM extraction (async). Wait for SBOM to appear in API.
echo "[3/4] Waiting for SBOM to appear in API (agent may take 1–3 min to extract)..."
MAX_WAIT=300
INTERVAL=15
elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
  body=$(curl -s "${CURL_AUTH[@]}" "${CORE_URL}/api/v1/sbom?limit=100" 2>/dev/null) || true
  if echo "$body" | grep -q "$POD_UID"; then
    echo "  SBOM found for pod $POD_UID after ${elapsed}s"
    break
  fi
  if echo "$body" | grep -q "$POD_NAME"; then
    echo "  SBOM found for pod name $POD_NAME after ${elapsed}s"
    break
  fi
  if echo "$body" | grep -q '"error"'; then
    echo "  API error: $(echo "$body" | head -c 120)"
  fi
  sleep $INTERVAL
  elapsed=$((elapsed + INTERVAL))
  echo "  ... ${elapsed}s (no SBOM yet)"
done
if [ $elapsed -ge $MAX_WAIT ]; then
  echo "  WARNING: SBOM did not appear within ${MAX_WAIT}s. Check:"
  echo "    - Agent logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent --tail=80"
  echo "    - Pod node:   kubectl get pod $POD_NAME -n $TEST_NS -o wide"
  echo "  Pod must run on a node where the agent runs; SBOM queue may be busy or extraction slow."
fi
echo ""

# 4. Call /sbom and /sbom/:podId
echo "[4/4] Verifying API..."
echo "  GET /api/v1/sbom (list):"
curl -s "${CURL_AUTH[@]}" "${CORE_URL}/api/v1/sbom?limit=20" | head -c 500
echo ""
echo ""
echo "  GET /api/v1/sbom/$POD_UID (detail):"
HTTP=$(curl -s -o /tmp/sbom_detail.json -w "%{http_code}" "${CURL_AUTH[@]}" "${CORE_URL}/api/v1/sbom/${POD_UID}")
if [ "$HTTP" = "200" ]; then
  echo "  HTTP 200 OK"
  head -c 400 /tmp/sbom_detail.json
  echo ""
else
  echo "  HTTP $HTTP"
  [ -s /tmp/sbom_detail.json ] && head -c 200 /tmp/sbom_detail.json
  echo ""
  if [ "$HTTP" = "401" ]; then
    echo "  Tip: Set API_USER and API_PASS (e.g. export API_USER=admin API_PASS=yourpassword)"
  fi
fi
echo ""

# Cleanup: delete test pod only if --cleanup passed
if [[ "${1:-}" == "--cleanup" ]]; then
  kubectl delete pod "$POD_NAME" -n "$TEST_NS" --ignore-not-found
  echo "Deleted $POD_NAME"
fi

echo "=========================================="
echo "Done. Check dashboard SBOM page for pod list and detail."
echo "=========================================="
