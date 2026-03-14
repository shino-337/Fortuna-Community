#!/usr/bin/env bash
# ============================================================================
# E2E: SBOM flow for a distroless test pod
# - Uses an image built FROM gcr.io/distroless/static-debian12
# - Container runs a simple binary that prints "hello word"
# - Verifies that Core exposes SBOM for this pod via API
# ============================================================================
# Prerequisites:
#   - Core running and reachable
#   - Agent running on the node where the pod is scheduled
#   - kubeconfig pointing to target cluster
#
# Options:
#   Mode A (auto build with nerdctl + containerd):
#     - Uses deploy/e2e/images/distroless-hello/Dockerfile as build context
#     - Requires nerdctl (namespace k8s.io) and containerd socket
#   Mode B (pre-built image):
#     - Set TEST_DISTROLESS_IMAGE to an existing image tag
#
# Env:
#   TEST_DISTROLESS_IMAGE   # optional; if empty, script will build local image
#   CORE_API_URL            # default: http://localhost:8080
#   API_USER                # default: admin
#   API_PASS                # default: admin123
#   TEST_NS                 # default: fortuna
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
IMAGES_DIR="${PROJECT_ROOT}/deploy/e2e/images/distroless-hello"

NAMESPACE="${NAMESPACE:-fortuna}"
CORE_URL="${CORE_API_URL:-http://localhost:8080}"
TEST_NS="${TEST_NS:-fortuna}"
API_USER="${API_USER:-admin}"
API_PASS="${API_PASS:-admin123}"
POD_NAME="sbom-distroless-hello-$(date +%s)"
AUTH_HEADER=""

TEST_DISTROLESS_IMAGE="${TEST_DISTROLESS_IMAGE:-}"
build_and_load_image() {
  local image_tag="${1:-distroless-hello:e2e}"
  local dockerfile="${IMAGES_DIR}/Dockerfile"

  if [ ! -f "$dockerfile" ]; then
    echo "ERROR: $dockerfile not found."
    echo "Create Dockerfile and main.go in $IMAGES_DIR, for example:"
    echo ""
    echo "  ${IMAGES_DIR}/main.go:"
    echo "    package main"
    echo "    import \"fmt\""
    echo "    func main() { fmt.Println(\"hello word\") }"
    echo ""
    echo "  ${IMAGES_DIR}/Dockerfile:"
    echo "    FROM golang:1.22 AS builder"
    echo "    WORKDIR /app"
    echo "    COPY main.go ."
    echo "    RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /hello main.go"
    echo "    FROM gcr.io/distroless/static-debian12"
    echo "    COPY --from=builder /hello /hello"
    echo "    ENTRYPOINT [\"/hello\"]"
    exit 1
  fi

  echo "Building distroless hello image with nerdctl (namespace k8s.io)..."
  nerdctl --namespace k8s.io build -t "$image_tag" -f "$dockerfile" "$IMAGES_DIR"

  echo "Saving image to tarball and loading into current containerd..."
  local tmp_tar
  tmp_tar="$(mktemp /tmp/distroless-hello-image.XXXXXX.tar)"
  nerdctl --namespace k8s.io save "$image_tag" -o "$tmp_tar"

  # Load into default containerd namespace used by Kubernetes (k8s.io)
  ctr --namespace k8s.io images import "$tmp_tar"
  rm -f "$tmp_tar"

  TEST_DISTROLESS_IMAGE="$image_tag"
}

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
    echo "$resp" | sed -n 's/.*\"token\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p'
  fi
}

if [ -z "$TEST_DISTROLESS_IMAGE" ]; then
  build_and_load_image "distroless-hello:e2e"
fi

echo "======================================================="
echo "SBOM Distroless Hello-Word E2E Test"
echo "======================================================="
echo "Namespace:      $TEST_NS"
echo "Test pod:       $POD_NAME"
echo "Core URL:       $CORE_URL/api/v1"
echo "Image:          $TEST_DISTROLESS_IMAGE"
echo "======================================================="
echo ""

echo "[1/4] Creating distroless test pod $POD_NAME..."

# Ensure namespace exists (mirrors other e2e tests)
kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: sbom-distroless-hello
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: k8s-master
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule
  containers:
    - name: sbom-distroless-hello
      image: $TEST_DISTROLESS_IMAGE
      imagePullPolicy: Never
EOF
echo ""

echo "[2/4] Waiting for pod to be Ready or Completed..."
kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$TEST_NS" --timeout=120s || true
kubectl get pod "$POD_NAME" -n "$TEST_NS"
POD_UID=$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
  echo "Could not get pod UID. Pod may not be ready."
  exit 1
fi
echo "  Pod UID: $POD_UID"
echo ""

echo "[2b] Fetching auth token (if required)..."
TOKEN=$(get_token) || true
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer $TOKEN"
  echo "  API auth: using token (user: $API_USER)"
else
  AUTH_HEADER=""
  echo "  API auth: no token – API calls may return 401 if auth is enabled."
fi
CURL_AUTH=()
[ -n "$AUTH_HEADER" ] && CURL_AUTH=(-H "$AUTH_HEADER")
echo ""

echo "[3/4] Waiting for SBOM to appear in API..."
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
  echo "  WARNING: SBOM did not appear within ${MAX_WAIT}s."
  echo "    - Check agent logs:"
  echo "        kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=agent --tail=80"
  echo "    - Check pod/node:"
  echo "        kubectl get pod $POD_NAME -n $TEST_NS -o wide"
fi
echo ""

echo "[4/4] Verifying SBOM detail API for this pod..."
echo "  GET /api/v1/inventory/pods/$POD_UID/sbom"
HTTP=$(curl -s -o /tmp/sbom_distroless_detail.json -w "%{http_code}" "${CURL_AUTH[@]}" "${CORE_URL}/api/v1/inventory/pods/${POD_UID}/sbom")
if [ "$HTTP" = "200" ]; then
  echo "  HTTP 200 OK"
  head -c 400 /tmp/sbom_distroless_detail.json
  echo ""
else
  echo "  HTTP $HTTP"
  [ -s /tmp/sbom_distroless_detail.json ] && head -c 200 /tmp/sbom_distroless_detail.json
  echo ""
  if [ "$HTTP" = "401" ]; then
    echo "  Tip: Set API_USER and API_PASS (e.g. export API_USER=admin API_PASS=yourpassword)"
  fi
fi
echo ""

if [[ "${1:-}" == "--cleanup" ]]; then
  echo "Cleaning up pod $POD_NAME..."
  kubectl delete pod "$POD_NAME" -n "$TEST_NS" --ignore-not-found
fi

echo "======================================================="
echo "SBOM Distroless Hello-Word E2E Test Complete"
echo "======================================================="

