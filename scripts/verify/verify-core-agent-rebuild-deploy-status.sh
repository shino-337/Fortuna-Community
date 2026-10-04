#!/usr/bin/env bash
# =============================================================================
# Check that Core and Agent run the images built and deployed from this workspace.
#
# How it checks:
# 1. Compares image IDs: running pod vs local image (nerdctl) fortuna-core:latest, fortuna-agent:latest.
# 2. A match means the pod runs the image built on this machine (loaded after build).
# 3. Reads the [Build] line from Core/Agent logs: version= commit= time= (with build args, the commit matches the current git HEAD).
#
# Run: ./scripts/verify/verify-core-agent-rebuild-deploy-status.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
# kubectl timeout (avoids hanging when the API is slow or unreachable). Skipped if `timeout` is unavailable.
KUBECTL_TIMEOUT="${KUBECTL_TIMEOUT:-15}"
kube() { if command -v timeout &>/dev/null; then timeout "$KUBECTL_TIMEOUT" kubectl "$@"; else kubectl "$@"; fi; }

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()   { echo -e "${GREEN}✅${NC} $1"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $1"; }
fail() { echo -e "${RED}❌${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

echo "=========================================="
echo "Core/Agent check – Rebuild & Deploy"
echo "=========================================="
echo ""

FAIL=0

# --- 1. Image tag trong deploy YAML (dùng tag này cho nerdctl, không hardcode latest) ---
echo "=== 1. Image trong deploy YAML ==="
CORE_YAML="${DEPLOY_CORE_YAML:-$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml}"
AGENT_YAML="${DEPLOY_AGENT_YAML:-$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml}"
# Extract tag from deploy YAML; supports local names and registry-qualified images.
# Fall back to latest if the file is missing or cannot be parsed.
get_image_tag() {
  local file="$1"
  local name="$2"
  [ -f "$file" ] || return 0
  grep -E "image:[[:space:]]*([^[:space:]#]+/)?${name}:" "$file" 2>/dev/null \
    | sed -E 's/.*'"${name}"':([^[:space:]#]+).*/\1/' \
    | tr -d '"' \
    | head -1
}
CORE_TAG=$(get_image_tag "$CORE_YAML" "fortuna-core")
AGENT_TAG=$(get_image_tag "$AGENT_YAML" "fortuna-agent")
CORE_TAG="${CORE_TAG:-latest}"
AGENT_TAG="${AGENT_TAG:-latest}"
info "fortuna-core:${CORE_TAG}  (from ${CORE_YAML##*/})"
info "fortuna-agent:${AGENT_TAG}  (from ${AGENT_YAML##*/})"
echo ""

# --- 2. Cluster và pod ---
echo "=== 2. Pod Core / Agent trên cluster ==="
if ! kube cluster-info &>/dev/null; then
  fail "Cannot reach the cluster (kubectl cluster-info)"
  FAIL=1
else
  ok "Cluster accessible"
fi

CORE_POD=$(kube get pods -n "$NAMESPACE" -l app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core --no-headers 2>/dev/null | head -1 | awk '{print $1}')
AGENT_POD=$(kube get pods -n "$NAMESPACE" -l app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent --no-headers 2>/dev/null | head -1 | awk '{print $1}')

if [ -z "$CORE_POD" ]; then
  warn "No Core pod found in namespace $NAMESPACE"
  CORE_IMAGE_ID=""
else
  ok "Core pod: $CORE_POD"
  CORE_IMAGE_ID=$(kube get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || echo "")
fi
if [ -z "$AGENT_POD" ]; then
  warn "No Agent pod found in namespace $NAMESPACE"
  AGENT_IMAGE_ID=""
else
  ok "Agent pod: $AGENT_POD"
  AGENT_IMAGE_ID=$(kube get pod -n "$NAMESPACE" "$AGENT_POD" -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || echo "")
fi
echo ""

# --- 3. Image local (nerdctl) ---
echo "=== 3. Image local (nerdctl, namespace=$CONTAINERD_NS) ==="
NERDCTL_BIN="${NERDCTL_BIN:-nerdctl}"
# Read Id from image inspect (same format as the Kubernetes imageID config digest); fall back to images -q.
get_local_image_id() {
  local img="$1"
  local id
  id=$("$NERDCTL_BIN" --namespace "$CONTAINERD_NS" image inspect "$img" --format '{{.Id}}' 2>/dev/null)
  [ -n "$id" ] && echo "$id" && return 0
  id=$("$NERDCTL_BIN" --namespace "$CONTAINERD_NS" images -q "$img" 2>/dev/null | head -1)
  [ -n "$id" ] && echo "$id"
}
if ! command -v "$NERDCTL_BIN" &>/dev/null; then
  warn "nerdctl not found; skipping local image ID comparison"
  LOCAL_CORE_ID=""
  LOCAL_AGENT_ID=""
else
  LOCAL_CORE_ID=$(get_local_image_id "fortuna-core:${CORE_TAG}")
  LOCAL_AGENT_ID=$(get_local_image_id "fortuna-agent:${AGENT_TAG}")
  if [ -z "$LOCAL_CORE_ID" ]; then
    warn "No local image fortuna-core:${CORE_TAG} (not built or removed; the build script tags :latest and :\$VERSION)"
  else
    info "fortuna-core:${CORE_TAG} local image ID: ${LOCAL_CORE_ID:0:19}..."
  fi
  if [ -z "$LOCAL_AGENT_ID" ]; then
    warn "No local image fortuna-agent:${AGENT_TAG} (not built or removed)"
  else
    info "fortuna-agent:${AGENT_TAG} local image ID: ${LOCAL_AGENT_ID:0:19}..."
  fi
fi
echo ""

# --- 4. So sánh image ID (pod vs local) ---
echo "=== 4. Image comparison (running pod vs local image) ==="
# Kubernetes imageID is the config digest (sha256:...). The local value comes from nerdctl image inspect {{.Id}} (same format).
# Compare the first 12 digest characters or the full string.
normalize_id() {
  local v="$1"
  if [[ "$v" == *sha256:* ]]; then
    echo "$v" | sed 's/.*sha256:\([a-f0-9]*\).*/\1/' | cut -c1-12
  else
    echo "$v" | tr -d '\n' | cut -c1-12
  fi
}

CORE_MATCH=false
AGENT_MATCH=false
if [ -n "$CORE_IMAGE_ID" ] && [ -n "$LOCAL_CORE_ID" ]; then
  POD_CORE_SHA=$(normalize_id "$CORE_IMAGE_ID")
  LOC_CORE_SHA=$(normalize_id "$LOCAL_CORE_ID")
  if [ "$POD_CORE_SHA" = "$LOC_CORE_SHA" ] || [ "$LOCAL_CORE_ID" = "$CORE_IMAGE_ID" ]; then
    ok "Core: pod runs the same image as local (rebuilt and deployed)"
    CORE_MATCH=true
  else
    warn "Core: pod image differs from local → rollout restart after rebuild (or not rebuilt yet)"
    info "  pod:  $CORE_IMAGE_ID"
    info "  local: $LOCAL_CORE_ID"
  fi
elif [ -z "$CORE_POD" ]; then
  warn "Core: no pod to compare"
elif [ -z "$LOCAL_CORE_ID" ]; then
  warn "Core: no local image to compare → run build-and-load-containerd.sh"
fi

if [ -n "$AGENT_IMAGE_ID" ] && [ -n "$LOCAL_AGENT_ID" ]; then
  POD_AGENT_SHA=$(normalize_id "$AGENT_IMAGE_ID")
  LOC_AGENT_SHA=$(normalize_id "$LOCAL_AGENT_ID")
  if [ "$POD_AGENT_SHA" = "$LOC_AGENT_SHA" ] || [ "$LOCAL_AGENT_ID" = "$AGENT_IMAGE_ID" ]; then
    ok "Agent: pod runs the same image as local (rebuilt and deployed)"
    AGENT_MATCH=true
  else
    warn "Agent: pod image differs from local → rollout restart after rebuild (or not rebuilt yet)"
    info "  pod:  $AGENT_IMAGE_ID"
    info "  local: $LOCAL_AGENT_ID"
  fi
elif [ -z "$AGENT_POD" ]; then
  warn "Agent: no pod to compare"
elif [ -z "$LOCAL_AGENT_ID" ]; then
  warn "Agent: no local image to compare → run build-and-load-containerd.sh"
fi
echo ""

# --- 5. Build info from logs (commit / time) ---
echo "=== 5. Build info trong log (version= commit= time=) ==="
CURRENT_COMMIT=$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "")

if [ -n "$CORE_POD" ]; then
  BUILD_LINE=$(kube logs -n "$NAMESPACE" "$CORE_POD" --tail=500 2>/dev/null | grep -m1 '\[Build\]' || true)
  if [ -n "$BUILD_LINE" ]; then
    info "Core: $BUILD_LINE"
    if [ -n "$CURRENT_COMMIT" ] && echo "$BUILD_LINE" | grep -q "commit=$CURRENT_COMMIT"; then
      ok "Core commit in the log matches the current git HEAD ($CURRENT_COMMIT)"
    elif echo "$BUILD_LINE" | grep -q "commit=none"; then
      warn "Core build has no commit (old build or no build args); rebuild with scripts/build/build-and-load-containerd.sh"
    fi
  else
    warn "Core: no [Build] line in the log"
  fi
fi
if [ -n "$AGENT_POD" ]; then
  BUILD_LINE=$(kube logs -n "$NAMESPACE" "$AGENT_POD" --tail=500 2>/dev/null | grep -m1 '\[Build\]' || true)
  if [ -n "$BUILD_LINE" ]; then
    info "Agent: $BUILD_LINE"
    if [ -n "$CURRENT_COMMIT" ] && echo "$BUILD_LINE" | grep -q "commit=$CURRENT_COMMIT"; then
      ok "Agent commit in the log matches the current git HEAD ($CURRENT_COMMIT)"
    elif echo "$BUILD_LINE" | grep -q "commit=none"; then
      warn "Agent build has no commit (old build); rebuild with scripts/build/build-and-load-containerd.sh"
    fi
  else
    warn "Agent: no [Build] line in the log"
  fi
fi
if [ -n "$CURRENT_COMMIT" ]; then
  info "Current git commit: $CURRENT_COMMIT"
fi
echo ""

# --- Result ---
echo "=========================================="
if [ "$CORE_MATCH" = true ] && [ "$AGENT_MATCH" = true ]; then
  echo -e "${GREEN}Result: Core and Agent run the same images as local (rebuilt and deployed).${NC}"
  echo "Local images are current and the pods use them."
  exit 0
fi

echo -e "${YELLOW}Result: not rebuilt/deployed yet, or the pods were not restarted.${NC}"
echo ""
echo "To make Core and Agent run the new code:"
echo "  1. Rebuild:  ./scripts/build/build-and-load-containerd.sh"
echo "     (or:    NO_CACHE=true ./scripts/build/build-and-load-containerd.sh)"
echo "  2. Deploy:   ./scripts/deploy/deploy-fortuna-robust.sh"
echo "     or:     ./scripts/pipeline/full-clean-database-rebuild-deploy.sh"
echo "  3. Restart:  kubectl rollout restart deployment/fortuna-core -n $NAMESPACE"
echo "               kubectl rollout restart daemonset/fortuna-agent -n $NAMESPACE"
echo "  4. Check again: ./scripts/verify/verify-core-agent-rebuild-deploy-status.sh"
echo ""
exit 0
