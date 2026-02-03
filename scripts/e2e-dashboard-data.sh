#!/usr/bin/env bash
# ============================================================================
# E2E test cases to populate data for Dashboard charts and Risk Center.
# - Threat Velocity: insights (vulnerability) with detected_at in last 7 days.
# - PCE Trend: pod_capabilities with created_at in last 7 days.
# - Runtime / Escape: runtime_signals from POST /runtime-events.
# ============================================================================
# 1. Create privileged pod, wait for it in Core /pods (agent sync), wait for PCE.
# 2. POST runtime-events for that pod (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT).
# 3. Optionally deploy E2E vuln pod, trigger insights evaluate for CVE insights.
# 4. Verify GET threat-velocity and GET pod-capabilities/trends (7 points).
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CORE_URL="${CORE_URL:-}"  # empty = use exec into Core pod

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()  { echo -e "${BLUE}[E2E]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_fail()  { echo -e "${RED}[FAIL]${NC} $1"; }

CORE_POD=""
TOKEN=""
get_core_pod() {
  [ -n "$CORE_POD" ] && return 0
  CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -z "$CORE_POD" ]; then
    CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  fi
  if [ -z "$CORE_POD" ]; then
    log_fail "Core pod not found in namespace $NAMESPACE"
    return 1
  fi
  log_ok "Core pod: $CORE_POD"
  return 0
}

get_token() {
  [ -n "$TOKEN" ] && return 0
  get_core_pod || return 1
  local resp
  resp=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null || echo "{}")
  TOKEN=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
  if [ -z "$TOKEN" ]; then
    log_warn "Login failed; some API calls may return 401"
    return 1
  fi
  log_ok "JWT obtained"
  return 0
}

api_get() {
  get_token 2>/dev/null || true
  local path="$1"
  if [ -n "$TOKEN" ]; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
  fi
}

api_post() {
  get_token 2>/dev/null || true
  local path="$1"
  local body="$2"
  if [ -n "$TOKEN" ]; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" -d "$body" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST -H "Content-Type: application/json" -d "$body" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
  fi
}

echo "=========================================="
echo "E2E: Dashboard chart data (Threat Velocity, PCE Trend, Runtime)"
echo "=========================================="
echo ""

get_core_pod || exit 1
get_token || true
echo ""

# --- Step 1: Create privileged pod and wait for it in Core /pods (agent sync) ---
TEST_POD_NAME="e2e-dashboard-pod-$(date +%s)"
log_info "Step 1: Creating privileged pod $TEST_POD_NAME..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD_NAME}
  namespace: ${NAMESPACE}
spec:
  containers:
  - name: test
    image: alpine:latest
    command: ["sleep", "3600"]
    securityContext:
      privileged: true
  hostPID: true
  hostNetwork: true
EOF
kubectl -n "$NAMESPACE" wait --for=condition=Ready "pod/$TEST_POD_NAME" --timeout=60s 2>/dev/null || true
POD_UID=$(kubectl -n "$NAMESPACE" get pod "$TEST_POD_NAME" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
  log_fail "Could not get pod UID"
  exit 1
fi
log_ok "Pod UID: $POD_UID"
echo ""

# Wait for pod to appear in Core /pods (agent sync) so PCE can run
log_info "Step 2: Waiting for pod in Core /pods (agent sync, up to 90s)..."
WAIT_POD=0
while [ $WAIT_POD -lt 90 ]; do
  BODY=$(api_get "pods?limit=200" 2>/dev/null || echo "{}")
  if echo "$BODY" | grep -q "$POD_UID"; then
    log_ok "Pod found in Core after ${WAIT_POD}s"
    break
  fi
  sleep 5
  WAIT_POD=$((WAIT_POD + 5))
done
if [ $WAIT_POD -ge 90 ]; then
  log_warn "Pod not seen in Core /pods after 90s; PCE may not have run yet"
fi
echo ""

# Wait for PCE (evaluatePodCapabilities runs on sync or on full sync)
log_info "Step 3: Waiting for PCE evaluation (15s)..."
sleep 15
echo ""

# --- Step 4: POST runtime-events for this pod (runtime_signals for Risk Center) ---
log_info "Step 4: POST runtime-events (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT)..."
TS=$(date +%s)
PAYLOAD="[{\"pod_uid\":\"$POD_UID\",\"namespace\":\"$NAMESPACE\",\"syscall\":\"openat\",\"target_path\":\"/proc/1/root\",\"capability\":\"\",\"timestamp\":$TS},{\"pod_uid\":\"$POD_UID\",\"namespace\":\"$NAMESPACE\",\"syscall\":\"mount\",\"target_path\":\"/proc\",\"capability\":\"\",\"timestamp\":$TS}]"
RESP=$(api_post "runtime-events" "$PAYLOAD" 2>/dev/null || echo "{}")
if echo "$RESP" | grep -q "processed"; then
  PROC=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('processed',0))" 2>/dev/null || echo "0")
  log_ok "Runtime events processed: $PROC"
