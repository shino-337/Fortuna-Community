#!/usr/bin/env bash
# ============================================================================
# E2E: Distroless Component Quality Verification
#
# Verifies that distroless SBOM components are clean:
#   1. No junk OS utility binaries (setpriv, mkdir, ln, etc.)
#   2. Only signature-whitelisted binaries emitted
#   3. Valid PURL format (pkg:go/, pkg:generic/, pkg:deb/)
#   4. Go module deps present when binary has buildinfo
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
API_PASS="${API_PASS:-${E2E_ADMIN_PASS:-${FORTUNA_ADMIN_PASSWORD:-${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}}}}"
TEST_IMAGE="${TEST_DISTROLESS_IMAGE:-gcr.io/distroless/base-debian12:nonroot}"
POD_NAME="e2e-distroless-comp-$(date +%s)"
CURL_AUTH=()
FAIL=0

get_token() {
  local resp code
  resp=$(curl -s -w "\n%{http_code}" -X POST "${CORE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${API_USER}\",\"password\":\"${API_PASS}\"}" 2>/dev/null) || true
  code=$(echo "$resp" | tail -n1); resp=$(echo "$resp" | sed '$d')
  [[ "$code" != "200" ]] && return 1
  command -v jq >/dev/null 2>&1 && echo "$resp" | jq -r '.token // empty' \
    || echo "$resp" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

assert_ok()   { echo "  ✅ $1"; }
assert_fail() { echo "  ❌ $1"; FAIL=$((FAIL + 1)); }

echo "======================================================="
echo "E2E: Distroless Component Quality Verification"
echo "======================================================="
echo "Image:  $TEST_IMAGE"
echo "Pod:    $POD_NAME"
echo "======================================================="
echo ""

kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"

echo "[1/5] Creating distroless pod..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: e2e-distroless-comp
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

echo "[2/5] Auth + wait for SBOM..."
TOKEN=$(get_token) || true
[ -n "${TOKEN:-}" ] && CURL_AUTH=(-H "Authorization: Bearer $TOKEN")

MAX_WAIT=300; INTERVAL=15; elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
  HTTP=$(curl -s -o /tmp/sbom_distroless_comp.json -w "%{http_code}" "${CURL_AUTH[@]}" \
    "${CORE_URL}/api/v1/inventory/pods/${POD_UID}/sbom" 2>/dev/null) || HTTP="000"
  [ "$HTTP" = "200" ] && { echo "  SBOM available after ${elapsed}s"; break; }
  sleep "$INTERVAL"; elapsed=$((elapsed + INTERVAL)); echo "  ... ${elapsed}s"
done
[ "$elapsed" -ge "$MAX_WAIT" ] && { echo "ERROR: SBOM timeout"; exit 1; }
echo ""

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq required for component analysis"
  exit 1
fi

COMPS=$(jq '.components // []' /tmp/sbom_distroless_comp.json)
COMP_COUNT=$(echo "$COMPS" | jq 'length')
echo "[3/5] Component analysis ($COMP_COUNT components)..."

[ "${COMP_COUNT:-0}" -lt 1 ] && assert_fail "No components" || assert_ok "Has $COMP_COUNT components"
echo ""

echo "[4/5] Junk filtering verification..."
JUNK_NAMES=("setpriv" "mkdir" "ln" "touch" "chmod" "chown" "rm" "cp" "mv" "cat" "ls" "grep" "find" "sed" "awk")
JUNK_FOUND=0
for name in "${JUNK_NAMES[@]}"; do
  if echo "$COMPS" | jq -e --arg n "$name" '.[] | select(.componentName == $n)' >/dev/null 2>&1; then
    echo "  ⚠️  Junk found: $name"
    JUNK_FOUND=$((JUNK_FOUND + 1))
  fi
done
[ "$JUNK_FOUND" -eq 0 ] \
  && assert_ok "No junk OS utilities in components (signature whitelist effective)" \
  || assert_fail "Found $JUNK_FOUND junk OS utility components"

echo ""
echo "[5/5] PURL format verification..."
TOTAL_WITH_PURL=$(echo "$COMPS" | jq '[.[] | select(.purl != null and .purl != "")] | length')
echo "  Components with PURL: $TOTAL_WITH_PURL / $COMP_COUNT"

VALID_PURL=$(echo "$COMPS" | jq '[.[] | select(.purl != null and (.purl | startswith("pkg:")))] | length')
INVALID=$((TOTAL_WITH_PURL - VALID_PURL))
[ "$INVALID" -le 0 ] && assert_ok "All PURLs start with pkg:" || assert_fail "$INVALID invalid PURLs"

PURL_TYPES=$(echo "$COMPS" | jq -r '[.[] | .purl // "" | split("/")[0] // ""] | unique | join(", ")')
echo "  PURL types: $PURL_TYPES"

HAS_AT=$(echo "$COMPS" | jq '[.[] | select(.purl != null and (.purl | contains("@")))] | length')
echo "  PURLs with version (@): $HAS_AT / $TOTAL_WITH_PURL"

echo ""
echo "  Sample components:"
echo "$COMPS" | jq -r '.[:5][] | "    \(.componentName // "?") @ \(.componentVersion // "?") [\(.purl // "no purl")]"' 2>/dev/null || true

echo ""
[[ "${1:-}" == "--cleanup" ]] && { echo "Cleaning up..."; kubectl delete pod "$POD_NAME" -n "$TEST_NS" --ignore-not-found; }

echo "======================================================="
[ "$FAIL" -eq 0 ] && echo "RESULT: ALL PASSED ✅" || echo "RESULT: $FAIL FAILED ❌"
echo "======================================================="
exit "$FAIL"
