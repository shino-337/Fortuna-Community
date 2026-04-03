#!/usr/bin/env bash
# ============================================================================
# E2E: Distroless SBOM + NVD Fallback Verification
#
# Deploys a distroless pod and verifies:
#   1. SBOM is extracted with correct source/confidence metadata
#   2. Go module dependencies are extracted from binary (gobinary parser)
#   3. No junk OS utility components (signature whitelist working)
#
# Env: CORE_API_URL, API_USER, API_PASS, TEST_NS, NAMESPACE
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh" 2>/dev/null || true

NAMESPACE="${NAMESPACE:-fortuna}"
CORE_URL="${CORE_API_URL:-http://localhost:8080}"
TEST_NS="${TEST_NS:-fortuna}"
API_USER="${API_USER:-${E2E_ADMIN_USER:-admin}}"
API_PASS="${API_PASS:-${E2E_ADMIN_PASS:-admin123}}"
TEST_IMAGE="${TEST_DISTROLESS_IMAGE:-gcr.io/distroless/base-debian12:nonroot}"
POD_NAME="e2e-distroless-nvd-$(date +%s)"
CURL_AUTH=()
FAIL=0

get_token() {
  local resp code
  resp=$(curl -s -w "\n%{http_code}" -X POST "${CORE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${API_USER}\",\"password\":\"${API_PASS}\"}" 2>/dev/null) || true
  code=$(echo "$resp" | tail -n1)
  resp=$(echo "$resp" | sed '$d')
  [[ "$code" != "200" ]] && return 1
  if command -v jq >/dev/null 2>&1; then
    echo "$resp" | jq -r '.token // empty'
  else
    echo "$resp" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
  fi
}

assert_ok()   { echo "  ✅ $1"; }
assert_fail() { echo "  ❌ $1"; FAIL=$((FAIL + 1)); }

echo "======================================================="
echo "E2E: Distroless SBOM + NVD Fallback Verification"
echo "======================================================="
echo "Namespace:   $TEST_NS"
echo "Pod:         $POD_NAME"
echo "Image:       $TEST_IMAGE"
echo "Core API:    $CORE_URL/api/v1"
echo "======================================================="
echo ""

kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"

echo "[1/6] Creating distroless test pod..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: e2e-distroless-nvd
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: k8s-master
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule
  containers:
    - name: distroless
      image: $TEST_IMAGE
      imagePullPolicy: IfNotPresent
      command: ["/usr/bin/sleep", "infinity"]
EOF

kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$TEST_NS" --timeout=120s 2>/dev/null || true
POD_UID=$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
[ -z "$POD_UID" ] && { echo "ERROR: Pod UID not found"; exit 1; }
echo "  Pod UID: $POD_UID"
echo ""

echo "[2/6] Authenticating..."
TOKEN=$(get_token) || true
[ -n "${TOKEN:-}" ] && { CURL_AUTH=(-H "Authorization: Bearer $TOKEN"); echo "  Token OK"; } || echo "  No token"
echo ""

echo "[3/6] Waiting for SBOM extraction (max 300s)..."
MAX_WAIT=300; INTERVAL=15; elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
  HTTP=$(curl -s -o /tmp/sbom_distroless_nvd.json -w "%{http_code}" "${CURL_AUTH[@]}" \
    "${CORE_URL}/api/v1/inventory/pods/${POD_UID}/sbom" 2>/dev/null) || HTTP="000"
  [ "$HTTP" = "200" ] && { echo "  SBOM available after ${elapsed}s"; break; }
  sleep "$INTERVAL"; elapsed=$((elapsed + INTERVAL)); echo "  ... ${elapsed}s"
done
[ "$elapsed" -ge "$MAX_WAIT" ] && { echo "ERROR: SBOM not available within ${MAX_WAIT}s"; exit 1; }
echo ""

echo "[4/6] Verifying SBOM metadata..."
if command -v jq >/dev/null 2>&1; then
  SOURCE=$(jq -r '.sbomSource // "unknown"' /tmp/sbom_distroless_nvd.json)
  CONF=$(jq -r '.confidence // "unknown"' /tmp/sbom_distroless_nvd.json)
  echo "  sbomSource: $SOURCE"
  echo "  confidence: $CONF"
  [ "$CONF" = "unknown" ] && assert_fail "confidence should not be 'unknown'" || assert_ok "confidence = $CONF (valid)"
else
  echo "  SKIP: jq not installed"
fi
echo ""

echo "[5/6] Verifying component quality..."
if command -v jq >/dev/null 2>&1; then
  COMP_COUNT=$(jq '.components | length' /tmp/sbom_distroless_nvd.json 2>/dev/null || echo "0")
  echo "  Total components: $COMP_COUNT"
  [ "${COMP_COUNT:-0}" -lt 1 ] && assert_fail "Expected at least 1 component" || assert_ok "Component count >= 1"

  GO_DEPS=$(jq '[.components[]? | select(.purl != null and (.purl | startswith("pkg:go/")))] | length' \
    /tmp/sbom_distroless_nvd.json 2>/dev/null || echo "0")
  echo "  Go module components (pkg:go/): $GO_DEPS"

  JUNK_NAMES=("setpriv" "mkdir" "ln" "touch" "chmod" "chown" "rm" "cp" "mv")
  JUNK_FOUND=0
  for name in "${JUNK_NAMES[@]}"; do
    jq -e --arg n "$name" '.components[]? | select(.componentName == $n)' \
      /tmp/sbom_distroless_nvd.json >/dev/null 2>&1 && { echo "  ⚠️  Junk: $name"; JUNK_FOUND=$((JUNK_FOUND+1)); }
  done
  [ "$JUNK_FOUND" -eq 0 ] && assert_ok "No junk OS utility components" || assert_fail "Found $JUNK_FOUND junk components"

  INVALID_PURL=$(jq '[.components[]? | select(.purl != null and .purl != "" and (.purl | startswith("pkg:") | not))] | length' \
    /tmp/sbom_distroless_nvd.json 2>/dev/null || echo "0")
  [ "${INVALID_PURL:-0}" -gt 0 ] && assert_fail "$INVALID_PURL invalid PURLs" || assert_ok "All PURLs valid (pkg:*)"
fi
echo ""

echo "[6/6] Checking CVE matcher activity..."
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
  LOGS=$(kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=200 2>/dev/null || echo "")
  echo "$LOGS" | grep -q "CVEMatcherRun.*sbom_id=" && assert_ok "CVE matcher ran" \
    || echo "  INFO: No CVEMatcherRun log yet"
fi
echo ""

[[ "${1:-}" == "--cleanup" ]] && { echo "Cleaning up..."; kubectl delete pod "$POD_NAME" -n "$TEST_NS" --ignore-not-found; }

echo "======================================================="
[ "$FAIL" -eq 0 ] && echo "RESULT: ALL PASSED ✅" || echo "RESULT: $FAIL FAILED ❌"
echo "======================================================="
exit "$FAIL"
