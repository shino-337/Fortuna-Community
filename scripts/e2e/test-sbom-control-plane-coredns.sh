#!/usr/bin/env bash
# ============================================================================
# E2E: SBOM extraction for a control-plane–style workload (CoreDNS image)
#
# Deploys CoreDNS with a minimal Corefile on :1053. Agent should produce SBOM
# with components suitable for go-binary / generic control-plane checks.
#
# Env: CORE_API_URL, API_USER, API_PASS, TEST_NS, NAMESPACE, TEST_COREDNS_IMAGE
#      SKIP_IF_NO_PULL=1 — exit 0 if rollout/pull fails
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
DEPLOY_NAME="e2e-coredns-sbom"
CM_NAME="e2e-coredns-corefile"
TEST_COREDNS_IMAGE="${TEST_COREDNS_IMAGE:-registry.k8s.io/coredns/coredns:v1.11.1}"
CURL_AUTH=()

get_token() {
  local resp
  resp=$(curl -s -w "\n%{http_code}" -X POST "${CORE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${API_USER}\",\"password\":\"${API_PASS}\"}" 2>/dev/null) || true
  local code
  code=$(echo "$resp" | tail -n1)
  resp=$(echo "$resp" | sed '$d')
  if [ -z "$code" ] || [ "$code" != "200" ]; then
    return 1
  fi
  if command -v jq >/dev/null 2>&1; then
    echo "$resp" | jq -r '.token // empty'
  else
    echo "$resp" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^\"]*\)".*/\1/p'
  fi
}

echo "======================================================="
echo "SBOM E2E: CoreDNS (control-plane style image)"
echo "======================================================="
echo "Namespace:   $TEST_NS"
echo "Deployment:  $DEPLOY_NAME"
echo "Image:       $TEST_COREDNS_IMAGE"
echo "Core API:    $CORE_URL/api/v1"
echo "======================================================="

kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"

echo "[1/5] Applying ConfigMap + Deployment..."

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${CM_NAME}
  namespace: ${TEST_NS}
data:
  Corefile: |
    .:1053 {
        forward . 8.8.8.8
        health :8080
        errors
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${DEPLOY_NAME}
  namespace: ${TEST_NS}
  labels:
    app.kubernetes.io/name: e2e-coredns-sbom
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${DEPLOY_NAME}
  template:
    metadata:
      labels:
        app: ${DEPLOY_NAME}
    spec:
      nodeSelector:
        kubernetes.io/hostname: k8s-master
      tolerations:
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
      containers:
        - name: coredns
          image: ${TEST_COREDNS_IMAGE}
          imagePullPolicy: IfNotPresent
          args: ["-conf", "/etc/coredns/Corefile"]
          ports:
            - containerPort: 1053
              name: dns
              protocol: UDP
            - containerPort: 8080
              name: health
          volumeMounts:
            - name: cfg
              mountPath: /etc/coredns
              readOnly: true
      volumes:
        - name: cfg
          configMap:
            name: ${CM_NAME}
EOF

echo "[2/5] Waiting for rollout..."
if ! kubectl rollout status "deployment/${DEPLOY_NAME}" -n "$TEST_NS" --timeout=180s; then
  echo "WARNING: rollout failed — describe:"
  kubectl describe deployment "$DEPLOY_NAME" -n "$TEST_NS" || true
  kubectl get pods -n "$TEST_NS" -l "app=${DEPLOY_NAME}" -o wide || true
  if [ "${SKIP_IF_NO_PULL:-}" = "1" ]; then
    echo "SKIP_IF_NO_PULL=1: exiting 0"
    exit 0
  fi
  exit 1
fi

POD_NAME=$(kubectl get pods -n "$TEST_NS" -l "app=${DEPLOY_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD_NAME" ]; then
  echo "ERROR: no pod for deployment"
  exit 1
fi
POD_UID=$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}')
echo "  Pod: $POD_NAME  UID: $POD_UID"
echo ""

TOKEN=$(get_token) || true
if [ -n "${TOKEN:-}" ]; then
  CURL_AUTH=(-H "Authorization: Bearer $TOKEN")
  echo "[3/5] API auth: token OK"
else
  echo "[3/5] API auth: no token"
fi
echo ""

echo "[4/5] Waiting for SBOM detail in API (up to 300s)..."
MAX_WAIT=300
INTERVAL=15
elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
  HTTP=$(curl -s -o /tmp/sbom_coredns_detail_poll.json -w "%{http_code}" "${CURL_AUTH[@]}"     "${CORE_URL}/api/v1/inventory/pods/${POD_UID}/sbom" 2>/dev/null) || HTTP="000"
  if [ "$HTTP" = "200" ]; then
    echo "  SBOM detail available for pod UID after ${elapsed}s"
    break
  fi
  sleep "$INTERVAL"
  elapsed=$((elapsed + INTERVAL))
  echo "  ... ${elapsed}s"
done
if [ "$elapsed" -ge "$MAX_WAIT" ]; then
  echo "  ERROR: SBOM detail not available for pod UID within ${MAX_WAIT}s."
  echo "    kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent --tail=100"
  exit 1
fi

echo "[5/5] GET inventory SBOM detail + assertions..."
HTTP=$(curl -s -o /tmp/sbom_coredns_detail.json -w "%{http_code}" "${CURL_AUTH[@]}" \
  "${CORE_URL}/api/v1/inventory/pods/${POD_UID}/sbom")
if [ "$HTTP" != "200" ]; then
  echo "  HTTP $HTTP"
  exit 1
fi
echo "  HTTP 200"

if ! command -v jq >/dev/null 2>&1; then
  echo "  SKIP: install jq for component assertions"
else
  COMP_COUNT=$(jq '.components | length' /tmp/sbom_coredns_detail.json 2>/dev/null || echo "0")
  if [ "${COMP_COUNT:-0}" -lt 1 ]; then
    echo "  FAIL: expected >=1 component, got $COMP_COUNT"
    exit 1
  fi
  echo "  OK: components count = $COMP_COUNT"
  HIT=$(jq '[.components[]? | (.name + " " + (.purl // ""))] | join(" ") | test("coredns|CoreDNS|go-binary|pkg:generic"; "i")' /tmp/sbom_coredns_detail.json 2>/dev/null || echo "false")
  if [ "$HIT" != "true" ]; then
    echo "  WARN: no obvious coredns/go-binary hit (dump first components):"
    jq '.components[:5]' /tmp/sbom_coredns_detail.json 2>/dev/null || head -c 400 /tmp/sbom_coredns_detail.json
    echo ""
  else
    echo "  OK: component text matches coredns / go-binary / pkg:generic"
  fi
  GV=$(jq -r '.goVersion // empty' /tmp/sbom_coredns_detail.json 2>/dev/null || echo "")
  if [ -n "$GV" ]; then
    echo "  INFO: goVersion = $GV"
  fi
fi

if [[ "${1:-}" == "--cleanup" ]]; then
  kubectl delete deployment "$DEPLOY_NAME" -n "$TEST_NS" --ignore-not-found
  kubectl delete configmap "$CM_NAME" -n "$TEST_NS" --ignore-not-found
fi

echo "======================================================="
echo "SBOM CoreDNS E2E complete"
echo "======================================================="
