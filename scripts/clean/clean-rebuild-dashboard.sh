#!/bin/bash
# ============================================================================
# Clean and Rebuild Dashboard - Complete Process
# ============================================================================
# 1. Stop port-forwards
# 2. Delete old dashboard images (all versions)
# 3. Clean build cache
# 4. Delete dashboard deployment (force restart)
# 5. Rebuild dashboard image
# 6. Verify image
# 7. Restart deployment
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} ❌ $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"; }

echo "=========================================="
echo "Clean & Rebuild Dashboard - Complete"
echo "=========================================="
echo ""

# 1. Stop port-forwards
log_info "Step 1/7: Stopping port-forward processes..."
pkill -f "kubectl.*port-forward.*fortuna-dashboard" 2>/dev/null || true
pkill -f "kubectl.*port-forward.*fortuna-core" 2>/dev/null || true
sleep 1
log_success "Port-forwards stopped"

# 2. Delete old dashboard images (ALL versions including latest)
log_info "Step 2/7: Deleting ALL old dashboard images..."
DASH_IMAGES=$(nerdctl --namespace "${CONTAINERD_NS}" images | grep "fortuna-dashboard" | awk '{print $3}' || echo "")
if [ -n "$DASH_IMAGES" ]; then
  echo "$DASH_IMAGES" | while read -r img; do
    [ -n "$img" ] && nerdctl --namespace "${CONTAINERD_NS}" rmi "$img" 2>/dev/null || true
  done
  log_success "Deleted old dashboard images"
else
  log_warning "No dashboard images found to delete"
fi

# Also clean via ctr (more thorough)
log_info "Cleaning via ctr..."
ctr -n "${CONTAINERD_NS}" images ls -q 2>/dev/null | grep "fortuna-dashboard" | while read -r img; do
  [ -n "$img" ] && ctr -n "${CONTAINERD_NS}" images rm "$img" 2>/dev/null || true
done
log_success "ctr cleanup done"

# 3. Clean build cache
log_info "Step 3/7: Pruning build cache..."
nerdctl builder prune --namespace "${CONTAINERD_NS}" -f 2>/dev/null || true
nerdctl --namespace "${CONTAINERD_NS}" system prune -f 2>/dev/null || true
log_success "Build cache cleaned"

# 4. Delete dashboard deployment (force restart with new image)
log_info "Step 4/7: Deleting dashboard deployment (will recreate with new image)..."
kubectl delete deployment -n "${NAMESPACE}" fortuna-dashboard --ignore-not-found=true
sleep 3
log_success "Deployment deleted"

# 5. Rebuild dashboard image
log_info "Step 5/7: Rebuilding dashboard image..."
cd "${PROJECT_ROOT}"
if [ -f "$SCRIPTS/build/build-dashboard-containerd.sh" ]; then
  bash "$SCRIPTS/build/build-dashboard-containerd.sh"
else
  log_error "build-dashboard-containerd.sh not found"
  exit 1
fi
log_success "Dashboard image rebuilt"

# 6. Verify image
log_info "Step 6/7: Verifying new image..."
NEW_IMAGE=$(nerdctl --namespace "${CONTAINERD_NS}" images | grep "fortuna-dashboard.*latest" | head -1 | awk '{print $1":"$2}' || echo "")
if [ -n "$NEW_IMAGE" ]; then
  log_success "New image: $NEW_IMAGE"
  nerdctl --namespace "${CONTAINERD_NS}" images | grep "fortuna-dashboard" | head -3
else
  log_error "New image not found"
  exit 1
fi

# 7. Restart deployment
log_info "Step 7/7: Recreating dashboard deployment..."
if [ -f "${PROJECT_ROOT}/deploy/dashboard-deployment.yaml" ]; then
  kubectl apply -f "${PROJECT_ROOT}/deploy/dashboard-deployment.yaml"
  log_success "Deployment recreated"
  
  log_info "Waiting for deployment to be ready..."
  kubectl wait --for=condition=available --timeout=120s deployment/fortuna-dashboard -n "${NAMESPACE}" || {
    log_warning "Deployment not ready within 120s, checking status..."
    kubectl get pods -n "${NAMESPACE}" -l app=fortuna-dashboard
  }
  
  # Check pod status
  sleep 5
  POD_STATUS=$(kubectl get pods -n "${NAMESPACE}" -l app=fortuna-dashboard -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
  if [ "$POD_STATUS" = "Running" ]; then
    log_success "Dashboard pod is Running"
  else
    log_warning "Dashboard pod status: $POD_STATUS"
    kubectl get pods -n "${NAMESPACE}" -l app=fortuna-dashboard
  fi
else
  log_error "dashboard-deployment.yaml not found"
  exit 1
fi

echo ""
echo "=========================================="
log_success "Clean & Rebuild Complete!"
echo "=========================================="
echo ""
echo "Dashboard Status:"
kubectl get deployment,pods -n "${NAMESPACE}" -l app=fortuna-dashboard
echo ""
echo "Image in use:"
kubectl get deployment fortuna-dashboard -n "${NAMESPACE}" -o jsonpath='{.spec.template.spec.containers[0].image}' && echo
echo ""
echo "To access Dashboard:"
echo "  kubectl port-forward -n ${NAMESPACE} svc/fortuna-dashboard 8081:80"
echo "  Then open: http://localhost:8081"
echo ""
