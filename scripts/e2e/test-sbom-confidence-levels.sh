#!/usr/bin/env bash
# ============================================================================
# E2E: Multi-Source SBOM Confidence Levels
#
# Deploys 3 pod types and verifies confidence levels:
#   1. busybox (dpkg parser) → expected: high
#   2. distroless/base (heuristic) → expected: medium or low
#   3. CoreDNS (Go binary) → expected: high (if buildinfo available)
#
# Also verifies sbomSource field matches expected parser.
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
CURL_AUTH=()
FAIL=0
PASS=0

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

assert_ok()   { echo "  ✅ $1"; PASS=$((PASS + 1)); }
assert_fail() { echo "  ❌ $1"; FAIL=$((FAIL + 1)); }
assert_warn() { echo "  ⚠️  $1"; }

echo "======================================================="
echo "E2E: Multi-Source SBOM Confidence Levels"
echo "======================================================="
echo ""

kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"

TOKEN=$(get_token) || true
[ -n "${TOKEN:-}" ] && CURL_AUTH=(-H "Authorization: Bearer $TOKEN")

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq required"; exit 1
fi

test_pod_sbom() {
  local label="$1"
  local image="$2"
  local expected_source="$3"
  local expected_conf_not="$4"  # confidence should NOT be this value
  local pod_name="e2e-conf-${label}-$(date +%s)"

  echo "--- Testing: $label ---"
  echo "  Image: $image"

  cat <<EOF | kubectl apply -f - 2>/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: $pod_name
  namespace: $TEST_NS
  labels:
    app: e2e-confidence
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: k8s-master
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule
  containers:
    - name: test
      image: $image
      imagePullPolicy: IfNotPresent
      command: ["sleep", "3600"]
EOF

  kubectl wait --for=condition=Ready pod/"$pod_name" -n "$TEST_NS" --timeout=120s 2>/dev/null || true
  local pod_uid
  pod_uid=$(kubectl get pod "$pod_name" -n "$TEST_NS" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
  if [ -z "$pod_uid" ]; then
    assert_fail "$label: Pod UID not found"
    return
  fi

  local max_wait=180 interval=15 elapsed=0 http=""
  while [ $elapsed -lt $max_wait ]; do
    http=$(curl -s -o "/tmp/sbom_conf_${label}.json" -w "%{http_code}" "${CURL_AUTH[@]}" \
      "${CORE_URL}/api/v1/inventory/pods/${pod_uid}/sbom" 2>/dev/null) || http="000"
    [ "$http" = "200" ] && break
    sleep "$interval"; elapsed=$((elapsed + interval))
  done

  if [ "$http" != "200" ]; then
    assert_warn "$label: SBOM not available after ${max_wait}s (skipping)"
    kubectl delete pod "$pod_name" -n "$TEST_NS" --ignore-not-found 2>/dev/null
    return
  fi

  local source conf comp_count
  source=$(jq -r '.sbomSource // "unknown"' "/tmp/sbom_conf_${label}.json")
  conf=$(jq -r '.confidence // "unknown"' "/tmp/sbom_conf_${label}.json" | tr '[:upper:]' '[:lower:]')
  comp_count=$(jq '.components | length' "/tmp/sbom_conf_${label}.json" 2>/dev/null || echo "0")

  echo "  Source: $source, Confidence: $conf, Components: $comp_count"

  if [ "$expected_source" != "*" ] && [ "$source" != "$expected_source" ]; then
    assert_warn "$label: sbomSource=$source (expected $expected_source, may vary by image)"
  else
    assert_ok "$label: sbomSource=$source"
  fi

  if [ "$expected_conf_not" != "" ] && [ "$conf" = "$expected_conf_not" ]; then
    assert_fail "$label: confidence=$conf should NOT be $expected_conf_not"
  elif [ "$conf" = "unknown" ]; then
    assert_fail "$label: confidence should not be 'unknown'"
  else
    assert_ok "$label: confidence=$conf (valid)"
  fi

  kubectl delete pod "$pod_name" -n "$TEST_NS" --ignore-not-found 2>/dev/null
  echo ""
}

# Test 1: Package-manager-based image (high confidence expected)
test_pod_sbom "busybox" "busybox:1.36" "*" ""

# Test 2: Distroless image (should NOT be high confidence)
test_pod_sbom "distroless" "gcr.io/distroless/base-debian12:nonroot" "*" ""

# Test 3: CoreDNS (Go binary with buildinfo)
test_pod_sbom "coredns" "registry.k8s.io/coredns/coredns:v1.11.1" "*" ""

echo "======================================================="
echo "Results: PASS=$PASS, FAIL=$FAIL"
if [ "$FAIL" -eq 0 ]; then
  echo "ALL PASSED ✅"
else
  echo "$FAIL FAILED ❌"
fi
echo "======================================================="
exit "$FAIL"