else
  log_warn "Runtime events response: $RESP"
fi
echo ""

# --- Step 5: Optional – trigger insights evaluate (CVE matcher) for Threat Velocity ---
log_info "Step 5: Triggering insights evaluate (historical) for CVE insights..."
RESP_EVAL=$(api_post "insights/evaluate/historical" "{}" 2>/dev/null || echo "{}")
if echo "$RESP_EVAL" | grep -q "message\|success\|triggered"; then
  log_ok "Insights evaluate triggered (async)"
else
  log_warn "Insights evaluate: $RESP_EVAL"
fi
echo ""

# --- Step 6: Verify dashboard APIs ---
log_info "Step 6: Verifying dashboard APIs (threat-velocity, pod-capabilities/trends)..."
TV=$(api_get "dashboard/metrics/threat-velocity?days=7" 2>/dev/null || echo "{}")
PC=$(api_get "pod-capabilities/trends?days=7" 2>/dev/null || echo "{}")
TV_COUNT=$(echo "$TV" | python3 -c "import sys,json; d=json.load(sys.stdin); t=d.get('trend',[]); print(len(t))" 2>/dev/null || echo "0")
PC_COUNT=$(echo "$PC" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('points',[]); print(len(p))" 2>/dev/null || echo "0")
TV_SUM=$(echo "$TV" | python3 -c "
import sys,json
d=json.load(sys.stdin)
t=d.get('trend',[])
s=0
for x in t:
  s+=(x.get('critical') or 0)+(x.get('high') or 0)+(x.get('medium') or 0)+(x.get('low') or 0)
print(s)
" 2>/dev/null || echo "0")
PC_SUM=$(echo "$PC" | python3 -c "
import sys,json
d=json.load(sys.stdin)
p=d.get('points',[])
s=0
for x in p:
  s+=(x.get('critical') or 0)+(x.get('high') or 0)+(x.get('medium') or 0)+(x.get('low') or 0)
print(s)
" 2>/dev/null || echo "0")
if [ "$TV_COUNT" -ge 7 ]; then
  log_ok "Threat Velocity: $TV_COUNT points, total risks in window: $TV_SUM"
else
  log_warn "Threat Velocity: $TV_COUNT points (expected 7)"
fi
if [ "$PC_COUNT" -ge 7 ]; then
  log_ok "PCE Trend: $PC_COUNT points, total capabilities in window: $PC_SUM"
else
  log_warn "PCE Trend: $PC_COUNT points (expected 7)"
fi
echo ""

# --- Step 7: Cleanup test pod ---
log_info "Step 7: Cleaning up test pod..."
kubectl -n "$NAMESPACE" delete pod "$TEST_POD_NAME" --wait=false 2>/dev/null || true
log_ok "Test pod delete requested"
echo ""

echo "=========================================="
log_ok "E2E dashboard data run finished."
echo "=========================================="
echo "Dashboard charts use: threat-velocity (insights), pod-capabilities/trends (PCE), runtime-signals (Risk Center)."
echo "Refresh dashboard (admin/admin123) to see data."
echo ""
