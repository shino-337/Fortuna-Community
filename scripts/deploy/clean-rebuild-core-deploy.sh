#!/usr/bin/env bash
# =============================================================================
# Clean → Rebuild Core only → Verify image → Push to nodes → Deploy → Rollout
# Use when Core is CrashLoopBackOff / Error and you want a full clean redeploy.
#
# Steps:
#   1. Delete Core deployment (clear stuck pods/ReplicaSets)
#   2. Remove fortuna-core images from local containerd (force fresh build)
#   3. Rebuild Core only (BUILD_CORE_ONLY=true, NO_CACHE=true)
#   4. Verify fortuna-core:latest exists in containerd
#   5. Push images to all nodes (core + agent; agent may be existing image)
#   6. Apply fortuna-core deployment YAML
#   7. Wait for rollout
#
# Usage:
#   ./scripts/deploy/clean-rebuild-core-deploy.sh
#   NAMESPACE=fortuna ./scripts/deploy/clean-rebuild-core-deploy.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"

NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()     { echo -e "${RED}[ERR]${NC} $1"; }

echo "=========================================="
echo "Clean → Rebuild Core → Verify → Push → Deploy → Rollout"
echo "=========================================="
echo "  NAMESPACE=$NAMESPACE  CONTAINERD_NS=$CONTAINERD_NS"
echo ""

# ---- Step 1: Delete Core deployment (clear stuck pods) ----
log_info "Step 1: Deleting Core deployment (clearing stuck pods/ReplicaSets)..."
if kubectl get deployment fortuna-core -n "$NAMESPACE" &>/dev/null; then
  kubectl delete deployment fortuna-core -n "$NAMESPACE" --timeout=30s || true
  log_ok "Core deployment deleted"
  sleep 2
else
  log_info "Core deployment not found (already deleted or not deployed)"
fi
echo ""

# ---- Step 2: Remove fortuna-core images from local containerd ----
log_info "Step 2: Removing fortuna-core images from local containerd..."
NERDCTL=""
for cmd in nerdctl "sudo nerdctl"; do
  if command -v ${cmd%% *} &>/dev/null; then
    NERDCTL="$cmd"
    break
  fi
done
if [ -n "$NERDCTL" ]; then
  $NERDCTL --namespace "$CONTAINERD_NS" images 2>/dev/null | grep "fortuna-core" | awk '{print $1":"$2}' | while read -r ref; do
    [ -n "$ref" ] && $NERDCTL --namespace "$CONTAINERD_NS" rmi --force "$ref" 2>/dev/null || true
  done
  log_ok "Local fortuna-core images removed"
else
  log_warn "nerdctl not found; skip local image clean"
fi
echo ""

# ---- Step 3: Rebuild Core only ----
log_info "Step 3: Rebuilding Core (BUILD_CORE_ONLY=true NO_CACHE=true)..."
cd "$PROJECT_ROOT"
if ! BUILD_CORE_ONLY=true NO_CACHE=true "$SCRIPTS/build/build-and-load-containerd.sh"; then
  log_err "Core build failed"
  exit 1
fi
log_ok "Core build complete"
echo ""

# ---- Step 4: Verify image fortuna-core:latest ----
log_info "Step 4: Verifying image fortuna-core:latest..."
CORE_SEEN=
for bin in nerdctl "sudo nerdctl"; do
  if command -v ${bin%% *} &>/dev/null; then
    if $bin --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -q "fortuna-core.*latest"; then
      CORE_SEEN=1
      log_ok "Image found (via $bin):"
      $bin --namespace "$CONTAINERD_NS" images 2>/dev/null | grep "fortuna-core" || true
      break
    fi
  fi
done
if [ -z "${CORE_SEEN:-}" ]; then
  if ctr -n "$CONTAINERD_NS" images list 2>/dev/null | grep -q "fortuna-core"; then
    CORE_SEEN=1
    log_ok "Image found (via ctr):"
    ctr -n "$CONTAINERD_NS" images list 2>/dev/null | grep "fortuna-core" || true
  fi
fi
if [ -z "${CORE_SEEN:-}" ]; then
  log_err "fortuna-core:latest not found in containerd after build. Abort."
  exit 1
fi
echo ""

# ---- Step 5: Push images to nodes ----
log_info "Step 5: Pushing images to all nodes (core + agent)..."
if [ -x "$SCRIPTS/utils/push-images-to-workers.sh" ]; then
  if "$SCRIPTS/utils/push-images-to-workers.sh" --clean-remote 2>&1; then
    log_ok "Images pushed to nodes"
  else
    log_warn "Push had errors (check SSH/config). Continuing deploy; Core may need image on node."
  fi
else
  log_warn "push-images-to-workers.sh not found; skip push. Ensure fortuna-core:latest is on the node that runs Core."
fi
echo ""

# ---- Step 6: Apply Core deployment ----
log_info "Step 6: Applying fortuna-core deployment..."
CORE_YAML="$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml"
if [ ! -f "$CORE_YAML" ]; then
  log_err "Not found: $CORE_YAML"
  exit 1
fi
kubectl apply -f "$CORE_YAML" -n "$NAMESPACE"
log_ok "Deployment applied (image: fortuna-core:latest)"
echo ""

# ---- Step 7: Wait for rollout ----
log_info "Step 7: Waiting for Core rollout (timeout 120s)..."
if kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>&1; then
  log_ok "Core rollout complete"
else
  log_warn "Rollout status failed or timed out. Check: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core"
  kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core 2>/dev/null || true
  kubectl describe deployment fortuna-core -n "$NAMESPACE" 2>/dev/null | tail -20 || true
  exit 1
fi
echo ""

echo "=========================================="
log_ok "Clean → Rebuild → Deploy finished. Core should be Running."
echo "=========================================="
echo "  Pods:    kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=core"
echo "  Logs:    kubectl logs -n $NAMESPACE deployment/fortuna-core -f"
echo "  Ready:   kubectl get endpoints -n $NAMESPACE fortuna-core"
echo ""
