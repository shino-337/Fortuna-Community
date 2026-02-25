#!/usr/bin/env bash
# ============================================================================
# E2E SBOM verification: check that a pod (e.g. pod-test) appears in API and
# optionally run full SBOM flow test. Use after creating a pod to verify
# dashboard will show it.
# ============================================================================
# Usage:
#   ./scripts/e2e/e2e-sbom-verify.sh                    # Check pod-test in API
#   ./scripts/e2e/e2e-sbom-verify.sh my-pod default   # Check my-pod in default ns
#   ./scripts/e2e/e2e-sbom-verify.sh --full            # Run full test-sbom-pod-flow
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
CORE_POD=""
get_core_pod() {
  CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
}

get_token() {
  get_core_pod
  [ -z "$CORE_POD" ] && { echo "Core pod not found in $NAMESPACE"; return 1; }
  kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token','') or '')" 2>/dev/null
}

# Check if pod name appears in GET /api/v1/sbom (with optional podName filter)
check_pod_in_sbom_api() {
  local pod_name="$1"
  local _ns="${2:-}"
  local token
  get_core_pod
  [ -z "$CORE_POD" ] && { echo "FAIL: Core pod not found"; return 1; }
  token=$(get_token) || true
  local filter=""
  [ -n "$pod_name" ] && filter="?podName=${pod_name}&limit=100"
  [ -z "$filter" ] && filter="?limit=100"
  local out
  out=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $token" "http://localhost:8080/api/v1/sbom${filter}" 2>/dev/null) || true
  if echo "$out" | grep -q "\"podName\":\"$pod_name\""; then
    echo "OK: Pod '$pod_name' found in SBOM API"
    return 0
  fi
  if echo "$out" | grep -q '"error"'; then
    echo "FAIL: API error: $(echo "$out" | head -c 200)"
    return 1
  fi
  echo "FAIL: Pod '$pod_name' not in SBOM list. Ensure: 1) Pod is Running on a node with agent, 2) Wait 1–3 min after pod start for SBOM extraction."
  return 1
}

# Resolve pod UID from name/namespace
get_pod_uid() {
  kubectl get pod "$1" -n "${2:-default}" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo ""
}

main() {
  cd "$PROJECT_ROOT"
  if [ "${1:-}" = "--full" ]; then
    echo "Running full SBOM flow test (create pod, wait for SBOM, verify API)..."
    exec "$SCRIPTS/e2e/test-sbom-pod-flow.sh" "$@"
  fi

  POD_NAME="${1:-pod-test}"
  POD_NS="${2:-default}"
  echo "=========================================="
  echo "E2E SBOM verify: $POD_NS/$POD_NAME"
  echo "=========================================="

  POD_UID=$(get_pod_uid "$POD_NAME" "$POD_NS")
  if [ -z "$POD_UID" ]; then
    echo "Pod $POD_NS/$POD_NAME not found. Create it first, e.g.:"
    echo "  kubectl run pod-test --image=nginx:stable --restart=Never -n default -- sleep 3600"
    exit 1
  fi
  echo "Pod UID: $POD_UID"
  echo ""

  if check_pod_in_sbom_api "$POD_NAME" "$POD_NS"; then
    echo ""
    echo "Dashboard: Open SBOM & Vulnerability Analysis, log in (admin / admin123), and confirm '$POD_NAME' appears in the list."
    echo "If list is empty, ensure you are logged in and refresh the page."
    exit 0
  fi
  exit 1
}

main "$@"
